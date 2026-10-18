package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const CollaborationAttachmentMaxBytes = 100 << 20
const collaborationMaxSelectedAttachments = 32

var errCollaborationAttachmentCommit = errors.New("collaboration attachment commit outcome unknown")

// This is the only public attachment projection. Storage keys, encryption keys
// and upload digests never cross an API boundary, even for the uploader.
type CollaborationAttachmentView struct {
	ID                pgtype.UUID `json:"id"`
	DraftID           pgtype.UUID `json:"draft_id"`
	IssueID           pgtype.UUID `json:"issue_id"`
	IntendedIssueID   pgtype.UUID `json:"intended_issue_id"`
	SourceWorkspaceID pgtype.UUID `json:"source_workspace_id"`
	UploaderUserID    pgtype.UUID `json:"uploader_user_id"`
	Name              string      `json:"name"`
	ContentType       string      `json:"content_type"`
	SizeBytes         int64       `json:"size_bytes"`
	Version           int64       `json:"version,string"`
	CreatedAt         time.Time   `json:"created_at"`
	DeliveredAt       *time.Time  `json:"delivered_at"`
}

func CollaborationAttachmentMetadata(row db.CollaborationAttachment) CollaborationAttachmentView {
	return CollaborationAttachmentView{ID: row.ID, DraftID: row.DraftID, IssueID: row.IssueID,
		IntendedIssueID:   row.IntendedIssueID,
		SourceWorkspaceID: row.SourceWorkspaceID, UploaderUserID: row.UploaderUserID,
		Name: row.Name, ContentType: row.ContentType, SizeBytes: row.SizeBytes, Version: row.Version,
		CreatedAt: row.CreatedAt.Time, DeliveredAt: collaborationContentTime(row.DeliveredAt)}
}

type CollaborationAttachmentUpload struct {
	DraftID, SourceWorkspaceID, IssueID pgtype.UUID
	RequestID, Name, ContentType        string
	Data                                []byte
}

