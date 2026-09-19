package service

import (
	"regexp"
	"strings"
	"testing"
)

var mcpTemplateKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestMcpServerTemplates_Roster(t *testing.T) {
	templates := McpServerTemplates()
	if len(templates) == 0 {
		t.Fatal("expected a non-empty built-in MCP template roster")
	}

	seen := make(map[string]bool, len(templates))
	for _, template := range templates {
		// Key must be a valid server name, or a save from the pre-filled add
		// form is rejected by the client name validation.
		if !mcpTemplateKeyPattern.MatchString(template.Key) {
			t.Errorf("template key %q does not match the server name grammar ^[A-Za-z0-9_-]+$", template.Key)
		}
		if seen[template.Key] {
			t.Errorf("duplicate template key %q", template.Key)
		}
		seen[template.Key] = true

		// Config must carry a target the add form can express: command (stdio)
		// or url (http). Mirrors parseServerJson's missing-target rule.
		if len(template.Config) == 0 {
			t.Errorf("template %q has empty config", template.Key)
		}
		_, hasCommand := template.Config["command"]
		_, hasURL := template.Config["url"]
		if !hasCommand && !hasURL {
			t.Errorf("template %q config has neither command nor url", template.Key)
		}

		// Every supported picker language must have non-empty copy.
		for _, language := range TemplateLanguages {
			if strings.TrimSpace(template.Title(language)) == "" {
				t.Errorf("template %q has empty title for language %q", template.Key, language)
			}
			if strings.TrimSpace(template.Description(language)) == "" {
				t.Errorf("template %q has empty description for language %q", template.Key, language)
			}
		}
	}
}

// TestMcpServerTemplates_NoSecrets enforces the product decision that the first
// batch ships only keyless templates. It is deliberately strict: adding a
// template that needs an API key, bearer token, or other credential must FAIL
// here first, forcing a maintainer to design the secret-placeholder / required-
// field flow (currently out of scope) instead of quietly shipping a template
// that cannot work without a value the catalog never collects.
func TestMcpServerTemplates_NoSecrets(t *testing.T) {
	secretHint := regexp.MustCompile(`(?i)token|secret|api[_-]?key|password|bearer|authorization`)

	var walk func(key string, value any)
	walk = func(key string, value any) {
		if secretHint.MatchString(key) {
			t.Errorf("template config key %q looks like a credential; keyless-only templates are the current contract", key)
		}
		switch v := value.(type) {
		case map[string]any:
			for k, item := range v {
				walk(k, item)
			}
		case []any:
			for _, item := range v {
				walk(key, item)
			}
		case string:
			if secretHint.MatchString(v) {
				t.Errorf("template config value %q looks like a credential; keyless-only templates are the current contract", v)
			}
		}
	}

	for _, template := range McpServerTemplates() {
		for key, value := range template.Config {
			walk(key, value)
		}
	}
}

func TestMcpServerTemplates_LanguageFallback(t *testing.T) {
	templates := McpServerTemplates()
	first := templates[0]

	// An unsupported language falls back to English rather than the key.
	if got, want := first.Title("fr"), first.Titles["en"]; got != want {
		t.Errorf("unsupported language title = %q, want English fallback %q", got, want)
	}
	if got, want := first.Description("fr"), first.Descriptions["en"]; got != want {
		t.Errorf("unsupported language description = %q, want English fallback %q", got, want)
	}
}
