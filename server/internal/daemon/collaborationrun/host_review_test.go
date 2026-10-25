package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestHostRunnerOpenCodeEmptyExitDoesNotInventBusinessOutcome(t *testing.T) {
	for _, mode := range []string{"empty_result", "opencode_error"} {
		t.Run(mode, func(t *testing.T) {
			f := newHostRunFixture(t, mode)
			f.runner.Host.Config.Provider = "opencode"
			capabilities, err := f.runner.Host.Config.Capabilities()
			if err != nil {
				t.Fatal(err)
			}
			f.request, f.claim = hostTestClaim(t, capabilities)
			if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.proofs) != 1 || f.proofs[0].State != "host_exit_observed" || len(f.starts(t)) != 1 {
				t.Fatal("provider output lost the independent native termination observation")
			}
			if mode == "opencode_error" {
				if len(f.outcomes) != 1 || f.outcomes[0].Outcome != "failure" || !strings.Contains(f.outcomes[0].Body, "fixture provider refusal") {
					t.Fatal("explicit provider failure became a missing result")
				}
			} else if len(f.outcomes) != 0 {
				t.Fatalf("empty provider output invented a business outcome: %+v", f.outcomes)
			}
		})
	}
}

func TestHostRunnerUsesSelectedCLIUnderCanonicalCommandName(t *testing.T) {
	f := newHostRunFixture(t, "selected_cli")
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	selected := filepath.Join(t.TempDir(), "selected-collaboration-tool"+suffix)
	contents, err := os.ReadFile(f.runner.Host.CLIExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selected, contents, 0700); err != nil {
		t.Fatal(err)
	}
	f.runner.Host.CLIExecutable = selected
	ambient := t.TempDir()
	if err := os.WriteFile(filepath.Join(ambient, "multica"+suffix), []byte("ambient CLI must not execute"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", ambient+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.outcomes) != 1 || f.outcomes[0].Outcome != "success" || f.outcomes[0].Body != "Shared result" {
		t.Fatal("the admitted provider did not execute the explicitly selected collaboration CLI")
	}
}

func TestHostRecordingRedactsCollaborationCapabilities(t *testing.T) {
	business, termination := "mcb_"+strings.Repeat("a", 64), "mct_"+strings.Repeat("b", 64)
	record := newHostRecording(&hostPreparedRun{Root: t.TempDir()}, "")
	activity := &HostActivity{}
	record.message(agent.Message{Type: agent.MessageText, Content: "copied value: " + business}, activity)
	record.message(agent.Message{Type: agent.MessageToolUse, Tool: "read", CallID: "call-1",
		Input: map[string]any{"values": []any{business, termination}}}, activity)
	record.message(agent.Message{Type: agent.MessageToolResult, CallID: "call-1", Output: termination}, activity)
	outcome, ok := record.outcome(testIdentity(), "1", uuid.NewString(), agent.Result{Status: "completed", Output: business})
	if !ok {
		t.Fatal("credential redaction discarded the independent business result")
	}
	raw, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), business) || strings.Contains(string(raw), termination) {
		t.Fatal("a plain collaboration capability reached the shared recording")
	}
}

