package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

// Shared-context support is declared per claim, never registered as a runtime
// capability: a daemon upgrade must not advance the configuration epoch or
// revoke owner consent. Only a host claim may declare it.
func collaborationClaimMemoryMode(p protocol.CollaborationRunClaimRequest) (string, error) {
	host := p.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion
	switch {
	case p.MemoryVersion == 0:
		return "unsupported", nil
	case host && p.MemoryVersion == protocol.CollaborationMemoryVersion:
		return "supported", nil
	default:
		return "", collaborationError(http.StatusBadRequest, "unsupported_runtime_protocol")
	}
}

// recordClaimMemoryMode runs in the first-claim transaction, beside the frozen
// prompt. A run frozen before shared context existed keeps its NULL mode.
func recordClaimMemoryMode(ctx context.Context, r *collaborationRuntimeState, mode string) error {
	updated, err := r.q.SetCollaborationRunMemoryMode(ctx, db.SetCollaborationRunMemoryModeParams{RunID: r.run.ID, MemoryMode: mode})
	if err != nil {
		return err
	}
	if updated != 1 {
		return invalidCollaborationLedger("run shared-context mode already recorded")
	}
	r.run.MemoryMode = pgtype.Text{String: mode, Valid: true}
	return nil
}

// lockRuntimeMemory applies the runtime comment gate, then requires that this
// run's claim declared shared-context support.
func (s *CollaborationService) lockRuntimeMemory(ctx context.Context, tx pgx.Tx, runID pgtype.UUID, token string) (*collaborationRuntimeState, error) {
	r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
	if err != nil {
		return nil, err
	}
	if r.run.MemoryMode.String != "supported" {
		return nil, collaborationError(http.StatusConflict, "memory_unsupported")
	}
	return r, nil
}

func (s *CollaborationService) runtimeMemoryView(r *collaborationRuntimeState, row *db.CollaborationMemoryVersion) protocol.CollaborationRunMemory {
	out := protocol.CollaborationRunMemory{IssueID: util.UUIDToString(r.access.Issue.ID), RunID: util.UUIDToString(r.run.ID), Version: "0", MaxBytes: s.MemoryLimit()}
	if row != nil {
		out.Version, out.Content, out.Bytes = strconv.FormatInt(row.Version, 10), row.Content, int(row.Bytes)
	}
	return out
}

func currentCollaborationMemory(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (*db.CollaborationMemoryVersion, error) {
	row, err := q.GetCollaborationMemoryCurrent(ctx, issueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RuntimeMemoryBase returns the document a run starts from. The first call
// records the current version; every later call, including a fresh-session
// retry, returns that same version. A purged base is replaced by the current
// version, which purge can never target.
func (s *CollaborationService) RuntimeMemoryBase(ctx context.Context, runID pgtype.UUID, token string) (protocol.CollaborationRunMemory, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunMemory, error) {
		r, err := s.lockRuntimeMemory(ctx, tx, runID, token)
		if err != nil {
			return protocol.CollaborationRunMemory{}, err
		}
		if r.run.MemoryBaseVersion.Valid {
			if r.run.MemoryBaseVersion.Int64 == 0 {
				return s.runtimeMemoryView(r, nil), nil
			}
			row, err := r.q.GetCollaborationMemoryVersion(ctx, db.GetCollaborationMemoryVersionParams{IssueID: r.access.Issue.ID, Version: r.run.MemoryBaseVersion.Int64})
			if errors.Is(err, pgx.ErrNoRows) {
				return protocol.CollaborationRunMemory{}, invalidCollaborationLedger("shared-context base version missing")
			}
			if err != nil {
				return protocol.CollaborationRunMemory{}, err
			}
			if !row.PurgedAt.Valid {
				return s.runtimeMemoryView(r, &row), nil
			}
		}
		current, err := currentCollaborationMemory(ctx, r.q, r.access.Issue.ID)
		if err != nil {
			return protocol.CollaborationRunMemory{}, err
		}
		if current != nil && current.PurgedAt.Valid {
			return protocol.CollaborationRunMemory{}, invalidCollaborationLedger("current shared-context version purged")
		}
		base := int64(0)
		if current != nil {
			base = current.Version
		}
		updated, err := r.q.SetCollaborationRunMemoryBase(ctx, db.SetCollaborationRunMemoryBaseParams{RunID: runID, Version: base})
		if err != nil {
			return protocol.CollaborationRunMemory{}, err
		}
		if updated != 1 {
			return protocol.CollaborationRunMemory{}, invalidCollaborationLedger("run shared-context base unavailable")
		}
		return s.runtimeMemoryView(r, current), nil
	})
}

// RuntimeMemory returns the latest version so parallel channels can see each
// other's writes during a run.
func (s *CollaborationService) RuntimeMemory(ctx context.Context, runID pgtype.UUID, token string) (protocol.CollaborationRunMemory, error) {
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationRunMemory, error) {
		r, err := s.lockRuntimeMemory(ctx, tx, runID, token)
		if err != nil {
			return protocol.CollaborationRunMemory{}, err
		}
		current, err := currentCollaborationMemory(ctx, r.q, r.access.Issue.ID)
		if err != nil {
			return protocol.CollaborationRunMemory{}, err
		}
		return s.runtimeMemoryView(r, current), nil
	})
}

type CollaborationRuntimeMemoryParams struct {
	RequestID   pgtype.UUID
	BaseVersion int64
	Content     string
}

