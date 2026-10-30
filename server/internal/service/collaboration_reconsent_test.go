package service

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Owner re-consent recovers only configuration edits that keep the execution
// boundary: instructions, model and thinking level. Everything else that the
// configuration epoch fences still needs a staged Switch.
func preparedReconsentFixture(t *testing.T) *configInvalidationFixture {
	t.Helper()
	f := newConfigInvalidationFixture(t)
	switchCleanup(t, f.collaborationQueueFixture)
	registerPreparationFixture(t, f.managedCollaborationFixture)
	refreshPreparationConsent(t, f.managedCollaborationFixture)
	f.drain(t)
	return f
}

func reconsentParams(f *configInvalidationFixture, target db.CollaborationTarget, agent pgtype.UUID) CollaborationConsentParams {
	return CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: target.ID, AgentID: agent,
		ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}}
}

func reconsentResource(t *testing.T, f *configInvalidationFixture, target pgtype.UUID, agent pgtype.UUID) CollaborationTargetResources {
	t.Helper()
	view, err := f.svc.Resources(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range view.Targets {
		if item.TargetID != target {
			continue
		}
		for _, resource := range item.Resources {
			if resource.AgentID == util.UUIDToString(agent) {
				item.Resources = []CollaborationResourceView{resource}
				return item
			}
		}
	}
	t.Fatal("resources view omitted the retained target resource")
	return CollaborationTargetResources{}
}

func activeTargetGrants(t *testing.T, f *configInvalidationFixture, target pgtype.UUID, configVersion int64) int {
	t.Helper()
	return f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NULL AND config_version=$2`, target, configVersion)
}

func TestCollaborationConfigurationReconsentRecoversNonWideningAgentEdits(t *testing.T) {
	for name, edit := range map[string]struct {
		statements []string
		want       protocol.CollaborationPreparationInvocation
	}{
		"instructions": {[]string{`UPDATE agent SET instructions='Reconsented execution instructions' WHERE id=$1`},
			protocol.CollaborationPreparationInvocation{Instructions: "Reconsented execution instructions"}},
		"model": {[]string{`UPDATE agent SET model='reconsented-model' WHERE id=$1`},
			protocol.CollaborationPreparationInvocation{Model: "reconsented-model"}},
		"thinking_level": {[]string{`UPDATE agent SET thinking_level='high' WHERE id=$1`},
			protocol.CollaborationPreparationInvocation{ThinkingLevel: "high"}},
		"reverted_instructions": {[]string{`UPDATE agent SET instructions=instructions||' temporarily' WHERE id=$1`,
			`UPDATE agent SET instructions=left(instructions, length(instructions)-length(' temporarily')) WHERE id=$1`},
			protocol.CollaborationPreparationInvocation{}},
	} {
		t.Run(name, func(t *testing.T) {
			f := preparedReconsentFixture(t)
			waiting := f.submitManaged(t, f.creator, "waiting input survives owner re-consent")
			before := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			for _, statement := range edit.statements {
				f.fx.Exec(t, statement, f.grant.AgentID)
			}
			f.drain(t)
			held := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			if held.HoldReason.String != "config_invalid" || held.ConfigVersion != before.ConfigVersion || activeTargetGrants(t, f, held.ID, held.ConfigVersion) != 0 {
				t.Fatalf("configuration edit did not hold the target and retire its consent: %+v", held)
			}
			view := reconsentResource(t, f, held.ID, f.grant.AgentID)
			if !view.Resources[0].CanConsent {
				t.Fatalf("owner was not offered re-consent for a non-widening edit: %+v", view)
			}
			identity, err := f.svc.ResourceIdentity(f.ctx, f.issue.Issue.ID, held.ID, f.grant.AgentID, pgtype.UUID{})
			if err != nil || identity.Reason != "" || identity.Identity == nil {
				t.Fatalf("owner identity reader refused a non-widening edit: %+v %v", identity, err)
			}

			p := reconsentParams(f, held, f.grant.AgentID)
			r, err := f.svc.ConsentTargetResource(f.ctx, p)
			if err != nil || r.Code != "resource_consented" || r.Status != http.StatusCreated {
				t.Fatalf("owner re-consent of a non-widening edit failed: %+v %v", r, err)
			}
			after := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			if after.ConfigVersion != held.ConfigVersion+1 || after.TargetEpoch != held.TargetEpoch || after.HoldReason.String != "config_invalid" ||
				after.HoldVersion != held.HoldVersion+1 || after.QueueState != held.QueueState || after.ControlVersion != held.ControlVersion {
				t.Fatalf("re-consent did not refresh exactly one configuration version, or released the hold: %+v", after)
			}
			if _, code, err := authorizeCollaborationTargetInput(f.ctx, f.svc.Queries, f.issue.Issue.ID, after); err != nil || code != "" {
				t.Fatalf("re-consented target is not authorized: %s %v", code, err)
			}
			if activeTargetGrants(t, f, after.ID, after.ConfigVersion) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NULL`, after.ID) != 1 {
				t.Fatal("re-consent did not leave exactly one current grant")
			}
			grants := switchGrants(t, f.collaborationQueueFixture, after.ID)
			replay, err := f.svc.ConsentTargetResource(f.ctx, p)
			if err != nil || !reflect.DeepEqual(replay, r) || !reflect.DeepEqual(after, queueTarget(t, f.collaborationQueueFixture, f.targetID)) ||
				!reflect.DeepEqual(grants, switchGrants(t, f.collaborationQueueFixture, after.ID)) {
				t.Fatalf("exact re-consent replay advanced state: %+v %v", replay, err)
			}
			repeated, err := f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, after, f.grant.AgentID))
			if err != nil || repeated.Code != "resource_consented" || repeated.Status != http.StatusOK || repeated.GrantID != r.GrantID ||
				!reflect.DeepEqual(after, queueTarget(t, f.collaborationQueueFixture, f.targetID)) {
				t.Fatalf("repeated current consent advanced state: %+v %v", repeated, err)
			}
			// The background consumer may observe the old intent after the refresh.
			f.drain(t)
			if !reflect.DeepEqual(after, queueTarget(t, f.collaborationQueueFixture, f.targetID)) || activeTargetGrants(t, f, after.ID, after.ConfigVersion) != 1 {
				t.Fatal("a late configuration intent revoked the re-consented authority")
			}

			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(list.RunIDs) != 0 {
				t.Fatalf("consent without Resume released the hold: %+v %v", list, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
				t.Fatal("re-consent discarded the retained waiting input")
			}
			if _, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
				InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: held.TargetEpoch, ExpectedConfigVersion: held.ConfigVersion,
				Content: "stale configuration view"}); !collaborationErrorCode(err, "stale_target") {
				t.Fatalf("input bound to the old configuration version was accepted: %v", err)
			}
			f.resumeManaged(t)
			if released := queueTarget(t, f.collaborationQueueFixture, f.targetID); released.HoldReason.Valid {
				t.Fatalf("explicit Resume did not release the re-consented hold: %+v", released)
			}
			runID := util.MustParseUUID(func() string {
				list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
				if err != nil || len(list.RunIDs) != 1 {
					t.Fatalf("one admitted candidate after Resume: %+v %v", list, err)
				}
				return list.RunIDs[0]
			}())
			_, claim := claimPreparedCandidate(t, f.managedCollaborationFixture, runID)
			if claim.Preparation.Invocation != edit.want || len(claim.Inputs) != 1 {
				t.Fatalf("claim after re-consent did not carry the current configuration: %+v", claim.Preparation.Invocation)
			}
		})
	}
}

