package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ManagedInstallationService struct {
	Queries           *db.Queries
	TxStarter         TxStarter
	DeploymentID      string
	EnrollmentEnabled bool
	Now               func() time.Time
}

func NewManagedInstallationService(q *db.Queries, tx TxStarter, deployment string, enabled bool) *ManagedInstallationService {
	return &ManagedInstallationService{Queries: q, TxStarter: tx, DeploymentID: deployment, EnrollmentEnabled: enabled, Now: time.Now}
}

type InstallationChallengeParams struct {
	Purpose              string  `json:"purpose"`
	PublicKey            string  `json:"public_key"`
	DesktopVersion       string  `json:"desktop_version,omitempty"`
	OS                   string  `json:"os,omitempty"`
	InstallationID       string  `json:"installation_id,omitempty"`
	WorkspaceID          string  `json:"workspace_id,omitempty"`
	DaemonID             string  `json:"daemon_id,omitempty"`
	ExpectedBindingEpoch *string `json:"expected_binding_epoch"`
}

type installationEnrollBody struct {
	PublicKey      string `json:"public_key"`
	DesktopVersion string `json:"desktop_version"`
	OS             string `json:"os"`
}

type installationBindBody struct {
	InstallationID       string  `json:"installation_id"`
	WorkspaceID          string  `json:"workspace_id"`
	DaemonID             string  `json:"daemon_id"`
	PublicKey            string  `json:"public_key"`
	ExpectedBindingEpoch *string `json:"expected_binding_epoch"`
}

type InstallationEnrollment struct {
	InstallationID    string                    `json:"installation_id"`
	KeyVersion        string                    `json:"key_version"`
	MetadataProof     string                    `json:"metadata_proof,omitempty"`
	Bindings          []InstallationBindingHint `json:"bindings"`
	BindingsTruncated bool                      `json:"bindings_truncated"`
}

type InstallationBindingHint struct {
	BindingID    string `json:"binding_id"`
	WorkspaceID  string `json:"workspace_id"`
	DaemonID     string `json:"daemon_id"`
	BindingEpoch string `json:"binding_epoch"`
	AuthVersion  string `json:"auth_version"`
}

type InstallationBindingLookup struct {
	Binding *InstallationBindingHint `json:"binding"`
}

// BindingHint resolves one namespace when the bounded enrollment hints omit it.
// A null result is authoritative only for this caller and requested namespace;
// issuing a binding challenge still performs the current-authority CAS.
func (s *ManagedInstallationService) BindingHint(ctx context.Context, installationID, workspaceID, daemonID string) (InstallationBindingLookup, error) {
	var result InstallationBindingLookup
	if s.Queries == nil {
		return result, installationError(503, "installation_unavailable")
	}
	_, user, err := s.source(ctx, s.Queries, true)
	if err != nil {
		return result, err
	}
	inst, err := installationUUID(installationID)
	if err != nil {
		return result, err
	}
	if auth.SubmissionInstallationFromContext(ctx) != inst {
		return result, installationError(403, "installation_proof_required")
	}
	workspace, err := installationUUID(workspaceID)
	if err != nil {
		return result, err
	}
	if _, err = installationUUID(daemonID); err != nil {
		return result, err
	}
	b, err := s.Queries.GetOwnInstallationBindingHint(ctx, db.GetOwnInstallationBindingHintParams{InstallationID: inst, PrincipalUserID: user, WorkspaceID: workspace, DaemonID: daemonID})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Binding = &InstallationBindingHint{util.UUIDToString(b.ID), util.UUIDToString(b.WorkspaceID), b.DaemonID, strconv.FormatInt(b.BindingEpoch, 10), strconv.FormatInt(b.AuthVersion, 10)}
	return result, nil
}

type InstallationBindingResult struct {
	BindingID         string `json:"binding_id"`
	BindingEpoch      string `json:"binding_epoch"`
	CapabilityVersion string `json:"capability_version"`
	DaemonToken       string `json:"daemon_token"`
	ExpiresAt         string `json:"expires_at"`
	PrincipalUserID   string `json:"principal_user_id"`
	AuthVersion       string `json:"auth_version"`
}

