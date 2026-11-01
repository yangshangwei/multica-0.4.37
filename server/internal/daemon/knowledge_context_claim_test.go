package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func attachKnowledgeClaimWS(t *testing.T, f *knowledgeDaemonFixture, rpc *wsRPCClient) {
	t.Helper()
	var tasks any = []Task{f.task}
	if len(f.claimTasks) > 0 {
		tasks = f.claimTasks
	}
	reply, err := json.Marshal(map[string]any{"tasks": tasks})
	if err != nil {
		t.Fatal(err)
	}
	generation := rpc.attach(func(frame []byte) (*wsOutbound, error) {
		var message protocol.Message
		if err := json.Unmarshal(frame, &message); err != nil {
			return nil, err
		}
		var request protocol.RPCRequestPayload
		if err := json.Unmarshal(message.Payload, &request); err != nil {
			return nil, err
		}
		if request.Method != "tasks.claim" {
			t.Error("unexpected RPC method")
		}
		f.mu.Lock()
		f.calls = append(f.calls, knowledgeHTTPCall{path: "ws/claim", body: append([]byte{}, request.Body...)})
		f.mu.Unlock()
		go rpc.deliver(protocol.RPCResponsePayload{RequestID: request.RequestID, Status: 200, Body: reply})
		return &wsOutbound{data: frame}, nil
	})
	rpc.markRPCV1Supported(generation)
}

