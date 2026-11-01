package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type taskKnowledgeHTTPFixture struct {
	runtimeID, agentID, daemonID, token, taskID, issueID, chatID string
	proposal                                                     protocol.CollaborationKnowledgeProposal
	inv                                                          protocol.TaskKnowledgeInvocation
}

func newTaskKnowledgeHTTPFixture(t *testing.T, surface string) *taskKnowledgeHTTPFixture {
	t.Helper()
	if testPool == nil || testHandler == nil {
		t.Fatal("explicit C2 database is required")
	}
	prior := testHandler.TaskService.KnowledgeContextEnabled
	testHandler.TaskService.KnowledgeContextEnabled = true
	t.Cleanup(func() { testHandler.TaskService.KnowledgeContextEnabled = prior })
	source, p := knowledgeHTTPProposal(t)
	p.Applicability.Surfaces = []string{"issue", "chat", "quick_create", "autopilot"}
	ctx := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
	if _, err := testHandler.knowledgeService().ProposeKnowledge(ctx, parseUUID(source), p); err != nil {
		t.Fatal(err)
	}
	if _, err := testHandler.knowledgeService().ApproveKnowledge(ctx, parseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		row, err := testHandler.Queries.GetKnowledgeCandidate(ctx, parseUUID(p.CandidateID))
		if err == nil && row.State == "approved" {
			_, err = testHandler.knowledgeService().WithdrawKnowledge(ctx, row.ID, protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: strconv.FormatInt(row.StateRevision, 10), Reason: "test fixture finished"})
			if err != nil {
				t.Errorf("withdraw retained fixture candidate: %v", err)
			}
		}
	})
	f := &taskKnowledgeHTTPFixture{daemonID: "knowledge-http-" + uuid.NewString(), token: "mdt_" + uuid.NewString(), proposal: p, inv: protocol.TaskKnowledgeInvocation{ProtocolVersion: 1, Provider: "claude", Mode: "local", Builtin: true, ConfigurationID: "http-config"}}
	f.runtimeID = dbfx.Runtime(t, "Knowledge HTTP runtime", testutil.Cols{"runtime_mode": "local", "provider": "claude", "daemon_id": f.daemonID})
	f.agentID = dbfx.Agent(t, "Knowledge HTTP agent", f.runtimeID, testutil.Cols{"runtime_mode": "local", "max_concurrent_tasks": 3})
	dbfx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(f.token), "workspace_id": testWorkspaceID, "daemon_id": f.daemonID, "user_id": testUserID, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	cols := testutil.Cols{"runtime_id": f.runtimeID, "originator_user_id": testUserID, "accountable_user_id": testUserID}
	switch surface {
	case "issue", "comment":
		f.issueID = dbfx.Issue(t, "Knowledge consumer", testutil.Cols{"assignee_type": "agent", "assignee_id": f.agentID})
		cols["issue_id"] = f.issueID
		if surface == "comment" {
			cols["trigger_comment_id"] = dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": f.issueID, "author_type": "member", "author_id": testUserID, "content": "Please continue this ordinary task", "type": "comment"})
		}
	case "chat":
		f.chatID = dbfx.ChatSession(t, f.agentID, testutil.Cols{"runtime_id": f.runtimeID})
		cols["chat_session_id"] = f.chatID
	case "quick_create":
		raw, err := json.Marshal(service.QuickCreateContext{Type: service.QuickCreateContextType, WorkspaceID: testWorkspaceID, RequesterID: testUserID, Prompt: "Create an ordinary task"})
		if err != nil {
			t.Fatal(err)
		}
		cols["context"] = raw
	case "autopilot":
		id := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "title": "Knowledge autopilot", "assignee_id": f.agentID, "execution_mode": "run_only", "created_by_type": "member", "created_by_id": testUserID})
		cols["autopilot_run_id"] = dbfx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": id, "source": "manual", "status": "running"})
	default:
		t.Fatal("unknown HTTP fixture surface")
	}
	f.taskID = dbfx.Task(t, f.agentID, cols)
	if surface == "chat" {
		dbfx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=id WHERE id=$1`, f.taskID)
		dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": f.chatID, "role": "user", "content": "Continue the private conversation", "task_id": f.taskID})
	}
	return f
}

func (f *taskKnowledgeHTTPFixture) call(t *testing.T, fn http.HandlerFunc, path string, body any, params ...string) *testutil.Response {
	t.Helper()
	req := testutil.WithURLParams(testutil.JSONRequest("POST", path, body), params...)
	req.Header.Set("Authorization", "Bearer "+f.token)
	wrapped := middleware.DaemonAuth(testHandler.Queries, nil, nil, nil)(fn)
	return testutil.Call(t, wrapped.ServeHTTP, req)
}

func (f *taskKnowledgeHTTPFixture) claim(t *testing.T, transport string, inv *protocol.TaskKnowledgeInvocation) AgentTaskResponse {
	t.Helper()
	var task AgentTaskResponse
	var raw []byte
	if transport == "single" {
		var body struct {
			Task AgentTaskResponse `json:"task"`
		}
		response := f.call(t, testHandler.ClaimTaskByRuntime, "/api/daemon/runtimes/"+f.runtimeID+"/tasks/claim", map[string]any{"knowledge_context": inv}, "runtimeId", f.runtimeID).Want(200)
		response.JSON(&body)
		task = body.Task
		raw = response.Body.Bytes()
	} else {
		body := map[string]any{"daemon_id": f.daemonID, "runtime_ids": []string{f.runtimeID}, "max_tasks": 1, "knowledge_context": map[string]*protocol.TaskKnowledgeInvocation{f.runtimeID: inv}}
		var response struct {
			Tasks []AgentTaskResponse `json:"tasks"`
		}
		if transport == "ws" {
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			status, reply, err := testHandler.rpcClaimTasks(t.Context(), daemonws.ClientIdentity{DaemonID: f.daemonID, UserID: testUserID, WorkspaceID: testWorkspaceID, RuntimeIDs: []string{f.runtimeID}}, encoded)
			if err != nil || status != 200 {
				t.Fatalf("actual WS claim bridge: status=%d err=%v body=%s", status, err, reply)
			}
			if json.Unmarshal(reply, &response) != nil {
				t.Fatal("invalid WS claim response")
			}
			raw = reply
		} else {
			r := f.call(t, testHandler.ClaimTasksByRuntime, "/api/daemon/tasks/claim", body).Want(200)
			r.JSON(&response)
			raw = r.Body.Bytes()
		}
		if len(response.Tasks) != 1 {
			t.Fatalf("expected one finalized claim, got %d: %s", len(response.Tasks), raw)
		}
		task = response.Tasks[0]
	}
	if task.ID != f.taskID {
		t.Fatal("claim selected a different task")
	}
	if strings.Contains(string(raw), f.proposal.Content) || strings.Contains(string(raw), f.proposal.Summary) || strings.Contains(string(raw), f.proposal.CandidateID) {
		t.Fatal("claim disclosed private knowledge")
	}
	return task
}

func (f *taskKnowledgeHTTPFixture) prepare(t *testing.T, d *protocol.TaskKnowledgeClaimDescriptor) protocol.TaskKnowledgePrepareResponse {
	t.Helper()
	if d == nil {
		t.Fatal("negotiation was not finalized")
	}
	req := protocol.TaskKnowledgePrepareRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: strings.Repeat("a", 64)}
	response := f.call(t, testHandler.PrepareTaskKnowledge, "/api/daemon/tasks/"+f.taskID+"/knowledge-context/prepare", req, "taskId", f.taskID).Want(200)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private preparation is cacheable")
	}
	var out protocol.TaskKnowledgePrepareResponse
	response.JSON(&out)
	return out
}

func TestKnowledgeAllFirstPartyClaimSurfaces(t *testing.T) {
	for _, surface := range []string{"issue", "comment", "chat", "quick_create", "autopilot"} {
		for _, transport := range []string{"single", "batch", "ws"} {
			t.Run(surface+"/"+transport, func(t *testing.T) {
				f := newTaskKnowledgeHTTPFixture(t, surface)
				task := f.claim(t, transport, &f.inv)
				out := f.prepare(t, task.KnowledgeContext)
				found := false
				for _, c := range out.Contributions {
					if c.CandidateID == f.proposal.CandidateID && c.Content == f.proposal.Content {
						found = true
					}
				}
				if !found {
					t.Fatalf("eligible current contribution missing for %s/%s", surface, transport)
				}
			})
		}
	}
}

func TestKnowledgeHTTPRejectsHumanCredentialFallback(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "issue")
	for _, fn := range []http.HandlerFunc{testHandler.PrepareTaskKnowledge, testHandler.AuthorizeTaskKnowledgeLaunch, testHandler.ObserveTaskKnowledgeLaunch} {
		r := testutil.WithURLParams(knowledgeHTTPRequest("POST", "/knowledge", testUserID, map[string]any{}), "taskId", f.taskID)
		response := testutil.Call(t, fn, r).Want(403)
		if !strings.Contains(response.Body.String(), "knowledge_authorization_denied") || strings.Contains(response.Body.String(), "PRIVATE_HTTP") {
			t.Fatal("human fallback leaked source or bypassed machine boundary")
		}
	}
}

func TestKnowledgeHTTPNegotiationDoesNotEnableClosedGate(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "issue")
	testHandler.TaskService.KnowledgeContextEnabled = false
	task := f.claim(t, "single", &f.inv)
	if task.KnowledgeContext != nil {
		t.Fatal("claim enabled a closed server gate")
	}
}

// Named regression for H1: a closed gate must not fail an ordinary claim whose
// daemon defaults or custom_args sit outside the negotiation allowlist. The
// rule's matrix lives in the service task_knowledge_invocation suite.
func TestKnowledgeClaimGateOffSafeArgsDispatchesOrdinaryTask(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "issue")
	testHandler.TaskService.KnowledgeContextEnabled = false
	dbfx.Exec(t, `UPDATE agent SET custom_args='["--allowedTools","Bash"]'::jsonb WHERE id=$1`, f.agentID)
	inv := f.inv
	inv.ExtraArgs = []string{"--add-dir", "/tmp/x"}
	task := f.claim(t, "single", &inv)
	if task.KnowledgeContext != nil {
		t.Fatal("closed gate negotiated knowledge")
	}
	var status string
	var failed bool
	dbfx.QueryRow(t, `SELECT status, failure_reason IS NOT NULL FROM agent_task_queue WHERE id=$1`, f.taskID).Scan(&status, &failed)
	if status != "dispatched" || failed {
		t.Fatalf("ordinary claim was not dispatched cleanly: status=%s failure_reason_set=%t", status, failed)
	}
}

func (f *taskKnowledgeHTTPFixture) recordNative(t *testing.T, native string) *protocol.TaskKnowledgeClaimDescriptor {
	t.Helper()
	task := f.claim(t, "single", &f.inv)
	d := task.KnowledgeContext
	fence := &protocol.TaskKnowledgeFence{ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt}
	f.call(t, testHandler.StartTask, "/start", map[string]any{"knowledge_fence": fence}, "taskId", f.taskID).Want(200)
	p := f.prepare(t, d)
	req := protocol.TaskKnowledgeAuthorizeRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: p.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, Session: d.Session, InvocationFingerprint: p.InvocationFingerprint, ManifestSHA256: p.ManifestSHA256, PromptSHA256: strings.Repeat("b", 64), AttemptSequence: 1, AttemptNonce: uuid.NewString()}
	var authorization protocol.TaskKnowledgeAuthorization
	f.call(t, testHandler.AuthorizeTaskKnowledgeLaunch, "/authorize", req, "taskId", f.taskID).Want(200).JSON(&authorization)
	for _, kind := range []string{"started", "native_session", "terminated"} {
		observation := protocol.TaskKnowledgeObservation{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: p.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, AttemptSequence: 1, AttemptNonce: req.AttemptNonce, Kind: kind}
		switch kind {
		case "started":
			observation.ProcessID = 42
			observation.ProcessInstanceID = "fixture-process"
		case "native_session":
			observation.NativeSessionID = native
			observation.Source = "system"
		case "terminated":
			observation.TerminationKind = "managed_launch_ended"
			observation.WaitObserved = true
		}
		f.call(t, testHandler.ObserveTaskKnowledgeLaunch, "/observations", observation, "taskId", f.taskID).Want(200)
	}
	f.call(t, testHandler.CompleteTask, "/complete", TaskCompleteRequest{Output: "Completed fixture", SessionID: native, WorkDir: "/tmp/knowledge-server-fixture", KnowledgeFence: fence}, "taskId", f.taskID).Want(200)
	return d
}

func (f *taskKnowledgeHTTPFixture) nextTask(t *testing.T) {
	t.Helper()
	cols := testutil.Cols{"runtime_id": f.runtimeID, "originator_user_id": testUserID, "accountable_user_id": testUserID}
	if f.issueID != "" {
		cols["issue_id"] = f.issueID
	}
	if f.chatID != "" {
		cols["chat_session_id"] = f.chatID
	}
	f.taskID = dbfx.Task(t, f.agentID, cols)
	if f.chatID != "" {
		dbfx.Exec(t, `UPDATE agent_task_queue SET chat_input_task_id=id WHERE id=$1`, f.taskID)
		dbfx.Insert(t, "chat_message", testutil.Cols{"chat_session_id": f.chatID, "role": "user", "content": "Next ordinary input", "task_id": f.taskID})
	}
}

func TestKnowledgeResumeDirectChatPointerUsesExactExposure(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "chat")
	f.recordNative(t, "direct-native")
	dbfx.Exec(t, `UPDATE agent_task_queue SET session_id=NULL WHERE id=$1`, f.taskID)
	f.nextTask(t)
	response := f.call(t, testHandler.ClaimTaskByRuntime, "/claim", nil, "runtimeId", f.runtimeID).Want(409)
	if !strings.Contains(response.Text(), "knowledge_invocation_unsupported") || strings.Contains(response.Text(), f.proposal.Content) {
		t.Fatal("direct chat pointer bypassed retained exposure")
	}
	row, err := testHandler.Queries.GetAgentTask(t.Context(), parseUUID(f.taskID))
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "failed" || row.FailureReason.String != "knowledge_invocation_unsupported" {
		t.Fatal("unsupported claim did not settle without replay")
	}
}

func TestKnowledgeResumeOlderSessionFallbackUsesChosenKey(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "issue")
	old := f.recordNative(t, "older-native")
	dbfx.Task(t, f.agentID, testutil.Cols{"runtime_id": f.runtimeID, "issue_id": f.issueID, "status": "failed", "session_id": "newer-poisoned-native", "failure_reason": "api_invalid_request", "completed_at": testutil.Raw("now()"), "work_dir": "/tmp/newer-poisoned"})
	f.nextTask(t)
	chosen, err := testHandler.Queries.GetLastTaskSession(t.Context(), db.GetLastTaskSessionParams{AgentID: parseUUID(f.agentID), IssueID: parseUUID(f.issueID)})
	if err != nil || chosen.SessionID.String != "older-native" {
		t.Fatalf("fixture did not select older actual session: %+v %v", chosen, err)
	}
	ctx := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
	if _, err = testHandler.knowledgeService().WithdrawKnowledge(ctx, parseUUID(f.proposal.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "withdraw old exposure"}); err != nil {
		t.Fatal(err)
	}
	claimed := f.claim(t, "single", &f.inv)
	if claimed.PriorSessionID != "" || claimed.KnowledgeContext == nil || claimed.KnowledgeContext.Session.Generation == old.Session.Generation {
		t.Fatal("latest task hid the actual older session's invalid exposure")
	}
}

func TestKnowledgeExplicitRerunUsesSourceSessionExposure(t *testing.T) {
	f := newTaskKnowledgeHTTPFixture(t, "issue")
	f.recordNative(t, "rerun-source-native")
	sourceTask := f.taskID
	f.nextTask(t)
	dbfx.Exec(t, `UPDATE agent_task_queue SET rerun_of_task_id=$2,force_fresh_session=true WHERE id=$1`, f.taskID, sourceTask)
	ctx := knowledgeHTTPRequest("GET", "/", testUserID, nil).Context()
	if _, err := testHandler.knowledgeService().WithdrawKnowledge(ctx, parseUUID(f.proposal.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "withdraw rerun exposure"}); err != nil {
		t.Fatal(err)
	}
	claimed := f.claim(t, "single", &f.inv)
	if claimed.PriorSessionID != "" || claimed.KnowledgeContext.Session.NativeSessionID != "" {
		t.Fatal("explicit source-session rerun bypassed knowledge revalidation")
	}
}

// taskClaimRow is the queue state the claim error tests settle on.
type taskClaimRow struct {
	status, failureReason string
	dispatched            bool
}

func loadTaskClaimRow(t *testing.T, taskID string) taskClaimRow {
	t.Helper()
	var row taskClaimRow
	dbfx.QueryRow(t, `SELECT status, COALESCE(failure_reason, ''), dispatched_at IS NOT NULL FROM agent_task_queue WHERE id=$1`, taskID).Scan(&row.status, &row.failureReason, &row.dispatched)
	return row
}

// M1: a transient read while building the claim's knowledge decision is an
// infrastructure failure, not a knowledge refusal. It must return that exact
// claim to the queue instead of settling the task as knowledge_unavailable.
// The fault swaps the shared testHandler.TaskService.Queries, so this test
// must never call t.Parallel.
func TestKnowledgeClaimTransientHistoryErrorRequeuesTask(t *testing.T) {
	for _, transport := range []string{"single", "batch"} {
		for _, provider := range []string{"codex", "claude"} {
			t.Run(transport+"/"+provider, func(t *testing.T) {
				f := newTaskKnowledgeHTTPFixture(t, "issue")
				testHandler.TaskService.KnowledgeContextEnabled = false
				if provider != "claude" {
					dbfx.Exec(t, `UPDATE agent_runtime SET provider=$2 WHERE id=$1`, f.runtimeID, provider)
				}
				saved := testHandler.TaskService.Queries
				t.Cleanup(func() { testHandler.TaskService.Queries = saved })
				testHandler.TaskService.Queries = db.New(failQueryDBTX{DBTX: testPool, failOn: "-- name: ListTaskKnowledgeRuntimeHistory", err: &pgconn.PgError{Code: "57P01", Message: "terminating connection due to administrator command"}})

				if transport == "single" {
					response := f.call(t, testHandler.ClaimTaskByRuntime, "/api/daemon/runtimes/"+f.runtimeID+"/tasks/claim", map[string]any{"knowledge_context": nil}, "runtimeId", f.runtimeID)
					if response.Code != http.StatusInternalServerError || strings.Contains(response.Text(), "knowledge_") {
						t.Errorf("transient history read answered %d %q, want a 500 that names no knowledge refusal", response.Code, response.Text())
					}
				} else {
					var out struct {
						Tasks []AgentTaskResponse `json:"tasks"`
					}
					body := map[string]any{"daemon_id": f.daemonID, "runtime_ids": []string{f.runtimeID}, "max_tasks": 1}
					f.call(t, testHandler.ClaimTasksByRuntime, "/api/daemon/tasks/claim", body).Want(200).JSON(&out)
					if len(out.Tasks) != 0 {
						t.Fatalf("batch claim delivered %d tasks through a failed knowledge read", len(out.Tasks))
					}
				}
				if row := loadTaskClaimRow(t, f.taskID); row != (taskClaimRow{status: "queued"}) {
					t.Fatalf("transient knowledge history read settled the task: %+v, want queued with no dispatch and no failure reason", row)
				}

				testHandler.TaskService.Queries = saved
				task := f.claim(t, "single", nil)
				if task.KnowledgeContext != nil {
					t.Fatal("ordinary reclaim negotiated knowledge with the gate off")
				}
			})
		}
	}
}

// A task that already holds 32 negotiated claims (maxTaskKnowledgeClaims; in
// production, stale reclaims of lost claim responses add them) makes every
// later finalizeTaskKnowledgeClaim refuse with knowledge_context_changed.
// Claim building never reads the claim count, so a requeue on that refusal
// rebuilds the same refused claim on every poll. A typed knowledge refusal
// from finalization must settle the task, as the build-time refusals do.
func TestKnowledgeClaimFinalizeRefusalAtClaimBoundSettlesTask(t *testing.T) {
	for _, transport := range []string{"single", "batch"} {
		t.Run(transport, func(t *testing.T) {
			f := newTaskKnowledgeHTTPFixture(t, "issue")
			for i := 0; i < 32; i++ {
				f.claim(t, "single", &f.inv)
				row, err := testHandler.Queries.GetAgentTask(t.Context(), parseUUID(f.taskID))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := testHandler.TaskService.RequeueTaskAfterClaimFailure(t.Context(), row); err != nil {
					t.Fatal(err)
				}
			}
			claimCount := func() int {
				var n int
				dbfx.QueryRow(t, `SELECT jsonb_array_length(knowledge_delivery->'claims') FROM agent_task_queue WHERE id=$1`, f.taskID).Scan(&n)
				return n
			}
			if n := claimCount(); n != 32 {
				t.Fatalf("fixture recorded %d claims, want the 32-claim bound", n)
			}

			claimPath := "/api/daemon/runtimes/" + f.runtimeID + "/tasks/claim"
			if transport == "single" {
				response := f.call(t, testHandler.ClaimTaskByRuntime, claimPath, map[string]any{"knowledge_context": &f.inv}, "runtimeId", f.runtimeID)
				if response.Code != http.StatusConflict || !strings.Contains(response.Text(), "knowledge_context_changed") {
					t.Errorf("claim past the bound answered %d %q, want 409 knowledge_context_changed", response.Code, response.Text())
				}
			} else {
				var out struct {
					Tasks []AgentTaskResponse `json:"tasks"`
				}
				body := map[string]any{"daemon_id": f.daemonID, "runtime_ids": []string{f.runtimeID}, "max_tasks": 1, "knowledge_context": map[string]*protocol.TaskKnowledgeInvocation{f.runtimeID: &f.inv}}
				f.call(t, testHandler.ClaimTasksByRuntime, "/api/daemon/tasks/claim", body).Want(200).JSON(&out)
				if len(out.Tasks) != 0 {
					t.Fatalf("batch claim delivered %d tasks past the claim bound", len(out.Tasks))
				}
			}
			if row := loadTaskClaimRow(t, f.taskID); row.status != "failed" || row.failureReason != "knowledge_context_changed" {
				t.Errorf("finalize refusal at the claim bound left the task %+v, want failed with knowledge_context_changed", row)
			}
			if n := claimCount(); n != 32 {
				t.Errorf("refused finalization changed the recorded claims to %d", n)
			}

			probe := f.call(t, testHandler.ClaimTaskByRuntime, claimPath, map[string]any{"knowledge_context": &f.inv}, "runtimeId", f.runtimeID)
			var next struct {
				Task *AgentTaskResponse `json:"task"`
			}
			if probe.Code != http.StatusOK {
				t.Fatalf("the refused task was claimed again: %d %q", probe.Code, probe.Text())
			}
			probe.JSON(&next)
			if next.Task != nil {
				t.Fatalf("the refused task was delivered again: %s", next.Task.ID)
			}
		})
	}
}

// Canonical classification for M1. The DB-backed requeue wiring is pinned by
// TestKnowledgeClaimTransientHistoryErrorRequeuesTask.
func TestKnowledgeClaimErrorDisposition(t *testing.T) {
	for _, tc := range []struct {
		name        string
		err         error
		wantCode    string
		wantRequeue bool
	}{
		{"typed refusal", &service.CollaborationError{Status: http.StatusConflict, Code: "knowledge_context_changed"}, "knowledge_context_changed", false},
		{"wrapped typed refusal", fmt.Errorf("claim: %w", &service.CollaborationError{Status: http.StatusConflict, Code: "knowledge_invocation_unsupported"}), "knowledge_invocation_unsupported", false},
		{"missing row", fmt.Errorf("x: %w", pgx.ErrNoRows), "knowledge_unavailable", false},
		{"postgres shutdown", &pgconn.PgError{Code: "57P01"}, "", true},
		{"deadline", context.DeadlineExceeded, "", true},
		{"cancelled", context.Canceled, "", true},
		{"connection reset", errors.New("connection reset"), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, requeue := knowledgeClaimErrorDisposition(tc.err)
			if code != tc.wantCode || requeue != tc.wantRequeue {
				t.Fatalf("knowledgeClaimErrorDisposition(%v) = (%q, %t), want (%q, %t)", tc.err, code, requeue, tc.wantCode, tc.wantRequeue)
			}
		})
	}
}
