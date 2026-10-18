package service

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Resource ownership consent and runtime launch support are separate facts.
// These fields cannot grant a provider permission to launch or read content.
type CollaborationMentionTarget struct {
	TargetID           pgtype.UUID              `json:"target_id"`
	Kind               collaboration.TargetKind `json:"kind"`
	ResourceID         pgtype.UUID              `json:"resource_id"`
	Name               string                   `json:"name"`
	TargetEpoch        int64                    `json:"target_epoch,string"`
	ConfigVersion      int64                    `json:"config_version,string"`
	QueueState         string                   `json:"queue_state"`
	WaitingReason      string                   `json:"waiting_reason"`
	ExecutionAvailable bool                     `json:"execution_available"`
}

type CollaborationResourceIdentity struct {
	Kind           string      `json:"kind"`
	InstallationID pgtype.UUID `json:"installation_id"`
	BindingID      pgtype.UUID `json:"binding_id"`
	BindingEpoch   int64       `json:"binding_epoch,string"`
}

type CollaborationConsentParams struct {
	CollaborationQueueRequest
	TargetID              pgtype.UUID
	AgentID               pgtype.UUID
	ExpectedTargetEpoch   int64
	ExpectedConfigVersion int64
	Identity              CollaborationResourceIdentity
}

type CollaborationReconfigureTargetParams struct {
	CollaborationQueueRequest
	Control CollaborationTargetControl
	Roster  []collaboration.TargetResource
}

type collaborationResourceSnapshot struct {
	Resource       collaboration.TargetResource `json:"resource"`
	AgentEpoch     int64                        `json:"agent_epoch,string"`
	RuntimeEpoch   int64                        `json:"runtime_epoch,string"`
	AgentCreated   pgtype.Timestamptz           `json:"agent_created"`
	RuntimeCreated pgtype.Timestamptz           `json:"runtime_created"`
	MembershipID   pgtype.UUID                  `json:"membership_id"`
	Invocation     []db.AgentInvocationTarget   `json:"invocation"`
}

type collaborationConfiguration struct {
	Fingerprint string
	Name        string
	Offline     bool
}

func collaborationResourceUUIDs(resource collaboration.TargetResource) (agent, runtime, owner, workspace pgtype.UUID, err error) {
	values := []struct {
		text string
		out  *pgtype.UUID
	}{
		{resource.AgentID, &agent}, {resource.RuntimeID, &runtime}, {resource.OwnerUserID, &owner}, {resource.ExecutionWorkspaceID, &workspace},
	}
	for _, value := range values {
		*value.out, err = util.ParseUUID(value.text)
		if err != nil {
			return
		}
	}
	return
}

