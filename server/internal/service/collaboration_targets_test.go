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
)

func TestCollaborationCommandRequiresExplicitTarget(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	_, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{
		CollaborationQueueRequest: f.request(), Content: "must not choose a current grant",
	})
	if !collaborationErrorCode(err, "invalid_command_target") {
		t.Fatalf("targetless input must be refused without guessing a resource: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("targetless input persisted a command")
	}
}

func queueTarget(t *testing.T, f *collaborationQueueFixture, id pgtype.UUID) db.CollaborationTarget {
	t.Helper()
	targets, err := f.svc.Queries.ListCollaborationTargets(f.ctx, f.issue.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		if target.ID == id {
			return target
		}
	}
	t.Fatal("test target missing")
	return db.CollaborationTarget{}
}

func submitTarget(t *testing.T, f *collaborationQueueFixture, target pgtype.UUID, author, content string, input pgtype.UUID) CollaborationQueueReceipt {
	t.Helper()
	current := queueTarget(t, f, target)
	if !input.Valid {
		input = util.MustParseUUID(uuid.NewString())
	}
	r, err := f.svc.SubmitCommand(collaborationContext(author), CollaborationSubmitCommandParams{
		CollaborationQueueRequest: f.request(), TargetID: target, InputEventID: input,
		ExpectedTargetEpoch: current.TargetEpoch, ExpectedConfigVersion: current.ConfigVersion, Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A squad may share its leader with a direct-agent target. Its resource grants
// do not materialize a separate direct-agent target for the member.
func seedQueueSquad(t *testing.T, f *collaborationQueueFixture, withMember bool) (db.CollaborationTarget, db.CollaborationGrant) {
	t.Helper()
	direct := queueTarget(t, f, f.targetID)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](direct.Roster)
	if err != nil {
		t.Fatal(err)
	}
	roster[0].Role = "leader"
	squad := f.fx.Squad(t, "Synthetic squad "+uuid.NewString(), roster[0].AgentID)
	if withMember {
		if f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) == 0 {
			f.fx.Member(t, f.workspace, f.handler, "member")
		}
		runtime := f.fx.Runtime(t, "Synthetic member runtime", testutil.Cols{"owner_id": f.handler, "status": "offline"})
		agent := f.fx.Agent(t, "Synthetic member "+uuid.NewString(), runtime, testutil.Cols{"owner_id": f.handler})
		f.fx.SquadMember(t, squad, "agent", agent)
		roster = append(roster, collaboration.TargetResource{Role: "member", AgentID: agent, RuntimeID: runtime, ExecutionWorkspaceID: f.workspace, OwnerUserID: f.handler})
	}
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad,
		ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: session.TargetSetVersion, Roster: roster})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_target WHERE id=$1`, r.TargetID)
	var member db.CollaborationGrant
	for _, resource := range roster {
		grant := seedCollaborationGrant(t, f.fx, f.issue.Issue.ID, r.TargetID, 1, resource, 1)
		if resource.Role == "member" {
			member = grant
		}
	}
	return queueTarget(t, f, r.TargetID), member
}

func seedQueueMember(t *testing.T, f *collaborationQueueFixture, root db.CollaborationRun, grant db.CollaborationGrant, attempts int64) db.CollaborationRun {
	t.Helper()
	id := util.MustParseUUID(f.fx.Insert(t, "collaboration_run", testutil.Cols{
		"session_id": root.SessionID, "target_id": root.TargetID, "group_id": root.GroupID, "command_id": root.CommandID, "parent_run_id": root.ID,
		"step_key": "member:" + util.UUIDToString(grant.AgentID), "step_kind": "member", "agent_id": grant.AgentID,
		"target_epoch": root.TargetEpoch.Int64, "config_version": root.ConfigVersion.Int64, "grant_id": grant.ID, "grant_epoch": grant.Epoch,
		"resource_epoch": root.ResourceEpoch, "execution_workspace_id": grant.ExecutionWorkspaceID, "runtime_id": grant.RuntimeID,
		"installation_id": grant.InstallationID, "binding_id": grant.BindingID, "binding_epoch": grant.BindingEpoch, "authorized_attempt_seq": attempts,
		"channel_id": root.ChannelID, "batch_id": root.BatchID,
	}))
	run, err := f.svc.Queries.GetCollaborationRun(f.ctx, db.GetCollaborationRunParams{SessionID: f.issue.SessionID, RunID: id})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestCollaborationTypedTargetsCoalesceRootAndKeepInputIdentity(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, _ := seedQueueSquad(t, f, false)
	event := util.MustParseUUID(uuid.NewString())
	a := submitTarget(t, f, f.targetID, f.creator, "same comment", event)
	s := submitTarget(t, f, squad.ID, f.creator, "same comment", event)
	replay := submitTarget(t, f, f.targetID, f.creator, "same comment", event)
	if a.CommandID == s.CommandID || a.CommandID != replay.CommandID || a.CommandSequence != 1 || s.CommandSequence != 1 {
		t.Fatal("typed roles merged, repeated an input, or shared a sequence")
	}
	a2 := submitTarget(t, f, f.targetID, f.handler, "agent second", pgtype.UUID{})
	s2 := submitTarget(t, f, squad.ID, f.handler, "squad second", pgtype.UUID{})
	if a2.CommandSequence != 2 || s2.CommandSequence != 2 {
		t.Fatal("each target must preserve its own FIFO")
	}
	if a.BatchID != s.BatchID || a.BatchID != a2.BatchID || a.BatchID != s2.BatchID || a.ChannelID != s.ChannelID {
		t.Fatal("same-root inputs did not coalesce into one pending batch")
	}
	// Synthetic group setup does not prove native squad launch. It proves the
	// single root holds every immutable input and uses the effective squad role.
	run := f.run(t, a, 1)
	if run.TargetID != squad.ID || run.StepKind.String != "leader" || run.BatchID != a.BatchID ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND settled_at IS NULL`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='in_flight'`, a.BatchID) != 4 {
		t.Fatal("batch setup lost inputs, duplicated the root, or lost the squad role")
	}
}

func TestCollaborationTargetStopLeavesOtherTargetAndIssueActive(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	b, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.handler), 1)
	ra := f.run(t, submitTarget(t, f, f.targetID, f.creator, "A first", pgtype.UUID{}), 1)
	rb := f.run(t, submitTarget(t, f, b.ID, f.handler, "B first", pgtype.UUID{}), 0)
	a2 := submitTarget(t, f, f.targetID, f.creator, "A waiting", pgtype.UUID{})
	b2 := submitTarget(t, f, b.ID, f.creator, "B waiting", pgtype.UUID{})
	before := queueTarget(t, f, b.ID)
	stopped, err := f.svc.StopQueue(f.ctx, f.stopParams(ra, 1))
	if err != nil || stopped.GroupSettled || stopped.TargetID != ra.TargetID {
		t.Fatalf("target stop: %+v %v", stopped, err)
	}
	if !reflect.DeepEqual(queueTarget(t, f, b.ID), before) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active' AND control_version=1`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL`, rb.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting'`, []pgtype.UUID{a2.CommandID, b2.CommandID}) != 2 {
		t.Fatal("target A stop affected B, the whole issue, or pending root inputs")
	}
	if _, err = f.svc.StopQueue(f.ctx, f.stopParams(rb, 1)); err != nil {
		t.Fatal(err)
	}
	control := CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{targetControl(queueTarget(t, f, b.ID))}}
	denied, err := f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: control, ExpectedExecutionEpoch: enableControlGate(t, f.collaborationFixture)})
	if !collaborationErrorCode(err, "runtime_upgrade_required") {
		t.Fatalf("unrelated A proof incorrectly blocked B or incomplete runtime resumed: %+v %v", denied, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused'`, b.ID) != 1 {
		t.Fatal("refused target resume saved future intent")
	}
}

