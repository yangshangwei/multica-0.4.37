package service

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/auth"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ArtifactError preserves a controlled-action refusal without owning a transport.
type ArtifactError struct {
	Status int
	Code   string
}

func (e ArtifactError) Error() string { return e.Code }
func artifactDeny() error             { return ArtifactError{403, "artifact_authority_denied"} }
func artifactConflict() error         { return ArtifactError{409, "artifact_binding_conflict"} }

// ArtifactCredentialSnapshot contains values supplied by a verified authentication
// adapter. It is not request JSON or a transferable authorization. It deliberately
// cannot serialize credential metadata into a public response.
type ArtifactCredentialSnapshot struct {
	Principal       auth.AuthenticatedPrincipal `json:"-"`
	PasswordSession *auth.PasswordSession       `json:"-"`
	TaskToken       *db.TaskToken               `json:"-"`
}

// Actor resolves only the identity actually verified by authentication. HTTP
// adapters additionally apply their deny-only actor-header checks.
func (c ArtifactCredentialSnapshot) Actor() (kind, id string, err error) {
	p := c.Principal
	if p.UserID == "" || p.Restricted {
		return "", "", artifactDeny()
	}
	if p.Kind == auth.CredentialTask {
		token := c.TaskToken
		if token == nil || !token.ID.Valid || len(token.ApiCapability) > 0 || util.UUIDToString(token.UserID) != p.UserID {
			return "", "", artifactDeny()
		}
		return "agent", util.UUIDToString(token.AgentID), nil
	}
	if p.AgentActor || (p.Kind != auth.CredentialJWT && p.Kind != auth.CredentialPAT) {
		return "", "", artifactDeny()
	}
	return "human", p.UserID, nil
}

// ArtifactAccess is the explicit request authority and workspace selection used
// by the common observer and controlled-action checks. It grants no authority
// without the current database checks performed inside the borrowed transaction.
type ArtifactAccess struct {
	Credential           ArtifactCredentialSnapshot
	RequestedWorkspaceID string
	Metrics              *obsmetrics.BusinessMetrics
	// Supplied only by the server's current GitHub webhook configuration.
	GitHubWebhookConfigIdentity        string `json:"-"`
	CollaborationAdministrationEnabled bool   `json:"-"`
}

// PrepareArtifactTransaction retains the controlled-action timeout and exact
// schema guard while leaving transaction ownership with the caller.
func PrepareArtifactTransaction(ctx context.Context, tx pgx.Tx) error {
	q := db.New(tx)
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout='250ms'"); err != nil {
		return err
	}
	ready, err := q.ArtifactExecutionSchemaReady(ctx)
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("schema unavailable")
	}
	return nil
}