func installKnowledgeManagedFixture(t *testing.T, f *knowledgeDaemonFixture) *managedWorkspaceTransport {
	t.Helper()
	c := f.d.client
	c.enableManagedTransport()
	c.SetToken("mul_discovery_not_for_execution")
	if err := c.installManagedCredential(f.task.WorkspaceID, ManagedDaemonCredential{BindingID: uuid.NewString(), BindingEpoch: "1", CapabilityVersion: "1",
		DaemonToken: "mdt_test_daemon_only", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: uuid.NewString(), AuthVersion: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.recordManagedRegistration(f.task.WorkspaceID, []Runtime{{ID: f.task.RuntimeID, Provider: "claude"}}); err != nil {
		t.Fatal(err)
	}
	return c.managed.workspaces[f.task.WorkspaceID]
}

func TestKnowledgeAllClaimTransportsNegotiateSourceFreeCapabilities(t *testing.T) {
	for _, transport := range []string{"http_single", "http_batch", "ws_batch", "http_legacy_fallback", "managed_single", "managed_http", "managed_ws"} {
		t.Run(transport, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			var scoped *managedWorkspaceTransport
			if strings.HasPrefix(transport, "managed") {
				scoped = installKnowledgeManagedFixture(t, f)
			}
			if transport == "ws_batch" {
				attachKnowledgeClaimWS(t, f, f.d.wsRPC)
			}
			if transport == "managed_ws" {
				attachKnowledgeClaimWS(t, f, scoped.rpc)
			}
			var tasks []*Task
			var err error
			switch transport {
			case "http_single", "managed_single":
				var task *Task
				task, err = f.d.client.ClaimTask(context.Background(), f.task.RuntimeID)
				tasks = []*Task{task}
			case "http_batch", "managed_http":
				tasks, err = f.d.client.ClaimTasks(context.Background(), "daemon", []string{f.task.RuntimeID}, 1)
			case "http_legacy_fallback":
				f.d.batchClaimUnsupported.Store(true)
				tasks, err = f.d.ClaimTasksWSFirst(context.Background(), "daemon", []string{f.task.RuntimeID}, 1)
			default:
				tasks, err = f.d.ClaimTasksWSFirst(context.Background(), "daemon", []string{f.task.RuntimeID}, 1)
			}
			if err != nil || len(tasks) != 1 || tasks[0] == nil || tasks[0].KnowledgeContext == nil {
				t.Fatalf("claim failed to preserve descriptor: %v", err)
			}
			if *tasks[0].KnowledgeContext != *f.task.KnowledgeContext {
				t.Fatal("claim descriptor changed across transport")
			}
			calls := f.callsFor("claim")
			if len(calls) != 1 {
				t.Fatalf("claim count %d", len(calls))
			}
			var body struct {
				Knowledge json.RawMessage `json:"knowledge_context"`
			}
			if err := json.Unmarshal(calls[0].body, &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Knowledge) == 0 {
				t.Fatal("claim omitted supported capability")
			}
			var inv protocol.TaskKnowledgeInvocation
			if strings.Contains(calls[0].path, "/runtimes/") {
				if err := json.Unmarshal(body.Knowledge, &inv); err != nil {
					t.Fatal(err)
				}
			} else {
				var byRuntime map[string]protocol.TaskKnowledgeInvocation
				if err := json.Unmarshal(body.Knowledge, &byRuntime); err != nil {
					t.Fatal(err)
				}
				inv = byRuntime[f.task.RuntimeID]
			}
			if !inv.Builtin || inv.Provider != "claude" || inv.ConfigurationID != f.task.KnowledgeContext.ConfigurationID {
				t.Fatal("capability was not derived from real runtime")
			}
			for _, forbidden := range []string{privateKnowledgeTestBody, "candidate_id", "source_issue_id", "manifest", "custom_args", "invocation_fingerprint"} {
				if bytes.Contains(calls[0].body, []byte(forbidden)) {
					t.Fatalf("claim contained premature/private field %s", forbidden)
				}
			}
			f.task = *tasks[0]
			if result := f.run(t); result.Status != "completed" {
				t.Fatalf("claimed task prepare/launch failed: %s", result.FailureReason)
			}
		})
	}
}

func TestKnowledgeCapabilityRequiresActualBuiltinAndDaemonCredential(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	if f.d.client.taskKnowledgeInvocation(f.task.RuntimeID) == nil {
		t.Fatal("test fixture did not advertise")
	}
	f.d.client.SetToken("mul_human_pat")
	if f.d.client.taskKnowledgeInvocation(f.task.RuntimeID) != nil {
		t.Fatal("PAT negotiated private execution")
	}
	f.d.client.SetToken("mdt_test_daemon_only")
	rt := f.d.runtimeIndex[f.task.RuntimeID]
	rt.ProfileID = uuid.NewString()
	f.d.runtimeIndex[rt.ID] = rt
	if f.d.client.taskKnowledgeInvocation(rt.ID) != nil {
		t.Fatal("custom Claude protocol family impersonated built-in support")
	}
	rt.ProfileID, rt.Provider = "", "codex"
	f.d.runtimeIndex[rt.ID] = rt
	if f.d.client.taskKnowledgeInvocation(rt.ID) != nil {
		t.Fatal("unsupported adapter negotiated private execution")
	}
	if f.d.client.taskKnowledgeInvocation(uuid.NewString()) != nil {
		t.Fatal("unknown runtime negotiated")
	}
}

func TestKnowledgeConfigurationNamespaceIsStableAndTaskIndependent(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	before := f.d.knowledgeInvocation(f.task.RuntimeID).ConfigurationID
	f.d.client.SetToken("mdt_refreshed")
	f.task.Agent.CustomArgs = []string{"--max-turns", "4"}
	f.task.PriorWorkDir = t.TempDir()
	if f.d.knowledgeInvocation(f.task.RuntimeID).ConfigurationID != before {
		t.Fatal("token/task options changed stable runtime namespace")
	}
	f.d.cfg.ClaudeArgs = []string{"--model", "test-model"}
	if f.d.knowledgeInvocation(f.task.RuntimeID).ConfigurationID == before {
		t.Fatal("local default change did not invalidate namespace")
	}
	f.d.cfg.ClaudeArgs = nil
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if f.d.knowledgeInvocation(f.task.RuntimeID).ConfigurationID == before {
		t.Fatal("account configuration source change did not invalidate namespace")
	}
}

func TestOrdinaryUnsupportedKnowledgeBackendRunsSafeBaseTask(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	profile := uuid.NewString()
	f.d.runtimeIndex[f.task.RuntimeID] = Runtime{ID: f.task.RuntimeID, Provider: "claude", ProfileID: profile}
	f.d.profileLaunchSpecs[profile] = profileLaunchSpec{path: f.bin}
	f.task.KnowledgeContext = nil // source-free old/unsupported claim selected a fresh base turn
	result := f.run(t)
	if result.Status != "completed" {
		t.Fatal("safe unsupported base turn failed")
	}
	if len(f.callsFor("prepare")) != 0 || len(f.callsFor("authorize")) != 0 {
		t.Fatal("unsupported backend requested private data")
	}
	input, err := os.ReadFile(f.capture)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(input, []byte(privateKnowledgeTestBody)) {
		t.Fatal("private knowledge entered unsupported stdin")
	}
}

func TestKnowledgeLostPreparationRechecksRevokedSource(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.dropAction, f.revokeAfterDrop = "prepare", true
	result := f.run(t)
	if result.FailureReason != agent.KnowledgeAuthorizationDenied {
		t.Fatal("lost response resumed stale preparation")
	}
	calls := f.callsFor("prepare")
	if len(calls) != 2 || !bytes.Equal(calls[0].body, calls[1].body) {
		t.Fatal("revocation recovery replaced original receipt")
	}
	if _, err := os.Stat(f.argv); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("revoked preparation launched")
	}
}

