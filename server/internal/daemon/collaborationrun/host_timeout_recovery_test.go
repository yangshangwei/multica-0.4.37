package collaborationrun

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHostTimeoutRetainsOriginalReceiptsAcrossNewRunner(t *testing.T) {
	for _, acknowledgements := range []struct {
		name    string
		outcome bool
		proof   bool
	}{{"outcome", true, false}, {"proof", false, true}, {"both", true, true}} {
		t.Run(acknowledgements.name, func(t *testing.T) {
			f := newHostRunFixture(t, "wait")
			f.runner.Host.Options.Timeout = 1500 * time.Millisecond
			f.loseOutcomeReply, f.loseProofReply = acknowledgements.outcome, acknowledgements.proof
			done := make(chan error, 1)
			go func() { done <- f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request) }()
			f.waitForStart(t)
			if _, err := os.Stat(f.release); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("timeout fixture was released instead of reaching its execution deadline")
			}
			err := waitHostTimeoutResult(t, done)
			if err == nil || acknowledgements.proof && !errors.Is(err, ErrProofPending) {
				t.Fatalf("lost original acknowledgement was not retained: %v", err)
			}
			before := readHostTimeoutLedger(t, f.runner.Ledger, f)
			if before.Host == nil || before.Host.Outcome == nil || before.Host.Outcome.Outcome != "failure" ||
				!strings.Contains(before.Host.Outcome.Body, "claude timed out after 1.5s") {
				t.Fatal("the real execution deadline did not retain its original timeout failure")
			}
			assertHostTimeoutTermination(t, before)
			if hostOutcomePending(before.Host) != acknowledgements.outcome ||
				(before.Phase == phaseProof) != acknowledgements.proof {
				t.Fatal("the on-disk ledger did not retain the specifically lost outcome/proof acknowledgement")
			}
			f.mu.Lock()
			claimsBefore, permitsBefore := f.claimRequests, f.permitRequests
			f.mu.Unlock()

			// Open a new ledger object and a Runner without a Host executor. A
			// successful recovery can only use the retained receipt path.
			ledger, err := OpenLedger(f.runner.Ledger.dir)
			if err != nil {
				t.Fatal(err)
			}
			client, err := NewClient(f.server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			restarted := &Runner{Ledger: ledger, Client: client}
			for range 2 {
				if err := restarted.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
					t.Fatalf("the new Runner failed to recover the original receipts: %v", err)
				}
			}
			after := readHostTimeoutLedger(t, ledger, f)
			if after.Phase != phaseAcknowledged || !after.Host.OutcomeRecorded || after.Host.OutcomeRejected ||
				after.Identity != before.Identity || after.AttemptNonce != before.AttemptNonce ||
				!reflect.DeepEqual(after.Host.Outcome, before.Host.Outcome) || !reflect.DeepEqual(after.Proof, before.Proof) ||
				!reflect.DeepEqual(after.Host.Launches, before.Host.Launches) {
				t.Fatal("receipt recovery changed the original failure, run, attempt, launch or proof")
			}
			retained, err := ledger.Retained()
			if err != nil || len(retained) != 0 {
				t.Fatalf("acknowledged original receipts stayed pending: %v", err)
			}
			if len(f.starts(t)) != 1 {
				t.Fatal("restart or repeated recovery launched the timed-out instruction again")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if claimsBefore != 1 || permitsBefore < 1 || f.claimRequests != claimsBefore || f.permitRequests != permitsBefore ||
				len(f.permits) != 1 || len(f.terminations) != 1 || len(f.outcomes) != 1 {
				t.Fatal("recovery acquired new execution authority or duplicated a retained business fact")
			}
			wantOutcomes, wantProofs := 1, 1
			if acknowledgements.outcome {
				wantOutcomes++
			}
			if acknowledgements.proof {
				wantProofs++
			}
			if len(f.outcomeRequests) != wantOutcomes || len(f.proofs) != wantProofs {
				t.Fatalf("recovery replayed the wrong receipts: outcomes=%d proofs=%d", len(f.outcomeRequests), len(f.proofs))
			}
			for _, outcome := range f.outcomeRequests {
				if !reflect.DeepEqual(outcome, *before.Host.Outcome) {
					t.Fatal("a recovered failure changed its event, input or payload identity")
				}
			}
			for _, proof := range f.proofs {
				if !reflect.DeepEqual(proof, *before.Proof) {
					t.Fatal("a recovered proof changed its original identity")
				}
			}
		})
	}
}

