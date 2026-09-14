package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestListSkillTemplates_ReturnsVerbatimCatalogWithoutDatabase(t *testing.T) {
	// An empty handler has no database or transaction handle. Browsing must
	// succeed without materializing or reading workspace skill records.
	h := &Handler{}
	response := testutil.Call(t, h.ListSkillTemplates,
		testutil.JSONRequest(http.MethodGet, "/api/skills/templates", nil)).Want(http.StatusOK)
	var out struct {
		Templates []SkillTemplateResponse `json:"templates"`
	}
	response.JSON(&out)

	wantNames := []string{
		"multica-architecture-decision-record",
		"multica-code-review",
		"multica-documentation-change",
		"multica-progress-report",
		"multica-release-check",
		"multica-requirement-clarification",
		"multica-security-review",
		"multica-test-report",
	}
	if len(out.Templates) != len(wantNames) {
		t.Fatalf("templates = %d, want all %d built-in role skills", len(out.Templates), len(wantNames))
	}
	for i, got := range out.Templates {
		if got.Name != wantNames[i] {
			t.Fatalf("template %d name = %q, want %q", i, got.Name, wantNames[i])
		}
		want, ok := service.RoleSkillTemplateByName(got.Name)
		if !ok {
			t.Fatalf("template %q is absent from the embedded registry", got.Name)
		}
		if got.Version != want.Version || got.Description != want.Description || got.Content != want.Content {
			t.Errorf("%s: version, description, and complete SKILL.md must match the registry verbatim", got.Name)
		}
		if got.Version < 1 || got.Description == "" || got.Content == "" {
			t.Errorf("%s: template must have a version, description, and content", got.Name)
		}
		if got.Files == nil || len(got.Files) != len(want.Files) {
			t.Fatalf("%s: files = %+v, want a non-null array with %d supporting files", got.Name, got.Files, len(want.Files))
		}
		for j, file := range got.Files {
			if file.Path != want.Files[j].Path || file.Content != want.Files[j].Content {
				t.Errorf("%s: supporting file %d must preserve its relative path and content", got.Name, j)
			}
		}
	}

	var raw struct {
		Templates []map[string]json.RawMessage `json:"templates"`
	}
	response.JSON(&raw)
	for i, template := range raw.Templates {
		for _, field := range []string{"id", "workspace_id", "created_by", "config", "origin"} {
			if _, exists := template[field]; exists {
				t.Errorf("%s: catalog entry must not carry workspace identity or provenance field %q", wantNames[i], field)
			}
		}
	}

	var again struct {
		Templates []SkillTemplateResponse `json:"templates"`
	}
	testutil.Call(t, h.ListSkillTemplates,
		testutil.JSONRequest(http.MethodGet, "/api/skills/templates", nil)).Want(http.StatusOK).JSON(&again)
	if !reflect.DeepEqual(out, again) {
		t.Fatal("repeated catalog reads changed the template content or ordering")
	}
}
