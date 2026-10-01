package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	PlatformRoleSuperAdmin = "super_admin"
	PlatformRoleObserver   = "platform_observer"
)

type PlatformAdminError struct {
	Status  int
	Code    string
	Message string
}

func (e *PlatformAdminError) Error() string { return e.Message }
func platformError(status int, code, message string) error {
	return &PlatformAdminError{Status: status, Code: code, Message: message}
}

type PlatformAdminIdentity struct {
	UserID         pgtype.UUID
	OrganizationID pgtype.UUID
	Role           string
	AllowedActions []string
	AuthVersion    int64
}

type PlatformAdminService struct {
	Queries   *db.Queries
	TxStarter TxStarter
}

func NewPlatformAdminService(q *db.Queries, tx TxStarter) *PlatformAdminService {
	return &PlatformAdminService{Queries: q, TxStarter: tx}
}

// Authorize always resolves current account and role state. A role claim or
// personal access token is never an administrative authorization source.
func (s *PlatformAdminService) Authorize(ctx context.Context, write bool) (PlatformAdminIdentity, error) {
	return authorizePlatformAdmin(ctx, s.Queries, write)
}
func authorizePlatformAdmin(ctx context.Context, q *db.Queries, write bool) (PlatformAdminIdentity, error) {
	var identity PlatformAdminIdentity
	if !auth.PasswordMode() {
		return identity, platformError(http.StatusForbidden, "admin_mode_disabled", "Platform administration requires password authentication")
	}
	session, ok := auth.PasswordSessionFromContext(ctx)
	if !ok {
		return identity, auth.ErrPasswordSession
	}
	if session.Kind != "jwt" || session.Setup || session.Change || session.Version <= 0 {
		return identity, platformError(http.StatusForbidden, "admin_session_required", "A complete password login is required")
	}
	id, err := util.ParseUUID(session.UserID)
	if err != nil {
		return identity, auth.ErrPasswordSession
	}
	if q == nil {
		return identity, platformError(http.StatusServiceUnavailable, "admin_unavailable", "Administration is unavailable")
	}
	account, err := q.GetPlatformAdminAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity, auth.ErrPasswordSession
	}
	if err != nil {
		return identity, err
	}
	if account.DisabledAt.Valid || account.MustChangePassword || account.SessionVersion != session.Version {
		return identity, auth.ErrPasswordSession
	}
	if auth.IsTemporarilyDisabledUser(session.UserID, account.Email.String) {
		return identity, platformError(http.StatusForbidden, "account_disabled", auth.TemporarilyDisabledUserError)
	}
	role := account.Role.String
	if role != PlatformRoleSuperAdmin && role != PlatformRoleObserver || write && role != PlatformRoleSuperAdmin {
		return identity, platformError(http.StatusForbidden, "admin_forbidden", "The platform role does not allow this action")
	}
	org, err := q.GetInternalOrganization(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity, platformError(http.StatusServiceUnavailable, "admin_unavailable", "The administration organization is unavailable")
	}
	if err != nil {
		return identity, err
	}
	if org.State != "active" {
		return identity, platformError(http.StatusServiceUnavailable, "admin_unavailable", "The administration organization is unavailable")
	}
	actions := []string{"platform.read"}
	if role == PlatformRoleSuperAdmin {
		actions = append(actions, "users.role", "users.disable", "users.restore", "users.recover-password")
	}
	return PlatformAdminIdentity{UserID: id, OrganizationID: org.ID, Role: role, AllowedActions: actions, AuthVersion: session.Version}, nil
}

