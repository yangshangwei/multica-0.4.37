package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Attribution is resolved from the admitted run, never supplied by the caller.
type CollaborationRuntimeCommentAttachmentParams struct {
	CollaborationRuntimeCommentParams
	Name string
	Data []byte
}

type collaborationRuntimeOutputFingerprint struct {
	Name          string `json:"name"`
	ContentType   string `json:"content_type"`
	SizeBytes     int64  `json:"size_bytes"`
	ContentSHA256 string `json:"content_sha256"`
	AttemptSeq    string `json:"attempt_seq"`
}

// The Provider's original request stays flat and unchanged. Only the server
// appends the output IDs committed by this attempt at the first outcome write.
type collaborationStoredOutcome struct {
	protocol.CollaborationRunOutcome
	OutputAttachmentIDs []string `json:"output_attachment_ids,omitempty"`
}

func decodeCollaborationStoredOutcome(raw []byte) (collaborationStoredOutcome, error) {
	var out collaborationStoredOutcome
	if decodeCollaborationPreparationJSON(raw, &out) != nil {
		return out, invalidCollaborationLedger("invalid stored runtime outcome")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return out, invalidCollaborationLedger("invalid stored runtime outcome")
	}
	if refs, present := fields["output_attachment_ids"]; present && strings.TrimSpace(string(refs)) == "null" || len(out.OutputAttachmentIDs) > collaborationMaxSelectedAttachments {
		return out, invalidCollaborationLedger("invalid stored output references")
	}
	seen := make(map[string]bool, len(out.OutputAttachmentIDs))
	for _, value := range out.OutputAttachmentIDs {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value || seen[value] {
			return out, invalidCollaborationLedger("invalid stored output references")
		}
		seen[value] = true
	}
	return out, nil
}

func (r *collaborationRuntimeState) frozenOutputAttachmentIDs(ctx context.Context) ([]string, error) {
	rows, err := r.q.ListCollaborationRuntimeOutputAttachments(ctx, db.ListCollaborationRuntimeOutputAttachmentsParams{RunID: r.run.ID, AttemptSeq: r.run.AuthorizedAttemptSeq})
	if err != nil {
		return nil, err
	}
	if len(rows) > collaborationMaxSelectedAttachments {
		return nil, invalidCollaborationLedger("runtime output bound exceeded")
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		p := collaborationRuntimeOutputFingerprint{Name: row.Name, ContentType: row.ContentType, SizeBytes: row.SizeBytes, ContentSHA256: row.ContentSha256.String, AttemptSeq: strconv.FormatInt(r.run.AuthorizedAttemptSeq, 10)}
		if err = validateCollaborationRuntimeOutput(row, r, p); err != nil {
			return nil, err
		}
		ids = append(ids, util.UUIDToString(row.ID))
	}
	return ids, nil
}

func validateCollaborationRuntimeOutput(row db.CollaborationAttachment, r *collaborationRuntimeState, p collaborationRuntimeOutputFingerprint) error {
	if row.UploaderUserID.Valid || row.DraftID.Valid || row.IntendedIssueID.Valid || row.UploadRequestID.Valid || row.UploaderAgentID != r.run.AgentID || row.RunID != r.run.ID || !row.AttemptSeq.Valid || row.AttemptSeq.Int64 != r.run.AuthorizedAttemptSeq || row.IssueID != r.access.Issue.ID || row.SourceWorkspaceID != r.access.Issue.WorkspaceID || !row.DeliveredAt.Valid || row.DiscardedAt.Valid ||
		row.Name != p.Name || row.ContentType != p.ContentType || row.SizeBytes != p.SizeBytes || row.ContentSha256.String != p.ContentSHA256 || strconv.FormatInt(row.AttemptSeq.Int64, 10) != p.AttemptSeq {
		return invalidCollaborationLedger("runtime output identity mismatch")
	}
	return validateCollaborationStoredAttachment(row)
}

func (r *collaborationRuntimeState) runtimeOutputAttempt(ctx context.Context) (int64, error) {
	attempt, err := r.q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: r.run.ID, AttemptSeq: r.run.AuthorizedAttemptSeq})
	if err != nil {
		return 0, err
	}
	if attempt.AttemptSeq < 1 || attempt.AttemptSeq != r.run.AuthorizedAttemptSeq || attempt.ClaimGeneration != r.run.ClaimGeneration || !attempt.AuthorizedAt.Valid {
		return 0, invalidCollaborationLedger("runtime output attempt mismatch")
	}
	return attempt.AttemptSeq, nil
}

