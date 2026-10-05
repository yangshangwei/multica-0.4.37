package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMcpServerTemplates_SharedResolverCopiesRecipe(t *testing.T) {
	for _, template := range McpServerTemplates() {
		if template.Key != "dbhub" {
			continue
		}
		template.Config["env"] = map[string]any{"STATIC": "unchanged"}
		original, err := json.Marshal(template.Config)
		if err != nil {
			t.Fatal(err)
		}
		inputs := map[string]string{"database_url": "postgres://user:private@localhost/db"}
		first, err := resolveMcpTemplateInputs(template, inputs)
		if err != nil {
			t.Fatal(err)
		}
		first["args"].([]any)[0] = "changed"
		first["env"].(map[string]any)["DSN"] = "changed"
		after, err := json.Marshal(template.Config)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(original) {
			t.Fatal("shared resolver changed its retained recipe")
		}
		second, err := resolveMcpTemplateInputs(template, inputs)
		if err != nil || second["env"].(map[string]any)["DSN"] != inputs["database_url"] {
			t.Fatal("one request affected another resolution")
		}
		return
	}
	t.Fatal("dbhub recipe missing")
}

func TestMcpServerTemplates_PostgresSDKCompatibility(t *testing.T) {
	inputs := map[string]string{"database_url": "postgresql://localhost/db"}
	t.Run("version 2 keeps the FastMCP v1 import available", func(t *testing.T) {
		config, err := ResolveMcpServerTemplate("postgres-mcp", "2", inputs)
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"--with", "mcp<2", "postgres-mcp", "--access-mode=restricted"}
		if config["command"] != "uvx" || !reflect.DeepEqual(config["args"], want) {
			t.Fatal("Postgres recipe must constrain its MCP SDK to the compatible major version")
		}
	})
	t.Run("withdrawn version 1 cannot create another broken server", func(t *testing.T) {
		if _, err := ResolveMcpServerTemplate("postgres-mcp", "1", inputs); err == nil {
			t.Fatal("withdrawn Postgres recipe version 1 was accepted")
		}
	})
}

func TestMcpServerTemplates_DevelopmentRoster(t *testing.T) {
	want := map[string]struct{ category, version string }{
		"serena": {"coding", "1"}, "codebase-memory": {"coding", "1"}, "repomix": {"coding", "1"},
		"markitdown": {"documentation", "1"}, "dbhub": {"database", "1"}, "postgres-mcp": {"database", "2"},
	}
	for _, template := range McpServerTemplates() {
		expected, ok := want[template.Key]
		if !ok {
			continue
		}
		if template.Category != expected.category || template.Version != expected.version {
			t.Errorf("%s: want %s recipe %s", template.Key, expected.category, expected.version)
		}
		delete(want, template.Key)
	}
	for key := range want {
		t.Errorf("requested development template %q is missing", key)
	}
}

func TestMcpServerTemplates_ResolveInputs(t *testing.T) {
	for _, tc := range []struct {
		key, version string
		inputs       map[string]string
		want         map[string]any
	}{
		{"serena", "1", map[string]string{"project_path": "/work/project with spaces"}, map[string]any{"command": "uvx", "args": []any{"--python", "3.13", "--from", "serena-agent", "serena", "start-mcp-server", "--project", "/work/project with spaces"}}},
		{"codebase-memory", "1", nil, map[string]any{"command": "codebase-memory-mcp", "args": []any{}}},
		{"repomix", "1", nil, map[string]any{"command": "npx", "args": []any{"-y", "repomix", "--mcp"}}},
		{"markitdown", "1", nil, map[string]any{"command": "uvx", "args": []any{"markitdown-mcp"}}},
		{"dbhub", "1", map[string]string{"database_url": "postgres://user:private@localhost/db?sslmode=disable"}, map[string]any{"command": "npx", "args": []any{"-y", "@bytebase/dbhub@latest", "--transport", "stdio"}, "env": map[string]any{"DSN": "postgres://user:private@localhost/db?sslmode=disable"}}},
		{"postgres-mcp", "2", map[string]string{"database_url": "postgresql://user:private@localhost/db"}, map[string]any{"command": "uvx", "args": []any{"--with", "mcp<2", "postgres-mcp", "--access-mode=restricted"}, "env": map[string]any{"DATABASE_URI": "postgresql://user:private@localhost/db"}}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			got, err := ResolveMcpServerTemplate(tc.key, tc.version, tc.inputs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected resolved configuration for %s", tc.key)
			}
			got["command"] = "overridden"
			if args := got["args"].([]any); len(args) > 0 {
				args[0] = "overridden"
			}
			if env, ok := got["env"].(map[string]any); ok {
				env["DATABASE_URI"] = "overridden"
			}
			again, err := ResolveMcpServerTemplate(tc.key, tc.version, tc.inputs)
			if err != nil || !reflect.DeepEqual(again, tc.want) {
				t.Fatal("resolved configuration mutated the catalog")
			}
		})
	}
}

