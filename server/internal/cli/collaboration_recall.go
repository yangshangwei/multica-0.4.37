package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (c *CollaborationContext) ReadComments(ctx context.Context, client *APIClient, limit, maxBytes int, cursor string) (protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment], error) {
	var out protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment]
	if limit < 1 || limit > 100 || maxBytes < protocol.CollaborationRecallMinBytes || maxBytes > protocol.CollaborationRecallMaxBytes {
		return out, errors.New("invalid collaboration comment limit or byte budget")
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}, "max_bytes": {strconv.Itoa(maxBytes)}}
	if cursor != "" {
		if _, err := protocol.DecodeCollaborationCommentCursor(cursor); err != nil {
			return out, err
		}
		query.Set("cursor", cursor)
	}
	var raw json.RawMessage
	if err := client.GetJSON(ctx, c.Path("/comments?")+query.Encode(), &raw); err != nil {
		return out, CollaborationError(err, "")
	}
	out, err := protocol.DecodeCollaborationCommentsRecall(raw)
	if err != nil {
		return out, err
	}
	if out.Identity == nil || *out.Identity != c.Identity || out.MaxBytes != maxBytes || out.Returned > limit || out.NextCursor != nil && *out.NextCursor == cursor {
		return out, errors.New("collaboration comments do not match the admitted read")
	}
	return out, nil
}

func (c *CollaborationContext) ReadCommentContent(ctx context.Context, client *APIClient, commentID, revision, cursor string, maxBytes int) (protocol.CollaborationCommentContentChunk, error) {
	var out protocol.CollaborationCommentContentChunk
	id, err := uuid.Parse(commentID)
	version, versionErr := strconv.ParseInt(revision, 10, 64)
	if err != nil || id == uuid.Nil || id.String() != commentID || versionErr != nil || version < 1 || strconv.FormatInt(version, 10) != revision || maxBytes < protocol.CollaborationRecallMinBytes || maxBytes > protocol.CollaborationRecallMaxBytes {
		return out, errors.New("comment content requires a comment UUID, positive --revision and valid --max-bytes")
	}
	offset := 0
	query := url.Values{"revision": {revision}, "max_bytes": {strconv.Itoa(maxBytes)}}
	if cursor != "" {
		value, err := protocol.DecodeCollaborationCommentContentCursor(cursor)
		if err != nil || value.RunID != c.Identity.RunID || value.IssueID != c.Identity.IssueID || value.CommentID != commentID || value.Revision != revision {
			return out, errors.New("comment content cursor does not match the admitted read")
		}
		offset = value.Offset
		query.Set("cursor", cursor)
	}
	var raw json.RawMessage
	if err := client.GetJSON(ctx, c.Path("/comments/"+commentID+"/content?")+query.Encode(), &raw); err != nil {
		return out, CollaborationError(err, "")
	}
	out, err = protocol.DecodeCollaborationCommentContent(raw)
	if err != nil {
		return out, err
	}
	if out.Identity != c.Identity || out.CommentID != commentID || out.Revision != revision || out.Offset != offset || out.MaxBytes != maxBytes {
		return out, errors.New("comment content does not match the admitted read")
	}
	return out, nil
}
