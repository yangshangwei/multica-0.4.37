package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Staffing a built-in squad template.

// cleanupStaffedSquad removes everything one staffing call created, in dependency
// order: the squad first (squad.leader_id is ON DELETE RESTRICT, so its agents cannot
// go first), then the agents, then the role skills they materialized.
func cleanupStaffedSquad(t *testing.T, squadID string, agentIDs []string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		testPool.Exec(ctx, `DELETE FROM squad WHERE id = $1`, squadID)
		for _, agentID := range agentIDs {
			testPool.Exec(ctx, `DELETE FROM agent WHERE id = $1`, agentID)
		}
		testPool.Exec(ctx,
			`DELETE FROM skill WHERE workspace_id = $1 AND name LIKE 'multica-%'`, testWorkspaceID)
	})
}

func staffSquad(t *testing.T, templateKey string, extra map[string]any) CreateSquadFromTemplateResponse {
	t.Helper()
	body := map[string]any{
		"template_key": templateKey,
		"runtime_id":   handlerTestRuntimeID(t),
	}
	for key, value := range extra {
		body[key] = value
	}
	var out CreateSquadFromTemplateResponse
	req := withURLParam(newRequest("POST", "/api/squads/from-template", body), "workspaceId", testWorkspaceID)
	testutil.Call(t, testHandler.CreateSquadFromTemplate, req).Want(http.StatusCreated).JSON(&out)
	cleanupStaffedSquad(t, out.Squad.ID, append(append([]string{}, out.CreatedAgents...), out.ReusedAgents...))
	return out
}

// TestListSquadTemplates_ReturnsTheWholeRoster checks the picker payload: every
// squad this binary ships is offered, in the registry's product order, and each one
// arrives renderable — a leader seat, member seats with role notes, and a policy.
//
// The count is a literal rather than len(service.SquadTemplates()) so that shipping a
// ninth squad has to be a deliberate edit here, the same way the listed-role roster
// is pinned in service.
func TestListSquadTemplates_ReturnsTheWholeRoster(t *testing.T) {
	var out struct {
		Templates []SquadTemplateResponse `json:"templates"`
	}
	testutil.Call(t, testHandler.ListSquadTemplates,
		newRequest("GET", "/api/squads/templates?language=zh", nil)).
		Want(http.StatusOK).JSON(&out)

	if len(out.Templates) != 8 {
		t.Fatalf("squad templates = %d, want 8", len(out.Templates))
	}
	wantKeys := []string{
		"feature-delivery", "bug-fix",
		"review-gate", "discovery", "docs", "maintenance",
		"release", "incident",
	}
	for i, key := range wantKeys {
		if out.Templates[i].Key != key {
			t.Errorf("template[%d].Key = %q, want %q (the picker renders registry order)", i, out.Templates[i].Key, key)
		}
	}
	for _, template := range out.Templates {
		// language=zh above: the localized copy must actually be localized, or the
		// picker shows an English card inside a Chinese screen.
		if template.Title == "" {
			t.Errorf("%s: no localized title", template.Key)
		}
		if template.Description == "" {
			t.Errorf("%s: no localized description", template.Key)
		}
		if template.Leader.TemplateKey == "" {
			t.Errorf("%s: leader seat is empty", template.Key)
		}
		if template.Leader.AutonomyLevel != string(service.AutonomyCoordinator) {
			t.Errorf("%s: leader autonomy = %q, want coordinator", template.Key, template.Leader.AutonomyLevel)
		}
		if len(template.Members) == 0 {
			t.Errorf("%s: no member seats", template.Key)
		}
		for _, member := range template.Members {
			if member.Role == "" {
				t.Errorf("%s: seat %s has no role note for the leader's roster", template.Key, member.TemplateKey)
			}
		}
		if template.Instructions == "" {
			t.Errorf("%s: routing policy is empty", template.Key)
		}
	}
}

