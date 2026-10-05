package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ListMcpServerTemplates reads the embedded roster and touches no database, so
// it runs on a bare handler regardless of DATABASE_URL. Router-level membership
// gating is a middleware concern covered by the shared auth tests.
func listMcpServerTemplatesForTest(t *testing.T, language string) (int, []McpServerTemplateResponse, string) {
	t.Helper()

	path := "/api/workspaces/" + testWorkspaceID + "/mcp-servers/templates"
	if language != "" {
		path += "?language=" + language
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req = withURLParam(req, "id", testWorkspaceID)
	w := httptest.NewRecorder()

	(&Handler{}).ListMcpServerTemplates(w, req)

	var body struct {
		Templates []McpServerTemplateResponse `json:"templates"`
	}
	raw := w.Body.String()
	if w.Code == http.StatusOK {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			t.Fatalf("decode response: %v (%s)", err, raw)
		}
	}
	return w.Code, body.Templates, raw
}

func TestListMcpServerTemplates_InputMetadataIsPublicOnly(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		_, _, raw := listMcpServerTemplatesForTest(t, language)
		var body struct {
			Templates []map[string]json.RawMessage `json:"templates"`
		}
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, template := range body.Templates {
			var key string
			if err := json.Unmarshal(template["key"], &key); err != nil {
				t.Fatal(err)
			}
			var inputs []map[string]any
			if err := json.Unmarshal(template["inputs"], &inputs); err != nil {
				t.Fatalf("%s lacks an inputs collection", key)
			}
			if key != "serena" && key != "dbhub" && key != "postgres-mcp" {
				continue
			}
			found[key] = true
			if len(inputs) != 1 {
				t.Fatalf("%s should expose one required input", key)
			}
			input := inputs[0]
			if len(input) != 5 || input["required"] != true || input["secret"] != (key != "serena") || input["label"] == "" || input["description"] == "" {
				t.Fatalf("%s has invalid public input metadata", key)
			}
			if strings.Contains(string(template["config"]), "DATABASE_URI") {
				t.Fatal("input destination leaked into public recipe")
			}
		}
		if len(found) != 3 {
			t.Fatal("missing templates with required inputs")
		}
	}
}

func TestListMcpServerTemplates_ReturnsCatalog(t *testing.T) {
	code, templates, raw := listMcpServerTemplatesForTest(t, "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", code, raw)
	}
	if len(templates) == 0 {
		t.Fatal("expected a non-empty catalog")
	}

	var chrome *McpServerTemplateResponse
	for i := range templates {
		if templates[i].Key == "chrome-devtools" {
			chrome = &templates[i]
		}
	}
	if chrome == nil {
		t.Fatalf("chrome-devtools missing from catalog: %s", raw)
	}
	if chrome.Title == "" || chrome.Description == "" {
		t.Errorf("chrome-devtools missing localized copy: %+v", chrome)
	}
	// The full config is served (the opposite of the write-only library) so the
	// client can pre-fill the add form.
	if chrome.Config["command"] != "npx" {
		t.Errorf("chrome-devtools config command = %v, want npx", chrome.Config["command"])
	}
}

func TestListMcpServerTemplates_LanguageSelectsCopy(t *testing.T) {
	_, en, _ := listMcpServerTemplatesForTest(t, "en")
	_, zh, _ := listMcpServerTemplatesForTest(t, "zh")

	titleByKey := func(list []McpServerTemplateResponse, key string) string {
		for _, template := range list {
			if template.Key == key {
				return template.Description
			}
		}
		return ""
	}
	// sequential-thinking is the entry whose copy actually differs by language.
	if titleByKey(en, "sequential-thinking") == titleByKey(zh, "sequential-thinking") {
		t.Error("expected sequential-thinking description to differ between en and zh")
	}
}

func TestListMcpServerTemplates_UnknownLanguageFallsBackToEnglish(t *testing.T) {
	_, en, _ := listMcpServerTemplatesForTest(t, "en")
	_, fallback, _ := listMcpServerTemplatesForTest(t, "xx")

	if len(en) != len(fallback) || len(en) == 0 {
		t.Fatalf("catalog size mismatch: en=%d fallback=%d", len(en), len(fallback))
	}
	for i := range en {
		if en[i].Description != fallback[i].Description {
			t.Errorf("template %q: unknown language did not fall back to English (%q vs %q)",
				en[i].Key, fallback[i].Description, en[i].Description)
		}
	}
}
