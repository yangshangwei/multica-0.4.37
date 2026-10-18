package service

import (
	"context"
	"net/http"
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
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func managedQueueSquad(t *testing.T, f *managedCollaborationFixture) db.CollaborationTarget {
	t.Helper()
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	squad := f.fx.Squad(t, "Managed FIFO squad "+uuid.NewString(), util.UUIDToString(f.grant.AgentID))
	registered, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{
		CollaborationQueueRequest: f.request(), Kind: collaboration.TargetSquad, ResourceID: util.MustParseUUID(squad), ExpectedTargetSetVersion: session.TargetSetVersion,
		Roster: []collaboration.TargetResource{{Role: "leader", AgentID: util.UUIDToString(f.grant.AgentID), RuntimeID: util.UUIDToString(f.runtime.ID), ExecutionWorkspaceID: f.workspace, OwnerUserID: f.creator}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ConsentTargetResource(f.ctx, CollaborationConsentParams{CollaborationQueueRequest: f.request(), TargetID: registered.TargetID, AgentID: f.grant.AgentID,
		ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: collaborationGrantIdentity(f.grant)})
	if err != nil {
		t.Fatal(err)
	}
	return queueTarget(t, f.collaborationQueueFixture, registered.TargetID)
}

// This uses real source/target/author/grant transactions to exercise the FIFO
// selection independently of the deliberately unavailable squad runtime.
func selectFixtureFIFO(t *testing.T, f *collaborationQueueFixture, channelID pgtype.UUID) collaborationInputSelection {
	t.Helper()
	var selected collaborationInputSelection
	request := f.request()
	_, err := f.svc.mutateCollaborationQueue(f.ctx, request, "test_fifo_selection", request, func(queue *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		for _, target := range queue.targets {
			if _, err := queue.q.LockCollaborationTargetGrants(f.ctx, target.ID); err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		if _, err := queue.q.LockCollaborationSessionCommands(f.ctx, queue.session.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if _, err := queue.q.LockCollaborationBatches(f.ctx, queue.session.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		channel, exists := queue.channel(channelID)
		if !exists {
			t.Fatal("fixture channel missing")
		}
		var err error
		selected, err = queue.selectWaitingInputs(f.ctx, channel)
		return queue.receipt("fifo_selected", http.StatusOK), err
	})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

func TestCollaborationFIFOStopsAtSecondEligibleSquad(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		targets []int
		held    int
		invalid int
		want    []int
		role    int
	}{
		{"second_squad_is_boundary", []int{1, 2, 0, 1}, 0, 0, []int{0}, 1},
		{"held_squad_does_not_truncate", []int{1, 2, 0, 1, 3, 0}, 2, 0, []int{0, 2, 3}, 1},
		{"invalid_squad_does_not_truncate", []int{1, 0, 2}, 0, 1, []int{1, 2}, 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			targets := []pgtype.UUID{f.targetID}
			for range 3 {
				target, _ := seedQueueSquad(t, f, false)
				targets = append(targets, target.ID)
			}
			inputs := make([]CollaborationQueueReceipt, 0, len(scenario.targets))
			for _, i := range scenario.targets {
				inputs = append(inputs, submitTarget(t, f, targets[i], f.creator, "FIFO contribution", pgtype.UUID{}))
			}
			f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason=NULL,hold_version=hold_version+1 WHERE session_id=$1`, f.issue.SessionID)
			if scenario.held != 0 {
				f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='switch_hold',hold_version=hold_version+1 WHERE id=$1`, targets[scenario.held])
			}
			if scenario.invalid != 0 {
				f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),epoch=epoch+1 WHERE target_id=$1`, targets[scenario.invalid])
			}
			selected := selectFixtureFIFO(t, f, inputs[0].ChannelID)
			if selected.target.ID != targets[scenario.role] || len(selected.inputs) != len(scenario.want) {
				t.Fatalf("wrong effective squad or selected count: %+v", selected)
			}
			for i, want := range scenario.want {
				if selected.inputs[i].ID != inputs[want].CommandID {
					t.Errorf("input %d crossed an eligible squad boundary or lost FIFO order", i)
				}
			}
			if scenario.invalid != 0 && queueTarget(t, f, targets[scenario.invalid]).HoldReason.String != "grant_required" {
				t.Fatal("selection did not durably hold the invalid target")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1`, f.issue.SessionID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, f.issue.SessionID) != len(inputs) {
				t.Fatal("selection alone launched a squad or consumed input")
			}
		})
	}
}

func TestCollaborationFIFOAllocatesOneSequenceAcrossTypedTargets(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, _ := seedQueueSquad(t, f, false)
	other, _ := seedQueueSquad(t, f, false)
	targets := []pgtype.UUID{squad.ID, other.ID, f.targetID, squad.ID}
	for i, target := range targets {
		r := submitTarget(t, f, target, f.creator, "ordered immutable input", pgtype.UUID{})
		if r.CommandSequence != int64(i+1) {
			t.Fatalf("channel input %d has sequence %d; per-target counters are not a channel FIFO", i+1, r.CommandSequence)
		}
	}
}

func TestCollaborationFIFOConcurrentPromotionConsumesEachInputOnce(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "one immutable input")
	f.resumeManaged(t)
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			done <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_group_input WHERE command_id=$1`, input.CommandID) != 1 {
		t.Fatal("competing promoters duplicated a group, run or consumed input")
	}
}

func waitFIFOBlocked(t *testing.T, f *collaborationQueueFixture, blocker int, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		f.fx.QueryRow(t, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1::int=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&blocked)
		if blocked {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("competing writer bypassed the source lock: %v", err)
		default:
		}
	}
	t.Fatal("competing writer did not reach the source lock")
}

func TestCollaborationFIFOCancellationAndPromotionSerialize(t *testing.T) {
	for _, first := range []string{"cancel", "promote"} {
		t.Run(first+"_first", func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			input := f.submitManaged(t, f.creator, "cancel or consume exactly once")
			f.resumeManaged(t)
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
			svc := *f.svc
			svc.TxStarter = barrier
			actorCtx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			daemonCtx, cancelDaemon := context.WithTimeout(f.daemonCtx, 15*time.Second)
			defer cancelDaemon()
			cancelInput := func(service *CollaborationService) error {
				_, err := service.CancelWaitingCommand(actorCtx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1})
				return err
			}
			promote := func(service *CollaborationService) error {
				_, err := service.ListRuntimeRuns(daemonCtx, f.runtime.ID)
				return err
			}
			leading, following := cancelInput, promote
			if first == "promote" {
				leading, following = promote, cancelInput
			}
			firstDone := make(chan error, 1)
			go func() { firstDone <- leading(&svc) }()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case err := <-firstDone:
				t.Fatalf("first writer failed before its commit barrier: %v", err)
			case <-actorCtx.Done():
				t.Fatal("first writer did not reach commit")
			}
			secondDone := make(chan error, 1)
			go func() { secondDone <- following(f.svc) }()
			waitFIFOBlocked(t, f.collaborationQueueFixture, blocker, secondDone)
			unblock()
			if err := <-firstDone; err != nil {
				t.Fatal(err)
			}
			err := <-secondDone
			if first == "promote" {
				if !collaborationErrorCode(err, "command_not_waiting") || f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, input.CommandID) != 1 {
					t.Fatalf("late cancellation did not reject the consumed input: %v", err)
				}
			} else if err != nil || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatalf("promotion consumed a cancelled input: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE id=$1 AND queue_state='active'`, input.ChannelID) != 1 {
				t.Fatal("R45 cancellation implicitly stopped the channel")
			}
		})
	}
}

