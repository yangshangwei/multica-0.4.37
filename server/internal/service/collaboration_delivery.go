package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const collaborationDeliveryTextLimit = 65536

// A missing exact handoff link cannot be reconstructed from a timestamp.
// Reads may omit the new summary while retaining the ordinary human detail;
// writes must require explicit replacement instead of accepting this evidence.
var errCollaborationDeliveryEvidence = errors.New("collaboration delivery evidence is unavailable")

type CollaborationDeliveryConfirmation struct {
	SubmissionID  pgtype.UUID   `json:"submission_id"`
	AttachmentIDs []pgtype.UUID `json:"attachment_ids"`
}

type CollaborationDeliveryDecision struct {
	Kind         string                       `json:"kind"`
	SubmissionID pgtype.UUID                  `json:"submission_id"`
	EventSeq     int64                        `json:"event_seq,string"`
	ActorUserID  pgtype.UUID                  `json:"actor_user_id"`
	Actor        *CollaborationContentAccount `json:"actor"`
	CreatedAt    time.Time                    `json:"created_at"`
	Reason       *string                      `json:"reason"`
	TargetUserID pgtype.UUID                  `json:"target_user_id"`
}

type CollaborationSubmission struct {
	ID              pgtype.UUID                    `json:"id"`
	IssueID         pgtype.UUID                    `json:"issue_id"`
	EventSeq        int64                          `json:"event_seq,string"`
	LifecycleEpoch  *string                        `json:"lifecycle_epoch"`
	AuthorUserID    pgtype.UUID                    `json:"author_user_id"`
	Author          *CollaborationContentAccount   `json:"author"`
	CreatedAt       time.Time                      `json:"created_at"`
	Result          *string                        `json:"result"`
	Basis           *string                        `json:"basis"`
	AttachmentIDs   []pgtype.UUID                  `json:"attachment_ids"`
	SourceCommentID pgtype.UUID                    `json:"source_comment_id"`
	EvidenceState   string                         `json:"evidence_state"`
	Decision        *CollaborationDeliveryDecision `json:"decision"`
}

type CollaborationDeliverySummary struct {
	Current    *CollaborationSubmission `json:"current"`
	Latest     *CollaborationSubmission `json:"latest"`
	HasHistory bool                     `json:"has_history"`
}

type CollaborationDeliveryHistoryQuery struct {
	Limit  int
	Cursor string
}

type CollaborationDeliveryHistory struct {
	IssueID    pgtype.UUID               `json:"issue_id"`
	Items      []CollaborationSubmission `json:"items"`
	NextCursor *string                   `json:"next_cursor"`
}

// Event payloads are internal immutable facts, never HTTP response objects.
type collaborationDeliveryPayload struct {
	Version         int           `json:"version"`
	IssueID         pgtype.UUID   `json:"issue_id"`
	SubmissionID    pgtype.UUID   `json:"submission_id"`
	AuthorUserID    pgtype.UUID   `json:"author_user_id"`
	LifecycleEpoch  int64         `json:"lifecycle_epoch,string"`
	Basis           string        `json:"basis"`
	AttachmentIDs   []pgtype.UUID `json:"attachment_ids"`
	SourceCommentID pgtype.UUID   `json:"source_comment_id"`
}

type collaborationDeliveryDecisionPayload struct {
	Version        int         `json:"version"`
	SubmissionID   pgtype.UUID `json:"submission_id"`
	LifecycleEpoch int64       `json:"lifecycle_epoch,string"`
	Kind           string      `json:"kind"`
	Reason         *string     `json:"reason"`
	TargetUserID   pgtype.UUID `json:"target_user_id"`
}

type collaborationDeliveryEventPayload struct {
	HandoffID    pgtype.UUID     `json:"handoff_id"`
	TargetUserID pgtype.UUID     `json:"target_user_id"`
	Delivery     json.RawMessage `json:"delivery"`
	Decision     json.RawMessage `json:"delivery_decision"`
}

type collaborationDeliveryInput struct {
	note, basis   string
	draft         *pgtype.UUID
	attachments   []pgtype.UUID
	sourceComment *pgtype.UUID
}

func collaborationDeliveryTextValid(text string) bool {
	return utf8.ValidString(text) && strings.TrimSpace(text) != "" && len(text) <= collaborationDeliveryTextLimit
}