func installationError(status int, code string) error {
	return platformError(status, code, "The installation request could not be authorized or applied")
}

func installationUUID(raw string) (pgtype.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return pgtype.UUID{}, installationError(400, "invalid_installation_request")
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func (s *ManagedInstallationService) source(ctx context.Context, q *db.Queries, human bool) (auth.PasswordSession, pgtype.UUID, error) {
	var empty pgtype.UUID
	session, ok := auth.PasswordSessionFromContext(ctx)
	if !auth.PasswordMode() || !ok || session.Setup || session.Change || session.Version <= 0 {
		return session, empty, auth.ErrPasswordSession
	}
	if human && session.Kind != "jwt" || !human && session.Kind != "jwt" && session.Kind != "pat" && session.Kind != "daemon_token" {
		return session, empty, installationError(403, "installation_session_required")
	}
	id, err := installationUUID(session.UserID)
	if err != nil {
		return session, empty, auth.ErrPasswordSession
	}
	if _, err = auth.CheckPasswordVersion(ctx, q, session.UserID, session.Version); err != nil {
		return session, empty, err
	}
	u, err := q.GetUser(ctx, id)
	if err != nil {
		return session, empty, err
	}
	if auth.IsTemporarilyDisabledUser(session.UserID, u.Email.String) {
		return session, empty, auth.ErrPasswordSession
	}
	return session, id, nil
}

func (s *ManagedInstallationService) IssueChallenge(ctx context.Context, p InstallationChallengeParams) (installation.Challenge, error) {
	var out installation.Challenge
	if s.Queries == nil {
		return out, installationError(503, "installation_unavailable")
	}
	if !s.EnrollmentEnabled && p.Purpose != "renew" {
		return out, installationError(403, "installation_enrollment_disabled")
	}
	deployment, err := installationUUID(s.DeploymentID)
	if err != nil {
		return out, installationError(503, "installation_unavailable")
	}
	actor, user, err := s.source(ctx, s.Queries, p.Purpose != "renew")
	if err != nil {
		return out, err
	}
	org, err := s.Queries.GetInternalOrganization(ctx)
	if err != nil {
		return out, installationError(503, "installation_unavailable")
	}
	key, err := installation.Decode(p.PublicKey, 32)
	if err != nil || len(key) != 32 {
		return out, installationError(400, "invalid_installation_key")
	}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return out, err
	}
	id := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	expires := s.Now().UTC().Add(2 * time.Minute)
	payload := installation.Payload{ProtocolVersion: "1", Purpose: p.Purpose, ChallengeID: util.UUIDToString(id), DeploymentID: s.DeploymentID, OrganizationID: util.UUIDToString(org.ID), UserID: actor.UserID, AuthVersion: strconv.FormatInt(actor.Version, 10), PublicKeyFingerprint: installation.Digest(key), Nonce: base64.RawURLEncoding.EncodeToString(nonce), ExpiresAt: strconv.FormatInt(expires.Unix(), 10), Method: "POST", Path: "/api/installations/enroll"}
	params := db.CreateInstallationChallengeParams{ID: id, Purpose: p.Purpose, DeploymentID: deployment, OrganizationID: org.ID, UserID: user, AuthVersion: actor.Version, PublicKey: key, KeyFingerprint: payload.PublicKeyFingerprint, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}}
	var body []byte
	switch p.Purpose {
	case "enroll":
		if p.InstallationID != "" || p.WorkspaceID != "" || p.DaemonID != "" || p.ExpectedBindingEpoch != nil || !validInstallationVersion(p.DesktopVersion, p.OS) {
			return out, installationError(400, "invalid_installation_request")
		}
		body, err = json.Marshal(installationEnrollBody{p.PublicKey, p.DesktopVersion, p.OS})
	case "bind", "renew":
		if p.DesktopVersion != "" || p.OS != "" {
			return out, installationError(400, "invalid_installation_request")
		}
		params.InstallationID, err = installationUUID(p.InstallationID)
		if err != nil {
			return out, err
		}
		params.WorkspaceID, err = installationUUID(p.WorkspaceID)
		if err != nil {
			return out, err
		}
		if _, err = installationUUID(p.DaemonID); err != nil {
			return out, err
		}
		params.DaemonID = pgtype.Text{String: p.DaemonID, Valid: true}
		if p.ExpectedBindingEpoch != nil {
			v, e := strconv.ParseInt(*p.ExpectedBindingEpoch, 10, 64)
			if e != nil || v <= 0 || strconv.FormatInt(v, 10) != *p.ExpectedBindingEpoch {
				return out, installationError(400, "invalid_binding_epoch")
			}
			params.ExpectedBindingEpoch = pgtype.Int8{Int64: v, Valid: true}
		}
		inst, e := s.Queries.GetManagedInstallation(ctx, params.InstallationID)
		if e != nil {
			return out, e
		}
		if inst.OrganizationID != org.ID || inst.DeploymentID != deployment || inst.Lifecycle != "active" || inst.KeyFingerprint != payload.PublicKeyFingerprint {
			return out, installationError(403, "installation_scope_forbidden")
		}
		if e = s.workspaceAccess(ctx, s.Queries, user, params.WorkspaceID, org.ID); e != nil {
			return out, e
		}
		if p.Purpose == "renew" {
			binding, e := s.Queries.GetActiveInstallationBinding(ctx, db.GetActiveInstallationBindingParams{WorkspaceID: params.WorkspaceID, DaemonID: p.DaemonID})
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return out, e
			}
			if e != nil || actor.Kind != "daemon_token" || actor.BindingID != util.UUIDToString(binding.ID) || actor.BindingEpoch != binding.BindingEpoch || !params.ExpectedBindingEpoch.Valid || params.ExpectedBindingEpoch.Int64 != binding.BindingEpoch || binding.InstallationID != inst.ID || binding.PrincipalUserID != user || binding.AuthVersion != actor.Version {
				return out, installationError(403, "installation_scope_forbidden")
			}
		}
		payload.InstallationID = &p.InstallationID
		payload.WorkspaceID = &p.WorkspaceID
		payload.DaemonID = &p.DaemonID
		payload.ExpectedBindingEpoch = p.ExpectedBindingEpoch
		payload.Path = "/api/daemon/installation-bindings"
		if p.Purpose == "renew" {
			payload.Path += "/renew"
		}
		body, err = json.Marshal(installationBindBody{p.InstallationID, p.WorkspaceID, p.DaemonID, p.PublicKey, p.ExpectedBindingEpoch})
	default:
		return out, installationError(400, "invalid_installation_purpose")
	}
	if err != nil {
		return out, err
	}
	var hashes installation.Hashes
	out, hashes, err = installation.Encode(payload, body)
	if err != nil {
		return out, err
	}
	params.PayloadHash = hashes.Payload
	params.BodyHash = hashes.Body
	params.NonceHash = hashes.Nonce
	_, err = s.Queries.CreateInstallationChallenge(ctx, params)
	return out, err
}

