package handler

import (
	"encoding/json"
	"testing"
)

// The autonomy gate reads a raw key map, while the write paths read a struct that
// encoding/json filled case-insensitively. These tests pin the two together: any
// casing that reaches the struct must also trip the gate, or an Observer sets
// status through the variant.

func TestRequestTouchesIssueDirectionIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"status":"done"}`,
		`{"Status":"done"}`,
		`{"STATUS":"done"}`,
		`{"sTaTuS":"done"}`,
		`{"assignee_type":"agent"}`,
		`{"Assignee_Type":"agent"}`,
		`{"ASSIGNEE_TYPE":"agent"}`,
		`{"assignee_id":"x"}`,
		`{"Assignee_ID":"x"}`,
		`{"ASSIGNEE_ID":"x"}`,
		// Explicit null is "unassign", which is why the gate reads presence at all.
		`{"assignee_id":null}`,
		`{"Assignee_Id":null}`,
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("%s: unmarshal: %v", body, err)
		}
		if !requestTouchesIssueDirection(raw) {
			t.Errorf("%s: gate did not fire; an Observer would reach the write", body)
		}
	}
}

func TestRequestTouchesIssueDirectionIgnoresOtherFields(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{}`,
		`{"title":"t"}`,
		`{"Title":"t"}`,
		`{"description":"d","priority":2}`,
		// Neither is gated at the API, and the prompt says so.
		`{"parent_issue_id":"x"}`,
		`{"statuses":"not the field"}`,
		`{"assignee":"not the field"}`,
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			t.Fatalf("%s: unmarshal: %v", body, err)
		}
		if requestTouchesIssueDirection(raw) {
			t.Errorf("%s: gate fired on a field it does not cover", body)
		}
	}
}

// requestSetsIssueDirection is the gate the handlers call. Each half has a hole
// the other closes, so both are pinned here.
func TestRequestSetsIssueDirectionCombinesPresenceAndPointers(t *testing.T) {
	t.Parallel()

	str := func(s string) *string { return &s }

	t.Run("presence alone is enough", func(t *testing.T) {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(`{"assignee_id":null}`), &raw); err != nil {
			t.Fatal(err)
		}
		// Explicit null decodes to a nil pointer, so only presence sees it.
		if !requestSetsIssueDirection(raw, nil, nil, nil) {
			t.Error("explicit null assignee_id must trip the gate")
		}
	})

	t.Run("pointer alone is enough", func(t *testing.T) {
		// An empty raw map stands in for the case the outer key's casing hid the
		// nested object: the struct is populated, the map is not.
		if !requestSetsIssueDirection(nil, str("done"), nil, nil) {
			t.Error("decoded status must trip the gate even with no raw keys")
		}
		if !requestSetsIssueDirection(map[string]json.RawMessage{}, nil, nil, str("x")) {
			t.Error("decoded assignee_id must trip the gate even with no raw keys")
		}
	})

	t.Run("neither means no direction change", func(t *testing.T) {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(`{"title":"t"}`), &raw); err != nil {
			t.Fatal(err)
		}
		if requestSetsIssueDirection(raw, nil, nil, nil) {
			t.Error("a title-only edit must not trip the gate")
		}
	})
}

// The struct decode is the reason the gate has to fold case. If this ever fails,
// encoding/json changed and the gate can be simplified.
func TestJSONDecodeIsCaseInsensitiveForDirectionFields(t *testing.T) {
	t.Parallel()

	var req UpdateIssueRequest
	if err := json.Unmarshal([]byte(`{"Status":"done","ASSIGNEE_ID":"a"}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Status == nil || *req.Status != "done" {
		t.Fatal("expected capitalised Status to populate the struct")
	}
	if req.AssigneeID == nil || *req.AssigneeID != "a" {
		t.Fatal("expected upper-case ASSIGNEE_ID to populate the struct")
	}
}
