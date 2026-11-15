package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationTestReportUncertainResponseKeepsOriginalReceipt(t *testing.T) {
	collaborationEnvironmentFixture(t)
	var run *CollaborationContext
	key := uuid.NewString()
	posts, gets := 0, 0
	ack := protocol.CollaborationTestReportReceipt{Version: 1, RequestID: key, ReportID: uuid.NewString(), ReportDigest: strings.Repeat("d", 64), Accepted: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+run.Token {
			t.Error("changed run credential")
		}
		if r.Method == "POST" {
			posts++
			var p protocol.CollaborationTestReportRequest
			if json.NewDecoder(r.Body).Decode(&p) != nil || p.RequestID != key || p.Identity != run.Identity {
				t.Error("changed original request")
			}
			ack.IssueID, ack.RunID = run.Identity.IssueID, run.Identity.RunID
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"code":"collaboration_unavailable"}`))
			return
		}
		gets++
		if r.URL.Path != run.Path("/test-reports/by-request/"+key) {
			t.Error("receipt changed original key")
		}
		_ = json.NewEncoder(w).Encode(ack)
	}))
	defer server.Close()
	t.Setenv(CollaborationServerEnv, server.URL)
	var err error
	run, err = ReadCollaborationContext()
	if err != nil {
		t.Fatal(err)
	}
	report := protocol.CollaborationTestReport{Subject: protocol.CollaborationTestSubject{CandidateID: uuid.NewString(), CandidateVersion: "1", CandidateDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64), SourceRevision: strings.Repeat("c", 40)}, SuiteID: "synthetic", Cases: []protocol.CollaborationTestCase{{ID: "case", Outcome: "failed", LogIndex: 0}}, Logs: []protocol.CollaborationTestLog{{Kind: "inline", Body: "failure"}}}
	if _, err = run.SubmitTestReport(t.Context(), run.Client(), key, report); err == nil || !strings.Contains(FormatError(err, false), key) || posts != 1 || gets != 0 {
		t.Fatalf("unknown report lost identity or retried: posts=%d gets=%d err=%v", posts, gets, err)
	}
	recovered, err := run.ReadTestReportReceipt(t.Context(), run.Client(), key)
	if err != nil || recovered != ack || posts != 1 || gets != 1 {
		t.Fatalf("receipt failed: %+v posts=%d gets=%d %v", recovered, posts, gets, err)
	}
}