func validInstallationVersion(version, os string) bool {
	if len(version) == 0 || len(version) > 80 || (os != "macos" && os != "windows" && os != "linux" && os != "unknown") {
		return false
	}
	for _, c := range version {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || strings.ContainsRune("._+-", c)) {
			return false
		}
	}
	return true
}

func (s *ManagedInstallationService) workspaceAccess(ctx context.Context, q *db.Queries, user, workspace, organization pgtype.UUID) error {
	org, err := q.GetInstallationWorkspaceOrganization(ctx, workspace)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err != nil || org != organization {
		return installationError(403, "installation_scope_forbidden")
	}
	if _, err = q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: user, WorkspaceID: workspace}); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return installationError(403, "installation_scope_forbidden")
	}
	return nil
}

func (s *ManagedInstallationService) lockedProof(ctx context.Context, q *db.Queries, proof installation.Proof, purpose string, human bool) (db.InstallationChallenge, []byte, auth.PasswordSession, error) {
	var row db.InstallationChallenge
	id, err := installationUUID(proof.ChallengeID)
	if err != nil {
		return row, nil, auth.PasswordSession{}, err
	}
	row, err = q.LockInstallationChallenge(ctx, id)
	if err != nil {
		return row, nil, auth.PasswordSession{}, err
	}
	_, body, err := installation.Verify(proof, row.PublicKey, installation.Hashes{Payload: row.PayloadHash, Body: row.BodyHash, Nonce: row.NonceHash})
	if err != nil {
		return row, nil, auth.PasswordSession{}, installationError(403, "invalid_installation_proof")
	}
	actor, user, err := s.source(ctx, q, human)
	if err != nil {
		return row, nil, actor, err
	}
	if _, err = auth.LockPasswordSession(ctx, q); err != nil {
		return row, nil, actor, err
	}
	if row.Purpose != purpose || util.UUIDToString(row.DeploymentID) != s.DeploymentID || row.UserID != user || row.AuthVersion != actor.Version {
		return row, nil, actor, installationError(403, "installation_scope_forbidden")
	}
	if !row.ConsumedAt.Valid && !s.Now().Before(row.ExpiresAt.Time) {
		return row, nil, actor, installationError(410, "installation_challenge_expired")
	}
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return row, nil, actor, installationError(503, "installation_unavailable")
	}
	if org.ID != row.OrganizationID {
		return row, nil, actor, installationError(403, "installation_scope_forbidden")
	}
	return row, body, actor, nil
}

