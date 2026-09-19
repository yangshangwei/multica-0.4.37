package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Presentation metadata (config.presentation) is validated against the shared
// category/icon whitelist on every write path and normalized before storage.
// The canonical validation matrix lives in internal/skill/presentation_test.go;
// these tests cover the HTTP wiring: 400 with the field-naming message, the
// stored shape, sibling-key preservation, template materialization, and import
// seeding from frontmatter.

func skillConfigFromDB(t *testing.T, skillID string) map[string]any {
	t.Helper()
	var raw []byte
	dbfx.QueryRow(t, `SELECT config FROM skill WHERE id = $1`, skillID).Scan(&raw)
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode stored config %s: %v", raw, err)
	}
	return cfg
}

func cleanupSkillByName(t *testing.T, name string) {
	t.Helper()
	dbfx.Cleanup(t, `DELETE FROM skill WHERE workspace_id = $1 AND name = $2`, testWorkspaceID, name)
}

func TestCreateSkill_RejectsInvalidPresentation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	tests := []struct {
		name         string
		presentation any
		wantErr      string
	}{
		{name: "unknown category", presentation: map[string]any{"category": "bogus"},
			wantErr: `config.presentation.category: unknown value "bogus"`},
		{name: "unknown icon", presentation: map[string]any{"icon": "PenLine"},
			wantErr: `config.presentation.icon: unknown value "PenLine"`},
		{name: "not an object", presentation: "engineering",
			wantErr: "config.presentation must be an object"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := "presentation-reject-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
			cleanupSkillByName(t, name)
			res := testutil.Call(t, testHandler.CreateSkill, newRequest(http.MethodPost, "/api/skills", map[string]any{
				"name":    name,
				"content": "# body",
				"config":  map[string]any{"presentation": tt.presentation},
			})).Want(http.StatusBadRequest)
			if got, _ := res.Map()["error"].(string); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("error = %q, want it to name the field: %q", got, tt.wantErr)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1 AND name = $2`, testWorkspaceID, name); n != 0 {
				t.Fatalf("rejected create still stored %d row(s)", n)
			}
		})
	}
}

func TestCreateSkill_StoresNormalizedPresentation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	name := "presentation-create-" + t.Name()
	cleanupSkillByName(t, name)

	var created SkillWithFilesResponse
	testutil.Call(t, testHandler.CreateSkill, newRequest(http.MethodPost, "/api/skills", map[string]any{
		"name":    name,
		"content": "# body",
		"config": map[string]any{
			"template_source": map[string]any{"name": "multica-code-review"},
			"presentation": map[string]any{
				"category": "engineering",
				"icon":     "code",             // equals the engineering default → dropped
				"tags":     []string{"legacy"}, // pre-label-system key → dropped
			},
		},
	})).Want(http.StatusCreated).JSON(&created)

	cfg := skillConfigFromDB(t, created.ID)
	if _, ok := cfg["template_source"]; !ok {
		t.Error("sibling key template_source was dropped by normalization")
	}
	pres, _ := cfg["presentation"].(map[string]any)
	if pres["category"] != "engineering" {
		t.Errorf("category = %v, want engineering", pres["category"])
	}
	if _, has := pres["icon"]; has {
		t.Errorf("icon = %v, want absent because it equals the category default", pres["icon"])
	}
	if _, has := pres["tags"]; has {
		t.Errorf("tags = %v, want the legacy key dropped: labels live in skill_to_label", pres["tags"])
	}
}

func TestUpdateSkill_PresentationOnlyPatchPreservesOrigin(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	skillID := dbfx.Insert(t, "skill", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"name":         "presentation-update-" + t.Name(),
		"description":  "fixture",
		"content":      "# body",
		"config":       testutil.Raw(`'{"origin":{"type":"github","source_url":"https://github.com/acme/skills"}}'::jsonb`),
		"created_by":   testUserID,
	})

	// The client sends the merged config (origin + presentation), as the
	// frontend's writeSkillPresentationMeta does; the server must store both.
	req := withURLParam(newRequest(http.MethodPatch, "/api/skills/"+skillID, map[string]any{
		"config": map[string]any{
			"origin":       map[string]any{"type": "github", "source_url": "https://github.com/acme/skills"},
			"presentation": map[string]any{"category": "data", "icon": "table"},
		},
	}), "id", skillID)
	testutil.Call(t, testHandler.UpdateSkill, req).Want(http.StatusOK)

	cfg := skillConfigFromDB(t, skillID)
	origin, _ := cfg["origin"].(map[string]any)
	if origin["type"] != "github" || origin["source_url"] != "https://github.com/acme/skills" {
		t.Errorf("origin = %v, want preserved github provenance", cfg["origin"])
	}
	pres, _ := cfg["presentation"].(map[string]any)
	if pres["category"] != "data" || pres["icon"] != "table" {
		t.Errorf("presentation = %v, want category data with icon table", pres)
	}

	// And a bad patch is rejected without touching the row.
	bad := withURLParam(newRequest(http.MethodPatch, "/api/skills/"+skillID, map[string]any{
		"config": map[string]any{"presentation": map[string]any{"category": "nope"}},
	}), "id", skillID)
	res := testutil.Call(t, testHandler.UpdateSkill, bad).Want(http.StatusBadRequest)
	if got, _ := res.Map()["error"].(string); !strings.Contains(got, `config.presentation.category: unknown value "nope"`) {
		t.Fatalf("error = %q, want the category error", got)
	}
	if after := skillConfigFromDB(t, skillID); after["presentation"].(map[string]any)["category"] != "data" {
		t.Errorf("rejected patch changed stored config: %v", after)
	}
}

func TestCreateAgentFromTemplate_MaterializedSkillCarriesPresentation(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	cleanupRoleSkill(t, "multica-code-review")

	var created AgentResponse
	testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest("POST", "/api/agents/from-template", map[string]any{
		"template_key": "code-reviewer",
		"runtime_id":   handlerTestRuntimeID(t),
		"name":         "Presentation Reviewer",
		"language":     "en",
	})).Want(http.StatusCreated).JSON(&created)
	cleanupTemplateAgent(t, created.ID)

	roleSkill, ok := service.RoleSkillTemplateByName("multica-code-review")
	if !ok {
		t.Fatal("multica-code-review skill missing from the registry")
	}
	var category, icon, originType string
	dbfx.QueryRow(t, `SELECT config->'presentation'->>'category', config->'presentation'->>'icon', config->'origin'->>'type'
		FROM skill WHERE workspace_id = $1 AND name = $2`, testWorkspaceID, "multica-code-review").
		Scan(&category, &icon, &originType)
	if category != roleSkill.Category || icon != roleSkill.Icon {
		t.Errorf("presentation = %q/%q, want the template's %q/%q", category, icon, roleSkill.Category, roleSkill.Icon)
	}
	if originType != roleSkillOriginType {
		t.Errorf("origin type = %q, want %q kept alongside presentation", originType, roleSkillOriginType)
	}
}

func TestListSkillTemplates_ExposesCategoryAndIcon(t *testing.T) {
	h := &Handler{}
	var out struct {
		Templates []SkillTemplateResponse `json:"templates"`
	}
	testutil.Call(t, h.ListSkillTemplates,
		testutil.JSONRequest(http.MethodGet, "/api/skills/templates", nil)).Want(http.StatusOK).JSON(&out)
	for _, tpl := range out.Templates {
		want, _ := service.RoleSkillTemplateByName(tpl.Name)
		if tpl.Category == "" || tpl.Icon == "" || tpl.Category != want.Category || tpl.Icon != want.Icon {
			t.Errorf("%s: category/icon = %q/%q, want %q/%q", tpl.Name, tpl.Category, tpl.Icon, want.Category, want.Icon)
		}
	}
}

// withMockClawHubImportContent serves a ClawHub skill whose SKILL.md body is
// the given content, so the import path sees author-written frontmatter.
func withMockClawHubImportContent(t *testing.T, skillName, content string) string {
	t.Helper()
	slug := "tagged-helper"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/" + slug:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"skill": map[string]any{
					"slug":        slug,
					"displayName": skillName,
					"summary":     "Imported tagged skill",
					"tags":        map[string]string{"latest": "1.0.0"},
				},
			})
		case "/api/v1/skills/" + slug + "/versions/1.0.0":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": map[string]any{
					"version": "1.0.0",
					"files":   []map[string]any{{"path": "SKILL.md", "size": len(content)}},
				},
			})
		case "/api/v1/skills/" + slug + "/file":
			_, _ = w.Write([]byte(content))
		default:
			t.Fatalf("unexpected ClawHub path: %s", r.URL.String())
		}
	}))
	prev := clawHubAPIBase
	clawHubAPIBase = srv.URL + "/api/v1"
	t.Cleanup(func() {
		clawHubAPIBase = prev
		srv.Close()
	})
	return "https://clawhub.ai/acme/" + slug
}

func TestImportSkill_SeedsPresentationFromFrontmatterMetadata(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	skillName := "url-import-presentation-" + t.Name()
	cleanupSkillByName(t, skillName)
	content := "---\nname: " + skillName + "\ndescription: tagged\nmetadata:\n  category: data\n  icon: NotAnIcon\n  tags: [csv, spreadsheet]\n---\n# Imported\n"
	importURL := withMockClawHubImportContent(t, skillName, content)

	var body SkillImportResult
	testutil.Call(t, testHandler.ImportSkill, newRequestAsUser(testUserID, http.MethodPost, "/api/skills/import", map[string]any{
		"url":         importURL,
		"on_conflict": "fail",
	})).Want(http.StatusCreated).JSON(&body)
	if body.Status != "created" || body.Skill == nil {
		t.Fatalf("result = %+v, want created with a skill", body)
	}

	cfg := skillConfigFromDB(t, body.Skill.ID)
	if _, ok := cfg["origin"]; !ok {
		t.Error("origin provenance missing after import")
	}
	pres, _ := cfg["presentation"].(map[string]any)
	if pres["category"] != "data" {
		t.Errorf("category = %v, want data seeded from frontmatter", pres["category"])
	}
	if _, has := pres["icon"]; has {
		t.Errorf("icon = %v, want the invalid frontmatter icon silently dropped", pres["icon"])
	}
	if _, has := pres["tags"]; has {
		t.Errorf("tags = %v, want frontmatter tags ignored: labels are workspace labels", pres["tags"])
	}
}

// --- Workspace labels on skills ---
//
// Skill labels are issue_label rows scoped resource_type = 'skill', linked via
// skill_to_label. The list embeds them per skill; create accepts label_ids and
// attaches them inside the create transaction.

func insertSkillLabel(t *testing.T, workspaceID, resourceType, name string) string {
	t.Helper()
	return dbfx.Insert(t, "issue_label", testutil.Cols{
		"workspace_id":  workspaceID,
		"resource_type": resourceType,
		"name":          name,
		"description":   "",
		"color":         "#3b82f6",
	})
}

func TestListSkills_EmbedsAttachedLabels(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	labeledID := insertHandlerTestSkill(t, "labels-embed-labeled", "# labeled")
	bareID := insertHandlerTestSkill(t, "labels-embed-bare", "# bare")
	labelID := insertSkillLabel(t, testWorkspaceID, "skill", "embed-"+t.Name())
	dbfx.InsertNoID(t, "skill_to_label", testutil.Cols{"skill_id": labeledID, "label_id": labelID},
		"skill_id = $1", labeledID)

	var resp []SkillSummaryResponse
	testutil.Call(t, testHandler.ListSkills, newRequest(http.MethodGet, "/api/skills", nil)).
		Want(http.StatusOK).JSON(&resp)

	found := map[string]SkillSummaryResponse{}
	for _, s := range resp {
		found[s.ID] = s
	}
	labeled, ok := found[labeledID]
	if !ok {
		t.Fatalf("labeled skill %s missing from list", labeledID)
	}
	if len(labeled.Labels) != 1 || labeled.Labels[0].ID != labelID || labeled.Labels[0].ResourceType != "skill" {
		t.Errorf("labeled.labels = %+v, want the one attached skill label %s", labeled.Labels, labelID)
	}
	bare, ok := found[bareID]
	if !ok {
		t.Fatalf("bare skill %s missing from list", bareID)
	}
	if bare.Labels == nil || len(bare.Labels) != 0 {
		t.Errorf("bare.labels = %#v, want an empty (non-nil) array", bare.Labels)
	}

	// The wire shape must be `[]`, never `null`, so clients can index it blindly.
	raw := testutil.Call(t, testHandler.ListSkills, newRequest(http.MethodGet, "/api/skills", nil)).
		Want(http.StatusOK).Text()
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode raw list: %v", err)
	}
	for _, row := range rows {
		var id string
		_ = json.Unmarshal(row["id"], &id)
		if id == bareID && string(row["labels"]) != "[]" {
			t.Errorf("bare skill labels wire value = %s, want []", row["labels"])
		}
	}
}

func TestCreateSkill_RejectsUnknownOrForeignScopeLabelIDs(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	issueLabelID := insertSkillLabel(t, testWorkspaceID, "issue", "issue-scope-"+t.Name())
	otherWorkspaceID := dbfx.Insert(t, "workspace", testutil.Cols{
		"name":         "Label Scope Workspace " + t.Name(),
		"slug":         "label-scope-" + strings.ToLower(strings.ReplaceAll(t.Name(), "_", "-")),
		"description":  "",
		"issue_prefix": "LSW",
	})
	foreignLabelID := insertSkillLabel(t, otherWorkspaceID, "skill", "foreign-"+t.Name())

	tests := []struct {
		name     string
		labelIDs []string
		wantErr  string
	}{
		{name: "malformed id", labelIDs: []string{"not-a-uuid"}, wantErr: "invalid label_ids"},
		{name: "unknown id", labelIDs: []string{"00000000-0000-0000-0000-000000000001"},
			wantErr: "label_ids: label not found in this workspace"},
		{name: "issue scope", labelIDs: []string{issueLabelID},
			wantErr: "label_ids: label not found in this workspace"},
		{name: "other workspace", labelIDs: []string{foreignLabelID},
			wantErr: "label_ids: label not found in this workspace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := "label-reject-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
			cleanupSkillByName(t, name)
			res := testutil.Call(t, testHandler.CreateSkill, newRequest(http.MethodPost, "/api/skills", map[string]any{
				"name":      name,
				"content":   "# body",
				"label_ids": tt.labelIDs,
			})).Want(http.StatusBadRequest)
			if got, _ := res.Map()["error"].(string); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("error = %q, want %q", got, tt.wantErr)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1 AND name = $2`, testWorkspaceID, name); n != 0 {
				t.Fatalf("rejected create still stored %d row(s)", n)
			}
		})
	}
}

