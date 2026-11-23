package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationOfferResourceParams struct {
	CollaborationQueueRequest
	ExecutionWorkspaceID pgtype.UUID
	Kind                 collaboration.TargetKind
	ResourceID           pgtype.UUID
	Roster               []collaboration.TargetResource
}

type CollaborationWithdrawResourceOfferParams struct {
	CollaborationQueueRequest
	OfferID              pgtype.UUID
	ExpectedOfferVersion int64
}

type CollaborationAdoptResourceOfferParams struct {
	CollaborationQueueRequest
	OfferID                  pgtype.UUID
	ExpectedOfferVersion     int64
	ExpectedTargetSetVersion int64
}

// Internal authority is retained with the explicit disclosure, never sent in
// an offer DTO. Restoring a participant, account or binding does not revive it.
type collaborationOfferedAuthority struct {
	AgentID               string                        `json:"agent_id"`
	OwnerUserID           string                        `json:"owner_user_id"`
	OwnerAuthEpoch        int64                         `json:"owner_auth_epoch,string"`
	OwnerParticipantEpoch int64                         `json:"owner_participant_epoch,string"`
	Identity              CollaborationResourceIdentity `json:"identity"`
}

func normalizeCollaborationOfferRoster(p CollaborationOfferResourceParams) ([]collaboration.TargetResource, bool) {
	if !p.ExecutionWorkspaceID.Valid || !p.ResourceID.Valid || !validCollaborationRoster(p.Kind, p.ResourceID, p.Roster) {
		return nil, false
	}
	roster := append([]collaboration.TargetResource(nil), p.Roster...)
	for i, resource := range roster {
		agent, runtime, owner, workspace, err := collaborationResourceUUIDs(resource)
		if err != nil || workspace != p.ExecutionWorkspaceID {
			return nil, false
		}
		roster[i] = collaboration.TargetResource{Role: resource.Role, AgentID: util.UUIDToString(agent), RuntimeID: util.UUIDToString(runtime),
			OwnerUserID: util.UUIDToString(owner), ExecutionWorkspaceID: util.UUIDToString(workspace)}
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i].AgentID < roster[j].AgentID })
	return roster, true
}

// OfferResource accepts only the caller's explicitly visible, complete roster.
// The adapter predicate is trusted server code and is not part of the payload.
func (s *CollaborationService) OfferResource(ctx context.Context, p CollaborationOfferResourceParams,
	visible func(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool) (CollaborationQueueReceipt, error) {
	roster, valid := normalizeCollaborationOfferRoster(p)
	if !valid {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_configuration")
	}
	p.Roster = roster
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "offer_resource", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if !t.access.Capabilities.SubmitCommand {
			return t.receipt("issue_closed", http.StatusConflict), nil
		}
		if err := lockCollaborationResources(ctx, t.q, p.Kind, p.ResourceID, p.Roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		// The inventory predicate can depend on the provider's workspace role.
		// KEY SHARE in the owner-resource fence does not prevent role updates.
		if _, err := t.q.TryLockCollaborationResourceOfferMember(ctx, db.TryLockCollaborationResourceOfferMemberParams{WorkspaceID: p.ExecutionWorkspaceID, UserID: t.actor}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return t.receipt("resource_workspace_unavailable", http.StatusNotFound), nil
			}
			return CollaborationQueueReceipt{}, err
		}
		code, err := s.collaborationOfferRosterVisible(ctx, t.q, t.actor, p.ExecutionWorkspaceID, p.Roster, visible)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			status := http.StatusNotFound
			if code == "resource_not_owned" {
				status = http.StatusForbidden
			}
			return t.receipt(code, status), nil
		}
		if code, err = t.validateResourceOwners(ctx, p.Roster); err != nil || code != "" {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return t.receipt(code, http.StatusConflict), nil
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, p.Kind, p.ResourceID, p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return t.receipt(code, http.StatusConflict), nil
		}
		authorities, code, err := s.collaborationOfferAuthorities(ctx, t.q, p.IssueID, p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return t.receipt(code, http.StatusConflict), nil
		}
		for i, authority := range authorities {
			if err = lockCollaborationResourceIdentity(ctx, t.q, authority.Identity); err != nil {
				return CollaborationQueueReceipt{}, err
			}
			current, err := s.currentCollaborationResourceIdentity(ctx, t.q, p.Roster[i])
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			if current == nil || *current != authority.Identity {
				return t.receipt("runtime_registration_required", http.StatusConflict), nil
			}
		}
		encodedRoster, err := json.Marshal(p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		encodedAuthority, err := json.Marshal(authorities)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		offer, err := t.q.CreateCollaborationResourceOffer(ctx, db.CreateCollaborationResourceOfferParams{IssueID: p.IssueID, OfferedBy: t.actor,
			LifecycleEpoch: t.access.LifecycleEpoch, Kind: string(p.Kind), ResourceID: p.ResourceID, ExecutionWorkspaceID: p.ExecutionWorkspaceID,
			Roster: encodedRoster, ConfigurationFingerprint: configuration.Fingerprint, ResourceAuthority: encodedAuthority})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.offerID, t.offerVersion = offer.ID, offer.Version
		r := t.receipt("resource_offered", http.StatusCreated)
		return r, t.event(ctx, "resource_offered", r)
	})
}

