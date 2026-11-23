package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func newManagedSquadFixture(t *testing.T) (*managedCollaborationFixture, pgtype.UUID) {
	t.Helper()
	f, _, member := newManagedSquadResourcesFixture(t, false)
	return f, member
}

func newManagedSquadResourcesFixture(t *testing.T, separateMemberNamespace bool) (*managedCollaborationFixture, *managedCollaborationFixture, pgtype.UUID) {
	t.Helper()
	f := newManagedCollaborationFixture(t)
	memberFixture := *f
	if separateMemberNamespace {
		daemon := uuid.NewString()
		runtimeID := f.fx.Runtime(t, "Independent managed member runtime", testutil.Cols{"owner_id": f.creator, "daemon_id": daemon, "provider": "codex", "status": "online"})
		bindingID := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": f.installation.ID, "workspace_id": f.workspace, "daemon_id": daemon, "principal_user_id": f.creator, "auth_version": 1, "binding_epoch": 1})
		var err error
		memberFixture.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, util.MustParseUUID(runtimeID))
		if err != nil {
			t.Fatal(err)
		}
		memberFixture.binding, err = f.svc.Queries.GetInstallationBinding(f.ctx, util.MustParseUUID(bindingID))
		if err != nil {
			t.Fatal(err)
		}
		memberFixture.daemonCtx = auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: f.creator, Version: 1, Kind: "daemon_token", WorkspaceID: f.workspace, DaemonID: daemon, BindingID: bindingID, BindingEpoch: 1})
	}
	leader := util.UUIDToString(f.grant.AgentID)
	member := f.fx.Agent(t, "Squad member "+uuid.NewString(), util.UUIDToString(memberFixture.runtime.ID), testutil.Cols{"owner_id": f.creator, "permission_mode": "private"})
	squad := f.fx.Squad(t, "Collaboration squad", leader)
	f.fx.SquadMember(t, squad, "agent", member)
	target, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad,
		ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: 2, Roster: []collaboration.TargetResource{
			{Role: "leader", AgentID: leader, RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
			{Role: "member", AgentID: member, RuntimeID: util.UUIDToString(memberFixture.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator},
		}})
	if err != nil {
		t.Fatal(err)
	}
	f.targetID = target.TargetID
	for _, agent := range []string{leader, member} {
		binding := f.binding
		if agent == member {
			binding = memberFixture.binding
		}
		_, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
			AgentID: util.MustParseUUID(agent), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1,
			Identity: CollaborationResourceIdentity{Kind: "managed", InstallationID: f.installation.ID, BindingID: binding.ID, BindingEpoch: 1}})
		if err != nil {
			t.Fatal(err)
		}
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, f.issue.SessionID)
	return f, &memberFixture, util.MustParseUUID(member)
}

func permitSquadRun(t *testing.T, f *managedCollaborationFixture, run pgtype.UUID, claim protocol.CollaborationRunClaimResponse) (string, string) {
	t.Helper()
	nonce := uuid.NewString()
	p, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	return p.BusinessToken, nonce
}

func finishSquadRun(t *testing.T, f *managedCollaborationFixture, run pgtype.UUID, claim protocol.CollaborationRunClaimResponse, token, nonce, outcome string) {
	t.Helper()
	err := f.svc.RecordRuntimeOutcome(context.Background(), run, token, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: nonce, Outcome: outcome})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationSquadCommentCapacityOneLeaderMemberContinuation(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "human instruction")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	p := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") implement this"}
	receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, p)
	if err != nil || len(receipt.RunIDs) != 1 {
		t.Fatalf("member dispatch: %+v %v", receipt, err)
	}
	replay, err := f.svc.CreateRuntimeComment(context.Background(), root, token, p)
	if err != nil || len(replay.RunIDs) != 1 || replay.RunIDs[0] != receipt.RunIDs[0] {
		t.Fatalf("comment replay duplicated dispatch: %+v %v", replay, err)
	}
	memberRun := receipt.RunIDs[0]
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request); !collaborationErrorCode(err, "runtime_busy") {
		t.Fatalf("member inherited leader capacity: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1`, memberRun) != 0 {
		t.Fatal("queued member reserved physical capacity")
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request)
	if err != nil {
		t.Fatal(err)
	}
	memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
	contextView, err := f.svc.RuntimeContext(context.Background(), memberRun, memberToken)
	if err != nil || contextView.StepKind != "member" || len(contextView.Roster) != 2 || contextView.DispatchComment != p.Content {
		t.Fatalf("member context: %+v %v", contextView, err)
	}
	finishSquadRun(t, f, memberRun, memberClaim, memberToken, memberNonce, "failure")
	continuation, _, continuationClaim := f.claimManaged(t)
	stored, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, continuation)
	if err != nil || stored.StepKind.String != "continuation" || continuation == root || continuation == memberRun {
		t.Fatalf("leader continuation: %+v %v", stored, err)
	}
	continuationToken, continuationNonce := permitSquadRun(t, f, continuation, continuationClaim)
	finishSquadRun(t, f, continuation, continuationClaim, continuationToken, continuationNonce, "success")
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, f.issue.SessionID) != 1 {
		t.Fatal("member failure overrode final leader result or group failed to settle")
	}
}

