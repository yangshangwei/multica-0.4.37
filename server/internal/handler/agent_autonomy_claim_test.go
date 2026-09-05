package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

// The autonomy policy an agent is shown at claim time.
//
// This is the half of enforcement that reaches the parts of a task the server cannot
// intercept — a shell command on the daemon host. It has to travel with the claim,
// and it has to be absent for agents that never opted into a role.

// claimWithAutonomyLevel sets an agent's declared level, enqueues a task for it, and
// returns the instructions the claim response carried.
func claimWithAutonomyLevel(t *testing.T, ctx context.Context, name, level, ownInstructions string) string {
	t.Helper()

	fx := newSquadBriefingClaimFixture(t, ctx, name)
	if _, err := testPool.Exec(ctx,
		`UPDATE agent SET autonomy_level = $2, instructions = $3 WHERE id = $1`,
		fx.AgentID, level, ownInstructions); err != nil {
		t.Fatalf("set autonomy level: %v", err)
	}
	// A non-leader task: this test is about the policy layer, not the squad briefing.
	enqueueClaimTask(t, ctx, fx, false /*isLeader*/, false /*withSquadID*/)

	_, instructions, _, raw := claimAgentInstructionsForTest(t, fx.RuntimeID)
	if instructions == "" && ownInstructions != "" {
		t.Fatalf("claim returned no agent instructions: %s", raw)
	}
	return instructions
}

func TestClaim_InjectsAutonomyPolicyAfterTheAgentsOwnInstructions(t *testing.T) {
	ctx := context.Background()
	instructions := claimWithAutonomyLevel(t, ctx, "autonomy-observer-claim", "observer", "Review only what you are asked about.")

	if !strings.Contains(instructions, "## Autonomy policy (system)") {
		t.Fatalf("claim did not inject the policy section:\n%s", instructions)
	}
	if !strings.Contains(instructions, "Your level: Observer") {
		t.Error("injected policy does not announce the agent's level")
	}
	// Order matters: the agent reads its role, then the ceiling that overrides it.
	own := strings.Index(instructions, "Review only what you are asked about.")
	policy := strings.Index(instructions, "## Autonomy policy (system)")
	if own == -1 || policy == -1 || own > policy {
		t.Errorf("policy must follow the agent's own instructions (own=%d policy=%d)", own, policy)
	}
	// The policy is never stored on the row — that is what keeps it hot-updatable and
	// impossible for a workspace to edit away.
	var stored string
	dbfx.QueryRow(t, `SELECT instructions FROM agent WHERE instructions LIKE 'Review only what%' LIMIT 1`).Scan(&stored)
	if strings.Contains(stored, "Autonomy policy") {
		t.Error("the policy was written into agent.instructions; it must only exist in the claim response")
	}
}

func TestClaim_OperatorGetsTheApprovalProtocol(t *testing.T) {
	ctx := context.Background()
	instructions := claimWithAutonomyLevel(t, ctx, "autonomy-operator-claim", "operator", "Prepare releases.")

	if !strings.Contains(instructions, "### The approval protocol") {
		t.Fatalf("operator claim is missing the approval protocol:\n%s", instructions)
	}
	for _, class := range service.ApprovalRiskClasses {
		if !strings.Contains(instructions, class) {
			t.Errorf("operator claim does not name risk class %q", class)
		}
	}
}

// TestClaim_UndeclaredAgentGetsNoPolicySection is the compatibility guarantee at the
// prompt level: an agent created before role templates must claim byte-identical
// instructions to what it claimed before.
func TestClaim_UndeclaredAgentGetsNoPolicySection(t *testing.T) {
	ctx := context.Background()
	own := "Do the work described in the issue."
	instructions := claimWithAutonomyLevel(t, ctx, "autonomy-undeclared-claim", "", own)

	if instructions != own {
		t.Errorf("claim instructions = %q, want exactly the agent's own text with nothing appended", instructions)
	}
}
