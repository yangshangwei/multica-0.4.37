package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The daemon interrupts a running agent the moment a task-status poll answers
// `404 task not found` (shouldInterruptAgent → isTaskNotFoundError). So that
// body is a kill signal, and only a lookup that completed and found nothing may
// produce it. #2127 established this for GetAgentTask; these tests hold the same
// line for the workspace resolution three lines below it, which #2127 missed and
// GH #8272 hit in production.

// lookupFaultPool fails one named query and passes everything else through, so
// a single link in ResolveTaskWorkspaceIDChecked can be made to time out while
// the task row itself stays readable.
type lookupFaultPool struct {
	db.DBTX
	query  string
	called bool
}

func (f *lookupFaultPool) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "-- name: "+f.query+" :one") {
		f.called = true
		return &mockRow{err: context.DeadlineExceeded}
	}
	return f.DBTX.QueryRow(ctx, query, args...)
}

// TestGetTaskStatus_WorkspaceLookupFailure_Returns500 covers every link kind the
// resolver walks. A timeout on any of them must be a 5xx the daemon retries, not
// a deletion it acts on.
func TestGetTaskStatus_WorkspaceLookupFailure_Returns500(t *testing.T) {
	ctx := context.Background()
	runtimeID := dbfx.Runtime(t, "MUL-7259 lookup runtime")
	agentID := dbfx.Agent(t, "MUL-7259 lookup agent", runtimeID)
	issueID := dbfx.Issue(t, "MUL-7259 lookup issue")
	chatID := dbfx.ChatSession(t, agentID)
	apID := dbfx.Insert(t, "autopilot", testutil.Cols{
		"workspace_id": testWorkspaceID, "title": "MUL-7259 lookup autopilot",
		"assignee_id": agentID, "status": "paused", "created_by_type": "member", "created_by_id": testUserID,
	})
	runID := dbfx.Insert(t, "autopilot_run", testutil.Cols{"autopilot_id": apID, "source": "manual", "status": "running"})

	for _, tc := range []struct{ query, column, id string }{
		{"GetIssue", "issue_id", issueID},
		{"GetChatSession", "chat_session_id", chatID},
		{"GetAutopilotRun", "autopilot_run_id", runID},
		{"GetAutopilot", "autopilot_run_id", runID},
	} {
		t.Run(tc.query, func(t *testing.T) {
			taskID := dbfx.Task(t, agentID, testutil.Cols{
				"runtime_id": runtimeID, "status": "running", "started_at": testutil.Raw("now()"), tc.column: tc.id,
			})
			fault := &lookupFaultPool{DBTX: testPool, query: tc.query}
			h := &Handler{Queries: db.New(testPool), TaskService: &service.TaskService{Queries: db.New(fault)}}
			req := newDaemonTokenRequest(http.MethodGet, "/api/daemon/tasks/"+taskID+"/status", nil, testWorkspaceID, "test-daemon")
			req = withURLParam(req, "taskId", taskID)

			w := testutil.Call(t, h.GetTaskStatus, req).Want(http.StatusInternalServerError)
			if !fault.called {
				t.Fatal("fault was never exercised — the resolver did not reach the injected query")
			}
			// isTaskNotFoundError requires BOTH a 404 and this body, so the
			// 500 above is already enough to keep the daemon from
			// interrupting. The body is asserted too so the response never
			// carries the cancel-triggering string at all, should either half
			// of that check ever be relaxed.
			if strings.Contains(w.Body.String(), "task not found") {
				t.Fatalf("5xx body must not carry the daemon's cancel-triggering string: %s", w.Body.String())
			}

			task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
			if err != nil || task.Status != "running" {
				t.Fatalf("task must be untouched and still running: status=%s err=%v", task.Status, err)
			}
			// Once the dependency recovers the identical request succeeds, so
			// the 5xx really was "ask again", not a masked permanent failure.
			testutil.Call(t, testHandler.GetTaskStatus, req).Want(http.StatusOK)
		})
	}
}