func TestKnowledgeMissingOriginatorCannotBeFilledFromDisplayIdentity(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.empty = true
	f.task.InitiatorID, f.task.RequestingUserName = uuid.NewString(), "Runtime installer display identity"
	if result := f.run(t); result.Status != "completed" {
		t.Fatal("authorized empty bundle could not run base task")
	}
	for _, call := range f.callsFor("prepare") {
		for _, forbidden := range []string{f.task.InitiatorID, f.task.RequestingUserName, "originator", "user_id", "audience"} {
			if bytes.Contains(call.body, []byte(forbidden)) {
				t.Fatal("daemon invented consumer authority")
			}
		}
	}
	input, _ := os.ReadFile(f.capture)
	if bytes.Contains(input, []byte(privateKnowledgeTestBody)) {
		t.Fatal("empty server resolution was expanded by local identity")
	}
}

func TestKnowledgeUnobservedFreshSessionIsNeverPublished(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.task.Agent.CustomEnv["FAKE_NATIVE"] = ""
	result := f.run(t)
	if result.Status != "completed" || result.SessionID != "" {
		t.Fatal("unknown fresh native identity was fabricated")
	}
	if len(f.callsFor("session")) != 0 || len(f.callsFor("observations")) != 2 {
		t.Fatal("unknown fresh session became a native binding")
	}
}

func TestKnowledgeNativeBindingRefusalNeverPinsUnapprovedID(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.observationRefusal = "native_session"
	result := f.run(t)
	if result.FailureReason != agent.KnowledgeContextChanged || result.SessionID != "" {
		t.Fatal("unapproved native ID escaped to terminal pointer")
	}
	if len(f.callsFor("session")) != 0 {
		t.Fatal("status event bypassed observation refusal")
	}
	if len(f.callsFor("observations")) < 3 {
		t.Fatal("refused native identity lost actual process cleanup")
	}
}

func TestKnowledgeActualResumeKeepsExactScopeGeneration(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	firstGeneration := f.task.KnowledgeContext.Session.Generation
	first := f.run(t)
	if first.Status != "completed" {
		t.Fatal("initial input failed")
	}
	f.task.ID = uuid.NewString()
	f.task.PriorSessionID, f.task.PriorWorkDir = first.SessionID, first.WorkDir
	f.bindClaim("issue", f.task.IssueID)
	f.task.KnowledgeContext.Session.Generation = firstGeneration
	f.task.KnowledgeContext.Session.NativeSessionID = first.SessionID
	second := f.run(t)
	if second.Status != "completed" || second.SessionID != first.SessionID {
		t.Fatalf("exact resume failed: %s", second.FailureReason)
	}
	argv, _ := os.ReadFile(f.argv)
	if !strings.Contains(string(argv), "--resume\n"+first.SessionID+"\n") {
		t.Fatal("admitted native selection did not reach actual argv")
	}
	calls := f.callsFor("authorize")
	if len(calls) != 2 {
		t.Fatal("resume did not receive its own launch authorization")
	}
	var auth protocol.TaskKnowledgeAuthorizeRequest
	if err := json.Unmarshal(calls[1].body, &auth); err != nil {
		t.Fatal(err)
	}
	if auth.Session.Generation != firstGeneration || auth.Session.NativeSessionID != first.SessionID || auth.Session.ScopeID != f.task.IssueID {
		t.Fatal("resume authorization used another exact binding")
	}
}

// unreachableResumeTurn models a follow-up input whose prior workdir is gone (GC
// or a non-reusable scope) while the server still bound the claim to that
// session. The daemon's reachability gate then drops the resume.
func (f *knowledgeDaemonFixture) unreachableResumeTurn(t *testing.T) (TaskResult, string) {
	t.Helper()
	firstGeneration := f.task.KnowledgeContext.Session.Generation
	first := f.run(t)
	if first.Status != "completed" || first.SessionID != f.native {
		t.Fatalf("initial input failed: %s %s", first.Status, first.FailureReason)
	}
	f.task.ID = uuid.NewString()
	f.task.PriorSessionID, f.task.PriorWorkDir = first.SessionID, filepath.Join(t.TempDir(), "collected")
	f.bindClaim("issue", f.task.IssueID)
	f.task.KnowledgeContext.Session.Generation = firstGeneration
	f.task.KnowledgeContext.Session.NativeSessionID = first.SessionID
	f.task.Agent.CustomEnv["FAKE_NATIVE"] = "fresh-after-drop"
	return first, firstGeneration
}

func knowledgeCapturedPrompt(t *testing.T, f *knowledgeDaemonFixture) string {
	t.Helper()
	input, err := os.ReadFile(f.capture)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(input, &wire); err != nil || len(wire.Message.Content) != 1 {
		t.Fatalf("captured stdin is not one final input: %v", err)
	}
	return wire.Message.Content[0].Text
}

