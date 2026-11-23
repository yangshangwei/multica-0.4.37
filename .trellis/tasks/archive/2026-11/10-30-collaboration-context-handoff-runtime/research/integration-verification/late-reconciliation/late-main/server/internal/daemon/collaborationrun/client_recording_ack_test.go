package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestClientRecordingSealRejectsMalformedRetention(t *testing.T) {
	f := newHostRunFixture(t, "success")
	observed := 1
	seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: f.claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), StreamID: uuid.NewString(), ObservedThrough: &observed, Reason: "completed"}
	bound := func(n int) *int { return &n }
	for _, tc := range []struct {
		name     string
		retained int
		observed *int
	}{
		{name: "negative retained", retained: -1, observed: bound(1)},
		{name: "retained above run limit", retained: protocol.CollaborationRecordItemLimit + 1, observed: bound(protocol.CollaborationRecordItemLimit + 1)},
		{name: "negative observed", observed: bound(-1)},
		{name: "observed above run limit", observed: bound(protocol.CollaborationRecordItemLimit + 1)},
		{name: "observed below retained", retained: 2, observed: bound(1)},
		{name: "completed unknown tail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(protocol.CollaborationRecordingRetention{Version: 1, StreamID: seal.StreamID, Sealed: true, Reason: seal.Reason, RetainedThrough: tc.retained, ObservedThrough: tc.observed}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.RecordingSeal(context.Background(), seal, f.claim.TerminationToken)
			if !errors.Is(err, ErrProtocol) || !reflect.DeepEqual(out, protocol.CollaborationRecordingRetention{}) {
				t.Fatalf("malformed seal receipt accepted: out=%+v err=%v", out, err)
			}
		})
	}
}

func TestClientRecordingSealAcceptsTruthfulRetention(t *testing.T) {
	f := newHostRunFixture(t, "success")
	bound := func(n int) *int { return &n }
	for _, tc := range []struct {
		name      string
		reason    string
		retained  int
		observed  *int
		requested *int
	}{
		{name: "empty complete recording", reason: "completed", observed: bound(0), requested: bound(0)},
		{name: "maximum complete recording", reason: "completed", retained: protocol.CollaborationRecordItemLimit, observed: bound(protocol.CollaborationRecordItemLimit), requested: bound(protocol.CollaborationRecordItemLimit)},
		// Completed is the sender's observation; the server still exposes missing retained messages.
		{name: "completed observation with missing retained tail", reason: "completed", retained: 1, observed: bound(2), requested: bound(2)},
		{name: "old completed seal ACK retains later known prefix", reason: "completed", retained: 2, observed: bound(2), requested: bound(1)},
		{name: "stopped unknown tail", reason: "stopped", retained: 1},
		{name: "stopped known missing tail", reason: "stopped", retained: 1, observed: bound(3), requested: bound(3)},
		{name: "transport uncertainty", reason: "transport_error"},
		{name: "fully retained stopped prefix", reason: "stopped", retained: 1, observed: bound(1), requested: bound(1)},
		{name: "bounded retained prefix with unknown tail", reason: "incomplete", retained: protocol.CollaborationRecordItemLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: f.claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), StreamID: uuid.NewString(), ObservedThrough: tc.requested, Reason: tc.reason}
			want := protocol.CollaborationRecordingRetention{Version: 1, StreamID: seal.StreamID, Sealed: true, Reason: tc.reason, RetainedThrough: tc.retained, ObservedThrough: tc.observed}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(want); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.RecordingSeal(context.Background(), seal, f.claim.TerminationToken)
			if err != nil || !reflect.DeepEqual(out, want) {
				t.Fatalf("truthful seal receipt rejected: out=%+v want=%+v err=%v", out, want, err)
			}
		})
	}
}

func TestClientRecordingSealRejectsMissingRetentionBounds(t *testing.T) {
	f := newHostRunFixture(t, "success")
	seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: f.claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), StreamID: uuid.NewString(), Reason: "stopped"}
	for _, tc := range []struct {
		name  string
		omit  string
		nulls bool
	}{
		{name: "missing retained", omit: "retained_through"},
		{name: "null retained", nulls: true},
		{name: "missing observed", omit: "observed_through"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"version": 1, "stream_id": seal.StreamID, "sealed": true, "reason": seal.Reason, "retained_through": 0, "observed_through": nil}
			if tc.omit != "" {
				delete(body, tc.omit)
			}
			if tc.nulls {
				body["retained_through"] = nil
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if err := json.NewEncoder(w).Encode(body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "mdt_fixture", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.RecordingSeal(context.Background(), seal, f.claim.TerminationToken)
			if !errors.Is(err, ErrProtocol) || !reflect.DeepEqual(out, protocol.CollaborationRecordingRetention{}) {
				t.Fatalf("missing required recording bounds became an ACK: out=%+v err=%v", out, err)
			}
		})
	}
}