func (s *CollaborationService) WithdrawResourceOffer(ctx context.Context, p CollaborationWithdrawResourceOfferParams) (CollaborationQueueReceipt, error) {
	if !p.OfferID.Valid || p.ExpectedOfferVersion < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resource_offer")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "withdraw_resource_offer", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		offer, err := t.q.LockCollaborationResourceOffer(ctx, db.LockCollaborationResourceOfferParams{IssueID: p.IssueID, ID: p.OfferID})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && offer.OfferedBy != t.actor {
			return t.receipt("resource_offer_not_found", http.StatusNotFound), nil
		}
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.offerVersion = offer.Version
		if code := collaborationResourceOfferTransition(offer, p.ExpectedOfferVersion); code != "" {
			return t.receipt(code, http.StatusConflict), nil
		}
		// Stale resources and closed issues remain withdrawable. Authority here
		// is the current participant's, not the offered configuration's.
		offer, err = t.q.WithdrawCollaborationResourceOffer(ctx, db.WithdrawCollaborationResourceOfferParams{
			IssueID: p.IssueID, OfferID: p.OfferID, OfferedBy: t.actor, ExpectedVersion: p.ExpectedOfferVersion})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.offerVersion = offer.Version
		r := t.receipt("resource_offer_withdrawn", http.StatusOK)
		return r, t.event(ctx, "resource_offer_withdrawn", r)
	})
}

func (s *CollaborationService) AdoptResourceOffer(ctx context.Context, p CollaborationAdoptResourceOfferParams) (CollaborationQueueReceipt, error) {
	if !p.OfferID.Valid || p.ExpectedOfferVersion < 1 || p.ExpectedTargetSetVersion < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resource_offer")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "adopt_resource_offer", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if !t.access.Capabilities.Invite {
			return t.receipt("forbidden", http.StatusForbidden), nil
		}
		if !t.access.Capabilities.SubmitCommand {
			return t.receipt("issue_closed", http.StatusConflict), nil
		}
		offer, err := t.q.LockCollaborationResourceOffer(ctx, db.LockCollaborationResourceOfferParams{IssueID: p.IssueID, ID: p.OfferID})
		if errors.Is(err, pgx.ErrNoRows) {
			return t.receipt("resource_offer_not_found", http.StatusNotFound), nil
		}
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.offerVersion = offer.Version
		if code := collaborationResourceOfferTransition(offer, p.ExpectedOfferVersion); code != "" {
			return t.receipt(code, http.StatusConflict), nil
		}
		roster, authority, err := decodeCollaborationResourceOffer(offer)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = lockCollaborationResources(ctx, t.q, collaboration.TargetKind(offer.Kind), offer.ResourceID, roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		for _, resource := range authority {
			if err = lockCollaborationResourceIdentity(ctx, t.q, resource.Identity); err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		_, code, err := s.collaborationResourceOfferCurrent(ctx, t.q, t.access, offer, roster, authority)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return t.receipt(code, http.StatusConflict), nil
		}
		r, err := t.registerTarget(ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: p.CollaborationQueueRequest,
			Kind: collaboration.TargetKind(offer.Kind), ResourceID: offer.ResourceID, ExpectedTargetSetVersion: p.ExpectedTargetSetVersion, Roster: roster})
		if err != nil || r.Status >= http.StatusBadRequest {
			return r, err
		}
		offer, err = t.q.AdoptCollaborationResourceOffer(ctx, db.AdoptCollaborationResourceOfferParams{IssueID: p.IssueID, OfferID: p.OfferID,
			ExpectedVersion: p.ExpectedOfferVersion, AdoptedBy: t.actor, TargetID: r.TargetID})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.offerVersion = offer.Version
		r.Code, r.OfferVersion = "resource_offer_adopted", offer.Version
		return r, t.event(ctx, "resource_offer_adopted", r)
	})
}

