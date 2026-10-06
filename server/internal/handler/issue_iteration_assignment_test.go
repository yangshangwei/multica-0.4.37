package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func issueIterationAssignmentFixture(t *testing.T) (*Handler, string) {
	t.Helper()
	h := iterationSettingsHandler(t)
	dbfx.InsertNoID(t, "workspace_iteration_settings", testutil.Cols{"workspace_id": testWorkspaceID, "enabled": true}, "workspace_id=$1", testWorkspaceID)
	iid := dbfx.Insert(t, "iteration", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Assignment target", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "created_by": testUserID})
	dbfx.Cleanup(t, "DELETE FROM iteration_participation WHERE iteration_id=$1", iid)
	dbfx.Cleanup(t, "DELETE FROM iteration_event WHERE iteration_id=$1", iid)
	dbfx.Exec(t, "UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1", testWorkspaceID)
	return h, iid
}
func TestIssueIterationAssignmentCreateAndDuplicateRetry(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	title := "Confirmed create " + uuid.NewString()
	dbfx.Cleanup(t, "DELETE FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title)
	body := map[string]any{"title": title, "current_iteration_id": iid, "expected_iteration_revision": 1}
	var created IssueResponse
	testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", body)).Want(201).JSON(&created)
	if created.CurrentIterationID == nil || *created.CurrentIterationID != iid || created.IterationRolloverCount != 0 {
		t.Fatalf("creation response lost membership: %+v", created)
	}
	testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", body)).Want(409)
	if n := dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2 AND current_iteration_id=$3 AND iteration_rollover_count=0", testWorkspaceID, title, iid); n != 1 {
		t.Fatalf("created association count=%d", n)
	}
	if dbfx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1", iid) != 1 || dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND issue_id IS NOT NULL", iid) != 1 {
		t.Fatal("create retry duplicated participation or event")
	}
}
func TestIssueIterationAssignmentCreateRejectsUnconfirmedOrStaleTarget(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	for _, tc := range []struct {
		name  string
		extra map[string]any
		want  int
	}{{"unconfirmed", nil, 428}, {"stale", map[string]any{"expected_iteration_revision": 2}, 409}, {"fractional", map[string]any{"expected_iteration_revision": 1.2}, 400}} {
		t.Run(tc.name, func(t *testing.T) {
			title := uuid.NewString()
			body := map[string]any{"title": title, "current_iteration_id": iid}
			for k, v := range tc.extra {
				body[k] = v
			}
			testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", body)).Want(tc.want)
			if dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title) != 0 {
				t.Fatal("failed join committed issue")
			}
		})
	}
}
func TestIssueIterationAssignmentUpdateCASReasonAndNoop(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	issueID := dbfx.Issue(t, "Existing task", testutil.Cols{"iteration_rollover_count": 3})
	call := func(body map[string]any, want int) {
		testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, body), "id", issueID)).Want(want)
	}
	call(map[string]any{"current_iteration_id": iid}, 428)
	call(map[string]any{"current_iteration_id": iid, "expected_revision": 1}, 200)
	call(map[string]any{"current_iteration_id": iid, "expected_revision": 2}, 200)
	if dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", iid) != 1 {
		t.Fatal("same target was not no-op")
	}
	call(map[string]any{"current_iteration_id": nil, "expected_revision": 1, "iteration_reason": "Leave"}, 409)
	call(map[string]any{"current_iteration_id": nil, "expected_revision": 2}, 422)
	call(map[string]any{"current_iteration_id": nil, "expected_revision": 2, "iteration_reason": "Leave"}, 200)
	call(map[string]any{"current_iteration_id": iid, "expected_revision": 3, "title": "Compound"}, 400)
	var revision, rollover int
	dbfx.QueryRow(t, "SELECT revision,iteration_rollover_count FROM issue WHERE id=$1", issueID).Scan(&revision, &rollover)
	if revision != 3 || rollover != 3 {
		t.Fatalf("wrong issue state revision=%d rollover=%d", revision, rollover)
	}
	var reason string
	dbfx.QueryRow(t, "SELECT reason FROM iteration_event WHERE iteration_id=$1 AND after_facts='null'::jsonb", iid).Scan(&reason)
	if reason != "Leave" {
		t.Fatal("leave reason discarded")
	}
}
func TestIssueIterationAssignmentDisabledAndServerOwnedFields(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	issueID := dbfx.Issue(t, "Existing task")
	for _, field := range []string{"current_iteration_id", "CURRENT_ITERATION_ID", "iteration_rollover_count"} {
		t.Run(field, func(t *testing.T) {
			testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{field: nil, "expected_revision": 1}), "id", issueID)).Want(428)
		})
	}
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"iteration_rollover_count": 1, "expected_revision": 1}), "id", issueID)).Want(428)
	dbfx.Exec(t, "UPDATE workspace_iteration_settings SET enabled=false WHERE workspace_id=$1", testWorkspaceID)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"current_iteration_id": iid, "expected_revision": 1}), "id", issueID)).Want(422)
}
func TestIssueIterationAssignmentCreateNullIsExplicitlyUnassociated(t *testing.T) {
	h, _ := issueIterationAssignmentFixture(t)
	title := fmt.Sprintf("Unassociated %s", uuid.NewString())
	dbfx.Cleanup(t, "DELETE FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title)
	testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": title, "current_iteration_id": nil})).Want(201)
	if dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2 AND current_iteration_id IS NULL", testWorkspaceID, title) != 1 {
		t.Fatal("explicit null not unassociated")
	}
}

