package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type CollaborationRunResource struct {
	AgentID              string  `json:"agent_id"`
	RuntimeID            string  `json:"runtime_id"`
	ResourceOwnerUserID  string  `json:"resource_owner_user_id"`
	ExecutionWorkspaceID string  `json:"execution_workspace_id"`
	GrantID              string  `json:"grant_id"`
	Provider             *string `json:"provider"`
	Model                *string `json:"model"`
	AccountSource        *string `json:"account_source"`
}

type collaborationResourceRecording struct {
	Provider      *string `json:"provider"`
	Model         *string `json:"model"`
	AccountSource *string `json:"account_source"`
}

// This is configured resource provenance at claim, not a billing assertion or
// an inference from subsequent live resource names/configuration.
func (s *CollaborationService) freezeResourceRecording(ctx context.Context, r *collaborationRuntimeState) error {
	runtime, err := s.collaborationRuntimeLookup(r.q).Get(ctx, r.run.RuntimeID)
	if err != nil {
		return err
	}
	agent, err := r.q.GetAgent(ctx, r.run.AgentID)
	if err != nil {
		return err
	}
	label := func(value string) *string {
		if value == "" || len(value) > 128 || !utf8.ValidString(value) || strings.HasPrefix(value, "/") || strings.HasPrefix(value, ".") || strings.HasPrefix(value, "~") || strings.Contains(value, "://") || strings.Contains(value, "..") {
			return nil
		}
		for _, char := range value {
			if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-:/", char) {
				continue
			}
			return nil
		}
		return &value
	}
	resource := collaborationResourceRecording{Provider: label(runtime.Provider), Model: label(agent.Model.String)}
	if resource.Provider != nil {
		source := "runtime_configuration"
		resource.AccountSource = &source
	}
	payload, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	_, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: r.run.ID, EventID: "resource-snapshot", Kind: "resource_snapshot", Payload: payload})
	return err
}

// Optional fields belong only to their discriminant. This explicit projection
// never serializes the raw runtime event (which also holds private capabilities
// and installation/attempt identity).
type CollaborationRunRecordItem struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Body          string    `json:"body"`
	CommandID     string    `json:"command_id,omitempty"`
	AuthorUserID  string    `json:"author_user_id,omitempty"`
	AttachmentIDs *[]string `json:"attachment_ids,omitempty"`
	Role          string    `json:"role,omitempty"`
	CallID        string    `json:"call_id,omitempty"`
	Name          string    `json:"name,omitempty"`
	IsError       *bool     `json:"is_error,omitempty"`
	CommentID     string    `json:"comment_id,omitempty"`
	Outcome       string    `json:"outcome,omitempty"`
}

type CollaborationRunRecording struct {
	Status         string `json:"status"`
	InputsRecorded bool   `json:"inputs_recorded"`
}

type CollaborationRunRecordsPage struct {
	ProtocolVersion int                             `json:"protocol_version"`
	IssueID         string                          `json:"issue_id"`
	Run             CollaborationExecutionRunView   `json:"run"`
	Resource        CollaborationRunResource        `json:"resource"`
	Recording       CollaborationRunRecording       `json:"recording"`
	Preparation     *CollaborationRunPreparation    `json:"preparation,omitempty"`
	Usage           *protocol.CollaborationRunUsage `json:"usage"`
	Items           []CollaborationRunRecordItem    `json:"items"`
	NextCursor      *string                         `json:"next_cursor"`
}

type CollaborationRunRecordsQuery struct {
	Limit  int
	Cursor string
}

type collaborationRunRecordsCursor struct {
	IssueID     pgtype.UUID `json:"issue_id"`
	RunID       pgtype.UUID `json:"run_id"`
	PromptHash  string      `json:"prompt_hash"`
	InputOffset int         `json:"input_offset"`
	ThroughSeq  string      `json:"through_seq"`
	EventSeq    string      `json:"event_seq"`
	ItemOffset  int         `json:"item_offset"`
}

