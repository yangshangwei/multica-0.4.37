package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type CollaborationHandoffQuery struct{ Cursor string }
type collaborationHandoffCursor struct {
	Kind      string     `json:"kind"`
	RunID     string     `json:"run_id"`
	Offset    int        `json:"offset,omitempty"`
	AfterID   string     `json:"after_id,omitempty"`
	BeforeAt  *time.Time `json:"before_at,omitempty"`
	CommandID string     `json:"command_id,omitempty"`
}

func handoffCursor(c collaborationHandoffCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}
func parseHandoffCursor(raw, runID string) (collaborationHandoffCursor, error) {
	var c collaborationHandoffCursor
	if raw == "" {
		c.RunID = runID
		return c, nil
	}
	invalid := collaborationError(http.StatusBadRequest, "invalid_handoff_cursor")
	if len(raw) > 1024 {
		return c, invalid
	}
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return c, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	token, e := decoder.Token()
	if e != nil || token != json.Delim('{') {
		return c, invalid
	}
	// Decode canonical fields once. Struct decoding otherwise accepts duplicate
	// and case-insensitive keys, and prefilled values can hide a missing run ID.
	seen := make(map[string]bool, 6)
	for decoder.More() {
		token, e = decoder.Token()
		name, ok := token.(string)
		if e != nil || !ok || seen[name] {
			return c, invalid
		}
		seen[name] = true
		var field any
		switch name {
		case "kind":
			field = &c.Kind
		case "run_id":
			field = &c.RunID
		case "offset":
			field = &c.Offset
		case "after_id":
			field = &c.AfterID
		case "before_at":
			field = &c.BeforeAt
		case "command_id":
			field = &c.CommandID
		default:
			return c, invalid
		}
		if decoder.Decode(field) != nil {
			return c, invalid
		}
	}
	if token, e = decoder.Token(); e != nil || token != json.Delim('}') {
		return c, invalid
	}
	if _, e = decoder.Token(); !errors.Is(e, io.EOF) {
		return c, invalid
	}
	if c.RunID == "" || c.RunID != runID || c.Offset < 0 {
		return c, invalid
	}
	switch c.Kind {
	case "inputs", "results", "causes", "readers":
	default:
		return c, invalid
	}
	for _, id := range []string{c.AfterID, c.CommandID} {
		if id != "" {
			if _, err := util.ParseUUID(id); err != nil {
				return c, invalid
			}
		}
	}
	return c, nil
}
func handoffID(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return util.UUIDToString(id)
}
func handoffDecimal(n int64) string { return strconv.FormatInt(n, 10) }
func handoffSource(kind, id, run, version, body string, maxBytes int) protocol.CollaborationHandoffSource {
	content := redact.Text(body)
	truncated := len(content) > maxBytes
	if truncated {
		content = content[:maxBytes]
		for !utf8.ValidString(content) {
			content = content[:len(content)-1]
		}
	}
	trust := "data_only"
	if kind == "delegation" {
		trust = "admitted_assignment"
	}
	return protocol.CollaborationHandoffSource{Kind: kind, ID: id, RunID: run, Version: version, SHA256: protocol.CollaborationHandoffHash(body), Content: content, Truncated: truncated, Availability: "available", Trust: trust}
}

func (s *CollaborationService) RuntimeHandoff(ctx context.Context, runID pgtype.UUID, token string, p CollaborationHandoffQuery) (protocol.CollaborationHandoffEnvelope, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationHandoffEnvelope, error) {
		r, e := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if e != nil {
			return protocol.CollaborationHandoffEnvelope{}, e
		}
		return buildCollaborationHandoff(ctx, r.q, r.access, r.run, p, nil)
	})
}
func (s *CollaborationService) RunHandoff(ctx context.Context, issueID, runID pgtype.UUID, p CollaborationHandoffQuery) (protocol.CollaborationHandoffEnvelope, error) {
	var out protocol.CollaborationHandoffEnvelope
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, a CollaborationIssueAccess) error {
		run, e := q.GetCollaborationRecordedRun(ctx, db.GetCollaborationRecordedRunParams{IssueID: issueID, RunID: runID})
		if e != nil {
			return e
		}
		out, e = buildCollaborationHandoff(ctx, q, a, run, p, nil)
		return e
	})
	return out, err
}

