package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const privateKnowledgeTestBody = "private-knowledge-stdin-sentinel"

type knowledgeHTTPCall struct {
	path string
	body []byte
}

type knowledgeDaemonFixture struct {
	observationAckFalse                          string
	observationAckRaw                            string
	cancelOnNative                               context.CancelFunc
	d                                            *Daemon
	task                                         Task
	bin, capture, argv, credentials              string
	logs                                         bytes.Buffer
	mu                                           sync.Mutex
	calls                                        []knowledgeHTTPCall
	empty                                        bool
	refuseAction, refuseCode, observationRefusal string
	refuseStatus                                 int
	dropAction                                   string
	dropped                                      bool
	wrongAuthorization                           bool
	wrongPreparation                             bool
	wrongManifest                                bool
	revokeAfterDrop                              bool
	prepareID                                    string
	sourceIssueID                                string
	native                                       string
}

func newKnowledgeDaemonFixture(t *testing.T) *knowledgeDaemonFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test-created executable; no installed CLI")
	}
	f := &knowledgeDaemonFixture{prepareID: uuid.NewString(), sourceIssueID: uuid.NewString(), native: "native-observed-test"}
	root := t.TempDir()
	f.bin, f.capture, f.argv, f.credentials = filepath.Join(root, "fake-claude"), filepath.Join(root, "stdin.json"), filepath.Join(root, "argv"), filepath.Join(root, "credentials")
	script := `#!/bin/sh
printf '%s\n' "$@" >> "$CAPTURE_ARGV"
printf '%s\n' 'PHYSICAL_START' >> "$CAPTURE_ARGV"
printf '%s' "$MULTICA_TOKEN" > "$CAPTURE_CREDENTIALS"
IFS= read -r payload
printf '%s' "$payload" > "$CAPTURE_INPUT"
printf '{"type":"system","session_id":"%s"}\n' "$FAKE_NATIVE"
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"private-knowledge-stdin-sentinel"}]}}'
printf '{"type":"result","subtype":"success","is_error":false,"session_id":"%s","result":"private-knowledge-stdin-sentinel","modelUsage":{"test-model":{"inputTokens":12,"outputTokens":7}}}\n' "${FAKE_RESULT_NATIVE:-$FAKE_NATIVE}"
`
	if err := os.WriteFile(f.bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	f.task = Task{ID: uuid.NewString(), WorkspaceID: uuid.NewString(), RuntimeID: uuid.NewString(), AgentID: uuid.NewString(), IssueID: uuid.NewString(), AuthToken: "mat_test_task_only",
		WorkspaceContext: "independent workspace base", ProjectDescription: "independent project base"}
	f.task.Agent = &AgentData{ID: f.task.AgentID, Name: "Test agent", CustomEnv: map[string]string{
		"CAPTURE_ARGV": f.argv, "CAPTURE_INPUT": f.capture, "CAPTURE_CREDENTIALS": f.credentials, "FAKE_NATIVE": f.native,
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.serve(t, w, r) }))
	t.Cleanup(server.Close)
	f.d = New(Config{ServerBaseURL: server.URL, WorkspacesRoot: filepath.Join(root, "workspaces"), AgentTimeout: 5 * time.Second,
		Agents: map[string]AgentEntry{"claude": {Path: f.bin}}}, slog.New(slog.NewTextHandler(&f.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	// Keep preparation in the test process; agent subprocesses still use the
	// actual production backend and test-owned absolute executable above.
	f.d.executionEnvironmentCommand = nil
	f.d.client.SetToken("mdt_test_daemon_only")
	f.d.runtimeIndex[f.task.RuntimeID] = Runtime{ID: f.task.RuntimeID, Provider: "claude"}
	f.bindClaim("issue", f.task.IssueID)
	return f
}

func (f *knowledgeDaemonFixture) bindClaim(kind, scope string) {
	inv := f.d.knowledgeInvocation(f.task.RuntimeID)
	dispatched := time.Now().UTC().Format(time.RFC3339Nano)
	f.task.KnowledgeContext = &protocol.TaskKnowledgeClaimDescriptor{ProtocolVersion: 1, TaskID: f.task.ID, RuntimeID: f.task.RuntimeID,
		ClaimGeneration: "2", DispatchedAt: dispatched, Provider: "claude", Mode: "local", ConfigurationID: inv.ConfigurationID, PreparationRequired: true,
		Session: protocol.TaskKnowledgeSessionBinding{Provider: "claude", RuntimeID: f.task.RuntimeID, AgentID: f.task.AgentID, ConfigurationID: inv.ConfigurationID,
			ScopeKind: kind, ScopeID: scope, Generation: uuid.NewString()}}
	f.task.ExecutionFence = &ExecutionFence{RuntimeID: f.task.RuntimeID, DispatchedAt: dispatched}
}

func (f *knowledgeDaemonFixture) prepare(req protocol.TaskKnowledgePrepareRequest) protocol.TaskKnowledgePrepareResponse {
	target := protocol.CollaborationKnowledgeTarget{Kind: "workspace", WorkspaceID: f.task.WorkspaceID}
	items := []protocol.CollaborationKnowledgeContribution{}
	entries := []protocol.TaskKnowledgeManifestEntry{}
	if !f.empty {
		item := protocol.CollaborationKnowledgeContribution{CandidateID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Version: "1", SourceIssueID: f.sourceIssueID, Target: target,
			Content: privateKnowledgeTestBody, ContentSHA256: protocol.CollaborationKnowledgeHash(privateKnowledgeTestBody), Summary: "Attributed approved context",
			Sources:  []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: f.sourceIssueID, Version: "1", SHA256: protocol.CollaborationKnowledgeHash("source")}},
			Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: protocol.CollaborationKnowledgeApplicability{Surfaces: []string{"issue", "chat", "quick_create", "autopilot"}}}
		items = append(items, item)
		entries = append(entries, protocol.TaskKnowledgeManifestEntry{CandidateID: item.CandidateID, Version: item.Version, ContentSHA256: item.ContentSHA256, SourceIssueID: item.SourceIssueID, Target: item.Target})
	}
	manifest := protocol.TaskKnowledgeManifest{ProtocolVersion: 1, Entries: entries}
	raw, _ := json.Marshal(manifest)
	p := protocol.TaskKnowledgePrepareResponse{ProtocolVersion: 1, PrepareID: f.prepareID, InvocationFingerprint: req.InvocationFingerprint,
		Session: req.Session, Contributions: items, Manifest: manifest, ManifestSHA256: protocol.CollaborationKnowledgeHash(string(raw)),
		Targets: []protocol.TaskKnowledgeTargetRevision{{Target: target, Revision: "3"}}}
	if f.wrongPreparation {
		p.Session.Generation = uuid.NewString()
	}
	if f.wrongManifest {
		p.ManifestSHA256 = strings.Repeat("0", 64)
	}
	return p
}

