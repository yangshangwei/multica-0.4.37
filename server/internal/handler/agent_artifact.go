package handler

import (
	"bytes"
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

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// The seam is the fixed adapter, never caller-selected commands or arguments.
type artifactExecutor interface {
	Prepare(context.Context, artifact.Request, artifact.Binding, time.Time) (artifact.Spec, error)
	Pack(context.Context, string, artifact.Spec) artifact.Outcome
}

type artifactFailure struct {
	status int
	code   string
}

func (e artifactFailure) Error() string { return e.code }
func artifactDeny() error               { return artifactFailure{403, "artifact_authority_denied"} }
func artifactConflict() error           { return artifactFailure{409, "artifact_binding_conflict"} }
func writeArtifactError(w http.ResponseWriter, err error) {
	var fail artifactFailure
	var pgerr *pgconn.PgError
	switch {
	case errors.As(err, &fail):
		writeError(w, fail.status, fail.code)
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, 404, "artifact_scope_not_found")
	case errors.Is(err, artifact.ErrInvalid):
		writeError(w, 422, "artifact_contract_invalid")
	case errors.Is(err, auth.ErrPasswordSession):
		writeError(w, 403, "artifact_credential_revoked")
	case errors.As(err, &pgerr) && (pgerr.Code == "55P03" || pgerr.Code == "40P01"):
		writeError(w, 409, "artifact_authority_busy")
	default:
		writeError(w, 503, "artifact_execution_unavailable")
	}
}
func artifactPrincipal(r *http.Request) (auth.AuthenticatedPrincipal, string, string, error) {
	p, ok := auth.AuthenticatedPrincipalFromContext(r.Context())
	if !ok || p.Restricted {
		return p, "", "", artifactDeny()
	}
	if p.Kind == auth.CredentialTask {
		tok, ok := auth.VerifiedTaskTokenFromContext(r.Context())
		if !ok || len(tok.ApiCapability) > 0 || uuidToString(tok.UserID) != p.UserID {
			return p, "", "", artifactDeny()
		}
		return p, "agent", uuidToString(tok.AgentID), nil
	}
	if p.AgentActor || (p.Kind != auth.CredentialJWT && p.Kind != auth.CredentialPAT) || r.Header.Get("X-Agent-ID") != "" || r.Header.Get("X-Task-ID") != "" {
		return p, "", "", artifactDeny()
	}
	return p, "human", p.UserID, nil
}
func (h *Handler) artifactTx(ctx context.Context) (pgx.Tx, *db.Queries, error) {
	if h.TxStarter == nil {
		return nil, nil, errors.New("transaction unavailable")
	}
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='250ms'"); err != nil {
		tx.Rollback(ctx)
		return nil, nil, err
	}
	q := db.New(tx)
	ready, err := q.ArtifactExecutionSchemaReady(ctx)
	if err != nil || !ready {
		tx.Rollback(ctx)
		if err == nil {
			err = errors.New("schema unavailable")
		}
		return nil, nil, err
	}
	return tx, q, nil
}
func artifactFence(ctx context.Context, q *db.Queries, ws pgtype.UUID) error {
	rows, err := q.TryLockCollaborationAuthorityWorkspaces(ctx, []pgtype.UUID{ws})
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

// Credential checks consume the originally verified identity under live locks.
func artifactCredential(ctx context.Context, q *db.Queries, tx pgx.Tx, p auth.AuthenticatedPrincipal) error {
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return err
	}
	switch p.Kind {
	case auth.CredentialJWT:
		if p.ExpiresAt.IsZero() || !now.Time.Before(p.ExpiresAt) {
			return artifactDeny()
		}
	case auth.CredentialPAT:
		if p.PATTokenHash == "" {
			return artifactDeny()
		}
		if _, err = q.LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: p.PATTokenHash, UserID: parseUUID(p.UserID)}); err != nil {
			return err
		}
	case auth.CredentialTask:
		token, ok := auth.VerifiedTaskTokenFromContext(ctx)
		if !ok {
			return artifactDeny()
		}
		if _, err = q.LockTaskCapabilityToken(ctx, token.ID); err != nil {
			return err
		}
		live, e := q.GetTaskTokenByHash(ctx, token.TokenHash)
		if e != nil {
			return e
		}
		if live.ID != token.ID || live.TaskID != token.TaskID || live.AgentID != token.AgentID || live.UserID != token.UserID || live.WorkspaceID != token.WorkspaceID || live.AuthVersion != token.AuthVersion || live.InstallationBindingID != token.InstallationBindingID || live.InstallationBindingEpoch != token.InstallationBindingEpoch || !bytes.Equal(live.ApiCapability, token.ApiCapability) || !now.Time.Before(live.ExpiresAt.Time) {
			return artifactDeny()
		}
	default:
		return artifactDeny()
	}
	if s, ok := auth.PasswordSessionFromContext(ctx); ok {
		if s.UserID != p.UserID || s.Setup || s.Change {
			return artifactDeny()
		}
		if _, err = q.LockArtifactPasswordCredential(ctx, parseUUID(s.UserID)); err != nil {
			return err
		}
		if s.BindingID != "" {
			acquired, e := q.TryLockCollaborationAuthorityNamespace(ctx, "daemon:"+s.WorkspaceID+":"+s.DaemonID)
			if e != nil {
				return e
			}
			if !acquired {
				return artifactFailure{409, "artifact_authority_busy"}
			}
			binding, e := q.TryLockCollaborationAuthorityBinding(ctx, parseUUID(s.BindingID))
			if e != nil {
				return e
			}
			if _, e = q.TryLockCollaborationAuthorityInstallation(ctx, binding.InstallationID); e != nil {
				return e
			}
			var id pgtype.UUID
			if e = tx.QueryRow(ctx, `SELECT ow.organization_id FROM organization_workspace ow JOIN organization o ON o.id=ow.organization_id WHERE ow.workspace_id=$1 FOR SHARE OF ow,o NOWAIT`, binding.WorkspaceID).Scan(&id); e != nil {
				return e
			}
			if e = auth.CheckManagedBinding(ctx, q, s); e != nil {
				return e
			}
		}
		_, err = auth.CheckPasswordVersion(ctx, q, s.UserID, s.Version)
		return err
	}
	if auth.PasswordMode() {
		return artifactDeny()
	}
	return nil
}

