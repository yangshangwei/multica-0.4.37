package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
)

func newCollaborationAttachmentFixture(t *testing.T) (*collaborationFixture, string) {
	t.Helper()
	f := newCollaborationFixture(t)
	dir := t.TempDir()
	t.Setenv("LOCAL_UPLOAD_DIR", dir)
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	f.svc.Storage = storage.NewLocalStorageFromEnv()
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE source_workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id IN (SELECT id FROM collaboration_attachment WHERE source_workspace_id=$1)`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, f.workspace)
	return f, dir
}

func attachmentUploadParams(f *collaborationFixture) CollaborationAttachmentUpload {
	return CollaborationAttachmentUpload{DraftID: util.MustParseUUID(uuid.NewString()), SourceWorkspaceID: util.MustParseUUID(f.workspace),
		RequestID: uuid.NewString(), Name: "private.txt", ContentType: "text/plain; charset=utf-8", Data: []byte("private attachment sentinel")}
}

func TestCollaborationAttachmentCreationBindingRollsBackWithRequiredWrites(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := f.params(t)
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{attachment.ID}
	f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, eventFailure: true}
	if _, err = f.svc.Create(f.ctx, p); err == nil {
		t.Fatal("required event failure committed a partial attachment creation")
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id IS NULL AND delivered_at IS NULL`, attachment.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1`, attachment.ID) != 0 {
		t.Fatal("failed creation left an issue or consumed the private draft")
	}
	f.svc.TxStarter = f.pool
	created, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := f.svc.Create(f.ctx, p)
	if err != nil || replayed.Issue.ID != created.Issue.ID {
		t.Fatalf("creation with attachments was not replayable: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='issue' AND reference_id=$2`, attachment.ID, created.Issue.ID) != 1 {
		t.Fatal("creation did not record exactly one permanent delivery")
	}
	p.RequestID = uuid.NewString()
	if _, err = f.svc.Create(f.ctx, p); !collaborationErrorCode(err, "attachment_scope_mismatch") {
		t.Fatalf("new creation key consumed an already delivered draft: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 {
		t.Fatal("failed draft reuse left a second issue")
	}
}

func TestCollaborationAttachmentScopeCiphertextAndLiveAccess(t *testing.T) {
	f, dir := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.svc.GetAttachment(f.ctx, pgtype.UUID{}, upload.DraftID, attachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := os.ReadFile(filepath.Join(dir, row.StorageKey))
	if err != nil || bytes.Contains(ciphertext, upload.Data) || len(ciphertext) != len(upload.Data)+28 {
		t.Fatalf("storage did not keep authenticated ciphertext: %v", err)
	}
	for _, ctx := range []context.Context{collaborationContext(f.handler), auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})} {
		if _, err = f.svc.GetAttachment(ctx, pgtype.UUID{}, upload.DraftID, attachment.ID); err == nil {
			t.Fatal("draft was readable without its current human uploader authority")
		}
	}
	p := f.params(t)
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{attachment.ID}
	created, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.GetAttachment(f.ctx, other.Issue.ID, pgtype.UUID{}, attachment.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("another issue path resolved this attachment: %v", err)
	}
	external := collaborationContext(f.handler)
	row, err = f.svc.GetAttachment(external, created.Issue.ID, pgtype.UUID{}, attachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.ReadAttachmentBytes(external, row)
	if err != nil || !bytes.Equal(got, upload.Data) {
		t.Fatalf("authorized reader could not decrypt its delivered attachment: %v", err)
	}
	stamp, err := f.svc.MintAttachmentReadStamp(external, created.Issue.ID, attachment.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(), epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, created.Issue.ID, f.handler)
	if _, err = f.svc.CheckAttachmentReadStamp(external, created.Issue.ID, attachment.ID, 1, stamp); err == nil {
		t.Fatal("revoked reader retained a native capability")
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=NULL, epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, created.Issue.ID, f.handler)
	if _, err = f.svc.CheckAttachmentReadStamp(external, created.Issue.ID, attachment.ID, 1, stamp); err == nil {
		t.Fatal("re-enrollment revived an old native capability")
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	if _, err = f.svc.GetAttachment(f.ctx, created.Issue.ID, pgtype.UUID{}, attachment.ID); err != nil {
		t.Fatalf("valid participant lost attachment access with source membership: %v", err)
	}
	if err = f.svc.DiscardAttachmentDraft(f.ctx, upload.DraftID, attachment.ID); err == nil {
		t.Fatal("delivered attachment was deleted through draft cleanup")
	}
}

func TestCollaborationAttachmentUnknownCommitPreservesOriginalBytes(t *testing.T) {
	f, dir := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.UploadAttachmentDraft(f.ctx, upload); !errors.Is(err, errCollaborationAttachmentCommit) {
		t.Fatalf("expected unknown upload commit: %v", err)
	}
	recovered, err := f.svc.GetAttachmentUpload(f.ctx, upload.DraftID, upload.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.svc.GetAttachment(f.ctx, pgtype.UUID{}, upload.DraftID, recovered.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, row.StorageKey)); err != nil {
		t.Fatal("uncertain commit deleted its successfully persisted object:", err)
	}
	replay, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil || replay.ID != recovered.ID || fault.begins.Load() != 1 {
		t.Fatalf("upload replay created another attempt/transaction: %v", err)
	}
	data, err := f.svc.ReadAttachmentBytes(f.ctx, row)
	if err != nil || !bytes.Equal(data, upload.Data) {
		t.Fatalf("unknown-commit recovery lost original bytes: %v", err)
	}
	upload.Data = []byte("different payload")
	if _, err = f.svc.UploadAttachmentDraft(f.ctx, upload); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("upload changed an original request's bytes: %v", err)
	}
}