func TestCollaborationIssueStopFencesChangedAndLaterTargetSets(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	root := f.run(t, f.submit(t, f.creator, "first"), 1)
	old := f.issueControl(t)
	seedQueueSquad(t, f, false)
	if _, err := f.svc.StopQueue(f.ctx, old); !collaborationErrorCode(err, "stale_scope") {
		t.Fatalf("old target set stopped newer work: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL`, root.ID) != 1 {
		t.Fatal("stale issue stop mutated a run")
	}
	waiting := f.submit(t, f.handler, "keep pending")
	r, err := f.svc.StopQueue(f.ctx, f.issueControl(t))
	if err != nil || r.Scope != collaboration.ControlIssue || r.QueueState != "paused" || len(r.Targets) != 2 {
		t.Fatalf("whole issue stop: %+v %v", r, err)
	}
	// Register a fresh pending target through the actual service after the issue
	// pause. Registration cannot create consent or an active control state.
	runtime := f.fx.Runtime(t, "Later target runtime", testutil.Cols{"owner_id": f.creator, "status": "offline"})
	agent := f.fx.Agent(t, "Later target "+uuid.NewString(), runtime, testutil.Cols{"owner_id": f.creator})
	resource := collaboration.TargetResource{Role: "agent", AgentID: agent, RuntimeID: runtime, ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetAgent, ResourceID: util.MustParseUUID(resource.AgentID), ExpectedTargetSetVersion: r.TargetSetVersion, Roster: []collaboration.TargetResource{resource}})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_target WHERE id=$1`, registered.TargetID)
	if registered.QueueState != "paused" || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE target_id=$1`, registered.TargetID) != 0 {
		t.Fatal("new target bypassed pause or manufactured consent")
	}
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: CollaborationStopQueueParams{CollaborationQueueRequest: f.request(), Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{targetControl(queueTarget(t, f, registered.TargetID))}}, ExpectedExecutionEpoch: 2}
	if _, err = f.svc.ResumeQueue(f.ctx, p); !collaborationErrorCode(err, "issue_paused") {
		t.Fatalf("new target resumed through issue pause: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
		t.Fatal("whole stop cancelled pending root input")
	}
}

