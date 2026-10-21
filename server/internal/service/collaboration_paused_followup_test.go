package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationPausedFollowupRequiresProofFreshResumeAndNeverRetriesStoppedInput(t *testing.T) {
	for _, scenario := range []string{"paused_inputs", "empty_target"} {
		t.Run(scenario, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			other, _ := registerManagedControlTarget(t, f, false)
			one := f.submitManaged(t, f.creator, "1 already attempted")
			f.resumeManaged(t)
			runID, claimRequest, claim := f.claimManaged(t)
			if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(one.CommandID) {
				t.Fatal("the original attempted run did not contain exactly input 1")
			}
			nonce := uuid.NewString()
			if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{
				Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
			}); err != nil {
				t.Fatal(err)
			}
			otherBefore := queueTarget(t, f.collaborationQueueFixture, other.ID)
			sessionBefore, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			stop := managedTargetResume(t, f, f.targetID).CollaborationStopQueueParams
			stopped, err := f.svc.StopQueue(f.ctx, stop)
			if err != nil || stopped.QueueState != "paused" || stopped.GroupSettled ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND authorized_attempt_seq=1 AND proof_state='unknown' AND settled_at IS NULL`, runID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
				t.Fatalf("Stop did not retain the original permitted run's barrier: %+v %v", stopped, err)
			}
			originalInputs, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, util.MustParseUUID(claim.Identity.GroupID))
			if err != nil || len(originalInputs) != 1 {
				t.Fatalf("original consumption history fixture: %+v %v", originalInputs, err)
			}
			var waiting []CollaborationQueueReceipt
			if scenario == "paused_inputs" {
				waiting = []CollaborationQueueReceipt{
					f.submitManaged(t, f.handler, "2 submitted during pause"),
					f.submitManaged(t, f.creator, "3 submitted during pause"),
				}
				if waiting[0].QueueState != "paused" || waiting[1].QueueState != "paused" || waiting[0].BatchID != waiting[1].BatchID ||
					waiting[0].BatchID == one.BatchID || waiting[0].ChannelID != one.ChannelID || waiting[1].ChannelID != one.ChannelID ||
					waiting[0].CommandSequence != one.CommandSequence+1 || waiting[1].CommandSequence != one.CommandSequence+2 {
					t.Fatal("inputs submitted after Stop did not retain one separate paused FIFO batch")
				}
			}
			readAndPoll := func() {
				t.Helper()
				before := controlStateSnapshot(t, f.collaborationQueueFixture)
				for range 2 {
					list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
					if err != nil || len(list.RunIDs) != 0 {
						t.Fatalf("poll created or retried work while paused or empty: %+v %v", list, err)
					}
					access, err := f.svc.ResolveIssueAccess(f.ctx, f.svc.Queries, f.issue.Issue.ID)
					if err != nil {
						t.Fatal(err)
					}
					view, err := f.svc.ExecutionView(f.ctx, access)
					if err != nil || view.ProtocolVersion != 5 || view.IssueID != util.UUIDToString(f.issue.Issue.ID) ||
						len(view.Commands) != 1+len(waiting) || len(view.Runs) != f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) {
						t.Fatalf("current execution read lost the retained old/new identities: %+v %v", view, err)
					}
				}
				if controlStateSnapshot(t, f.collaborationQueueFixture) != before {
					t.Fatal("poll/reload changed the retained business state")
				}
			}
			readAndPoll()
			resume := managedTargetResume(t, f, f.targetID)
			beforeRefusal := controlStateSnapshot(t, f.collaborationQueueFixture)
			denied, err := f.svc.ResumeQueue(f.ctx, resume)
			if !collaborationErrorCode(err, "stop_unconfirmed") || controlStateSnapshot(t, f.collaborationQueueFixture) != beforeRefusal {
				t.Fatalf("Resume before original proof bypassed the physical barrier: %+v %v", denied, err)
			}
			proof := managedProof(claim.Identity, nonce)
			if accepted, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil || !accepted.Accepted {
				t.Fatalf("original synthetic proof was not accepted: %+v %v", accepted, err)
			}
			if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
				t.Fatal(err)
			}
			if target := queueTarget(t, f.collaborationQueueFixture, f.targetID); target.QueueState != "paused" || target.ActiveGroupID.Valid ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, one.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != len(waiting) {
				t.Fatal("proof automatically resumed, returned input 1, or consumed paused-period input")
			}
			readAndPoll()
			beforeReplay := controlStateSnapshot(t, f.collaborationQueueFixture)
			if replay, err := f.svc.ResumeQueue(f.ctx, resume); !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(replay, denied) ||
				controlStateSnapshot(t, f.collaborationQueueFixture) != beforeReplay {
				t.Fatalf("old refused Resume became a deferred intention after proof: %+v %v", replay, err)
			}
			if own, err := f.svc.GetControlOperation(f.ctx, f.issue.Issue.ID, resume.RequestID); err != nil || !reflect.DeepEqual(own, denied) {
				t.Fatalf("read recovery recomputed the original refused Resume: %+v %v", own, err)
			}
			if _, err := f.svc.ClaimRuntimeRun(f.daemonCtx, runID, claimRequest); !collaborationErrorCode(err, "execution_stopped") || controlStateSnapshot(t, f.collaborationQueueFixture) != beforeReplay {
				t.Fatal("the old settled run was claimable or its failed claim changed state")
			}
			fresh := managedTargetResume(t, f, f.targetID)
			if resumed, err := f.svc.ResumeQueue(f.ctx, fresh); err != nil || resumed.QueueState != "active" ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1 {
				t.Fatalf("fresh Resume launched work synchronously or failed to release future input: %+v %v", resumed, err)
			}
			if len(waiting) != 0 {
				nextID, nextRequest, next := f.claimManaged(t)
				if nextID == runID || next.Identity.GroupID == claim.Identity.GroupID || next.Identity.LaunchNonce == claim.Identity.LaunchNonce ||
					next.Identity.BatchID != util.UUIDToString(waiting[0].BatchID) || next.Identity.ChannelID != claim.Identity.ChannelID ||
					next.Identity.TargetID != claim.Identity.TargetID || next.Identity.IssueID != claim.Identity.IssueID || next.Identity.SessionID != claim.Identity.SessionID ||
					len(next.Inputs) != 2 || next.Inputs[0].CommandID != util.UUIDToString(waiting[0].CommandID) || next.Inputs[0].AuthorUserID != f.handler || next.Inputs[0].Body != "2 submitted during pause" ||
					next.Inputs[1].CommandID != util.UUIDToString(waiting[1].CommandID) || next.Inputs[1].AuthorUserID != f.creator || next.Inputs[1].Body != "3 submitted during pause" {
					t.Fatalf("fresh Resume did not consume exactly inputs 2/3 once as a new batch: %+v", next)
				}
				if replay, err := f.svc.ClaimRuntimeRun(f.daemonCtx, nextID, nextRequest); err != nil || !reflect.DeepEqual(replay, next) {
					t.Fatalf("same run claim replay changed its input set or authority: %+v %v", replay, err)
				}
				beforeStopReplay := controlStateSnapshot(t, f.collaborationQueueFixture)
				if replay, err := f.svc.StopQueue(f.ctx, stop); err != nil || !reflect.DeepEqual(replay, stopped) || controlStateSnapshot(t, f.collaborationQueueFixture) != beforeStopReplay {
					t.Fatalf("old Stop key reached the new followup run: %+v %v", replay, err)
				}
				nextNonce := uuid.NewString()
				permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, nextID, protocol.CollaborationRunPermitRequest{Identity: next.Identity,
					AttemptSeq: "1", AttemptNonce: nextNonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
				if err != nil {
					t.Fatal(err)
				}
				if err = f.svc.RecordRuntimeOutcome(context.Background(), nextID, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: next.Identity,
					EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: nextNonce, Outcome: "success", Body: "one merged followup result"}); err != nil {
					t.Fatal(err)
				}
				if _, err = f.svc.AcceptRuntimeProof(context.Background(), nextID, next.TerminationToken, managedProof(next.Identity, nextNonce)); err != nil {
					t.Fatal(err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=ANY($1::uuid[]) AND state='succeeded' AND batch_id=$2`,
					[]pgtype.UUID{waiting[0].CommandID, waiting[1].CommandID}, waiting[0].BatchID) != 2 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND batch_id=$2 AND authorized_attempt_seq=1 AND settled_at IS NOT NULL`, f.issue.SessionID, waiting[0].BatchID) != 1 {
					t.Fatal("coalesced paused inputs were not completed by exactly one attempted run")
				}
			}
			readAndPoll()
			beforeOldReplay := controlStateSnapshot(t, f.collaborationQueueFixture)
			if replay, err := f.svc.ResumeQueue(f.ctx, resume); !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(replay, denied) ||
				controlStateSnapshot(t, f.collaborationQueueFixture) != beforeOldReplay {
				t.Fatalf("completed new work changed the earlier refused Resume: %+v %v", replay, err)
			}
			afterInputs, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, util.MustParseUUID(claim.Identity.GroupID))
			session, sessionErr := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
			if err != nil || sessionErr != nil || !reflect.DeepEqual(originalInputs, afterInputs) || !reflect.DeepEqual(otherBefore, queueTarget(t, f.collaborationQueueFixture, other.ID)) ||
				session.QueueState != sessionBefore.QueueState || session.ControlVersion != sessionBefore.ControlVersion ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='stopped'`, one.CommandID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1+len(waiting)/2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1`, f.issue.SessionID) != 1+len(waiting)/2 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1`, runID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != 0 {
				t.Fatal("Resume/poll changed another scope, old history, or retried input 1")
			}
			if target := queueTarget(t, f.collaborationQueueFixture, f.targetID); target.QueueState != "active" || target.ActiveGroupID.Valid {
				t.Fatal("resumed target without further input did not remain idle")
			}
		})
	}
}
