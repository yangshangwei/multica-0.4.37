package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func runtimeOutputMultipart(t *testing.T, run, token, comment, content string, fields map[string][]string, files int) *http.Request {
	t.Helper()
	var body bytes.Buffer
	multi := multipart.NewWriter(&body)
	for _, field := range []struct{ name, value string }{{"comment_id", comment}, {"content", content}} {
		if err := multi.WriteField(field.name, field.value); err != nil {
			t.Fatal(err)
		}
	}
	for name, values := range fields {
		for _, value := range values {
			if err := multi.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	for range files {
		part, err := multi.CreateFormFile("file", "output.bin")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.WriteString(part, "original Agent bytes"); err != nil {
			t.Fatal(err)
		}
	}
	if err := multi.Close(); err != nil {
		t.Fatal(err)
	}
	r := testutil.JSONRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+run+"/comments/attachments", body.Bytes())
	r.Header.Set("Content-Type", multi.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-User-ID", testUserID)
	r.Header.Set("X-Agent-ID", uuid.NewString())
	r.Header.Set("X-Workspace-ID", testWorkspaceID)
	return withURLParam(r, "runId", run)
}

func TestCollaborationRuntimeOutputHTTPRejectsAuthorityAndMalformedMultipart(t *testing.T) {
	for _, scenario := range []string{"human", "termination", "draft_id", "uploader_agent_id", "attempt_seq", "run_id", "duplicate_comment", "duplicate_content", "two_files", "no_file", "invalid_parent", "zero_parent", "oversized_content"} {
		t.Run(scenario, func(t *testing.T) {
			token, expected := "mcb_"+strings.Repeat("a", 64), http.StatusBadRequest
			fields := map[string][]string{}
			files, content := 1, "original Agent output"
			switch scenario {
			case "human":
				token, expected = "mul_"+strings.Repeat("a", 64), http.StatusUnauthorized
			case "termination":
				token, expected = "mct_"+strings.Repeat("a", 64), http.StatusUnauthorized
			case "draft_id", "uploader_agent_id", "attempt_seq", "run_id":
				fields[scenario] = []string{uuid.NewString()}
			case "duplicate_comment":
				fields["comment_id"] = []string{uuid.NewString()}
			case "duplicate_content":
				fields["content"] = []string{"changed"}
			case "two_files":
				files = 2
			case "no_file":
				files = 0
			case "invalid_parent":
				fields["parent_id"] = []string{"not-a-comment"}
			case "zero_parent":
				fields["parent_id"] = []string{uuid.Nil.String()}
			case "oversized_content":
				content = strings.Repeat("x", service.CollaborationCommentMaxBytes+1)
			}
			r := runtimeOutputMultipart(t, uuid.NewString(), token, uuid.NewString(), content, fields, files)
			// Nil dependencies prove these requests stop at the actual boundary,
			// before a caller header can supply a run, uploader or DB write.
			response := testutil.Call(t, (&Handler{}).CreateCollaborationRuntimeCommentAttachment, r).Want(expected)
			if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Text(), "original Agent bytes") {
				t.Fatal("runtime upload boundary exposed private bytes or cacheable response")
			}
		})
	}
}

func TestCollaborationRuntimeOutputHTTPOriginalReceiptAndFinalByteRevocation(t *testing.T) {
	f := newRuntimePreparationHTTPFixture(t)
	comment := uuid.NewString()
	request := func() *http.Request {
		return runtimeOutputMultipart(t, f.claim.Identity.RunID, f.permit.BusinessToken, comment, "original Agent output", nil, 1)
	}
	var original service.CollaborationRuntimeCommentReceipt
	response := testutil.Call(t, f.handler.CreateCollaborationRuntimeCommentAttachment, request()).Want(http.StatusCreated)
	response.JSON(&original)
	if original.Attachment == nil || original.CommentID != parseUUID(comment) || original.Attachment.UploaderAgentID != parseUUID(f.agent) || original.Attachment.UploaderUserID.Valid || original.Attachment.DraftID.Valid || original.Attachment.RunID != parseUUID(f.claim.Identity.RunID) || original.Attachment.ContentType != http.DetectContentType([]byte("original Agent bytes")) {
		t.Fatalf("HTTP output forged human identity or trusted filename type: %+v", original)
	}
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, original.Attachment.ID)
	dbfx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, original.Attachment.ID)
	dbfx.Cleanup(t, `DELETE FROM comment WHERE id=$1`, original.CommentID)
	var replay service.CollaborationRuntimeCommentReceipt
	testutil.Call(t, f.handler.CreateCollaborationRuntimeCommentAttachment, request()).Want(http.StatusCreated).JSON(&replay)
	if replay.Attachment == nil || replay.Attachment.ID != original.Attachment.ID {
		t.Fatal("HTTP original-key recovery created a second output identity")
	}
	file := uuidToString(original.Attachment.ID)
	read := func() *http.Request {
		r := preparationHTTPRequest(http.MethodGet, f.claim.Identity.RunID, file, f.permit.BusinessToken, nil)
		r.URL.RawQuery = "version=1"
		return r
	}
	bytes := testutil.Call(t, f.handler.GetCollaborationRuntimeAttachmentContent, read()).Want(http.StatusOK)
	if bytes.Text() != "original Agent bytes" || bytes.Header().Get("X-Multica-Attachment-ID") != file || bytes.Header().Get("X-Multica-Run-ID") != f.claim.Identity.RunID {
		t.Fatal("own output reader did not preserve original bytes and exact identity")
	}
	f.handler.Storage = collaborationReadHookStorage{Storage: f.handler.Storage, read: func() {
		dbfx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),epoch=epoch+1 WHERE id=$1`, f.grant)
	}}
	denied := testutil.Call(t, f.handler.GetCollaborationRuntimeAttachmentContent, read()).Want(http.StatusConflict)
	if denied.Map()["code"] != "grant_required" || strings.Contains(denied.Text(), "original Agent bytes") || denied.Header().Get("X-Multica-Attachment-ID") != "" {
		t.Fatal("withdrawal during output storage read crossed final byte authorization")
	}
	replayDenied := testutil.Call(t, f.handler.CreateCollaborationRuntimeCommentAttachment, request()).Want(http.StatusConflict)
	if replayDenied.Map()["code"] != "grant_required" {
		t.Fatal("original output replay bypassed the withdrawn grant")
	}
}