type issueAssignmentStarter struct {
	inner     txStarter
	failEvent bool
	before    func()
	once      sync.Once
	pid       chan uint32
}
type issueAssignmentTx struct {
	pgx.Tx
	failEvent bool
}

func (s *issueAssignmentStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	if s.before != nil {
		s.once.Do(s.before)
	}
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if s.pid != nil {
		s.pid <- tx.Conn().PgConn().PID()
	}
	return &issueAssignmentTx{Tx: tx, failEvent: s.failEvent}, nil
}
func (tx *issueAssignmentTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.failEvent && strings.Contains(sql, "AppendIterationLifecycleEvent") {
		return pgconn.CommandTag{}, errors.New("injected assignment event failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestIssueIterationAssignmentCreationRecorderRollback(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	title := uuid.NewString()
	s := *h.IssueService
	s.TxStarter = &issueAssignmentStarter{inner: h.TxStarter, failEvent: true}
	h.IssueService = &s
	var counter int64
	dbfx.QueryRow(t, "SELECT issue_counter FROM workspace WHERE id=$1", testWorkspaceID).Scan(&counter)
	testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": title, "current_iteration_id": iid, "expected_iteration_revision": 1})).Want(500)
	var after int64
	dbfx.QueryRow(t, "SELECT issue_counter FROM workspace WHERE id=$1", testWorkspaceID).Scan(&after)
	if counter != after || dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title) != 0 || dbfx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1", iid) != 0 {
		t.Fatal("failed creation retained issue, counter or participation")
	}
}

func TestIssueIterationAssignmentObserverAndCurrentAutonomy(t *testing.T) {
	for _, race := range []bool{false, true} {
		t.Run(fmt.Sprint(race), func(t *testing.T) {
			h, iid := issueIterationAssignmentFixture(t)
			issueID := dbfx.Issue(t, "Observer cannot schedule")
			level := "observer"
			if race {
				level = "contributor"
			}
			agentID, taskID := autonomyTestAgent(t, "Assignment actor", level)
			if race {
				h.TxStarter = &issueAssignmentStarter{inner: h.TxStarter, before: func() { dbfx.Exec(t, "UPDATE agent SET autonomy_level='observer' WHERE id=$1", agentID) }}
			}
			req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"CURRENT_ITERATION_ID": iid, "expected_revision": 1}), agentID, taskID)
			testutil.Call(t, h.UpdateIssue, withURLParam(req, "id", issueID)).Want(403)
			if dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id IS NULL AND revision=1", issueID) != 1 {
				t.Fatal("observer changed membership")
			}
		})
	}
}

