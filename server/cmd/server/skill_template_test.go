package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestSkillTemplateCatalog_RouterAndWorkspaceGates(t *testing.T) {
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	workspaceID := base.Workspace(t, "Skill template catalog", fmt.Sprintf("skill-template-catalog-%d", time.Now().UnixNano()))
	base.Member(t, workspaceID, testUserID, "member")
	otherWorkspaceID := base.Workspace(t, "Other skill template workspace", fmt.Sprintf("skill-template-other-%d", time.Now().UnixNano()))

	for _, tc := range []struct {
		name        string
		token       string
		workspaceID string
		want        int
	}{
		{"member", testToken, workspaceID, http.StatusOK},
		{"unauthenticated", "", workspaceID, http.StatusUnauthorized},
		{"invalid_token", "invalid-token", workspaceID, http.StatusUnauthorized},
		{"nonmember", testToken, otherWorkspaceID, http.StatusNotFound},
		{"missing_workspace", testToken, "", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.JSONRequest(http.MethodGet, "/api/skills/templates", nil)
			req.Header.Set("X-Workspace-ID", tc.workspaceID)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			response := testutil.Call(t, testServer.Config.Handler.ServeHTTP, req).Want(tc.want)
			if tc.want == http.StatusOK {
				var out struct {
					Templates []map[string]any `json:"templates"`
				}
				response.JSON(&out)
				if len(out.Templates) != 8 {
					t.Fatalf("templates = %d, want 8 in an empty workspace", len(out.Templates))
				}
			}
		})
	}

	if count := base.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1`, workspaceID); count != 0 {
		t.Fatalf("browsing the catalog created %d workspace skills", count)
	}
}

func TestSkillTemplateCopy_OrdinaryCreatePreservesSourceAndBindings(t *testing.T) {
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	workspaceID := base.Workspace(t, "Skill template copies", fmt.Sprintf("skill-template-copy-%d", time.Now().UnixNano()))
	base.Member(t, workspaceID, testUserID, "member")
	fx := testutil.New(testPool, workspaceID, testUserID)
	sourceOwnerID := fx.User(t, "Template source owner", fmt.Sprintf("skill-template-owner-%d@example.com", time.Now().UnixNano()))
	template, ok := service.RoleSkillTemplateByName("multica-release-check")
	if !ok {
		t.Fatal("release-check template is missing")
	}
	sourceID := fx.Insert(t, "skill", testutil.Cols{
		"workspace_id": workspaceID,
		"name":         template.Name,
		"description":  "Workspace-specific release checklist",
		"content":      "# Our existing release checks\n\nKeep this customized content.\n",
		"config":       `{"origin":{"type":"builtin_role_skill","name":"multica-release-check","version":2}}`,
		"created_by":   sourceOwnerID,
	})
	files := []handler.CreateSkillFileRequest{
		{Path: "references/release.md", Content: "# Release reference\n\n保留相对路径与原文。  \n"},
		{Path: "scripts/verify.sh", Content: "#!/bin/sh\nprintf '%s\\n' 'isolated verification'\n"},
	}
	for _, file := range files {
		fx.Insert(t, "skill_file", testutil.Cols{"skill_id": sourceID, "path": file.Path, "content": file.Content})
	}
	agentID := fx.Agent(t, "Existing observer", "", testutil.Cols{"autonomy_level": "observer"})
	fx.InsertNoID(t, "agent_skill", testutil.Cols{"agent_id": agentID, "skill_id": sourceID, "enabled": false},
		"agent_id = $1 AND skill_id = $2", agentID, sourceID)
	labelID := fx.Insert(t, "issue_label", testutil.Cols{
		"workspace_id": workspaceID, "resource_type": "skill", "name": "Existing release label", "color": "#888888",
	})
	fx.InsertNoID(t, "skill_to_label", testutil.Cols{"skill_id": sourceID, "label_id": labelID},
		"skill_id = $1 AND label_id = $2", sourceID, labelID)

	call := func(method, path string, body any) *testutil.Response {
		t.Helper()
		req := testutil.WithHeaders(testutil.JSONRequest(method, path, body),
			"Authorization", "Bearer "+testToken, "X-Workspace-ID", workspaceID)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	var sourceBefore handler.SkillWithFilesResponse
	call(http.MethodGet, "/api/skills/"+sourceID, nil).Want(http.StatusOK).JSON(&sourceBefore)
	catalogBefore := call(http.MethodGet, "/api/skills/templates", nil).Want(http.StatusOK).Text()
	if count := fx.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1`, workspaceID); count != 1 {
		t.Fatalf("catalog preview changed skill count to %d, want the single existing skill", count)
	}

	name := template.Name + "-copy"
	description := "团队发布检查"
	content := strings.Replace(template.Content, "name: "+template.Name+"\n", "name: "+name+"\n", 1)
	content = strings.Replace(content, "description: "+strconv.Quote(template.Description)+"\n",
		"description: "+strconv.Quote(description)+"\n", 1) + "\n## 团队补充\n\n先复核隔离环境中的测试结果。  \n"
	request := handler.CreateSkillRequest{
		Name: name, Description: description, Content: content, Files: files,
		Config: map[string]any{"template_source": map[string]any{"name": template.Name, "version": template.Version}},
	}
	var created handler.SkillWithFilesResponse
	call(http.MethodPost, "/api/skills", request).Want(http.StatusCreated).JSON(&created)
	fx.Cleanup(t, `DELETE FROM skill WHERE id = $1`, created.ID)
	if _, err := uuid.Parse(created.ID); err != nil || created.ID == sourceID {
		t.Fatalf("copy must have a new UUID, got %q (source %q)", created.ID, sourceID)
	}
	if created.WorkspaceID != workspaceID || created.CreatedBy == nil || *created.CreatedBy != testUserID {
		t.Fatalf("copy identity must belong to the submitting member and workspace: %+v", created.SkillResponse)
	}
	if created.Name != name || created.Description != description || created.Content != content {
		t.Fatal("ordinary POST did not preserve the edited name, description, and complete content")
	}
	wantConfig := map[string]any{"template_source": map[string]any{"name": template.Name, "version": float64(template.Version)}}
	if !reflect.DeepEqual(created.Config, wantConfig) {
		t.Fatalf("copy config = %#v, want informational source without official provenance", created.Config)
	}
	if len(created.Files) != len(files) {
		t.Fatalf("copy files = %d, want %d", len(created.Files), len(files))
	}
	for i, file := range created.Files {
		if file.SkillID != created.ID || file.Path != files[i].Path || file.Content != files[i].Content {
			t.Errorf("copy attachment %d did not preserve its relative path, content, or new owner", i)
		}
	}

	var persisted handler.SkillWithFilesResponse
	call(http.MethodGet, "/api/skills/"+created.ID, nil).Want(http.StatusOK).JSON(&persisted)
	if !reflect.DeepEqual(created, persisted) {
		t.Fatal("copy detail does not match the committed create response, including supporting files")
	}

	// A stale client may submit the same name again. It must get a conflict,
	// preserving both the previously created copy and the source bundle.
	conflict := request
	conflict.Content = "replacement that must not be saved"
	conflict.Files = []handler.CreateSkillFileRequest{{Path: "references/release.md", Content: "must not overwrite"}}
	call(http.MethodPost, "/api/skills", conflict).Want(http.StatusConflict)
	var afterConflict handler.SkillWithFilesResponse
	call(http.MethodGet, "/api/skills/"+created.ID, nil).Want(http.StatusOK).JSON(&afterConflict)
	if !reflect.DeepEqual(persisted, afterConflict) {
		t.Fatal("name conflict changed the already-created skill or its supporting files")
	}
	if count := fx.Count(t, `SELECT count(*) FROM skill WHERE workspace_id = $1`, workspaceID); count != 2 {
		t.Fatalf("create and conflicting retry left %d skills, want source and one copy", count)
	}
	if count := fx.Count(t, `SELECT count(*) FROM agent_skill WHERE skill_id = $1`, created.ID); count != 0 {
		t.Fatalf("copy was automatically bound to %d agents", count)
	}
	if count := fx.Count(t, `SELECT count(*) FROM skill_to_label WHERE skill_id = $1`, created.ID); count != 0 {
		t.Fatalf("copy inherited %d labels from the existing workspace skill", count)
	}
	var sourceAfter handler.SkillWithFilesResponse
	call(http.MethodGet, "/api/skills/"+sourceID, nil).Want(http.StatusOK).JSON(&sourceAfter)
	if !reflect.DeepEqual(sourceBefore, sourceAfter) {
		t.Fatal("creating a template copy changed the existing workspace skill")
	}
	var enabled bool
	fx.QueryRow(t, `SELECT enabled FROM agent_skill WHERE agent_id = $1 AND skill_id = $2`, agentID, sourceID).Scan(&enabled)
	if enabled {
		t.Fatal("creating a copy enabled the source skill's disabled binding")
	}
	if count := fx.Count(t, `SELECT count(*) FROM skill_to_label WHERE skill_id = $1 AND label_id = $2`, sourceID, labelID); count != 1 {
		t.Fatal("creating a copy changed the source skill's label")
	}
	var autonomy string
	fx.QueryRow(t, `SELECT autonomy_level FROM agent WHERE id = $1`, agentID).Scan(&autonomy)
	if autonomy != "observer" {
		t.Fatalf("using the release template changed the existing agent's autonomy to %q", autonomy)
	}
	if catalogAfter := call(http.MethodGet, "/api/skills/templates", nil).Want(http.StatusOK).Text(); catalogAfter != catalogBefore {
		t.Fatal("creating and editing a copy changed the server's template catalog")
	}
}
