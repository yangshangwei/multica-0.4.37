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

// Group/run/delegation insertion is deliberately absent until runtime admission
// exists. These helpers validate retained relationships before closing work.
func (t *collaborationQueueTx) validateGroup(ctx context.Context, target db.CollaborationTarget, group db.CollaborationExecutionGroup, runs []db.CollaborationRun) error {
	if group.SessionID != t.session.ID || group.TargetID != target.ID || group.Generation != target.GroupGeneration {
		return invalidCollaborationLedger("group scope mismatch")
	}
	command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: group.CommandID})
	if err != nil {
		return err
	}
	if command.LegacyUnresolved || command.TargetID != target.ID || command.State != "in_flight" {
		return invalidCollaborationLedger("group command mismatch")
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
		if run.LegacyUnresolved || run.GroupID != group.ID || run.TargetID != target.ID || run.CommandID != group.CommandID || run.SessionID != t.session.ID ||
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
	delegations, err := t.q.LockCollaborationGroupDelegations(ctx, group.ID)
	if err != nil {
		return CollaborationTargetControlResult{}, err
	}
	for _, delegation := range delegations {
		parentFound := false
		for _, run := range runs {
			if run.ID == delegation.ParentRunID {
				parentFound = true
				break
			}
		}
		if !parentFound {
			return CollaborationTargetControlResult{}, invalidCollaborationLedger("delegation parent missing from group")
		}
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
	settled, err := t.q.SettleStoppedCollaborationGroup(ctx, group.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, nil
	}
	if err != nil {
		return CollaborationTargetControlResult{}, err
	}
	if _, err = t.q.FinishCollaborationCommand(ctx, db.FinishCollaborationCommandParams{SessionID: t.session.ID, CommandID: group.CommandID,
		State: settled.State, Reason: settled.DispositionReason.String}); err != nil {
		return CollaborationTargetControlResult{}, err
	}
	updated, err := t.q.ClearCollaborationActiveGroup(ctx, db.ClearCollaborationActiveGroupParams{TargetID: target.ID, GroupID: group.ID})
	if err != nil {
		return CollaborationTargetControlResult{}, err
	}
	t.replaceTarget(updated)
	r.GroupSettled = true
	return r, nil
}