func TestHostTimeoutAndStopKeepTheirActualFirstOutcome(t *testing.T) {
	for _, stopFirst := range []bool{false, true} {
		name := "timeout_before_stop"
		if stopFirst {
			name = "stop_before_timeout"
		}
		t.Run(name, func(t *testing.T) {
			f := newHostRunFixture(t, "wait")
			f.runner.Host.Options.Timeout = 1500 * time.Millisecond
			f.stopAfterOutcome = !stopFirst
			done := make(chan error, 1)
			go func() { done <- f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request) }()
			f.waitForStart(t)
			if stopFirst {
				f.mu.Lock()
				f.stop = true
				f.mu.Unlock()
			}
			err := waitHostTimeoutResult(t, done)
			if err != nil {
				t.Fatalf("known termination failed to converge: %v", err)
			}
			before := readHostTimeoutLedger(t, f.runner.Ledger, f)
			assertHostTimeoutTermination(t, before)
			if before.Phase != phaseAcknowledged {
				t.Fatal("the original proof was not acknowledged after Stop/timeout")
			}
			f.mu.Lock()
			claimsBefore, permitsBefore := f.claimRequests, f.permitRequests
			f.mu.Unlock()
			if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
				t.Fatal(err)
			}
			after := readHostTimeoutLedger(t, f.runner.Ledger, f)
			if !reflect.DeepEqual(before, after) || len(f.starts(t)) != 1 {
				t.Fatal("a later original request replaced the first terminal result or relaunched")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if !f.stop || claimsBefore != 1 || permitsBefore < 1 || f.claimRequests != claimsBefore || f.permitRequests != permitsBefore ||
				len(f.proofs) != 1 || len(f.terminations) != 1 {
				t.Fatal("Stop/timeout ordering duplicated launch or terminal authority")
			}
			if stopFirst {
				if len(f.outcomes) != 0 {
					t.Fatal("Stop-first termination invented an accepted business outcome")
				}
			} else if len(f.outcomes) != 1 || f.outcomes[0].Outcome != "failure" ||
				!strings.Contains(f.outcomes[0].Body, "claude timed out after 1.5s") {
				t.Fatal("later Stop replaced the already accepted timeout failure")
			}
		})
	}
}

func waitHostTimeoutResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the real controlled process did not finish its bounded execution")
		return nil
	}
}

func readHostTimeoutLedger(t *testing.T, ledger *Ledger, f *hostRunFixture) ledgerRecord {
	t.Helper()
	local, err := ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if local.record == nil {
		t.Fatal("the original run has no durable ledger")
	}
	return *local.record
}

func assertHostTimeoutTermination(t *testing.T, record ledgerRecord) {
	t.Helper()
	if record.Proof == nil || record.Proof.State != "host_exit_observed" || record.Host == nil ||
		!record.Host.Stopped || len(record.Host.Launches) != 1 {
		t.Fatal("timeout/Stop lacks a sealed original host termination proof")
	}
	launch := record.Host.Launches[0]
	if launch.PID == 0 || launch.ProcessInstance == "" || launch.Termination == nil ||
		launch.Termination.Kind != "managed_launch_ended" || !launch.TerminalAccepted ||
		launch.Authorized == nil || !*launch.Authorized {
		t.Fatal("elapsed time was substituted for an actual observed start and wait")
	}
}