func TestCollaborationGroupStopWaitsForEveryDescendant(t *testing.T) {
	for _, attempts := range []int64{0, 1} {
		t.Run(string(rune('0'+attempts)), func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			squad, memberGrant := seedQueueSquad(t, f, true)
			root := f.run(t, submitTarget(t, f, squad.ID, f.creator, "squad work", pgtype.UUID{}), 0)
			stop := f.stopParams(root, 1)
			member := seedQueueMember(t, f, root, memberGrant, attempts)
			f.fx.Insert(t, "collaboration_delegation", testutil.Cols{"group_id": root.GroupID, "parent_run_id": root.ID, "request_key": "phase-1", "payload_hash": "synthetic phase", "phase_key": "phase-1", "members": "[]", "continuation_key": "resume-1"})
			r, err := f.svc.StopQueue(f.ctx, stop)
			if err != nil || r.GroupSettled != (attempts == 0) {
				t.Fatalf("descendant stop: %+v %v", r, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='never_launched_proven' AND settled_at IS NOT NULL`, root.ID) != 1 {
				t.Fatal("parent's never-permitted state was not settled")
			}
			if attempts > 0 {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='unknown' AND settled_at IS NULL AND forced_disposition='stopped'`, member.ID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND settled_at IS NULL AND expansion_sealed_at IS NOT NULL`, root.GroupID) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, root.CommandID) != 1 {
					t.Fatal("parent proof incorrectly settled a live descendant and its root input")
				}
			} else if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, root.CommandID) != 1 {
				t.Fatal("all proven-unlaunched runs did not release their root group")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_delegation WHERE group_id=$1 AND state='cancelled'`, root.GroupID) != 1 {
				t.Fatal("stopped group left an internal continuation intent")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1`, f.issue.SessionID) != 2 {
				t.Fatal("squad member consent created an independent direct-agent target")
			}
		})
	}
}

func TestCollaborationDepartureStopsAuthorAndOwnerGroupsOnly(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	b, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.handler), 1)
	d, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	squad, memberGrant := seedQueueSquad(t, f, true)
	aInput := submitTarget(t, f, f.targetID, f.handler, "removed author's work", pgtype.UUID{})
	submitTarget(t, f, squad.ID, f.creator, "same batch with removed resource owner", pgtype.UUID{})
	aRun := f.run(t, aInput, 1)
	bRun := f.run(t, submitTarget(t, f, b.ID, f.creator, "removed owner's resource", pgtype.UUID{}), 1)
	member := seedQueueMember(t, f, aRun, memberGrant, 1)
	dRun := f.run(t, submitTarget(t, f, d.ID, f.creator, "independent target", pgtype.UUID{}), 1)
	var removed, retained []pgtype.UUID
	for _, target := range []pgtype.UUID{f.targetID, b.ID, squad.ID, d.ID} {
		removed = append(removed, submitTarget(t, f, target, f.handler, "removed waiting", pgtype.UUID{}).CommandID)
		retained = append(retained, submitTarget(t, f, target, f.creator, "retained waiting", pgtype.UUID{}).CommandID)
	}
	before := queueTarget(t, f, d.ID)
	_, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)})
	if err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=ANY($1::uuid[]) AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL`, []pgtype.UUID{aRun.ID, bRun.ID, member.ID}) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='cancelled'`, removed) != 4 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting'`, retained) != 4 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1 AND owner_user_id=$2 AND revoked_at IS NULL`, f.issue.Issue.ID, f.handler) != 0 {
		t.Fatal("departure missed an author/owner dependency or cancelled someone else's input")
	}
	if !reflect.DeepEqual(before, queueTarget(t, f, d.ID)) || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL`, dRun.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active' AND control_version=1`, f.issue.SessionID) != 1 {
		t.Fatal("departure paused an independent target or the whole issue")
	}
}

func TestCollaborationResourceRevocationPreservesAllWaitingAuthors(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	b, grant := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.handler), 1)
	run := f.run(t, submitTarget(t, f, b.ID, f.creator, "using another owner's resource", pgtype.UUID{}), 1)
	one := submitTarget(t, f, b.ID, f.handler, "owner waiting", pgtype.UUID{})
	two := submitTarget(t, f, b.ID, f.creator, "creator waiting", pgtype.UUID{})
	p := CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: grant.ID, ExpectedGrantEpoch: 1}
	r, err := f.svc.RevokeGrant(collaborationContext(f.handler), p)
	if err != nil || r.Code != "grant_revoked" {
		t.Fatalf("revoke: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='waiting'`, []pgtype.UUID{one.CommandID, two.CommandID}) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND settled_at IS NULL`, run.ID) != 1 {
		t.Fatal("resource revocation discarded inputs or fabricated termination")
	}
	replay, err := f.svc.RevokeGrant(collaborationContext(f.handler), p)
	if err != nil || !reflect.DeepEqual(replay, r) {
		t.Fatal("resource revocation did not replay its original receipt")
	}
}