// Keep the replay identity after a historical version's text is purged. The
// decimal base and NUL separator unambiguously frame the redacted UTF-8 text.
func collaborationRuntimeMemoryHash(baseVersion int64, content string) string {
	digest := sha256.Sum256([]byte(strconv.FormatInt(baseVersion, 10) + "\x00" + content))
	return hex.EncodeToString(digest[:])
}

// UpdateRuntimeMemory appends one AI-authored version. The content is redacted
// before the byte limit, the replay comparison and storage, so a secret never
// reaches the version history. A refusal writes nothing; its receipt carries
// the current version it was decided against.
func (s *CollaborationService) UpdateRuntimeMemory(ctx context.Context, runID pgtype.UUID, token string, p CollaborationRuntimeMemoryParams) (protocol.CollaborationRunMemoryReceipt, int, error) {
	if !p.RequestID.Valid || p.BaseVersion < 0 {
		return protocol.CollaborationRunMemoryReceipt{}, 0, collaborationError(http.StatusBadRequest, "invalid_memory_request")
	}
	content := redact.Text(p.Content)
	requestHash := collaborationRuntimeMemoryHash(p.BaseVersion, content)
	requestID := util.UUIDToString(p.RequestID)
	type result struct {
		receipt protocol.CollaborationRunMemoryReceipt
		status  int
	}
	out, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (result, error) {
		r, err := s.lockRuntimeMemory(ctx, tx, runID, token)
		if err != nil {
			return result{}, err
		}
		maxBytes := s.MemoryLimit()
		receipt := protocol.CollaborationRunMemoryReceipt{IssueID: util.UUIDToString(r.access.Issue.ID), RunID: util.UUIDToString(runID), RequestID: requestID, MaxBytes: maxBytes}
		// Replay precedes the version check: the original write already moved
		// the current version, so a lost response must not look stale.
		prior, err := r.q.GetCollaborationMemoryRunWrite(ctx, db.GetCollaborationMemoryRunWriteParams{RunID: runID, RequestID: requestID})
		if err == nil {
			current, err := r.q.GetCollaborationMemoryMaxVersion(ctx, r.access.Issue.ID)
			if err != nil {
				return result{}, err
			}
			receipt.CurrentVersion = strconv.FormatInt(current, 10)
			// The fingerprint survives a purge; equal lengths do not prove that
			// the original base and redacted content were resubmitted.
			same := prior.IssueID == r.access.Issue.ID && prior.Version == p.BaseVersion+1 &&
				prior.RequestHash.Valid && prior.RequestHash.String == requestHash
			if !same {
				receipt.Code, receipt.Error = "memory_write_conflict", "memory_write_conflict"
				return result{receipt, http.StatusConflict}, nil
			}
			version, bytes := strconv.FormatInt(prior.Version, 10), int(prior.Bytes)
			receipt.Code, receipt.Version, receipt.Bytes = "memory_updated", &version, &bytes
			return result{receipt, http.StatusOK}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return result{}, err
		}
		written, err := writeCollaborationMemoryVersion(ctx, r.q, collaborationMemoryWrite{IssueID: r.access.Issue.ID, ExpectedVersion: p.BaseVersion,
			Content: content, MaxBytes: maxBytes, AuthorKind: "agent", AgentID: r.run.AgentID, RunID: runID,
			ResourceOwnerUserID: r.run.ResourceOwnerUserID, RequestID: pgtype.Text{String: requestID, Valid: true},
			RequestHash: pgtype.Text{String: requestHash, Valid: true}, Action: "update"})
		if err != nil {
			return result{}, err
		}
		receipt.Code, receipt.CurrentVersion = written.Code, strconv.FormatInt(written.CurrentVersion, 10)
		if written.Status >= 400 {
			receipt.Error = written.Code
		}
		if written.Version.ID.Valid {
			version := strconv.FormatInt(written.Version.Version, 10)
			receipt.Version = &version
		}
		if written.Version.ID.Valid || written.Code == "memory_too_large" {
			bytes := written.Bytes
			receipt.Bytes = &bytes
		}
		return result{receipt, written.Status}, nil
	})
	return out.receipt, out.status, err
}

// CollaborationRunMemoryView marks one recorded run's shared-context use. It is
// absent for runs created before shared context existed.
type CollaborationRunMemoryView struct {
	Support        string  `json:"support"`
	BaseVersion    *string `json:"base_version"`
	WrittenVersion *string `json:"written_version"`
}

func collaborationRunMemoryMarker(ctx context.Context, q *db.Queries, run db.CollaborationRun) (*CollaborationRunMemoryView, error) {
	if !run.MemoryMode.Valid {
		return nil, nil
	}
	if run.MemoryMode.String != "supported" && run.MemoryMode.String != "unsupported" {
		return nil, invalidCollaborationLedger("run shared-context mode invalid")
	}
	out := &CollaborationRunMemoryView{Support: run.MemoryMode.String}
	if run.MemoryMode.String == "unsupported" {
		// The write route refuses unsupported runs, so there is nothing to look up.
		return out, nil
	}
	if run.MemoryBaseVersion.Valid {
		base := strconv.FormatInt(run.MemoryBaseVersion.Int64, 10)
		out.BaseVersion = &base
	}
	written, err := q.GetCollaborationMemoryRunWrittenVersion(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	if written > 0 {
		value := strconv.FormatInt(written, 10)
		out.WrittenVersion = &value
	}
	return out, nil
}