func TestCollaborationConfigurationReconsentRefusesWideningAndIdentityEdits(t *testing.T) {
	for _, change := range []string{"permission_mode", "custom_args", "skill_binding", "runtime_visibility"} {
		t.Run(change, func(t *testing.T) {
			f := preparedReconsentFixture(t)
			switch change {
			case "permission_mode":
				f.fx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, f.grant.AgentID)
			case "custom_args":
				f.fx.Exec(t, `UPDATE agent SET custom_args='["--widened"]' WHERE id=$1`, f.grant.AgentID)
			case "skill_binding":
				skill := f.fx.Insert(t, "skill", testutil.Cols{"workspace_id": f.workspace, "name": "Reconsent widening skill " + uuid.NewString(), "content": "test-owned skill", "created_by": f.creator})
				if err := f.svc.Queries.AddAgentSkill(f.ctx, db.AddAgentSkillParams{AgentID: f.grant.AgentID, SkillID: util.MustParseUUID(skill)}); err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, `DELETE FROM agent_skill WHERE agent_id=$1`, f.grant.AgentID)
			case "runtime_visibility":
				if _, err := f.svc.Queries.UpdateAgentRuntimeVisibility(f.ctx, db.UpdateAgentRuntimeVisibilityParams{ID: f.runtime.ID, Visibility: "public"}); err != nil {
					t.Fatal(err)
				}
			}
			f.drain(t)
			held := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			grants := switchGrants(t, f.collaborationQueueFixture, held.ID)
			if held.HoldReason.String != "config_invalid" {
				t.Fatalf("widening edit did not hold the target: %+v", held)
			}
			if view := reconsentResource(t, f, held.ID, f.grant.AgentID); view.Resources[0].CanConsent {
				t.Fatal("owner was offered re-consent for a widening edit")
			}
			if _, err := f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, held, f.grant.AgentID)); !collaborationErrorCode(err, "target_configuration_changed") ||
				!reflect.DeepEqual(held, queueTarget(t, f.collaborationQueueFixture, f.targetID)) || !reflect.DeepEqual(grants, switchGrants(t, f.collaborationQueueFixture, held.ID)) {
				t.Fatalf("owner re-consent recovered a widening edit or its refusal changed state: %v", err)
			}

			// The creator or handler still restores it through a staged Switch.
			proposal, _ := proposeSwitch(t, f.collaborationQueueFixture, held, switchRoster(t, held))
			consent := CollaborationSwitchConsentParams{CollaborationQueueRequest: f.request(), TargetID: held.ID, ProposalID: proposal.ProposalID, AgentID: f.grant.AgentID,
				Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}}
			if _, err := f.svc.ConsentTargetSwitchResource(f.ctx, consent); err != nil {
				t.Fatal(err)
			}
			committed, err := f.svc.CommitTargetSwitch(f.ctx, CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID,
				Control: targetControl(queueTarget(t, f.collaborationQueueFixture, held.ID))})
			if err != nil || committed.Code != "target_switched" {
				t.Fatalf("Switch did not restore a widening edit: %+v %v", committed, err)
			}
			switched := queueTarget(t, f.collaborationQueueFixture, held.ID)
			if switched.HoldReason.String != "switch_hold" || switched.ConfigVersion != held.ConfigVersion+1 || switched.TargetEpoch != held.TargetEpoch+1 {
				t.Fatalf("Switch did not install a new configuration behind an explicit hold: %+v", switched)
			}
			if _, code, err := authorizeCollaborationTargetInput(f.ctx, f.svc.Queries, f.issue.Issue.ID, switched); err != nil || code != "" {
				t.Fatalf("switched target is not authorized: %s %v", code, err)
			}
			// Switch records the new boundary, so a later non-widening edit of the
			// switched configuration is again recoverable by its owner.
			f.fx.Exec(t, `UPDATE agent SET instructions='after Switch' WHERE id=$1`, f.grant.AgentID)
			f.drain(t)
			if _, err = f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, queueTarget(t, f.collaborationQueueFixture, held.ID), f.grant.AgentID)); err != nil {
				t.Fatalf("Switch did not record the boundary of its new configuration: %v", err)
			}
		})
	}
}

