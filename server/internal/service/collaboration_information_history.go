package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const informationHistoryDomain = "collaboration-human-information-history-cursor-v1"

type CollaborationInformationProvenance struct {
	State         string      `json:"state"`
	EventID       pgtype.UUID `json:"event_id"`
	EventSequence *string     `json:"event_sequence"`
	ReasonCode    *string     `json:"reason_code,omitempty"`
}

type CollaborationInformationTuple struct {
	CreatedAt string      `json:"created_at"`
	HandoffID pgtype.UUID `json:"handoff_id"`
}

type CollaborationInformationHistoryItem struct {
	Kind       string
	HandoffID  pgtype.UUID
	CreatedAt  string
	Provenance CollaborationInformationProvenance
	row        db.IssueHandoff
	restored   *string
	fallback   *bool
	repair     *informationRepairHistoryFacts
}

type informationRepairHistoryFacts struct {
	PriorActiveRequestID pgtype.UUID                         `json:"prior_active_request_id"`
	PreviousStatus       string                              `json:"previous_status"`
	Status               string                              `json:"status"`
	Basis                CollaborationInformationRepairBasis `json:"basis"`
	SupersededCount      string                              `json:"superseded_count"`
}

type informationHistoryCommon struct {
	Kind           string                             `json:"kind"`
	HandoffID      pgtype.UUID                        `json:"handoff_id"`
	CreatedAt      string                             `json:"created_at"`
	Provenance     CollaborationInformationProvenance `json:"provenance"`
	ActorUserID    pgtype.UUID                        `json:"actor_user_id"`
	Note           string                             `json:"note"`
	FromUserID     pgtype.UUID                        `json:"from_user_id"`
	ToUserID       pgtype.UUID                        `json:"to_user_id"`
	PreviousStatus string                             `json:"previous_status"`
	Resolution     string                             `json:"resolution"`
}

// Each variant has a closed wire shape. An unavailable row has only its exact
// identity/provenance; it never acquires invented actor, prose or state fields.
func (item CollaborationInformationHistoryItem) MarshalJSON() ([]byte, error) {
	minimal := struct {
		Kind       string                             `json:"kind"`
		HandoffID  pgtype.UUID                        `json:"handoff_id"`
		CreatedAt  string                             `json:"created_at"`
		Provenance CollaborationInformationProvenance `json:"provenance"`
	}{item.Kind, item.HandoffID, item.CreatedAt, item.Provenance}
	if item.Provenance.State == "unavailable" {
		return json.Marshal(minimal)
	}
	common := informationHistoryCommon{item.Kind, item.HandoffID, item.CreatedAt, item.Provenance, item.row.ActorUserID, item.row.Note,
		item.row.FromUserID, item.row.ToUserID, item.row.PreviousStatus, item.row.Resolution}
	switch item.Kind {
	case "information_request":
		return json.Marshal(struct {
			informationHistoryCommon
			RequesterUserID             pgtype.UUID                   `json:"requester_user_id"`
			ParentRequestID             pgtype.UUID                   `json:"parent_request_id"`
			Block                       CollaborationInformationBlock `json:"block"`
			SupersededByRepairHandoffID pgtype.UUID                   `json:"superseded_by_repair_handoff_id"`
		}{common, item.row.RequesterUserID, item.row.ParentRequestID, informationBlock(item.row), item.row.SupersededByRepairHandoffID})
	case "information_return":
		return json.Marshal(struct {
			informationHistoryCommon
			ConsumedRequestID    pgtype.UUID `json:"consumed_request_id"`
			RestoredStatus       *string     `json:"restored_status"`
			ReturnStatusFallback *bool       `json:"return_status_fallback"`
		}{common, item.row.ParentRequestID, item.restored, item.fallback})
	case "information_repair":
		if item.repair == nil {
			return nil, errors.New("missing information repair facts")
		}
		return json.Marshal(struct {
			informationHistoryCommon
			PriorActiveRequestID   pgtype.UUID                         `json:"prior_active_request_id"`
			RestoredStatus         string                              `json:"restored_status"`
			TargetUserID           pgtype.UUID                         `json:"target_user_id"`
			Basis                  CollaborationInformationRepairBasis `json:"basis"`
			SupersededRequestCount string                              `json:"superseded_request_count"`
		}{common, item.repair.PriorActiveRequestID, item.repair.Status, item.row.ToUserID, item.repair.Basis, item.repair.SupersededCount})
	default:
		return nil, errors.New("invalid information history kind")
	}
}