func TestKnowledgeUnreachablePriorWorkdirRebindsFreshGeneration(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	first, firstGeneration := f.unreachableResumeTurn(t)
	second := f.run(t)
	if second.Status != "completed" || second.FailureReason != "" || second.SessionID != "fresh-after-drop" {
		t.Fatalf("unreachable resume did not start a fresh bound session: status=%s reason=%s session=%s", second.Status, second.FailureReason, second.SessionID)
	}
	prepares := f.callsFor("prepare")
	if len(prepares) != 2 {
		t.Fatalf("prepare count = %d, want 2", len(prepares))
	}
	var unflagged map[string]json.RawMessage
	if err := json.Unmarshal(prepares[0].body, &unflagged); err != nil {
		t.Fatal(err)
	}
	if _, ok := unflagged["resume_unavailable"]; ok {
		t.Fatal("a reachable or fresh prepare carried resume_unavailable")
	}
	var flagged map[string]json.RawMessage
	if err := json.Unmarshal(prepares[1].body, &flagged); err != nil {
		t.Fatal(err)
	}
	if string(flagged["resume_unavailable"]) != "true" {
		t.Fatalf("dropped resume did not report resume_unavailable: %s", prepares[1].body)
	}
	var stale protocol.TaskKnowledgeSessionBinding
	if err := json.Unmarshal(flagged["session"], &stale); err != nil {
		t.Fatal(err)
	}
	if stale.Generation != firstGeneration || stale.NativeSessionID != first.SessionID {
		t.Fatal("flagged prepare did not carry the original claimed binding")
	}
	authorizations := f.callsFor("authorize")
	if len(authorizations) != 2 {
		t.Fatalf("authorize count = %d, want 2", len(authorizations))
	}
	var auth protocol.TaskKnowledgeAuthorizeRequest
	if err := json.Unmarshal(authorizations[1].body, &auth); err != nil {
		t.Fatal(err)
	}
	if auth.Session.Generation != f.freshGeneration || auth.Session.NativeSessionID != "" || auth.Session.ScopeID != f.task.IssueID {
		t.Fatal("fresh launch was not authorized under the server-minted binding")
	}
	argv, err := os.ReadFile(f.argv)
	if err != nil {
		t.Fatal(err)
	}
	segments := strings.Split(string(argv), "PHYSICAL_START\n")
	if len(segments) != 3 || strings.Contains(segments[1], "--resume") {
		t.Fatalf("second process was not a fresh launch: %q", argv)
	}
	prompt := knowledgeCapturedPrompt(t, f)
	if !strings.Contains(prompt, privateKnowledgeTestBody) {
		t.Fatal("fresh session did not receive the current approved knowledge")
	}
	if !strings.Contains(prompt, execenv.SessionContinuityNoticeIssue) {
		t.Fatal("fresh session lost the ordinary continuity notice")
	}
	observed := false
	for _, call := range f.callsFor("observations") {
		var observation protocol.TaskKnowledgeObservation
		if err := json.Unmarshal(call.body, &observation); err != nil {
			t.Fatal(err)
		}
		if observation.AttemptNonce == auth.AttemptNonce && observation.Kind == "native_session" && observation.NativeSessionID == "fresh-after-drop" {
			observed = true
		}
	}
	if !observed {
		t.Fatal("fresh native identity was not observed under its own attempt")
	}
}

// The daemon binds only a response that differs from the claim in generation
// and native ID alone; anything else is refused with zero processes.
func TestKnowledgeUnreachablePriorWorkdirRefusesEchoedStaleBinding(t *testing.T) {
	for _, tc := range []string{"echoed stale binding", "reused claimed generation"} {
		t.Run(tc, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			_, firstGeneration := f.unreachableResumeTurn(t)
			if tc == "echoed stale binding" {
				f.ignoreResumeUnavailable = true
			} else {
				f.freshGeneration = firstGeneration
			}
			second := f.run(t)
			if second.Status != "blocked" || second.FailureReason != agent.KnowledgeContextChanged {
				t.Fatalf("unsafe rebind was not refused: status=%s reason=%s", second.Status, second.FailureReason)
			}
			prepares := f.callsFor("prepare")
			if len(prepares) != 2 {
				t.Fatalf("prepare count = %d, want 2", len(prepares))
			}
			var flagged map[string]json.RawMessage
			if err := json.Unmarshal(prepares[1].body, &flagged); err != nil {
				t.Fatal(err)
			}
			if string(flagged["resume_unavailable"]) != "true" {
				t.Fatal("dropped resume did not report resume_unavailable")
			}
			if len(f.callsFor("authorize")) != 1 {
				t.Fatal("refused rebind requested a launch authorization")
			}
			argv, _ := os.ReadFile(f.argv)
			if strings.Count(string(argv), "PHYSICAL_START") != 1 {
				t.Fatal("refused rebind started a process")
			}
		})
	}
}

