package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/daemon/collaborationrun"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRuntimeRecordedContentHTTP(t *testing.T) {
	f := newCollaborationRouteFixture(t)
	f.comment(t, false, "immutable HTTP run input")
	f.control(t, uuid.NewString(), f.controlBody(t, "resume", "issue"), http.StatusOK)
	client, err := collaborationrun.NewClient(f.server.URL, f.binding.DaemonToken, nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := client.List(t.Context(), f.runtime)
	if err != nil || len(list.RunIDs) != 1 {
		t.Fatalf("runtime list: %+v %v", list, err)
	}
	run := list.RunIDs[0]
	claim, err := client.Claim(t.Context(), run, protocol.CollaborationRunClaimRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion, DeploymentID: f.deployment, InstallationID: f.installationID, RuntimeID: f.runtime, BindingID: f.binding.BindingID, BindingEpoch: f.binding.BindingEpoch, Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	permit, err := client.Permit(t.Context(), protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]protocol.CollaborationRecordedMessage, 60)
	for i := range messages {
		messages[i] = protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: fmt.Sprintf("actual HTTP recorded message %d", i)}
	}
	zero, large := "0", "9007199254740993"
	outcome := protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce, Outcome: "success", Body: "actual HTTP final result", Record: &protocol.CollaborationRunRecord{Version: 1, Completeness: "complete", Messages: messages, Usage: &protocol.CollaborationRunUsage{InputTokens: &large, OutputTokens: &zero}}}
	if err = client.Outcome(t.Context(), outcome, permit.BusinessToken); err != nil {
		t.Fatal(err)
	}
	if err = client.Outcome(t.Context(), outcome, permit.BusinessToken); err != nil {
		t.Fatal(err)
	}
	path := "/api/collaboration/issues/" + f.issue + "/runs/" + run + "/records"
	seen := map[string]bool{}
	query := "?limit=7"
	for {
		var page service.CollaborationRunRecordsPage
		response := f.managed.request(t, f.contributorToken, false, http.MethodGet, path+query, nil, nil).Want(http.StatusOK)
		response.JSON(&page)
		if response.Header().Get("Cache-Control") != "no-store" || page.Run.ID != run || page.IssueID != f.issue || page.Recording.Status != "complete" || page.Usage == nil || *page.Usage.InputTokens != large || *page.Usage.OutputTokens != zero {
			t.Fatal("HTTP recording lost privacy, identity or exact telemetry")
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("HTTP record cursor repeated an item")
			}
			seen[item.ID] = true
		}
		if page.NextCursor == nil {
			break
		}
		query = "?limit=7&cursor=" + *page.NextCursor
	}
	if len(seen) != 62 {
		t.Fatalf("paged runtime read lost input/messages/result: %d", len(seen))
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=1&limit=2", "?cursor=invalid", "?cursor=e30", "?unrecognized=private"} {
		f.managed.request(t, f.contributorToken, false, http.MethodGet, path+query, nil, nil).Want(http.StatusBadRequest)
	}
	missing := f.managed.request(t, f.contributorToken, false, http.MethodGet, "/api/collaboration/issues/"+f.issue+"/runs/"+uuid.NewString()+"/records", nil, nil).Want(http.StatusNotFound).Map()
	if missing["code"] != "run_not_found" {
		t.Fatal("missing run was confused with revoked issue authority")
	}
	// This is the real API's historical proof path. Kernel containment is
	// exercised separately by the static Linux runner integration test.
	exit := 0
	proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: claim.Identity, LastAuthorizedAttemptSeq: "1", SealedAttemptSeq: "1", AttemptNonce: permit.AttemptNonce, State: "exited_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace, ExitCode: &exit}
	if _, err = client.Proof(t.Context(), proof, claim.TerminationToken); err != nil {
		t.Fatal(err)
	}
	if _, err = client.Proof(t.Context(), proof, claim.TerminationToken); err != nil {
		t.Fatal(err)
	}
	if f.managed.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='ai_completed' AND run_id=$2`, f.issue, run) != 2 {
		t.Fatal("real result/proof route failed to produce one durable result per participant")
	}
	var notifications struct {
		Items []service.CollaborationNotificationView `json:"items"`
	}
	f.managed.request(t, f.contributorToken, false, http.MethodGet, "/api/me/collaboration-notifications", nil, nil).Want(http.StatusOK).JSON(&notifications)
	found := false
	for _, item := range notifications.Items {
		if item.Kind == "ai_completed" {
			found = item.CanOpen && item.RunID != nil && *item.RunID == run && item.SourceKind == "issue" && !item.SourceID.Valid
		}
	}
	if !found {
		t.Fatal("offline HTTP notification lost its additive run anchor")
	}
	f.managed.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue, f.contributor)
	f.managed.request(t, f.contributorToken, false, http.MethodGet, path, nil, nil).Want(http.StatusNotFound)
	// An absent additive run_id must be decoded into fresh response storage;
	// encoding/json otherwise preserves omitted pointer fields in reused items.
	notifications.Items = nil
	f.managed.request(t, f.contributorToken, false, http.MethodGet, "/api/me/collaboration-notifications", nil, nil).Want(http.StatusOK).JSON(&notifications)
	for _, item := range notifications.Items {
		if item.Kind == "ai_completed" && (item.CanOpen || item.RunID != nil || item.IssueTitle != nil || item.SourceID.Valid) {
			t.Fatal("retained notification restored revoked content or a private run anchor")
		}
	}
}

func TestCollaborationChildrenHintsUseOnlyAuthorizedIssueScope(t *testing.T) {
	bus := events.New()
	b := &fakeBroadcaster{}
	registerListeners(bus, b)
	id := uuid.NewString()
	bus.Publish(events.Event{Type: service.CollaborationChildrenChanged, WorkspaceID: "must-not-broadcast", ActorID: "must-not-copy", Payload: map[string]any{"issue_id": id}})
	if len(b.scopeCalls) != 1 || b.scopeCalls[0].scopeType != realtime.ScopeIssue || b.scopeCalls[0].scopeID != id || len(b.workspaceCalls) != 0 || len(b.userCalls) != 0 || b.broadcastCalled != 0 {
		t.Fatalf("child hint used a broader audience: %+v", b)
	}
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(b.scopeCalls[0].msg, &frame); err != nil || len(frame) != 2 || strings.Contains(string(b.scopeCalls[0].msg), "must-not") {
		t.Fatal("child hint leaked actor/workspace payload")
	}
	bus.Publish(events.Event{Type: service.CollaborationChildrenChanged, Payload: map[string]any{"issue_id": id, "body": "PRIVATE"}})
	if len(b.scopeCalls) != 1 {
		t.Fatal("malformed child hint was forwarded")
	}
}
