package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/agent"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TestQuickCreateRefusesAgentsThatCannotCreateIssues pins the pre-enqueue
// autonomy gate. A quick-create run's only job is `multica issue create`, which
// CreateIssue refuses below Contributor; queueing it anyway turned the refusal
// into an inbox failure one agent turn later. The gate answers in the modal
// instead, and must not touch agents with no declared level.
func TestQuickCreateRefusesAgentsThatCannotCreateIssues(t *testing.T) {
	if testHandler == nil || dbfx == nil {
		t.Skip("database not available")
	}
	metadata, err := json.Marshal(map[string]string{"cli_version": agent.MinQuickCreateFieldsCLIVersion})
	if err != nil {
		t.Fatal(err)
	}
	runtimeID := dbfx.Runtime(t, "Quick-create autonomy runtime", testutil.Cols{"metadata": string(metadata)})
	agentAt := func(name, level string) string {
		return dbfx.Agent(t, name, runtimeID, testutil.Cols{"autonomy_level": level})
	}
	observerID := agentAt("Quick-create observer", "observer")
	contributorID := agentAt("Quick-create contributor", "contributor")
	undeclaredID := agentAt("Quick-create undeclared", "")

	countTasks := func(t *testing.T, agentID string) int {
		t.Helper()
		var n int
		if err := testPool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM agent_task_queue WHERE agent_id = $1 AND context->>'type' = 'quick_create'`,
			agentID,
		).Scan(&n); err != nil {
			t.Fatalf("count quick-create tasks: %v", err)
		}
		return n
	}
	quickCreate := func(t *testing.T, body map[string]any) *testutil.Response {
		t.Helper()
		body["prompt"] = "Investigate why the ES query takes three minutes"
		return testutil.Call(t, testHandler.QuickCreateIssue, newRequest(http.MethodPost, "/api/issues/quick-create", body))
	}
	wantRefusal := func(t *testing.T, res *testutil.Response) {
		t.Helper()
		var body struct {
			Code             string `json:"code"`
			AutonomyLevel    string `json:"autonomy_level"`
			RequiredAutonomy string `json:"required_autonomy"`
			Error            string `json:"error"`
		}
		res.Want(http.StatusUnprocessableEntity).JSON(&body)
		if body.Code != "agent_autonomy_insufficient" || body.AutonomyLevel != "observer" || body.RequiredAutonomy != "contributor" {
			t.Fatalf("unexpected refusal body: %+v", body)
		}
		// Older clients show `error` verbatim, so it has to name the fix.
		if body.Error == "" {
			t.Fatal("refusal must carry a readable error sentence for older clients")
		}
	}

	t.Run("observer agent is refused before enqueue", func(t *testing.T) {
		before := countTasks(t, observerID)
		wantRefusal(t, quickCreate(t, map[string]any{"agent_id": observerID}))
		if got := countTasks(t, observerID); got != before {
			t.Fatalf("observer quick-create must not enqueue: %d -> %d", before, got)
		}
	})

	t.Run("squad led by an observer is refused", func(t *testing.T) {
		squadID := dbfx.Squad(t, "Observer-led squad", observerID)
		before := countTasks(t, observerID)
		wantRefusal(t, quickCreate(t, map[string]any{"squad_id": squadID}))
		if got := countTasks(t, observerID); got != before {
			t.Fatalf("observer-led squad quick-create must not enqueue: %d -> %d", before, got)
		}
	})

	// The level is the permanent cause; an offline runtime is not. Reporting
	// "offline" first would send the user to fix a machine and then fail again.
	t.Run("autonomy is reported ahead of an offline runtime", func(t *testing.T) {
		offlineRuntime := dbfx.Runtime(t, "Quick-create offline runtime", testutil.Cols{
			"metadata": string(metadata), "status": "offline",
		})
		offlineObserver := dbfx.Agent(t, "Offline observer", offlineRuntime, testutil.Cols{"autonomy_level": "observer"})
		wantRefusal(t, quickCreate(t, map[string]any{"agent_id": offlineObserver}))
	})

	for _, tc := range []struct {
		name    string
		agentID string
	}{
		{"contributor agent is accepted", contributorID},
		{"agent with no declared level is accepted", undeclaredID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var resp QuickCreateIssueResponse
			quickCreate(t, map[string]any{"agent_id": tc.agentID}).Want(http.StatusAccepted).JSON(&resp)
			t.Cleanup(func() {
				testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, resp.TaskID)
			})
		})
	}

	// The comment sub-issue entry point copies the thread's attachments into a
	// source-context snapshot. The refusal has to land before that copy. The
	// runtime advertises the capability and the comment carries an attachment,
	// so without the early check the request would reach the copy and only be
	// refused at enqueue.
	t.Run("comment sub-issue refuses an observer before copying attachments", func(t *testing.T) {
		capable, err := json.Marshal(map[string]any{
			"cli_version":  agent.MinQuickCreateFieldsCLIVersion,
			"capabilities": []string{protocol.DaemonCapabilitySourceContextQuickCreateV1},
		})
		if err != nil {
			t.Fatal(err)
		}
		capableRuntime := dbfx.Runtime(t, "Quick-create autonomy capable runtime", testutil.Cols{"metadata": string(capable)})
		capableObserver := dbfx.Agent(t, "Capable observer", capableRuntime, testutil.Cols{"autonomy_level": "observer"})

		originalStorage := testHandler.Storage
		store := &mockStorage{}
		testHandler.Storage = store
		t.Cleanup(func() { testHandler.Storage = originalStorage })

		issueID := dbfx.Issue(t, "Quick-create autonomy source")
		commentID := dbfx.Comment(t, issueID, "Split the slow ES query into its own task")
		store.put("autonomy/explain.txt", []byte("slow query plan"))
		dbfx.Insert(t, "attachment", testutil.Cols{
			"workspace_id": testWorkspaceID, "issue_id": issueID, "comment_id": commentID,
			"uploader_type": "member", "uploader_id": testUserID, "filename": "explain.txt",
			"url": store.ObjectURL("autonomy/explain.txt"), "content_type": "text/plain", "size_bytes": len("slow query plan"),
		})
		var preview sourceContextPreviewResponse
		testutil.Call(t, testHandler.PreviewCommentSubIssue,
			withURLParam(newRequest(http.MethodGet, "/api/comments/"+commentID+"/sub-issue-preview", nil), "commentId", commentID),
		).Want(http.StatusOK).JSON(&preview)

		before := countTasks(t, capableObserver)
		beforeReads, beforeUploads := store.streamCopyCalls()
		res := testutil.Call(t, testHandler.CreateCommentSubIssue,
			withURLParam(newRequest(http.MethodPost, "/api/comments/"+commentID+"/sub-issues", map[string]any{
				"mode":          "agent",
				"capture_token": preview.CaptureToken,
				"quick_create":  map[string]any{"agent_id": capableObserver, "prompt": "Analyse the slow query"},
			}), "commentId", commentID),
		)
		wantRefusal(t, res)
		if got := countTasks(t, capableObserver); got != before {
			t.Fatalf("observer comment sub-issue must not enqueue: %d -> %d", before, got)
		}
		if reads, uploads := store.streamCopyCalls(); reads != beforeReads || uploads != beforeUploads {
			t.Fatalf("attachments copied before the refusal: reads %d->%d uploads %d->%d", beforeReads, reads, beforeUploads, uploads)
		}
	})
}

// The handler pre-check is a fast path. Slack /issue and the inbox's
// retry-with-context reach the service without it, so the service has to
// refuse on its own.
func TestQuickCreateServiceRefusesObserverWithoutHandlerPrecheck(t *testing.T) {
	if testHandler == nil || dbfx == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	runtimeID := dbfx.Runtime(t, "Quick-create service autonomy runtime")
	observerID := dbfx.Agent(t, "Service observer", runtimeID, testutil.Cols{"autonomy_level": "observer"})
	countTasks := func(t *testing.T) int {
		t.Helper()
		var n int
		if err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_task_queue WHERE agent_id = $1`, observerID).Scan(&n); err != nil {
			t.Fatalf("count tasks: %v", err)
		}
		return n
	}

	t.Run("direct enqueue", func(t *testing.T) {
		before := countTasks(t)
		_, err := testHandler.TaskService.EnqueueQuickCreateTask(ctx,
			util.MustParseUUID(testWorkspaceID), util.MustParseUUID(testUserID), util.MustParseUUID(observerID),
			pgtype.UUID{}, "File the slow ES query", "", "", pgtype.UUID{}, pgtype.UUID{}, nil)
		var autonomyErr *service.QuickCreateAutonomyError
		if !errors.As(err, &autonomyErr) {
			t.Fatalf("EnqueueQuickCreateTask(observer) error = %v, want QuickCreateAutonomyError", err)
		}
		if got := countTasks(t); got != before {
			t.Fatalf("refused enqueue wrote a task: %d -> %d", before, got)
		}
	})

	// A quick-create that failed at CreateIssue before this gate existed is
	// still retryable from the inbox. The retry must be refused with the code
	// the inbox explains, not queued to fail the same way again.
	t.Run("retry with source context", func(t *testing.T) {
		contextID := uuid.NewString()
		payload, err := json.Marshal(service.QuickCreateContext{
			Type: service.QuickCreateContextType, Prompt: "File the slow ES query",
			RequesterID: testUserID, WorkspaceID: testWorkspaceID, SourceContextID: contextID,
		})
		if err != nil {
			t.Fatal(err)
		}
		failedID := dbfx.Task(t, observerID, testutil.Cols{
			"runtime_id": runtimeID, "status": "failed", "context": string(payload),
			"originator_user_id": testUserID, "accountable_user_id": testUserID,
		})
		before := countTasks(t)
		member, err := testHandler.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
			UserID: util.MustParseUUID(testUserID), WorkspaceID: util.MustParseUUID(testWorkspaceID),
		})
		if err != nil {
			t.Fatalf("load member: %v", err)
		}
		req := withURLParam(newRequest(http.MethodPost, "/api/tasks/"+failedID+"/retry-source-context", nil), "taskId", failedID)
		req = req.WithContext(middleware.SetMemberContext(req.Context(), testWorkspaceID, member))
		body := testutil.Call(t, testHandler.RetrySourceContextQuickCreate, req).Want(http.StatusUnprocessableEntity).Map()
		if body["code"] != "agent_autonomy_insufficient" {
			t.Fatalf("retry refusal code = %v, want agent_autonomy_insufficient", body["code"])
		}
		if got := countTasks(t); got != before {
			t.Fatalf("refused retry wrote a task: %d -> %d", before, got)
		}
	})
}