type collaborationUploadHookStorage struct {
	storage.Storage
	uploaded func()
}

func (s collaborationUploadHookStorage) Upload(ctx context.Context, key string, data []byte, contentType, name string) (string, error) {
	url, err := s.Storage.Upload(ctx, key, data, contentType, name)
	if err == nil {
		s.uploaded()
	}
	return url, err
}

func TestCollaborationAttachmentConcurrentUploadKeepsOnlyWinner(t *testing.T) {
	f, dir := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	arrived, release := make(chan struct{}, 2), make(chan struct{})
	f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() { arrived <- struct{}{}; <-release }}
	var wg sync.WaitGroup
	results := make([]CollaborationAttachmentView, 2)
	errors := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results[i], errors[i] = f.svc.UploadAttachmentDraft(f.ctx, upload) }()
	}
	<-arrived
	<-arrived
	close(release)
	wg.Wait()
	if errors[0] != nil || errors[1] != nil || results[0].ID != results[1].ID {
		t.Fatalf("concurrent original-key upload did not converge: %v", errors)
	}
	entries, err := os.ReadDir(filepath.Join(dir, storage.CollaborationPrivatePrefix))
	if err != nil || len(entries) != 1 || entries[0].Name() != util.UUIDToString(results[0].ID) {
		t.Fatalf("losing attempt survived or deleted winner: %v %v", entries, err)
	}
	row, err := f.svc.GetAttachment(f.ctx, pgtype.UUID{}, upload.DraftID, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := f.svc.ReadAttachmentBytes(f.ctx, row)
	if err != nil || !bytes.Equal(data, upload.Data) {
		t.Fatalf("winner ciphertext/key were mismatched: %v", err)
	}
}

