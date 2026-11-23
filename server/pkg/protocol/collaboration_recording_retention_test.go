package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func testRecordingRetention() CollaborationRecordingRetention {
	return CollaborationRecordingRetention{Version: 1, StreamID: checkpointFixture().StreamID, Reason: "incomplete"}
}

func TestCollaborationRecordingRetentionWireRequiresFacts(t *testing.T) {
	for _, field := range []string{"version", "stream_id", "retained_through", "observed_through", "sealed", "reason"} {
		for _, mode := range []string{"missing", "null"} {
			if field == "observed_through" && mode == "null" {
				continue // Explicit null is the supported unknown tail.
			}
			t.Run(field+"/"+mode, func(t *testing.T) {
				object := handoffWireObject(t, testRecordingRetention())
				if mode == "missing" {
					delete(object, field)
				} else {
					object[field] = nil
				}
				raw, _ := json.Marshal(object)
				var out CollaborationRecordingRetention
				if json.Unmarshal(raw, &out) == nil {
					t.Error("retention accepted a missing required fact")
				}
				handoff := handoffWireObject(t, testHandoffEnvelope())
				handoff["recording"] = object
				raw, _ = json.Marshal(handoff)
				var envelope CollaborationHandoffEnvelope
				if json.Unmarshal(raw, &envelope) == nil {
					t.Error("handoff accepted invalid retention")
				}
			})
		}
	}
}

func TestCollaborationHandoffRejectsMalformedRetention(t *testing.T) {
	for name, mutate := range map[string]func(*CollaborationRecordingRetention){
		"version":           func(v *CollaborationRecordingRetention) { v.Version = 999 },
		"stream":            func(v *CollaborationRecordingRetention) { v.StreamID = "not-a-uuid" },
		"retained_negative": func(v *CollaborationRecordingRetention) { v.RetainedThrough = -1 },
		"retained_overflow": func(v *CollaborationRecordingRetention) { v.RetainedThrough = 257 },
		"observed_negative": func(v *CollaborationRecordingRetention) { n := -5; v.ObservedThrough = &n; v.Sealed = true },
		"observed_overflow": func(v *CollaborationRecordingRetention) { n := 257; v.ObservedThrough = &n; v.Sealed = true },
		"observed_before_retained": func(v *CollaborationRecordingRetention) {
			n := 1
			v.ObservedThrough = &n
			v.RetainedThrough = 2
			v.Sealed = true
		},
		"arbitrary_reason":         func(v *CollaborationRecordingRetention) { v.Reason = "anything" },
		"unsealed_known_tail":      func(v *CollaborationRecordingRetention) { n := 1; v.ObservedThrough = &n },
		"unsealed_terminal_reason": func(v *CollaborationRecordingRetention) { v.Reason = "stopped" },
		"completed_unknown_tail":   func(v *CollaborationRecordingRetention) { v.Sealed = true; v.Reason = "completed" },
	} {
		t.Run(name, func(t *testing.T) {
			retention := testRecordingRetention()
			mutate(&retention)
			value := testHandoffEnvelope()
			value.Recording = &retention
			if ValidateCollaborationHandoff(value) == nil {
				t.Error("typed handoff accepted invalid recording retention")
			}
			raw, _ := json.Marshal(value)
			var out CollaborationHandoffEnvelope
			if json.Unmarshal(raw, &out) == nil {
				t.Error("wire handoff accepted invalid recording retention")
			}
		})
	}
}

func TestCollaborationRecordingRetentionPreservesKnownAndUnknownBounds(t *testing.T) {
	for _, reason := range []string{"completed", "stopped", "authority_lost", "transport_error", "recording_limit", "incomplete"} {
		t.Run(reason, func(t *testing.T) {
			for _, observed := range []*int{nil, new(int), func() *int { n := 256; return &n }()} {
				if reason == "completed" && observed == nil {
					continue
				}
				retention := testRecordingRetention()
				retention.Sealed, retention.Reason, retention.ObservedThrough = true, reason, observed
				if observed != nil {
					retention.RetainedThrough = *observed
				}
				value := testHandoffEnvelope()
				value.Recording = &retention
				raw, _ := json.Marshal(value)
				var out CollaborationHandoffEnvelope
				if err := json.Unmarshal(raw, &out); err != nil || !reflect.DeepEqual(value, out) {
					t.Fatalf("changed known/unknown retention bounds: %v", err)
				}
			}
		})
	}
	retention := testRecordingRetention()
	raw, _ := json.Marshal(retention)
	var out CollaborationRecordingRetention
	if err := json.Unmarshal(raw, &out); err != nil || !reflect.DeepEqual(retention, out) {
		t.Fatalf("changed unsealed zero-retained/unknown-observed facts: %v", err)
	}
}