func TestCollaborationSquadRuntimeCommentAuthorityAndMentionBoundary(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "instruction")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, root, claim)
	unknown := util.MustParseUUID(uuid.NewString())
	p := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@outside](mention://agent/" + util.UUIDToString(unknown) + ") [@human](mention://member/" + f.handler + ")"}
	r, err := f.svc.CreateRuntimeComment(context.Background(), root, token, p)
	if err != nil || len(r.RunIDs) != 0 {
		t.Fatalf("unfrozen member dispatched: %+v %v", r, err)
	}
	p.CommentID = util.MustParseUUID(uuid.NewString())
	p.Content = "[@member](mention://agent/" + util.UUIDToString(member) + ") do work"
	if _, err = f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.CreateRuntimeComment(context.Background(), root, token, p); err == nil {
		t.Fatal("stopped business capability dispatched work")
	}
}

func TestCollaborationSquadHumanJoinClaimAndUnreadCancellation(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "root instruction")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	if _, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"}); err != nil {
		t.Fatal(err)
	}
	joined := f.submitManaged(t, f.handler, "joined human followup")
	cancelled := f.submitManaged(t, f.creator, "cancel this unread join")
	if joined.Code != "command_joined_squad" || cancelled.Code != "command_joined_squad" {
		t.Fatalf("human followup did not join current squad: %+v %+v", joined, cancelled)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NOT NULL`, joined.CommandID) != 1 {
		t.Fatal("join consumed input before claim")
	}
	assertSquadExecutionRead(t, f, 2)
	_, err := f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: cancelled.CommandID, ExpectedCollaborationRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_group_dependency WHERE command_id=$1 AND detached_at IS NOT NULL`, cancelled.CommandID) != 1 {
		t.Fatal("cancelled join retained active grants")
	}
	assertSquadExecutionRead(t, f, 1)
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	var memberRun, continuation pgtype.UUID
	for _, id := range list.RunIDs {
		r, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, util.MustParseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		if r.StepKind.String == "member" {
			memberRun = r.ID
		} else if r.StepKind.String == "continuation" {
			continuation = r.ID
		}
	}
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request)
	if err != nil {
		t.Fatal(err)
	}
	memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
	finishSquadRun(t, f, memberRun, memberClaim, memberToken, memberNonce, "success")
	// Runtime polling offers one currently eligible candidate. Discover the
	// continuation after its member releases capacity, as the daemon does.
	list, err = f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range list.RunIDs {
		r, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, util.MustParseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		if r.StepKind.String == "continuation" {
			continuation = r.ID
		}
	}
	continuationClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, continuation, request)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, input := range continuationClaim.Inputs {
		if input.CommandID == util.UUIDToString(joined.CommandID) {
			found = true
		}
		if input.CommandID == util.UUIDToString(cancelled.CommandID) {
			t.Fatal("cancelled unread input entered prompt")
		}
	}
	if !found {
		t.Fatal("continuation omitted joined human input")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1 AND consuming_run_id=$2`, joined.CommandID, continuation) != 1 {
		t.Fatal("claim did not atomically retain consuming continuation")
	}
	assertSquadExecutionRead(t, f, 0)
	_, err = f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: joined.CommandID, ExpectedCollaborationRevision: 1})
	if !collaborationErrorCode(err, "command_not_waiting") {
		t.Fatalf("consumed join remained cancellable: %v", err)
	}
}

func assertSquadExecutionRead(t *testing.T, f *managedCollaborationFixture, unread int) {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.ExecutionView(f.ctx, access)
	if err != nil || view.ProtocolVersion != 5 || len(view.Groups) != 1 || len(view.Groups[0].UnreadInputs) != unread {
		t.Fatalf("squad execution projection: %+v %v", view, err)
	}
}

func TestCollaborationSquadUnreadAuthorDepartureDoesNotStopOtherWork(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unread", true: "consumed"}[consumed], func(t *testing.T) {
			f, member := newManagedSquadFixture(t)
			f.submitManaged(t, f.creator, "root author")
			f.resumeManaged(t)
			root, request, claim := f.claimManaged(t)
			token, nonce := permitSquadRun(t, f, root, claim)
			if _, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"}); err != nil {
				t.Fatal(err)
			}
			joined := f.submitManaged(t, f.handler, "other author's joined input")
			if consumed {
				finishSquadRun(t, f, root, claim, token, nonce, "success")
				claimNext := func(kind string) (pgtype.UUID, protocol.CollaborationRunClaimResponse) {
					t.Helper()
					list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
					if err != nil || len(list.RunIDs) != 1 {
						t.Fatalf("one offered %s: %+v %v", kind, list, err)
					}
					run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, util.MustParseUUID(list.RunIDs[0]))
					if err != nil || run.StepKind.String != kind {
						t.Fatalf("offered run must be %s: got %s, %v", kind, run.StepKind.String, err)
					}
					claimed, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run.ID, request)
					if err != nil {
						t.Fatal(err)
					}
					return run.ID, claimed
				}
				memberRun, memberClaim := claimNext("member")
				memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
				finishSquadRun(t, f, memberRun, memberClaim, memberToken, memberNonce, "success")
				claimNext("continuation")
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, joined.CommandID) != 1 {
					t.Fatal("continuation did not consume the joined author's input before departure")
				}
			}
			_, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(f.handler)})
			if err != nil {
				t.Fatal(err)
			}
			if consumed {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND forced_disposition IS NOT NULL`, f.issue.SessionID) != 1 {
					t.Fatal("consumed author departure did not stop group")
				}
			} else {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND group_id IS NULL`, joined.CommandID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND forced_disposition IS NULL`, f.issue.SessionID) != 1 {
					t.Fatal("unread author departure stopped unrelated work")
				}
				if _, err = f.svc.RuntimeContext(context.Background(), root, token); err != nil {
					t.Fatalf("remaining run lost authority after unread detach: %v", err)
				}
			}
		})
	}
}

