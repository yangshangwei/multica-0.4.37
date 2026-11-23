package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationSwitchResourceView struct {
	collaboration.TargetResource
	// v6 consent forms need the same execution summary as the resources view.
	CollaborationResourceExecution
	AgentName  string                        `json:"agent_name"`
	Runtime    *CollaborationResourceRuntime `json:"runtime"`
	Owner      *CollaborationContentAccount  `json:"owner"`
	Consented  bool                          `json:"consented"`
	Revoked    bool                          `json:"revoked"`
	GrantID    *string                       `json:"grant_id"`
	GrantEpoch *string                       `json:"grant_epoch"`
}

// This view excludes configuration bodies/fingerprints and installation IDs.
// Recorded consent is distinct from current readiness, which Status describes.
type CollaborationSwitchProposalView struct {
	ProposalID            pgtype.UUID                       `json:"proposal_id"`
	TargetID              pgtype.UUID                       `json:"target_id"`
	ProposedBy            pgtype.UUID                       `json:"proposed_by"`
	LifecycleEpoch        int64                             `json:"lifecycle_epoch,string"`
	BaseTargetEpoch       int64                             `json:"base_target_epoch,string"`
	BaseConfigVersion     int64                             `json:"base_config_version,string"`
	ProposedTargetEpoch   int64                             `json:"proposed_target_epoch,string"`
	ProposedConfigVersion int64                             `json:"proposed_config_version,string"`
	Status                string                            `json:"status"`
	Reason                string                            `json:"reason"`
	Resources             []CollaborationSwitchResourceView `json:"resources"`
}

func (s *CollaborationService) ListTargetSwitchProposals(ctx context.Context, issueID, targetID pgtype.UUID) ([]CollaborationSwitchProposalView, error) {
	return s.readTargetSwitchProposals(ctx, issueID, targetID, pgtype.UUID{})
}

func (s *CollaborationService) GetTargetSwitchProposal(ctx context.Context, issueID, targetID, proposalID pgtype.UUID) (CollaborationSwitchProposalView, error) {
	if !proposalID.Valid {
		return CollaborationSwitchProposalView{}, collaborationError(http.StatusBadRequest, "invalid_switch_proposal")
	}
	views, err := s.readTargetSwitchProposals(ctx, issueID, targetID, proposalID)
	if err != nil {
		return CollaborationSwitchProposalView{}, err
	}
	if len(views) != 1 {
		return CollaborationSwitchProposalView{}, collaborationError(http.StatusNotFound, "switch_proposal_not_found")
	}
	return views[0], nil
}

