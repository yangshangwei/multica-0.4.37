package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationProposeSwitchParams struct {
	CollaborationQueueRequest
	TargetID              pgtype.UUID
	ExpectedTargetEpoch   int64
	ExpectedConfigVersion int64
	Roster                []collaboration.TargetResource
}

type CollaborationSwitchConsentParams struct {
	CollaborationQueueRequest
	TargetID   pgtype.UUID
	ProposalID pgtype.UUID
	AgentID    pgtype.UUID
	Identity   CollaborationResourceIdentity
}

type CollaborationRevokeSwitchConsentParams struct {
	CollaborationQueueRequest
	TargetID   pgtype.UUID
	ProposalID pgtype.UUID
	AgentID    pgtype.UUID
}

type CollaborationCommitSwitchParams struct {
	CollaborationQueueRequest
	ProposalID pgtype.UUID
	Control    CollaborationTargetControl
}

func switchReceipt(t *collaborationQueueTx, request CollaborationQueueRequest, action, code string, status int, target db.CollaborationTarget, proposalID pgtype.UUID) CollaborationQueueReceipt {
	r := t.targetReceipt(code, status, target)
	r.RequestID, r.Action, r.ProposalID = request.RequestID, action, proposalID
	return r
}

// ProposeTargetSwitch freezes an independently consentable configuration. It
// deliberately does not change live grants, target hold or either channel.
func (s *CollaborationService) ProposeTargetSwitch(ctx context.Context, p CollaborationProposeSwitchParams) (CollaborationQueueReceipt, error) {
	if !p.TargetID.Valid || p.ExpectedTargetEpoch < 1 || p.ExpectedConfigVersion < 1 ||
		p.ExpectedTargetEpoch == math.MaxInt64 || p.ExpectedConfigVersion == math.MaxInt64 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_switch_proposal")
	}
	p.Roster = append([]collaboration.TargetResource(nil), p.Roster...)
	sort.Slice(p.Roster, func(i, j int) bool { return p.Roster[i].AgentID < p.Roster[j].AgentID })
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "propose_target_switch", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		target, exists := t.target(p.TargetID)
		deny := func(code string, status int) (CollaborationQueueReceipt, error) {
			return switchReceipt(t, p.CollaborationQueueRequest, "propose_target_switch", code, status, target, pgtype.UUID{}), nil
		}
		if !exists {
			return deny("target_not_found", http.StatusNotFound)
		}
		if !t.access.Capabilities.Invite {
			return deny("forbidden", http.StatusForbidden)
		}
		if !t.access.Capabilities.SubmitCommand {
			return deny("issue_closed", http.StatusConflict)
		}
		if target.TargetEpoch != p.ExpectedTargetEpoch || target.ConfigVersion != p.ExpectedConfigVersion {
			return deny("stale_target", http.StatusConflict)
		}
		if !validCollaborationRoster(collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster) {
			return deny("invalid_target_configuration", http.StatusBadRequest)
		}
		if code, err := t.validateResourceOwners(ctx, p.Roster); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code, http.StatusConflict)
		}
		if err := lockCollaborationResources(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" {
			return deny(code, http.StatusConflict)
		}
		roster, err := json.Marshal(p.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		proposal, err := t.q.CreateCollaborationSwitchProposal(ctx, db.CreateCollaborationSwitchProposalParams{
			IssueID: p.IssueID, SessionID: t.session.ID, TargetID: target.ID, ProposedBy: t.actor,
			LifecycleEpoch: t.access.LifecycleEpoch, BaseTargetEpoch: target.TargetEpoch, BaseConfigVersion: target.ConfigVersion,
			Roster: roster, ConfigurationFingerprint: configuration.Fingerprint,
		})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := switchReceipt(t, p.CollaborationQueueRequest, "propose_target_switch", "switch_proposed", http.StatusCreated, target, proposal.ID)
		return r, t.event(ctx, "switch_proposed", r)
	})
}

func switchProposalCurrent(proposal db.CollaborationSwitchProposal, target db.CollaborationTarget, lifecycle int64) string {
	if proposal.CommittedAt.Valid {
		return "switch_already_committed"
	}
	if proposal.TargetID != target.ID || proposal.SessionID != target.SessionID || proposal.LifecycleEpoch != lifecycle ||
		proposal.BaseTargetEpoch != target.TargetEpoch || proposal.BaseConfigVersion != target.ConfigVersion {
		return "stale_switch_proposal"
	}
	return ""
}