func TestCollaborationFIFOSelectsDirectInputFromHeldMixedBatch(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	squad := managedQueueSquad(t, f)
	held, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "held squad input"})
	if err != nil {
		t.Fatal(err)
	}
	direct := f.submitManaged(t, f.handler, "eligible direct input")
	if held.BatchID != direct.BatchID {
		t.Fatal("fixture must exercise a pre-existing mixed waiting batch")
	}
	f.resumeManaged(t)
	// The direct target is runnable; the unsupported squad retains its own hold.
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='switch_hold',hold_version=hold_version+1 WHERE id=$1`, squad.ID)
	_, _, claim := f.claimManaged(t)
	if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(direct.CommandID) || claim.Inputs[0].AuthorUserID != f.handler {
		t.Fatalf("promotion must consume only the eligible direct input: %+v", claim.Inputs)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND target_id=$2 AND content='held squad input'`, held.CommandID, squad.ID) != 1 {
		t.Fatal("skipping the held input changed or consumed it")
	}
}

func TestCollaborationFIFOFullHeldStagingBatchDoesNotRejectDirectInput(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	squad := managedQueueSquad(t, f)
	held, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: strings.Repeat("s", maxCollaborationBatchBytes-1024)})
	if err != nil {
		t.Fatal(err)
	}
	direct := f.submitManaged(t, f.creator, strings.Repeat("a", 2048))
	if direct.BatchID == held.BatchID {
		t.Fatal("full staging batch should split before accepting more input")
	}
	// The internal squad runner is now supported. Preserve this scenario's
	// explicit hold instead of relying on the old unsupported-runtime refusal.
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='grant_required',hold_version=hold_version+1 WHERE id=$1`, squad.ID)
	resume := managedTargetResume(t, f, f.targetID)
	f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='active' WHERE id=$1`, f.issue.SessionID)
	if _, err = f.svc.ResumeQueue(f.ctx, resume); err != nil {
		t.Fatal(err)
	}
	_, _, claim := f.claimManaged(t)
	if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(direct.CommandID) {
		t.Fatal("held staging batch blocked the later direct input")
	}
	_, err = f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: strings.Repeat("x", maxCollaborationBatchBytes)})
	if !collaborationErrorCode(err, "batch_capacity_exceeded") {
		t.Fatalf("splitting staging must still reject an individually oversized input: %v", err)
	}
}

