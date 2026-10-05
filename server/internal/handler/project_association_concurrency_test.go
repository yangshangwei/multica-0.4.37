package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A barrier at the real transaction boundary proves that a writer retains the
// project fence until commit, rather than merely checking existence beforehand.
type projectAssociationCommitStarter struct {
	base    txStarter
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (s projectAssociationCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &projectAssociationCommitTx{Tx: tx, reached: s.reached, release: s.release, once: s.once}, nil
}

type projectAssociationCommitTx struct {
	pgx.Tx
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (tx *projectAssociationCommitTx) Commit(ctx context.Context) error {
	tx.once.Do(func() {
		select {
		case tx.reached <- struct{}{}:
		case <-ctx.Done():
			return
		}
		select {
		case <-tx.release:
		case <-ctx.Done():
		}
	})
	return tx.Tx.Commit(ctx)
}

func projectAssociationWriterForTest(t *testing.T, h *Handler, operation string) (string, string, *http.Request, http.HandlerFunc, int) {
	t.Helper()
	projectID := dbfx.Project(t, "Project association fence")
	issueID := dbfx.Issue(t, "Association target")
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
	var request *http.Request
	var call http.HandlerFunc
	status := http.StatusOK
	switch operation {
	case "issue_view_create":
		request, call, status = newRequest("POST", "/api/issue-views", map[string]any{"name": "Project race view", "scope_type": "project", "scope_id": projectID, "query": map[string]any{}}), h.CreateIssueView, http.StatusCreated
	case "issue_view_preference":
		request, call = newRequest("PUT", "/api/issue-view-preferences", map[string]any{"scope_type": "project", "scope_id": projectID, "prefs": map[string]any{}}), h.PutIssueViewPreference
	case "issue_create", "issue_parent_create":
		body := map[string]any{"title": "Association create", "status": "backlog", "project_id": projectID}
		if operation == "issue_parent_create" {
			dbfx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, projectID, issueID)
			body = map[string]any{"title": "Association child", "status": "backlog", "parent_issue_id": issueID}
		}
		request, call, status = newRequest("POST", "/api/issues", body), h.CreateIssue, http.StatusCreated
	case "issue_update":
		request, call = withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"project_id": projectID}), "id", issueID), h.UpdateIssue
	case "issue_batch_update":
		request, call = newRequest("POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{issueID}, "updates": map[string]any{"project_id": projectID}}), h.BatchUpdateIssues
	case "triage_candidate":
		triageEnableForTest(t)
		body := triageInputForTest()
		body["candidate_project_id"] = projectID
		request, call, status = newRequest("POST", "/api/triage/items", body), h.CreateTriageItem, http.StatusCreated
	case "triage_accept":
		triageEnableForTest(t)
		item := triageCreateForTest(t)
		body := triageInputForTest()
		delete(body, "title")
		body["action"], body["expected_revision"], body["fields"] = "accept", item.Issue.Revision, map[string]any{"project_id": projectID}
		request, call = withURLParam(newRequest("POST", "/api/triage/items/actions", body), "id", item.Issue.ID), h.ActOnTriageItem
	case "chat_create", "chat_update":
		agentID := createHandlerTestAgent(t, "Association chat agent", nil)
		if operation == "chat_create" {
			request, call, status = newRequest("POST", "/api/chat-sessions", map[string]any{"agent_id": agentID, "project_id": projectID, "workspace_id": testWorkspaceID}), h.CreateChatSession, http.StatusCreated
			request = withChatTestWorkspaceCtx(t, request)
		} else {
			sessionID := createChatSessionWithProjectForTest(t, agentID, projectID)
			request, call = withURLParam(newRequest("PATCH", "/api/chat-sessions/"+sessionID, map[string]any{"project_id": projectID}), "sessionId", sessionID), h.UpdateChatSession
			request = withChatTestWorkspaceCtx(t, request)
		}
	case "project_squad_configure":
		request, call = withURLParam(newRequest("PUT", "/api/projects/"+projectID+"/execution-squads", map[string]any{"squads": []map[string]any{{"template_key": "feature-delivery"}}}), "id", projectID), h.ConfigureProjectSquads
	case "autopilot_template_create":
		agentID := createHandlerTestAgent(t, "Association template agent", nil)
		request, call, status = newRequest("POST", "/api/autopilots/from-template", map[string]any{"template_key": "daily-change-review", "assignee_id": agentID, "project_id": projectID}), h.CreateAutopilotFromTemplate, http.StatusCreated
	case "autopilot_create":
		agentID := createHandlerTestAgent(t, "Association automation agent", nil)
		request, call, status = newRequest("POST", "/api/autopilots", map[string]any{"title": "Association automation", "assignee_id": agentID, "execution_mode": "create_issue", "project_id": projectID}), h.CreateAutopilot, http.StatusCreated
	case "autopilot_update", "trigger_create", "trigger_update", "webhook_create":
		autopilotID := createAutopilotAs(t, "", "Association automation")
		if operation == "autopilot_update" {
			request, call = withURLParam(newRequest("PUT", "/api/autopilots/"+autopilotID, map[string]any{"project_id": projectID}), "id", autopilotID), h.UpdateAutopilot
		} else {
			dbfx.Exec(t, `UPDATE autopilot SET project_id=$1 WHERE id=$2`, projectID, autopilotID)
			body := map[string]any{"kind": "schedule", "cron_expression": "0 0 * * *"}
			if operation == "webhook_create" {
				body = map[string]any{"kind": "webhook"}
			}
			request, call, status = withURLParam(newRequest("POST", "/api/autopilots/"+autopilotID+"/triggers", body), "id", autopilotID), h.CreateAutopilotTrigger, http.StatusCreated
			if operation == "trigger_update" {
				triggerID := dbfx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": autopilotID, "kind": "schedule", "cron_expression": "0 0 * * *", "enabled": false})
				request = withURLParam(withURLParam(newRequest("PUT", "/api/autopilots/"+autopilotID+"/triggers/"+triggerID, map[string]any{"enabled": true}), "id", autopilotID), "triggerId", triggerID)
				chi.RouteContext(request.Context()).URLParams.Add("id", autopilotID)
				call, status = h.UpdateAutopilotTrigger, http.StatusOK
			}
		}
	}
	return projectID, issueID, request, call, status
}