// The fingerprint only uses authority and execution configuration. Names,
// heartbeat/status, capacity counters and human squad metadata are excluded.
func collaborationConfigurationSnapshot(ctx context.Context, q *db.Queries, kind collaboration.TargetKind, resourceID pgtype.UUID, roster []collaboration.TargetResource) (collaborationConfiguration, string, error) {
	var result collaborationConfiguration
	if !validCollaborationRoster(kind, resourceID, roster) {
		return result, "", invalidCollaborationLedger("invalid resource configuration")
	}
	snapshot := struct {
		Kind         collaboration.TargetKind        `json:"kind"`
		ResourceID   pgtype.UUID                     `json:"resource_id"`
		SquadEpoch   int64                           `json:"squad_epoch,string"`
		SquadCreated pgtype.Timestamptz              `json:"squad_created"`
		Members      []pgtype.UUID                   `json:"members"`
		Resources    []collaborationResourceSnapshot `json:"resources"`
	}{Kind: kind, ResourceID: resourceID}
	var squadWorkspace pgtype.UUID
	if kind == collaboration.TargetSquad {
		squad, err := q.GetSquad(ctx, resourceID)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && squad.ArchivedAt.Valid {
			return result, "target_configuration_changed", nil
		}
		if err != nil {
			return result, "", err
		}
		members, err := q.ListSquadMembers(ctx, resourceID)
		if err != nil {
			return result, "", err
		}
		expected := map[pgtype.UUID]bool{squad.LeaderID: true}
		for _, member := range members {
			if member.MemberType == "agent" {
				expected[member.MemberID] = true
				snapshot.Members = append(snapshot.Members, member.ID)
			}
		}
		if len(expected) != len(roster) {
			return result, "target_configuration_changed", nil
		}
		for _, resource := range roster {
			agent, _, _, _, err := collaborationResourceUUIDs(resource)
			if err != nil {
				return result, "", err
			}
			if !expected[agent] || (resource.Role == "leader") != (agent == squad.LeaderID) {
				return result, "target_configuration_changed", nil
			}
		}
		sort.Slice(snapshot.Members, func(i, j int) bool {
			return util.UUIDToString(snapshot.Members[i]) < util.UUIDToString(snapshot.Members[j])
		})
		result.Name, snapshot.SquadEpoch, snapshot.SquadCreated, squadWorkspace = squad.Name, squad.CollaborationConfigEpoch, squad.CreatedAt, squad.WorkspaceID
	}
	for _, resource := range roster {
		agentID, runtimeID, ownerID, workspaceID, err := collaborationResourceUUIDs(resource)
		if err != nil {
			return result, "", err
		}
		if squadWorkspace.Valid && squadWorkspace != workspaceID {
			return result, "target_configuration_changed", nil
		}
		agent, err := q.GetAgent(ctx, agentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, "target_configuration_changed", nil
		}
		if err != nil {
			return result, "", err
		}
		runtime, err := (RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, runtimeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, "target_configuration_changed", nil
		}
		if err != nil {
			return result, "", err
		}
		if agent.WorkspaceID != workspaceID || agent.RuntimeID != runtimeID || agent.ArchivedAt.Valid || agent.Kind != "user" ||
			runtime.WorkspaceID != workspaceID || runtime.OwnerID != ownerID || !runtime.OwnerID.Valid {
			return result, "target_configuration_changed", nil
		}
		member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{WorkspaceID: workspaceID, UserID: ownerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return result, "target_configuration_changed", nil
		}
		if err != nil {
			return result, "", err
		}
		invocation, err := q.ListAgentInvocationTargets(ctx, agent.ID)
		if err != nil {
			return result, "", err
		}
		canInvoke := agent.OwnerID == ownerID
		if agent.PermissionMode == "public_to" {
			for _, target := range invocation {
				if target.TargetType == "member" && target.TargetID == ownerID || target.TargetType == "workspace" && target.TargetID == workspaceID {
					canInvoke = true
				}
			}
		}
		if !canInvoke {
			return result, "target_configuration_changed", nil
		}
		sort.Slice(invocation, func(i, j int) bool { return util.UUIDToString(invocation[i].ID) < util.UUIDToString(invocation[j].ID) })
		snapshot.Resources = append(snapshot.Resources, collaborationResourceSnapshot{Resource: resource,
			AgentEpoch: agent.CollaborationConfigEpoch, RuntimeEpoch: runtime.CollaborationConfigEpoch,
			AgentCreated: agent.CreatedAt, RuntimeCreated: runtime.CreatedAt, MembershipID: member.ID, Invocation: invocation})
		result.Offline = result.Offline || runtime.Status != "online"
		if kind == collaboration.TargetAgent {
			result.Name = agent.Name
		}
	}
	sort.Slice(snapshot.Resources, func(i, j int) bool {
		return snapshot.Resources[i].Resource.AgentID < snapshot.Resources[j].Resource.AgentID
	})
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return result, "", err
	}
	digest := sha256.Sum256(encoded)
	result.Fingerprint = hex.EncodeToString(digest[:])
	return result, "", nil
}

func validCollaborationResourceIdentity(identity CollaborationResourceIdentity) bool {
	return (identity.Kind == "managed" || identity.Kind == "enrolled") && identity.InstallationID.Valid && identity.BindingID.Valid && identity.BindingEpoch > 0
}