func buildCollaborationHandoff(ctx context.Context, q *db.Queries, a CollaborationIssueAccess, run db.CollaborationRun, p CollaborationHandoffQuery, memoryVersion *string) (protocol.CollaborationHandoffEnvelope, error) {
	out := protocol.CollaborationHandoffEnvelope{ProtocolVersion: 1, IssueID: handoffID(a.Issue.ID), RunID: handoffID(run.ID), Mode: "live", EventSequence: handoffDecimal(a.EventSequence),
		Authority:  protocol.CollaborationHandoffAuthority{Status: a.Issue.Status, HandlerID: handoffID(a.Issue.AssigneeID), CreatorID: handoffID(a.Issue.CreatorID)},
		Assignment: protocol.CollaborationHandoffAssignment{Role: run.StepKind.String, StepKey: run.StepKey.String, ParentRunID: handoffID(run.ParentRunID)}, Memory: protocol.CollaborationHandoffMemory{Version: "0", Availability: "absent"},
		Inputs: protocol.CollaborationHandoffInputs{Items: []protocol.CollaborationHandoffInput{}}, Results: protocol.CollaborationHandoffResults{Items: []protocol.CollaborationHandoffResult{}}, WakeupCauses: protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "unknown"}, Sources: []protocol.CollaborationHandoffSource{}, Missing: []string{}}
	c, err := parseHandoffCursor(p.Cursor, out.RunID)
	if err != nil {
		return out, err
	}
	if out.Assignment.Role == "" {
		out.Assignment.Role = "agent"
	}
	delegation, err := q.GetCollaborationHandoffDelegation(ctx, db.GetCollaborationHandoffDelegationParams{GroupID: run.GroupID, RunID: run.ID})
	if err == nil {
		body, event, e := immutableHandoffDelegation(ctx, q, a.Issue.ID, delegation)
		if e != nil {
			return out, e
		}
		out.Assignment.DelegationID = handoffID(delegation.ID)
		out.Assignment.SourceID = handoffID(event.ID)
		out.Sources = append(out.Sources, handoffSource("delegation", handoffID(event.ID), handoffID(event.RunID), handoffDecimal(event.Seq), body, 64<<10))
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	} else if out.Assignment.Role == "member" {
		out.Missing = append(out.Missing, "delegation_unavailable")
	}
	latest, err := q.GetCollaborationLatestHandoff(ctx, a.Issue.ID)
	if err == nil {
		t := protocol.CollaborationHandoffTransfer{ID: handoffID(latest.ID), AuthorID: handoffID(latest.ActorUserID), FromID: handoffID(latest.FromUserID), ToID: handoffID(latest.ToUserID), Purpose: latest.Purpose, RequesterID: handoffID(latest.RequesterUserID), EventSequence: handoffDecimal(latest.EventSeq), SourceID: handoffID(latest.ID)}
		out.Handoff = &t
		out.Sources = append(out.Sources, handoffSource("handoff", t.ID, "", "", latest.Note, 4096))
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if a.InformationRequestID.Valid {
		h, e := q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: a.Issue.ID, RequestID: a.InformationRequestID})
		if e != nil {
			return out, e
		}
		seq, e := q.GetCollaborationInformationRequestEventSequence(ctx, db.GetCollaborationInformationRequestEventSequenceParams{IssueID: a.Issue.ID, RequestID: h.ID})
		if errors.Is(e, pgx.ErrNoRows) {
			// Zero is the legacy unknown sentinel; identify the missing provenance
			// explicitly rather than attaching a later return's event sequence.
			out.Missing = append(out.Missing, "information_request_event_unknown:"+handoffID(h.ID))
		} else if e != nil {
			return out, e
		}
		t := protocol.CollaborationHandoffTransfer{ID: handoffID(h.ID), AuthorID: handoffID(h.ActorUserID), FromID: handoffID(h.FromUserID), ToID: handoffID(h.ToUserID), Purpose: h.Purpose, RequesterID: handoffID(h.RequesterUserID), EventSequence: handoffDecimal(seq), SourceID: handoffID(h.ID)}
		out.InformationRequest = &t
		if out.Handoff == nil || out.Handoff.ID != t.ID {
			out.Sources = append(out.Sources, handoffSource("handoff", t.ID, "", "", h.Note, 4096))
		}
	}
	memory, e := currentCollaborationMemory(ctx, q, a.Issue.ID)
	if e != nil {
		return out, e
	}
	if memoryVersion != nil {
		if *memoryVersion == "" {
			memory = nil
			out.Memory.Availability = "unsupported"
		} else if *memoryVersion == "0" {
			memory = nil
		} else {
			v, e := strconv.ParseInt(*memoryVersion, 10, 64)
			if e != nil || !protocol.HandoffDecimal(*memoryVersion, true) {
				return out, collaborationError(http.StatusBadRequest, "invalid_memory_version")
			}
			m, e := q.GetCollaborationMemoryVersion(ctx, db.GetCollaborationMemoryVersionParams{IssueID: a.Issue.ID, Version: v})
			if e != nil {
				return out, e
			}
			memory = &m
		}
	}
	if memory != nil {
		out.Memory = protocol.CollaborationHandoffMemory{Version: handoffDecimal(memory.Version), SHA256: protocol.CollaborationHandoffHash(memory.Content), Availability: "available"}
		if memory.PurgedAt.Valid {
			out.Memory.Availability = "purged"
			out.Memory.SHA256 = ""
			out.Missing = append(out.Missing, "memory_purged")
		}
	}
	if err = projectHandoffInputs(ctx, q, run, c, &out); err != nil {
		return out, err
	}
	if err = projectHandoffResults(ctx, q, a, run, c, &out); err != nil {
		return out, err
	}
	if err = projectHandoffCauses(ctx, q, a, run, c, &out); err != nil {
		return out, err
	}
	through, err := q.GetCollaborationRunRecordSequence(ctx, db.GetCollaborationRunRecordSequenceParams{IssueID: a.Issue.ID, RunID: run.ID})
	if err != nil {
		return out, err
	}
	recording, err := readCollaborationRecording(ctx, q, a.Issue.ID, run.ID, through)
	if err != nil {
		return out, err
	}
	out.Recording = recording.retention()
	if err = boundCollaborationHandoff(&out); err != nil {
		return out, invalidCollaborationLedger("invalid handoff projection")
	}
	return out, nil
}

