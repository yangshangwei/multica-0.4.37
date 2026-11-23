package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type collaborationStoredHandoff struct {
	Identity      protocol.CollaborationRunIdentity     `json:"identity"`
	MemoryVersion string                                `json:"memory_version"`
	Envelope      protocol.CollaborationHandoffEnvelope `json:"envelope"`
}

func (s *CollaborationService) RuntimeHandoffSnapshot(ctx context.Context, runID pgtype.UUID, token string, p protocol.CollaborationHandoffSnapshotRequest) (protocol.CollaborationHandoffEnvelope, error) {
	var empty protocol.CollaborationHandoffEnvelope
	if _, err := util.ParseUUID(p.RequestID); err != nil || p.MemoryVersion != "" && !protocol.HandoffDecimal(p.MemoryVersion, true) {
		return empty, collaborationError(http.StatusBadRequest, "invalid_handoff_snapshot")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationHandoffEnvelope, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return empty, err
		}
		saved, err := loadStoredHandoff(ctx, r.q, runID, p.RequestID)
		if err == nil {
			if saved.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) || saved.MemoryVersion != p.MemoryVersion {
				return empty, collaborationError(http.StatusConflict, "handoff_snapshot_conflict")
			}
			return restoreStoredHandoff(ctx, r.q, r.access, r.run, saved)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return empty, err
		}
		count, err := r.q.CountCollaborationHandoffSnapshots(ctx, runID)
		if err != nil {
			return empty, err
		}
		if count >= 32 {
			return empty, collaborationError(http.StatusConflict, "handoff_snapshot_limit")
		}
		out, err := buildCollaborationHandoff(ctx, r.q, r.access, r.run, CollaborationHandoffQuery{}, &p.MemoryVersion)
		if err != nil {
			return empty, err
		}
		if out.Memory.Availability == "purged" {
			return empty, collaborationError(http.StatusConflict, "handoff_source_unavailable")
		}
		out.Mode, out.SnapshotID = "snapshot", p.RequestID
		if err = boundCollaborationHandoff(&out); err != nil {
			return empty, err
		}
		saved = collaborationStoredHandoff{Identity: collaborationRunIdentity(r.run, r.access.Issue.ID), MemoryVersion: p.MemoryVersion, Envelope: out}
		saved.Envelope.Sources = append([]protocol.CollaborationHandoffSource{}, out.Sources...)
		for i := range saved.Envelope.Sources {
			saved.Envelope.Sources[i].Content = ""
		}
		payload, err := json.Marshal(saved)
		if err != nil {
			return empty, err
		}
		if _, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: runID, EventID: "handoff:" + p.RequestID, Kind: "handoff_snapshot", Payload: payload}); err != nil {
			return empty, err
		}
		return out, nil
	})
}
func loadStoredHandoff(ctx context.Context, q *db.Queries, runID pgtype.UUID, id string) (collaborationStoredHandoff, error) {
	var saved collaborationStoredHandoff
	e, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: "handoff:" + id})
	if err != nil {
		return saved, err
	}
	if e.Kind != "handoff_snapshot" || json.Unmarshal(e.Payload, &saved) != nil || saved.Envelope.SnapshotID != id {
		return saved, invalidCollaborationLedger("invalid saved handoff")
	}
	return saved, nil
}
func restoreStoredHandoff(ctx context.Context, q *db.Queries, a CollaborationIssueAccess, run db.CollaborationRun, saved collaborationStoredHandoff) (protocol.CollaborationHandoffEnvelope, error) {
	out := saved.Envelope
	out.Sources = append([]protocol.CollaborationHandoffSource{}, saved.Envelope.Sources...)
	if out.Memory.Availability == "available" {
		version, _ := strconv.ParseInt(out.Memory.Version, 10, 64)
		m, err := q.GetCollaborationMemoryVersion(ctx, db.GetCollaborationMemoryVersionParams{IssueID: a.Issue.ID, Version: version})
		if err != nil {
			return out, err
		}
		if m.PurgedAt.Valid || protocol.CollaborationHandoffHash(m.Content) != out.Memory.SHA256 {
			return out, collaborationError(http.StatusConflict, "handoff_source_unavailable")
		}
	}
	for i, source := range out.Sources {
		body, err := readHandoffSource(ctx, q, a, run, source)
		if err != nil {
			return out, err
		}
		limit := 4096
		if source.Kind == "delegation" {
			limit = 64 << 10
		}
		restored := handoffSource(source.Kind, source.ID, source.RunID, source.Version, body, limit)
		if restored.SHA256 != source.SHA256 {
			return out, collaborationError(http.StatusConflict, "stale_handoff_source")
		}
		out.Sources[i] = restored
	}
	return out, boundCollaborationHandoff(&out)
}
func readHandoffSource(ctx context.Context, q *db.Queries, a CollaborationIssueAccess, run db.CollaborationRun, s protocol.CollaborationHandoffSource) (string, error) {
	id, err := util.ParseUUID(s.ID)
	if err != nil {
		return "", collaborationError(http.StatusBadRequest, "invalid_handoff_source")
	}
	if s.Kind == "handoff" {
		if s.RunID != "" || s.Version != "" {
			return "", collaborationError(http.StatusBadRequest, "invalid_handoff_source")
		}
		h, err := q.GetCollaborationHandoffByID(ctx, db.GetCollaborationHandoffByIDParams{IssueID: a.Issue.ID, ID: id})
		return h.Note, err
	}
	e, err := q.GetCollaborationHandoffSourceEvent(ctx, db.GetCollaborationHandoffSourceEventParams{IssueID: a.Issue.ID, ID: id})
	if err != nil {
		return "", err
	}
	if handoffID(e.RunID) != s.RunID || handoffDecimal(e.Seq) != s.Version {
		return "", collaborationError(http.StatusConflict, "stale_handoff_source")
	}
	if s.Kind == "delegation" {
		d, err := q.GetCollaborationHandoffDelegation(ctx, db.GetCollaborationHandoffDelegationParams{GroupID: run.GroupID, RunID: run.ID})
		if err != nil {
			return "", err
		}
		body, source, err := immutableHandoffDelegation(ctx, q, a.Issue.ID, d)
		if err != nil {
			return "", err
		}
		if source.ID != e.ID {
			return "", collaborationError(http.StatusNotFound, "handoff_source_not_found")
		}
		return body, nil
	}
	body, kind, err := handoffResultText(e)
	if err != nil {
		return "", err
	}
	if kind != s.Kind {
		return "", collaborationError(http.StatusBadRequest, "invalid_handoff_source")
	}
	return body, nil
}
func (s *CollaborationService) BindRuntimeHandoff(ctx context.Context, runID pgtype.UUID, token string, p protocol.CollaborationHandoffBinding) (protocol.CollaborationHandoffBindingReceipt, error) {
	empty := protocol.CollaborationHandoffBindingReceipt{}
	if _, err := util.ParseUUID(p.SnapshotID); err != nil || !protocol.HandoffDigest(p.PromptSHA256) || !protocol.HandoffDecimal(p.AttemptSeq, false) {
		return empty, collaborationError(http.StatusBadRequest, "invalid_handoff_binding")
	}
	if _, err := util.ParseUUID(p.AttemptNonce); err != nil {
		return empty, collaborationError(http.StatusBadRequest, "invalid_handoff_binding")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationHandoffBindingReceipt, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return empty, err
		}
		seq, _ := strconv.ParseInt(p.AttemptSeq, 10, 64)
		if seq != r.run.AuthorizedAttemptSeq {
			return empty, collaborationError(http.StatusConflict, "stale_execution")
		}
		attempt, err := r.q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: runID, AttemptSeq: seq})
		if err != nil {
			return empty, err
		}
		if attempt.AttemptNonce != p.AttemptNonce {
			return empty, collaborationError(http.StatusConflict, "stale_execution")
		}
		saved, err := loadStoredHandoff(ctx, r.q, runID, p.SnapshotID)
		if err != nil {
			return empty, err
		}
		if saved.Identity != collaborationRunIdentity(r.run, r.access.Issue.ID) {
			return empty, collaborationError(http.StatusConflict, "stale_execution")
		}
		if _, err = restoreStoredHandoff(ctx, r.q, r.access, r.run, saved); err != nil {
			return empty, err
		}
		key := "handoff_bind:" + p.AttemptNonce
		prior, err := r.q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: runID, ProducerEventID: key})
		if err == nil {
			var b protocol.CollaborationHandoffBinding
			if prior.Kind != "handoff_binding" || json.Unmarshal(prior.Payload, &b) != nil || b != p {
				return empty, collaborationError(http.StatusConflict, "handoff_binding_conflict")
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return empty, err
		} else {
			payload, err := json.Marshal(p)
			if err != nil {
				return empty, err
			}
			if _, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: runID, EventID: key, Kind: "handoff_binding", Payload: payload}); err != nil {
				return empty, err
			}
		}
		return protocol.CollaborationHandoffBindingReceipt{ProtocolVersion: 1, IssueID: handoffID(r.access.Issue.ID), RunID: handoffID(runID), SnapshotID: p.SnapshotID, PromptSHA256: p.PromptSHA256, AttemptSeq: p.AttemptSeq, AttemptNonce: p.AttemptNonce, Accepted: true}, nil
	})
}

