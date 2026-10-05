package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func writeDeploymentMcpManifest(t *testing.T, directory, key, title string) string {
	t.Helper()
	path := filepath.Join(directory, key, "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"titles":         map[string]string{"en": title, "zh": "部署检索"},
		"config":         map[string]any{"type": "http", "url": "https://search.example.internal/mcp"},
		"inputs": []any{map[string]any{
			"key": "access_token", "labels": map[string]string{"en": "Access token"}, "required": true,
			"secret": true, "target": map[string]string{"kind": "header", "name": "Authorization", "prefix": "Bearer "},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func deploymentMcpVersion(t *testing.T, h *Handler, key string) string {
	t.Helper()
	body := testutil.Call(t, h.ListMcpServerTemplates, testutil.JSONRequest(http.MethodGet, "/templates", nil)).Want(http.StatusOK).Map()
	for _, entry := range body["templates"].([]any) {
		template := entry.(map[string]any)
		if template["key"] == key && template["source"] == "deployment" {
			return template["version"].(string)
		}
	}
	t.Fatal("deployment template missing from catalog")
	return ""
}

func TestListMcpServerTemplates_DeploymentPublicBoundaryAndSameKey(t *testing.T) {
	directory := t.TempDir()
	writeDeploymentMcpManifest(t, directory, "playwright", "Company Search")
	h := &Handler{McpCatalog: service.McpCatalog{Directory: directory}}
	for _, language := range []string{"en", "zh"} {
		req := testutil.JSONRequest(http.MethodGet, "/templates?language="+language, nil)
		response := testutil.Call(t, h.ListMcpServerTemplates, req).Want(http.StatusOK)
		body := response.Map()
		found := map[string]map[string]any{}
		for _, entry := range body["templates"].([]any) {
			template := entry.(map[string]any)
			if template["key"] == "playwright" {
				found[template["source"].(string)] = template
			}
		}
		if len(found) != 2 || found["builtin"]["config"] == nil {
			t.Fatal("same-key builtin and deployment entries must coexist, preserving builtin public config")
		}
		deployment := found["deployment"]
		if deployment["transport"] != "http" || !strings.HasPrefix(deployment["version"].(string), "sha256:") {
			t.Fatalf("missing deployment identity/transport: %v", deployment)
		}
		if _, exists := deployment["config"]; exists {
			t.Fatal("deployment catalog must omit its runtime configuration")
		}
		input := deployment["inputs"].([]any)[0].(map[string]any)
		if len(input) != 5 || input["secret"] != true || input["required"] != true {
			t.Fatalf("unexpected public input metadata: %v", input)
		}
		for _, secret := range []string{"search.example.internal", "Authorization", "Bearer ", directory} {
			if strings.Contains(response.Body.String(), secret) {
				t.Fatal("catalog response disclosed deployment configuration or paths")
			}
		}
	}
}

func TestListMcpServerTemplates_DeploymentRootFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-root")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &Handler{McpCatalog: service.McpCatalog{Directory: path}}
	body := testutil.Call(t, h.ListMcpServerTemplates, testutil.JSONRequest(http.MethodGet, "/templates", nil)).Want(http.StatusServiceUnavailable).Map()
	if body["code"] != "mcp_catalog_unavailable" || strings.Contains(body["error"].(string), path) {
		t.Fatalf("root failure must use a safe stable error: %v", body)
	}
}

func TestMcpMarketDeploymentLifecycleAndLegacySummaries(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	directory := t.TempDir()
	path := writeDeploymentMcpManifest(t, directory, "playwright", "Company Search")
	h := *testHandler
	h.McpCatalog = service.McpCatalog{Directory: directory}
	version := deploymentMcpVersion(t, &h, "playwright")
	assertSummary := func(t *testing.T, summary map[string]any, source, key, version any) {
		t.Helper()
		if summary["template_source"] != source || summary["template_key"] != key || summary["template_version"] != version {
			t.Fatalf("unexpected provenance: %v", summary)
		}
		assertSafeMcpInputSummary(t, summary)
	}
	for _, capability := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "source aware"}[capability], func(t *testing.T) {
			query := ""
			if capability {
				query = "?mcp_source_version=1"
			}
			fields := map[string]any{"name": "deployment-playwright", "template_source": "deployment", "template_key": "playwright", "template_version": version,
				"template_inputs": map[string]string{"access_token": "private-password"}}
			req := withURLParam(newRequest(http.MethodPost, "/mcp-servers"+query, fields), "id", testWorkspaceID)
			created := testutil.Call(t, h.CreateWorkspaceMcpServer, req).Want(http.StatusCreated).Map()
			id := created["id"].(string)
			dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, id)
			var source, key, expectedVersion any
			if capability {
				source, key, expectedVersion = "deployment", "playwright", version
			}
			assertSummary(t, created, source, key, expectedVersion)
			if created["transport"] != "http" {
				t.Fatal("source lookup silently used the builtin recipe")
			}
			var raw []byte
			var savedSource, savedKey, savedVersion string
			dbfx.QueryRow(t, `SELECT config, template_source, template_key, template_version FROM workspace_mcp_server WHERE id = $1`, id).Scan(&raw, &savedSource, &savedKey, &savedVersion)
			if savedSource != "deployment" || savedKey != "playwright" || savedVersion != version {
				t.Fatal("saved provenance must be independent of response capability")
			}
			var config map[string]any
			if err := json.Unmarshal(raw, &config); err != nil {
				t.Fatal(err)
			}
			if config["headers"].(map[string]any)["Authorization"] != "Bearer private-password" {
				t.Fatal("saved runtime config is missing the secret input")
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM agent_mcp_server WHERE server_id = $1`, id); count != 0 {
				t.Fatal("creating a deployment recipe must not assign it")
			}
			agentID := dbfx.Agent(t, "deployment-agent", handlerTestRuntimeID(t))
			req = withURLParam(newRequest(http.MethodPost, "/agents/mcp-servers"+query, map[string]any{"server_id": id}), "id", agentID)
			var assigned []map[string]any
			testutil.Call(t, h.AddAgentMcpServer, req).Want(http.StatusOK).JSON(&assigned)
			dbfx.Cleanup(t, `DELETE FROM agent_mcp_server WHERE server_id = $1`, id)
			if len(assigned) != 1 || assigned[0]["id"] != id || assigned[0]["enabled"] != true {
				t.Fatalf("explicit assignment failed: %v", assigned)
			}
			assertSummary(t, assigned[0], source, key, expectedVersion)
			for _, toggle := range []bool{false, true} {
				req = withURLParams(newRequest(http.MethodPut, "/agents/mcp-servers/"+id+query, map[string]any{"enabled": toggle}), "id", agentID, "serverId", id)
				testutil.Call(t, h.SetAgentMcpServerEnabled, req).Want(http.StatusOK).JSON(&assigned)
				assertSummary(t, assigned[0], source, key, expectedVersion)
			}
			// A catalog withdrawal cannot revoke a saved snapshot or its binding.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			req = withURLParams(newRequest(http.MethodPut, "/mcp-servers/"+id+query, map[string]any{"name": "deployment-renamed"}), "id", testWorkspaceID, "serverId", id)
			renamed := testutil.Call(t, h.UpdateWorkspaceMcpServer, req).Want(http.StatusOK).Map()
			assertSummary(t, renamed, source, key, expectedVersion)
			if renamed["name"] != "deployment-renamed" || renamed["id"] != id {
				t.Fatal("rename changed identity")
			}
			for _, list := range []struct {
				handler http.HandlerFunc
				id      string
			}{{h.ListWorkspaceMcpServers, testWorkspaceID}, {h.ListAgentMcpServers, agentID}} {
				req = withURLParam(newRequest(http.MethodGet, "/mcp-servers"+query, nil), "id", list.id)
				var summaries []map[string]any
				testutil.Call(t, list.handler, req).Want(http.StatusOK).JSON(&summaries)
				found := false
				for _, summary := range summaries {
					if summary["id"] == id {
						found = true
						assertSummary(t, summary, source, key, expectedVersion)
					}
				}
				if !found {
					t.Fatal("catalog withdrawal removed saved configuration")
				}
			}
			bound, err := h.Queries.ListEnabledAgentMcpServers(t.Context(), parseUUID(agentID))
			if err != nil || len(bound) != 1 {
				t.Fatalf("withdrawal changed runtime assignment: %v", err)
			}
			var runtime map[string]any
			if err := json.Unmarshal(bound[0].Config, &runtime); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(config, runtime) {
				t.Fatal("withdrawal changed the saved runtime config")
			}
			// Replacing config discards all three provenance fields, even if bytes match.
			req = withURLParams(newRequest(http.MethodPut, "/mcp-servers/"+id+query, map[string]any{"config": config}), "id", testWorkspaceID, "serverId", id)
			assertSummary(t, testutil.Call(t, h.UpdateWorkspaceMcpServer, req).Want(http.StatusOK).Map(), nil, nil, nil)
			if count := dbfx.Count(t, `SELECT count(*) FROM workspace_mcp_server WHERE id = $1 AND template_source IS NULL AND template_key IS NULL AND template_version IS NULL`, id); count != 1 {
				t.Fatal("config replacement retained persisted provenance")
			}
			path = writeDeploymentMcpManifest(t, directory, "playwright", "Company Search")
		})
	}
}

func TestMcpMarketDeploymentCreationErrors(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, state := range []string{"changed", "removed", "invalid", "root failure"} {
		t.Run(state, func(t *testing.T) {
			directory := t.TempDir()
			path := writeDeploymentMcpManifest(t, directory, "playwright", "Before")
			h := *testHandler
			h.McpCatalog = service.McpCatalog{Directory: directory}
			version := deploymentMcpVersion(t, &h, "playwright")
			wantStatus, wantCode := http.StatusConflict, "mcp_template_unavailable"
			switch state {
			case "changed":
				writeDeploymentMcpManifest(t, directory, "playwright", "After")
				wantCode = "mcp_template_changed"
			case "removed":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "invalid":
				if err := os.WriteFile(path, []byte(`{"invalid":"private-password"}`), 0o600); err != nil {
					t.Fatal(err)
				}
			case "root failure":
				h.McpCatalog.Directory = path
				wantStatus, wantCode = http.StatusServiceUnavailable, "mcp_catalog_unavailable"
			}
			fields := map[string]any{"name": "invalid-deployment", "template_source": "deployment", "template_key": "playwright", "template_version": version,
				"template_inputs": map[string]string{"access_token": "private-password"}}
			req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", fields), "id", testWorkspaceID)
			response := testutil.Call(t, h.CreateWorkspaceMcpServer, req).Want(wantStatus)
			if response.Map()["code"] != wantCode || strings.Contains(response.Body.String(), "private-password") || strings.Contains(response.Body.String(), directory) {
				t.Fatalf("expected safe %s response: %s", wantCode, response.Body.String())
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM workspace_mcp_server WHERE workspace_id = $1 AND name = 'invalid-deployment'`, testWorkspaceID); count != 0 {
				t.Fatal("unavailable/stale template created a saved configuration")
			}
			// Legacy builtin creation remains independent of deployment directory health.
			req = withURLParam(newRequest(http.MethodPost, "/mcp-servers", map[string]any{"name": "builtin-independent", "template_key": "playwright", "template_version": "1"}), "id", testWorkspaceID)
			builtin := testutil.Call(t, h.CreateWorkspaceMcpServer, req).Want(http.StatusCreated).Map()
			dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, builtin["id"])
			if builtin["template_source"] != "builtin" || builtin["template_key"] != "playwright" || builtin["transport"] != "stdio" {
				t.Fatal("legacy request failed to resolve the builtin source")
			}
		})
	}
}