func TestCollaborationSquadUnreadGrantRevocationDetachesOnlyAlias(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	squadTarget := f.targetID
	f.submitManaged(t, f.creator, "squad root")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, root, claim)
	if _, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"}); err != nil {
		t.Fatal(err)
	}
	f.targetID = f.grant.TargetID
	joined := f.submitManaged(t, f.handler, "direct leader alias joins squad")
	if joined.Code != "command_joined_squad" {
		t.Fatalf("leader alias did not join: %+v", joined)
	}
	if _, err := f.svc.RevokeGrant(f.ctx, CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(), GrantID: f.grant.ID, ExpectedGrantEpoch: f.grant.Epoch}); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL`, joined.CommandID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_group_dependency WHERE command_id=$1 AND detached_at IS NOT NULL`, joined.CommandID) != 1 {
		t.Fatal("unread alias revocation did not return input with detached dependency")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND forced_disposition IS NULL`, f.issue.SessionID) != 1 {
		t.Fatal("unused alias grant revocation stopped squad")
	}
	f.targetID = squadTarget
	if _, err := f.svc.RuntimeContext(context.Background(), root, token); err != nil {
		t.Fatalf("squad's frozen resources lost authority: %v", err)
	}
	assertSquadExecutionRead(t, f, 0)
}

func TestCollaborationSquadStopReturnsUnreadJoinWithoutRestartingIt(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "root")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, root, claim)
	if _, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"}); err != nil {
		t.Fatal(err)
	}
	joined := f.submitManaged(t, f.handler, "unread next turn")
	if _, err := f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL`, joined.CommandID) != 1 {
		t.Fatal("Stop consumed or cancelled unread valid input")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE released_at IS NULL AND run_id IN (SELECT id FROM collaboration_run WHERE session_id=$1)`, f.issue.SessionID) != 1 {
		t.Fatal("Stop released the potentially live root reservation")
	}
	assertSquadExecutionRead(t, f, 0)
}

func TestCollaborationSquadActivityAndProofRemainSeparate(t *testing.T) {
	f, _ := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "squad outcome")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	event := uuid.NewString()
	for range 2 {
		if err := f.svc.RuntimeSquadActivity(context.Background(), root, token, event, "no_action", "nothing to dispatch"); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.RuntimeSquadActivity(context.Background(), root, token, event, "failed", "changed replay"); !collaborationErrorCode(err, "squad_activity_conflict") {
		t.Fatalf("activity replay overwrote evidence: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND business_outcome='success' AND settled_at IS NULL`, root) != 1 {
		t.Fatal("activity incorrectly supplied termination proof")
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), root, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, f.issue.SessionID) != 1 {
		t.Fatal("leader no_action outcome did not settle after independent proof")
	}
}

