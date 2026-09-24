package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Creating an agent from a built-in role template, end to end through the HTTP
// handlers against a real database.

// cleanupTemplateAgent removes an agent this test created through the API. Rows the
// API creates are outside dbfx's cleanup ledger, so each test that creates one has
// to name it.
func cleanupTemplateAgent(t *testing.T, agentID string) {
	t.Helper()
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})
}

// cleanupRoleSkill removes a materialized role skill by name. Skills survive the
// agent that pulled them in — that is the point of materializing them — so a test
// that triggers materialization owns the row.
func cleanupRoleSkill(t *testing.T, name string) {
	t.Helper()
	t.Cleanup(func() {
		testPool.Exec(context.Background(),
			`DELETE FROM skill WHERE workspace_id = $1 AND name = $2`, testWorkspaceID, name)
	})
}

func TestListAgentRoleTemplates_ReturnsTheRosterWithInstructions(t *testing.T) {
	var out struct {
		Templates []AgentRoleTemplateResponse `json:"templates"`
	}
	testutil.Call(t, testHandler.ListAgentRoleTemplates,
		newRequest("GET", "/api/agents/templates?language=zh", nil)).
		Want(http.StatusOK).JSON(&out)

	if len(out.Templates) != 14 {
		t.Fatalf("templates = %d, want 14", len(out.Templates))
	}
	for _, template := range out.Templates {
		if template.Key == "" || template.Name == "" {
			t.Errorf("template %+v is missing key or name", template)
		}
		if strings.TrimSpace(template.Instructions) == "" {
			// The picker shows the prompt before a person adopts it. An empty body here
			// would mean the embedded file failed to load in the built binary.
			t.Errorf("%s: instructions are empty", template.Key)
		}
		if !service.IsKnownAutonomyLevel(template.AutonomyLevel) {
			t.Errorf("%s: autonomy_level = %q is not a known level", template.Key, template.AutonomyLevel)
		}
	}
	// language=zh must actually change the copy, or the parameter is decoration.
	analyst := findTemplate(t, out.Templates, "product-analyst")
	if analyst.Title == "Product Analyst" {
		t.Errorf("title for language=zh = %q, want the localized label", analyst.Title)
	}
	reporter := findTemplate(t, out.Templates, "progress-reporter")
	// The name is the default agent name and follows the request language like
	// every other picker string; the English DefaultName is only the fallback.
	if reporter.Title != "进展报告员" || reporter.Name != "进展报告员" {
		t.Errorf("reporter title/name = %q/%q, want 进展报告员/进展报告员", reporter.Title, reporter.Name)
	}
	diagnostician := findTemplate(t, out.Templates, "diagnostician")
	if diagnostician.Title != "诊断工程师" || diagnostician.Name != "诊断工程师" {
		t.Errorf("diagnostician title/name = %q/%q, want 诊断工程师/诊断工程师", diagnostician.Title, diagnostician.Name)
	}
	for _, key := range []string{"experience-validation-engineer", "migration-reviewer"} {
		template := findTemplate(t, out.Templates, key)
		if template.Key != key || strings.TrimSpace(template.Instructions) == "" {
			t.Errorf("specialist template %q missing from API roster: %+v", key, template)
		}
	}
}

func TestCreateAgentFromTemplate_WorkloadBackedSpecialistsCopyTheirContracts(t *testing.T) {
	runtimeID := handlerTestRuntimeID(t)
	for _, tc := range []struct {
		key   string
		skill string
	}{
		{key: "experience-validation-engineer", skill: "multica-experience-validation"},
		{key: "migration-reviewer", skill: "multica-migration-review"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			cleanupRoleSkill(t, tc.skill)
			var out AgentResponse
			testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
				"template_key": tc.key,
				"runtime_id":   runtimeID,
				"language":     "zh",
			})).Want(http.StatusCreated).JSON(&out)
			cleanupTemplateAgent(t, out.ID)

			role, ok := service.AgentRoleTemplateByKey(tc.key)
			if !ok {
				t.Fatalf("template %q missing from registry", tc.key)
			}
			if out.TemplateKey != tc.key || out.TemplateVersion != role.Version || out.Instructions != role.Instructions() {
				t.Errorf("%s provenance/instructions = %s v%d, want %s v%d and canonical body", tc.key, out.TemplateKey, out.TemplateVersion, tc.key, role.Version)
			}
			if len(out.Skills) != 1 || out.Skills[0].Name != tc.skill {
				t.Errorf("%s skills = %+v, want %s", tc.key, out.Skills, tc.skill)
			}
		})
	}
}

