package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type CollaborationCommentsRecallQuery struct {
	Limit    int
	MaxBytes int
	Cursor   string
}

func (p CollaborationCommentsRecallQuery) pagination() (pgtype.Timestamptz, pgtype.UUID, error) {
	var at pgtype.Timestamptz
	var id pgtype.UUID
	if p.Limit < 1 || p.Limit > 100 || p.MaxBytes < protocol.CollaborationRecallMinBytes || p.MaxBytes > protocol.CollaborationRecallMaxBytes {
		return at, id, collaborationError(http.StatusBadRequest, "invalid_comment_query")
	}
	if p.Cursor != "" {
		cursor, err := protocol.DecodeCollaborationCommentCursor(p.Cursor)
		if err != nil {
			return at, id, collaborationError(http.StatusBadRequest, "invalid_cursor")
		}
		at = pgtype.Timestamptz{Time: cursor.CreatedAt, Valid: true}
		id, err = util.ParseUUID(cursor.ID)
		if err != nil {
			return at, id, collaborationError(http.StatusBadRequest, "invalid_cursor")
		}
	}
	return at, id, nil
}

func collaborationCommentAuthorities(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, comments []db.Comment) (map[pgtype.UUID]protocol.CollaborationCommentAuthorAuthority, error) {
	ids := make([]pgtype.UUID, 0, len(comments))
	for _, comment := range comments {
		if comment.AuthorType == "member" {
			ids = append(ids, comment.AuthorID)
		}
	}
	rows, err := q.ListCollaborationCommentAuthorFacts(ctx, db.ListCollaborationCommentAuthorFactsParams{IssueID: access.Issue.ID, WorkspaceID: access.Issue.WorkspaceID, UserIds: ids})
	if err != nil {
		return nil, err
	}
	out := make(map[pgtype.UUID]protocol.CollaborationCommentAuthorAuthority, len(rows))
	at := time.Now().UTC()
	for _, row := range rows {
		id := util.UUIDToString(row.ID)
		active := row.AccountActive && !auth.IsTemporarilyDisabledUser(id, row.Email.String)
		creator := access.Issue.CreatorType == "member" && access.Issue.CreatorID == row.ID
		handler := access.Issue.AssigneeType.String == "member" && access.Issue.AssigneeID == row.ID
		role := "unavailable"
		if active {
			switch {
			case row.Participant && creator:
				role = "creator"
			case row.Participant && handler:
				role = "handler"
			case row.Participant:
				role = "participant"
			case row.SourceMember:
				role = "source_member"
			}
		}
		// This evaluates account capability; it never installs an author as the
		// requesting principal or bypasses the caller's read authorization.
		caps := collaboration.CapabilitiesFor(collaboration.Principal{UserID: id, Kind: auth.CredentialJWT}, collaboration.AccessState{
			AccountActive: active, SourceMember: row.SourceMember, Participant: row.Participant, Creator: creator, Handler: handler,
			StatusCategory: access.StatusCategory, QueueState: access.QueueState})
		out[row.ID] = protocol.CollaborationCommentAuthorAuthority{Role: role, CanSubmitCommand: caps.SubmitCommand, EvaluatedAt: at}
	}
	return out, nil
}

func collaborationCommentContent(row db.Comment, authorities map[pgtype.UUID]protocol.CollaborationCommentAuthorAuthority) protocol.CollaborationCommentContent {
	authority, ok := authorities[row.AuthorID]
	if !ok || row.AuthorType != "member" {
		role := "unknown"
		if row.AuthorType == "agent" {
			role = "agent"
		}
		authority = protocol.CollaborationCommentAuthorAuthority{Role: role, EvaluatedAt: time.Now().UTC()}
	}
	state, size := "complete", len(row.Content)
	if row.DeletedAt.Valid {
		state, size = "deleted", 0
	}
	return protocol.CollaborationCommentContent{Trust: "data_only", AuthorAuthority: authority, ContentState: state, ContentBytes: size}
}

// The handler's JSON encoder appends a newline. Include it, escaping, metadata
// and the self-describing byte count; changing the count may add another digit.
func collaborationRecallJSONBytes(value any, size *int) error {
	for {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		n := len(data) + 1
		if n == *size {
			return nil
		}
		*size = n
	}
}