type CollaborationInformationAffectedItem struct {
	RequestID                   pgtype.UUID                        `json:"request_id"`
	RequestEventID              pgtype.UUID                        `json:"request_event_id"`
	Provenance                  CollaborationInformationProvenance `json:"provenance"`
	Resolution                  string                             `json:"resolution"`
	SupersededByRepairHandoffID pgtype.UUID                        `json:"superseded_by_repair_handoff_id"`
}

type CollaborationInformationHistoryPage struct {
	ProtocolVersion       int                                   `json:"protocol_version"`
	IssueID               pgtype.UUID                           `json:"issue_id"`
	CollaborationRevision string                                `json:"collaboration_revision"`
	LifecycleEpoch        string                                `json:"lifecycle_epoch"`
	UpperHandoff          *CollaborationInformationTuple        `json:"upper_handoff"`
	Items                 []CollaborationInformationHistoryItem `json:"items"`
	NextCursor            *string                               `json:"next_cursor"`
}

type CollaborationInformationAffectedPage struct {
	ProtocolVersion        int                                    `json:"protocol_version"`
	IssueID                pgtype.UUID                            `json:"issue_id"`
	CollaborationRevision  string                                 `json:"collaboration_revision"`
	LifecycleEpoch         string                                 `json:"lifecycle_epoch"`
	UpperHandoff           *CollaborationInformationTuple         `json:"upper_handoff"`
	RepairHandoffID        pgtype.UUID                            `json:"repair_handoff_id"`
	SupersededRequestCount string                                 `json:"superseded_request_count"`
	Items                  []CollaborationInformationAffectedItem `json:"items"`
	NextCursor             *string                                `json:"next_cursor"`
}

type CollaborationInformationDetail struct {
	ProtocolVersion int                                 `json:"protocol_version"`
	IssueID         pgtype.UUID                         `json:"issue_id"`
	Item            CollaborationInformationHistoryItem `json:"item"`
}

type CollaborationInformationHistoryQuery struct {
	Limit  int
	Cursor string
}

func informationProvenance(event *db.CollaborationEvent, state string) CollaborationInformationProvenance {
	out := CollaborationInformationProvenance{State: state}
	if event != nil && state == "linked" {
		seq := strconv.FormatInt(event.EventSeq, 10)
		out.EventID, out.EventSequence = event.ID, &seq
	}
	if state == "unavailable" {
		code := "information_context_unavailable"
		out.ReasonCode = &code
	}
	return out
}