func TestMcpServerTemplates_RejectInvalidInputs(t *testing.T) {
	for _, tc := range []struct {
		name, key, version string
		inputs             map[string]string
	}{
		{"unknown template", "missing", "1", nil},
		{"stale version", "serena", "2", map[string]string{"project_path": "/work"}},
		{"missing path", "serena", "1", nil},
		{"blank path", "serena", "1", map[string]string{"project_path": " \t "}},
		{"missing database", "postgres-mcp", "2", nil},
		{"unknown input", "dbhub", "1", map[string]string{"database_url": "postgres://user:private@localhost/db", "private-key": "private"}},
		{"no-input recipe", "playwright", "1", map[string]string{"private-key": "private"}},
		{"newline", "dbhub", "1", map[string]string{"database_url": "private\nvalue"}},
		{"carriage return", "dbhub", "1", map[string]string{"database_url": "private\rvalue"}},
		{"nul", "postgres-mcp", "2", map[string]string{"database_url": "private\x00value"}},
		{"too long", "serena", "1", map[string]string{"project_path": strings.Repeat("private", 2048)}},
		{"relative project", "serena", "1", map[string]string{"project_path": "private/project"}},
		{"flag project", "serena", "1", map[string]string{"project_path": "--private"}},
		{"invalid database url", "dbhub", "1", map[string]string{"database_url": "private"}},
		{"parser error", "dbhub", "1", map[string]string{"database_url": "postgres://private%zz@localhost/db"}},
		{"unsupported scheme", "dbhub", "1", map[string]string{"database_url": "https://private/db"}},
		{"wrong database", "postgres-mcp", "2", map[string]string{"database_url": "mysql://private@localhost/db"}},
		{"missing database host", "postgres-mcp", "2", map[string]string{"database_url": "postgresql:///private"}},
		{"empty sqlite path", "dbhub", "1", map[string]string{"database_url": "sqlite:///"}},
		{"remote sqlite host", "dbhub", "1", map[string]string{"database_url": "sqlite://private/db"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveMcpServerTemplate(tc.key, tc.version, tc.inputs)
			if err == nil || got != nil {
				t.Fatal("invalid input was accepted")
			}
			if tc.name != "unknown template" && tc.name != "stale version" && strings.Contains(err.Error(), "outdated recipe version") {
				t.Fatal("input validation was bypassed by an outdated recipe version")
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("error echoed user input")
			}
		})
	}
}

func TestMcpServerTemplates_CrossPlatformInputs(t *testing.T) {
	for _, path := range []string{"/work/project", `C:\work\project`, "C:/work/project", `\\host\share\project`} {
		if _, err := ResolveMcpServerTemplate("serena", "1", map[string]string{"project_path": path}); err != nil {
			t.Errorf("absolute project path was rejected: %v", err)
		}
	}
	for _, dsn := range []string{
		"postgres://localhost/db", "postgresql://localhost/db", "mysql://localhost/db", "mariadb://localhost/db", "sqlserver://localhost/db", "oracle://localhost/service",
		"sqlite:///var/lib/data.db", "sqlite:///C:/data/db.sqlite", "sqlite:///:memory:", "sqlite://./relative/db.sqlite",
	} {
		if _, err := ResolveMcpServerTemplate("dbhub", "1", map[string]string{"database_url": dsn}); err != nil {
			t.Errorf("documented database URL was rejected: %v", err)
		}
	}
}

func TestMcpServerTemplates_InputMetadata(t *testing.T) {
	want := map[string]string{"serena": "project_path", "dbhub": "database_url", "postgres-mcp": "database_url"}
	for _, template := range McpServerTemplates() {
		key, needsInput := want[template.Key]
		if !needsInput {
			if len(template.Inputs) != 0 {
				t.Errorf("unexpected inputs on %s", template.Key)
			}
			continue
		}
		if len(template.Inputs) != 1 {
			t.Fatalf("%s must declare exactly one input", template.Key)
		}
		input := template.Inputs[0]
		if input.Key != key || !input.Required || input.Secret != (key == "database_url") {
			t.Errorf("wrong input contract for %s", template.Key)
		}
		for _, language := range TemplateLanguages {
			if input.Labels[language] == "" || input.Descriptions[language] == "" {
				t.Errorf("missing %s input copy for %s", language, template.Key)
			}
		}
		if input.Label("unknown") != input.Label("en") || input.Description("unknown") != input.Description("en") {
			t.Errorf("missing input language fallback for %s", template.Key)
		}
	}
}