func collaborationDeliveryProseCode(note, basis string) string {
	if strings.TrimSpace(note) == "" {
		return "result_required"
	}
	if !utf8.ValidString(note) || len(note) > collaborationDeliveryTextLimit {
		return "result_too_large"
	}
	if strings.TrimSpace(basis) == "" {
		return "basis_required"
	}
	if !utf8.ValidString(basis) || len(basis) > collaborationDeliveryTextLimit {
		return "basis_too_large"
	}
	return ""
}

func (t *collaborationWorkflowTx) prepareDelivery(ctx context.Context, input collaborationDeliveryInput) (*CollaborationWorkflowReceipt, error) {
	if code := collaborationDeliveryProseCode(input.note, input.basis); code != "" {
		r := t.result(code, http.StatusBadRequest)
		return &r, nil
	}
	if input.sourceComment != nil {
		if !input.sourceComment.Valid {
			r := t.result("invalid_comment_request", http.StatusBadRequest)
			return &r, nil
		}
		if _, err := t.q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{IssueID: t.access.Issue.ID,
			WorkspaceID: t.access.Issue.WorkspaceID, CommentID: *input.sourceComment}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				r := t.result("comment_not_found", http.StatusNotFound)
				return &r, nil
			}
			return nil, err
		}
	}
	_, err := bindCollaborationAttachments(ctx, t.q, t.access.Issue.ID, t.access.Issue.WorkspaceID, t.actor,
		input.draft, input.attachments, false)
	var domain *CollaborationError
	if errors.As(err, &domain) {
		r := t.result(domain.Code, domain.Status)
		return &r, nil
	}
	return nil, err
}

func (t *collaborationWorkflowTx) deliveryPayload(input collaborationDeliveryInput, handoff db.IssueHandoff) collaborationDeliveryPayload {
	payload := collaborationDeliveryPayload{Version: 1, IssueID: t.access.Issue.ID, SubmissionID: handoff.ID,
		AuthorUserID: t.actor, LifecycleEpoch: t.access.LifecycleEpoch, Basis: input.basis,
		AttachmentIDs: append([]pgtype.UUID{}, input.attachments...)}
	if input.sourceComment != nil {
		payload.SourceCommentID = *input.sourceComment
	}
	t.deliveryConfirmation = &CollaborationDeliveryConfirmation{SubmissionID: handoff.ID, AttachmentIDs: slices.Clone(payload.AttachmentIDs)}
	return payload
}

// Self delivery has no responsibility change or iteration delta. The final
// lifecycle operation advances the collaboration revision exactly once.
func (t *collaborationWorkflowTx) recordSelfDelivery(ctx context.Context, input collaborationDeliveryInput) (db.IssueHandoff, error) {
	handoff, err := t.q.CreateCollaborationHandoff(ctx, db.CreateCollaborationHandoffParams{IssueID: t.access.Issue.ID,
		ActorUserID: t.actor, FromUserID: t.actor, ToUserID: t.actor, Purpose: "acceptance", Note: input.note, PreviousStatus: t.access.Issue.Status})
	if err != nil {
		return db.IssueHandoff{}, err
	}
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return db.IssueHandoff{}, err
	}
	payload, err := json.Marshal(map[string]any{"handoff_id": handoff.ID, "target_user_id": t.actor,
		"request_id": t.operation.RequestID, "delivery": t.deliveryPayload(input, handoff)})
	if err != nil {
		return db.IssueHandoff{}, err
	}
	_, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: t.access.Issue.ID,
		EventSeq: seq, ActorUserID: t.actor, Kind: "acceptance_submitted", Payload: payload})
	return handoff, err
}

func decodeCollaborationDeliveryJSON(raw json.RawMessage, out any) bool {
	if iteration.ValidateObjectJSON(raw) != nil {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out) == nil
}

