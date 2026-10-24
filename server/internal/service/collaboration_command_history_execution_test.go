package service

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationCommandCancellationBackgroundAttribution(t *testing.T) {
	t.Run("account_invalidation", func(t *testing.T) {
		f := newAccountReconciliationFixture(t)
		input := f.submit(t, f.first, f.target, 1)
		f.change(t, f.target, "disable", 1)
		f.drain(t)
		page, err := f.queue.CommandHistory(collaborationPasswordContext(f.other, 1), f.first, CollaborationCommandHistoryQuery{Limit: 50})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("invalidated input history: %+v %v", page, err)
		}
		item := page.Items[0]
		if item.ID != input.CommandID || item.State != "cancelled" || item.Reason == nil || *item.Reason != "credentials_revoked" || item.Cancellation == nil || item.Cancellation.EventID == nil || item.Cancellation.ActorUserID != nil || item.Cancellation.Actor != nil || item.Cancellation.CreatedAt == nil || item.Cancellation.Kind == nil || *item.Cancellation.Kind != "automatic" {
			t.Fatalf("background account cancellation fabricated a human operator: %+v", item)
		}
	})
	t.Run("promotion_stale_author", func(t *testing.T) {
		f := newManagedCollaborationFixture(t)
		stale := f.submitManaged(t, f.handler, "obsolete input")
		valid := f.submitManaged(t, f.creator, "eligible successor")
		f.resumeManaged(t)
		f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.handler)
		_, _, claim := f.claimManaged(t)
		if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(valid.CommandID) {
			t.Fatal("obsolete-author cancellation changed eligible FIFO selection")
		}
		page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		var item CollaborationCommandHistoryEntry
		for _, entry := range page.Items {
			if entry.ID == stale.CommandID {
				item = entry
			}
		}
		if item.State != "cancelled" || item.Reason == nil || *item.Reason != "stale_authority" || item.Cancellation == nil || item.Cancellation.EventID == nil || item.Cancellation.ActorUserID != nil || item.Cancellation.Kind == nil || *item.Cancellation.Kind != "automatic" {
			t.Fatalf("daemon promotion confused input attribution with a human canceller: %+v", item)
		}
	})
	t.Run("proven_nonlaunch_return", func(t *testing.T) {
		f := newManagedCollaborationFixture(t)
		input := f.submitManaged(t, f.handler, "obsolete claimed but never launched")
		f.resumeManaged(t)
		run, _, claim := f.claimManaged(t)
		history, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, util.MustParseUUID(claim.Identity.GroupID))
		if err != nil {
			t.Fatal(err)
		}
		f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.handler)
		proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: claim.Identity,
			LastAuthorizedAttemptSeq: "0", SealedAttemptSeq: "0", State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
		if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof); err != nil {
			t.Fatal(err)
		}
		page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("nonlaunch cancellation history: %+v %v", page, err)
		}
		item := page.Items[0]
		if item.ID != input.CommandID || item.State != "cancelled" || item.GroupID.Valid || item.Cancellation == nil || item.Cancellation.EventID == nil || item.Cancellation.ActorUserID != nil || item.Cancellation.CreatedAt == nil || item.Cancellation.Kind == nil || *item.Cancellation.Kind != "automatic" {
			t.Fatalf("nonlaunch return lost exact system cancellation evidence: %+v", item)
		}
		after, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, util.MustParseUUID(claim.Identity.GroupID))
		if err != nil || !reflect.DeepEqual(history, after) {
			t.Fatalf("nonlaunch cancellation rewrote immutable consumption history: %v", err)
		}
	})
}

