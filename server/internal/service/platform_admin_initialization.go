package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const InitialPlatformAdminUsername = "platform_admin"

// PlatformAdminInitializationState is internal coordination data, never an HTTP
// DTO or log payload. Credential is present only while the original login is usable.
type PlatformAdminInitializationState struct {
	Status     string
	Marker     db.PlatformAdminInitialization
	Credential db.UserPasswordCredential
}

// InspectPlatformAdminInitializationInTx takes the global fence before reading
// eligibility. The caller owns the transaction and must retain it through file
// durability, creation, and commit. No KDF or file I/O belongs in this service.
func InspectPlatformAdminInitializationInTx(ctx context.Context, tx pgx.Tx, organization, deployment pgtype.UUID) (PlatformAdminInitializationState, error) {
	var result PlatformAdminInitializationState
	if !auth.PasswordMode() || os.Getenv("MULTICA_PLATFORM_ADMIN_ENABLED") != "true" {
		return result, platformError(http.StatusForbidden, "admin_mode_disabled", "Automatic initialization is disabled")
	}
	if !organization.Valid || !deployment.Valid || util.UUIDToString(deployment) != auth.ManagedDeploymentID() {
		return result, platformError(http.StatusConflict, "initialization_identity_conflict", "The deployment identity does not match")
	}
	if err := LockPlatformAdminMutation(ctx, tx); err != nil {
		return result, err
	}
	q := db.New(tx)
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return result, err
	}
	if org.ID != organization {
		return result, platformError(http.StatusConflict, "initialization_identity_conflict", "The organization identity does not match")
	}
	marker, err := q.GetPlatformAdminInitialization(ctx)
	if err == nil {
		if marker.OrganizationID != organization || marker.DeploymentID.Valid && marker.DeploymentID != deployment {
			return result, platformError(http.StatusConflict, "initialization_identity_conflict", "Initialization belongs to another organization or deployment")
		}
		result.Marker = marker
		result.Status = "preserved_existing_admin"
		if marker.State != "created" {
			return result, nil
		}
		result.Status = "already_initialized"
		// Account deletion and any later maintenance leave the marker terminal.
		if _, err := q.LockPasswordUser(ctx, marker.UserID); errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		} else if err != nil {
			return result, err
		}
		user, err := q.GetUser(ctx, marker.UserID)
		if err != nil {
			return result, err
		}
		credential, err := q.GetPasswordCredential(ctx, marker.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		if err != nil {
			return result, err
		}
		role, err := platformRole(ctx, q, marker.UserID)
		if err != nil {
			return result, err
		}
		if !user.DisabledAt.Valid && !auth.IsTemporarilyDisabledUser(util.UUIDToString(user.ID), user.Email.String) && role != nil && *role == PlatformRoleSuperAdmin && credential.MustChangePassword && credential.SessionVersion == marker.InitialCredentialVersion.Int64 && credential.Username == InitialPlatformAdminUsername {
			result.Credential = credential
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	history, err := q.HasPlatformAdminHistory(ctx)
	if err != nil {
		return result, err
	}
	if history.Bool {
		result.Status = "preserved_existing_admin"
		return result, nil
	}
	_, err = q.GetPasswordCredentialByUsername(ctx, InitialPlatformAdminUsername)
	if err == nil {
		return result, platformError(http.StatusConflict, "username_conflict", "The dedicated username is already in use")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	result.Status = "eligible"
	return result, nil
}

// PreservePlatformAdminInitializationInTx is also used by manual role grants.
// It never replaces provenance and deliberately needs no feature/deployment config.
// The caller must already hold LockPlatformAdminMutation.
func PreservePlatformAdminInitializationInTx(ctx context.Context, tx pgx.Tx, organization pgtype.UUID) (db.PlatformAdminInitialization, error) {
	q := db.New(tx)
	marker, err := q.GetPlatformAdminInitialization(ctx)
	if err == nil {
		return marker, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return marker, err
	}
	return q.CreatePlatformAdminInitialization(ctx, db.CreatePlatformAdminInitializationParams{
		OrganizationID: organization, InitializationID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, State: "preserved_existing_admin",
	})
}

type PlatformAdminInitializationParams struct {
	OrganizationID   pgtype.UUID
	DeploymentID     pgtype.UUID
	InitializationID pgtype.UUID
	PasswordHash     string
	Reason           string
	RequestID        string
}

// CreatePlatformAdminInitializationInTx may only be invoked by the deployment
// CLI after durable credential publication. Roll back on every error. This is
// the sole first-create exception to the effective-super-admin invariant; the
// new principal is intentionally restricted until a different password is set.
func CreatePlatformAdminInitializationInTx(ctx context.Context, tx pgx.Tx, p PlatformAdminInitializationParams) (db.PlatformAdminInitialization, error) {
	var empty db.PlatformAdminInitialization
	if !p.InitializationID.Valid || p.PasswordHash == "" {
		return empty, platformError(http.StatusBadRequest, "invalid_initialization", "An initialization intent and password hash are required")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return empty, err
	}
	state, err := InspectPlatformAdminInitializationInTx(ctx, tx, p.OrganizationID, p.DeploymentID)
	if err != nil {
		return empty, err
	}
	if state.Status != "eligible" {
		return empty, platformError(http.StatusConflict, "initialization_state_changed", "Initialization eligibility changed")
	}
	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{Name: "Platform Administrator"})
	if err != nil {
		return empty, err
	}
	credential, err := q.CreatePasswordCredential(ctx, db.CreatePasswordCredentialParams{UserID: user.ID, Username: InitialPlatformAdminUsername, PasswordHash: p.PasswordHash, MustChangePassword: true})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return empty, platformError(http.StatusConflict, "username_conflict", "The dedicated username is already in use")
		}
		return empty, err
	}
	if err = q.SetPlatformRole(ctx, db.SetPlatformRoleParams{UserID: user.ID, Role: PlatformRoleSuperAdmin}); err != nil {
		return empty, err
	}
	marker, err := q.CreatePlatformAdminInitialization(ctx, db.CreatePlatformAdminInitializationParams{
		OrganizationID: p.OrganizationID, DeploymentID: p.DeploymentID, InitializationID: p.InitializationID,
		State: "created", UserID: user.ID, InitialCredentialVersion: pgtype.Int8{Int64: credential.SessionVersion, Valid: true},
	})
	if err != nil {
		return empty, err
	}
	// Enumerate only public provenance; neither plaintext nor the KDF hash can
	// enter operation digests or audit snapshots.
	payload, err := json.Marshal(struct {
		InitializationID string `json:"initialization_id"`
		DeploymentID     string `json:"deployment_id"`
		Reason           string `json:"reason"`
	}{util.UUIDToString(p.InitializationID), util.UUIDToString(p.DeploymentID), strings.TrimSpace(p.Reason)})
	if err != nil {
		return empty, err
	}
	digest := sha256.Sum256(payload)
	op, err := q.CreateAdminOperation(ctx, db.CreateAdminOperationParams{
		OrganizationID: p.OrganizationID, ActorKind: "deployment_operator", TargetKind: "user", TargetID: user.ID,
		Kind: "user.role.initialize", IdempotencyKey: p.InitializationID, PayloadHash: hex.EncodeToString(digest[:]), Reason: strings.TrimSpace(p.Reason),
		State: "applied", ResultCode: "administrator_initialized", AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return empty, err
	}
	after := []byte(`{"role":"super_admin","auth_version":1,"requires_password_change":true}`)
	if err = recordRoleAudit(ctx, q, op, p.RequestID, []byte(`{}`), after); err != nil {
		return empty, err
	}
	return marker, nil
}
