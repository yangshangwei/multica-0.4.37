package protocol

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func populatedHandoffEnvelope() CollaborationHandoffEnvelope {
	v := testHandoffEnvelope()
	v.Handoff = &CollaborationHandoffTransfer{ID: v.IssueID, AuthorID: v.IssueID, FromID: v.IssueID, ToID: v.RunID, Purpose: "transfer", EventSequence: "0", SourceID: v.IssueID}
	v.InformationRequest = v.Handoff
	v.Inputs = CollaborationHandoffInputs{CollaborationHandoffPage: CollaborationHandoffPage{Total: 1, Returned: 1},
		Items: []CollaborationHandoffInput{{CommandID: v.IssueID, AuthorID: v.IssueID, State: "consumed", ReaderRunIDs: []string{v.RunID}}}}
	v.Results = CollaborationHandoffResults{CollaborationHandoffPage: CollaborationHandoffPage{Total: 1, Returned: 1},
		Items: []CollaborationHandoffResult{{RunID: v.RunID, Role: "agent", Progress: "completed", ProofState: "host_exit_observed", ResultState: "absent", FinalAnswerState: "unknown"}}}
	v.WakeupCauses = CollaborationHandoffCauses{CollaborationHandoffPage: CollaborationHandoffPage{Total: 1, Returned: 1}, Completeness: "recorded",
		Items: []CollaborationHandoffCause{{ID: v.IssueID, SourceKind: "proof", SourceID: v.IssueID, SourceRunID: v.RunID, Sequence: "1"}}}
	v.Sources = []CollaborationHandoffSource{{Kind: "handoff", ID: v.IssueID, SHA256: CollaborationHandoffHash(""), Content: "", Availability: "available", Trust: "data_only"}}
	return v
}

func handoffWireObject(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func handoffWireAt(object map[string]any, path string) map[string]any {
	var current any = object
	if path != "" {
		for _, part := range strings.Split(path, ".") {
			if items, ok := current.([]any); ok {
				index, _ := strconv.Atoi(part)
				current = items[index]
			} else {
				current = current.(map[string]any)[part]
			}
		}
	}
	return current.(map[string]any)
}

func TestCollaborationHandoffWireRequiresDeclaredFields(t *testing.T) {
	// Completeness and empty text are facts even when their value is false/zero.
	for path, fields := range map[string][]string{
		"":                      {"protocol_version", "issue_id", "run_id", "mode", "event_seq", "authority", "assignment", "memory", "inputs", "results", "wakeup_causes", "sources", "missing"},
		"authority":             {"status"},
		"assignment":            {"role", "step_key"},
		"handoff":               {"id", "author_id", "from_id", "to_id", "purpose", "event_seq", "source_id"},
		"information_request":   {"id", "author_id", "from_id", "to_id", "purpose", "event_seq", "source_id"},
		"memory":                {"version", "availability"},
		"inputs":                {"total", "returned", "truncated", "items"},
		"inputs.items.0":        {"command_id", "author_id", "state", "reader_run_ids", "readers_truncated"},
		"results":               {"total", "returned", "truncated", "items"},
		"results.items.0":       {"run_id", "role", "step_key", "progress", "proof_state", "recording_incomplete", "result_state", "final_answer_state"},
		"wakeup_causes":         {"total", "returned", "truncated", "items", "completeness"},
		"wakeup_causes.items.0": {"id", "source_kind", "source_id", "source_run_id", "sequence"},
		"sources.0":             {"kind", "id", "sha256", "content", "truncated", "availability", "trust"},
	} {
		for _, field := range fields {
			for _, mode := range []string{"missing", "null"} {
				t.Run(path+"/"+field+"/"+mode, func(t *testing.T) {
					object := handoffWireObject(t, populatedHandoffEnvelope())
					nested := handoffWireAt(object, path)
					if mode == "missing" {
						delete(nested, field)
					} else {
						nested[field] = nil
					}
					raw, _ := json.Marshal(object)
					var out CollaborationHandoffEnvelope
					if json.Unmarshal(raw, &out) == nil {
						t.Fatal("accepted a handoff without a required fact")
					}
				})
			}
		}
	}
}

func TestCollaborationHandoffWireRequiresEmptyPageFacts(t *testing.T) {
	for _, page := range []string{"inputs", "results", "wakeup_causes"} {
		for _, field := range []string{"total", "returned", "truncated"} {
			for _, mode := range []string{"missing", "null"} {
				t.Run(page+"/"+field+"/"+mode, func(t *testing.T) {
					object := handoffWireObject(t, testHandoffEnvelope())
					nested := handoffWireAt(object, page)
					if mode == "missing" {
						delete(nested, field)
					} else {
						nested[field] = nil
					}
					raw, _ := json.Marshal(object)
					var out CollaborationHandoffEnvelope
					if json.Unmarshal(raw, &out) == nil {
						t.Fatal("fabricated a complete empty page")
					}
				})
			}
		}
	}
}

func TestCollaborationHandoffWirePreservesZeroFalseAndOptionalFields(t *testing.T) {
	for name, value := range map[string]CollaborationHandoffEnvelope{"empty": testHandoffEnvelope(), "populated": populatedHandoffEnvelope()} {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(value)
			var out CollaborationHandoffEnvelope
			if err := json.Unmarshal(raw, &out); err != nil || !reflect.DeepEqual(value, out) {
				t.Fatalf("changed a valid envelope with omitted optional fields: %v", err)
			}
		})
	}
	for _, field := range []string{"handoff", "information_request"} {
		t.Run(field, func(t *testing.T) {
			object := handoffWireObject(t, testHandoffEnvelope())
			delete(object, field)
			raw, _ := json.Marshal(object)
			var out CollaborationHandoffEnvelope
			if json.Unmarshal(raw, &out) == nil {
				t.Fatal("missing transfer must not be interpreted as explicit null")
			}
		})
	}
	object := handoffWireObject(t, testHandoffEnvelope())
	object["recording"] = nil
	raw, _ := json.Marshal(object)
	var out CollaborationHandoffEnvelope
	if err := json.Unmarshal(raw, &out); err != nil || out.Recording != nil {
		t.Fatalf("optional null recording refused: %v", err)
	}
}

