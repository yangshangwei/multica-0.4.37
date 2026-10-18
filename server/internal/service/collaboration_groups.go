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

func collaborationRunProvenEnded(run db.CollaborationRun) bool {
	if !run.SettledAt.Valid || run.ProgressState != "settled" || !run.SealedAttemptSeq.Valid || run.SealedAttemptSeq.Int64 < run.AuthorizedAttemptSeq {
		return false
	}
	switch run.ProofState {
	case "never_launched_proven":
		return !run.StartedAt.Valid && run.BusinessOutcome.String != "success"
	case "exited_proven":
		return true
	default:
		return false
	}
}

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
	if command.TargetID != target.ID || command.State != "in_flight" || command.GroupID != group.ID || command.ChannelID != group.ChannelID || command.BatchID != group.BatchID {
		return invalidCollaborationLedger("group command mismatch")
	}
	inputs, err := t.q.ListCollaborationBatchInputs(ctx, group.BatchID)
	if err != nil {
		return err
	}
	byCommand := make(map[pgtype.UUID]db.CollaborationCommand, len(inputs))
	for _, input := range inputs {
		if input.SessionID != group.SessionID || input.ChannelID != group.ChannelID ||
			(input.State != "in_flight" && input.State != "cancelled" && input.State != "waiting") || ((input.State == "in_flight" || input.State == "waiting") && input.GroupID != group.ID) {
			return invalidCollaborationLedger("batch input scope mismatch")
		}
		byCommand[input.ID] = input
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
	retained, err := t.q.ListCollaborationGroupInputs(ctx, group.ID)
	if err != nil {
		return err
	}
	consumed := make(map[pgtype.UUID]bool, len(retained))
	for _, input := range retained {
		command, exists := byCommand[input.CommandID]
		if !exists {
			command, err = t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: input.CommandID})
			if err != nil {
				return err
			}
		}
		run, runExists := byID[input.ConsumingRunID]
		if !runExists || consumed[command.ID] || input.ChannelID != group.ChannelID || input.BatchID != group.BatchID || run.GroupID != group.ID {
			return invalidCollaborationLedger("retained group input scope mismatch")
		}
		consumed[command.ID] = true
		if command.GroupID != group.ID {
			readers, err := collaborationInputReaders(input, runs)
			if err != nil {
				return err
			}
			if run.StepKind.String != "continuation" || !collaborationReadersNeverLaunched(readers) {
				return invalidCollaborationLedger("returned input has a potentially launched reader")
			}
			dependencies, err := t.q.ListCollaborationGroupDependencies(ctx, group.ID)
			if err != nil {
				return err
			}
			detached := false
			for _, dependency := range dependencies {
				detached = detached || dependency.CommandID == command.ID && dependency.DetachedAt.Valid && dependency.DetachReason.String == "never_launched"
			}
			if !detached {
				return invalidCollaborationLedger("returned input dependency is still active")
			}
			continue
		}
		if command.State != "in_flight" || command.BatchID != input.BatchID || command.ChannelID != input.ChannelID || command.Seq != input.Seq {
			return invalidCollaborationLedger("retained group input scope mismatch")
		}
	}
	for _, input := range inputs {
		if input.State == "in_flight" && !consumed[input.ID] {
			return invalidCollaborationLedger("consumed group input history missing")
		}
	}
	leaderTurns := make(map[int64]bool)
	for _, run := range runs {
		if run.SettledAt.Valid && !collaborationRunProvenEnded(run) {
			return invalidCollaborationLedger("settled run has contradictory termination evidence")
		}
		if run.GroupID != group.ID || run.TargetID != target.ID || run.CommandID != group.CommandID || run.SessionID != t.session.ID ||
			run.ChannelID != group.ChannelID || run.BatchID != group.BatchID ||
			!run.TargetEpoch.Valid || run.TargetEpoch.Int64 != group.TargetEpoch || !run.ConfigVersion.Valid || run.ConfigVersion.Int64 != group.ConfigVersion {
			return invalidCollaborationLedger("run scope mismatch")
		}
		matched := false
		role := run.StepKind.String
		if !group.SettledAt.Valid && (role == "agent" || role == "leader" || role == "continuation") {
			if !run.LeaderTurn.Valid || run.LeaderTurn.Int64 < 1 || leaderTurns[run.LeaderTurn.Int64] {
				return invalidCollaborationLedger("missing or repeated leader turn")
			}
			leaderTurns[run.LeaderTurn.Int64] = true
		}
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
		if delegation.SourceCommentID.Valid {
			child, exists := byID[delegation.RunID]
			if !exists || child.ParentRunID != delegation.ParentRunID || child.AgentID != delegation.MemberAgentID || child.StepKind.String != "member" {
				return invalidCollaborationLedger("comment delegation child identity mismatch")
			}
			members, err := decodeCollaborationRoster[string](delegation.Members)
			if err != nil {
				return err
			}
			if len(members) != 1 || members[0] != util.UUIDToString(child.AgentID) {
				return invalidCollaborationLedger("comment delegation member identity mismatch")
			}
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
	if err = t.detachGroupUnreadInputs(ctx, group, reason); err != nil {
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
	if group.ExpansionSealedAt.Valid {
		if err = t.detachGroupUnreadInputs(ctx, group, "group_ended"); err != nil {
			return false, err
		}
	}
	if err = t.returnNeverLaunchedInputs(ctx, group, runs); err != nil {
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
	if _, err = t.q.FinishCollaborationBatch(ctx, db.FinishCollaborationBatchParams{BatchID: group.BatchID, State: settled.State}); err != nil {
		return false, err
	}
	if err = t.returnNeverLaunchedInputs(ctx, settled, runs); err != nil {
		return false, err
	}
	if err = t.q.FinishCollaborationBatchInputs(ctx, db.FinishCollaborationBatchInputsParams{BatchID: group.BatchID,
		State: settled.State, Reason: settled.DispositionReason.String}); err != nil {
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

// Only affirmative nonlaunch evidence makes a consumed input unread again.
// A missing started report, failed outcome or one unlaunched descendant is
// insufficient. Retained membership keeps the settled group's original facts.
func (t *collaborationQueueTx) returnNeverLaunchedInputs(ctx context.Context, group db.CollaborationExecutionGroup, runs []db.CollaborationRun) error {
	if len(runs) == 0 {
		return invalidCollaborationLedger("settled group has no runs")
	}
	allNeverLaunched := true
	byID := make(map[pgtype.UUID]db.CollaborationRun, len(runs))
	for _, run := range runs {
		byID[run.ID] = run
		if !collaborationRunProvenEnded(run) || run.ProofState != "never_launched_proven" {
			allNeverLaunched = false
		}
	}
	retained, err := t.q.ListCollaborationGroupInputs(ctx, group.ID)
	if err != nil {
		return err
	}
	var batch db.CollaborationBatch
	for _, input := range retained {
		consumer, exists := byID[input.ConsumingRunID]
		if !exists {
			return invalidCollaborationLedger("nonlaunch input consumer missing")
		}
		if consumer.StepKind.String == "continuation" {
			readers, err := collaborationInputReaders(input, runs)
			if err != nil {
				return err
			}
			if !collaborationReadersNeverLaunched(readers) {
				continue
			}
		} else if !group.SettledAt.Valid || !allNeverLaunched {
			continue
		}
		command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: input.CommandID})
		if err != nil {
			return err
		}
		if command.GroupID != group.ID || command.State != "in_flight" {
			continue
		}
		if err = t.q.DetachCollaborationGroupDependency(ctx, db.DetachCollaborationGroupDependencyParams{GroupID: group.ID, CommandID: command.ID, Reason: "never_launched"}); err != nil {
			return err
		}
		code, err := collaborationCurrentBatchAuthors(ctx, t.q, t.access.Issue.ID, t.access.LifecycleEpoch, []db.CollaborationCommand{command}, "in_flight")
		if err != nil {
			return err
		}
		state, reason := "waiting", "never_launched"
		if t.session.QueueState == "closed" || code != "" {
			state, reason = "cancelled", group.DispositionReason.String
			if reason == "" {
				reason = "stale_authority"
			}
		}
		if !batch.ID.Valid {
			batch, err = t.newWaitingBatch(ctx, group.ChannelID, group.TargetID)
			if err != nil {
				return err
			}
		}
		if _, err = t.q.ReturnNeverLaunchedCollaborationInput(ctx, db.ReturnNeverLaunchedCollaborationInputParams{CommandID: command.ID, GroupID: group.ID, BatchID: batch.ID, State: state, Reason: reason}); err != nil {
			return err
		}
	}
	return t.q.CloseEmptyCollaborationBatches(ctx, t.session.ID)
}
