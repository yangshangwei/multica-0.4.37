package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationInformationOrigin struct {
	OriginRequestID pgtype.UUID `json:"origin_request_id"`
	OriginEventID   pgtype.UUID `json:"origin_event_id"`
	Status          string      `json:"status"`
}

type CollaborationInformationPending struct {
	SubmissionID      pgtype.UUID `json:"submission_id"`
	SubmissionEventID pgtype.UUID `json:"submission_event_id"`
	CreatorUserID     pgtype.UUID `json:"creator_user_id"`
}

type CollaborationInformationRepairContext struct {
	ProtocolVersion       int                              `json:"protocol_version"`
	ContextVersion        string                           `json:"context_version"`
	ActiveRequestID       pgtype.UUID                      `json:"active_request_id"`
	HistoricalOrigin      *CollaborationInformationOrigin  `json:"historical_origin"`
	PendingSubmission     *CollaborationInformationPending `json:"pending_submission"`
	HumanSelectionAllowed bool                             `json:"human_selection_allowed"`
}

type CollaborationInformationRepairBasis struct {
	Kind              string       `json:"kind"`
	OriginRequestID   *pgtype.UUID `json:"origin_request_id,omitempty"`
	OriginEventID     *pgtype.UUID `json:"origin_event_id,omitempty"`
	SubmissionID      *pgtype.UUID `json:"submission_id,omitempty"`
	SubmissionEventID *pgtype.UUID `json:"submission_event_id,omitempty"`
}

func informationUUID(value *pgtype.UUID) bool {
	return value != nil && value.Valid && value.Bytes != [16]byte{}
}

func (basis CollaborationInformationRepairBasis) valid() bool {
	switch basis.Kind {
	case "human_selected":
		return basis.OriginRequestID == nil && basis.OriginEventID == nil && basis.SubmissionID == nil && basis.SubmissionEventID == nil
	case "historical_information":
		return informationUUID(basis.OriginRequestID) && informationUUID(basis.OriginEventID) && basis.SubmissionID == nil && basis.SubmissionEventID == nil
	case "pending_submission":
		return informationUUID(basis.SubmissionID) && informationUUID(basis.SubmissionEventID) && basis.OriginRequestID == nil && basis.OriginEventID == nil
	default:
		return false
	}
}

type CollaborationInformationRepairSelection struct {
	ContextVersion  string                              `json:"context_version"`
	ActiveRequestID pgtype.UUID                         `json:"active_request_id"`
	Basis           CollaborationInformationRepairBasis `json:"basis"`
}

type CollaborationInformationRepairParams struct {
	CollaborationTransferParams
	ExpectedStatus       string
	StatusOptionsVersion string
	InformationRepair    CollaborationInformationRepairSelection
}

type CollaborationInformationRepairConfirmation struct {
	RepairHandoffID      pgtype.UUID `json:"repair_handoff_id"`
	PriorActiveRequestID pgtype.UUID `json:"prior_active_request_id"`
	Status               string      `json:"status"`
	HandlerUserID        pgtype.UUID `json:"handler_user_id"`
	SupersededCount      string      `json:"superseded_count"`
}

func informationNonnegative(value string) bool {
	parsed, err := strconv.ParseInt(value, 10, 64)
	return err == nil && parsed >= 0 && strconv.FormatInt(parsed, 10) == value
}

func (confirmation *CollaborationInformationRepairConfirmation) valid() bool {
	return confirmation != nil && informationUUID(&confirmation.RepairHandoffID) && informationUUID(&confirmation.HandlerUserID) &&
		confirmation.Status != "" && informationNonnegative(confirmation.SupersededCount) &&
		(!confirmation.PriorActiveRequestID.Valid || confirmation.PriorActiveRequestID.Bytes != [16]byte{})
}