func switchConsentIdentity(consent db.CollaborationSwitchConsent) CollaborationResourceIdentity {
	return CollaborationResourceIdentity{Kind: consent.IdentityKind, InstallationID: consent.InstallationID, BindingID: consent.BindingID, BindingEpoch: consent.BindingEpoch}
}

func switchResource(roster []collaboration.TargetResource, agent pgtype.UUID) (collaboration.TargetResource, bool) {
	for _, resource := range roster {
		if resource.AgentID == util.UUIDToString(agent) {
			return resource, true
		}
	}
	return collaboration.TargetResource{}, false
}

// ConsentTargetSwitchResource is performed by the actual owner. No grant for
// the current live configuration can satisfy an immutable proposal's consent.
func (s *CollaborationService) ConsentTargetSwitchResource(ctx context.Context, p CollaborationSwitchConsentParams) (CollaborationQueueReceipt, error) {
	if !p.TargetID.Valid || !p.ProposalID.Valid || !p.AgentID.Valid || !validCollaborationResourceIdentity(p.Identity) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_switch_consent")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "consent_target_switch", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		target, exists := t.target(p.TargetID)
		deny := func(code string, status int) (CollaborationQueueReceipt, error) {
			return switchReceipt(t, p.CollaborationQueueRequest, "consent_target_switch", code, status, target, p.ProposalID), nil
		}
		if !exists {
			return deny("target_not_found", http.StatusNotFound)
		}
		if !t.access.Capabilities.SubmitCommand {
			return deny("issue_closed", http.StatusConflict)
		}
		proposal, err := t.q.GetCollaborationSwitchProposal(ctx, db.GetCollaborationSwitchProposalParams{ID: p.ProposalID, IssueID: p.IssueID, TargetID: p.TargetID})
		if errors.Is(err, pgx.ErrNoRows) {
			return deny("switch_proposal_not_found", http.StatusNotFound)
		}
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code := switchProposalCurrent(proposal, target, t.access.LifecycleEpoch); code != "" {
			return deny(code, http.StatusConflict)
		}
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](proposal.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		selected, exists := switchResource(roster, p.AgentID)
		if !exists || selected.OwnerUserID != util.UUIDToString(t.actor) {
			return deny("forbidden", http.StatusForbidden)
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
		if code != "" || configuration.Fingerprint != proposal.ConfigurationFingerprint {
			return deny("stale_switch_proposal", http.StatusConflict)
		}
		current, err := collaborationIdentityCurrent(ctx, t.q, selected, p.Identity)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if !current {
			return deny("runtime_registration_required", http.StatusConflict)
		}
		consents, err := t.q.ListCollaborationSwitchConsents(ctx, proposal.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		for _, prior := range consents {
			if prior.AgentID != p.AgentID {
				continue
			}
			if prior.RevokedAt.Valid || prior.OwnerAuthEpoch != t.account.Epoch || prior.OwnerParticipantEpoch != t.participant.Epoch ||
				prior.OwnerUserID != t.actor || switchConsentIdentity(prior) != p.Identity {
				return deny("stale_switch_consent", http.StatusConflict)
			}
			return deny("switch_resource_consented", http.StatusOK)
		}
		if _, err = t.q.CreateCollaborationSwitchConsent(ctx, db.CreateCollaborationSwitchConsentParams{ProposalID: proposal.ID, AgentID: p.AgentID,
			OwnerUserID: t.actor, OwnerAuthEpoch: t.account.Epoch, OwnerParticipantEpoch: t.participant.Epoch,
			IdentityKind: p.Identity.Kind, InstallationID: p.Identity.InstallationID, BindingID: p.Identity.BindingID, BindingEpoch: p.Identity.BindingEpoch}); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := switchReceipt(t, p.CollaborationQueueRequest, "consent_target_switch", "switch_resource_consented", http.StatusCreated, target, proposal.ID)
		return r, t.event(ctx, "switch_resource_consented", r)
	})
}