// LockArtifactWorkspace preserves the controlled-action workspace fence.
func LockArtifactWorkspace(ctx context.Context, q *db.Queries, ws pgtype.UUID) error {
	rows, err := q.TryLockCollaborationAuthorityWorkspaces(ctx, []pgtype.UUID{ws})
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

// CheckArtifactCredentialInTx consumes the original verified identity under live
// locks without reading a different actor from the execution context.
func CheckArtifactCredentialInTx(ctx context.Context, tx pgx.Tx, credential ArtifactCredentialSnapshot) error {
	q := db.New(tx)
	p := credential.Principal
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
		userID, e := util.ParseUUID(p.UserID)
		if e != nil {
			return artifactDeny()
		}
		if _, err = q.LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: p.PATTokenHash, UserID: userID}); err != nil {
			return err
		}
	case auth.CredentialTask:
		token := credential.TaskToken
		if token == nil || !token.ID.Valid {
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
	if credential.PasswordSession != nil {
		s := *credential.PasswordSession
		if s.UserID != p.UserID || s.Setup || s.Change {
			return artifactDeny()
		}
		userID, e := util.ParseUUID(s.UserID)
		if e != nil {
			return artifactDeny()
		}
		if _, err = q.LockArtifactPasswordCredential(ctx, userID); err != nil {
			return err
		}
		if s.BindingID != "" {
			acquired, e := q.TryLockCollaborationAuthorityNamespace(ctx, "daemon:"+s.WorkspaceID+":"+s.DaemonID)
			if e != nil {
				return e
			}
			if !acquired {
				return ArtifactError{409, "artifact_authority_busy"}
			}
			bindingID, e := util.ParseUUID(s.BindingID)
			if e != nil {
				return artifactDeny()
			}
			binding, e := q.TryLockCollaborationAuthorityBinding(ctx, bindingID)
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
		_, err = auth.CheckPasswordSessionVersion(ctx, q, s)
		return err
	}
	if auth.PasswordMode() {
		return artifactDeny()
	}
	return nil
}

// LockArtifactUsers locks and rechecks the exact current users and membership.
func LockArtifactUsers(ctx context.Context, q *db.Queries, ws pgtype.UUID, ids ...string) (map[string]db.Member, error) {
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
		parsed, err := util.ParseUUID(id)
		if err != nil {
			return nil, artifactDeny()
		}
		uuids[i] = parsed
	}
	users, err := q.LockArtifactUsers(ctx, uuids)
	if err != nil {
		return nil, err
	}
	if len(users) != len(uuids) {
		return nil, artifactDeny()
	}
	for _, u := range users {
		if u.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(u.ID), u.Email.String) {
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
		out[util.UUIDToString(m.UserID)] = m
	}
	return out, nil
}
func artifactManager(agent db.Agent, id string, members map[string]db.Member) bool {
	m, ok := members[id]
	return ok && (util.UUIDToString(agent.OwnerID) == id || m.Role == "owner" || m.Role == "admin")
}

// CheckArtifactAuthorityInTx checks the original action authority in the borrowed
// transaction. Full controls new authority; reads and human cancellation may
// outlive the task. It neither owns the transaction nor starts an execution.
func CheckArtifactAuthorityInTx(ctx context.Context, tx pgx.Tx, access ArtifactAccess, req artifact.Request, prior *db.AgentArtifactExecution, approver string, full bool, manage bool) (artifact.Binding, error) {
	q := db.New(tx)
	p := access.Credential.Principal
	kind, id, err := access.Credential.Actor()
	if err != nil {
		return artifact.Binding{}, err
	}
	wsText := access.RequestedWorkspaceID
	if prior != nil {
		wsText = util.UUIDToString(prior.WorkspaceID)
		if requested := access.RequestedWorkspaceID; requested != "" && requested != wsText {
			return artifact.Binding{}, pgx.ErrNoRows
		}
	}
	ws, err := util.ParseUUID(wsText)
	if err != nil {
		return artifact.Binding{}, artifactDeny()
	}
	if err = LockArtifactWorkspace(ctx, q, ws); err != nil {
		return artifact.Binding{}, err
	}
	if kind == "agent" {
		tok := access.Credential.TaskToken
		if util.UUIDToString(tok.TaskID) != req.TaskID || util.UUIDToString(tok.AgentID) != req.AgentID || tok.WorkspaceID != ws {
			return artifact.Binding{}, artifactDeny()
		}
		if _, err = q.LockTaskCapabilityToken(ctx, tok.ID); err != nil {
			return artifact.Binding{}, err
		}
	}
	binding := artifact.Binding{WorkspaceID: wsText, InitiatorKind: kind, InitiatorID: id}
	if prior != nil {
		binding.InitiatorKind = prior.InitiatorKind
		binding.InitiatorID = util.UUIDToString(prior.InitiatorID)
		binding.OriginatorUserID = util.UUIDToString(prior.OriginatorUserID)
		binding.RuntimeID = util.UUIDToString(prior.RuntimeID)
		binding.TaskGeneration = prior.TaskGeneration
	}
	var task db.AgentTaskQueue
	if full || kind == "agent" {
		taskID, e := util.ParseUUID(req.TaskID)
		if e != nil {
			return binding, artifactDeny()
		}
		if _, err = q.LockTaskCapabilityPrincipals(ctx, taskID); err != nil {
			return binding, err
		}
		task, err = q.GetAgentTask(ctx, taskID)
		if err != nil {
			return binding, err
		}
		if task.Status != "dispatched" && task.Status != "running" || util.UUIDToString(task.AgentID) != req.AgentID || util.UUIDToString(task.IssueID) != req.ResourceID || !task.OriginatorUserID.Valid || !task.RuntimeID.Valid {
			return binding, artifactDeny()
		}
		if prior != nil && (task.ClaimGeneration != prior.TaskGeneration || task.RuntimeID != prior.RuntimeID || task.OriginatorUserID != prior.OriginatorUserID) {
			return binding, artifactConflict()
		}
		binding.OriginatorUserID = util.UUIDToString(task.OriginatorUserID)
		binding.RuntimeID = util.UUIDToString(task.RuntimeID)
		binding.TaskGeneration = task.ClaimGeneration
	}
	agentID, err := util.ParseUUID(req.AgentID)
	if err != nil {
		return binding, artifactDeny()
	}
	agents, err := q.TryLockCollaborationAuthorityAgents(ctx, []pgtype.UUID{agentID})
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
	resourceID, err := util.ParseUUID(req.ResourceID)
	if err != nil {
		return binding, artifactDeny()
	}
	issueScope, err := q.GetIssue(ctx, resourceID)
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
		runtime, e := (RuntimeLookup{Queries: q, Metrics: access.Metrics, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, task.RuntimeID)
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
		if kind == "agent" && util.UUIDToString(runtime.OwnerID) != p.UserID {
			return binding, artifactDeny()
		}
		ids = append(ids, binding.OriginatorUserID, util.UUIDToString(runtime.OwnerID))
	}
	members, err := LockArtifactUsers(ctx, q, ws, ids...)
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
	if err = CheckArtifactCredentialInTx(ctx, tx, access.Credential); err != nil {
		return binding, err
	}
	return binding, nil
}