func TestCreateSkill_AttachesLabelIDsInCreateTransaction(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("handler test DB not configured")
	}
	name := "label-create-" + t.Name()
	cleanupSkillByName(t, name)
	firstID := insertSkillLabel(t, testWorkspaceID, "skill", "first-"+t.Name())
	secondID := insertSkillLabel(t, testWorkspaceID, "skill", "second-"+t.Name())

	var created SkillWithFilesResponse
	testutil.Call(t, testHandler.CreateSkill, newRequest(http.MethodPost, "/api/skills", map[string]any{
		"name":      name,
		"content":   "# body",
		"label_ids": []string{firstID, secondID},
	})).Want(http.StatusCreated).JSON(&created)
	dbfx.Cleanup(t, `DELETE FROM skill_to_label WHERE skill_id = $1`, created.ID)

	var listed struct {
		Labels []LabelResponse `json:"labels"`
	}
	testutil.Call(t, testHandler.ListLabelsForSkill,
		withURLParam(newRequest(http.MethodGet, "/api/skills/"+created.ID+"/labels", nil), "id", created.ID)).
		Want(http.StatusOK).JSON(&listed)
	got := map[string]bool{}
	for _, l := range listed.Labels {
		got[l.ID] = true
	}
	if len(listed.Labels) != 2 || !got[firstID] || !got[secondID] {
		t.Fatalf("labels after create = %+v, want exactly %s and %s", listed.Labels, firstID, secondID)
	}
}
