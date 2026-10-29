package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// POST /api/autopilots used to drop `status` on the floor: the request struct
// had no such field, so a caller that asked for a paused automation got an
// active one and could see it run before it had finished adding triggers.

// cleanupAutopilotsFor removes every autopilot the handler created for agentID.
// Rule versions go first because nothing cascades.
func cleanupAutopilotsFor(t *testing.T, agentID string) {
	t.Helper()
	dbfx.Cleanup(t, `DELETE FROM autopilot WHERE assignee_id = $1`, agentID)
	dbfx.Cleanup(t, `DELETE FROM autopilot_rule_version WHERE autopilot_id IN (SELECT id FROM autopilot WHERE assignee_id = $1)`, agentID)
}

func createAutopilotRequest(agentID string, extra map[string]any) *http.Request {
	body := map[string]any{
		"title":          "Create status autopilot",
		"assignee_id":    agentID,
		"execution_mode": "run_only",
	}
	for k, v := range extra {
		body[k] = v
	}
	return newRequest("POST", "/api/autopilots?workspace_id="+testWorkspaceID, body)
}

func TestCreateAutopilot_HonorsRequestedStatus(t *testing.T) {
	if testHandler == nil || dbfx == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "Create status runtime")
	for _, tc := range []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{"omitted", nil, "active"},
		{"null", map[string]any{"status": nil}, "active"},
		{"active", map[string]any{"status": "active"}, "active"},
		{"paused", map[string]any{"status": "paused"}, "paused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agentID := dbfx.Agent(t, "Create status agent "+tc.name, runtimeID)
			cleanupAutopilotsFor(t, agentID)

			var resp AutopilotResponse
			testutil.Call(t, testHandler.CreateAutopilot, createAutopilotRequest(agentID, tc.extra)).
				Want(http.StatusCreated).JSON(&resp)
			if resp.Status != tc.want {
				t.Fatalf("response status = %q, want %q", resp.Status, tc.want)
			}

			var status string
			var pauseReason *string
			dbfx.QueryRow(t, `SELECT status, pause_reason FROM autopilot WHERE id = $1`, resp.ID).Scan(&status, &pauseReason)
			if status != tc.want || pauseReason != nil {
				t.Fatalf("stored (status, pause_reason) = (%q, %v), want (%q, NULL)", status, pauseReason, tc.want)
			}
		})
	}
}

// A paused automation cannot dispatch, so it does not need a Runtime yet, the
// same rule UpdateAutopilot applies. TestCreateAutopilot_UnboundAgentRejected
// still pins the active case.
func TestCreateAutopilot_PausedAcceptsAgentWithoutRuntime(t *testing.T) {
	if testHandler == nil || dbfx == nil {
		t.Skip("database not available")
	}
	agentID := dbfx.Agent(t, "Create paused unbound agent", "")
	cleanupAutopilotsFor(t, agentID)

	var resp AutopilotResponse
	testutil.Call(t, testHandler.CreateAutopilot, createAutopilotRequest(agentID, map[string]any{"status": "paused"})).
		Want(http.StatusCreated).JSON(&resp)
	if resp.Status != "paused" {
		t.Fatalf("response status = %q, want paused", resp.Status)
	}
}

func TestCreateAutopilot_RejectsUnsupportedStatus(t *testing.T) {
	if testHandler == nil || dbfx == nil {
		t.Skip("database not available")
	}
	agentID := dbfx.Agent(t, "Create bad status agent", dbfx.Runtime(t, "Create bad status runtime"))
	cleanupAutopilotsFor(t, agentID)

	for _, status := range []string{"archived", "", "Paused", "deleted"} {
		t.Run(status, func(t *testing.T) {
			body := testutil.Call(t, testHandler.CreateAutopilot, createAutopilotRequest(agentID, map[string]any{"status": status})).
				Want(http.StatusBadRequest).Map()
			if body["error"] != "status must be active or paused" {
				t.Fatalf("error = %v, want the status validation message", body["error"])
			}
		})
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE assignee_id = $1`, agentID); n != 0 {
		t.Fatalf("rejected requests created %d autopilots, want 0", n)
	}
}