func TestHostLedgerRejectsCorruptRetainedClaim(t *testing.T) {
	for name, corrupt := range map[string]func(*ledgerRecord){
		"missing preparation": func(r *ledgerRecord) { r.Host.Claim.Preparation = nil },
		"missing context":     func(r *ledgerRecord) { r.Host.Claim.HostContext = nil },
		"different session":   func(r *ledgerRecord) { r.Host.SessionKey = strings.Repeat("a", 64) },
		"different capability": func(r *ledgerRecord) {
			r.Host.CapabilityDigest = strings.Repeat("b", 64)
		},
		"different configuration": func(r *ledgerRecord) {
			r.Host.ConfigurationDigest = strings.Repeat("c", 64)
		},
		"missing author epoch": func(r *ledgerRecord) { r.Host.Claim.Inputs[1].AccountEpoch = "0" },
		"different agent owner": func(r *ledgerRecord) {
			r.Host.Claim.HostContext.AgentOwnerUserID = uuid.NewString()
			key, _ := hostSessionKey(*r.Host.Claim)
			r.Host.SessionKey = key
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newHostRunFixture(t, "result")
			local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			key, err := hostSessionKey(f.claim)
			if err != nil {
				t.Fatal(err)
			}
			if err := local.registerHost(f.claim, key, f.request.CapabilityDigest, f.claim.Preparation.Host.ConfigurationDigest); err != nil {
				t.Fatal(err)
			}
			if retained, err := f.runner.Ledger.Retained(); err != nil || len(retained) != 1 || !retained[0].Waiting {
				t.Fatalf("valid retained wait rejected: %v", err)
			}
			corrupt(local.record)
			raw, err := json.Marshal(local.record)
			if err != nil {
				t.Fatal(err)
			}
			if err := atomicPrivateWrite(local.path, raw); err != nil {
				t.Fatal(err)
			}
			if err := local.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := f.runner.Ledger.Retained(); !errors.Is(err, ErrUnconfirmed) {
				t.Fatalf("corrupt retained claim remained eligible: %v", err)
			}
			if reopened, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID); !errors.Is(err, ErrUnconfirmed) {
				if reopened != nil {
					_ = reopened.Close()
				}
				t.Fatalf("corrupt retained claim reopened for execution: %v", err)
			}
		})
	}
}

func TestHostRunnerPreparationFactsDoNotGateTermination(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "prepared"
		if failed {
			name = "failed"
		}
		t.Run(name, func(t *testing.T) {
			f := newHostRunFixture(t, "result")
			f.failPreparationFact = true
			if failed {
				f.claim.Preparation.Attachments[0].SizeBytes++
				if err := protocol.SealCollaborationPreparation(f.claim.Preparation); err != nil {
					t.Fatal(err)
				}
			}
			err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
			if err == nil || errors.Is(err, ErrUnconfirmed) || errors.Is(err, ErrProofPending) {
				t.Fatalf("observation failure must remain separate from accepted termination: %v", err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.proofs) != 1 {
				t.Fatalf("preparation reporting blocked terminal proof: %d proofs", len(f.proofs))
			}
			if failed {
				if len(f.preparations) != 1 || f.preparations[0].Stage != "attachments" || f.preparations[0].State != "failed" ||
					f.proofs[0].State != "never_launched_proven" || len(f.outcomes) != 0 || len(f.starts(t)) != 0 {
					t.Fatal("failed preparation lost its fact or became a provider result")
				}
			} else if len(f.preparations) != 2 || f.preparations[0].Stage != "attachments" || f.preparations[1].Stage != "workspace" ||
				f.preparations[0].State != "prepared" || f.preparations[1].State != "prepared" || f.proofs[0].State != "host_exit_observed" {
				t.Fatal("successful preparation facts or actual provider termination were lost")
			}
		})
	}
}

func TestClientInterruptedStopResponseIsTransportUncertainty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte(`{"protocol_version":3,`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.HostStopIntent(t.Context(), testIdentity(), "termination-only")
	if !errors.Is(err, ErrTransport) || definitiveHostRejection(err) {
		t.Fatalf("interrupted response became definitive revocation: %v", err)
	}
}

func TestHostRunnerInterruptedStopResponseKeepsExistingExecution(t *testing.T) {
	f := newHostRunFixture(t, "wait")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.runner.Run(ctx, f.claim.Identity.RunID, f.request) }()
	f.waitForStart(t)
	f.mu.Lock()
	f.partialStopReply = true
	polls := f.stopPolls
	f.mu.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for {
		f.mu.Lock()
		observed := f.stopPolls - polls
		f.mu.Unlock()
		if observed >= 3 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("partial stop reply cancelled active execution: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("stop poll did not keep retrying the interrupted response")
		}
	}
	if err := os.WriteFile(f.release, []byte("finish"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not finish after release")
	}
	if len(f.starts(t)) != 1 {
		t.Fatal("transport recovery restarted the existing instruction")
	}
}

