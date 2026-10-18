package service

import (
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

func holdChannelSnapshot(t *testing.T, f *managedCollaborationFixture, channel pgtype.UUID) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_channel c WHERE id=$1`, channel).Scan(&snapshot)
	return snapshot
}

func TestCollaborationTargetConsentRetainsInitialHold(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target t WHERE id=$1
		AND to_jsonb(t)->>'hold_reason'='initial' AND to_jsonb(t)->>'hold_version'='2'`, f.targetID) != 1 {
		t.Fatal("new owner consent must advance the independent hold version without releasing the initial hold")
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatal("an already-current consent must not change the target hold, channel control, or event history")
	}
}

func TestCollaborationResourceHoldDoesNotPauseChannel(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "running on the revoked resource")
	f.resumeManaged(t)
	run := f.run(t, input, 1)
	seedControlReservation(t, f.collaborationQueueFixture, run)
	waiting := f.submitManaged(t, f.handler, "retained after revocation")
	before := holdChannelSnapshot(t, f, run.ChannelID)
	p := CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}
	r, err := f.svc.RevokeGrant(f.ctx, p)
	if err != nil || r.Code != "grant_revoked" {
		t.Fatalf("revoke current grant: %+v %v", r, err)
	}
	if before != holdChannelSnapshot(t, f, run.ChannelID) {
		t.Fatal("resource-only revocation changed the channel pause, control version, or active group")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target t WHERE id=$1 AND to_jsonb(t)->>'hold_reason'='grant_required'`, f.targetID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND settled_at IS NULL`, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
		t.Fatal("resource hold lost a waiting input, stop intent, or unproven physical reservation")
	}
	after := controlStateSnapshot(t, f.collaborationQueueFixture)
	replay, err := f.svc.RevokeGrant(f.ctx, p)
	if err != nil || !reflect.DeepEqual(replay, r) || after != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("revocation replay advanced the target hold: %+v %v", replay, err)
	}
}