func TestCollaborationAttachmentUploadRechecksAuthorityAfterStorage(t *testing.T) {
	f, dir := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() {
		f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	}}
	if _, err := f.svc.UploadAttachmentDraft(f.ctx, upload); err == nil {
		t.Fatal("lost source membership still committed an upload")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE source_workspace_id=$1`, f.workspace) != 0 {
		t.Fatal("failed authorization left private draft metadata")
	}
	entries, err := os.ReadDir(filepath.Join(dir, storage.CollaborationPrivatePrefix))
	if err != nil || len(entries) != 0 {
		t.Fatalf("known rollback did not clean its own ciphertext: %v %v", entries, err)
	}
}

func TestCollaborationAttachmentCommandSnapshotOutlivesCommentDeletion(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"collaboration_channel", "collaboration_batch", "collaboration_command"} {
		f.fx.Cleanup(t, "DELETE FROM "+table+" WHERE session_id=$1", issue.SessionID)
	}
	content := &collaborationContentFixture{collaborationFixture: f, issue: issue}
	target, _ := seedCollaborationTarget(t, f.fx, issue.Issue.ID, issue.SessionID, util.MustParseUUID(f.creator), 1)
	upload := attachmentUploadParams(f)
	upload.SourceWorkspaceID, upload.IssueID = pgtype.UUID{}, issue.Issue.ID
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := content.input("[@A](mention://agent/"+util.UUIDToString(target.ResourceID)+") use the attached file", target.ID)
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{attachment.ID}
	comment, err := f.svc.CreateComment(f.ctx, p)
	if err != nil || len(comment.Targets) != 1 || !comment.Targets[0].CommandID.Valid {
		t.Fatalf("attachment comment did not create an immutable input: %+v %v", comment, err)
	}
	second := content.input("Separate display reference")
	second.AttachmentIDs = []pgtype.UUID{attachment.ID}
	if _, err = f.svc.CreateComment(collaborationContext(f.handler), second); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DeleteComment(f.ctx, CollaborationEditCommentParams{IssueID: issue.Issue.ID, CommentID: comment.CommentID,
		RequestID: uuid.NewString(), ExpectedCommentRevision: 1}); err != nil {
		t.Fatal(err)
	}
	var snapshot []byte
	f.fx.QueryRow(t, `SELECT attachments FROM collaboration_command WHERE id=$1`, comment.Targets[0].CommandID).Scan(&snapshot)
	var ids []pgtype.UUID
	if json.Unmarshal(snapshot, &ids) != nil || len(ids) != 1 || ids[0] != attachment.ID {
		t.Fatalf("comment deletion rewrote command attachment snapshot: %s", snapshot)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='command' AND removed_at IS NULL`, attachment.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='comment' AND removed_at IS NULL`, attachment.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='comment' AND removed_at IS NOT NULL`, attachment.ID) != 1 {
		t.Fatal("comment deletion erased another comment, command or historical reference")
	}
	history, err := f.svc.CommentHistory(f.ctx, issue.Issue.ID, comment.CommentID)
	if err != nil || len(history) != 2 || len(history[1].BeforeAttachmentIDs) != 1 || history[1].BeforeAttachmentIDs[0] != attachment.ID {
		t.Fatalf("comment history lost delivered attachment: %+v %v", history, err)
	}
	conflict := CollaborationSubmitCommandParams{CollaborationQueueRequest: p.CollaborationQueueRequest, TargetID: target.ID, InputEventID: comment.EventID,
		ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: p.Content}
	conflict.RequestID = uuid.NewString()
	if _, err = f.svc.SubmitCommand(f.ctx, conflict); !collaborationErrorCode(err, "input_event_conflict") {
		t.Fatalf("same input event discarded its immutable attachments: %v", err)
	}
}

func TestCollaborationAttachmentSourceOnlyCapabilityTracksMembershipIdentity(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	attachment, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := f.params(t)
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{attachment.ID}
	issue, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Member(t, f.workspace, f.outsider, "member")
	reader := collaborationContext(f.outsider)
	stamp, err := f.svc.MintAttachmentReadStamp(reader, issue.Issue.ID, attachment.ID)
	if err != nil || stamp.ParticipantEpoch != 0 || !stamp.MemberID.Valid || stamp.AccountEpoch != 1 {
		t.Fatalf("source-only capability lost its real membership basis: %+v %v", stamp, err)
	}
	if _, err = f.svc.CheckAttachmentReadStamp(reader, issue.Issue.ID, attachment.ID, 1, stamp); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.outsider)
	f.fx.Member(t, f.workspace, f.outsider, "member")
	if _, err = f.svc.CheckAttachmentReadStamp(reader, issue.Issue.ID, attachment.ID, 1, stamp); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("source rejoin revived previous membership's capability: %v", err)
	}
	if _, err = f.svc.GetAttachment(reader, issue.Issue.ID, pgtype.UUID{}, attachment.ID); err != nil {
		t.Fatalf("new membership did not retain legitimate direct access: %v", err)
	}
}
