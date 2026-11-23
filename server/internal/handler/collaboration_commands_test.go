package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationWaitingCancelHTTPRejectsMalformedScope(t *testing.T) {
	bare := &Handler{}
	for _, bad := range []any{nil, 1, "01", "1e1", "9223372036854775808"} {
		body := switchHTTPBody()
		body["target_id"], body["expected_collaboration_revision"] = uuid.NewString(), bad
		r := withURLParams(collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), body), "commandId", uuid.NewString())
		response := testutil.Call(t, bare.CancelCollaborationWaitingCommand, r).Want(http.StatusBadRequest)
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cancellation rejection can be cached")
		}
	}
	body := switchHTTPBody()
	body["target_id"], body["expected_collaboration_revision"] = uuid.NewString(), "1"
	r := withURLParams(collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), body), "commandId", uuid.NewString())
	r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, bare.CancelCollaborationWaitingCommand, r).Want(http.StatusForbidden)
}

// The service suite covers real cancellation and participant departure. This
// HTTP regression pins the content-free shape required by the strict client.
func TestCollaborationCommandHTTPRedactedRecoveryOmitsExecutionFields(t *testing.T) {
	f := newConfigHTTPFixture(t)
	external := dbfx.User(t, "Departed command author", "command-author-"+uuid.NewString()+"@example.test")
	key := uuid.NewString()
	full := service.CollaborationQueueReceipt{Status: http.StatusOK, Code: "command_cancelled", IssueID: parseUUID(f.issue),
		RequestID: key, Action: "cancel_command", ProtocolVersion: collaboration.ExecutionProtocolVersion,
		Scope: collaboration.ControlTarget, TargetID: parseUUID(uuid.NewString()), CommandID: parseUUID(uuid.NewString()),
		ChannelID: parseUUID(uuid.NewString()), BatchID: parseUUID(uuid.NewString()), CommandSequence: 1,
		QueueState: "paused", ControlVersion: 1,
		Channels: []service.CollaborationChannelControlResult{}, TargetHolds: []service.CollaborationTargetHoldResult{}}
	encoded, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "collaboration_operation", testutil.Cols{"scope_kind": "issue", "scope_id": f.issue, "actor_user_id": external,
		"request_id": key, "kind": full.Action, "payload_hash": "test-only", "result_code": full.Code, "result": encoded, "completed_at": testutil.Raw("now()")})
	r := withURLParams(collaborationContentRequest(http.MethodGet, f.issue, "", external, key, nil), "requestId", key)
	response := testutil.Call(t, testHandler.GetCollaborationCommandOperation, r).Want(http.StatusOK)
	wire := response.Map()
	if len(wire) != 7 || wire["redacted"] != true || wire["protocol_version"] != float64(collaboration.ExecutionProtocolVersion) ||
		wire["issue_id"] != f.issue || wire["request_id"] != key || wire["action"] != full.Action || wire["code"] != full.Code || wire["status"] != float64(full.Status) {
		t.Fatalf("redacted cancellation recovery disclosed execution or lost original intent: %+v", wire)
	}
	if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" {
		t.Fatal("redacted cancellation inherited ambient workspace or cache state")
	}
}