// An upgraded daemon talking to an API that predates resume_unavailable gets
// the strict decoder's 400 and fails closed with no process for that input.
func TestKnowledgeUnreachableResumeOnOlderServerFailsClosed(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.unreachableResumeTurn(t)
	f.mu.Lock()
	f.refuseAction, f.refuseCode, f.refuseStatus = "prepare", "invalid_knowledge_request", http.StatusBadRequest
	f.mu.Unlock()
	second := f.run(t)
	if second.Status != "blocked" || second.FailureReason != agent.KnowledgeUnavailable {
		t.Fatalf("older API rejection did not fail closed: status=%s reason=%s", second.Status, second.FailureReason)
	}
	if len(f.callsFor("prepare")) != 2 || len(f.callsFor("authorize")) != 1 {
		t.Fatal("older API rejection was retried or reached launch authorization")
	}
	argv, _ := os.ReadFile(f.argv)
	if strings.Count(string(argv), "PHYSICAL_START") != 1 {
		t.Fatal("older API rejection started a process")
	}
}

func TestKnowledgeChangedNativeIDIsObservedButNeverRelabeled(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.task.Agent.CustomEnv["FAKE_RESULT_NATIVE"] = "unapproved-new-native"
	result := f.run(t)
	if result.FailureReason != agent.KnowledgeContextChanged || result.SessionID != f.native {
		t.Fatal("native mismatch relabeled prior exposure")
	}
	var natives []protocol.TaskKnowledgeObservation
	for _, call := range f.callsFor("observations") {
		var observation protocol.TaskKnowledgeObservation
		if err := json.Unmarshal(call.body, &observation); err != nil {
			t.Fatal(err)
		}
		if observation.Kind == "native_session" {
			natives = append(natives, observation)
		}
	}
	if len(natives) != 2 || natives[0].NativeSessionID != f.native || natives[1].NativeSessionID != "unapproved-new-native" || natives[1].Source != "result" || natives[0].AttemptNonce != natives[1].AttemptNonce {
		t.Fatal("raw native mismatch was not retained under original attempt")
	}
	if len(f.callsFor("session")) != 0 {
		t.Fatal("native mismatch used ordinary pin bypass")
	}
}

func TestKnowledgePrepareBusyExhaustionRetainsOriginalReceipt(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.refuseAction, f.refuseCode, f.refuseStatus = "prepare", "knowledge_busy", 503
	result := f.run(t)
	if result.FailureReason != "knowledge_busy" {
		t.Fatal("busy exhaustion lost canonical terminal reason")
	}
	calls := f.callsFor("prepare")
	if len(calls) != 3 || !bytes.Equal(calls[0].body, calls[1].body) || !bytes.Equal(calls[1].body, calls[2].body) {
		t.Fatal("busy retry exceeded bound or replaced receipt")
	}
	if len(f.callsFor("authorize")) != 0 {
		t.Fatal("busy preparation downgraded into a launch")
	}
}

func TestKnowledgeStartRefusalKeepsTerminalClassificationAndFence(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.refuseAction, f.refuseCode = "start", agent.KnowledgeContextChanged
	_, err := f.d.runTask(context.Background(), f.task, "claude", 0, f.d.logger)
	if err == nil || taskRunFailureReason(err) != agent.KnowledgeContextChanged {
		t.Fatal("start refusal became retryable generic agent error")
	}
	calls := f.callsFor("start")
	var body struct {
		Fence protocol.TaskKnowledgeFence `json:"knowledge_fence"`
	}
	if len(calls) != 1 || json.Unmarshal(calls[0].body, &body) != nil || body.Fence.ClaimGeneration != "2" || body.Fence.DispatchedAt != f.task.KnowledgeContext.DispatchedAt {
		t.Fatal("start lost original claim fence")
	}
	if len(f.callsFor("prepare")) != 0 {
		t.Fatal("failed StartTask disclosed private knowledge")
	}
}

func TestKnowledgeCancellationAccountingUsesActualProcesses(t *testing.T) {
	for _, phase := range []string{"refused", "started", "native_refused"} {
		t.Run(phase, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			if phase == "refused" {
				f.refuseAction, f.refuseCode = "authorize", agent.KnowledgeAuthorizationDenied
			}
			if phase == "native_refused" {
				f.observationRefusal = "native_session"
			}
			execution := &cancellationExecution{}
			ctx := context.WithValue(context.Background(), cancelExecutionKey{}, execution)
			if _, err := f.d.runTask(ctx, f.task, "claude", 0, f.d.logger); err != nil {
				t.Fatal(err)
			}
			execution.mu.Lock()
			defer execution.mu.Unlock()
			if !execution.productionRunner {
				t.Fatal("ordinary runner identity lost")
			}
			if phase == "refused" {
				if execution.attempts != 0 || execution.exited != 0 {
					t.Fatal("refused authorization was counted as a physical process")
				}
			} else if execution.attempts != 1 || execution.exited != 1 {
				t.Fatal("actual process start/termination lost from cancellation accounting")
			}
		})
	}
}

