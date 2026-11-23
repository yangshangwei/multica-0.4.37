package service

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func consentManagedControlTarget(t *testing.T, f *managedCollaborationFixture, target db.CollaborationTarget, agent pgtype.UUID) db.CollaborationGrant {
	t.Helper()
	r, err := f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: target.ID, AgentID: agent,
		ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
		Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: f.binding.ID, BindingEpoch: f.binding.BindingEpoch}})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := f.svc.Queries.GetCollaborationGrant(f.ctx, r.GrantID)
	if err != nil {
		t.Fatal(err)
	}
	return grant
}

func registerManagedControlTarget(t *testing.T, f *managedCollaborationFixture, squad bool) (db.CollaborationTarget, db.CollaborationGrant) {
	t.Helper()
	role, kind := "agent", collaboration.TargetAgent
	agent := f.grant.AgentID
	var resource pgtype.UUID
	if squad {
		role, kind = "leader", collaboration.TargetSquad
		resource = util.MustParseUUID(f.fx.Squad(t, "Control alias "+uuid.NewString(), util.UUIDToString(agent)))
	} else {
		agent = util.MustParseUUID(f.fx.Agent(t, "Control target "+uuid.NewString(), util.UUIDToString(f.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"}))
		resource = agent
	}
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: kind, ResourceID: resource,
		ExpectedTargetSetVersion: session.TargetSetVersion, Roster: []collaboration.TargetResource{{Role: role, AgentID: util.UUIDToString(agent),
			RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}}})
	if err != nil {
		t.Fatal(err)
	}
	target := queueTarget(t, f.collaborationQueueFixture, r.TargetID)
	return target, consentManagedControlTarget(t, f, target, agent)
}

func submitManagedControlInput(t *testing.T, f *managedCollaborationFixture, target db.CollaborationTarget, author, content string) CollaborationQueueReceipt {
	t.Helper()
	r, err := f.svc.SubmitCommand(collaborationPasswordContext(util.MustParseUUID(author), 1), CollaborationSubmitCommandParams{
		CollaborationQueueRequest: f.request(), TargetID: target.ID, InputEventID: util.MustParseUUID(uuid.NewString()),
		ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion, Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func managedTargetResume(t *testing.T, f *managedCollaborationFixture, target pgtype.UUID) CollaborationResumeQueueParams {
	t.Helper()
	return CollaborationResumeQueueParams{CollaborationStopQueueParams: CollaborationStopQueueParams{CollaborationQueueRequest: f.request(),
		Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{targetControl(queueTarget(t, f.collaborationQueueFixture, target))}}, ExpectedExecutionEpoch: f.gateEpoch}
}

func TestCollaborationResumeAllSettlesChannelsIndependently(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	b, _ := registerManagedControlTarget(t, f, false)
	c, grantC := registerManagedControlTarget(t, f, false)
	one := f.submitManaged(t, f.creator, "first waiting author")
	two := f.submitManaged(t, f.handler, "second waiting author")
	activeB := submitManagedControlInput(t, f, b, f.handler, "unproven B")
	runB := f.run(t, activeB, 1)
	seedControlReservation(t, f.collaborationQueueFixture, runB)
	waitingC := submitManagedControlInput(t, f, c, f.creator, "C requires renewed consent")
	if _, err := f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: grantC.ID, ExpectedGrantEpoch: grantC.Epoch}); err != nil {
		t.Fatal(err)
	}
	beforeA := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	beforeB := queueTarget(t, f.collaborationQueueFixture, b.ID)
	beforeC := queueTarget(t, f.collaborationQueueFixture, c.ID)
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || r.Code != "partially_resumed" || r.QueueState != "active" || r.ControlVersion != p.ExpectedControlVersion+1 {
		t.Fatalf("eligible channel did not survive another channel's refusal: %+v %v", r, err)
	}
	results := make(map[pgtype.UUID]CollaborationTargetControlResult)
	for _, result := range r.Targets {
		results[result.TargetID] = result
	}
	if len(results) != 3 || results[f.targetID].Code != "resumed" || results[f.targetID].QueueState != "active" ||
		results[f.targetID].ControlVersion != beforeA.ControlVersion+1 || results[b.ID].Code != "stop_unconfirmed" ||
		results[b.ID].QueueState != "paused" || results[b.ID].ControlVersion != beforeB.ControlVersion ||
		results[c.ID].Code != "resumed" || results[c.ID].QueueState != "active" || results[c.ID].ControlVersion != beforeC.ControlVersion+1 {
		t.Fatalf("partial resume did not retain the per-channel result and exact controls: %+v", results)
	}
	holds := make(map[pgtype.UUID]CollaborationTargetHoldResult)
	for _, hold := range r.TargetHolds {
		holds[hold.TargetID] = hold
	}
	if len(r.Channels) != 3 || len(holds) != 3 || holds[f.targetID].Code != "hold_released" || holds[f.targetID].HoldReason != nil ||
		holds[b.ID].Code != "stop_unconfirmed" || holds[b.ID].HoldVersion != beforeB.HoldVersion ||
		holds[c.ID].Code != "grant_required" || holds[c.ID].HoldReason == nil || *holds[c.ID].HoldReason != collaboration.HoldGrantRequired {
		t.Fatalf("channel proof and target authority were not resolved independently: %+v", r)
	}
	if one.BatchID != two.BatchID || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting'`, []pgtype.UUID{one.CommandID, two.CommandID, waitingC.CommandID}) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runB.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL AND proof_state='unknown'`, runB.ID) != 1 {
		t.Fatal("partial resume consumed waiting inputs or cleared another channel's physical barrier")
	}
	consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, c.ID), grantC.AgentID)
	beforeReplay := controlStateSnapshot(t, f.collaborationQueueFixture)
	replayed, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || !reflect.DeepEqual(r, replayed) || beforeReplay != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("repaired consent turned the old partial result into a new resume: %+v %v", replayed, err)
	}
	resumeC := managedTargetResume(t, f, c.ID)
	cResult, err := f.svc.ResumeQueue(f.ctx, resumeC)
	if err != nil || cResult.Code != "resumed" || cResult.QueueState != "active" || cResult.ControlVersion != beforeC.ControlVersion+1 ||
		!reflect.DeepEqual(beforeB, queueTarget(t, f.collaborationQueueFixture, b.ID)) {
		t.Fatalf("fresh target resume was blocked by unrelated B or changed it: %+v %v", cResult, err)
	}
	stopC := managedTargetResume(t, f, c.ID).CollaborationStopQueueParams
	if _, err = f.svc.StopQueue(f.ctx, stopC); err != nil {
		t.Fatal(err)
	}
	beforeReplay = controlStateSnapshot(t, f.collaborationQueueFixture)
	replayed, err = f.svc.ResumeQueue(f.ctx, resumeC)
	if err != nil || !reflect.DeepEqual(replayed, cResult) || controlStateSnapshot(t, f.collaborationQueueFixture) != beforeReplay {
		t.Fatalf("successful resume replay resurrected a later stopped channel: %+v %v", replayed, err)
	}
}