func (s *CollaborationService) RuntimeHandoffSource(ctx context.Context, runID pgtype.UUID, token string, source protocol.CollaborationHandoffSource, offset int) (protocol.CollaborationHandoffSourcePage, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationHandoffSourcePage, error) {
		out := protocol.CollaborationHandoffSourcePage{}
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return out, err
		}
		body, err := readHandoffSource(ctx, r.q, r.access, r.run, source)
		if errors.Is(err, pgx.ErrNoRows) {
			// A well-formed ID that names nothing this run may read is a wrong
			// reference; reporting it as an outage would invite retries.
			return out, collaborationError(http.StatusNotFound, "handoff_source_not_found")
		}
		if err != nil {
			return out, err
		}
		if !protocol.HandoffDigest(source.SHA256) || protocol.CollaborationHandoffHash(body) != source.SHA256 {
			return out, collaborationError(http.StatusConflict, "stale_handoff_source")
		}
		body = redact.Text(body)
		if offset < 0 || offset > len(body) || !utf8.ValidString(body[offset:]) {
			return out, collaborationError(http.StatusBadRequest, "invalid_handoff_offset")
		}
		end := min(len(body), offset+(32<<10))
		for !utf8.ValidString(body[offset:end]) {
			end--
		}
		source.Content = body[offset:end]
		source.Truncated = end < len(body)
		source.Availability = "available"
		source.Trust = "data_only"
		if source.Kind == "delegation" {
			source.Trust = "admitted_assignment"
		}
		out = protocol.CollaborationHandoffSourcePage{ProtocolVersion: 1, IssueID: handoffID(r.access.Issue.ID), RunID: handoffID(runID), Source: source, Offset: offset, TotalBytes: len(body)}
		if end < len(body) {
			out.NextOffset = &end
		}
		return out, nil
	})
}