// TestGetTaskStatus_GenuinelyUnreachable_Returns404 is the other half of the
// contract: the 404 must survive for real absence, otherwise a deleted task
// would leave its agent running to its full timeout.
func TestGetTaskStatus_GenuinelyUnreachable_Returns404(t *testing.T) {
	runtimeID := dbfx.Runtime(t, "MUL-7259 absent runtime")
	agentID := dbfx.Agent(t, "MUL-7259 absent agent", runtimeID)

	t.Run("task row missing", func(t *testing.T) {
		missing := uuid.NewString()
		req := newDaemonTokenRequest(http.MethodGet, "/api/daemon/tasks/"+missing+"/status", nil, testWorkspaceID, "test-daemon")
		req = withURLParam(req, "taskId", missing)
		w := testutil.Call(t, testHandler.GetTaskStatus, req).Want(http.StatusNotFound)
		if !strings.Contains(w.Body.String(), "task not found") {
			t.Fatalf("a genuinely missing task must still interrupt the daemon: %s", w.Body.String())
		}
	})

	t.Run("link target missing", func(t *testing.T) {
		// agent_task_queue.issue_id is ON DELETE CASCADE, so an issue task can
		// never outlive its issue — that absence surfaces as ErrNoRows on the
		// task itself (covered above). chat_session_id is ON DELETE SET NULL,
		// so this is the shape where the resolver genuinely has nothing left to
		// resolve, and it must stay a 404 rather than becoming a 5xx the daemon
		// retries forever.
		chatID := dbfx.ChatSession(t, agentID)
		taskID := dbfx.Task(t, agentID, testutil.Cols{
			"runtime_id": runtimeID, "status": "running",
			"started_at": testutil.Raw("now()"), "chat_session_id": chatID,
		})
		dbfx.Exec(t, "DELETE FROM chat_session WHERE id = $1", chatID)

		req := newDaemonTokenRequest(http.MethodGet, "/api/daemon/tasks/"+taskID+"/status", nil, testWorkspaceID, "test-daemon")
		req = withURLParam(req, "taskId", taskID)
		w := testutil.Call(t, testHandler.GetTaskStatus, req).Want(http.StatusNotFound)
		if !strings.Contains(w.Body.String(), "task not found") {
			t.Fatalf("a task whose only link is gone is unreachable: %s", w.Body.String())
		}
	})
}

// TestGetTaskStatus_ForeignWorkspace_Returns404 pins the permission boundary:
// splitting lookup failures out of the 404 must not turn a cross-workspace task
// into a distinguishable response. A foreign task and a missing one look alike.
func TestGetTaskStatus_ForeignWorkspace_Returns404(t *testing.T) {
	runtimeID := dbfx.Runtime(t, "MUL-7259 foreign runtime")
	agentID := dbfx.Agent(t, "MUL-7259 foreign agent", runtimeID)
	issueID := dbfx.Issue(t, "MUL-7259 foreign issue")
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID,
		"status": "running", "started_at": testutil.Raw("now()"),
	})

	otherWorkspace := uuid.NewString()
	req := newDaemonTokenRequest(http.MethodGet, "/api/daemon/tasks/"+taskID+"/status", nil, otherWorkspace, "other-daemon")
	req = withURLParam(req, "taskId", taskID)
	testutil.Call(t, testHandler.GetTaskStatus, req).Want(http.StatusNotFound)
}

func TestListTaskMessagesByUser_WorkspaceLookupFailureIsRetryable(t *testing.T) {
	runtimeID := dbfx.Runtime(t, "Message lookup runtime")
	agentID := dbfx.Agent(t, "Message lookup agent", runtimeID)
	issueID := dbfx.Issue(t, "Message lookup issue")
	taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
	const content = "The agent is still working."
	dbfx.Insert(t, "task_message", testutil.Cols{
		"task_id": taskID, "seq": 1, "type": "text", "content": content,
	})
	fault := &lookupFaultPool{DBTX: testPool, query: "GetIssue"}
	h := &Handler{Queries: db.New(testPool), TaskService: &service.TaskService{Queries: db.New(fault)}}
	req := testutil.WithHeaders(
		testutil.JSONRequest(http.MethodGet, "/api/tasks/"+taskID+"/messages", nil),
		"X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID,
	)
	req = withURLParam(req, "taskId", taskID)
	wrapped := middleware.RequireWorkspaceMember(h.Queries)(http.HandlerFunc(h.ListTaskMessagesByUser)).ServeHTTP
	w := testutil.Call(t, wrapped, req).Want(http.StatusInternalServerError)
	if !fault.called {
		t.Fatal("workspace lookup fault was not exercised")
	}
	if strings.Contains(w.Body.String(), "task not found") || strings.Contains(w.Body.String(), content) {
		t.Fatalf("lookup failure must neither report absence nor expose messages: %s", w.Body.String())
	}

	// A retry after the lookup recovers must return the original transcript.
	var messages []protocol.TaskMessagePayload
	wrapped = middleware.RequireWorkspaceMember(testHandler.Queries)(http.HandlerFunc(testHandler.ListTaskMessagesByUser)).ServeHTTP
	testutil.Call(t, wrapped, req).Want(http.StatusOK).JSON(&messages)
	if len(messages) != 1 || messages[0].TaskID != taskID || messages[0].Content != content {
		t.Fatalf("recovered transcript = %+v, want the task's unchanged message", messages)
	}
}

