package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type DeploymentPasswordRecoveryParams struct {
	TargetUserID pgtype.UUID
	Username     string
	PasswordHash string
	Reason       string
	RequestID    string
	BreakGlass   bool
}

type DeploymentPasswordRecoveryResult struct {
	Operation            db.AdminOperation
	Revocation           auth.PasswordRevocation
	EffectiveSuperAdmins int64
}

// RecoverPasswordForDeploymentInTx is a deployment-only credential recovery
// boundary. Never expose it directly through HTTP. The caller hashes a secret
// read from protected stdin before opening the transaction, and must roll back
// on any error. Recovery never grants roles or changes account disabled state.
func RecoverPasswordForDeploymentInTx(ctx context.Context, tx pgx.Tx, p DeploymentPasswordRecoveryParams) (DeploymentPasswordRecoveryResult, error) {
	var result DeploymentPasswordRecoveryResult
	if !auth.PasswordMode() {
		return result, platformError(http.StatusForbidden, "admin_mode_disabled", "Password recovery requires password authentication")
	}
	if !p.TargetUserID.Valid || p.PasswordHash == "" {
		return result, platformError(http.StatusBadRequest, "invalid_recovery", "A user and a temporary password are required")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	if err := LockPlatformAdminMutation(ctx, tx, p.TargetUserID); err != nil {
		return result, err
	}
	q := db.New(tx)
	org, err := q.EnsureInternalOrganization(ctx)
	if err != nil {
		return result, err
	}
	if org.State != "active" {
		return result, platformError(http.StatusServiceUnavailable, "admin_unavailable", "The administration organization is unavailable")
	}
	role, err := platformRole(ctx, q, p.TargetUserID)
	if err != nil {
		return result, err
	}
	isAdmin := role != nil && *role == PlatformRoleSuperAdmin
	if p.BreakGlass && !isAdmin {
		return result, platformError(http.StatusConflict, "break_glass_not_applicable", "Break-glass recovery requires an existing super administrator")
	}
	beforeCount, err := CountEffectiveSuperAdmins(ctx, q)
	if err != nil {
		return result, err
	}
	current, err := q.GetPasswordCredential(ctx, p.TargetUserID)
	before := passwordRecoveryAuditState{EffectiveSuperAdmins: beforeCount, BreakGlass: p.BreakGlass}
	var recovered db.UserPasswordCredential
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		username, normalizeErr := auth.NormalizeUsername(p.Username)
		if normalizeErr != nil {
			return result, platformError(http.StatusBadRequest, "username_required", "--username is required for an unconfigured account")
		}
		recovered, err = q.CreatePasswordCredential(ctx, db.CreatePasswordCredentialParams{UserID: p.TargetUserID, Username: username, PasswordHash: p.PasswordHash, MustChangePassword: true})
	case err != nil:
		return result, err
	default:
		if p.Username != "" && p.Username != current.Username {
			return result, platformError(http.StatusBadRequest, "username_immutable", "Recovery cannot change an existing username")
		}
		before.AuthVersion, before.MustChangePassword = current.SessionVersion, current.MustChangePassword
		recovered, err = q.ChangePasswordCredential(ctx, db.ChangePasswordCredentialParams{UserID: p.TargetUserID, PasswordHash: p.PasswordHash, MustChangePassword: true, SessionVersion: current.SessionVersion})
		if errors.Is(err, pgx.ErrNoRows) {
			return result, platformError(http.StatusConflict, "auth_version_exhausted", "The credential version cannot be advanced")
		}
	}
	if err != nil {
		return result, err
	}
	if isAdmin && !p.BreakGlass {
		if err := EnsureEffectiveSuperAdmin(ctx, tx); err != nil {
			return result, err
		}
	}
	result.EffectiveSuperAdmins, err = CountEffectiveSuperAdmins(ctx, q)
	if err != nil {
		return result, err
	}
	if p.BreakGlass && result.EffectiveSuperAdmins != 0 {
		return result, platformError(http.StatusConflict, "break_glass_not_applicable", "Another effective administrator remains; use ordinary recovery")
	}
	result.Revocation, err = auth.RevokePasswordCredentials(ctx, tx, p.TargetUserID)
	if err != nil {
		return DeploymentPasswordRecoveryResult{}, err
	}
	// Enumerate only non-secret fields. A password or its KDF output must never
	// enter an operation digest, audit, or database diagnostic payload.
	payload, err := json.Marshal(struct {
		TargetID   pgtype.UUID `json:"target_id"`
		Username   string      `json:"username"`
		Reason     string      `json:"reason"`
		BreakGlass bool        `json:"break_glass"`
	}{p.TargetUserID, p.Username, strings.TrimSpace(p.Reason), p.BreakGlass})
	if err != nil {
		return DeploymentPasswordRecoveryResult{}, err
	}
	digest := sha256.Sum256(payload)
	code := "password_recovered"
	if p.BreakGlass {
		code = "password_recovered_break_glass"
	}
	op, err := q.CreateAdminOperation(ctx, db.CreateAdminOperationParams{
		OrganizationID: org.ID, ActorKind: "deployment_operator", TargetKind: "user", TargetID: p.TargetUserID,
		Kind: "user.password.recover", IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		PayloadHash: hex.EncodeToString(digest[:]), Reason: strings.TrimSpace(p.Reason),
		State: "applied", ResultCode: code, AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return DeploymentPasswordRecoveryResult{}, err
	}
	beforeState, _ := json.Marshal(before)
	afterState, _ := json.Marshal(passwordRecoveryAuditState{AuthVersion: recovered.SessionVersion, MustChangePassword: true, EffectiveSuperAdmins: result.EffectiveSuperAdmins, BreakGlass: p.BreakGlass})
	if err := q.CreateAdminAuditEvent(ctx, db.CreateAdminAuditEventParams{
		OperationID: op.ID, OrganizationID: org.ID, ActorKind: "deployment_operator", TargetKind: "user", TargetID: p.TargetUserID,
		Action: op.Kind, Phase: "applied", RequestID: p.RequestID, Reason: op.Reason,
		BeforeState: beforeState, AfterState: afterState, ResultCode: code,
	}); err != nil {
		return DeploymentPasswordRecoveryResult{}, err
	}
	result.Operation = op
	return result, nil
}

type passwordRecoveryAuditState struct {
	AuthVersion          int64 `json:"auth_version"`
	MustChangePassword   bool  `json:"must_change_password"`
	EffectiveSuperAdmins int64 `json:"effective_super_admins"`
	BreakGlass           bool  `json:"break_glass"`
}