func (s *ManagedInstallationService) Enroll(ctx context.Context, proof installation.Proof) (InstallationEnrollment, error) {
	var result InstallationEnrollment
	if s.Queries == nil || s.TxStarter == nil {
		return result, installationError(503, "installation_unavailable")
	}
	if !s.EnrollmentEnabled {
		return result, installationError(403, "installation_enrollment_disabled")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	row, body, actor, err := s.lockedProof(ctx, q, proof, "enroll", true)
	if err != nil {
		return result, err
	}
	if row.ConsumedAt.Valid {
		inst, err := q.GetManagedInstallation(ctx, row.ResultID)
		if err != nil {
			return result, err
		}
		if inst.Lifecycle != "active" || inst.KeyFingerprint != row.KeyFingerprint || inst.DeploymentID != row.DeploymentID {
			return result, installationError(409, "installation_not_active")
		}
		return s.enrollmentResult(ctx, q, actor, inst, s.Now().Before(row.ExpiresAt.Time))
	}
	if err = q.LockInstallationNamespace(ctx, "key:"+util.UUIDToString(row.OrganizationID)+":"+row.KeyFingerprint); err != nil {
		return result, err
	}
	inst, err := q.GetManagedInstallationByKey(ctx, db.GetManagedInstallationByKeyParams{OrganizationID: row.OrganizationID, KeyFingerprint: row.KeyFingerprint})
	if errors.Is(err, pgx.ErrNoRows) {
		var p installationEnrollBody
		if json.Unmarshal(body, &p) != nil {
			return result, installationError(400, "invalid_installation_proof")
		}
		inst, err = q.CreateManagedInstallation(ctx, db.CreateManagedInstallationParams{DeploymentID: row.DeploymentID, OrganizationID: row.OrganizationID, PublicKey: row.PublicKey, KeyFingerprint: row.KeyFingerprint, ResponsibleUserID: row.UserID, DesktopVersion: pgtype.Text{String: p.DesktopVersion, Valid: true}, Os: pgtype.Text{String: p.OS, Valid: true}})
	}
	if err != nil {
		return result, err
	}
	if inst.DeploymentID != row.DeploymentID || inst.Lifecycle != "active" {
		return result, installationError(409, "installation_not_active")
	}
	if err = q.TouchInstallationUser(ctx, db.TouchInstallationUserParams{InstallationID: inst.ID, UserID: row.UserID}); err != nil {
		return result, err
	}
	if err = q.ConsumeInstallationChallenge(ctx, db.ConsumeInstallationChallengeParams{ID: row.ID, ResultID: inst.ID}); err != nil {
		return result, err
	}
	if err = installationAudit(ctx, q, actor, row.OrganizationID, inst.ID, "installation.enroll", nil, map[string]string{"key_fingerprint": inst.KeyFingerprint}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return s.enrollmentResult(ctx, s.Queries, actor, inst, true)
}

func (s *ManagedInstallationService) enrollmentResult(ctx context.Context, q *db.Queries, actor auth.PasswordSession, inst db.ManagedInstallation, mint bool) (InstallationEnrollment, error) {
	result := InstallationEnrollment{InstallationID: util.UUIDToString(inst.ID), KeyVersion: strconv.FormatInt(inst.KeyVersion, 10), Bindings: []InstallationBindingHint{}}
	user, err := util.ParseUUID(actor.UserID)
	if err != nil {
		return result, err
	}
	rows, err := q.ListOwnInstallationBindingHints(ctx, db.ListOwnInstallationBindingHintsParams{InstallationID: inst.ID, PrincipalUserID: user})
	if err != nil {
		return result, err
	}
	result.BindingsTruncated = len(rows) > 100
	if len(rows) > 100 {
		rows = rows[:100]
	}
	for _, b := range rows {
		result.Bindings = append(result.Bindings, InstallationBindingHint{util.UUIDToString(b.ID), util.UUIDToString(b.WorkspaceID), b.DaemonID, strconv.FormatInt(b.BindingEpoch, 10), strconv.FormatInt(b.AuthVersion, 10)})
	}
	if mint {
		result.MetadataProof, err = auth.MintInstallationMetadataProof(actor, inst, s.Now().UTC())
	}
	return result, err
}

func (s *ManagedInstallationService) Bind(ctx context.Context, proof installation.Proof) (InstallationBindingResult, error) {
	return s.bind(ctx, proof, false)
}

func (s *ManagedInstallationService) Renew(ctx context.Context, proof installation.Proof) (InstallationBindingResult, error) {
	return s.bind(ctx, proof, true)
}

func (s *ManagedInstallationService) bind(ctx context.Context, proof installation.Proof, renew bool) (InstallationBindingResult, error) {
	var result InstallationBindingResult
	if s.Queries == nil || s.TxStarter == nil {
		return result, installationError(503, "installation_unavailable")
	}
	if !renew && !s.EnrollmentEnabled {
		return result, installationError(403, "installation_enrollment_disabled")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	purpose := "bind"
	if renew {
		purpose = "renew"
	}
	// Inspect and verify immutable proof data before taking ordered locks.
	// Workspace deletion locks the workspace before expiring challenges, so
	// redemption must acquire that workspace fence before challenge FOR UPDATE.
	challengeID, err := installationUUID(proof.ChallengeID)
	if err != nil {
		return result, err
	}
	candidate, err := q.GetInstallationChallenge(ctx, challengeID)
	if err != nil {
		return result, err
	}
	if _, _, err = installation.Verify(proof, candidate.PublicKey, installation.Hashes{Payload: candidate.PayloadHash, Body: candidate.BodyHash, Nonce: candidate.NonceHash}); err != nil {
		return result, installationError(403, "invalid_installation_proof")
	}
	initialActor, initialUser, err := s.source(ctx, q, false)
	if err != nil {
		return result, err
	}
	if candidate.Purpose != purpose || candidate.UserID != initialUser || candidate.AuthVersion != initialActor.Version || util.UUIDToString(candidate.DeploymentID) != s.DeploymentID {
		return result, installationError(403, "installation_scope_forbidden")
	}
	if _, err = auth.LockPasswordSession(ctx, q); err != nil {
		return result, err
	}
	if err = q.LockInstallationNamespace(ctx, "daemon:"+util.UUIDToString(candidate.WorkspaceID)+":"+candidate.DaemonID.String); err != nil {
		return result, err
	}
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, candidate.WorkspaceID); err != nil {
		return result, err
	}
	row, _, actor, err := s.lockedProof(ctx, q, proof, purpose, false)
	if err != nil {
		return result, err
	}
	if actor.Kind != "pat" && actor.Kind != "daemon_token" {
		return result, installationError(403, "daemon_credential_required")
	}
	if actor.Kind == "daemon_token" && (actor.WorkspaceID != util.UUIDToString(row.WorkspaceID) || actor.DaemonID != row.DaemonID.String) {
		return result, installationError(403, "installation_scope_forbidden")
	}
	if err = s.workspaceAccess(ctx, q, row.UserID, row.WorkspaceID, row.OrganizationID); err != nil {
		return result, err
	}
	foreign, err := q.CountForeignInstallationDaemonRuntimes(ctx, db.CountForeignInstallationDaemonRuntimesParams{WorkspaceID: row.WorkspaceID, DaemonID: row.DaemonID, PrincipalUserID: row.UserID})
	if err != nil {
		return result, err
	}
	if foreign > 0 {
		return result, installationError(409, "historical_daemon_owner_mismatch")
	}
	inst, err := q.LockManagedInstallation(ctx, row.InstallationID)
	if err != nil {
		return result, err
	}
	if inst.OrganizationID != row.OrganizationID || inst.DeploymentID != row.DeploymentID || inst.Lifecycle != "active" || inst.KeyFingerprint != row.KeyFingerprint {
		return result, installationError(403, "installation_scope_forbidden")
	}
	if row.ConsumedAt.Valid {
		binding, e := q.GetInstallationBinding(ctx, row.ResultID)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return result, e
		}
		if e != nil || binding.State != "active" || binding.PrincipalUserID != row.UserID || binding.AuthVersion != actor.Version {
			return result, installationError(409, "binding_changed")
		}
		if err = openInstallationResult(row.ResultCredential, row.ID, &result); err != nil {
			return result, err
		}
		expires, e := time.Parse(time.RFC3339, result.ExpiresAt)
		if e != nil || !s.Now().Before(expires) {
			return InstallationBindingResult{}, installationError(410, "installation_credential_expired")
		}
		return result, nil
	}
	current, err := q.GetActiveInstallationBinding(ctx, db.GetActiveInstallationBindingParams{WorkspaceID: row.WorkspaceID, DaemonID: row.DaemonID.String})
	hasCurrent := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	epoch := int64(1)
	reuse := renew
	if renew {
		if !hasCurrent || actor.Kind != "daemon_token" || actor.BindingID != util.UUIDToString(current.ID) || actor.BindingEpoch != current.BindingEpoch || current.InstallationID != inst.ID || current.PrincipalUserID != row.UserID || current.AuthVersion != actor.Version || !row.ExpectedBindingEpoch.Valid || row.ExpectedBindingEpoch.Int64 != current.BindingEpoch {
			return result, installationError(403, "installation_scope_forbidden")
		}
		epoch = current.BindingEpoch
	} else if hasCurrent {
		if current.InstallationID != inst.ID || !row.ExpectedBindingEpoch.Valid || row.ExpectedBindingEpoch.Int64 != current.BindingEpoch {
			return result, installationError(409, "binding_conflict")
		}
		if current.PrincipalUserID == row.UserID && current.AuthVersion == actor.Version {
			reuse = true
			epoch = current.BindingEpoch
		} else {
			if current.BindingEpoch == math.MaxInt64 {
				return result, installationError(409, "binding_epoch_exhausted")
			}
			epoch = current.BindingEpoch + 1
			if err = q.RevokeInstallationBinding(ctx, current.ID); err != nil {
				return result, err
			}
			if err = q.DeleteInstallationBindingTokens(ctx, current.ID); err != nil {
				return result, err
			}
		}
	} else {
		previous, e := q.GetLatestInstallationBinding(ctx, db.GetLatestInstallationBindingParams{WorkspaceID: row.WorkspaceID, DaemonID: row.DaemonID.String})
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return result, e
		}
		if e == nil {
			if previous.InstallationID != inst.ID || previous.PrincipalUserID != row.UserID || !row.ExpectedBindingEpoch.Valid || row.ExpectedBindingEpoch.Int64 != previous.BindingEpoch {
				return result, installationError(409, "binding_conflict")
			}
			if previous.BindingEpoch == math.MaxInt64 {
				return result, installationError(409, "binding_epoch_exhausted")
			}
			epoch = previous.BindingEpoch + 1
		} else if row.ExpectedBindingEpoch.Valid {
			return result, installationError(409, "binding_conflict")
		}
		count, e := q.CountInstallationDaemonRuntimes(ctx, db.CountInstallationDaemonRuntimesParams{WorkspaceID: row.WorkspaceID, DaemonID: row.DaemonID})
		if e != nil {
			return result, e
		}
		if count > 0 && epoch == 1 && (actor.Kind != "daemon_token" || actor.WorkspaceID != util.UUIDToString(row.WorkspaceID) || actor.DaemonID != row.DaemonID.String) {
			return result, installationError(409, "historical_daemon_proof_required")
		}
	}
	if !reuse {
		current, err = q.CreateInstallationBinding(ctx, db.CreateInstallationBindingParams{InstallationID: inst.ID, WorkspaceID: row.WorkspaceID, DaemonID: row.DaemonID.String, PrincipalUserID: row.UserID, AuthVersion: actor.Version, BindingEpoch: epoch})
		if err != nil {
			return result, err
		}
	}
	raw, err := auth.GenerateDaemonToken()
	if err != nil {
		return result, err
	}
	expires := s.Now().UTC().Add(24 * time.Hour)
	if _, err = q.CreateManagedDaemonToken(ctx, db.CreateManagedDaemonTokenParams{TokenHash: auth.HashToken(raw), WorkspaceID: row.WorkspaceID, DaemonID: row.DaemonID.String, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}, UserID: row.UserID, AuthVersion: actor.Version, InstallationBindingID: current.ID, InstallationBindingEpoch: pgtype.Int8{Int64: epoch, Valid: true}}); err != nil {
		return result, err
	}
	result = InstallationBindingResult{BindingID: util.UUIDToString(current.ID), BindingEpoch: strconv.FormatInt(epoch, 10), CapabilityVersion: "1", DaemonToken: raw, ExpiresAt: expires.Format(time.RFC3339), PrincipalUserID: actor.UserID, AuthVersion: strconv.FormatInt(actor.Version, 10)}
	sealed, err := sealInstallationResult(row.ID, result)
	if err != nil {
		return InstallationBindingResult{}, err
	}
	if err = q.ConsumeInstallationChallenge(ctx, db.ConsumeInstallationChallengeParams{ID: row.ID, ResultID: current.ID, ResultCredential: sealed}); err != nil {
		return InstallationBindingResult{}, err
	}
	if err = installationAudit(ctx, q, actor, row.OrganizationID, inst.ID, "installation."+purpose, nil, map[string]string{"binding_id": result.BindingID, "binding_epoch": result.BindingEpoch, "workspace_id": util.UUIDToString(row.WorkspaceID)}); err != nil {
		return InstallationBindingResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return InstallationBindingResult{}, err
	}
	return result, nil
}