func TestCollaborationCommandHistoryM11CancelClaimRace(t *testing.T) {
	for _, first := range []string{"cancel", "claim"} {
		t.Run(first+"_first", func(t *testing.T) {
			f, _ := newManagedSquadFixture(t)
			f.submitManaged(t, f.creator, "root input")
			f.resumeManaged(t)
			root, request, rootClaim := f.claimManaged(t)
			token, nonce := permitSquadRun(t, f, root, rootClaim)
			input := f.submitManaged(t, f.handler, "unread M11 cancellation candidate")
			next := f.submitManaged(t, f.creator, "other unread M11 input")
			finishSquadRun(t, f, root, rootClaim, token, nonce, "success")
			continuation, err := f.svc.Queries.GetCollaborationQueuedContinuation(f.ctx, util.MustParseUUID(rootClaim.Identity.GroupID))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			daemon, cancelDaemon := context.WithTimeout(f.daemonCtx, 15*time.Second)
			defer cancelDaemon()
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := runtimeSourceCommitStarter{recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}}
			svc := *f.svc
			svc.TxStarter = barrier
			cancelInput := func(s *CollaborationService) error {
				_, err := s.CancelWaitingCommand(ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1})
				return err
			}
			claim := func(s *CollaborationService) error {
				_, err := s.ClaimRuntimeRun(daemon, continuation.ID, request)
				return err
			}
			leading, following := cancelInput, claim
			if first == "claim" {
				leading, following = claim, cancelInput
			}
			firstDone := make(chan error, 1)
			go func() { firstDone <- leading(&svc) }()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case err := <-firstDone:
				t.Fatalf("first M11 writer failed before its locked commit: %v", err)
			case <-ctx.Done():
				t.Fatal("first M11 writer did not reach commit")
			}
			secondDone := make(chan error, 1)
			go func() { secondDone <- following(f.svc) }()
			waitFIFOBlocked(t, f.collaborationQueueFixture, blocker, secondDone)
			unblock()
			if err = <-firstDone; err != nil {
				t.Fatal(err)
			}
			err = <-secondDone
			if first == "claim" && !collaborationErrorCode(err, "command_not_waiting") || first == "cancel" && err != nil {
				t.Fatalf("M11 claim/cancel boundary produced the wrong decision: %v", err)
			}
			page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
			if err != nil {
				t.Fatal(err)
			}
			var item CollaborationCommandHistoryEntry
			for _, entry := range page.Items {
				if entry.ID == input.CommandID {
					item = entry
				}
			}
			if first == "cancel" {
				if item.State != "cancelled" || item.GroupID.Valid || item.Cancellation == nil || item.Cancellation.Kind == nil || *item.Cancellation.Kind != "manual" || item.Cancellation.ActorUserID == nil || *item.Cancellation.ActorUserID != f.creator || f.fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1`, input.CommandID) != 0 {
					t.Fatalf("unread M11 cancellation became consumption or lost provenance: %+v", item)
				}
			} else if item.State != "in_flight" || item.Cancellation != nil || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='command_cancelled'`, f.issue.Issue.ID) != 0 {
				t.Fatalf("late M11 cancellation invented a cancellation event: %+v", item)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight' AND seq=$2`, next.CommandID, next.CommandSequence) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND queue_state='active'`, input.ChannelID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active'`, f.issue.SessionID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NOT NULL`, continuation.ID) != 0 {
				t.Fatal("M11 cancellation changed the successor, either pause layer, or implicitly stopped the claimed continuation")
			}
		})
	}
}

