package collaborationrun

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var ErrProofPending = errors.New("collaboration sealed proof awaits acknowledgement")

// Runner is the single-attempt daemon adapter. Ordinary providers, task tokens,
// automatic fallback and squad delegation are intentionally absent here.
type Runner struct {
	Ledger           *Ledger
	Client           *Client
	Controller       *NamespaceController
	StopPollInterval time.Duration
}

func (r *Runner) Run(ctx context.Context, runID string, request protocol.CollaborationRunClaimRequest) error {
	if r.Ledger == nil || r.Client == nil {
		return ErrUnsupported
	}
	local, err := r.Ledger.Lock(request.DeploymentID, request.InstallationID, runID)
	if err != nil {
		return err
	}
	defer local.Close()
	if local.record != nil {
		switch local.record.Phase {
		case phaseAcknowledged:
			return nil
		case phaseReaped, phaseNeverLaunched, phaseSealed, phaseProof:
			return r.finish(ctx, local)
		default:
			return ErrUnconfirmed
		}
	}
	if r.Controller == nil {
		return ErrUnsupported
	}
	if err := r.Controller.Probe(ctx); err != nil {
		return err
	}
	claim, err := r.Client.Claim(ctx, runID, request)
	if err != nil {
		return err
	}
	if err := local.Register(claim.Identity, claim.TerminationToken); err != nil {
		return err
	}
	permitRequest := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if err := local.RequestPermit(permitRequest.AttemptSeq, permitRequest.AttemptNonce); err != nil {
		return err
	}
	permit, err := r.Client.Permit(ctx, permitRequest)
	if err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	if permit.BusinessToken == claim.TerminationToken {
		return ErrUnconfirmed
	}
	if err := local.Authorize(); err != nil {
		return err
	}
	stop, stopErr := r.Client.StopIntent(ctx, claim.Identity, claim.TerminationToken)
	if stopErr != nil || stop.StopRequested || ctx.Err() != nil {
		if err := local.NeverLaunched(); err != nil {
			return err
		}
		return errors.Join(stopErr, r.finish(context.WithoutCancel(ctx), local))
	}
	// This is the last durable transition before any fork. No code path returns
	// from a burned/unknown record to authorization, including after restart.
	if err := local.Burn(); err != nil {
		return err
	}
	process, err := r.Controller.Start(ctx, ProviderInput{Identity: claim.Identity, Inputs: claim.Inputs, BusinessToken: permit.BusinessToken, ServerURL: r.Client.baseURL})
	if err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	if err := local.RecordProcess(process.NamespaceID()); err != nil {
		_ = process.Stop()
		_, _ = process.Wait()
		return err
	}
	result, err := r.observe(ctx, process, claim.Identity, claim.TerminationToken)
	if err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	if err := local.Reap(result.ExitCode); err != nil {
		return err
	}
	// A process exit is never a business result. Only the explicit provider
	// record may be uploaded, using the still-current business capability.
	var outcome protocol.CollaborationProviderResult
	var outcomeErr error
	if !result.OutputTruncated && decodeStrict(result.Output, &outcome) == nil && protocol.ValidateCollaborationProviderResult(outcome) == nil {
		outcomeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		outcomeErr = r.Client.Outcome(outcomeCtx, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, Outcome: outcome.Outcome, Body: outcome.Body, Record: outcome.Record}, permit.BusinessToken)
		cancel()
	}
	// Revoked business rights or a missing final result must not block the
	// narrow proof path. The server marks absent outcomes as unconfirmed.
	return errors.Join(outcomeErr, r.finish(context.WithoutCancel(ctx), local))
}

func (r *Runner) finish(ctx context.Context, local *RunLedger) error {
	proof, err := local.Seal()
	if err != nil {
		return err
	}
	proofCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	receipt, err := r.Client.Proof(proofCtx, proof, local.record.TerminationToken)
	if err != nil {
		return errors.Join(ErrProofPending, err)
	}
	return local.Acknowledge(receipt.ReceiptID)
}

func (r *Runner) observe(ctx context.Context, p *Process, id protocol.CollaborationRunIdentity, token string) (ContainmentResult, error) {
	type result struct {
		value ContainmentResult
		err   error
	}
	done := make(chan result, 1)
	go func() { value, err := p.Wait(); done <- result{value, err} }()
	interval := r.StopPollInterval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	cancelled := ctx.Done()
	var deadline <-chan time.Time
	var stopTimer *time.Timer
	defer func() {
		if stopTimer != nil {
			stopTimer.Stop()
		}
	}()
	stop := func() {
		if stopTimer != nil {
			return
		}
		_ = p.Stop()
		ticker.Stop()
		cancelled = nil
		stopTimer = time.NewTimer(10 * time.Second)
		deadline = stopTimer.C
	}
	for {
		select {
		case completed := <-done:
			return completed.value, completed.err
		case <-cancelled:
			stop()
		case <-deadline:
			return ContainmentResult{}, ErrUnconfirmed
		case <-ticker.C:
			intent, err := r.Client.StopIntent(ctx, id, token)
			if err != nil || intent.StopRequested {
				stop()
			}
		}
	}
}