func artifactTimeMatches(raw string, value pgtype.Timestamptz) bool {
	t, err := time.Parse(time.RFC3339Nano, raw)
	return err == nil && value.Valid && t.Equal(value.Time)
}

// ReadArtifactSpec validates the retained canonical specification and all row bindings.
func ReadArtifactSpec(e db.AgentArtifactExecution) (artifact.Spec, error) {
	s, err := artifact.ParseSpec(e.CanonicalSpec)
	if err != nil {
		return s, err
	}
	_, digest, err := artifact.CanonicalSpec(s)
	if err != nil {
		return s, err
	}
	if digest != e.SpecDigest || s.Request.AgentID != util.UUIDToString(e.AgentID) || s.Request.TaskID != util.UUIDToString(e.TaskID) || s.Request.ResourceID != util.UUIDToString(e.ResourceID) || s.Binding.WorkspaceID != util.UUIDToString(e.WorkspaceID) || s.Binding.InitiatorKind != e.InitiatorKind || s.Binding.InitiatorID != util.UUIDToString(e.InitiatorID) || s.Binding.OriginatorUserID != util.UUIDToString(e.OriginatorUserID) || s.Binding.RuntimeID != util.UUIDToString(e.RuntimeID) || s.Binding.TaskGeneration != e.TaskGeneration || s.RootIdentity != e.RootConfigIdentity || s.InvocationDigest != e.InvocationDigest || s.Request.ResourceKind != e.ResourceKind || s.Request.Action != e.Action || s.Request.SourceRevision != e.SourceRevision || s.Request.StagingID != e.StagingID || s.Request.InputManifestDigest != e.InputManifestDigest || s.Request.StagingManifestDigest != e.StagingManifestDigest || s.Request.TargetEnvironment != e.TargetEnvironment || s.Request.PolicyVersion != e.PolicyVersion || !artifactTimeMatches(s.CreatedAt, e.CreatedAt) || !artifactTimeMatches(s.ExpiresAt, e.ExpiresAt) {
		return s, artifactConflict()
	}
	return s, nil
}
