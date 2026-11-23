package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func resourceHTTPQuery(r *http.Request, values url.Values) *http.Request {
	r.URL.RawQuery = values.Encode()
	return r
}

func TestCollaborationResourceOptionsRespectExistingVisibility(t *testing.T) {
	f := newConfigHTTPFixture(t)
	viewer := dbfx.User(t, "Resource organizer", "resource-organizer-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, viewer, "member")
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": viewer}, "user_id=$1", viewer)
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue, "user_id": viewer}, "issue_id=$1 AND user_id=$2", f.issue, viewer)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='member',assignee_id=$2 WHERE id=$1`, f.issue, viewer)
	runtime := dbfx.Runtime(t, "Visible resource runtime", testutil.Cols{"owner_id": viewer})
	agent := dbfx.Agent(t, "Visible resource", runtime, testutil.Cols{"owner_id": viewer})
	request := func(workspace string) *http.Request {
		return resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", viewer, "", nil), url.Values{"execution_workspace_id": {workspace}})
	}
	var result service.CollaborationResourceOptions
	response := testutil.Call(t, testHandler.GetCollaborationResourceOptions, request(testWorkspaceID)).Want(http.StatusOK)
	response.JSON(&result)
	if len(result.Items) != 1 || result.Items[0].ResourceID != parseUUID(agent) || !result.Items[0].Available {
		t.Fatalf("ordinary member saw private resources or lost own candidate: %+v", result)
	}
	if strings.Contains(response.Body.String(), f.runtime) || strings.Contains(response.Body.String(), f.agent) {
		t.Fatal("candidate discovery widened the ordinary resource directory")
	}
	unknownWorkspace := dbfx.Workspace(t, "Other resources", "other-resources-"+uuid.NewString())
	testutil.Call(t, testHandler.GetCollaborationResourceOptions, request(unknownWorkspace)).Want(http.StatusNotFound)
	dbfx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue, testUserID)
	testutil.Call(t, testHandler.GetCollaborationResourceOptions, request(testWorkspaceID)).Want(http.StatusNotFound)
}

func TestCollaborationResourceHTTPExplicitScopeAndOwnerIdentity(t *testing.T) {
	f := newConfigHTTPFixture(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	target := f.register(t, collaboration.TargetAgent, f.agent)
	installation := uuid.NewString()
	binding := dbfx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": testUserID,
		"execution_workspace_id": testWorkspaceID, "runtime_id": f.runtime, "installation_id": installation, "public_key": make([]byte, 32)})
	query := url.Values{"target_id": {uuidToString(target.ID)}, "agent_id": {f.agent}}
	r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", testUserID, "", nil), query)
	r.Header.Set("X-Workspace-ID", uuid.NewString())
	var identity service.CollaborationResourceIdentityView
	response := testutil.Call(t, testHandler.GetCollaborationResourceIdentity, r).Want(http.StatusOK)
	response.JSON(&identity)
	if identity.Identity == nil || uuidToString(identity.Identity.BindingID) != binding || response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" {
		t.Fatal("identity did not use an explicit, uncached issue scope")
	}
	consent := switchHTTPBody()
	consent["agent_id"], consent["expected_target_epoch"], consent["expected_config_version"] = f.agent, "1", "1"
	consent["identity"] = identity.Identity
	key := uuid.NewString()
	testutil.Call(t, testHandler.ConsentCollaborationTarget, withURLParams(collaborationContentRequest(http.MethodPost, f.issue, "", testUserID, key, consent), "targetId", uuidToString(target.ID))).Want(http.StatusCreated)
	for _, current := range []bool{true, false} {
		if !current {
			dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue, testUserID)
		}
		var receipt map[string]json.RawMessage
		response := testutil.Call(t, testHandler.GetCollaborationResourceOperation, withURLParams(collaborationContentRequest(http.MethodGet, f.issue, "", testUserID, "", nil), "requestId", key)).Want(http.StatusOK)
		response.JSON(&receipt)
		if !current && (len(receipt) != 7 || string(receipt["redacted"]) != "true") {
			t.Fatalf("departure leaked receipt details: %s", response.Body.String())
		}
	}
}

func TestCollaborationResourceOptionsExplainDisabledOwner(t *testing.T) {
	f := newConfigHTTPFixture(t)
	owner := dbfx.User(t, "Disabled resource owner", "disabled-resource-owner-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, owner, "member")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue, "user_id": owner}, "issue_id=$1 AND user_id=$2", f.issue, owner)
	runtime := dbfx.Runtime(t, "Disabled owner runtime", testutil.Cols{"owner_id": owner})
	agent := dbfx.Agent(t, "Disabled owner's agent", runtime, testutil.Cols{"owner_id": owner})
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, owner)
	request := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", testUserID, "", nil), url.Values{"execution_workspace_id": {testWorkspaceID}})
	var result service.CollaborationResourceOptions
	testutil.Call(t, testHandler.GetCollaborationResourceOptions, request).Want(http.StatusOK).JSON(&result)
	for _, item := range result.Items {
		if item.ResourceID == parseUUID(agent) {
			if item.Available || item.Reason != "resource_owner_unavailable" {
				t.Fatalf("existing disabled participant received an invitation instruction: %+v", item)
			}
			return
		}
	}
	t.Fatal("visible disabled-owner resource missing from the management candidate list")
}

func TestCollaborationResourceHTTPRefusesMachineBeforeDatabase(t *testing.T) {
	bare := &Handler{}
	for _, handler := range []http.HandlerFunc{bare.GetCollaborationResources, bare.GetCollaborationParticipants, bare.GetCollaborationResourceOptions, bare.GetCollaborationResourceIdentity, bare.GetCollaborationResourceOperation} {
		r := withURLParams(collaborationContentRequest(http.MethodGet, uuid.NewString(), "", testUserID, "", nil), "requestId", uuid.NewString())
		r = resourceHTTPQuery(r, url.Values{"execution_workspace_id": {uuid.NewString()}, "target_id": {uuid.NewString()}, "agent_id": {uuid.NewString()}})
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
		testutil.Call(t, handler, r).Want(http.StatusNotFound)
	}
}
