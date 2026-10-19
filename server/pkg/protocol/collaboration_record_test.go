package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCollaborationOutcomePreservesTypedRecording(t *testing.T) {
	raw := []byte(`{"outcome":"success","body":"final result","record":{"version":1,"completeness":"complete","messages":[{"kind":"message","role":"assistant","body":"recorded response"},{"kind":"tool_call","call_id":"read-1","name":"read","body":"input"},{"kind":"tool_result","call_id":"read-1","body":"result","is_error":false}],"usage":{"input_tokens":"9007199254740993","output_tokens":"0"}}}`)
	var outcome CollaborationRunOutcome
	if err := json.Unmarshal(raw, &outcome); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &stored); err != nil {
		t.Fatal(err)
	}
	if _, exists := stored["record"]; !exists {
		t.Fatal("typed recorded messages and exact usage were discarded at the runtime wire boundary")
	}
}

func TestCollaborationRecordingRejectsMalformedAndOversizedTelemetry(t *testing.T) {
	valid := `{"outcome":"success","body":"result","record":{"version":1,"completeness":"complete","messages":[{"kind":"message","role":"assistant","body":"recorded"}],"usage":{"input_tokens":"0"}}}`
	for _, raw := range []string{
		strings.Replace(valid, `"version":1`, `"version":2`, 1),
		strings.Replace(valid, `"complete"`, `"inferred"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"input_tokens":"-1"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"input_tokens":"01"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"input_tokens":"9223372036854775808"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"cost_usd":"1e2"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"cost_usd":"0.00000000001"`, 1),
		strings.Replace(valid, `"input_tokens":"0"`, `"cost_usd":"10000000000000000000"`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"body":"`+strings.Repeat("a", CollaborationRecordTextLimit+1)+`"`, 1),
		strings.Replace(valid, `"role":"assistant"`, `"role":"private_thought"`, 1),
		strings.Replace(valid, `"kind":"message"`, `"kind":"tool_result"`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"name":""`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"body":null`, 1),
		strings.Replace(valid, `"body":"recorded"`, `"body":"recorded","call_id":""`, 1),
	} {
		var result CollaborationProviderResult
		if err := json.Unmarshal([]byte(raw), &result); err == nil && ValidateCollaborationProviderResult(result) == nil {
			t.Fatal("malformed typed recording was accepted")
		}
	}
	var result CollaborationProviderResult
	if err := json.Unmarshal([]byte(valid), &result); err != nil || ValidateCollaborationProviderResult(result) != nil {
		t.Fatal("explicit recorded zero was refused")
	}
	result.Record.Messages = make([]CollaborationRecordedMessage, CollaborationRecordItemLimit+1)
	if ValidateCollaborationProviderResult(result) == nil {
		t.Fatal("message count cap was absent")
	}
	result.Record.Messages = nil
	if ValidateCollaborationProviderResult(result) == nil {
		t.Fatal("missing messages became a recorded empty transcript")
	}
}