func collaborationIdentityCurrent(ctx context.Context, q *db.Queries, resource collaboration.TargetResource, identity CollaborationResourceIdentity) (bool, error) {
	if !validCollaborationResourceIdentity(identity) || auth.ManagedDeploymentID() == "" {
		return false, nil
	}
	_, runtimeID, ownerID, workspaceID, err := collaborationResourceUUIDs(resource)
	if err != nil {
		return false, err
	}
	runtime, err := (RuntimeLookup{Queries: q, Source: obsmetrics.RuntimeLookupSourceTask}).Get(ctx, runtimeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if runtime.OwnerID != ownerID || runtime.WorkspaceID != workspaceID {
		return false, nil
	}
	if identity.Kind == "enrolled" {
		// A managed namespace never downgrades to a legacy enrollment after
		// revocation. History, not the presence of an active binding, decides.
		if runtime.DaemonID.Valid && runtime.DaemonID.String != "" {
			_, err = q.GetLatestInstallationBinding(ctx, db.GetLatestInstallationBindingParams{WorkspaceID: workspaceID, DaemonID: runtime.DaemonID.String})
			if err == nil {
				return false, nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return false, err
			}
		}
		enrollment, err := q.GetCollaborationRuntimeEnrollment(ctx, identity.BindingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return !enrollment.RevokedAt.Valid && enrollment.RuntimeID == runtimeID && enrollment.OwnerUserID == ownerID &&
			enrollment.ExecutionWorkspaceID == workspaceID && enrollment.InstallationID == identity.InstallationID && enrollment.BindingEpoch == identity.BindingEpoch &&
			util.UUIDToString(enrollment.DeploymentID) == auth.ManagedDeploymentID() && len(enrollment.PublicKey) == ed25519.PublicKeySize, nil
	}
	binding, err := q.GetInstallationBinding(ctx, identity.BindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if binding.InstallationID != identity.InstallationID || binding.BindingEpoch != identity.BindingEpoch || binding.PrincipalUserID != ownerID ||
		binding.WorkspaceID != workspaceID || !runtime.DaemonID.Valid || runtime.DaemonID.String != binding.DaemonID || binding.State != "active" || binding.RevokedAt.Valid {
		return false, nil
	}
	if _, err = auth.CheckPasswordVersion(ctx, q, resource.OwnerUserID, binding.AuthVersion); errors.Is(err, auth.ErrPasswordSession) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = auth.CheckManagedBinding(ctx, q, auth.PasswordSession{UserID: resource.OwnerUserID, Version: binding.AuthVersion, Kind: "daemon_token",
		WorkspaceID: resource.ExecutionWorkspaceID, DaemonID: binding.DaemonID, BindingID: util.UUIDToString(binding.ID), BindingEpoch: binding.BindingEpoch})
	if errors.Is(err, auth.ErrPasswordSession) {
		return false, nil
	}
	return err == nil, err
}

// These locks meet existing namespace/resource writers after source locks.
// NOWAIT rejects an inverted intersection; the transaction owner retries the
// whole operation. None of these locks may be replaced by a pool connection.
func lockCollaborationResources(ctx context.Context, q *db.Queries, kind collaboration.TargetKind, resourceID pgtype.UUID, roster []collaboration.TargetResource) error {
	var agents, runtimes, owners, workspaces []pgtype.UUID
	for _, resource := range roster {
		agent, runtime, owner, workspace, err := collaborationResourceUUIDs(resource)
		if err != nil {
			return err
		}
		agents, runtimes, owners, workspaces = append(agents, agent), append(runtimes, runtime), append(owners, owner), append(workspaces, workspace)
	}
	if _, err := q.TryLockCollaborationAuthorityWorkspaces(ctx, workspaces); err != nil {
		return err
	}
	if _, err := q.TryLockCollaborationAuthorityMembers(ctx, db.TryLockCollaborationAuthorityMembersParams{WorkspaceIds: workspaces, UserIds: owners}); err != nil {
		return err
	}
	if kind == collaboration.TargetSquad {
		if _, err := q.TryLockCollaborationAuthoritySquad(ctx, resourceID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if _, err := q.TryLockCollaborationAuthorityAgents(ctx, agents); err != nil {
		return err
	}
	if _, err := q.TryLockCollaborationAuthorityInvocationTargets(ctx, agents); err != nil {
		return err
	}
	rows, err := q.TryLockCollaborationAuthorityRuntimes(ctx, runtimes)
	if err != nil {
		return err
	}
	for _, runtime := range rows {
		if !runtime.DaemonID.Valid || runtime.DaemonID.String == "" {
			continue
		}
		acquired, err := q.TryLockCollaborationAuthorityNamespace(ctx, installationNamespace(runtime.WorkspaceID, runtime.DaemonID.String))
		if err != nil {
			return err
		}
		if !acquired {
			return &pgconn.PgError{Code: "55P03", Message: "collaboration resource namespace busy"}
		}
	}
	return nil
}

func lockCollaborationResourceIdentity(ctx context.Context, q *db.Queries, identity CollaborationResourceIdentity) error {
	if identity.Kind == "enrolled" {
		_, err := q.TryLockCollaborationAuthorityEnrollment(ctx, identity.BindingID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := q.TryLockCollaborationAuthorityBinding(ctx, identity.BindingID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err := q.TryLockCollaborationAuthorityInstallation(ctx, identity.InstallationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func collaborationGrantIdentity(grant db.CollaborationGrant) CollaborationResourceIdentity {
	return CollaborationResourceIdentity{Kind: grant.IdentityKind, InstallationID: grant.InstallationID, BindingID: grant.BindingID, BindingEpoch: grant.BindingEpoch}
}

func authorizeCollaborationTargetInput(ctx context.Context, q *db.Queries, issueID pgtype.UUID, target db.CollaborationTarget) (collaborationConfiguration, string, error) {
	var empty collaborationConfiguration
	if target.RevokedAt.Valid {
		return empty, "grant_required", nil
	}
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		return empty, "", err
	}
	configuration, code, err := collaborationConfigurationSnapshot(ctx, q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
	if code != "" || err != nil {
		return empty, code, err
	}
	if !target.ConfigurationFingerprint.Valid {
		return empty, "grant_required", nil
	}
	if configuration.Fingerprint != target.ConfigurationFingerprint.String {
		return empty, "target_configuration_changed", nil
	}
	grants, err := q.ListCollaborationTargetGrants(ctx, target.ID)
	if err != nil {
		return empty, "", err
	}
	for _, resource := range roster {
		var consent *db.CollaborationGrant
		for i := range grants {
			g := &grants[i]
			if g.RevokedAt.Valid || g.IssueID != issueID || g.ConfigVersion.Int64 != target.ConfigVersion ||
				!g.TargetEpoch.Valid || g.TargetEpoch.Int64 != target.TargetEpoch || !g.OwnerParticipantEpoch.Valid ||
				g.ConfigurationFingerprint.String != configuration.Fingerprint || !collaborationResourceMatchesGrant(resource, *g) {
				continue
			}
			if consent != nil {
				return empty, "", invalidCollaborationLedger("ambiguous resource consent")
			}
			consent = g
		}
		if consent == nil {
			return empty, "grant_required", nil
		}
		participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: issueID, UserID: consent.OwnerUserID})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (participant.RevokedAt.Valid || participant.Epoch != consent.OwnerParticipantEpoch.Int64) {
			return empty, "grant_required", nil
		}
		if err != nil {
			return empty, "", err
		}
		current, err := q.CollaborationGrantOwnerCurrent(ctx, consent.ID)
		if err != nil {
			return empty, "", err
		}
		if !current || auth.IsTemporarilyDisabledUserID(resource.OwnerUserID) {
			return empty, "grant_required", nil
		}
		eligible, err := collaborationRecipientEligible(ctx, q, consent.OwnerUserID, consent.ExecutionWorkspaceID)
		if err != nil {
			return empty, "", err
		}
		if !eligible {
			return empty, "grant_required", nil
		}
		current, err = collaborationIdentityCurrent(ctx, q, resource, collaborationGrantIdentity(*consent))
		if err != nil {
			return empty, "", err
		}
		if !current {
			return empty, "target_configuration_changed", nil
		}
	}
	return configuration, "", nil
}

// AuthorizeTargetInputInTx accepts an already locked issue/session/target set.
// Its empty code authorizes only durable waiting input, never execution.
func (s *CollaborationService) AuthorizeTargetInputInTx(ctx context.Context, tx pgx.Tx, issueID pgtype.UUID, target db.CollaborationTarget) (string, error) {
	return lockAndAuthorizeCollaborationTargetInput(ctx, s.Queries.WithTx(tx), issueID, target)
}

func lockAndAuthorizeCollaborationTargetInput(ctx context.Context, q *db.Queries, issueID pgtype.UUID, target db.CollaborationTarget) (string, error) {
	grants, err := q.LockCollaborationTargetGrants(ctx, target.ID)
	if err != nil {
		return "", err
	}
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
	if err != nil {
		return "", err
	}
	if err = lockCollaborationResources(ctx, q, collaboration.TargetKind(target.Kind), target.ResourceID, roster); err != nil {
		return "", err
	}
	for _, grant := range grants {
		if grant.RevokedAt.Valid || grant.ConfigVersion.Int64 != target.ConfigVersion {
			continue
		}
		if err = lockCollaborationResourceIdentity(ctx, q, collaborationGrantIdentity(grant)); err != nil {
			return "", err
		}
	}
	_, code, err := authorizeCollaborationTargetInput(ctx, q, issueID, target)
	return code, err
}

// AuthorizedMentionTargets never enumerates personal/workspace directories.
// The caller owns an authorized repeatable-read snapshot, including preview.
func (s *CollaborationService) AuthorizedMentionTargets(ctx context.Context, q *db.Queries, access CollaborationIssueAccess) ([]CollaborationMentionTarget, error) {
	result := make([]CollaborationMentionTarget, 0)
	if !access.Capabilities.Read || !access.Issue.ID.Valid {
		return nil, collaborationError(http.StatusNotFound, "not_found")
	}
	targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		configuration, code, err := authorizeCollaborationTargetInput(ctx, q, access.Issue.ID, target)
		if err != nil {
			return nil, err
		}
		if code != "" {
			continue
		}
		reason := "shared_runtime_unavailable"
		switch {
		case !access.Capabilities.SubmitCommand:
			reason = "command_unavailable"
		case access.QueueState != "active":
			reason = "issue_paused"
		case target.QueueState != "active":
			reason = "target_paused"
		case target.ActiveGroupID.Valid:
			reason = "target_busy"
		case configuration.Offline:
			reason = "runtime_offline"
		}
		result = append(result, CollaborationMentionTarget{TargetID: target.ID, Kind: collaboration.TargetKind(target.Kind), ResourceID: target.ResourceID,
			Name: configuration.Name, TargetEpoch: target.TargetEpoch, ConfigVersion: target.ConfigVersion, QueueState: target.QueueState, WaitingReason: reason})
	}
	return result, nil
}

// ConsentTargetResource records the actual owner's confirmation of one current
// target resource. Every squad resource is independently required; no member or
// direct-agent target is created by this operation.
func (s *CollaborationService) ConsentTargetResource(ctx context.Context, p CollaborationConsentParams) (CollaborationQueueReceipt, error) {
	if !p.TargetID.Valid || !p.AgentID.Valid || p.ExpectedTargetEpoch < 1 || p.ExpectedConfigVersion < 1 || !validCollaborationResourceIdentity(p.Identity) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resource_consent")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "consent_resource", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		target, ok := t.target(p.TargetID)
		if !ok {
			return t.receipt("target_not_found", http.StatusNotFound), nil
		}
		deny := func(code string, status int) (CollaborationQueueReceipt, error) {
			return t.targetReceipt(code, status, target), nil
		}
		if !t.access.Capabilities.SubmitCommand {
			return deny("issue_closed", http.StatusConflict)
		}
		if target.TargetEpoch != p.ExpectedTargetEpoch || target.ConfigVersion != p.ExpectedConfigVersion {
			return deny("stale_target", http.StatusConflict)
		}
		if target.RevokedAt.Valid {
			return deny("target_configuration_changed", http.StatusConflict)
		}
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		var selected collaboration.TargetResource
		for _, resource := range roster {
			if resource.AgentID == util.UUIDToString(p.AgentID) {
				selected = resource
				break
			}
		}
		if selected.AgentID == "" || selected.OwnerUserID != util.UUIDToString(t.actor) {
			return deny("forbidden", http.StatusForbidden)
		}
		grants, err := t.q.LockCollaborationTargetGrants(ctx, target.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = lockCollaborationResources(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = lockCollaborationResourceIdentity(ctx, t.q, p.Identity); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return deny(code, http.StatusConflict)
		}
		if !target.ConfigurationFingerprint.Valid || target.ConfigurationFingerprint.String != configuration.Fingerprint {
			return deny("target_configuration_changed", http.StatusConflict)
		}
		current, err := collaborationIdentityCurrent(ctx, t.q, selected, p.Identity)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if !current {
			return deny("runtime_registration_required", http.StatusConflict)
		}
		for _, prior := range grants {
			if prior.RevokedAt.Valid || prior.ConfigVersion.Int64 != target.ConfigVersion || !collaborationResourceMatchesGrant(selected, prior) {
				continue
			}
			if prior.OwnerAuthEpoch != t.account.Epoch || prior.OwnerParticipantEpoch.Int64 != t.participant.Epoch ||
				prior.TargetEpoch.Int64 != target.TargetEpoch || prior.ConfigurationFingerprint.String != configuration.Fingerprint || collaborationGrantIdentity(prior) != p.Identity {
				return deny("target_configuration_changed", http.StatusConflict)
			}
			r := t.targetReceipt("resource_consented", http.StatusOK, target)
			r.GrantID = prior.ID
			return r, nil
		}
		_, runtime, _, workspace, err := collaborationResourceUUIDs(selected)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		grant, err := t.q.CreateCollaborationResourceConsent(ctx, db.CreateCollaborationResourceConsentParams{
			IssueID: p.IssueID, TargetID: target.ID, ConfigVersion: target.ConfigVersion, Role: selected.Role,
			OwnerUserID: t.actor, OwnerAuthEpoch: t.account.Epoch, OwnerParticipantEpoch: t.participant.Epoch,
			TargetEpoch: target.TargetEpoch, ConfigurationFingerprint: configuration.Fingerprint,
			ExecutionWorkspaceID: workspace, AgentID: p.AgentID, RuntimeID: runtime,
			IdentityKind: p.Identity.Kind, InstallationID: p.Identity.InstallationID, BindingID: p.Identity.BindingID, BindingEpoch: p.Identity.BindingEpoch,
		})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := t.targetReceipt("resource_consented", http.StatusCreated, target)
		r.GrantID = grant.ID
		return r, t.event(ctx, "resource_consented", r)
	})
}

// ReconfigureTarget keeps the typed target and immutable input history. It
// requires the old resource's proof barrier to be clear and leaves it paused.
func (s *CollaborationService) ReconfigureTarget(ctx context.Context, p CollaborationReconfigureTargetParams) (CollaborationQueueReceipt, error) {
	control := CollaborationStopQueueParams{Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{p.Control}}
	if !normalizeCollaborationControl(&control) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_configuration")
	}
	p.Roster = append([]collaboration.TargetResource(nil), p.Roster...)
	sort.Slice(p.Roster, func(i, j int) bool { return p.Roster[i].AgentID < p.Roster[j].AgentID })
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "reconfigure_target", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if !t.access.Capabilities.Invite {
			return t.receipt("forbidden", http.StatusForbidden), nil
		}
		if !t.access.Capabilities.SubmitCommand {
			return t.receipt("issue_closed", http.StatusConflict), nil
		}
		if code, err := t.checkControl(ctx, control); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return t.controlReceipt(code, http.StatusConflict, control), nil
		}
		target, _ := t.target(p.Control.TargetID)
		deny := func(code string) (CollaborationQueueReceipt, error) {
			return t.targetReceipt(code, http.StatusConflict, target), nil
		}
		if !validCollaborationRoster(collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster) {
			return t.receipt("invalid_target_configuration", http.StatusBadRequest), nil
		}
		if target.QueueState != "paused" {
			return deny("target_pause_required")
		}
		if target.ActiveGroupID.Valid {
			return deny("stop_unconfirmed")
		}
		// The aggregate fence stabilizes these retained barriers without taking
		// group/run locks before resource-grant locks.
		groups, err := t.q.ListCollaborationExecutionGroups(ctx, t.session.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		for _, group := range groups {
			if group.TargetID == target.ID && !group.SettledAt.Valid {
				return deny("stop_unconfirmed")
			}
		}
		runs, err := t.q.ListCollaborationRuns(ctx, t.session.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		for _, run := range runs {
			if (run.TargetID == target.ID) && !run.SettledAt.Valid {
				return deny("stop_unconfirmed")
			}
		}
		if code, err := t.validateResourceOwners(ctx, p.Roster); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code)
		}
		if _, err = t.q.LockCollaborationTargetGrants(ctx, target.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = lockCollaborationResources(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return deny(code)
		}
		encoded, err := json.Marshal(p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = t.q.RevokeCollaborationTargetConsents(ctx, db.RevokeCollaborationTargetConsentsParams{TargetID: target.ID, Reason: "configuration_changed"}); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		target, err = t.q.ReconfigureCollaborationTarget(ctx, db.ReconfigureCollaborationTargetParams{TargetID: target.ID, SessionID: t.session.ID, Roster: encoded, ConfigurationFingerprint: configuration.Fingerprint})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.replaceTarget(target)
		r := t.targetReceipt("target_reconfigured", http.StatusOK, target)
		return r, t.event(ctx, "target_reconfigured", r)
	})
}

func (t *collaborationQueueTx) validateResourceOwners(ctx context.Context, roster []collaboration.TargetResource) (string, error) {
	for _, resource := range roster {
		_, _, owner, _, err := collaborationResourceUUIDs(resource)
		if err != nil {
			return "", err
		}
		participant, err := t.q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: t.access.Issue.ID, UserID: owner})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && participant.RevokedAt.Valid {
			return "resource_owner_not_participant", nil
		}
		if err != nil {
			return "", err
		}
		eligible, err := collaborationRecipientEligible(ctx, t.q, owner, t.access.Issue.WorkspaceID)
		if err != nil {
			return "", err
		}
		if !eligible {
			return "resource_owner_unavailable", nil
		}
	}
	return "", nil
}
