package iteration

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CommitMembershipChanges pipelines a complete lifecycle set after preparation.
// Every source leave precedes every target join, including cross-swapped scopes.
// Queries retain their canonical SQL and each result is consumed before the
// next phase. Any statement or CAS failure belongs to the owner's rollback.
func CommitMembershipChanges(ctx context.Context, tx pgx.Tx, changes []*MembershipChange, actor json.RawMessage, operationID pgtype.UUID, businessAt time.Time) error {
	if len(changes) == 0 {
		return nil
	}
	if len(changes) == 1 {
		_, e := CommitMembershipChange(ctx, tx, changes[0], actor, operationID, businessAt)
		return e
	}
	if e := validateIssueRecordIdentity(actor, operationID); e != nil {
		return e
	}
	sources := []db.AppendIterationLifecycleEventBatchParams{}
	targets := []db.AppendIterationLifecycleEventBatchParams{}
	leaves := []db.LeaveDeletedIssueIterationParticipationBatchParams{}
	joins := []db.JoinIterationParticipationBatchParams{}
	sourceRevisions := []db.AdvanceIterationScopeRevisionBatchParams{}
	targetRevisions := []db.AdvanceIterationScopeRevisionBatchParams{}
	updates := []db.SetIssueCurrentIterationBatchParams{}
	sample := pgtype.Timestamptz{Time: businessAt, Valid: true}
	for _, change := range changes {
		if change == nil {
			return errors.New("membership change is required")
		}
		before := change.before
		if before.CurrentIterationID == change.target.ID {
			continue
		}
		rollover := before.IterationRolloverCount
		if change.Rollover {
			if rollover == 2147483647 {
				return errors.New("rollover count exhausted")
			}
			rollover++
		}
		var reason pgtype.Text
		if change.Reason != nil {
			reason = pgtype.Text{String: *change.Reason, Valid: true}
		}
		event := db.AppendIterationLifecycleEventBatchParams{WorkspaceID: before.WorkspaceID, IssueID: before.ID, OperationID: operationID, Actor: actor, SampledAt: sample, Reason: reason}
		if change.source.ID.Valid {
			raw, e := marshalMembershipFacts(change.sourceFacts, change.source.ID, change.target.ID)
			if e != nil {
				return e
			}
			event.IterationID = change.source.ID
			event.Kind = "leave"
			if change.source.Status == "planned" {
				event.Kind = "planned_activity"
			}
			event.BeforeFacts = raw
			event.AfterFacts = []byte("null")
			sources = append(sources, event)
			leaves = append(leaves, db.LeaveDeletedIssueIterationParticipationBatchParams{WorkspaceID: before.WorkspaceID, IterationID: change.source.ID, IssueID: before.ID, BusinessAt: sample})
			sourceRevisions = append(sourceRevisions, db.AdvanceIterationScopeRevisionBatchParams{WorkspaceID: before.WorkspaceID, ID: change.source.ID})
		}
		if change.target.ID.Valid {
			targetFacts := change.facts
			targetFacts.RolloverCount = rollover
			raw, e := marshalMembershipFacts(targetFacts, change.source.ID, change.target.ID)
			if e != nil {
				return e
			}
			event.IterationID = change.target.ID
			event.Kind = "join"
			if change.target.Status == "planned" {
				event.Kind = "planned_activity"
			} else if change.reentry {
				event.Kind = "reenter"
			}
			event.BeforeFacts = []byte("null")
			event.AfterFacts = raw
			targets = append(targets, event)
			joins = append(joins, db.JoinIterationParticipationBatchParams{WorkspaceID: before.WorkspaceID, IterationID: change.target.ID, IssueID: before.ID, BusinessAt: sample, HasStarted: change.facts.HasStarted})
			targetRevisions = append(targetRevisions, db.AdvanceIterationScopeRevisionBatchParams{WorkspaceID: before.WorkspaceID, ID: change.target.ID})
		}

		updates = append(updates, db.SetIssueCurrentIterationBatchParams{WorkspaceID: before.WorkspaceID, IssueID: before.ID, ExpectedRevision: before.Revision, CurrentIterationID: change.target.ID, RolloverCount: rollover, BusinessAt: sample})
	}
	q := db.New(tx)
	if len(sources) > 0 {
		if e := finishMembershipExec(q.AppendIterationLifecycleEventBatch(ctx, sources)); e != nil {
			return e
		}
		var first error
		b := q.LeaveDeletedIssueIterationParticipationBatch(ctx, leaves)
		b.QueryRow(func(_ int, _ pgtype.UUID, e error) {
			if first == nil {
				first = e
			}
		})
		closeErr := b.Close()
		if first != nil {
			return first
		}
		if closeErr != nil {
			return closeErr
		}
		if e := finishMembershipExec(q.AdvanceIterationScopeRevisionBatch(ctx, sourceRevisions)); e != nil {
			return e
		}
	}
	if len(targets) > 0 {
		if e := finishMembershipExec(q.AppendIterationLifecycleEventBatch(ctx, targets)); e != nil {
			return e
		}
		if e := finishMembershipExec(q.JoinIterationParticipationBatch(ctx, joins)); e != nil {
			return e
		}
		if e := finishMembershipExec(q.AdvanceIterationScopeRevisionBatch(ctx, targetRevisions)); e != nil {
			return e
		}
	}
	if len(updates) > 0 {
		var first error
		b := q.SetIssueCurrentIterationBatch(ctx, updates)
		b.QueryRow(func(_ int, _ db.Issue, e error) {
			if first == nil {
				first = e
			}
		})
		closeErr := b.Close()
		if errors.Is(first, pgx.ErrNoRows) {
			return &OperationError{Status: 409, Code: "iteration_preview_stale", Message: "Issue changed"}
		}
		if first != nil {
			return first
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func finishMembershipExec(batch interface {
	Exec(func(int, error))
	Close() error
}) error {
	var first error
	batch.Exec(func(_ int, e error) {
		if first == nil {
			first = e
		}
	})
	closeErr := batch.Close()
	if first != nil {
		return first
	}
	return closeErr
}
