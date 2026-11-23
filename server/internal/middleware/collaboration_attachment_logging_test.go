package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollaborationAttachmentRequestLogOmitsCapabilityAndContent(t *testing.T) {
	logs := withCapturedLogs(t)
	h := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"not_found","private":"file-body-sentinel"}`))
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/collaboration/issues/issue/attachments/file/capability?cap=private-token-sentinel", nil))
	if text := logs.String(); strings.Contains(text, "private-token-sentinel") || strings.Contains(text, "file-body-sentinel") || !strings.Contains(text, "/capability") {
		t.Fatalf("private capability or body entered request logs: %s", text)
	}
}