func TestCollaborationResourceNonlaunchRetainsValidAuthorInput(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.handler, "other author's unread resource input")
	f.resumeManaged(t)
	runID, _, _ := f.claimManaged(t)
	if err := deleteCollaborationRuntime(context.Background(), f, true); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NOT NULL AND proof_state='never_launched_proven'`, runID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL AND author_user_id=$2`, input.CommandID, f.handler) != 1 {
		t.Fatal("resource-only nonlaunch invalidation cancelled another valid author's unread input")
	}
}

func TestCollaborationFIFORemovesStaleWaitingAuthorWithoutBlockingNext(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	stale := f.submitManaged(t, f.handler, "obsolete author input")
	valid := f.submitManaged(t, f.creator, "current author input")
	f.resumeManaged(t)
	f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.handler)
	_, _, claim := f.claimManaged(t)
	if len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(valid.CommandID) {
		t.Fatalf("an obsolete waiting author blocked the valid FIFO input: %+v", claim.Inputs)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND author_user_id=$2 AND content='obsolete author input'`, stale.CommandID, f.handler) != 1 {
		t.Fatal("obsolete unstarted input was not cancelled with its identity intact")
	}
}

func TestCollaborationNeverLaunchedReturnsInputsToWaiting(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	first := f.submitManaged(t, f.creator, "never started first input")
	second := f.submitManaged(t, f.handler, "never started second input")
	f.resumeManaged(t)
	runID, _, _ := f.claimManaged(t)
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.StopQueue(f.ctx, f.stopParams(run, queueTarget(t, f.collaborationQueueFixture, f.targetID).ControlVersion))
	if err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND proof_state='never_launched_proven' AND settled_at IS NOT NULL`, runID) != 1 {
		t.Fatal("server did not retain its safe nonlaunch proof")
	}
	for _, input := range []CollaborationQueueReceipt{first, second} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND seq=$2`, input.CommandID, input.CommandSequence) != 1 {
			t.Fatal("a proven unread input became terminal instead of returning to its original queue position")
		}
	}
	oldMembership, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
	if err != nil || len(oldMembership) != 2 {
		t.Fatalf("nonlaunch lost consumed membership: %+v %v", oldMembership, err)
	}
	f.resumeManaged(t)
	nextID, _, claim := f.claimManaged(t)
	if nextID == runID || len(claim.Inputs) != 2 || claim.Inputs[0].CommandID != util.UUIDToString(first.CommandID) || claim.Inputs[1].CommandID != util.UUIDToString(second.CommandID) {
		t.Fatal("explicit Resume did not create a new run with the same unread inputs")
	}
	retained, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
	if err != nil || !reflect.DeepEqual(retained, oldMembership) {
		t.Fatalf("repromotion rewrote the first group's historical inputs: %v", err)
	}
}

func TestCollaborationNeverLaunchedReturnExcludesObsoleteInputs(t *testing.T) {
	for _, scenario := range []string{"removed_author", "closed_session", "exited_failure"} {
		t.Run(scenario, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			first := f.submitManaged(t, f.handler, "first author")
			second := f.submitManaged(t, f.creator, "second author")
			f.resumeManaged(t)
			runID, _, claim := f.claimManaged(t)
			proof := protocol.CollaborationRunProof{ReceiptID: uuid.NewString(), Identity: claim.Identity, LastAuthorizedAttemptSeq: "0", SealedAttemptSeq: "0", State: "never_launched_proven", Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
			switch scenario {
			case "removed_author":
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
			case "closed_session":
				f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
			case "exited_failure":
				permit := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
				capability, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, permit)
				if err != nil {
					t.Fatal(err)
				}
				if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, capability.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce, Outcome: "failure"}); err != nil {
					t.Fatal(err)
				}
				proof = managedProof(claim.Identity, permit.AttemptNonce)
			}
			if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
				t.Fatal(err)
			}
			states := []string{"cancelled", "waiting"}
			if scenario == "closed_session" {
				states[1] = "cancelled"
			} else if scenario == "exited_failure" {
				states = []string{"failed", "failed"}
			}
			for i, input := range []CollaborationQueueReceipt{first, second} {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state=$2`, input.CommandID, states[i]) != 1 {
					t.Fatalf("input %d should be %s after %s", i, states[i], scenario)
				}
			}
		})
	}
}

