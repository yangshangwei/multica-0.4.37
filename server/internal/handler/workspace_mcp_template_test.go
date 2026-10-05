package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Input-value validation belongs to the service suite. These cases lock the
// HTTP request shape and its separation from arbitrary custom configurations.
func TestMcpMarketTemplateInputRequestShape(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		valid bool
	}{
		{"legacy recipe", `{"template_key":"playwright","template_version":"1"}`, true},
		{"empty inputs", `{"template_key":"playwright","template_version":"1","template_inputs":{}}`, true},
		{"database input", `{"template_key":"dbhub","template_version":"1","template_inputs":{"database_url":"postgres://user:private@localhost/db"}}`, true},
		{"postgres current recipe", `{"template_key":"postgres-mcp","template_version":"2","template_inputs":{"database_url":"postgres://user:private@localhost/db"}}`, true},
		{"postgres withdrawn recipe", `{"template_key":"postgres-mcp","template_version":"1","template_inputs":{"database_url":"postgres://user:private@localhost/db"}}`, false},
		{"custom config", `{"config":{"command":"custom"}}`, true},
		{"explicit builtin source", `{"template_source":"builtin","template_key":"playwright","template_version":"1"}`, true},
		{"unknown source", `{"template_source":"private","template_key":"playwright","template_version":"1"}`, false},
		{"empty source", `{"template_source":"","template_key":"playwright","template_version":"1"}`, false},
		{"source without key", `{"template_source":"deployment","template_version":"1"}`, false},
		{"source with custom config", `{"template_source":"deployment","config":{"command":"custom"}}`, false},
		{"missing identity", `{"config":{"command":"custom"},"template_inputs":{"database_url":"private"}}`, false},
		{"null without identity", `{"config":{"command":"custom"},"template_inputs":null}`, false},
		{"forged provenance", `{"template_key":"dbhub","template_version":"1","config":{"command":"custom"},"template_inputs":{"database_url":"postgres://user:private@localhost/db"}}`, false},
		{"null config with provenance", `{"template_key":"playwright","template_version":"1","config":null}`, false},
		{"null inputs", `{"template_key":"dbhub","template_version":"1","template_inputs":null}`, false},
		{"array inputs", `{"template_key":"dbhub","template_version":"1","template_inputs":["private"]}`, false},
		{"number input", `{"template_key":"dbhub","template_version":"1","template_inputs":{"database_url":123}}`, false},
		{"null input", `{"template_key":"dbhub","template_version":"1","template_inputs":{"database_url":null}}`, false},
		{"oversize map", `{"template_key":"dbhub","template_version":"1","template_inputs":{"database_url":"` + strings.Repeat("private", 10000) + `"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req WorkspaceMcpServerRequest
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatal(err)
			}
			err := prepareWorkspaceMcpTemplate(service.McpCatalog{}, &req)
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected request result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("validation error disclosed an input value")
			}
		})
	}
}

func TestMcpMarketDevelopmentRecipeLifecycle(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, tc := range []struct {
		key, version string
		inputs       map[string]string
	}{
		{"serena", "1", map[string]string{"project_path": "/workspace/private-project"}},
		{"codebase-memory", "1", nil}, {"repomix", "1", nil}, {"markitdown", "1", nil},
		{"dbhub", "1", map[string]string{"database_url": "postgres://user:private-password@localhost/db"}},
		{"postgres-mcp", "2", map[string]string{"database_url": "postgresql://user:private-password@localhost/db"}},
		{"gitlab", "1", map[string]string{"gitlab_api_url": "https://gitlab.internal/api/v4", "gitlab_token": "private-gitlab"}},
		{"atlassian", "1", map[string]string{"jira_url": "https://jira.internal", "jira_token": "private-jira"}},
		{"grafana", "1", map[string]string{"grafana_url": "https://grafana.internal", "grafana_token": "private-grafana"}},
		{"kubernetes", "1", map[string]string{"kubeconfig_path": "/etc/private-kubeconfig"}},
		{"mongodb", "1", map[string]string{"mongodb_uri": "mongodb://reader:private-mongo@mongodb.internal/app"}},
		{"redis", "1", map[string]string{"redis_url": "rediss://redis.internal:6379/0", "redis_password": ""}},
		{"clickhouse", "1", map[string]string{"clickhouse_url": "http://clickhouse.internal:8123", "clickhouse_username": "default", "clickhouse_password": ""}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			fields := map[string]any{"name": "market-local-" + tc.key, "template_key": tc.key, "template_version": tc.version}
			if tc.inputs != nil {
				fields["template_inputs"] = tc.inputs
			}
			req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", fields), "id", testWorkspaceID)
			body := testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req).Want(http.StatusCreated).Map()
			id := body["id"].(string)
			dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, id)
			if body["template_key"] != tc.key || body["template_version"] != tc.version || body["transport"] != "stdio" {
				t.Fatal("created recipe lost provenance or transport")
			}
			assertSafeMcpInputSummary(t, body)
			want, err := service.ResolveMcpServerTemplate(tc.key, tc.version, tc.inputs)
			if err != nil {
				t.Fatal(err)
			}
			var raw []byte
			dbfx.QueryRow(t, `SELECT config FROM workspace_mcp_server WHERE id = $1`, id).Scan(&raw)
			var actual map[string]any
			if err := json.Unmarshal(raw, &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, want) {
				t.Fatal("stored configuration differs from the resolved trusted recipe")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM agent_mcp_server WHERE server_id = $1`, id); n != 0 {
				t.Fatal("creating a recipe implicitly assigned it")
			}
			req = withURLParam(newRequest(http.MethodGet, "/mcp-servers", nil), "id", testWorkspaceID)
			var library []map[string]any
			testutil.Call(t, testHandler.ListWorkspaceMcpServers, req).Want(http.StatusOK).JSON(&library)
			for _, item := range library {
				assertSafeMcpInputSummary(t, item)
			}
			agentID := dbfx.Agent(t, "local-recipe-agent", handlerTestRuntimeID(t))
			req = withURLParam(newRequest(http.MethodPost, "/agents/"+agentID+"/mcp-servers", map[string]any{"server_id": id}), "id", agentID)
			var assigned []map[string]any
			testutil.Call(t, testHandler.AddAgentMcpServer, req).Want(http.StatusOK).JSON(&assigned)
			dbfx.Cleanup(t, `DELETE FROM agent_mcp_server WHERE server_id = $1`, id)
			if len(assigned) != 1 || assigned[0]["id"] != id || assigned[0]["enabled"] != true {
				t.Fatal("explicit assignment failed")
			}
			assertSafeMcpInputSummary(t, assigned[0])
			bound, err := testHandler.Queries.ListEnabledAgentMcpServers(t.Context(), parseUUID(agentID))
			if err != nil {
				t.Fatal(err)
			}
			bindings := make([]WorkspaceMcpBinding, 0, len(bound))
			for _, server := range bound {
				bindings = append(bindings, WorkspaceMcpBinding{Name: server.Name, Config: server.Config})
			}
			resolved, err := ResolveAgentMcpConfig(bindings, nil)
			if err != nil {
				t.Fatal(err)
			}
			var effective struct {
				Servers map[string]map[string]any `json:"mcpServers"`
			}
			if err := json.Unmarshal(resolved, &effective); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(effective.Servers[fields["name"].(string)], want) {
				t.Fatal("assigned runtime did not receive the complete recipe inputs")
			}
		})
	}
}

