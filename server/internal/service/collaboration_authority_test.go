package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationRegistrationChecksActualResourceOwner(t *testing.T) {
	f := newCollaborationFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	runtime := f.fx.Runtime(t, "Owner-only registration runtime", testutil.Cols{"owner_id": f.creator})
	agent := f.fx.Agent(t, "Owner-only registration "+uuid.NewString(), runtime, testutil.Cols{"owner_id": f.creator})
	p := CollaborationRegisterTargetParams{
		CollaborationQueueRequest: CollaborationQueueRequest{IssueID: issue.Issue.ID, RequestID: uuid.NewString(), ExpectedAccountEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedLifecycleEpoch: 1},
		Kind:                      collaboration.TargetAgent, ResourceID: util.MustParseUUID(agent), ExpectedTargetSetVersion: 1,
		Roster: []collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: runtime, ExecutionWorkspaceID: f.workspace, OwnerUserID: f.handler}},
	}
	_, err = f.svc.RegisterTarget(f.ctx, p)
	if !collaborationErrorCode(err, "target_configuration_changed") {
		t.Fatalf("a participating account must not be declared owner of another person's runtime: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1`, issue.SessionID) != 0 {
		t.Fatal("invalid owner configuration created a target")
	}
}

func authorityCatalogue(t *testing.T, f *collaborationQueueFixture, ctx context.Context) []CollaborationMentionTarget {
	t.Helper()
	tx, err := f.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := f.svc.Queries.WithTx(tx)
	access, err := f.svc.ResolveIssueAccess(ctx, q, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := f.svc.AuthorizedMentionTargets(ctx, q, access)
	if err != nil {
		t.Fatal(err)
	}
	return targets
}

func pendingAuthorityConsent(t *testing.T, f *collaborationQueueFixture) CollaborationConsentParams {
	t.Helper()
	grants, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, f.targetID)
	if err != nil || len(grants) != 1 {
		t.Fatalf("synthetic grant setup: %v", err)
	}
	grant := grants[0]
	f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, grant.ID)
	return CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, AgentID: grant.AgentID,
		ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: collaborationGrantIdentity(grant)}
}

func TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	p := pendingAuthorityConsent(t, f)
	if len(authorityCatalogue(t, f, f.ctx)) != 0 {
		t.Fatal("pending target entered mention catalogue")
	}
	participants := f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1`, f.issue.Issue.ID)
	forbidden := p
	forbidden.RequestID = uuid.NewString()
	_, err := f.svc.ConsentTargetResource(collaborationContext(f.handler), forbidden)
	if !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("another participant consented for the actual owner: %v", err)
	}
	first, err := f.svc.ConsentTargetResource(f.ctx, p)
	if err != nil || !first.GrantID.Valid {
		t.Fatalf("owner consent: %+v %v", first, err)
	}
	again, err := f.svc.ConsentTargetResource(f.ctx, p)
	if err != nil || first.GrantID != again.GrantID {
		t.Fatalf("consent retry changed grant: %+v %v", again, err)
	}
	catalogue := authorityCatalogue(t, f, collaborationContext(f.handler))
	if len(catalogue) != 1 || catalogue[0].TargetID != f.targetID || catalogue[0].ExecutionAvailable || catalogue[0].WaitingReason != "issue_paused" {
		t.Fatalf("consent must admit the existing participant's waiting input without enabling execution: %+v", catalogue)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1`, f.targetID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, f.targetID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1`, f.issue.Issue.ID) != participants ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND control_version=1`, f.targetID) != 1 {
		t.Fatal("owner confirmation duplicated state, enrolled people or changed execution")
	}
}

func TestCollaborationConsentUsesCompleteSquadRoster(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, member := seedQueueSquad(t, f, true)
	f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, member.ID)
	catalogue := authorityCatalogue(t, f, f.ctx)
	if len(catalogue) != 1 || catalogue[0].TargetID != f.targetID {
		t.Fatal("a leader grant authorized an unconsented member")
	}
	p := CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID, AgentID: member.AgentID,
		ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: collaborationGrantIdentity(member)}
	if _, err := f.svc.ConsentTargetResource(collaborationContext(f.handler), p); err != nil {
		t.Fatal(err)
	}
	if len(authorityCatalogue(t, f, f.ctx)) != 2 {
		t.Fatal("complete owner consent did not admit the squad")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1 AND kind='agent' AND resource_id=$2`, f.issue.SessionID, member.AgentID) != 0 {
		t.Fatal("squad consent created an independent member target")
	}
}

func TestCollaborationPresenceAndNamesDoNotInvalidateConsent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	target := queueTarget(t, f, f.targetID)
	grants, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent SET name='Renamed authorized agent', status='working', max_concurrent_tasks=2, updated_at=now() WHERE id=$1`, target.ResourceID)
	f.fx.Exec(t, `UPDATE agent_runtime SET status='offline', last_seen_at=now(), metadata='{"capacity":1}'::jsonb, name='Display name' WHERE id=$1`, grants[0].RuntimeID)
	items := authorityCatalogue(t, f, f.ctx)
	if len(items) != 1 || items[0].Name != "Renamed authorized agent" || items[0].ExecutionAvailable {
		t.Fatalf("presence/display changes revoked consent or implied runtime support: %+v", items)
	}
	f.submit(t, f.handler, "offline is valid waiting input")
}

