package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Shared context ("memory") byte limits. The deployment limit comes from
// MULTICA_COLLABORATION_MEMORY_MAX_BYTES; the hard ceiling matches the
// collaboration_memory_version content CHECK.
const (
	CollaborationMemoryDefaultMaxBytes = 128 << 10
	CollaborationMemoryMinMaxBytes     = 32 << 10
	CollaborationMemoryHardMaxBytes    = 512 << 10
)

// CollaborationMemoryChangedEvent is an outbox kind that publishes only the
// issue-scoped invalidation. It never reaches activity, workbench or notifications.
const CollaborationMemoryChangedEvent = "memory_changed"

// Memory receipts have their own contract version; they are not comment receipts.
const CollaborationMemoryProtocolVersion = 1

// CollaborationMemoryReadLimit bounds a write request body: JSON escaping can
// expand each content byte to at most six bytes, plus a fixed envelope.
func CollaborationMemoryReadLimit(maxBytes int) int64 {
	return 6*int64(maxBytes) + 64<<10
}

// EffectiveCollaborationMemoryMaxBytes maps an unset value to the default.
// Startup rejects out-of-range configuration; this only covers zero values.
func EffectiveCollaborationMemoryMaxBytes(configured int) int {
	if configured >= CollaborationMemoryMinMaxBytes && configured <= CollaborationMemoryHardMaxBytes {
		return configured
	}
	return CollaborationMemoryDefaultMaxBytes
}

// MemoryLimit is the shared-context byte limit every read reports as max_bytes.
func (s *CollaborationService) MemoryLimit() int {
	return EffectiveCollaborationMemoryMaxBytes(s.MemoryMaxBytes)
}

// collaborationMemoryWrite is the author-neutral input of one appended version.
// Human writes set AuthorUserID; runtime writes set the agent/run/owner fields.
type collaborationMemoryWrite struct {
	IssueID             pgtype.UUID
	ExpectedVersion     int64
	Content             string
	MaxBytes            int
	AuthorKind          string
	AuthorUserID        pgtype.UUID
	AgentID             pgtype.UUID
	RunID               pgtype.UUID
	ResourceOwnerUserID pgtype.UUID
	RequestID           pgtype.Text
	RequestHash         pgtype.Text
	RestoredFromVersion pgtype.Int8
	// EventActor is the human recorded on the outbox event; NULL for AI writes.
	EventActor pgtype.UUID
	Action     string
}

type collaborationMemoryWriteResult struct {
	Code           string
	Status         int
	Version        db.CollaborationMemoryVersion
	CurrentVersion int64
	Bytes          int
	EventSeq       int64
}