func TestHostRunnerPendingOutcomeSurvivesProofAcknowledgement(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		name := "missing outcome remains incomplete"
		if recorded {
			name = "recorded outcome recovers acknowledgement"
		}
		t.Run(name, func(t *testing.T) {
			f := newHostRunFixture(t, "result")
			f.failOutcome, f.loseOutcomeReply = !recorded, recorded
			if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err == nil || errors.Is(err, ErrProofPending) || errors.Is(err, ErrUnconfirmed) {
				t.Fatalf("business uncertainty must not become pending termination: %v", err)
			}
			retained, err := f.runner.Ledger.Retained()
			if err != nil || len(retained) != 1 || retained[0].Unconfirmed || !retained[0].CanProve {
				t.Fatalf("pending outcome did not retain capacity-free recovery: %+v, %v", retained, err)
			}
			local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if local.record.Phase != phaseAcknowledged || local.record.Host.Outcome == nil || local.record.Host.OutcomeRecorded || local.record.Host.OutcomeRejected {
				t.Fatal("outcome uncertainty prevented proof ACK or was falsely marked recorded")
			}
			original := *local.record.Host.Outcome
			if err := local.Close(); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			proofCount := len(f.proofs)
			f.failOutcome, f.stop = false, true
			f.mu.Unlock()
			if proofCount != 1 {
				t.Fatalf("outcome outage blocked final proof: %d", proofCount)
			}
			// Recovery has no executor or current managed authority. It may
			// recover only the original result's receipt with its original mcb.
			recoveryClient, err := NewClient(f.server.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			recoveryRunner := &Runner{Ledger: f.runner.Ledger, Client: recoveryClient}
			request := protocol.CollaborationRunClaimRequest{DeploymentID: f.request.DeploymentID, InstallationID: f.request.InstallationID}
			if err := recoveryRunner.Run(t.Context(), f.claim.Identity.RunID, request); err != nil {
				t.Fatal(err)
			}
			local, err = f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			defer local.Close()
			if local.record.Phase != phaseAcknowledged || !reflect.DeepEqual(*local.record.Host.Outcome, original) ||
				local.record.Host.OutcomeRecorded != recorded || local.record.Host.OutcomeRejected == recorded {
				t.Fatal("recovery changed outcome identity or invented recording success")
			}
			if retained, err := f.runner.Ledger.Retained(); err != nil || len(retained) != 0 {
				t.Fatalf("settled outcome receipt remained pending: %v", err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.proofs) != 1 || len(f.outcomeRequests) != 2 || !reflect.DeepEqual(f.outcomeRequests[0], f.outcomeRequests[1]) || len(f.starts(t)) != 1 {
				t.Fatal("receipt recovery changed payload, repeated proof or relaunched execution")
			}
		})
	}
}

type hostReviewTransport func(*http.Request) (*http.Response, error)

func (f hostReviewTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHostRunnerPendingProofHasPriorityOverBlockedOutcome(t *testing.T) {
	f := newHostRunFixture(t, "result")
	f.failOutcome, f.loseProofReply = true, true
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); !errors.Is(err, ErrProofPending) {
		t.Fatalf("fixture did not retain the lost final proof: %v", err)
	}
	transport := hostReviewTransport(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/outcome") {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}
		return http.DefaultTransport.RoundTrip(request)
	})
	client, err := NewClient(f.server.URL, "", transport)
	if err != nil {
		t.Fatal(err)
	}
	recovery := &Runner{Ledger: f.runner.Ledger, Client: client}
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	err = recovery.Run(ctx, f.claim.Identity.RunID, protocol.CollaborationRunClaimRequest{
		DeploymentID: f.request.DeploymentID, InstallationID: f.request.InstallationID})
	if !errors.Is(err, ErrTransport) || errors.Is(err, ErrProofPending) {
		t.Fatalf("blocked outcome consumed the final-proof recovery budget: %v", err)
	}
	local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	if local.record.Phase != phaseAcknowledged || !hostOutcomePending(local.record.Host) {
		t.Fatal("final proof did not advance independently of the pending outcome")
	}
}