func TestCreateAgentFromTemplate_DiagnosticianCopiesAndPreservesWorkspaceContent(t *testing.T) {
	cleanupRoleSkill(t, "multica-debugging")
	runtimeID := handlerTestRuntimeID(t)
	var first AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key":         "diagnostician",
		"runtime_id":           runtimeID,
		"language":             "zh",
		"instructions":         "Ship the fix",
		"autonomy_level":       "operator",
		"max_concurrent_tasks": 9,
		"template_version":     9,
		"skill_ids":            []string{},
	})).Want(http.StatusCreated).JSON(&first)
	cleanupTemplateAgent(t, first.ID)

	role, ok := service.AgentRoleTemplateByKey("diagnostician")
	if !ok {
		t.Fatal("diagnostician missing from registry")
	}
	if first.Name != "诊断工程师" || first.Description != role.Description("zh") || first.Instructions != role.Instructions() {
		t.Error("diagnostician must copy the localized identity and canonical server instructions")
	}
	if first.TemplateKey != role.Key || first.TemplateVersion != 1 || first.AutonomyLevel != "contributor" || first.MaxConcurrentTasks != 1 || first.PermissionMode != "private" {
		t.Errorf("diagnostician policy/provenance = %s v%d %s/%d/%s", first.TemplateKey, first.TemplateVersion, first.AutonomyLevel, first.MaxConcurrentTasks, first.PermissionMode)
	}
	if len(first.Skills) != 1 || first.Skills[0].Name != "multica-debugging" {
		t.Fatalf("diagnostician skills = %+v, want only multica-debugging", first.Skills)
	}
	skillID := first.Skills[0].ID
	var content, description, category, icon, origin string
	var version int
	dbfx.QueryRow(t, `SELECT content, description, config->'presentation'->>'category',
		config->'presentation'->>'icon', config->'origin'->>'type', (config->'origin'->>'version')::int
		FROM skill WHERE id = $1 AND workspace_id = $2`, skillID, testWorkspaceID).
		Scan(&content, &description, &category, &icon, &origin, &version)
	method, ok := service.RoleSkillTemplateByName("multica-debugging")
	if !ok || content != method.Content || description != method.Description {
		t.Error("materialized debugging skill must copy the complete embedded content and description")
	}
	if category != "quality" || icon != "microscope" || origin != roleSkillOriginType || version != 1 {
		t.Errorf("debugging skill metadata = %s/%s/%s/v%d", category, icon, origin, version)
	}

	const customInstructions = "Investigate only the team's local test fixtures."
	const customContent = "# Team debugging\n\nUse the team's reproduction harness."
	const customDescription = "Team debugging method"
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+first.ID, map[string]any{
		"instructions": customInstructions,
	}), "id", first.ID)).Want(http.StatusOK)
	testutil.Call(t, testHandler.UpdateSkill, withURLParam(newRequest("PUT", "/api/skills/"+skillID, map[string]any{
		"content": customContent, "description": customDescription,
	}), "id", skillID)).Want(http.StatusOK)

	var second AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "diagnostician", "runtime_id": runtimeID, "language": "en",
	})).Want(http.StatusCreated).JSON(&second)
	cleanupTemplateAgent(t, second.ID)
	if second.Name != "Diagnostician" || second.Instructions != role.Instructions() {
		t.Error("new diagnostician must use canonical instructions independently of locale or existing agents")
	}
	if len(second.Skills) != 1 || second.Skills[0].ID != skillID {
		t.Fatalf("second diagnostician skills = %+v, want existing skill %s", second.Skills, skillID)
	}
	var existing AgentResponse
	testutil.Call(t, testHandler.GetAgent, withURLParam(newRequest("GET", "/api/agents/"+first.ID, nil), "id", first.ID)).
		Want(http.StatusOK).JSON(&existing)
	if existing.Instructions != customInstructions {
		t.Error("creating a second diagnostician must preserve existing agent instructions")
	}
	dbfx.QueryRow(t, `SELECT content, description FROM skill WHERE id = $1`, skillID).Scan(&content, &description)
	if content != customContent || description != customDescription {
		t.Error("creating a second diagnostician must preserve the workspace's customized debugging skill")
	}
	if count := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1 AND name = $2`, testWorkspaceID, "multica-debugging"); count != 1 {
		t.Errorf("debugging skill copies = %d, want 1", count)
	}
}

func findTemplate(t *testing.T, templates []AgentRoleTemplateResponse, key string) AgentRoleTemplateResponse {
	t.Helper()
	for _, template := range templates {
		if template.Key == key {
			return template
		}
	}
	t.Fatalf("template %q not in response", key)
	return AgentRoleTemplateResponse{}
}

// TestCreateAgentFromTemplate_CopiesTheRoleOntoAnOrdinaryAgent is the core contract:
// the row that comes back is a normal agent whose instructions ARE the template's,
// carrying provenance and the role's autonomy level, with the role skill attached.
func TestCreateAgentFromTemplate_CopiesTheRoleOntoAnOrdinaryAgent(t *testing.T) {
	cleanupRoleSkill(t, "multica-code-review")

	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "code-reviewer",
		"runtime_id":   handlerTestRuntimeID(t),
		"name":         "Template Reviewer",
		"language":     "en",
	})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAgent(t, created.ID)

	template, ok := service.AgentRoleTemplateByKey("code-reviewer")
	if !ok {
		t.Fatal("code-reviewer template missing from the registry")
	}
	if created.Instructions != template.Instructions() {
		t.Error("created agent's instructions are not the template's text verbatim")
	}
	if created.TemplateKey != "code-reviewer" {
		t.Errorf("template_key = %q, want code-reviewer", created.TemplateKey)
	}
	if created.TemplateVersion != template.Version {
		t.Errorf("template_version = %d, want %d", created.TemplateVersion, template.Version)
	}
	if created.AutonomyLevel != string(template.Autonomy) {
		t.Errorf("autonomy_level = %q, want %q", created.AutonomyLevel, template.Autonomy)
	}
	if created.MaxConcurrentTasks != template.MaxConcurrentTasks {
		t.Errorf("max_concurrent_tasks = %d, want the template's %d", created.MaxConcurrentTasks, template.MaxConcurrentTasks)
	}
	// Default access is private, the same default a blank agent gets: adopting a
	// template must not widen who can run something.
	if created.PermissionMode != "private" {
		t.Errorf("permission_mode = %q, want private", created.PermissionMode)
	}
	if len(created.Skills) != 1 || created.Skills[0].Name != "multica-code-review" {
		t.Errorf("skills = %+v, want the materialized multica-code-review", created.Skills)
	}

	// The skill is a real workspace row, editable on the Skills page — not a hidden
	// server-side attachment.
	var content, description string
	dbfx.QueryRow(t, `SELECT content, description FROM skill WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "multica-code-review").Scan(&content, &description)
	roleSkill, ok := service.RoleSkillTemplateByName("multica-code-review")
	if !ok {
		t.Fatal("multica-code-review skill missing from the registry")
	}
	if content != roleSkill.Content {
		t.Error("materialized skill content is not the embedded SKILL.md verbatim")
	}
	if description == "" {
		t.Error("materialized skill has no description; the frontmatter summary was dropped")
	}
	var origin string
	dbfx.QueryRow(t, `SELECT config->'origin'->>'type' FROM skill WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "multica-code-review").Scan(&origin)
	if origin != roleSkillOriginType {
		t.Errorf("skill origin type = %q, want %q so 'update from source' cannot offer to refetch it", origin, roleSkillOriginType)
	}
}

func TestCreateAgentFromTemplate_ProgressReporterUsesServerDefaults(t *testing.T) {
	cleanupRoleSkill(t, "multica-progress-report")
	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key":         "progress-reporter",
		"runtime_id":           handlerTestRuntimeID(t),
		"language":             "zh",
		"instructions":         "Replace the reporting instructions",
		"autonomy_level":       "operator",
		"max_concurrent_tasks": 9,
		"template_version":     9,
		"skill_ids":            []string{},
	})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAgent(t, created.ID)

	template, ok := service.AgentRoleTemplateByKey("progress-reporter")
	if !ok {
		t.Fatal("progress-reporter template missing from the registry")
	}
	// No name override and language=zh: the default name is the localized label,
	// not the English DefaultName.
	if created.Name != "进展报告员" || created.Description != template.Description("zh") {
		t.Errorf("reporter name/description = %q/%q, want the localized default name and Chinese description", created.Name, created.Description)
	}
	if created.TemplateKey != template.Key || created.TemplateVersion != 1 {
		t.Errorf("reporter provenance = %q v%d, want progress-reporter v1", created.TemplateKey, created.TemplateVersion)
	}
	if created.Instructions != template.Instructions() {
		t.Error("reporter instructions must be copied verbatim from the server, independently of language or client input")
	}
	if created.AutonomyLevel != "contributor" || created.MaxConcurrentTasks != 1 || created.PermissionMode != "private" {
		t.Errorf("reporter policy = %s/%d/%s, want contributor/1/private", created.AutonomyLevel, created.MaxConcurrentTasks, created.PermissionMode)
	}
	if len(created.Skills) != 1 || created.Skills[0].Name != "multica-progress-report" {
		t.Fatalf("reporter skills = %+v, want only multica-progress-report", created.Skills)
	}

	skill, ok := service.RoleSkillTemplateByName("multica-progress-report")
	if !ok {
		t.Fatal("multica-progress-report skill missing from the registry")
	}
	var content, description, origin, version string
	dbfx.QueryRow(t, `SELECT content, description, config->'origin'->>'type', config->'origin'->>'version' FROM skill WHERE id = $1`,
		created.Skills[0].ID).Scan(&content, &description, &origin, &version)
	if content != skill.Content || description != skill.Description || origin != roleSkillOriginType || version != "3" {
		t.Error("reporter's materialized role skill must preserve the embedded body, description and v3 provenance")
	}
}

func TestCreateAgentFromTemplate_ProgressReporterPreservesWorkspaceCopies(t *testing.T) {
	const editedSkill = "# 团队进展报告\n\n只汇总移动端项目。\n"
	skillID := dbfx.Insert(t, "skill", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"name":         "multica-progress-report",
		"description":  "团队自定义统计口径",
		"content":      editedSkill,
		"created_by":   testUserID,
	})
	runtimeID := handlerTestRuntimeID(t)
	var first AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "progress-reporter",
		"runtime_id":   runtimeID,
		"name":         "Daily Reporter",
		"language":     "zh",
	})).Want(http.StatusCreated).JSON(&first)
	cleanupTemplateAgent(t, first.ID)
	const editedInstructions = "只报告已批准的移动端任务，保留团队自定义的统计规则。"
	testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+first.ID, map[string]any{
		"instructions": editedInstructions,
	}), "id", first.ID)).Want(http.StatusOK)

	var second AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "progress-reporter",
		"runtime_id":   runtimeID,
		"name":         "Weekly Reporter",
		"language":     "en",
	})).Want(http.StatusCreated).JSON(&second)
	cleanupTemplateAgent(t, second.ID)

	var existing AgentResponse
	testutil.Call(t, testHandler.GetAgent, withURLParam(newRequest("GET", "/api/agents/"+first.ID, nil),
		"id", first.ID)).Want(http.StatusOK).JSON(&existing)
	if existing.Instructions != editedInstructions {
		t.Error("creating another reporter or changing catalog language must not overwrite the existing agent's instructions")
	}
	template, ok := service.AgentRoleTemplateByKey("progress-reporter")
	if !ok || second.Instructions != template.Instructions() {
		t.Error("a new reporter must copy the canonical template independently of existing agents")
	}
	for _, agent := range []AgentResponse{existing, second} {
		if len(agent.Skills) != 1 || agent.Skills[0].ID != skillID {
			t.Errorf("%s skills = %+v, want the same existing progress report skill %s", agent.Name, agent.Skills, skillID)
		}
	}
	var content, description string
	dbfx.QueryRow(t, `SELECT content, description FROM skill WHERE id = $1`, skillID).Scan(&content, &description)
	if content != editedSkill || description != "团队自定义统计口径" {
		t.Error("creating reporters must reuse the customized role skill without rewriting its content or description")
	}
	if count := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "multica-progress-report"); count != 1 {
		t.Errorf("progress report skill copies = %d, want 1", count)
	}
}

// TestCreateAgentFromTemplate_ReusesAnEditedSkill is the "never overwrite a
// workspace admin's local change" rule. It is the reason materialization has no
// update path at all.
func TestCreateAgentFromTemplate_ReusesAnEditedSkill(t *testing.T) {
	edited := "# Our own security review\n\nAsk the platform team first.\n"
	skillID := dbfx.Insert(t, "skill", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"name":         "multica-security-review",
		"description":  "workspace edited",
		"content":      edited,
		"created_by":   testUserID,
	})

	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "security-reviewer",
		"runtime_id":   handlerTestRuntimeID(t),
		"name":         "Template Security Reviewer",
	})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAgent(t, created.ID)

	if len(created.Skills) != 1 || created.Skills[0].ID != skillID {
		t.Fatalf("skills = %+v, want the workspace's existing skill %s attached", created.Skills, skillID)
	}
	var content string
	dbfx.QueryRow(t, `SELECT content FROM skill WHERE id = $1`, skillID).Scan(&content)
	if content != edited {
		t.Error("the workspace's edited skill was overwritten by the embedded copy")
	}
	if count := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "multica-security-review"); count != 1 {
		t.Errorf("%d skills named multica-security-review, want 1 — a second copy diverges from the first", count)
	}
}

// TestCreateAgentFromTemplate_SecondAgentSharesTheSkill covers the other half of
// reuse: two agents of the same role attach one skill, so editing it reaches both.
func TestCreateAgentFromTemplate_SecondAgentSharesTheSkill(t *testing.T) {
	cleanupRoleSkill(t, "multica-test-report")

	var first, second AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "implementer",
		"runtime_id":   handlerTestRuntimeID(t),
		"name":         "Implementer One",
	})).Want(http.StatusCreated).JSON(&first)
	cleanupTemplateAgent(t, first.ID)

	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "implementer",
		"runtime_id":   handlerTestRuntimeID(t),
		"name":         "Implementer Two",
	})).Want(http.StatusCreated).JSON(&second)
	cleanupTemplateAgent(t, second.ID)

	if len(first.Skills) != 1 || len(second.Skills) != 1 {
		t.Fatalf("skills attached: first=%d second=%d, want 1 each", len(first.Skills), len(second.Skills))
	}
	if first.Skills[0].ID != second.Skills[0].ID {
		t.Error("the two implementers attached different skill rows; one edit would only reach one of them")
	}
	if count := dbfx.Count(t, `SELECT COUNT(*) FROM skill WHERE workspace_id = $1 AND name = $2`,
		testWorkspaceID, "multica-test-report"); count != 1 {
		t.Errorf("%d copies of multica-test-report, want 1", count)
	}
}

func TestCreateAgentFromTemplate_RejectsUnknownAndUnlistedKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{name: "unknown", key: "frontend-engineer"},
		{name: "empty", key: ""},
		// The squad-leader definitions are real templates but are provisioned by the
		// squad flow, which also creates the roster a coordinator needs.
		{name: "unlisted squad leader", key: "feature-delivery-lead"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
				"template_key": tc.key,
				"runtime_id":   handlerTestRuntimeID(t),
			})).Want(http.StatusBadRequest)
		})
	}
}

// TestCreateAgentFromTemplate_WidensAccessOnRequest covers the path the UI takes
// when a person picks "Entire workspace": the template flow passes permission
// input through to the shared create path rather than re-implementing it.
func TestCreateAgentFromTemplate_WidensAccessOnRequest(t *testing.T) {
	cleanupRoleSkill(t, "multica-architecture-decision-record")

	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key":       "architect",
		"runtime_id":         handlerTestRuntimeID(t),
		"name":               "Workspace Architect",
		"permission_mode":    "public_to",
		"invocation_targets": []map[string]any{{"target_type": "workspace"}},
	})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAgent(t, created.ID)

	if created.PermissionMode != "public_to" {
		t.Errorf("permission_mode = %q, want public_to", created.PermissionMode)
	}
	// Legacy `visibility` is derived from the permission model, so a workspace
	// target has to surface as "workspace" for older clients too.
	if created.Visibility != "workspace" {
		t.Errorf("visibility = %q, want workspace", created.Visibility)
	}
	if len(created.InvocationTargets) != 1 || created.InvocationTargets[0].TargetType != "workspace" {
		t.Errorf("invocation_targets = %+v, want one workspace target", created.InvocationTargets)
	}
}

func TestCreateAgentFromTemplate_RequiresRuntime(t *testing.T) {
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "architect",
	})).Want(http.StatusBadRequest)
}

// TestCreateAgent_IgnoresClientSuppliedProvenance pins the security property behind
// making provenance a server decision: the public create endpoint has no way to
// claim a template or to mint an autonomy level.
func TestCreateAgent_IgnoresClientSuppliedProvenance(t *testing.T) {
	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgent, newRequest("POST", "/api/agents", map[string]any{
		"name":            "Provenance Claimer",
		"runtime_id":      handlerTestRuntimeID(t),
		"template_key":    "release-engineer",
		"autonomy_level":  "operator",
		"templateVersion": 9,
	})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAgent(t, created.ID)

	if created.TemplateKey != "" {
		t.Errorf("template_key = %q, want empty: a client must not be able to claim a template it did not use", created.TemplateKey)
	}
	if created.AutonomyLevel != "" {
		t.Errorf("autonomy_level = %q, want empty: create must not mint an operator", created.AutonomyLevel)
	}
}
