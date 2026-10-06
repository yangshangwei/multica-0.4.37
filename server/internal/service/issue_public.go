package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ErrIssueRevisionConflict means the caller based an update on a stale issue
// revision. Transports map it to their stable conflict response.
var ErrIssueRevisionConflict = errors.New("issue revision conflict")

// IssueContentPatch is the first shared Public API write primitive. It is
// deliberately transport- and credential-agnostic: App, PAT, and Plugin
// entrypoints authorize independently, then call the same business operation.
type IssueContentPatch struct {
	Title            *string
	Description      *string
	ExpectedRevision *int64
}

// IssueContentAuthorization revalidates the transport principal inside the
// owning transaction. It acquires applicable member/credential fences before
// returning the durable actor facts. It runs again after all issue locks to
// check time-sensitive grants; repeat calls must reuse the same principal and
// locks. It must not publish or perform execution.
type IssueContentAuthorization func(context.Context, pgx.Tx) (json.RawMessage, error)

// UpdateContent updates only the low-risk issue content fields exposed in the
// first Public API slice. Assignment, status, project, and hierarchy changes
// remain separate operations because each has additional policy and side
// effects.
func (s *IssueService) UpdateContent(ctx context.Context, issue db.Issue, patch IssueContentPatch, authorize IssueContentAuthorization) (db.Issue, error) {
	if s.TxStarter == nil || authorize == nil {
		return db.Issue{}, errors.New("content update requires transaction starter and current authorization")
	}
	operationID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.Issue{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, issue.WorkspaceID); err != nil {
		if patch.ExpectedRevision != nil && errors.Is(err, pgx.ErrNoRows) {
			return db.Issue{}, ErrIssueRevisionConflict
		}
		return db.Issue{}, err
	}
	actor, err := authorize(ctx, tx)
	if err != nil {
		return db.Issue{}, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, issue.WorkspaceID); err != nil {
		return db.Issue{}, err
	}
	if err = iteration.LockWorkspace(ctx, tx, issue.WorkspaceID); err != nil {
		return db.Issue{}, err
	}
	lockedIteration, err := iteration.LockIssueIteration(ctx, tx, issue.WorkspaceID, issue.ID)
	if err != nil {
		return db.Issue{}, err
	}
	current, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if patch.ExpectedRevision != nil && errors.Is(err, pgx.ErrNoRows) {
		return db.Issue{}, ErrIssueRevisionConflict
	}

	if err != nil {
		return db.Issue{}, err
	}
	currentActor, err := authorize(ctx, tx)
	if err != nil {
		return db.Issue{}, err
	}
	if !bytes.Equal(actor, currentActor) {
		return db.Issue{}, errors.New("issue content actor changed during authorization")
	}
	record, err := iteration.PrepareIssueRecord(ctx, tx, current, lockedIteration)
	if err != nil {
		return db.Issue{}, err
	}

	params := db.UpdateIssueContentOnlyParams{ID: issue.ID}
	if patch.ExpectedRevision != nil {
		params.ExpectedRevision = pgtype.Int8{Int64: *patch.ExpectedRevision, Valid: true}
	}
	if patch.Title != nil {
		params.Title = pgtype.Text{String: util.SanitizeTextForPostgres(*patch.Title), Valid: true}
	}
	if patch.Description != nil {
		params.Description = pgtype.Text{String: util.SanitizeTextForPostgres(*patch.Description), Valid: true}
	}

	updated, err := q.UpdateIssueContentOnly(ctx, params)
	if patch.ExpectedRevision != nil && errors.Is(err, pgx.ErrNoRows) {
		return db.Issue{}, ErrIssueRevisionConflict
	}
	if err != nil {
		return db.Issue{}, err
	}
	if err = iteration.RecordIssueChange(ctx, tx, record, updated, actor, operationID); err != nil {
		return db.Issue{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return db.Issue{}, fmt.Errorf("commit issue content update: %w", err)
	}
	return updated, nil
}