func TestCollaborationUnusedAliasHoldKeepsHealthyGroup(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "direct target is running")
	f.resumeManaged(t)
	run := f.run(t, input, 1)
	squad, grant := registerManagedControlTarget(t, f, true)
	before := holdChannelSnapshot(t, f, run.ChannelID)
	direct := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if squad.ChannelID != run.ChannelID {
		t.Fatal("test must exercise aliases on the same physical channel")
	}
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: grant.ID, ExpectedGrantEpoch: grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	current, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: run.SessionID, RunID: run.ID})
	if err != nil || !reflect.DeepEqual(current, run) || !reflect.DeepEqual(direct, queueTarget(t, f.collaborationQueueFixture, f.targetID)) ||
		before != holdChannelSnapshot(t, f, run.ChannelID) {
		t.Fatalf("revoking an unused alias stopped or changed healthy direct work: %+v %v", current, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target t WHERE id=$1 AND queue_state='active' AND to_jsonb(t)->>'hold_reason'='grant_required'`, squad.ID) != 1 {
		t.Fatal("only the unused alias should retain its target hold while its channel stays active")
	}
	state := controlStateSnapshot(t, f.collaborationQueueFixture)
	r, err := f.svc.ResumeQueue(f.ctx, managedTargetResume(t, f, f.targetID))
	if err != nil || r.Code != "already_active" || state != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("an invalid unused alias refused healthy direct-target Resume: %+v %v", r, err)
	}
}

func TestCollaborationTargetResumeHealthyBusyIsAlreadyActive(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "a healthy active group")
	f.resumeManaged(t)
	f.run(t, input, 1)
	p := managedTargetResume(t, f, f.targetID)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || r.Code != "already_active" || len(r.Channels) != 1 || r.Channels[0].UnsettledRuns != 1 ||
		len(r.Targets) != 1 || r.Targets[0].UnsettledRuns != 1 || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("healthy active work must be acknowledged without a stop or deferred resume: %+v %v", r, err)
	}
}

func TestCollaborationAccountInvalidationFindsNonRootBatchAuthor(t *testing.T) {
	f := newAccountReconciliationFixture(t)
	first := f.submit(t, f.first, f.other, 1)
	second := f.submit(t, f.first, f.target, 1)
	_, target := f.targetFor(t, f.first)
	grants, err := f.queue.Queries.ListCollaborationTargetGrants(t.Context(), target.ID)
	if err != nil || len(grants) != 1 {
		t.Fatalf("test target grants: %d %v", len(grants), err)
	}
	run := f.run(t, f.first, first, grants[0].ID, 1)
	if first.BatchID != second.BatchID || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE owner_user_id=$1`, f.target) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE author_user_id=$1 AND state='waiting'`, f.target) != 0 {
		t.Fatal("test must isolate a non-root in-flight author without owned resources or waiting inputs")
	}
	f.change(t, f.target, "disable", 1)
	f.drain(t)
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND settled_at IS NULL`, run.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND queue_state='paused'`, run.ChannelID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, second.CommandID) != 1 {
		t.Fatal("account invalidation missed a non-root batch author's consumed input or cleared its proof barrier")
	}
}

func TestCollaborationReconsentHoldRequiresFreshResumeBeforeLaunch(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "waiting for explicit resumed authority")
	f.resumeManaged(t)
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	f.grant = consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	r, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(r.RunIDs) != 0 || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("runtime polling bypassed the durable hold after renewed consent: %+v %v", r, err)
	}
	if _, err = f.svc.ResumeQueue(f.ctx, managedTargetResume(t, f, f.targetID)); err != nil {
		t.Fatal(err)
	}
	_, _, claim := f.claimManaged(t)
	if len(claim.Inputs) != 1 {
		t.Fatal("fresh explicit Resume did not release exactly the retained waiting input")
	}
}

func TestCollaborationRuntimeHoldBlocksClaimAndPermit(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "prepared execution must honor target hold")
	f.resumeManaged(t)
	runID, claimRequest, claim := f.claimManaged(t)
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='config_invalid',hold_version=hold_version+1 WHERE id=$1`, f.targetID)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, claimRequest); !collaborationErrorCode(err, "target_held") {
		t.Fatalf("claim ignored target hold: %v", err)
	}
	p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p); !collaborationErrorCode(err, "target_held") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("hold allowed a new launch permission or changed execution state: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1`, runID) != 0 {
		t.Fatal("held execution retained a launch attempt")
	}
}

func TestCollaborationResumeRejectsHoldObservedBeforeReconsent(t *testing.T) {
	for _, scope := range []collaboration.ControlScope{collaboration.ControlTarget, collaboration.ControlIssue} {
		t.Run(string(scope), func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.resumeManaged(t)
			p := managedTargetResume(t, f, f.targetID)
			if scope == collaboration.ControlIssue {
				p.CollaborationStopQueueParams = f.issueControl(t)
			}
			beforeChannel := holdChannelSnapshot(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID).ChannelID)
			if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
				t.Fatal(err)
			}
			consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
			current := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			if current.HoldVersion != p.Targets[0].ExpectedHoldVersion+2 || current.HoldReason.String != "grant_required" || beforeChannel != holdChannelSnapshot(t, f, current.ChannelID) {
				t.Fatal("revoke and renewed consent must advance only the target hold observation")
			}
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			r, err := f.svc.ResumeQueue(f.ctx, p)
			if !collaborationErrorCode(err, "stale_hold") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("old Resume released newly confirmed authority: %+v %v", r, err)
			}
			if _, err = f.svc.ResumeQueue(f.ctx, managedTargetResume(t, f, f.targetID)); err != nil {
				t.Fatal(err)
			}
			before = controlStateSnapshot(t, f.collaborationQueueFixture)
			replayed, err := f.svc.ResumeQueue(f.ctx, p)
			if !collaborationErrorCode(err, "stale_hold") || !reflect.DeepEqual(replayed, r) || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("repaired hold changed an old refused request: %+v %v", replayed, err)
			}
		})
	}
}