// Rows arrive newest-first. Only the first oversized row becomes a placeholder;
// a later row that does not fit must remain the first candidate of the next page.
func budgetCollaborationComments[T any](issueID string, identity *protocol.CollaborationRunIdentity, rows []T, total int64, more bool, p CollaborationCommentsRecallQuery,
	cursorFor func(T) string, omit func(T) T) (protocol.CollaborationCommentsRecallPage[T], error) {
	out := protocol.CollaborationCommentsRecallPage[T]{ProtocolVersion: protocol.CollaborationRecallVersion, Identity: identity, IssueID: issueID,
		Items: make([]T, 0), Total: total, MaxBytes: p.MaxBytes, TruncationReasons: make([]string, 0)}
	build := func(items []T, hasMore, byteLimited, omitted bool) (protocol.CollaborationCommentsRecallPage[T], error) {
		page := out
		page.NextCursor = nil
		page.Items = slices.Clone(items)
		slices.Reverse(page.Items)
		page.Returned = len(items)
		page.TruncationReasons = make([]string, 0)
		if hasMore && len(items) > 0 {
			cursor := cursorFor(items[len(items)-1])
			page.NextCursor = &cursor
			page.TruncationReasons = append(page.TruncationReasons, "more_comments")
		}
		if byteLimited || omitted {
			page.TruncationReasons = append(page.TruncationReasons, "byte_budget")
		}
		page.Truncated = hasMore || omitted
		err := collaborationRecallJSONBytes(&page, &page.Bytes)
		return page, err
	}
	selected := make([]T, 0, len(rows))
	for i, row := range rows {
		candidate, err := build(append(selected, row), more || i+1 < len(rows), false, false)
		if err != nil {
			return out, err
		}
		if candidate.Bytes <= p.MaxBytes {
			selected = append(selected, row)
			out = candidate
			continue
		}
		if len(selected) == 0 {
			candidate, err = build([]T{omit(row)}, more || len(rows) > 1, true, true)
			if err != nil {
				return out, err
			}
			if candidate.Bytes > p.MaxBytes {
				return out, collaborationError(http.StatusBadRequest, "budget_too_small")
			}
			return candidate, nil
		}
		candidate, err = build(selected, true, true, false)
		if err != nil {
			return out, err
		}
		// Reserve the budget-reason metadata while filling, so a boundary page
		// cannot overflow merely because its next row was larger than expected.
		for candidate.Bytes > p.MaxBytes && len(selected) > 1 {
			selected = selected[:len(selected)-1]
			candidate, err = build(selected, true, true, false)
			if err != nil {
				return out, err
			}
		}
		if candidate.Bytes > p.MaxBytes {
			candidate, err = build([]T{omit(selected[0])}, true, true, true)
		}
		if err != nil {
			return out, err
		}
		if candidate.Bytes > p.MaxBytes {
			return out, collaborationError(http.StatusBadRequest, "budget_too_small")
		}
		return candidate, nil
	}
	if len(rows) == 0 {
		page, err := build(selected, false, false, false)
		if err == nil && page.Bytes > p.MaxBytes {
			err = collaborationError(http.StatusBadRequest, "budget_too_small")
		}
		return page, err
	}
	return out, nil
}