// writeCollaborationMemoryVersion validates and appends one version inside the
// caller's transaction. The caller must already hold its authority locks and
// have checked request idempotency: a replay never reaches this version check.
func writeCollaborationMemoryVersion(ctx context.Context, q *db.Queries, w collaborationMemoryWrite) (collaborationMemoryWriteResult, error) {
	out := collaborationMemoryWriteResult{Bytes: len(w.Content)}
	// Every refusal reports the current version it was decided against.
	current, err := q.GetCollaborationMemoryMaxVersion(ctx, w.IssueID)
	if err != nil {
		return out, err
	}
	out.CurrentVersion = current
	// Reject rather than sanitize: a replayed request must hash identically.
	if !utf8.ValidString(w.Content) || strings.ContainsRune(w.Content, 0) {
		out.Code, out.Status = "invalid_memory_content", http.StatusBadRequest
		return out, nil
	}
	maxBytes := w.MaxBytes
	if maxBytes < 1 || maxBytes > CollaborationMemoryHardMaxBytes {
		maxBytes = CollaborationMemoryHardMaxBytes
	}
	if len(w.Content) > maxBytes {
		out.Code, out.Status = "memory_too_large", http.StatusRequestEntityTooLarge
		return out, nil
	}
	if current != w.ExpectedVersion {
		out.Code, out.Status = "stale_memory_version", http.StatusConflict
		return out, nil
	}
	row, err := q.CreateCollaborationMemoryVersion(ctx, db.CreateCollaborationMemoryVersionParams{
		IssueID: w.IssueID, Version: w.ExpectedVersion + 1, Content: w.Content, Bytes: int32(len(w.Content)),
		AuthorKind: w.AuthorKind, AuthorUserID: w.AuthorUserID, AgentID: w.AgentID, RunID: w.RunID,
		ResourceOwnerUserID: w.ResourceOwnerUserID, RequestID: w.RequestID, RequestHash: w.RequestHash, RestoredFromVersion: w.RestoredFromVersion,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent writer won the unique (issue, version) slot.
		if out.CurrentVersion, err = q.GetCollaborationMemoryMaxVersion(ctx, w.IssueID); err != nil {
			return out, err
		}
		out.Code, out.Status = "stale_memory_version", http.StatusConflict
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Version, out.CurrentVersion = row, row.Version
	if out.EventSeq, err = recordCollaborationMemoryChange(ctx, q, w.IssueID, w.EventActor, row.Version, w.Action); err != nil {
		return out, err
	}
	out.Code, out.Status = "memory_updated", http.StatusOK
	if w.Action == "restore" {
		out.Code = "memory_restored"
	}
	return out, nil
}

type collaborationMemoryPurge struct {
	IssueID pgtype.UUID
	Version int64
	Actor   pgtype.UUID
	Admin   bool
	Reason  string
}

// purgeCollaborationMemoryVersion clears one historical version's text inside
// the caller's transaction. Byte count and authorship remain as history.
func purgeCollaborationMemoryVersion(ctx context.Context, q *db.Queries, p collaborationMemoryPurge) (collaborationMemoryWriteResult, error) {
	var out collaborationMemoryWriteResult
	current, err := q.GetCollaborationMemoryMaxVersion(ctx, p.IssueID)
	if err != nil {
		return out, err
	}
	out.CurrentVersion = current
	row, err := q.LockCollaborationMemoryVersion(ctx, db.LockCollaborationMemoryVersionParams{IssueID: p.IssueID, Version: p.Version})
	if errors.Is(err, pgx.ErrNoRows) {
		out.Code, out.Status = "memory_version_not_found", http.StatusNotFound
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Bytes = int(row.Bytes)
	// The current version only grows, so this refusal stays correct under
	// concurrency: a later write can only make the target historical.
	if row.Version == current {
		out.Code, out.Status = "memory_version_current", http.StatusConflict
		return out, nil
	}
	if row.PurgedAt.Valid {
		out.Code, out.Status = "memory_version_purged", http.StatusConflict
		return out, nil
	}
	if out.Version, err = q.PurgeCollaborationMemoryVersion(ctx, db.PurgeCollaborationMemoryVersionParams{
		IssueID: p.IssueID, Version: p.Version, PurgedByUserID: p.Actor, PurgedByAdmin: p.Admin, PurgeReason: p.Reason}); err != nil {
		return out, err
	}
	if out.EventSeq, err = recordCollaborationMemoryChange(ctx, q, p.IssueID, p.Actor, p.Version, "purge"); err != nil {
		return out, err
	}
	out.Code, out.Status = "memory_purged", http.StatusOK
	return out, nil
}

// The payload is content-free: no text, author or request identity.
func recordCollaborationMemoryChange(ctx context.Context, q *db.Queries, issue, actor pgtype.UUID, version int64, action string) (int64, error) {
	seq, err := q.AdvanceCollaborationEventSequence(ctx, issue)
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(struct {
		Version int64  `json:"version,string"`
		Action  string `json:"action"`
	}{version, action})
	if err != nil {
		return 0, err
	}
	_, err = q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: issue, EventSeq: seq, ActorUserID: actor,
		Kind: CollaborationMemoryChangedEvent, Payload: payload})
	return seq, err
}

func validCollaborationMemoryReason(reason string) bool {
	return reason != "" && len(reason) <= 1000 && utf8.ValidString(reason) && !strings.ContainsRune(reason, 0)
}

// CollaborationMemoryReceipt is the original-key result. It never repeats text.
type CollaborationMemoryReceipt struct {
	RequestID       string      `json:"request_id"`
	Action          string      `json:"action"`
	Status          int         `json:"status"`
	Code            string      `json:"code"`
	ProtocolVersion int         `json:"protocol_version"`
	IssueID         pgtype.UUID `json:"issue_id"`
	Version         *int64      `json:"version,string"`
	CurrentVersion  int64       `json:"current_version,string"`
	Bytes           *int        `json:"bytes,omitempty"`
	MaxBytes        int         `json:"max_bytes"`
	EventSeq        *int64      `json:"event_seq,string,omitempty"`
}

type CollaborationMemoryUpdateParams struct {
	IssueID         pgtype.UUID
	RequestID       string
	ExpectedVersion int64
	Content         string
}

type CollaborationMemoryRestoreParams struct {
	IssueID         pgtype.UUID
	RequestID       string
	Version         int64
	ExpectedVersion int64
}

type CollaborationMemoryPurgeParams struct {
	IssueID   pgtype.UUID
	RequestID string
	Version   int64
	Reason    string
}

type collaborationMemoryTx struct {
	q         *db.Queries
	actor     pgtype.UUID
	access    CollaborationIssueAccess
	session   db.CollaborationSession
	operation db.CollaborationOperation
	maxBytes  int
}

func (t *collaborationMemoryTx) receipt(code string, status int) CollaborationMemoryReceipt {
	return CollaborationMemoryReceipt{RequestID: t.operation.RequestID, Action: t.operation.Kind, Status: status, Code: code,
		ProtocolVersion: CollaborationMemoryProtocolVersion, IssueID: t.access.Issue.ID, MaxBytes: t.maxBytes}
}

func (t *collaborationMemoryTx) fromResult(r collaborationMemoryWriteResult) CollaborationMemoryReceipt {
	receipt := t.receipt(r.Code, r.Status)
	receipt.CurrentVersion = r.CurrentVersion
	if r.Version.ID.Valid {
		version := r.Version.Version
		receipt.Version = &version
	}
	if r.Code == "memory_too_large" || r.Status < 400 && r.Version.ID.Valid {
		bytes := r.Bytes
		receipt.Bytes = &bytes
	}
	if r.EventSeq > 0 {
		seq := r.EventSeq
		receipt.EventSeq = &seq
	}
	return receipt
}

// closed reports the edit/restore refusal for an issue that is not open,
// with the current version it was decided against.
func (t *collaborationMemoryTx) closed(ctx context.Context) (CollaborationMemoryReceipt, bool, error) {
	code := ""
	if collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" {
		code = "issue_closed"
	} else if !collaboration.NonterminalCategory(t.access.StatusCategory) {
		code = "status_unavailable"
	}
	if code == "" {
		return CollaborationMemoryReceipt{}, false, nil
	}
	receipt := t.receipt(code, http.StatusConflict)
	var err error
	receipt.CurrentVersion, err = t.q.GetCollaborationMemoryMaxVersion(ctx, t.access.Issue.ID)
	return receipt, true, err
}

