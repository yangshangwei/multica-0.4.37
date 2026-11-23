package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// These cursor contracts are pure parsing tests; they do not use DB fixtures.
func TestCollaborationHandoffCursorEmptyStartsAtRequestedRun(t *testing.T) {
	const runID = "11111111-1111-4111-8111-111111111111"
	got, err := parseHandoffCursor("", runID)
	if err != nil || !reflect.DeepEqual(got, collaborationHandoffCursor{RunID: runID}) {
		t.Fatalf("first-page cursor=%+v err=%v", got, err)
	}
}

func TestCollaborationHandoffCursorRoundTripsEmittedKinds(t *testing.T) {
	const runID = "11111111-1111-4111-8111-111111111111"
	const afterID = "22222222-2222-4222-8222-222222222222"
	const commandID = "33333333-3333-4333-8333-333333333333"
	before := time.Date(2026, time.November, 1, 12, 34, 56, 123456789, time.UTC)
	for _, want := range []collaborationHandoffCursor{
		{Kind: "inputs", RunID: runID, Offset: 32},
		{Kind: "results", RunID: runID, AfterID: afterID, BeforeAt: &before},
		{Kind: "causes", RunID: runID, Offset: 32},
		{Kind: "readers", RunID: runID, AfterID: afterID, CommandID: commandID},
	} {
		t.Run(want.Kind, func(t *testing.T) {
			got, err := parseHandoffCursor(handoffCursor(want), runID)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("cursor=%+v err=%v, want %+v", got, err, want)
			}
		})
	}
	t.Run("whitespace_at_encoded_limit", func(t *testing.T) {
		want := collaborationHandoffCursor{Kind: "inputs", RunID: runID, Offset: 32}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		// 768 decoded bytes encode to the existing 1,024-byte cursor limit.
		raw = append(raw, []byte(strings.Repeat(" ", 768-len(raw)))...)
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		if len(encoded) != 1024 {
			t.Fatalf("fixture encoded length=%d", len(encoded))
		}
		got, err := parseHandoffCursor(encoded, runID)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("boundary cursor=%+v err=%v", got, err)
		}
	})
}

func TestCollaborationHandoffCursorRejectsMalformedBindings(t *testing.T) {
	const runID = "11111111-1111-4111-8111-111111111111"
	const otherRunID = "22222222-2222-4222-8222-222222222222"
	valid := fmt.Sprintf(`{"kind":"inputs","run_id":%q,"offset":32}`, runID)
	for _, tc := range []struct {
		name string
		json string
		raw  string
	}{
		{name: "omitted_run_id", json: `{"kind":"inputs","offset":32}`},
		{name: "null_run_id", json: `{"kind":"inputs","run_id":null,"offset":32}`},
		{name: "empty_run_id", json: `{"kind":"inputs","run_id":"","offset":32}`},
		{name: "wrong_run_id", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q}`, otherRunID)},
		{name: "duplicate_same_run_id", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"run_id":%q}`, runID, runID)},
		{name: "duplicate_replaced_run_id", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"run_id":%q}`, otherRunID, runID)},
		{name: "duplicate_escaped_run_id", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"r\u0075n_id":%q}`, runID, runID)},
		{name: "duplicate_kind", json: fmt.Sprintf(`{"kind":"inputs","kind":"results","run_id":%q}`, runID)},
		{name: "duplicate_offset", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"offset":1,"offset":32}`, runID)},
		{name: "unknown_field", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"scope":"all"}`, runID)},
		{name: "case_variant_run_id", json: fmt.Sprintf(`{"kind":"inputs","RUN_ID":%q}`, runID)},
		{name: "case_variant_duplicate", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"Run_ID":%q}`, runID, runID)},
		{name: "missing_kind", json: fmt.Sprintf(`{"run_id":%q}`, runID)},
		{name: "unknown_kind", json: fmt.Sprintf(`{"kind":"all","run_id":%q}`, runID)},
		{name: "negative_offset", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"offset":-1}`, runID)},
		{name: "fractional_offset", json: fmt.Sprintf(`{"kind":"inputs","run_id":%q,"offset":1.5}`, runID)},
		{name: "bad_after_id", json: fmt.Sprintf(`{"kind":"causes","run_id":%q,"after_id":"invalid"}`, runID)},
		{name: "bad_command_id", json: fmt.Sprintf(`{"kind":"readers","run_id":%q,"command_id":"invalid"}`, runID)},
		{name: "bad_before_at", json: fmt.Sprintf(`{"kind":"causes","run_id":%q,"before_at":"invalid"}`, runID)},
		{name: "null_document", json: `null`},
		{name: "array_document", json: `[]`},
		{name: "trailing_document", json: valid + `{}`},
		{name: "trailing_garbage", json: valid + `x`},
		{name: "truncated_document", json: strings.TrimSuffix(valid, "}")},
		{name: "invalid_base64", raw: "%not-base64"},
		{name: "encoded_limit_exceeded", raw: strings.Repeat("a", 1025)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.raw
			if raw == "" {
				raw = base64.RawURLEncoding.EncodeToString([]byte(tc.json))
			}
			_, err := parseHandoffCursor(raw, runID)
			var problem *CollaborationError
			if !errors.As(err, &problem) || problem.Status != http.StatusBadRequest || problem.Code != "invalid_handoff_cursor" {
				t.Fatalf("malformed cursor accepted or wrong error: %v", err)
			}
		})
	}
}