// UploadAttachmentDraft performs external storage I/O outside database locks.
// Every attempt has its own object and key. A loser only deletes its own object;
// an uncertain database commit preserves the possible winner for key recovery.
func (s *CollaborationService) UploadAttachmentDraft(ctx context.Context, p CollaborationAttachmentUpload) (CollaborationAttachmentView, error) {
	var out CollaborationAttachmentView
	actor, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return out, err
	}
	if !p.DraftID.Valid || p.SourceWorkspaceID.Valid == p.IssueID.Valid || p.RequestID == "" || len(p.RequestID) > 128 ||
		strings.TrimSpace(p.Name) == "" || len(p.Name) > 1024 || !utf8.ValidString(p.Name) || strings.ContainsRune(p.Name, 0) ||
		p.ContentType == "" || len(p.ContentType) > 256 || len(p.Data) > CollaborationAttachmentMaxBytes {
		return out, collaborationError(http.StatusBadRequest, "invalid_attachment_upload")
	}
	if s.Storage == nil {
		return out, collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	sum := sha256.Sum256(p.Data)
	digest := hex.EncodeToString(sum[:])
	lookup := db.GetCollaborationAttachmentUploadParams{UploaderUserID: actor, DraftID: p.DraftID,
		UploadRequestID: pgtype.Text{String: p.RequestID, Valid: true}}
	previous, err := s.Queries.GetCollaborationAttachmentUpload(ctx, lookup)
	if err == nil {
		if err = s.authorizeCollaborationAttachment(ctx, s.Queries, previous, actor); err != nil {
			return out, err
		}
		if err = compareCollaborationAttachmentUpload(previous, p, digest); err != nil {
			return out, err
		}
		return CollaborationAttachmentMetadata(previous), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	source, err := s.authorizeAttachmentUploadContext(ctx, s.Queries, actor, p.SourceWorkspaceID, p.IssueID, true)
	if err != nil {
		return out, err
	}
	if _, err = checkCollaborationAttachmentDraft(ctx, s.Queries, actor, p.DraftID, source, p.IssueID); err != nil {
		return out, err
	}
	keyBytes := make([]byte, secretbox.KeySize)
	if _, err = rand.Read(keyBytes); err != nil {
		return out, err
	}
	box, err := secretbox.New(keyBytes)
	if err != nil {
		return out, err
	}
	sealed, err := box.Seal(p.Data)
	if err != nil {
		return out, err
	}
	id := util.MustParseUUID(uuid.NewString())
	key := storage.CollaborationPrivatePrefix + util.UUIDToString(id)
	cleanup := true
	defer func() {
		if cleanup {
			s.cleanupCollaborationAttachment(ctx, id, key)
		}
	}()
	if _, err = s.Storage.Upload(ctx, key, sealed, "application/octet-stream", ""); err != nil {
		return out, collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	var row db.CollaborationAttachment
	inserted := false
	err = s.withCollaborationAttachmentWrite(ctx, source, p.IssueID, true, func(q *db.Queries, lockedActor pgtype.UUID) error {
		var err error
		row, err = q.GetCollaborationAttachmentUpload(ctx, lookup)
		if err == nil {
			return compareCollaborationAttachmentUpload(row, p, digest)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		count, err := checkCollaborationAttachmentDraft(ctx, q, lockedActor, p.DraftID, source, p.IssueID)
		if err != nil {
			return err
		}
		if count >= collaborationMaxSelectedAttachments {
			return collaborationError(http.StatusConflict, "attachment_limit")
		}
		row, err = q.CreateCollaborationAttachment(ctx, db.CreateCollaborationAttachmentParams{ID: id, DraftID: p.DraftID,
			UploaderUserID: lockedActor, SourceWorkspaceID: source, IntendedIssueID: p.IssueID, UploadRequestID: lookup.UploadRequestID,
			StorageKey: key, ContentSha256: pgtype.Text{String: digest, Valid: true}, EncryptionKey: keyBytes,
			Name: p.Name, ContentType: p.ContentType, SizeBytes: int64(len(p.Data))})
		inserted = err == nil
		return err
	})
	if err != nil {
		if inserted && errors.Is(err, errCollaborationAttachmentCommit) {
			cleanup = false
		}
		return out, err
	}
	cleanup = !inserted
	return CollaborationAttachmentMetadata(row), nil
}

func compareCollaborationAttachmentUpload(row db.CollaborationAttachment, p CollaborationAttachmentUpload, digest string) error {
	if row.DiscardedAt.Valid {
		return collaborationError(http.StatusGone, "attachment_discarded")
	}
	if row.ContentSha256.String != digest || row.Name != p.Name || row.ContentType != p.ContentType || row.SizeBytes != int64(len(p.Data)) ||
		row.IntendedIssueID != p.IssueID || (p.SourceWorkspaceID.Valid && row.SourceWorkspaceID != p.SourceWorkspaceID) {
		return collaborationError(http.StatusConflict, "idempotency_conflict")
	}
	return validateCollaborationStoredAttachment(row)
}

func (s *CollaborationService) cleanupCollaborationAttachment(ctx context.Context, id pgtype.UUID, key string) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.Storage.DeleteObject(cleanup, key); err != nil {
		slog.Warn("collaboration attachment orphan retained", "attachment_id", util.UUIDToString(id), "code", "storage_unavailable")
	}
}

// File writers use account -> source workspace -> issue -> attachment order.
// They borrow existing ACLs; delivery/comment writers additionally use the
// content transaction owner and its session/event/operation fences.
func (s *CollaborationService) withCollaborationAttachmentWrite(ctx context.Context, source, issue pgtype.UUID, creating bool, write func(*db.Queries, pgtype.UUID) error) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	actor, err := collaborationNotificationViewer(ctx, q)
	if err != nil {
		return err
	}
	users, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{actor})
	if err != nil || len(users) != 1 {
		if err != nil {
			return err
		}
		return collaborationError(http.StatusForbidden, "forbidden")
	}
	if err = authorizeWorkflowAccount(ctx, q, actor, users[0]); err != nil {
		return err
	}
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, source); err != nil {
		return err
	}
	if _, err = q.LockCollaborationSourceMember(ctx, db.LockCollaborationSourceMemberParams{WorkspaceID: source, UserID: actor}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if issue.Valid {
		if _, err = q.LockCollaborationIssue(ctx, issue); err != nil {
			return err
		}
	} else if creating {
		if _, err = q.LockCollaborationGateShared(ctx); err != nil {
			return err
		}
	}
	resolved, err := s.authorizeAttachmentUploadContext(ctx, q, actor, source, issue, creating)
	if err != nil {
		return err
	}
	if resolved != source {
		return collaborationError(http.StatusConflict, "stale_source")
	}
	if err = write(q, actor); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("%w: %w", errCollaborationAttachmentCommit, err)
	}
	return nil
}

