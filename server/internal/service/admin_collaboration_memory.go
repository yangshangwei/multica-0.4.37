package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// R38 shared-context disposal: a platform administrator may purge one
// historical version only while the issue creator is disabled. It reuses the
// lifecycle exception's authorization, locks, both ledgers and audit, and never
// exposes version text or grants ordinary collaboration authority.
const adminCollaborationMemoryPurge = "purge_memory"

type AdminCollaborationMemoryPurgeParams struct {
	OrganizationID, IssueID, IdempotencyKey pgtype.UUID
	Version                                 int64
	Reason, RequestID                       string
}

type AdminCollaborationMemoryReceipt struct {
	IssueID        pgtype.UUID `json:"issue_id"`
	RequestID      string      `json:"request_id"`
	Action         string      `json:"action"`
	Code           string      `json:"code"`
	Version        int64       `json:"version,string"`
	CurrentVersion int64       `json:"current_version,string"`
	Bytes          int         `json:"bytes"`
	CreatorUserID  pgtype.UUID `json:"creator_user_id"`
	PurgedAt       time.Time   `json:"purged_at"`
}

type adminCollaborationMemoryStoredResult struct {
	OperationID pgtype.UUID                     `json:"operation_id"`
	Result      AdminCollaborationMemoryReceipt `json:"result"`
}

type AdminCollaborationMemoryVersionsPage struct {
	Scope          pgtype.UUID                      `json:"scope"`
	IssueID        pgtype.UUID                      `json:"issue_id"`
	CurrentVersion int64                            `json:"current_version,string"`
	Items          []CollaborationMemoryVersionView `json:"items"`
	NextCursor     *int64                           `json:"next_cursor,string"`
}

func adminCollaborationMemoryHash(p AdminCollaborationMemoryPurgeParams) (string, error) {
	payload, err := json.Marshal(struct {
		OrganizationID pgtype.UUID `json:"organization_id"`
		IssueID        pgtype.UUID `json:"issue_id"`
		Action         string      `json:"action"`
		Version        int64       `json:"version"`
		Reason         string      `json:"reason"`
	}{p.OrganizationID, p.IssueID, adminCollaborationMemoryPurge, p.Version, strings.TrimSpace(p.Reason)})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

type adminCollaborationMemoryAuditState struct {
	IssueID           pgtype.UUID `json:"issue_id"`
	SourceWorkspaceID pgtype.UUID `json:"source_workspace_id"`
	CreatorUserID     pgtype.UUID `json:"creator_user_id"`
	Version           int64       `json:"version,string"`
	CurrentVersion    int64       `json:"current_version,string"`
	Bytes             int         `json:"bytes"`
	AuthorKind        string      `json:"author_kind"`
	Purged            bool        `json:"purged"`
}

// PurgeMemoryVersion owns one transaction with both operation ledgers. The real
// administrator is the purge actor; the creator and roster are unchanged.
func (s *AdminCollaborationService) PurgeMemoryVersion(ctx context.Context, p AdminCollaborationMemoryPurgeParams) (out AdminCollaborationResult, resultErr error) {
	defer func() { resultErr = adminCollaborationServiceError(resultErr) }()
	actor, err := s.authorize(ctx, s.Queries, p.OrganizationID)
	if err != nil {
		return out, err
	}
	if !p.IssueID.Valid || !p.IdempotencyKey.Valid || p.Version < 1 {
		return out, platformError(400, "invalid_memory_purge", "An exact shared-context version is required")
	}
	if err = validateAdminReason(p.Reason, p.RequestID); err != nil {
		return out, err
	}
	hash, err := adminCollaborationMemoryHash(p)
	if err != nil {
		return out, err
	}
	if s.TxStarter == nil {
		return out, platformError(503, "admin_unavailable", "Collaboration administration is unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	t, err := s.lockLifecycle(ctx, tx, actor, AdminCollaborationLifecycleParams{OrganizationID: p.OrganizationID, IssueID: p.IssueID})
	if err != nil {
		return out, err
	}
	creator := t.users[t.access.Issue.CreatorID]
	if !adminCollaborationAccountDisabled(creator.ID, creator.Email, creator.DisabledAt) {
		return out, platformError(409, "creator_not_disabled", "Only a disabled creator permits shared-context administration")
	}
	reason := strings.TrimSpace(p.Reason)
	op, replayed, err := NewPlatformAdminService(s.Queries, s.TxStarter).CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{
		OrganizationID: p.OrganizationID, TargetKind: "collaboration_issue", TargetID: p.IssueID,
		Kind: "collaboration." + adminCollaborationMemoryPurge, IdempotencyKey: p.IdempotencyKey, PayloadHash: hash,
		Reason: reason, State: "applied", ResultCode: "memory_purged",
		AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return out, err
	}
	if replayed {
		receipt, err := loadAdminCollaborationMemoryReceipt(ctx, t.q, op)
		return AdminCollaborationResult{Operation: op, MemoryResult: &receipt, Replayed: true}, err
	}
	// Refusals roll back the reserved administration record: only an applied
	// disposal is audited, matching the lifecycle exception.
	result, err := purgeCollaborationMemoryVersion(ctx, t.q, collaborationMemoryPurge{IssueID: p.IssueID, Version: p.Version,
		Actor: actor.UserID, Admin: true, Reason: reason})
	if err != nil {
		return out, err
	}
	if result.Status >= 400 {
		return out, platformError(result.Status, result.Code, "The shared-context version cannot be purged in its current state")
	}
	t.operation, err = t.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: p.IssueID, ActorUserID: actor.UserID,
		RequestID: util.UUIDToString(p.IdempotencyKey), Kind: "admin_" + adminCollaborationMemoryPurge, PayloadHash: hash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, platformError(409, "idempotency_conflict", "The request key already identifies another collaboration action")
	}
	if err != nil {
		return out, err
	}
	receipt := AdminCollaborationMemoryReceipt{IssueID: p.IssueID, RequestID: t.operation.RequestID, Action: adminCollaborationMemoryPurge,
		Code: result.Code, Version: p.Version, CurrentVersion: result.CurrentVersion, Bytes: result.Bytes,
		CreatorUserID: t.access.Issue.CreatorID, PurgedAt: result.Version.PurgedAt.Time}
	state := adminCollaborationMemoryAuditState{IssueID: p.IssueID, SourceWorkspaceID: t.access.Issue.WorkspaceID,
		CreatorUserID: t.access.Issue.CreatorID, Version: p.Version, CurrentVersion: result.CurrentVersion,
		Bytes: result.Bytes, AuthorKind: result.Version.AuthorKind}
	before, err := json.Marshal(state)
	if err != nil {
		return out, err
	}
	state.Purged = true
	after, err := json.Marshal(state)
	if err != nil {
		return out, err
	}
	if err = recordRoleAudit(ctx, t.q, op, p.RequestID, before, after); err != nil {
		return out, err
	}
	encoded, err := json.Marshal(adminCollaborationMemoryStoredResult{OperationID: op.ID, Result: receipt})
	if err != nil {
		return out, err
	}
	if _, err = t.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{
		ID: t.operation.ID, ResultCode: receipt.Code, Result: encoded, ResolvedIssueID: p.IssueID,
	}); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		// Recover only by the original actor/org/key; never retry as a new purge.
		return out, fmt.Errorf("commit collaboration memory administration %s: %w", util.UUIDToString(op.ID), err)
	}
	return AdminCollaborationResult{Operation: op, MemoryResult: &receipt}, nil
}