func collaborationResourceOfferTransition(offer db.CollaborationResourceOffer, expected int64) string {
	if offer.Version != expected {
		return "stale_offer"
	}
	switch offer.State {
	case "offered":
		return ""
	case "withdrawn":
		return "offer_withdrawn"
	case "adopted":
		return "offer_adopted"
	default:
		return "stale_offer"
	}
}

func (s *CollaborationService) collaborationOfferAuthorities(ctx context.Context, q *db.Queries, issueID pgtype.UUID, roster []collaboration.TargetResource) ([]collaborationOfferedAuthority, string, error) {
	result := make([]collaborationOfferedAuthority, 0, len(roster))
	for _, resource := range roster {
		owner, err := util.ParseUUID(resource.OwnerUserID)
		if err != nil {
			return nil, "", err
		}
		participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: issueID, UserID: owner})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && participant.RevokedAt.Valid {
			return nil, "resource_owner_not_participant", nil
		}
		if err != nil {
			return nil, "", err
		}
		account, err := q.GetCollaborationAccountFence(ctx, owner)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "resource_owner_unavailable", nil
		}
		if err != nil {
			return nil, "", err
		}
		identity, err := s.currentCollaborationResourceIdentity(ctx, q, resource)
		if err != nil {
			return nil, "", err
		}
		if identity == nil {
			return nil, "runtime_registration_required", nil
		}
		result = append(result, collaborationOfferedAuthority{AgentID: resource.AgentID, OwnerUserID: resource.OwnerUserID,
			OwnerAuthEpoch: account.Epoch, OwnerParticipantEpoch: participant.Epoch, Identity: *identity})
	}
	return result, "", nil
}

func decodeCollaborationResourceOffer(offer db.CollaborationResourceOffer) ([]collaboration.TargetResource, []collaborationOfferedAuthority, error) {
	roster, err := decodeCollaborationRoster[collaboration.TargetResource](offer.Roster)
	if err != nil {
		return nil, nil, err
	}
	authority, err := decodeCollaborationRoster[collaborationOfferedAuthority](offer.ResourceAuthority)
	if err != nil {
		return nil, nil, err
	}
	if !validCollaborationRoster(collaboration.TargetKind(offer.Kind), offer.ResourceID, roster) || len(authority) != len(roster) {
		return nil, nil, invalidCollaborationLedger("invalid offered resource configuration")
	}
	owned := false
	owners := make(map[string]collaborationOfferedAuthority)
	for i, resource := range roster {
		a := authority[i]
		if a.AgentID != resource.AgentID || a.OwnerUserID != resource.OwnerUserID || a.OwnerAuthEpoch < 1 || a.OwnerParticipantEpoch < 1 ||
			!validCollaborationResourceIdentity(a.Identity) || resource.ExecutionWorkspaceID != util.UUIDToString(offer.ExecutionWorkspaceID) ||
			i > 0 && roster[i-1].AgentID >= resource.AgentID {
			return nil, nil, invalidCollaborationLedger("invalid offered resource authority")
		}
		if previous, exists := owners[a.OwnerUserID]; exists && (previous.OwnerAuthEpoch != a.OwnerAuthEpoch || previous.OwnerParticipantEpoch != a.OwnerParticipantEpoch) {
			return nil, nil, invalidCollaborationLedger("inconsistent offered owner authority")
		}
		owners[a.OwnerUserID] = a
		owned = owned || resource.OwnerUserID == util.UUIDToString(offer.OfferedBy)
	}
	if !owned {
		return nil, nil, invalidCollaborationLedger("resource offer was not supplied by an owner")
	}
	return roster, authority, nil
}

