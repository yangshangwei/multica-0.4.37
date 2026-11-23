package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func collaborationFileHandler(t *testing.T) *Handler {
	t.Helper()
	h := *testHandler
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	h.Storage = storage.NewLocalStorageFromEnv()
	return &h
}

func collaborationFileRequest(method, issue, draft, attachment, actor string, body any) *http.Request {
	r := testutil.JSONRequest(method, "/api/collaboration/issues/"+issue+"/attachments/"+attachment, body)
	r = withURLParams(r, "id", issue, "draftId", draft, "attachmentId", attachment)
	r.Header.Set("Idempotency-Key", uuid.NewString())
	r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
	if actor != "" {
		r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT}))
	}
	return r
}

func collaborationUploadedFile(t *testing.T, h *Handler, issue, actor string) (string, service.CollaborationAttachmentView) {
	t.Helper()
	draft := uuid.NewString()
	ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: actor, Kind: auth.CredentialJWT})
	upload, err := h.collaborationAttachmentService().UploadAttachmentDraft(ctx, service.CollaborationAttachmentUpload{
		DraftID: parseUUID(draft), IssueID: parseUUID(issue), RequestID: uuid.NewString(), Name: "private.txt", ContentType: "text/plain",
		Data: []byte("controlled private bytes"),
	})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, upload.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, upload.ID)
	return draft, upload
}

func TestCollaborationAttachmentHTTPUsesScopedUploadAndRevocableNativeBytes(t *testing.T) {
	h := collaborationFileHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Attachment reader", uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=$1`, external)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	draft := uuid.NewString()
	var body bytes.Buffer
	multi := multipart.NewWriter(&body)
	if err := multi.WriteField("draft_id", draft); err != nil {
		t.Fatal(err)
	}
	if err := multi.WriteField("issue_id", issue); err != nil {
		t.Fatal(err)
	}
	file, err := multi.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("controlled private bytes")); err != nil {
		t.Fatal(err)
	}
	if err = multi.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "/api/collaboration/attachments/drafts", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", multi.FormDataContentType())
	req.Header.Set("Idempotency-Key", uuid.NewString())
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	var uploaded service.CollaborationAttachmentView
	response := testutil.Call(t, h.UploadCollaborationAttachmentDraft, req).Want(http.StatusCreated)
	response.JSON(&uploaded)
	if uploaded.IntendedIssueID != parseUUID(issue) {
		t.Fatal("upload metadata lost its intended issue before delivery")
	}
	if uploaded.IssueID.Valid || uploaded.DeliveredAt != nil || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(response.Text(), "encryption_key") || strings.Contains(response.Text(), "storage_key") || strings.Contains(response.Text(), "/uploads/") {
		t.Fatal("private upload exposed a backend capability or became delivered early")
	}
	id := uuidToString(uploaded.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, uploaded.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, uploaded.ID)
	testutil.Call(t, h.GetCollaborationAttachmentContent, collaborationFileRequest(http.MethodGet, "", draft, id, external, nil)).Want(http.StatusNotFound)
	testutil.Call(t, h.GetCollaborationAttachmentContent, collaborationFileRequest(http.MethodGet, "", draft, id, testUserID, nil)).Want(http.StatusOK)
	testutil.Call(t, h.DeliverCollaborationAttachments, collaborationFileRequest(http.MethodPost, issue, "", "", testUserID,
		map[string]any{"draft_id": draft, "attachment_ids": []string{id}})).Want(http.StatusOK)
	get := collaborationFileRequest(http.MethodGet, issue, "", id, external, nil)
	get.Header.Set("Range", "bytes=0-9")
	partial := testutil.Call(t, h.DownloadCollaborationAttachment, get).Want(http.StatusPartialContent)
	if partial.Text() != "controlled" || partial.Header().Get("Content-Range") != "bytes 0-9/24" || partial.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authorized Range returned incorrect bytes/headers: %q %v", partial.Text(), partial.Header())
	}
	var cap struct {
		URL string `json:"url"`
	}
	testutil.Call(t, h.CreateCollaborationAttachmentCapability, collaborationFileRequest(http.MethodPost, issue, "", id, external,
		map[string]string{"intent": "download"})).Want(http.StatusOK).JSON(&cap)
	parsed, err := url.Parse(cap.URL)
	if err != nil || parsed.IsAbs() || !strings.HasPrefix(parsed.Path, "/api/collaboration/issues/") {
		t.Fatalf("native download escaped the controlled proxy: %q", cap.URL)
	}
	native := func() *http.Request {
		r := collaborationFileRequest(http.MethodGet, issue, "", id, "", nil)
		r.URL = parsed
		return r
	}
	result := testutil.Call(t, h.ReadCollaborationAttachmentCapability, native()).Want(http.StatusOK)
	if result.Text() != "controlled private bytes" || result.Header().Get("Location") != "" || result.Header().Get("Referrer-Policy") != "no-referrer" || !strings.Contains(result.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("native link did not proxy sandboxed private bytes")
	}
	dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, external)
	testutil.Call(t, h.ReadCollaborationAttachmentCapability, native()).Want(http.StatusNotFound)
	testutil.Call(t, h.GetCollaborationAttachment, collaborationFileRequest(http.MethodGet, issue, "", id, external, nil)).Want(http.StatusNotFound)
	testutil.Call(t, h.GetCollaborationAttachment, collaborationFileRequest(http.MethodGet, issue, "", id, testUserID, nil)).Want(http.StatusOK)
}