func TestCollaborationTargetResumeInvalidGrantIsAtomic(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.resumeManaged(t)
	if _, err := f.svc.StopQueue(f.ctx, managedTargetResume(t, f, f.targetID).CollaborationStopQueueParams); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	p := managedTargetResume(t, f, f.targetID)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if !collaborationErrorCode(err, "grant_required") || before != controlStateSnapshot(t, f.collaborationQueueFixture) || r.QueueState != "paused" {
		t.Fatalf("invalid selected grant partially resumed a proof-ready channel: %+v %v", r, err)
	}
	consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
	before = controlStateSnapshot(t, f.collaborationQueueFixture)
	replayed, err := f.svc.ResumeQueue(f.ctx, p)
	if !collaborationErrorCode(err, "grant_required") || !reflect.DeepEqual(replayed, r) || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("renewed authority turned rejected Resume into future intent: %+v %v", replayed, err)
	}
}

func TestCollaborationTargetResumeReleasesOnlySelectedBusyAlias(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	squad, _ := registerManagedControlTarget(t, f, true)
	// Synthetic squad work only verifies control/proof accounting. The runtime
	// capability gate still refuses actual squad launch in the M09 slice.
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL,hold_version=hold_version+1 WHERE id=$1`, squad.ID)
	run := f.run(t, submitManagedControlInput(t, f, squad, f.handler, "healthy busy squad alias"), 1)
	squad = queueTarget(t, f.collaborationQueueFixture, squad.ID)
	direct := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	beforeChannel := holdChannelSnapshot(t, f, direct.ChannelID)
	r, err := f.svc.ResumeQueue(f.ctx, managedTargetResume(t, f, f.targetID))
	if err != nil || r.Code != "resumed" || len(r.Channels) != 1 || r.Channels[0].Code != "already_active" ||
		len(r.TargetHolds) != 1 || r.TargetHolds[0].TargetID != f.targetID || r.TargetHolds[0].Code != "hold_released" ||
		r.TargetHolds[0].HoldVersion != direct.HoldVersion+1 || r.TargetHolds[0].HoldReason != nil ||
		beforeChannel != holdChannelSnapshot(t, f, direct.ChannelID) || !reflect.DeepEqual(squad, queueTarget(t, f.collaborationQueueFixture, squad.ID)) {
		t.Fatalf("selected hold release changed busy channel or an unrelated alias: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL`, run.ID) != 1 {
		t.Fatal("hold release replaced healthy running work")
	}
}

