package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const fixtureTestIssue = "00000000-0000-4000-8000-000000000001"
const fixtureTestCandidate = "00000000-0000-4000-8000-000000000002"
const fixtureTestContent = "Approved test data"

// The test-created CLI is this exact test executable. It never resolves an
// installed Multica or provider command and does not contact a server.
func init() {
	if os.Getenv("MULTICA_FIXTURE_REPORT_HELPER") != "1" {
		return
	}
	if err := fixtureReportTestCLI(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func fixtureReportTestCLI() error {
	args := os.Args[1:]
	if len(args) < 4 || args[0] != "collaboration" || args[1] != "test-report" || args[3] != fixtureTestIssue {
		return fmt.Errorf("unexpected fake CLI command")
	}
	if os.Getenv("MULTICA_FIXTURE_REPORT_HELPER_MODE") == "unavailable" {
		return fmt.Errorf("HTTP 409 test_subject_unavailable")
	}
	requestID := "00000000-0000-4000-8000-000000000003"
	ack := map[string]any{"version": 1, "issue_id": fixtureTestIssue, "run_id": fixtureTestCandidate,
		"request_id": requestID, "report_id": "00000000-0000-4000-8000-000000000004", "report_digest": strings.Repeat("e", 64), "accepted": true}
	var output any
	switch args[2] {
	case "subject":
		output = map[string]any{"version": 1, "issue_id": fixtureTestIssue, "run_id": fixtureTestCandidate,
			"subject": map[string]any{"candidate_id": fixtureTestCandidate, "candidate_version": "2", "candidate_digest": strings.Repeat("b", 64),
				"binding_digest": strings.Repeat("c", 64), "source_revision": strings.Repeat("d", 40), "artifact_digest": nil}}
	case "submit":
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 64<<10))
		if err != nil || !json.Valid(raw) {
			return fmt.Errorf("invalid fake report stdin")
		}
		if err := os.WriteFile(os.Getenv("MULTICA_FIXTURE_REPORT_CAPTURE"), raw, 0600); err != nil {
			return err
		}
		output = ack
	case "receipt":
		output = ack
	default:
		return fmt.Errorf("unexpected fake report action")
	}
	return json.NewEncoder(os.Stdout).Encode(output)
}

func TestFixtureTestReportUsesPreparedSourceAndOriginalCLIReceipt(t *testing.T) {
	for _, fault := range []string{"none", "duplicate_effect"} {
		t.Run(fault, func(t *testing.T) {
			work := t.TempDir()
			marker := `{"version":1,"revision":2,"callbacks":["callback-1","callback-1"],"expected_effects":1}`
			if err := os.WriteFile(filepath.Join(work, "stage-regression.json"), []byte(marker), 0600); err != nil {
				t.Fatal(err)
			}
			cli, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("MULTICA_FIXTURE_REPORT_HELPER", "1")
			t.Setenv("MULTICA_FIXTURE_REPORT_CAPTURE", filepath.Join(work, "submitted.json"))
			var action fixtureAction
			if err := json.Unmarshal([]byte(`{"kind":"test_report","request_id":"00000000-0000-4000-8000-000000000003","fault":"`+fault+`"}`), &action); err != nil {
				t.Fatal(err)
			}
			receipt := runFixtureAction(cli, fixtureTestIssue, work, action)
			if receipt["error"] != nil {
				t.Fatalf("prepared report action failed: %v", receipt["error"])
			}
			raw, err := os.ReadFile(filepath.Join(work, "submitted.json"))
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Cases []struct {
					ID, Outcome string
					LogIndex    int `json:"log_index"`
				}
				Logs []struct{ Kind, Body string }
			}
			if json.Unmarshal(raw, &report) != nil || len(report.Cases) != 2 || len(report.Logs) != 2 {
				t.Fatal("fixture did not submit its actual bounded cases/logs")
			}
			want := "passed"
			if fault == "duplicate_effect" {
				want = "failed"
			}
			if report.Cases[1].ID != "duplicate-callback" || report.Cases[1].Outcome != want || report.Cases[1].LogIndex != 1 || report.Logs[1].Kind != "inline" {
				t.Fatalf("wrong observed case: %+v", report)
			}
			if !strings.Contains(report.Logs[0].Body, fixtureDigest([]byte(marker))) {
				t.Fatal("report log omitted actual prepared source hash")
			}
			encoded, _ := json.Marshal(receipt)
			if !strings.Contains(string(encoded), `"submit"`) || !strings.Contains(string(encoded), `"receipt"`) || !strings.Contains(string(encoded), `"source_revision"`) {
				t.Fatal("actual CLI subject/submit/original receipt evidence is missing")
			}
		})
	}
}