func artifactUsers(ctx context.Context, q *db.Queries, ws pgtype.UUID, ids ...string) (map[string]db.Member, error) {
	seen := map[string]bool{}
	var sorted []string
	for _, id := range ids {
		if id != "" && !seen[id] {
			if _, err := util.ParseUUID(id); err != nil {
				return nil, artifactDeny()
			}
			seen[id] = true
			sorted = append(sorted, id)
		}
	}
	sort.Strings(sorted)
	uuids := make([]pgtype.UUID, len(sorted))
	for i, id := range sorted {
		uuids[i] = parseUUID(id)
	}
	users, err := q.LockArtifactUsers(ctx, uuids)
	if err != nil {
		return nil, err
	}
	if len(users) != len(uuids) {
		return nil, artifactDeny()
	}
	for _, u := range users {
		if u.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(uuidToString(u.ID), u.Email.String) {
			return nil, artifactDeny()
		}
	}
	members, err := q.LockArtifactMembers(ctx, db.LockArtifactMembersParams{WorkspaceID: ws, UserIds: uuids})
	if err != nil {
		return nil, err
	}
	if len(members) != len(uuids) {
		return nil, artifactDeny()
	}
	out := map[string]db.Member{}
	for _, m := range members {
		out[uuidToString(m.UserID)] = m
	}
	return out, nil
}
func artifactManager(agent db.Agent, id string, members map[string]db.Member) bool {
	m, ok := members[id]
	return ok && (uuidToString(agent.OwnerID) == id || m.Role == "owner" || m.Role == "admin")
}

