package protocol

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	CollaborationRecallVersion      = 1
	CollaborationRecallDefaultLimit = 50
	CollaborationRecallDefaultBytes = 64 << 10
	CollaborationRecallMinBytes     = 4 << 10
	CollaborationRecallMaxBytes     = 1 << 20
)

// AuthorAuthority describes current account facts, never historical admission.
// Only the server's frozen inputs/delegation carry execution intent.
type CollaborationCommentAuthorAuthority struct {
	Role             string    `json:"role"`
	CanSubmitCommand bool      `json:"can_submit_command"`
	EvaluatedAt      time.Time `json:"evaluated_at"`
}

func (a *CollaborationCommentAuthorAuthority) UnmarshalJSON(raw []byte) error {
	type wire CollaborationCommentAuthorAuthority
	var value wire
	if err := decodeCollaborationHostObject(raw, &value, "role", "can_submit_command", "evaluated_at"); err != nil {
		return err
	}
	*a = CollaborationCommentAuthorAuthority(value)
	return nil
}

type CollaborationCommentContent struct {
	Trust           string                              `json:"trust"`
	AuthorAuthority CollaborationCommentAuthorAuthority `json:"author_authority"`
	ContentState    string                              `json:"content_state"`
	ContentBytes    int                                 `json:"content_bytes"`
	ContentRef      string                              `json:"content_ref,omitempty"`
	OmissionReason  string                              `json:"omission_reason,omitempty"`
}

type CollaborationRecallComment struct {
	CollaborationCommentContent
	ID              string    `json:"id"`
	IssueID         string    `json:"issue_id"`
	AuthorType      string    `json:"author_type"`
	AuthorID        string    `json:"author_id"`
	Content         string    `json:"content"`
	ParentID        *string   `json:"parent_id"`
	CommentRevision string    `json:"comment_revision"`
	CreatedAt       time.Time `json:"created_at"`
}

func (c *CollaborationRecallComment) UnmarshalJSON(raw []byte) error {
	type wire CollaborationRecallComment
	var value wire
	if err := decodeCollaborationHostObject(raw, &value, "id", "issue_id", "author_type", "author_id", "content", "comment_revision", "created_at", "trust", "author_authority", "content_state", "content_bytes"); err != nil {
		return err
	}
	if !collaborationRecallHasFields(raw, "parent_id") {
		return errors.New("invalid collaboration recall comment")
	}
	*c = CollaborationRecallComment(value)
	return nil
}

func collaborationRecallHasFields(raw []byte, fields ...string) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return false
	}
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return false
		}
	}
	return true
}

type CollaborationCommentsRecallPage[T any] struct {
	ProtocolVersion   int                       `json:"protocol_version"`
	Identity          *CollaborationRunIdentity `json:"identity,omitempty"`
	IssueID           string                    `json:"issue_id"`
	Items             []T                       `json:"items"`
	NextCursor        *string                   `json:"next_cursor"`
	Total             int64                     `json:"total"`
	Returned          int                       `json:"returned"`
	Bytes             int                       `json:"bytes"`
	MaxBytes          int                       `json:"max_bytes"`
	Truncated         bool                      `json:"truncated"`
	TruncationReasons []string                  `json:"truncation_reasons"`
}

type CollaborationCommentContentChunk struct {
	ProtocolVersion int                      `json:"protocol_version"`
	Identity        CollaborationRunIdentity `json:"identity"`
	IssueID         string                   `json:"issue_id"`
	CommentID       string                   `json:"comment_id"`
	Revision        string                   `json:"revision"`
	Offset          int                      `json:"offset"`
	TotalBytes      int                      `json:"total_bytes"`
	Content         string                   `json:"content"`
	NextCursor      *string                  `json:"next_cursor"`
	Complete        bool                     `json:"complete"`
	Bytes           int                      `json:"bytes"`
	MaxBytes        int                      `json:"max_bytes"`
}

// Keep the existing (created_at,id) cursor wire usable by installed clients.
type CollaborationCommentCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func EncodeCollaborationCommentCursor(at time.Time, id string) string {
	data, _ := json.Marshal(CollaborationCommentCursor{CreatedAt: at, ID: id})
	return base64.RawURLEncoding.EncodeToString(data)
}