// TestCreateSquadFromTemplate_StaffsTheWholeRoster is the core contract: one call
// produces a working squad — leader bound as leader_id, every seat filled, the
// routing policy copied onto the row.
func TestCreateSquadFromTemplate_StaffsTheWholeRoster(t *testing.T) {
	staffed := staffSquad(t, "feature-delivery", map[string]any{"language": "en"})

	template, ok := service.SquadTemplateByKey("feature-delivery")
	if !ok {
		t.Fatal("feature-delivery template missing")
	}
	wantAgents := len(template.Members) + 1 // five seats plus the lead
	if len(staffed.CreatedAgents) != wantAgents {
		t.Errorf("created %d agents, want %d", len(staffed.CreatedAgents), wantAgents)
	}
	if staffed.Squad.TemplateKey != "feature-delivery" {
		t.Errorf("squad template_key = %q, want feature-delivery", staffed.Squad.TemplateKey)
	}
	if staffed.Squad.Instructions != template.Instructions() {
		t.Error("squad instructions are not the template's routing policy verbatim")
	}

	// The leader must be a coordinator: routing means mentioning members and moving
	// the parent issue, which an Observer is forbidden from doing.
	var leaderTemplate, leaderAutonomy string
	dbfx.QueryRow(t, `SELECT template_key, autonomy_level FROM agent WHERE id = $1`, staffed.Squad.LeaderID).
		Scan(&leaderTemplate, &leaderAutonomy)
	if leaderTemplate != template.LeaderTemplateKey {
		t.Errorf("leader template_key = %q, want %q", leaderTemplate, template.LeaderTemplateKey)
	}
	if leaderAutonomy != string(service.AutonomyCoordinator) {
		t.Errorf("leader autonomy_level = %q, want coordinator", leaderAutonomy)
	}

	// The leader is also the squad's first member with role "leader" — the literal the
	// roster renderer and the member-status endpoint key off.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM squad_member WHERE squad_id = $1 AND member_id = $2 AND role = 'leader'`,
		staffed.Squad.ID, staffed.Squad.LeaderID); count != 1 {
		t.Errorf("leader member rows with role 'leader' = %d, want 1", count)
	}
	if count := dbfx.Count(t, `SELECT COUNT(*) FROM squad_member WHERE squad_id = $1`, staffed.Squad.ID); count != wantAgents {
		t.Errorf("squad_member rows = %d, want %d (leader + members, leader seated once)", count, wantAgents)
	}
	// Every non-leader seat carries the role note the leader routes by.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM squad_member WHERE squad_id = $1 AND role <> '' `, staffed.Squad.ID); count != wantAgents {
		t.Errorf("%d of %d seats carry a role note", count, wantAgents)
	}

	// Each created agent carries its role's own instructions and autonomy.
	for _, agentID := range staffed.CreatedAgents {
		var templateKey, autonomy, instructions string
		dbfx.QueryRow(t, `SELECT template_key, autonomy_level, instructions FROM agent WHERE id = $1`, agentID).
			Scan(&templateKey, &autonomy, &instructions)
		role, ok := service.AgentRoleTemplateByKey(templateKey)
		if !ok {
			t.Errorf("agent %s has template_key %q, which is not a known role", agentID, templateKey)
			continue
		}
		if autonomy != string(role.Autonomy) {
			t.Errorf("%s autonomy = %q, want the role's %q", templateKey, autonomy, role.Autonomy)
		}
		if instructions != role.Instructions() {
			t.Errorf("%s instructions are not the template's text", templateKey)
		}
	}
}

// TestCreateSquadFromTemplate_ReusesExistingRoleAgents keeps a team that staffs both
// squads from ending up with two Implementers competing for one runtime.
func TestCreateSquadFromTemplate_ReusesExistingRoleAgents(t *testing.T) {
	first := staffSquad(t, "feature-delivery", nil)
	second := staffSquad(t, "bug-fix", nil)

	bugFix, ok := service.SquadTemplateByKey("bug-fix")
	if !ok {
		t.Fatal("bug-fix template missing")
	}
	// Only its own lead and the diagnostician are new; the analyst, implementer and QA already exist.
	if len(second.CreatedAgents) != 2 {
		t.Errorf("bug-fix created %d agents, want 2 (its lead and diagnostician)", len(second.CreatedAgents))
	}
	if len(second.ReusedAgents) != len(bugFix.Members)-1 {
		t.Errorf("bug-fix reused %d agents, want %d", len(second.ReusedAgents), len(bugFix.Members)-1)
	}
	firstAgents := map[string]bool{}
	for _, agentID := range first.CreatedAgents {
		firstAgents[agentID] = true
	}
	for _, agentID := range second.ReusedAgents {
		if !firstAgents[agentID] {
			t.Errorf("reused agent %s did not come from the first squad", agentID)
		}
	}
	// One Implementer in the workspace, in both rosters.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM agent WHERE workspace_id = $1 AND template_key = 'implementer' AND archived_at IS NULL`,
		testWorkspaceID); count != 1 {
		t.Errorf("%d implementers in the workspace, want 1", count)
	}
}