func TestFixtureTestSubjectUnavailableIsNotAPassingReport(t *testing.T) {
	cli, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MULTICA_FIXTURE_REPORT_HELPER", "1")
	t.Setenv("MULTICA_FIXTURE_REPORT_HELPER_MODE", "unavailable")
	receipt := runFixtureAction(cli, fixtureTestIssue, t.TempDir(), fixtureAction{Kind: "test_subject"})
	if receipt["error"] != nil {
		t.Fatalf("expected explicit subject absence: %v", receipt["error"])
	}
	raw, _ := json.Marshal(receipt)
	if !strings.Contains(string(raw), `"test_subject_unavailable"`) || strings.Contains(string(raw), `"accepted":true`) {
		t.Fatal("bare host subject absence was fabricated into report success")
	}
}

func TestFixturePaymentCasesRejectUnboundedOrUnrelatedMarkers(t *testing.T) {
	for _, raw := range []string{`null`, `{}`, `{"version":1,"revision":1,"callbacks":["x"],"expected_effects":1,"extra":true}`,
		`{"version":1,"revision":1,"callbacks":["x"],"expected_effects":1} {}`, strings.Repeat("x", (16<<10)+1)} {
		if _, _, err := fixturePaymentCases([]byte(raw), "none"); err == nil {
			t.Fatal("invalid prepared marker was accepted")
		}
	}
}

func TestFixtureReleaseFailureKeepsDefaultSuccessSeparate(t *testing.T) {
	success := fixtureHostResult("session", "run", false)
	failed := fixtureHostResult("session", "run", true)
	if success["subtype"] != "success" || success["is_error"] != false || success["result"] != "Host fixture result for run" {
		t.Fatal("default host completion changed")
	}
	if failed["subtype"] != "error_during_execution" || failed["is_error"] != true || failed["session_id"] != "session" || !strings.Contains(failed["result"].(string), "Synthetic") {
		t.Fatal("controlled failure did not remain an ordinary provider error result")
	}
}

func TestFixtureCLIOutputLimitAlsoAppliesToPipeCopies(t *testing.T) {
	var output fixtureBoundedOutput
	_, err := io.Copy(&output, io.LimitReader(strings.NewReader(strings.Repeat("x", (64<<10)+1)), (64<<10)+1))
	if err == nil || output.Len() > 64<<10 {
		t.Fatal("CLI output limit was bypassed by io.Copy's ReaderFrom path")
	}
}

func fixtureTestFrozen() string {
	return `{"issue_id":"` + fixtureTestIssue + `","inputs":[{"command_id":"command","author_user_id":"author","body":"accepted assignment","attachment_ids":[]}]}`
}

func fixtureTestContribution(t *testing.T, change func(map[string]any)) string {
	t.Helper()
	item := map[string]any{"candidate_id": fixtureTestCandidate, "version": "1", "source_issue_id": fixtureTestIssue,
		"content": fixtureTestContent, "content_sha256": fixtureDigest([]byte(fixtureTestContent))}
	if change != nil {
		change(item)
	}
	value, err := json.Marshal([]map[string]any{item})
	if err != nil {
		t.Fatal(err)
	}
	return string(value)
}

