package iteration

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const draftIssueA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
const draftIssueB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
const draftIteration = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"

func draftFixture() map[string]any {
	return map[string]any{"operation": "move", "iteration_id": nil, "expected_iteration_revision": nil, "expected_scope_revision": nil, "expected_settings_revision": 1, "reason": " move\r\nwith care ", "moves": []any{
		map[string]any{"issue_id": draftIssueB, "expected_issue_revision": 2, "expected_source_id": nil, "target_id": draftIteration, "allow_completed": false},
		map[string]any{"issue_id": strings.ToUpper(draftIssueA), "expected_issue_revision": 3, "expected_source_id": nil, "target_id": draftIteration, "allow_completed": true},
	}, "start": nil}
}

func canonicalFixture(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	out, err := CanonicalDraftJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCanonicalDraftOrderingAndHash(t *testing.T) {
	input := draftFixture()
	a := canonicalFixture(t, input)
	moves := input["moves"].([]any)
	moves[0], moves[1] = moves[1], moves[0]
	input["reason"] = "move\nwith care"
	b := canonicalFixture(t, input)
	if !bytes.Equal(a, b) {
		t.Fatalf("unstable canonical draft:\n%s\n%s", a, b)
	}
	if !bytes.Contains(a, []byte(`"iteration_id":null`)) || bytes.Contains(a, []byte(strings.ToUpper(draftIssueA))) {
		t.Fatalf("wrong normalization: %s", a)
	}
	h1, err := CanonicalHash(map[string]any{"workspace_id": draftIteration, "draft": json.RawMessage(a), "contract_version": 1})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := CanonicalHash(map[string]any{"draft": json.RawMessage(b), "contract_version": 1, "workspace_id": strings.ToUpper(draftIteration)})
	if err != nil || h1 != h2 || len(h1) != 64 {
		t.Fatalf("unstable hash: %s %s %v", h1, h2, err)
	}
	h3, _ := CanonicalHash(map[string]any{"workspace_id": draftIteration, "draft": json.RawMessage(a), "contract_version": 2})
	if h3 == h1 {
		t.Fatal("changed contract did not change hash")
	}
}

func TestCanonicalDraftRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"duplicate move", func(d map[string]any) { d["moves"] = append(d["moves"].([]any), d["moves"].([]any)[0]) }},
		{"unknown field", func(d map[string]any) { d["silent_change"] = true }},
		{"bad uuid", func(d map[string]any) { d["moves"].([]any)[0].(map[string]any)["issue_id"] = "bad" }},
		{"zero revision", func(d map[string]any) { d["expected_settings_revision"] = 0 }},
		{"unsafe revision", func(d map[string]any) { d["expected_settings_revision"] = int64(9007199254740992) }},
		{"fraction revision", func(d map[string]any) { d["expected_settings_revision"] = 1.5 }},
		{"missing boolean", func(d map[string]any) { delete(d["moves"].([]any)[0].(map[string]any), "allow_completed") }},
		{"inapplicable start", func(d map[string]any) {
			d["start"] = map[string]any{"target_id": draftIteration, "mode": "today", "terminal_choices": []any{}}
		}},
		{"disable custom moves", func(d map[string]any) { d["operation"] = "disable" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := draftFixture()
			test.mutate(d)
			raw, _ := json.Marshal(d)
			if _, err := CanonicalDraftJSON(raw); err == nil {
				t.Fatalf("accepted invalid input: %s", raw)
			}
		})
	}
	for _, raw := range []string{`null`, `[]`, `{} {}`, `{"operation":"move","operation":"disable"}`} {
		if _, err := CanonicalDraftJSON([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestCanonicalDraftRejectsRevisionPrecisionLoss(t *testing.T) {
	baseline := string(canonicalFixture(t, draftFixture()))
	for _, field := range []struct{ name, original string }{
		{"expected_settings_revision", "1"},
		{"expected_iteration_revision", "null"},
		{"expected_scope_revision", "null"},
		{"expected_issue_revision", "2"},
	} {
		for _, number := range []string{"1.00000000000000001", "0.99999999999999999999", "1.0", "1e0"} {
			t.Run(field.name+"/"+number, func(t *testing.T) {
				// Preserve the original JSON token: float64 construction would
				// already round away the invalid fractional revision.
				raw := strings.Replace(baseline, `"`+field.name+`":`+field.original, `"`+field.name+`":`+number, 1)
				if raw == baseline {
					t.Fatal("revision fixture was not replaced")
				}
				if _, err := CanonicalDraftJSON([]byte(raw)); err == nil {
					t.Fatalf("accepted non-integer wire revision: %s", raw)
				}
			})
		}
	}
}

func TestCanonicalStartTerminalChoices(t *testing.T) {
	d := draftFixture()
	d["operation"] = "start"
	d["iteration_id"] = draftIteration
	d["expected_iteration_revision"] = 1
	d["expected_scope_revision"] = 2
	d["moves"] = []any{}
	d["start"] = map[string]any{"target_id": draftIteration, "mode": "today", "terminal_choices": []any{map[string]any{"issue_id": draftIssueB, "retain": false}, map[string]any{"issue_id": draftIssueA, "retain": true}}}
	normalized := canonicalFixture(t, d)
	if bytes.Index(normalized, []byte(draftIssueA)) > bytes.Index(normalized, []byte(draftIssueB)) {
		t.Fatal("terminal choices are not sorted")
	}
	choices := d["start"].(map[string]any)["terminal_choices"].([]any)
	d["start"].(map[string]any)["terminal_choices"] = append(choices, choices[0])
	raw, _ := json.Marshal(d)
	if _, err := CanonicalDraftJSON(raw); err == nil {
		t.Fatal("duplicate terminal choice accepted")
	}
}

func TestCanonicalHashCalendarValidation(t *testing.T) {
	if _, err := CanonicalHash(map[string]any{"start_date": "2026-02-29"}); err == nil {
		t.Fatal("invalid calendar fact accepted")
	}
	a, err := CanonicalHash(map[string]any{"start_date": "2026-10-05", "reference_date": "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := CanonicalHash(map[string]any{"start_date": "2026-10-05", "reference_date": "2026-10-06"})
	if a == b {
		t.Fatal("local midnight did not invalidate hash")
	}
}

func TestCanonicalHashPreservesDisplayTextCase(t *testing.T) {
	// A title that happens to look like a UUID is still display text.
	upper, err := CanonicalHash(map[string]any{"title": strings.ToUpper(draftIssueA)})
	if err != nil {
		t.Fatal(err)
	}
	lower, _ := CanonicalHash(map[string]any{"title": draftIssueA})
	if upper == lower {
		t.Fatal("display-text change disappeared from comparison facts")
	}
}

func TestCanonicalDraftMatchesSharedDTO(t *testing.T) {
	rev := int64(1)
	source := draftIteration
	input := Draft{Operation: "start", IterationID: &source, ExpectedIterationRevision: &rev, ExpectedScopeRevision: &rev, ExpectedSettingsRevision: 1, Moves: []Move{}, Start: &StartDraft{TargetID: source, Mode: "today", TerminalChoices: []TerminalChoice{{IssueID: draftIssueA, Retain: true}}}}
	normalized := canonicalFixture(t, input)
	var got Draft
	if err := json.Unmarshal(normalized, &got); err != nil {
		t.Fatal(err)
	}
	again := canonicalFixture(t, got)
	if !bytes.Equal(normalized, again) {
		t.Fatalf("DTO roundtrip changed intent: %s vs %s", normalized, again)
	}
}

func TestCanonicalDraftRejectsUnknownNestedFields(t *testing.T) {
	for _, path := range []string{"move", "start", "terminal_choice"} {
		t.Run(path, func(t *testing.T) {
			d := draftFixture()
			if path == "move" {
				d["moves"].([]any)[0].(map[string]any)["run_execution"] = true
			} else {
				d["operation"] = "start"
				d["iteration_id"] = draftIteration
				d["expected_iteration_revision"] = 1
				d["expected_scope_revision"] = 1
				d["moves"] = []any{}
				terminal := map[string]any{"issue_id": draftIssueA, "retain": true}
				start := map[string]any{"target_id": draftIteration, "mode": "today", "terminal_choices": []any{terminal}}
				d["start"] = start
				if path == "start" {
					start["auto_start"] = true
				} else {
					terminal["target_id"] = draftIteration
				}
			}
			raw, _ := json.Marshal(d)
			if _, err := CanonicalDraftJSON(raw); err == nil {
				t.Fatal("unknown nested field was silently ignored")
			}
		})
	}
}

func FuzzCanonicalDraftJSON(f *testing.F) {
	raw, _ := json.Marshal(draftFixture())
	f.Add(raw)
	f.Add([]byte(`{"operation":"disable","expected_settings_revision":1,"moves":[]}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		normalized, err := CanonicalDraftJSON(raw)
		if err != nil {
			return
		}
		again, err := CanonicalDraftJSON(normalized)
		if err != nil || !bytes.Equal(normalized, again) {
			t.Fatalf("canonicalization is not idempotent: %s %v", normalized, err)
		}
		var draft Draft
		if err := json.Unmarshal(normalized, &draft); err != nil {
			t.Fatalf("accepted JSON cannot decode into shared Draft: %v", err)
		}
		encoded, err := json.Marshal(draft)
		if err != nil {
			t.Fatal(err)
		}
		roundtrip, err := CanonicalDraftJSON(encoded)
		if err != nil || !bytes.Equal(normalized, roundtrip) {
			t.Fatalf("shared Draft changed normalized intent: %s / %s / %v", normalized, roundtrip, err)
		}
	})
}