func TestCreateSquadFromTemplate_EmptyAgentListsAreArrays(t *testing.T) {
	first := staffSquad(t, "discovery", nil)
	if len(first.CreatedAgents) == 0 {
		t.Fatal("first staffing created no agents")
	}
	// Decoding JSON [] gives a non-nil slice; null or an omitted field does not.
	if first.ReusedAgents == nil || len(first.ReusedAgents) != 0 {
		t.Errorf("all-new staffing reused_agent_ids = %#v, want an empty JSON array", first.ReusedAgents)
	}

	second := staffSquad(t, "discovery", map[string]any{"name": "Second Discovery Squad"})
	if second.Squad.ID == first.Squad.ID {
		t.Error("second staffing did not create a new squad")
	}
	if second.CreatedAgents == nil || len(second.CreatedAgents) != 0 {
		t.Errorf("all-reused staffing created_agent_ids = %#v, want an empty JSON array", second.CreatedAgents)
	}
	if len(second.ReusedAgents) != len(first.CreatedAgents) {
		t.Errorf("second staffing reused %d agents, want %d", len(second.ReusedAgents), len(first.CreatedAgents))
	}
}

// TestCreateSquadFromTemplate_StaffsASecondBatchSquad covers the squads added after
// the two pilots. review-gate is the useful one to pin: its roster overlaps
// feature-delivery on two seats, so one call exercises both halves of staffing a
// later squad — the seats it has to create, and the role agents it must reuse
// rather than duplicate.
func TestCreateSquadFromTemplate_StaffsASecondBatchSquad(t *testing.T) {
	staffSquad(t, "feature-delivery", nil)
	gate := staffSquad(t, "review-gate", nil)

	template, ok := service.SquadTemplateByKey("review-gate")
	if !ok {
		t.Fatal("review-gate template missing")
	}
	if gate.Squad.Instructions != template.Instructions() {
		t.Error("squad instructions are not the template's routing policy verbatim")
	}
	// Its own lead plus the Security Reviewer; the Code Reviewer and QA Engineer came
	// from feature-delivery.
	if len(gate.CreatedAgents) != 2 {
		t.Errorf("review-gate created %d agents, want 2 (its lead and the security reviewer)", len(gate.CreatedAgents))
	}
	if len(gate.ReusedAgents) != 2 {
		t.Errorf("review-gate reused %d agents, want 2 (code reviewer, QA engineer)", len(gate.ReusedAgents))
	}
	if count := dbfx.Count(t, `SELECT COUNT(*) FROM squad_member WHERE squad_id = $1`, gate.Squad.ID); count != len(template.Members)+1 {
		t.Errorf("squad_member rows = %d, want %d", count, len(template.Members)+1)
	}
	// Each template's autonomy ceiling is pinned in
	// service/builtin_agent_autonomy_test.go (TestSquadTemplates_MaxAutonomyMatchesRoster);
	// the gate that reads it is TestCreateSquadFromTemplate_CoordinatorCannotStaffAnOperatorRoster.
	//
	// One Code Reviewer in the workspace, seated in both squads.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM agent WHERE workspace_id = $1 AND template_key = 'code-reviewer' AND archived_at IS NULL`,
		testWorkspaceID); count != 1 {
		t.Errorf("%d code reviewers in the workspace, want 1", count)
	}
}

// TestCreateSquadFromTemplate_ReusedAgentKeepsItsEdits pins that staffing a second
// squad is not consent to undo a workspace's tuning of an existing role agent.
func TestCreateSquadFromTemplate_ReusedAgentKeepsItsEdits(t *testing.T) {
	first := staffSquad(t, "feature-delivery", nil)

	var implementerID string
	dbfx.QueryRow(t,
		`SELECT id FROM agent WHERE workspace_id = $1 AND template_key = 'implementer'`, testWorkspaceID).
		Scan(&implementerID)
	if implementerID == "" {
		t.Fatalf("feature-delivery did not create an implementer (created %v)", first.CreatedAgents)
	}
	dbfx.Exec(t, `UPDATE agent SET instructions = $1, autonomy_level = 'observer' WHERE id = $2`,
		"Our own implementer rules.", implementerID)

	staffSquad(t, "bug-fix", nil)

	var instructions, autonomy string
	dbfx.QueryRow(t, `SELECT instructions, autonomy_level FROM agent WHERE id = $1`, implementerID).
		Scan(&instructions, &autonomy)
	if instructions != "Our own implementer rules." {
		t.Error("staffing a second squad overwrote the workspace's edited instructions")
	}
	if autonomy != "observer" {
		t.Errorf("autonomy_level = %q, want the workspace's own choice preserved", autonomy)
	}
}

