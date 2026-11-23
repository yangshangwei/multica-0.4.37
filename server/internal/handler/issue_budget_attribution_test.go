package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIssueBudgetCreateUsesVerifiedExecutionIdentity(t *testing.T) {
	parent := dbfx.Issue(t, "Quick-create root")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	otherParent := dbfx.Issue(t, "Self-reported root")
	otherRoot := enableIssueBudget(t, testWorkspaceID, otherParent)
	runtime := handlerTestRuntimeID(t)
	agent := dbfx.Agent(t, "Budget quick creator", runtime, testutil.Cols{"autonomy_level": "contributor"})
	makeTask := func(parent, root string) string {
		payload, err := json.Marshal(service.QuickCreateContext{Type: service.QuickCreateContextType, WorkspaceID: testWorkspaceID, RequesterID: testUserID, ParentIssueID: parent, Prompt: "create an issue"})
		if err != nil {
			t.Fatal(err)
		}
		task := dbfx.Task(t, agent, testutil.Cols{"status": "running", "runtime_id": runtime, "context": string(payload), "claim_generation": 1})
		dbfx.Insert(t, "issue_budget_attempt", testutil.Cols{
			"id": uuid.NewString(), "workspace_id": testWorkspaceID, "budget_root_id": root, "task_id": task,
			"claim_generation": 1, "runtime_id": runtime, "agent_id": agent, "context_parent_issue_id": parent,
		})
		return task
	}
	actualTask, originTask := makeTask(parent, root), makeTask(otherParent, otherRoot)
	// Both tasks belong to the same agent, so the ordinary origin validation
	// accepts the supplied provenance. Budget attribution must still choose
	// the task authenticated by the server.
	for _, verified := range []bool{false, true} {
		body := map[string]any{"title": uuid.NewString(), "origin_type": "quick_create", "origin_id": originTask, "verified_task_id": originTask}
		req := asAgent(newRequest(http.MethodPost, "/api/issues", body), agent, actualTask)
		if verified {
			token := db.TaskToken{ID: parseUUID(uuid.NewString()), TaskID: parseUUID(actualTask), AgentID: parseUUID(agent), WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(testUserID)}
			req = req.WithContext(auth.WithVerifiedTaskToken(req.Context(), token))
		}
		dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
		var created IssueResponse
		testutil.Call(t, testHandler.CreateIssue, req).Want(http.StatusCreated).JSON(&created)
		dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, created.ID)
		want := 0
		if verified {
			want = 1
		}
		if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE issue_id=$1 AND budget_root_id=$2 AND task_id=$3 AND kind='attribution_difference'`, created.ID, root, actualTask); n != want {
			t.Fatalf("verified=%v: got %d actual-task differences, want %d", verified, n, want)
		}
		if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE issue_id=$1 AND budget_root_id=$2`, created.ID, otherRoot); n != 0 {
			t.Fatal("caller origin selected budget root")
		}
	}
}

func TestIssueBudgetVerifiedCreationTaskIsWorkspaceScoped(t *testing.T) {
	workspace := parseUUID(uuid.NewString())
	task := parseUUID(uuid.NewString())
	ctx := auth.WithVerifiedTaskToken(context.Background(), db.TaskToken{ID: parseUUID(uuid.NewString()), WorkspaceID: workspace, TaskID: task})
	if got := verifiedIssueCreationTaskID(ctx, workspace); got != task {
		t.Fatal("verified identity lost")
	}
	if got := verifiedIssueCreationTaskID(ctx, parseUUID(uuid.NewString())); got.Valid {
		t.Fatal("foreign workspace task accepted")
	}
	if got := verifiedIssueCreationTaskID(context.Background(), workspace); got.Valid {
		t.Fatal("missing verification supplied a task")
	}
}

