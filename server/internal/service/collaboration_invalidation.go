package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// stopInvalidatedWork borrows the writer's source/target/grant locks. Grants are
// the pre-revocation rows; epoch zero means participant departure rather than a
// credential generation. Root authorship is immutable across all descendants.
func (t *collaborationQueueTx) stopInvalidatedWork(ctx context.Context, user pgtype.UUID, epoch int64, grants []db.CollaborationGrant, reason string) (*CollaborationQueueReceipt, error) {
	selected := make(map[pgtype.UUID]string)
	invalidGrants := make(map[pgtype.UUID]bool)
	for _, grant := range grants {
		if epoch > 0 && grant.OwnerAuthEpoch >= epoch {
			continue
		}
		invalidGrants[grant.ID] = true

		if grant.RevokedAt.Valid {
			continue
		}
		target, ok := t.target(grant.TargetID)
		if !ok {
			return nil, invalidCollaborationLedger("grant target missing")
		}
		if !grant.ConfigVersion.Valid || grant.ConfigVersion.Int64 != target.ConfigVersion {
			continue
		}
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
		if err != nil {
			return nil, err
		}
		for _, resource := range roster {
			if collaborationResourceMatchesGrant(resource, grant) {
				selected[target.ID] = "stopped"
			}
		}
	}
	if _, err := t.q.LockCollaborationSessionCommands(ctx, t.session.ID); err != nil {
		return nil, err
	}
	if err := t.q.CloseEmptyCollaborationBatches(ctx, t.session.ID); err != nil {
		return nil, err
	}
	groups, err := t.q.LockCollaborationUnsettledGroups(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: group.CommandID})
		if err != nil {
			return nil, err
		}
		if command.TargetID != group.TargetID {
			return nil, invalidCollaborationLedger("invalidated group command mismatch")
		}
		author := user.Valid && command.AuthorUserID == user && (epoch == 0 || command.AuthorAuthEpoch < epoch)
		batchAuthor, err := collaborationBatchContainsAuthor(ctx, t.q, group.BatchID, user, epoch)
		if err != nil {
			return nil, err
		}
		author = author || batchAuthor
		if author {
			selected[group.TargetID] = "cancelled"
		}
		roster, err := decodeCollaborationRoster[collaboration.GroupResource](group.Roster)
		if err != nil {
			return nil, err
		}
		for _, resource := range roster {
			grantID, err := util.ParseUUID(resource.GrantID)
			if err != nil {
				return nil, invalidCollaborationLedger("group grant identity malformed")
			}
			if invalidGrants[grantID] {
				if selected[group.TargetID] == "" {
					selected[group.TargetID] = "stopped"
				}
			}
		}
	}
	runs, err := t.q.LockCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: run.CommandID})
		if err != nil {
			return nil, err
		}
		author := user.Valid && command.AuthorUserID == user && (epoch == 0 || command.AuthorAuthEpoch < epoch)
		batchAuthor, err := collaborationBatchContainsAuthor(ctx, t.q, run.BatchID, user, epoch)
		if err != nil {
			return nil, err
		}
		author = author || batchAuthor
		if !author && !invalidGrants[run.GrantID] {
			continue
		}
		if author {
			selected[run.TargetID] = "cancelled"
		} else if selected[run.TargetID] == "" {
			selected[run.TargetID] = "stopped"
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}

	// stopGroup decides cancelled versus stopped from every run's original
	// evidence, including descendants that the removed author never directly ran.
	r, err := t.stopTargets(ctx, selected, false, reason, false)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// A started batch retains every human authorization, even though one immutable
// input anchors its root group. Losing any contributing author stops the batch.
func collaborationBatchContainsAuthor(ctx context.Context, q *db.Queries, batchID, user pgtype.UUID, epoch int64) (bool, error) {
	if !batchID.Valid || !user.Valid {
		return false, nil
	}
	inputs, err := q.ListCollaborationBatchInputs(ctx, batchID)
	if err != nil {
		return false, err
	}
	for _, input := range inputs {
		if input.State == "in_flight" && input.AuthorUserID == user && (epoch == 0 || input.AuthorAuthEpoch < epoch) {
			return true, nil
		}
	}
	return false, nil
}