func (s *CollaborationService) RevokeTargetSwitchConsent(ctx context.Context, p CollaborationRevokeSwitchConsentParams) (CollaborationQueueReceipt, error) {
	if !p.TargetID.Valid || !p.ProposalID.Valid || !p.AgentID.Valid {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_switch_consent")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "revoke_target_switch_consent", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		target, exists := t.target(p.TargetID)
		deny := func(code string, status int) (CollaborationQueueReceipt, error) {
			return switchReceipt(t, p.CollaborationQueueRequest, "revoke_target_switch_consent", code, status, target, p.ProposalID), nil
		}
		if !exists {
			return deny("target_not_found", http.StatusNotFound)
		}
		proposal, err := t.q.GetCollaborationSwitchProposal(ctx, db.GetCollaborationSwitchProposalParams{ID: p.ProposalID, IssueID: p.IssueID, TargetID: p.TargetID})
		if errors.Is(err, pgx.ErrNoRows) {
			return deny("switch_proposal_not_found", http.StatusNotFound)
		}
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if proposal.CommittedAt.Valid {
			return deny("switch_already_committed", http.StatusConflict)
		}
		consents, err := t.q.ListCollaborationSwitchConsents(ctx, proposal.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		for _, consent := range consents {
			if consent.AgentID != p.AgentID {
				continue
			}
			if consent.OwnerUserID != t.actor {
				return deny("forbidden", http.StatusForbidden)
			}
			if consent.RevokedAt.Valid {
				return deny("switch_consent_revoked", http.StatusOK)
			}
			if _, err = t.q.RevokeCollaborationSwitchConsent(ctx, db.RevokeCollaborationSwitchConsentParams{ProposalID: p.ProposalID, AgentID: p.AgentID}); err != nil {
				return CollaborationQueueReceipt{}, err
			}
			r := switchReceipt(t, p.CollaborationQueueRequest, "revoke_target_switch_consent", "switch_consent_revoked", http.StatusOK, target, proposal.ID)
			return r, t.event(ctx, "switch_consent_revoked", r)
		}
		return deny("switch_consent_not_found", http.StatusNotFound)
	})
}

// authorizeSwitchConsents borrows already-held source/user/resource/identity
// fences on writes, or the caller's repeatable-read snapshot on reads.
func authorizeSwitchConsents(ctx context.Context, q *db.Queries, proposal db.CollaborationSwitchProposal, roster []collaboration.TargetResource, consents []db.CollaborationSwitchConsent) (string, error) {
	for _, resource := range roster {
		agent, _, owner, workspace, err := collaborationResourceUUIDs(resource)
		if err != nil {
			return "", err
		}
		var selected *db.CollaborationSwitchConsent
		for i := range consents {
			if consents[i].AgentID == agent {
				selected = &consents[i]
				break
			}
		}
		if selected == nil {
			return "switch_consent_required", nil
		}
		if selected.RevokedAt.Valid || selected.ProposalID != proposal.ID || selected.OwnerUserID != owner {
			return "stale_switch_consent", nil
		}
		participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: proposal.IssueID, UserID: owner})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (participant.RevokedAt.Valid || participant.Epoch != selected.OwnerParticipantEpoch) {
			return "stale_switch_consent", nil
		}
		if err != nil {
			return "", err
		}
		account, err := q.GetCollaborationAccountFence(ctx, owner)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && account.Epoch != selected.OwnerAuthEpoch {
			return "stale_switch_consent", nil
		}
		if err != nil {
			return "", err
		}
		eligible, err := collaborationRecipientEligible(ctx, q, owner, workspace)
		if err != nil {
			return "", err
		}
		if !eligible {
			return "stale_switch_consent", nil
		}
		current, err := collaborationIdentityCurrent(ctx, q, resource, switchConsentIdentity(*selected))
		if err != nil {
			return "", err
		}
		if !current {
			return "stale_switch_consent", nil
		}
	}
	return "", nil
}

