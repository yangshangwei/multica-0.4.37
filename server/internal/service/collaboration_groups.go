package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Control and proof writers share these retained-ledger checks. Current grant
// authority is deliberately not required to finish a historical execution.
func (t *collaborationQueueTx) validateGroup(ctx context.Context, target db.CollaborationTarget, group db.CollaborationExecutionGroup, runs []db.CollaborationRun) error {
	channel, exists := t.channel(group.ChannelID)
	if group.SessionID != t.session.ID || group.TargetID != target.ID || !exists ||
		group.ChannelID != target.ChannelID || channel.SessionID != t.session.ID ||
		group.Generation != channel.GroupGeneration || channel.ActiveGroupID != group.ID {
		return invalidCollaborationLedger("group scope mismatch")
	}
	batch, err := t.q.GetCollaborationBatch(ctx, group.BatchID)
	if err != nil {
		return err
	}
	if batch.SessionID != group.SessionID || batch.ChannelID != group.ChannelID || batch.EffectiveTargetID != group.TargetID ||
		batch.GroupID != group.ID || batch.RootCommandID != group.CommandID || batch.State != "in_flight" {
		return invalidCollaborationLedger("group batch mismatch")
	}
	command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: group.CommandID})
	if err != nil {
		return err
	}
	if command.TargetID != target.ID || command.State != "in_flight" || command.ChannelID != group.ChannelID || command.BatchID != group.BatchID {
		return invalidCollaborationLedger("group command mismatch")
	}
	inputs, err := t.q.ListCollaborationBatchInputs(ctx, group.BatchID)
	if err != nil {
		return err
	}
	for _, input := range inputs {
		if input.SessionID != group.SessionID || input.ChannelID != group.ChannelID ||
			(input.State != "in_flight" && input.State != "cancelled") {
			return invalidCollaborationLedger("batch input scope mismatch")
		}
	}
	roster, err := decodeCollaborationRoster[collaboration.GroupResource](group.Roster)
	if err != nil {
		return err
	}
	if len(roster) == 0 {
		return invalidCollaborationLedger("missing frozen roster")
	}
	byID := make(map[pgtype.UUID]db.CollaborationRun, len(runs))
	for _, run := range runs {
		byID[run.ID] = run
	}
	for _, run := range runs {
		if run.GroupID != group.ID || run.TargetID != target.ID || run.CommandID != group.CommandID || run.SessionID != t.session.ID ||
			run.ChannelID != group.ChannelID || run.BatchID != group.BatchID ||
			!run.TargetEpoch.Valid || run.TargetEpoch.Int64 != group.TargetEpoch || !run.ConfigVersion.Valid || run.ConfigVersion.Int64 != group.ConfigVersion {
			return invalidCollaborationLedger("run scope mismatch")
		}
		matched := false
		role := run.StepKind.String
		if role == "continuation" {
			role = "leader"
		}
		for _, resource := range roster {
			if resource.GrantID == util.UUIDToString(run.GrantID) && resource.GrantEpoch == run.GrantEpoch && resource.Role == role &&
				resource.AgentID == util.UUIDToString(run.AgentID) && resource.RuntimeID == util.UUIDToString(run.RuntimeID) &&
				resource.ExecutionWorkspaceID == util.UUIDToString(run.ExecutionWorkspaceID) {
				matched = true
				break
			}
		}
		if !matched {
			return invalidCollaborationLedger("run resource is outside frozen roster")
		}
		seen := make(map[pgtype.UUID]bool)
		for current := run; current.ParentRunID.Valid; {
			if seen[current.ID] {
				return invalidCollaborationLedger("cyclic run ancestry")
			}
			seen[current.ID] = true
			parent, exists := byID[current.ParentRunID]
			if !exists {
				return invalidCollaborationLedger("run parent missing from group")
			}
			current = parent
		}
	}
	delegations, err := t.q.LockCollaborationGroupDelegations(ctx, group.ID)
	if err != nil {
		return err
	}
	for _, delegation := range delegations {
		if _, exists := byID[delegation.ParentRunID]; !exists {
			return invalidCollaborationLedger("delegation parent missing from group")
		}
	}
	return nil
}

