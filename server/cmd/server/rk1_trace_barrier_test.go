//go:build agentintegration && !windows

package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRK1TraceSnapshotWaitsForCompletedHandler(t *testing.T) {
	wrote, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	trace := &rk1Trace{next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "2")
		_, _ = w.Write([]byte(`{}`))
		close(wrote)
		<-release
	})}
	go func() {
		defer close(finished)
		trace.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/collaboration/runtime/runs/run/recording/seal", nil))
	}()
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); <-finished })
	<-wrote
	snapshot := make(chan []rk1Call, 1)
	go func() { snapshot <- trace.snapshot() }()
	select {
	case calls := <-snapshot:
		t.Fatalf("snapshot finished before its response handler: %d captured calls", len(calls))
	case <-time.After(25 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case calls := <-snapshot:
		if len(calls) != 1 || calls[0].Status != http.StatusOK || calls[0].Path != "/api/collaboration/runtime/runs/run/recording/seal" || string(calls[0].Response) != `{}` {
			t.Fatalf("completed handler was not captured exactly once: %+v", calls)
		}
	case <-time.After(time.Second):
		t.Fatal("snapshot remained blocked after the response handler finished")
	}
}