// full controls new authority; reads and human cancellation may outlive the task.
func (h *Handler) artifactAuthority(r *http.Request, q *db.Queries, tx pgx.Tx, req artifact.Request, prior *db.AgentArtifactExecution, approver string, full bool, manage bool) (artifact.Binding, error) {
	ctx := r.Context()
	p, kind, id, err := artifactPrincipal(r)
	if err != nil {
		return artifact.Binding{}, err
	}
	wsText := h.resolveWorkspaceID(r)
	if prior != nil {
		wsText = uuidToString(prior.WorkspaceID)
		if requested := h.resolveWorkspaceID(r); requested != "" && requested != wsText {
			return artifact.Binding{}, pgx.ErrNoRows
		}
	}
	ws, err := util.ParseUUID(wsText)
	if err != nil {
		return artifact.Binding{}, artifactDeny()
	}
	if err = artifactFence(ctx, q, ws); err != nil {
		return artifact.Binding{}, err
	}
	if kind == "agent" {
		tok, _ := auth.VerifiedTaskTokenFromContext(ctx)
		if uuidToString(tok.TaskID) != req.TaskID || uuidToString(tok.AgentID) != req.AgentID || tok.WorkspaceID != ws {
			return artifact.Binding{}, artifactDeny()
		}
		if _, err = q.LockTaskCapabilityToken(ctx, tok.ID); err != nil {
			return artifact.Binding{}, err
		}
	}
	binding := artifact.Binding{WorkspaceID: wsText, InitiatorKind: kind, InitiatorID: id}
	if prior != nil {
		binding.InitiatorKind = prior.InitiatorKind
		binding.InitiatorID = uuidToString(prior.InitiatorID)
		binding.OriginatorUserID = uuidToString(prior.OriginatorUserID)
		binding.RuntimeID = uuidToString(prior.RuntimeID)
		binding.TaskGeneration = prior.TaskGeneration
	}
	var task db.AgentTaskQueue
	if full || kind == "agent" {
		if _, err = q.LockTaskCapabilityPrincipals(ctx, parseUUID(req.TaskID)); err != nil {
			return binding, err
		}
		task, err = q.GetAgentTask(ctx, parseUUID(req.TaskID))
		if err != nil {
			return binding, err
		}
		if task.Status != "dispatched" && task.Status != "running" || uuidToString(task.AgentID) != req.AgentID || uuidToString(task.IssueID) != req.ResourceID || !task.OriginatorUserID.Valid || !task.RuntimeID.Valid {
			return binding, artifactDeny()
		}
		if prior != nil && (task.ClaimGeneration != prior.TaskGeneration || task.RuntimeID != prior.RuntimeID || task.OriginatorUserID != prior.OriginatorUserID) {
			return binding, artifactConflict()
		}
		binding.OriginatorUserID = uuidToString(task.OriginatorUserID)
		binding.RuntimeID = uuidToString(task.RuntimeID)
		binding.TaskGeneration = task.ClaimGeneration
	}
	agents, err := q.TryLockCollaborationAuthorityAgents(ctx, []pgtype.UUID{parseUUID(req.AgentID)})
	if err != nil {
		return binding, err
	}
	if len(agents) != 1 {
		return binding, artifactDeny()
	}
	agent := agents[0]
	if agent.WorkspaceID != ws || agent.ArchivedAt.Valid || auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) != "" {
		return binding, artifactDeny()
	}
	issueScope, err := q.GetIssue(ctx, parseUUID(req.ResourceID))
	if err != nil {
		return binding, err
	}
	if issueScope.WorkspaceID != ws {
		return binding, artifactDeny()
	}
	targets, err := q.LockTaskCapabilityInvocationTargets(ctx, agent.ID)
	if err != nil {
		return binding, err
	}
	ids := []string{p.UserID}
	if binding.InitiatorKind == "human" {
		ids = append(ids, binding.InitiatorID)
	}
	if approver != "" {
		ids = append(ids, approver)
	}
	if full || kind == "agent" {
		runtime, e := (service.RuntimeLookup{Queries: q, Metrics: h.Metrics, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, task.RuntimeID)
		if e != nil {
			return binding, e
		}
		issue, e := q.GetIssue(ctx, task.IssueID)
		if e != nil {
			return binding, e
		}
		if runtime.WorkspaceID != ws || issue.WorkspaceID != ws || agent.RuntimeID != runtime.ID {
			return binding, artifactDeny()
		}
		if kind == "agent" && uuidToString(runtime.OwnerID) != p.UserID {
			return binding, artifactDeny()
		}
		ids = append(ids, binding.OriginatorUserID, uuidToString(runtime.OwnerID))
	}
	members, err := artifactUsers(ctx, q, ws, ids...)
	if err != nil {
		return binding, err
	}
	if kind == "human" && (manage || prior == nil || binding.InitiatorKind != "human" || binding.InitiatorID != p.UserID) && !artifactManager(agent, p.UserID, members) {
		return binding, artifactDeny()
	}
	if approver != "" && !artifactManager(agent, approver, members) {
		return binding, artifactDeny()
	}
	if full || kind == "agent" {
		if !auth.AgentInvocationAllowed(agent, targets, binding.OriginatorUserID, true, false) || kind == "human" && !auth.AgentInvocationAllowed(agent, targets, p.UserID, true, false) {
			return binding, artifactDeny()
		}
	}
	if err = artifactCredential(ctx, q, tx, p); err != nil {
		return binding, err
	}
	return binding, nil
}

