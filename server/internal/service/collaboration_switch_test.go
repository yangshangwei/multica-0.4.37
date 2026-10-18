package service

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func switchCleanup(t *testing.T, f *collaborationQueueFixture) {
	t.Helper()
	f.fx.Cleanup(t, `DELETE FROM collaboration_switch_proposal WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_switch_consent WHERE proposal_id IN (SELECT id FROM collaboration_switch_proposal WHERE issue_id=$1)`, f.issue.Issue.ID)
}

func proposeSwitch(t *testing.T, f *collaborationQueueFixture, target db.CollaborationTarget, roster []collaboration.TargetResource) (CollaborationQueueReceipt, CollaborationProposeSwitchParams) {
	t.Helper()
	p := CollaborationProposeSwitchParams{CollaborationQueueRequest: f.request(), TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch,
		ExpectedConfigVersion: target.ConfigVersion, Roster: roster}
	r, err := f.svc.ProposeTargetSwitch(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	return r, p
}

func switchRoster(t *testing.T, target db.CollaborationTarget) []collaboration.TargetResource {
	t.Helper()
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	return roster
}

func switchGrants(t *testing.T, f *collaborationQueueFixture, target pgtype.UUID) []db.CollaborationGrant {
	t.Helper()
	grants, err := f.svc.Queries.ListCollaborationTargetGrants(f.ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	return grants
}

func switchConsent(f *collaborationQueueFixture, target, proposal pgtype.UUID, grant db.CollaborationGrant) CollaborationSwitchConsentParams {
	return CollaborationSwitchConsentParams{CollaborationQueueRequest: f.request(), TargetID: target, ProposalID: proposal, AgentID: grant.AgentID, Identity: collaborationGrantIdentity(grant)}
}

func consentSwitch(t *testing.T, f *collaborationQueueFixture, target, proposal pgtype.UUID, grants []db.CollaborationGrant) {
	t.Helper()
	for _, grant := range grants {
		if _, err := f.svc.ConsentTargetSwitchResource(collaborationContext(util.UUIDToString(grant.OwnerUserID)), switchConsent(f, target, proposal, grant)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCollaborationAtomicSwitchStagingDoesNotChangeLiveConfiguration(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	f.submit(t, f.handler, "retained waiting input")
	before := queueTarget(t, f, f.targetID)
	grants := switchGrants(t, f, before.ID)
	inputs, err := f.svc.Queries.ListCollaborationCommands(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	proposal, request := proposeSwitch(t, f, before, switchRoster(t, before))
	if !proposal.ProposalID.Valid || proposal.Code != "switch_proposed" {
		t.Fatalf("missing durable proposal identity: %+v", proposal)
	}
	replay, err := f.svc.ProposeTargetSwitch(f.ctx, request)
	if err != nil || !reflect.DeepEqual(replay, proposal) {
		t.Fatalf("proposal replay changed its receipt: %+v %v", replay, err)
	}
	consent := switchConsent(f, before.ID, proposal.ProposalID, grants[0])
	if _, err = f.svc.ConsentTargetSwitchResource(collaborationContext(f.handler), consent); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("organizer consented for another resource owner: %v", err)
	}
	consent.RequestID = uuid.NewString()
	if _, err = f.svc.ConsentTargetSwitchResource(f.ctx, consent); err != nil {
		t.Fatal(err)
	}
	afterInputs, err := f.svc.Queries.ListCollaborationCommands(f.ctx, f.issue.SessionID)
	if err != nil || !reflect.DeepEqual(before, queueTarget(t, f, before.ID)) || !reflect.DeepEqual(grants, switchGrants(t, f, before.ID)) || !reflect.DeepEqual(inputs, afterInputs) {
		t.Fatalf("staging or candidate consent mutated live configuration/grants/input: %v", err)
	}
	view, err := f.svc.GetTargetSwitchProposal(f.ctx, f.issue.Issue.ID, before.ID, proposal.ProposalID)
	if err != nil || view.Status != "consent_ready" || len(view.Resources) != 1 || !view.Resources[0].Consented {
		t.Fatalf("candidate consent readiness: %+v %v", view, err)
	}
	if _, err = f.svc.GetTargetSwitchProposal(collaborationContext(f.outsider), f.issue.Issue.ID, before.ID, proposal.ProposalID); err == nil {
		t.Fatal("unrelated account read resource identities")
	}
	recovered, err := f.svc.GetSwitchOperation(f.ctx, f.issue.Issue.ID, request.RequestID)
	if err != nil || !reflect.DeepEqual(recovered, proposal) {
		t.Fatalf("GET recovery did not return the original proposal receipt: %+v %v", recovered, err)
	}
}

func TestCollaborationAtomicSwitchRequiresEachOwnersNewConsent(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	target, _ := seedQueueSquad(t, f, true)
	grants := switchGrants(t, f, target.ID)
	proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
	consentSwitch(t, f, target.ID, proposal.ProposalID, grants[:1])
	p := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}
	denied, err := f.svc.CommitTargetSwitch(f.ctx, p)
	if !collaborationErrorCode(err, "switch_consent_required") || !reflect.DeepEqual(target, queueTarget(t, f, target.ID)) || !reflect.DeepEqual(grants, switchGrants(t, f, target.ID)) {
		t.Fatalf("old live/leader grants satisfied the unconsented proposal: %v", err)
	}
	consentSwitch(t, f, target.ID, proposal.ProposalID, grants[1:])
	replay, err := f.svc.CommitTargetSwitch(f.ctx, p)
	if !collaborationErrorCode(err, "switch_consent_required") || !reflect.DeepEqual(denied, replay) {
		t.Fatalf("late consent changed a refused original commit intent: %+v %v", replay, err)
	}
	p.RequestID = uuid.NewString()
	r, err := f.svc.CommitTargetSwitch(f.ctx, p)
	if err != nil || r.Code != "target_switched" {
		t.Fatalf("fully consented Switch failed: %+v %v", r, err)
	}
	after := queueTarget(t, f, target.ID)
	if after.ID != target.ID || after.ResourceID != target.ResourceID || after.Kind != target.Kind || after.TargetEpoch != target.TargetEpoch+1 ||
		after.ConfigVersion != target.ConfigVersion+1 || after.HoldVersion != target.HoldVersion+1 || after.HoldReason.String != "switch_hold" || after.QueueState != "paused" {
		t.Fatalf("Switch changed typed identity or bypassed explicit Resume: %+v", after)
	}
	newGrants := switchGrants(t, f, target.ID)
	if len(newGrants) != 4 || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NULL AND config_version=2 AND target_epoch=2`, target.ID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("atomic Switch did not materialize exactly the proposed grants, or launched work")
	}
	if _, code, err := authorizeCollaborationTargetInput(f.ctx, f.svc.Queries, f.issue.Issue.ID, after); err != nil || code != "" {
		t.Fatalf("committed Switch was not fully authorized: %s %v", code, err)
	}
	if replay, err = f.svc.CommitTargetSwitch(f.ctx, p); err != nil || !reflect.DeepEqual(replay, r) || !reflect.DeepEqual(after, queueTarget(t, f, target.ID)) {
		t.Fatalf("Switch replay advanced a version/hold: %+v %v", replay, err)
	}
	if recovered, err := f.svc.GetSwitchOperation(f.ctx, f.issue.Issue.ID, p.RequestID); err != nil || !reflect.DeepEqual(recovered, r) {
		t.Fatalf("committed Switch GET recovery: %+v %v", recovered, err)
	}
}

func TestCollaborationAtomicSwitchRejectsStaleConfigurationAndConsent(t *testing.T) {
	for _, scenario := range []string{"configuration", "owner_account", "owner_participant", "owner_disabled", "binding", "lifecycle", "target_version", "hold", "not_paused", "revoked_consent"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			switchCleanup(t, f)
			target, member := seedQueueSquad(t, f, true)
			proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
			consentSwitch(t, f, target.ID, proposal.ProposalID, switchGrants(t, f, target.ID))
			p := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}
			want := "stale_switch_consent"
			switch scenario {
			case "configuration":
				f.fx.Exec(t, `UPDATE agent SET collaboration_config_epoch=collaboration_config_epoch+1 WHERE id=$1`, member.AgentID)
				want = "stale_switch_proposal"
			case "owner_account":
				f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, member.OwnerUserID)
			case "owner_participant":
				f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, member.OwnerUserID)
			case "owner_disabled":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, member.OwnerUserID)
			case "binding":
				f.fx.Exec(t, `UPDATE collaboration_runtime_enrollment SET binding_epoch=binding_epoch+1 WHERE id=$1`, member.BindingID)
			case "lifecycle":
				f.fx.Exec(t, `UPDATE issue_collaboration SET lifecycle_epoch=lifecycle_epoch+1 WHERE issue_id=$1`, f.issue.Issue.ID)
				p.ExpectedLifecycleEpoch++
				want = "stale_switch_proposal"
			case "target_version":
				f.fx.Exec(t, `UPDATE collaboration_target SET target_epoch=target_epoch+1 WHERE id=$1`, target.ID)
				p.Control.ExpectedTargetEpoch++
				want = "stale_switch_proposal"
			case "hold":
				f.fx.Exec(t, `UPDATE collaboration_target SET hold_version=hold_version+1 WHERE id=$1`, target.ID)
				want = "stale_hold"
			case "not_paused":
				f.fx.Exec(t, `UPDATE collaboration_target SET queue_state='active',hold_reason=NULL WHERE id=$1`, target.ID)
				want = "target_pause_required"
			case "revoked_consent":
				_, err := f.svc.RevokeTargetSwitchConsent(collaborationContext(f.handler), CollaborationRevokeSwitchConsentParams{CollaborationQueueRequest: f.request(), TargetID: target.ID, ProposalID: proposal.ProposalID, AgentID: member.AgentID})
				if err != nil {
					t.Fatal(err)
				}
			}
			before := queueTarget(t, f, target.ID)
			grants := switchGrants(t, f, target.ID)
			_, err := f.svc.CommitTargetSwitch(f.ctx, p)
			if !collaborationErrorCode(err, want) || !reflect.DeepEqual(before, queueTarget(t, f, target.ID)) || !reflect.DeepEqual(grants, switchGrants(t, f, target.ID)) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_switch_proposal WHERE id=$1 AND committed_at IS NOT NULL`, proposal.ProposalID) != 0 {
				t.Fatalf("stale authority/control was applied or refusal changed live state: want %s, got %v", want, err)
			}
		})
	}
}