func TestCollaborationWholeLifecycleKeepsAllTargetBarriersOnReopen(t *testing.T) {
	f := newCollaborationLifecycleFixture(t)
	squad, memberGrant := seedQueueSquad(t, f, true)
	f.submit(t, f.handler, "direct input in the squad batch")
	root := f.run(t, submitTarget(t, f, squad.ID, f.creator, "squad root", pgtype.UUID{}), 0)
	member := seedQueueMember(t, f, root, memberGrant, 1)
	w1 := f.submit(t, f.creator, "direct waiting")
	w2 := submitTarget(t, f, squad.ID, f.handler, "squad waiting", pgtype.UUID{})
	if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "complete", "done", 1, 1)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='cancelled'`, []pgtype.UUID{w1.CommandID, w2.CommandID}) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1 AND queue_state='paused'`, f.issue.SessionID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NULL AND proof_state='unknown'`, member.ID) != 1 {
		t.Fatal("whole completion missed target work or a member barrier")
	}
	if _, err := f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", 2, 2)); err != nil {
		t.Fatal(err)
	}
	control := f.issueControl(t)
	control.ExpectedLifecycleEpoch = 3
	p := CollaborationResumeQueueParams{CollaborationStopQueueParams: control, ExpectedExecutionEpoch: enableControlGate(t, f.collaborationFixture)}
	resumed, err := f.svc.ResumeQueue(f.ctx, p)
	if err != nil || resumed.Code != "partially_resumed" || len(resumed.Targets) != 2 ||
		resumed.Targets[0].Code != "stop_unconfirmed" || resumed.Targets[1].Code != "stop_unconfirmed" {
		t.Fatalf("reopen lost the per-channel descendant barrier: %+v %v", resumed, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active'`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1 AND queue_state='paused'`, f.issue.SessionID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='cancelled'`, []pgtype.UUID{w1.CommandID, w2.CommandID}) != 2 {
		t.Fatal("reopen revived old queues")
	}
}