func TestIssueBudgetLifecycleFollowUpUsesVerifiedExecutionIdentity(t *testing.T) {
	parent := dbfx.Issue(t, "Execution budget root")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	source := lifecycleAtomicSource(t, "Follow-up destination root")
	destination := enableIssueBudget(t, testWorkspaceID, source)
	creator := dbfx.Agent(t, "Quick-create lifecycle caller", testRuntimeID, testutil.Cols{"autonomy_level": "contributor"})
	target := dbfx.Agent(t, "Follow-up assignee", testRuntimeID)
	payload, err := json.Marshal(service.QuickCreateContext{Type: service.QuickCreateContextType, WorkspaceID: testWorkspaceID, RequesterID: testUserID, ParentIssueID: parent, Prompt: "create follow-up"})
	if err != nil {
		t.Fatal(err)
	}
	acting := dbfx.Task(t, creator, testutil.Cols{"runtime_id": testRuntimeID, "status": "running", "context": string(payload), "claim_generation": 1, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	attempt := dbfx.Insert(t, "issue_budget_attempt", testutil.Cols{"id": uuid.NewString(), "workspace_id": testWorkspaceID, "budget_root_id": root, "task_id": acting, "claim_generation": 1, "runtime_id": testRuntimeID, "agent_id": creator, "context_parent_issue_id": parent})
	req := withURLParam(asAgent(newRequest(http.MethodPost, "/api/issues/"+source+"/lifecycle-handoffs", map[string]any{
		"kind": "rca", "route": "maintenance", "cause_state": "unknown", "follow_up_title": "Different-root follow-up", "assignee_type": "agent", "assignee_id": target,
	}), creator, acting), "id", source)
	// Custom task credentials have no restricted capability and may use this
	// ingress; the verified token supplies execution identity to the creator.
	token := db.TaskToken{ID: parseUUID(uuid.NewString()), TaskID: parseUUID(acting), AgentID: parseUUID(creator), WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(testUserID)}
	req = req.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{Kind: auth.CredentialTask, UserID: testUserID}), token))
	before := issueBudgetRootRows(t, root)
	var created lifecycleHandoffResponse
	testutil.Call(t, testHandler.CreateLifecycleHandoff, req).Want(http.StatusCreated).JSON(&created)
	if !created.FollowUpCreated || created.FollowUpIssueID == "" {
		t.Fatalf("missing child: %+v", created)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE issue_id=$1 AND budget_root_id=$2 AND bound_parent_issue_id=$3`, created.FollowUpIssueID, destination, source); n != 1 {
		t.Fatal("follow-up did not bind to destination root")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE issue_id=$1 AND budget_root_id=$2 AND task_id=$3 AND attempt_id=$4 AND kind='attribution_difference'`, created.FollowUpIssueID, root, acting, attempt); n != 1 {
		t.Fatalf("follow-up has %d verified attribution observations", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE issue_id=$1 AND budget_root_id=$2`, created.FollowUpIssueID, destination); n != 0 {
		t.Fatal("follow-up charged destination root")
	}
	requireIssueBudgetRootRows(t, root, before, "lifecycle follow-up", "issue_budget_observation")
}

func TestIssueBudgetTriageCreationUsesVerifiedExecutionIdentity(t *testing.T) {
	triageEnableForTest(t)
	parent := dbfx.Issue(t, "Quick-create triage budget root")
	root := enableIssueBudget(t, testWorkspaceID, parent)
	agent := dbfx.Agent(t, "Budget triage creator", testRuntimeID, testutil.Cols{"autonomy_level": "contributor"})
	payload, err := json.Marshal(service.QuickCreateContext{Type: service.QuickCreateContextType, WorkspaceID: testWorkspaceID, RequesterID: testUserID, ParentIssueID: parent, Prompt: "file triage item"})
	if err != nil {
		t.Fatal(err)
	}
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": testRuntimeID, "status": "running", "context": string(payload), "claim_generation": 1, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	attempt := dbfx.Insert(t, "issue_budget_attempt", testutil.Cols{"id": uuid.NewString(), "workspace_id": testWorkspaceID, "budget_root_id": root, "task_id": task, "claim_generation": 1, "runtime_id": testRuntimeID, "agent_id": agent, "context_parent_issue_id": parent})
	req := asAgent(newRequest(http.MethodPost, "/api/triage/items", triageInputForTest()), agent, task)
	token := db.TaskToken{ID: parseUUID(uuid.NewString()), TaskID: parseUUID(task), AgentID: parseUUID(agent), WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(testUserID)}
	req = req.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{Kind: auth.CredentialTask, UserID: testUserID}), token))
	dbfx.Exec(t, `UPDATE workspace SET issue_counter=(SELECT COALESCE(MAX(number),0) FROM issue WHERE workspace_id=$1) WHERE id=$1`, testWorkspaceID)
	before := issueBudgetRootRows(t, root)
	var item TriageItem
	testutil.Call(t, testHandler.CreateTriageItem, req).Want(http.StatusCreated).JSON(&item)
	dbfx.Cleanup(t, `DELETE FROM issue_triage WHERE issue_id=$1`, item.Issue.ID)
	dbfx.Cleanup(t, `DELETE FROM issue WHERE id=$1`, item.Issue.ID)
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_member WHERE issue_id=$1`, item.Issue.ID); n != 0 {
		t.Fatal("parentless triage issue unexpectedly bound")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue_budget_observation WHERE issue_id=$1 AND budget_root_id=$2 AND task_id=$3 AND attempt_id=$4 AND kind='attribution_difference'`, item.Issue.ID, root, task, attempt); n != 1 {
		t.Fatalf("triage item has %d verified attribution observations", n)
	}
	requireIssueBudgetRootRows(t, root, before, "triage creation", "issue_budget_observation")
}
