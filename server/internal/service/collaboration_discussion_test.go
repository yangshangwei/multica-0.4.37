package service

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationDiscussionKeepsContentVersionsAndOriginalActions(t *testing.T) {
	f := newCollaborationContentFixture(t)
	issueID := f.issue.Issue.ID
	commentID := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(issueID), "Human discussion", testutil.Cols{"author_id": f.creator, "revision": int64(9007199254740993)}))
	f.fx.Cleanup(t, `DELETE FROM comment_reaction WHERE comment_id=$1`, commentID)
	f.fx.Cleanup(t, `DELETE FROM issue_reaction WHERE issue_id=$1`, issueID)
	beforeNotifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, issueID)
	beforeCommands := f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID)
	beforeRevision := f.fx.Count(t, `SELECT revision FROM issue_collaboration WHERE issue_id=$1`, issueID)
	beforeIssueRevision := f.fx.Count(t, `SELECT revision FROM issue WHERE id=$1`, issueID)
	for _, target := range []pgtype.UUID{commentID, {}} {
		p := CollaborationDiscussionParams{IssueID: issueID, CommentID: target, RequestID: uuid.NewString(), Action: "add_reaction", Emoji: "👍"}
		first, err := f.svc.ChangeDiscussion(collaborationContext(f.handler), p)
		if err != nil || first.Code != "reaction_added" {
			t.Fatalf("add reaction: %+v %v", first, err)
		}
		remove := p
		remove.Action, remove.RequestID = "remove_reaction", uuid.NewString()
		if _, err = f.svc.ChangeDiscussion(collaborationContext(f.handler), remove); err != nil {
			t.Fatal(err)
		}
		replay, err := f.svc.ChangeDiscussion(collaborationContext(f.handler), p)
		if err != nil || !reflect.DeepEqual(replay, first) {
			t.Fatalf("replay changed original result: %+v %v", replay, err)
		}
		original, err := f.svc.GetDiscussionOperation(collaborationContext(f.handler), issueID, p.RequestID)
		if err != nil || !reflect.DeepEqual(original, first) || original.Emoji == nil || *original.Emoji != p.Emoji ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2 AND result->>'emoji'=$3`, issueID, p.RequestID, p.Emoji) != 1 {
			t.Fatalf("original emoji was not persisted and projected: %+v %v", original, err)
		}
		p.Emoji = "🎉"
		if _, err = f.svc.ChangeDiscussion(collaborationContext(f.handler), p); !collaborationErrorCode(err, "idempotency_conflict") {
			t.Fatalf("changed emoji reused original key: %v", err)
		}
	}
	p := CollaborationDiscussionParams{IssueID: issueID, CommentID: commentID, RequestID: uuid.NewString(), Action: "resolve"}
	resolved, err := f.svc.ChangeDiscussion(collaborationContext(f.handler), p)
	if err != nil || resolved.Emoji != nil || f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2 AND result->'emoji'='null'::jsonb`, issueID, p.RequestID) != 1 {
		t.Fatalf("thread resolution lacks explicit null emoji: %+v %v", resolved, err)
	}
	view, err := f.svc.GetComment(f.ctx, issueID, commentID)
	if err != nil || view.ResolvedBy == nil || util.UUIDToString(view.ResolvedBy.UserID) != f.handler || view.CommentRevision != 9007199254740993 || !view.CanResolve {
		t.Fatalf("resolution identity or content version changed: %+v %v", view, err)
	}
	if f.fx.Count(t, `SELECT revision FROM issue_collaboration WHERE issue_id=$1`, issueID) != beforeRevision ||
		f.fx.Count(t, `SELECT revision FROM issue WHERE id=$1`, issueID) != beforeIssueRevision ||
		f.fx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1`, issueID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, issueID) != beforeNotifications ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != beforeCommands ||
		f.fx.Count(t, `SELECT count(*) FROM comment_reaction WHERE comment_id=$1`, commentID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_reaction WHERE issue_id=$1`, issueID) != 0 {
		t.Fatal("discussion action changed edits, notifications, AI, or replayed a removed reaction")
	}
}