func TestCollaborationResumeAllowsOfflineAuthorizedWaiting(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	one := f.submitManaged(t, f.creator, "waiting while offline")
	two := f.submitManaged(t, f.handler, "same future batch")
	f.fx.Exec(t, `UPDATE agent_runtime SET status='offline' WHERE id=$1`, f.runtime.ID)
	r, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch})
	if err != nil || r.Code != "resumed" || r.QueueState != "active" || len(r.Targets) != 1 || r.Targets[0].QueueState != "active" {
		t.Fatalf("offline availability was incorrectly treated as missing authority: %+v %v", r, err)
	}
	if one.BatchID != two.BatchID || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='waiting'`, one.BatchID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("resume pretended offline waiting inputs had started")
	}
}

func TestCollaborationResumeAliasesKeepIndependentHolds(t *testing.T) {
	for _, scenario := range []string{"unused_revoked_squad", "squad_head", "revoked_input_target"} {
		t.Run(scenario, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			squad, squadGrant := registerManagedControlTarget(t, f, true)
			if scenario == "unused_revoked_squad" {
				if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: squadGrant.ID, ExpectedGrantEpoch: squadGrant.Epoch}); err != nil {
					t.Fatal(err)
				}
			} else {
				f.submitManaged(t, f.creator, "A input retains direct consent")
				submitManagedControlInput(t, f, squad, f.handler, "S is the effective head")
				if scenario == "revoked_input_target" {
					if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
						t.Fatal(err)
					}
				}
			}
			p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
			r, err := f.svc.ResumeQueue(f.ctx, p)
			expectedCode := "partially_resumed"
			if scenario == "squad_head" {
				expectedCode = "resumed"
			}
			if err != nil || r.Code != expectedCode || r.QueueState != "active" || len(r.Targets) != 2 || len(r.Channels) != 1 {
				t.Fatalf("alias resume did not return both views of one channel: %+v %v", r, err)
			}
			for _, result := range r.Targets {
				if result.ChannelID != squad.ChannelID || result.Code != "resumed" || result.QueueState != "active" {
					t.Fatalf("target authority incorrectly kept the proof-ready channel paused: %+v", r)
				}
			}
			holds := make(map[pgtype.UUID]CollaborationTargetHoldResult)
			for _, hold := range r.TargetHolds {
				holds[hold.TargetID] = hold
			}
			directCode, squadCode := "hold_released", "hold_released"
			if scenario == "unused_revoked_squad" {
				squadCode = "grant_required"
			} else if scenario == "revoked_input_target" {
				directCode = "grant_required"
			}
			if len(holds) != 2 || holds[f.targetID].Code != directCode || holds[squad.ID].Code != squadCode || (holds[squad.ID].HoldReason != nil) != (squadCode != "hold_released") {
				t.Fatalf("an alias erased another target's independent authorization hold: %+v", holds)
			}
			if r.Targets[0].ControlVersion != r.Targets[1].ControlVersion ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatal("alias resume advanced the same channel twice or launched work during Resume")
			}
		})
	}
}

func TestCollaborationResumePreservesHealthyActiveExecution(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "already running")
	f.resumeManaged(t)
	run := f.run(t, input, 1)
	beforeTarget := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || r.ControlVersion != p.ExpectedControlVersion || len(r.Targets) != 1 || r.Targets[0].Code != "already_active" ||
		!reflect.DeepEqual(beforeTarget, queueTarget(t, f.collaborationQueueFixture, f.targetID)) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL`, run.ID) != 1 {
		t.Fatalf("whole resume replaced, stopped, or recreated healthy active work: %+v %v", r, err)
	}
}

