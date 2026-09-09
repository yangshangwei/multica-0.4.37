package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// Autonomy enforcement on agent-actor requests.
//
// Every case here drives a handler as an AGENT — X-Agent-ID plus the X-Task-ID of a
// running task, the pair resolveActor requires — because that is the only shape the
// policy applies to. Human requests are untouched, and the last test in this file is
// the regression guard for the agents that existed before this feature.

// autonomyTestAgent creates an agent at a declared level and returns (agentID,
// taskID) ready to be sent as an actor.
func autonomyTestAgent(t *testing.T, name, level string) (string, string) {
	t.Helper()
	agentID := dbfx.Agent(t, name, handlerTestRuntimeID(t), testutil.Cols{
		"visibility":      "workspace",
		"permission_mode": "public_to",
		"autonomy_level":  level,
	})
	return agentID, createHandlerTestTaskForAgent(t, agentID)
}

// asAgent stamps the actor headers onto a request. Deliberately the same headers the
// auth middleware sets from a verified task token, so the test exercises the same
// resolveActor path production does.
func asAgent(req *http.Request, agentID, taskID string) *http.Request {
	req.Header.Set("X-Agent-ID", agentID)
	req.Header.Set("X-Task-ID", taskID)
	return req
}

func TestUpdateIssue_ObserverAgentCannotChangeStatus(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer", "observer")
	issueID := dbfx.Issue(t, "Observer status attempt")

	req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"status": "in_progress"}), agentID, taskID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(req, "id", issueID)).
		Want(http.StatusForbidden)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status)
	if status == "in_progress" {
		t.Error("the status changed despite the 403; the gate must run before the write")
	}
}

func TestUpdateIssue_ObserverAgentCannotChangeAssignee(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Assign", "observer")
	other := createHandlerTestAgent(t, "Autonomy Reassign Target", nil)
	issueID := dbfx.Issue(t, "Observer assignee attempt")

	req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"assignee_type": "agent",
		"assignee_id":   other,
	}), agentID, taskID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(req, "id", issueID)).
		Want(http.StatusForbidden)
}

// TestUpdateIssue_ObserverAgentMayStillEditText separates "cannot move work" from
// "cannot write anything". An Observer's whole job is analysis, and a title or
// description correction is analysis.
func TestUpdateIssue_ObserverAgentMayStillEditText(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Text", "observer")
	issueID := dbfx.Issue(t, "Observer text edit")

	req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"description": "Clarified by the analyst.",
	}), agentID, taskID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(req, "id", issueID)).
		Want(http.StatusOK)
}

func TestUpdateIssue_ContributorAgentMayChangeStatus(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Contributor", "contributor")
	issueID := dbfx.Issue(t, "Contributor status change")

	req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"status": "in_progress"}), agentID, taskID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(req, "id", issueID)).
		Want(http.StatusOK)
}

// TestUpdateIssue_UndeclaredAgentIsUnaffected is the compatibility guarantee. Every
// agent created before role templates has an empty autonomy_level, and this feature
// must not change what any of them can do.
func TestUpdateIssue_UndeclaredAgentIsUnaffected(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Undeclared", "")
	issueID := dbfx.Issue(t, "Undeclared status change")

	req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"status": "in_progress"}), agentID, taskID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(req, "id", issueID)).
		Want(http.StatusOK)
}

// TestUpdateIssue_HumanIsUnaffectedByAgentPolicy makes explicit that the policy is
// about agents, not about issues: a person moving the same issue is never gated.
func TestUpdateIssue_HumanIsUnaffectedByAgentPolicy(t *testing.T) {
	issueID := dbfx.Issue(t, "Human status change")
	testutil.Call(t, testHandler.UpdateIssue,
		withURLParam(newRequest("PUT", "/api/issues/"+issueID, map[string]any{"status": "in_progress"}), "id", issueID)).
		Want(http.StatusOK)
}