func informationRepairOptions(ctx context.Context, q *db.Queries, state *informationState, catalog []db.IssueStatus) (CollaborationStatusOptions, error) {
	out := CollaborationStatusOptions{Action: "repair_information", DefaultStatus: "", Options: []CollaborationStatusOption{}}
	if !state.Policy.Managed || state.Access.StatusCategory != "blocked" || state.Access.QueueState == "closed" {
		return out, collaborationError(http.StatusConflict, "information_repair_not_applicable")
	}
	if !state.Access.Capabilities.Transfer || !state.Access.Capabilities.ChangeStatus {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	if state.PendingUnknown {
		return out, collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
	}
	context := &CollaborationInformationRepairContext{ProtocolVersion: 1, ContextVersion: state.ContextVersion,
		ActiveRequestID: state.RawActive, HumanSelectionAllowed: state.Pending == nil}
	if state.Pending != nil {
		eligible, err := informationReturnEligible(ctx, q, state.Access, state.Access.Issue.CreatorID)
		if err != nil {
			return out, err
		}
		if !eligible || state.PendingEvent == nil {
			return out, collaborationError(http.StatusConflict, "return_target_unavailable")
		}
		context.PendingSubmission = &CollaborationInformationPending{SubmissionID: state.Pending.ID,
			SubmissionEventID: state.PendingEvent.ID, CreatorUserID: state.Access.Issue.CreatorID}
	} else {
		context.HistoricalOrigin = state.Origin
	}
	for _, entry := range catalog {
		if entry.ArchivedAt.Valid {
			continue
		}
		allowed := entry.Category == "backlog" || entry.Category == "todo" || entry.Category == "in_progress"
		if state.Pending != nil {
			allowed = entry.Category == "in_review"
		} else if state.Origin != nil && entry.Key == state.Origin.Status {
			allowed = collaboration.NonterminalCategory(entry.Category) && entry.Category != "blocked"
		}
		if allowed {
			out.Options = append(out.Options, CollaborationStatusOption{Key: entry.Key, Name: entry.Name, Category: entry.Category})
		}
	}
	if len(out.Options) == 0 {
		return out, collaborationError(http.StatusConflict, "status_unavailable")
	}
	sort.Slice(out.Options, func(i, j int) bool { return out.Options[i].Key < out.Options[j].Key })
	out.InformationRepairContext = context
	catalogFacts := slices.Clone(catalog)
	sort.Slice(catalogFacts, func(i, j int) bool { return catalogFacts[i].Key < catalogFacts[j].Key })
	encoded, err := json.Marshal(struct {
		Source  pgtype.UUID
		Context *CollaborationInformationRepairContext
		Catalog []db.IssueStatus
		Options []CollaborationStatusOption
	}{state.Access.Issue.WorkspaceID, context, catalogFacts, out.Options})
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(encoded)
	out.StatusOptionsVersion = hex.EncodeToString(digest[:])
	return out, nil
}

func (s *CollaborationService) RepairInformation(ctx context.Context, p CollaborationInformationRepairParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, p.TargetUserID, "repair_information", p,
		func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
			if !t.access.Capabilities.Transfer || !t.access.Capabilities.ChangeStatus {
				return t.result("forbidden", http.StatusForbidden), nil
			}
			if !informationText(p.Note) || !p.InformationRepair.Basis.valid() || !stageDigest(p.InformationRepair.ContextVersion) ||
				!informationUUID(&p.TargetUserID) || !validateCollaborationStatusInput(p.ExpectedStatus, p.StatusOptionsVersion) {
				return t.result("information_metadata_invalid", http.StatusBadRequest), nil
			}
			state, err := loadInformationState(ctx, t.q, t.access, t.catalog, true)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if !state.Policy.Managed || t.access.StatusCategory != "blocked" || t.session.QueueState == "closed" {
				return t.result("information_repair_not_applicable", http.StatusConflict), nil
			}
			if state.ContextVersion != p.InformationRepair.ContextVersion || state.RawActive != p.InformationRepair.ActiveRequestID {
				return t.result("information_context_changed", http.StatusConflict), nil
			}
			options, err := informationRepairOptions(ctx, t.q, state, t.catalog)
			if err != nil {
				var refusal *CollaborationError
				if errors.As(err, &refusal) {
					return t.result(refusal.Code, refusal.Status), nil
				}
				return CollaborationWorkflowReceipt{}, err
			}
			if options.StatusOptionsVersion != p.StatusOptionsVersion {
				r := t.result("status_options_changed", http.StatusConflict)
				r.StatusOptions = &options
				return r, nil
			}
			var selected *CollaborationStatusOption
			for i := range options.Options {
				if options.Options[i].Key == p.ExpectedStatus {
					selected = &options.Options[i]
				}
			}
			if selected == nil {
				return t.result("status_unavailable", http.StatusConflict), nil
			}
			basis := p.InformationRepair.Basis
			if state.Pending != nil {
				if basis.Kind != "pending_submission" || state.PendingEvent == nil || *basis.SubmissionID != state.Pending.ID ||
					*basis.SubmissionEventID != state.PendingEvent.ID || p.TargetUserID != t.access.Issue.CreatorID || selected.Category != "in_review" {
					return t.result("information_repair_basis_unavailable", http.StatusConflict), nil
				}
			} else {
				switch basis.Kind {
				case "human_selected":
					if selected.Category != "backlog" && selected.Category != "todo" && selected.Category != "in_progress" {
						return t.result("information_repair_basis_unavailable", http.StatusConflict), nil
					}
				case "historical_information":
					if state.Origin == nil || *basis.OriginRequestID != state.Origin.OriginRequestID || *basis.OriginEventID != state.Origin.OriginEventID || p.ExpectedStatus != state.Origin.Status {
						return t.result("information_repair_basis_unavailable", http.StatusConflict), nil
					}
				default:
					return t.result("information_repair_basis_unavailable", http.StatusConflict), nil
				}
			}
			eligible, err := t.eligibleRecipient(ctx, p.TargetUserID)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if !eligible {
				return t.result("target_unavailable", http.StatusConflict), nil
			}
			badLinks, err := t.q.CountCollaborationInformationOpenRepairLinks(ctx, t.access.Issue.ID)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if badLinks != 0 {
				return t.result("information_context_unavailable", http.StatusConflict), nil
			}
			if _, err = t.enroll(ctx, p.TargetUserID); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			previous := t.access.Issue.Status
			handoff, err := t.handoff(ctx, collaborationHandoff{target: p.TargetUserID, purpose: "transfer", note: p.Note,
				status: p.ExpectedStatus, keepSameHandler: true})
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			count, err := t.q.SupersedeCollaborationInformationForRepair(ctx, db.SupersedeCollaborationInformationForRepairParams{
				IssueID: t.access.Issue.ID, RepairHandoffID: handoff.ID})
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if err = t.q.ClearCollaborationInformationRequest(ctx, t.access.Issue.ID); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			t.access.InformationRequestID = pgtype.UUID{}
			t.informationRepairConfirmation = &CollaborationInformationRepairConfirmation{RepairHandoffID: handoff.ID,
				PriorActiveRequestID: state.RawActive, Status: p.ExpectedStatus, HandlerUserID: p.TargetUserID, SupersededCount: strconv.FormatInt(count, 10)}
			return t.finishWith(ctx, "information_repaired", p.TargetUserID, nil, map[string]any{
				"prior_active_request_id": state.RawActive, "previous_status": previous, "status": p.ExpectedStatus,
				"basis": basis, "superseded_count": strconv.FormatInt(count, 10)})
		})
}