func TestCollaborationNonlaunchContradictoryProgressKeepsBarrier(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "unconfirmed execution keeps its barrier")
	f.resumeManaged(t)
	runID, _, _ := f.claimManaged(t)
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	// Retained contradictory evidence is corruption, never a release signal.
	f.fx.Exec(t, `UPDATE collaboration_run SET settled_at=now(),sealed_attempt_seq=0,proof_state='never_launched_proven',progress_state='running' WHERE id=$1`, runID)
	_, err = f.svc.StopQueue(f.ctx, f.stopParams(run, queueTarget(t, f.collaborationQueueFixture, f.targetID).ControlVersion))
	if err == nil {
		t.Fatal("Stop accepted a settled nonlaunch proof that still claims running")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight'`, input.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE id=$1 AND settled_at IS NULL`, run.GroupID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
		t.Fatal("contradictory proof released a command, group or physical reservation")
	}
}

func TestCollaborationSwitchMovesWaitingInputsToDestinationTail(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, _ := seedQueueSquad(t, f, false)
	first := submitTarget(t, f, squad.ID, f.creator, "first transferred input", pgtype.UUID{})
	oldDirect := f.submit(t, f.handler, "keep on old channel")
	second := submitTarget(t, f, squad.ID, f.handler, "second transferred input", pgtype.UUID{})
	destination, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	head := submitTarget(t, f, destination.ID, f.creator, "destination head", pgtype.UUID{})
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](destination.Roster)
	if err != nil {
		t.Fatal(err)
	}
	roster[0].Role = "leader"
	f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, destination.ResourceID)
	before, err := f.svc.Queries.GetCollaborationChannel(f.ctx, first.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	request := CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(queueTarget(t, f, squad.ID)), Roster: roster}
	receipt, err := f.svc.ReconfigureTarget(f.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	updated := queueTarget(t, f, squad.ID)
	if updated.ChannelID != head.ChannelID || updated.HoldReason.String != "switch_hold" {
		t.Fatal("reconfigured squad did not move to the new leader channel under switch_hold")
	}
	for i, input := range []CollaborationQueueReceipt{first, second} {
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND channel_id=$2 AND seq=$3 AND state='waiting' AND target_id=$4`,
			input.CommandID, head.ChannelID, head.CommandSequence+int64(i)+1, squad.ID) != 1 {
			t.Fatal("Switch did not append each immutable waiting input in relative order")
		}
	}
	after, err := f.svc.Queries.GetCollaborationChannel(f.ctx, first.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if after.QueueState != before.QueueState || after.ControlVersion != before.ControlVersion || after.GroupGeneration != before.GroupGeneration || after.ActiveGroupID != before.ActiveGroupID ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND channel_id=$2 AND seq=$3`, oldDirect.CommandID, before.ID, oldDirect.CommandSequence) != 1 {
		t.Fatal("Switch changed the old lane's controls or another target's input")
	}
	replay, err := f.svc.ReconfigureTarget(f.ctx, request)
	if err != nil || !reflect.DeepEqual(receipt, replay) {
		t.Fatalf("Switch did not replay its original receipt: %+v %v", replay, err)
	}
	if current := queueTarget(t, f, squad.ID); current.TargetEpoch != updated.TargetEpoch || current.HoldVersion != updated.HoldVersion {
		t.Fatal("original Switch key repeated configuration or hold changes")
	}
}