func loadAdminCollaborationMemoryReceipt(ctx context.Context, q *db.Queries, op db.AdminOperation) (AdminCollaborationMemoryReceipt, error) {
	var out AdminCollaborationMemoryReceipt
	if op.TargetKind != "collaboration_issue" || op.Kind != "collaboration."+adminCollaborationMemoryPurge {
		return out, pgx.ErrNoRows
	}
	record, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: op.TargetID,
		ActorUserID: op.ActorID, RequestID: util.UUIDToString(op.IdempotencyKey)})
	if err != nil {
		return out, err
	}
	var stored adminCollaborationMemoryStoredResult
	if !record.CompletedAt.Valid || record.ResolvedIssueID != op.TargetID || record.Kind != "admin_"+adminCollaborationMemoryPurge ||
		record.PayloadHash != op.PayloadHash || record.ResultCode.String != op.ResultCode || op.ResultCode != "memory_purged" ||
		op.State != "applied" || json.Unmarshal(record.Result, &stored) != nil || stored.OperationID != op.ID {
		return out, platformError(503, "admin_operation_unavailable", "The original administration result is unavailable")
	}
	out = stored.Result
	if out.IssueID != op.TargetID || out.RequestID != util.UUIDToString(op.IdempotencyKey) || out.Action != adminCollaborationMemoryPurge ||
		out.Code != op.ResultCode || out.Version < 1 || out.CurrentVersion <= out.Version || out.Bytes < 0 || !out.CreatorUserID.Valid || out.PurgedAt.IsZero() {
		return AdminCollaborationMemoryReceipt{}, platformError(503, "admin_operation_unavailable", "The original administration result is unavailable")
	}
	return out, nil
}

// ListMemoryVersions returns history metadata only, so an administrator can
// identify the reported version without reading any shared-context text.
func (s *AdminCollaborationService) ListMemoryVersions(ctx context.Context, organization, issueID pgtype.UUID, p CollaborationMemoryVersionsQuery) (AdminCollaborationMemoryVersionsPage, error) {
	out := AdminCollaborationMemoryVersionsPage{Items: make([]CollaborationMemoryVersionView, 0)}
	err := s.read(ctx, organization, func(q *db.Queries, actor PlatformAdminIdentity) error {
		out.Scope = actor.OrganizationID
		if !issueID.Valid {
			return platformError(400, "invalid_issue_id", "A collaboration issue ID is required")
		}
		if p.Limit < 1 || p.Limit > 100 || p.BeforeVersion.Valid && p.BeforeVersion.Int64 < 1 {
			return platformError(400, "invalid_memory_query", "The shared-context page is invalid")
		}
		row, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: organization, IssueID: issueID})
		if err != nil {
			return err
		}
		if !adminCollaborationIssueView(row).Creator.Disabled {
			return platformError(409, "creator_not_disabled", "Only a disabled creator permits shared-context administration")
		}
		out.IssueID = issueID
		out.Items, out.CurrentVersion, out.NextCursor, err = listCollaborationMemoryVersions(ctx, q, row.SourceWorkspaceID, issueID, p)
		return err
	})
	if err != nil {
		return AdminCollaborationMemoryVersionsPage{}, err
	}
	return out, nil
}
