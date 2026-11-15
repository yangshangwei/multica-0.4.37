package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestStageHTTPTestReportPurposeAndMalformedBodies(t *testing.T) {
	h := &Handler{}
	for _, fn := range []http.HandlerFunc{h.RecordCollaborationRuntimeTestReport, h.GetCollaborationRuntimeTestSubject, h.GetCollaborationRuntimeTestReportReceipt} {
		for _, token := range []string{"", "mul_human", "mat_task", "mdt_daemon", "mct_" + strings.Repeat("a", 64)} {
			r := withURLParam(newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+uuid.NewString()+"/test-reports", map[string]any{}), "runId", uuid.NewString())
			r.Header.Set("Authorization", "Bearer "+token)
			out := testutil.Call(t, fn, r).Want(http.StatusUnauthorized)
			if out.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("purpose denial cacheable")
			}
		}
	}
	for _, body := range []string{`{}`, `{"version":1,"version":1}`, `{"identity":null,"report":{"suite_id":"PRIVATE_REPORT_SENTINEL"}}`, `null`, `{"actor_id":"PRIVATE_REPORT_SENTINEL"}`} {
		run := uuid.NewString()
		r := withURLParam(newRequest(http.MethodPost, "/api/collaboration/runtime/runs/"+run+"/test-reports", json.RawMessage(body)), "runId", run)
		r.Header.Set("Authorization", "Bearer "+service.CollaborationBusinessTokenPrefix+strings.Repeat("a", 64))
		out := testutil.Call(t, h.RecordCollaborationRuntimeTestReport, r).Want(http.StatusBadRequest)
		if out.Map()["code"] != "invalid_test_report" || strings.Contains(out.Text(), "PRIVATE_REPORT_SENTINEL") {
			t.Fatalf("unsafe parse refusal: %s", out.Text())
		}
	}
}

func TestStageHTTPCreateOnlyTestSubjectMalformed(t *testing.T) {
	for _, subject := range []string{`null`, `{}`, `{"candidate_id":"` + uuid.NewString() + `","candidate_version":"1","candidate_digest":"` + strings.Repeat("a", 64) + `","unknown":true}`, `{"candidate_id":"` + uuid.NewString() + `","candidate_version":"1","candidate_version":"2","candidate_digest":"` + strings.Repeat("a", 64) + `"}`} {
		body := json.RawMessage(`{"content":"synthetic","expected_lifecycle_epoch":"1","execution_view_version":6,"test_subject":` + subject + `}`)
		r := withURLParam(newRequest(http.MethodPost, "/api/collaboration/issues/"+uuid.NewString()+"/comments", body), "id", uuid.NewString())
		out := testutil.Call(t, (&Handler{}).CreateCollaborationComment, r).Want(http.StatusBadRequest)
		if out.Map()["code"] != "invalid_test_subject" {
			t.Fatalf("test subject malformed error lost: %s", out.Text())
		}
	}
}