func TestKnowledgeObservationRequiresPositiveAcknowledgement(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.observationAckFalse = "native_session"
	result := f.run(t)
	if result.FailureReason != agent.KnowledgeUnavailable || result.SessionID != "" {
		t.Fatal("HTTP success without acceptance published native identity")
	}
	if len(f.callsFor("session")) != 0 {
		t.Fatal("unaccepted observation escaped through normal pin")
	}
}

func TestKnowledgeObservationRejectsNoncanonicalAcknowledgement(t *testing.T) {
	for name, raw := range map[string]string{
		"case_alias": `{"accepted":false,"Accepted":true}`,
		"duplicate":  `{"accepted":false,"accepted":true}`,
		"private":    `{"accepted":true,"body":"private-ack-sentinel"}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			f.observationAckRaw = raw
			result := f.run(t)
			if result.FailureReason != agent.KnowledgeUnavailable || result.SessionID != "" || len(f.callsFor("session")) != 0 {
				t.Fatal("noncanonical acknowledgement published native identity")
			}
			var requests [][]byte
			for _, call := range f.callsFor("observations") {
				var observation protocol.TaskKnowledgeObservation
				if err := json.Unmarshal(call.body, &observation); err != nil {
					t.Fatal(err)
				}
				if observation.Kind == "native_session" {
					requests = append(requests, call.body)
				}
			}
			if len(requests) != 3 || !bytes.Equal(requests[0], requests[1]) || !bytes.Equal(requests[1], requests[2]) {
				t.Fatal("uncertain acknowledgement did not preserve the original observation identity")
			}
			if strings.Contains(f.logs.String(), "private-ack-sentinel") {
				t.Fatal("private acknowledgement data escaped into ordinary logs")
			}
		})
	}
}

func TestKnowledgeCleanupObservationSurvivesCallerCancellation(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.cancelOnNative = cancel
	if _, err := f.d.runTask(ctx, f.task, "claude", 0, f.d.logger); err != nil {
		t.Fatal(err)
	}
	terminated := false
	for _, call := range f.callsFor("observations") {
		var observation protocol.TaskKnowledgeObservation
		if err := json.Unmarshal(call.body, &observation); err != nil {
			t.Fatal(err)
		}
		if observation.Kind == "terminated" && observation.WaitObserved {
			terminated = true
		}
	}
	if ctx.Err() == nil || !terminated {
		t.Fatal("caller cancellation prevented original process cleanup observation")
	}
}

func TestKnowledgePreparationBudgetIncludesFullEnvelope(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	fingerprint := strings.Repeat("a", 64)
	p := f.prepare(protocol.TaskKnowledgePrepareRequest{Session: f.task.KnowledgeContext.Session, InvocationFingerprint: fingerprint})
	item := p.Contributions[0]
	p.Contributions, p.Manifest.Entries = nil, nil
	for i := 0; i < 4; i++ {
		next := item
		next.CandidateID = uuid.NewString()
		next.Content = strings.Repeat("x", 60000)
		p.Contributions = append(p.Contributions, next)
	}
	material, _, err := protocol.CollaborationKnowledgeSizes(p.Contributions)
	if err != nil {
		t.Fatal(err)
	}
	growth := (protocol.CollaborationKnowledgeMaxRuntimeBytes-material)/4 - 1
	for i := range p.Contributions {
		next := &p.Contributions[i]
		next.Content += strings.Repeat("x", growth)
		next.ContentSHA256 = protocol.CollaborationKnowledgeHash(next.Content)
		p.Manifest.Entries = append(p.Manifest.Entries, protocol.TaskKnowledgeManifestEntry{CandidateID: next.CandidateID, Version: next.Version, ContentSHA256: next.ContentSHA256, SourceIssueID: next.SourceIssueID, Target: next.Target})
	}
	manifest, _ := json.Marshal(p.Manifest)
	p.ManifestSHA256 = protocol.CollaborationKnowledgeHash(string(manifest))
	if err := protocol.ValidateCollaborationKnowledgeRuntimeBudget(p.Contributions, 0); err != nil {
		t.Fatal("fixture contributions alone exceed budget")
	}
	material, _, err = protocol.CollaborationKnowledgeSizes(p)
	if err != nil || material <= protocol.CollaborationKnowledgeMaxRuntimeBytes {
		t.Fatal("fixture full envelope must exceed budget")
	}
	if reason := agent.KnowledgeFailureReason(validateKnowledgePreparation(f.task, f.task.KnowledgeContext.Session, false, fingerprint, p)); reason != agent.KnowledgeUnavailable {
		t.Fatal("full private response was not budgeted")
	}
}

// knowledgeDescriptorMutations are claim descriptors this daemon cannot decode
// strictly: a session field from a newer server, a case alias of a declared
// field, a malformed session identity, and a fence it cannot read back.
var knowledgeDescriptorMutations = []struct {
	name        string
	recoverable bool
	mutate      func(descriptor, session map[string]any)
}{
	{"future_session_field", true, func(_, session map[string]any) { session["future_field"] = "x" }},
	{"session_case_alias", true, func(_, session map[string]any) { session["Native_Session_ID"] = "x" }},
	{"malformed_session_generation", true, func(_, session map[string]any) { session["generation"] = "not-a-uuid" }},
	{"unreadable_fence", false, func(descriptor, session map[string]any) {
		session["future_field"] = "x"
		descriptor["claim_generation"] = 2
	}},
}

// mutatedKnowledgeTask is task with its knowledge_context descriptor changed by
// mutate, encoded the way a claim response carries it.
func mutatedKnowledgeTask(t *testing.T, task Task, mutate func(descriptor, session map[string]any)) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	descriptor := generic["knowledge_context"].(map[string]any)
	mutate(descriptor, descriptor["session"].(map[string]any))
	raw, err = json.Marshal(generic)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// plainSiblingTask is a base task in the same claim, with no knowledge context.
func plainSiblingTask(f *knowledgeDaemonFixture) Task {
	plain := f.task
	plain.ID, plain.KnowledgeContext = uuid.NewString(), nil
	return plain
}

func TestKnowledgeClaimDescriptorDecodeFailureRefusesOnlyThatTask(t *testing.T) {
	for _, tc := range knowledgeDescriptorMutations {
		t.Run(tc.name, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			fence := protocol.TaskKnowledgeFence{ClaimGeneration: f.task.KnowledgeContext.ClaimGeneration, DispatchedAt: f.task.KnowledgeContext.DispatchedAt}
			plain := plainSiblingTask(f)
			sibling := f.task
			sibling.ID = uuid.NewString()
			descriptor := *f.task.KnowledgeContext
			descriptor.TaskID = sibling.ID
			sibling.KnowledgeContext = &descriptor
			body, err := json.Marshal(map[string]any{"tasks": []any{plain, sibling, mutatedKnowledgeTask(t, f.task, tc.mutate)}})
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Tasks []*Task `json:"tasks"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatalf("one undecodable descriptor discarded the whole claim: %v", err)
			}
			if len(envelope.Tasks) != 3 || !reflect.DeepEqual(*envelope.Tasks[0], plain) || !reflect.DeepEqual(*envelope.Tasks[1], sibling) {
				t.Fatal("well-formed tasks in the same claim changed")
			}
			if taskKnowledgeFence(*envelope.Tasks[0]) != nil || *taskKnowledgeFence(*envelope.Tasks[1]) != fence {
				t.Fatal("well-formed tasks lost their own knowledge fences")
			}
			rejected := envelope.Tasks[2]
			if rejected.KnowledgeContext != nil || rejected.ID != f.task.ID {
				t.Fatal("an undecodable descriptor survived as a knowledge context")
			}
			got := taskKnowledgeFence(*rejected)
			if tc.recoverable && (got == nil || *got != fence) || !tc.recoverable && got != nil {
				t.Fatalf("recovered fence = %+v, want original=%v", got, tc.recoverable)
			}
			f.task = *rejected
			result := f.run(t)
			if result.Status != "blocked" || result.FailureReason != agent.KnowledgeContextChanged {
				t.Fatalf("rejected descriptor was not refused: status=%s reason=%s", result.Status, result.FailureReason)
			}
			if len(f.callsFor("prepare")) != 0 || len(f.callsFor("authorize")) != 0 {
				t.Fatal("rejected descriptor requested private knowledge")
			}
			if _, err := os.Stat(f.argv); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("rejected descriptor started a provider process")
			}
			if tc.recoverable && (result.KnowledgeFence == nil || *result.KnowledgeFence != fence) || !tc.recoverable && result.KnowledgeFence != nil {
				t.Fatalf("terminal report fence = %+v", result.KnowledgeFence)
			}
			starts := f.callsFor("start")
			var start struct {
				Fence *protocol.TaskKnowledgeFence `json:"knowledge_fence"`
			}
			if len(starts) != 1 || json.Unmarshal(starts[0].body, &start) != nil {
				t.Fatal("rejected task did not report its start")
			}
			if tc.recoverable && (start.Fence == nil || *start.Fence != fence) || !tc.recoverable && start.Fence != nil {
				t.Fatalf("start fence = %+v", start.Fence)
			}
		})
	}
}