func collaborationRecordedRunView(run db.CollaborationRun) CollaborationExecutionRunView {
	return CollaborationExecutionRunView{
		ID: util.UUIDToString(run.ID), CommandID: util.UUIDToString(run.CommandID), TargetID: util.UUIDToString(run.TargetID), GroupID: util.UUIDToString(run.GroupID),
		ChannelID: util.UUIDToString(run.ChannelID), BatchID: util.UUIDToString(run.BatchID), ParentRunID: util.UUIDToPtr(run.ParentRunID),
		StepKey: run.StepKey.String, StepKind: run.StepKind.String, AgentID: util.UUIDToString(run.AgentID),
		TargetEpoch: strconv.FormatInt(run.TargetEpoch.Int64, 10), ConfigVersion: strconv.FormatInt(run.ConfigVersion.Int64, 10),
		ProgressState: run.ProgressState, ProofState: run.ProofState, BusinessOutcome: util.TextToPtr(run.BusinessOutcome), ForcedDisposition: util.TextToPtr(run.ForcedDisposition),
		RecordingIncomplete: run.RecordingIncomplete, SettledAt: util.TimestampToPtr(run.SettledAt),
	}
}

func (s *CollaborationService) RunRecords(ctx context.Context, issueID, runID pgtype.UUID, p CollaborationRunRecordsQuery) (CollaborationRunRecordsPage, error) {
	out := CollaborationRunRecordsPage{ProtocolVersion: 1, IssueID: util.UUIDToString(issueID), Items: []CollaborationRunRecordItem{}}
	invalidQuery := collaborationError(http.StatusBadRequest, "invalid_records_query")
	if !issueID.Valid || !runID.Valid || p.Limit < 1 || p.Limit > 100 || len(p.Cursor) > 1024 {
		return out, invalidQuery
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		run, err := q.GetCollaborationRecordedRun(ctx, db.GetCollaborationRecordedRunParams{IssueID: issueID, RunID: runID})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusNotFound, "run_not_found")
		}
		if err != nil {
			return err
		}
		if run.SessionID != access.SessionID || !run.GroupID.Valid || !run.AgentID.Valid || !run.RuntimeID.Valid || !run.ResourceOwnerUserID.Valid || !run.ExecutionWorkspaceID.Valid || !run.GrantID.Valid {
			return invalidCollaborationLedger("record identity invalid")
		}
		group, err := q.GetCollaborationExecutionGroup(ctx, db.GetCollaborationExecutionGroupParams{SessionID: access.SessionID, ID: run.GroupID})
		if err != nil {
			return invalidCollaborationLedger("record group unavailable")
		}
		if run.TargetID != group.TargetID || run.ChannelID != group.ChannelID || run.BatchID != group.BatchID || run.CommandID != group.CommandID || !run.TargetEpoch.Valid || run.TargetEpoch.Int64 != group.TargetEpoch || !run.ConfigVersion.Valid || run.ConfigVersion.Int64 != group.ConfigVersion || !run.StepKey.Valid || run.StepKey.String == "" || run.SettledAt.Valid && !collaborationRunProvenEnded(run) {
			return invalidCollaborationLedger("record group identity invalid")
		}
		switch run.StepKind.String {
		case "agent", "leader", "member", "continuation":
		default:
			return invalidCollaborationLedger("record step invalid")
		}
		switch run.ProgressState {
		case "prepared", "dispatched", "preparing", "launch_authorized", "running", "ending", "settled":
		default:
			return invalidCollaborationLedger("record progress invalid")
		}
		switch run.ProofState {
		case "unknown", "never_launched_proven", "never_started_proven", "exited_proven":
		default:
			return invalidCollaborationLedger("record proof invalid")
		}
		out.Run = collaborationRecordedRunView(run)
		out.Resource = CollaborationRunResource{AgentID: util.UUIDToString(run.AgentID), RuntimeID: util.UUIDToString(run.RuntimeID), ResourceOwnerUserID: util.UUIDToString(run.ResourceOwnerUserID), ExecutionWorkspaceID: util.UUIDToString(run.ExecutionWorkspaceID), GrantID: util.UUIDToString(run.GrantID)}
		out.Recording = CollaborationRunRecording{Status: "not_recorded", InputsRecorded: run.PromptSnapshot != nil}
		if run.RecordingIncomplete {
			out.Recording.Status = "partial"
		}
		var inputs []protocol.CollaborationRunInput
		if run.PromptSnapshot != nil {
			// JSONB adds separator whitespace to the admitted compact snapshot.
			// Bound the stored representation without narrowing the claim limit.
			if len(run.PromptSnapshot) > 4<<20 {
				return invalidCollaborationLedger("record prompt too large")
			}
			inputs, err = decodeCollaborationRoster[protocol.CollaborationRunInput](run.PromptSnapshot)
			if err != nil || inputs == nil || len(inputs) > 1024 {
				return invalidCollaborationLedger("record prompt invalid")
			}
		}
		digest := sha256.Sum256(run.PromptSnapshot)
		latest, err := q.GetCollaborationRunRecordSequence(ctx, db.GetCollaborationRunRecordSequenceParams{IssueID: issueID, RunID: runID})
		if err != nil {
			return err
		}
		cursor := collaborationRunRecordsCursor{IssueID: issueID, RunID: runID, PromptHash: hex.EncodeToString(digest[:]), ThroughSeq: strconv.FormatInt(latest, 10), EventSeq: "1"}
		if p.Cursor != "" {
			raw, err := base64.RawURLEncoding.DecodeString(p.Cursor)
			var supplied collaborationRunRecordsCursor
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err != nil || iteration.ValidateObjectJSON(raw) != nil || decoder.Decode(&supplied) != nil || supplied.IssueID != issueID || supplied.RunID != runID || supplied.PromptHash != hex.EncodeToString(digest[:]) {
				return invalidQuery
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(raw, &fields) != nil || len(fields) != 7 {
				return invalidQuery
			}
			for _, key := range []string{"issue_id", "run_id", "prompt_hash", "input_offset", "through_seq", "event_seq", "item_offset"} {
				value, exists := fields[key]
				if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
					return invalidQuery
				}
			}
			cursor = supplied
		}
		through, throughOK := collaborationRuntimeVersion(cursor.ThroughSeq, true)
		position, positionOK := collaborationRuntimeVersion(cursor.EventSeq, false)
		if !throughOK || through > latest || !positionOK || position > through && position-through != 1 || cursor.InputOffset < 0 || cursor.InputOffset > len(inputs) || cursor.ItemOffset < 0 || cursor.ItemOffset > protocol.CollaborationRecordItemLimit {
			return invalidQuery
		}
		out.Preparation, err = readCollaborationPreparationRecords(ctx, q, issueID, run, inputs, through)
		if err != nil {
			return err
		}
		metadata, err := q.GetCollaborationRunRecordMetadata(ctx, db.GetCollaborationRunRecordMetadataParams{IssueID: issueID, RunID: runID})
		if err != nil {
			return err
		}
		seenMetadata := map[string]bool{}
		for _, event := range metadata {
			if event.Seq > through {
				continue
			}
			if seenMetadata[event.Kind] {
				return invalidCollaborationLedger("duplicate record metadata")
			}
			seenMetadata[event.Kind] = true
			switch event.Kind {
			case "resource_snapshot":
				var saved collaborationResourceRecording
				if json.Unmarshal(event.Payload, &saved) != nil {
					return invalidCollaborationLedger("resource record invalid")
				}
				for _, label := range []*string{saved.Provider, saved.Model} {
					if label != nil && (*label == "" || len(*label) > 128) {
						return invalidCollaborationLedger("resource record invalid")
					}
				}
				if saved.AccountSource != nil && *saved.AccountSource != "runtime_configuration" {
					return invalidCollaborationLedger("resource record invalid")
				}
				out.Resource.Provider, out.Resource.Model, out.Resource.AccountSource = saved.Provider, saved.Model, saved.AccountSource
			case "outcome":
				var saved protocol.CollaborationRunOutcome
				if json.Unmarshal(event.Payload, &saved) != nil || saved.Identity.RunID != util.UUIDToString(event.RunID) || saved.Identity.IssueID != util.UUIDToString(event.IssueID) || protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: saved.Outcome, Body: saved.Body, Record: saved.Record}) != nil {
					return invalidCollaborationLedger("outcome record invalid")
				}
				if saved.Record != nil {
					out.Recording.Status, out.Usage = saved.Record.Completeness, saved.Record.Usage
					if run.RecordingIncomplete {
						out.Recording.Status = "partial"
					}
				}
			}
		}
		pageBytes := 0
		appendItem := func(item CollaborationRunRecordItem) bool {
			encoded, _ := json.Marshal(item)
			// One legal historic large result may exceed the page budget; always
			// advance it intact so pagination neither truncates nor livelocks.
			if len(out.Items) >= p.Limit || len(out.Items) > 0 && pageBytes+len(encoded) > 256<<10 {
				return false
			}
			out.Items = append(out.Items, item)
			pageBytes += len(encoded)
			return true
		}
		saveCursor := func() error {
			raw, err := json.Marshal(cursor)
			if err != nil {
				return err
			}
			encoded := base64.RawURLEncoding.EncodeToString(raw)
			out.NextCursor = &encoded
			return nil
		}
		seenInputs := map[string]bool{}
		for _, input := range inputs {
			if _, err := util.ParseUUID(input.CommandID); err != nil || seenInputs[input.CommandID] {
				return invalidCollaborationLedger("record input invalid")
			}
			if _, err := util.ParseUUID(input.AuthorUserID); err != nil {
				return invalidCollaborationLedger("record input author invalid")
			}
			seenInputs[input.CommandID] = true
			attachments := map[string]bool{}
			for _, id := range input.AttachmentIDs {
				if _, err := util.ParseUUID(id); err != nil || attachments[id] {
					return invalidCollaborationLedger("record attachment invalid")
				}
				attachments[id] = true
			}
		}
		for cursor.InputOffset < len(inputs) {
			input := inputs[cursor.InputOffset]
			attachments := input.AttachmentIDs
			if attachments == nil {
				attachments = []string{}
			}
			item := CollaborationRunRecordItem{ID: "input:" + input.CommandID, Kind: "input", Body: input.Body, CommandID: input.CommandID, AuthorUserID: input.AuthorUserID, AttachmentIDs: &attachments}
			if !appendItem(item) {
				return saveCursor()
			}
			cursor.InputOffset++
		}
		for position <= through {
			rows, err := q.ListCollaborationRunRecordEvents(ctx, db.ListCollaborationRunRecordEventsParams{IssueID: issueID, RunID: runID, AfterSeq: position, ThroughSeq: through, RowLimit: 1})
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				break
			}
			event := rows[0]
			if event.Seq != position && cursor.ItemOffset != 0 {
				return invalidQuery
			}
			cursor.EventSeq = strconv.FormatInt(event.Seq, 10)
			items, err := collaborationRecordEventItems(event)
			if err != nil {
				return err
			}
			if cursor.ItemOffset >= len(items) {
				return invalidQuery
			}
			for cursor.ItemOffset < len(items) {
				if !appendItem(items[cursor.ItemOffset]) {
					return saveCursor()
				}
				cursor.ItemOffset++
			}
			if event.Seq == math.MaxInt64 {
				break
			}
			position = event.Seq + 1
			cursor.EventSeq = strconv.FormatInt(position, 10)
			cursor.ItemOffset = 0
		}
		return nil
	})
	return out, err
}