func TestCollaborationAtomicSwitchMovesWaitingAndRetainsProofHistory(t *testing.T) {
	for _, proven := range []bool{false, true} {
		name := "unproven"
		if proven {
			name = "proven_nonlaunch"
		}
		t.Run(name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			switchCleanup(t, f)
			squad, _ := seedQueueSquad(t, f, false)
			input := submitTarget(t, f, squad.ID, f.creator, "old consumed input", pgtype.UUID{})
			attempts := int64(1)
			if proven {
				attempts = 0
			}
			run := f.run(t, input, attempts)
			if _, err := f.svc.StopQueue(f.ctx, f.stopParams(run, 1)); err != nil {
				t.Fatal(err)
			}
			history, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
			if err != nil {
				t.Fatal(err)
			}
			second := submitTarget(t, f, squad.ID, f.handler, "later waiting input", pgtype.UUID{})
			destination, grant := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
			head := submitTarget(t, f, destination.ID, f.creator, "destination head", pgtype.UUID{})
			roster := switchRoster(t, destination)
			roster[0].Role = "leader"
			f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, destination.ResourceID)
			before := queueTarget(t, f, squad.ID)
			oldChannel, err := f.svc.Queries.GetCollaborationChannel(f.ctx, before.ChannelID)
			if err != nil {
				t.Fatal(err)
			}
			proposal, _ := proposeSwitch(t, f, before, roster)
			consentSwitch(t, f, squad.ID, proposal.ProposalID, []db.CollaborationGrant{grant})
			_, err = f.svc.CommitTargetSwitch(f.ctx, CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(before)})
			if !proven {
				if !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(before, queueTarget(t, f, squad.ID)) {
					t.Fatalf("Switch bypassed an old run proof barrier: %v", err)
				}
			} else {
				if err != nil || queueTarget(t, f, squad.ID).ChannelID != head.ChannelID {
					t.Fatalf("consented Switch did not move to new leader: %v", err)
				}
				for i, queued := range []CollaborationQueueReceipt{input, second} {
					if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND channel_id=$2 AND seq=$3 AND state='waiting' AND target_id=$4`, queued.CommandID, head.ChannelID, head.CommandSequence+int64(i)+1, squad.ID) != 1 {
						t.Fatal("waiting inputs did not move in relative order to the destination tail")
					}
				}
			}
			retained, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
			if err != nil || !reflect.DeepEqual(history, retained) {
				t.Fatalf("Switch rewrote consumed history: %v", err)
			}
			current, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, run.ID)
			if err != nil || current.ChannelID != run.ChannelID || current.BatchID != run.BatchID || current.Fence != run.Fence || current.LaunchNonce != run.LaunchNonce {
				t.Fatalf("Switch rewrote old run/proof identity: %v", err)
			}
			afterChannel, err := f.svc.Queries.GetCollaborationChannel(f.ctx, before.ChannelID)
			if err != nil || !reflect.DeepEqual(oldChannel, afterChannel) {
				t.Fatalf("Switch altered old channel controls: %v", err)
			}
		})
	}
}

func TestCollaborationAtomicSwitchLastConsentAndCommitSerialize(t *testing.T) {
	for _, first := range []string{"consent", "commit"} {
		t.Run(first+"_first", func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			switchCleanup(t, f)
			target, member := seedQueueSquad(t, f, true)
			proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
			for _, grant := range switchGrants(t, f, target.ID) {
				if grant.AgentID != member.AgentID {
					consentSwitch(t, f, target.ID, proposal.ProposalID, []db.CollaborationGrant{grant})
				}
			}
			consent := switchConsent(f, target.ID, proposal.ProposalID, member)
			commit := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}
			actorCtx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			ownerCtx, ownerCancel := context.WithTimeout(collaborationContext(f.handler), 15*time.Second)
			defer ownerCancel()
			approve := func(s *CollaborationService) error {
				_, err := s.ConsentTargetSwitchResource(ownerCtx, consent)
				return err
			}
			apply := func(s *CollaborationService) error { _, err := s.CommitTargetSwitch(actorCtx, commit); return err }
			leading, following := approve, apply
			if first == "commit" {
				leading, following = apply, approve
			}
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
			svc := *f.svc
			svc.TxStarter = barrier
			firstDone := make(chan error, 1)
			go func() { firstDone <- leading(&svc) }()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case err := <-firstDone:
				t.Fatalf("first writer did not reach commit: %v", err)
			case <-actorCtx.Done():
				t.Fatal("first writer timed out")
			}
			secondDone := make(chan error, 1)
			go func() { secondDone <- following(f.svc) }()
			waitFIFOBlocked(t, f, blocker, secondDone)
			unblock()
			leadingErr, followingErr := <-firstDone, <-secondDone
			if first == "commit" {
				if !collaborationErrorCode(leadingErr, "switch_consent_required") || followingErr != nil || !reflect.DeepEqual(target, queueTarget(t, f, target.ID)) {
					t.Fatalf("late consent changed refused commit or live state: %v / %v", leadingErr, followingErr)
				}
			} else if leadingErr != nil || followingErr != nil || queueTarget(t, f, target.ID).ConfigVersion != 2 {
				t.Fatalf("commit did not observe complete committed consent: %v / %v", leadingErr, followingErr)
			}
		})
	}
}

func TestCollaborationAtomicSwitchRejectsContradictoryOldChannel(t *testing.T) {
	for _, scenario := range []string{"ended_group_pointer", "paused_target_active_channel"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			switchCleanup(t, f)
			input := f.submit(t, f.creator, "old command")
			run := f.run(t, input, 0)
			if _, err := f.svc.StopQueue(f.ctx, f.stopParams(run, 1)); err != nil {
				t.Fatal(err)
			}
			target := queueTarget(t, f, f.targetID)
			proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
			consentSwitch(t, f, target.ID, proposal.ProposalID, switchGrants(t, f, target.ID))
			if scenario == "ended_group_pointer" {
				f.fx.Exec(t, `UPDATE collaboration_target SET active_group_id=$2 WHERE id=$1`, target.ID, run.GroupID)
				f.fx.Exec(t, `UPDATE collaboration_channel SET active_group_id=$2 WHERE id=$1`, target.ChannelID, run.GroupID)
			} else {
				f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL WHERE id=$1`, target.ID)
				f.fx.Exec(t, `UPDATE collaboration_channel SET queue_state='active' WHERE id=$1`, target.ChannelID)
			}
			before := queueTarget(t, f, target.ID)
			_, err := f.svc.CommitTargetSwitch(f.ctx, CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(before)})
			if !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(before, queueTarget(t, f, target.ID)) {
				t.Fatalf("contradictory old lane allowed Switch: %v", err)
			}
		})
	}
}

