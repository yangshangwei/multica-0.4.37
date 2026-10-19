package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationResourceRuntime struct {
	ID         pgtype.UUID `json:"id"`
	Name       string      `json:"name"`
	CustomName *string     `json:"custom_name"`
	Provider   string      `json:"provider"`
}

type CollaborationResourceView struct {
	collaboration.TargetResource
	AgentName  string                        `json:"agent_name"`
	Runtime    *CollaborationResourceRuntime `json:"runtime"`
	Owner      *CollaborationContentAccount  `json:"owner"`
	Consented  bool                          `json:"consented"`
	GrantID    *string                       `json:"grant_id"`
	GrantEpoch *string                       `json:"grant_epoch"`
	CanConsent bool                          `json:"can_consent"`
	CanRevoke  bool                          `json:"can_revoke"`
}

type CollaborationTargetResources struct {
	TargetID      pgtype.UUID                 `json:"target_id"`
	Kind          collaboration.TargetKind    `json:"kind"`
	ResourceID    pgtype.UUID                 `json:"resource_id"`
	Name          string                      `json:"name"`
	TargetEpoch   int64                       `json:"target_epoch,string"`
	ConfigVersion int64                       `json:"config_version,string"`
	Authorized    bool                        `json:"authorized"`
	Reason        string                      `json:"reason"`
	Resources     []CollaborationResourceView `json:"resources"`
}

type CollaborationResourcesView struct {
	ProtocolVersion  int                            `json:"protocol_version"`
	IssueID          pgtype.UUID                    `json:"issue_id"`
	LifecycleEpoch   int64                          `json:"lifecycle_epoch,string"`
	TargetSetVersion int64                          `json:"target_set_version,string"`
	CanOrganize      bool                           `json:"can_organize"`
	Targets          []CollaborationTargetResources `json:"targets"`
}

// Resources projects only this issue's retained configuration. It is not a
// resource inventory or a launch authorization. Installation identities have a
// separate owner-only reader; a stale grant remains withdrawable by its owner.
func (s *CollaborationService) Resources(ctx context.Context, issueID pgtype.UUID) (CollaborationResourcesView, error) {
	out := CollaborationResourcesView{ProtocolVersion: 1, IssueID: issueID, Targets: make([]CollaborationTargetResources, 0)}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if !access.Capabilities.ControlAI {
			return collaborationError(http.StatusNotFound, "resource_access_denied")
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		session, err := q.GetCollaborationSession(ctx, issueID)
		if err != nil {
			return err
		}
		out.LifecycleEpoch, out.TargetSetVersion = access.LifecycleEpoch, session.TargetSetVersion
		out.CanOrganize = access.Capabilities.Invite && access.Capabilities.SubmitCommand
		targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
		if err != nil {
			return err
		}
		for _, target := range targets {
			roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
			if err != nil {
				return err
			}
			configuration, code, err := collaborationConfigurationSnapshot(ctx, q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
			if err != nil {
				return err
			}
			configurationCurrent := code == "" && target.ConfigurationFingerprint.Valid && configuration.Fingerprint == target.ConfigurationFingerprint.String && !target.RevokedAt.Valid
			_, authorizationCode, err := authorizeCollaborationTargetInput(ctx, q, issueID, target)
			if err != nil {
				return err
			}
			item := CollaborationTargetResources{TargetID: target.ID, Kind: collaboration.TargetKind(target.Kind), ResourceID: target.ResourceID,
				Name: configuration.Name, TargetEpoch: target.TargetEpoch, ConfigVersion: target.ConfigVersion, Authorized: authorizationCode == "", Reason: authorizationCode,
				Resources: make([]CollaborationResourceView, 0, len(roster))}
			grants, err := q.ListCollaborationTargetGrants(ctx, target.ID)
			if err != nil {
				return err
			}
			for _, resource := range roster {
				view, err := s.collaborationResourceSummary(ctx, q, access, resource)
				if err != nil {
					return err
				}
				owned := resource.OwnerUserID == principal.UserID
				for _, grant := range grants {
					if grant.RevokedAt.Valid || grant.ConfigVersion.Int64 != target.ConfigVersion || !collaborationResourceMatchesGrant(resource, grant) {
						continue
					}
					current, err := collaborationResourceConsentCurrent(ctx, q, issueID, target, resource, grant)
					if err != nil {
						return err
					}
					view.Consented = configurationCurrent && current
					if owned {
						if view.GrantID != nil {
							return invalidCollaborationLedger("ambiguous resource consent")
						}
						epoch := strconv.FormatInt(grant.Epoch, 10)
						view.GrantID, view.GrantEpoch, view.CanRevoke = util.UUIDToPtr(grant.ID), &epoch, true
					}
				}
				view.CanConsent = owned && configurationCurrent && access.Capabilities.SubmitCommand && !view.Consented && !view.CanRevoke
				item.Resources = append(item.Resources, view)
			}
			out.Targets = append(out.Targets, item)
		}
		return nil
	})
	return out, err
}

