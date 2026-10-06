package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// IssueStatusWritePolicy is evaluated against the freshly locked issue. Queries
// use separate READ COMMITTED statements after the issue FOR UPDATE lock, so an
// enqueue that finished while the lock was waiting is visible to the policy.
type IssueStatusWritePolicy func(context.Context, *db.Queries, db.Issue) (bool, error)

// WriteIssueStatus owns one transaction without a retry or autocommit fallback.
// FOR UPDATE is intentional: execution admission holds KEY SHARE, which would
// not conflict with NO KEY UPDATE. Callers retain their existing retry budget
// and may broadcast or notify only after changed=true is returned.
func WriteIssueStatus(ctx context.Context, starter TxStarter, queries *db.Queries, issue db.Issue, target, source string, policy IssueStatusWritePolicy) (before, after db.Issue, changed bool, err error) {
	if starter == nil || queries == nil || policy == nil || strings.TrimSpace(source) == "" || !issuestatus.IsBuiltIn(target) {
		return before, after, false, errors.New("status write requires transaction, policy, source and canonical target")
	}
	operationID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	actor, err := json.Marshal(map[string]any{"type": "system", "id": nil, "user_id": nil, "source": source})
	if err != nil {
		return before, after, false, err
	}
	tx, err := starter.Begin(ctx)
	if err != nil {
		return before, after, false, err
	}
	defer tx.Rollback(ctx)
	// These former autocommit writers own a new RC transaction. A deployment's
	// default isolation must not make the post-lock active-task read stale.
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return before, after, false, err
	}
	q := queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, issue.WorkspaceID); err != nil {
		return before, after, false, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, issue.WorkspaceID); err != nil {
		return before, after, false, err
	}
	if err = iteration.LockWorkspace(ctx, tx, issue.WorkspaceID); err != nil {
		return before, after, false, err
	}
	lockedIteration, err := iteration.LockIssueIteration(ctx, tx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return before, after, false, err
	}
	before, err = q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return before, after, false, err
	}
	allowed, err := policy(ctx, q, before)
	if err != nil {
		return before, after, false, err
	}
	if !allowed || before.Status == target {
		return before, before, false, nil
	}
	record, err := iteration.PrepareIssueRecord(ctx, tx, before, lockedIteration)
	if err != nil {
		return before, after, false, err
	}
	after, err = q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: target})
	if err != nil {
		return before, after, false, err
	}
	if err = iteration.RecordIssueChange(ctx, tx, record, after, actor, operationID); err != nil {
		return before, after, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return before, after, false, fmt.Errorf("commit issue status write: %w", err)
	}
	return before, after, true, nil
}

// IssueStatusCategory resolves persisted categories, including archived custom
// entries; it never guesses a category if the catalog is missing or invalid.
func IssueStatusCategory(ctx context.Context, q *db.Queries, issue db.Issue) (string, error) {
	if issuestatus.IsBuiltIn(issue.Status) {
		return issue.Status, nil
	}
	entry, err := q.GetIssueStatusEntryByKey(ctx, db.GetIssueStatusEntryByKeyParams{WorkspaceID: issue.WorkspaceID, Key: issue.Status})
	if err != nil {
		return "", err
	}
	if !issuestatus.IsCategory(entry.Category) {
		return "", fmt.Errorf("unknown issue status category %q", entry.Category)
	}
	return entry.Category, nil
}

func resetFailedIssuePolicy(ctx context.Context, q *db.Queries, issue db.Issue) (bool, error) {
	category, err := IssueStatusCategory(ctx, q, issue)
	if err != nil {
		return false, err
	}
	if category != "in_progress" {
		return false, nil
	}
	active, err := q.HasActiveTaskForIssue(ctx, issue.ID)
	return !active, err
}
