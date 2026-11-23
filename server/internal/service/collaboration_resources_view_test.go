package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationResourcesOnlyExposeOwnGrantIdentity(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	owner, err := f.svc.Resources(f.ctx, f.issue.Issue.ID)
	if err != nil || len(owner.Targets) != 1 || len(owner.Targets[0].Resources) != 1 {
		t.Fatalf("owner resource view: %+v %v", owner, err)
	}
	resource := owner.Targets[0].Resources[0]
	if !resource.Consented || resource.GrantID == nil || resource.GrantEpoch == nil || !resource.CanRevoke || resource.CanConsent {
		t.Fatalf("current owner consent: %+v", resource)
	}
	other, err := f.svc.Resources(collaborationContext(f.handler), f.issue.Issue.ID)
	if err != nil || len(other.Targets) != 1 {
		t.Fatalf("participant resource view: %+v %v", other, err)
	}
	visible := other.Targets[0].Resources[0]
	if !visible.Consented || visible.GrantID != nil || visible.GrantEpoch != nil || visible.CanRevoke || visible.CanConsent {
		t.Fatalf("another participant acquired owner controls: %+v", visible)
	}
	encoded, _ := json.Marshal(other)
	for _, private := range []string{"configuration_fingerprint", "instructions", "installation_id", "binding_id", "public_key", "custom_env"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("resource summary leaked %s", private)
		}
	}
	if _, err = f.svc.Resources(collaborationContext(f.outsider), f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("unrelated actor read resources: %v", err)
	}
}

func TestCollaborationResourceIdentityIsCurrentOwnerOnly(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	p := pendingAuthorityConsent(t, f)
	view, err := f.svc.ResourceIdentity(f.ctx, f.issue.Issue.ID, p.TargetID, p.AgentID, pgtype.UUID{})
	if err != nil || view.Identity == nil || *view.Identity != p.Identity {
		t.Fatalf("current enrollment was not projected: %+v %v", view, err)
	}
	if _, err = f.svc.ResourceIdentity(collaborationContext(f.handler), f.issue.Issue.ID, p.TargetID, p.AgentID, pgtype.UUID{}); !collaborationErrorCode(err, "resource_not_found") {
		t.Fatalf("another participant learned binding identity: %v", err)
	}
	f.fx.Exec(t, `UPDATE collaboration_runtime_enrollment SET revoked_at=now() WHERE id=$1`, p.Identity.BindingID)
	view, err = f.svc.ResourceIdentity(f.ctx, f.issue.Issue.ID, p.TargetID, p.AgentID, pgtype.UUID{})
	if err != nil || view.Identity != nil || view.Reason != "runtime_registration_required" {
		t.Fatalf("revoked enrollment remained selectable: %+v %v", view, err)
	}
	machine := auth.WithAuthenticatedPrincipal(f.ctx, auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
	if _, err = f.svc.ResourceIdentity(machine, f.issue.Issue.ID, p.TargetID, p.AgentID, pgtype.UUID{}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("machine borrowed owner identity: %v", err)
	}
}

func TestCollaborationResourceProjectionRetainsRevocationAfterConfigurationChange(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	target := queueTarget(t, f, f.targetID)
	f.fx.Exec(t, `UPDATE agent SET instructions='changed execution configuration' WHERE id=$1`, target.ResourceID)
	view, err := f.svc.Resources(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	item := view.Targets[0]
	if item.Authorized || item.Resources[0].Consented || item.Resources[0].CanConsent || !item.Resources[0].CanRevoke {
		t.Fatalf("configuration drift authorized input or hid owner revocation: %+v", item)
	}
}

func TestCollaborationResourceOperationRecoveryKeepsOriginalScope(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	p := pendingAuthorityConsent(t, f)
	r, err := f.svc.ConsentTargetResource(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := f.svc.GetResourceOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || recovered.GrantID != r.GrantID || recovered.Action != "consent_resource" {
		t.Fatalf("resource receipt: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetResourceOperation(collaborationContext(f.handler), f.issue.Issue.ID, p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("other actor recovered owner receipt: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
	recovered, err = f.svc.GetResourceOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || !recovered.Redacted || recovered.GrantID.Valid || recovered.TargetID.Valid {
		t.Fatalf("departed owner recovered private resource identity: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetResourceOperation(f.ctx, util.MustParseUUID(uuid.NewString()), p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("cross-issue receipt recovered: %v", err)
	}
}

func TestCollaborationResourceProposalShowsNamesWithoutInstallationIdentity(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	target := queueTarget(t, f, f.targetID)
	roster := switchRoster(t, target)
	f.fx.Exec(t, `UPDATE agent SET name='Proposal display agent' WHERE id=$1`, target.ResourceID)
	f.fx.Exec(t, `UPDATE agent_runtime SET custom_name='Proposal display machine' WHERE id=$1`, roster[0].RuntimeID)
	created, _ := proposeSwitch(t, f, target, roster)
	view, err := f.svc.GetTargetSwitchProposal(f.ctx, f.issue.Issue.ID, target.ID, created.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "Proposal display agent") || !strings.Contains(string(encoded), "Proposal display machine") {
		t.Fatalf("proposal cannot be reviewed by display names: %s", encoded)
	}
	for _, private := range []string{"installation_id", "binding_id", "public_key", "configuration_fingerprint"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("proposal exposed %s", private)
		}
	}
}

func TestCollaborationParticipantsRetainDisabledManagementWithoutMentionAuthority(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	view, err := f.svc.Participants(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, participant := range view.Participants {
		if participant.UserID == util.MustParseUUID(f.handler) {
			found = true
			if participant.AccountActive {
				t.Fatal("disabled participant was marked active")
			}
		}
	}
	if !found {
		t.Fatal("disabled participant disappeared from management")
	}
	mentions, err := f.svc.MentionTargets(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, participant := range mentions.Participants {
		if participant.UserID == util.MustParseUUID(f.handler) {
			t.Fatal("management revived disabled mention authority")
		}
	}
	if _, err = f.svc.Participants(collaborationContext(f.handler), f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("disabled viewer read participants: %v", err)
	}
	result, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: CollaborationWorkflowRequest{CollaborationQueueRequest: f.request(), ExpectedCollaborationRevision: view.CollaborationRevision}, TargetUserID: util.MustParseUUID(f.handler)})
	if err != nil || result.Code != "participant_removed" {
		t.Fatalf("creator could not remove disabled handler: %+v %v", result, err)
	}
	current, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil || current.AssigneeID != util.MustParseUUID(f.creator) {
		t.Fatalf("disabled removal did not return responsibility atomically: %v", err)
	}
}