func TestCollaborationHandoffWireRejectsFieldAliases(t *testing.T) {
	for _, tc := range []struct {
		name, path, extra string
	}{
		{"source_case_override", "sources.0", `"TRUNCATED":false`},
		{"source_mixed_case", "sources.0", `"TrUnCaTeD":false`},
		{"source_duplicate", "sources.0", `"truncated":false`},
		{"source_escaped_duplicate", "sources.0", `"\u0074runcated":false`},
		{"source_escaped_case", "sources.0", `"\u0054RUNCATED":false`},
		{"source_optional_alias", "sources.0", `"RUN_ID":"22222222-2222-4222-8222-222222222222"`},
		{"snapshot_optional_alias", "", `"SNAPSHOT_ID":"22222222-2222-4222-8222-222222222222"`},
		{"recording_optional_override", "", `"RECORDING":null`},
		{"authority_optional_alias", "authority", `"HANDLER_ID":"22222222-2222-4222-8222-222222222222"`},
		{"assignment_optional_alias", "assignment", `"PARENT_RUN_ID":"22222222-2222-4222-8222-222222222222"`},
		{"embedded_page_total", "inputs", `"TOTAL":1`},
		{"embedded_page_returned", "results", `"RETURNED":1`},
		{"embedded_page_truncated", "wakeup_causes", `"TRUNCATED":false`},
		{"input_readers", "inputs.items.0", `"READERS_TRUNCATED":false`},
		{"result_completeness", "results.items.0", `"RECORDING_INCOMPLETE":true`},
		{"result_optional_alias", "results.items.0", `"SOURCE_KIND":"provider_result"`},
		{"retention_sealed", "recording", `"SEALED":false`},
		{"retention_observed", "recording", `"OBSERVED_THROUGH":null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := populatedHandoffEnvelope()
			value.Sources[0].Truncated = true
			retention := testRecordingRetention()
			value.Recording = &retention
			object := handoffWireObject(t, value)
			nested, _ := json.Marshal(handoffWireAt(object, tc.path))
			raw, _ := json.Marshal(object)
			// Append the alias after the canonical key, preserving wire order.
			alias := strings.TrimSuffix(string(nested), "}") + "," + tc.extra + "}"
			wire := strings.Replace(string(raw), string(nested), alias, 1)
			if wire == string(raw) {
				t.Fatal("fixture failed to insert the alias")
			}
			var out CollaborationHandoffEnvelope
			if json.Unmarshal([]byte(wire), &out) == nil {
				t.Fatal("accepted a noncanonical or duplicate typed field")
			}
		})
	}
}

func TestCollaborationHostObjectChecksTypedChildrenWithoutChangingContentMaps(t *testing.T) {
	type row struct {
		Flag bool `json:"flag"`
	}
	type payload struct {
		Identity CollaborationRunIdentity `json:"identity"`
		Rows     []row                    `json:"rows"`
		Content  map[string]any           `json:"content"`
	}
	value := payload{Identity: checkpointFixture().Identity, Rows: []row{{Flag: true}},
		Content: map[string]any{"truncated": true, "TRUNCATED": false, "nested": map[string]any{"NAME": "one", "name": "two"}}}
	raw, _ := json.Marshal(value)
	var out payload
	if err := decodeCollaborationHostObject(raw, &out, "identity", "rows", "content"); err != nil || !reflect.DeepEqual(value, out) {
		t.Fatalf("changed arbitrary content-map key semantics: %v", err)
	}
	for _, replacement := range [][2]string{
		{`"run_id":"` + value.Identity.RunID + `"`, `"run_id":"` + value.Identity.RunID + `","RUN_ID":"` + value.Identity.RunID + `"`},
		{`"flag":true`, `"flag":true,"FLAG":false`},
	} {
		wire := strings.Replace(string(raw), replacement[0], replacement[1], 1)
		if wire == string(raw) {
			t.Fatal("fixture failed to insert a typed child alias")
		}
		if decodeCollaborationHostObject([]byte(wire), &out, "identity", "rows", "content") == nil {
			t.Error("accepted an alias in a typed child without its own decoder")
		}
	}
}

func TestCollaborationHandoffWirePreservesEscapedCanonicalNamesAndLiteralText(t *testing.T) {
	value := populatedHandoffEnvelope()
	value.Sources[0].Content = `{"truncated":true,"TRUNCATED":false,"\u0074runcated":false}`
	value.Sources[0].SHA256 = CollaborationHandoffHash(value.Sources[0].Content)
	raw, _ := json.Marshal(value)
	// JSON escaping is decoded before canonical-name and duplicate checks.
	wire := strings.ReplaceAll(string(raw), `"truncated":`, `"\u0074runcated":`)
	var out CollaborationHandoffEnvelope
	if err := json.Unmarshal([]byte(wire), &out); err != nil || !reflect.DeepEqual(value, out) {
		t.Fatalf("changed a canonical decoded name or literal source text: %v", err)
	}
}
