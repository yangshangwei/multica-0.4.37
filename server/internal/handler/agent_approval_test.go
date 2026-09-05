package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// The human approval boundary, end to end.
//
// The property worth testing is narrow and important: no agent, by any route, can
// produce the record that authorizes its own high-risk action, and nothing can be
// marked executed without that record.

// approvalTestAgent creates an agent at a level and returns (agentID, taskID).
func approvalTestAgent(t *testing.T, name, level string) (string, string) {
	t.Helper()
	agentID := dbfx.Agent(t, name, handlerTestRuntimeID(t), testutil.Cols{
		"visibility":      "workspace",
		"permission_mode": "public_to",
		"autonomy_level":  level,
	})
	return agentID, createHandlerTestTaskForAgent(t, agentID)
}

// fileApproval has an agent file a request and returns it.
func fileApproval(t *testing.T, agentID, taskID, riskClass string) AgentApprovalResponse {
	t.Helper()
	var out AgentApprovalResponse
	req := asAgent(newRequest("POST", "/api/agent-approvals", map[string]any{
		"risk_class": riskClass,
		"summary":    "Deploy v1.2.3 to production",
		"plan":       "1. tag v1.2.3\n2. run deploy.sh\nRollback: redeploy v1.2.2",
	}), agentID, taskID)
	testutil.Call(t, testHandler.CreateAgentApproval, req).Want(http.StatusCreated).JSON(&out)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_approval_request WHERE id = $1`, out.ID)
	})
	return out
}

func TestCreateAgentApproval_FilesAPendingRequest(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Operator", "operator")
	approval := fileApproval(t, agentID, taskID, "production_release")

	if approval.Status != "pending" {
		t.Errorf("status = %q, want pending", approval.Status)
	}
	if approval.AgentID != agentID {
		t.Errorf("agent_id = %q, want %q", approval.AgentID, agentID)
	}
	if approval.TaskID == nil || *approval.TaskID != taskID {
		t.Errorf("task_id = %v, want the filing task %q", approval.TaskID, taskID)
	}
	if approval.DecidedBy != nil {
		t.Errorf("decided_by = %v on a fresh request, want nil", approval.DecidedBy)
	}
	// The workspace timeline is the discoverable half of the audit trail.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM activity_log WHERE workspace_id = $1 AND action = $2 AND details->>'approval_id' = $3`,
		testWorkspaceID, approvalActivityRequested, approval.ID); count != 1 {
		t.Errorf("%d activity rows for the filed request, want 1", count)
	}
}

// TestCreateAgentApproval_AgentIdentityComesFromTheToken pins that an agent cannot
// file in another agent's name and then collect its approval.
func TestCreateAgentApproval_AgentIdentityComesFromTheToken(t *testing.T) {
	filer, taskID := approvalTestAgent(t, "Approval Filer", "operator")
	victim := createHandlerTestAgent(t, "Approval Victim", nil)

	var out AgentApprovalResponse
	req := asAgent(newRequest("POST", "/api/agent-approvals", map[string]any{
		"agent_id":   victim,
		"risk_class": "secret_access",
		"summary":    "Read the deploy key",
	}), filer, taskID)
	testutil.Call(t, testHandler.CreateAgentApproval, req).Want(http.StatusCreated).JSON(&out)
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_approval_request WHERE id = $1`, out.ID)
	})

	if out.AgentID != filer {
		t.Errorf("agent_id = %q, want the acting agent %q — the body must not override token identity", out.AgentID, filer)
	}
}

func TestCreateAgentApproval_ValidatesRiskClassAndSummary(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Validation", "operator")

	testutil.Call(t, testHandler.CreateAgentApproval,
		asAgent(newRequest("POST", "/api/agent-approvals", map[string]any{
			"risk_class": "yolo",
			"summary":    "ship it",
		}), agentID, taskID)).Want(http.StatusBadRequest)

	testutil.Call(t, testHandler.CreateAgentApproval,
		asAgent(newRequest("POST", "/api/agent-approvals", map[string]any{
			"risk_class": "production_release",
			"summary":    "   ",
		}), agentID, taskID)).Want(http.StatusBadRequest)
}

// TestDecideAgentApproval_AgentCannotDecide is the whole point of the mechanism. An
// agent holding a task token must not be able to approve anything, including its own
// request, even though that token carries its owner's user id.
func TestDecideAgentApproval_AgentCannotDecide(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Self Approver", "operator")
	approval := fileApproval(t, agentID, taskID, "production_release")

	req := asAgent(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{
		"decision": "approve",
	}), agentID, taskID)
	req.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.DecideAgentApproval, withURLParam(req, "approvalId", approval.ID)).
		Want(http.StatusForbidden)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_approval_request WHERE id = $1`, approval.ID).Scan(&status)
	if status != "pending" {
		t.Errorf("status = %q after the rejected decision, want pending", status)
	}
}

