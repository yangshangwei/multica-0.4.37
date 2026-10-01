package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrInstallationMetadata = errors.New("invalid installation metadata proof")

type installationMetadataClaims struct {
	Kind           string `json:"kind"`
	DeploymentID   string `json:"deployment_id"`
	InstallationID string `json:"installation_id"`
	UserID         string `json:"user_id"`
	AuthVersion    string `json:"auth_version"`
	KeyVersion     string `json:"key_version"`
	jwt.RegisteredClaims
}

func installationMetadataKey() []byte {
	mac := hmac.New(sha256.New, JWTSecret())
	mac.Write([]byte("multica-installation-metadata-v1"))
	return mac.Sum(nil)
}

// This proof conveys submission attribution only. It deliberately has neither
// the login signing key nor a human subject claim and is never authentication.
func MintInstallationMetadataProof(session PasswordSession, inst db.ManagedInstallation, now time.Time) (string, error) {
	if session.Kind != "jwt" || session.Setup || session.Change || session.Version <= 0 || !inst.ID.Valid || !inst.DeploymentID.Valid || inst.KeyVersion <= 0 {
		return "", ErrInstallationMetadata
	}
	if _, err := util.ParseUUID(session.UserID); err != nil {
		return "", ErrInstallationMetadata
	}
	claims := installationMetadataClaims{Kind: "installation_metadata", DeploymentID: util.UUIDToString(inst.DeploymentID), InstallationID: util.UUIDToString(inst.ID), UserID: session.UserID, AuthVersion: strconv.FormatInt(session.Version, 10), KeyVersion: strconv.FormatInt(inst.KeyVersion, 10), RegisteredClaims: jwt.RegisteredClaims{Issuer: "multica-installation", Audience: jwt.ClaimStrings{"submission"}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(installationMetadataKey())
	if err != nil {
		return "", err
	}
	return "mip_" + token, nil
}

func ValidateInstallationMetadata(ctx context.Context, q *db.Queries, proof string, session PasswordSession) (pgtype.UUID, error) {
	var empty pgtype.UUID
	if len(proof) > 4096 || !strings.HasPrefix(proof, "mip_") || session.Kind != "jwt" && session.Kind != "pat" || session.Setup || session.Change || session.Version <= 0 {
		return empty, ErrInstallationMetadata
	}
	var claims installationMetadataClaims
	token, err := jwt.ParseWithClaims(proof[4:], &claims, func(*jwt.Token) (any, error) { return installationMetadataKey(), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithIssuer("multica-installation"), jwt.WithAudience("submission"))
	if err != nil || !token.Valid || claims.Kind != "installation_metadata" || claims.UserID != session.UserID || claims.AuthVersion != strconv.FormatInt(session.Version, 10) || claims.DeploymentID == "" || claims.DeploymentID != ManagedDeploymentID() {
		return empty, ErrInstallationMetadata
	}
	id, err := util.ParseUUID(claims.InstallationID)
	if err != nil {
		return empty, ErrInstallationMetadata
	}
	if q == nil {
		return empty, errors.New("installation metadata database unavailable")
	}
	if _, err = CheckPasswordVersion(ctx, q, session.UserID, session.Version); err != nil {
		return empty, err
	}
	inst, err := q.GetManagedInstallation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrInstallationMetadata
	}
	if err != nil {
		return empty, err
	}
	if inst.Lifecycle != "active" || claims.DeploymentID != util.UUIDToString(inst.DeploymentID) || claims.KeyVersion != strconv.FormatInt(inst.KeyVersion, 10) {
		return empty, ErrInstallationMetadata
	}
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return empty, errors.New("installation metadata organization unavailable")
	}
	if inst.OrganizationID != org.ID {
		return empty, ErrInstallationMetadata
	}
	return id, nil
}

type submissionInstallationKey struct{}

func WithSubmissionInstallation(ctx context.Context, id pgtype.UUID) context.Context {
	return context.WithValue(ctx, submissionInstallationKey{}, id)
}

func SubmissionInstallationFromContext(ctx context.Context) pgtype.UUID {
	id, _ := ctx.Value(submissionInstallationKey{}).(pgtype.UUID)
	return id
}
