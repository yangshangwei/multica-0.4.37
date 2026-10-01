package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// PasswordTokenIdentity binds every use to the token's original issuing version.
// It intentionally never consults the legacy PAT/daemon authorization caches.
type PasswordTokenIdentity struct {
	Session     PasswordSession
	WorkspaceID string
	DaemonID    string
}

func CheckPasswordToken(ctx context.Context, q *db.Queries, raw string, allowDaemon bool) (PasswordTokenIdentity, error) {
	var identity PasswordTokenIdentity
	if strings.HasPrefix(raw, "mcn_") || strings.HasPrefix(raw, "mat_") || strings.HasPrefix(raw, "mpc_") || strings.HasPrefix(raw, "mpi_") || (!allowDaemon && strings.HasPrefix(raw, "mdt_")) {
		return identity, ErrPasswordSession
	}
	if q == nil {
		return identity, errors.New("authentication database unavailable")
	}
	switch {
	case strings.HasPrefix(raw, "mul_"):
		pat, err := q.GetPersonalAccessTokenByHash(ctx, HashToken(raw))
		if err != nil {
			return identity, passwordTokenError(err)
		}
		identity.Session = PasswordSession{UserID: util.UUIDToString(pat.UserID), Version: pat.AuthVersion, Kind: "pat"}
	case strings.HasPrefix(raw, "mdt_"):
		token, err := q.GetDaemonTokenByHash(ctx, HashToken(raw))
		if err != nil {
			return identity, passwordTokenError(err)
		}
		if !token.UserID.Valid {
			return identity, ErrPasswordSession
		}
		identity.Session = PasswordSession{UserID: util.UUIDToString(token.UserID), Version: token.AuthVersion, Kind: "daemon_token"}
		identity.WorkspaceID = util.UUIDToString(token.WorkspaceID)
		identity.DaemonID = token.DaemonID
		identity.Session.WorkspaceID = identity.WorkspaceID
		identity.Session.DaemonID = identity.DaemonID
		if token.InstallationBindingID.Valid {
			identity.Session.BindingID = util.UUIDToString(token.InstallationBindingID)
			identity.Session.BindingEpoch = token.InstallationBindingEpoch.Int64
		}
	default:
		token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return JWTSecret(), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithJSONNumber())
		if err != nil || !token.Valid {
			return identity, ErrPasswordSession
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return identity, ErrPasswordSession
		}
		identity.Session, err = CheckPasswordJWT(ctx, q, claims)
		if err != nil {
			return identity, err
		}
		if identity.Session.Setup || identity.Session.Change {
			return identity, ErrPasswordSession
		}
		email, _ := claims["email"].(string)
		if IsTemporarilyDisabledUser(identity.Session.UserID, email) {
			return identity, ErrPasswordSession
		}
	}
	if IsTemporarilyDisabledUserID(identity.Session.UserID) {
		return identity, ErrPasswordSession
	}
	if identity.Session.Kind == "jwt" {
		return identity, nil
	}
	_, err := CheckPasswordVersion(WithPasswordSession(ctx, identity.Session), q, identity.Session.UserID, identity.Session.Version)
	return identity, err
}
func passwordTokenError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPasswordSession
	}
	return err
}
