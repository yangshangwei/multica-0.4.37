package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Generic attachment contributions have independent uploader provenance. They
// do not grant another member permission to rewrite the comment's body/author.
// TestLegacyUploadCannotAttachToCollaboration owns the private-flow matrix.
func TestUploadFileCommentMembershipContract(t *testing.T) {
	workspace := dbfx.Workspace(t, "Comment contributions", "comment-upload-"+uuid.NewString())
	fx := testutil.New(testPool, workspace, testUserID)
	fx.Member(t, workspace, testUserID, "owner")
	uploader := fx.User(t, "Member uploader", uuid.NewString()+"@example.test")
	fx.Member(t, workspace, uploader, "member")
	outsider := fx.User(t, "Nonmember", uuid.NewString()+"@example.test")
	issue := fx.Issue(t, "Attachment contributions")
	foreignWorkspace := fx.Workspace(t, "Other contribution scope", "comment-upload-other-"+uuid.NewString())
	foreign := testutil.New(testPool, foreignWorkspace, testUserID)
	foreignIssue := foreign.Issue(t, "Foreign comment")

	for _, scenario := range []struct {
		name    string
		user    string
		fixture *testutil.Fixture
		issue   string
		status  int
	}{
		{"same_workspace_member", uploader, fx, issue, http.StatusOK},
		{"nonmember", outsider, fx, issue, http.StatusForbidden},
		{"foreign_comment", uploader, foreign, foreignIssue, http.StatusForbidden},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			comment := scenario.fixture.Comment(t, scenario.issue, "Original author's body")
			storage := &mockStorage{}
			h := *testHandler
			h.Storage, h.Bus = storage, events.New()
			published := 0
			h.Bus.SubscribeAll(func(events.Event) { published++ })
			var before string
			fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM comment c WHERE id=$1`, comment).Scan(&before)
			attachmentsBefore := fx.Count(t, `SELECT count(*) FROM attachment`)
			request := resourceMultipartRequest(t, "/api/upload-file", "", [][2]string{{"comment_id", comment}}, "A member's supporting file")
			request.Header.Del("Authorization")
			testutil.WithHeaders(request, "X-User-ID", scenario.user, "X-Workspace-ID", workspace)
			request = request.WithContext(auth.WithAuthenticatedPrincipal(request.Context(), auth.AuthenticatedPrincipal{UserID: scenario.user, Kind: auth.CredentialJWT}))
			response := testutil.Call(t, h.UploadFile, request).Want(scenario.status)
			if scenario.status != http.StatusOK {
				var after string
				fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM comment c WHERE id=$1`, comment).Scan(&after)
				if after != before || fx.Count(t, `SELECT count(*) FROM attachment`) != attachmentsBefore || len(storage.files) != 0 || published != 0 {
					t.Fatal("denied comment upload changed comment, attachment rows, storage, or events")
				}
				return
			}
			var attachment AttachmentResponse
			response.JSON(&attachment)
			fx.Cleanup(t, `DELETE FROM attachment WHERE comment_id=$1`, comment)
			if attachment.ID == "" || attachment.CommentID == nil || *attachment.CommentID != comment || attachment.WorkspaceID != workspace || attachment.UploaderType != "member" || attachment.UploaderID != uploader {
				t.Fatalf("upload lost the actual member's independent provenance: %+v", attachment)
			}
			if fx.Count(t, `SELECT count(*) FROM attachment WHERE id=$1 AND comment_id=$2 AND workspace_id=$3 AND uploader_type='member' AND uploader_id=$4`, attachment.ID, comment, workspace, uploader) != 1 || len(storage.files) != 1 {
				t.Fatal("successful member contribution was not stored with its own attribution")
			}
			if fx.Count(t, `SELECT count(*) FROM comment WHERE id=$1 AND author_type='member' AND author_id=$2 AND content=$3`, comment, testUserID, "Original author's body") != 1 {
				t.Fatal("member attachment contribution rewrote the original comment author or body")
			}
		})
	}
}