func (s *CollaborationService) authorizeAttachmentUploadContext(ctx context.Context, q *db.Queries, actor, source, issue pgtype.UUID, creating bool) (pgtype.UUID, error) {
	if issue.Valid {
		access, err := s.ResolveIssueAccess(ctx, q, issue)
		if err != nil {
			return pgtype.UUID{}, err
		}
		if !access.Capabilities.Comment {
			return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
		}
		return access.Issue.WorkspaceID, nil
	}
	_, err := q.GetCollaborationRecipientSource(ctx, db.GetCollaborationRecipientSourceParams{ActorID: actor, SourceWorkspaceID: source})
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if creating {
		gate, err := q.GetCollaborationGate(ctx)
		if err != nil {
			return pgtype.UUID{}, err
		}
		if !gate.CreationEnabled {
			return pgtype.UUID{}, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
		}
	}
	return source, nil
}

func validateCollaborationStoredAttachment(row db.CollaborationAttachment) error {
	if !row.SourceWorkspaceID.Valid || !row.DraftID.Valid || !row.ID.Valid || row.Version < 1 ||
		len(row.EncryptionKey) != secretbox.KeySize || !row.ContentSha256.Valid || len(row.ContentSha256.String) != sha256.Size*2 ||
		row.SizeBytes < 0 || row.SizeBytes > CollaborationAttachmentMaxBytes ||
		!strings.HasPrefix(row.StorageKey, storage.CollaborationPrivatePrefix) || row.StorageKey != storage.CollaborationPrivatePrefix+util.UUIDToString(row.ID) {
		return collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
	}
	return nil
}

func (s *CollaborationService) authorizeCollaborationAttachment(ctx context.Context, q *db.Queries, row db.CollaborationAttachment, actor pgtype.UUID) error {
	if row.DiscardedAt.Valid {
		return collaborationError(http.StatusNotFound, "not_found")
	}
	if row.IssueID.Valid {
		access, err := s.ResolveIssueAccess(ctx, q, row.IssueID)
		if err != nil {
			return err
		}
		if !row.DeliveredAt.Valid || access.Issue.WorkspaceID != row.SourceWorkspaceID {
			return collaborationError(http.StatusNotFound, "not_found")
		}
	} else {
		if row.UploaderUserID != actor {
			return collaborationError(http.StatusNotFound, "not_found")
		}
		if _, err := s.authorizeAttachmentUploadContext(ctx, q, actor, row.SourceWorkspaceID, row.IntendedIssueID, false); err != nil {
			return err
		}
	}
	return validateCollaborationStoredAttachment(row)
}