func TestMcpMarketDeploymentCreationPermission(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	directory := t.TempDir()
	writeDeploymentMcpManifest(t, directory, "company-search", "Company Search")
	h := *testHandler
	h.McpCatalog = service.McpCatalog{Directory: directory}
	version := deploymentMcpVersion(t, &h, "company-search")
	fields := map[string]any{"name": "deployment-forbidden", "template_source": "deployment", "template_key": "company-search", "template_version": version,
		"template_inputs": map[string]string{"access_token": "private-password"}}
	memberID := dbfx.User(t, "Deployment member", "deployment-member@example.test")
	dbfx.Member(t, testWorkspaceID, memberID, "member")
	req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", fields), "id", testWorkspaceID)
	req.Header.Set("X-User-ID", memberID)
	testutil.Call(t, h.CreateWorkspaceMcpServer, req).Want(http.StatusForbidden)
	agentID := dbfx.Agent(t, "deployment-caller", handlerTestRuntimeID(t))
	taskID := insertHandlerTestTask(t, agentID)
	req = withURLParam(newRequest(http.MethodPost, "/mcp-servers", fields), "id", testWorkspaceID)
	testutil.WithHeaders(req, "X-Actor-Source", "task_token", "X-Agent-ID", agentID, "X-Task-ID", taskID)
	testutil.Call(t, h.CreateWorkspaceMcpServer, req).Want(http.StatusForbidden)
	if count := dbfx.Count(t, `SELECT count(*) FROM workspace_mcp_server WHERE workspace_id = $1 AND name = 'deployment-forbidden'`, testWorkspaceID); count != 0 {
		t.Fatal("unauthorized deployment adoption wrote a configuration")
	}
}

