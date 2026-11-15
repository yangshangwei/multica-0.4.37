package protocol

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func testReportFixture() CollaborationTestReportRequest {
	id := func() string { return uuid.NewString() }
	return CollaborationTestReportRequest{Version: 1, RequestID: id(), Identity: CollaborationRunIdentity{
		DeploymentID: id(), InstallationID: id(), IssueID: id(), SessionID: id(), TargetID: id(), ChannelID: id(), BatchID: id(), GroupID: id(), RunID: id(), LaunchNonce: id(), Fence: "1", ClaimGeneration: "1"},
		Report: CollaborationTestReport{Subject: CollaborationTestSubject{CandidateID: id(), CandidateVersion: "1", CandidateDigest: strings.Repeat("a", 64), BindingDigest: strings.Repeat("b", 64), SourceRevision: strings.Repeat("c", 40)}, SuiteID: "payment-regression",
			Cases: []CollaborationTestCase{{ID: "callback-delay", Outcome: "failed", LogIndex: 0}}, Logs: []CollaborationTestLog{{Kind: "inline", Body: "expected paid, observed pending"}}}}
}

func TestCollaborationTestReportStrictDecode(t *testing.T) {
	p := testReportFixture()
	raw, _ := json.Marshal(p)
	if _, err := DecodeCollaborationTestReportRequest(raw); err != nil {
		t.Fatal(err)
	}
	for name, wire := range map[string]string{
		"duplicate":     strings.Replace(string(raw), `"version":1`, `"version":1,"version":1`, 1),
		"alias":         strings.Replace(string(raw), `"suite_id"`, `"Suite_id"`, 1),
		"unknown":       strings.Replace(string(raw), `"suite_id":`, `"passed":true,"suite_id":`, 1),
		"missing_null":  strings.Replace(string(raw), `,"artifact_digest":null`, ``, 1),
		"missing_zero":  strings.Replace(string(raw), `,"log_index":0`, ``, 1),
		"null_zero":     strings.Replace(string(raw), `"log_index":0`, `"log_index":null`, 1),
		"nul":           strings.Replace(string(raw), "callback-delay", `callback\u0000delay`, 1),
		"invalid_utf8":  strings.Replace(string(raw), "callback-delay", "callback\xffdelay", 1),
		"trailing":      string(raw) + `{}`,
		"attempt_spoof": strings.Replace(string(raw), `"suite_id":`, `"attempt_seq":"2","suite_id":`, 1),
		"mixed_log":     strings.Replace(string(raw), `"kind":"inline"`, `"kind":"inline","event_id":"`+uuid.NewString()+`"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCollaborationTestReportRequest([]byte(wire)); err == nil {
				t.Fatal("malformed report accepted")
			}
		})
	}
}

func TestCollaborationTestReportLimitsAndDerivedResult(t *testing.T) {
	for _, outcome := range []string{"passed", "failed", "error", "skipped"} {
		t.Run(outcome, func(t *testing.T) {
			p := testReportFixture()
			p.Report.Cases[0].Outcome = outcome
			want := map[string]string{"passed": "passed", "failed": "failed", "error": "failed", "skipped": "incomplete"}[outcome]
			if err := ValidateCollaborationTestReport(p.Report); err != nil || p.Report.ReportedResult() != want {
				t.Fatalf("result=%s err=%v", p.Report.ReportedResult(), err)
			}
		})
	}
	for name, mutate := range map[string]func(*CollaborationTestReport){
		"empty_cases":     func(p *CollaborationTestReport) { p.Cases = nil },
		"duplicate_case":  func(p *CollaborationTestReport) { p.Cases = append(p.Cases, p.Cases[0]) },
		"unknown_outcome": func(p *CollaborationTestReport) { p.Cases[0].Outcome = "success" },
		"missing_log":     func(p *CollaborationTestReport) { p.Logs = nil },
		"log_index":       func(p *CollaborationTestReport) { p.Cases[0].LogIndex = 1 },
		"negative_index":  func(p *CollaborationTestReport) { p.Cases[0].LogIndex = -1 },
		"large_inline":    func(p *CollaborationTestReport) { p.Logs[0].Body = strings.Repeat("x", (64<<10)+1) },
		"aggregate_inline": func(p *CollaborationTestReport) {
			p.Logs[0].Body = strings.Repeat("x", 64<<10)
			p.Logs = append(p.Logs, CollaborationTestLog{Kind: "inline", Body: "x"})
		},
		"utf8_bytes":           func(p *CollaborationTestReport) { p.Cases[0].ID = strings.Repeat("测", 86) },
		"noncanonical_version": func(p *CollaborationTestReport) { p.Subject.CandidateVersion = "01" },
		"nil_uuid":             func(p *CollaborationTestReport) { p.Subject.CandidateID = uuid.Nil.String() },
	} {
		t.Run(name, func(t *testing.T) {
			p := testReportFixture().Report
			mutate(&p)
			if ValidateCollaborationTestReport(p) == nil {
				t.Fatal("invalid report accepted")
			}
		})
	}
}
