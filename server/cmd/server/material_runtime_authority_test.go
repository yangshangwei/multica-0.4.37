package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestMaterialRuntimeRouterHostReceiptBoundary(t *testing.T) {
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	q := db.New(testPool)
	project := fx.Project(t, "Runtime material authority")
	rid := fx.Runtime(t, "Selected material runtime", testutil.Cols{"daemon_id": "material-router-" + uuid.NewString(), "visibility": "public"})
	resource := fx.Insert(t, "project_resource", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": project, "resource_type": "github_repo", "resource_ref": `{"url":"https://github.com/fixture/material.git"}`, "position": 0})
	for _, table := range []string{"material_repository_decision", "material_repository_revision", "project_material_repository", "git_material_operation", "git_material_job", "git_material_request", "git_material_revision", "git_material", "project_material_settings"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE project_id=$1", project)
	}
	fx.Cleanup(t, "DELETE FROM agent WHERE workspace_id=$1 AND system_key LIKE $2", testWorkspaceID, "material:"+project+":%")
	call := func(token, method, path string, body any) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(method, path, body)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Workspace-ID", testWorkspaceID)
		r.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityMaterialOperationsV1+","+protocol.DaemonCapabilityMaterialRepositoryV1)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, r)
	}
	base := "/api/projects/" + project + "/materials"
	off := false
	call(testToken, "PUT", base+"/settings", material.SettingsInput{RequestID: uuid.NewString(), Enabled: true, Provider: "github", ResourceID: resource, ExecutionRuntimeID: &rid, Repository: "fixture/material", BaseBranch: "main", RootPath: "engineering", ReviewerIDs: []string{testUserID}, AutoStartPlan: &off}).Want(200)
	svc := service.NewMaterialService(q, testPool, nil, nil)
	if _, e := svc.ProcessNext(t.Context()); e != nil {
		t.Fatal(e)
	}
	var taskID string
	if e := testPool.QueryRow(t.Context(), "SELECT task_id::text FROM git_material_operation WHERE project_id=$1", project).Scan(&taskID); e != nil {
		t.Fatal(e)
	}
	other := fx.User(t, "Other member", uuid.NewString()+"@example.test")
	fx.Member(t, testWorkspaceID, other, "member")
	pat, e := auth.GeneratePATToken()
	if e != nil {
		t.Fatal(e)
	}
	fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": other, "name": "material unrelated member", "token_hash": auth.HashToken(pat), "token_prefix": pat[:8], "expires_at": time.Now().Add(time.Hour)})
	call(pat, "POST", "/api/daemon/runtimes/"+rid+"/tasks/claim", nil).Want(http.StatusForbidden)
	op, e := q.GetMaterialOperationForTask(t.Context(), util.MustParseUUID(taskID))
	if e != nil {
		t.Fatal(e)
	}
	if op.ChallengeHash != "" {
		t.Fatal("unrelated workspace member bootstrapped host challenge")
	}
	// A refused caller cannot strand work owned by the registered host.
	queued, e := q.GetAgentTask(t.Context(), util.MustParseUUID(taskID))
	if e != nil || queued.Status != "queued" {
		t.Fatalf("refused host stranded the real queue: %v %s", e, queued.Status)
	}
	var claim struct {
		Task handler.AgentTaskResponse `json:"task"`
	}
	call(testToken, "POST", "/api/daemon/runtimes/"+rid+"/tasks/claim", nil).Want(200).JSON(&claim)
	if claim.Task.MaterialClaim == nil || claim.Task.MaterialClaim.HostChallenge == "" || claim.Task.MaterialExecution == nil {
		raw, _ := json.Marshal(claim)
		t.Fatalf("missing host claim binding: %s", raw)
	}
	read := call(testToken, "GET", "/api/daemon/tasks/"+taskID+"/material-operation", nil).Want(200).Map()
	raw, _ := json.Marshal(read)
	if strings.Contains(string(raw), claim.Task.MaterialClaim.HostChallenge) {
		t.Fatal("GET leaked host challenge")
	}
	call(claim.Task.AuthToken, "POST", "/api/daemon/tasks/"+taskID+"/material-observations", map[string]any{}).Want(http.StatusUnauthorized)
	call(pat, "GET", "/api/daemon/tasks/"+taskID+"/material-operation", nil).Want(http.StatusForbidden)
	call(testToken, "POST", "/api/daemon/tasks/"+taskID+"/material-observations", protocol.MaterialObservation{ProtocolVersion: 1, TaskID: taskID, OperationID: claim.Task.MaterialExecution.OperationID, RuntimeID: rid, ClaimGeneration: claim.Task.MaterialClaim.ClaimGeneration, ExpectedDigest: claim.Task.MaterialExecution.ExpectedDigest, HostChallenge: strings.Repeat("x", 64), ObservationID: uuid.NewString()}).Want(http.StatusForbidden)
}
