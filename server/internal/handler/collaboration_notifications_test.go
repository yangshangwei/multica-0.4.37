package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationNotificationHTTPRechecksRecipientAndObjectAccess(t *testing.T) {
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Notification reader", "reader-"+uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	comment := dbfx.Comment(t, issue, "private comment sentinel")
	notification := dbfx.Insert(t, "collaboration_notification", testutil.Cols{"recipient_user_id": external, "issue_id": issue, "event_seq": 1,
		"kind": "comment_created", "is_mention": true, "source_kind": "comment", "source_id": comment})
	request := func(actor, action string) *http.Request {
		r := newRequest(http.MethodGet, "/api/me/collaboration-notifications", nil)
		if action != "" {
			r.Method = http.MethodPost
			r = withURLParam(r, "notificationId", notification)
		}
		r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
		return r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
	}
	var page struct {
		Items       []service.CollaborationNotificationView `json:"items"`
		UnreadCount int64                                   `json:"unread_count"`
	}
	testutil.Call(t, testHandler.ListCollaborationNotifications, request(external, "")).Want(http.StatusOK).JSON(&page)
	if len(page.Items) != 1 || !page.Items[0].CanOpen || uuidToString(page.Items[0].SourceID) != comment || page.UnreadCount != 1 {
		t.Fatalf("notification cannot locate the original comment: %+v", page)
	}
	testutil.Call(t, testHandler.ReadCollaborationNotification, request(testUserID, "read")).Want(http.StatusNotFound)
	read := testutil.Call(t, testHandler.ReadCollaborationNotification, request(external, "read")).Want(http.StatusOK)
	if strings.Contains(read.Body.String(), "issue_id") || strings.Contains(read.Body.String(), "private") || strings.Contains(read.Body.String(), "source_id") {
		t.Fatal("notification mutation returned private row content")
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, external)
	testutil.Call(t, testHandler.ListCollaborationNotifications, request(external, "")).Want(http.StatusOK).JSON(&page)
	if page.Items[0].CanOpen || page.Items[0].IssueTitle != nil || page.Items[0].SourceID.Valid {
		t.Fatalf("old notification restored access: %+v", page.Items[0])
	}
	testutil.Call(t, testHandler.ArchiveCollaborationNotification, request(external, "archive")).Want(http.StatusOK)
	testutil.Call(t, testHandler.ListCollaborationNotifications, request(external, "")).Want(http.StatusOK).JSON(&page)
	if len(page.Items) != 0 || page.UnreadCount != 0 {
		t.Fatalf("archived notification remains unread: %+v", page)
	}
}