func TestCollaborationAtomicSwitchRequiredWritesRollbackTogether(t *testing.T) {
	for _, query := range []string{"CreateCollaborationResourceConsent", "CreateCollaborationChannel", "CompleteCollaborationSwitchProposal", "CreateCollaborationEvent", "CompleteCollaborationOperation"} {
		t.Run(query, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			switchCleanup(t, f)
			target := queueTarget(t, f, f.targetID)
			proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
			grants := switchGrants(t, f, target.ID)
			consentSwitch(t, f, target.ID, proposal.ProposalID, grants)
			p := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}
			f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: query}
			_, err := f.svc.CommitTargetSwitch(f.ctx, p)
			if err == nil || !strings.Contains(err.Error(), "injected collaboration persistence failure") {
				t.Fatalf("required write failure was accepted: %v", err)
			}
			if !reflect.DeepEqual(target, queueTarget(t, f, target.ID)) || !reflect.DeepEqual(grants, switchGrants(t, f, target.ID)) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_switch_proposal WHERE id=$1 AND committed_at IS NOT NULL`, proposal.ProposalID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatal("required write failure left partial Switch state")
			}
		})
	}
}

func TestCollaborationAtomicSwitchUnknownCommitUsesOriginalGETReceipt(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	target := queueTarget(t, f, f.targetID)
	proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
	consentSwitch(t, f, target.ID, proposal.ProposalID, switchGrants(t, f, target.ID))
	p := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	_, err := f.svc.CommitTargetSwitch(f.ctx, p)
	if err == nil || fault.begins.Load() != 1 {
		t.Fatalf("unknown Switch commit succeeded or was automatically retried: %v, attempts=%d", err, fault.begins.Load())
	}
	f.svc.TxStarter = f.pool
	receipt, err := f.svc.GetSwitchOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || receipt.Code != "target_switched" || receipt.ProposalID != proposal.ProposalID || queueTarget(t, f, target.ID).ConfigVersion != 2 {
		t.Fatalf("GET did not recover the original committed Switch: %+v %v", receipt, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1 AND revoked_at IS NULL`, target.ID) != 1 {
		t.Fatal("unknown commit duplicated grants")
	}
}

