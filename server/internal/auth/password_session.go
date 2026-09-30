package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type PasswordSession struct {
	UserID  string
	Version int64
	Kind    string
	Setup   bool
	Change  bool
}
type passwordSessionKey struct{}

func WithPasswordSession(ctx context.Context, s PasswordSession) context.Context {
	return context.WithValue(ctx, passwordSessionKey{}, s)
}

func PasswordSessionFromContext(ctx context.Context) (PasswordSession, bool) {
	s, ok := ctx.Value(passwordSessionKey{}).(PasswordSession)
	return s, ok
}

var ErrPasswordSession = errors.New("session is no longer valid")

func ClaimVersion(claims jwt.MapClaims) int64 {
	switch v := claims["auth_version"].(type) {
	case json.Number:
		n, e := strconv.ParseInt(string(v), 10, 64)
		if e == nil && n > 0 {
			return n
		}
	case float64:
		if v > 0 && v < 9223372036854775807 && v == float64(int64(v)) {
			return int64(v)
		}
	}
	return 0
}

func CheckPasswordVersion(ctx context.Context, q *db.Queries, userID string, version int64) (db.UserPasswordCredential, error) {
	if q == nil {
		return db.UserPasswordCredential{}, errors.New("authentication database unavailable")
	}
	id, err := util.ParseUUID(userID)
	if err != nil {
		return db.UserPasswordCredential{}, ErrPasswordSession
	}
	c, err := q.GetPasswordCredential(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrPasswordSession
	}
	if err != nil {
		return c, err
	}
	if version <= 0 || c.SessionVersion != version || c.MustChangePassword {
		return c, ErrPasswordSession
	}
	return c, nil
}

func CheckPasswordJWT(ctx context.Context, q *db.Queries, claims jwt.MapClaims) (PasswordSession, error) {
	id, _ := claims["sub"].(string)
	s := PasswordSession{UserID: id, Version: ClaimVersion(claims), Kind: "jwt"}
	uid, err := util.ParseUUID(id)
	if err != nil {
		return s, ErrPasswordSession
	}
	if q == nil {
		return s, errors.New("authentication database unavailable")
	}
	c, err := q.GetPasswordCredential(ctx, uid)
	if err == nil {
		if s.Version <= 0 || s.Version != c.SessionVersion {
			return s, ErrPasswordSession
		}
		s.Change = c.MustChangePassword
		return s, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	if _, err = q.GetUser(ctx, uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s, ErrPasswordSession
		}
		return s, err
	}
	cutoff, deadline, err := PasswordMigrationWindow()
	if err != nil {
		return s, err
	}
	issued, e1 := claims.GetIssuedAt()
	expiry, e2 := claims.GetExpirationTime()
	now := time.Now()
	if cutoff.IsZero() || e1 != nil || e2 != nil || issued == nil || expiry == nil || issued.Time.After(now) || !issued.Time.Before(cutoff) || !now.Before(expiry.Time) || !now.Before(deadline) || s.Version != 0 {
		return s, ErrPasswordSession
	}
	s.Setup = true
	return s, nil
}

func (s PasswordSession) Allows(method, path string) bool {
	if !s.Setup && !s.Change {
		return true
	}
	if method == "GET" && path == "/api/me" || method == "POST" && path == "/auth/logout" {
		return true
	}
	return method == "POST" && (s.Setup && path == "/api/me/password/setup" || s.Change && path == "/api/me/password/change")
}

// LockPasswordSession fences minting against concurrent password changes. Call
// inside the transaction that persists the new credential, using its queries.
func LockPasswordSession(ctx context.Context, q *db.Queries) (PasswordSession, error) {
	s, ok := PasswordSessionFromContext(ctx)
	if !ok || s.Setup || s.Change || s.Version <= 0 {
		return s, ErrPasswordSession
	}
	id, err := util.ParseUUID(s.UserID)
	if err != nil {
		return s, ErrPasswordSession
	}
	if _, err = q.LockPasswordUser(ctx, id); err != nil {
		return s, err
	}
	_, err = CheckPasswordVersion(ctx, q, s.UserID, s.Version)
	return s, err
}