func TestCollaborationResumeAllMaterializesDiscoveredHold(t *testing.T) {
	for _, cause := range []string{"grant", "configuration"} {
		t.Run(cause, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.resumeManaged(t)
			beforeTarget := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			beforeChannel := holdChannelSnapshot(t, f, beforeTarget.ChannelID)
			want := "grant_required"
			if cause == "grant" {
				f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),epoch=epoch+1 WHERE id=$1`, f.grant.ID)
			} else {
				want = "config_invalid"
				f.fx.Exec(t, `UPDATE agent SET instructions='updated execution configuration' WHERE id=$1`, f.grant.AgentID)
			}
			p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
			r, err := f.svc.ResumeQueue(f.ctx, p)
			if err != nil || r.Code != "partially_resumed" || len(r.TargetHolds) != 1 || r.TargetHolds[0].Code != want ||
				r.TargetHolds[0].HoldReason == nil || string(*r.TargetHolds[0].HoldReason) != want || r.TargetHolds[0].HoldVersion != beforeTarget.HoldVersion+1 ||
				beforeChannel != holdChannelSnapshot(t, f, beforeTarget.ChannelID) {
				t.Fatalf("newly discovered target invalidity affected channel or lost its hold: %+v %v", r, err)
			}
		})
	}
}

func TestCollaborationHoldControlWriteFailuresRollback(t *testing.T) {
	for _, needle := range []string{"ResumeCollaborationSession", "ResumeCollaborationChannel", "MirrorCollaborationChannelControl", "SetCollaborationTargetHold", "INSERT INTO collaboration_event", "UPDATE collaboration_operation"} {
		t.Run(needle, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			if _, err := f.svc.ResumeQueue(f.ctx, p); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("required %s failure left a partial channel or hold transition: %v", needle, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("rolled-back Resume retained a false operation receipt")
			}
		})
	}
}

func TestCollaborationControlOperationReplaysOnlyAuthorizedOriginalResult(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	p := managedTargetResume(t, f, f.targetID)
	actor := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	r, err := f.svc.ResumeQueue(actor, p)
	if !collaborationErrorCode(err, "issue_paused") || r.IssueID != f.issue.Issue.ID || r.RequestID != p.RequestID || r.Action != "resume" || r.Scope != collaboration.ControlTarget {
		t.Fatalf("target refusal did not retain original operation identity: %+v %v", r, err)
	}
	f.resumeManaged(t)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	recovered, err := f.svc.GetControlOperation(actor, f.issue.Issue.ID, p.RequestID)
	if err != nil || !reflect.DeepEqual(recovered, r) || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("control recovery reran or replaced the original refusal: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetControlOperation(f.ctx, f.issue.Issue.ID, p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("another participant read an operation belonging to a different actor: %v", err)
	}
	if _, err = f.svc.GetControlOperation(actor, f.issue.Issue.ID, uuid.NewString()); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("unknown operation confused a missing key with lost access: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	if _, err = f.svc.GetControlOperation(actor, f.issue.Issue.ID, p.RequestID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("former participant recovered protected control state: %v", err)
	}
}

func TestCollaborationTargetResumeRefusesContradictoryMirrors(t *testing.T) {
	for _, mutation := range []string{"queue_state='paused'", "control_version=control_version+1", "pause_reason='unmirrored_stop'"} {
		t.Run(mutation, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.resumeManaged(t)
			p := managedTargetResume(t, f, f.targetID)
			f.fx.Exec(t, `UPDATE collaboration_target SET `+mutation+` WHERE id=$1`, f.targetID)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			r, err := f.svc.ResumeQueue(f.ctx, p)
			if !collaborationErrorCode(err, "stop_unconfirmed") || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("Resume silently repaired a contradictory channel mirror: %+v %v", r, err)
			}
		})
	}
}

func TestCollaborationReconfigureRetainsSwitchHoldAndChannelControl(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.resumeManaged(t)
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	beforeChannel := holdChannelSnapshot(t, f, target.ChannelID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE agent SET instructions='explicit revised target configuration' WHERE id=$1`, f.grant.AgentID)
	p := CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(target), Roster: roster}
	r, err := f.svc.ReconfigureTarget(f.ctx, p)
	if err != nil || r.Code != "target_reconfigured" {
		t.Fatalf("prepare revised held target: %+v %v", r, err)
	}
	configured := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if configured.HoldReason.String != "switch_hold" || configured.HoldVersion != target.HoldVersion+1 || configured.ConfigVersion != target.ConfigVersion+1 ||
		configured.TargetEpoch != target.TargetEpoch+1 || configured.ControlVersion != target.ControlVersion || configured.QueueState != target.QueueState ||
		beforeChannel != holdChannelSnapshot(t, f, target.ChannelID) {
		t.Fatal("pending reconfiguration released its target hold or changed channel control")
	}
	consentManagedControlTarget(t, f, configured, f.grant.AgentID)
	consented := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	if consented.HoldReason.String != "switch_hold" || consented.HoldVersion != configured.HoldVersion+1 || beforeChannel != holdChannelSnapshot(t, f, target.ChannelID) {
		t.Fatal("new configuration consent implicitly resumed the target or its channel")
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	replayed, err := f.svc.ReconfigureTarget(f.ctx, p)
	if err != nil || !reflect.DeepEqual(replayed, r) || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("reconfiguration replay revoked renewed consent or advanced its hold: %+v %v", replayed, err)
	}
	resumed, err := f.svc.ResumeQueue(f.ctx, managedTargetResume(t, f, f.targetID))
	if err != nil || resumed.Code != "resumed" || len(resumed.TargetHolds) != 1 || resumed.TargetHolds[0].Code != "hold_released" || beforeChannel != holdChannelSnapshot(t, f, target.ChannelID) {
		t.Fatalf("fresh Resume did not release only the newly consented configuration: %+v %v", resumed, err)
	}
}