func collaborationMemoryInitiator(access CollaborationIssueAccess, viewer pgtype.UUID) bool {
	return access.Capabilities.ControlAI && access.Issue.CreatorType == "member" && access.Issue.CreatorID == viewer
}

func collaborationMemoryEditable(access CollaborationIssueAccess) bool {
	return access.Capabilities.ControlAI && collaboration.NonterminalCategory(access.StatusCategory) && access.QueueState != "closed"
}

func (s *CollaborationService) UpdateMemory(ctx context.Context, p CollaborationMemoryUpdateParams) (CollaborationMemoryReceipt, error) {
	if p.ExpectedVersion < 0 {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_memory_request")
	}
	return s.mutateCollaborationMemory(ctx, p.IssueID, p.RequestID, "memory_update", p, func(t *collaborationMemoryTx) (CollaborationMemoryReceipt, error) {
		if refusal, closed, err := t.closed(ctx); closed || err != nil {
			return refusal, err
		}
		result, err := writeCollaborationMemoryVersion(ctx, t.q, collaborationMemoryWrite{IssueID: t.access.Issue.ID,
			ExpectedVersion: p.ExpectedVersion, Content: p.Content, MaxBytes: t.maxBytes, AuthorKind: "member",
			AuthorUserID: t.actor, EventActor: t.actor, Action: "update"})
		if err != nil {
			return CollaborationMemoryReceipt{}, err
		}
		return t.fromResult(result), nil
	})
}