func (t *collaborationQueueTx) stopGroup(ctx context.Context, target db.CollaborationTarget, group db.CollaborationExecutionGroup, disposition, reason string) (CollaborationTargetControlResult, error) {
	runs, err := t.q.LockCollaborationGroupRuns(ctx, group.ID)
	if err != nil {
		return CollaborationTargetControlResult{}, err
	}
	if err = t.validateGroup(ctx, target, group, runs); err != nil {
		return CollaborationTargetControlResult{}, err
	}
	// Removing an author cancels an entirely never-permitted group. Any live
	// descendant turns the root disposition into stopped, preserving first cause.
	if disposition == "cancelled" {
		for _, run := range runs {
			if !run.SettledAt.Valid && !collaborationRunNeverLaunched(run) || run.AuthorizedAttemptSeq > 0 || run.ProofState == "exited_proven" {
				disposition = "stopped"
				break
			}
		}
	}
	group, err = t.q.SealCollaborationGroup(ctx, db.SealCollaborationGroupParams{GroupID: group.ID, Disposition: disposition, Reason: reason})
	if err != nil {
		return CollaborationTargetControlResult{}, err
	}
	if err = t.q.CancelCollaborationGroupDelegations(ctx, db.CancelCollaborationGroupDelegationsParams{GroupID: group.ID, Reason: reason}); err != nil {
		return CollaborationTargetControlResult{}, err
	}
	r := CollaborationTargetControlResult{TargetID: target.ID, GroupID: group.ID}
	for _, run := range runs {
		if run.SettledAt.Valid {
			continue
		}
		stopped, err := t.stopRun(ctx, run, group.ForcedDisposition.String, reason)
		if err != nil {
			return CollaborationTargetControlResult{}, err
		}
		r.StoppedRuns++
		if !stopped.RunSettled {
			r.UnsettledRuns++
		}
	}
	r.GroupSettled, err = t.settleProvenGroup(ctx, target, group)
	return r, err
}

// settleProvenGroup never resumes a channel. The source issue/session/channel,
// command and group locks belong to the caller's transaction. Each proven run
// may release its physical reservation, but the next batch waits for the whole
// sealed group and every recorded descendant intention to finish.
func (t *collaborationQueueTx) settleProvenGroup(ctx context.Context, target db.CollaborationTarget, group db.CollaborationExecutionGroup) (bool, error) {
	runs, err := t.q.LockCollaborationGroupRuns(ctx, group.ID)
	if err != nil {
		return false, err
	}
	if err = t.validateGroup(ctx, target, group, runs); err != nil {
		return false, err
	}
	for _, run := range runs {
		if err = t.q.ReleaseCollaborationRunReservation(ctx, run.ID); err != nil {
			return false, err
		}
	}
	settled, err := t.q.SettleCollaborationProvenGroup(ctx, group.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = t.q.FinishCollaborationBatchInputs(ctx, db.FinishCollaborationBatchInputsParams{BatchID: group.BatchID,
		State: settled.State, Reason: settled.DispositionReason.String}); err != nil {
		return false, err
	}
	if _, err = t.q.FinishCollaborationBatch(ctx, db.FinishCollaborationBatchParams{BatchID: group.BatchID, State: settled.State}); err != nil {
		return false, err
	}
	if err = t.q.ClearCollaborationChannelGroup(ctx, db.ClearCollaborationChannelGroupParams{ID: group.ChannelID, ActiveGroupID: group.ID}); err != nil {
		return false, err
	}
	channel, _ := t.channel(group.ChannelID)
	channel.ActiveGroupID = pgtype.UUID{}
	if err = t.mirrorChannel(ctx, channel); err != nil {
		return false, err
	}
	return true, nil
}
