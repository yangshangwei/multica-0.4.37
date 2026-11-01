package service

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// knowledgePATLockTimeout bounds the wait for the PAT row lock. It stays below
// PostgreSQL's default 1s deadlock_timeout, so contention surfaces as a
// retryable 55P03 rather than a deadlock check.
const knowledgePATLockTimeout = "250ms"

func knowledgePAT(ctx context.Context, q *db.Queries, actor pgtype.UUID, lock bool) (*time.Time, error) {
	p, ok := auth.AuthenticatedPrincipalFromContext(ctx)
	if !ok || p.UserID != util.UUIDToString(actor) {
		return nil, knowledgeFailure(403, "knowledge_permission_denied")
	}
	if p.Kind != auth.CredentialPAT {
		return nil, nil
	}
	hash, err := hex.DecodeString(p.PATTokenHash)
	if err != nil || len(hash) != 32 {
		return nil, knowledgeFailure(403, "knowledge_permission_denied")
	}
	var expires pgtype.Timestamptz
	if lock {
		row, e := lockKnowledgePAT(ctx, q, db.LockKnowledgePATParams{TokenHash: p.PATTokenHash, UserID: actor})
		err = e
		expires = row.ExpiresAt
	} else {
		row, e := q.GetKnowledgePAT(ctx, db.GetKnowledgePATParams{TokenHash: p.PATTokenHash, UserID: actor})
		err = e
		expires = row.ExpiresAt
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, knowledgeFailure(403, "knowledge_permission_denied")
	}
	if err != nil {
		return nil, err
	}
	if expires.Valid && !expires.Time.After(time.Now()) {
		return nil, knowledgeFailure(403, "knowledge_permission_denied")
	}
	return knowledgeTime(expires), nil
}

// lockKnowledgePAT waits briefly behind a concurrent row update, such as the
// middleware's last_used_at refresh, instead of failing at once. The saved
// lock_timeout is restored so later locks in the transaction keep theirs.
// On 55P03 the transaction is already aborted and rolls back as a whole.
func lockKnowledgePAT(ctx context.Context, q *db.Queries, arg db.LockKnowledgePATParams) (db.LockKnowledgePATRow, error) {
	previous, err := q.GetKnowledgeLockTimeout(ctx)
	if err != nil {
		return db.LockKnowledgePATRow{}, err
	}
	if _, err = q.SetKnowledgeLockTimeout(ctx, knowledgePATLockTimeout); err != nil {
		return db.LockKnowledgePATRow{}, err
	}
	row, err := q.LockKnowledgePAT(ctx, arg)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return row, err
	}
	if _, restoreErr := q.SetKnowledgeLockTimeout(ctx, previous); restoreErr != nil {
		return row, restoreErr
	}
	return row, err
}