func (s *CollaborationService) readTargetSwitchProposals(ctx context.Context, issueID, targetID, proposalID pgtype.UUID) ([]CollaborationSwitchProposalView, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return nil, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || !targetID.Valid {
		return nil, collaborationError(http.StatusBadRequest, "invalid_switch_proposal")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return nil, err
	}
	q := s.Queries.WithTx(tx)
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return nil, err
	}
	if !access.Capabilities.ControlAI {
		return nil, collaborationError(http.StatusNotFound, "not_found")
	}
	targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
	if err != nil {
		return nil, err
	}
	var target db.CollaborationTarget
	for _, candidate := range targets {
		if candidate.ID == targetID {
			target = candidate
			break
		}
	}
	if !target.ID.Valid {
		return nil, collaborationError(http.StatusNotFound, "target_not_found")
	}
	var proposals []db.CollaborationSwitchProposal
	if proposalID.Valid {
		proposal, err := q.GetCollaborationSwitchProposal(ctx, db.GetCollaborationSwitchProposalParams{ID: proposalID, IssueID: issueID, TargetID: targetID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, collaborationError(http.StatusNotFound, "switch_proposal_not_found")
		}
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, proposal)
	} else {
		proposals, err = q.ListCollaborationSwitchProposals(ctx, db.ListCollaborationSwitchProposalsParams{IssueID: issueID, TargetID: targetID})
		if err != nil {
			return nil, err
		}
	}
	views := make([]CollaborationSwitchProposalView, 0, len(proposals))
	for _, proposal := range proposals {
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](proposal.Roster)
		if err != nil {
			return nil, err
		}
		allowed := access.Capabilities.Invite
		for _, resource := range roster {
			allowed = allowed || resource.OwnerUserID == util.UUIDToString(actor)
		}
		if !allowed {
			continue
		}
		consents, err := q.ListCollaborationSwitchConsents(ctx, proposal.ID)
		if err != nil {
			return nil, err
		}
		var grants []db.CollaborationGrant
		if proposal.CommittedAt.Valid && target.TargetEpoch == proposal.BaseTargetEpoch+1 && target.ConfigVersion == proposal.BaseConfigVersion+1 {
			grants, err = q.ListCollaborationTargetGrants(ctx, target.ID)
			if err != nil {
				return nil, err
			}
		}
		view := CollaborationSwitchProposalView{ProposalID: proposal.ID, TargetID: proposal.TargetID, ProposedBy: proposal.ProposedBy,
			LifecycleEpoch: proposal.LifecycleEpoch, BaseTargetEpoch: proposal.BaseTargetEpoch, BaseConfigVersion: proposal.BaseConfigVersion,
			ProposedTargetEpoch: proposal.BaseTargetEpoch + 1, ProposedConfigVersion: proposal.BaseConfigVersion + 1,
			Status: "pending", Resources: make([]CollaborationSwitchResourceView, 0, len(roster))}
		for _, resource := range roster {
			summary, err := s.collaborationResourceSummary(ctx, q, access, resource)
			if err != nil {
				return nil, err
			}
			item := CollaborationSwitchResourceView{TargetResource: resource, CollaborationResourceExecution: summary.CollaborationResourceExecution,
				AgentName: summary.AgentName, Runtime: summary.Runtime, Owner: summary.Owner}
			for _, consent := range consents {
				if resource.AgentID == util.UUIDToString(consent.AgentID) {
					item.Consented, item.Revoked = !consent.RevokedAt.Valid, consent.RevokedAt.Valid
				}
			}
			if resource.OwnerUserID == util.UUIDToString(actor) {
				for _, grant := range grants {
					if grant.RevokedAt.Valid || grant.IssueID != issueID || grant.ConfigVersion.Int64 != proposal.BaseConfigVersion+1 ||
						grant.TargetEpoch.Int64 != proposal.BaseTargetEpoch+1 || grant.ConfigurationFingerprint.String != proposal.ConfigurationFingerprint ||
						!collaborationResourceMatchesGrant(resource, grant) {
						continue
					}
					if item.GrantID != nil {
						return nil, invalidCollaborationLedger("ambiguous committed Switch grant")
					}
					epoch := strconv.FormatInt(grant.Epoch, 10)
					item.GrantID, item.GrantEpoch = util.UUIDToPtr(grant.ID), &epoch
				}
			}
			view.Resources = append(view.Resources, item)
		}
		switch {
		case proposal.CommittedAt.Valid:
			view.Status = "committed"
		case switchProposalCurrent(proposal, target, access.LifecycleEpoch) != "":
			view.Status, view.Reason = "stale", "stale_switch_proposal"
		default:
			configuration, code, err := collaborationConfigurationSnapshot(ctx, q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
			if err != nil {
				return nil, err
			}
			if code != "" || configuration.Fingerprint != proposal.ConfigurationFingerprint {
				view.Status, view.Reason = "stale", "stale_switch_proposal"
			} else {
				code, err = authorizeSwitchConsents(ctx, q, proposal, roster, consents)
				if err != nil {
					return nil, err
				}
				switch code {
				case "":
					view.Status = "consent_ready"
				case "switch_consent_required":
					view.Reason = code
				default:
					view.Status, view.Reason = "stale", code
				}
			}
		}
		views = append(views, view)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return views, nil
}

func (s *CollaborationService) GetSwitchOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationQueueReceipt, error) {
	return s.getCollaborationResourceOperation(ctx, issueID, requestID, "propose_target_switch", "consent_target_switch", "revoke_target_switch_consent", "commit_target_switch")
}

// Resource writes share actor-scoped receipt recovery, including the exact
// content-free outcome after participation is lost. The route pins allowed kinds.
func (s *CollaborationService) getCollaborationResourceOperation(ctx context.Context, issueID pgtype.UUID, requestID string, kinds ...string) (CollaborationQueueReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || requestID == "" || len(requestID) > 128 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_queue_request")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	// Account authority survives departure, while access to execution state
	// does not. Reuse the account fence before reading only this actor's key.
	users, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{actor})
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if len(users) != 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if err = authorizeWorkflowAccount(ctx, q, actor, users[0]); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if !slices.Contains(kinds, op.Kind) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if !op.CompletedAt.Valid {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	var receipt CollaborationQueueReceipt
	if err = json.Unmarshal(op.Result, &receipt); err != nil || receipt.Code == "" || receipt.Code != op.ResultCode.String ||
		receipt.Status < 200 || receipt.Status > 599 || receipt.IssueID != issueID || receipt.RequestID != requestID || receipt.Action != op.Kind ||
		receipt.ProtocolVersion != collaboration.ExecutionProtocolVersion || receipt.Channels == nil || receipt.TargetHolds == nil ||
		!validCollaborationOfferReceipt(op.Kind, issueID, requestID, receipt) {
		return CollaborationQueueReceipt{}, errors.New("invalid collaboration resource receipt")
	}
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	var accessDenied *CollaborationError
	if err != nil && (!errors.As(err, &accessDenied) || accessDenied.Code != "not_found") {
		return CollaborationQueueReceipt{}, err
	}
	if err != nil || !access.Capabilities.ControlAI {
		receipt = CollaborationQueueReceipt{Status: receipt.Status, Code: receipt.Code, Redacted: true,
			ProtocolVersion: receipt.ProtocolVersion, IssueID: issueID, RequestID: requestID, Action: receipt.Action}
	}
	if err = tx.Commit(ctx); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	return receipt, nil
}
