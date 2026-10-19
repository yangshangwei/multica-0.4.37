package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationActivityQuery struct {
	Limit     int
	BeforeSeq pgtype.Int8
}

type CollaborationActivityChange struct {
	Field  string  `json:"field"`
	Before *string `json:"before"`
	After  *string `json:"after"`
}

type CollaborationActivityHandoff struct {
	ID      pgtype.UUID                  `json:"id"`
	Purpose string                       `json:"purpose"`
	From    *CollaborationContentAccount `json:"from"`
	To      *CollaborationContentAccount `json:"to"`
	Note    string                       `json:"note"`
}

type CollaborationActivityEntry struct {
	ID               pgtype.UUID                   `json:"id"`
	IssueID          pgtype.UUID                   `json:"issue_id"`
	EventSeq         int64                         `json:"event_seq,string"`
	Kind             string                        `json:"kind"`
	Note             string                        `json:"note,omitempty"`
	Actor            *CollaborationContentAccount  `json:"actor"`
	CreatedAt        time.Time                     `json:"created_at"`
	DetailsAvailable bool                          `json:"details_available"`
	Target           *CollaborationContentAccount  `json:"target"`
	Handoff          *CollaborationActivityHandoff `json:"handoff"`
	CommentID        pgtype.UUID                   `json:"comment_id"`
	Changes          []CollaborationActivityChange `json:"changes"`
}

type CollaborationActivityPage struct {
	Items   []CollaborationActivityEntry
	HasMore bool
}

// Only fields with an explicit display contract are decoded. Stored receipts,
// resource identities, request bodies and unknown payload keys never leave here.
type collaborationActivityPayload struct {
	TargetUserID   pgtype.UUID     `json:"target_user_id"`
	HandoffID      pgtype.UUID     `json:"handoff_id"`
	CommentID      pgtype.UUID     `json:"comment_id"`
	PreviousStatus *string         `json:"previous_status"`
	Status         *string         `json:"status"`
	Before         json.RawMessage `json:"before"`
	After          json.RawMessage `json:"after"`
	Reason         string          `json:"reason"`
}

func collaborationActivityKind(kind string) string {
	switch kind {
	case "created", "participant_added", "participant_removed", "participant_left", "transferred",
		"information_requested", "information_returned", "acceptance_submitted", "status_changed",
		"completed", "cancelled", "reopened", "issue_edited", "iteration_changed", "comment_created", "comment_edited", "comment_deleted":
		return kind
	default:
		return "other"
	}
}

func (s *CollaborationService) ListActivity(ctx context.Context, issueID pgtype.UUID, p CollaborationActivityQuery) (CollaborationActivityPage, error) {
	out := CollaborationActivityPage{Items: make([]CollaborationActivityEntry, 0)}
	if p.Limit < 1 || p.Limit > 100 || p.BeforeSeq.Valid && p.BeforeSeq.Int64 < 1 {
		return out, collaborationError(http.StatusBadRequest, "invalid_activity_query")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		rows, err := q.ListCollaborationActivityEvents(ctx, db.ListCollaborationActivityEventsParams{IssueID: issueID, BeforeSeq: p.BeforeSeq, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		if len(rows) > p.Limit {
			out.HasMore, rows = true, rows[:p.Limit]
		}
		payloads := make([]collaborationActivityPayload, len(rows))
		valid := make([]bool, len(rows))
		users, handoffIDs := make([]pgtype.UUID, 0, len(rows)*2), make([]pgtype.UUID, 0)
		for i, row := range rows {
			users = append(users, row.ActorUserID)
			valid[i] = collaborationActivityKind(row.Kind) != "other" && json.Unmarshal(row.Payload, &payloads[i]) == nil
			if valid[i] {
				users = append(users, payloads[i].TargetUserID)
				if payloads[i].HandoffID.Valid {
					handoffIDs = append(handoffIDs, payloads[i].HandoffID)
				}
			}
		}
		handoffs, err := q.ListCollaborationActivityHandoffs(ctx, db.ListCollaborationActivityHandoffsParams{IssueID: issueID, HandoffIds: handoffIDs})
		if err != nil {
			return err
		}
		byHandoff := make(map[pgtype.UUID]db.ListCollaborationActivityHandoffsRow, len(handoffs))
		for _, handoff := range handoffs {
			byHandoff[handoff.ID] = handoff
			users = append(users, handoff.FromUserID, handoff.ToUserID)
		}
		accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, users)
		if err != nil {
			return err
		}
		for i, row := range rows {
			entry := CollaborationActivityEntry{ID: row.ID, IssueID: issueID, EventSeq: row.EventSeq, Kind: collaborationActivityKind(row.Kind),
				Actor: accounts[row.ActorUserID], CreatedAt: row.CreatedAt.Time, Changes: make([]CollaborationActivityChange, 0)}
			if valid[i] {
				entry.DetailsAvailable = projectCollaborationActivity(&entry, payloads[i], row.ActorUserID, accounts, byHandoff)
			}
			if !entry.DetailsAvailable {
				entry.Target, entry.Handoff, entry.CommentID, entry.Changes = nil, nil, pgtype.UUID{}, make([]CollaborationActivityChange, 0)
				entry.Note = ""
			}
			out.Items = append(out.Items, entry)
		}
		return nil
	})
	return out, err
}