// External encryption/storage never holds runtime locks. Both the preflight
// and the final transaction require the current mcb, grant, input and attempt.
func (s *CollaborationService) CreateRuntimeCommentAttachment(ctx context.Context, runID pgtype.UUID, token string, p CollaborationRuntimeCommentAttachmentParams) (CollaborationRuntimeCommentReceipt, error) {
	var empty CollaborationRuntimeCommentReceipt
	if !runID.Valid || runID.Bytes == [16]byte{} || !p.CommentID.Valid || p.CommentID.Bytes == [16]byte{} || p.ParentID.Valid && p.ParentID.Bytes == [16]byte{} || strings.TrimSpace(p.Name) == "" || p.Name == "." || p.Name == ".." || len(p.Name) > 1024 || !utf8.ValidString(p.Name) || strings.ContainsAny(p.Name, "\x00/\\") {
		return empty, collaborationError(http.StatusBadRequest, "invalid_attachment_upload")
	}
	if len(p.Data) > CollaborationAttachmentMaxBytes {
		return empty, collaborationError(http.StatusRequestEntityTooLarge, "attachment_too_large")
	}
	if err := validateCollaborationCommentInput(CollaborationCommentInput{Content: p.Content, ParentID: p.ParentID}); err != nil {
		return empty, err
	}
	p.Content = util.SanitizeTextForPostgres(p.Content)
	sum := sha256.Sum256(p.Data)
	fingerprint := collaborationRuntimeOutputFingerprint{Name: p.Name, ContentType: http.DetectContentType(p.Data), SizeBytes: int64(len(p.Data)), ContentSHA256: hex.EncodeToString(sum[:])}
	type preflight struct {
		receipt CollaborationRuntimeCommentReceipt
		found   bool
	}
	prior, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (preflight, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return preflight{}, err
		}
		attempt, err := r.runtimeOutputAttempt(ctx)
		if err != nil {
			return preflight{}, err
		}
		fingerprint.AttemptSeq = strconv.FormatInt(attempt, 10)
		input := collaborationRuntimeCommentInput{CollaborationRuntimeCommentParams: p.CollaborationRuntimeCommentParams, Attachment: &fingerprint}
		receipt, found, err := r.runtimeCommentReceipt(ctx, input)
		if found || err != nil {
			return preflight{receipt, found}, err
		}
		if r.run.BusinessOutcome.Valid {
			return preflight{}, collaborationError(http.StatusConflict, "outcome_already_recorded")
		}
		if err = validateCollaborationCommentParent(ctx, r.q, r.access, p.ParentID); err != nil {
			return preflight{}, err
		}
		return preflight{}, nil
	})
	if err != nil || prior.found {
		return prior.receipt, err
	}
	if s.Storage == nil {
		return empty, collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	keyBytes := make([]byte, secretbox.KeySize)
	if _, err = rand.Read(keyBytes); err != nil {
		return empty, err
	}
	box, err := secretbox.New(keyBytes)
	if err != nil {
		return empty, err
	}
	sealed, err := box.Seal(p.Data)
	if err != nil {
		return empty, err
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
		return empty, collaborationError(http.StatusServiceUnavailable, "storage_unavailable")
	}
	inserted := false
	out, err := collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (CollaborationRuntimeCommentReceipt, error) {
		// A rolled-back callback owns no row. An earlier callback's insert must
		// not cause a later replay or failed callback to retain its losing object.
		inserted = false
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return empty, err
		}
		attempt, err := r.runtimeOutputAttempt(ctx)
		if err != nil {
			return empty, err
		}
		if strconv.FormatInt(attempt, 10) != fingerprint.AttemptSeq {
			return empty, collaborationError(http.StatusConflict, "stale_execution")
		}
		input := collaborationRuntimeCommentInput{CollaborationRuntimeCommentParams: p.CollaborationRuntimeCommentParams, Attachment: &fingerprint}
		if prior, found, err := r.runtimeCommentReceipt(ctx, input); found || err != nil {
			return prior, err
		}
		if r.run.BusinessOutcome.Valid {
			return empty, collaborationError(http.StatusConflict, "outcome_already_recorded")
		}
		if err = validateCollaborationCommentParent(ctx, r.q, r.access, p.ParentID); err != nil {
			return empty, err
		}
		count, err := r.q.CountCollaborationRuntimeOutputAttachments(ctx, db.CountCollaborationRuntimeOutputAttachmentsParams{RunID: runID, AttemptSeq: r.run.AuthorizedAttemptSeq})
		if err != nil {
			return empty, err
		}
		if count >= collaborationMaxSelectedAttachments {
			return empty, collaborationError(http.StatusConflict, "attachment_limit")
		}
		row, err := r.q.CreateCollaborationRuntimeOutputAttachment(ctx, db.CreateCollaborationRuntimeOutputAttachmentParams{ID: id, IssueID: r.access.Issue.ID, SourceWorkspaceID: r.access.Issue.WorkspaceID, UploaderAgentID: r.run.AgentID, RunID: runID, AttemptSeq: r.run.AuthorizedAttemptSeq, StorageKey: key, ContentSha256: fingerprint.ContentSHA256, EncryptionKey: keyBytes, Name: p.Name, ContentType: fingerprint.ContentType, SizeBytes: fingerprint.SizeBytes})
		inserted = err == nil
		if err != nil {
			return empty, err
		}
		if err = recordCollaborationAttachmentReferences(ctx, r.q, "issue", r.access.Issue.ID, row.Version, []pgtype.UUID{row.ID}); err != nil {
			return empty, err
		}
		view := CollaborationAttachmentMetadata(row)
		return r.writeRuntimeComment(ctx, input, &view)
	})
	if err != nil {
		var unknown *collaborationRuntimeCommitError
		if inserted && errors.As(err, &unknown) {
			cleanup = false
		}
		return empty, err
	}
	cleanup = !inserted
	return out, nil
}
