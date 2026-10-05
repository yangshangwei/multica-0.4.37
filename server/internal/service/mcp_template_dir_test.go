package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMcpTemplateDir_InvalidEntriesAreIsolated(t *testing.T) {
	base := `{"schema_version":1,"titles":{"en":"Tool"},"config":{"command":"tool","args":[]}}`
	for name, manifest := range map[string]string{
		"duplicate root":        strings.Replace(base, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		"duplicate escaped key": strings.Replace(base, `"en":"Tool"`, `"en":"Tool","\u0065n":"hidden"`, 1),
		"unknown root":          strings.Replace(base, `"schema_version":1`, `"schema_version":1,"private":"hidden"`, 1),
		"unknown nested":        strings.Replace(base, `"command":"tool"`, `"command":"tool","private":"hidden"`, 1),
		"wrong case":            strings.Replace(base, `"schema_version"`, `"SCHEMA_VERSION"`, 1),
		"unknown schema":        strings.Replace(base, `"schema_version":1`, `"schema_version":2`, 1),
		"trailing document":     base + `{}`,
		"null root":             `null`,
		"too deep":              `{"schema_version":1,"config":` + strings.Repeat(`{"x":`, 40) + `null` + strings.Repeat(`}`, 40) + `}`,
		"static env":            strings.Replace(base, `"command":"tool"`, `"command":"tool","env":{"TOKEN":"private"}`, 1),
		"static headers":        strings.Replace(testMcpHTTPManifest, `"type":"http"`, `"type":"http","headers":{}`, 1),
		"http userinfo":         strings.Replace(testMcpHTTPManifest, `https://search`, `https://private@search`, 1),
		"http query":            strings.Replace(testMcpHTTPManifest, `/mcp"`, `/mcp?token=private"`, 1),
		"http fragment":         strings.Replace(testMcpHTTPManifest, `/mcp"`, `/mcp#private"`, 1),
		"http empty fragment":   strings.Replace(testMcpHTTPManifest, `/mcp"`, `/mcp#"`, 1),
		"http insecure":         strings.Replace(testMcpHTTPManifest, `https://`, `http://`, 1),
		"http opaque":           strings.Replace(testMcpHTTPManifest, `https://search.example.internal/mcp`, `https:private`, 1),
		"bad category":          strings.Replace(base, `"schema_version":1`, `"schema_version":1,"category":"private"`, 1),
		"bad docs":              strings.Replace(base, `"schema_version":1`, `"schema_version":1,"documentation_url":"http://example.com"`, 1),
		"no title":              strings.Replace(base, `"en":"Tool"`, `"en":" "`, 1),
		"unknown locale":        strings.Replace(base, `"en":"Tool"`, `"fr":"Tool"`, 1),
		"blank command":         strings.Replace(base, `"command":"tool"`, `"command":" "`, 1),
		"wrong args":            strings.Replace(base, `"args":[]`, `"args":[3]`, 1),
		"mixed transport":       strings.Replace(base, `"command":"tool"`, `"command":"tool","url":"https://example.com"`, 1),
		"input key hyphen":      strings.Replace(testMcpHTTPManifest, `"key":"token"`, `"key":"access-token"`, 1),
		"input key underscore":  strings.Replace(testMcpHTTPManifest, `"key":"token"`, `"key":"_token"`, 1),
		"input key digit":       strings.Replace(testMcpHTTPManifest, `"key":"token"`, `"key":"1token"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			catalog := McpCatalog{Directory: t.TempDir()}
			writeMcpManifest(t, catalog.Directory, "valid", base)
			writeMcpManifest(t, catalog.Directory, "invalid", manifest)
			items := deploymentMcpTemplates(t, catalog)
			if len(items) != 1 || items[0].Key != "valid" {
				t.Fatalf("invalid entry accepted or healthy entry lost: %d", len(items))
			}
		})
	}
}

func TestMcpTemplateDir_InputDefinitions(t *testing.T) {
	for name, input := range map[string]string{
		"duplicate names":         `{"key":"a","target":{"kind":"env","name":"A"}},{"key":"a","target":{"kind":"env","name":"B"}}`,
		"duplicate target":        `{"key":"a","target":{"kind":"env","name":"A"}},{"key":"b","target":{"kind":"env","name":"A"}}`,
		"unknown target":          `{"key":"a","target":{"kind":"jsonpath","name":"args.0"}}`,
		"missing target":          `{"key":"a"}`,
		"secret argv":             `{"key":"a","secret":true,"target":{"kind":"arg","flag":"--token"}}`,
		"bad env":                 `{"key":"a","target":{"kind":"env","name":"BAD-NAME"}}`,
		"bad flag":                `{"key":"a","target":{"kind":"arg","flag":"--bad flag"}}`,
		"mixed target properties": `{"key":"a","target":{"kind":"env","name":"A","flag":"--a"}}`,
		"header on stdio":         `{"key":"a","target":{"kind":"header","name":"Authorization"}}`,
		"unknown validator":       `{"key":"a","validator":"regexp","target":{"kind":"env","name":"A"}}`,
		"bad input key":           `{"key":"private input","target":{"kind":"env","name":"A"}}`,
		"null secret":             `{"key":"a","secret":null,"target":{"kind":"env","name":"A"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			catalog := McpCatalog{Directory: t.TempDir()}
			writeMcpManifest(t, catalog.Directory, "bad", `{"schema_version":1,"titles":{"en":"Tool"},"config":{"command":"tool"},"inputs":[`+input+`]}`)
			if len(deploymentMcpTemplates(t, catalog)) != 0 {
				t.Fatal("invalid input definition accepted")
			}
		})
	}
	for name, replacement := range map[string]string{
		"env on http":    `"kind":"env","name":"A"`,
		"newline prefix": `"kind":"header","name":"Authorization","prefix":"private\n"`,
		"bad header":     `"kind":"header","name":"Bad Header"`,
	} {
		t.Run(name, func(t *testing.T) {
			catalog := McpCatalog{Directory: t.TempDir()}
			manifest := `{"schema_version":1,"titles":{"en":"Tool"},"config":{"type":"http","url":"https://example.com/mcp"},"inputs":[{"key":"a","secret":false,"target":{` + replacement + `}}]}`
			writeMcpManifest(t, catalog.Directory, "bad", manifest)
			if len(deploymentMcpTemplates(t, catalog)) != 0 {
				t.Fatal("invalid header definition accepted")
			}
		})
	}
}

func TestMcpTemplateDir_FilesystemAndLimits(t *testing.T) {
	t.Run("symlink entry and manifest", func(t *testing.T) {
		catalog := McpCatalog{Directory: t.TempDir()}
		outside := t.TempDir()
		path := writeMcpManifest(t, outside, "external", testMcpHTTPManifest)
		if err := os.Symlink(filepath.Dir(path), filepath.Join(catalog.Directory, "linked-dir")); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(catalog.Directory, "linked-file"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(catalog.Directory, "linked-file", "mcp.json")); err != nil {
			t.Fatal(err)
		}
		writeMcpManifest(t, catalog.Directory, "bad.name", testMcpHTTPManifest)
		if len(deploymentMcpTemplates(t, catalog)) != 0 {
			t.Fatal("unsafe entry accepted")
		}
		if _, err := catalog.Resolve("deployment", "../external", "old", nil); err == nil {
			t.Fatal("path escape accepted")
		}
	})
	t.Run("symlink root", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(t.TempDir(), link); err != nil {
			t.Fatal(err)
		}
		for _, directory := range []string{link, link + string(os.PathSeparator)} {
			if _, err := (McpCatalog{Directory: directory}).List(); !errors.Is(err, ErrMcpCatalogUnavailable) {
				t.Fatal("symlink root accepted")
			}
		}
	})
	t.Run("entry count", func(t *testing.T) {
		catalog := McpCatalog{Directory: t.TempDir()}
		for i := 0; i < 257; i++ {
			if err := os.Mkdir(filepath.Join(catalog.Directory, fmt.Sprint(i)), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := catalog.List(); !errors.Is(err, ErrMcpCatalogUnavailable) {
			t.Fatal("entry limit did not fail entire catalog")
		}
	})
	t.Run("single file", func(t *testing.T) {
		catalog := McpCatalog{Directory: t.TempDir()}
		writeMcpManifest(t, catalog.Directory, "large", strings.Repeat(" ", 65537))
		if len(deploymentMcpTemplates(t, catalog)) != 0 {
			t.Fatal("large manifest accepted")
		}
	})
	t.Run("total bytes including invalid entries", func(t *testing.T) {
		catalog := McpCatalog{Directory: t.TempDir()}
		for i := 0; i < 65; i++ {
			writeMcpManifest(t, catalog.Directory, fmt.Sprintf("item-%03d", i), strings.Repeat("x", 65536))
		}
		if _, err := catalog.List(); !errors.Is(err, ErrMcpCatalogUnavailable) {
			t.Fatal("invalid entries bypassed read budget")
		}
	})
}

func TestMcpTemplateDir_HTTPOptInAndAggregateInputs(t *testing.T) {
	catalog := McpCatalog{Directory: t.TempDir(), AllowHTTP: true}
	writeMcpManifest(t, catalog.Directory, "search", strings.Replace(testMcpHTTPManifest, "https://", "http://", 1))
	item := deploymentMcpTemplates(t, catalog)[0]
	if item.Transport != "http" {
		t.Fatal("HTTP opt-in lost transport")
	}
	for _, language := range []string{"en", "zh"} {
		if requirements := item.RequirementLabels(language); len(requirements) == 0 || !strings.Contains(strings.Join(requirements, " "), "HTTP") {
			t.Fatal("insecure HTTP transport must have a visible localized requirement")
		}
	}
	var definitions []map[string]any
	values := map[string]string{}
	for i := 0; i < 9; i++ {
		key := fmt.Sprintf("input%d", i)
		definitions = append(definitions, map[string]any{"key": key, "target": map[string]any{"kind": "env", "name": fmt.Sprintf("VAR%d", i)}})
		values[key] = strings.Repeat("v", 8192)
	}
	manifest, _ := json.Marshal(map[string]any{"schema_version": 1, "titles": map[string]string{"en": "Tool"}, "config": map[string]string{"command": "tool"}, "inputs": definitions})
	writeMcpManifest(t, catalog.Directory, "tool", string(manifest))
	for _, entry := range deploymentMcpTemplates(t, catalog) {
		if entry.Key == "tool" {
			if _, err := catalog.Resolve("deployment", "tool", entry.Version, values); err == nil {
				t.Fatal("aggregate input limit accepted")
			}
		}
	}
}