func TestCollaborationConsentCurrentAuthority(t *testing.T) {
	for _, scenario := range []string{"participant_epoch", "account_epoch", "binding_epoch", "deployment", "namespace", "old_grant"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			grants, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, f.targetID)
			if err != nil {
				t.Fatal(err)
			}
			grant := grants[0]
			switch scenario {
			case "participant_epoch":
				f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
			case "account_epoch":
				f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.creator)
			case "binding_epoch":
				f.fx.Exec(t, `UPDATE collaboration_runtime_enrollment SET binding_epoch=binding_epoch+1 WHERE id=$1`, grant.BindingID)
			case "deployment":
				f.fx.Exec(t, `UPDATE collaboration_runtime_enrollment SET deployment_id=$2 WHERE id=$1`, grant.BindingID, uuid.NewString())
			case "namespace":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, grant.ExecutionWorkspaceID, f.creator)
			case "old_grant":
				f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, grant.ID)
				f.fx.Insert(t, "collaboration_grant", testutil.Cols{
					"issue_id": grant.IssueID, "target_id": grant.TargetID, "config_version": grant.ConfigVersion.Int64, "role": grant.Role.String,
					"owner_user_id": grant.OwnerUserID, "owner_auth_epoch": grant.OwnerAuthEpoch, "execution_workspace_id": grant.ExecutionWorkspaceID,
					"agent_id": grant.AgentID, "runtime_id": grant.RuntimeID, "identity_kind": grant.IdentityKind,
					"installation_id": grant.InstallationID, "binding_id": grant.BindingID, "binding_epoch": grant.BindingEpoch,
				})
			}
			if len(authorityCatalogue(t, f, collaborationContext(f.handler))) != 0 {
				t.Fatal("obsolete or incomplete consent stayed selectable")
			}
			p := CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "must not enter queue"}
			if _, err := f.svc.SubmitCommand(collaborationContext(f.handler), p); err == nil {
				t.Fatal("old target selection bypassed current consent")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, f.targetID) != 0 {
				t.Fatal("invalid consent inserted an input")
			}
			stop := f.issueControl(t)
			stop.CollaborationQueueRequest = f.request()
			if _, err := f.svc.StopQueue(collaborationContext(f.handler), stop); err != nil {
				t.Fatalf("legitimate stop must survive lost resource authority: %v", err)
			}
		})
	}
}

func TestCollaborationReconfigureRequiresNewOwnerConsent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	target := queueTarget(t, f, f.targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent SET instructions='new execution instructions' WHERE id=$1`, target.ResourceID)
	p := CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Roster: roster,
		Control: CollaborationTargetControl{TargetID: target.ID, ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, ExpectedControlVersion: 1}}
	if _, err = f.svc.ReconfigureTarget(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if len(authorityCatalogue(t, f, f.ctx)) != 0 {
		t.Fatal("reconfiguration copied the old consent")
	}
	target = queueTarget(t, f, target.ID)
	if target.ConfigVersion != 2 || target.TargetEpoch != 2 || target.QueueState != "paused" {
		t.Fatal("new configuration did not preserve pause and advance authority")
	}
	grants, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, target.ID)
	if err != nil || len(grants) != 1 || !grants[0].RevokedAt.Valid {
		t.Fatalf("old consent history missing: %v", err)
	}
	grant := grants[0]
	if _, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: target.ID,
		AgentID: grant.AgentID, ExpectedTargetEpoch: 2, ExpectedConfigVersion: 2, Identity: collaborationGrantIdentity(grant)}); err != nil {
		t.Fatal(err)
	}
	if len(authorityCatalogue(t, f, f.ctx)) != 1 {
		t.Fatal("new explicit owner consent did not restore the candidate")
	}
}

func TestCollaborationPermissionRestoreCannotReviveConsent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	target := queueTarget(t, f, f.targetID)
	f.fx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, target.ResourceID)
	f.fx.Exec(t, `UPDATE agent SET permission_mode='private' WHERE id=$1`, target.ResourceID)
	_, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{
		CollaborationQueueRequest: f.request(), TargetID: target.ID, InputEventID: util.MustParseUUID(uuid.NewString()),
		ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: "old consent must stay invalid",
	})
	if !collaborationErrorCode(err, "target_configuration_changed") {
		t.Fatalf("restoring an earlier permission value must require renewed configuration consent: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE target_id=$1`, target.ID) != 0 {
		t.Fatal("obsolete consent persisted a new command")
	}
}