func immutableHandoffDelegation(ctx context.Context, q *db.Queries, issueID pgtype.UUID, d db.CollaborationDelegation) (string, db.CollaborationRunEvent, error) {
	e, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: d.ParentRunID, ProducerEventID: "comment:" + handoffID(d.SourceCommentID)})
	if err != nil {
		return "", e, err
	}
	var saved struct {
		Input CollaborationRuntimeCommentParams `json:"input"`
	}
	if e.IssueID != issueID || e.Kind != "comment" || json.Unmarshal(e.Payload, &saved) != nil || saved.Input.CommentID != d.SourceCommentID || collaborationCapabilityHash(saved.Input.Content) != d.PayloadHash {
		return "", e, invalidCollaborationLedger("delegation source mismatch")
	}
	return saved.Input.Content, e, nil
}

func projectHandoffInputs(ctx context.Context, q *db.Queries, run db.CollaborationRun, c collaborationHandoffCursor, out *protocol.CollaborationHandoffEnvelope) error {
	prompt := []protocol.CollaborationRunInput{}
	var err error
	if run.PromptSnapshot != nil {
		prompt, err = decodeCollaborationRoster[protocol.CollaborationRunInput](run.PromptSnapshot)
	}
	if err != nil {
		return err
	}
	out.Inputs.Total = len(prompt)
	start := 0
	if c.Kind == "inputs" {
		start = c.Offset
	}
	if start > len(prompt) {
		return collaborationError(http.StatusBadRequest, "invalid_handoff_cursor")
	}
	if c.Kind == "readers" {
		start = -1
		for i, v := range prompt {
			if v.CommandID == c.CommandID {
				start = i
				break
			}
		}
		if start < 0 {
			return collaborationError(http.StatusBadRequest, "invalid_handoff_cursor")
		}
	}
	end := min(len(prompt), start+32)
	if c.Kind == "readers" {
		end = start + 1
	}
	associations, err := q.ListCollaborationGroupInputs(ctx, run.GroupID)
	if err != nil {
		return err
	}
	byID := map[string]db.CollaborationGroupInput{}
	for _, i := range associations {
		byID[handoffID(i.CommandID)] = i
	}
	for _, p := range prompt[start:end] {
		id, _ := util.ParseUUID(p.CommandID)
		command, e := q.GetCollaborationHandoffCommand(ctx, db.GetCollaborationHandoffCommandParams{SessionID: run.SessionID, ID: id})
		if e != nil {
			return e
		}
		i := protocol.CollaborationHandoffInput{CommandID: p.CommandID, AuthorID: p.AuthorUserID, State: command.State, ReaderRunIDs: []string{}}
		if association, ok := byID[p.CommandID]; ok {
			var after pgtype.UUID
			if c.Kind == "readers" && c.AfterID != "" {
				after, _ = util.ParseUUID(c.AfterID)
			}
			rows, e := q.ListCollaborationHandoffReaderRuns(ctx, db.ListCollaborationHandoffReaderRunsParams{GroupID: run.GroupID, AfterID: after, RowLimit: 33})
			if e != nil {
				return e
			}
			if len(rows) > 32 {
				i.ReadersTruncated = true
				rows = rows[:32]
				i.ReadersCursor = handoffCursor(collaborationHandoffCursor{Kind: "readers", RunID: out.RunID, CommandID: p.CommandID, AfterID: handoffID(rows[len(rows)-1].ID)})
			}
			selected := map[pgtype.UUID]bool{}
			for _, r := range rows {
				selected[r.ID] = true
			}
			if !selected[association.ConsumingRunID] {
				consumer, e := q.GetCollaborationRuntimeRun(ctx, association.ConsumingRunID)
				if e != nil {
					return e
				}
				rows = append(rows, consumer)
			}
			readers, e := collaborationInputReaders(association, rows)
			if e != nil {
				return e
			}
			for _, r := range readers {
				if selected[r.ID] {
					i.ReaderRunIDs = append(i.ReaderRunIDs, handoffID(r.ID))
				}
			}
		}
		out.Inputs.Items = append(out.Inputs.Items, i)
	}
	out.Inputs.Returned = len(out.Inputs.Items)
	if end < len(prompt) && c.Kind != "readers" {
		out.Inputs.Truncated = true
		out.Inputs.NextCursor = handoffCursor(collaborationHandoffCursor{Kind: "inputs", RunID: out.RunID, Offset: end})
	}
	return nil
}

