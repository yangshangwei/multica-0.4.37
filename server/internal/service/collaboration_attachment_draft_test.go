package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationAttachmentDraftCanCreateOnlyOneIssue(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	first, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	secondUpload := upload
	secondUpload.RequestID = uuid.NewString()
	second, err := f.svc.UploadAttachmentDraft(f.ctx, secondUpload)
	if err != nil {
		t.Fatal(err)
	}
	create := f.params(t)
	create.DraftID, create.AttachmentIDs = &upload.DraftID, []pgtype.UUID{first.ID}
	created, err := f.svc.Create(f.ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	later := f.params(t)
	later.DraftID, later.AttachmentIDs = &upload.DraftID, []pgtype.UUID{second.ID}
	if _, err = f.svc.Create(f.ctx, later); !collaborationErrorCode(err, "attachment_draft_consumed") {
		t.Fatalf("another creation consumed the same upload draft: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id IS NULL`, second.ID) != 1 {
		t.Fatal("a refused second draft consumption left a partial issue or binding")
	}
	replayed, err := f.svc.Create(f.ctx, create)
	if err != nil || replayed.Issue.ID != created.Issue.ID {
		t.Fatalf("consuming a draft broke original creation recovery: %v", err)
	}
	recovered, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil || recovered.ID != first.ID || recovered.IssueID != created.Issue.ID {
		t.Fatalf("consuming a draft broke original upload recovery: %v", err)
	}
	newUpload := upload
	newUpload.RequestID = uuid.NewString()
	if _, err = f.svc.UploadAttachmentDraft(f.ctx, newUpload); !collaborationErrorCode(err, "attachment_draft_consumed") {
		t.Fatalf("new upload extended a consumed draft: %v", err)
	}
	content := &collaborationContentFixture{collaborationFixture: f, issue: created}
	comment := content.input("An independent reference to a delivered attachment")
	comment.AttachmentIDs = []pgtype.UUID{first.ID}
	if _, err = f.svc.CreateComment(f.ctx, comment); err != nil {
		t.Fatalf("draft consumption blocked a same-issue reference without a draft: %v", err)
	}
}

func TestCollaborationAttachmentDraftConcurrentCreationsChooseOneIssue(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	var creates [2]CollaborationCreateParams
	for i := range creates {
		upload.RequestID = uuid.NewString()
		file, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
		if err != nil {
			t.Fatal(err)
		}
		creates[i] = f.params(t)
		creates[i].DraftID, creates[i].AttachmentIDs = &upload.DraftID, []pgtype.UUID{file.ID}
	}
	start := make(chan struct{})
	var results [2]error
	var wg sync.WaitGroup
	for i := range creates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, results[i] = f.svc.Create(f.ctx, creates[i])
		}()
	}
	close(start)
	wg.Wait()
	succeeded, refused := 0, 0
	for _, err := range results {
		if err == nil {
			succeeded++
		} else if collaborationErrorCode(err, "attachment_draft_consumed") {
			refused++
		} else {
			t.Fatalf("unexpected concurrent draft result: %v", err)
		}
	}
	if succeeded != 1 || refused != 1 || f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(DISTINCT issue_id) FROM collaboration_attachment WHERE draft_id=$1`, upload.DraftID) != 1 {
		t.Fatalf("one draft created multiple issues: successes=%d refusals=%d", succeeded, refused)
	}
}

func TestCollaborationAttachmentDraftConsumptionWinsAgainstPreparedUpload(t *testing.T) {
	f, dir := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	first, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	create := f.params(t)
	create.DraftID, create.AttachmentIDs = &upload.DraftID, []pgtype.UUID{first.ID}
	f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() {
		if _, err := f.svc.Create(f.ctx, create); err != nil {
			t.Fatal(err)
		}
	}}
	upload.RequestID = uuid.NewString()
	if _, err = f.svc.UploadAttachmentDraft(f.ctx, upload); !collaborationErrorCode(err, "attachment_draft_consumed") {
		t.Fatalf("an upload prepared before consumption committed afterward: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, storage.CollaborationPrivatePrefix))
	if err != nil || len(entries) != 1 || entries[0].Name() != util.UUIDToString(first.ID) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE draft_id=$1`, upload.DraftID) != 1 {
		t.Fatalf("losing prepared upload survived or deleted the delivered winner: %v", err)
	}
}

func TestCollaborationAttachmentDraftContextSurvivesDiscard(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	firstIssue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	secondIssue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	upload := attachmentUploadParams(f)
	upload.SourceWorkspaceID, upload.IssueID = pgtype.UUID{}, firstIssue.Issue.ID
	file, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.svc.DiscardAttachmentDraft(f.ctx, upload.DraftID, file.ID); err != nil {
		t.Fatal(err)
	}
	upload.RequestID, upload.IssueID = uuid.NewString(), secondIssue.Issue.ID
	if _, err = f.svc.UploadAttachmentDraft(f.ctx, upload); !collaborationErrorCode(err, "attachment_scope_mismatch") {
		t.Fatalf("a draft's intended issue changed after discarding its files: %v", err)
	}
	upload.IssueID = firstIssue.Issue.ID
	if _, err = f.svc.UploadAttachmentDraft(f.ctx, upload); err != nil {
		t.Fatalf("a new upload in an unconsumed original context failed: %v", err)
	}
}

func TestCollaborationAttachmentDraftMetadataIncludesIntendedIssue(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, intended := range []pgtype.UUID{{}, issue.Issue.ID} {
		upload := attachmentUploadParams(f)
		if intended.Valid {
			upload.SourceWorkspaceID, upload.IssueID = pgtype.UUID{}, intended
		}
		file, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(file)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err = json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		value, exists := fields["intended_issue_id"]
		if !exists || intended.Valid && value != util.UUIDToString(intended) || !intended.Valid && value != nil {
			t.Fatalf("draft metadata omitted or changed the intended issue: %v", value)
		}
	}
}