func TestMcpMarketTemplateSourceMigration(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	tx, err := testPool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	// A temporary relation shadows the real table on this connection so the
	// migration can exercise pre-upgrade rows without modifying shared fixtures.
	if _, err := tx.Exec(t.Context(), `CREATE TEMP TABLE workspace_mcp_server (template_key TEXT) ON COMMIT DROP;
		INSERT INTO workspace_mcp_server (template_key) VALUES ('playwright'), (NULL)`); err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile("../../migrations/512_workspace_mcp_template_source.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), string(up)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), `INSERT INTO workspace_mcp_server (template_key, template_source) VALUES ('playwright', 'deployment')`); err != nil {
		t.Fatal(err)
	}
	// Replay must not reclassify deployment copies, including same-key copies.
	if _, err := tx.Exec(t.Context(), string(up)); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/512_workspace_mcp_template_source.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), string(down)); err != nil {
		t.Fatal(err)
	}
	var builtins, deployments, custom int
	if err := tx.QueryRow(t.Context(), `SELECT
		count(*) FILTER (WHERE template_key = 'playwright' AND template_source = 'builtin'),
		count(*) FILTER (WHERE template_key = 'playwright' AND template_source = 'deployment'),
		count(*) FILTER (WHERE template_key IS NULL AND template_source IS NULL)
		FROM workspace_mcp_server`).Scan(&builtins, &deployments, &custom); err != nil {
		t.Fatal(err)
	}
	if builtins != 1 || deployments != 1 || custom != 1 {
		t.Fatalf("migration/replay/rollback changed provenance: builtin=%d deployment=%d custom=%d", builtins, deployments, custom)
	}
}