func TestCollaborationConfigurationReconsentRecoversSquadMemberInstructions(t *testing.T) {
	f := preparedReconsentFixture(t)
	member := util.MustParseUUID(f.fx.Agent(t, "Reconsent squad member "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"}))
	squad := util.MustParseUUID(f.fx.Squad(t, "Reconsent squad "+uuid.NewString(), util.UUIDToString(f.grant.AgentID)))
	f.fx.SquadMember(t, util.UUIDToString(squad), "agent", util.UUIDToString(member))
	f.owned = append(f.owned, member, squad)
	f.drain(t)
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	resource := func(role string, agent pgtype.UUID) collaboration.TargetResource {
		return collaboration.TargetResource{Role: role, AgentID: util.UUIDToString(agent), RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}
	}
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad, ResourceID: squad,
		ExpectedTargetSetVersion: session.TargetSetVersion, Roster: []collaboration.TargetResource{resource("leader", f.grant.AgentID), resource("member", member)}})
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range []pgtype.UUID{f.grant.AgentID, member} {
		if _, err = f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, queueTarget(t, f.collaborationQueueFixture, registered.TargetID), agent)); err != nil {
			t.Fatal(err)
		}
	}
	direct := queueTarget(t, f.collaborationQueueFixture, f.targetID)

	f.fx.Exec(t, `UPDATE agent SET instructions='Reconsented member instructions' WHERE id=$1`, member)
	f.drain(t)
	held := queueTarget(t, f.collaborationQueueFixture, registered.TargetID)
	if held.HoldReason.String != "config_invalid" || activeTargetGrants(t, f, held.ID, held.ConfigVersion) != 0 {
		t.Fatalf("member edit did not hold the squad and retire its consents: %+v", held)
	}
	if _, err = f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, held, member)); err != nil {
		t.Fatalf("member owner could not re-consent a non-widening member edit: %v", err)
	}
	refreshed := queueTarget(t, f.collaborationQueueFixture, held.ID)
	if refreshed.ConfigVersion != held.ConfigVersion+1 || refreshed.TargetEpoch != held.TargetEpoch || refreshed.HoldReason.String != "config_invalid" {
		t.Fatalf("squad re-consent did not refresh one configuration version behind its hold: %+v", refreshed)
	}
	// Every roster resource is consented again for the refreshed configuration.
	if _, code, err := authorizeCollaborationTargetInput(f.ctx, f.svc.Queries, f.issue.Issue.ID, refreshed); err != nil || code != "grant_required" {
		t.Fatalf("one member's re-consent authorized the whole squad: %s %v", code, err)
	}
	if _, err = f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, refreshed, f.grant.AgentID)); err != nil {
		t.Fatal(err)
	}
	final := queueTarget(t, f.collaborationQueueFixture, held.ID)
	if final.ConfigVersion != refreshed.ConfigVersion || activeTargetGrants(t, f, final.ID, final.ConfigVersion) != 2 {
		t.Fatalf("leader consent of the refreshed squad advanced it again or lost a grant: %+v", final)
	}
	if _, code, err := authorizeCollaborationTargetInput(f.ctx, f.svc.Queries, f.issue.Issue.ID, final); err != nil || code != "" {
		t.Fatalf("fully re-consented squad is not authorized: %s %v", code, err)
	}
	if !reflect.DeepEqual(direct, queueTarget(t, f.collaborationQueueFixture, f.targetID)) {
		t.Fatal("squad member edit changed the leader's direct-agent target")
	}
}

