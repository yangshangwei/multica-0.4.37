package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationContentAccount struct {
	UserID    pgtype.UUID `json:"user_id"`
	Name      string      `json:"name"`
	AvatarURL string      `json:"avatar_url"`
	External  bool        `json:"external"`
}

type CollaborationCommentRecipient struct {
	Recipient CollaborationContentAccount `json:"recipient"`
	IsMention bool                        `json:"is_mention"`
}

func collaborationMentionParticipants(ctx context.Context, q *db.Queries, access CollaborationIssueAccess) ([]CollaborationContentAccount, error) {
	rows, err := q.ListCollaborationMentionParticipants(ctx, db.ListCollaborationMentionParticipantsParams{
		IssueID: access.Issue.ID, WorkspaceID: access.Issue.WorkspaceID})
	if err != nil {
		return nil, err
	}
	out := make([]CollaborationContentAccount, 0, len(rows))
	for _, row := range rows {
		if auth.IsTemporarilyDisabledUser(util.UUIDToString(row.UserID), row.Email.String) {
			continue
		}
		out = append(out, CollaborationContentAccount{UserID: row.UserID, Name: row.Name, AvatarURL: row.AvatarUrl, External: row.External.Bool})
	}
	return out, nil
}

func collaborationCommentRecipients(participants []CollaborationContentAccount, actor pgtype.UUID, mentions collaborationMentionSet) []CollaborationCommentRecipient {
	out := make([]CollaborationCommentRecipient, 0, len(participants))
	for _, person := range participants {
		if person.UserID != actor {
			out = append(out, CollaborationCommentRecipient{Recipient: person, IsMention: mentions.All || mentions.People[person.UserID]})
		}
	}
	return out
}

func writeCollaborationCommentNotifications(ctx context.Context, q *db.Queries, issue, source pgtype.UUID, seq int64, kind, sourceKind string, recipients []CollaborationCommentRecipient) error {
	for _, recipient := range recipients {
		if _, err := q.CreateCollaborationContentNotification(ctx, db.CreateCollaborationContentNotificationParams{
			RecipientID: recipient.Recipient.UserID, IssueID: issue, EventSeq: seq, Kind: kind,
			IsMention: recipient.IsMention, SourceKind: sourceKind, SourceID: source}); err != nil {
			return err
		}
	}
	return nil
}

// Both sides expand against the same locked participant set. Editing an old
// @all after somebody joins must not turn that old mention into a new event.
func notifyAddedCollaborationMentions(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, actor, source pgtype.UUID, seq int64, sourceKind, before, after string) error {
	oldMentions, newMentions := parseCollaborationMentions(before), parseCollaborationMentions(after)
	participants, err := collaborationMentionParticipants(ctx, q, access)
	if err != nil {
		return err
	}
	added := make([]CollaborationCommentRecipient, 0)
	for _, person := range participants {
		if person.UserID != actor && (newMentions.All || newMentions.People[person.UserID]) && !(oldMentions.All || oldMentions.People[person.UserID]) {
			added = append(added, CollaborationCommentRecipient{Recipient: person, IsMention: true})
		}
	}
	return writeCollaborationCommentNotifications(ctx, q, access.Issue.ID, source, seq, "mention_added", sourceKind, added)
}

type CollaborationNotificationsQuery struct {
	Limit           int
	IncludeArchived bool
	BeforeAt        pgtype.Timestamptz
	BeforeID        pgtype.UUID
}

type CollaborationNotificationView struct {
	ID         pgtype.UUID `json:"id"`
	IssueID    pgtype.UUID `json:"issue_id"`
	EventSeq   int64       `json:"event_seq,string"`
	Kind       string      `json:"kind"`
	IsMention  bool        `json:"is_mention"`
	SourceKind string      `json:"source_kind"`
	SourceID   pgtype.UUID `json:"source_id"`
	IssueTitle *string     `json:"issue_title"`
	CanOpen    bool        `json:"can_open"`
	CreatedAt  time.Time   `json:"created_at"`
	ReadAt     *time.Time  `json:"read_at"`
	ArchivedAt *time.Time  `json:"archived_at"`
}

