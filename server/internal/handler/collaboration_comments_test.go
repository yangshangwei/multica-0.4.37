package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationContentRequest(method, issue, comment, actor, key string, body any) *http.Request {
	path := "/api/collaboration/issues/" + issue + "/comments"
	if comment != "" {
		path += "/" + comment
	}
	r := withURLParam(newRequest(method, path, body), "id", issue)
	if comment != "" {
		r = withURLParams(r, "id", issue, "commentId", comment)
	}
	r.Header.Set("Idempotency-Key", key)
	r.Header.Set("X-Workspace-ID", "unrelated-ambient-workspace")
	r.Header.Set("X-Workspace-Slug", "unrelated-ambient-workspace")
	return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
}

func TestCollaborationCommentHTTPKeepsAuthorControlHistoryAndPrivateProjection(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "External comment author", "private-author-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	comment := dbfx.Comment(t, issue, "Original human text", testutil.Cols{"author_id": external, "revision": int64(9007199254740993)})
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	read := collaborationContentRequest(http.MethodGet, issue, comment, testUserID, "", nil)
	var view service.CollaborationCommentView
	response := testutil.Call(t, testHandler.GetCollaborationComment, read).Want(http.StatusOK)
	response.JSON(&view)
	if view.Author == nil || !view.Author.External || view.Author.Name != "External comment author" || view.CanEdit || view.CommentRevision != 9007199254740993 {
		t.Fatalf("external author projection: %+v", view)
	}
	if response.Header().Get("Cache-Control") != "no-store" || read.Header.Get("X-Workspace-ID") != "" ||
		strings.Contains(response.Body.String(), "private-author-") || strings.Contains(response.Body.String(), `"email"`) || strings.Contains(response.Body.String(), `"username"`) {
		t.Fatal("comment read leaked a private identity or ambient workspace")
	}
	body := map[string]any{"content": "Edited human text", "expected_comment_revision": "9007199254740993"}
	key := uuid.NewString()
	var receipt service.CollaborationCommentReceipt
	testutil.Call(t, testHandler.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, comment, external, key, body)).Want(http.StatusOK).JSON(&receipt)
	if receipt.CommentRevision != 9007199254740994 || len(receipt.Targets) != 0 || receipt.EventSeq < 1 {
		t.Fatalf("comment edit lost exact identity or started a target: %+v", receipt)
	}
	testutil.Call(t, testHandler.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, comment, external, key, body)).Want(http.StatusOK)
	deletion := map[string]any{"expected_comment_revision": "9007199254740994"}
	testutil.Call(t, testHandler.DeleteCollaborationComment, collaborationContentRequest(http.MethodDelete, issue, comment, external, uuid.NewString(), deletion)).Want(http.StatusOK)
	testutil.Call(t, testHandler.GetCollaborationComment, collaborationContentRequest(http.MethodGet, issue, comment, external, "", nil)).Want(http.StatusOK).JSON(&view)
	if view.DeletedAt == nil || view.Content != "" || view.CommentRevision != 9007199254740995 {
		t.Fatalf("deleted comment did not keep its identity/tombstone: %+v", view)
	}
	var history struct {
		Items []service.CollaborationCommentHistoryEntry `json:"items"`
	}
	testutil.Call(t, testHandler.GetCollaborationCommentHistory, collaborationContentRequest(http.MethodGet, issue, comment, external, "", nil)).Want(http.StatusOK).JSON(&history)
	if len(history.Items) != 2 || history.Items[0].Before == nil || *history.Items[0].Before != "Original human text" {
		t.Fatalf("comment history missing: %+v", history)
	}
}

func TestCollaborationCommentHTTPRejectsForeignCommentsAndMalformedVersions(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	other := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, other, "foreign private body")
	testutil.Call(t, testHandler.GetCollaborationComment, collaborationContentRequest(http.MethodGet, issue, comment, testUserID, "", nil)).Want(http.StatusNotFound)
	for _, raw := range []string{
		`{"content":"edit","expected_comment_revision":1}`,
		`{"content":"edit","expected_comment_revision":"1e3"}`,
		`{"content":"edit","expected_comment_revision":"01"}`,
		`{"content":"edit","content":"replacement","expected_comment_revision":"1"}`,
	} {
		testutil.Call(t, testHandler.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, comment, testUserID, uuid.NewString(), json.RawMessage(raw))).Want(http.StatusBadRequest)
	}
	machine := collaborationContentRequest(http.MethodGet, other, comment, testUserID, "", nil)
	machine = machine.WithContext(auth.WithAuthenticatedPrincipal(machine.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.GetCollaborationComment, machine).Want(http.StatusNotFound)
}

