package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const fixtureTestIssue = "00000000-0000-4000-8000-000000000001"
const fixtureTestCandidate = "00000000-0000-4000-8000-000000000002"
const fixtureTestContent = "Approved test data"

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