func (s *CollaborationService) RecallComments(ctx context.Context, issueID pgtype.UUID, p CollaborationCommentsRecallQuery) (protocol.CollaborationCommentsRecallPage[CollaborationCommentView], error) {
	var out protocol.CollaborationCommentsRecallPage[CollaborationCommentView]
	before, beforeID, err := p.pagination()
	if err != nil {
		return out, err
	}
	err = s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		rows, err := q.ListCollaborationComments(ctx, db.ListCollaborationCommentsParams{IssueID: issueID, WorkspaceID: access.Issue.WorkspaceID, BeforeAt: before, BeforeID: beforeID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		total, err := q.CountCollaborationRecallComments(ctx, db.CountCollaborationRecallCommentsParams{IssueID: issueID, WorkspaceID: access.Issue.WorkspaceID, IncludeDeleted: true})
		if err != nil {
			return err
		}
		more := len(rows) > p.Limit
		if more {
			rows = rows[:p.Limit]
		}
		items, err := collaborationCommentViews(ctx, q, access, rows)
		if err != nil {
			return err
		}
		out, err = budgetCollaborationComments(util.UUIDToString(issueID), nil, items, total, more, p,
			func(item CollaborationCommentView) string {
				return protocol.EncodeCollaborationCommentCursor(item.CreatedAt, util.UUIDToString(item.ID))
			},
			func(item CollaborationCommentView) CollaborationCommentView {
				item.Content, item.ContentState, item.OmissionReason = "", "omitted", "byte_budget"
				item.ContentRef = "/api/collaboration/issues/" + util.UUIDToString(issueID) + "/comments/" + util.UUIDToString(item.ID)
				item.CanEdit = false
				item.AttachmentIDs, item.Reactions = []pgtype.UUID{}, []CollaborationCommentReaction{}
				return item
			})
		return err
	})
	return out, err
}

func (s *CollaborationService) RecallRuntimeComments(ctx context.Context, runID pgtype.UUID, token string, p CollaborationCommentsRecallQuery) (protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment], error) {
	var out protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment]
	before, beforeID, err := p.pagination()
	if err != nil {
		return out, err
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationCommentsRecallPage[protocol.CollaborationRecallComment], error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return out, err
		}
		rows, err := r.q.ListCollaborationRecallComments(ctx, db.ListCollaborationRecallCommentsParams{IssueID: r.access.Issue.ID, WorkspaceID: r.access.Issue.WorkspaceID, BeforeAt: before, BeforeID: beforeID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return out, err
		}
		total, err := r.q.CountCollaborationRecallComments(ctx, db.CountCollaborationRecallCommentsParams{IssueID: r.access.Issue.ID, WorkspaceID: r.access.Issue.WorkspaceID})
		if err != nil {
			return out, err
		}
		more := len(rows) > p.Limit
		if more {
			rows = rows[:p.Limit]
		}
		authorities, err := collaborationCommentAuthorities(ctx, r.q, r.access, rows)
		if err != nil {
			return out, err
		}
		authorIDs := make([]pgtype.UUID, 0, len(rows))
		for _, row := range rows {
			if row.AuthorType == "member" {
				authorIDs = append(authorIDs, row.AuthorID)
			}
		}
		accounts, err := collaborationContentAccounts(ctx, r.q, r.access.Issue.WorkspaceID, authorIDs)
		if err != nil {
			return out, err
		}
		items := make([]protocol.CollaborationRecallComment, 0, len(rows))
		for _, row := range rows {
			var parent *string
			if row.ParentID.Valid {
				value := util.UUIDToString(row.ParentID)
				parent = &value
			}
			name := ""
			if account := accounts[row.AuthorID]; row.AuthorType == "member" && account != nil {
				name = account.Name
			}
			items = append(items, protocol.CollaborationRecallComment{CollaborationCommentContent: collaborationCommentContent(row, authorities),
				ID: util.UUIDToString(row.ID), IssueID: util.UUIDToString(row.IssueID), AuthorType: row.AuthorType, AuthorID: util.UUIDToString(row.AuthorID),
				AuthorName: name, Content: row.Content, ParentID: parent, CreatedAt: row.CreatedAt.Time, CommentRevision: strconv.FormatInt(row.Revision, 10)})
		}
		identity := collaborationRunIdentity(r.run, r.access.Issue.ID)
		return budgetCollaborationComments(identity.IssueID, &identity, items, total, more, p,
			func(item protocol.CollaborationRecallComment) string {
				return protocol.EncodeCollaborationCommentCursor(item.CreatedAt, item.ID)
			},
			func(item protocol.CollaborationRecallComment) protocol.CollaborationRecallComment {
				item.Content, item.ContentState, item.OmissionReason = "", "omitted", "byte_budget"
				item.ContentRef = "/api/collaboration/runtime/runs/" + identity.RunID + "/comments/" + item.ID + "/content?revision=" + item.CommentRevision
				return item
			})
	})
}

type CollaborationCommentContentQuery struct {
	Revision string
	Cursor   string
	MaxBytes int
}