type CollaborationNotificationsPage struct {
	Items       []CollaborationNotificationView
	UnreadCount int64
	HasMore     bool
}

type CollaborationNotificationState struct {
	ID         pgtype.UUID `json:"id"`
	ReadAt     *time.Time  `json:"read_at"`
	ArchivedAt *time.Time  `json:"archived_at"`
}

func collaborationContentTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func collaborationNotificationViewer(ctx context.Context, q *db.Queries) (pgtype.UUID, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	viewer, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	user, err := q.GetUser(ctx, viewer)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if err = authorizeWorkflowAccount(ctx, q, viewer, user); err != nil {
		return pgtype.UUID{}, err
	}
	return viewer, nil
}

func (s *CollaborationService) ListNotifications(ctx context.Context, p CollaborationNotificationsQuery) (CollaborationNotificationsPage, error) {
	var out CollaborationNotificationsPage
	if p.Limit < 1 || p.Limit > 100 || p.BeforeAt.Valid != p.BeforeID.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_notification_query")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return out, err
	}
	q := s.Queries.WithTx(tx)
	viewer, err := collaborationNotificationViewer(ctx, q)
	if err != nil {
		return out, err
	}
	rows, err := q.ListCollaborationNotifications(ctx, db.ListCollaborationNotificationsParams{
		RecipientID: viewer, IncludeArchived: p.IncludeArchived, BeforeAt: p.BeforeAt, BeforeID: p.BeforeID, RowLimit: int32(p.Limit + 1)})
	if err != nil {
		return out, err
	}
	out.UnreadCount, err = q.CountUnreadCollaborationNotifications(ctx, viewer)
	if err != nil {
		return out, err
	}
	out.Items = make([]CollaborationNotificationView, 0, p.Limit)
	for _, row := range rows {
		if len(out.Items) == p.Limit {
			out.HasMore = true
			break
		}
		n := row.CollaborationNotification
		item := CollaborationNotificationView{ID: n.ID, IssueID: n.IssueID, EventSeq: n.EventSeq, Kind: n.Kind, IsMention: n.IsMention,
			SourceKind: "issue", CreatedAt: n.CreatedAt.Time, ReadAt: collaborationContentTime(n.ReadAt), ArchivedAt: collaborationContentTime(n.ArchivedAt)}
		// Retained notifications are not durable access grants. Never hydrate a
		// removed/denied notification with a title or private comment/event anchor.
		if row.CanOpen && n.Kind != "removed" {
			item.CanOpen, item.IssueTitle, item.SourceKind, item.SourceID = true, &row.IssueTitle, n.SourceKind, n.SourceID
		}
		out.Items = append(out.Items, item)
	}
	if err = tx.Commit(ctx); err != nil {
		return CollaborationNotificationsPage{}, err
	}
	return out, nil
}

func (s *CollaborationService) SetNotificationState(ctx context.Context, id pgtype.UUID, action string) (CollaborationNotificationState, error) {
	var out CollaborationNotificationState
	if !id.Valid || (action != "read" && action != "archive") {
		return out, collaborationError(http.StatusBadRequest, "invalid_notification_request")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	viewer, err := collaborationNotificationViewer(ctx, q)
	if err != nil {
		return out, err
	}
	users, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{viewer})
	if err != nil {
		return out, err
	}
	if len(users) != 1 {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err = authorizeWorkflowAccount(ctx, q, viewer, users[0]); err != nil {
		return out, err
	}
	var row db.CollaborationNotification
	if action == "read" {
		row, err = q.MarkCollaborationNotificationRead(ctx, db.MarkCollaborationNotificationReadParams{NotificationID: id, RecipientID: viewer})
	} else {
		row, err = q.ArchiveCollaborationNotification(ctx, db.ArchiveCollaborationNotificationParams{NotificationID: id, RecipientID: viewer})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return CollaborationNotificationState{ID: row.ID, ReadAt: collaborationContentTime(row.ReadAt), ArchivedAt: collaborationContentTime(row.ArchivedAt)}, nil
}
