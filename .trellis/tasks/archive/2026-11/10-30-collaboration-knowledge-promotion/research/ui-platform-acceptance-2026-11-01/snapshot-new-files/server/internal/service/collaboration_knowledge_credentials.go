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
		row, e := q.LockKnowledgePAT(ctx, db.LockKnowledgePATParams{TokenHash: p.PATTokenHash, UserID: actor})
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