// TestUpdateIssue_ObserverAgentCannotUnassignWithExplicitNull covers the shape a
// pointer-based gate misses: `{"assignee_id": null}` decodes to a nil pointer, so
// the first version of this check waved it through while the write path below read
// the same null out of rawFields and unassigned the issue.
func TestUpdateIssue_ObserverAgentCannotUnassignWithExplicitNull(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Unassign", "observer")
	assignee := createHandlerTestAgent(t, "Autonomy Unassign Target", nil)
	issueID := dbfx.Issue(t, "Observer unassign attempt", testutil.Cols{
		"assignee_type": "agent",
		"assignee_id":   assignee,
	})

	req := asAgent(newRequest("PUT", "/api/issues/"+issueID, map[string]any{
		"assignee_type": nil,
		"assignee_id":   nil,
	}), agentID, taskID)
	testutil.Call(t, testHandler.UpdateIssue, withURLParam(req, "id", issueID)).
		Want(http.StatusForbidden)

	var stillAssigned string
	dbfx.QueryRow(t, `SELECT COALESCE(assignee_id::text, '') FROM issue WHERE id = $1`, issueID).Scan(&stillAssigned)
	if stillAssigned != assignee {
		t.Errorf("assignee is %q, want %q: the 403 must land before the write", stillAssigned, assignee)
	}
}

// The batch endpoint writes the same fields as UpdateIssue, so it answers to the
// same rule. A batch of one is the shortest path around a gate that only guards the
// single-issue route, and it is the shape these tests exist to keep closed.

func TestBatchUpdateIssues_ObserverAgentCannotChangeStatus(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Batch Status", "observer")
	issueID := dbfx.Issue(t, "Observer batch status attempt")

	req := asAgent(newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates":   map[string]any{"status": "in_progress"},
	}), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusForbidden)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status)
	if status == "in_progress" {
		t.Error("the status changed despite the 403; the gate must run before the write")
	}
}

func TestBatchUpdateIssues_ObserverAgentCannotChangeAssignee(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Batch Assign", "observer")
	other := createHandlerTestAgent(t, "Autonomy Batch Reassign Target", nil)
	issueID := dbfx.Issue(t, "Observer batch assignee attempt")

	req := asAgent(newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates": map[string]any{
			"assignee_type": "agent",
			"assignee_id":   other,
		},
	}), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusForbidden)

	var assigned string
	dbfx.QueryRow(t, `SELECT COALESCE(assignee_id::text, '') FROM issue WHERE id = $1`, issueID).Scan(&assigned)
	if assigned == other {
		t.Error("the assignee changed despite the 403")
	}
}

// Explicit nulls again, through the batch route: the presence check has to look at
// the nested `updates` object, which is where this endpoint keeps its raw fields.
func TestBatchUpdateIssues_ObserverAgentCannotUnassignWithExplicitNull(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Batch Null", "observer")
	assignee := createHandlerTestAgent(t, "Autonomy Batch Unassign Target", nil)
	issueID := dbfx.Issue(t, "Observer batch unassign attempt", testutil.Cols{
		"assignee_type": "agent",
		"assignee_id":   assignee,
	})

	req := asAgent(newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates":   map[string]any{"assignee_type": nil, "assignee_id": nil},
	}), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusForbidden)

	var stillAssigned string
	dbfx.QueryRow(t, `SELECT COALESCE(assignee_id::text, '') FROM issue WHERE id = $1`, issueID).Scan(&stillAssigned)
	if stillAssigned != assignee {
		t.Errorf("assignee is %q, want %q", stillAssigned, assignee)
	}
}

// The batch route's half of "cannot move work is not cannot write anything".
func TestBatchUpdateIssues_ObserverAgentMayStillEditText(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Batch Text", "observer")
	issueID := dbfx.Issue(t, "Observer batch text edit")

	req := asAgent(newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates":   map[string]any{"description": "Clarified by the analyst."},
	}), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusOK)
}

func TestBatchUpdateIssues_ContributorAgentMayChangeStatus(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Contributor Batch", "contributor")
	issueID := dbfx.Issue(t, "Contributor batch status change")

	req := asAgent(newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates":   map[string]any{"status": "in_progress"},
	}), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusOK)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&status)
	if status != "in_progress" {
		t.Errorf("status is %q, want in_progress", status)
	}
}

// The compatibility guarantee, on the endpoint the first version left open: an
// agent with no declared level is not newly restricted by closing that gap.
func TestBatchUpdateIssues_UndeclaredAgentIsUnaffected(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Undeclared Batch", "")
	issueID := dbfx.Issue(t, "Undeclared batch status change")

	req := asAgent(newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates":   map[string]any{"status": "in_progress"},
	}), agentID, taskID)
	testutil.Call(t, testHandler.BatchUpdateIssues, req).Want(http.StatusOK)
}