func TestCollaborationSwitchLeavesUnrelatedOldChannelRunUntouched(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	squad := managedQueueSquad(t, f)
	f.submitManaged(t, f.creator, "direct agent is already working")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	f.fx.Exec(t, `UPDATE collaboration_target SET hold_reason='switch_hold',hold_version=hold_version+1 WHERE id=$1`, squad.ID)
	input, err := f.svc.SubmitCommand(f.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID,
		InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "squad waits for Switch"})
	if err != nil {
		t.Fatal(err)
	}
	destination, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](destination.Roster)
	if err != nil {
		t.Fatal(err)
	}
	roster[0].Role = "leader"
	f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, destination.ResourceID)
	before, err := f.svc.Queries.GetCollaborationChannel(f.ctx, input.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(queueTarget(t, f.collaborationQueueFixture, squad.ID)), Roster: roster})
	if err != nil {
		t.Fatalf("an unrelated direct run blocked switching an unread squad target: %v", err)
	}
	updated := queueTarget(t, f.collaborationQueueFixture, squad.ID)
	if updated.ChannelID == before.ID || updated.HoldReason.String != "switch_hold" {
		t.Fatal("Switch did not create the destination channel under hold")
	}
	after, err := f.svc.Queries.GetCollaborationChannel(f.ctx, before.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("Switch changed the unrelated old channel: %+v %v", after, err)
	}
	if _, err = f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}); err != nil {
		t.Fatalf("Switch changed the old direct run's grant or proof identity: %v", err)
	}
}