func TestDecideAgentApproval_HumanApprovesThenExecutionIsRecorded(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Happy Operator", "operator")
	approval := fileApproval(t, agentID, taskID, "production_release")

	var approved AgentApprovalResponse
	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{
			"decision": "approve",
			"note":     "checked the rollback",
		}), "approvalId", approval.ID)).
		Want(http.StatusOK).JSON(&approved)

	if approved.Status != "approved" {
		t.Fatalf("status = %q, want approved", approved.Status)
	}
	if approved.DecidedBy == nil || *approved.DecidedBy != testUserID {
		t.Errorf("decided_by = %v, want the deciding human %q", approved.DecidedBy, testUserID)
	}
	if approved.DecidedAt == nil {
		t.Error("decided_at is nil on an approved request")
	}
	if approved.DecisionNote != "checked the rollback" {
		t.Errorf("decision_note = %q, want the reviewer's note", approved.DecisionNote)
	}

	var executed AgentApprovalResponse
	execReq := asAgent(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/execution", map[string]any{
		"note": "deployed; health checks green",
	}), agentID, taskID)
	testutil.Call(t, testHandler.RecordAgentApprovalExecution, withURLParam(execReq, "approvalId", approval.ID)).
		Want(http.StatusOK).JSON(&executed)
	if executed.Status != "executed" {
		t.Errorf("status = %q, want executed", executed.Status)
	}
	if executed.ExecutedAt == nil {
		t.Error("executed_at is nil after recording execution")
	}
	// Requested, decided, executed: three rows, which is what "traceable" means here.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM activity_log WHERE workspace_id = $1 AND details->>'approval_id' = $2`,
		testWorkspaceID, approval.ID); count != 3 {
		t.Errorf("%d activity rows across the request's life, want 3 (requested, decided, executed)", count)
	}
}

func TestDecideAgentApproval_SecondDecisionConflicts(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Double Decide", "operator")
	approval := fileApproval(t, agentID, taskID, "database_migration")

	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{"decision": "approve"}), "approvalId", approval.ID)).
		Want(http.StatusOK)
	// A second reviewer must not overwrite a decision already on the record.
	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{"decision": "reject"}), "approvalId", approval.ID)).
		Want(http.StatusConflict)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_approval_request WHERE id = $1`, approval.ID).Scan(&status)
	if status != "approved" {
		t.Errorf("status = %q, want the first decision to stand", status)
	}
}

func TestDecideAgentApproval_RejectsUnknownDecision(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Bad Decision", "operator")
	approval := fileApproval(t, agentID, taskID, "secret_access")

	// "executed" is a status, not a decision — a client must not be able to walk the
	// row forward through this endpoint.
	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{"decision": "executed"}), "approvalId", approval.ID)).
		Want(http.StatusBadRequest)
}

// TestRecordExecution_RequiresApproval covers "Operator without approval can only
// produce a plan": with the request still pending, there is nothing to execute.
func TestRecordExecution_RequiresApproval(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Unapproved Execute", "operator")
	approval := fileApproval(t, agentID, taskID, "production_release")

	req := asAgent(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/execution", map[string]any{}), agentID, taskID)
	testutil.Call(t, testHandler.RecordAgentApprovalExecution, withURLParam(req, "approvalId", approval.ID)).
		Want(http.StatusConflict)
}

func TestRecordExecution_RejectedRequestStaysRejected(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Rejected Execute", "operator")
	approval := fileApproval(t, agentID, taskID, "destructive_operation")

	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{
			"decision": "reject", "note": "not now",
		}), "approvalId", approval.ID)).Want(http.StatusOK)

	req := asAgent(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/execution", map[string]any{}), agentID, taskID)
	testutil.Call(t, testHandler.RecordAgentApprovalExecution, withURLParam(req, "approvalId", approval.ID)).
		Want(http.StatusConflict)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_approval_request WHERE id = $1`, approval.ID).Scan(&status)
	if status != "rejected" {
		t.Errorf("status = %q, want rejected", status)
	}
}

