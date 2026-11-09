package iteration

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type MembershipInput struct {
	Before         db.Issue
	Source, Target db.Iteration
	AllowCompleted bool
}

// PrepareMembershipChanges requires the caller's complete issue, iteration and
// reference locks, including the status catalog. Inputs are unique and ordered
// by the caller's issue lock order. Participation locks retain that order and
// source-before-target ordering; invalid inputs may take extra locks but never
// write. Single-issue callers keep their original query and validation path.
func PrepareMembershipChanges(ctx context.Context, tx pgx.Tx, inputs []MembershipInput) ([]*MembershipChange, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) == 1 {
		in := inputs[0]
		change, err := PrepareMembershipChange(ctx, tx, in.Before, in.Source, in.Target, in.AllowCompleted)
		if err != nil {
			return nil, err
		}
		return []*MembershipChange{change}, nil
	}
	keys := []db.LockIssueIterationParticipationBatchParams{}
	seen := map[pgtype.UUID]bool{}
	for _, in := range inputs {
		if seen[in.Before.ID] {
			return nil, errors.New("duplicate membership issue")
		}
		seen[in.Before.ID] = true
		if in.Before.CurrentIterationID == in.Target.ID {
			continue
		}
		for _, row := range []db.Iteration{in.Source, in.Target} {
			if row.ID.Valid {
				keys = append(keys, db.LockIssueIterationParticipationBatchParams{WorkspaceID: in.Before.WorkspaceID, IterationID: row.ID, IssueID: in.Before.ID})
			}
		}
	}
	type result struct {
		row db.IterationParticipation
		err error
	}
	saved := map[db.LockIssueIterationParticipationParams]result{}
	if len(keys) > 0 {
		batch := db.New(tx).LockIssueIterationParticipationBatch(ctx, keys)
		var first error
		batch.QueryRow(func(i int, row db.IterationParticipation, err error) {
			key := keys[i]
			saved[db.LockIssueIterationParticipationParams{WorkspaceID: key.WorkspaceID, IterationID: key.IterationID, IssueID: key.IssueID}] = result{row, err}
			if err != nil && !errors.Is(err, pgx.ErrNoRows) && first == nil {
				first = err
			}
		})
		if err := batch.Close(); first != nil {
			return nil, first
		} else if err != nil {
			return nil, err
		}
	}
	lookup := func(_ context.Context, key db.LockIssueIterationParticipationParams) (db.IterationParticipation, error) {
		r, ok := saved[key]
		if !ok {
			return db.IterationParticipation{}, errors.New("membership participation was not prepared")
		}
		return r.row, r.err
	}
	changes := make([]*MembershipChange, 0, len(inputs))
	for _, in := range inputs {
		change, err := prepareMembershipChange(ctx, tx, in.Before, in.Source, in.Target, in.AllowCompleted, lookup)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}
