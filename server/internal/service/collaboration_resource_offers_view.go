package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationOfferedResource struct {
	collaboration.TargetResource
	AgentName string                        `json:"agent_name"`
	Runtime   *CollaborationResourceRuntime `json:"runtime"`
	Owner     *CollaborationContentAccount  `json:"owner"`
}

type CollaborationResourceOfferView struct {
	ProtocolVersion      int                            `json:"protocol_version"`
	IssueID              pgtype.UUID                    `json:"issue_id"`
	OfferID              pgtype.UUID                    `json:"offer_id"`
	Version              int64                          `json:"version,string"`
	State                string                         `json:"state"`
	OfferedBy            *CollaborationContentAccount   `json:"offered_by"`
	Kind                 collaboration.TargetKind       `json:"kind"`
	ResourceID           pgtype.UUID                    `json:"resource_id"`
	ExecutionWorkspaceID pgtype.UUID                    `json:"execution_workspace_id"`
	Name                 string                         `json:"name"`
	Resources            []CollaborationOfferedResource `json:"resources"`
	CreatedAt            time.Time                      `json:"created_at"`
	WithdrawnAt          pgtype.Timestamptz             `json:"withdrawn_at"`
	AdoptedAt            pgtype.Timestamptz             `json:"adopted_at"`
	AdoptedBy            *CollaborationContentAccount   `json:"adopted_by"`
	TargetID             pgtype.UUID                    `json:"target_id"`
	Available            bool                           `json:"available"`
	Reason               string                         `json:"reason"`
	CanAdopt             bool                           `json:"can_adopt"`
	CanWithdraw          bool                           `json:"can_withdraw"`
}

type CollaborationResourceOfferQuery struct {
	Limit           int
	BeforeCreatedAt pgtype.Timestamptz
	BeforeID        pgtype.UUID
}

type CollaborationResourceOffersPage struct {
	ProtocolVersion  int                              `json:"protocol_version"`
	IssueID          pgtype.UUID                      `json:"issue_id"`
	LifecycleEpoch   int64                            `json:"lifecycle_epoch,string"`
	TargetSetVersion int64                            `json:"target_set_version,string"`
	CanOffer         bool                             `json:"can_offer"`
	Items            []CollaborationResourceOfferView `json:"items"`
	NextCursor       *string                          `json:"next_cursor"`
	HasMore          bool                             `json:"-"`
}

func (s *CollaborationService) ListResourceOffers(ctx context.Context, issueID pgtype.UUID, p CollaborationResourceOfferQuery) (CollaborationResourceOffersPage, error) {
	out := CollaborationResourceOffersPage{ProtocolVersion: 1, IssueID: issueID, Items: make([]CollaborationResourceOfferView, 0)}
	if p.Limit < 1 || p.Limit > 100 || p.BeforeID.Valid != p.BeforeCreatedAt.Valid ||
		p.BeforeCreatedAt.Valid && (p.BeforeCreatedAt.Time.IsZero() || p.BeforeCreatedAt.InfinityModifier != pgtype.Finite) {
		return out, collaborationError(http.StatusBadRequest, "invalid_resource_offer_query")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if !access.Capabilities.ControlAI {
			return collaborationError(http.StatusNotFound, "resource_access_denied")
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		viewer, err := util.ParseUUID(principal.UserID)
		if err != nil {
			return err
		}
		session, err := q.GetCollaborationSession(ctx, issueID)
		if err != nil {
			return err
		}
		out.LifecycleEpoch, out.TargetSetVersion, out.CanOffer = access.LifecycleEpoch, session.TargetSetVersion, access.Capabilities.SubmitCommand
		rows, err := q.ListVisibleCollaborationResourceOffers(ctx, db.ListVisibleCollaborationResourceOffersParams{
			IssueID: issueID, ViewerID: viewer, CanOrganize: access.Capabilities.Invite, BeforeCreatedAt: p.BeforeCreatedAt,
			BeforeID: p.BeforeID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		if len(rows) > p.Limit {
			out.HasMore, rows = true, rows[:p.Limit]
		}
		targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			view, err := s.collaborationResourceOfferView(ctx, q, access, viewer, row, targets)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, view)
		}
		return nil
	})
	return out, err
}

