package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestClientCheckpointACKMatchesOriginalRequestBound(t *testing.T) {
	checkpoint := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: testIdentity(), AttemptSeq: "1",
		AttemptNonce: uuid.NewString(), StreamID: uuid.NewString(), Sequence: 1,
		Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: "original"}}}
	for _, through := range []int{0, 1, 2} {
		t.Run(strconv.Itoa(through), func(t *testing.T) {
			want := protocol.CollaborationCheckpointReceipt{Version: 1, Identity: checkpoint.Identity, StreamID: checkpoint.StreamID,
				Sequence: checkpoint.Sequence, RetainedThrough: through, Accepted: true}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(want)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.Checkpoint(context.Background(), checkpoint, "original-business-token")
			if through == 1 {
				if err != nil || out != want {
					t.Fatalf("original checkpoint ACK rejected: out=%+v err=%v", out, err)
				}
			} else if !errors.Is(err, ErrProtocol) || out != (protocol.CollaborationCheckpointReceipt{}) {
				t.Fatalf("receipt did not match the original checkpoint: out=%+v err=%v", out, err)
			}
		})
	}
}

// The exhaustive retention field/count matrix belongs to
// pkg/protocol/collaboration_recording_retention_test.go. These cases exercise
// the HTTP decoder and the additional request/response binding in the client.
func TestClientRecordingSealRejectsInvalidACK(t *testing.T) {
	seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: testIdentity(), AttemptSeq: "1",
		AttemptNonce: uuid.NewString(), StreamID: uuid.NewString(), Reason: "incomplete"}
	for name, mutate := range map[string]func(map[string]any){
		"negative observed": func(v map[string]any) { v["observed_through"] = -1 },
		"missing observed":  func(v map[string]any) { delete(v, "observed_through") },
		"null retained":     func(v map[string]any) { v["retained_through"] = nil },
		"other stream":      func(v map[string]any) { v["stream_id"] = uuid.NewString() },
		"other reason":      func(v map[string]any) { v["reason"] = "stopped" },
		"unsealed":          func(v map[string]any) { v["sealed"], v["reason"] = false, "incomplete" },
	} {
		t.Run(name, func(t *testing.T) {
			body := map[string]any{"version": 1, "stream_id": seal.StreamID, "sealed": true, "reason": seal.Reason,
				"retained_through": 0, "observed_through": nil}
			mutate(body)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.RecordingSeal(context.Background(), seal, "original-termination-token")
			if !errors.Is(err, ErrProtocol) || !reflect.DeepEqual(out, protocol.CollaborationRecordingRetention{}) {
				t.Fatalf("invalid seal response became an ACK: out=%+v err=%v", out, err)
			}
		})
	}
}

func TestClientRecordingSealAcceptsTruthfulRetention(t *testing.T) {
	bound := func(n int) *int { return &n }
	for _, tc := range []struct {
		name      string
		reason    string
		retained  int
		observed  *int
		requested *int
	}{
		{name: "empty complete recording", reason: "completed", observed: bound(0), requested: bound(0)},
		{name: "completed observation with missing retained tail", reason: "completed", retained: 1, observed: bound(2), requested: bound(2)},
		{name: "old completed seal ACK retains later known prefix", reason: "completed", retained: 2, observed: bound(2), requested: bound(1)},
		{name: "stopped unknown tail", reason: "stopped", retained: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seal := protocol.CollaborationRecordingSeal{Version: 1, Sequence: 1, Identity: testIdentity(), AttemptSeq: "1",
				AttemptNonce: uuid.NewString(), StreamID: uuid.NewString(), ObservedThrough: tc.requested, Reason: tc.reason}
			want := protocol.CollaborationRecordingRetention{Version: 1, StreamID: seal.StreamID, Sealed: true, Reason: tc.reason,
				RetainedThrough: tc.retained, ObservedThrough: tc.observed}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(want)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := client.RecordingSeal(context.Background(), seal, "original-termination-token")
			if err != nil || !reflect.DeepEqual(out, want) {
				t.Fatalf("truthful seal receipt rejected: out=%+v want=%+v err=%v", out, want, err)
			}
		})
	}
}