type artifactResponse struct {
	Approval           AgentApprovalResponse `json:"approval"`
	ExecutionID        string                `json:"execution_id"`
	Spec               json.RawMessage       `json:"spec"`
	SpecDigest         string                `json:"spec_digest"`
	RevisionProvenance string                `json:"revision_provenance"`
	Status             string                `json:"status"`
	ResultCode         string                `json:"result_code"`
	Receipt            json.RawMessage       `json:"receipt"`
	ReceiptDigest      string                `json:"receipt_digest"`
}

func artifactReply(a db.AgentApprovalRequest, e db.AgentArtifactExecution) artifactResponse {
	status := "unclaimed"
	if e.ResultStatus.Valid {
		status = e.ResultStatus.String
	}
	receipt := json.RawMessage(e.ReceiptJson)
	if len(receipt) == 0 {
		receipt = json.RawMessage(`{}`)
	}
	return artifactResponse{agentApprovalToResponse(a), uuidToString(e.ExecutionID), json.RawMessage(e.CanonicalSpec), e.SpecDigest, "declared_not_independently_git_verified", status, e.ResultCode, receipt, e.ReceiptDigest}
}
func artifactTimeMatches(raw string, value pgtype.Timestamptz) bool {
	t, err := time.Parse(time.RFC3339Nano, raw)
	return err == nil && value.Valid && t.Equal(value.Time)
}

func artifactSpec(e db.AgentArtifactExecution) (artifact.Spec, error) {
	s, err := artifact.ParseSpec(e.CanonicalSpec)
	if err != nil {
		return s, err
	}
	_, digest, err := artifact.CanonicalSpec(s)
	if err != nil {
		return s, err
	}
	if digest != e.SpecDigest || s.Request.AgentID != uuidToString(e.AgentID) || s.Request.TaskID != uuidToString(e.TaskID) || s.Request.ResourceID != uuidToString(e.ResourceID) || s.Binding.WorkspaceID != uuidToString(e.WorkspaceID) || s.Binding.InitiatorKind != e.InitiatorKind || s.Binding.InitiatorID != uuidToString(e.InitiatorID) || s.Binding.OriginatorUserID != uuidToString(e.OriginatorUserID) || s.Binding.RuntimeID != uuidToString(e.RuntimeID) || s.Binding.TaskGeneration != e.TaskGeneration || s.RootIdentity != e.RootConfigIdentity || s.InvocationDigest != e.InvocationDigest || s.Request.ResourceKind != e.ResourceKind || s.Request.Action != e.Action || s.Request.SourceRevision != e.SourceRevision || s.Request.StagingID != e.StagingID || s.Request.InputManifestDigest != e.InputManifestDigest || s.Request.StagingManifestDigest != e.StagingManifestDigest || s.Request.TargetEnvironment != e.TargetEnvironment || s.Request.PolicyVersion != e.PolicyVersion || !artifactTimeMatches(s.CreatedAt, e.CreatedAt) || !artifactTimeMatches(s.ExpiresAt, e.ExpiresAt) {
		return s, artifactConflict()
	}
	return s, nil
}

