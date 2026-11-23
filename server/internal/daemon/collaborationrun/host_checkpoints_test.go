package collaborationrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestHostCheckpointsKeepOriginalPayloadAcrossLostACK(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	hostTestPermit(t, local)
	var mu sync.Mutex
	requests := []protocol.CollaborationRecordingCheckpoint{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p protocol.CollaborationRecordingCheckpoint
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		requests = append(requests, p)
		count := len(requests)
		mu.Unlock()
		if count == 1 {
			http.Error(w, "lost response", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(protocol.CollaborationCheckpointReceipt{Version: 1, Identity: p.Identity, StreamID: p.StreamID, Sequence: p.Sequence, RetainedThrough: p.MessageOffset + len(p.Messages), Accepted: true})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{Client: client}
	supervisor := &hostSupervisor{runner: runner, local: local}
	record := &hostRecording{}
	sender, err := startHostCheckpoints(t.Context(), supervisor, record, 1)
	if err != nil {
		t.Fatal(err)
	}
	record.add(protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: "kept original"})
	sender.finish()
	if local.record.Host.Recording.Chunks[0].Payload == nil || local.record.Host.Recording.Chunks[0].Accepted {
		t.Fatal("uncertain checkpoint lost original body")
	}
	if err := runner.recoverHostRecording(t.Context(), local); err != nil {
		t.Fatal(err)
	}
	if !local.record.Host.Recording.Chunks[0].Accepted || local.record.Host.Recording.Chunks[0].Payload != nil {
		t.Fatal("accepted payload must be replaced by bounded metadata")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("requests=%d", len(requests))
	}
	a, _ := json.Marshal(requests[0])
	b, _ := json.Marshal(requests[1])
	if string(a) != string(b) {
		t.Fatal("recovery changed original payload/key")
	}
}

func TestHostCheckpointBackpressureDoesNotBlockMessageDrain(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	hostTestPermit(t, local)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	client, err := NewClient(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	supervisor := &hostSupervisor{runner: &Runner{Client: client}, local: local}
	record := &hostRecording{}
	sender, err := startHostCheckpoints(ctx, supervisor, record, 1)
	if err != nil {
		t.Fatal(err)
	}
	for range 16 {
		record.add(protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: "message"})
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("checkpoint was never sent")
	}
	start := time.Now()
	for range 80 {
		record.add(protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: "message"})
	}
	if time.Since(start) > 200*time.Millisecond || !record.partial || !record.closed {
		t.Fatal("backpressure blocked the provider drain or failed to mark a gap")
	}
	cancel()
	sender.finish()
	if !local.record.Host.Recording.Closed {
		t.Fatal("dropped message did not seal stream append")
	}
	through := 0
	for _, c := range local.record.Host.Recording.Chunks {
		if c.Offset != through {
			t.Fatal("checkpoint offsets jumped over a dropped message")
		}
		through += c.Count
	}
}

func TestHostCheckpointLedgerFitsMaximumEscapedRecordingAndClaim(t *testing.T) {
	ledger := newHostTestLedger(t)
	local := registerHostTestRun(t, ledger)
	defer local.Close()
	permit := hostTestPermit(t, local)
	next, err := local.recordingNext()
	if err != nil {
		t.Fatal(err)
	}
	claim := *next.Host.Claim
	claim.Inputs = append([]protocol.CollaborationRunInput{}, claim.Inputs...)
	claim.Inputs[0].Body = strings.Repeat("a", (2<<20)-(64<<10))
	next.Host.Claim = &claim
	if err := local.persist(next); err != nil {
		t.Fatal(err)
	}
	stream := uuid.NewString()
	if err := local.startHostRecording(stream); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		p := protocol.CollaborationRecordingCheckpoint{Version: 1, Identity: local.record.Identity, AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, StreamID: stream, Sequence: i + 1, MessageOffset: i, Messages: []protocol.CollaborationRecordedMessage{{Kind: "message", Role: "assistant", Body: strings.Repeat("<", 64<<10)}}}
		if err := local.stageHostCheckpoint(p, local.record.Host.BusinessToken); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(local.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= 4<<20 || len(data) > maxLedgerBytes {
		t.Fatalf("unexpected bounded ledger size: %d", len(data))
	}
	var decoded ledgerRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := validateHostLedgerRecord(decoded); err != nil {
		t.Fatal(err)
	}
}