// LockPlatformAdminMutation establishes the shared role/account lock order.
// Password-only flows take the user lock but never acquire the advisory lock.
func LockPlatformAdminMutation(ctx context.Context, tx pgx.Tx, userIDs ...pgtype.UUID) error {
	q := db.New(tx)
	if err := q.LockPlatformAdminChanges(ctx); err != nil {
		return err
	}
	ids := append([]pgtype.UUID(nil), userIDs...)
	sort.Slice(ids, func(i, j int) bool {
		return strings.Compare(uuid.UUID(ids[i].Bytes).String(), uuid.UUID(ids[j].Bytes).String()) < 0
	})
	var previous pgtype.UUID
	for _, id := range ids {
		if !id.Valid {
			return platformError(http.StatusBadRequest, "invalid_user_id", "A valid user ID is required")
		}
		if previous.Valid && id.Bytes == previous.Bytes {
			continue
		}
		if _, err := q.LockPasswordUser(ctx, id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return platformError(http.StatusNotFound, "user_not_found", "The user does not exist")
			}
			return err
		}
		previous = id
	}
	return nil
}

// CountEffectiveSuperAdmins mirrors all account availability requirements,
// including the emergency denylist enforced at password login. Keeping this
// filter in the service avoids duplicating the denylist in SQL.
func CountEffectiveSuperAdmins(ctx context.Context, q *db.Queries) (int64, error) {
	accounts, err := q.ListEffectiveSuperAdminCandidates(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	for _, account := range accounts {
		if !auth.IsTemporarilyDisabledUser(util.UUIDToString(account.ID), account.Email.String) {
			count++
		}
	}
	return count, nil
}

// EnsureEffectiveSuperAdmin must run inside the shared administrative lock,
// after the proposed mutation and before committing its audit.
func EnsureEffectiveSuperAdmin(ctx context.Context, tx pgx.Tx) error {
	count, err := CountEffectiveSuperAdmins(ctx, db.New(tx))
	if err != nil {
		return err
	}
	if count == 0 {
		return platformError(http.StatusConflict, "last_super_admin", "At least one active super administrator is required")
	}
	return nil
}

type PlatformRoleChangeParams struct {
	OrganizationID      pgtype.UUID
	TargetUserID        pgtype.UUID
	IdempotencyKey      pgtype.UUID
	Role                *string
	ExpectedRole        *string
	ExpectedAuthVersion int64
	Reason              string
	Password            string
	RequestID           string
}

type PlatformRoleChangeResult struct {
	Operation          db.AdminOperation
	TargetUserID       pgtype.UUID
	Revocation         auth.PasswordRevocation
	CredentialsRevoked bool
	Replayed           bool
}

// ChangeRole verifies the password outside locks and repeats all mutable
// authorization checks under locks. Only the caller publishes revocations,
// after this method has successfully committed.
func (s *PlatformAdminService) ChangeRole(ctx context.Context, p PlatformRoleChangeParams) (PlatformRoleChangeResult, error) {
	var result PlatformRoleChangeResult
	if err := validateRoleChange(p); err != nil {
		return result, err
	}
	actor, err := s.Authorize(ctx, true)
	if err != nil {
		return result, err
	}
	if actor.OrganizationID != p.OrganizationID {
		return result, platformError(http.StatusForbidden, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	credential, err := s.Queries.GetPasswordCredential(ctx, actor.UserID)
	if err != nil {
		return result, err
	}
	if len(p.Password) > 512 {
		return result, platformError(http.StatusForbidden, "password_verification_failed", "The password could not be verified")
	}
	verified, err := auth.VerifyPassword(ctx, credential.PasswordHash, p.Password)
	if err != nil {
		return result, err
	}
	if !verified {
		return result, platformError(http.StatusForbidden, "password_verification_failed", "The password could not be verified")
	}
	if s.TxStarter == nil {
		return result, platformError(http.StatusServiceUnavailable, "admin_unavailable", "Administration is unavailable")
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
	currentActor, err := authorizePlatformAdmin(ctx, q, true)
	if err != nil {
		return result, err
	}
	currentCredential, err := q.GetPasswordCredential(ctx, actor.UserID)
	if err != nil {
		return result, err
	}
	if currentCredential.PasswordHash != credential.PasswordHash {
		return result, platformError(http.StatusForbidden, "password_verification_stale", "The password changed; verify it again")
	}
	if currentActor.OrganizationID != p.OrganizationID {
		return result, platformError(http.StatusForbidden, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	hash, err := roleChangeHash(p)
	if err != nil {
		return result, err
	}
	op, found, err := s.CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{
		OrganizationID: p.OrganizationID, TargetKind: "user", TargetID: p.TargetUserID,
		Kind: "user.role", IdempotencyKey: p.IdempotencyKey, PayloadHash: hash,
		Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: "role_changed",
		AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return result, err
	}
	if found {
		return PlatformRoleChangeResult{Operation: op, TargetUserID: p.TargetUserID, Replayed: true}, nil
	}
	target, err := q.GetUser(ctx, p.TargetUserID)
	if err != nil {
		return result, err
	}
	targetCredential, err := q.GetPasswordCredential(ctx, p.TargetUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, platformError(http.StatusConflict, "account_not_ready", "The user must complete password setup first")
	}
	if err != nil {
		return result, err
	}
	previousRole, err := platformRole(ctx, q, p.TargetUserID)
	if err != nil {
		return result, err
	}
	if !sameRole(previousRole, p.ExpectedRole) || targetCredential.SessionVersion != p.ExpectedAuthVersion {
		return result, platformError(http.StatusConflict, "version_conflict", "The user or role changed; refresh before retrying")
	}
	if p.Role != nil && (target.DisabledAt.Valid || targetCredential.MustChangePassword || auth.IsTemporarilyDisabledUser(util.UUIDToString(target.ID), target.Email.String)) {
		return result, platformError(http.StatusConflict, "account_not_ready", "The user must be active and have completed password setup")
	}
	if p.Role == nil {
		err = q.DeletePlatformRole(ctx, p.TargetUserID)
	} else {
		err = q.SetPlatformRole(ctx, db.SetPlatformRoleParams{UserID: p.TargetUserID, Role: *p.Role, GrantedBy: actor.UserID})
	}
	if err != nil {
		return result, err
	}
	if err = EnsureEffectiveSuperAdmin(ctx, tx); err != nil {
		return result, err
	}
	version := targetCredential.SessionVersion
	elevated := p.Role != nil && (previousRole == nil || *previousRole == PlatformRoleObserver && *p.Role == PlatformRoleSuperAdmin)
	if elevated {
		bumped, err := q.BumpPlatformUserAuthVersion(ctx, db.BumpPlatformUserAuthVersionParams{UserID: p.TargetUserID, SessionVersion: version})
		if errors.Is(err, pgx.ErrNoRows) {
			return result, platformError(http.StatusConflict, "auth_version_exhausted", "The credential version cannot be advanced")
		}
		if err != nil {
			return result, err
		}
		version = bumped.SessionVersion
		result.Revocation, err = auth.RevokePasswordCredentials(ctx, tx, p.TargetUserID)
		if err != nil {
			return PlatformRoleChangeResult{}, err
		}
		result.CredentialsRevoked = true
	}
	before, _ := json.Marshal(roleAuditState{Role: previousRole, AuthVersion: targetCredential.SessionVersion})
	after, _ := json.Marshal(roleAuditState{Role: p.Role, AuthVersion: version})
	if err = recordRoleAudit(ctx, q, op, p.RequestID, before, after); err != nil {
		return PlatformRoleChangeResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PlatformRoleChangeResult{}, fmt.Errorf("commit administrative role operation %s: %w", uuid.UUID(op.ID.Bytes).String(), err)
	}
	result.Operation = op
	result.TargetUserID = p.TargetUserID
	return result, nil
}

type roleAuditState struct {
	Role        *string `json:"role"`
	AuthVersion int64   `json:"auth_version"`
}

func recordRoleAudit(ctx context.Context, q *db.Queries, op db.AdminOperation, requestID string, before, after []byte) error {
	return q.CreateAdminAuditEvent(ctx, db.CreateAdminAuditEventParams{
		OperationID: op.ID, OrganizationID: op.OrganizationID, ActorKind: op.ActorKind, ActorUserID: op.ActorID,
		TargetKind: op.TargetKind, TargetID: op.TargetID, Action: op.Kind, Phase: "applied", RequestID: requestID,
		Reason: op.Reason, BeforeState: before, AfterState: after, ResultCode: op.ResultCode,
	})
}
func platformRole(ctx context.Context, q *db.Queries, id pgtype.UUID) (*string, error) {
	role, err := q.GetPlatformRole(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}
func sameRole(a, b *string) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func validRole(role *string) bool {
	return role == nil || *role == PlatformRoleSuperAdmin || *role == PlatformRoleObserver
}
func validateRoleChange(p PlatformRoleChangeParams) error {
	if !p.OrganizationID.Valid || !p.TargetUserID.Valid || !p.IdempotencyKey.Valid || p.ExpectedAuthVersion <= 0 || !validRole(p.Role) || !validRole(p.ExpectedRole) {
		return platformError(http.StatusBadRequest, "invalid_role_change", "The role change parameters are invalid")
	}
	return validateAdminReason(p.Reason, p.RequestID)
}
func validateAdminReason(reason, requestID string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 1000 || !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) || requestID == "" || len(requestID) > 128 {
		return platformError(http.StatusBadRequest, "invalid_reason", "A reason and request ID are required")
	}
	return nil
}

// roleChangeHash deliberately enumerates non-secret fields. Never marshal the
// request struct: it contains the password used only for live verification.
func roleChangeHash(p PlatformRoleChangeParams) (string, error) {
	payload, err := json.Marshal(struct {
		Kind                string      `json:"kind"`
		TargetID            pgtype.UUID `json:"target_id"`
		Role                *string     `json:"role"`
		ExpectedRole        *string     `json:"expected_role"`
		ExpectedAuthVersion int64       `json:"expected_auth_version"`
		Reason              string      `json:"reason"`
	}{"user.role", p.TargetUserID, p.Role, p.ExpectedRole, p.ExpectedAuthVersion, strings.TrimSpace(p.Reason)})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// CreateOperationInTx is the common idempotency boundary for platform writes.
// Call it before modifying the actor's role. Role/account mutations must first
// take LockPlatformAdminMutation; other writes call this before target locks.
// The action owns its non-secret hash allowlist, business changes and audit,
// all in this same transaction. Commit errors must be reconciled by key.
func (s *PlatformAdminService) CreateOperationInTx(ctx context.Context, tx pgx.Tx, p db.CreateAdminOperationParams) (db.AdminOperation, bool, error) {
	var empty db.AdminOperation
	session, ok := auth.PasswordSessionFromContext(ctx)
	if !ok || session.Kind != "jwt" || session.Setup || session.Change || session.Version <= 0 {
		return empty, false, platformError(http.StatusForbidden, "admin_session_required", "A complete password login is required")
	}
	actorID, err := util.ParseUUID(session.UserID)
	if err != nil {
		return empty, false, auth.ErrPasswordSession
	}
	q := db.New(tx)
	if _, err = q.LockPasswordUser(ctx, actorID); err != nil {
		return empty, false, err
	}
	actor, err := authorizePlatformAdmin(ctx, q, true)
	if err != nil {
		return empty, false, err
	}
	if actor.OrganizationID != p.OrganizationID {
		return empty, false, platformError(http.StatusForbidden, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	decoded, err := hex.DecodeString(p.PayloadHash)
	if !p.IdempotencyKey.Valid || !p.TargetID.Valid || p.TargetKind == "" || p.Kind == "" || err != nil || len(decoded) != sha256.Size {
		return empty, false, platformError(http.StatusBadRequest, "invalid_operation", "The operation parameters are invalid")
	}
	if err = validateAdminReason(p.Reason, "operation"); err != nil {
		return empty, false, err
	}
	op, err := q.GetAdminOperationByKey(ctx, db.GetAdminOperationByKeyParams{OrganizationID: actor.OrganizationID, ActorID: actor.UserID, IdempotencyKey: p.IdempotencyKey})
	if err == nil {
		if op.PayloadHash != p.PayloadHash || op.Kind != p.Kind || op.TargetKind != p.TargetKind || op.TargetID != p.TargetID {
			return empty, false, platformError(http.StatusConflict, "idempotency_conflict", "The idempotency key was already used for a different request")
		}
		return op, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, false, err
	}
	// Each new administrative action must register its controlled target and
	// state contract here alongside the service that implements that action.
	expectedCode := map[string]string{"user.role": "role_changed", "user.disable": "account_disabled", "user.restore": "account_restored", "user.password.recover": "password_recovered"}[p.Kind]
	if expectedCode == "" || p.TargetKind != "user" || p.State != "applied" || p.ResultCode != expectedCode {
		return empty, false, platformError(http.StatusBadRequest, "invalid_operation", "The operation kind or state is not supported")
	}
	p.ActorID, p.ActorKind, p.ActorAuthVersion = actor.UserID, "user", actor.AuthVersion
	op, err = q.CreateAdminOperation(ctx, p)
	return op, false, err
}

// FindOperationByKey supports recovery after a response was lost. Even a
// super administrator can recover only their own operation in this scope.
func (s *PlatformAdminService) FindOperationByKey(ctx context.Context, organization, key pgtype.UUID) (db.AdminOperation, error) {
	actor, err := s.Authorize(ctx, false)
	if err != nil {
		return db.AdminOperation{}, err
	}
	if !key.Valid {
		return db.AdminOperation{}, platformError(http.StatusBadRequest, "invalid_idempotency_key", "A valid idempotency key is required")
	}
	if actor.OrganizationID != organization {
		return db.AdminOperation{}, platformError(http.StatusForbidden, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	return s.Queries.GetAdminOperationByKey(ctx, db.GetAdminOperationByKeyParams{OrganizationID: organization, ActorID: actor.UserID, IdempotencyKey: key})
}
func (s *PlatformAdminService) GetOperation(ctx context.Context, organization, id pgtype.UUID) (db.AdminOperation, error) {
	actor, err := s.Authorize(ctx, false)
	if err != nil {
		return db.AdminOperation{}, err
	}
	if !id.Valid {
		return db.AdminOperation{}, platformError(http.StatusBadRequest, "invalid_operation_id", "A valid operation ID is required")
	}
	if actor.OrganizationID != organization {
		return db.AdminOperation{}, platformError(http.StatusForbidden, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	return s.Queries.GetAdminOperationForActor(ctx, db.GetAdminOperationForActorParams{OrganizationID: organization, ActorID: actor.UserID, ID: id})
}

type PlatformAdminBootstrapParams struct {
	OrganizationID pgtype.UUID
	TargetUserID   pgtype.UUID
	Reason         string
	RequestID      string
}

// Bootstrap is deployment-only; it must not be exposed as an HTTP handler.
// It requires an existing ready account and never creates a password or user.
func (s *PlatformAdminService) Bootstrap(ctx context.Context, p PlatformAdminBootstrapParams) (PlatformRoleChangeResult, error) {
	var result PlatformRoleChangeResult
	if !auth.PasswordMode() {
		return result, platformError(http.StatusForbidden, "admin_mode_disabled", "Platform administration requires password authentication")
	}
	if !p.OrganizationID.Valid || !p.TargetUserID.Valid {
		return result, platformError(http.StatusBadRequest, "invalid_user_id", "Valid organization and user IDs are required")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	if s.TxStarter == nil {
		return result, platformError(http.StatusServiceUnavailable, "admin_unavailable", "Administration is unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if err = LockPlatformAdminMutation(ctx, tx, p.TargetUserID); err != nil {
		return result, err
	}
	q := db.New(tx)
	org, err := q.GetInternalOrganization(ctx)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && org.State != "active" {
		return result, platformError(http.StatusServiceUnavailable, "admin_unavailable", "The administration organization is unavailable")
	}
	if err != nil {
		return result, err
	}
	if org.ID != p.OrganizationID {
		return result, platformError(http.StatusForbidden, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	count, err := CountEffectiveSuperAdmins(ctx, q)
	if err != nil {
		return result, err
	}
	if count != 0 {
		return result, platformError(http.StatusConflict, "admin_already_initialized", "An effective super administrator already exists")
	}
	target, err := q.GetUser(ctx, p.TargetUserID)
	if err != nil {
		return result, err
	}
	credential, err := q.GetPasswordCredential(ctx, p.TargetUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, platformError(http.StatusConflict, "account_not_ready", "The user must complete password setup first")
	}
	if err != nil {
		return result, err
	}
	if target.DisabledAt.Valid || credential.MustChangePassword || credential.SessionVersion <= 0 || auth.IsTemporarilyDisabledUser(util.UUIDToString(target.ID), target.Email.String) {
		return result, platformError(http.StatusConflict, "account_not_ready", "The user must be active and have completed password setup")
	}
	beforeRole, err := platformRole(ctx, q, p.TargetUserID)
	if err != nil {
		return result, err
	}
	credential, err = q.BumpPlatformUserAuthVersion(ctx, db.BumpPlatformUserAuthVersionParams{UserID: p.TargetUserID, SessionVersion: credential.SessionVersion})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, platformError(http.StatusConflict, "auth_version_exhausted", "The credential version cannot be advanced")
	}
	if err != nil {
		return result, err
	}
	if err = q.SetPlatformRole(ctx, db.SetPlatformRoleParams{UserID: p.TargetUserID, Role: PlatformRoleSuperAdmin}); err != nil {
		return result, err
	}
	result.Revocation, err = auth.RevokePasswordCredentials(ctx, tx, p.TargetUserID)
	if err != nil {
		return PlatformRoleChangeResult{}, err
	}
	role := PlatformRoleSuperAdmin
	hash, err := roleChangeHash(PlatformRoleChangeParams{TargetUserID: p.TargetUserID, Role: &role, ExpectedRole: beforeRole, ExpectedAuthVersion: credential.SessionVersion - 1, Reason: p.Reason})
	if err != nil {
		return PlatformRoleChangeResult{}, err
	}
	op, err := q.CreateAdminOperation(ctx, db.CreateAdminOperationParams{
		OrganizationID: p.OrganizationID, ActorKind: "deployment_operator", TargetKind: "user", TargetID: p.TargetUserID,
		Kind: "user.role.bootstrap", IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, PayloadHash: hash,
		Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: "role_changed", AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return PlatformRoleChangeResult{}, err
	}
	before, _ := json.Marshal(roleAuditState{Role: beforeRole, AuthVersion: credential.SessionVersion - 1})
	after, _ := json.Marshal(roleAuditState{Role: &role, AuthVersion: credential.SessionVersion})
	if err = recordRoleAudit(ctx, q, op, p.RequestID, before, after); err != nil {
		return PlatformRoleChangeResult{}, err
	}
	if err = EnsureEffectiveSuperAdmin(ctx, tx); err != nil {
		return PlatformRoleChangeResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PlatformRoleChangeResult{}, fmt.Errorf("commit administrator bootstrap %s: %w", uuid.UUID(op.ID.Bytes).String(), err)
	}
	result.Operation = op
	result.TargetUserID = p.TargetUserID
	result.CredentialsRevoked = true
	return result, nil
}