func TestKnowledgeBatchClaimSurvivesOneUndecodableDescriptor(t *testing.T) {
	for _, transport := range []string{"http_single", "http_batch", "ws_batch", "http_legacy_fallback", "managed_single", "managed_http", "managed_ws"} {
		t.Run(transport, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			fence := protocol.TaskKnowledgeFence{ClaimGeneration: f.task.KnowledgeContext.ClaimGeneration, DispatchedAt: f.task.KnowledgeContext.DispatchedAt}
			plain := plainSiblingTask(f)
			plainRaw, err := json.Marshal(plain)
			if err != nil {
				t.Fatal(err)
			}
			f.claimTasks = []json.RawMessage{plainRaw, mutatedKnowledgeTask(t, f.task, knowledgeDescriptorMutations[0].mutate)}
			var scoped *managedWorkspaceTransport
			if strings.HasPrefix(transport, "managed") {
				scoped = installKnowledgeManagedFixture(t, f)
			}
			if transport == "ws_batch" {
				attachKnowledgeClaimWS(t, f, f.d.wsRPC)
			}
			if transport == "managed_ws" {
				attachKnowledgeClaimWS(t, f, scoped.rpc)
			}
			var tasks []*Task
			switch transport {
			case "http_single", "managed_single":
				var task *Task
				task, err = f.d.client.ClaimTask(context.Background(), f.task.RuntimeID)
				tasks = []*Task{task}
			case "http_batch", "managed_http":
				tasks, err = f.d.client.ClaimTasks(context.Background(), "daemon", []string{f.task.RuntimeID}, 2)
			case "http_legacy_fallback":
				f.d.batchClaimUnsupported.Store(true)
				tasks, err = f.d.ClaimTasksWSFirst(context.Background(), "daemon", []string{f.task.RuntimeID}, 2)
			default:
				tasks, err = f.d.ClaimTasksWSFirst(context.Background(), "daemon", []string{f.task.RuntimeID}, 2)
			}
			if err != nil {
				t.Fatalf("one undecodable descriptor lost the committed claim: %v", err)
			}
			batch := !strings.HasSuffix(transport, "_single") && transport != "http_legacy_fallback"
			if want := map[bool]int{true: 2, false: 1}[batch]; len(tasks) != want {
				t.Fatalf("claim returned %d tasks, want %d", len(tasks), want)
			}
			if batch && (tasks[0] == nil || !reflect.DeepEqual(*tasks[0], plain) || taskKnowledgeFence(*tasks[0]) != nil) {
				t.Fatal("the well-formed task was changed or marked rejected")
			}
			rejected := tasks[len(tasks)-1]
			if rejected == nil || rejected.ID != f.task.ID || rejected.KnowledgeContext != nil {
				t.Fatal("the undecodable descriptor survived as a knowledge context")
			}
			if got := taskKnowledgeFence(*rejected); got == nil || *got != fence {
				t.Fatalf("rejected task lost its original fence: %+v", got)
			}
			calls := f.callsFor("claim")
			if strings.HasSuffix(transport, "_ws") || transport == "ws_batch" {
				if len(calls) != 1 || calls[0].path != "ws/claim" {
					t.Fatalf("ws claim fell back to http: %d calls", len(calls))
				}
			} else if len(calls) != 1 {
				t.Fatalf("claim count %d", len(calls))
			}
		})
	}
}