func (f *knowledgeDaemonFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
		w.WriteHeader(400)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, knowledgeHTTPCall{r.URL.Path, raw})
	action := filepath.Base(r.URL.Path)
	if strings.Contains(r.URL.Path, "/knowledge-context/") && r.Header.Get("Authorization") != "Bearer mdt_test_daemon_only" {
		t.Error("private endpoint received another credential")
		w.WriteHeader(403)
		return
	}
	if action == f.dropAction && !f.dropped {
		f.dropped = true
		if f.revokeAfterDrop {
			f.refuseAction, f.refuseCode = action, agent.KnowledgeAuthorizationDenied
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
		return
	}
	if action == f.refuseAction {
		status := f.refuseStatus
		if status == 0 {
			status = http.StatusConflict
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": f.refuseCode})
		return
	}
	switch action {
	case "claim":
		if strings.Contains(r.URL.Path, "/runtimes/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"task": f.task})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{"tasks": []Task{f.task}})
		}
	case "prepare":
		var req protocol.TaskKnowledgePrepareRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if !protocol.ValidTaskKnowledgeDigest(req.InvocationFingerprint) {
			t.Error("missing actual invocation fingerprint")
		}
		_ = json.NewEncoder(w).Encode(f.prepare(req))
	case "authorize":
		var req protocol.TaskKnowledgeAuthorizeRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		resp := protocol.TaskKnowledgeAuthorization{ProtocolVersion: 1, PrepareID: req.PrepareID, AttemptSequence: req.AttemptSequence, AttemptNonce: req.AttemptNonce,
			ManifestSHA256: req.ManifestSHA256, PromptSHA256: req.PromptSHA256, InvocationFingerprint: req.InvocationFingerprint, Session: req.Session}
		if f.wrongAuthorization {
			resp.AttemptNonce = uuid.NewString()
		}
		_ = json.NewEncoder(w).Encode(resp)
	case "observations":
		var req protocol.TaskKnowledgeObservation
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if req.Kind == f.observationRefusal {
			w.WriteHeader(409)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": agent.KnowledgeContextChanged})
			return
		}
		if req.Kind == f.observationAckFalse {
			_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": false})
			return
		}
		if req.Kind == "native_session" && f.observationAckRaw != "" {
			_, _ = w.Write([]byte(f.observationAckRaw))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
		if req.Kind == "native_session" && f.cancelOnNative != nil {
			f.cancelOnNative()
		}
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}
}

func (f *knowledgeDaemonFixture) callsFor(action string) []knowledgeHTTPCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var calls []knowledgeHTTPCall
	for _, call := range f.calls {
		if filepath.Base(call.path) == action {
			calls = append(calls, call)
		}
	}
	return calls
}

func (f *knowledgeDaemonFixture) run(t *testing.T) TaskResult {
	t.Helper()
	result, err := f.d.runTask(context.Background(), f.task, "claude", 0, f.d.logger)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestKnowledgeAllOrdinaryPromptBranchesReachActualStdin(t *testing.T) {
	for _, surface := range []string{"issue", "comment", "chat", "quick_create", "autopilot"} {
		t.Run(surface, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			switch surface {
			case "comment":
				f.task.TriggerCommentID = uuid.NewString()
				f.task.TriggerCommentContent = "comment user intent"
			case "chat":
				f.task.IssueID = ""
				f.task.ChatSessionID = uuid.NewString()
				f.task.ChatMessage = "chat user intent"
				f.bindClaim("chat", f.task.ChatSessionID)
			case "quick_create":
				f.task.IssueID = ""
				f.task.QuickCreatePrompt = "quick create user intent"
				f.bindClaim("quick_create", f.task.ID)
			case "autopilot":
				f.task.IssueID = ""
				f.task.AutopilotRunID = uuid.NewString()
				f.task.AutopilotDescription = "autopilot user intent"
				f.bindClaim("autopilot", f.task.AutopilotRunID)
			}
			result := f.run(t)
			if result.Status != "completed" || result.SessionID != f.native || len(result.Usage) != 1 {
				t.Fatalf("unexpected outcome: %s %s %s", result.Status, result.FailureReason, result.SessionID)
			}
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
			if err := json.Unmarshal(input, &wire); err != nil {
				t.Fatal(err)
			}
			if len(wire.Message.Content) != 1 {
				t.Fatal("expected one final input")
			}
			prompt := wire.Message.Content[0].Text
			if !strings.Contains(prompt, privateKnowledgeTestBody) {
				t.Fatal("approved contribution did not reach actual stdin")
			}
			var auth protocol.TaskKnowledgeAuthorizeRequest
			if err := json.Unmarshal(f.callsFor("authorize")[0].body, &auth); err != nil {
				t.Fatal(err)
			}
			if auth.PromptSHA256 != protocol.CollaborationKnowledgeHash(prompt) || auth.Session != f.task.KnowledgeContext.Session {
				t.Fatal("authorization does not bind actual stdin and session")
			}
			var prepare protocol.TaskKnowledgePrepareRequest
			if err := json.Unmarshal(f.callsFor("prepare")[0].body, &prepare); err != nil {
				t.Fatal(err)
			}
			if auth.InvocationFingerprint != prepare.InvocationFingerprint || auth.PrepareID != f.prepareID || auth.ClaimGeneration != "2" {
				t.Fatal("launch changed finalized preparation identity")
			}
			observations := f.callsFor("observations")
			if len(observations) != 3 {
				t.Fatalf("observation count = %d", len(observations))
			}
			var processID int
			for i, call := range observations {
				var got protocol.TaskKnowledgeObservation
				if err := json.Unmarshal(call.body, &got); err != nil {
					t.Fatal(err)
				}
				if got.AttemptNonce != auth.AttemptNonce || got.AttemptSequence != auth.AttemptSequence {
					t.Fatal("observation lost physical attempt identity")
				}
				switch i {
				case 0:
					if got.Kind != "started" || got.ProcessID <= 0 || got.ProcessInstanceID == "" {
						t.Fatal("no actual Started fact")
					}
					processID = got.ProcessID
				case 1:
					if got.Kind != "native_session" || got.NativeSessionID != f.native || got.Source != "system" {
						t.Fatal("native identity was not directly observed")
					}
				case 2:
					if got.Kind != "terminated" || !got.WaitObserved || got.ProcessID != processID {
						t.Fatal("original process termination was lost")
					}
				}
			}
			if len(f.callsFor("session")) != 0 {
				t.Fatal("negotiated execution used asynchronous ordinary pin")
			}
			credential, _ := os.ReadFile(f.credentials)
			if string(credential) != f.task.AuthToken {
				t.Fatal("child did not retain task credential")
			}
			if strings.Contains(f.logs.String(), privateKnowledgeTestBody) {
				t.Fatal("private body leaked into generic log")
			}
			brief, err := os.ReadFile(filepath.Join(result.WorkDir, "CLAUDE.md"))
			if err != nil || !bytes.Contains(brief, []byte("independent workspace base")) {
				t.Fatal("knowledge displaced independent stable workspace base")
			}
			if err := filepath.WalkDir(f.d.cfg.WorkspacesRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if bytes.Contains(b, []byte(privateKnowledgeTestBody)) {
					t.Errorf("private contribution persisted in stable file: %s", filepath.Base(path))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOrdinaryKnowledgeFirstLaunchRefusalIsTerminal(t *testing.T) {
	for _, reason := range []string{agent.KnowledgeContextChanged, agent.KnowledgeAuthorizationDenied, agent.KnowledgeUnavailable, agent.KnowledgeInvocationUnsupported, "knowledge_busy"} {
		t.Run(reason, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			f.refuseAction, f.refuseCode = "authorize", reason
			result := f.run(t)
			if result.Status != "blocked" || result.FailureReason != reason || result.Comment != reason {
				t.Fatal("refusal lost terminal reason")
			}
			if _, err := os.Stat(f.argv); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused first launch executed a process")
			}
			if len(f.callsFor("prepare")) != 1 || len(f.callsFor("authorize")) != 1 || len(f.callsFor("observations")) != 0 {
				t.Fatal("refusal replayed or fabricated process observations")
			}
			f.d.reportTaskResult(context.Background(), f.task.ID, result, f.d.logger)
			var failure struct {
				Reason string                      `json:"failure_reason"`
				Fence  protocol.TaskKnowledgeFence `json:"knowledge_fence"`
			}
			if err := json.Unmarshal(f.callsFor("fail")[0].body, &failure); err != nil {
				t.Fatal(err)
			}
			if failure.Reason != reason || failure.Fence.ClaimGeneration != "2" || failure.Fence.DispatchedAt != f.task.KnowledgeContext.DispatchedAt {
				t.Fatal("terminal report lost original claim fence")
			}
		})
	}
}

func TestKnowledgeOriginalReceiptSurvivesLostResponse(t *testing.T) {
	for _, action := range []string{"prepare", "authorize", "observations"} {
		t.Run(action, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			f.dropAction = action
			if result := f.run(t); result.Status != "completed" {
				t.Fatalf("lost response recovery failed: %s", result.FailureReason)
			}
			calls := f.callsFor(action)
			if len(calls) < 2 || !bytes.Equal(calls[0].body, calls[1].body) {
				t.Fatal("uncertain request was replayed under a different receipt identity")
			}
			argv, _ := os.ReadFile(f.argv)
			if strings.Count(string(argv), "PHYSICAL_START") != 1 {
				t.Fatal("transport recovery replayed business input")
			}
		})
	}
}

func TestKnowledgeWrongIdentitiesNeverLaunch(t *testing.T) {
	for _, which := range []string{"scope", "runtime", "config", "claim", "preparation", "manifest", "authorization", "selector"} {
		t.Run(which, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			switch which {
			case "scope":
				f.task.KnowledgeContext.Session.ScopeID = uuid.NewString()
			case "runtime":
				f.task.KnowledgeContext.RuntimeID = uuid.NewString()
			case "config":
				f.task.KnowledgeContext.ConfigurationID = "changed"
			case "claim":
				f.task.ExecutionFence.DispatchedAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
			case "preparation":
				f.wrongPreparation = true
			case "manifest":
				f.wrongManifest = true
			case "authorization":
				f.wrongAuthorization = true
			case "selector":
				f.task.Agent.CustomArgs = []string{"--continue"}
			}
			result := f.run(t)
			if result.Status != "blocked" || !knowledgeFailureCode(result.FailureReason) {
				t.Fatal("invalid binding was not refused")
			}
			if _, err := os.Stat(f.argv); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid binding reached actual executable")
			}
			if which == "selector" && len(f.callsFor("prepare")) != 0 {
				t.Fatal("selector was validated after private disclosure")
			}
		})
	}
}

func TestKnowledgeNativeRefusalRetainsFailureEvidence(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.observationRefusal = "terminated"
	result := f.run(t)
	if result.Status != "blocked" || result.FailureReason != agent.KnowledgeContextChanged || len(result.Usage) != 1 {
		t.Fatal("post-start refusal lost terminal classification or usage")
	}
	if result.Comment != agent.KnowledgeContextChanged {
		t.Fatal("private output became terminal success text")
	}
	var text strings.Builder
	for _, call := range f.callsFor("messages") {
		var body struct {
			Messages []TaskMessageData `json:"messages"`
		}
		if err := json.Unmarshal(call.body, &body); err != nil {
			t.Fatal(err)
		}
		for _, msg := range body.Messages {
			if msg.Type == "text" {
				text.WriteString(msg.Content)
			}
		}
	}
	if strings.Count(text.String(), privateKnowledgeTestBody) != 1 {
		t.Fatal("failed evidence was dropped or duplicated")
	}
	if strings.Contains(f.logs.String(), privateKnowledgeTestBody) {
		t.Fatal("failed evidence leaked to generic logs")
	}
}

func TestKnowledgeEmptyAuthorizedBundleKeepsBaseAndLaunchFence(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	f.empty = true
	result := f.run(t)
	if result.Status != "completed" || len(f.callsFor("authorize")) != 1 {
		t.Fatal("empty authorized contribution set skipped lifecycle")
	}
	input, _ := os.ReadFile(f.capture)
	if bytes.Contains(input, []byte(privateKnowledgeTestBody)) || bytes.Contains(input, []byte("Approved knowledge")) {
		t.Fatal("empty resolution fabricated private material")
	}
}

func TestKnowledgeFailuresNeverEnterFreshReplay(t *testing.T) {
	for _, reason := range []string{agent.KnowledgeContextChanged, agent.KnowledgeAuthorizationDenied, agent.KnowledgeUnavailable, "knowledge_busy", agent.KnowledgeInvocationUnsupported} {
		result := agent.Result{Status: "failed", Error: "no conversation found", KnowledgeFailure: reason, ResumeRejected: true}
		if shouldRetryWithFreshSession(result, "old", 0, "claude") || freshSessionMayHelp(reason) {
			t.Fatalf("knowledge reason %s triggered replay", reason)
		}
		first := agent.Result{Status: "failed", Output: "prior work", Usage: map[string]agent.TokenUsage{"m": {InputTokens: 4}}}
		merged, tools := reconcileFreshRetryResult(first, first.Usage, 2, agent.Result{}, 0, knowledgeRefusal(reason))
		if merged.KnowledgeFailure != reason || merged.Output != "" || merged.KnowledgeOutput != "prior work" || merged.Usage["m"].InputTokens != 4 || tools != 2 {
			t.Fatal("later refusal lost earlier observed work")
		}
	}
}

// A synthetic multi-command backend uses the same real Claude leaf twice. This
// exercises executeAndDrain's handling when command two is refused after the
// first command has emitted usage/output and its actual termination callback.
type knowledgeTwoLaunchBackend struct {
	inner      agent.Backend
	afterFirst func()
}

func (b knowledgeTwoLaunchBackend) Execute(ctx context.Context, prompt string, opts agent.ExecOptions) (*agent.Session, error) {
	first, err := b.inner.Execute(ctx, prompt, opts)
	if err != nil {
		return nil, err
	}
	messages := make(chan agent.Message, 32)
	results := make(chan agent.Result, 1)
	go func() {
		defer close(messages)
		defer close(results)
		for msg := range first.Messages {
			messages <- msg
		}
		result := <-first.Result
		b.afterFirst()
		_, err := b.inner.Execute(ctx, prompt, opts)
		result.KnowledgeOutput, result.Output = result.Output, ""
		result.KnowledgeFailure, result.Error, result.Status = agent.KnowledgeFailureReason(err), agent.KnowledgeFailureReason(err), "failed"
		results <- result
	}()
	return &agent.Session{Messages: messages, Result: results}, nil
}

func TestOrdinaryKnowledgeLaterLaunchRefusalDoesNotReplay(t *testing.T) {
	f := newKnowledgeDaemonFixture(t)
	cfg := agent.Config{ExecutablePath: f.bin, BuiltinRuntime: true, Logger: f.d.logger, Env: f.task.Agent.CustomEnv}
	opts := agent.ExecOptions{Cwd: t.TempDir(), Timeout: 5 * time.Second}
	prompt, err := f.d.prepareKnowledgePrompt(context.Background(), f.task, "claude", cfg, &opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := agent.ResolveBackend("claude", cfg)
	if err != nil {
		t.Fatal(err)
	}
	backend := knowledgeTwoLaunchBackend{inner: inner, afterFirst: func() {
		f.mu.Lock()
		f.refuseAction, f.refuseCode = "authorize", agent.KnowledgeContextChanged
		f.mu.Unlock()
	}}
	var sequence atomic.Int32
	result, _, err := f.d.executeAndDrain(context.Background(), backend, prompt, opts, f.d.logger, f.task.ID, "", &sequence)
	if err != nil || result.KnowledgeFailure != agent.KnowledgeContextChanged || result.Usage["test-model"].InputTokens != 12 || result.KnowledgeOutput != privateKnowledgeTestBody {
		t.Fatal("later refusal did not retain failed evidence and usage")
	}
	argv, _ := os.ReadFile(f.argv)
	if strings.Count(string(argv), "PHYSICAL_START") != 1 || len(f.callsFor("authorize")) != 2 {
		t.Fatal("later refusal executed or replayed input")
	}
	if len(f.callsFor("observations")) != 3 {
		t.Fatal("lost original command's process/native/termination observations")
	}
}
