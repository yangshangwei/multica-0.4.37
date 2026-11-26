package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/plugincontract"
)

func TestPluginMaterialBaseline(t *testing.T) {
	project := dbfx.Project(t, "Baseline reader")
	issue := dbfx.Issue(t, "Plan", testutil.Cols{"project_id": project})
	install := installPluginForAction(t, []string{plugincontract.ScopeMaterialBaselineReadPrefix + project})
	installation, err := testHandler.Queries.GetPluginInstallation(t.Context(), parseUUID(install))
	if err != nil {
		t.Fatal(err)
	}
	version := util.UUIDToString(installation.PackageVersionID)
	revision := util.UUIDToString(dbid.NewV7())
	content := "Approved requirement"
	digest := material.Digest(content)
	commit := strings.Repeat("a", 40)
	dbfx.InsertNoID(t, "project_material_settings", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "version": 1, "settings": map[string]any{"enabled": true}, "authorized_by": testUserID, "member_id": testUserID}, "project_id=$1", project)
	item := dbfx.Insert(t, "git_material", testutil.Cols{"id": dbid.NewV7(), "workspace_id": testWorkspaceID, "project_id": project, "title": "Requirement", "path": "prd.md", "current_revision_id": revision, "baseline": material.Baseline{ID: revision, RevisionID: revision, Commit: commit, Path: "prd.md", ContentSHA256: digest, VerifiedAt: time.Now()}})
	dbfx.Insert(t, "git_material_revision", testutil.Cols{"id": revision, "material_id": item, "workspace_id": testWorkspaceID, "project_id": project, "kind": "requirement", "payload": material.Revision{ID: revision, Kind: "requirement", Content: content, ContentSHA256: digest}, "settings": "{}", "config_version": 1, "created_by": testUserID, "publication_status": "merged"})
	dbfx.InsertNoID(t, "git_material_plan", testutil.Cols{"material_id": item, "workspace_id": testWorkspaceID, "project_id": project, "baseline_id": revision, "issue_id": issue, "status": "waiting_manual", "requested": false, "authorized_by": testUserID, "member_id": testUserID}, "issue_id=$1", issue)
	request := func(target, installedVersion string) *http.Request {
		req := pluginActionRequest("GET", "/plugin-material", install, nil, map[string]string{"versionID": installedVersion, "issueID": target})
		return req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
	}
	read := func(target, installedVersion string, status int) {
		t.Helper()
		testutil.Call(t, testHandler.GetPluginMaterialBaseline, request(target, installedVersion)).Want(status)
	}
	var out map[string]any
	testutil.Call(t, testHandler.GetPluginMaterialBaseline, request(issue, version)).Want(200).JSON(&out)
	if len(out) != 5 || out["content"] != content || out["issue_id"] != issue || out["baseline_id"] != revision || out["content_sha256"] != digest || out["commit"] != commit {
		t.Fatalf("unexpected plugin response: %v", out)
	}
	read(issue, revision, 404)
	read(dbfx.Issue(t, "Unbound", testutil.Cols{"project_id": project}), version, 404)
	read(dbfx.Issue(t, "Elsewhere", testutil.Cols{"project_id": dbfx.Project(t, "Other")}), version, 403)
	for _, token := range []string{"mpi_test", "mpc_test"} {
		req := request(issue, version)
		req.Header.Set("Authorization", "Bearer "+token)
		testutil.Call(t, testHandler.GetPluginMaterialBaseline, req).Want(403)
	}
	expired := request(issue, version)
	expired = expired.WithContext(auth.WithAuthenticatedPrincipal(expired.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Minute)}))
	testutil.Call(t, testHandler.GetPluginMaterialBaseline, expired).Want(403)
	nonMember := dbfx.User(t, "Not in project workspace", "plugin-baseline-"+revision+"@example.invalid")
	req := request(issue, version)
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: nonMember, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
	testutil.Call(t, testHandler.GetPluginMaterialBaseline, req).Want(403)
	dbfx.Exec(t, "UPDATE git_material SET current_revision_id=$2 WHERE id=$1", item, dbid.NewV7())
	read(issue, version, 404)
	dbfx.Exec(t, "UPDATE git_material SET current_revision_id=$2 WHERE id=$1", item, revision)
	dbfx.Exec(t, "UPDATE git_material_revision SET payload=jsonb_set(payload,'{content}',to_jsonb('changed'::text)) WHERE id=$1", revision)
	read(issue, version, 404)
	dbfx.Exec(t, "UPDATE plugin_installation SET granted_scopes='[]' WHERE id=$1", install)
	read(issue, version, 403)
	dbfx.Exec(t, "UPDATE plugin_installation SET enabled=false WHERE id=$1", install)
	read(issue, version, 403)
}