func (s *CollaborationService) RestoreMemoryVersion(ctx context.Context, p CollaborationMemoryRestoreParams) (CollaborationMemoryReceipt, error) {
	if p.Version < 1 || p.ExpectedVersion < 0 {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_memory_request")
	}
	return s.mutateCollaborationMemory(ctx, p.IssueID, p.RequestID, "memory_restore", p, func(t *collaborationMemoryTx) (CollaborationMemoryReceipt, error) {
		if refusal, closed, err := t.closed(ctx); closed || err != nil {
			return refusal, err
		}
		source, err := t.q.GetCollaborationMemoryVersion(ctx, db.GetCollaborationMemoryVersionParams{IssueID: t.access.Issue.ID, Version: p.Version})
		if errors.Is(err, pgx.ErrNoRows) {
			receipt := t.receipt("memory_version_not_found", http.StatusNotFound)
			receipt.CurrentVersion, err = t.q.GetCollaborationMemoryMaxVersion(ctx, t.access.Issue.ID)
			return receipt, err
		}
		if err != nil {
			return CollaborationMemoryReceipt{}, err
		}
		if source.PurgedAt.Valid {
			receipt := t.receipt("memory_version_purged", http.StatusConflict)
			receipt.CurrentVersion, err = t.q.GetCollaborationMemoryMaxVersion(ctx, t.access.Issue.ID)
			return receipt, err
		}
		result, err := writeCollaborationMemoryVersion(ctx, t.q, collaborationMemoryWrite{IssueID: t.access.Issue.ID,
			ExpectedVersion: p.ExpectedVersion, Content: source.Content, MaxBytes: t.maxBytes, AuthorKind: "member",
			AuthorUserID: t.actor, RestoredFromVersion: pgtype.Int8{Int64: source.Version, Valid: true}, EventActor: t.actor, Action: "restore"})
		if err != nil {
			return CollaborationMemoryReceipt{}, err
		}
		return t.fromResult(result), nil
	})
}

// PurgeMemoryVersion is an initiator-only credential disposal. It stays
// available after completion or cancellation; only content edits are closed.
func (s *CollaborationService) PurgeMemoryVersion(ctx context.Context, p CollaborationMemoryPurgeParams) (CollaborationMemoryReceipt, error) {
	p.Reason = strings.TrimSpace(p.Reason)
	if p.Version < 1 {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_memory_request")
	}
	if !validCollaborationMemoryReason(p.Reason) {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_reason")
	}
	return s.mutateCollaborationMemory(ctx, p.IssueID, p.RequestID, "memory_purge", p, func(t *collaborationMemoryTx) (CollaborationMemoryReceipt, error) {
		result, err := purgeCollaborationMemoryVersion(ctx, t.q, collaborationMemoryPurge{IssueID: t.access.Issue.ID,
			Version: p.Version, Actor: t.actor, Reason: p.Reason})
		if err != nil {
			return CollaborationMemoryReceipt{}, err
		}
		return t.fromResult(result), nil
	})
}
