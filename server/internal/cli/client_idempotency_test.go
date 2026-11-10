package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPostJSONWithIdempotencyKeyIsRequestScoped(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		want := ""
		if n == 1 {
			want = "55555555-5555-4555-8555-555555555555"
		}
		if got := r.Header.Get("Idempotency-Key"); got != want {
			t.Errorf("request %d key=%q want=%q", n, got, want)
		}
		if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	client := NewAPIClient(srv.URL, "", "test-token")
	var out map[string]any
	if err := client.PostJSONWithIdempotencyKey(context.Background(), "/keyed", map[string]string{"content": "hello"}, &out, "55555555-5555-4555-8555-555555555555"); err != nil {
		t.Fatal(err)
	}
	if err := client.PostJSON(context.Background(), "/ordinary", map[string]string{"content": "hello"}, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestPostJSONWithIdempotencyKeyDoesNotFollowRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(status)
			}))
			defer srv.Close()
			client := NewAPIClient(srv.URL, "", "test-token")
			err := client.PostJSONWithIdempotencyKey(context.Background(), "/keyed", map[string]string{"content": "hello"}, nil, "55555555-5555-4555-8555-555555555555")
			if err == nil || calls.Load() != 1 {
				t.Fatalf("error=%v calls=%d", err, calls.Load())
			}
		})
	}
}

func TestPostJSONWithIdempotencyKeyDoesNotReplayReusedConnection(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{}`)
			return
		}
		posts.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()
	client := NewAPIClient(srv.URL, "", "test-token")
	var out map[string]any
	if err := client.GetJSON(context.Background(), "/warm", &out); err != nil {
		t.Fatal(err)
	}
	err := client.PostJSONWithIdempotencyKey(context.Background(), "/keyed", map[string]string{"content": "hello"}, &out, "55555555-5555-4555-8555-555555555555")
	if err == nil || posts.Load() != 1 {
		t.Fatalf("error=%v posts=%d", err, posts.Load())
	}
}