func collaborationRecordEventItems(event db.CollaborationRunEvent) ([]CollaborationRunRecordItem, error) {
	items := []CollaborationRunRecordItem{}
	invalid := invalidCollaborationLedger("run event record invalid")
	switch event.Kind {
	case "outcome":
		var saved protocol.CollaborationRunOutcome
		if json.Unmarshal(event.Payload, &saved) != nil || saved.Identity.RunID != util.UUIDToString(event.RunID) || saved.Identity.IssueID != util.UUIDToString(event.IssueID) || protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: saved.Outcome, Body: saved.Body, Record: saved.Record}) != nil {
			return nil, invalid
		}
		if saved.Record != nil {
			for _, message := range saved.Record.Messages {
				items = append(items, CollaborationRunRecordItem{Kind: message.Kind, Body: message.Body, Role: message.Role, CallID: message.CallID, Name: message.Name, IsError: message.IsError})
			}
		}
		items = append(items, CollaborationRunRecordItem{Kind: "result", Outcome: saved.Outcome, Body: saved.Body})
	case "comment":
		var saved struct {
			Input struct {
				CommentID pgtype.UUID `json:"comment_id"`
				Content   string      `json:"content"`
			} `json:"input"`
		}
		if json.Unmarshal(event.Payload, &saved) != nil || !saved.Input.CommentID.Valid {
			return nil, invalid
		}
		items = append(items, CollaborationRunRecordItem{Kind: "comment", CommentID: util.UUIDToString(saved.Input.CommentID), Body: saved.Input.Content})
	case "squad_activity":
		var saved struct {
			Outcome string `json:"outcome"`
			Reason  string `json:"reason"`
		}
		if json.Unmarshal(event.Payload, &saved) != nil || (saved.Outcome != "action" && saved.Outcome != "no_action" && saved.Outcome != "failed") {
			return nil, invalid
		}
		items = append(items, CollaborationRunRecordItem{Kind: "squad_activity", Outcome: saved.Outcome, Body: saved.Reason})
	default:
		return nil, invalid
	}
	for i := range items {
		items[i].ID = fmt.Sprintf("event:%s:%d", util.UUIDToString(event.ID), i)
	}
	return items, nil
}