// A refused started observation still leaves a real process. The daemon counts
// it for cancellation, the lifecycle kills and waits it, and the terminated
// observation reports that waited exit, so a user cancel gets process-exit
// confirmation. No second process is authorized.
func TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.observationRefusal = "started"
	f.task.Agent.CustomEnv["FAKE_NATIVE"] = ""
	execution := &cancellationExecution{}
	ctx := context.WithValue(context.Background(), cancelExecutionKey{}, execution)
	result, err := f.d.runTask(ctx, f.task, "claude", 0, f.d.logger)
	if err != nil {
		t.Fatal(err)
	}
	if result.FailureReason != agent.KnowledgeContextChanged {
		t.Fatalf("refused started observation lost its terminal reason: %s", result.FailureReason)
	}
	execution.mu.Lock()
	confirmed := execution.productionRunner && execution.attempts == 1 && execution.exited == 1
	execution.mu.Unlock()
	if !confirmed {
		t.Fatal("cancellation cannot confirm the exit of the process whose started observation was refused")
	}
	var started, terminated []protocol.TaskKnowledgeObservation
	for _, call := range f.callsFor("observations") {
		var observation protocol.TaskKnowledgeObservation
		if err := json.Unmarshal(call.body, &observation); err != nil {
			t.Fatal(err)
		}
		switch observation.Kind {
		case "started":
			started = append(started, observation)
		case "terminated":
			terminated = append(terminated, observation)
		}
	}
	if len(started) == 0 || len(terminated) != 1 {
		t.Fatalf("started=%d terminated=%d, want the refused start and exactly one termination", len(started), len(terminated))
	}
	end := terminated[0]
	if end.TerminationKind != agent.LaunchManagedEnded || !end.WaitObserved || end.ProcessID != started[0].ProcessID ||
		end.ProcessInstanceID != started[0].ProcessInstanceID || end.AttemptNonce != started[0].AttemptNonce {
		t.Fatalf("termination was not the waited exit of the started process: %+v", end)
	}
	if len(f.callsFor("authorize")) != 1 {
		t.Fatal("refused started observation authorized another process")
	}
}