func TestFixturePromptAcceptsManagedKnowledgeSuffix(t *testing.T) {
	prompt := "Frozen assignment:\n" + fixtureTestFrozen() + fixtureKnowledgeMarker + fixtureTestContribution(t, nil)
	input, got, err := parseFixturePrompt(prompt, fixtureTestIssue)
	if err != nil {
		t.Fatalf("valid product knowledge suffix was refused: %v", err)
	}
	expected := fixtureKnowledgeObservation{PromptSHA256: fixtureDigest([]byte(prompt)), SectionPresent: true,
		Contributions: []fixtureKnowledgeFact{{CandidateID: fixtureTestCandidate, Version: "1", SourceIssueID: fixtureTestIssue,
			ContentSHA256: fixtureDigest([]byte(fixtureTestContent))}}}
	if input.IssueID != fixtureTestIssue || len(input.Inputs) != 1 || input.Inputs[0].Body != "accepted assignment" || !reflect.DeepEqual(got, expected) {
		t.Fatal("actual frozen assignment or safe knowledge observation changed")
	}
	encoded, err := json.Marshal(got)
	if err != nil || strings.Contains(string(encoded), fixtureTestContent) {
		t.Fatal("observation exposed private knowledge body")
	}
}

func TestFixturePromptKeepsStrictFrameAndKnowledgeBoundaries(t *testing.T) {
	valid := fixtureTestContribution(t, nil)
	for _, tc := range []struct {
		name, tail        string
		accepted, section bool
	}{
		{"no_knowledge", "", true, false},
		{"whitespace_only", " \n\t", true, false},
		{"valid_empty_array", fixtureKnowledgeMarker + "[]\n", true, true},
		{"valid_array_trailing_whitespace", fixtureKnowledgeMarker + valid + " \n\t", true, true},
		{"garbage_suffix", "not approved data", false, false},
		{"second_frozen_value", " {}", false, false},
		{"wrong_marker", "\nApproved managed knowledge:\n" + valid, false, false},
		{"extra_prefix_whitespace", " " + fixtureKnowledgeMarker + valid, false, false},
		{"malformed_array", fixtureKnowledgeMarker + "[{", false, false},
		{"not_array", fixtureKnowledgeMarker + "null", false, false},
		{"knowledge_second_value", fixtureKnowledgeMarker + valid + " []", false, false},
		{"knowledge_garbage_tail", fixtureKnowledgeMarker + valid + "no", false, false},
		{"hash_mismatch", fixtureKnowledgeMarker + fixtureTestContribution(t, func(v map[string]any) { v["content_sha256"] = strings.Repeat("0", 64) }), false, false},
		{"invalid_candidate", fixtureKnowledgeMarker + fixtureTestContribution(t, func(v map[string]any) { v["candidate_id"] = "not-an-id" }), false, false},
		{"invalid_source", fixtureKnowledgeMarker + fixtureTestContribution(t, func(v map[string]any) { v["source_issue_id"] = "not-an-id" }), false, false},
		{"zero_version", fixtureKnowledgeMarker + fixtureTestContribution(t, func(v map[string]any) { v["version"] = "0" }), false, false},
		{"noncanonical_version", fixtureKnowledgeMarker + fixtureTestContribution(t, func(v map[string]any) { v["version"] = "01" }), false, false},
		{"duplicate_identity", fixtureKnowledgeMarker + strings.TrimSuffix(valid, "]") + "," + strings.TrimPrefix(valid, "["), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, observed, err := parseFixturePrompt("Frozen assignment:\n"+fixtureTestFrozen()+tc.tail, fixtureTestIssue)
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%t want=%t", err == nil, tc.accepted)
			}
			if tc.accepted && (observed.SectionPresent != tc.section || !tc.section && len(observed.Contributions) != 0) {
				t.Fatal("knowledge framing observation differed")
			}
		})
	}
}

func TestFixturePromptStillRequiresAdmittedIssueAndInputs(t *testing.T) {
	for _, tc := range []struct{ name, prompt, issue string }{
		{"missing", "no frozen input", fixtureTestIssue},
		{"wrong_issue", fixtureTestFrozen(), fixtureTestCandidate},
		{"missing_inputs", `{"issue_id":"` + fixtureTestIssue + `","inputs":[]}`, fixtureTestIssue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseFixturePrompt(tc.prompt, tc.issue); err == nil {
				t.Fatal("unadmitted frozen input accepted")
			}
		})
	}
}
