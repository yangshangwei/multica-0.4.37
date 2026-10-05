package service

import (
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var mcpTemplateKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var mcpTemplatePlaceholderPattern = regexp.MustCompile(`\$(\{|[A-Za-z_][A-Za-z0-9_]*)|\{\{|<[^>]+>|%[A-Za-z_][A-Za-z0-9_]*%`)

func TestMcpServerTemplates_Placeholders(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"$MCP_COMMAND", true},
		{"--root=$WORKSPACE_ROOT", true},
		{"%WORKSPACE_ROOT%/project", true},
		{"${WORKSPACE_ROOT}", true},
		{"{{WORKSPACE_ROOT}}", true},
		{"<workspace-root>", true},
		{"https://example.com/$PROJECT/mcp", true},
		{"npx", false},
		{"@playwright/mcp@latest", false},
		{"https://learn.microsoft.com/api/mcp", false},
		{"--limit=50%", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if got := mcpTemplatePlaceholderPattern.MatchString(tc.value); got != tc.want {
				t.Errorf("placeholder in %q = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestMcpServerTemplates_ExcludesExternalDocumentation(t *testing.T) {
	for _, template := range McpServerTemplates() {
		switch template.Key {
		case "microsoft-learn", "deepwiki":
			t.Errorf("external documentation recipe %q must not be built in", template.Key)
		}
	}
}

// This is the offline publishing gate for compiled catalog content. Do not use
// localization fallback to conceal missing copy, or call live providers here.
func TestMcpServerTemplates_PublishingContract(t *testing.T) {
	versionPattern := regexp.MustCompile(`^[1-9][0-9]*$`)
	for _, template := range McpServerTemplates() {
		t.Run(template.Key, func(t *testing.T) {
			if !versionPattern.MatchString(template.Version) {
				t.Errorf("recipe version must be a positive integer, got %q", template.Version)
			}
			switch template.Category {
			case "browser", "reasoning", "documentation", "coding", "database", "collaboration", "operations":
			default:
				t.Errorf("category %q needs a shared UI label before publication", template.Category)
			}
			for _, language := range TemplateLanguages {
				if strings.TrimSpace(template.Titles[language]) == "" || strings.TrimSpace(template.Descriptions[language]) == "" {
					t.Errorf("missing explicit %s title or description", language)
				}
				labels := template.Requirements[language]
				if len(labels) == 0 {
					t.Errorf("missing explicit %s runtime requirements", language)
				}
				for _, label := range labels {
					if strings.TrimSpace(label) == "" {
						t.Errorf("empty %s runtime requirement", language)
					}
				}
			}
			assertCatalogHTTPSURL(t, template.DocumentationURL)
			_, hasCommand := template.Config["command"]
			_, hasURL := template.Config["url"]
			if hasCommand == hasURL {
				t.Fatal("recipe must contain exactly one command or URL target")
			}
			allowed := map[string]bool{"type": true}
			if env, present := template.Config["env"]; present {
				allowed["env"] = true
				expected, reviewed := intranetStaticEnv[template.Key]
				if !reviewed || !reflect.DeepEqual(env, expected) {
					t.Error("public environment must match reviewed static controls and explicit credential-clearing values")
				}
			}
			if hasURL {
				allowed["url"] = true
				endpoint, ok := template.Config["url"].(string)
				if !ok || template.Config["type"] != "http" {
					t.Fatal("remote recipe must declare type=http and a string URL")
				}
				assertCatalogHTTPSURL(t, endpoint)
			} else {
				allowed["command"], allowed["args"] = true, true
				command, ok := template.Config["command"].(string)
				if !ok || strings.TrimSpace(command) == "" || mcpTemplatePlaceholderPattern.MatchString(command) {
					t.Error("stdio recipe must have a concrete command")
				}
				if transport, present := template.Config["type"]; present && transport != "stdio" {
					t.Error("command recipes may only declare stdio transport")
				}
				args, ok := template.Config["args"].([]any)
				if !ok {
					t.Error("stdio recipe must have explicit arguments")
				}
				for _, arg := range args {
					value, ok := arg.(string)
					if !ok || strings.TrimSpace(value) == "" || mcpTemplatePlaceholderPattern.MatchString(value) {
						t.Error("arguments must be non-empty strings without input placeholders")
					}
				}
			}
			for field := range template.Config {
				if !allowed[field] {
					t.Errorf("public config field %q is not allowed; credentials must use write-only inputs", field)
				}
			}
			for _, input := range template.Inputs {
				if (input.argument == "") == (input.environment == "") {
					t.Error("input must declare exactly one server-owned destination")
				}
				if input.Secret && input.environment == "" {
					t.Error("secret inputs must be passed through environment variables")
				}
			}
		})
	}
}

func assertCatalogHTTPSURL(t *testing.T, value string) {
	t.Helper()
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(value, "{}<> ") || mcpTemplatePlaceholderPattern.MatchString(value) {
		t.Errorf("catalog URL must be concrete HTTPS without credentials, query or fragment: %q", value)
	}
}

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

// TestMcpServerTemplates_NoSecrets keeps public recipes credential-free.
// Required credentials are collected separately through declared write-only
// inputs, never embedded as values or unresolved placeholders in Config.
func TestMcpServerTemplates_NoSecrets(t *testing.T) {
	secretHint := regexp.MustCompile(`(?i)token|secret|api[_-]?key|password|bearer|authorization`)

	var walk func(key string, value any)
	walk = func(key string, value any) {
		// Empty values explicitly clear inherited credentials. The publishing
		// contract separately restricts these keys to the reviewed static map.
		if secretHint.MatchString(key) && value != "" {
			t.Errorf("template config key %q may only contain an empty credential-clearing value", key)
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
