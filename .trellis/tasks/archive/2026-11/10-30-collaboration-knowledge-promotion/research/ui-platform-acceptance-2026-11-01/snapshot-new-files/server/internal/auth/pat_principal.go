package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ValidateAuthenticatedPAT rechecks the original credential inside a caller's
// authorized read snapshot. JWT behavior is unchanged; a cached PAT principal
// alone is insufficient for private content after token revocation/expiry.
func ValidateAuthenticatedPAT(ctx context.Context, q *db.Queries) error {
	p, ok := AuthenticatedPrincipalFromContext(ctx)
	if !ok {
		return ErrPasswordSession
	}
	if p.Kind != CredentialPAT {
		return nil
	}
	hash, err := hex.DecodeString(p.PATTokenHash)
	if err != nil || len(hash) != 32 {
		return ErrPasswordSession
	}
	if q == nil {
		return errors.New("authentication database unavailable")
	}
	pat, err := q.GetPersonalAccessTokenByHash(ctx, p.PATTokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPasswordSession
	}
	if err != nil {
		return err
	}
	if util.UUIDToString(pat.UserID) != p.UserID || pat.ExpiresAt.Valid && !pat.ExpiresAt.Time.After(time.Now()) {
		return ErrPasswordSession
	}
	if PasswordMode() {
		session, ok := PasswordSessionFromContext(ctx)
		if !ok || session.UserID != p.UserID || session.Kind != "pat" || session.Version != pat.AuthVersion {
			return ErrPasswordSession
		}
	}
	return nil
}
