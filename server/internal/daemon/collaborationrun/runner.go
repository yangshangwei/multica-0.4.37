package collaborationrun

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var ErrProofPending = errors.New("collaboration sealed proof awaits acknowledgement")

// Launcher is the containment boundary. NamespaceController is the production
// implementation; tests substitute a fake that never resolves an agent CLI.
type Launcher interface {
	Probe(context.Context) error
	Start(context.Context, ProviderInput) (*Process, error)
}

// Runner is the single-attempt daemon adapter. Ordinary providers, task tokens,
// automatic fallback and squad delegation are intentionally absent here.
type Runner struct {
	Ledger           *Ledger
	Client           *Client
	Controller       Launcher
	Host             *HostExecutor
	StopPollInterval time.Duration
	// PermitAttempts bounds replays of the same attempt seq/nonce after a lost
	// or transient response. Zero values use the defaults below.
	PermitAttempts     int
	PermitBackoff      time.Duration
	RuntimeConfigEpoch string
}

const (
	defaultPermitAttempts = 3
	defaultPermitBackoff  = 250 * time.Millisecond
)

func (r *Runner) Run(ctx context.Context, runID string, request protocol.CollaborationRunClaimRequest) error {
	if r.Ledger == nil || r.Client == nil {
		return ErrUnsupported
	}
	local, err := r.Ledger.Lock(request.DeploymentID, request.InstallationID, runID)
	if err != nil {
		return err
	}
	defer local.Close()
	if request.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion || local.record != nil && local.record.Version == 2 {
		return r.runHost(ctx, local, runID, request)
	}
	if local.record != nil {
		switch local.record.Phase {
		case phaseAcknowledged:
			return nil
		case phaseReaped, phaseNeverLaunched, phaseSealed, phaseProof:
			return r.finish(ctx, local)
		case phaseRegistered, phasePermitPending, phaseAuthorized:
			// Nothing forked before the durable burn. The original claim inputs
			// are not retained, so converge instead of relaunching. Recovery uses
			// the caller's shared deadline; foreground abandonment below may
			// still finish a proof after execution shutdown.
			if err := local.NeverLaunched(); err != nil {
				return err
			}
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
	permit, err := r.permit(ctx, permitRequest)
	if err != nil {
		// An explicit refusal or exhausted replay leaves no provider process:
		// the burn below has not happened, so prove nonlaunch and free capacity.
		return errors.Join(err, r.abandon(ctx, local))
	}
	if permit.BusinessToken == claim.TerminationToken {
		return errors.Join(ErrProtocol, r.abandon(ctx, local))
	}
	if err := local.Authorize(); err != nil {
		return err
	}
	input := ProviderInput{Identity: claim.Identity, Inputs: claim.Inputs, BusinessToken: permit.BusinessToken, ServerURL: r.Client.baseURL, Preparation: claim.Preparation}
	var prepared *PreparedRun
	burned := false
	defer func() {
		if !burned {
			_ = prepared.discard()
		}
	}()
	if claim.Preparation != nil {
		prepare := func() error {
			controller, ok := r.Controller.(PreparationController)
			if !ok || r.RuntimeConfigEpoch != "" && r.RuntimeConfigEpoch != claim.Preparation.Resource.RuntimeConfigEpoch {
				return preparationFailure("bundle", "configuration_changed", ErrUnsupported)
			}
			prepareCtx, cancel := context.WithTimeout(ctx, time.Duration(claim.Preparation.Limits.TimeoutSeconds)*time.Second)
			defer cancel()
			var err error
			prepared, err = controller.Prepare(prepareCtx, input, r.Client)
			if err != nil {
				return err
			}
			if err := r.Client.CheckPreparation(prepareCtx, *claim.Preparation, claim.Inputs, permit.BusinessToken); err != nil {
				return preparationFailure("workspace", "configuration_changed", err)
			}
			return nil
		}
		if err := prepare(); err != nil {
			fact := preparationFact{Stage: "workspace", State: "failed", Code: "preparation_unavailable"}
			var failed *PreparationFailure
			if errors.As(err, &failed) {
				fact = failed.fact
			}
			return errors.Join(err, r.recordPreparationFact(ctx, claim, permit, fact), r.abandon(ctx, local))
		}
		input.prepared = prepared
	}
	stop, stopErr := r.Client.StopIntent(ctx, claim.Identity, claim.TerminationToken)
	if stopErr != nil || stop.StopRequested || ctx.Err() != nil {
		return errors.Join(stopErr, r.abandon(ctx, local))
	}
	// This is the last durable transition before any fork. No code path returns
	// from a burned/unknown record to authorization, including after restart.
	if err := local.Burn(); err != nil {
		return err
	}
	burned = true
	process, err := r.Controller.Start(ctx, input)
	if err != nil {
		return errors.Join(ErrUnconfirmed, err)
	}
	if err := local.RecordProcess(process.NamespaceID()); err != nil {
		_ = process.Stop()
		_, _ = process.Wait()
		return errors.Join(ErrUnconfirmed, err)
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
	if claim.Preparation != nil {
		for _, fact := range result.PreparationFacts {
			outcomeErr = errors.Join(outcomeErr, r.recordPreparationFact(ctx, claim, permit, fact))
		}
	}
	if !result.OutputTruncated && decodeStrict(result.Output, &outcome) == nil && protocol.ValidateCollaborationProviderResult(outcome) == nil {
		outcomeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		outcomeErr = errors.Join(outcomeErr, r.Client.Outcome(outcomeCtx, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, Outcome: outcome.Outcome, Body: outcome.Body, Record: outcome.Record}, permit.BusinessToken))
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

// abandon is reachable only before Burn. It never returns to authorization; the
// proof uses the narrow termination capability, so shutdown cannot cancel it.
func (r *Runner) abandon(ctx context.Context, local *RunLedger) error {
	if err := local.NeverLaunched(); err != nil {
		return err
	}
	return r.finish(context.WithoutCancel(ctx), local)
}

// permit replays the identical attempt seq/nonce, which the server treats as
// idempotent. Only transport loss and transient statuses are retried.
func (r *Runner) permit(ctx context.Context, in protocol.CollaborationRunPermitRequest) (protocol.CollaborationRunPermitResponse, error) {
	attempts, backoff := r.PermitAttempts, r.PermitBackoff
	if attempts <= 0 {
		attempts = defaultPermitAttempts
	}
	if backoff <= 0 {
		backoff = defaultPermitBackoff
	}
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(backoff << (attempt - 1))
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return protocol.CollaborationRunPermitResponse{}, errors.Join(err, ctx.Err())
			}
		}
		var out protocol.CollaborationRunPermitResponse
		out, err = r.Client.Permit(ctx, in)
		if err == nil || !retryablePermitError(err) {
			return out, err
		}
	}
	return protocol.CollaborationRunPermitResponse{}, err
}

func retryablePermitError(err error) bool {
	var status *HTTPError
	if errors.As(err, &status) {
		return status.Status == http.StatusRequestTimeout || status.Status == http.StatusTooManyRequests || status.Status >= 500
	}
	return errors.Is(err, ErrTransport) || errors.Is(err, ErrProtocol)
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
