package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func bindHTTPConfigurationSkill(t *testing.T, f *configHTTPFixture, skill string) {
	t.Helper()
	if err := testHandler.Queries.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: parseUUID(f.agent), SkillID: parseUUID(skill)}); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM agent_skill WHERE skill_id=$1`, skill)
}

func TestCollaborationSharedSkillHTTPSaveAndReimportPreserveNoop(t *testing.T) {
	for _, writer := range []string{"update", "overwrite", "single_file", "binding_set"} {
		t.Run(writer, func(t *testing.T) {
			f := newConfigHTTPFixture(t)
			skill := dbfx.Insert(t, "skill", testutil.Cols{"workspace_id": testWorkspaceID, "name": "HTTP shared skill " + uuid.NewString(), "content": "same instructions", "created_by": testUserID})
			file := dbfx.Insert(t, "skill_file", testutil.Cols{"skill_id": skill, "path": "references/guide.md", "content": "same file"})
			bindHTTPConfigurationSkill(t, f, skill)
			target := f.register(t, collaboration.TargetAgent, f.agent)
			f.drain(t)
			before, err := testHandler.Queries.GetAgent(f.ctx, parseUUID(f.agent))
			if err != nil {
				t.Fatal(err)
			}
			intents := dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent)
			files := []CreateSkillFileRequest{{Path: "references/guide.md", Content: "same file"}}
			switch writer {
			case "update":
				testutil.Call(t, testHandler.UpdateSkill, withURLParam(newRequest(http.MethodPut, "/api/skills/"+skill, map[string]any{"content": "same instructions", "files": files, "config": map[string]any{"presentation": map[string]any{"category": "engineering"}}}), "id", skill)).Want(http.StatusOK)
			case "overwrite":
				if _, err = testHandler.overwriteSkillWithFiles(f.ctx, skillOverwriteInput{WorkspaceID: parseUUID(testWorkspaceID), TargetSkillID: parseUUID(skill), UserID: testUserID, Content: "same instructions", Files: files}); err != nil {
					t.Fatal(err)
				}
			case "single_file":
				testutil.Call(t, testHandler.UpsertSkillFile, withURLParam(newRequest(http.MethodPut, "/api/skills/"+skill+"/files", files[0]), "id", skill)).Want(http.StatusOK)
			case "binding_set":
				testutil.Call(t, testHandler.SetAgentSkills, withURLParam(newRequest(http.MethodPut, "/api/agents/"+f.agent+"/skills", map[string]any{"skill_ids": []string{skill}}), "id", f.agent)).Want(http.StatusOK)
			}
			after, err := testHandler.Queries.GetAgent(f.ctx, parseUUID(f.agent))
			if err != nil {
				t.Fatal(err)
			}
			f.drain(t)
			if before.CollaborationConfigEpoch != after.CollaborationConfigEpoch || intents != dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent) || !reflect.DeepEqual(target, f.target(t, target.ID)) || dbfx.Count(t, `SELECT count(*) FROM skill_file WHERE id=$1 AND content='same file'`, file) != 1 {
				t.Fatal("same bundle or display-only HTTP write changed consent, hold, or file identity")
			}
			// The same HTTP writer must still invalidate an actual file removal.
			testutil.Call(t, testHandler.UpdateSkill, withURLParam(newRequest(http.MethodPut, "/api/skills/"+skill, map[string]any{"files": []CreateSkillFileRequest{}}), "id", skill)).Want(http.StatusOK)
			f.drain(t)
			if f.target(t, target.ID).HoldReason.String != "config_invalid" || dbfx.Count(t, `SELECT count(*) FROM skill_file WHERE skill_id=$1`, skill) != 0 {
				t.Fatal("HTTP replacement failed to remove missing files and hold their target")
			}
		})
	}
}

func TestCollaborationSharedSkillRefreshPersistsReleaseVersion(t *testing.T) {
	f := newConfigHTTPFixture(t)
	slug := "release-" + uuid.NewString()
	name, content := "Versioned skill "+uuid.NewString(), "# same released content"
	config := fmt.Sprintf(`{"origin":{"type":"clawhub","source_url":"https://clawhub.ai/acme/%s","slug":"%s","version":"0.9.0"}}`, slug, slug)
	skill := dbfx.Insert(t, "skill", testutil.Cols{"workspace_id": testWorkspaceID, "name": name, "content": content, "description": "same description", "config": config, "created_by": testUserID})
	bindHTTPConfigurationSkill(t, f, skill)
	target := f.register(t, collaboration.TargetAgent, f.agent)
	f.drain(t)
	installClawHubFixture(t, slug, name, "same description", map[string]string{"SKILL.md": content})
	request := func() *http.Request {
		return withURLParam(newRequest(http.MethodPost, "/api/skills/"+skill+"/refresh", nil), "id", skill)
	}
	testutil.Call(t, testHandler.RefreshSkill, request()).Want(http.StatusOK)
	f.drain(t)
	if dbfx.Count(t, `SELECT count(*) FROM skill WHERE id=$1 AND config->'origin'->>'version'='1.0.0'`, skill) != 1 || f.target(t, target.ID).HoldReason.String != "config_invalid" {
		t.Fatal("same-content upstream release change did not record its version and invalidate consent")
	}
	intents := dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent)
	testutil.Call(t, testHandler.RefreshSkill, request()).Want(http.StatusOK)
	if intents != dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent) {
		t.Fatal("refreshing an unchanged release invalidated execution again")
	}
}

func TestCollaborationSharedSkillPluginVersionInvalidatesBoundTarget(t *testing.T) {
	f := newConfigHTTPFixture(t)
	withPluginsV1Flag(t, testHandler, true)
	cleanupPluginInstallations(t)
	root := t.TempDir()
	name := "shared-version-" + uuid.NewString()
	manifest := strings.ReplaceAll(skillPluginManifest, "pr-review", name)
	writePluginSkillFile(t, root, name, prReviewSkill)
	installation := installSkillPlugin(t, root, manifest)
	rows, err := testHandler.Queries.ListPluginSkills(f.ctx, parseUUID(installation))
	if err != nil || len(rows) != 1 {
		t.Fatalf("plugin skill fixture: %v %v", rows, err)
	}
	dbfx.Cleanup(t, `DELETE FROM skill WHERE id=$1`, rows[0].ID)
	bindHTTPConfigurationSkill(t, f, uuidToString(rows[0].ID))
	target := f.register(t, collaboration.TargetAgent, f.agent)
	f.drain(t)
	upgrade := strings.Replace(manifest, `"version": "1.0.0"`, `"version": "2.0.0"`, 1)
	if installSkillPlugin(t, root, upgrade) != installation {
		t.Fatal("plugin upgrade replaced installation identity")
	}
	f.drain(t)
	if f.target(t, target.ID).HoldReason.String != "config_invalid" {
		t.Fatal("new plugin release with identical skill bytes preserved obsolete consent")
	}
	intents := dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent)
	var version string
	dbfx.QueryRow(t, `SELECT package_version_id::text FROM plugin_installation WHERE id=$1`, installation).Scan(&version)
	body, err := json.Marshal(map[string]any{"version_id": version, "granted_scopes": []string{"issues:read"}})
	if err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.InstallPlugin, pluginHandlerRequest(http.MethodPost, "/plugins", body, map[string]string{"id": testWorkspaceID})).Want(http.StatusCreated)
	if intents != dbfx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_id=$1`, f.agent) {
		t.Fatal("reinstalling the same plugin release invalidated consent again")
	}
}
