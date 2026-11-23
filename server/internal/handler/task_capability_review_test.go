package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// An allowed target issue may carry a deliberately frozen source snapshot.
// Reading it must not grant ongoing access to changes on a different issue.
func TestTaskCapabilityReviewSourceContextDoesNotReadOutsideScope(t *testing.T) {
	for _, tc := range []struct {
		name, role               string
		sameProject, liveAllowed bool
	}{
		{"reviewer_other_issue", "code-reviewer", true, false},
		{"reporter_other_project", "progress-reporter", false, false},
		{"reporter_same_project", "progress-reporter", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTaskCapabilityFixture(t, tc.role, 2, false)
			project := f.project
			if !tc.sameProject {
				project = dbfx.Project(t, "Outside source project")
			}
			source := dbfx.Issue(t, "Supplied frozen source", testutil.Cols{"project_id": project})
			anchor := dbfx.Insert(t, "comment", testutil.Cols{"issue_id": source, "workspace_id": testWorkspaceID, "author_type": "member", "author_id": testUserID, "type": "comment", "content": "Supplied frozen comment"})
			build, err := service.BuildSourceContext(context.Background(), testHandler.Queries, parseUUID(testWorkspaceID), parseUUID(anchor))
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := json.Marshal(build.Snapshot)
			if err != nil {
				t.Fatal(err)
			}
			dbfx.Insert(t, "issue_source_context", testutil.Cols{"id": uuid.NewString(), "workspace_id": testWorkspaceID, "issue_id": f.issue, "source_issue_id": source, "anchor_comment_id": anchor, "captured_by_user_id": testUserID, "snapshot_version": 1, "snapshot": snapshot, "capture_digest": build.Digest, "state": "attached", "attached_at": testutil.Raw("now()")})

			// This filename did not exist in the captured input. A later source
			// attachment edit is a new read, not part of the target's snapshot.
			const liveSecret = "Unshared-confidential-source-attachment"
			attachment := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "issue_id": source, "uploader_type": "member", "uploader_id": testUserID, "filename": liveSecret, "url": "https://invalid.example/new-source-attachment", "content_type": "text/plain", "size_bytes": 4})
			dbfx.Exec(t, `UPDATE issue SET description=$2 WHERE id=$1`, source, "["+liveSecret+"](/api/attachments/"+attachment+"/download)")
			response := capabilityCall(t, f.token, "GET", "/api/issues/"+f.issue, nil).Want(200)
			body := response.Body.String()
			if !strings.Contains(body, "Supplied frozen comment") {
				t.Fatal("authorized frozen snapshot was lost")
			}
			if strings.Contains(body, liveSecret) != tc.liveAllowed {
				t.Fatalf("live source attachment disclosure = %v, allowed = %v", strings.Contains(body, liveSecret), tc.liveAllowed)
			}
		})
	}
}