func TestProjectAssociationWritersHoldProjectUntilCommit(t *testing.T) {
	for _, operation := range []string{"issue_create", "issue_parent_create", "issue_update", "issue_batch_update", "triage_candidate", "triage_accept", "chat_create", "chat_update", "autopilot_create", "autopilot_update", "trigger_create", "trigger_update", "webhook_create", "autopilot_template_create", "project_squad_configure", "issue_view_create", "issue_view_preference"} {
		t.Run(operation, func(t *testing.T) {
			h := *testHandler
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			barrier := projectAssociationCommitStarter{base: testHandler.TxStarter, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
			defer func() {
				select {
				case <-barrier.release:
				default:
					close(barrier.release)
				}
			}()
			h.TxStarter = barrier
			issueService := *h.IssueService
			issueService.TxStarter = barrier
			h.IssueService = &issueService
			projectID, _, request, call, status := projectAssociationWriterForTest(t, &h, operation)
			requestCtx, requestCancel := context.WithTimeout(request.Context(), 5*time.Second)
			defer requestCancel()
			request = request.WithContext(requestCtx)
			result := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); call(result, request) }()
			select {
			case <-barrier.reached:
			case <-done:
				t.Fatalf("writer stopped before commit: %d %s", result.Code, result.Body.String())
			case <-ctx.Done():
				t.Fatal("writer never reached commit")
			}
			// NO KEY UPDATE conflicts with SHARE but not an incidental FK KEY SHARE.
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, `SELECT id FROM project WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, projectID)
			_ = tx.Rollback(context.Background())
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
				t.Errorf("%s did not retain project association fence through commit: %v", operation, err)
			}
			deletion := *testHandler
			deletePID := make(chan int32, 1)
			deletion.TxStarter = projectAssociationObservedStarter{base: testHandler.TxStarter, pid: deletePID}
			deleteResponse := httptest.NewRecorder()
			deleteDone := make(chan struct{})
			deleteRequest := withURLParam(newRequest("DELETE", "/api/projects/"+projectID, nil), "id", projectID)
			go func() { defer close(deleteDone); deletion.DeleteProject(deleteResponse, deleteRequest) }()
			var pid int32
			select {
			case pid = <-deletePID:
			case <-deleteDone:
				t.Fatalf("delete stopped before transaction: %d %s", deleteResponse.Code, deleteResponse.Body.String())
			case <-ctx.Done():
				t.Fatal("delete did not start")
			}
			blocked := false
			for !blocked {
				if err := testPool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				select {
				case <-deleteDone:
					t.Fatalf("deletion bypassed the writer transaction: %d %s", deleteResponse.Code, deleteResponse.Body.String())
				default:
				}
				if !blocked {
					time.Sleep(time.Millisecond)
				}
			}
			close(barrier.release)
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("writer did not finish")
			}
			if result.Code != status {
				t.Fatalf("writer returned %d, want %d: %s", result.Code, status, result.Body.String())
			}
			select {
			case <-deleteDone:
			case <-ctx.Done():
				t.Fatal("deletion did not finish after writer commit")
			}
			if deleteResponse.Code != http.StatusNoContent {
				t.Fatalf("delete returned %d: %s", deleteResponse.Code, deleteResponse.Body.String())
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, projectID); n != 0 {
				t.Fatalf("deletion left %d issue references", n)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, projectID); n != 0 {
				t.Fatalf("deletion left %d automation references", n)
			}
			for _, table := range []string{"issue_view", "issue_view_preference"} {
				if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND scope_type='project' AND scope_id=$2", testWorkspaceID, projectID); n != 0 {
					t.Errorf("%s retains %d project scope rows", table, n)
				}
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM chat_session WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, projectID); n != 0 {
				t.Fatalf("deletion left %d chat references", n)
			}

		})
	}
}

type projectAssociationCountingStarter struct {
	base  txStarter
	count int
}

func (s *projectAssociationCountingStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.count++
	return s.base.Begin(ctx)
}

func TestProjectAssociationBusyRollsBackEveryAttempt(t *testing.T) {
	for _, operation := range []string{"create", "update", "batch", "accept"} {
		t.Run(operation, func(t *testing.T) {
			projectID := dbfx.Project(t, "Busy project")
			issueID := dbfx.Issue(t, "Unchanged issue")
			dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
			h := *testHandler
			counter := &projectAssociationCountingStarter{base: h.TxStarter}
			h.TxStarter = counter
			svc := *h.IssueService
			svc.TxStarter = counter
			h.IssueService = &svc
			var req *http.Request
			var call http.HandlerFunc
			switch operation {
			case "create":
				req, call = newRequest("POST", "/api/issues", map[string]any{"title": "Busy create", "status": "backlog", "project_id": projectID}), h.CreateIssue
			case "update":
				req, call = withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"title": "Should roll back", "project_id": projectID}), "id", issueID), h.UpdateIssue
			case "batch":
				req, call = newRequest("POST", "/api/issues/batch-update", map[string]any{"issue_ids": []string{issueID}, "updates": map[string]any{"title": "Should roll back", "project_id": projectID}}), h.BatchUpdateIssues
			case "accept":
				triageEnableForTest(t)
				item := triageCreateForTest(t)
				issueID = item.Issue.ID
				req, call = withURLParam(newRequest("POST", "/api/triage/items/actions", map[string]any{"request_id": triageInputForTest()["request_id"], "action": "accept", "expected_revision": item.Issue.Revision, "fields": map[string]any{"project_id": projectID}}), "id", issueID), h.ActOnTriageItem
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(ctx, `SELECT id FROM project WHERE id=$1 FOR UPDATE`, projectID); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			call(response, req)
			if response.Code != http.StatusConflict {
				t.Fatalf("busy %s must conflict: %d %s", operation, response.Code, response.Body.String())
			}
			if counter.count != 4 {
				t.Fatalf("want 4 complete transaction attempts, got %d", counter.count)
			}
			// A failed attempt must release the issue lock instead of retaining it
			// while waiting on the project, otherwise deletion cannot finish its sweep.
			if _, err = tx.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, issueID); err != nil {
				t.Fatalf("failed attempt retained issue lock: %v", err)
			}
			var project *string
			var admission string
			if err = tx.QueryRow(ctx, `SELECT project_id,admission_status FROM issue WHERE id=$1`, issueID).Scan(&project, &admission); err != nil {
				t.Fatal(err)
			}
			if project != nil {
				t.Fatalf("failed association was committed: %v", project)
			}
			if operation == "accept" && admission != "pending" {
				t.Fatalf("failed acceptance changed admission: %s", admission)
			}
		})
	}
}

type projectAssociationObservedStarter struct {
	base txStarter
	pid  chan int32
}

func (s projectAssociationObservedStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.base.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Do not issue SQL here: P1 sets transaction isolation before its first query.
	s.pid <- int32(tx.Conn().PgConn().PID())
	return tx, nil
}

// Pause after transport preflight but before the final transaction. Deletion
// commits in this gap, so the final write must reject stale project references.
type projectAssociationBeforeBeginStarter struct {
	base    txStarter
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (s projectAssociationBeforeBeginStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.once.Do(func() {
		select {
		case s.reached <- struct{}{}:
		case <-ctx.Done():
			return
		}
		select {
		case <-s.release:
		case <-ctx.Done():
		}
	})
	return s.base.Begin(ctx)
}
func TestProjectAssociationDeletionWinsBeforeFinalWrite(t *testing.T) {
	for _, operation := range []string{"issue_create", "issue_parent_create", "issue_update", "issue_batch_update", "triage_candidate", "triage_accept", "chat_create", "chat_update", "autopilot_create", "autopilot_update", "trigger_create", "trigger_update", "webhook_create", "autopilot_template_create", "project_squad_configure", "issue_view_create", "issue_view_preference"} {
		t.Run(operation, func(t *testing.T) {
			h := *testHandler
			barrier := projectAssociationBeforeBeginStarter{base: h.TxStarter, reached: make(chan struct{}, 1), release: make(chan struct{}), once: &sync.Once{}}
			defer func() {
				select {
				case <-barrier.release:
				default:
					close(barrier.release)
				}
			}()
			h.TxStarter = barrier
			issueService := *h.IssueService
			issueService.TxStarter = barrier
			h.IssueService = &issueService
			projectID, issueID, req, call, _ := projectAssociationWriterForTest(t, &h, operation)
			ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); call(response, req) }()
			select {
			case <-barrier.reached:
			case <-done:
				t.Fatalf("writer stopped before final transaction: %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("writer never entered final transaction")
			}
			deletion := httptest.NewRecorder()
			testHandler.DeleteProject(deletion, withURLParam(newRequest("DELETE", "/api/projects/"+projectID, nil), "id", projectID))
			if deletion.Code != http.StatusNoContent {
				t.Fatalf("deletion failed: %d %s", deletion.Code, deletion.Body.String())
			}
			close(barrier.release)
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("writer did not finish")
			}
			// An inherited parent project was cleared by deletion before the child
			// transaction reads it. Creating an unassociated child remains valid.
			if operation != "issue_parent_create" && response.Code < 400 {
				t.Fatalf("stale project write succeeded: %d %s", response.Code, response.Body.String())
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, projectID); n != 0 {
				t.Fatalf("left %d dangling issue references", n)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, projectID); n != 0 {
				t.Fatalf("left %d dangling automation references", n)
			}
			for _, table := range []string{"issue_view", "issue_view_preference"} {
				if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND scope_type='project' AND scope_id=$2", testWorkspaceID, projectID); n != 0 {
					t.Errorf("%s retains %d project scope rows", table, n)
				}
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM chat_session WHERE workspace_id=$1 AND project_id=$2`, testWorkspaceID, projectID); n != 0 {
				t.Fatalf("left %d dangling chat references", n)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1`, issueID); n != 1 {
				t.Fatal("project deletion removed existing issue")
			}
		})
	}
}

func TestProjectDeletedAutomationTriggerEditsRemainInert(t *testing.T) {
	autopilotID := createAutopilotAs(t, "", "Deleted project trigger")
	dbfx.Exec(t, `UPDATE autopilot SET status='paused',pause_reason='project_deleted' WHERE id=$1`, autopilotID)
	triggerID := dbfx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": autopilotID, "kind": "schedule", "cron_expression": "0 0 * * *", "enabled": false})
	for _, tc := range []struct {
		body   map[string]any
		status int
	}{{map[string]any{"label": "Kept configuration"}, http.StatusOK}, {map[string]any{"enabled": false}, http.StatusOK}, {map[string]any{"enabled": true}, http.StatusConflict}} {
		request := withURLParam(newRequest("PUT", "/api/autopilots/"+autopilotID+"/triggers/"+triggerID, tc.body), "id", autopilotID)
		chi.RouteContext(request.Context()).URLParams.Add("triggerId", triggerID)
		response := httptest.NewRecorder()
		testHandler.UpdateAutopilotTrigger(response, request)
		if response.Code != tc.status {
			t.Fatalf("trigger edit %v: got %d want %d: %s", tc.body, response.Code, tc.status, response.Body.String())
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM autopilot_trigger WHERE id=$1 AND enabled`, triggerID); n != 0 {
		t.Fatal("deleted project's trigger became enabled")
	}
}

type projectViewPreflightGate struct {
	db.DBTX
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (g projectViewPreflightGate) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := g.DBTX.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: GetProjectInWorkspace ") {
		return projectViewPreflightRow{Row: row, ctx: ctx, reached: g.reached, release: g.release, once: g.once}
	}
	return row
}

