package util

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestIssueMetadataForResponsePreservesLifecycleEvidence(t *testing.T) {
	raw := []byte(`{
		"lifecycle_handoff":{"kind":"rca","evidence":{"facts":["parser regression"]}},
		"lifecycle_handoff_history":[{"kind":"rca"},{"kind":"rollout"}],
		"lifecycle_rca_evidence":["parser regression"],
		"lifecycle_rca_unknowns":["impact window"],
		"pipeline_status":"waiting","pr_number":3,"is_blocked":true
	}`)
	stored := JSONObjectOrEmpty(raw)
	response := IssueMetadataForResponse(raw)
	for _, key := range []string{"lifecycle_handoff", "lifecycle_handoff_history", "lifecycle_rca_evidence", "lifecycle_rca_unknowns"} {
		encoded, ok := response[key].(string)
		if !ok {
			t.Errorf("metadata[%q] is %T, want a JSON string accepted by installed clients", key, response[key])
			continue
		}
		var decoded any
		if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
			t.Fatalf("decode metadata[%q]: %v", key, err)
		}
		if !reflect.DeepEqual(decoded, stored[key]) {
			t.Errorf("metadata[%q] lost lifecycle evidence: got %#v, want %#v", key, decoded, stored[key])
		}
	}
	for _, key := range []string{"pipeline_status", "pr_number", "is_blocked"} {
		if !reflect.DeepEqual(response[key], stored[key]) {
			t.Errorf("metadata[%q] changed: got %#v, want %#v", key, response[key], stored[key])
		}
	}
	if _, ok := JSONObjectOrEmpty(raw)["lifecycle_handoff"].(map[string]any); !ok {
		t.Fatal("raw lifecycle readers must retain structured metadata")
	}
}

func TestIssueMetadataForResponseLeavesOtherValuesUnchanged(t *testing.T) {
	raw := []byte(`{"lifecycle_handoff":"{\"kind\":\"rca\"}","lifecycle_handoff_history":42,"lifecycle_rca_evidence":false,"lifecycle_rca_unknowns":null,"other":{"nested":true},"lifecycle_other":[1]}`)
	if got, want := IssueMetadataForResponse(raw), JSONObjectOrEmpty(raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("only reserved composite lifecycle values should change: got %#v, want %#v", got, want)
	}
}

func TestIssueMetadataForResponseDefaultsToEmptyObject(t *testing.T) {
	for _, raw := range []string{"", "null", "{invalid", "[]", "{}"} {
		t.Run(raw, func(t *testing.T) {
			if got := IssueMetadataForResponse([]byte(raw)); got == nil || len(got) != 0 {
				t.Fatalf("metadata = %#v, want an empty object", got)
			}
		})
	}
}