func installationAudit(ctx context.Context, q *db.Queries, actor auth.PasswordSession, org, target pgtype.UUID, action string, before, after any) error {
	oldJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	user, err := util.ParseUUID(actor.UserID)
	if err != nil {
		return err
	}
	requestID := chimw.GetReqID(ctx)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return q.CreateAdminAuditEvent(ctx, db.CreateAdminAuditEventParams{OrganizationID: org, ActorKind: "user", ActorUserID: user, TargetKind: "installation", TargetID: target, Action: action, Phase: "applied", RequestID: requestID, Reason: "Authenticated installation lifecycle", BeforeState: oldJSON, AfterState: newJSON, ResultCode: "applied"})
}

// Encrypt only the short-lived credential response needed for exact retries.
// The nonce, signature, password and private installation key are never stored.
func installationAEAD() (cipher.AEAD, error) {
	key := sha256.Sum256(append([]byte("installation-result-v1\x00"), auth.JWTSecret()...))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func sealInstallationResult(id pgtype.UUID, result InstallationBindingResult) ([]byte, error) {
	aead, err := installationAEAD()
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, raw, id.Bytes[:]), nil
}
func openInstallationResult(sealed []byte, id pgtype.UUID, result *InstallationBindingResult) error {
	aead, err := installationAEAD()
	if err != nil {
		return err
	}
	if len(sealed) < aead.NonceSize() {
		return installationError(503, "installation_result_unavailable")
	}
	raw, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], id.Bytes[:])
	if err != nil {
		return installationError(503, "installation_result_unavailable")
	}
	return json.Unmarshal(raw, result)
}
