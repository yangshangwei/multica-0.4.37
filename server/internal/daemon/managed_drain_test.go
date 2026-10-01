package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagedDrainStopsNewClaimsAndSurvivesRequestCancellation(t *testing.T) {
	identity, handoff := managedHandoffFixture(t)
	d := New(Config{ManagementDeploymentID: handoff.DeploymentID, WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.managed = &managedDaemonLifecycle{identity: identity, controlToken: strings.Repeat("A", 43), store: &ManagedCredentialStore{Bindings: []ManagedStoredBinding{}}}
	d.ready.Store(true)
	root, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.rootCtx = root
	d.cancelFunc = cancel
	if !d.tryEnterClaim() {
		t.Fatal("claim should start before drain")
	}
	intentA := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	intentB := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	shutdown := func(intent string, expected *string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(struct {
			Intent   string  `json:"intent_id"`
			Expected *string `json:"expected_intent_id"`
		}{intent, expected})
		ctx, abort := context.WithCancel(context.Background())
		request := httptest.NewRequest("POST", "/shutdown", bytes.NewReader(body)).WithContext(ctx)
		request.Header, _ = ManagementRequestHeaders(d.managed.controlToken, request.Method, request.URL.Path, body)
		response := httptest.NewRecorder()
		d.shutdownHandler()(response, request)
		abort()
		if response.Code == 202 {
			if err := VerifyManagementResponse(d.managed.controlToken, request.Method, request.URL.Path, request.Header, response.Code, response.Body.Bytes(), response.Header()); err != nil {
				t.Fatal(err)
			}
		}
		return response
	}
	if got := shutdown(intentA, nil); got.Code != 202 {
		t.Fatalf("busy drain = %d, want accepted202", got.Code)
	}
	if d.tryEnterClaim() {
		t.Fatal("drain admitted new claim")
	}
	var admitted atomic.Int32
	queueCtx, stopQueue := context.WithCancel(context.Background())
	defer stopQueue()
	queueDone := make(chan struct{})
	go func() {
		defer close(queueDone)
		for queueCtx.Err() == nil {
			if d.tryEnterClaim() {
				admitted.Add(1)
				d.exitClaim()
			}
			time.Sleep(time.Millisecond)
		}
	}()
	// A previously admitted claim still dispatches its task before releasing the claim gate.
	d.activeTasks.Add(1)
	d.exitClaim()
	if got := shutdown(intentB, nil); got.Code != 409 {
		t.Fatalf("stale expected intent accepted: %d", got.Code)
	}
	if got := shutdown(intentB, &intentA); got.Code != 202 {
		t.Fatalf("CAS replacement rejected: %d", got.Code)
	}
	if got := shutdown(intentA, nil); got.Code != 409 {
		t.Fatal("stale retry changed newer drain")
	}
	d.releaseClaimBarrier()
	if d.tryEnterClaim() {
		t.Fatal("other barrier release reopened draining claims")
	}
	select {
	case <-root.Done():
		t.Fatal("drain cancelled active work")
	case <-time.After(60 * time.Millisecond):
	}
	d.activeTasks.Add(-1)
	select {
	case <-root.Done():
	case <-time.After(time.Second):
		t.Fatal("drain did not complete independently of cancelled request")
	}
	stopQueue()
	<-queueDone
	if admitted.Load() != 0 {
		t.Fatalf("continuous queued work admitted %d claims after drain", admitted.Load())
	}
}