// A person is never gated, on this endpoint either.
func TestBatchUpdateIssues_HumanIsUnaffectedByAgentPolicy(t *testing.T) {
	issueID := dbfx.Issue(t, "Human batch status change")
	testutil.Call(t, testHandler.BatchUpdateIssues, newRequest("POST", "/api/issues/batch-update", map[string]any{
		"issue_ids": []string{issueID},
		"updates":   map[string]any{"status": "in_progress"},
	})).Want(http.StatusOK)
}

func TestCreateIssue_ObserverAgentCannotCreate(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Observer Create", "observer")

	before := dbfx.Count(t, `SELECT COUNT(*) FROM issue WHERE workspace_id = $1`, testWorkspaceID)
	testutil.Call(t, testHandler.CreateIssue,
		asAgent(newRequest("POST", "/api/issues", map[string]any{"title": "Filed by an observer"}), agentID, taskID)).
		Want(http.StatusForbidden)
	if after := dbfx.Count(t, `SELECT COUNT(*) FROM issue WHERE workspace_id = $1`, testWorkspaceID); after != before {
		t.Errorf("issue count went %d -> %d despite the 403", before, after)
	}
}

func TestCreateSquad_ContributorAgentCannotCreate(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Contributor Squad", "contributor")
	leader := createHandlerTestAgent(t, "Autonomy Squad Leader", nil)

	req := asAgent(newRequest("POST", "/api/squads", map[string]any{
		"name":      "Contributor squad attempt",
		"leader_id": leader,
	}), agentID, taskID)
	testutil.Call(t, testHandler.CreateSquad, withURLParam(req, "workspaceId", testWorkspaceID)).
		Want(http.StatusForbidden)
}

