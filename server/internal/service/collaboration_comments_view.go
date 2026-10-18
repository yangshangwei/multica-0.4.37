package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationCommentsQuery struct {
	Limit    int
	BeforeAt pgtype.Timestamptz
	BeforeID pgtype.UUID
}

type CollaborationCommentReaction struct {
	Emoji         string `json:"emoji"`
	Count         int    `json:"count"`
	ViewerReacted bool   `json:"viewer_reacted"`
}

type CollaborationCommentView struct {
	ID              pgtype.UUID                    `json:"id"`
	IssueID         pgtype.UUID                    `json:"issue_id"`
	AuthorType      string                         `json:"author_type"`
	Author          *CollaborationContentAccount   `json:"author"`
	Content         string                         `json:"content"`
	Type            string                         `json:"type"`
	ParentID        pgtype.UUID                    `json:"parent_id"`
	CommentRevision int64                          `json:"comment_revision,string"`
	CreatedAt       time.Time                      `json:"created_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
	DeletedAt       *time.Time                     `json:"deleted_at"`
	ResolvedAt      *time.Time                     `json:"resolved_at"`
	ResolvedBy      *CollaborationContentAccount   `json:"resolved_by"`
	CanEdit         bool                           `json:"can_edit"`
	CanResolve      bool                           `json:"can_resolve"`
	Reactions       []CollaborationCommentReaction `json:"reactions"`
	AttachmentIDs   []pgtype.UUID                  `json:"attachment_ids"`
}

type CollaborationCommentsPage struct {
	Items   []CollaborationCommentView
	HasMore bool
}

func (s *CollaborationService) ListComments(ctx context.Context, issueID pgtype.UUID, p CollaborationCommentsQuery) (CollaborationCommentsPage, error) {
	out := CollaborationCommentsPage{Items: make([]CollaborationCommentView, 0)}
	if p.Limit < 1 || p.Limit > 100 || p.BeforeAt.Valid != p.BeforeID.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_comment_query")
	}
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		rows, err := q.ListCollaborationComments(ctx, db.ListCollaborationCommentsParams{
			IssueID: issueID, WorkspaceID: access.Issue.WorkspaceID, BeforeAt: p.BeforeAt, BeforeID: p.BeforeID, RowLimit: int32(p.Limit + 1)})
		if err != nil {
			return err
		}
		if len(rows) > p.Limit {
			out.HasMore, rows = true, rows[:p.Limit]
		}
		// A page is selected newest-first and displayed chronologically. The
		// next cursor is its first item, not the last displayed comment.
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
		out.Items, err = collaborationCommentViews(ctx, q, access, rows)
		return err
	})
	return out, err
}

func (s *CollaborationService) GetComment(ctx context.Context, issueID, commentID pgtype.UUID) (CollaborationCommentView, error) {
	var out CollaborationCommentView
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		comment, err := q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{
			IssueID: issueID, WorkspaceID: access.Issue.WorkspaceID, CommentID: commentID})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(http.StatusNotFound, "not_found")
		}
		if err != nil {
			return err
		}
		items, err := collaborationCommentViews(ctx, q, access, []db.Comment{comment})
		if err == nil {
			out = items[0]
		}
		return err
	})
	return out, err
}

func collaborationContentAccounts(ctx context.Context, q *db.Queries, workspace pgtype.UUID, ids []pgtype.UUID) (map[pgtype.UUID]*CollaborationContentAccount, error) {
	rows, err := q.ListCollaborationContentAccounts(ctx, db.ListCollaborationContentAccountsParams{WorkspaceID: workspace, UserIds: ids})
	if err != nil {
		return nil, err
	}
	out := make(map[pgtype.UUID]*CollaborationContentAccount, len(rows))
	for _, row := range rows {
		out[row.UserID] = &CollaborationContentAccount{UserID: row.UserID, Name: row.Name, AvatarURL: row.AvatarUrl, External: row.External.Bool}
	}
	return out, nil
}

func collaborationCommentViews(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, rows []db.Comment) ([]CollaborationCommentView, error) {
	principal, _ := collaboration.PrincipalFromContext(ctx)
	viewer, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return nil, err
	}
	users := make([]pgtype.UUID, 0, len(rows)*2)
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		if row.AuthorType == "member" {
			users = append(users, row.AuthorID)
		}
		if row.ResolvedByType.String == "member" && row.ResolvedByID.Valid {
			users = append(users, row.ResolvedByID)
		}
	}
	accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, users)
	if err != nil {
		return nil, err
	}
	attachmentRows, err := q.ListCollaborationCommentAttachmentReferences(ctx, db.ListCollaborationCommentAttachmentReferencesParams{IssueID: access.Issue.ID, CommentIds: ids})
	if err != nil {
		return nil, err
	}
	attachments := make(map[pgtype.UUID][]pgtype.UUID)
	for _, row := range attachmentRows {
		attachments[row.CommentID] = append(attachments[row.CommentID], row.AttachmentID)
	}
	reactions, err := q.ListReactionsByCommentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byComment := make(map[pgtype.UUID]map[string]*CollaborationCommentReaction)
	for _, reaction := range reactions {
		if byComment[reaction.CommentID] == nil {
			byComment[reaction.CommentID] = make(map[string]*CollaborationCommentReaction)
		}
		entry := byComment[reaction.CommentID][reaction.Emoji]
		if entry == nil {
			entry = &CollaborationCommentReaction{Emoji: reaction.Emoji}
			byComment[reaction.CommentID][reaction.Emoji] = entry
		}
		entry.Count++
		entry.ViewerReacted = entry.ViewerReacted || (reaction.ActorType == "member" && reaction.ActorID == viewer)
	}
	out := make([]CollaborationCommentView, 0, len(rows))
	for _, row := range rows {
		item := CollaborationCommentView{ID: row.ID, IssueID: row.IssueID, AuthorType: row.AuthorType, Content: row.Content,
			Type: row.Type, ParentID: row.ParentID, CommentRevision: row.Revision, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
			DeletedAt: collaborationContentTime(row.DeletedAt), ResolvedAt: collaborationContentTime(row.ResolvedAt),
			CanEdit: access.Capabilities.Comment && row.AuthorType == "member" && row.AuthorID == viewer && row.Type == "comment" && !row.DeletedAt.Valid,
			// Thread mutation routes are outside this vertical. Do not advertise
			// a resolution action just because the underlying role permits it.
			CanResolve: false, Reactions: make([]CollaborationCommentReaction, 0), AttachmentIDs: make([]pgtype.UUID, 0)}
		if row.AuthorType == "member" {
			item.Author = accounts[row.AuthorID]
		}
		if row.ResolvedByType.String == "member" {
			item.ResolvedBy = accounts[row.ResolvedByID]
		}
		if row.DeletedAt.Valid {
			item.Content = ""
		} else {
			item.AttachmentIDs = append(item.AttachmentIDs, attachments[row.ID]...)
		}
		for _, reaction := range byComment[row.ID] {
			item.Reactions = append(item.Reactions, *reaction)
		}
		sort.Slice(item.Reactions, func(i, j int) bool { return item.Reactions[i].Emoji < item.Reactions[j].Emoji })
		out = append(out, item)
	}
	return out, nil
}

type collaborationCommentHistoryPayload struct {
	CommentID           pgtype.UUID   `json:"comment_id"`
	CommentRevision     int64         `json:"comment_revision,string"`
	Before              *string       `json:"before"`
	After               string        `json:"after"`
	RequestID           string        `json:"request_id"`
	BeforeAttachmentIDs []pgtype.UUID `json:"before_attachment_ids"`
	AfterAttachmentIDs  []pgtype.UUID `json:"after_attachment_ids"`
}

type CollaborationCommentHistoryEntry struct {
	EventID             pgtype.UUID                  `json:"event_id"`
	EventSeq            int64                        `json:"event_seq,string"`
	Kind                string                       `json:"kind"`
	Actor               *CollaborationContentAccount `json:"actor"`
	CreatedAt           time.Time                    `json:"created_at"`
	CommentRevision     int64                        `json:"comment_revision,string"`
	Before              *string                      `json:"before"`
	After               string                       `json:"after"`
	BeforeAttachmentIDs []pgtype.UUID                `json:"before_attachment_ids"`
	AfterAttachmentIDs  []pgtype.UUID                `json:"after_attachment_ids"`
}

func (s *CollaborationService) CommentHistory(ctx context.Context, issueID, commentID pgtype.UUID) ([]CollaborationCommentHistoryEntry, error) {
	out := make([]CollaborationCommentHistoryEntry, 0)
	err := s.readCollaborationContent(ctx, issueID, func(q *db.Queries, access CollaborationIssueAccess) error {
		if _, err := q.GetCollaborationComment(ctx, db.GetCollaborationCommentParams{IssueID: issueID, WorkspaceID: access.Issue.WorkspaceID, CommentID: commentID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return collaborationError(http.StatusNotFound, "not_found")
			}
			return err
		}
		rows, err := q.ListCollaborationCommentHistory(ctx, db.ListCollaborationCommentHistoryParams{IssueID: issueID, CommentID: util.UUIDToString(commentID)})
		if err != nil {
			return err
		}
		ids := make([]pgtype.UUID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ActorUserID)
		}
		accounts, err := collaborationContentAccounts(ctx, q, access.Issue.WorkspaceID, ids)
		if err != nil {
			return err
		}
		for _, row := range rows {
			var payload collaborationCommentHistoryPayload
			if json.Unmarshal(row.Payload, &payload) != nil || payload.CommentID != commentID || payload.CommentRevision < 1 {
				return errors.New("invalid collaboration comment history")
			}
			out = append(out, CollaborationCommentHistoryEntry{EventID: row.ID, EventSeq: row.EventSeq, Kind: row.Kind,
				Actor: accounts[row.ActorUserID], CreatedAt: row.CreatedAt.Time, CommentRevision: payload.CommentRevision, Before: payload.Before, After: payload.After,
				BeforeAttachmentIDs: payload.BeforeAttachmentIDs, AfterAttachmentIDs: payload.AfterAttachmentIDs})
		}
		return nil
	})
	return out, err
}
