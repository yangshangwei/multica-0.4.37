package collaborationrun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const hostFixtureSession = "b9f0480a-016a-4b46-8ef7-b90cc0968ab7"

func runHostProviderFixture() (int, bool) {
	mode := os.Getenv("COLLAB_HOST_TEST_MODE")
	if mode == "" {
		return 0, false
	}
	if len(os.Args) == 2 && os.Args[1] == "--selected-cli-fixture" {
		_, _ = fmt.Fprintln(os.Stdout, "selected collaboration CLI")
		return 0, true
	}
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return 21, true
	}
	run, err := cli.ReadCollaborationContext()
	if err != nil || run == nil || run.ProtocolVersion != 3 || os.Getenv("MULTICA_TOKEN") != "" || os.Getenv("MULTICA_DAEMON_TOKEN") != "" {
		return 22, true
	}
	f, err := os.OpenFile(os.Getenv("COLLAB_HOST_TEST_CAPTURE"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 23, true
	}
	cwd, _ := os.Getwd()
	_ = json.NewEncoder(f).Encode(map[string]any{"run_id": run.Identity.RunID, "token": run.Token, "cwd": cwd, "temp": os.Getenv("TMPDIR"), "args": os.Args[1:], "input": input})
	_ = f.Close()
	resume := false
	for _, arg := range os.Args[1:] {
		resume = resume || arg == "--resume"
	}
	if mode == "reject_resume" && resume {
		_, _ = fmt.Fprintln(os.Stderr, "No conversation found with session ID "+hostFixtureSession)
		_, _ = fmt.Fprintln(os.Stdout, `{"type":"result","is_error":true,"result":"No conversation found with session ID"}`)
		return 1, true
	}
	if mode == "closed_stdio" {
		_ = os.Stdout.Close()
		_ = os.Stderr.Close()
		time.Sleep(220 * time.Millisecond)
		_ = os.WriteFile(os.Getenv("COLLAB_HOST_TEST_RELEASE"), []byte("waited"), 0600)
		return 0, true
	}
	if mode == "empty_result" {
		return 0, true
	}
	if mode == "opencode_error" {
		_, _ = fmt.Fprintln(os.Stdout, `{"type":"error","error":{"name":"APIError","data":{"message":"fixture provider refusal"}}}`)
		return 0, true
	}
	if mode == "selected_cli" {
		output, err := exec.Command("multica", "--selected-cli-fixture").CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) != "selected collaboration CLI" {
			return 24, true
		}
	}
	if mode == "wait" {
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(os.Getenv("COLLAB_HOST_TEST_RELEASE")); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"system","subtype":"init","session_id":"`+hostFixtureSession+`"}`)
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"assistant","message":{"model":"fixture","content":[{"type":"text","text":"Shared result"}],"usage":{"input_tokens":17,"output_tokens":3}}}`)
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"result","is_error":false,"session_id":"`+hostFixtureSession+`","result":"Shared result"}`)
	return 0, true
}

type hostRunFixture struct {
	t                    *testing.T
	mu                   sync.Mutex
	runner               *Runner
	request              protocol.CollaborationRunClaimRequest
	claim                protocol.CollaborationRunClaimResponse
	server               *httptest.Server
	capture              string
	release              string
	permits              map[string]protocol.CollaborationRunPermitRequest
	terminations         map[string]protocol.CollaborationLaunchTermination
	outcomes             []protocol.CollaborationRunOutcome
	outcomeRequests      []protocol.CollaborationRunOutcome
	proofs               []protocol.CollaborationRunProof
	preparations         []protocol.CollaborationPreparationObservation
	failPreparationFact  bool
	partialStopReply     bool
	stop                 bool
	offline              bool
	stopPolls            int
	stopAfterTermination bool
	losePermitReply      bool
	loseProofReply       bool
	loseOutcomeReply     bool
	failOutcome          bool
	// Shared context served by memory/base; the zero value is the empty
	// document. memoryFailures answer reads in order; a zero Status entry
	// serves the document for that read.
	claimMemoryVersion int
	memoryVersion      string
	memoryContent      string
	memoryFailures     []HTTPError
	memoryReads        int
}

func newHostRunFixture(t *testing.T, mode string) *hostRunFixture {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &HostConfig{Provider: "claude", ProviderVersion: "fixture-1", ProviderMode: "local", AccountSource: "fixture-account", Executable: executable,
		Limits: preparationTestCapabilities().Limits}
	caps, err := cfg.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	request, claim := hostTestClaim(t, caps)
	f := &hostRunFixture{t: t, request: request, claim: claim, permits: map[string]protocol.CollaborationRunPermitRequest{}, terminations: map[string]protocol.CollaborationLaunchTermination{},
		capture: filepath.Join(t.TempDir(), "starts.jsonl"), release: filepath.Join(t.TempDir(), "released")}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	client, err := NewClient(f.server.URL, "mdt_fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	f.runner = &Runner{Ledger: newHostTestLedger(t), Client: client, Host: &HostExecutor{Config: cfg, Directory: filepath.Join(t.TempDir(), "workspaces"), CLIExecutable: executable,
		Options: agent.ExecOptions{Timeout: 10 * time.Second}}, StopPollInterval: 10 * time.Millisecond, PermitAttempts: 2, PermitBackoff: time.Millisecond}
	t.Setenv("COLLAB_HOST_TEST_MODE", mode)
	t.Setenv("COLLAB_HOST_TEST_CAPTURE", f.capture)
	t.Setenv("COLLAB_HOST_TEST_RELEASE", f.release)
	t.Setenv("MULTICA_TOKEN", "ordinary-token-must-not-leak")
	t.Setenv("MULTICA_DAEMON_TOKEN", "managed-token-must-not-leak")
	return f
}

func hostTestClaim(t *testing.T, caps protocol.CollaborationRuntimeCapabilities) (protocol.CollaborationRunClaimRequest, protocol.CollaborationRunClaimResponse) {
	t.Helper()
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	request, claim := preparationTestClaim(t)
	request.ProtocolVersion, request.Containment, request.PreparationVersion, request.BundleDigest, request.CapabilityDigest = 3, protocol.CollaborationContainmentHostProcessSupervision, 2, "", digest
	claim.ProtocolVersion, claim.Containment = 3, request.Containment
	claim.HostContext = &protocol.CollaborationHostContext{AgentOwnerUserID: uuid.NewString(), RuntimeID: request.RuntimeID, Role: "agent", StepKey: "agent", SessionGeneration: "2:5"}
	claim.Preparation.Version, claim.Preparation.Provider, claim.Preparation.Host, claim.Preparation.CapabilityDigest = 2, caps.Provider, caps.Host, digest
	claim.Preparation.Resource.AgentOwnerUserID = claim.HostContext.AgentOwnerUserID
	if err := protocol.SealCollaborationPreparation(claim.Preparation); err != nil {
		t.Fatal(err)
	}
	return request, claim
}

func (f *hostRunFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("Cookie") != "" {
		f.t.Error("ambient auth reached collaboration endpoint")
	}
	if strings.HasSuffix(r.URL.Path, "/stop-intent") {
		f.stopPolls++
	}
	if f.offline {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/claim"):
		var claimed protocol.CollaborationRunClaimRequest
		if err := json.NewDecoder(r.Body).Decode(&claimed); err != nil {
			f.t.Error(err)
		}
		f.claimMemoryVersion = claimed.MemoryVersion
		_ = json.NewEncoder(w).Encode(f.claim)
	case strings.HasSuffix(r.URL.Path, "/memory/base"):
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer mcb_") {
			f.t.Error("shared context was not read with the business capability")
		}
		f.memoryReads++
		if len(f.memoryFailures) > 0 {
			failure := f.memoryFailures[0]
			f.memoryFailures = f.memoryFailures[1:]
			if failure.Status != 0 {
				w.WriteHeader(failure.Status)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": failure.Code})
				return
			}
		}
		version := f.memoryVersion
		if version == "" {
			version = "0"
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunMemory{IssueID: f.claim.Identity.IssueID, RunID: f.claim.Identity.RunID,
			Version: version, Content: f.memoryContent, Bytes: len(f.memoryContent), MaxBytes: 64 << 10})
	case strings.HasSuffix(r.URL.Path, "/permit"):
		var p protocol.CollaborationRunPermitRequest
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			f.t.Error(err)
			w.WriteHeader(400)
			return
		}
		if f.stop {
			w.WriteHeader(http.StatusConflict)
			return
		}
		if p.ProtocolVersion != 3 || p.Containment != f.request.Containment || p.LaunchDescriptor == nil {
			f.t.Error("wrong host permit mode")
		}
		seq, _ := strconv.Atoi(p.AttemptSeq)
		if seq > 1 {
			if _, ok := f.terminations[strconv.Itoa(seq-1)]; !ok {
				f.t.Error("retry requested before prior termination receipt")
				w.WriteHeader(409)
				return
			}
		}
		if old, ok := f.permits[p.AttemptSeq]; ok && !reflect.DeepEqual(old, p) {
			f.t.Error("permit retry changed nonce")
			w.WriteHeader(409)
			return
		}
		f.permits[p.AttemptSeq] = p
		if f.losePermitReply {
			f.losePermitReply = false
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunPermitResponse{ProtocolVersion: 3, Identity: p.Identity, AttemptSeq: p.AttemptSeq, AttemptNonce: p.AttemptNonce,
			BusinessToken: "mcb_" + strings.Repeat(p.AttemptSeq, 64)})
	case strings.HasSuffix(r.URL.Path, "/stop-intent"):
		if r.Header.Get("Authorization") != "Bearer "+f.claim.TerminationToken {
			f.t.Error("stop used business token")
		}
		if f.partialStopReply {
			w.Header().Set("Content-Length", "1024")
			_, _ = w.Write([]byte(`{"protocol_version":3,`))
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunStopIntent{ProtocolVersion: 3, Containment: f.claim.Containment, Identity: f.claim.Identity, StopRequested: f.stop})
	case strings.HasSuffix(r.URL.Path, "/preparation/observations"):
		var observation protocol.CollaborationPreparationObservation
		if err := json.NewDecoder(r.Body).Decode(&observation); err != nil {
			f.t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if protocol.ValidateCollaborationPreparationObservation(observation) != nil || observation.ProtocolVersion != 3 ||
			observation.Identity != f.claim.Identity || observation.ManifestDigest != f.claim.Preparation.ManifestDigest ||
			r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat(observation.AttemptSeq, 64) {
			f.t.Error("preparation fact lost its host identity or attempt credential")
		}
		f.preparations = append(f.preparations, observation)
		if f.failPreparationFact {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationPreparationObservationReceipt{Identity: observation.Identity,
			EventID: observation.EventID, ManifestDigest: observation.ManifestDigest, Recorded: true})
	case strings.HasSuffix(r.URL.Path, "/preparation"):
		_ = json.NewEncoder(w).Encode(f.claim.Preparation)
	case strings.Contains(r.URL.Path, "/attachments/"):
		file := f.claim.Preparation.Attachments[0]
		w.Header().Set("X-Multica-Run-ID", f.claim.Identity.RunID)
		w.Header().Set("X-Multica-Attachment-ID", file.ID)
		w.Header().Set("X-Multica-Attachment-Version", file.Version)
		_, _ = w.Write([]byte("data"))
	case strings.HasSuffix(r.URL.Path, "/termination"):
		var p protocol.CollaborationLaunchTermination
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			f.t.Error(err)
			w.WriteHeader(400)
			return
		}
		if err := protocol.ValidateCollaborationLaunchTermination(p); err != nil {
			f.t.Error(err)
		}
		if old, ok := f.terminations[p.AttemptSeq]; ok && !reflect.DeepEqual(old, p) {
			f.t.Error("termination replay changed original event")
			w.WriteHeader(409)
			return
		}
		f.terminations[p.AttemptSeq] = p
		_, authorized := f.permits[p.AttemptSeq]
		if f.stopAfterTermination {
			f.stop = true
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: 3, Identity: p.Identity, LaunchID: p.LaunchID, EventID: p.EventID, Accepted: true, Authorized: authorized})
	case strings.HasSuffix(r.URL.Path, "/outcome"):
		var p protocol.CollaborationRunOutcome
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			f.t.Error(err)
			w.WriteHeader(400)
			return
		}
		if r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat(p.AttemptSeq, 64) {
			f.t.Error("stale business token on retried outcome")
		}
		f.outcomeRequests = append(f.outcomeRequests, p)
		if f.failOutcome {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		for _, existing := range f.outcomes {
			if existing.EventID == p.EventID {
				if !reflect.DeepEqual(existing, p) {
					f.t.Error("outcome replay changed its original payload")
					w.WriteHeader(http.StatusConflict)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		if f.stop {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.outcomes = append(f.outcomes, p)
		if f.loseOutcomeReply {
			f.loseOutcomeReply = false
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case strings.HasSuffix(r.URL.Path, "/proof"):
		var p protocol.CollaborationRunProof
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			f.t.Error(err)
			w.WriteHeader(400)
			return
		}
		if err := protocol.ValidateCollaborationHostProof(p); err != nil {
			f.t.Error(err)
		}
		f.proofs = append(f.proofs, p)
		if f.loseProofReply {
			f.loseProofReply = false
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationRunProofReceipt{ProtocolVersion: 3, Identity: p.Identity, ReceiptID: p.ReceiptID, Accepted: true})
	default:
		f.t.Errorf("unexpected host endpoint %s", r.URL.Path)
		w.WriteHeader(404)
	}
}

func (f *hostRunFixture) starts(t *testing.T) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(f.capture)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatal(err)
		}
		out = append(out, value)
	}
	return out
}

func (f *hostRunFixture) waitForStart(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(f.capture); err == nil && info.Size() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("test-owned provider did not start")
}

func TestHostRunnerReusesOrdinaryProviderAndOriginalPermitAndProof(t *testing.T) {
	f := newHostRunFixture(t, "result")
	f.losePermitReply, f.loseProofReply = true, true
	err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
	if !errors.Is(err, ErrProofPending) {
		t.Fatalf("lost receipt was not retained: %v", err)
	}
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.starts(t)
	if len(starts) != 1 {
		t.Fatalf("replayed request relaunched provider %d times", len(starts))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.permits) != 1 || len(f.proofs) != 2 || f.proofs[0].ReceiptID != f.proofs[1].ReceiptID || f.proofs[0].State != "host_exit_observed" {
		t.Fatalf("wrong permit/proof replay: %d/%d", len(f.permits), len(f.proofs))
	}
	if len(f.outcomes) != 1 || f.outcomes[0].Outcome != "success" || f.outcomes[0].Body != "Shared result" || f.outcomes[0].Record.Usage == nil || *f.outcomes[0].Record.Usage.InputTokens != "17" {
		t.Fatalf("ordinary output/usage was not recorded: %+v", f.outcomes)
	}
	data, err := os.ReadFile(filepath.Join(starts[0]["cwd"].(string), f.claim.Preparation.Attachments[0].Destination))
	if err != nil || string(data) != "data" {
		t.Fatalf("frozen delivered input missing: %v", err)
	}
}

func TestHostRunnerEOFDoesNotEndExecutionBeforeRealWait(t *testing.T) {
	f := newHostRunFixture(t, "closed_stdio")
	start := time.Now()
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 200*time.Millisecond {
		t.Fatal("stdio EOF became an early terminal proof")
	}
	if _, err := os.Stat(f.release); err != nil {
		t.Fatal("provider never reached its real exit")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.proofs) != 1 || f.proofs[0].State != "host_exit_observed" {
		t.Fatal("actual wait did not settle host execution")
	}
	if len(f.outcomes) > 0 && f.outcomes[0].Outcome == "success" {
		t.Fatal("empty stdio/exit zero invented business success")
	}
}

func TestHostRunnerOfflineKeepsRunningAndOnlyReplaysReceipts(t *testing.T) {
	f := newHostRunFixture(t, "wait")
	done := make(chan error, 1)
	go func() { done <- f.runner.Run(context.Background(), f.claim.Identity.RunID, f.request) }()
	f.waitForStart(t)
	f.mu.Lock()
	f.offline = true
	f.mu.Unlock()
	time.Sleep(80 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("transient disconnect cancelled active host execution: %v", err)
	default:
	}
	if err := os.WriteFile(f.release, []byte("finish"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrProofPending) {
			t.Fatalf("offline receipt not retained: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not finish")
	}
	f.mu.Lock()
	f.offline = false
	polls := f.stopPolls
	f.mu.Unlock()
	if polls == 0 {
		t.Fatal("test did not exercise stop-intent disconnect")
	}
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	if len(f.starts(t)) != 1 {
		t.Fatal("reconnect re-executed an observed launch")
	}
}

func TestHostRunnerFreshRetryGetsNewTokenAndStopForbidsFallback(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(strconv.FormatBool(stop), func(t *testing.T) {
			f := newHostRunFixture(t, "reject_resume")
			key, err := hostSessionKey(f.claim)
			if err != nil {
				t.Fatal(err)
			}
			s, err := f.runner.Ledger.lockHostSession(key, f.claim.Identity.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.save(hostFixtureSession, false); err != nil {
				t.Fatal(err)
			}
			_ = s.Close()
			f.stopAfterTermination = stop
			err = f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
			starts := f.starts(t)
			if stop {
				if len(starts) != 1 {
					t.Fatalf("stop allowed %d starts", len(starts))
				}
				if errors.Is(err, ErrUnconfirmed) {
					t.Fatalf("confirmed stopped fallback became unknown: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if len(starts) != 2 || starts[0]["token"] == starts[1]["token"] {
					t.Fatalf("fresh retry did not receive its own attempt authority: %d", len(starts))
				}
			}
		})
	}
}
