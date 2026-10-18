package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// The service test owns the account/permission matrix; this checks that the
// HTTP recovery projection contains only the permitted redacted wire fields.
func TestCollaborationSwitchHTTPRedactedRecoveryOmitsExecutionFields(t *testing.T) {
	f := newConfigHTTPFixture(t)
	external := dbfx.User(t, "Departed Switch owner", "switch-owner-"+uuid.NewString()+"@example.test")
	key := uuid.NewString()
	full := service.CollaborationQueueReceipt{Status: http.StatusOK, Code: "switch_consent_revoked", IssueID: parseUUID(f.issue),
		RequestID: key, Action: "revoke_target_switch_consent", ProtocolVersion: collaboration.ExecutionProtocolVersion,
		TargetID: parseUUID(uuid.NewString()), ProposalID: parseUUID(uuid.NewString()), ChannelID: parseUUID(uuid.NewString()),
		Channels: []service.CollaborationChannelControlResult{}, TargetHolds: []service.CollaborationTargetHoldResult{}}
	encoded, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "collaboration_operation", testutil.Cols{"scope_kind": "issue", "scope_id": f.issue, "actor_user_id": external,
		"request_id": key, "kind": full.Action, "payload_hash": "test-only", "result_code": full.Code, "result": encoded, "completed_at": testutil.Raw("now()")})
	r := withURLParams(collaborationContentRequest(http.MethodGet, f.issue, "", external, key, nil), "requestId", key)
	response := testutil.Call(t, testHandler.GetCollaborationSwitchOperation, r).Want(http.StatusOK)
	wire := response.Map()
	if len(wire) != 7 || wire["redacted"] != true || wire["protocol_version"] != float64(collaboration.ExecutionProtocolVersion) ||
		wire["issue_id"] != f.issue || wire["request_id"] != key || wire["action"] != full.Action || wire["code"] != full.Code || wire["status"] != float64(full.Status) {
		t.Fatalf("redacted HTTP recovery disclosed execution or lost original intent: %+v", wire)
	}
	if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" {
		t.Fatal("redacted receipt inherited ambient workspace or cache state")
	}
}
