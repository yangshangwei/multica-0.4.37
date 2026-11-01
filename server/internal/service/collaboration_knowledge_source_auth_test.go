package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestKnowledgeSourceReadersRevalidateCachedPATForMemoryCommentsAndRecords(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "PRIVATE_SOURCE_RECORD_INPUT")
	f.resumeManaged(t)
	run, _, _ := f.claimManaged(t)
	if _, err := f.svc.UpdateMemory(f.ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), Content: "PRIVATE_SOURCE_MEMORY"}); err != nil {
		t.Fatal(err)
	}
	comment := f.fx.Insert(t, "comment", testutil.Cols{"workspace_id": f.workspace, "issue_id": f.issue.Issue.ID, "author_type": "member", "author_id": f.creator, "type": "comment", "content": "PRIVATE_SOURCE_COMMENT"})
	ctx, tokenID, _ := knowledgePATFixture(t, f.collaborationQueueFixture)
	reads := map[string]func() error{
		"memory": func() error {
			out, err := f.svc.GetMemory(ctx, f.issue.Issue.ID)
			if err == nil && out.Content != "PRIVATE_SOURCE_MEMORY" {
				t.Error("memory positive missing source")
			}
			return err
		},
		"comment": func() error {
			out, err := f.svc.GetComment(ctx, f.issue.Issue.ID, util.MustParseUUID(comment))
			if err == nil && out.Content != "PRIVATE_SOURCE_COMMENT" {
				t.Error("comment positive missing source")
			}
			return err
		},
		"records": func() error {
			out, err := f.svc.RunRecords(ctx, f.issue.Issue.ID, run, CollaborationRunRecordsQuery{Limit: 10})
			if err == nil && (len(out.Items) == 0 || out.Items[0].Body != "PRIVATE_SOURCE_RECORD_INPUT") {
				t.Error("record positive missing source")
			}
			return err
		},
	}
	for name, read := range reads {
		if err := read(); err != nil {
			t.Fatalf("live PAT %s read: %v", name, err)
		}
	}
	// Reuse the exact already-verified context, as a stale Auth cache would.
	f.fx.Exec(t, `UPDATE personal_access_token SET revoked=true WHERE id=$1`, tokenID)
	for name, read := range reads {
		if err := read(); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("revoked cached PAT %s source read: %v", name, err)
		}
	}
	f.fx.Exec(t, `UPDATE personal_access_token SET revoked=false,expires_at=now()-interval '1 second' WHERE id=$1`, tokenID)
	for name, read := range reads {
		if err := read(); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("expired cached PAT %s source read: %v", name, err)
		}
	}
	if _, err := f.svc.GetMemory(f.ctx, f.issue.Issue.ID); err != nil {
		t.Fatal("independent JWT source read changed", err)
	}
}