func TestIssueIterationAssignmentTerminalAdmissionAndHistory(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	for _, state := range []string{"pending", "rejected", "duplicate"} {
		issueID := dbfx.Issue(t, state, testutil.Cols{"admission_status": state})
		testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"current_iteration_id": iid, "expected_revision": 1}), "id", issueID)).Want(409)
	}
	done := dbfx.Issue(t, "Done task", testutil.Cols{"status": "done"})
	body := map[string]any{"current_iteration_id": iid, "expected_revision": 1, "allow_completed": true}
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+done, body), "id", done)).Want(422)
	dbfx.Exec(t, "UPDATE iteration SET status='active',started_at=clock_timestamp(),started_by=$2 WHERE id=$1", iid, testUserID)
	body["allow_completed"] = false
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+done, body), "id", done)).Want(422)
	body["allow_completed"] = true
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+done, body), "id", done)).Want(200)
	dbfx.Exec(t, "UPDATE iteration_participation SET in_original=true,original_facts=$3::jsonb WHERE iteration_id=$1 AND issue_id=$2", iid, done, `{"title":"Permanent baseline"}`)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+done, map[string]any{"current_iteration_id": nil, "expected_revision": 2, "iteration_reason": "Remove"}), "id", done)).Want(200)
	body["expected_revision"] = 3
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+done, body), "id", done)).Want(200)
	if dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='reenter'", iid) != 1 || dbfx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1 AND original_facts->>'title'='Permanent baseline'", iid) != 1 {
		t.Fatal("reentry changed original history")
	}
	dbfx.Exec(t, "UPDATE iteration SET status='completed' WHERE id=$1", iid)
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+done, map[string]any{"current_iteration_id": nil, "expected_revision": 4, "iteration_reason": "Attempt historical edit"}), "id", done)).Want(409)
}

func TestIssueIterationAssignmentLocksTargetBeforeIssue(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	issueID := dbfx.Issue(t, "Wait before issue lock")
	owner, err := testPool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Rollback(t.Context()) }()
	if _, err = owner.Exec(t.Context(), "SELECT 1 FROM iteration WHERE id=$1 FOR UPDATE", iid); err != nil {
		t.Fatal(err)
	}
	pids := make(chan uint32, 1)
	h.TxStarter = &issueAssignmentStarter{inner: h.TxStarter, pid: pids}
	done := make(chan *testutil.Response, 1)
	go func() {
		response := testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"current_iteration_id": iid, "expected_revision": 1}), "id", issueID))
		done <- response
	}()
	pid := <-pids
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-done:
			t.Fatal("update bypassed target lock")
		case <-deadline:
			t.Fatal("target wait not observed")
		default:
		}
		var blocked bool
		dbfx.QueryRow(t, "SELECT $1::integer=ANY(pg_blocking_pids($2::integer))", owner.Conn().PgConn().PID(), pid).Scan(&blocked)
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err = owner.Exec(t.Context(), "SELECT 1 FROM issue WHERE id=$1 FOR UPDATE NOWAIT", issueID); err != nil {
		t.Fatalf("issue locked before target: %v", err)
	}
	if err = owner.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	(<-done).Want(200)
}

func TestIssueIterationAssignmentDoesNotInvokeUnchangedPrivateAssignee(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	other := dbfx.User(t, "Private agent owner", uuid.NewString()+"@test.invalid")
	agent := dbfx.Agent(t, "Existing private assignee", handlerTestRuntimeID(t), testutil.Cols{"owner_id": other, "permission_mode": "private"})
	issueID := dbfx.Issue(t, "Existing private assignment", testutil.Cols{"assignee_type": "agent", "assignee_id": agent, "status": "todo"})
	taskID := dbfx.Task(t, agent, testutil.Cols{"issue_id": issueID, "status": "running", "runtime_id": handlerTestRuntimeID(t), "session_id": "existing-session"})
	before, err := h.Queries.GetAgentTask(t.Context(), parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, h.UpdateIssue, withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"current_iteration_id": iid, "expected_revision": 1}), "id", issueID)).Want(200)
	after, err := h.Queries.GetAgentTask(t.Context(), parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != after.Status || before.SessionID != after.SessionID || before.StartedAt != after.StartedAt || dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issueID) != 1 {
		t.Fatal("membership changed execution")
	}
}

