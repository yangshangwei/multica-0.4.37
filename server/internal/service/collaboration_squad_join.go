package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A joined input remains waiting until the next leader continuation claims it.
// The lane lock makes FIFO eligibility, dependency capture and join atomic.
func (t *collaborationQueueTx) tryJoinSquad(ctx context.Context, command db.CollaborationCommand, target db.CollaborationTarget, channel db.CollaborationChannel) (bool, error) {
	if !t.gate.Enabled || t.session.QueueState != "active" || channel.QueueState != "active" || target.HoldReason.Valid || !channel.ActiveGroupID.Valid {
		return false, nil
	}
	group, err := t.q.LockCollaborationRuntimeGroup(ctx, channel.ActiveGroupID)
	if err != nil {
		return false, err
	}
	groupTarget, ok := t.target(group.TargetID)
	if !ok {
		return false, invalidCollaborationLedger("join group target missing")
	}
	if groupTarget.Kind != "squad" || group.ExpansionSealedAt.Valid || group.SettledAt.Valid || group.ForcedDisposition.Valid {
		return false, nil
	}
	if target.ID != groupTarget.ID && (target.Kind != "agent" || target.ResourceID != channel.RootAgentID) {
		return false, nil
	}
	runs, err := t.q.LockCollaborationGroupRuns(ctx, group.ID)
	if err != nil {
		return false, err
	}
	if err = t.validateGroup(ctx, groupTarget, group, runs); err != nil {
		return false, err
	}
	var parent db.CollaborationRun
	for _, run := range runs {
		if !run.SettledAt.Valid && !run.ForcedDisposition.Valid {
			parent = run
		}
	}
	if !parent.ID.Valid {
		return false, nil
	}
	waiting, err := t.q.ListCollaborationChannelWaitingInputs(ctx, channel.ID)
	if err != nil {
		return false, err
	}
	for _, earlier := range waiting {
		if earlier.Seq >= command.Seq {
			break
		}
		if earlier.GroupID.Valid {
			continue
		}
		other, ok := t.target(earlier.TargetID)
		if !ok {
			return false, invalidCollaborationLedger("join earlier target missing")
		}
		if other.HoldReason.Valid {
			continue
		}
		if code, err := collaborationCurrentBatchAuthors(ctx, t.q, t.access.Issue.ID, t.access.LifecycleEpoch, []db.CollaborationCommand{earlier}, "waiting"); err != nil {
			return false, err
		} else if code != "" {
			continue
		}
		if code, err := t.authorizeTargetInput(ctx, other); err != nil {
			return false, err
		} else if code != "" {
			continue
		}
		return false, nil
	}
	grant, err := t.q.GetCollaborationGrant(ctx, parent.GrantID)
	if err != nil {
		return false, err
	}
	batch, err := t.q.GetCollaborationBatch(ctx, group.BatchID)
	if err != nil {
		return false, err
	}
	r := &collaborationRuntimeState{collaborationQueueTx: t, run: parent, group: group, target: groupTarget, channel: channel, batch: batch, grant: grant}
	r.inputs, err = t.q.ListCollaborationBatchInputs(ctx, batch.ID)
	if err != nil {
		return false, err
	}
	if err = r.currentGroupAdmission(ctx); err != nil {
		var denied *CollaborationError
		if errors.As(err, &denied) {
			return false, nil
		}
		return false, err
	}
	prompt, err := collaborationRunInputs(append(r.inputs, command))
	if err != nil {
		return false, err
	}
	encoded, err := json.Marshal(prompt)
	if err != nil {
		return false, err
	}
	if len(prompt) > maxCollaborationBatchInputs || len(encoded) > maxCollaborationBatchBytes {
		return false, nil
	}
	if err = t.freezeGroupDependencies(ctx, group.ID, []db.CollaborationCommand{command}); err != nil {
		return false, err
	}
	if _, err = r.queueContinuation(ctx, parent.ID); err != nil {
		return false, err
	}
	if _, err = t.q.JoinCollaborationUnreadInput(ctx, db.JoinCollaborationUnreadInputParams{GroupID: group.ID, BatchID: group.BatchID, CommandID: command.ID}); err != nil {
		return false, err
	}
	return true, t.q.CloseEmptyCollaborationBatches(ctx, t.session.ID)
}

func (t *collaborationQueueTx) detachUnreadInput(ctx context.Context, group db.CollaborationExecutionGroup, command db.CollaborationCommand, state, reason string) error {
	if command.GroupID != group.ID || command.State != "waiting" {
		return invalidCollaborationLedger("detach requires an unread joined input")
	}
	if err := t.q.DetachCollaborationGroupDependency(ctx, db.DetachCollaborationGroupDependencyParams{GroupID: group.ID, CommandID: command.ID, Reason: reason}); err != nil {
		return err
	}
	batchID := command.BatchID
	if state == "waiting" {
		batch, err := t.newWaitingBatch(ctx, group.ChannelID, command.TargetID)
		if err != nil {
			return err
		}
		batchID = batch.ID
	}
	_, err := t.q.DetachCollaborationUnreadInput(ctx, db.DetachCollaborationUnreadInputParams{GroupID: group.ID, CommandID: command.ID, BatchID: batchID, State: state, Reason: reason})
	return err
}

func (t *collaborationQueueTx) detachGroupUnreadInputs(ctx context.Context, group db.CollaborationExecutionGroup, reason string) error {
	commands, err := t.q.ListCollaborationGroupCommands(ctx, group.ID)
	if err != nil {
		return err
	}
	for _, command := range commands {
		if command.State != "waiting" {
			continue
		}
		state := "waiting"
		if t.session.QueueState == "closed" {
			state = "cancelled"
		} else if code, err := collaborationCurrentBatchAuthors(ctx, t.q, t.access.Issue.ID, t.access.LifecycleEpoch, []db.CollaborationCommand{command}, "waiting"); err != nil {
			return err
		} else if code != "" {
			state = "cancelled"
		}
		if err = t.detachUnreadInput(ctx, group, command, state, reason); err != nil {
			return err
		}
	}
	return nil
}

// A claimed continuation consumes every still-attached input in the same
// transaction as its immutable prompt. Replay cannot consume later joins.
func (r *collaborationRuntimeState) consumeJoinedInputs(ctx context.Context) error {
	if r.run.StepKind.String != "continuation" || r.run.PromptSnapshot != nil {
		return nil
	}
	if err := r.q.ConsumeCollaborationJoinedInputs(ctx, db.ConsumeCollaborationJoinedInputsParams{GroupID: r.group.ID, RunID: r.run.ID}); err != nil {
		return err
	}
	var err error
	r.inputs, err = r.q.ListCollaborationBatchInputs(ctx, r.batch.ID)
	return err
}

// Only commands that remain associated with this group contribute authority.
// Historical detached dependencies remain immutable tombstones.
func groupInputState(input db.CollaborationCommand, group pgtype.UUID) bool {
	return input.GroupID == group && (input.State == "waiting" || input.State == "in_flight")
}