func (s *CollaborationService) CommitTargetSwitch(ctx context.Context, p CollaborationCommitSwitchParams) (CollaborationQueueReceipt, error) {
	control := CollaborationStopQueueParams{Scope: collaboration.ControlTarget, Targets: []CollaborationTargetControl{p.Control}}
	if !p.ProposalID.Valid || !normalizeCollaborationControl(&control) || p.Control.ExpectedHoldVersion < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_target_switch")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "commit_target_switch", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		target, exists := t.target(p.Control.TargetID)
		deny := func(code string, status int) (CollaborationQueueReceipt, error) {
			return switchReceipt(t, p.CollaborationQueueRequest, "commit_target_switch", code, status, target, p.ProposalID), nil
		}
		if !exists {
			return deny("target_not_found", http.StatusNotFound)
		}
		if !t.access.Capabilities.Invite {
			return deny("forbidden", http.StatusForbidden)
		}
		if !t.access.Capabilities.SubmitCommand {
			return deny("issue_closed", http.StatusConflict)
		}
		proposal, err := t.q.GetCollaborationSwitchProposal(ctx, db.GetCollaborationSwitchProposalParams{ID: p.ProposalID, IssueID: p.IssueID, TargetID: target.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return deny("switch_proposal_not_found", http.StatusNotFound)
		}
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code := switchProposalCurrent(proposal, target, t.access.LifecycleEpoch); code != "" {
			return deny(code, http.StatusConflict)
		}
		if code, err := t.checkControl(ctx, control); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code, http.StatusConflict)
		}
		if target.HoldVersion != p.Control.ExpectedHoldVersion {
			return deny("stale_hold", http.StatusConflict)
		}
		if target.QueueState != "paused" && !target.HoldReason.Valid {
			return deny("target_pause_required", http.StatusConflict)
		}
		if code, err := t.checkTargetSwitchProof(ctx, target); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code, http.StatusConflict)
		}
		roster, err := decodeCollaborationRoster[collaboration.TargetResource](proposal.Roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if _, err = t.q.LockCollaborationTargetGrants(ctx, target.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if err = lockCollaborationResources(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, roster); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		consents, err := t.q.ListCollaborationSwitchConsents(ctx, proposal.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		for _, consent := range consents {
			if err = lockCollaborationResourceIdentity(ctx, t.q, switchConsentIdentity(consent)); err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		configuration, code, err := collaborationConfigurationSnapshot(ctx, t.q, collaboration.TargetKind(target.Kind), target.ResourceID, roster)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if code != "" || configuration.Fingerprint != proposal.ConfigurationFingerprint {
			return deny("stale_switch_proposal", http.StatusConflict)
		}
		if code, err = authorizeSwitchConsents(ctx, t.q, proposal, roster, consents); code != "" || err != nil {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return deny(code, http.StatusConflict)
		}
		// Every refusal above leaves live business state unchanged. Any error
		// below rolls back the whole Switch, including grants and waiting moves.
		if err = t.q.RevokeCollaborationTargetConsents(ctx, db.RevokeCollaborationTargetConsentsParams{TargetID: target.ID, Reason: "configuration_switched"}); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		target, err = t.q.ReconfigureCollaborationTarget(ctx, db.ReconfigureCollaborationTargetParams{TargetID: target.ID, SessionID: t.session.ID, Roster: proposal.Roster, ConfigurationFingerprint: proposal.ConfigurationFingerprint})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		t.replaceTarget(target)
		for _, consent := range consents {
			resource, exists := switchResource(roster, consent.AgentID)
			if !exists {
				return CollaborationQueueReceipt{}, invalidCollaborationLedger("Switch consent outside proposal roster")
			}
			_, runtime, owner, workspace, err := collaborationResourceUUIDs(resource)
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			if _, err = t.q.CreateCollaborationResourceConsent(ctx, db.CreateCollaborationResourceConsentParams{
				IssueID: p.IssueID, TargetID: target.ID, ConfigVersion: target.ConfigVersion, TargetEpoch: target.TargetEpoch,
				Role: resource.Role, OwnerUserID: owner, OwnerAuthEpoch: consent.OwnerAuthEpoch, OwnerParticipantEpoch: consent.OwnerParticipantEpoch,
				ConfigurationFingerprint: proposal.ConfigurationFingerprint, ExecutionWorkspaceID: workspace, AgentID: consent.AgentID, RuntimeID: runtime,
				IdentityKind: consent.IdentityKind, InstallationID: consent.InstallationID, BindingID: consent.BindingID, BindingEpoch: consent.BindingEpoch,
			}); err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		target, err = t.moveReconfiguredTarget(ctx, target)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if _, err = t.q.CompleteCollaborationSwitchProposal(ctx, proposal.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := switchReceipt(t, p.CollaborationQueueRequest, "commit_target_switch", "target_switched", http.StatusOK, target, proposal.ID)
		return r, t.event(ctx, "target_switched", r)
	})
}
