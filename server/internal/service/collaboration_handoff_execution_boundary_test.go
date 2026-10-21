package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Managed protocol operations below are real database-backed service writes.
// The sealed proof is a test prerequisite, not evidence of a provider process.
func TestCollaborationTransferredHandlersShareExistingGrantAndControls(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": f.outsider, "username": "handoff" + uuid.NewString()[:8],
		"password_hash": "fixture-not-used-for-login", "must_change_password": false}, "user_id=$1", f.outsider)
	grantBefore, err := f.svc.Queries.GetCollaborationGrant(f.ctx, f.grant.ID)
	if err != nil {
		t.Fatal(err)
	}
	controlBefore := handoffBoundaryExecutionSnapshot(t, f)
	invite := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(f.outsider)}
	if receipt, err := f.svc.InviteParticipant(f.ctx, invite); err != nil || receipt.Code != "participant_added" {
		t.Fatalf("invite new collaborator: %+v %v", receipt, err)
	}
	transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 2), TargetUserID: util.MustParseUUID(f.outsider)}, Note: "Stage ready for the new handler"}
	formerCtx := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	receipt, err := f.svc.Transfer(formerCtx, transfer)
	if err != nil || receipt.Code != "transferred" || receipt.CollaborationRevision != 3 || controlBefore != handoffBoundaryExecutionSnapshot(t, f) {
		t.Fatalf("ordinary transfer changed existing authorization or execution: %+v %v", receipt, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=ANY($2::uuid[]) AND revoked_at IS NULL`, f.issue.Issue.ID, []string{f.creator, f.handler, f.outsider}) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.outsider) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatal("invitation/transfer removed a former participant or expanded source/resource authorization")
	}
	afterTransfer := controlStateSnapshot(t, f.collaborationQueueFixture)
	recovered, err := f.svc.GetWorkflowOperation(formerCtx, f.issue.Issue.ID, transfer.RequestID)
	if err != nil || recovered.Code != receipt.Code || recovered.CollaborationRevision != receipt.CollaborationRevision || afterTransfer != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("former handler could not recover the original transfer by its read-only key: %+v %v", recovered, err)
	}
	for _, actor := range []struct{ role, id string }{{"creator", f.creator}, {"current_handler", f.outsider}, {"former_handler", f.handler}} {
		t.Run(actor.role, func(t *testing.T) {
			ctx := collaborationPasswordContext(util.MustParseUUID(actor.id), 1)
			access, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
			if err != nil || !access.Capabilities.SubmitCommand || !access.Capabilities.ControlAI ||
				(actor.role == "former_handler" && (access.Capabilities.Transfer || access.Capabilities.SubmitAcceptance)) {
				t.Fatalf("transferred participant's current permissions: %+v %v", access, err)
			}
			target := queueTarget(t, f.collaborationQueueFixture, f.targetID)
			input := CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: target.ID,
				InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch, ExpectedConfigVersion: target.ConfigVersion,
				Content: "instruction from " + actor.role}
			queued, err := f.svc.SubmitCommand(ctx, input)
			if err != nil || queued.Code != "command_queued" || !queued.CommandID.Valid {
				t.Fatalf("participant could not use the pre-existing grant: %+v %v", queued, err)
			}
			resume := managedTargetResume(t, f, target.ID)
			if actor.role == "creator" {
				resume.CollaborationStopQueueParams = f.issueControl(t)
			}
			resumed, err := f.svc.ResumeQueue(ctx, resume)
			if err != nil || (resumed.Code != "resumed" && resumed.Code != "already_active") {
				t.Fatalf("participant initial Resume: %+v %v", resumed, err)
			}
			runID, _, claim := f.claimManaged(t)
			if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(queued.CommandID) ||
				claim.Inputs[0].AuthorUserID != actor.id || claim.Inputs[0].Body != input.Content ||
				claim.Identity.IssueID != util.UUIDToString(f.issue.Issue.ID) || claim.Identity.TargetID != util.UUIDToString(target.ID) {
				t.Fatal("using a shared grant lost the new participant's immutable input/target identity")
			}
			nonce := uuid.NewString()
			if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity,
				AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace}); err != nil {
				t.Fatal(err)
			}
			stop := managedTargetResume(t, f, target.ID).CollaborationStopQueueParams
			stopped, err := f.svc.StopQueue(ctx, stop)
			if err != nil || stopped.GroupSettled || stopped.GroupID != util.MustParseUUID(claim.Identity.GroupID) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='unknown' AND settled_at IS NULL`, runID) != 1 {
				t.Fatalf("participant Stop missed the exact permitted execution: %+v %v", stopped, err)
			}
			refusedIntent := managedTargetResume(t, f, target.ID)
			refused, err := f.svc.ResumeQueue(ctx, refusedIntent)
			if !collaborationErrorCode(err, "stop_unconfirmed") {
				t.Fatalf("participant resumed an unproven run: %+v %v", refused, err)
			}
			if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
				t.Fatal(err)
			}
			afterProof := controlStateSnapshot(t, f.collaborationQueueFixture)
			replayed, err := f.svc.ResumeQueue(ctx, refusedIntent)
			if !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(replayed, refused) || afterProof != controlStateSnapshot(t, f.collaborationQueueFixture) {
				t.Fatalf("old refused Resume became deferred authority after proof: %+v %v", replayed, err)
			}
			if fresh, err := f.svc.ResumeQueue(ctx, managedTargetResume(t, f, target.ID)); err != nil || fresh.Code != "resumed" ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, queued.CommandID) != 1 {
				t.Fatalf("participant fresh Resume failed or replayed its stopped command: %+v %v", fresh, err)
			}
			currentGrant, err := f.svc.Queries.GetCollaborationGrant(ctx, f.grant.ID)
			if err != nil || !reflect.DeepEqual(currentGrant, grantBefore) || f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE issue_id=$1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_target WHERE session_id=$1`, f.issue.SessionID) != 1 {
				t.Fatal("per-participant use required new owner consent or expanded targets/grants")
			}
			t.Logf("role=%s actor=%s issue=%s grant=%s target=%s command=%s run=%s", actor.role, actor.id, util.UUIDToString(f.issue.Issue.ID),
				util.UUIDToString(f.grant.ID), util.UUIDToString(target.ID), util.UUIDToString(queued.CommandID), util.UUIDToString(runID))
		})
	}
}

func TestCollaborationActiveAcceptanceAndResultsRequireExplicitCompletion(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
	input := f.submitManaged(t, f.handler, "run continues through acceptance")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	nonce := uuid.NewString()
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity,
		AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	waiting := f.submitManaged(t, f.creator, "future input stays queued")
	before := handoffBoundaryExecutionSnapshot(t, f)
	formerCtx := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	submit := handoffAcceptance(t, f.collaborationQueueFixture, 1, "Stage result and tests are ready; the AI is still active")
	receipt, err := f.svc.SubmitAcceptance(formerCtx, submit)
	if err != nil || receipt.Code != "acceptance_submitted" || receipt.QueueState != "active" || before != handoffBoundaryExecutionSnapshot(t, f) {
		t.Fatalf("acceptance stopped/resumed/restarted the live permitted run: %+v %v", receipt, err)
	}
	creator := mustAccess(t, f.collaborationQueueFixture, f.ctx)
	former := mustAccess(t, f.collaborationQueueFixture, formerCtx)
	if creator.Issue.AssigneeID != util.MustParseUUID(f.creator) || creator.StatusCategory != "in_review" || !creator.Capabilities.Complete ||
		!former.Capabilities.SubmitCommand || !former.Capabilities.ControlAI || former.Capabilities.Transfer || former.Capabilities.SubmitAcceptance {
		t.Fatalf("acceptance completed the issue or removed former-handler rights: creator=%+v former=%+v", creator, former)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='unknown' AND business_outcome IS NULL AND forced_disposition IS NULL AND settled_at IS NULL`, runID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND stop_requested_at IS NULL AND confirmed_at IS NULL AND proof IS NULL`, runID) != 1 {
		t.Fatal("acceptance altered live execution, proof intent or future waiting input")
	}
	afterAcceptance := controlStateSnapshot(t, f.collaborationQueueFixture)
	if replay, err := f.svc.SubmitAcceptance(formerCtx, submit); err != nil || !reflect.DeepEqual(replay, receipt) || afterAcceptance != controlStateSnapshot(t, f.collaborationQueueFixture) {
		t.Fatalf("former-handler recovery reapplied acceptance: %+v %v", replay, err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity,
		EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: nonce, Outcome: "success", Body: "AI execution and validation succeeded after acceptance"}); err != nil {
		t.Fatalf("acceptance revoked a valid running business capability: %v", err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
		t.Fatal(err)
	}
	afterAI := handoffBoundaryExecutionSnapshot(t, f)
	comment, err := f.svc.CreateComment(formerCtx, CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(),
		CollaborationCommentInput: CollaborationCommentInput{Content: "Stage result: tests passed. This records validation; creator acceptance is still required."}})
	if err != nil || comment.Code != "comment_created" || len(comment.Targets) != 0 || afterAI != handoffBoundaryExecutionSnapshot(t, f) {
		t.Fatalf("human test-pass record changed whole completion or execution: %+v %v", comment, err)
	}
	current := mustAccess(t, f.collaborationQueueFixture, f.ctx)
	if current.StatusCategory != "in_review" || current.Issue.AssigneeID != util.MustParseUUID(f.creator) || current.LifecycleEpoch != 1 || current.QueueState != "active" ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='exited_proven' AND business_outcome='success' AND settled_at IS NOT NULL`, runID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='succeeded'`, input.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND author_id=$2 AND content=$3`, comment.CommentID, f.handler, "Stage result: tests passed. This records validation; creator acceptance is still required.") != 1 {
		t.Fatal("AI success, stage submission or human validation implicitly completed the issue")
	}
	complete := lifecycleParams(t, f.collaborationQueueFixture, "complete", "done", current.Revision, current.LifecycleEpoch)
	beforeRefusal := handoffBoundaryExecutionSnapshot(t, f)
	if _, err = f.svc.ChangeLifecycle(formerCtx, complete); !collaborationErrorCode(err, "forbidden") || beforeRefusal != handoffBoundaryExecutionSnapshot(t, f) {
		t.Fatalf("former handler's successful validation granted whole completion: %v", err)
	}
	complete.RequestID = uuid.NewString()
	completed, err := f.svc.ChangeLifecycle(f.ctx, complete)
	if err != nil || completed.Code != "completed" || completed.QueueState != "closed" ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done' AND creator_id=$2`, f.issue.Issue.ID, f.creator) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND reason='issue_completed'`, waiting.CommandID) != 1 {
		t.Fatalf("explicit creator completion did not close only the whole issue: %+v %v", completed, err)
	}
	t.Logf("acceptance_issue=%s author=%s creator=%s grant=%s active_run=%s original_command=%s stage_comment=%s completion_revision=%d",
		util.UUIDToString(f.issue.Issue.ID), f.handler, f.creator, util.UUIDToString(f.grant.ID), util.UUIDToString(runID), util.UUIDToString(input.CommandID),
		util.UUIDToString(comment.CommentID), completed.CollaborationRevision)
}

