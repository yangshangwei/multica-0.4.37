package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationSwitchRecoveryAfterDepartureIsOwnAndRedacted(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	target, member := seedQueueSquad(t, f, true)
	proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
	p := switchConsent(f, target.ID, proposal.ProposalID, member)
	ownerCtx := collaborationContext(f.handler)
	original, err := f.svc.ConsentTargetSwitchResource(ownerCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	full, err := f.svc.GetSwitchOperation(ownerCtx, f.issue.Issue.ID, p.RequestID)
	if err != nil || !reflect.DeepEqual(full, original) {
		t.Fatalf("authorized owner did not recover the original full receipt: %+v %v", full, err)
	}
	if _, err = f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{
		CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler),
	}); err != nil {
		t.Fatal(err)
	}
	redacted, err := f.svc.GetSwitchOperation(ownerCtx, f.issue.Issue.ID, p.RequestID)
	if err != nil {
		t.Fatalf("departed owner lost recovery of their own committed operation: %v", err)
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["redacted"] != true || redacted.Status != original.Status || redacted.Code != original.Code ||
		redacted.IssueID != original.IssueID || redacted.RequestID != p.RequestID || redacted.Action != original.Action ||
		redacted.ProtocolVersion != collaboration.ExecutionProtocolVersion || redacted.TargetID.Valid || redacted.ProposalID.Valid ||
		redacted.GrantID.Valid || redacted.ChannelID.Valid || redacted.GroupID.Valid || redacted.RunID.Valid ||
		len(redacted.Targets) != 0 || len(redacted.Channels) != 0 || len(redacted.TargetHolds) != 0 || redacted.ControlVersion != 0 {
		t.Fatalf("lost-access recovery disclosed execution state or lost original outcome: %+v", redacted)
	}
	if _, err := f.svc.GetTargetSwitchProposal(ownerCtx, f.issue.Issue.ID, target.ID, proposal.ProposalID); err == nil {
		t.Fatal("receipt recovery restored proposal access after departure")
	}
	if _, err := f.svc.GetSwitchOperation(f.ctx, f.issue.Issue.ID, p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("creator recovered another actor's operation: %v", err)
	}
	if _, err := f.svc.GetSwitchOperation(ownerCtx, f.issue.Issue.ID, uuid.NewString()); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("missing own operation was not a safe miss: %v", err)
	}
	machine := auth.WithAuthenticatedPrincipal(ownerCtx, auth.AuthenticatedPrincipal{UserID: f.handler, Kind: auth.CredentialTask})
	if _, err := f.svc.GetSwitchOperation(machine, f.issue.Issue.ID, p.RequestID); err == nil {
		t.Fatal("machine recovered the resource owner's receipt")
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	if _, err := f.svc.GetSwitchOperation(ownerCtx, f.issue.Issue.ID, p.RequestID); err == nil {
		t.Fatal("disabled account recovered a receipt")
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{
		"user_id": f.handler, "username": "switch-recovery-" + uuid.NewString(), "password_hash": "test-only-unused-hash", "session_version": 2,
	}, "user_id=$1", f.handler)
	t.Setenv("MULTICA_AUTH_MODE", "password")
	stale := auth.WithPasswordSession(ownerCtx, auth.PasswordSession{UserID: f.handler, Version: 1, Kind: "jwt"})
	if _, err := f.svc.GetSwitchOperation(stale, f.issue.Issue.ID, p.RequestID); err == nil {
		t.Fatal("stale password credential recovered a receipt")
	}
	current := auth.WithPasswordSession(ownerCtx, auth.PasswordSession{UserID: f.handler, Version: 2, Kind: "jwt"})
	if _, err := f.svc.GetSwitchOperation(current, f.issue.Issue.ID, p.RequestID); err != nil {
		t.Fatalf("current password session could not recover its redacted receipt: %v", err)
	}
}