func assertSafeMcpInputSummary(t *testing.T, summary map[string]any) {
	t.Helper()
	for _, field := range []string{"config", "inputs", "template_inputs", "command", "args", "url", "env", "headers"} {
		if _, exists := summary[field]; exists {
			t.Fatalf("summary exposes %s", field)
		}
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-") {
		t.Fatal("summary disclosed a saved input")
	}
}

func TestMcpMarketRejectsInvalidInputRequests(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, fields := range []map[string]any{
		{"template_key": "dbhub", "template_version": "1"},
		{"template_key": "dbhub", "template_version": "1", "template_inputs": map[string]any{"database_url": 123}},
		{"template_key": "serena", "template_version": "1", "template_inputs": map[string]string{"project_path": "private-relative"}},
		{"template_key": "postgres-mcp", "template_version": "2", "template_inputs": map[string]string{"database_url": "private-not-a-url"}},
		{"template_key": "postgres-mcp", "template_version": "2", "template_inputs": map[string]string{"database_url": "postgres://user:private@localhost/db", "command": "private-override"}},
		{"config": map[string]string{"command": "custom"}, "template_inputs": map[string]string{"database_url": "private"}},
		{"template_key": "atlassian", "template_version": "1", "template_inputs": map[string]string{"jira_url": "https://private-jira.internal"}},
		{"template_key": "redis", "template_version": "1", "template_inputs": map[string]string{"redis_url": "redis://private-user:private-password@redis.internal/0"}},
		{"template_key": "clickhouse", "template_version": "1", "template_inputs": map[string]string{"clickhouse_url": "http://clickhouse.internal:8123/private-path", "clickhouse_username": "default"}},
	} {
		fields["name"] = "market-invalid-inputs"
		req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", fields), "id", testWorkspaceID)
		response := testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req)
		if response.Code == http.StatusCreated {
			dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, response.Map()["id"])
		}
		response.Want(http.StatusBadRequest)
		if strings.Contains(response.Body.String(), "outdated recipe version") {
			t.Fatal("input validation was bypassed by an outdated recipe version")
		}
		if strings.Contains(response.Body.String(), "private") {
			t.Fatal("HTTP error disclosed input")
		}
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM workspace_mcp_server WHERE workspace_id = $1 AND name = 'market-invalid-inputs'`, testWorkspaceID); n != 0 {
		t.Fatal("invalid inputs wrote a workspace configuration")
	}
}

func TestMcpMarketCatalogMetadata(t *testing.T) {
	req := testutil.JSONRequest(http.MethodGet, "/templates?language=zh", nil)
	body := testutil.Call(t, (&Handler{}).ListMcpServerTemplates, req).Want(http.StatusOK).Map()
	templates := body["templates"].([]any)
	roster := service.McpServerTemplates()
	if len(templates) != len(roster) {
		t.Fatalf("catalog has %d entries, want %d published recipes", len(templates), len(roster))
	}
	for i, item := range templates {
		template := item.(map[string]any)
		if template["key"] != roster[i].Key || template["version"] != roster[i].Version {
			t.Errorf("wrong recipe identity at position %d: %v", i, template)
		}
		if template["category"] != roster[i].Category {
			t.Errorf("wrong category: %v", template)
		}
		if requirements, ok := template["requirements"].([]any); !ok || len(requirements) == 0 {
			t.Errorf("missing requirements: %v", template)
		}
		if url, ok := template["documentation_url"].(string); !ok || url == "" {
			t.Errorf("missing documentation: %v", template)
		}
	}
}

func TestMcpMarketPublicHTTPRecipeLifecycle(t *testing.T) {
	templates := service.McpServerTemplates()
	if !slices.ContainsFunc(templates, func(template service.McpServerTemplate) bool { return template.Config["type"] == "http" }) {
		// Report a skip rather than a vacuous pass; adding an HTTP recipe re-arms this check.
		t.Skip("the catalog ships no HTTP recipes")
	}
	for _, template := range templates {
		if template.Config["type"] != "http" {
			continue
		}
		t.Run(template.Key, func(t *testing.T) {
			base := "/mcp-servers"
			req := withURLParam(newRequest(http.MethodPost, base, map[string]any{
				"name": template.Key, "template_key": template.Key, "template_version": template.Version,
			}), "id", testWorkspaceID)
			body := testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req).Want(http.StatusCreated).Map()
			id := body["id"].(string)
			dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, id)
			if body["transport"] != "http" || body["template_key"] != template.Key || body["template_version"] != template.Version {
				t.Fatalf("wrong HTTP recipe summary: %v", body)
			}
			for _, field := range []string{"config", "url", "headers", "env"} {
				if _, exists := body[field]; exists {
					t.Fatalf("summary exposes %s", field)
				}
			}
			var config []byte
			dbfx.QueryRow(t, `SELECT config FROM workspace_mcp_server WHERE id = $1`, id).Scan(&config)
			var saved map[string]any
			if err := json.Unmarshal(config, &saved); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved, template.Config) {
				t.Fatalf("HTTP recipe was not preserved: %v", saved)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM agent_mcp_server WHERE server_id = $1`, id); n != 0 {
				t.Fatalf("save implicitly created %d agent bindings", n)
			}
			agentID := dbfx.Agent(t, "public-docs-agent", handlerTestRuntimeID(t))
			req = withURLParam(newRequest(http.MethodPost, "/agents/"+agentID+base, map[string]any{"server_id": id}), "id", agentID)
			var assignments []map[string]any
			testutil.Call(t, testHandler.AddAgentMcpServer, req).Want(http.StatusOK).JSON(&assignments)
			if len(assignments) != 1 || assignments[0]["id"] != id || assignments[0]["enabled"] != true || assignments[0]["transport"] != "http" {
				t.Fatalf("explicit HTTP assignment failed: %v", assignments)
			}
			dbfx.Cleanup(t, `DELETE FROM agent_mcp_server WHERE server_id = $1`, id)
			// The runtime must receive the trusted URL, not just its public summary.
			bound, err := testHandler.Queries.ListEnabledAgentMcpServers(t.Context(), parseUUID(agentID))
			if err != nil {
				t.Fatal(err)
			}
			bindings := make([]WorkspaceMcpBinding, 0, len(bound))
			for _, server := range bound {
				bindings = append(bindings, WorkspaceMcpBinding{Name: server.Name, Config: server.Config})
			}
			resolved, err := ResolveAgentMcpConfig(bindings, nil)
			if err != nil {
				t.Fatal(err)
			}
			var effective struct {
				Servers map[string]map[string]any `json:"mcpServers"`
			}
			if err := json.Unmarshal(resolved, &effective); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(effective.Servers[template.Key], template.Config) {
				t.Fatalf("runtime received the wrong HTTP config: %s", resolved)
			}
		})
	}
}