// Compare only execution-owned facts: human handoffs legitimately add their
// own event/operation and change issue responsibility/status, never these rows.
func handoffBoundaryExecutionSnapshot(t *testing.T, f *managedCollaborationFixture) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
		'session',(SELECT to_jsonb(s)-'event_seq' FROM collaboration_session s WHERE s.id=$1),
		'channels',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_channel c WHERE c.session_id=$1),
		'targets',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM collaboration_target t WHERE t.session_id=$1),
		'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_grant g WHERE g.issue_id=$2),
		'commands',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM collaboration_command c WHERE c.session_id=$1),
		'batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY b.id) FROM collaboration_batch b WHERE b.session_id=$1),
		'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM collaboration_execution_group g WHERE g.session_id=$1),
		'runs',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM collaboration_run r WHERE r.session_id=$1),
		'inputs',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.group_id,i.command_id) FROM collaboration_group_input i JOIN collaboration_execution_group g ON g.id=i.group_id WHERE g.session_id=$1),
		'dependencies',(SELECT jsonb_agg(to_jsonb(d) ORDER BY d.group_id,d.command_id,d.target_id) FROM collaboration_group_dependency d JOIN collaboration_execution_group g ON g.id=d.group_id WHERE g.session_id=$1),
		'terminations',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.run_id) FROM collaboration_termination p WHERE p.issue_id=$2),
		'attempts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.run_id,a.attempt_seq) FROM collaboration_attempt a JOIN collaboration_run r ON r.id=a.run_id WHERE r.session_id=$1),
		'capabilities',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.run_id,c.attempt_seq) FROM collaboration_business_capability c JOIN collaboration_run r ON r.id=c.run_id WHERE r.session_id=$1),
		'reservations',(SELECT jsonb_agg(to_jsonb(v) ORDER BY v.run_id) FROM collaboration_runtime_reservation v JOIN collaboration_run r ON r.id=v.run_id WHERE r.session_id=$1)
	)::text`, f.issue.SessionID, f.issue.Issue.ID).Scan(&snapshot)
	return snapshot
}