func projectCollaborationActivity(entry *CollaborationActivityEntry, payload collaborationActivityPayload, actor pgtype.UUID,
	accounts map[pgtype.UUID]*CollaborationContentAccount, handoffs map[pgtype.UUID]db.ListCollaborationActivityHandoffsRow) bool {
	switch entry.Kind {
	case "created":
		return true
	case "comment_created", "comment_edited", "comment_deleted":
		entry.CommentID = payload.CommentID
		return payload.CommentID.Valid
	case "status_changed", "completed", "cancelled", "reopened":
		if payload.PreviousStatus == nil || payload.Status == nil || *payload.PreviousStatus == "" || *payload.Status == "" {
			return false
		}
		entry.Changes = append(entry.Changes, CollaborationActivityChange{Field: "status", Before: payload.PreviousStatus, After: payload.Status})
		return true
	case "iteration_changed":
		if len(payload.Reason) > 2000 {
			return false
		}
		entry.Note = payload.Reason
		changes, ok := collaborationActivityEdits(payload.Before, payload.After)
		if !ok || len(changes) != 1 || changes[0].Field != "current_iteration_id" {
			return false
		}
		entry.Changes = changes
		return true
	case "issue_edited":
		var ok bool
		entry.Changes, ok = collaborationActivityEdits(payload.Before, payload.After)
		return ok
	case "participant_added", "participant_removed", "participant_left", "transferred", "information_requested", "information_returned", "acceptance_submitted":
		if !payload.TargetUserID.Valid {
			return false
		}
		entry.Target = accounts[payload.TargetUserID]
		purpose := map[string]string{"transferred": "transfer", "information_requested": "information", "information_returned": "information_return", "acceptance_submitted": "acceptance", "participant_removed": "removal_return"}[entry.Kind]
		if !payload.HandoffID.Valid {
			// Older events have no exact link. Never guess a note by timestamp.
			return entry.Kind == "participant_added" || entry.Kind == "participant_removed" || entry.Kind == "participant_left"
		}
		handoff, found := handoffs[payload.HandoffID]
		if !found || handoff.ActorUserID != actor || purpose == "" || handoff.Purpose != purpose ||
			(entry.Kind != "participant_removed" && handoff.ToUserID != payload.TargetUserID) {
			return false
		}
		entry.Handoff = &CollaborationActivityHandoff{ID: handoff.ID, Purpose: handoff.Purpose, From: accounts[handoff.FromUserID], To: accounts[handoff.ToUserID], Note: handoff.Note}
		return true
	default:
		return false
	}
}

func collaborationActivityEdits(before, after json.RawMessage) ([]CollaborationActivityChange, bool) {
	var oldFields, newFields map[string]json.RawMessage
	changes := make([]CollaborationActivityChange, 0)
	if json.Unmarshal(before, &oldFields) != nil || json.Unmarshal(after, &newFields) != nil || oldFields == nil || newFields == nil {
		return changes, false
	}
	for _, field := range []string{"title", "description", "start_date", "due_date", "priority", "labels", "properties", "project_id", "parent_issue_id", "stage", "current_iteration_id"} {
		oldRaw, oldPresent := oldFields[field]
		newRaw, newPresent := newFields[field]
		if !oldPresent && !newPresent {
			continue
		}
		oldValue, oldOK := collaborationActivityField(field, oldRaw)
		newValue, newOK := collaborationActivityField(field, newRaw)
		if !oldPresent || !newPresent || !oldOK || !newOK {
			return nil, false
		}
		if oldValue == nil && newValue == nil || oldValue != nil && newValue != nil && *oldValue == *newValue {
			continue
		}
		changes = append(changes, CollaborationActivityChange{Field: field, Before: oldValue, After: newValue})
	}
	return changes, true
}

func collaborationActivityField(field string, raw json.RawMessage) (*string, bool) {
	if field == "stage" {
		var stage *int32
		if json.Unmarshal(raw, &stage) != nil || stage != nil && *stage < 1 {
			return nil, false
		}
		if stage == nil {
			return nil, true
		}
		value := strconv.FormatInt(int64(*stage), 10)
		return &value, true
	}
	if field == "labels" {
		var labels []struct {
			ID   pgtype.UUID `json:"id"`
			Name string      `json:"name"`
		}
		if json.Unmarshal(raw, &labels) != nil {
			return nil, false
		}
		names := make([]string, 0, len(labels))
		for _, label := range labels {
			if !label.ID.Valid {
				return nil, false
			}
			names = append(names, label.Name)
		}
		value := strings.Join(names, ", ")
		return &value, true
	}
	if field == "properties" {
		var properties map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&properties) != nil {
			return nil, false
		}
		keys := make([]string, 0, len(properties))
		for key := range properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		values := make([]string, 0, len(keys))
		for _, key := range keys {
			if _, err := util.ParseUUID(key); err != nil {
				return nil, false
			}
			switch value := properties[key].(type) {
			case string, json.Number, bool, nil:
			case []any:
				for _, item := range value {
					if _, ok := item.(string); !ok {
						return nil, false
					}
				}
			default:
				return nil, false
			}
			encoded, err := json.Marshal(properties[key])
			if err != nil {
				return nil, false
			}
			values = append(values, key+": "+string(encoded))
		}
		value := strings.Join(values, "\n")
		return &value, true
	}
	var value *string
	if json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	if value == nil && (field == "title" || field == "priority") {
		return nil, false
	}
	if value != nil && (field == "start_date" || field == "due_date") {
		if _, err := util.ParseCalendarDate(*value); err != nil {
			return nil, false
		}
	}
	if value != nil && (field == "project_id" || field == "parent_issue_id" || field == "current_iteration_id") {
		if _, err := util.ParseUUID(*value); err != nil {
			return nil, false
		}
	}
	return value, true
}