func TestCollaborationTargetResumeForcedActiveGroupRefusesWithoutWrites(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "currently running")
	f.resumeManaged(t)
	f.run(t, input, 1)
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
	p := managedTargetResume(t, f, f.targetID)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if !collaborationErrorCode(err, "stop_unconfirmed") || r.QueueState != "active" || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("refused Resume must not become a Stop or advance controls: %+v %v", r, err)
	}
}

func TestCollaborationResumeRollbackAndOriginalCommitRecovery(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "only one waiting batch")
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	fault := &queueFaultStarter{TxStarter: f.pool, eventFailure: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.ResumeQueue(f.ctx, p); err == nil || before != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatal("required event failure left a partial global/channel resume")
	}
	fault.eventFailure, fault.lostCommit = false, true
	beforeBegins := fault.begins.Load()
	if _, err := f.svc.ResumeQueue(f.ctx, p); err == nil || fault.begins.Load() != beforeBegins+1 {
		t.Fatal("unknown resume commit was reported successful or automatically replayed")
	}
	f.svc.TxStarter = f.pool
	committed := controlStateSnapshot(t, f.collaborationQueueFixture)
	lookedUp, err := f.svc.GetControlOperation(f.ctx, f.issue.Issue.ID, p.RequestID)
	if err != nil || lookedUp.Code != "resumed" || committed != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("unknown commit could not recover its original full receipt by GET: %+v %v", lookedUp, err)
	}
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || !reflect.DeepEqual(r, lookedUp) || r.Code != "resumed" || r.ControlVersion != p.ExpectedControlVersion+1 || committed != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("original-key resume recovery changed the committed controls: %+v %v", r, err)
	}
}

func TestCollaborationResumeChannelVersionsRemainExact(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	const sessionVersion int64 = 9007199254740993
	const channelVersion int64 = 9007199254740995
	const holdVersion int64 = 9007199254740997
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	f.fx.Exec(t, `UPDATE collaboration_session SET control_version=$2 WHERE id=$1`, f.issue.SessionID, sessionVersion)
	f.fx.Exec(t, `UPDATE collaboration_channel SET control_version=$2 WHERE id=$1`, target.ChannelID, channelVersion)
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_version=$2 WHERE id=$1`, target.ID, holdVersion)
	if _, err := f.svc.Queries.MirrorCollaborationChannelControl(f.ctx, target.ChannelID); err != nil {
		t.Fatal(err)
	}
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: f.issueControl(t), ExpectedExecutionEpoch: f.gateEpoch}
	r, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || r.ControlVersion != sessionVersion+1 || len(r.Targets) != 1 || r.Targets[0].ControlVersion != channelVersion+1 || len(r.TargetHolds) != 1 || r.TargetHolds[0].HoldVersion != holdVersion+1 {
		t.Fatalf("resume narrowed an exact control version: %+v %v", r, err)
	}
	encoded, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(encoded), `"control_version":"9007199254740994"`) || !strings.Contains(string(encoded), `"control_version":"9007199254740996"`) || !strings.Contains(string(encoded), `"hold_version":"9007199254740998"`) {
		t.Fatalf("control receipt lost decimal-string precision: %s %v", encoded, err)
	}
	replayed, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || !reflect.DeepEqual(r, replayed) {
		t.Fatalf("large control receipt changed on replay: %+v %v", replayed, err)
	}
}
