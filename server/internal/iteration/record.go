package iteration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// IssueFacts is the durable factual portion of an issue change. Revision and
// content-only edits deliberately do not manufacture statistical activity.
type IssueFacts struct {
	IssueID        pgtype.UUID `json:"issue_id"`
	Title          string      `json:"title"`
	ProjectID      pgtype.UUID `json:"project_id"`
	AssigneeType   pgtype.Text `json:"assignee_type"`
	AssigneeID     pgtype.UUID `json:"assignee_id"`
	StatusKey      string      `json:"status_key"`
	StatusCategory string      `json:"status_category"`
	RolloverCount  int32       `json:"rollover_count"`
	HasStarted     bool        `json:"has_started"`
}

// IssueRecord is prepared after the issue lock, before any business write.
// Its caller owns the transaction, authorization, catalogue/workspace fence,
// and iteration row lock. The recorder never commits or performs side effects.
type IssueRecord struct {
	before     db.Issue
	facts      IssueFacts
	businessAt time.Time
	planned    bool
}

// LockIssueIteration acquires the current iteration before attachment/issue
// locks. Even an unassociated issue must already be behind LockWorkspace.
func LockIssueIteration(ctx context.Context, tx pgx.Tx, workspaceID, issueID pgtype.UUID) (db.Iteration, error) {
	row, err := db.New(tx).LockIssueIteration(ctx, db.LockIssueIterationParams{WorkspaceID: workspaceID, ID: issueID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Iteration{}, nil
	}
	return row, err
}

func issueFacts(ctx context.Context, q *db.Queries, issue db.Issue, started bool) (IssueFacts, error) {
	category := issue.Status
	if !issuestatus.IsBuiltIn(category) {
		entry, err := q.GetIssueStatusEntryByKey(ctx, db.GetIssueStatusEntryByKeyParams{WorkspaceID: issue.WorkspaceID, Key: issue.Status})
		if err != nil {
			return IssueFacts{}, fmt.Errorf("resolve iteration status facts: %w", err)
		}
		category = entry.Category
	}
	if !issuestatus.IsCategory(category) {
		return IssueFacts{}, fmt.Errorf("unknown iteration status category %q", category)
	}
	return IssueFacts{IssueID: issue.ID, Title: issue.Title, ProjectID: issue.ProjectID, AssigneeType: issue.AssigneeType, AssigneeID: issue.AssigneeID, StatusKey: issue.Status, StatusCategory: category, RolloverCount: issue.IterationRolloverCount, HasStarted: started}, nil
}

func PrepareIssueRecord(ctx context.Context, tx pgx.Tx, before db.Issue, locked db.Iteration) (*IssueRecord, error) {
	if !before.CurrentIterationID.Valid {
		return nil, nil
	}
	if before.CurrentIterationID != locked.ID || before.WorkspaceID != locked.WorkspaceID || (locked.Status != "planned" && locked.Status != "active") {
		return nil, errors.New("current iteration missing or closed")
	}
	q := db.New(tx)
	participation, err := q.LockIssueIterationParticipation(ctx, db.LockIssueIterationParticipationParams{WorkspaceID: before.WorkspaceID, IterationID: before.CurrentIterationID, IssueID: before.ID})
	if err != nil {
		return nil, fmt.Errorf("lock current iteration participation: %w", err)
	}
	if !participation.CurrentJoinedAt.Valid {
		return nil, errors.New("current iteration participation is not joined")
	}
	facts, err := issueFacts(ctx, q, before, participation.HasStartedCurrentParticipation)
	if err != nil {
		return nil, err
	}
	businessAt, err := SampleBusinessTime(ctx, tx, nil)
	if err != nil {
		return nil, err
	}
	return &IssueRecord{before: before, facts: facts, businessAt: businessAt, planned: locked.Status == "planned"}, nil
}

// RecordIssueChange persists the after facts, started evidence, event sequence
// and scope revision in the caller's transaction. Original commitment facts and
// the current pointer/rollover counter are never rewritten by ordinary edits.
func RecordIssueChange(ctx context.Context, tx pgx.Tx, record *IssueRecord, after db.Issue, actor json.RawMessage, operationID pgtype.UUID) error {
	if record == nil {
		return nil
	}
	before := record.before
	if before.ID != after.ID || before.WorkspaceID != after.WorkspaceID || before.CurrentIterationID != after.CurrentIterationID {
		return errors.New("issue record identity or iteration changed")
	}
	q := db.New(tx)
	facts, err := issueFacts(ctx, q, after, record.facts.HasStarted)
	if err != nil {
		return err
	}
	facts.HasStarted = facts.HasStarted || startedCategory(facts.StatusCategory)
	if facts == record.facts {
		return nil
	}
	var identity struct {
		Type   string      `json:"type"`
		ID     pgtype.UUID `json:"id"`
		UserID pgtype.UUID `json:"user_id"`
	}
	if !operationID.Valid || operationID.Bytes == [16]byte{} || json.Unmarshal(actor, &identity) != nil || (identity.Type != "member" && identity.Type != "agent") || !identity.ID.Valid || identity.ID.Bytes == [16]byte{} || !identity.UserID.Valid || identity.UserID.Bytes == [16]byte{} {
		return errors.New("iteration event requires operation and actor identity")
	}
	if facts.HasStarted && !record.facts.HasStarted {
		if err = q.MarkIterationParticipationStarted(ctx, db.MarkIterationParticipationStartedParams{WorkspaceID: after.WorkspaceID, IterationID: after.CurrentIterationID, IssueID: after.ID}); err != nil {
			return err
		}
	}
	beforeJSON, err := json.Marshal(record.facts)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(facts)
	if err != nil {
		return err
	}
	kind := "issue_changed"
	if record.planned {
		kind = "planned_activity"
	} else if record.facts.StatusCategory != facts.StatusCategory {
		switch {
		case facts.StatusCategory == "cancelled":
			kind = "cancel"
		case record.facts.StatusCategory == "cancelled" || (record.facts.StatusCategory == "done" && facts.StatusCategory != "done"):
			kind = "reopen"
		default:
			kind = "status"
		}
	}
	if err = q.AppendIterationIssueEvent(ctx, db.AppendIterationIssueEventParams{WorkspaceID: after.WorkspaceID, IterationID: after.CurrentIterationID, OperationID: operationID, IssueID: after.ID, Actor: actor, Kind: kind, SampledAt: pgtype.Timestamptz{Time: record.businessAt, Valid: true}, BeforeFacts: beforeJSON, AfterFacts: afterJSON}); err != nil {
		return fmt.Errorf("append iteration issue event: %w", err)
	}
	return q.AdvanceIterationScopeRevision(ctx, db.AdvanceIterationScopeRevisionParams{WorkspaceID: after.WorkspaceID, ID: after.CurrentIterationID})
}
