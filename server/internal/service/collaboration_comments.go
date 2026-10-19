package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Content submission does not inherit changes to the execution read protocol.
const CollaborationContentProtocolVersion = 2

type CollaborationCommentInput struct {
	Content             string
	ParentID            pgtype.UUID
	SuppressedTargetIDs []pgtype.UUID
	DraftID             *pgtype.UUID  `json:",omitempty"`
	AttachmentIDs       []pgtype.UUID `json:",omitempty"`
}

type CollaborationCommentTargetVersion struct {
	TargetID              pgtype.UUID
	ExpectedTargetEpoch   int64
	ExpectedConfigVersion int64
}

type CollaborationCreateCommentParams struct {
	CollaborationQueueRequest
	CollaborationCommentInput
	TargetVersions []CollaborationCommentTargetVersion
}

type CollaborationEditCommentParams struct {
	IssueID                 pgtype.UUID
	CommentID               pgtype.UUID
	RequestID               string
	Content                 string
	ExpectedCommentRevision int64
	DraftID                 *pgtype.UUID   `json:",omitempty"`
	AttachmentIDs           *[]pgtype.UUID `json:",omitempty"`
}

type CollaborationCommentTargetResult struct {
	TargetID        pgtype.UUID                     `json:"target_id"`
	Kind            string                          `json:"kind"`
	ResourceID      pgtype.UUID                     `json:"resource_id"`
	TargetEpoch     int64                           `json:"target_epoch,string,omitempty"`
	ConfigVersion   int64                           `json:"config_version,string,omitempty"`
	HoldReason      *collaboration.TargetHoldReason `json:"hold_reason"`
	HoldVersion     *int64                          `json:"hold_version,string"`
	State           string                          `json:"state"`
	Code            string                          `json:"code"`
	WaitingReason   string                          `json:"waiting_reason,omitempty"`
	CommandID       pgtype.UUID                     `json:"command_id"`
	CommandSequence int64                           `json:"command_sequence,string,omitempty"`
	ChannelID       pgtype.UUID                     `json:"channel_id"`
	BatchID         pgtype.UUID                     `json:"batch_id"`
	GroupID         pgtype.UUID                     `json:"group_id"`
}

// A saved receipt contains identity and outcomes, never a replay of old content.
type CollaborationCommentReceipt struct {
	RequestID       string                             `json:"request_id,omitempty"`
	Action          string                             `json:"action,omitempty"`
	Emoji           json.RawMessage                    `json:"emoji,omitempty"`
	Status          int                                `json:"status"`
	Code            string                             `json:"code"`
	ProtocolVersion int                                `json:"protocol_version"`
	IssueID         pgtype.UUID                        `json:"issue_id"`
	CommentID       pgtype.UUID                        `json:"comment_id"`
	CommentRevision int64                              `json:"comment_revision,string,omitempty"`
	EventID         pgtype.UUID                        `json:"event_id"`
	EventSeq        int64                              `json:"event_seq,string,omitempty"`
	Targets         []CollaborationCommentTargetResult `json:"targets"`
	AttachmentIDs   []pgtype.UUID                      `json:"attachment_ids,omitempty"`
}

type collaborationContentTx struct {
	*collaborationQueueTx
	tx        pgx.Tx
	operation db.CollaborationOperation
}

func (t *collaborationContentTx) result(code string, status int) CollaborationCommentReceipt {
	return CollaborationCommentReceipt{Status: status, Code: code, ProtocolVersion: CollaborationContentProtocolVersion,
		IssueID: t.access.Issue.ID, Targets: make([]CollaborationCommentTargetResult, 0)}
}

func (s *CollaborationService) readCollaborationContent(ctx context.Context, issueID pgtype.UUID, read func(*db.Queries, CollaborationIssueAccess) error) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return err
	}
	q := s.Queries.WithTx(tx)
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return err
	}
	if err = read(q, access); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Content has a broader ACL than the execution queue: a source member may