func (s *CollaborationService) GetResourceOffer(ctx context.Context, issueID, offerID pgtype.UUID) (CollaborationResourceOfferView, error) {
	var out CollaborationResourceOfferView
	if !offerID.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_resource_offer")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if !access.Capabilities.ControlAI {
			return collaborationError(http.StatusNotFound, "resource_access_denied")
		}
		principal, _ := collaboration.PrincipalFromContext(ctx)
		viewer, err := util.ParseUUID(principal.UserID)
		if err != nil {
			return err
		}
		row, err := q.GetVisibleCollaborationResourceOffer(ctx, db.GetVisibleCollaborationResourceOfferParams{
			IssueID: issueID, OfferID: offerID, ViewerID: viewer, CanOrganize: access.Capabilities.Invite})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusNotFound, "resource_offer_not_found")
		}
		if err != nil {
			return err
		}
		targets, err := q.ListCollaborationTargets(ctx, access.SessionID)
		if err != nil {
			return err
		}
		out, err = s.collaborationResourceOfferView(ctx, q, access, viewer, row, targets)
		return err
	})
	return out, err
}

func (s *CollaborationService) collaborationResourceOfferView(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, viewer pgtype.UUID,
	offer db.CollaborationResourceOffer, targets []db.CollaborationTarget) (CollaborationResourceOfferView, error) {
	view := CollaborationResourceOfferView{ProtocolVersion: 1, IssueID: offer.IssueID, OfferID: offer.ID, Version: offer.Version,
		State: offer.State, Kind: collaboration.TargetKind(offer.Kind), ResourceID: offer.ResourceID, ExecutionWorkspaceID: offer.ExecutionWorkspaceID,
		CreatedAt: offer.CreatedAt.Time, WithdrawnAt: offer.WithdrawnAt, AdoptedAt: offer.AdoptedAt, TargetID: offer.TargetID,
		Resources: make([]CollaborationOfferedResource, 0), CanWithdraw: offer.State == "offered" && offer.OfferedBy == viewer}
	roster, authority, err := decodeCollaborationResourceOffer(offer)
	if err != nil {
		return view, err
	}
	accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, []pgtype.UUID{offer.OfferedBy, offer.AdoptedBy})
	if err != nil {
		return view, err
	}
	view.OfferedBy, view.AdoptedBy = accounts[offer.OfferedBy], accounts[offer.AdoptedBy]
	// Summaries use only IDs disclosed by the immutable offer. A changed squad
	// cannot cause its new members to appear in this historical resource list.
	for _, resource := range roster {
		summary, err := s.collaborationResourceSummary(ctx, q, access, resource)
		if err != nil {
			return view, err
		}
		view.Resources = append(view.Resources, CollaborationOfferedResource{TargetResource: resource, AgentName: summary.AgentName, Runtime: summary.Runtime, Owner: summary.Owner})
		if view.Kind == collaboration.TargetAgent {
			view.Name = summary.AgentName
		}
	}
	if view.Kind == collaboration.TargetSquad {
		squad, err := q.GetSquad(ctx, offer.ResourceID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return view, err
		}
		if err == nil {
			view.Name = squad.Name
		}
	}
	if code := collaborationResourceOfferTransition(offer, offer.Version); code != "" {
		view.Reason = code
		return view, nil
	}
	if !access.Capabilities.SubmitCommand {
		view.Reason = "issue_closed"
		return view, nil
	}
	_, view.Reason, err = s.collaborationResourceOfferCurrent(ctx, q, access, offer, roster, authority)
	if err != nil {
		return view, err
	}
	if view.Reason == "" {
		for _, target := range targets {
			if target.Kind == offer.Kind && target.ResourceID == offer.ResourceID {
				view.Reason = "target_exists"
				break
			}
		}
	}
	view.Available = view.Reason == ""
	view.CanAdopt = view.Available && access.Capabilities.Invite
	return view, nil
}