func (s *CollaborationService) RuntimeCommentContent(ctx context.Context, runID pgtype.UUID, token string, commentID pgtype.UUID, p CollaborationCommentContentQuery) (protocol.CollaborationCommentContentChunk, error) {
	var out protocol.CollaborationCommentContentChunk
	revision, err := strconv.ParseInt(p.Revision, 10, 64)
	if err != nil || revision < 1 || strconv.FormatInt(revision, 10) != p.Revision || !commentID.Valid || p.MaxBytes < protocol.CollaborationRecallMinBytes || p.MaxBytes > protocol.CollaborationRecallMaxBytes {
		return out, collaborationError(http.StatusBadRequest, "invalid_comment_query")
	}
	return collaborationRuntimeMutation(ctx, s, func(tx pgx.Tx) (protocol.CollaborationCommentContentChunk, error) {
		r, err := s.lockRuntimeBusiness(ctx, tx, runID, token)
		if err != nil {
			return out, err
		}
		row, err := r.q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{IssueID: r.access.Issue.ID, WorkspaceID: r.access.Issue.WorkspaceID, CommentID: commentID})
		if errors.Is(err, pgx.ErrNoRows) || err == nil && row.DeletedAt.Valid {
			return out, collaborationError(http.StatusNotFound, "comment_not_found")
		}
		if err != nil {
			return out, err
		}
		if row.Revision != revision {
			return out, collaborationError(http.StatusConflict, "stale_comment_revision")
		}
		identity := collaborationRunIdentity(r.run, r.access.Issue.ID)
		return chunkCollaborationComment(identity, util.UUIDToString(commentID), row.Content, p)
	})
}

func chunkCollaborationComment(identity protocol.CollaborationRunIdentity, commentID, body string, p CollaborationCommentContentQuery) (protocol.CollaborationCommentContentChunk, error) {
	out := protocol.CollaborationCommentContentChunk{ProtocolVersion: protocol.CollaborationRecallVersion, Identity: identity, IssueID: identity.IssueID, CommentID: commentID, Revision: p.Revision, TotalBytes: len(body), MaxBytes: p.MaxBytes}
	if p.Cursor != "" {
		cursor, err := protocol.DecodeCollaborationCommentContentCursor(p.Cursor)
		if err != nil || cursor.RunID != identity.RunID || cursor.IssueID != identity.IssueID || cursor.CommentID != commentID || cursor.Revision != p.Revision || cursor.Offset >= len(body) || !utf8.RuneStart(body[cursor.Offset]) {
			return out, collaborationError(http.StatusBadRequest, "invalid_cursor")
		}
		out.Offset = cursor.Offset
	}
	build := func(end int) (protocol.CollaborationCommentContentChunk, error) {
		chunk := out
		chunk.Content = body[out.Offset:end]
		chunk.Complete = end == len(body)
		if !chunk.Complete {
			cursor := protocol.EncodeCollaborationCommentContentCursor(protocol.CollaborationCommentContentCursor{RunID: identity.RunID, IssueID: identity.IssueID, CommentID: commentID, Revision: p.Revision, Offset: end})
			chunk.NextCursor = &cursor
		}
		err := collaborationRecallJSONBytes(&chunk, &chunk.Bytes)
		return chunk, err
	}
	// Try the complete remainder first: dropping the next cursor can make the
	// final chunk smaller than a nonfinal chunk with fewer body bytes.
	full, err := build(len(body))
	if err != nil || full.Bytes <= p.MaxBytes {
		return full, err
	}
	ends := make([]int, 0, len(body)-out.Offset)
	for end := out.Offset; end < len(body); {
		_, width := utf8.DecodeRuneInString(body[end:])
		end += width
		if end < len(body) {
			ends = append(ends, end)
		}
	}
	low, high, best := 0, len(ends)-1, -1
	for low <= high {
		mid := low + (high-low)/2
		chunk, err := build(ends[mid])
		if err != nil {
			return out, err
		}
		if chunk.Bytes <= p.MaxBytes {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if best < 0 {
		return out, collaborationError(http.StatusBadRequest, "budget_too_small")
	}
	return build(ends[best])
}
