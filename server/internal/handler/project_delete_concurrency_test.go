package handler

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
	"strings"
	"testing"
)

func TestProjectDeletePausesAutomationAndOwnsProgress(t *testing.T) {
	project := dbfx.Project(t, "P1 delete")
	runtime := dbfx.Runtime(t, "P1 runtime")
	agent := dbfx.Agent(t, "P1 delete agent", runtime)
	issue := dbfx.Issue(t, "P1 retained issue", testutil.Cols{"project_id": project, "status": "in_progress"})
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "status": "running", "runtime_id": runtime})
	chat := dbfx.ChatSession(t, agent, testutil.Cols{"project_id": project})
	update := dbfx.Insert(t, "project_update", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "author_user_id": testUserID})
	dbfx.Insert(t, "project_update_revision", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "update_id": update, "revision": 1, "editor_user_id": testUserID, "kind": "progress", "body": "retained until project deletion"})
	dbfx.Insert(t, "project_update_notification", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "update_id": update, "recipient_user_id": testUserID, "source_revision": 1})
	autos := map[string]string{}
	for _, status := range []string{"active", "paused", "archived"} {
		id := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "title": "delete " + status, "assignee_id": agent, "status": status, "created_by_type": "member", "created_by_id": testUserID})
		dbfx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": id, "kind": "api", "enabled": true})
		autos[status] = id
	}
	testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+project, nil), "id", project)).Want(204)
	for before, id := range autos {
		var status string
		var reason, projectID *string
		dbfx.QueryRow(t, "SELECT status,pause_reason,project_id::text FROM autopilot WHERE id=$1", id).Scan(&status, &reason, &projectID)
		if projectID != nil || (before == "archived" && status != "archived") || (before != "archived" && (status != "paused" || reason == nil || *reason != "project_deleted")) {
			t.Fatalf("automation %s became %s reason=%v project=%v", before, status, reason, projectID)
		}
		if n := dbfx.Count(t, "SELECT count(*) FROM autopilot_trigger WHERE autopilot_id=$1 AND enabled", id); n != 0 {
			t.Fatalf("%d triggers stayed enabled", n)
		}
	}
	for _, table := range []string{"project_update", "project_update_revision", "project_update_notification"} {
		if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE project_id=$1", project); n != 0 {
			t.Fatalf("%s retains %d rows", table, n)
		}
	}
	var state string
	var projectID *string
	dbfx.QueryRow(t, "SELECT status,project_id::text FROM issue WHERE id=$1", issue).Scan(&state, &projectID)
	if state != "in_progress" || projectID != nil {
		t.Fatalf("issue lost or mutated: %s %v", state, projectID)
	}
	dbfx.QueryRow(t, "SELECT status FROM agent_task_queue WHERE id=$1", task).Scan(&state)
	if state != "running" {
		t.Fatalf("execution stopped: %s", state)
	}
	dbfx.QueryRow(t, "SELECT project_id::text FROM chat_session WHERE id=$1", chat).Scan(&projectID)
	if projectID != nil {
		t.Fatalf("chat still bound: %v", projectID)
	}
}

// Inject a failure through the actual generated delete statement, after all
// preceding work has happened in the same transaction.
type projectDeleteFailStarter struct {
	txStarter
	step string
}
type projectDeleteFailTx struct {
	pgx.Tx
	step string
}

func (s projectDeleteFailStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return projectDeleteFailTx{Tx: tx, step: s.step}, nil
}
func (tx projectDeleteFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: "+tx.step+" ") {
		return pgconn.CommandTag{}, errors.New("injected project cleanup failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestProjectDeleteFailureRollsBackEveryCleanupStage(t *testing.T) {
	for _, step := range []string{"DisableProjectAutopilotTriggers", "DetachProjectAutopilots", "DetachProjectIssues", "ClearChatSessionProjectByProject", "DeleteIssueViewsByProjectScope", "DeleteProjectIssueViewPreferences", "DeleteProjectResources", "DeleteProjectProgressInbox", "DeleteProjectProgress", "DeleteProject"} {
		t.Run(step, func(t *testing.T) {
			project := dbfx.Project(t, "P1 rollback")
			issue := dbfx.Issue(t, "P1 retained", testutil.Cols{"project_id": project})
			agent := dbfx.Agent(t, "P1 rollback agent", "")
			ap := dbfx.Insert(t, "autopilot", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "title": "rollback", "assignee_id": agent, "created_by_type": "member", "created_by_id": testUserID})
			trigger := dbfx.Insert(t, "autopilot_trigger", testutil.Cols{"autopilot_id": ap, "kind": "api", "enabled": true})
			update := dbfx.Insert(t, "project_update", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "author_user_id": testUserID})
			dbfx.InsertNoID(t, "issue_view_preference", testutil.Cols{"workspace_id": testWorkspaceID, "user_id": testUserID, "scope_type": "project", "scope_id": project, "prefs": "{}"}, "workspace_id=$1 AND user_id=$2 AND scope_type='project' AND scope_id=$3", testWorkspaceID, testUserID, project)
			h := *testHandler
			h.TxStarter = projectDeleteFailStarter{txStarter: h.TxStarter, step: step}
			testutil.Call(t, h.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+project, nil), "id", project)).Want(503)
			if n := dbfx.Count(t, "SELECT count(*) FROM project WHERE id=$1", project); n != 1 {
				t.Fatal("project escaped rollback")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM issue WHERE id=$1 AND project_id=$2", issue, project); n != 1 {
				t.Fatal("issue escaped rollback")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM autopilot WHERE id=$1 AND project_id=$2 AND status='active' AND pause_reason IS NULL", ap, project); n != 1 {
				t.Fatal("autopilot escaped rollback")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM autopilot_trigger WHERE id=$1 AND enabled", trigger); n != 1 {
				t.Fatal("trigger escaped rollback")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM project_update WHERE id=$1", update); n != 1 {
				t.Fatal("progress escaped rollback")
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM issue_view_preference WHERE workspace_id=$1 AND user_id=$2 AND scope_type='project' AND scope_id=$3", testWorkspaceID, testUserID, project); n != 1 {
				t.Fatal("project preferences escaped rollback")
			}
		})
	}
}

func TestProjectDeleteImpactFormalAdmissionAndRevision(t *testing.T) {
	project := dbfx.Project(t, "P1 impact")
	dbfx.Issue(t, "formal", testutil.Cols{"project_id": project})
	dbfx.Issue(t, "pending", testutil.Cols{"project_id": project, "admission_status": "pending"})
	var out ProjectDeleteImpact
	testutil.Call(t, testHandler.GetProjectDeleteImpact, withURLParam(newRequest("GET", "/api/projects/"+project+"/delete-impact", nil), "id", project)).Want(200).JSON(&out)
	if out.IssueCount != 2 || out.FormalIssueCount != 1 || out.ProjectRevision != 1 || !out.PreservesIssues || !out.PreservesExecutions {
		t.Fatalf("impact: %+v", out)
	}
	testutil.Call(t, testHandler.DeleteProject, withURLParam(newRequest("DELETE", "/api/projects/"+project, map[string]any{"expected_revision": 2}), "id", project)).Want(409)
	if n := dbfx.Count(t, "SELECT count(*) FROM project WHERE id=$1", project); n != 1 {
		t.Fatal("revision conflict deleted project")
	}
	req := withURLParam(newRequest("DELETE", "/api/projects/"+project, nil), "id", project)
	req.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.DeleteProject, req).Want(403)
}