func (h *Handler) CreateArtifactPackage(w http.ResponseWriter, r *http.Request) {
	if h.ArtifactPackager == nil {
		writeArtifactError(w, artifact.ErrDisabled)
		return
	}
	req, err := artifact.ParseRequest(r.Body)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	key, ok := parseUUIDOrBadRequest(w, r.Header.Get("Idempotency-Key"), "Idempotency-Key")
	if !ok {
		return
	}
	if uuidToString(key) != r.Header.Get("Idempotency-Key") || uuidToString(key) == "00000000-0000-0000-0000-000000000000" {
		writeArtifactError(w, artifact.ErrInvalid)
		return
	}
	tx, q, err := h.artifactTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	_, kind, actor, err := artifactPrincipal(r)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	ws, err := util.ParseUUID(h.resolveWorkspaceID(r))
	if err != nil {
		writeArtifactError(w, artifactDeny())
		return
	}
	old, err := q.GetAgentArtifactExecutionByIdempotency(r.Context(), db.GetAgentArtifactExecutionByIdempotencyParams{WorkspaceID: ws, InitiatorKind: kind, InitiatorID: parseUUID(actor), IdempotencyKey: key})
	if err == nil {
		original, e := artifactSpec(old)
		if e != nil {
			writeArtifactError(w, e)
			return
		}
		if _, e = h.artifactAuthority(r, q, tx, original.Request, &old, "", false, false); e != nil {
			writeArtifactError(w, e)
			return
		}
		digest, e := artifact.RequestDigest(req, original.Binding)
		if e != nil {
			writeArtifactError(w, e)
			return
		}
		if digest != old.RequestDigest {
			writeArtifactError(w, artifactConflict())
			return
		}
		a, e := q.GetAgentApprovalRequest(r.Context(), old.ApprovalID)
		if e != nil {
			writeArtifactError(w, e)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, artifactReply(a, old))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeArtifactError(w, err)
		return
	}
	binding, err := h.artifactAuthority(r, q, tx, req, nil, "", true, true)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	digest, err := artifact.RequestDigest(req, binding)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	now, err := q.ArtifactExecutionDatabaseTime(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	spec, err := h.ArtifactPackager.Prepare(r.Context(), req, binding, now.Time)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	raw, specDigest, err := artifact.CanonicalSpec(spec)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	created, _ := time.Parse(time.RFC3339Nano, spec.CreatedAt)
	expires, _ := time.Parse(time.RFC3339Nano, spec.ExpiresAt)
	totalBytes := int64(0)
	for _, entry := range req.Entries {
		totalBytes += entry.SizeBytes
	}
	summary := fmt.Sprintf("Package %d files for local-synthetic (%s)", len(req.Entries), req.PolicyVersion)
	plan := fmt.Sprintf("Action: %s\nEnvironment: %s\nDeclared revision (not independently Git-verified): %s\nFiles: %d\nBytes: %d\nExpiry: %s\nSpecification SHA-256: %s\nInput SHA-256: %s\nReview the restricted artifact-package detail before deciding.", req.Action, req.TargetEnvironment, req.SourceRevision, len(req.Entries), totalBytes, spec.ExpiresAt, specDigest, req.InputManifestDigest)
	principal, _, _, err := artifactPrincipal(r)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if err = artifactCredential(r.Context(), q, tx, principal); err != nil {
		writeArtifactError(w, err)
		return
	}
	approval, err := q.CreateAgentApprovalRequest(r.Context(), db.CreateAgentApprovalRequestParams{WorkspaceID: parseUUID(binding.WorkspaceID), AgentID: parseUUID(req.AgentID), TaskID: parseUUID(req.TaskID), IssueID: parseUUID(req.ResourceID), RiskClass: "production_release", Summary: summary, Plan: plan})
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	e, err := q.CreateAgentArtifactExecution(r.Context(), db.CreateAgentArtifactExecutionParams{ExecutionID: dbid.NewV7(), ApprovalID: approval.ID, WorkspaceID: approval.WorkspaceID, InitiatorKind: binding.InitiatorKind, InitiatorID: parseUUID(binding.InitiatorID), OriginatorUserID: parseUUID(binding.OriginatorUserID), AgentID: approval.AgentID, TaskID: approval.TaskID, RuntimeID: parseUUID(binding.RuntimeID), TaskGeneration: binding.TaskGeneration, ResourceKind: req.ResourceKind, ResourceID: approval.IssueID, Action: req.Action, IdempotencyKey: key, RequestDigest: digest, CanonicalSpec: raw, SpecDigest: specDigest, SourceRevision: req.SourceRevision, StagingID: req.StagingID, StagingManifestDigest: req.StagingManifestDigest, InputManifestDigest: req.InputManifestDigest, InvocationDigest: spec.InvocationDigest, PolicyVersion: req.PolicyVersion, TargetEnvironment: req.TargetEnvironment, RootConfigIdentity: spec.RootIdentity, CreatedAt: pgtype.Timestamptz{Time: created, Valid: true}, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}, UpdatedAt: now})
	if errors.Is(err, pgx.ErrNoRows) {
		if rollbackErr := tx.Rollback(r.Context()); rollbackErr != nil {
			writeArtifactError(w, rollbackErr)
			return
		}
		winner, e2 := h.Queries.GetAgentArtifactExecutionByIdempotency(r.Context(), db.GetAgentArtifactExecutionByIdempotencyParams{WorkspaceID: parseUUID(binding.WorkspaceID), InitiatorKind: binding.InitiatorKind, InitiatorID: parseUUID(binding.InitiatorID), IdempotencyKey: key})
		if e2 != nil {
			writeArtifactError(w, e2)
			return
		}
		if winner.RequestDigest != digest {
			writeArtifactError(w, artifactConflict())
			return
		}
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("approvalId", uuidToString(winner.ApprovalID))
		h.GetArtifactPackage(w, r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx)))
		return
	}
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeArtifactError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, artifactReply(approval, e))
}
func (h *Handler) artifactLoad(r *http.Request) (db.AgentArtifactExecution, artifact.Spec, error) {
	id, err := util.ParseUUID(chi.URLParam(r, "approvalId"))
	if err != nil {
		return db.AgentArtifactExecution{}, artifact.Spec{}, artifact.ErrInvalid
	}
	e, err := h.Queries.GetAgentArtifactExecutionByApproval(r.Context(), id)
	if err != nil {
		return e, artifact.Spec{}, err
	}
	s, err := artifactSpec(e)
	return e, s, err
}
func (h *Handler) GetArtifactPackage(w http.ResponseWriter, r *http.Request) {
	e, s, err := h.artifactLoad(r)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	tx, q, err := h.artifactTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = h.artifactAuthority(r, q, tx, s.Request, &e, "", false, false); err != nil {
		writeArtifactError(w, err)
		return
	}
	a, err := q.GetAgentApprovalRequest(r.Context(), e.ApprovalID)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, artifactReply(a, e))
}
func (h *Handler) ExecuteArtifactPackage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExpectedSpecDigest string `json:"expected_spec_digest"`
	}
	if err := artifact.DecodeStrict(r.Body, &body); err != nil {
		writeArtifactError(w, err)
		return
	}
	e, s, err := h.artifactLoad(r)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if body.ExpectedSpecDigest != e.SpecDigest {
		writeArtifactError(w, artifactConflict())
		return
	}
	tx, q, err := h.artifactTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	hint, err := q.GetAgentApprovalRequest(r.Context(), e.ApprovalID)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	approver := ""
	if !e.ClaimedAt.Valid {
		approver = uuidToString(hint.DecidedBy)
	}
	binding, err := h.artifactAuthority(r, q, tx, s.Request, &e, approver, !e.ClaimedAt.Valid, !e.ClaimedAt.Valid)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	a, err := q.LockAgentApprovalRequest(r.Context(), e.ApprovalID)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	current, err := q.LockAgentArtifactExecutionByApproval(r.Context(), e.ApprovalID)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if a.DecidedBy != hint.DecidedBy || current.SpecDigest != e.SpecDigest {
		writeArtifactError(w, artifactConflict())
		return
	}
	if current.ClaimedAt.Valid {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, artifactReply(a, current))
		return
	}
	if a.Status != "approved" {
		writeArtifactError(w, artifactConflict())
		return
	}
	agent, err := q.GetAgent(r.Context(), e.AgentID)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if !service.AutonomyAtLeast(agent.AutonomyLevel, service.AutonomyOperator) {
		writeArtifactError(w, artifactDeny())
		return
	}
	if h.ArtifactPackager == nil {
		writeArtifactError(w, artifact.ErrDisabled)
		return
	}
	fresh, err := h.ArtifactPackager.Prepare(r.Context(), s.Request, binding, e.CreatedAt.Time)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	freshBytes, _, err := artifact.CanonicalSpec(fresh)
	if err != nil || !bytes.Equal(freshBytes, e.CanonicalSpec) {
		writeArtifactError(w, artifactConflict())
		return
	}
	now, err := q.ArtifactExecutionDatabaseTime(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if !now.Time.Before(e.ExpiresAt.Time) {
		writeArtifactError(w, artifactFailure{410, "artifact_expired"})
		return
	}
	_, kind, id, err := artifactPrincipal(r)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	principal, _, _, err := artifactPrincipal(r)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if err = artifactCredential(r.Context(), q, tx, principal); err != nil {
		writeArtifactError(w, err)
		return
	}
	claimed, err := q.ClaimAgentArtifactExecution(r.Context(), db.ClaimAgentArtifactExecutionParams{ExecutionID: e.ExecutionID, SpecDigest: e.SpecDigest, ClaimNonce: dbid.NewV7(), ClaimedKind: pgtype.Text{String: kind, Valid: true}, ClaimedBy: parseUUID(id)})
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeArtifactError(w, err)
		return
	}
	// A definite claim owns one bounded server operation; client disconnect does not
	// discard its result. No future request may take over this operation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), artifact.ExecutionTimeout)
	defer cancel()
	outcome := runArtifact(ctx, h.ArtifactPackager, uuidToString(e.ExecutionID), s)
	final, approval, err := h.finalizeArtifact(ctx, claimed, outcome)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "artifact_execution_unknown", "status": "unknown", "execution_id": uuidToString(claimed.ExecutionID)})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	status := 200
	if final.ResultStatus.String == "unknown" {
		status = 503
	}
	writeJSON(w, status, artifactReply(approval, final))
}
func runArtifact(ctx context.Context, p artifactExecutor, id string, s artifact.Spec) (out artifact.Outcome) {
	out = artifact.Outcome{Status: "unknown", Code: "adapter_interrupted"}
	defer func() {
		if recover() != nil {
			out = artifact.Outcome{Status: "unknown", Code: "adapter_panic"}
		}
	}()
	return p.Pack(ctx, id, s)
}
func (h *Handler) finalizeArtifact(ctx context.Context, e db.AgentArtifactExecution, out artifact.Outcome) (db.AgentArtifactExecution, db.AgentApprovalRequest, error) {
	tx, q, err := h.artifactTx(ctx)
	if err != nil {
		return e, db.AgentApprovalRequest{}, err
	}
	defer tx.Rollback(ctx)
	a, err := q.LockAgentApprovalRequest(ctx, e.ApprovalID)
	if err != nil {
		return e, a, err
	}
	live, err := q.LockAgentArtifactExecutionByApproval(ctx, e.ApprovalID)
	if err != nil {
		return e, a, err
	}
	if live.ClaimNonce != e.ClaimNonce || live.SpecDigest != e.SpecDigest || a.Status != "approved" {
		return e, a, artifactConflict()
	}
	if out.Status != "succeeded" && out.Status != "failed" {
		out = artifact.Outcome{Status: "unknown", Code: "adapter_uncertain"}
	}
	finished, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return e, a, err
	}
	receipt, err := json.Marshal(map[string]any{"finished_at": finished.Time.UTC(), "execution_id": uuidToString(e.ExecutionID), "approval_id": uuidToString(e.ApprovalID), "spec_digest": e.SpecDigest, "actor_kind": e.ClaimedKind.String, "actor_id": uuidToString(e.ClaimedBy), "resource_id": uuidToString(e.ResourceID), "action": e.Action, "declared_revision": e.SourceRevision, "input_digest": e.InputManifestDigest, "environment": e.TargetEnvironment, "policy": e.PolicyVersion, "result": out.Status, "code": out.Code, "artifact_id": out.ArtifactID, "output_digest": out.OutputDigest, "output_bytes": out.OutputSize, "claimed_at": e.ClaimedAt.Time.UTC()})
	if err != nil || len(receipt) > 4096 {
		return e, a, errors.New("receipt invalid")
	}
	hash := sha256.Sum256(receipt)
	params := db.FinalizeAgentArtifactExecutionParams{ExecutionID: e.ExecutionID, SpecDigest: e.SpecDigest, ClaimNonce: e.ClaimNonce, ResultStatus: pgtype.Text{String: out.Status, Valid: true}, ResultCode: out.Code, ReceiptJson: receipt, ReceiptDigest: hex.EncodeToString(hash[:]), OutputArtifactDigest: out.OutputDigest}
	if out.Status == "succeeded" {
		params.OutputArtifactID, err = util.ParseUUID(out.ArtifactID)
		if err != nil {
			return e, a, err
		}
		params.OutputSize = pgtype.Int8{Int64: out.OutputSize, Valid: true}
	}
	final, err := q.FinalizeAgentArtifactExecution(ctx, params)
	if err != nil {
		return e, a, err
	}
	if out.Status == "succeeded" {
		a, err = q.MarkAgentApprovalRequestExecuted(ctx, db.MarkAgentApprovalRequestExecutedParams{ID: a.ID, ExecutionNote: "Verified local artifact receipt " + params.ReceiptDigest})
		if err != nil {
			return e, a, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return e, a, err
	}
	return final, a, nil
}