// comment without participation. Execution intent is evaluated separately.
func (s *CollaborationService) mutateCollaborationContent(ctx context.Context, issueID pgtype.UUID, requestID, kind string, payload any,
	mutate func(*collaborationContentTx) (CollaborationCommentReceipt, error)) (CollaborationCommentReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || requestID == "" || len(requestID) > 128 {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_comment_request")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	digest := sha256.Sum256(encoded)
	hash := hex.EncodeToString(digest[:])
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.TxStarter.Begin(ctx)
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		receipt, err := s.collaborationContentInTx(ctx, tx, actor, issueID, requestID, kind, hash, mutate)
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				rollbackCollaborationQueue(ctx, tx)
				return CollaborationCommentReceipt{}, fmt.Errorf("commit collaboration content: %w", err)
			}
			if receipt.Status >= 400 {
				return receipt, collaborationError(receipt.Status, receipt.Code)
			}
			return receipt, nil
		}
		rollbackCollaborationQueue(ctx, tx)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return CollaborationCommentReceipt{}, err
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(25+attempt*50+rand.IntN(26)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return CollaborationCommentReceipt{}, ctx.Err()
			}
		}
	}
	return CollaborationCommentReceipt{}, collaborationError(http.StatusServiceUnavailable, "collaboration_busy")
}

func (s *CollaborationService) collaborationContentInTx(ctx context.Context, tx pgx.Tx, actor, issueID pgtype.UUID, requestID, kind, hash string,
	mutate func(*collaborationContentTx) (CollaborationCommentReceipt, error)) (CollaborationCommentReceipt, error) {
	t := &collaborationContentTx{collaborationQueueTx: &collaborationQueueTx{q: s.Queries.WithTx(tx), actor: actor}, tx: tx}
	hint, err := s.ResolveIssueAccess(ctx, t.q, issueID)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	idsParam := db.GetCollaborationWorkflowUserIDsParams{IssueID: issueID, ActorID: actor}
	ids, err := t.q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	users, err := t.q.LockCollaborationUsers(ctx, ids)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	var actorRow db.User
	for _, user := range users {
		if user.ID == actor {
			actorRow = user
		}
	}
	if err = authorizeWorkflowAccount(ctx, t.q, actor, actorRow); err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if kind == "create_comment" {
		t.gate, err = t.q.LockCollaborationGateShared(ctx)
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
	}
	if _, err = t.q.LockWorkspaceForChatSessionCreate(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if _, err = t.q.LockCollaborationSourceMember(ctx, db.LockCollaborationSourceMemberParams{WorkspaceID: hint.Issue.WorkspaceID, UserID: actor}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationCommentReceipt{}, err
	}
	if err = t.q.LockIssueStatusCatalogShared(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationCommentReceipt{}, err
	}
	issue, err := t.q.LockCollaborationIssue(ctx, issueID)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if issue.WorkspaceID != hint.Issue.WorkspaceID {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusConflict, "stale_source")
	}
	if _, err = t.q.LockIssueCollaboration(ctx, issueID); err != nil {
		return CollaborationCommentReceipt{}, err
	}
	t.session, err = t.q.LockCollaborationSession(ctx, issueID)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	currentIDs, err := t.q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if !sameCollaborationUserIDs(ids, currentIDs) {
		return CollaborationCommentReceipt{}, &pgconn.PgError{Code: "40001", Message: "collaboration content account lock set changed"}
	}
	t.channels, err = t.q.LockCollaborationChannels(ctx, t.session.ID)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	t.targets, err = t.q.LockCollaborationTargets(ctx, t.session.ID)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	t.access, err = s.ResolveIssueAccess(ctx, t.q, issueID)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	if !t.access.Capabilities.Comment {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	t.participant, err = t.q.LockCollaborationParticipant(ctx, db.LockCollaborationParticipantParams{IssueID: issueID, UserID: actor})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationCommentReceipt{}, err
	}
	t.account, err = t.q.GetCollaborationAccountFence(ctx, actor)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationCommentReceipt{}, err
	}
	previous, err := t.q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID})
	if err == nil {
		if previous.Kind != kind || previous.PayloadHash != hash {
			return CollaborationCommentReceipt{}, collaborationError(http.StatusConflict, "idempotency_conflict")
		}
		if !previous.CompletedAt.Valid {
			return CollaborationCommentReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
		}
		var receipt CollaborationCommentReceipt
		if err = json.Unmarshal(previous.Result, &receipt); err != nil || receipt.Code == "" || receipt.Code != previous.ResultCode.String ||
			receipt.IssueID != issueID || receipt.Status < 200 || receipt.Status > 599 || receipt.ProtocolVersion != CollaborationContentProtocolVersion {
			return CollaborationCommentReceipt{}, errors.New("invalid collaboration content receipt")
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationCommentReceipt{}, err
	}
	t.operation, err = t.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID, Kind: kind, PayloadHash: hash})
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	receipt, err := mutate(t)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return CollaborationCommentReceipt{}, err
	}
	_, err = t.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{
		ID: t.operation.ID, ResultCode: receipt.Code, Result: encoded, ResolvedIssueID: issueID})
	return receipt, err
}