func TestHostSupervisorLateCallbackCannotChangeClosedUnknownRun(t *testing.T) {
	for _, ended := range []bool{false, true} {
		name := "late start observation"
		if ended {
			name = "late terminal observation"
		}
		t.Run(name, func(t *testing.T) {
			f := newHostRunFixture(t, "result")
			local, err := f.runner.Ledger.Lock(f.request.DeploymentID, f.request.InstallationID, f.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			defer local.Close()
			key, err := hostSessionKey(f.claim)
			if err != nil {
				t.Fatal(err)
			}
			if err := local.registerHost(f.claim, key, f.request.CapabilityDigest, f.claim.Preparation.Host.ConfigurationDigest); err != nil {
				t.Fatal(err)
			}
			if err := local.hostSessionAcquired(); err != nil {
				t.Fatal(err)
			}
			supervisor := &hostSupervisor{runner: f.runner, local: local, claim: f.claim}
			supervisor.lifecycle = agent.NewLaunchLifecycle(supervisor)
			if err := f.runner.Host.Probe(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := supervisor.authorize(t.Context(), agent.LaunchKindProvider); err != nil {
				t.Fatal(err)
			}
			supervisor.prepared, err = f.runner.Host.prepare(t.Context(), f.claim, key, supervisor.businessToken, f.runner.Client, supervisor.lifecycle)
			if err != nil {
				t.Fatal(err)
			}
			callbacks, err := supervisor.BeforeLaunch(t.Context(), agent.LaunchInfo{Provider: "claude", Kind: agent.LaunchKindProvider})
			if err != nil {
				t.Fatal(err)
			}
			process := agent.ProcessObservation{PID: 1234, InstanceID: "handle:" + uuid.NewString(), SupervisionScope: f.claim.Preparation.Host.SupervisionScopes[0]}
			if ended {
				if err := callbacks.Started(process); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(local.path)
			if err != nil {
				t.Fatal(err)
			}
			// runHost fences an unconfirmed return before Runner releases the
			// OS lock. A provider callback may still finish after that timeout.
			supervisor.mu.Lock()
			local.failed = true
			supervisor.mu.Unlock()
			start := make(chan struct{})
			callbackErr := make(chan error, 1)
			var closing sync.WaitGroup
			closing.Go(func() {
				<-start
				if err := local.Close(); err != nil {
					t.Error(err)
				}
			})
			go func() {
				<-start
				if ended {
					callbackErr <- callbacks.Terminated(agent.LaunchTermination{Kind: agent.LaunchManagedEnded, Process: process,
						WaitObserved: true, Cleanup: agent.CleanupUnverified, CleanupScope: process.SupervisionScope})
				} else {
					callbackErr <- callbacks.Started(process)
				}
			}()
			close(start)
			if err := <-callbackErr; !errors.Is(err, ErrUnconfirmed) {
				t.Fatalf("late callback changed a fenced run: %v", err)
			}
			closing.Wait()
			after, err := os.ReadFile(local.path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("late callback rewrote retained evidence: %v", err)
			}
			if retained, err := f.runner.Ledger.Retained(); err != nil || len(retained) != 1 || !retained[0].Unconfirmed {
				t.Fatalf("closed unknown execution lost its capacity barrier: %+v, %v", retained, err)
			}
			if session, err := f.runner.Ledger.lockHostSession(key, uuid.NewString()); !errors.Is(err, ErrSessionBusy) {
				if session != nil {
					_ = session.Close()
				}
				t.Fatalf("late callback released unknown session ownership: %v", err)
			}
		})
	}
}