func TestCollaborationDiscussionDoesNotGuessMissingPersistedEmoji(t *testing.T) {
	f := newCollaborationContentFixture(t)
	issueID := f.issue.Issue.ID
	f.fx.Cleanup(t, `DELETE FROM issue_reaction WHERE issue_id=$1`, issueID)
	p := CollaborationDiscussionParams{IssueID: issueID, RequestID: uuid.NewString(), Action: "add_reaction", Emoji: "👍"}
	if _, err := f.svc.ChangeDiscussion(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE collaboration_operation SET result=result-'emoji' WHERE scope_id=$1 AND request_id=$2`, issueID, p.RequestID)
	if _, err := f.svc.GetDiscussionOperation(f.ctx, issueID, p.RequestID); !collaborationErrorCode(err, "operation_incomplete") {
		t.Fatalf("GET guessed original emoji: %v", err)
	}
	if _, err := f.svc.ChangeDiscussion(f.ctx, p); !collaborationErrorCode(err, "operation_incomplete") {
		t.Fatalf("replay guessed original emoji: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_reaction WHERE issue_id=$1`, issueID) != 1 {
		t.Fatal("incomplete receipt replay changed reactions")
	}
}

func TestCollaborationDiscussionEmojiLimitUsesUTF8BytesBeforeReservation(t *testing.T) {
	f := newCollaborationContentFixture(t)
	issueID := f.issue.Issue.ID
	f.fx.Cleanup(t, `DELETE FROM issue_reaction WHERE issue_id=$1`, issueID)
	p := CollaborationDiscussionParams{IssueID: issueID, RequestID: uuid.NewString(), Action: "add_reaction", Emoji: strings.Repeat("👍", 16)}
	receipt, err := f.svc.ChangeDiscussion(f.ctx, p)
	if err != nil || receipt.Emoji == nil || *receipt.Emoji != p.Emoji {
		t.Fatalf("64-byte reaction: %+v %v", receipt, err)
	}
	p.RequestID, p.Emoji = uuid.NewString(), strings.Repeat("字", 22)
	if _, err := f.svc.ChangeDiscussion(f.ctx, p); !collaborationErrorCode(err, "invalid_discussion_request") {
		t.Fatalf("oversized reaction accepted: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, issueID, p.RequestID) != 0 {
		t.Fatal("invalid reaction reserved an operation")
	}
}

func TestCollaborationDiscussionRequiredWriteFailureRollsBackResolution(t *testing.T) {
	f := newCollaborationContentFixture(t)
	root := f.comment(t, f.creator, "Root", pgtype.UUID{})
	reply := f.comment(t, f.handler, "Reply", root)
	p := CollaborationDiscussionParams{IssueID: f.issue.Issue.ID, CommentID: root, RequestID: uuid.NewString(), Action: "resolve"}
	if _, err := f.svc.ChangeDiscussion(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: "CreateCollaborationContentEvent"}
	p.CommentID, p.RequestID = reply, uuid.NewString()
	if _, err := f.svc.ChangeDiscussion(f.ctx, p); err == nil {
		t.Fatal("required write failure was ignored")
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND resolved_at IS NOT NULL`, root) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND resolved_at IS NOT NULL`, reply) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, p.IssueID, p.RequestID) != 0 {
		t.Fatal("failed replacement removed the original resolution or kept a partial operation")
	}
}