// TestCreateSquadFromTemplate_ConflictingAgentNameIsAPersonsDecision covers the one
// failure a user can act on: a hand-built agent already holds a role's default name.
// Adopting it silently would put an agent with unknown instructions into a roster
// that assumes the role's contract.
func TestCreateSquadFromTemplate_ConflictingAgentNameIsAPersonsDecision(t *testing.T) {
	dbfx.Agent(t, "Implementer", handlerTestRuntimeID(t), testutil.Cols{
		"visibility":      "workspace",
		"permission_mode": "public_to",
	})
	t.Cleanup(func() {
		ctx := context.Background()
		testPool.Exec(ctx, `DELETE FROM squad WHERE workspace_id = $1 AND template_key = 'bug-fix'`, testWorkspaceID)
		testPool.Exec(ctx, `DELETE FROM agent WHERE workspace_id = $1 AND template_key <> ''`, testWorkspaceID)
		testPool.Exec(ctx, `DELETE FROM skill WHERE workspace_id = $1 AND name LIKE 'multica-%'`, testWorkspaceID)
	})

	req := withURLParam(newRequest("POST", "/api/squads/from-template", map[string]any{
		"template_key": "bug-fix",
		"runtime_id":   handlerTestRuntimeID(t),
	}), "workspaceId", testWorkspaceID)
	testutil.Call(t, testHandler.CreateSquadFromTemplate, req).Want(http.StatusConflict)

	// All or nothing: a squad that came up without its implementer would be worse
	// than one that failed outright.
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM squad WHERE workspace_id = $1 AND template_key = 'bug-fix'`, testWorkspaceID); count != 0 {
		t.Errorf("%d squads survived the conflict, want 0", count)
	}
	if count := dbfx.Count(t,
		`SELECT COUNT(*) FROM agent WHERE workspace_id = $1 AND template_key <> ''`, testWorkspaceID); count != 0 {
		t.Errorf("%d template agents survived the conflict, want 0", count)
	}
}

func TestCreateSquadFromTemplate_ValidatesInput(t *testing.T) {
	base := map[string]any{"runtime_id": handlerTestRuntimeID(t)}
	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{name: "unknown template", body: map[string]any{"template_key": "platform-squad", "runtime_id": base["runtime_id"]}, want: http.StatusBadRequest},
		{name: "missing runtime", body: map[string]any{"template_key": "bug-fix"}, want: http.StatusBadRequest},
		{name: "runtime from another workspace", body: map[string]any{"template_key": "bug-fix", "runtime_id": "00000000-0000-0000-0000-000000000000"}, want: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := withURLParam(newRequest("POST", "/api/squads/from-template", tc.body), "workspaceId", testWorkspaceID)
			testutil.Call(t, testHandler.CreateSquadFromTemplate, req).Want(tc.want)
		})
	}
}

// TestCreateSquadFromTemplate_WorkspaceAccessWidensOnRequest pins that access is an
// explicit choice: the default is private, and a squad the whole team assigns work to
// has to ask for the workspace target.
func TestCreateSquadFromTemplate_WorkspaceAccessWidensOnRequest(t *testing.T) {
	staffed := staffSquad(t, "bug-fix", map[string]any{
		"permission_mode":    "public_to",
		"invocation_targets": []map[string]any{{"target_type": "workspace"}},
	})
	for _, agentID := range staffed.CreatedAgents {
		var mode string
		dbfx.QueryRow(t, `SELECT permission_mode FROM agent WHERE id = $1`, agentID).Scan(&mode)
		if mode != "public_to" {
			t.Errorf("agent %s permission_mode = %q, want public_to", agentID, mode)
		}
		if count := dbfx.Count(t,
			`SELECT COUNT(*) FROM agent_invocation_target WHERE agent_id = $1 AND target_type = 'workspace'`,
			agentID); count != 1 {
			t.Errorf("agent %s has %d workspace invocation targets, want 1", agentID, count)
		}
	}
}

func TestCreateSquadFromTemplate_DefaultsToPrivateAccess(t *testing.T) {
	staffed := staffSquad(t, "bug-fix", nil)
	for _, agentID := range staffed.CreatedAgents {
		var mode string
		dbfx.QueryRow(t, `SELECT permission_mode FROM agent WHERE id = $1`, agentID).Scan(&mode)
		if mode != "private" {
			t.Errorf("agent %s permission_mode = %q, want private: staffing must not widen access silently", agentID, mode)
		}
	}
}