func (s *CollaborationService) collaborationResourceSummary(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, resource collaboration.TargetResource) (CollaborationResourceView, error) {
	view := CollaborationResourceView{TargetResource: resource}
	agentID, runtimeID, ownerID, _, err := collaborationResourceUUIDs(resource)
	if err != nil {
		return view, err
	}
	agent, err := q.GetAgent(ctx, agentID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return view, err
	}
	if err == nil {
		view.AgentName = agent.Name
	}
	runtime, err := s.collaborationRuntimeLookup(q).Get(ctx, runtimeID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return view, err
	}
	if err == nil && runtime.OwnerID == ownerID && util.UUIDToString(runtime.WorkspaceID) == resource.ExecutionWorkspaceID {
		var custom *string
		if runtime.CustomName.Valid {
			custom = &runtime.CustomName.String
		}
		view.Runtime = &CollaborationResourceRuntime{ID: runtime.ID, Name: runtime.Name, CustomName: custom, Provider: runtime.Provider}
	}
	accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, []pgtype.UUID{ownerID})
	if err != nil {
		return view, err
	}
	view.Owner = accounts[ownerID]
	return view, nil
}

// This is a read projection of the persisted consent facts, not an admission
// shortcut: all writes continue through authorizeCollaborationTargetInput.
func collaborationResourceConsentCurrent(ctx context.Context, q *db.Queries, issueID pgtype.UUID, target db.CollaborationTarget, resource collaboration.TargetResource, grant db.CollaborationGrant) (bool, error) {
	if grant.IssueID != issueID || grant.TargetEpoch.Int64 != target.TargetEpoch || !grant.TargetEpoch.Valid || !grant.OwnerParticipantEpoch.Valid ||
		grant.ConfigurationFingerprint != target.ConfigurationFingerprint {
		return false, nil
	}
	participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: issueID, UserID: grant.OwnerUserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if participant.RevokedAt.Valid || participant.Epoch != grant.OwnerParticipantEpoch.Int64 {
		return false, nil
	}
	current, err := q.CollaborationGrantOwnerCurrent(ctx, grant.ID)
	if err != nil || !current || auth.IsTemporarilyDisabledUserID(resource.OwnerUserID) {
		return false, err
	}
	eligible, err := collaborationRecipientEligible(ctx, q, grant.OwnerUserID, grant.ExecutionWorkspaceID)
	if err != nil || !eligible {
		return false, err
	}
	return collaborationIdentityCurrent(ctx, q, resource, collaborationGrantIdentity(grant))
}

type CollaborationResourceIdentityView struct {
	IssueID    pgtype.UUID                    `json:"issue_id"`
	TargetID   pgtype.UUID                    `json:"target_id"`
	AgentID    pgtype.UUID                    `json:"agent_id"`
	ProposalID pgtype.UUID                    `json:"proposal_id"`
	Identity   *CollaborationResourceIdentity `json:"identity"`
	Reason     string                         `json:"reason"`
}