func projectHandoffResults(ctx context.Context, q *db.Queries, a CollaborationIssueAccess, run db.CollaborationRun, c collaborationHandoffCursor, out *protocol.CollaborationHandoffEnvelope) error {
	total, err := q.CountCollaborationHandoffRuns(ctx, run.SessionID)
	if err != nil {
		return err
	}
	out.Results.Total = int(total)
	p := db.ListCollaborationHandoffRunsParams{SessionID: run.SessionID, RowLimit: 33}
	if c.Kind == "results" {
		if c.BeforeAt == nil || c.AfterID == "" {
			return collaborationError(http.StatusBadRequest, "invalid_handoff_cursor")
		}
		p.BeforeAt = pgtype.Timestamptz{Time: *c.BeforeAt, Valid: true}
		p.BeforeID, _ = util.ParseUUID(c.AfterID)
	}
	rows, err := q.ListCollaborationHandoffRuns(ctx, p)
	if err != nil {
		return err
	}
	if len(rows) > 32 {
		rows = rows[:32]
		last := rows[len(rows)-1]
		out.Results.Truncated = true
		out.Results.NextCursor = handoffCursor(collaborationHandoffCursor{Kind: "results", RunID: out.RunID, BeforeAt: &last.CreatedAt.Time, AfterID: handoffID(last.ID)})
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	events, err := q.ListCollaborationHandoffRunResults(ctx, db.ListCollaborationHandoffRunResultsParams{IssueID: a.Issue.ID, Column2: ids})
	if err != nil {
		return err
	}
	byRun := map[pgtype.UUID]db.CollaborationRunEvent{}
	for _, e := range events {
		prior, exists := byRun[e.RunID]
		if !exists || prior.Kind != "outcome" {
			byRun[e.RunID] = e
		}
	}
	for _, r := range rows {
		i := protocol.CollaborationHandoffResult{RunID: handoffID(r.ID), ParentRunID: handoffID(r.ParentRunID), Role: r.StepKind.String, StepKey: r.StepKey.String, Progress: r.ProgressState, Outcome: r.BusinessOutcome.String, ProofState: r.ProofState, RecordingIncomplete: r.RecordingIncomplete, ResultState: "absent", FinalAnswerState: "unknown"}
		if i.Role == "" {
			i.Role = "agent"
		}
		if r.LeaderTurn.Valid {
			i.LeaderTurn = handoffDecimal(r.LeaderTurn.Int64)
		}
		if e, ok := byRun[r.ID]; ok {
			body, kind, err := handoffResultText(e)
			if err != nil {
				return err
			}
			i.ResultState = "present"
			i.SourceID = handoffID(e.ID)
			i.SourceKind = kind
			out.Sources = append(out.Sources, handoffSource(kind, i.SourceID, i.RunID, handoffDecimal(e.Seq), body, 4096))
		} else if r.SettledAt.Valid {
			i.FinalAnswerState = "absent"
		}
		out.Results.Items = append(out.Results.Items, i)
	}
	out.Results.Returned = len(rows)
	return nil
}
func handoffResultText(e db.CollaborationRunEvent) (string, string, error) {
	if e.Kind == "outcome" {
		var p struct {
			Body string `json:"body"`
		}
		if json.Unmarshal(e.Payload, &p) != nil {
			return "", "", invalidCollaborationLedger("invalid result source")
		}
		return p.Body, "provider_result", nil
	}
	var p struct {
		Reason string `json:"reason"`
	}
	if e.Kind != "squad_activity" || json.Unmarshal(e.Payload, &p) != nil {
		return "", "", invalidCollaborationLedger("invalid activity source")
	}
	return p.Reason, "squad_activity", nil
}

// Retain the complete admitted assignment. Optional source text is a bounded
// prefix with its original reference/hash and an explicit per-source marker.
func boundCollaborationHandoff(out *protocol.CollaborationHandoffEnvelope) error {
	material, wire, err := protocol.CollaborationHandoffSizes(*out)
	if err != nil {
		return err
	}
	if material <= protocol.CollaborationHandoffMaxBytes && wire <= protocol.CollaborationHandoffMaxJSONBytes {
		return protocol.ValidateCollaborationHandoff(*out)
	}
	full := append([]protocol.CollaborationHandoffSource(nil), out.Sources...)
	for i := range out.Sources {
		if out.Sources[i].Kind != "delegation" {
			out.Sources[i].Content = ""
			out.Sources[i].Truncated = full[i].Truncated || full[i].Content != ""
		}
	}
	material, _, err = protocol.CollaborationHandoffSizes(*out)
	if err != nil || material > protocol.CollaborationHandoffMaxBytes {
		return invalidCollaborationLedger("handoff metadata and assignment exceed material budget")
	}
	for i, source := range full {
		if source.Kind == "delegation" {
			continue
		}
		// Reserve the extra byte needed when truncated changes true -> false.
		limit := max(0, protocol.CollaborationHandoffMaxBytes-material-1)
		end := min(len(source.Content), limit)
		for !utf8.ValidString(source.Content[:end]) {
			end--
		}
		out.Sources[i].Content = source.Content[:end]
		out.Sources[i].Truncated = source.Truncated || end < len(source.Content)
		material, _, err = protocol.CollaborationHandoffSizes(*out)
		if err != nil {
			return err
		}
	}
	return protocol.ValidateCollaborationHandoff(*out)
}
