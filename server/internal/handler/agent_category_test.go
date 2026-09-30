package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestAgentCategoryRoundTrip(t *testing.T) {
	runtimeID := handlerTestRuntimeID(t)
	created := testutil.Call(t, testHandler.CreateAgent, newRequest(http.MethodPost, "/api/agents", map[string]any{
		"name": fmt.Sprintf("category-%d", time.Now().UnixNano()), "runtime_id": runtimeID, "category": "　研发 🚀  ",
	})).Want(http.StatusCreated).Map()
	id := created["id"].(string)
	dbfx.Cleanup(t, `DELETE FROM agent WHERE id = $1`, id)
	if got := created["category"]; got != "研发 🚀" {
		t.Fatalf("create category = %v, want trimmed Unicode category", got)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"omitted preserves", map[string]any{"description": "Updated description"}, "研发 🚀"},
		{"rename", map[string]any{"category": "  运营  "}, "运营"},
		{"empty clears", map[string]any{"category": ""}, ""},
		{"fifty Unicode code points", map[string]any{"category": strings.Repeat("🚀", 50)}, strings.Repeat("🚀", 50)},
		{"whitespace clears", map[string]any{"category": "　 "}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated := testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+id, tc.body), "id", id)).Want(http.StatusOK).Map()
			if got := updated["category"]; got != tc.want {
				t.Fatalf("update category = %v, want %q", got, tc.want)
			}
			read := testutil.Call(t, testHandler.GetAgent, withURLParam(newRequest(http.MethodGet, "/api/agents/"+id, nil), "id", id)).Want(http.StatusOK).Map()
			if got := read["category"]; got != tc.want {
				t.Fatalf("persisted category = %v, want %q", got, tc.want)
			}
		})
	}
}

func TestAgentCategoryValidation(t *testing.T) {
	runtimeID := handlerTestRuntimeID(t)
	agentID := dbfx.Agent(t, "category-validation", runtimeID)
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"long ASCII", strings.Repeat("a", 51)},
		{"long Unicode", strings.Repeat("🚀", 51)},
		{"newline", "研发\n运营"},
		{"NUL", "研发\x00运营"},
		{"C1 control", "研发\u0085运营"},
		{"number", 123},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, template := range []bool{false, true} {
				body := map[string]any{"name": fmt.Sprintf("invalid-category-%d", time.Now().UnixNano()), "runtime_id": runtimeID, "category": tc.value}
				endpoint := testHandler.CreateAgent
				path := "/api/agents"
				if template {
					body["template_key"] = "product-analyst"
					endpoint = testHandler.CreateAgentFromTemplate
					path += "/from-template"
				}
				count := dbfx.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID)
				testutil.Call(t, endpoint, newRequest(http.MethodPost, path, body)).Want(http.StatusBadRequest)
				if after := dbfx.Count(t, `SELECT count(*) FROM agent WHERE workspace_id = $1`, testWorkspaceID); after != count {
					t.Fatalf("invalid category created an agent: before %d, after %d", count, after)
				}
			}
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+agentID, map[string]any{"category": tc.value}), "id", agentID)).Want(http.StatusBadRequest)
		})
	}
}

func TestAgentCategoryTemplateProvenance(t *testing.T) {
	cleanupRoleSkill(t, "multica-requirement-clarification")
	created := testutil.Call(t, testHandler.CreateAgentFromTemplate, newRequest(http.MethodPost, "/api/agents/from-template", map[string]any{
		"template_key": "product-analyst", "runtime_id": handlerTestRuntimeID(t), "category": " 内容 ",
	})).Want(http.StatusCreated).Map()
	id := created["id"].(string)
	cleanupTemplateAgent(t, id)
	if created["category"] != "内容" {
		t.Fatalf("template create category = %v, want 内容", created["category"])
	}
	role, _ := service.AgentRoleTemplateByKey("product-analyst")
	updated := testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest(http.MethodPut, "/api/agents/"+id, map[string]any{"category": "研发"}), "id", id)).Want(http.StatusOK).Map()
	if updated["category"] != "研发" || updated["template_key"] != role.Key || updated["template_version"] != float64(role.Version) || updated["autonomy_level"] != string(role.Autonomy) || updated["instructions"] != role.Instructions() {
		t.Fatalf("category edit must preserve role provenance and behavior: %#v", updated)
	}
}

func TestAgentCategoryRespectsManagementPermissions(t *testing.T) {
	agentID := dbfx.Agent(t, "category-permissions", handlerTestRuntimeID(t), testutil.Cols{"visibility": "workspace", "permission_mode": "public_to"})
	dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": agentID, "target_type": "workspace", "target_id": testWorkspaceID})
	for _, role := range []string{"member", "admin"} {
		t.Run(role, func(t *testing.T) {
			userID := dbfx.User(t, "Category "+role, "category-"+role+"@multica.test")
			dbfx.Member(t, testWorkspaceID, userID, role)
			want := http.StatusForbidden
			if role == "admin" {
				want = http.StatusOK
			}
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequestAs(userID, http.MethodPut, "/api/agents/"+agentID, map[string]any{"category": "研发"}), "id", agentID)).Want(want)
			got := testutil.Call(t, testHandler.GetAgent, withURLParam(newRequest(http.MethodGet, "/api/agents/"+agentID, nil), "id", agentID)).Want(http.StatusOK).Map()
			wantCategory := ""
			if role == "admin" {
				wantCategory = "研发"
			}
			if got["category"] != wantCategory || got["permission_mode"] != "public_to" {
				t.Fatalf("category management changed unexpected fields: %#v", got)
			}
		})
	}
}
