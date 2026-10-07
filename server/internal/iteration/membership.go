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

// MembershipChange is prepared after all iteration/issue/reference locks. Its
// caller samples once after preparing the entire set and commits in the same
// transaction. It never starts a transaction, retries, or invokes execution.
type MembershipChange struct {
	Reason      *string
	Rollover    bool
	before      db.Issue
	source      db.Iteration
	target      db.Iteration
	facts       IssueFacts
	sourceFacts IssueFacts
	reentry     bool
}

func PrepareMembershipChange(ctx context.Context, tx pgx.Tx, before db.Issue, source, target db.Iteration, allowCompleted bool) (*MembershipChange, error) {
	if before.AdmissionStatus != "not_required" && before.AdmissionStatus != "accepted" {
		return nil, &OperationError{Status: 409, Code: "triage_review_required", Message: "Issue requires triage acceptance"}
	}
	if before.CurrentIterationID.Valid && (source.ID != before.CurrentIterationID || source.WorkspaceID != before.WorkspaceID) {
		return nil, errors.New("membership source was not locked")
	}
	for _, row := range []db.Iteration{source, target} {
		if row.ID.Valid && (row.WorkspaceID != before.WorkspaceID || (row.Status != "planned" && row.Status != "active")) {
			return nil, &OperationError{Status: 409, Code: "iteration_history_move_unsupported", Message: "Historical iterations cannot change membership"}
		}
	}
	q := db.New(tx)
	facts, err := issueFacts(ctx, q, before, false)
	if err != nil {
		return nil, err
	}
	facts.HasStarted = startedCategory(facts.StatusCategory)
	change := &MembershipChange{before: before, source: source, target: target, facts: facts, sourceFacts: facts}
	if before.CurrentIterationID == target.ID {
		return change, nil
	}
	if target.ID.Valid && (facts.StatusCategory == "cancelled" || (facts.StatusCategory == "done" && (target.Status != "active" || !allowCompleted))) {
		return nil, &OperationError{Status: 422, Code: "iteration_validation_failed", Message: "Terminal issue cannot join this iteration without confirmation"}
	}
	if source.ID.Valid {
		p, err := q.LockIssueIterationParticipation(ctx, db.LockIssueIterationParticipationParams{WorkspaceID: before.WorkspaceID, IterationID: source.ID, IssueID: before.ID})
		if err != nil {
			return nil, err
		}
		if !p.CurrentJoinedAt.Valid {
			return nil, errors.New("source participation is not current")
		}
		change.sourceFacts.HasStarted = p.HasStartedCurrentParticipation
	}
	if target.ID.Valid {
		p, err := q.LockIssueIterationParticipation(ctx, db.LockIssueIterationParticipationParams{WorkspaceID: before.WorkspaceID, IterationID: target.ID, IssueID: before.ID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil && p.CurrentJoinedAt.Valid {
			return nil, errors.New("target participation is already current")
		}
		if target.Status == "active" {
			seen, err := q.HasIterationActiveJoin(ctx, db.HasIterationActiveJoinParams{WorkspaceID: before.WorkspaceID, IterationID: target.ID, IssueID: before.ID})
			if err != nil {
				return nil, err
			}
			change.reentry = p.InOriginal || seen
		}
	}
	return change, nil
}

func CommitMembershipChange(ctx context.Context, tx pgx.Tx, change *MembershipChange, actor json.RawMessage, operationID pgtype.UUID, businessAt time.Time) (db.Issue, error) {
	if change == nil {
		return db.Issue{}, errors.New("membership change is required")
	}
	before := change.before
	if before.CurrentIterationID == change.target.ID {
		return before, nil
	}
	if err := validateIssueRecordIdentity(actor, operationID); err != nil {
		return db.Issue{}, err
	}
	rollover := before.IterationRolloverCount
	if change.Rollover {
		if rollover == 2147483647 {
			return db.Issue{}, errors.New("rollover count exhausted")
		}
		rollover++
	}
	q := db.New(tx)
	sample := pgtype.Timestamptz{Time: businessAt, Valid: true}
	appendEvent := func(iid pgtype.UUID, kind string, old, new []byte) error {
		var reason pgtype.Text
		if change.Reason != nil {
			reason = pgtype.Text{String: *change.Reason, Valid: true}
		}
		return q.AppendIterationLifecycleEvent(ctx, db.AppendIterationLifecycleEventParams{WorkspaceID: before.WorkspaceID, IterationID: iid, IssueID: before.ID, OperationID: operationID, Kind: kind, Actor: actor, SampledAt: sample, BeforeFacts: old, AfterFacts: new, Reason: reason})
	}
	if change.source.ID.Valid {
		facts, err := marshalMembershipFacts(change.sourceFacts, change.source.ID, change.target.ID)
		if err != nil {
			return db.Issue{}, err
		}
		kind := "leave"
		if change.source.Status == "planned" {
			kind = "planned_activity"
		}
		if err = appendEvent(change.source.ID, kind, facts, []byte("null")); err != nil {
			return db.Issue{}, err
		}
		affected, err := q.LeaveDeletedIssueIterationParticipation(ctx, db.LeaveDeletedIssueIterationParticipationParams{WorkspaceID: before.WorkspaceID, IterationID: change.source.ID, IssueID: before.ID, BusinessAt: sample})
		if err != nil {
			return db.Issue{}, err
		}
		if affected != 1 {
			return db.Issue{}, errors.New("membership source changed")
		}
		if err = q.AdvanceIterationScopeRevision(ctx, db.AdvanceIterationScopeRevisionParams{WorkspaceID: before.WorkspaceID, ID: change.source.ID}); err != nil {
			return db.Issue{}, err
		}
	}
	if change.target.ID.Valid {
		targetFacts := change.facts
		targetFacts.RolloverCount = rollover
		facts, err := marshalMembershipFacts(targetFacts, change.source.ID, change.target.ID)
		if err != nil {
			return db.Issue{}, err
		}
		kind := "join"
		if change.target.Status == "planned" {
			kind = "planned_activity"
		} else if change.reentry {
			kind = "reenter"
		}
		if err = appendEvent(change.target.ID, kind, []byte("null"), facts); err != nil {
			return db.Issue{}, err
		}
		if err = q.JoinIterationParticipation(ctx, db.JoinIterationParticipationParams{WorkspaceID: before.WorkspaceID, IterationID: change.target.ID, IssueID: before.ID, BusinessAt: sample, HasStarted: change.facts.HasStarted}); err != nil {
			return db.Issue{}, err
		}
		if err = q.AdvanceIterationScopeRevision(ctx, db.AdvanceIterationScopeRevisionParams{WorkspaceID: before.WorkspaceID, ID: change.target.ID}); err != nil {
			return db.Issue{}, err
		}
	}

	after, err := q.SetIssueCurrentIteration(ctx, db.SetIssueCurrentIterationParams{WorkspaceID: before.WorkspaceID, IssueID: before.ID, ExpectedRevision: before.Revision, CurrentIterationID: change.target.ID, RolloverCount: rollover, BusinessAt: sample})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Issue{}, &OperationError{Status: 409, Code: "iteration_preview_stale", Message: "Issue changed"}
	}
	return after, err
}

// marshalMembershipFacts adds immutable transition identity without changing
// IssueFacts or the reducer's null-before/null-after membership semantics.
// Explicit null means unassigned; missing keys on older events remain unknown.
func marshalMembershipFacts(facts IssueFacts, source, target pgtype.UUID) ([]byte, error) {
	return json.Marshal(struct {
		IssueFacts
		SourceIterationID *string `json:"source_iteration_id"`
		TargetIterationID *string `json:"target_iteration_id"`
	}{facts, historyNullableUUID(source), historyNullableUUID(target)})
}