// Bound approvals cannot use the legacy self-report write. Their scope is immutable.
func (h *Handler) controlledApproval(w http.ResponseWriter, r *http.Request, approval db.AgentApprovalRequest, action string) bool {
	e, err := h.Queries.GetAgentArtifactExecutionByApproval(r.Context(), approval.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		writeArtifactError(w, err)
		return true
	}
	s, err := artifactSpec(e)
	if err != nil {
		writeArtifactError(w, err)
		return true
	}
	if action == "execution" {
		writeArtifactError(w, artifactConflict())
		return true
	}
	var body struct {
		Decision           string `json:"decision"`
		Note               string `json:"note"`
		ExpectedSpecDigest string `json:"expected_spec_digest"`
	}
	if action == "decision" {
		if err = artifact.DecodeStrict(r.Body, &body); err != nil {
			writeArtifactError(w, err)
			return true
		}
		if body.ExpectedSpecDigest != e.SpecDigest {
			writeArtifactError(w, artifactConflict())
			return true
		}
		if body.Decision != "approve" && body.Decision != "reject" || utf8.RuneCountInString(body.Note) > maxApprovalNoteLength {
			writeArtifactError(w, artifact.ErrInvalid)
			return true
		}
	}
	tx, q, err := h.artifactTx(r.Context())
	if err != nil {
		writeArtifactError(w, err)
		return true
	}
	defer tx.Rollback(r.Context())
	if _, err = h.artifactAuthority(r, q, tx, s.Request, &e, "", action == "decision", true); err != nil {
		writeArtifactError(w, err)
		return true
	}
	a, err := q.LockAgentApprovalRequest(r.Context(), approval.ID)
	if err != nil {
		writeArtifactError(w, err)
		return true
	}
	live, err := q.LockAgentArtifactExecutionByApproval(r.Context(), approval.ID)
	if err != nil {
		writeArtifactError(w, err)
		return true
	}
	if live.SpecDigest != e.SpecDigest || live.ClaimedAt.Valid {
		writeArtifactError(w, artifactConflict())
		return true
	}
	if action == "decision" {
		now, e2 := q.ArtifactExecutionDatabaseTime(r.Context())
		if e2 != nil {
			writeArtifactError(w, e2)
			return true
		}
		if !now.Time.Before(live.ExpiresAt.Time) {
			writeArtifactError(w, artifactFailure{410, "artifact_expired"})
			return true
		}
		status := "approved"
		if body.Decision == "reject" {
			status = "rejected"
		}
		a, err = q.DecideAgentApprovalRequest(r.Context(), db.DecideAgentApprovalRequestParams{ID: a.ID, Status: status, DecidedBy: parseUUID(requestUserID(r)), DecisionNote: strings.TrimSpace(body.Note)})
	} else {
		a, err = q.CancelAgentApprovalRequest(r.Context(), a.ID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = artifactConflict()
	}
	if err != nil {
		writeArtifactError(w, err)
		return true
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeArtifactError(w, err)
		return true
	}
	writeJSON(w, 200, agentApprovalToResponse(a))
	return true
}