// GetAttachment is private service data. Only Metadata may be serialized.
func (s *CollaborationService) GetAttachment(ctx context.Context, issue, draft, id pgtype.UUID) (db.CollaborationAttachment, error) {
	var row db.CollaborationAttachment
	actor, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return row, err
	}
	row, err = s.Queries.GetCollaborationAttachment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return row, err
	}
	if issue.Valid {
		if row.IssueID != issue {
			return row, collaborationError(http.StatusNotFound, "not_found")
		}
	} else if !draft.Valid || row.DraftID != draft || row.UploaderUserID != actor || row.IssueID.Valid {
		return row, collaborationError(http.StatusNotFound, "not_found")
	}
	return row, s.authorizeCollaborationAttachment(ctx, s.Queries, row, actor)
}

func (s *CollaborationService) GetAttachmentUpload(ctx context.Context, draft pgtype.UUID, requestID string) (CollaborationAttachmentView, error) {
	var out CollaborationAttachmentView
	actor, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return out, err
	}
	if !draft.Valid || requestID == "" || len(requestID) > 128 {
		return out, collaborationError(http.StatusBadRequest, "invalid_attachment_request")
	}
	row, err := s.Queries.GetCollaborationAttachmentUpload(ctx, db.GetCollaborationAttachmentUploadParams{DraftID: draft, UploaderUserID: actor, UploadRequestID: pgtype.Text{String: requestID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return out, err
	}
	if err = s.authorizeCollaborationAttachment(ctx, s.Queries, row, actor); err != nil {
		return out, err
	}
	return CollaborationAttachmentMetadata(row), nil
}

func (s *CollaborationService) ListAttachments(ctx context.Context, issue pgtype.UUID) ([]CollaborationAttachmentView, error) {
	out := make([]CollaborationAttachmentView, 0)
	err := s.readCollaborationContent(ctx, issue, func(q *db.Queries, access CollaborationIssueAccess) error {
		rows, err := q.ListCollaborationAttachments(ctx, issue)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err = validateCollaborationStoredAttachment(row); err != nil || row.SourceWorkspaceID != access.Issue.WorkspaceID {
				return collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
			}
			out = append(out, CollaborationAttachmentMetadata(row))
		}
		return nil
	})
	return out, err
}

func (s *CollaborationService) ListDraftAttachments(ctx context.Context, draft pgtype.UUID) ([]CollaborationAttachmentView, error) {
	out := make([]CollaborationAttachmentView, 0)
	actor, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return out, err
	}
	rows, err := s.Queries.ListCollaborationDraftAttachments(ctx, db.ListCollaborationDraftAttachmentsParams{DraftID: draft, UploaderUserID: actor})
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		if err = s.authorizeCollaborationAttachment(ctx, s.Queries, row, actor); err != nil {
			return nil, err
		}
		out = append(out, CollaborationAttachmentMetadata(row))
	}
	return out, nil
}

func (s *CollaborationService) DiscardAttachmentDraft(ctx context.Context, draft, id pgtype.UUID) error {
	actor, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return err
	}
	row, err := s.Queries.GetCollaborationAttachment(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (row.DraftID != draft || row.UploaderUserID != actor || row.IssueID.Valid) {
		return collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return err
	}
	if s.Storage == nil {
		return collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	if err = validateCollaborationStoredAttachment(row); err != nil {
		return err
	}
	err = s.withCollaborationAttachmentWrite(ctx, row.SourceWorkspaceID, row.IntendedIssueID, false, func(q *db.Queries, actor pgtype.UUID) error {
		_, err := q.DiscardCollaborationDraftAttachment(ctx, db.DiscardCollaborationDraftAttachmentParams{ID: id, DraftID: draft, UploaderUserID: actor})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusConflict, "attachment_already_delivered")
		}
		return err
	})
	if err != nil {
		return err
	}
	if err = s.Storage.DeleteObject(ctx, row.StorageKey); err != nil {
		return collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	return nil
}