func TestCollaborationConfigurationReconsentKeepsSwitchOnlyForTargetsWithoutBoundary(t *testing.T) {
	f := preparedReconsentFixture(t)
	// Targets created before the boundary was recorded keep Switch-only recovery.
	f.fx.Exec(t, `UPDATE collaboration_target SET configuration_boundary_fingerprint=NULL WHERE id=$1`, f.targetID)
	f.fx.Exec(t, `UPDATE agent SET instructions='Instructions of an unbounded target' WHERE id=$1`, f.grant.AgentID)
	f.drain(t)
	held := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	grants := switchGrants(t, f.collaborationQueueFixture, held.ID)
	if view := reconsentResource(t, f, held.ID, f.grant.AgentID); view.Resources[0].CanConsent {
		t.Fatal("owner was offered re-consent for a target without a recorded boundary")
	}
	if _, err := f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, held, f.grant.AgentID)); !collaborationErrorCode(err, "target_configuration_changed") ||
		!reflect.DeepEqual(held, queueTarget(t, f.collaborationQueueFixture, f.targetID)) || !reflect.DeepEqual(grants, switchGrants(t, f.collaborationQueueFixture, held.ID)) {
		t.Fatalf("a target without a recorded boundary was recovered by owner re-consent: %v", err)
	}
}

