package main

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRuntimeBusinessRoutesKeepRunScopeAndMachineAttribution(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	f.comment(t, true, "Original human request")
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := client.List(t.Context(), f.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("managed runtime admission: %+v %v", list, err)
	}
	run := list.RunIDs[0]
	claim, err := client.Claim(t.Context(), run, protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: f.deployment, InstallationID: f.installationID, RuntimeID: f.runtime, BindingID: f.binding.BindingID,
		BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	permit, err := client.Permit(t.Context(), protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/collaboration/runtime/runs/" + run
	var context service.CollaborationRuntimeContext
	f.managed.request(t, permit.BusinessToken, false, http.MethodGet, path+"/context", nil, nil).Want(http.StatusOK).JSON(&context)
	if context.Identity != claim.Identity || util.UUIDToString(context.AgentID) != f.agent || len(context.Inputs) != 1 ||
		context.Inputs[0].AuthorUserID != f.contributor {
		t.Fatalf("business context lost frozen run or human attribution: %+v", context)
	}
	body := map[string]any{"comment_id": uuid.NewString(), "content": "Run-owned progress, without a new root command"}
	var saved, replay service.CollaborationRuntimeCommentReceipt
	f.managed.request(t, permit.BusinessToken, false, http.MethodPost, path+"/comments", body, nil).Want(http.StatusCreated).JSON(&saved)
	f.managed.request(t, permit.BusinessToken, false, http.MethodPost, path+"/comments", body, nil).Want(http.StatusCreated).JSON(&replay)
	if !reflect.DeepEqual(saved, replay) || !saved.CommentID.Valid || len(saved.RunIDs) != 0 || saved.ContinuationRunID.Valid ||
		f.managed.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND author_type='agent' AND author_id=$2`, saved.CommentID, f.agent) != 1 ||
		f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.session) != 1 {
		t.Fatal("run comment borrowed human identity or created duplicate/external execution")
	}
	for _, token := range []string{f.ownerToken, f.binding.DaemonToken, claim.TerminationToken} {
		f.managed.request(t, token, false, http.MethodGet, path+"/context", nil, nil).Want(http.StatusUnauthorized)
	}
	f.managed.request(t, permit.BusinessToken, false, http.MethodGet, "/api/collaboration/runtime/runs/"+uuid.NewString()+"/context", nil, nil).Want(http.StatusNotFound)
	f.control(t, uuid.NewString(), f.controlBody(t, "stop", "issue"), http.StatusOK)
	f.managed.request(t, permit.BusinessToken, false, http.MethodGet, path+"/context", nil, nil).Want(http.StatusForbidden)
	intent, err := client.StopIntent(t.Context(), claim.Identity, claim.TerminationToken)
	if err != nil || !intent.StopRequested {
		t.Fatalf("business revocation removed the independent termination capability: %+v %v", intent, err)
	}
}