func TestCollaborationCommandHistoryAutomaticM11Detach(t *testing.T) {
	for _, action := range []string{"remove", "complete"} {
		t.Run(action, func(t *testing.T) {
			f, _ := newManagedSquadFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			f.submitManaged(t, f.creator, "current root remains separate from unread input")
			f.resumeManaged(t)
			root, _, claim := f.claimManaged(t)
			permitSquadRun(t, f, root, claim)
			input := f.submitManaged(t, f.handler, "automatically detached unread M11 input")
			var err error
			if action == "remove" {
				_, err = f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f.collaborationQueueFixture, 1), TargetUserID: util.MustParseUUID(f.handler)})
			} else {
				if err = f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
				_, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f.collaborationQueueFixture, "complete", "done", 1, 1))
			}
			if err != nil {
				t.Fatal(err)
			}
			page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
			if err != nil {
				t.Fatal(err)
			}
			var item CollaborationCommandHistoryEntry
			for _, entry := range page.Items {
				if entry.ID == input.CommandID {
					item = entry
				}
			}
			if item.State != "cancelled" || item.GroupID.Valid || item.Cancellation == nil || item.Cancellation.EventID == nil || item.Cancellation.ActorUserID == nil || *item.Cancellation.ActorUserID != f.creator || item.Cancellation.Kind == nil || *item.Cancellation.Kind != "automatic" || f.fx.Count(t, `SELECT count(*) FROM collaboration_group_dependency WHERE group_id=$1 AND command_id=$2 AND detached_at IS NOT NULL`, claim.Identity.GroupID, input.CommandID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1`, input.CommandID) != 0 {
				t.Fatalf("automatic M11 detach lost original input/event identity or invented consumption: %+v", item)
			}
			if action == "remove" && (f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL`, root) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND queue_state='active'`, input.ChannelID) != 1) {
				t.Fatal("unread-author departure stopped the unrelated running root or channel")
			}
		})
	}
}

func TestCollaborationCommandHistorySwitchThenCancelPreservesHistoricalPlacement(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	switchCleanup(t, f)
	squad, _ := seedQueueSquad(t, f, false)
	old := submitTarget(t, f, squad.ID, f.creator, "old consumed input", pgtype.UUID{})
	run := f.run(t, old, 0)
	if _, err := f.svc.StopQueue(f.ctx, f.stopParams(run, 1)); err != nil {
		t.Fatal(err)
	}
	history, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
	if err != nil {
		t.Fatal(err)
	}
	priorCancelled := submitTarget(t, f, squad.ID, f.handler, "cancel before Switch", pgtype.UUID{})
	if _, err = f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID, CommandID: priorCancelled.CommandID, ExpectedCollaborationRevision: 1}); err != nil {
		t.Fatal(err)
	}
	waiting := submitTarget(t, f, squad.ID, f.handler, "cancel after Switch", pgtype.UUID{})
	destination, grant := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	head := submitTarget(t, f, destination.ID, f.creator, "destination head", pgtype.UUID{})
	roster := switchRoster(t, destination)
	roster[0].Role = "leader"
	f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, destination.ResourceID)
	before := queueTarget(t, f, squad.ID)
	proposal, _ := proposeSwitch(t, f, before, roster)
	consentSwitch(t, f, squad.ID, proposal.ProposalID, []db.CollaborationGrant{grant})
	if _, err = f.svc.CommitTargetSwitch(f.ctx, CollaborationCommitSwitchParams{CollaborationQueueRequest: f.request(), ProposalID: proposal.ProposalID, Control: targetControl(before)}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID, CommandID: waiting.CommandID, ExpectedCollaborationRevision: 1}); err != nil {
		t.Fatal(err)
	}
	page, err := f.svc.CommandHistory(f.ctx, f.issue.Issue.ID, CollaborationCommandHistoryQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	items := make(map[pgtype.UUID]CollaborationCommandHistoryEntry, len(page.Items))
	for _, item := range page.Items {
		items[item.ID] = item
	}
	previous, current := items[priorCancelled.CommandID], items[waiting.CommandID]
	if previous.ChannelID != priorCancelled.ChannelID || previous.Seq != priorCancelled.CommandSequence || previous.Cancellation == nil || current.ChannelID != head.ChannelID || current.Seq != head.CommandSequence+2 || current.State != "cancelled" || current.Cancellation == nil || current.Cancellation.EventID == nil || current.Target.ResourceID != squad.ResourceID || current.Target.Kind != "squad" {
		t.Fatalf("Switch cancellation conflated current and old input positions: previous=%+v current=%+v", previous, current)
	}
	after, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
	if err != nil || !reflect.DeepEqual(history, after) || items[old.CommandID].ChannelID != head.ChannelID || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND channel_id=$2 AND batch_id=$3`, run.ID, run.ChannelID, run.BatchID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND seq=$2`, head.CommandID, head.CommandSequence) != 1 {
		t.Fatal("cancel after Switch changed historical consumption, run identity or the destination's earlier FIFO input")
	}
}