func TestCollaborationPromotePersistsDiscoveredTargetHold(t *testing.T) {
	for _, cause := range []string{"configuration", "grant"} {
		t.Run(cause, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			input := f.submitManaged(t, f.handler, "waiting input whose current target authority changed")
			f.resumeManaged(t)
			before := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			beforeChannel := holdChannelSnapshot(t, f, before.ChannelID)
			want := "config_invalid"
			if cause == "configuration" {
				f.fx.Exec(t, `UPDATE agent SET instructions='configuration changed before next poll' WHERE id=$1`, f.grant.AgentID)
			} else {
				want = "grant_required"
				f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
			}
			runs, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(runs.RunIDs) != 0 {
				t.Fatalf("invalid target authority must not create a run: %+v %v", runs, err)
			}
			after := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			if !after.HoldReason.Valid || after.HoldReason.String != want || after.HoldVersion != before.HoldVersion+1 ||
				beforeChannel != holdChannelSnapshot(t, f, before.ChannelID) {
				t.Fatalf("promote detected invalid authority without persisting only its hold: before=%+v after=%+v", before, after)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, input.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1`, f.issue.SessionID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1`, f.runtime.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='target_hold_changed' AND actor_user_id IS NULL`, f.issue.Issue.ID) != 1 {
				t.Fatal("held promotion consumed input, reserved execution capacity, or lost its system audit")
			}
			state := controlStateSnapshot(t, f.collaborationQueueFixture)
			runs, err = f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(runs.RunIDs) != 0 || state != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("duplicate polling repeated the hold transition or created execution: %+v %v", runs, err)
			}
		})
	}
}

func TestCollaborationPromoteHoldRequiredWritesRollback(t *testing.T) {
	for _, failure := range []string{"SetCollaborationTargetHold", "INSERT INTO collaboration_event", "commit"} {
		t.Run(failure, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.handler, "hold failure must not partially commit")
			f.resumeManaged(t)
			f.fx.Exec(t, `UPDATE agent SET instructions='changed before hold fault' WHERE id=$1`, f.grant.AgentID)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			if failure == "commit" {
				f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, commitFailure: true}
			} else {
				f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: failure}
			}
			if _, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("required hold %s failure was hidden or left partial state: %v", failure, err)
			}
		})
	}
}

func TestCollaborationPromoteHoldUnknownCommitIsNotRetried(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.handler, "unknown hold commit must not repeat its audit")
	f.resumeManaged(t)
	f.fx.Exec(t, `UPDATE agent SET instructions='changed before uncertain commit' WHERE id=$1`, f.grant.AgentID)
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("uncertain promotion hold commit was retried or reported successful: %v", err)
	}
	f.svc.TxStarter = f.pool
	state := controlStateSnapshot(t, f.collaborationQueueFixture)
	runs, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(runs.RunIDs) != 0 || state != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='target_hold_changed'`, f.issue.Issue.ID) != 1 {
		t.Fatalf("uncertain hold commit recovery repeated mutation or launched input: %+v %v", runs, err)
	}
}

func TestCollaborationPromoteAvailabilityDoesNotHoldTarget(t *testing.T) {
	for _, reason := range []string{"offline", "busy", "database_failure"} {
		t.Run(reason, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.handler, "temporary unavailability is not lost authority")
			f.resumeManaged(t)
			switch reason {
			case "offline":
				f.fx.Exec(t, `UPDATE agent_runtime SET status='offline' WHERE id=$1`, f.runtime.ID)
			case "busy":
				f.fx.Task(t, util.UUIDToString(f.grant.AgentID), testutil.Cols{"runtime_id": f.runtime.ID, "status": "running"})
			case "database_failure":
				f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: "CollaborationRuntimeNamespaceOccupied"}
			}
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			runs, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if (err != nil) != (reason == "database_failure") || len(runs.RunIDs) != 0 || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("temporary %s changed target authority or consumed an input: %+v %v", reason, runs, err)
			}
		})
	}
}
