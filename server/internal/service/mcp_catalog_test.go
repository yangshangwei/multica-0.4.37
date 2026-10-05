package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testMcpHTTPManifest = `{"schema_version":1,"titles":{"zh":"公司搜索"},"config":{"type":"http","url":"https://search.example.internal/mcp"},"inputs":[{"key":"token","labels":{"zh":"令牌"},"required":true,"target":{"kind":"header","name":"Authorization","prefix":"Bearer "}}]}`

func writeMcpManifest(t *testing.T, directory, key, data string) string {
	t.Helper()
	dir := filepath.Join(directory, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func deploymentMcpTemplates(t *testing.T, catalog McpCatalog) []McpServerTemplate {
	t.Helper()
	all, err := catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	var result []McpServerTemplate
	for _, item := range all {
		if item.Source == "deployment" {
			result = append(result, item)
		}
	}
	return result
}

func TestMcpCatalog_DynamicSnapshotAndIdentity(t *testing.T) {
	catalog := McpCatalog{Directory: t.TempDir()}
	path := writeMcpManifest(t, catalog.Directory, "playwright", testMcpHTTPManifest)
	items := deploymentMcpTemplates(t, catalog)
	if len(items) != 1 {
		t.Fatalf("expected one deployment, got %d", len(items))
	}
	item := items[0]
	if item.Transport != "http" || item.Config != nil || item.Title("en") != "公司搜索" || !item.Inputs[0].Secret {
		t.Fatal("public metadata must identify transport, hide configuration and normalize localized/header defaults")
	}
	if !strings.HasPrefix(item.Version, "sha256:") || len(item.Version) != 71 {
		t.Fatalf("bad content version: %s", item.Version)
	}
	config, err := catalog.Resolve("deployment", "playwright", item.Version, map[string]string{"token": "private-token"})
	if err != nil || !reflect.DeepEqual(config["headers"], map[string]any{"Authorization": "Bearer private-token"}) {
		t.Fatalf("header mapping failed: %v", err)
	}
	builtin, err := catalog.Resolve("", "playwright", "1", nil)
	if err != nil || builtin["command"] != "npx" {
		t.Fatal("deployment name shadowed builtin")
	}
	if err := os.WriteFile(path, []byte(strings.Replace(testMcpHTTPManifest, "公司搜索", "新名称", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Resolve("deployment", "playwright", item.Version, nil); !errors.Is(err, ErrMcpTemplateChanged) {
		t.Fatalf("expected changed: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Resolve("deployment", "playwright", item.Version, nil); !errors.Is(err, ErrMcpTemplateUnavailable) {
		t.Fatalf("expected unavailable: %v", err)
	}
	writeMcpManifest(t, catalog.Directory, "playwright", `{broken`)
	if _, err := catalog.Resolve("deployment", "playwright", item.Version, nil); !errors.Is(err, ErrMcpTemplateUnavailable) {
		t.Fatalf("expected invalid entry unavailable: %v", err)
	}
}

func TestMcpCatalog_VersionCanonicalization(t *testing.T) {
	catalog := McpCatalog{Directory: t.TempDir()}
	writeMcpManifest(t, catalog.Directory, "search", testMcpHTTPManifest)
	want := deploymentMcpTemplates(t, catalog)[0].Version
	var manifest map[string]any
	if err := json.Unmarshal([]byte(testMcpHTTPManifest), &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["category"] = "other"
	manifest["inputs"].([]any)[0].(map[string]any)["secret"] = true
	manifest["inputs"].([]any)[0].(map[string]any)["validator"] = "string"
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeMcpManifest(t, catalog.Directory, "search", string(data))
	if got := deploymentMcpTemplates(t, McpCatalog{Directory: catalog.Directory})[0].Version; got != want {
		t.Fatal("formatting/default normalization changed content version")
	}
	manifest["descriptions"] = map[string]string{"en": "New copy"}
	data, _ = json.Marshal(manifest)
	writeMcpManifest(t, catalog.Directory, "search", string(data))
	if got := deploymentMcpTemplates(t, catalog)[0].Version; got == want {
		t.Fatal("metadata mutation did not change content version")
	}
}

func TestMcpCatalog_TypedInputMappingAndIsolation(t *testing.T) {
	catalog := McpCatalog{Directory: t.TempDir()}
	manifest := `{"schema_version":1,"titles":{"en":"Tool"},"config":{"command":"tool","args":["serve"]},"inputs":[{"key":"path","required":true,"validator":"absolute_path","target":{"kind":"arg","flag":"--project"}},{"key":"extra","target":{"kind":"arg"}},{"key":"database","secret":true,"validator":"database_url","target":{"kind":"env","name":"DSN"}}]}`
	writeMcpManifest(t, catalog.Directory, "tool", manifest)
	item := deploymentMcpTemplates(t, catalog)[0]
	inputs := map[string]string{"path": `C:\work\project`, "extra": "one value with spaces", "database": "postgresql://private@localhost/db"}
	want := map[string]any{"command": "tool", "args": []any{"serve", "--project", `C:\work\project`, "one value with spaces"}, "env": map[string]any{"DSN": "postgresql://private@localhost/db"}}
	got, err := catalog.Resolve("deployment", "tool", item.Version, inputs)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("typed maps failed: %v", err)
	}
	got["args"].([]any)[0] = "mutated"
	got["env"].(map[string]any)["DSN"] = "mutated"
	again, err := catalog.Resolve("deployment", "tool", item.Version, inputs)
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatal("resolution leaked mutations")
	}
	for _, invalid := range []map[string]string{
		nil, {"path": "private/relative"}, {"path": "/ok", "database": "private://invalid"}, {"path": "/ok", "extra": "private\nvalue"}, {"path": "/ok", "extra": strings.Repeat("private", 1500)}, {"path": "/ok", "private-unknown": "private"},
	} {
		if result, err := catalog.Resolve("deployment", "tool", item.Version, invalid); err == nil || result != nil || strings.Contains(err.Error(), "private") {
			t.Fatal("invalid input accepted or disclosed")
		}
	}
}

func TestMcpCatalog_RootErrorsAndBuiltinIndependence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "private-root")
	if err := os.WriteFile(file, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog := McpCatalog{Directory: file}
	if _, err := catalog.List(); !errors.Is(err, ErrMcpCatalogUnavailable) || strings.Contains(err.Error(), "private") {
		t.Fatalf("root error must be typed and private: %v", err)
	}
	if _, err := catalog.Resolve("deployment", "search", "old", nil); !errors.Is(err, ErrMcpCatalogUnavailable) {
		t.Fatal("deployment creation hid unhealthy root")
	}
	if _, err := catalog.Resolve("builtin", "playwright", "1", nil); err != nil {
		t.Fatal("builtin resolution depended on root health")
	}
	if _, err := catalog.Resolve("unknown", "playwright", "1", nil); err == nil {
		t.Fatal("unknown source accepted")
	}
	for _, directory := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		items, err := (McpCatalog{Directory: directory}).List()
		if err != nil || len(items) != len(McpServerTemplates()) {
			t.Fatalf("empty directory behavior: %v", err)
		}
		for _, item := range items {
			if item.Source != "builtin" || item.Transport == "" {
				t.Fatal("builtin identity not normalized")
			}
		}
	}
}