func TestMcpMarketTemplateLifecycle(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	template := service.McpServerTemplates()[0]
	req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", map[string]any{
		"name": "market-original", "template_key": template.Key, "template_version": "1",
	}), "id", testWorkspaceID)
	body := testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req).Want(http.StatusCreated).Map()
	id := body["id"].(string)
	dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, id)
	assertSource := func(body map[string]any, key, version any) {
		t.Helper()
		var source any
		if key != nil {
			source = "builtin"
		}
		if body["template_source"] != source || body["template_key"] != key || body["template_version"] != version {
			t.Fatalf("wrong source identity: %v", body)
		}
		for _, field := range []string{"config", "command", "args", "url", "headers", "env"} {
			if _, exists := body[field]; exists {
				t.Fatalf("response leaks %s", field)
			}
		}
	}
	assertSource(body, template.Key, "1")
	var config []byte
	dbfx.QueryRow(t, `SELECT config FROM workspace_mcp_server WHERE id = $1`, id).Scan(&config)
	var actual map[string]any
	if err := json.Unmarshal(config, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, template.Config) {
		t.Fatalf("config differs from trusted recipe: %s", config)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_mcp_server WHERE server_id = $1`, id); n != 0 {
		t.Fatalf("unexpected %d automatic bindings", n)
	}
	agentID := dbfx.Agent(t, "market-agent", handlerTestRuntimeID(t))
	dbfx.InsertNoID(t, "agent_mcp_server", testutil.Cols{"agent_id": agentID, "server_id": id, "enabled": false}, "agent_id = $1 AND server_id = $2", agentID, id)
	req = withURLParams(newRequest(http.MethodPut, "/mcp-servers/"+id, map[string]any{"name": "market-renamed"}), "id", testWorkspaceID, "serverId", id)
	assertSource(testutil.Call(t, testHandler.UpdateWorkspaceMcpServer, req).Want(http.StatusOK).Map(), template.Key, "1")
	req = withURLParam(newRequest(http.MethodGet, "/agents/"+agentID+"/mcp-servers", nil), "id", agentID)
	var assignments []map[string]any
	testutil.Call(t, testHandler.ListAgentMcpServers, req).Want(http.StatusOK).JSON(&assignments)
	if len(assignments) != 1 || assignments[0]["enabled"] != false {
		t.Fatalf("assignment changed: %v", assignments)
	}
	assertSource(assignments[0], template.Key, "1")
	// Even replacing config with the same bytes leaves the trusted recipe flow.
	req = withURLParams(newRequest(http.MethodPut, "/mcp-servers/"+id, map[string]any{"config": template.Config}), "id", testWorkspaceID, "serverId", id)
	assertSource(testutil.Call(t, testHandler.UpdateWorkspaceMcpServer, req).Want(http.StatusOK).Map(), nil, nil)
	req = withURLParams(newRequest(http.MethodPut, "/mcp-servers/"+id, map[string]any{"config": json.RawMessage(workspaceMcpTestEntry)}), "id", testWorkspaceID, "serverId", id)
	assertSource(testutil.Call(t, testHandler.UpdateWorkspaceMcpServer, req).Want(http.StatusOK).Map(), nil, nil)
	req = withURLParam(newRequest(http.MethodPost, "/mcp-servers", map[string]any{"name": template.Key, "config": template.Config}), "id", testWorkspaceID)
	custom := testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req).Want(http.StatusCreated).Map()
	dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, custom["id"])
	assertSource(custom, nil, nil)
}

func TestMcpMarketRejectsUntrustedRecipes(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, fields := range []map[string]any{
		{"template_key": "missing", "template_version": "1"},
		{"template_key": "chrome-devtools", "template_version": "stale"},
		{"template_key": "chrome-devtools"},
		{"template_version": "1"},
		{"template_key": "chrome-devtools", "template_version": ""},
		{"template_key": "chrome-devtools", "template_version": "1", "config": map[string]any{"command": "evil"}},
		{"template_key": "chrome-devtools", "template_version": "1", "config": nil},
	} {
		fields["name"] = "market-invalid"
		req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", fields), "id", testWorkspaceID)
		response := testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req)
		if response.Code == http.StatusCreated {
			dbfx.Cleanup(t, `DELETE FROM workspace_mcp_server WHERE id = $1`, response.Map()["id"])
		}
		response.Want(http.StatusBadRequest)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM workspace_mcp_server WHERE workspace_id = $1 AND name = 'market-invalid'`, testWorkspaceID); n != 0 {
		t.Fatalf("invalid recipes wrote %d rows", n)
	}
}

func TestMcpMarketTemplateCreationPermission(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	body := map[string]any{"name": "market-forbidden", "template_key": "playwright", "template_version": "1"}
	memberID := dbfx.User(t, "Market member", "market-member@example.test")
	dbfx.Member(t, testWorkspaceID, memberID, "member")
	req := withURLParam(newRequest(http.MethodPost, "/mcp-servers", body), "id", testWorkspaceID)
	req.Header.Set("X-User-ID", memberID)
	testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req).Want(http.StatusForbidden)
	agentID := dbfx.Agent(t, "market-caller", handlerTestRuntimeID(t))
	taskID := insertHandlerTestTask(t, agentID)
	req = withURLParam(newRequest(http.MethodPost, "/mcp-servers", body), "id", testWorkspaceID)
	testutil.WithHeaders(req, "X-Actor-Source", "task_token", "X-Agent-ID", agentID, "X-Task-ID", taskID)
	testutil.Call(t, testHandler.CreateWorkspaceMcpServer, req).Want(http.StatusForbidden)
}