func (s *CollaborationService) ResourceIdentity(ctx context.Context, issueID, targetID, agentID, proposalID pgtype.UUID) (CollaborationResourceIdentityView, error) {
	out := CollaborationResourceIdentityView{IssueID: issueID, TargetID: targetID, AgentID: agentID, ProposalID: proposalID}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if !access.Capabilities.ControlAI {
			return collaborationError(http.StatusNotFound, "resource_not_found")
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
		if err != nil {
			return err
		}
		var target db.CollaborationTarget
		for _, item := range targets {
			if item.ID == targetID {
				target = item
				break
			}
		}
		if !target.ID.Valid {
			return collaborationError(http.StatusNotFound, "resource_not_found")
		}
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](target.Roster)
		if err != nil {
			return err
		}
		fingerprint := target.ConfigurationFingerprint.String
		stale := target.RevokedAt.Valid
		if proposalID.Valid {
			proposal, err := q.GetCollaborationSwitchProposal(ctx, db.GetCollaborationSwitchProposalParams{ID: proposalID, IssueID: issueID, TargetID: targetID})
			if errors.Is(err, pgx.ErrNoRows) {
				return collaborationError(http.StatusNotFound, "resource_not_found")
			}
			if err != nil {
				return err
			}
			roster, err = decodeCollaborationRoster[collaboration.TargetResource](proposal.Roster)
			if err != nil {
				return err
			}
			fingerprint = proposal.ConfigurationFingerprint
			stale = proposal.CommittedAt.Valid || switchProposalCurrent(proposal, target, access.LifecycleEpoch) != ""
		}
		var selected collaboration.TargetResource
		for _, resource := range roster {
			if resource.AgentID == util.UUIDToString(agentID) && resource.OwnerUserID == principal.UserID {
				selected = resource
				break
			}
		}
		if selected.AgentID == "" {
			return collaborationError(http.StatusNotFound, "resource_not_found")
		}
		if !access.Capabilities.SubmitCommand {
			out.Reason = "issue_closed"
			return nil
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
		if err != nil {
			return err
		}
		if stale || code != "" || configuration.Fingerprint != fingerprint {
			out.Reason = "target_configuration_changed"
			return nil
		}
		out.Identity, err = s.currentCollaborationResourceIdentity(ctx, q, selected)
		if err != nil {
			return err
		}
		if out.Identity == nil {
			out.Reason = "runtime_registration_required"
		}
		return nil
	})
	return out, err
}

func (s *CollaborationService) currentCollaborationResourceIdentity(ctx context.Context, q *db.Queries, resource collaboration.TargetResource) (*CollaborationResourceIdentity, error) {
	_, runtimeID, ownerID, workspaceID, err := collaborationResourceUUIDs(resource)
	if err != nil {
		return nil, err
	}
	runtime, err := s.collaborationRuntimeLookup(q).Get(ctx, runtimeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if runtime.DaemonID.Valid && runtime.DaemonID.String != "" {
		binding, err := q.GetLatestInstallationBinding(ctx, db.GetLatestInstallationBindingParams{WorkspaceID: workspaceID, DaemonID: runtime.DaemonID.String})
		if err == nil {
			identity := CollaborationResourceIdentity{Kind: "managed", InstallationID: binding.InstallationID, BindingID: binding.ID, BindingEpoch: binding.BindingEpoch}
			current, err := collaborationIdentityCurrent(ctx, q, resource, identity)
			if err != nil || !current {
				return nil, err
			}
			return &identity, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	enrollments, err := q.ListCollaborationResourceEnrollments(ctx, db.ListCollaborationResourceEnrollmentsParams{RuntimeID: runtimeID, OwnerUserID: ownerID, ExecutionWorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	var selected *CollaborationResourceIdentity
	for _, enrollment := range enrollments {
		identity := CollaborationResourceIdentity{Kind: "enrolled", InstallationID: enrollment.InstallationID, BindingID: enrollment.ID, BindingEpoch: enrollment.BindingEpoch}
		current, err := collaborationIdentityCurrent(ctx, q, resource, identity)
		if err != nil {
			return nil, err
		}
		if !current {
			continue
		}
		if selected != nil {
			return nil, nil
		}
		selected = &identity
	}
	return selected, nil
}

func (s *CollaborationService) GetResourceOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationQueueReceipt, error) {
	return s.getCollaborationResourceOperation(ctx, issueID, requestID, "register_target", "consent_resource", "revoke_grant")
}
