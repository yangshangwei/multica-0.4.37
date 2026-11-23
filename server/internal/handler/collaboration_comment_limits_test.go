package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationCommentUTF8LimitIncludesJSONEnvelopeAndOriginalRecovery(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, issue, "Original content")
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1`, issue)
	content := strings.Repeat("界", 21845) + "a"
	var receipt service.CollaborationCommentReceipt
	testutil.Call(t, testHandler.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, comment, testUserID, uuid.NewString(), map[string]string{
		"content": content, "expected_comment_revision": "1",
	})).Want(http.StatusOK).JSON(&receipt)
	if receipt.Code != "comment_edited" || receipt.CommentRevision != 2 {
		t.Fatalf("exact byte limit rejected with JSON envelope: %+v", receipt)
	}
	key := uuid.NewString()
	testutil.Call(t, testHandler.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, comment, testUserID, key, map[string]string{
		"content": strings.Repeat("界", 30000), "expected_comment_revision": "2",
	})).Want(http.StatusBadRequest).JSON(&receipt)
	if receipt.Code != "invalid_comment_request" || receipt.RequestID != key || receipt.Action != "edit_comment" {
		t.Fatalf("oversized original was not durably refused: %+v", receipt)
	}
	read := withURLParams(collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil), "id", issue, "requestId", key)
	testutil.Call(t, testHandler.GetCollaborationCommentOperation, read).Want(http.StatusOK).JSON(&receipt)
	if receipt.Code != "invalid_comment_request" || receipt.Status != http.StatusBadRequest || receipt.RequestID != key {
		t.Fatalf("GET lost original refusal: %+v", receipt)
	}
	if dbfx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND content=$2 AND revision=2`, comment, content) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='comment_edited'`, issue) != 1 {
		t.Fatal("refused original changed comment/history")
	}

	// Other workflow requests keep their existing total-body bound.
	testutil.Call(t, testHandler.UpdateCollaborationIssue, collaborationContentRequest(http.MethodPatch, issue, "", testUserID, uuid.NewString(), map[string]string{
		"description": content,
	})).Want(http.StatusRequestEntityTooLarge)
}