// TestCreateSquadFromTemplate_CoordinatorCannotStaffAnOperatorRoster covers the
// ceiling a squad template raises above its own leader: staffing mints every agent in
// the roster, so the level that has to be granted is the roster's highest, not the
// leader's. Release and Incident seat a Release Engineer (operator), which a
// coordinator may not create — the same rule that stops it minting a lone operator
// through POST /api/agents/from-template.
func TestCreateSquadFromTemplate_CoordinatorCannotStaffAnOperatorRoster(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Coordinator Staffing", "coordinator")

	agentsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID)
	squadsBefore := dbfx.Count(t, `SELECT COUNT(*) FROM squad WHERE workspace_id = $1`, testWorkspaceID)

	for _, templateKey := range []string{"release", "incident"} {
		t.Run(templateKey, func(t *testing.T) {
			req := asAgent(newRequest("POST", "/api/squads/from-template", map[string]any{
				"template_key": templateKey,
				"runtime_id":   handlerTestRuntimeID(t),
			}), agentID, taskID)
			testutil.Call(t, testHandler.CreateSquadFromTemplate, withURLParam(req, "workspaceId", testWorkspaceID)).
				Want(http.StatusForbidden)
		})
	}

	// The refusal happens before anything is written, so no half-staffed roster is
	// left behind for the caller to discover later.
	if got := dbfx.Count(t, `SELECT COUNT(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID); got != agentsBefore {
		t.Errorf("agents after refused staffing = %d, want %d", got, agentsBefore)
	}
	if got := dbfx.Count(t, `SELECT COUNT(*) FROM squad WHERE workspace_id = $1`, testWorkspaceID); got != squadsBefore {
		t.Errorf("squads after refused staffing = %d, want %d", got, squadsBefore)
	}
}

// TestCreateSquadFromTemplate_CoordinatorMayStaffAContributorRoster is the other half
// of the ceiling: the refusal above must come from the roster's operator seat, not
// from squad staffing being coordinator-only. Review Gate tops out at contributor.
func TestCreateSquadFromTemplate_CoordinatorMayStaffAContributorRoster(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Coordinator Review Gate", "coordinator")

	var out CreateSquadFromTemplateResponse
	req := asAgent(newRequest("POST", "/api/squads/from-template", map[string]any{
		"template_key": "review-gate",
		"runtime_id":   handlerTestRuntimeID(t),
	}), agentID, taskID)
	testutil.Call(t, testHandler.CreateSquadFromTemplate, withURLParam(req, "workspaceId", testWorkspaceID)).
		Want(http.StatusCreated).JSON(&out)
	cleanupStaffedSquad(t, out.Squad.ID, append(append([]string{}, out.CreatedAgents...), out.ReusedAgents...))
}

func TestCreateAutopilot_ContributorAgentCannotCreate(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Contributor Autopilot", "contributor")
	target := createHandlerTestAgent(t, "Autonomy Autopilot Target", nil)

	testutil.Call(t, testHandler.CreateAutopilot,
		asAgent(newRequest("POST", "/api/autopilots", map[string]any{
			"title":          "Contributor automation attempt",
			"assignee_type":  "agent",
			"assignee_id":    target,
			"execution_mode": "run_only",
			"prompt":         "check dependencies",
		}), agentID, taskID)).
		Want(http.StatusForbidden)
}

// TestUpdateAgent_AgentActorCannotRaiseItsOwnAutonomy closes the obvious escalation:
// an agent's task token carries its owner's user id, so without this check
// canManageAgent alone would let it PATCH itself to operator and pass every later
// gate.
func TestUpdateAgent_AgentActorCannotRaiseItsOwnAutonomy(t *testing.T) {
	agentID, taskID := autonomyTestAgent(t, "Autonomy Self Raiser", "observer")

	req := asAgent(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"autonomy_level": "operator"}), agentID, taskID)
	// The header the auth middleware stamps for a task token, which is what
	// isMachineCredentialActor keys off.
	req.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(req, "id", agentID)).
		Want(http.StatusForbidden)

	var level string
	dbfx.QueryRow(t, `SELECT autonomy_level FROM agent WHERE id = $1`, agentID).Scan(&level)
	if level != "observer" {
		t.Errorf("autonomy_level = %q after the rejected PUT, want observer", level)
	}
}

func TestUpdateAgent_HumanCanSetAndClearAutonomy(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Autonomy Editable", nil)

	var set AgentResponse
	testutil.Call(t, testHandler.UpdateAgent,
		withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"autonomy_level": "coordinator"}), "id", agentID)).
		Want(http.StatusOK).JSON(&set)
	if set.AutonomyLevel != "coordinator" {
		t.Fatalf("autonomy_level = %q, want coordinator", set.AutonomyLevel)
	}

	// The empty string is the documented way to clear the policy back to "none
	// declared" — it is not NULL, so the query's COALESCE overwrites with it. Decoded
	// into a fresh struct because the field is `omitempty`: a cleared value is absent
	// from the payload, and reusing the previous struct would keep reading the old one.
	var cleared AgentResponse
	testutil.Call(t, testHandler.UpdateAgent,
		withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"autonomy_level": ""}), "id", agentID)).
		Want(http.StatusOK).JSON(&cleared)
	if cleared.AutonomyLevel != "" {
		t.Errorf("autonomy_level = %q after clearing, want empty", cleared.AutonomyLevel)
	}
	var stored string
	dbfx.QueryRow(t, `SELECT autonomy_level FROM agent WHERE id = $1`, agentID).Scan(&stored)
	if stored != "" {
		t.Errorf("stored autonomy_level = %q after clearing, want empty", stored)
	}

	// Omitting the field leaves the value alone.
	testutil.Call(t, testHandler.UpdateAgent,
		withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"autonomy_level": "operator"}), "id", agentID)).
		Want(http.StatusOK)
	var preserved AgentResponse
	testutil.Call(t, testHandler.UpdateAgent,
		withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"description": "unchanged policy"}), "id", agentID)).
		Want(http.StatusOK).JSON(&preserved)
	if preserved.AutonomyLevel != "operator" {
		t.Errorf("autonomy_level = %q after an unrelated update, want operator preserved", preserved.AutonomyLevel)
	}
}

func TestUpdateAgent_RejectsUnknownAutonomyLevel(t *testing.T) {
	agentID := createHandlerTestAgent(t, "Autonomy Invalid Level", nil)
	testutil.Call(t, testHandler.UpdateAgent,
		withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"autonomy_level": "supervisor"}), "id", agentID)).
		Want(http.StatusBadRequest)
}
