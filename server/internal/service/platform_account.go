package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type PlatformAccountChangeParams struct {
	OrganizationID      pgtype.UUID
	TargetUserID        pgtype.UUID
	IdempotencyKey      pgtype.UUID
	Action              string
	ExpectedAuthVersion int64
	Reason              string
	Password            string
	TemporaryPassword   string
	Username            string
	RequestID           string
}

func accountOperationKind(action string) (kind, code string) {
	switch action {
	case "disable":
		return "user.disable", "account_disabled"
	case "restore":
		return "user.restore", "account_restored"
	case "recover-password":
		return "user.password.recover", "password_recovered"
	default:
		return "", ""
	}
}
func accountChangeHash(p PlatformAccountChangeParams) (string, error) {
	body, err := json.Marshal(struct {
		Action   string      `json:"action"`
		Target   pgtype.UUID `json:"target"`
		Version  int64       `json:"expected_auth_version"`
		Reason   string      `json:"reason"`
		Username string      `json:"username"`
	}{p.Action, p.TargetUserID, p.ExpectedAuthVersion, strings.TrimSpace(p.Reason), p.Username})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

func (s *PlatformAdminService) ChangeAccount(ctx context.Context, p PlatformAccountChangeParams) (PlatformRoleChangeResult, error) {
	var result PlatformRoleChangeResult
	kind, code := accountOperationKind(p.Action)
	if kind == "" || !p.OrganizationID.Valid || !p.TargetUserID.Valid || !p.IdempotencyKey.Valid || p.ExpectedAuthVersion < 0 {
		return result, platformError(400, "invalid_account_change", "The account change parameters are invalid")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	actor, err := s.Authorize(ctx, true)
	if err != nil {
		return result, err
	}
	if actor.OrganizationID != p.OrganizationID {
		return result, platformError(403, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	if actor.UserID == p.TargetUserID {
		return result, platformError(409, "self_action_forbidden", "Use your personal account settings to change your own credentials")
	}
	credential, err := s.Queries.GetPasswordCredential(ctx, actor.UserID)
	if err != nil {
		return result, err
	}
	if len(p.Password) > 512 {
		return result, platformError(403, "password_verification_failed", "The password could not be verified")
	}
	verified, err := auth.VerifyPassword(ctx, credential.PasswordHash, p.Password)
	if err != nil {
		return result, err
	}
	if !verified {
		return result, platformError(403, "password_verification_failed", "The password could not be verified")
	}
	if p.Username != "" {
		normalized, err := auth.NormalizeUsername(p.Username)
		if err != nil {
			return result, platformError(400, "invalid_username", err.Error())
		}
		p.Username = normalized
	}
	digest, err := accountChangeHash(p)
	if err != nil {
		return result, err
	}
	// Replay does not need a temporary password, nor should it validate/hash one.
	// The immutable operation is checked again under the actor lock below.
	var temporaryHash string
	if p.Action == "recover-password" {
		_, lookupErr := s.Queries.GetAdminOperationByKey(ctx, db.GetAdminOperationByKeyParams{OrganizationID: p.OrganizationID, ActorID: actor.UserID, IdempotencyKey: p.IdempotencyKey})
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return result, lookupErr
		}
		if errors.Is(lookupErr, pgx.ErrNoRows) {
			if err := auth.ValidatePassword(p.TemporaryPassword); err != nil {
				return result, platformError(400, "invalid_temporary_password", err.Error())
			}
			temporaryHash, err = auth.HashPassword(ctx, p.TemporaryPassword)
			if err != nil {
				return result, err
			}
		}
	}
	if s.TxStarter == nil {
		return result, platformError(503, "admin_unavailable", "Administration is unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = LockPlatformAdminMutation(ctx, tx, actor.UserID, p.TargetUserID); err != nil {
		return result, err
	}
	q := db.New(tx)
	if _, err = authorizePlatformAdmin(ctx, q, true); err != nil {
		return result, err
	}
	currentActor, err := q.GetPasswordCredential(ctx, actor.UserID)
	if err != nil {
		return result, err
	}
	if currentActor.PasswordHash != credential.PasswordHash {
		return result, platformError(403, "password_verification_stale", "The password changed; verify it again")
	}
	op, replayed, err := s.CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{OrganizationID: p.OrganizationID, TargetKind: "user", TargetID: p.TargetUserID, Kind: kind, IdempotencyKey: p.IdempotencyKey, PayloadHash: digest, Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: code, AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}})
	if err != nil {
		return result, err
	}
	if replayed {
		return PlatformRoleChangeResult{Operation: op, TargetUserID: p.TargetUserID, Replayed: true}, nil
	}
	target, err := q.GetUser(ctx, p.TargetUserID)
	if err != nil {
		return result, err
	}
	current, err := q.GetPasswordCredential(ctx, p.TargetUserID)
	configured := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	err = nil
	if current.SessionVersion != p.ExpectedAuthVersion {
		return result, platformError(409, "version_conflict", "The account changed; refresh before retrying")
	}
	if p.Username != "" && configured && p.Username != current.Username {
		return result, platformError(400, "username_immutable", "An existing username cannot be changed")
	}
	if p.Action == "recover-password" && !configured && p.Username == "" {
		return result, platformError(400, "username_required", "An initial username is required for this account")
	}
	before := accountAuditState{Disabled: target.DisabledAt.Valid, MustChangePassword: current.MustChangePassword, AuthVersion: current.SessionVersion}
	switch p.Action {
	case "disable":
		if target.DisabledAt.Valid {
			return result, platformError(409, "state_conflict", "The account is already disabled")
		}
		err = q.SetPlatformUserDisabled(ctx, db.SetPlatformUserDisabledParams{UserID: target.ID, DisabledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, DisabledReason: pgtype.Text{String: strings.TrimSpace(p.Reason), Valid: true}})
	case "restore":
		if auth.IsTemporarilyDisabledUser(util.UUIDToString(target.ID), target.Email.String) {
			return result, platformError(409, "account_remains_disabled", "The account remains blocked by the deployment's emergency access policy")
		}
		if !target.DisabledAt.Valid {
			return result, platformError(409, "state_conflict", "The account is already active")
		}
		err = q.SetPlatformUserDisabled(ctx, db.SetPlatformUserDisabledParams{UserID: target.ID})
	}
	if err != nil {
		return result, err
	}
	var updated db.UserPasswordCredential
	if p.Action == "recover-password" {
		if temporaryHash == "" {
			return result, platformError(http.StatusServiceUnavailable, "admin_unavailable", "Recovery could not be prepared")
		}
		if err = q.RevokePlatformLegacySessions(ctx, target.ID); err != nil {
			return result, err
		}
		if configured {
			updated, err = q.ChangePasswordCredential(ctx, db.ChangePasswordCredentialParams{UserID: target.ID, PasswordHash: temporaryHash, MustChangePassword: true, SessionVersion: current.SessionVersion})
		} else {
			updated, err = q.CreatePasswordCredential(ctx, db.CreatePasswordCredentialParams{UserID: target.ID, Username: p.Username, PasswordHash: temporaryHash, MustChangePassword: true})
		}
	} else if configured {
		updated, err = q.BumpPlatformUserAuthVersion(ctx, db.BumpPlatformUserAuthVersionParams{UserID: target.ID, SessionVersion: current.SessionVersion})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return result, platformError(409, "auth_version_exhausted", "The credential version cannot be advanced")
	}
	var conflict *pgconn.PgError
	if errors.As(err, &conflict) && conflict.Code == "23505" {
		return result, platformError(409, "username_taken", "The username is already in use")
	}
	if err != nil {
		return result, err
	}
	if err = EnsureEffectiveSuperAdmin(ctx, tx); err != nil {
		return result, err
	}
	result.Revocation, err = auth.RevokePasswordCredentials(ctx, tx, target.ID)
	if err != nil {
		return PlatformRoleChangeResult{}, err
	}
	after := accountAuditState{Disabled: target.DisabledAt.Valid, MustChangePassword: updated.MustChangePassword, AuthVersion: updated.SessionVersion}
	if p.Action == "disable" {
		after.Disabled = true
	}
	if p.Action == "restore" {
		after.Disabled = false
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if err = recordRoleAudit(ctx, q, op, p.RequestID, beforeJSON, afterJSON); err != nil {
		return PlatformRoleChangeResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PlatformRoleChangeResult{}, fmt.Errorf("commit administrative account operation %s: %w", util.UUIDToString(op.ID), err)
	}
	result.Operation = op
	result.TargetUserID = target.ID
	result.CredentialsRevoked = true
	return result, nil
}

type accountAuditState struct {
	Disabled           bool  `json:"disabled"`
	MustChangePassword bool  `json:"must_change_password"`
	AuthVersion        int64 `json:"auth_version"`
}
