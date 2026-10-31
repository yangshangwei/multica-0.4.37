package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationRecordingSealRejectsBodyAndWrongCredential(t *testing.T) {
	h := &Handler{}
	for _, body := range []string{`{"version":1,"body":"forbidden discovery"}`, `{"version":1,"reason":"stopped","reason":"completed"}`, `{"version":1,"messages":[]}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/collaboration/runtime/runs/00000000-0000-4000-8000-000000000001/recording/seal", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer mct_"+strings.Repeat("a", 64))
		testutil.Call(t, h.SealCollaborationRuntimeRecording, req).Want(http.StatusBadRequest)
	}
	req := httptest.NewRequest(http.MethodPost, "/recording/seal", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mcb_"+strings.Repeat("a", 64))
	testutil.Call(t, h.SealCollaborationRuntimeRecording, req).Want(http.StatusUnauthorized)
}