type projectViewPreflightRow struct {
	pgx.Row
	ctx     context.Context
	reached chan struct{}
	release chan struct{}
	once    *sync.Once
}

func (r projectViewPreflightRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	r.once.Do(func() {
		select {
		case r.reached <- struct{}{}:
		case <-r.ctx.Done():
			return
		}
		select {
		case <-r.release:
		case <-r.ctx.Done():
		}
	})
	return nil
}
func TestProjectScopeViewDeletionWinsAfterPreflight(t *testing.T) {
	for _, kind := range []string{"view", "preference"} {
		t.Run(kind, func(t *testing.T) {
			projectID := dbfx.Project(t, "View preflight race")
			reached, release := make(chan struct{}, 1), make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			h := *testHandler
			h.Queries = db.New(projectViewPreflightGate{DBTX: testPool, reached: reached, release: release, once: &sync.Once{}})
			var req *http.Request
			var call http.HandlerFunc
			if kind == "view" {
				req, call = newRequest("POST", "/api/issue-views", map[string]any{"name": "Race view", "scope_type": "project", "scope_id": projectID, "query": map[string]any{}}), h.CreateIssueView
			} else {
				req, call = newRequest("PUT", "/api/issue-view-preferences", map[string]any{"scope_type": "project", "scope_id": projectID, "prefs": map[string]any{}}), h.PutIssueViewPreference
			}
			ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { defer close(done); call(response, req) }()
			select {
			case <-reached:
			case <-done:
				t.Fatalf("writer never passed preflight: %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("preflight timed out")
			}
			deletion := httptest.NewRecorder()
			testHandler.DeleteProject(deletion, withURLParam(newRequest("DELETE", "/api/projects/"+projectID, nil), "id", projectID))
			if deletion.Code != 204 {
				t.Fatalf("delete failed: %d %s", deletion.Code, deletion.Body.String())
			}
			close(release)
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("writer timed out")
			}
			if response.Code != http.StatusNotFound {
				t.Errorf("late project scope save must return404: %d %s", response.Code, response.Body.String())
			}
			for _, table := range []string{"issue_view", "issue_view_preference"} {
				if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1 AND scope_type='project' AND scope_id=$2", testWorkspaceID, projectID); n != 0 {
					t.Errorf("%s retains %d project rows", table, n)
				}
			}
		})
	}
}

