package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCollaborationCommentOversizedOriginalGetsDurableRefusal(t *testing.T) {
	f := newCollaborationContentFixture(t)
	comment := f.comment(t, f.creator, "Original content", pgtype.UUID{})
	content := strings.Repeat("界", 21845) + "a"
	p := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment, RequestID: uuid.NewString(), ExpectedCommentRevision: 1, Content: content}
	valid, err := f.svc.EditComment(f.ctx, p)
	if err != nil || valid.CommentRevision != 2 {
		t.Fatalf("exact UTF-8 limit was rejected: %+v %v", valid, err)
	}
	p.RequestID, p.Content, p.ExpectedCommentRevision = uuid.NewString(), content+"a", 2
	notifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, p.IssueID)
	events := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, p.IssueID)
	receipt, err := f.svc.EditComment(f.ctx, p)
	if !collaborationErrorCode(err, "invalid_comment_request") || receipt.Status != http.StatusBadRequest || receipt.CommentID != comment || receipt.RequestID != p.RequestID || receipt.Action != "edit_comment" {
		t.Fatalf("oversized original lacks a correlated refusal: %+v %v", receipt, err)
	}
	read, err := f.svc.GetCommentOperation(f.ctx, p.IssueID, p.RequestID)
	if err != nil || read.Code != "invalid_comment_request" || read.RequestID != p.RequestID || read.Status != http.StatusBadRequest {
		t.Fatalf("original refusal cannot be recovered: %+v %v", read, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND content=$2 AND revision=2`, comment, content) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, p.IssueID) != events ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, p.IssueID) != notifications {
		t.Fatal("size refusal changed content or emitted events")
	}
	if _, err := f.svc.EditComment(f.ctx, p); !collaborationErrorCode(err, "invalid_comment_request") {
		t.Fatalf("original replay changed refusal: %v", err)
	}
	p.Content = "Corrected content"
	if _, err := f.svc.EditComment(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed content reused original key: %v", err)
	}
	p.RequestID = uuid.NewString()
	if _, err := f.svc.EditComment(f.ctx, p); err != nil {
		t.Fatal(err)
	}
}