// TestRecordExecution_ContributorCannotExecute is "Contributor cannot operate
// production" in enforceable form: even holding a human's approval, an agent below
// Operator may not record a high-risk action.
func TestRecordExecution_ContributorCannotExecute(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Contributor Execute", "contributor")
	approval := fileApproval(t, agentID, taskID, "production_release")

	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{"decision": "approve"}), "approvalId", approval.ID)).
		Want(http.StatusOK)

	req := asAgent(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/execution", map[string]any{}), agentID, taskID)
	testutil.Call(t, testHandler.RecordAgentApprovalExecution, withURLParam(req, "approvalId", approval.ID)).
		Want(http.StatusForbidden)

	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_approval_request WHERE id = $1`, approval.ID).Scan(&status)
	if status != "approved" {
		t.Errorf("status = %q, want approved (unexecuted)", status)
	}
}

// TestGetAgentApproval_AgentSeesOnlyItsOwn keeps one agent from reading — or later
// executing against — another's approval.
func TestGetAgentApproval_AgentSeesOnlyItsOwn(t *testing.T) {
	ownerAgent, ownerTask := approvalTestAgent(t, "Approval Owner Agent", "operator")
	approval := fileApproval(t, ownerAgent, ownerTask, "production_release")
	otherAgent, otherTask := approvalTestAgent(t, "Approval Other Agent", "operator")

	req := asAgent(newRequest("GET", "/api/agent-approvals/"+approval.ID, nil), otherAgent, otherTask)
	testutil.Call(t, testHandler.GetAgentApproval, withURLParam(req, "approvalId", approval.ID)).
		Want(http.StatusForbidden)

	// Its own request is readable — that is how it learns the decision.
	own := asAgent(newRequest("GET", "/api/agent-approvals/"+approval.ID, nil), ownerAgent, ownerTask)
	testutil.Call(t, testHandler.GetAgentApproval, withURLParam(own, "approvalId", approval.ID)).
		Want(http.StatusOK)
}

// TestListAgentApprovals_AgentSeesOnlyItsOwnQueue keeps the review queue a human
// surface.
func TestListAgentApprovals_AgentSeesOnlyItsOwnQueue(t *testing.T) {
	first, firstTask := approvalTestAgent(t, "Approval Queue One", "operator")
	second, secondTask := approvalTestAgent(t, "Approval Queue Two", "operator")
	mine := fileApproval(t, first, firstTask, "production_release")
	theirs := fileApproval(t, second, secondTask, "secret_access")

	var out struct {
		Approvals []AgentApprovalResponse `json:"approvals"`
	}
	testutil.Call(t, testHandler.ListAgentApprovals,
		asAgent(newRequest("GET", "/api/agent-approvals?status=pending", nil), first, firstTask)).
		Want(http.StatusOK).JSON(&out)

	for _, approval := range out.Approvals {
		if approval.ID == theirs.ID {
			t.Error("an agent's queue included another agent's request")
		}
	}
	found := false
	for _, approval := range out.Approvals {
		if approval.ID == mine.ID {
			found = true
		}
	}
	if !found {
		t.Error("an agent's own pending request is missing from its queue")
	}

	// A person sees the whole workspace queue.
	var humanView struct {
		Approvals []AgentApprovalResponse `json:"approvals"`
	}
	testutil.Call(t, testHandler.ListAgentApprovals, newRequest("GET", "/api/agent-approvals?status=pending", nil)).
		Want(http.StatusOK).JSON(&humanView)
	seen := map[string]bool{}
	for _, approval := range humanView.Approvals {
		seen[approval.ID] = true
	}
	if !seen[mine.ID] || !seen[theirs.ID] {
		t.Errorf("human queue is missing requests: mine=%v theirs=%v", seen[mine.ID], seen[theirs.ID])
	}
}

func TestListAgentApprovals_RejectsUnknownStatusFilter(t *testing.T) {
	testutil.Call(t, testHandler.ListAgentApprovals, newRequest("GET", "/api/agent-approvals?status=maybe", nil)).
		Want(http.StatusBadRequest)
}

func TestCancelAgentApproval_ByTheFilingAgent(t *testing.T) {
	agentID, taskID := approvalTestAgent(t, "Approval Canceller", "operator")
	approval := fileApproval(t, agentID, taskID, "external_notification")

	var cancelled AgentApprovalResponse
	req := asAgent(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/cancel", nil), agentID, taskID)
	testutil.Call(t, testHandler.CancelAgentApproval, withURLParam(req, "approvalId", approval.ID)).
		Want(http.StatusOK).JSON(&cancelled)
	if cancelled.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", cancelled.Status)
	}

	// A cancelled request can no longer be decided — the plan it described is gone.
	testutil.Call(t, testHandler.DecideAgentApproval,
		withURLParam(newRequest("POST", "/api/agent-approvals/"+approval.ID+"/decision", map[string]any{"decision": "approve"}), "approvalId", approval.ID)).
		Want(http.StatusConflict)
}
