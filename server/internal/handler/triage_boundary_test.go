package handler

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestTriageBoundaryOrdinaryMutations(t *testing.T) {
	for _, state := range []string{"pending", "rejected", "duplicate"} {
		t.Run(state, func(t *testing.T) {
			id := dbfx.Issue(t, "Review first", testutil.Cols{"admission_status": state, "status": "backlog"})
			for _, patch := range []map[string]any{{"status": "todo"}, {"assignee_type": "member", "assignee_id": testUserID}, {"project_id": nil}, {"parent_issue_id": nil}, {"position": 1}, {"stage": 1}} {
				req := withURLParam(newRequest("PATCH", "/api/issues/"+id, patch), "id", id)
				var out map[string]any
				testutil.Call(t, testHandler.UpdateIssue, req).Want(409).JSON(&out)
				if out["code"] != "triage_review_required" || out["issue_id"] != id {
					t.Fatalf("missing actionable conflict: %v", out)
				}
			}
			testutil.Call(t, testHandler.BatchUpdateIssues, newRequest("POST", "/api/issues/batch?workspace_id="+testWorkspaceID, map[string]any{"issue_ids": []string{id}, "updates": map[string]any{"status": "todo"}})).Want(409)
			testutil.Call(t, testHandler.RerunIssue, withURLParam(newRequest("POST", "/api/issues/"+id+"/rerun", nil), "id", id)).Want(409)
			var issue IssueResponse
			testutil.Call(t, testHandler.UpdateIssue, withURLParam(newRequest("PATCH", "/api/issues/"+id, map[string]any{"title": "Content remains editable", "description": "Discuss this input"}), "id", id)).Want(200).JSON(&issue)
			if issue.AdmissionStatus != state || issue.Status != "backlog" {
				t.Fatalf("content edit changed workflow: %+v", issue)
			}
		})
	}
}

func TestTriageBoundaryFormalCollectionsAndExplicitSearch(t *testing.T) {
	project := dbfx.Project(t, "Triage boundary statistics")
	parent := dbfx.Issue(t, "Triage boundary parent")
	for _, state := range []string{"pending", "rejected", "duplicate", "accepted", "not_required"} {
		dbfx.Issue(t, "Triage collection "+state, testutil.Cols{"admission_status": state, "project_id": project, "parent_issue_id": parent, "status": "todo"})
	}
	var list struct {
		Issues []IssueResponse `json:"issues"`
		Total  int             `json:"total"`
	}
	testutil.Call(t, testHandler.ListIssues, newRequest("GET", "/api/issues?workspace_id="+testWorkspaceID+"&project_id="+project, nil)).Want(200).JSON(&list)
	if len(list.Issues) != 2 || list.Total != 2 {
		t.Fatalf("ordinary list includes inputs: %+v", list)
	}
	children, err := testHandler.Queries.ListChildIssues(t.Context(), parseUUID(parent))
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("formal child count=%d", len(children))
	}
	stats, err := testHandler.Queries.GetProjectIssueStats(t.Context(), db.GetProjectIssueStatsParams{WorkspaceID: parseUUID(testWorkspaceID), ProjectIds: []pgtype.UUID{parseUUID(project)}, TerminalStatusKeys: []string{"done", "cancelled"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].TotalCount != 2 || stats[0].DoneCount != 0 {
		t.Fatalf("project stats include inputs: %+v", stats)
	}
	// Execute the shared compiler's predicate: rows, groups and facets all consume it.
	spec := issueTableQuerySpec{Scope: issueTableScope{Kind: "workspace"}, Filters: issueTableFiltersRequest{ProjectIDs: []string{project}}, Sort: issueTableSortRequest{Field: "position", Direction: "asc"}}
	req := newRequest(http.MethodPost, "/api/issues/table/rows?workspace_id="+testWorkspaceID, nil)
	var compiled issueTableSQL
	testutil.Call(t, func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		compiled, ok = testHandler.compileIssueTableQuery(w, r, spec)
		if ok {
			w.WriteHeader(200)
		}
	}, req).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM issue i WHERE "+compiled.where, compiled.args...); n != 2 {
		t.Fatalf("table/group/facet base included inputs: %d", n)
	}
	var search struct {
		Issues []IssueResponse `json:"issues"`
		Total  int             `json:"total"`
	}
	testutil.Call(t, testHandler.SearchIssues, newRequest("GET", "/api/issues/search?workspace_id="+testWorkspaceID+"&q=Triage%20collection", nil)).Want(200).JSON(&search)
	if search.Total != 5 || len(search.Issues) != 5 {
		t.Fatalf("explicit search lost triage inputs: %+v", search)
	}
	for _, item := range search.Issues {
		if item.AdmissionStatus == "" {
			t.Fatal("search omitted admission marker")
		}
	}
}