func DecodeCollaborationCommentCursor(value string) (CollaborationCommentCursor, error) {
	var out CollaborationCommentCursor
	if len(value) > 512 {
		return out, errors.New("invalid_cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || decodeCollaborationHostObject(data, &out, "created_at", "id") != nil || out.CreatedAt.IsZero() {
		return out, errors.New("invalid_cursor")
	}
	id, err := uuid.Parse(out.ID)
	if err != nil || id == uuid.Nil || id.String() != out.ID {
		return out, errors.New("invalid_cursor")
	}
	return out, nil
}

type CollaborationCommentContentCursor struct {
	RunID     string `json:"run_id"`
	IssueID   string `json:"issue_id"`
	CommentID string `json:"comment_id"`
	Revision  string `json:"revision"`
	Offset    int    `json:"offset"`
}

func EncodeCollaborationCommentContentCursor(value CollaborationCommentContentCursor) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func DecodeCollaborationCommentContentCursor(value string) (CollaborationCommentContentCursor, error) {
	var out CollaborationCommentContentCursor
	if len(value) > 1024 {
		return out, errors.New("invalid_cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || decodeCollaborationHostObject(data, &out, "run_id", "issue_id", "comment_id", "revision", "offset") != nil ||
		!preparationUUID(out.RunID) || !preparationUUID(out.IssueID) || !preparationUUID(out.CommentID) || !preparationVersion(out.Revision) || out.Revision == "0" || out.Offset <= 0 {
		return out, errors.New("invalid_cursor")
	}
	return out, nil
}

func DecodeCollaborationCommentsRecall(raw []byte) (CollaborationCommentsRecallPage[CollaborationRecallComment], error) {
	var out CollaborationCommentsRecallPage[CollaborationRecallComment]
	invalid := errors.New("invalid collaboration comments recall")
	if decodeCollaborationHostObject(raw, &out, "protocol_version", "identity", "issue_id", "items", "total", "returned", "bytes", "max_bytes", "truncated", "truncation_reasons") != nil ||
		!collaborationRecallHasFields(raw, "next_cursor") ||
		out.ProtocolVersion != CollaborationRecallVersion || out.Identity == nil || !preparationIdentity(*out.Identity) || out.IssueID != out.Identity.IssueID ||
		out.Items == nil || out.Returned != len(out.Items) || out.Returned > 100 || out.Total < int64(out.Returned) ||
		out.MaxBytes < CollaborationRecallMinBytes || out.MaxBytes > CollaborationRecallMaxBytes || out.Bytes != len(bytes.TrimSpace(raw))+1 || out.Bytes > out.MaxBytes ||
		out.TruncationReasons == nil || out.NextCursor != nil && (out.Returned == 0 || !out.Truncated) {
		return out, invalid
	}
	omitted := false
	seen := make(map[string]bool)
	for _, item := range out.Items {
		if !preparationUUID(item.ID) || seen[item.ID] || item.IssueID != out.IssueID || !preparationUUID(item.AuthorID) || item.AuthorType == "" ||
			!preparationVersion(item.CommentRevision) || item.CommentRevision == "0" || item.CreatedAt.IsZero() || item.Trust != "data_only" || item.AuthorAuthority.Role == "" || item.AuthorAuthority.EvaluatedAt.IsZero() ||
			item.ParentID != nil && !preparationUUID(*item.ParentID) || !utf8.ValidString(item.Content) || item.ContentBytes < 0 {
			return out, invalid
		}
		seen[item.ID] = true
		switch item.ContentState {
		case "complete":
			if item.ContentBytes != len(item.Content) || item.OmissionReason != "" {
				return out, invalid
			}
		case "omitted":
			omitted = true
			ref := "/api/collaboration/runtime/runs/" + out.Identity.RunID + "/comments/" + item.ID + "/content?revision=" + item.CommentRevision
			if item.Content != "" || item.OmissionReason != "byte_budget" || item.ContentRef != ref {
				return out, invalid
			}
		default:
			return out, invalid
		}
	}
	if out.Truncated != (out.NextCursor != nil || omitted) {
		return out, invalid
	}
	reasons := map[string]bool{}
	for _, reason := range out.TruncationReasons {
		if reasons[reason] || (reason != "more_comments" && reason != "byte_budget") {
			return out, invalid
		}
		reasons[reason] = true
	}
	if reasons["more_comments"] != (out.NextCursor != nil) || omitted && !reasons["byte_budget"] || !out.Truncated && len(reasons) != 0 {
		return out, invalid
	}
	if out.NextCursor != nil {
		cursor, err := DecodeCollaborationCommentCursor(*out.NextCursor)
		if err != nil || cursor.ID != out.Items[0].ID || !cursor.CreatedAt.Equal(out.Items[0].CreatedAt) {
			return out, invalid
		}
	}
	return out, nil
}

func DecodeCollaborationCommentContent(raw []byte) (CollaborationCommentContentChunk, error) {
	var out CollaborationCommentContentChunk
	invalid := errors.New("invalid collaboration comment content")
	if decodeCollaborationHostObject(raw, &out, "protocol_version", "identity", "issue_id", "comment_id", "revision", "offset", "total_bytes", "content", "complete", "bytes", "max_bytes") != nil ||
		!collaborationRecallHasFields(raw, "next_cursor") ||
		out.ProtocolVersion != CollaborationRecallVersion || !preparationIdentity(out.Identity) || out.IssueID != out.Identity.IssueID || !preparationUUID(out.CommentID) || !preparationVersion(out.Revision) || out.Revision == "0" ||
		out.Offset < 0 || out.TotalBytes < out.Offset+len(out.Content) || !utf8.ValidString(out.Content) ||
		out.MaxBytes < CollaborationRecallMinBytes || out.MaxBytes > CollaborationRecallMaxBytes || out.Bytes != len(bytes.TrimSpace(raw))+1 || out.Bytes > out.MaxBytes ||
		out.Complete != (out.Offset+len(out.Content) == out.TotalBytes) || out.Complete != (out.NextCursor == nil) {
		return out, invalid
	}
	if out.NextCursor != nil {
		cursor, err := DecodeCollaborationCommentContentCursor(*out.NextCursor)
		if err != nil || len(out.Content) == 0 || cursor.RunID != out.Identity.RunID || cursor.IssueID != out.IssueID || cursor.CommentID != out.CommentID || cursor.Revision != out.Revision || cursor.Offset != out.Offset+len(out.Content) {
			return out, invalid
		}
	}
	return out, nil
}