func informationHistoryItem(ctx context.Context, q *db.Queries, row db.IssueHandoff) (CollaborationInformationHistoryItem, error) {
	kind, eventKind := "information_request", "information_requested"
	if row.Purpose == "information_return" {
		kind, eventKind = "information_return", "information_returned"
	} else if row.Purpose == "transfer" {
		kind, eventKind = "information_repair", "information_repaired"
	}
	event, provenance, err := informationEvent(ctx, q, row, eventKind)
	if err != nil {
		return CollaborationInformationHistoryItem{}, err
	}
	out := CollaborationInformationHistoryItem{Kind: kind, HandoffID: row.ID, CreatedAt: informationTime(row.CreatedAt.Time), row: row,
		Provenance: informationProvenance(event, provenance)}
	unavailable := func() (CollaborationInformationHistoryItem, error) {
		out.Provenance = informationProvenance(nil, "unavailable")
		return out, nil
	}
	if !informationUUID(&row.ID) || !row.CreatedAt.Valid || !informationUUID(&row.ActorUserID) || row.PreviousStatus == "" ||
		(row.Resolution != "open" && row.Resolution != "returned" && row.Resolution != "superseded") {
		return unavailable()
	}
	if provenance == "unavailable" {
		return out, nil
	}
	if kind == "information_request" {
		return out, nil
	}
	if kind == "information_repair" {
		if event == nil || provenance != "linked" || !informationText(row.Note) {
			return unavailable()
		}
		var facts informationRepairHistoryFacts
		var fields map[string]json.RawMessage
		if json.Unmarshal(event.Payload, &facts) != nil || facts.PreviousStatus != row.PreviousStatus || facts.Status == "" ||
			!facts.Basis.valid() || !informationNonnegative(facts.SupersededCount) || !informationUUID(&row.ToUserID) ||
			(facts.PriorActiveRequestID.Valid && facts.PriorActiveRequestID.Bytes == [16]byte{}) || json.Unmarshal(event.Payload, &fields) != nil ||
			len(fields["prior_active_request_id"]) == 0 {
			return unavailable()
		}
		basisFields := []string{"kind"}
		if facts.Basis.Kind == "historical_information" {
			basisFields = append(basisFields, "origin_request_id", "origin_event_id")
		} else if facts.Basis.Kind == "pending_submission" {
			basisFields = append(basisFields, "submission_id", "submission_event_id")
		}
		var basis CollaborationInformationRepairBasis
		if !strictInformationJSON(fields["basis"], &basis, basisFields...) {
			return unavailable()
		}
		valid, err := informationHistoryRepairBasis(ctx, q, row, facts)
		if err != nil {
			return out, err
		}
		if !valid {
			return unavailable()
		}
		out.repair = &facts
		return out, nil
	}
	if event == nil {
		return out, nil
	}
	if !row.ParentRequestID.Valid {
		return unavailable()
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(event.Payload, &fields) != nil {
		return unavailable()
	}
	var consumed pgtype.UUID
	if json.Unmarshal(fields["information_request_id"], &consumed) != nil || consumed != row.ParentRequestID {
		return unavailable()
	}
	_, hasRestored := fields["restored_status"]
	_, hasFallback := fields["return_status_fallback"]
	var status string
	if json.Unmarshal(fields["status"], &status) != nil || status == "" {
		return unavailable()
	}
	if hasRestored || hasFallback {
		var restored string
		var fallback bool
		if !hasRestored || !hasFallback || bytes.Equal(fields["restored_status"], []byte("null")) || bytes.Equal(fields["return_status_fallback"], []byte("null")) ||
			json.Unmarshal(fields["restored_status"], &restored) != nil || json.Unmarshal(fields["return_status_fallback"], &fallback) != nil || restored != status {
			return unavailable()
		}
		out.restored, out.fallback = &restored, &fallback
		return out, nil
	}
	// Old linked returns may infer this only from their exact consumed request
	// and committed target. Missing legacy linkage never manufactures a value.
	request, err := q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: row.IssueID, RequestID: consumed})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if err == nil && request.PreviousStatus != "" {
		fallback := request.PreviousStatus != status
		out.restored, out.fallback = &status, &fallback
	}
	return out, nil
}