func TestTriageBoundaryCommentsAndLifecycleStayInert(t *testing.T) {
	for _, state := range []string{"pending", "rejected", "duplicate"} {
		t.Run(state, func(t *testing.T) {
			agent := dbfx.Agent(t, "Triage mention target", testRuntimeID)
			squad := dbfx.Squad(t, "Triage squad", agent)
			id := dbfx.Issue(t, "Triage inert comments", testutil.Cols{"admission_status": state, "status": "backlog"})
			content := "[@Agent](mention://agent/" + agent + ") [@Squad](mention://squad/" + squad + ") please run"
			var comment CommentResponse
			testutil.Call(t, testHandler.CreateComment, withURLParam(newRequest("POST", "/api/issues/"+id+"/comments", map[string]any{"content": content}), "id", id)).Want(201).JSON(&comment)
			if comment.Content != content || len(comment.TriggerOutcomes) != 2 {
				t.Fatalf("comment lost or no blocked outcome: %+v", comment)
			}
			for _, outcome := range comment.TriggerOutcomes {
				if outcome.ReasonCode != ReasonTriageReviewRequired {
					t.Fatalf("unclear mention outcome: %+v", outcome)
				}
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", id); n != 0 {
				t.Fatal("pending text dispatched execution")
			}
			body := map[string]any{"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_title": "Must not exist"}
			testutil.Call(t, testHandler.CreateLifecycleHandoff, withURLParam(newRequest("POST", "/api/issues/"+id+"/lifecycle-handoffs", body), "id", id)).Want(409)
			testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{"title": "Must not attach", "parent_issue_id": id})).Want(409)
			source := lifecycleAtomicSource(t, "Formal source for blocked target")
			child := dbfx.Issue(t, "Nonformal followup", testutil.Cols{"parent_issue_id": source, "admission_status": state, "assignee_type": "agent", "assignee_id": agent})
			body["follow_up_issue_id"] = child
			delete(body, "follow_up_title")
			testutil.Call(t, testHandler.CreateLifecycleHandoff, withURLParam(newRequest("POST", "/api/issues/"+source+"/lifecycle-handoffs", body), "id", source)).Want(409)
			if n := dbfx.Count(t, "SELECT count(*) FROM comment WHERE issue_id=$1", source); n != 0 {
				t.Fatal("rejected lifecycle changed source history")
			}
		})
	}
}

type triageAfterReadDB struct {
	db.DBTX
	once  sync.Once
	after func()
}
type triageAfterReadRow struct {
	pgx.Row
	after func()
}

func (r triageAfterReadRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	if err == nil {
		r.after()
	}
	return err
}
func (d *triageAfterReadDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := d.DBTX.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "-- name: GetIssueInWorkspace ") {
		return triageAfterReadRow{Row: row, after: func() { d.once.Do(d.after) }}
	}
	return row
}
func TestTriageBoundaryBatchContentCannotRestorePreAcceptanceFields(t *testing.T) {
	id := dbfx.Issue(t, "Stale batch text", testutil.Cols{"admission_status": "pending", "status": "backlog"})
	wrapper := &triageAfterReadDB{DBTX: testPool, after: func() {
		dbfx.Exec(t, "UPDATE issue SET admission_status='accepted',assignee_type='member',assignee_id=$2,status='todo' WHERE id=$1", id, testUserID)
	}}
	h := *testHandler
	h.Queries = db.New(wrapper)
	testutil.Call(t, h.BatchUpdateIssues, newRequest("POST", "/api/issues/batch?workspace_id="+testWorkspaceID, map[string]any{"issue_ids": []string{id}, "updates": map[string]any{"title": "New content"}})).Want(200)
	current, err := testHandler.Queries.GetIssue(t.Context(), parseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	if current.AssigneeID != parseUUID(testUserID) || current.AssigneeType.String != "member" || current.AdmissionStatus != "accepted" {
		t.Fatalf("stale batch overwrote acceptance: %+v", current)
	}
}