func TestIssueIterationAssignmentCreateKeepsOneOrdinaryAssignedRun(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	agent := dbfx.Agent(t, "Assigned creator flow", handlerTestRuntimeID(t))
	title := uuid.NewString()
	dbfx.Cleanup(t, "DELETE FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title)
	dbfx.Cleanup(t, "DELETE FROM agent_task_queue WHERE agent_id=$1", agent)
	testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": title, "status": "todo", "assignee_type": "agent", "assignee_id": agent, "current_iteration_id": iid, "expected_iteration_revision": 1})).Want(201)
	if count := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue t JOIN issue i ON i.id=t.issue_id WHERE i.workspace_id=$1 AND i.title=$2 AND i.current_iteration_id=$3", testWorkspaceID, title, iid); count != 1 {
		t.Fatalf("ordinary assigned creation enqueued %d runs", count)
	}
	if dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='execution_started'", iid) != 0 {
		t.Fatal("creation or enqueue invented execution start")
	}
}

func TestIssueIterationAssignmentResponseBuildersKeepKnownFields(t *testing.T) {
	for _, associated := range []bool{false, true} {
		t.Run(fmt.Sprint(associated), func(t *testing.T) {
			var id pgtype.UUID
			var rollover int32
			if associated {
				id = parseUUID(uuid.NewString())
				rollover = 3
			}
			values := map[string]any{
				"detail":    issueToResponse(db.Issue{CurrentIterationID: id, IterationRolloverCount: rollover}, "I1"),
				"list":      issueListRowToResponse(db.ListIssuesRow{CurrentIterationID: id, IterationRolloverCount: rollover}, "I1"),
				"open":      openIssueRowToResponse(db.ListOpenIssuesRow{CurrentIterationID: id, IterationRolloverCount: rollover}, "I1"),
				"websocket": service.IssueToMap(db.Issue{CurrentIterationID: id, IterationRolloverCount: rollover}, "I1"),
			}
			for name, value := range values {
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err = json.Unmarshal(raw, &fields); err != nil {
					t.Fatal(err)
				}
				if _, exists := fields["current_iteration_id"]; !exists {
					t.Errorf("%s omitted current membership", name)
				}
				if string(fields["iteration_rollover_count"]) != fmt.Sprint(rollover) {
					t.Errorf("%s omitted or changed known rollover: %s", name, fields["iteration_rollover_count"])
				}
				if associated && string(fields["current_iteration_id"]) != fmt.Sprintf("%q", uuidToString(id)) {
					t.Errorf("%s lost association", name)
				} else if !associated && string(fields["current_iteration_id"]) != "null" {
					t.Errorf("%s unassociated must be explicit null", name)
				}
			}
		})
	}
}