func TestListTaskMessagesByUser_ForeignWorkspaceMatchesMissingTask(t *testing.T) {
	runtimeID := dbfx.Runtime(t, "Private transcript runtime")
	agentID := dbfx.Agent(t, "Private transcript agent", runtimeID)
	issueID := dbfx.Issue(t, "Private transcript issue")
	taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
	dbfx.Insert(t, "task_message", testutil.Cols{
		"task_id": taskID, "seq": 1, "type": "text", "content": "Private transcript",
	})
	otherWorkspace := dbfx.Workspace(t, "Other transcript workspace", "transcript-other-"+uuid.NewString())
	dbfx.Member(t, otherWorkspace, testUserID, "owner")
	wrapped := middleware.RequireWorkspaceMember(testHandler.Queries)(http.HandlerFunc(testHandler.ListTaskMessagesByUser)).ServeHTTP

	var missingBody string
	for _, id := range []string{uuid.NewString(), taskID} {
		req := testutil.WithHeaders(
			testutil.JSONRequest(http.MethodGet, "/api/tasks/"+id+"/messages", nil),
			"X-User-ID", testUserID, "X-Workspace-ID", otherWorkspace,
		)
		req = withURLParam(req, "taskId", id)
		w := testutil.Call(t, wrapped, req).Want(http.StatusNotFound)
		if id != taskID {
			missingBody = w.Body.String()
		} else if w.Body.String() != missingBody {
			t.Fatalf("foreign task response = %q, want the same response as a missing task %q", w.Body.String(), missingBody)
		}
	}
}

// TestResolveTaskWorkspaceIDChecked_SeparatesAbsenceFromFailure asserts the
// distinction at the service boundary, where the two callers now rely on it.
func TestResolveTaskWorkspaceIDChecked_SeparatesAbsenceFromFailure(t *testing.T) {
	ctx := context.Background()
	runtimeID := dbfx.Runtime(t, "MUL-7259 resolver runtime")
	agentID := dbfx.Agent(t, "MUL-7259 resolver agent", runtimeID)
	issueID := dbfx.Issue(t, "MUL-7259 resolver issue")

	t.Run("resolves", func(t *testing.T) {
		taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
		task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
		if err != nil {
			t.Fatal(err)
		}
		got, err := testHandler.TaskService.ResolveTaskWorkspaceIDChecked(ctx, task)
		if err != nil || got != testWorkspaceID {
			t.Fatalf("workspace=%q err=%v, want %q / nil", got, err, testWorkspaceID)
		}
	})

	t.Run("absent link is not an error", func(t *testing.T) {
		chatID := dbfx.ChatSession(t, agentID)
		taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "chat_session_id": chatID})
		dbfx.Exec(t, "DELETE FROM chat_session WHERE id = $1", chatID)
		task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
		if err != nil {
			t.Fatal(err)
		}
		got, err := testHandler.TaskService.ResolveTaskWorkspaceIDChecked(ctx, task)
		if got != "" || err != nil {
			t.Fatalf("workspace=%q err=%v, want \"\" / nil", got, err)
		}
	})

	t.Run("failed lookup is an error", func(t *testing.T) {
		taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
		task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
		if err != nil {
			t.Fatal(err)
		}
		svc := &service.TaskService{Queries: db.New(&lookupFaultPool{DBTX: testPool, query: "GetIssue"})}
		got, err := svc.ResolveTaskWorkspaceIDChecked(ctx, task)
		if got != "" || err == nil {
			t.Fatalf("workspace=%q err=%v, want \"\" / error", got, err)
		}
		// The best-effort wrapper keeps flattening both cases to "" — broadcast
		// callers depend on that and must not start panicking on a blip.
		if ws := svc.ResolveTaskWorkspaceID(ctx, task); ws != "" {
			t.Fatalf("best-effort resolver should still return \"\", got %q", ws)
		}
	})

	t.Run("later association resolves despite an earlier query failure", func(t *testing.T) {
		chatID := dbfx.ChatSession(t, agentID)
		taskID := dbfx.Task(t, agentID, testutil.Cols{
			"runtime_id": runtimeID, "issue_id": issueID, "chat_session_id": chatID,
		})
		task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
		if err != nil {
			t.Fatal(err)
		}
		fault := &lookupFaultPool{DBTX: testPool, query: "GetIssue"}
		svc := &service.TaskService{Queries: db.New(fault)}
		got, err := svc.ResolveTaskWorkspaceIDChecked(ctx, task)
		if !fault.called {
			t.Fatal("earlier issue lookup fault was not exercised")
		}
		if got != testWorkspaceID || err != nil {
			t.Fatalf("workspace=%q err=%v, want the chat's workspace %q / nil", got, err, testWorkspaceID)
		}
	})

	t.Run("quick-create resolves workspace from context JSONB", func(t *testing.T) {
		payload, err := json.Marshal(service.QuickCreateContext{
			Type: service.QuickCreateContextType, WorkspaceID: testWorkspaceID,
			RequesterID: testUserID, Prompt: "Create a task from this request",
		})
		if err != nil {
			t.Fatal(err)
		}
		taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "context": payload})
		task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
		if err != nil {
			t.Fatal(err)
		}
		if task.IssueID.Valid || task.ChatSessionID.Valid || task.AutopilotRunID.Valid {
			t.Fatal("quick-create fixture unexpectedly has a linked entity")
		}
		got, err := testHandler.TaskService.ResolveTaskWorkspaceIDChecked(ctx, task)
		if got != testWorkspaceID || err != nil {
			t.Fatalf("workspace=%q err=%v, want context workspace %q / nil", got, err, testWorkspaceID)
		}
	})
}