func TestCollaborationCommentAttachmentDisplayRemovalKeepsDeliveredFile(t *testing.T) {
	h := collaborationFileHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	draft, uploaded := collaborationUploadedFile(t, h, issue, testUserID)
	for _, table := range []string{"comment", "collaboration_event", "collaboration_notification"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	var receipt service.CollaborationCommentReceipt
	body := map[string]any{"content": "First delivery", "expected_lifecycle_epoch": "1", "draft_id": draft, "attachment_ids": []string{uuidToString(uploaded.ID)}}
	testutil.Call(t, h.CreateCollaborationComment, collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), body)).Want(http.StatusCreated).JSON(&receipt)
	comment := uuidToString(receipt.CommentID)
	var view map[string]any
	testutil.Call(t, h.GetCollaborationComment, collaborationContentRequest(http.MethodGet, issue, comment, testUserID, "", nil)).Want(http.StatusOK).JSON(&view)
	ids, ok := view["attachment_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0] != uuidToString(uploaded.ID) {
		t.Fatalf("comment did not expose its current attachment reference: %v", view)
	}
	testutil.Call(t, h.EditCollaborationComment, collaborationContentRequest(http.MethodPatch, issue, comment, testUserID, uuid.NewString(),
		map[string]any{"content": "First delivery", "expected_comment_revision": "1", "attachment_ids": []string{}})).Want(http.StatusOK)
	testutil.Call(t, h.GetCollaborationComment, collaborationContentRequest(http.MethodGet, issue, comment, testUserID, "", nil)).Want(http.StatusOK).JSON(&view)
	ids, ok = view["attachment_ids"].([]any)
	if !ok || len(ids) != 0 || dbfx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1 AND reference_kind='comment' AND removed_at IS NOT NULL`, uploaded.ID) != 1 {
		t.Fatal("comment display removal did not retain a reference tombstone")
	}
	var list struct {
		Attachments []service.CollaborationAttachmentView `json:"attachments"`
	}
	testutil.Call(t, h.ListCollaborationAttachments, collaborationFileRequest(http.MethodGet, issue, "", "", testUserID, nil)).Want(http.StatusOK).JSON(&list)
	if len(list.Attachments) != 1 || list.Attachments[0].ID != uploaded.ID {
		t.Fatal("display removal erased permanent issue delivery")
	}
	testutil.Call(t, h.GetCollaborationAttachmentContent, collaborationFileRequest(http.MethodGet, issue, "", uuidToString(uploaded.ID), testUserID, nil)).Want(http.StatusOK)
	if _, err := h.collaborationAttachmentService().GetAttachment(context.Background(), parseUUID(issue), pgtype.UUID{}, uploaded.ID); err == nil {
		t.Fatal("retention restored access for an unauthenticated actor")
	}
}

func TestCollaborationAttachmentCapabilityRejectsTamperingAndInvalidSelections(t *testing.T) {
	h := collaborationFileHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	testutil.Call(t, h.ReadCollaborationAttachmentCapability, collaborationFileRequest(http.MethodGet, issue, "", uuid.NewString(), "", nil)).Want(http.StatusNotFound)
	for _, raw := range []string{`{"intent":"download","intent":"content"}`, `{"intent":"redirect"}`, `{"intent":"download","url":"https://example.test"}`} {
		testutil.Call(t, h.CreateCollaborationAttachmentCapability, collaborationFileRequest(http.MethodPost, issue, "", uuid.NewString(), testUserID, json.RawMessage(raw))).Want(http.StatusBadRequest)
	}
}

func TestCollaborationPrivateObjectsRejectOrdinaryFileCapabilities(t *testing.T) {
	h := collaborationFileHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	_, uploaded := collaborationUploadedFile(t, h, issue, testUserID)
	row, err := h.Queries.GetCollaborationAttachment(context.Background(), uploaded.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Even an ordinary attachment row that names this object is not a bridge
	// into the private namespace. Ordinary readers have no delivery authority.
	ordinary := dbfx.Insert(t, "attachment", testutil.Cols{"workspace_id": testWorkspaceID, "uploader_type": "member", "uploader_id": testUserID,
		"filename": "forged.txt", "url": h.Storage.ObjectURL(row.StorageKey), "content_type": "text/plain", "size_bytes": row.SizeBytes})
	for _, handler := range []http.HandlerFunc{h.GetAttachmentByID, h.GetAttachmentContent, h.DownloadAttachment} {
		r := withURLParam(newRequest(http.MethodGet, "/api/attachments/"+ordinary, nil), "id", ordinary)
		testutil.Call(t, handler, r).Want(http.StatusNotFound)
	}
	r := withURLParam(newRequest(http.MethodGet, attachmentCapabilityPath(ordinary, time.Now()), nil), "id", ordinary)
	testutil.Call(t, h.DownloadAttachmentWithCapability, r).Want(http.StatusNotFound)
	h.deleteS3Object(context.Background(), h.Storage.ObjectURL(row.StorageKey))
	h.deleteS3Objects(context.Background(), []string{h.Storage.ObjectURL(row.StorageKey)})
	reader, err := h.Storage.GetReader(context.Background(), row.StorageKey)
	if err != nil {
		t.Fatal("ordinary cleanup removed a retained collaboration object:", err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
}

type collaborationReadHookStorage struct {
	storage.Storage
	read func()
}

func (s collaborationReadHookStorage) GetReader(ctx context.Context, key string) (io.ReadCloser, error) {
	reader, err := s.Storage.GetReader(ctx, key)
	if err == nil {
		s.read()
	}
	return reader, err
}

func TestCollaborationAttachmentSlowReadRechecksRevocationBeforeBytes(t *testing.T) {
	h := collaborationFileHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Slow attachment reader", uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	draft, uploaded := collaborationUploadedFile(t, h, issue, testUserID)
	id := uuidToString(uploaded.ID)
	testutil.Call(t, h.DeliverCollaborationAttachments, collaborationFileRequest(http.MethodPost, issue, "", "", testUserID,
		map[string]any{"draft_id": draft, "attachment_ids": []string{id}})).Want(http.StatusOK)
	h.Storage = collaborationReadHookStorage{Storage: h.Storage, read: func() {
		dbfx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue, external)
	}}
	result := testutil.Call(t, h.DownloadCollaborationAttachment, collaborationFileRequest(http.MethodGet, issue, "", id, external, nil)).Want(http.StatusNotFound)
	if strings.Contains(result.Text(), "controlled private bytes") || result.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("a slow storage read sent private content after revocation")
	}
}

type collaborationRepeatedByteReader struct{}

func (collaborationRepeatedByteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestCollaborationAttachmentUploadEnforcesRealFileLimitBeforeStorage(t *testing.T) {
	h := &Handler{}
	reader, writer := io.Pipe()
	multi := multipart.NewWriter(writer)
	done := make(chan error, 1)
	go func() {
		file, err := multi.CreateFormFile("file", "too-large.bin")
		if err == nil {
			_, err = io.CopyN(file, collaborationRepeatedByteReader{}, service.CollaborationAttachmentMaxBytes+1)
		}
		if err == nil {
			err = multi.Close()
		}
		_ = writer.CloseWithError(err)
		done <- err
	}()
	req, err := http.NewRequest(http.MethodPost, "/api/collaboration/attachments/drafts", reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", multi.FormDataContentType())
	req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT}))
	testutil.Call(t, h.UploadCollaborationAttachmentDraft, req).Want(http.StatusRequestEntityTooLarge)
	_ = reader.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	testutil.Call(t, h.UploadCollaborationAttachmentDraft, req.WithContext(ctx)).Want(http.StatusRequestTimeout)
}

func TestCollaborationAttachmentForeignDraftDoesNotRevealExistence(t *testing.T) {
	h := collaborationFileHandler(t)
	issue := collaborationBoundaryIssue(t, testWorkspaceID)
	external := dbfx.User(t, "Foreign draft reader", uuid.NewString()+"@example.test")
	dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": external}, "issue_id=$1 AND user_id=$2", issue, external)
	dbfx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	dbfx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	draft, uploaded := collaborationUploadedFile(t, h, issue, testUserID)
	for _, id := range []string{uuidToString(uploaded.ID), uuid.NewString()} {
		r := collaborationFileRequest(http.MethodPost, issue, "", "", external, map[string]any{"draft_id": draft, "attachment_ids": []string{id}})
		result := testutil.Call(t, h.DeliverCollaborationAttachments, r).Want(http.StatusNotFound).Map()
		if result["code"] != "attachment_not_found" {
			t.Fatalf("private and nonexistent draft IDs must have the same result: %v", result)
		}
	}
	if dbfx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue) != 0 {
		t.Fatal("foreign draft attempt left a content operation")
	}
}