func TestCollaborationCommentHTTPPaginationHasStableChronologicalPages(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	for i := 0; i < 3; i++ {
		dbfx.Comment(t, issue, "Page comment")
	}
	seen := make(map[string]bool)
	cursor := ""
	for pageIndex := 0; pageIndex < 4; pageIndex++ {
		r := collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil)
		r.URL.RawQuery = "limit=1&cursor=" + cursor
		var page struct {
			Items      []service.CollaborationCommentView `json:"items"`
			NextCursor *string                            `json:"next_cursor"`
		}
		testutil.Call(t, testHandler.ListCollaborationComments, r).Want(http.StatusOK).JSON(&page)
		for _, item := range page.Items {
			id := uuidToString(item.ID)
			if seen[id] {
				t.Fatal("comment pagination repeated an item")
			}
			seen[id] = true
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 3 {
		t.Fatalf("comment pagination lost entries: %d", len(seen))
	}
}

func TestCollaborationCommentHTTPCreateKeepsOneCommentAndLocatedNotification(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "External commenter", "external-comment-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": external}, "user_id=$1", external)
	for _, table := range []string{"comment", "collaboration_event", "collaboration_notification"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	content := "[@creator](mention://member/" + testUserID + ") [@unknown](mention://agent/" + uuid.NewString() + ")\x00"
	body := map[string]any{"content": content, "parent_id": nil, "suppressed_target_ids": []string{}, "target_versions": []any{},
		"expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1"}
	var preview service.CollaborationCommentPreview
	testutil.Call(t, testHandler.PreviewCollaborationComment, collaborationContentRequest(http.MethodPost, issue, "", external, "", map[string]any{"content": content})).Want(http.StatusOK).JSON(&preview)
	if len(preview.Recipients) != 1 || !preview.Recipients[0].IsMention || len(preview.Targets) != 1 || preview.Targets[0].State != "blocked" ||
		dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1`, issue) != 0 {
		t.Fatalf("preview changed state or lost independent human recipient: %+v", preview)
	}
	key := uuid.NewString()
	var receipt service.CollaborationCommentReceipt
	r := collaborationContentRequest(http.MethodPost, issue, "", external, key, body)
	testutil.Call(t, testHandler.CreateCollaborationComment, r).Want(http.StatusCreated).JSON(&receipt)
	if !receipt.CommentID.Valid || receipt.CommentRevision != 1 || len(receipt.Targets) != 1 || receipt.Targets[0].State != "blocked" ||
		!receipt.EventID.Valid || receipt.EventSeq < 1 || r.Header.Get("X-Workspace-ID") != "" {
		t.Fatalf("create receipt lost frozen identity/outcome: %+v", receipt)
	}
	testutil.Call(t, testHandler.CreateCollaborationComment, collaborationContentRequest(http.MethodPost, issue, "", external, key, body)).Want(http.StatusCreated)
	if dbfx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND author_id=$2 AND content=$3 AND NOT dispatch_eligible`, issue, external, strings.ReplaceAll(content, "\x00", "")) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND recipient_user_id=$2 AND is_mention AND source_id=$3`, issue, testUserID, receipt.CommentID) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("create replay duplicated content/notification or entered ordinary execution")
	}
	unchanged := map[string]any{"content": strings.ReplaceAll(content, "\x00", ""), "expected_comment_revision": "1"}
	var noChange service.CollaborationCommentReceipt
	testutil.Call(t, testHandler.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, uuidToString(receipt.CommentID), external, uuid.NewString(), unchanged)).Want(http.StatusOK).JSON(&noChange)
	if noChange.Code != "unchanged" || noChange.CommentID != receipt.CommentID || noChange.CommentRevision != 1 || noChange.EventID.Valid || noChange.EventSeq != 0 || len(noChange.Targets) != 0 {
		t.Fatalf("no-op manufactured a new content event: %+v", noChange)
	}
}
