package handler

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationDiscussionHTTPUsesPrivateRoutesAndOwnReceipts(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, issue, "Protected discussion")
	dbfx.Cleanup(t, `DELETE FROM comment_reaction WHERE comment_id=$1`, comment)
	dbfx.Cleanup(t, `DELETE FROM issue_reaction WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_id=$1`, issue)
	key := uuid.NewString()
	r := withURLParams(collaborationContentRequest(http.MethodPost, issue, comment, testUserID, key, nil), "id", issue, "commentId", comment, "emoji", "👍")
	var receipt service.CollaborationDiscussionReceipt
	response := testutil.Call(t, testHandler.AddCollaborationCommentReaction, r).Want(http.StatusOK)
	response.JSON(&receipt)
	if receipt.RequestID != key || receipt.Action != "add_reaction" || receipt.Emoji == nil || *receipt.Emoji != "👍" || receipt.Code != "reaction_added" || response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" {
		t.Fatalf("wrong private action result: %+v", receipt)
	}
	read := withURLParams(collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil), "id", issue, "requestId", key)
	var replay service.CollaborationDiscussionReceipt
	testutil.Call(t, testHandler.GetCollaborationDiscussionOperation, read).Want(http.StatusOK).JSON(&replay)
	if !reflect.DeepEqual(replay, receipt) {
		t.Fatalf("GET changed original result: %+v", replay)
	}
	other := collaborationBoundaryIssue(t, testWorkspaceID)
	foreign := withURLParams(collaborationContentRequest(http.MethodPost, other, comment, testUserID, uuid.NewString(), nil), "id", other, "commentId", comment, "emoji", "👍")
	testutil.Call(t, testHandler.AddCollaborationCommentReaction, foreign).Want(http.StatusNotFound)
	machine := collaborationContentRequest(http.MethodPost, issue, comment, testUserID, uuid.NewString(), nil)
	machine = machine.WithContext(auth.WithAuthenticatedPrincipal(machine.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}))
	testutil.Call(t, testHandler.ResolveCollaborationComment, machine).Want(http.StatusForbidden)
	if dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("discussion action generated a source inbox item")
	}
}

func TestCollaborationDiscussionLegacyPathsCannotMutateOrNotify(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	comment := dbfx.Comment(t, issue, "Retained text")
	for _, tc := range []struct {
		name, method string
		handler      http.HandlerFunc
	}{
		{"resolve", http.MethodPost, testHandler.ResolveComment},
		{"unresolve", http.MethodPost, testHandler.UnresolveComment},
		{"add_comment_reaction", http.MethodPost, testHandler.AddReaction},
		{"remove_comment_reaction", http.MethodDelete, testHandler.RemoveReaction},
		{"add_issue_reaction", http.MethodPost, testHandler.AddIssueReaction},
		{"remove_issue_reaction", http.MethodDelete, testHandler.RemoveIssueReaction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := withURLParams(newRequest(tc.method, "/api/issues/"+issue+"/comments/"+comment, map[string]string{"emoji": "👍"}), "id", issue, "commentId", comment)
			testutil.Call(t, tc.handler, r).Want(http.StatusConflict)
		})
	}
	if dbfx.Count(t, `SELECT count(*) FROM comment_reaction WHERE comment_id=$1`, comment) != 0 ||
		dbfx.Count(t, `SELECT count(*) FROM issue_reaction WHERE issue_id=$1`, issue) != 0 ||
		dbfx.Count(t, `SELECT revision FROM comment WHERE id=$1`, comment) != 1 ||
		dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("legacy discussion path bypassed the dedicated mutation")
	}
}