func TestCollaborationSwitchAndCancellationSerialize(t *testing.T) {
	for _, first := range []string{"switch", "cancel"} {
		t.Run(first+"_first", func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			squad, _ := seedQueueSquad(t, f, false)
			input := submitTarget(t, f, squad.ID, f.creator, "cancel during held Switch", pgtype.UUID{})
			destination, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
			roster, err := decodeCollaborationRoster[collaboration.TargetResource](destination.Roster)
			if err != nil {
				t.Fatal(err)
			}
			roster[0].Role = "leader"
			f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, destination.ResourceID)
			request := CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(queueTarget(t, f, squad.ID)), Roster: roster}
			ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			move := func(service *CollaborationService) error {
				_, err := service.ReconfigureTarget(ctx, request)
				return err
			}
			cancelInput := func(service *CollaborationService) error {
				_, err := service.CancelWaitingCommand(ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: squad.ID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1})
				return err
			}
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
			svc := *f.svc
			svc.TxStarter = barrier
			leading, following := move, cancelInput
			if first == "cancel" {
				leading, following = cancelInput, move
			}
			firstDone := make(chan error, 1)
			go func() { firstDone <- leading(&svc) }()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case err := <-firstDone:
				t.Fatalf("first writer failed before commit: %v", err)
			case <-ctx.Done():
				t.Fatal("first writer did not reach commit")
			}
			secondDone := make(chan error, 1)
			go func() { secondDone <- following(f.svc) }()
			waitFIFOBlocked(t, f, blocker, secondDone)
			unblock()
			for _, done := range []<-chan error{firstDone, secondDone} {
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			wantChannel := input.ChannelID
			if first == "switch" {
				wantChannel = queueTarget(t, f, squad.ID).ChannelID
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='cancelled' AND channel_id=$2 AND content='cancel during held Switch'`, input.CommandID, wantChannel) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
				t.Fatal("Switch/cancel race moved terminal history or consumed the cancelled input")
			}
		})
	}
}

func TestCollaborationSwitchRequiresOldTargetProof(t *testing.T) {
	for _, proven := range []bool{false, true} {
		name := "unproven"
		if proven {
			name = "proven_nonlaunch"
		}
		t.Run(name, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			squad, _ := seedQueueSquad(t, f, false)
			input := submitTarget(t, f, squad.ID, f.creator, "old squad input retains its history", pgtype.UUID{})
			attempts := int64(1)
			if proven {
				attempts = 0
			}
			// Synthetic root evidence exercises the Switch service barrier, not
			// a declaration that the squad runtime can launch.
			run := f.run(t, input, attempts)
			if _, err := f.svc.StopQueue(f.ctx, f.stopParams(run, 1)); err != nil {
				t.Fatal(err)
			}
			history, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
			if err != nil {
				t.Fatal(err)
			}
			destination, _ := seedCollaborationTarget(t, f.fx, f.issue.Issue.ID, f.issue.SessionID, util.MustParseUUID(f.creator), 1)
			roster, err := decodeCollaborationRoster[collaboration.TargetResource](destination.Roster)
			if err != nil {
				t.Fatal(err)
			}
			roster[0].Role = "leader"
			f.fx.Exec(t, `UPDATE squad SET leader_id=$2 WHERE id=$1`, squad.ResourceID, destination.ResourceID)
			before := queueTarget(t, f, squad.ID)
			channels := f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE session_id=$1`, f.issue.SessionID)
			_, err = f.svc.ReconfigureTarget(f.ctx, CollaborationReconfigureTargetParams{CollaborationQueueRequest: f.request(), Control: targetControl(before), Roster: roster})
			if !proven {
				if !collaborationErrorCode(err, "stop_unconfirmed") || !reflect.DeepEqual(before, queueTarget(t, f, squad.ID)) ||
					channels != f.fx.Count(t, `SELECT count(*) FROM collaboration_channel WHERE session_id=$1`, f.issue.SessionID) {
					t.Fatalf("Switch bypassed an old proof barrier or wrote a destination on refusal: %v", err)
				}
			} else if err != nil || queueTarget(t, f, squad.ID).ChannelID == before.ChannelID {
				t.Fatalf("proven unread input did not move under Switch hold: %v", err)
			}
			retained, err := f.svc.Queries.ListCollaborationGroupInputs(f.ctx, run.GroupID)
			if err != nil || !reflect.DeepEqual(history, retained) {
				t.Fatalf("Switch changed the old consumed-input ledger: %v", err)
			}
			current, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, run.ID)
			if err != nil || current.ChannelID != run.ChannelID || current.BatchID != run.BatchID || current.Fence != run.Fence || current.LaunchNonce != run.LaunchNonce {
				t.Fatalf("Switch moved a past run or its proof identity: %v", err)
			}
		})
	}
}