// The owner may confirm before the configuration consumer runs. The old
// configuration's running group must still stop, and the late consumer must keep
// the confirmation of the refreshed configuration and its hold.
func TestCollaborationConfigurationReconsentBeforeConsumerStopsOldConfigurationGroup(t *testing.T) {
	f := preparedReconsentFixture(t)
	run, claim, _ := startResourceCollaboration(t, f.managedCollaborationFixture)
	f.fx.Exec(t, `UPDATE agent SET instructions='Confirmed before the consumer' WHERE id=$1`, f.grant.AgentID)
	edited := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if edited.HoldReason.Valid {
		t.Fatalf("the edit was reconciled before the owner confirmed it: %+v", edited)
	}
	r, err := f.svc.ConsentTargetResource(f.ctx, reconsentParams(f, edited, f.grant.AgentID))
	if err != nil || r.Status != http.StatusCreated {
		t.Fatalf("owner re-consent before the consumer failed: %+v %v", r, err)
	}
	refreshed := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if refreshed.HoldReason.String != "config_invalid" || refreshed.ConfigVersion != edited.ConfigVersion+1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NULL`, refreshed.ID) != 1 {
		t.Fatalf("re-consent did not hold the refreshed configuration behind one current grant: %+v", refreshed)
	}
	f.drain(t)
	if refreshed.ConfigVersion != queueTarget(t, f.collaborationQueueFixture, f.targetID).ConfigVersion ||
		activeTargetGrants(t, f, refreshed.ID, refreshed.ConfigVersion) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='config_invalid'`, f.targetID) != 1 {
		t.Fatal("the late consumer revoked the refreshed confirmation or released its hold")
	}
	intent, err := f.svc.RuntimeStopIntent(t.Context(), run, claim.TerminationToken)
	if err != nil || !intent.StopRequested {
		t.Fatalf("a group of the old configuration kept running after re-consent: %+v %v", intent, err)
	}
}

// Only the resource epoch writer advances the boundary epoch. A child writer's
// explicit configuration advance widens it, because it may change tools.
func TestCollaborationConfigurationBoundaryEpochAdvancesOnlyThroughItsWriter(t *testing.T) {
	f := newConfigInvalidationFixture(t)
	boundary := func() int64 {
		var epoch pgtype.Int8
		if err := f.pool.QueryRow(f.ctx, `SELECT collaboration_boundary_epoch FROM agent WHERE id=$1`, f.grant.AgentID).Scan(&epoch); err != nil {
			t.Fatal(err)
		}
		return epoch.Int64
	}
	start := boundary()
	if _, err := f.pool.Exec(f.ctx, `UPDATE agent SET collaboration_boundary_epoch=COALESCE(collaboration_boundary_epoch,0)+1 WHERE id=$1`, f.grant.AgentID); err == nil {
		t.Fatal("a direct boundary epoch assignment was accepted")
	}
	f.fx.Exec(t, `UPDATE agent SET instructions='inside the boundary', model='inside-model', thinking_level='low' WHERE id=$1`, f.grant.AgentID)
	if boundary() != start {
		t.Fatal("an instructions/model/thinking edit widened the boundary")
	}
	f.fx.Exec(t, `UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE id=$1`, f.grant.AgentID)
	if boundary() != start+1 {
		t.Fatal("an explicit child configuration advance did not widen the boundary")
	}
	f.fx.Exec(t, `UPDATE agent SET service_tier='widened-tier' WHERE id=$1`, f.grant.AgentID)
	if boundary() != start+2 {
		t.Fatal("a managed tool or execution column edit did not widen the boundary")
	}
}
