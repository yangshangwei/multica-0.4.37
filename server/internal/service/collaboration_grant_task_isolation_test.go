package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// These tests exercise actual issue/grant/control writers and persisted managed
// protocol state. Their sealed proof is synthetic; no provider process is run.
func TestCollaborationGrantWithdrawalAndOwnerRejoinIsolateOtherTask(t *testing.T) {
	for _, action := range []string{"revoke_grant", "leave_rejoin"} {
		t.Run(action, func(t *testing.T) {
			peer := newManagedCollaborationFixture(t)
			peerInput := peer.submitManaged(t, peer.creator, "other task completed input")
			peer.resumeManaged(t)
			peerRun, _, peerClaim := peer.claimManaged(t)
			peerNonce := uuid.NewString()
			peerPermit, err := peer.svc.PermitRuntimeRun(peer.daemonCtx, peerRun, protocol.CollaborationRunPermitRequest{
				Identity: peerClaim.Identity, AttemptSeq: "1", AttemptNonce: peerNonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err = peer.svc.RecordRuntimeOutcome(context.Background(), peerRun, peerPermit.BusinessToken, protocol.CollaborationRunOutcome{
				Identity: peerClaim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: peerNonce,
				Outcome: "success", Body: "other task's original successful result",
			}); err != nil {
				t.Fatal(err)
			}
			if _, err = peer.svc.AcceptRuntimeProof(context.Background(), peerRun, peerClaim.TerminationToken, managedProof(peerClaim.Identity, peerNonce)); err != nil {
				t.Fatal(err)
			}
			beforePeer := grantTaskIsolationSnapshot(t, peer)
			var peerState struct {
				Control   map[string]json.RawMessage `json:"control"`
				Relations map[string]json.RawMessage `json:"relations"`
			}
			if err = json.Unmarshal([]byte(beforePeer), &peerState); err != nil {
				t.Fatal(err)
			}
			for _, group := range []struct {
				fields map[string]json.RawMessage
				names  []string
			}{
				{peerState.Control, []string{"channels", "targets", "commands", "groups", "runs", "grants", "terminations", "capabilities", "attempts", "reservations", "events"}},
				{peerState.Relations, []string{"participants", "operations"}},
			} {
				for _, relation := range group.names {
					var rows []json.RawMessage
					if err = json.Unmarshal(group.fields[relation], &rows); err != nil || len(rows) == 0 {
						t.Fatalf("peer snapshot has no %s evidence: %v", relation, err)
					}
				}
			}
			beforeAuthority, err := peer.svc.ResolveIssueAccess(peer.ctx, nil, peer.issue.Issue.ID)
			beforeCatalogue := authorityCatalogue(t, peer.collaborationQueueFixture, peer.ctx)
			if err != nil || !beforeAuthority.Capabilities.SubmitCommand || !beforeAuthority.Capabilities.ControlAI ||
				len(beforeCatalogue) != 1 || beforeCatalogue[0].TargetID != peer.targetID {
				t.Fatalf("peer task has no current participant and resource authority: %+v %v", beforeAuthority, err)
			}
			unchangedPeer := func(stage string) {
				t.Helper()
				current, err := peer.svc.ResolveIssueAccess(peer.ctx, nil, peer.issue.Issue.ID)
				if err != nil || !reflect.DeepEqual(current, beforeAuthority) || beforePeer != grantTaskIsolationSnapshot(t, peer) ||
					!reflect.DeepEqual(beforeCatalogue, authorityCatalogue(t, peer.collaborationQueueFixture, peer.ctx)) {
					t.Fatalf("%s on one task changed another task's same-resource grant, participation, controls or proof: %v", stage, err)
				}
			}

			affected := grantTaskIsolationIssue(t, peer)
			unchangedPeer("separate issue creation and consent")
			first := submitManagedControlInput(t, affected, queueTarget(t, affected.collaborationQueueFixture, affected.targetID), peer.handler, "affected task running input")
			affected.resumeManaged(t)
			runID, _, claim := affected.claimManaged(t)
			nonce := uuid.NewString()
			if _, err = affected.svc.PermitRuntimeRun(affected.daemonCtx, runID, protocol.CollaborationRunPermitRequest{
				Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
			}); err != nil {
				t.Fatal(err)
			}
			ownWaiting := affected.submitManaged(t, peer.creator, "resource owner's waiting input")
			otherWaiting := affected.submitManaged(t, peer.handler, "remaining collaborator's waiting input")
			ownerBefore, err := affected.svc.ResolveIssueAccess(peer.ctx, nil, affected.issue.Issue.ID)
			if err != nil || !ownerBefore.Capabilities.SubmitCommand || ownerBefore.SourceMember {
				t.Fatalf("resource owner must be an external participant: %+v %v", ownerBefore, err)
			}
			originalGrant := affected.grant
			if action == "revoke_grant" {
				receipt, err := affected.svc.RevokeGrant(peer.ctx, CollaborationRevokeGrantParams{
					CollaborationQueueRequest: affected.request(), GrantID: originalGrant.ID, ExpectedGrantEpoch: originalGrant.Epoch,
				})
				if err != nil || receipt.Code != "grant_revoked" {
					t.Fatalf("per-issue owner withdrawal: %+v %v", receipt, err)
				}
				ownerAfter, err := affected.svc.ResolveIssueAccess(peer.ctx, nil, affected.issue.Issue.ID)
				if err != nil || ownerAfter.ParticipantEpoch != ownerBefore.ParticipantEpoch || !reflect.DeepEqual(ownerAfter.Capabilities, ownerBefore.Capabilities) {
					t.Fatalf("grant-only withdrawal removed participant permissions: %+v %v", ownerAfter, err)
				}
				if affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, ownWaiting.CommandID) != 1 {
					t.Fatal("grant-only withdrawal cancelled its still-participating owner's waiting input")
				}
			} else {
				leave := CollaborationWorkflowRequest{CollaborationQueueRequest: affected.request(), ExpectedCollaborationRevision: ownerBefore.Revision}
				receipt, err := affected.svc.Leave(peer.ctx, leave)
				if err != nil || receipt.Code != "participant_left" {
					t.Fatalf("actual resource-owner leave: %+v %v", receipt, err)
				}
				if _, err = affected.svc.ResolveIssueAccess(peer.ctx, nil, affected.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
					t.Fatalf("departed external owner retained the affected task: %v", err)
				}
				if affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason='participant_left'`, ownWaiting.CommandID) != 1 {
					t.Fatal("owner leave did not cancel its own waiting input")
				}
				unchangedPeer("owner leave")
				request := affected.request()
				request.ExpectedParticipantEpoch = 1
				invited, err := affected.svc.InviteParticipant(affected.ctx, CollaborationParticipantParams{
					CollaborationWorkflowRequest: CollaborationWorkflowRequest{CollaborationQueueRequest: request, ExpectedCollaborationRevision: receipt.CollaborationRevision},
					TargetUserID:                 util.MustParseUUID(peer.creator),
				})
				if err != nil || invited.Code != "participant_added" {
					t.Fatalf("resource-owner reinvite: %+v %v", invited, err)
				}
				ownerAfter, err := affected.svc.ResolveIssueAccess(peer.ctx, nil, affected.issue.Issue.ID)
				if err != nil || ownerAfter.ParticipantEpoch.Int64 != ownerBefore.ParticipantEpoch.Int64+2 ||
					affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled'`, ownWaiting.CommandID) != 1 {
					t.Fatalf("rejoin revived old input or participation epoch: %+v %v", ownerAfter, err)
				}
			}
			unchangedPeer("grant withdrawal or owner rejoin")
			if affected.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=$2`, originalGrant.ID, originalGrant.Epoch+1) != 1 ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='grant_required'`, affected.targetID) != 1 ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL`, runID) != 1 ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NOT NULL AND confirmed_at IS NULL AND proof IS NULL`, runID) != 1 ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, otherWaiting.CommandID) != 1 {
				t.Fatal("withdrawal/rejoin lost the old proof barrier, waiting peer input or independent resource hold")
			}
			if _, err = affected.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
				t.Fatal(err)
			}
			unchangedPeer("old affected execution proof")
			if affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, first.CommandID) != 1 {
				t.Fatal("sealed old execution did not leave its original command terminal")
			}
			withoutConsent := managedTargetResume(t, affected, affected.targetID)
			if _, err = affected.svc.ResumeQueue(affected.ctx, withoutConsent); !collaborationErrorCode(err, "grant_required") {
				t.Fatalf("confirmed execution resumed without renewed owner consent: %v", err)
			}
			owner, err := affected.svc.ResolveIssueAccess(peer.ctx, nil, affected.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			consentRequest := affected.request()
			consentRequest.ExpectedParticipantEpoch = owner.ParticipantEpoch.Int64
			target := queueTarget(t, affected.collaborationQueueFixture, affected.targetID)
			consent, err := affected.svc.ConsentTargetResource(peer.ctx, CollaborationConsentParams{
				CollaborationQueueRequest: consentRequest, TargetID: target.ID, AgentID: originalGrant.AgentID,
				ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Identity: collaborationGrantIdentity(originalGrant),
			})
			if err != nil || consent.GrantID == originalGrant.ID || !consent.GrantID.Valid {
				t.Fatalf("explicit reconsent must create a fresh per-issue grant: %+v %v", consent, err)
			}
			afterConsent := controlStateSnapshot(t, affected.collaborationQueueFixture)
			if _, err = affected.svc.ResumeQueue(affected.ctx, withoutConsent); !collaborationErrorCode(err, "grant_required") ||
				afterConsent != controlStateSnapshot(t, affected.collaborationQueueFixture) {
				t.Fatalf("renewed consent replayed an earlier refused Resume: %v", err)
			}
			if queueTarget(t, affected.collaborationQueueFixture, target.ID).HoldReason.String != "grant_required" ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, affected.issue.SessionID) != 1 {
				t.Fatal("rejoin/reconsent automatically restored a stopped command or started a new run")
			}
			unchangedPeer("fresh affected consent and refused Resume replay")
			if _, err = affected.svc.ResumeQueue(affected.ctx, managedTargetResume(t, affected, target.ID)); err != nil {
				t.Fatal(err)
			}
			unchangedPeer("fresh affected Resume")
			if queueTarget(t, affected.collaborationQueueFixture, target.ID).HoldReason.Valid ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, first.CommandID) != 1 ||
				affected.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL`, originalGrant.ID) != 1 {
				t.Fatal("explicit Resume revived the old grant or stopped input")
			}
			if action == "leave_rejoin" && affected.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason='participant_left'`, ownWaiting.CommandID) != 1 {
				t.Fatal("fresh consent/Resume revived an input cancelled by the earlier leave")
			}
			// The real submission writer still accepts this exact peer grant,
			// without polling shared machine capacity or changing the other task.
			peerWaiting := peer.submitManaged(t, peer.handler, "other task remains usable")
			if peerWaiting.TargetID != peer.targetID || peerWaiting.CommandID == peerInput.CommandID ||
				peerWaiting.BatchID == peerInput.BatchID || peerWaiting.Code != "command_queued" ||
				peer.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND author_user_id=$2`, peerWaiting.CommandID, peer.handler) != 1 ||
				peer.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='succeeded'`, peerInput.CommandID) != 1 ||
				peer.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=$2`, peer.grant.ID, peer.grant.Epoch) != 1 {
				t.Fatal("per-issue withdrawal/leave prevented the same resource's other-task input or revived its completed command")
			}
			t.Logf("%s: affected_issue=%s peer_issue=%s shared_owner=%s shared_agent=%s shared_runtime=%s affected_grant=%s peer_grant=%s old_run=%s peer_proven_run=%s peer_new_command=%s",
				action, util.UUIDToString(affected.issue.Issue.ID), util.UUIDToString(peer.issue.Issue.ID), peer.creator,
				util.UUIDToString(peer.grant.AgentID), util.UUIDToString(peer.runtime.ID), util.UUIDToString(originalGrant.ID), util.UUIDToString(peer.grant.ID),
				util.UUIDToString(runID), util.UUIDToString(peerRun), util.UUIDToString(peerWaiting.CommandID))
		})
	}
}

func grantTaskIsolationIssue(t *testing.T, peer *managedCollaborationFixture) *managedCollaborationFixture {
	t.Helper()
	organizer := peer.outsider
	source := peer.fx.Workspace(t, "Other organizer source", "grant-isolation-"+uuid.NewString())
	peer.fx.Member(t, source, organizer, "owner")
	peer.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": organizer, "username": "isolation" + uuid.NewString()[:8],
		"password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", organizer)
	base := *peer.collaborationFixture
	base.ctx = collaborationPasswordContext(util.MustParseUUID(organizer), 1)
	base.creator = organizer
	base.workspace = source
	created, err := base.svc.Create(base.ctx, base.params(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"issue", "issue_collaboration", "issue_participant", "collaboration_session", "collaboration_event", "collaboration_notification"} {
		if table == "issue" {
			peer.fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1`, source)
			continue
		}
		peer.fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id=$1`, created.Issue.ID)
	}
	peer.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1`, source)
	if _, err = base.svc.InviteParticipant(base.ctx, CollaborationParticipantParams{
		CollaborationWorkflowRequest: CollaborationWorkflowRequest{CollaborationQueueRequest: CollaborationQueueRequest{IssueID: created.Issue.ID, RequestID: uuid.NewString(),
			ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1}, ExpectedCollaborationRevision: 1},
		TargetUserID: util.MustParseUUID(peer.creator),
	}); err != nil {
		t.Fatal(err)
	}
	affected := *peer
	affected.collaborationQueueFixture = &collaborationQueueFixture{collaborationFixture: &base, issue: created}
	registered, err := affected.svc.RegisterTarget(affected.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: affected.request(),
		Kind: collaboration.TargetAgent, ResourceID: peer.grant.AgentID, ExpectedTargetSetVersion: 1,
		Roster: []collaboration.TargetResource{{Role: "agent", AgentID: util.UUIDToString(peer.grant.AgentID), RuntimeID: util.UUIDToString(peer.runtime.ID),
			OwnerUserID: peer.creator, ExecutionWorkspaceID: peer.workspace}}})
	if err != nil {
		t.Fatal(err)
	}
	affected.targetID = registered.TargetID
	target := queueTarget(t, affected.collaborationQueueFixture, affected.targetID)
	consented, err := affected.svc.ConsentTargetResource(peer.ctx, CollaborationConsentParams{CollaborationQueueRequest: affected.request(),
		TargetID: target.ID, AgentID: peer.grant.AgentID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Identity: collaborationGrantIdentity(peer.grant)})
	if err != nil {
		t.Fatal(err)
	}
	affected.grant, err = affected.svc.Queries.GetCollaborationGrant(affected.ctx, consented.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	if affected.grant.OwnerUserID != peer.grant.OwnerUserID || affected.grant.AgentID != peer.grant.AgentID || affected.grant.RuntimeID != peer.grant.RuntimeID ||
		affected.grant.BindingID != peer.grant.BindingID || affected.grant.ID == peer.grant.ID || affected.targetID == peer.targetID || created.Issue.ID == peer.issue.Issue.ID {
		t.Fatal("isolation fixture must share one real owner/resource/binding across distinct task grants and targets")
	}
	for _, table := range []string{"collaboration_channel", "collaboration_target", "collaboration_batch", "collaboration_command", "collaboration_execution_group", "collaboration_run"} {
		peer.fx.Cleanup(t, `DELETE FROM `+table+` WHERE session_id=$1`, created.SessionID)
	}
	peer.fx.Cleanup(t, `DELETE FROM collaboration_grant WHERE issue_id=$1`, created.Issue.ID)
	peer.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, created.Issue.ID)
	for _, table := range []string{"collaboration_termination", "collaboration_attempt", "collaboration_business_capability", "collaboration_runtime_reservation", "collaboration_run_event"} {
		peer.fx.Cleanup(t, `DELETE FROM `+table+` WHERE run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)`, created.SessionID)
	}
	peer.fx.Cleanup(t, `DELETE FROM collaboration_group_input WHERE channel_id IN (SELECT id FROM collaboration_channel WHERE session_id=$1)`, created.SessionID)
	peer.fx.Cleanup(t, `DELETE FROM collaboration_group_dependency WHERE target_id=$1`, target.ID)
	return &affected
}

func grantTaskIsolationSnapshot(t *testing.T, f *managedCollaborationFixture) string {
	t.Helper()
	var state json.RawMessage
	var relations string
	state = json.RawMessage(controlStateSnapshot(t, f.collaborationQueueFixture))
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'issue',(SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
		'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
		'participants',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.user_id) FROM issue_participant p WHERE p.issue_id=$1),
		'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM collaboration_operation o WHERE o.scope_kind='issue' AND o.scope_id=$1),
		'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=$1)
	)::text`, f.issue.Issue.ID).Scan(&relations)
	encoded, err := json.Marshal(struct {
		Control   json.RawMessage `json:"control"`
		Relations json.RawMessage `json:"relations"`
	}{state, json.RawMessage(relations)})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