func TestCollaborationSquadMissingOutcomeProofSettlesAsIncomplete(t *testing.T) {
	f, _ := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "missing outcome")
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), root, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND business_outcome IS NULL AND recording_incomplete AND settled_at IS NOT NULL`, root) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='failed' AND settled_at IS NOT NULL`, f.issue.SessionID) != 1 {
		t.Fatal("exit zero invented business success or waited forever for lost outcome")
	}
	if err := f.svc.RuntimeSquadActivity(context.Background(), root, token, uuid.NewString(), "action", "late result"); err == nil {
		t.Fatal("late business evidence rewrote settled outcome")
	}
}

func TestCollaborationSquadHumanCommentQueuesAndJoinsWithTypedReceipt(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
	comment := func(body string) CollaborationCommentReceipt {
		p := CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: "[@squad](mention://squad/" + util.UUIDToString(target.ResourceID) + ") " + body}, TargetVersions: []CollaborationCommentTargetVersion{{TargetID: target.ID, ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion}}}
		preview, err := f.svc.PreviewComment(f.ctx, f.issue.Issue.ID, p.CollaborationCommentInput)
		if err != nil || len(preview.Targets) != 1 || preview.Targets[0].State != "waiting" {
			t.Fatalf("fully consented squad comment preview: %+v %v", preview, err)
		}
		r, err := f.svc.CreateComment(f.ctx, p)
		if err != nil || len(r.Targets) != 1 || r.Targets[0].State != "waiting" {
			t.Fatalf("squad comment receipt: %+v %v", r, err)
		}
		return r
	}
	first := comment("first public instruction")
	if first.Targets[0].Code != "command_queued" || first.Targets[0].GroupID.Valid {
		t.Fatal("root comment claimed a joined group")
	}
	f.resumeManaged(t)
	root, _, claim := f.claimManaged(t)
	token, _ := permitSquadRun(t, f, root, claim)
	if _, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") work"}); err != nil {
		t.Fatal(err)
	}
	joined := comment("additional instruction")
	if joined.Targets[0].Code != "command_joined_squad" || joined.Targets[0].WaitingReason != "joined_squad" || joined.Targets[0].GroupID != util.MustParseUUID(claim.Identity.GroupID) {
		t.Fatalf("human join receipt lost group identity: %+v", joined.Targets[0])
	}
	assertSquadExecutionRead(t, f, 1)
}

func TestCollaborationSquadContinuationNonlaunchReturnsOnlyItsInput(t *testing.T) {
	f, _ := newManagedSquadFixture(t)
	first := f.submitManaged(t, f.creator, "root already launched")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	joined := f.submitManaged(t, f.handler, "continuation never launched")
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	continuation, _, continuationClaim := f.claimManaged(t)
	if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, continuation, request); err != nil {
		t.Fatal(err)
	}
	proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: continuationClaim.Identity, LastAuthorizedAttemptSeq: "0", SealedAttemptSeq: "0", State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), continuation, continuationClaim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL`, joined.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='failed' AND group_id IS NOT NULL`, first.CommandID) != 1 {
		t.Fatal("continuation nonlaunch revived root or consumed its unread human input")
	}
	assertSquadExecutionRead(t, f, 0)
}