func informationHistoryRepairBasis(ctx context.Context, q *db.Queries, repair db.IssueHandoff, facts informationRepairHistoryFacts) (bool, error) {
	if facts.Basis.Kind == "human_selected" {
		return true, nil
	}
	requestID, eventID, kind := facts.Basis.OriginRequestID, facts.Basis.OriginEventID, "information_requested"
	if facts.Basis.Kind == "pending_submission" {
		requestID, eventID, kind = facts.Basis.SubmissionID, facts.Basis.SubmissionEventID, "acceptance_submitted"
	}
	if !informationUUID(requestID) || !informationUUID(eventID) {
		return false, nil
	}
	source, err := q.GetCollaborationHandoffByID(ctx, db.GetCollaborationHandoffByIDParams{IssueID: repair.IssueID, ID: *requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	event, err := q.GetCollaborationInformationBasisEvent(ctx, db.GetCollaborationInformationBasisEventParams{IssueID: repair.IssueID, EventID: *eventID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload struct {
		HandoffID    pgtype.UUID `json:"handoff_id"`
		TargetUserID pgtype.UUID `json:"target_user_id"`
	}
	if event.Kind != kind || event.ActorUserID != source.ActorUserID || json.Unmarshal(event.Payload, &payload) != nil || payload.HandoffID != source.ID || payload.TargetUserID != source.ToUserID {
		return false, nil
	}
	if kind == "acceptance_submitted" {
		return source.Purpose == "acceptance" && source.ToUserID == repair.ToUserID, nil
	}
	if source.Purpose != "information" || source.PreviousStatus != facts.Status || source.SupersededByRepairHandoffID != repair.ID || !facts.PriorActiveRequestID.Valid {
		return false, nil
	}
	current := facts.PriorActiveRequestID
	seen := map[pgtype.UUID]bool{}
	for current.Valid {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if seen[current] {
			return false, nil
		}
		seen[current] = true
		row, err := q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: repair.IssueID, RequestID: current})
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if row.SupersededByRepairHandoffID != repair.ID || row.Resolution != "superseded" {
			return false, nil
		}
		if row.ID == source.ID {
			return true, nil
		}
		current = row.ParentRequestID
	}
	return false, nil
}

type informationHistoryCursor struct {
	ProtocolVersion       int                           `json:"protocol_version"`
	ViewerID              pgtype.UUID                   `json:"viewer_id"`
	IssueID               pgtype.UUID                   `json:"issue_id"`
	Projection            string                        `json:"projection"`
	CollaborationRevision string                        `json:"collaboration_revision"`
	LifecycleEpoch        string                        `json:"lifecycle_epoch"`
	AuthorityVersion      string                        `json:"authority_version"`
	Upper                 CollaborationInformationTuple `json:"upper_handoff"`
	Last                  CollaborationInformationTuple `json:"last_handoff"`
	RepairHandoffID       pgtype.UUID                   `json:"repair_handoff_id"`
}

func informationCursorKey() ([]byte, error) {
	key := auth.JWTSecret()
	if auth.ValidateJWTSecret(string(key)) != nil {
		return nil, collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
	}
	derived := hmac.New(sha256.New, key)
	_, _ = derived.Write([]byte(informationHistoryDomain))
	return derived.Sum(nil), nil
}

func strictInformationJSON(raw []byte, target any, fields ...string) bool {
	if iteration.ValidateObjectJSON(raw) != nil {
		return false
	}
	var present map[string]json.RawMessage
	if json.Unmarshal(raw, &present) != nil || len(present) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, ok := present[field]; !ok {
			return false
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil
}

func validInformationTuple(tuple CollaborationInformationTuple) bool {
	instant, err := time.Parse("2006-01-02T15:04:05.000000Z", tuple.CreatedAt)
	return err == nil && informationTime(instant) == tuple.CreatedAt && informationUUID(&tuple.HandoffID)
}

func informationTuple(row db.IssueHandoff) CollaborationInformationTuple {
	return CollaborationInformationTuple{CreatedAt: informationTime(row.CreatedAt.Time), HandoffID: row.ID}
}

func (cursor informationHistoryCursor) valid() bool {
	if cursor.ProtocolVersion != 1 || !informationUUID(&cursor.ViewerID) || !informationUUID(&cursor.IssueID) ||
		!stagePositiveVersion(cursor.CollaborationRevision) || !stagePositiveVersion(cursor.LifecycleEpoch) || !stageDigest(cursor.AuthorityVersion) ||
		!validInformationTuple(cursor.Upper) || !validInformationTuple(cursor.Last) {
		return false
	}
	upper, _ := time.Parse(time.RFC3339Nano, cursor.Upper.CreatedAt)
	last, _ := time.Parse(time.RFC3339Nano, cursor.Last.CreatedAt)
	if last.After(upper) || last.Equal(upper) && bytes.Compare(cursor.Last.HandoffID.Bytes[:], cursor.Upper.HandoffID.Bytes[:]) > 0 {
		return false
	}
	return cursor.Projection == "information_history" && !cursor.RepairHandoffID.Valid ||
		cursor.Projection == "information_repair_affected" && informationUUID(&cursor.RepairHandoffID)
}

func encodeInformationCursor(cursor informationHistoryCursor) (string, error) {
	key, err := informationCursorKey()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(cursor)
	if err != nil || !cursor.valid() {
		return "", collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	raw, err := json.Marshal(struct {
		Payload json.RawMessage `json:"payload"`
		MAC     string          `json:"mac"`
	}{payload, base64.RawURLEncoding.EncodeToString(mac.Sum(nil))})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > 2048 {
		return "", collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
	}
	return encoded, nil
}

func decodeInformationCursor(raw string, state *informationState, projection string, repair pgtype.UUID) (informationHistoryCursor, error) {
	invalid := func() (informationHistoryCursor, error) {
		return informationHistoryCursor{}, collaborationError(http.StatusBadRequest, "invalid_information_query")
	}
	if len(raw) == 0 || len(raw) > 2048 {
		return invalid()
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != raw {
		return invalid()
	}
	var wrapper struct {
		Payload json.RawMessage `json:"payload"`
		MAC     string          `json:"mac"`
	}
	if !strictInformationJSON(decoded, &wrapper, "payload", "mac") {
		return invalid()
	}
	var cursor informationHistoryCursor
	if !strictInformationJSON(wrapper.Payload, &cursor, "protocol_version", "viewer_id", "issue_id", "projection", "collaboration_revision", "lifecycle_epoch", "authority_version", "upper_handoff", "last_handoff", "repair_handoff_id") || !cursor.valid() {
		return invalid()
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(wrapper.Payload, &fields) != nil {
		return invalid()
	}
	var upper, last CollaborationInformationTuple
	if !strictInformationJSON(fields["upper_handoff"], &upper, "created_at", "handoff_id") || !strictInformationJSON(fields["last_handoff"], &last, "created_at", "handoff_id") {
		return invalid()
	}
	key, err := informationCursorKey()
	if err != nil {
		return informationHistoryCursor{}, err
	}
	canonical, err := json.Marshal(cursor)
	if err != nil || !bytes.Equal(canonical, wrapper.Payload) {
		return invalid()
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(canonical)
	signature, err := base64.RawURLEncoding.DecodeString(wrapper.MAC)
	if err != nil || len(signature) != sha256.Size || base64.RawURLEncoding.EncodeToString(signature) != wrapper.MAC || !hmac.Equal(signature, mac.Sum(nil)) {
		return invalid()
	}
	if cursor.ViewerID != state.Actor || cursor.IssueID != state.Access.Issue.ID || cursor.Projection != projection || cursor.RepairHandoffID != repair ||
		cursor.CollaborationRevision != stageVersion(state.Access.Revision) || cursor.LifecycleEpoch != stageVersion(state.Access.LifecycleEpoch) || cursor.AuthorityVersion != state.AuthorityVersion {
		return informationHistoryCursor{}, collaborationError(http.StatusConflict, "information_context_changed")
	}
	return cursor, nil
}

func informationPageCursor(state *informationState, projection string, repair pgtype.UUID, upper, last CollaborationInformationTuple) informationHistoryCursor {
	return informationHistoryCursor{ProtocolVersion: 1, ViewerID: state.Actor, IssueID: state.Access.Issue.ID, Projection: projection,
		CollaborationRevision: stageVersion(state.Access.Revision), LifecycleEpoch: stageVersion(state.Access.LifecycleEpoch), AuthorityVersion: state.AuthorityVersion,
		Upper: upper, Last: last, RepairHandoffID: repair}
}

func informationTupleParams(tuple *CollaborationInformationTuple) (pgtype.Timestamptz, pgtype.UUID) {
	if tuple == nil {
		return pgtype.Timestamptz{}, pgtype.UUID{}
	}
	instant, _ := time.Parse(time.RFC3339Nano, tuple.CreatedAt)
	return pgtype.Timestamptz{Time: instant, Valid: true}, tuple.HandoffID
}

func informationHistoryState(ctx context.Context, q *db.Queries, access CollaborationIssueAccess) (*informationState, error) {
	catalog, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: access.Issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return nil, err
	}
	return loadInformationState(ctx, q, access, catalog, false)
}

// The RR snapshot owns consistent facts. A fresh final check prevents a read
// completed after a known withdrawal from returning that older private page.
func (s *CollaborationService) informationReadFence(ctx context.Context, state *informationState) error {
	if state == nil {
		return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
	}
	access, err := s.ResolveIssueAccess(ctx, nil, state.Access.Issue.ID)
	if err != nil {
		return err
	}
	if access.Revision != state.Access.Revision || access.LifecycleEpoch != state.Access.LifecycleEpoch {
		return collaborationError(http.StatusConflict, "information_context_changed")
	}
	version, err := informationAuthorityVersion(ctx, s.Queries, access, state.Actor)
	if err != nil {
		return err
	}
	if version != state.AuthorityVersion {
		return collaborationError(http.StatusConflict, "information_context_changed")
	}
	return nil
}

func (s *CollaborationService) ListInformationHistory(ctx context.Context, issueID pgtype.UUID, p CollaborationInformationHistoryQuery) (CollaborationInformationHistoryPage, error) {
	out := CollaborationInformationHistoryPage{ProtocolVersion: 1, IssueID: issueID, Items: []CollaborationInformationHistoryItem{}}
	ctx, cancel := context.WithTimeout(ctx, informationReadTimeout)
	defer cancel()
	var fence *informationState
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if p.Limit < 1 || p.Limit > 50 || len(p.Cursor) > 2048 {
			return collaborationError(http.StatusBadRequest, "invalid_information_query")
		}
		state, err := informationHistoryState(ctx, q, access)
		if err != nil {
			return err
		}
		fence = state
		if _, err = informationCursorKey(); err != nil {
			return err
		}
		out.CollaborationRevision, out.LifecycleEpoch = stageVersion(access.Revision), stageVersion(access.LifecycleEpoch)
		var upper, last *CollaborationInformationTuple
		if p.Cursor != "" {
			cursor, err := decodeInformationCursor(p.Cursor, state, "information_history", pgtype.UUID{})
			if err != nil {
				return err
			}
			upper, last = &cursor.Upper, &cursor.Last
		}
		upperAt, upperID := informationTupleParams(upper)
		beforeAt, beforeID := informationTupleParams(last)
		rows, err := q.ListCollaborationInformationHandoffs(ctx, db.ListCollaborationInformationHandoffsParams{IssueID: issueID,
			UpperAt: upperAt, UpperID: upperID, BeforeAt: beforeAt, BeforeID: beforeID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			out.UpperHandoff = upper
			return nil
		}
		if upper == nil {
			tuple := informationTuple(rows[0])
			upper = &tuple
		}
		out.UpperHandoff = upper
		for i, row := range rows {
			if i >= p.Limit {
				break
			}
			item, err := informationHistoryItem(ctx, q, row)
			if err != nil {
				return err
			}
			candidate := out
			candidate.Items = append(append([]CollaborationInformationHistoryItem{}, out.Items...), item)
			candidate.NextCursor = nil
			if i+1 < len(rows) {
				cursor, err := encodeInformationCursor(informationPageCursor(state, "information_history", pgtype.UUID{}, *upper, informationTuple(row)))
				if err != nil {
					return err
				}
				candidate.NextCursor = &cursor
			}
			var size int
			if err = collaborationRecallJSONBytes(&candidate, &size); err != nil {
				return err
			}
			if size > 65536 {
				if len(out.Items) == 0 {
					return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
				}
				break
			}
			out = candidate
		}
		return nil
	})
	if err == nil {
		err = s.informationReadFence(ctx, fence)
	}
	return out, err
}

func (s *CollaborationService) InformationHistoryDetail(ctx context.Context, issueID, handoffID pgtype.UUID) (CollaborationInformationDetail, error) {
	out := CollaborationInformationDetail{ProtocolVersion: 1, IssueID: issueID}
	ctx, cancel := context.WithTimeout(ctx, informationReadTimeout)
	defer cancel()
	var fence *informationState
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		var err error
		fence, err = informationHistoryState(ctx, q, access)
		if err != nil {
			return err
		}
		if _, err := informationCursorKey(); err != nil {
			return err
		}
		row, err := q.GetCollaborationInformationHandoff(ctx, db.GetCollaborationInformationHandoffParams{IssueID: issueID, HandoffID: handoffID})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusNotFound, "not_found")
		}
		if err != nil {
			return err
		}
		if out.Item, err = informationHistoryItem(ctx, q, row); err != nil {
			return err
		}
		var size int
		if err = collaborationRecallJSONBytes(&out, &size); err != nil {
			return err
		}
		if size > 65536 {
			return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
		}
		return nil
	})
	if err == nil {
		err = s.informationReadFence(ctx, fence)
	}
	return out, err
}

func (s *CollaborationService) ListInformationRepairAffected(ctx context.Context, issueID, repairID pgtype.UUID, p CollaborationInformationHistoryQuery) (CollaborationInformationAffectedPage, error) {
	out := CollaborationInformationAffectedPage{ProtocolVersion: 1, IssueID: issueID, RepairHandoffID: repairID, Items: []CollaborationInformationAffectedItem{}}
	ctx, cancel := context.WithTimeout(ctx, informationReadTimeout)
	defer cancel()
	var fence *informationState
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if p.Limit < 1 || p.Limit > 50 || len(p.Cursor) > 2048 {
			return collaborationError(http.StatusBadRequest, "invalid_information_query")
		}
		state, err := informationHistoryState(ctx, q, access)
		if err != nil {
			return err
		}
		fence = state
		if _, err = informationCursorKey(); err != nil {
			return err
		}
		repair, err := q.GetCollaborationInformationHandoff(ctx, db.GetCollaborationInformationHandoffParams{IssueID: issueID, HandoffID: repairID})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusNotFound, "not_found")
		}
		if err != nil {
			return err
		}
		item, err := informationHistoryItem(ctx, q, repair)
		if err != nil {
			return err
		}
		if item.Kind != "information_repair" {
			return collaborationError(http.StatusNotFound, "not_found")
		}
		if item.Provenance.State != "linked" || item.repair == nil {
			return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
		}
		count, err := q.CountCollaborationInformationRepairAffected(ctx, db.CountCollaborationInformationRepairAffectedParams{IssueID: issueID, RepairHandoffID: repairID})
		if err != nil {
			return err
		}
		if strconv.FormatInt(count, 10) != item.repair.SupersededCount {
			return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
		}
		out.CollaborationRevision, out.LifecycleEpoch, out.SupersededRequestCount = stageVersion(access.Revision), stageVersion(access.LifecycleEpoch), item.repair.SupersededCount
		var upper, last *CollaborationInformationTuple
		if p.Cursor != "" {
			cursor, err := decodeInformationCursor(p.Cursor, state, "information_repair_affected", repairID)
			if err != nil {
				return err
			}
			upper, last = &cursor.Upper, &cursor.Last
		}
		upperAt, upperID := informationTupleParams(upper)
		beforeAt, beforeID := informationTupleParams(last)
		rows, err := q.ListCollaborationInformationRepairAffected(ctx, db.ListCollaborationInformationRepairAffectedParams{IssueID: issueID, RepairHandoffID: repairID,
			UpperAt: upperAt, UpperID: upperID, BeforeAt: beforeAt, BeforeID: beforeID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			out.UpperHandoff = upper
			return nil
		}
		if upper == nil {
			tuple := informationTuple(rows[0])
			upper = &tuple
		}
		out.UpperHandoff = upper
		for i, row := range rows {
			if i >= p.Limit {
				break
			}
			if row.Purpose != "information" || row.Resolution != "superseded" || row.SupersededByRepairHandoffID != repairID {
				return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
			}
			event, provenance, err := informationEvent(ctx, q, row, "information_requested")
			if err != nil {
				return err
			}
			entry := CollaborationInformationAffectedItem{RequestID: row.ID, Provenance: informationProvenance(event, provenance), Resolution: row.Resolution,
				SupersededByRepairHandoffID: row.SupersededByRepairHandoffID}
			if event != nil && provenance == "linked" {
				entry.RequestEventID = event.ID
			}
			candidate := out
			candidate.Items = append(append([]CollaborationInformationAffectedItem{}, out.Items...), entry)
			candidate.NextCursor = nil
			if i+1 < len(rows) {
				cursor, err := encodeInformationCursor(informationPageCursor(state, "information_repair_affected", repairID, *upper, informationTuple(row)))
				if err != nil {
					return err
				}
				candidate.NextCursor = &cursor
			}
			var size int
			if err = collaborationRecallJSONBytes(&candidate, &size); err != nil {
				return err
			}
			if size > 65536 {
				if len(out.Items) == 0 {
					return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
				}
				break
			}
			out = candidate
		}
		return nil
	})
	if err == nil {
		err = s.informationReadFence(ctx, fence)
	}
	return out, err
}