// projectDeliverySubmissions shares the exact decoder for detail and history.
// Missing/old payloads remain legacy; malformed present facts are unavailable.
func projectDeliverySubmissions(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, events []db.CollaborationEvent) ([]CollaborationSubmission, error) {
	items := make([]CollaborationSubmission, 0, len(events))
	if len(events) == 0 {
		return items, nil
	}
	outer := make([]collaborationDeliveryEventPayload, len(events))
	ids, texts, users := make([]pgtype.UUID, 0, len(events)), make([]string, 0, len(events)), make([]pgtype.UUID, 0, len(events)*2)
	for i, event := range events {
		if event.IssueID != access.Issue.ID || event.EventSeq < 1 || !event.ActorUserID.Valid || !event.CreatedAt.Valid ||
			json.Unmarshal(event.Payload, &outer[i]) != nil || !outer[i].HandoffID.Valid || slices.Contains(ids, outer[i].HandoffID) {
			return nil, errCollaborationDeliveryEvidence
		}
		ids, texts, users = append(ids, outer[i].HandoffID), append(texts, util.UUIDToString(outer[i].HandoffID)), append(users, event.ActorUserID)
	}
	handoffs, err := q.ListCollaborationDeliveryHandoffs(ctx, db.ListCollaborationDeliveryHandoffsParams{IssueID: access.Issue.ID, HandoffIds: ids})
	if err != nil {
		return nil, err
	}
	byHandoff := make(map[pgtype.UUID]db.IssueHandoff, len(handoffs))
	for _, handoff := range handoffs {
		byHandoff[handoff.ID] = handoff
	}
	decisions, err := q.ListCollaborationDeliveryDecisions(ctx, db.ListCollaborationDeliveryDecisionsParams{IssueID: access.Issue.ID,
		SubmissionIds: texts, RowLimit: int32(2*len(events) + 1)})
	if err != nil {
		return nil, err
	}
	byDecision := make(map[pgtype.UUID][]db.CollaborationEvent, len(events))
	for _, event := range decisions {
		var payload collaborationDeliveryEventPayload
		var decision collaborationDeliveryDecisionPayload
		if json.Unmarshal(event.Payload, &payload) != nil || json.Unmarshal(payload.Decision, &decision) != nil || !decision.SubmissionID.Valid {
			return nil, errCollaborationDeliveryEvidence
		}
		byDecision[decision.SubmissionID] = append(byDecision[decision.SubmissionID], event)
		users = append(users, event.ActorUserID)
	}
	accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, users)
	if err != nil {
		return nil, err
	}
	for i, event := range events {
		handoff, found := byHandoff[outer[i].HandoffID]
		if !found || handoff.IssueID != event.IssueID || handoff.Purpose != "acceptance" || handoff.ActorUserID != event.ActorUserID ||
			handoff.FromUserID != event.ActorUserID || access.Issue.CreatorType != "member" || handoff.ToUserID != access.Issue.CreatorID ||
			outer[i].TargetUserID != handoff.ToUserID || !handoff.CreatedAt.Valid {
			return nil, errCollaborationDeliveryEvidence
		}
		item := CollaborationSubmission{ID: handoff.ID, IssueID: event.IssueID, EventSeq: event.EventSeq, AuthorUserID: handoff.ActorUserID,
			Author: accounts[handoff.ActorUserID], CreatedAt: handoff.CreatedAt.Time, AttachmentIDs: make([]pgtype.UUID, 0), EvidenceState: "legacy"}
		if utf8.ValidString(handoff.Note) && len(handoff.Note) <= collaborationDeliveryTextLimit && strings.TrimSpace(handoff.Note) != "" {
			item.Result = &handoff.Note
		}
		var delivery collaborationDeliveryPayload
		if len(outer[i].Delivery) > 0 {
			item.EvidenceState = "unavailable"
			if decodeCollaborationDeliveryJSON(outer[i].Delivery, &delivery) && delivery.Version == 1 && delivery.IssueID == event.IssueID &&
				delivery.SubmissionID == handoff.ID && delivery.AuthorUserID == handoff.ActorUserID && delivery.LifecycleEpoch > 0 && delivery.LifecycleEpoch <= access.LifecycleEpoch &&
				collaborationDeliveryTextValid(delivery.Basis) && item.Result != nil && delivery.AttachmentIDs != nil &&
				validateCollaborationAttachmentIDs(delivery.AttachmentIDs) == nil {
				attachments, err := q.ListCollaborationDeliveryAttachments(ctx, db.ListCollaborationDeliveryAttachmentsParams{IssueID: event.IssueID, AttachmentIds: delivery.AttachmentIDs})
				if err != nil {
					return nil, err
				}
				valid := len(attachments) == len(delivery.AttachmentIDs)
				for _, attachment := range attachments {
					if attachment.SourceWorkspaceID != access.Issue.WorkspaceID || !attachment.DeliveredAt.Valid {
						valid = false
						break
					}
				}
				if valid && delivery.SourceCommentID.Valid {
					// Tombstones retain this relationship. Current comment text or
					// deletion must never replace the immutable result snapshot.
					_, err := q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{IssueID: event.IssueID,
						WorkspaceID: access.Issue.WorkspaceID, CommentID: delivery.SourceCommentID})
					if err != nil && !errors.Is(err, pgx.ErrNoRows) {
						return nil, err
					}
					valid = err == nil
				}
				if valid {
					epoch := strconv.FormatInt(delivery.LifecycleEpoch, 10)
					item.LifecycleEpoch, item.Basis, item.AttachmentIDs, item.SourceCommentID = &epoch, &delivery.Basis, slices.Clone(delivery.AttachmentIDs), delivery.SourceCommentID
					item.EvidenceState = "available"
				}
			}
		}
		if retained := byDecision[handoff.ID]; len(retained) > 0 {
			var payload collaborationDeliveryEventPayload
			var decision collaborationDeliveryDecisionPayload
			e := retained[0]
			valid := len(retained) == 1 && e.EventSeq > event.EventSeq && e.ActorUserID.Valid && e.ActorUserID == access.Issue.CreatorID && e.CreatedAt.Valid &&
				json.Unmarshal(e.Payload, &payload) == nil && decodeCollaborationDeliveryJSON(payload.Decision, &decision) && decision.Version == 1 &&
				decision.SubmissionID == handoff.ID && decision.LifecycleEpoch > 0 && decision.LifecycleEpoch <= access.LifecycleEpoch &&
				(item.LifecycleEpoch == nil || decision.LifecycleEpoch == delivery.LifecycleEpoch) &&
				(decision.Reason == nil || utf8.ValidString(*decision.Reason) && len(*decision.Reason) <= collaborationDeliveryTextLimit)
			valid = valid && ((e.Kind == "completed" && decision.Kind == "accepted" && !decision.TargetUserID.Valid) ||
				(e.Kind == "cancelled" && decision.Kind == "cancelled" && !decision.TargetUserID.Valid) ||
				(e.Kind == "acceptance_returned" && decision.Kind == "returned" && decision.TargetUserID.Valid && decision.Reason != nil && strings.TrimSpace(*decision.Reason) != ""))
			if valid && decision.Kind == "returned" {
				valid = payload.HandoffID.Valid && payload.TargetUserID == decision.TargetUserID
				if valid {
					revision, err := q.GetCollaborationHandoffByID(ctx, db.GetCollaborationHandoffByIDParams{IssueID: e.IssueID, ID: payload.HandoffID})
					if err != nil && !errors.Is(err, pgx.ErrNoRows) {
						return nil, err
					}
					valid = err == nil && revision.Purpose == "transfer" && revision.ActorUserID == e.ActorUserID && revision.FromUserID == e.ActorUserID &&
						revision.ToUserID == decision.TargetUserID && revision.Note == *decision.Reason && revision.CreatedAt.Valid
				}
			}
			if valid {
				item.Decision = &CollaborationDeliveryDecision{Kind: decision.Kind, SubmissionID: handoff.ID, EventSeq: e.EventSeq,
					ActorUserID: e.ActorUserID, Actor: accounts[e.ActorUserID], CreatedAt: e.CreatedAt.Time, Reason: decision.Reason, TargetUserID: decision.TargetUserID}
			} else {
				item.EvidenceState = "unavailable"
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func collaborationDeliverySummary(ctx context.Context, q *db.Queries, access CollaborationIssueAccess) (CollaborationDeliverySummary, error) {
	out := CollaborationDeliverySummary{}
	events, err := q.ListCollaborationDeliveryEvents(ctx, db.ListCollaborationDeliveryEventsParams{IssueID: access.Issue.ID,
		ThroughSeq: access.EventSequence, RowLimit: 1})
	if err != nil {
		return out, err
	}
	items, err := projectDeliverySubmissions(ctx, q, access, events)
	if err != nil || len(items) == 0 {
		return out, err
	}
	out.Latest, out.HasHistory = &items[0], true
	boundary, err := q.GetCollaborationDeliveryLifecycleBoundary(ctx, access.Issue.ID)
	if err != nil {
		return out, err
	}
	if collaboration.NonterminalCategory(access.StatusCategory) && access.QueueState != "closed" && items[0].Decision == nil && items[0].EventSeq > boundary &&
		(items[0].LifecycleEpoch == nil || *items[0].LifecycleEpoch == strconv.FormatInt(access.LifecycleEpoch, 10)) {
		out.Current = out.Latest
	}
	return out, nil
}

// DeliveryView borrows the caller's authorized detail snapshot.
func (s *CollaborationService) DeliveryView(ctx context.Context, access CollaborationIssueAccess) (*CollaborationDeliverySummary, error) {
	out, err := collaborationDeliverySummary(ctx, s.Queries, access)
	if errors.Is(err, errCollaborationDeliveryEvidence) {
		return nil, nil
	}
	return &out, err
}

// Capabilities describe current actions; writes still recheck exact submission,
// responsibility and authority under the original workflow owner.
func CollaborationDeliveryCapabilities(access CollaborationIssueAccess, summary *CollaborationDeliverySummary) collaboration.Capabilities {
	caps := access.Capabilities
	if !caps.ControlAI || !collaboration.NonterminalCategory(access.StatusCategory) || access.QueueState == "closed" {
		return caps
	}
	pending := summary == nil || summary.Current != nil
	if pending {
		caps.Transfer, caps.ChangeStatus = false, false
	}
	replaceable := summary == nil || summary.Current != nil && summary.Current.EvidenceState != "available"
	caps.SubmitAcceptance = caps.SubmitAcceptance && (summary == nil || summary.Current == nil || replaceable)
	creatorHandler := caps.ChangeAIPolicy && access.Issue.CreatorID == access.Issue.AssigneeID
	if creatorHandler {
		caps.SubmitAcceptance = replaceable
	}
	caps.ReturnAcceptance = caps.ReturnAcceptance && summary != nil && summary.Current != nil && creatorHandler &&
		access.StatusCategory == "in_review" && !access.InformationRequestID.Valid
	return caps
}

type collaborationDeliveryCursor struct {
	IssueID pgtype.UUID `json:"issue_id"`
	ActorID pgtype.UUID `json:"actor_user_id"`
	Through int64       `json:"through_seq,string"`
	Before  int64       `json:"before_seq,string"`
}

func (s *CollaborationService) ListAcceptanceSubmissions(ctx context.Context, issueID pgtype.UUID, p CollaborationDeliveryHistoryQuery) (CollaborationDeliveryHistory, error) {
	out := CollaborationDeliveryHistory{IssueID: issueID, Items: make([]CollaborationSubmission, 0)}
	if p.Limit < 1 || p.Limit > 100 || len(p.Cursor) > 512 {
		return out, collaborationError(http.StatusBadRequest, "invalid_delivery_query")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		principal, _ := collaboration.PrincipalFromContext(ctx)
		actor, err := util.ParseUUID(principal.UserID)
		if err != nil {
			return err
		}
		cursor := collaborationDeliveryCursor{IssueID: issueID, ActorID: actor, Through: access.EventSequence}
		before := pgtype.Int8{}
		if p.Cursor != "" {
			raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
			if err != nil || !decodeCollaborationDeliveryJSON(raw, &cursor) || cursor.IssueID != issueID || cursor.ActorID != actor ||
				cursor.Through < 1 || cursor.Through > access.EventSequence || cursor.Before < 1 || cursor.Before > cursor.Through {
				return collaborationError(http.StatusBadRequest, "invalid_delivery_query")
			}
			before = pgtype.Int8{Int64: cursor.Before, Valid: true}
		}
		events, err := q.ListCollaborationDeliveryEvents(ctx, db.ListCollaborationDeliveryEventsParams{IssueID: issueID,
			ThroughSeq: cursor.Through, BeforeSeq: before, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		more := len(events) > p.Limit
		if more {
			events = events[:p.Limit]
		}
		out.Items, err = projectDeliverySubmissions(ctx, q, access, events)
		if err != nil {
			return err
		}
		if more {
			cursor.Before = out.Items[len(out.Items)-1].EventSeq
			raw, err := json.Marshal(cursor)
			if err != nil {
				return err
			}
			encoded := base64.RawURLEncoding.EncodeToString(raw)
			out.NextCursor = &encoded
		}
		return nil
	})
	return out, err
}

func (t *collaborationWorkflowTx) currentDelivery(ctx context.Context) (*CollaborationSubmission, error) {
	// Earlier writes in this transaction may have advanced the session sequence.
	access := t.access
	access.EventSequence = max(access.EventSequence, t.session.EventSeq)
	summary, err := collaborationDeliverySummary(ctx, t.q, access)
	return summary.Current, err
}