// ReadAttachmentBytes never asks the storage backend for a public URL. The
// ciphertext read and decrypted output are each bounded to the upload limit.
func (s *CollaborationService) ReadAttachmentBytes(ctx context.Context, row db.CollaborationAttachment) ([]byte, error) {
	if err := validateCollaborationStoredAttachment(row); err != nil {
		return nil, err
	}
	if s.Storage == nil {
		return nil, collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	r, err := s.Storage.GetReader(ctx, row.StorageKey)
	if err != nil {
		return nil, collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
	}
	defer r.Close()
	// The existing secretbox format is a 12-byte nonce and a 16-byte tag.
	sealed := make([]byte, int(row.SizeBytes)+28)
	if _, err = io.ReadFull(r, sealed); err != nil || ctx.Err() != nil {
		return nil, collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
	}
	var excess [1]byte
	if n, readErr := io.ReadFull(r, excess[:]); n != 0 || !errors.Is(readErr, io.EOF) {
		return nil, collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
	}
	box, err := secretbox.New(row.EncryptionKey)
	if err != nil {
		return nil, err
	}
	data, err := box.Open(sealed)
	if err != nil || int64(len(data)) != row.SizeBytes || ctx.Err() != nil {
		return nil, collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != row.ContentSha256.String {
		return nil, collaborationError(http.StatusServiceUnavailable, "attachment_unavailable")
	}
	return data, nil
}

func validateCollaborationAttachmentIDs(ids []pgtype.UUID) error {
	if len(ids) > collaborationMaxSelectedAttachments {
		return collaborationError(http.StatusBadRequest, "invalid_attachment_selection")
	}
	seen := make(map[pgtype.UUID]bool, len(ids))
	for _, id := range ids {
		if !id.Valid || seen[id] {
			return collaborationError(http.StatusBadRequest, "invalid_attachment_selection")
		}
		seen[id] = true
	}
	return nil
}

func sameCollaborationAttachmentIDs(a, b []pgtype.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for _, id := range a {
		if !slices.Contains(b, id) {
			return false
		}
	}
	return true
}

// The preflight avoids unnecessary storage work; the authoritative check is
// repeated while the upload/create/content transaction holds the uploader's
// user-row lock. Retained delivery rows consume the entire draft permanently,
// while discarded rows preserve its original source and intended issue.
func checkCollaborationAttachmentDraft(ctx context.Context, q *db.Queries, actor, draft, source, intended pgtype.UUID) (int64, error) {
	state, err := q.GetCollaborationAttachmentDraftState(ctx, db.GetCollaborationAttachmentDraftStateParams{
		DraftID: draft, UploaderUserID: actor, SourceWorkspaceID: source, IntendedIssueID: intended})
	if err != nil {
		return 0, err
	}
	if !state.ContextMatches {
		return 0, collaborationError(http.StatusConflict, "attachment_scope_mismatch")
	}
	if state.Consumed {
		return 0, collaborationError(http.StatusConflict, "attachment_draft_consumed")
	}
	return state.UndeliveredCount, nil
}

func recordCollaborationAttachmentReferences(ctx context.Context, q *db.Queries, kind string, reference pgtype.UUID, revision int64, ids []pgtype.UUID) error {
	for _, id := range ids {
		if err := q.CreateCollaborationAttachmentReference(ctx, db.CreateCollaborationAttachmentReferenceParams{
			AttachmentID: id, ReferenceKind: kind, ReferenceID: reference, ReferenceRevision: revision}); err != nil {
			return err
		}
	}
	return nil
}

func collaborationCommentAttachmentIDs(ctx context.Context, q *db.Queries, issue, comment pgtype.UUID) ([]pgtype.UUID, error) {
	rows, err := q.ListCollaborationCommentAttachmentReferences(ctx, db.ListCollaborationCommentAttachmentReferencesParams{IssueID: issue, CommentIds: []pgtype.UUID{comment}})
	if err != nil {
		return nil, err
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AttachmentID)
	}
	return ids, nil
}

func bindCollaborationAttachments(ctx context.Context, q *db.Queries, issue, source, actor pgtype.UUID, draft *pgtype.UUID, ids []pgtype.UUID, creating bool) ([]db.CollaborationAttachment, error) {
	if err := validateCollaborationAttachmentIDs(ids); err != nil {
		return nil, err
	}
	if draft != nil && !draft.Valid {
		return nil, collaborationError(http.StatusBadRequest, "invalid_attachment_selection")
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := q.LockCollaborationAttachments(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, collaborationError(http.StatusNotFound, "attachment_not_found")
	}
	for _, row := range rows {
		if row.SourceWorkspaceID != source || (row.IssueID.Valid && !creating && row.IssueID != issue) ||
			(!row.IssueID.Valid && row.UploaderUserID != actor) {
			return nil, collaborationError(http.StatusNotFound, "attachment_not_found")
		}
		if row.DiscardedAt.Valid || (row.IssueID.Valid && creating) {
			return nil, collaborationError(http.StatusConflict, "attachment_scope_mismatch")
		}
		if err = validateCollaborationStoredAttachment(row); err != nil {
			return nil, err
		}
		if row.IssueID.Valid {
			continue
		}
		if draft == nil || row.DraftID != *draft || row.UploaderUserID != actor ||
			(creating && row.IntendedIssueID.Valid) || (!creating && row.IntendedIssueID != issue) {
			return nil, collaborationError(http.StatusConflict, "attachment_scope_mismatch")
		}
	}
	if draft != nil {
		intended := issue
		if creating {
			intended = pgtype.UUID{}
		}
		if _, err = checkCollaborationAttachmentDraft(ctx, q, actor, *draft, source, intended); err != nil {
			return nil, err
		}
	}
	for i, row := range rows {
		if row.IssueID.Valid {
			continue
		}
		rows[i], err = q.DeliverCollaborationAttachment(ctx, db.DeliverCollaborationAttachmentParams{IssueID: issue, ID: row.ID})
		if err != nil {
			return nil, err
		}
		if err = q.CreateCollaborationAttachmentReference(ctx, db.CreateCollaborationAttachmentReferenceParams{
			AttachmentID: row.ID, ReferenceKind: "issue", ReferenceID: issue, ReferenceRevision: row.Version}); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

type CollaborationAttachmentDelivery struct {
	IssueID       pgtype.UUID
	DraftID       *pgtype.UUID
	AttachmentIDs []pgtype.UUID
	RequestID     string
}

func (s *CollaborationService) DeliverAttachments(ctx context.Context, p CollaborationAttachmentDelivery) (CollaborationCommentReceipt, error) {
	if len(p.AttachmentIDs) == 0 {
		return CollaborationCommentReceipt{}, collaborationError(http.StatusBadRequest, "invalid_attachment_selection")
	}
	return s.mutateCollaborationContent(ctx, p.IssueID, p.RequestID, "deliver_attachments", p, func(t *collaborationContentTx) (CollaborationCommentReceipt, error) {
		rows, err := bindCollaborationAttachments(ctx, t.q, p.IssueID, t.access.Issue.WorkspaceID, t.actor, p.DraftID, p.AttachmentIDs, false)
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		receipt := t.result("attachments_delivered", http.StatusOK)
		receipt.AttachmentIDs = slices.Clone(p.AttachmentIDs)
		var payload []byte
		payload, err = json.Marshal(map[string]any{"attachment_ids": receipt.AttachmentIDs})
		if err != nil {
			return CollaborationCommentReceipt{}, err
		}
		if len(rows) > 0 {
			receipt.EventSeq, err = t.q.AdvanceCollaborationEventSequence(ctx, p.IssueID)
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
			event, err := t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: p.IssueID, EventSeq: receipt.EventSeq, ActorUserID: t.actor, Kind: receipt.Code, Payload: payload})
			if err != nil {
				return CollaborationCommentReceipt{}, err
			}
			receipt.EventID = event.ID
		}
		return receipt, nil
	})
}