// This helper reads current facts only. Writes hold all resource and identity
// fences first; reads borrow the authorized repeatable-read content snapshot.
func (s *CollaborationService) collaborationResourceOfferCurrent(ctx context.Context, q *db.Queries, access CollaborationIssueAccess,
	offer db.CollaborationResourceOffer, roster []collaboration.TargetResource, authority []collaborationOfferedAuthority) (collaborationConfiguration, string, error) {
	var configuration collaborationConfiguration
	if offer.LifecycleEpoch != access.LifecycleEpoch {
		return configuration, "stale_offer", nil
	}
	t := &collaborationQueueTx{q: q, access: access}
	if code, err := t.validateResourceOwners(ctx, roster); code != "" || err != nil {
		return configuration, code, err
	}
	configuration, code, err := collaborationConfigurationSnapshot(ctx, q, collaboration.TargetKind(offer.Kind), offer.ResourceID, roster)
	if code != "" || err != nil {
		return configuration, code, err
	}
	if configuration.Fingerprint != offer.ConfigurationFingerprint {
		return configuration, "stale_offer", nil
	}
	current, code, err := s.collaborationOfferAuthorities(ctx, q, offer.IssueID, roster)
	if code != "" || err != nil {
		return configuration, code, err
	}
	for i := range authority {
		if current[i] != authority[i] {
			return configuration, "stale_offer", nil
		}
	}
	return configuration, "", nil
}

func validCollaborationOfferReceipt(action string, issueID pgtype.UUID, requestID string, r CollaborationQueueReceipt) bool {
	switch action {
	case "offer_resource", "withdraw_resource_offer", "adopt_resource_offer":
	default:
		return true
	}
	if r.Action != action || r.Redacted || r.IssueID != issueID || r.RequestID != requestID || r.ProtocolVersion != collaboration.ExecutionProtocolVersion ||
		r.Channels == nil || r.TargetHolds == nil || r.Scope != collaboration.ControlIssue && r.Scope != collaboration.ControlTarget ||
		r.OfferVersion < 0 || r.OfferVersion > 2 || r.OfferVersion > 0 && r.OfferID == nil {
		return false
	}
	if r.OfferID != nil {
		if _, err := util.ParseUUID(*r.OfferID); err != nil {
			return false
		}
	}
	if r.Status >= http.StatusBadRequest {
		return r.Status <= 599 && r.Code != "resource_offered" && r.Code != "resource_offer_adopted" && r.Code != "resource_offer_withdrawn"
	}
	if r.OfferID == nil {
		return false
	}
	switch action {
	case "offer_resource":
		return r.Status == http.StatusCreated && r.Code == "resource_offered" && r.OfferVersion == 1 && r.Scope == collaboration.ControlIssue && !r.TargetID.Valid
	case "withdraw_resource_offer":
		return r.Status == http.StatusOK && r.Code == "resource_offer_withdrawn" && r.OfferVersion == 2 && r.Scope == collaboration.ControlIssue && !r.TargetID.Valid
	default:
		return r.Status == http.StatusCreated && r.Code == "resource_offer_adopted" && r.OfferVersion == 2 && r.Scope == collaboration.ControlTarget && r.TargetID.Valid
	}
}