func TestProjectDeletionRemovesExistingViewPreferences(t *testing.T) {
	projectID := dbfx.Project(t, "Existing project preference")
	pref := httptest.NewRecorder()
	testHandler.PutIssueViewPreference(pref, newRequest("PUT", "/api/issue-view-preferences", map[string]any{"scope_type": "project", "scope_id": projectID, "prefs": map[string]any{"hidden": []string{"builtin:all"}}}))
	if pref.Code != 200 {
		t.Fatalf("create preference: %d %s", pref.Code, pref.Body.String())
	}
	viewID := dbfx.Insert(t, "issue_view", testutil.Cols{"workspace_id": testWorkspaceID, "owner_id": testUserID, "name": "Pinned project view", "scope_type": "project", "scope_id": projectID, "query": "{}", "display": "{}"})
	dbfx.Insert(t, "pinned_item", testutil.Cols{"workspace_id": testWorkspaceID, "item_type": "view", "item_id": viewID, "user_id": testUserID, "position": 0})
	response := httptest.NewRecorder()
	testHandler.DeleteProject(response, withURLParam(newRequest("DELETE", "/api/projects/"+projectID, nil), "id", projectID))
	if response.Code != 204 {
		t.Fatalf("delete: %d %s", response.Code, response.Body.String())
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_view_preference WHERE workspace_id=$1 AND scope_type='project' AND scope_id=$2`, testWorkspaceID, projectID); n != 0 {
		t.Errorf("retained %d project preferences", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM pinned_item WHERE item_type='view' AND item_id=$1`, viewID); n != 0 {
		t.Errorf("retained %d project view pins", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_view WHERE id=$1`, viewID); n != 0 {
		t.Errorf("retained %d project views", n)
	}
}