func TestCollaborationAtomicSwitchCompetingProposalsOnlyOneCanCommit(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	target := queueTarget(t, f, f.targetID)
	first, _ := proposeSwitch(t, f, target, switchRoster(t, target))
	second, _ := proposeSwitch(t, f, target, switchRoster(t, target))
	grants := switchGrants(t, f, target.ID)
	consentSwitch(t, f, target.ID, first.ProposalID, grants)
	consentSwitch(t, f, target.ID, second.ProposalID, grants)
	start, done := make(chan struct{}), make(chan error, 2)
	for _, proposal := range []CollaborationQueueReceipt{first, second} {
		p := CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}
		go func() { <-start; _, err := f.svc.CommitTargetSwitch(f.ctx, p); done <- err }()
	}
	close(start)
	success, stale := 0, 0
	for range 2 {
		err := <-done
		if err == nil {
			success++
		} else if collaborationErrorCode(err, "stale_switch_proposal") {
			stale++
		} else {
			t.Fatalf("unexpected proposal race result: %v", err)
		}
	}
	if success != 1 || stale != 1 || queueTarget(t, f, target.ID).ConfigVersion != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_switch_proposal WHERE issue_id=$1 AND committed_at IS NOT NULL`, f.issue.Issue.ID) != 1 {
		t.Fatal("competing proposals both committed or advanced the target twice")
	}
}

func TestCollaborationAtomicSwitchGrantViewBelongsToOwnerAndProposal(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	target, _ := seedQueueSquad(t, f, true)
	proposal, _ := proposeSwitch(t, f, target, switchRoster(t, target))
	consentSwitch(t, f, target.ID, proposal.ProposalID, switchGrants(t, f, target.ID))
	if _, err := f.svc.CommitTargetSwitch(f.ctx, CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(target)}); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []string{f.creator, f.handler} {
		view, err := f.svc.GetTargetSwitchProposal(collaborationContext(viewer), f.issue.Issue.ID, target.ID, proposal.ProposalID)
		if err != nil {
			t.Fatal(err)
		}
		for _, resource := range view.Resources {
			if resource.OwnerUserID == viewer {
				if resource.GrantID == nil || resource.GrantEpoch == nil || *resource.GrantEpoch != "1" {
					t.Fatal("actual owner cannot discover their committed grant for explicit revocation")
				}
			} else if resource.GrantID != nil || resource.GrantEpoch != nil {
				t.Fatal("another owner or organizer can see a grant they did not authorize")
			}
		}
	}
	current := queueTarget(t, f, target.ID)
	next, _ := proposeSwitch(t, f, current, switchRoster(t, current))
	var currentGrants []db.CollaborationGrant
	for _, grant := range switchGrants(t, f, target.ID) {
		if !grant.RevokedAt.Valid {
			currentGrants = append(currentGrants, grant)
		}
	}
	consentSwitch(t, f, target.ID, next.ProposalID, currentGrants)
	if _, err := f.svc.CommitTargetSwitch(f.ctx, CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: next.ProposalID, Control: targetControl(current)}); err != nil {
		t.Fatal(err)
	}
	oldView, err := f.svc.GetTargetSwitchProposal(f.ctx, f.issue.Issue.ID, target.ID, proposal.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range oldView.Resources {
		if resource.GrantID != nil || resource.GrantEpoch != nil {
			t.Fatal("historical proposal exposed a later configuration's grant")
		}
	}
}
