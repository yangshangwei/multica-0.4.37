package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

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
	for _, template := range service.McpServerTemplates() {
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
		if body["template_key"] != key || body["template_version"] != version {
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
