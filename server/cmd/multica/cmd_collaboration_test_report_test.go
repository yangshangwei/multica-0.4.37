package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationTestReportCommandsRegistered(t *testing.T) {
	for _, action := range []string{"subject", "submit", "receipt"} {
		cmd, _, err := rootCmd.Find([]string{"collaboration", "test-report", action})
		if err != nil || cmd.Name() != action || !collaborationCommandAllowed("collaboration test-report "+action) {
			t.Fatalf("missing run-local command %s: %v", action, err)
		}
	}
}

func TestCollaborationTestReportCommandsUseOnlyRunCapability(t *testing.T) {
	var identity protocol.CollaborationRunIdentity
	request := uuid.NewString()
	subject := protocol.CollaborationTestSubject{CandidateID: uuid.NewString(), CandidateVersion: "1", CandidateDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64), SourceRevision: strings.Repeat("c", 40)}
	receipt := protocol.CollaborationTestReportReceipt{Version: 1, RequestID: request, ReportID: uuid.NewString(), ReportDigest: strings.Repeat("d", 64), Accepted: true}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) || r.Header.Get("X-Workspace-ID") != "" {
			t.Error("borrowed ambient credentials")
		}
		base := "/api/collaboration/runtime/runs/" + identity.RunID
		switch r.URL.Path {
		case base + "/test-subject":
			if r.Method != "GET" {
				t.Error("subject method")
			}
			_ = json.NewEncoder(w).Encode(protocol.CollaborationTestSubjectResponse{Version: 1, IssueID: identity.IssueID, RunID: identity.RunID, Subject: subject})
		case base + "/test-reports":
			var p protocol.CollaborationTestReportRequest
			if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&p) != nil || p.RequestID != request || p.Identity != identity || p.Report.Subject.BindingDigest != subject.BindingDigest {
				t.Error("report changed original request/subject/identity")
			}
			receipt.IssueID, receipt.RunID = identity.IssueID, identity.RunID
			_ = json.NewEncoder(w).Encode(receipt)
		case base + "/test-reports/by-request/" + request:
			if r.Method != "GET" {
				t.Error("receipt submitted again")
			}
			_ = json.NewEncoder(w).Encode(receipt)
		default:
			t.Errorf("escaped runtime route: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	identity = collaborationCLIEnvironment(t, server.URL)
	report := protocol.CollaborationTestReport{Subject: subject, SuiteID: "synthetic CLI", Cases: []protocol.CollaborationTestCase{{ID: "callback", Outcome: "failed", LogIndex: 0}}, Logs: []protocol.CollaborationTestLog{{Kind: "inline", Body: "actual synthetic failure"}}}
	raw, _ := json.Marshal(report)
	for _, action := range []string{"subject", "submit", "receipt"} {
		cmd := newCollaborationTestReportAction(action)
		var out bytes.Buffer
		cmd.SetOut(&out)
		if action != "subject" {
			_ = cmd.Flags().Set("request-id", request)
		}
		if action == "submit" {
			_ = cmd.Flags().Set("report-stdin", "true")
			cmd.SetIn(bytes.NewReader(raw))
		}
		if err := cmd.RunE(cmd, []string{identity.IssueID}); err != nil || out.Len() == 0 {
			t.Fatalf("%s: %v output=%s", action, err, out.String())
		}
	}
	if calls != 3 {
		t.Fatalf("unexpected automatic requests: %d", calls)
	}
	for _, kind := range []string{"wrong_issue", "bad_request", "profile", "bad_report", "oversized", "no_stdin"} {
		cmd := newCollaborationTestReportAction("submit")
		_ = cmd.Flags().Set("request-id", request)
		_ = cmd.Flags().Set("report-stdin", "true")
		cmd.SetIn(bytes.NewReader(raw))
		args := []string{identity.IssueID}
		switch kind {
		case "wrong_issue":
			args[0] = uuid.NewString()
		case "bad_request":
			_ = cmd.Flags().Set("request-id", "not-a-uuid")
		case "profile":
			cmd.Flags().String("profile", "", "")
			_ = cmd.Flags().Set("profile", "ambient-human")
		case "bad_report":
			cmd.SetIn(strings.NewReader(`{"subject":null}`))
		case "oversized":
			cmd.SetIn(strings.NewReader(strings.Repeat("x", protocol.CollaborationTestReportJSONLimit+1)))
		case "no_stdin":
			_ = cmd.Flags().Set("report-stdin", "false")
		}
		if err := cmd.RunE(cmd, args); err == nil || calls != 3 {
			t.Fatalf("%s reached HTTP or succeeded: %d %v", kind, calls, err)
		}
	}
}

func TestCollaborationTestReportCommandRefusesOutsideRun(t *testing.T) {
	for _, key := range []string{"MULTICA_COLLABORATION_TOKEN", "MULTICA_COLLABORATION_SERVER", "MULTICA_COLLABORATION_CONTEXT", "MULTICA_COLLABORATION_ISSUE_ID", "MULTICA_COLLABORATION_RUN_ID"} {
		t.Setenv(key, "")
	}
	cmd := newCollaborationTestReportAction("submit")
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.RunE(cmd, []string{"132a61f0-45e6-484c-8d0b-e7a237164263"}); err == nil || !strings.Contains(err.Error(), "inside a collaboration run") {
		t.Fatalf("outside-run fallback: %v", err)
	}
}