func TestIssueIterationAssignmentReadPathsKeepKnownFields(t *testing.T) {
	h, iid := issueIterationAssignmentFixture(t)
	project := dbfx.Project(t, "Iteration response parity")
	title := "iterationmembership" + strings.ReplaceAll(uuid.NewString(), "-", "")
	issueID := dbfx.Issue(t, title, testutil.Cols{"current_iteration_id": iid, "iteration_rollover_count": 3, "project_id": project, "status": "todo"})
	for _, path := range []string{"detail", "list", "open", "search", "grouped", "table"} {
		t.Run(path, func(t *testing.T) {
			var issues []IssueResponse
			switch path {
			case "detail":
				var issue IssueResponse
				testutil.Call(t, h.GetIssue, withURLParam(newRequest("GET", "/api/issues/"+issueID, nil), "id", issueID)).Want(200).JSON(&issue)
				issues = []IssueResponse{issue}
			case "list", "open", "search":
				url := "/api/issues?project_id=" + project
				handler := h.ListIssues
				if path == "open" {
					url += "&open_only=true"
				}
				if path == "search" {
					url = "/api/issues/search?q=" + title
					handler = h.SearchIssues
				}
				var response struct {
					Issues []IssueResponse `json:"issues"`
				}
				testutil.Call(t, handler, newRequest("GET", url, nil)).Want(200).JSON(&response)
				issues = response.Issues
			case "grouped":
				var response GroupedIssuesResponse
				testutil.Call(t, h.ListGroupedIssues, newRequest("GET", "/api/issues/grouped?project_id="+project, nil)).Want(200).JSON(&response)
				for _, group := range response.Groups {
					issues = append(issues, group.Issues...)
				}
			case "table":
				var response issueTableRowsResponse
				testutil.Call(t, h.ListIssueTableRows, newRequest("POST", "/api/issues/table/rows", issueTableRowsRequest{Query: issueTableQuerySpec{Scope: issueTableScope{Kind: "project", ProjectID: project}, Sort: issueTableSortRequest{Field: "position", Direction: "asc"}}, Group: issueTableGroupSpec{Kind: "none"}, Page: issueTablePageRequest{Limit: 10}})).Want(200).JSON(&response)
				for _, row := range response.Rows {
					issues = append(issues, row.Issue)
				}
			}
			found := false
			for _, issue := range issues {
				if issue.ID == issueID {
					found = true
					if issue.CurrentIterationID == nil || *issue.CurrentIterationID != iid || issue.IterationRolloverCount != 3 {
						t.Fatalf("%s fabricated missing membership/zero rollover: %+v", path, issue)
					}
				}
			}
			if !found {
				t.Fatalf("%s omitted fixture issue", path)
			}
		})
	}
}

func TestIssueIterationAssignmentCreateCompletedAcknowledgment(t *testing.T) {
	cases := []struct {
		name, target, status string
		sendAck              bool
		ack                  any
		want                 int
	}{
		{"active_explicit", "active", "done", true, true, 201},
		{"active_default", "active", "done", false, nil, 422},
		{"active_false", "active", "done", true, false, 422},
		{"planned_rejects_completed", "planned", "done", true, true, 422},
		{"active_rejects_cancelled", "active", "cancelled", true, true, 422},
		{"no_target", "none", "done", true, true, 400},
		{"null_target", "null", "done", true, false, 400},
		{"null_acknowledgment", "active", "done", true, nil, 400},
		{"string_acknowledgment", "active", "done", true, "true", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, iid := issueIterationAssignmentFixture(t)
			if tc.target == "active" {
				dbfx.Exec(t, "UPDATE iteration SET status='active',started_at=clock_timestamp(),started_by=$2 WHERE id=$1", iid, testUserID)
			}
			agent := dbfx.Agent(t, "Completed creation", handlerTestRuntimeID(t))
			title := uuid.NewString()
			dbfx.Cleanup(t, "DELETE FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title)
			dbfx.Cleanup(t, "DELETE FROM agent_task_queue WHERE agent_id=$1", agent)
			body := map[string]any{"title": title, "status": tc.status, "assignee_type": "agent", "assignee_id": agent}
			switch tc.target {
			case "none":
			case "null":
				body["current_iteration_id"] = nil
			default:
				body["current_iteration_id"] = iid
				body["expected_iteration_revision"] = 1
			}
			if tc.sendAck {
				body["allow_completed"] = tc.ack
			}
			response := testutil.Call(t, h.CreateIssue, newRequest("POST", "/api/issues", body)).Want(tc.want)
			wantCount := 0
			if tc.want == 201 {
				wantCount = 1
				var issue IssueResponse
				response.JSON(&issue)
				if issue.CurrentIterationID == nil || *issue.CurrentIterationID != iid || issue.Status != "done" {
					t.Fatalf("confirmed completed creation lost state: %+v", issue)
				}
			}
			if dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2", testWorkspaceID, title) != wantCount || dbfx.Count(t, "SELECT count(*) FROM iteration_participation WHERE iteration_id=$1", iid) != wantCount || dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1", iid) != wantCount {
				t.Fatal("completed creation partially committed or duplicated membership")
			}
			if dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", agent) != wantCount {
				t.Fatal("completed iteration assignment changed the ordinary creation enqueue count")
			}
			if dbfx.Count(t, "SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND kind='execution_started'", iid) != 0 {
				t.Fatal("completed iteration assignment invented execution-start facts")
			}
		})
	}
}