func TestCollaborationDiscussionOwnReceiptSurvivesDepartureWithoutReapplying(t *testing.T) {
	f := newCollaborationContentFixture(t)
	comment := f.comment(t, f.creator, "Private retained comment", pgtype.UUID{})
	p := CollaborationDiscussionParams{IssueID: f.issue.Issue.ID, CommentID: comment, RequestID: uuid.NewString(), Action: "resolve"}
	first, err := f.svc.ChangeDiscussion(collaborationContext(f.handler), p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(), epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, p.IssueID, f.handler)
	receipt, err := f.svc.GetDiscussionOperation(collaborationContext(f.handler), p.IssueID, p.RequestID)
	if err != nil || receipt != first {
		t.Fatalf("lost original own receipt: %+v %v", receipt, err)
	}
	if _, err := f.svc.GetDiscussionOperation(f.ctx, p.IssueID, p.RequestID); !collaborationErrorCode(err, "operation_not_found") {
		t.Fatalf("another account read the receipt: %v", err)
	}
	p.RequestID, p.Action = uuid.NewString(), "unresolve"
	if _, err := f.svc.ChangeDiscussion(collaborationContext(f.handler), p); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("receipt recovery restored write access: %v", err)
	}
}

func TestCollaborationDiscussionResolvesOneReplyAndPreservesDeletedRoot(t *testing.T) {
	f := newCollaborationContentFixture(t)
	issueID := f.issue.Issue.ID
	root := f.fx.Comment(t, util.UUIDToString(issueID), "Root", testutil.Cols{"author_id": f.creator})
	a := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(issueID), "First reply", testutil.Cols{"parent_id": root, "author_id": f.handler}))
	b := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(issueID), "Nested reply", testutil.Cols{"parent_id": util.UUIDToString(a), "author_id": f.creator}))
	if _, err := f.svc.DeleteComment(f.ctx, CollaborationEditCommentParams{IssueID: issueID, CommentID: util.MustParseUUID(root), RequestID: uuid.NewString(), ExpectedCommentRevision: 1}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	for _, id := range []pgtype.UUID{a, b} {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := f.svc.ChangeDiscussion(f.ctx, CollaborationDiscussionParams{IssueID: issueID, CommentID: id, RequestID: uuid.NewString(), Action: "resolve"})
			errors <- err
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND resolved_at IS NOT NULL`, issueID) != 1 {
		t.Fatal("thread has more than one resolution")
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE id IN ($1,$2) AND revision=1`, a, b) != 2 {
		t.Fatal("resolution changed reply content revisions")
	}
	view, err := f.svc.GetComment(f.ctx, issueID, a)
	if err != nil || util.UUIDToString(view.ParentID) != root || view.Content != "First reply" {
		t.Fatalf("deleted root lost reply anchor: %+v %v", view, err)
	}
}

func TestCollaborationDiscussionSourceAccessAndOwnReactionRemoval(t *testing.T) {
	f := newCollaborationContentFixture(t)
	issueID := f.issue.Issue.ID
	commentID := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(issueID), "Read from source", testutil.Cols{"author_id": f.creator}))
	f.fx.Cleanup(t, `DELETE FROM comment_reaction WHERE comment_id=$1`, commentID)
	f.fx.Member(t, f.workspace, f.outsider, "member")
	p := CollaborationDiscussionParams{IssueID: issueID, CommentID: commentID, RequestID: uuid.NewString(), Action: "add_reaction", Emoji: "👍"}
	if _, err := f.svc.ChangeDiscussion(collaborationContext(f.outsider), p); err != nil {
		t.Fatal(err)
	}
	p.RequestID, p.Action = uuid.NewString(), "remove_reaction"
	if _, err := f.svc.ChangeDiscussion(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetComment(collaborationContext(f.outsider), issueID, commentID)
	if err != nil || len(view.Reactions) != 1 || !view.Reactions[0].ViewerReacted || !view.CanResolve {
		t.Fatalf("source member lost own reaction or resolution ability: %+v %v", view, err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.outsider)
	p.RequestID = uuid.NewString()
	if _, err := f.svc.ChangeDiscussion(collaborationContext(f.outsider), p); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("departed commenter can still mutate: %v", err)
	}
}
