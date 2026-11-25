package middleware

import (
	"bytes"
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// withCapturedLogs swaps the default slog logger for one that writes to buf,
// then restores it on cleanup. Returns the buffer so tests can inspect what
// RequestLogger emitted.
//
// Uses a shared mutex because t.Parallel tests would otherwise race on the
// global slog.Default — tests in this file intentionally do NOT run in
// parallel for that reason.
var defaultLoggerMu sync.Mutex

func withCapturedLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	defaultLoggerMu.Lock()
	buf := &bytes.Buffer{}
	orig := slog.Default()
	legacyWriter, legacyFlags := log.Writer(), log.Flags()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(orig)
		log.SetOutput(legacyWriter)
		log.SetFlags(legacyFlags)
		defaultLoggerMu.Unlock()
	})
	return buf
}

func runRequestLogger(t *testing.T, status int, body string) *bytes.Buffer {
	t.Helper()
	logs := withCapturedLogs(t)
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/heartbeat", nil).
		WithContext(context.Background())
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return logs
}

// requireLogLevel asserts that the captured output contains exactly the
// expected slog level prefix and not any of the disallowed ones.
func requireLogLevel(t *testing.T, logs *bytes.Buffer, want string, disallowed ...string) {
	t.Helper()
	out := logs.String()
	if !strings.Contains(out, "level="+want) {
		t.Fatalf("expected level=%s in logs, got:\n%s", want, out)
	}
	for _, dis := range disallowed {
		if strings.Contains(out, "level="+dis) {
			t.Fatalf("did not expect level=%s in logs, got:\n%s", dis, out)
		}
	}
}

func TestRequestLogger_RuntimeNotFound404DowngradesToInfo(t *testing.T) {
	// The whole reason this middleware change exists: a flood of WRN lines
	// after a runtime is deleted (issue #2391). The daemon catches the same
	// body and self-heals, so the line is signal-not-noise.
	logs := runRequestLogger(t, http.StatusNotFound, `{"error":"runtime not found"}`)
	requireLogLevel(t, logs, "INFO", "WARN", "ERROR")
}

func TestRequestLogger_TaskNotFound404DowngradesToInfo(t *testing.T) {
	logs := runRequestLogger(t, http.StatusNotFound, `{"error":"task not found"}`)
	requireLogLevel(t, logs, "INFO", "WARN", "ERROR")
}

func TestRequestLogger_GenericNotFound404KeepsWarn(t *testing.T) {
	// A 404 with an unfamiliar body is still a real 404 — most likely a
	// daemon hitting a wrong path, which is what Warn is for. We do NOT
	// want to downgrade these blindly.
	logs := runRequestLogger(t, http.StatusNotFound, `{"error":"not found"}`)
	requireLogLevel(t, logs, "WARN", "INFO", "ERROR")
}

func TestRequestLogger_400StaysWarn(t *testing.T) {
	logs := runRequestLogger(t, http.StatusBadRequest, `{"error":"bad input"}`)
	requireLogLevel(t, logs, "WARN", "INFO", "ERROR")
}

func TestRequestLogger_500StaysError(t *testing.T) {
	logs := runRequestLogger(t, http.StatusInternalServerError, `{"error":"boom"}`)
	requireLogLevel(t, logs, "ERROR", "WARN", "INFO")
}

func TestRequestLogger_ContextAtCompletion(t *testing.T) {
	const privateCause = "private-request-context-cause"
	for _, state := range []string{"none", "canceled", "deadline"} {
		t.Run(state, func(t *testing.T) {
			logs := withCapturedLogs(t)
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			if state == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadlineCause(ctx, time.Now().Add(-time.Second), errors.New(privateCause))
				defer stop()
			}
			req := httptest.NewRequest(http.MethodGet, "/api/me", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if state == "canceled" {
					if r.Context().Err() != nil {
						t.Fatal("request must be live before the handler cancels it")
					}
					cancel(errors.New(privateCause))
				}
				err := r.Context().Err()
				if err == nil {
					err = errors.New("synthetic database unavailable")
				}
				passwordAuthError(w, err)
			})).ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != "{\"code\":\"auth_unavailable\",\"error\":\"Service Unavailable\"}\n" {
				t.Fatalf("response changed: status=%d body=%q", rec.Code, rec.Body.String())
			}
			requireLogLevel(t, logs, "ERROR", "WARN", "INFO")
			if !strings.Contains(logs.String(), "request_context="+state) || !strings.Contains(logs.String(), "status=503") {
				t.Fatalf("missing completed context or attempted status: %s", logs.String())
			}
			if strings.Contains(logs.String(), privateCause) || strings.Contains(logs.String(), "synthetic database unavailable") {
				t.Fatalf("private error detail leaked: %s", logs.String())
			}
		})
	}
}

func TestRequestLogger_ClientCancellationKeepsAttemptedStatus(t *testing.T) {
	logs := withCapturedLogs(t)
	entered, finished := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-r.Context().Done()
			passwordAuthError(w, r.Context().Err())
		})).ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		response, err := server.Client().Do(req)
		if response != nil {
			_ = response.Body.Close()
		}
		result <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("server did not receive request")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("client received %v instead of cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client did not finish cancellation")
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not observe client cancellation")
	}
	requireLogLevel(t, logs, "ERROR", "WARN", "INFO")
	if !strings.Contains(logs.String(), "request_context=canceled") || !strings.Contains(logs.String(), "status=503") {
		t.Fatalf("missing cancellation or attempted status: %s", logs.String())
	}
}

func TestRequestLogger_200StaysInfo(t *testing.T) {
	logs := runRequestLogger(t, http.StatusOK, `{"ok":true}`)
	requireLogLevel(t, logs, "INFO", "WARN", "ERROR")
}

func TestRequestLogger_HealthEndpointIsSkipped(t *testing.T) {
	logs := withCapturedLogs(t)
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if logs.Len() != 0 {
		t.Fatalf("/health should not be logged, got:\n%s", logs.String())
	}
}

func TestRequestLogger_BodyStillReachesClient(t *testing.T) {
	// The body capture is implemented via Tee, which must mirror writes
	// rather than swallow them. Regress-protect: assert the response writer
	// still gets the full body.
	rec := httptest.NewRecorder()
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"runtime not found"}`))
	}))
	_ = withCapturedLogs(t)
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/daemon/heartbeat", nil))
	if got := rec.Body.String(); got != `{"error":"runtime not found"}` {
		t.Fatalf("response body lost or mutated: got %q", got)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestRequestLogger_LargeBodyBeyondCaptureLimit(t *testing.T) {
	// If the soft-404 marker only appears beyond the capture limit we
	// intentionally keep Warn — capturing arbitrary-size bodies is the
	// memory blowup we are guarding against. This test pins that
	// trade-off.
	prefix := strings.Repeat("x", softNotFoundBodyCaptureLimit+8)
	logs := runRequestLogger(t, http.StatusNotFound, prefix+`{"error":"runtime not found"}`)
	requireLogLevel(t, logs, "WARN", "INFO", "ERROR")
}

func TestRedactWebhookPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"/api/webhooks/autopilots/awt_secret", "/api/webhooks/autopilots/[redacted]"},
		{"/api/webhooks/autopilots/awt_secret/", "/api/webhooks/autopilots/[redacted]/"},
		{"/api/webhooks/autopilots/", "/api/webhooks/autopilots/"},
		{"/api/webhooks/github", "/api/webhooks/github"},
		{"/api/runtimes/abc", "/api/runtimes/abc"},
		{"/", "/"},
	}
	for _, tc := range cases {
		if got := redactWebhookPath(tc.in); got != tc.want {
			t.Errorf("redactWebhookPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRequestLogger_RedactsWebhookTokenInPath(t *testing.T) {
	logs := withCapturedLogs(t)
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/autopilots/awt_supersecret", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	out := logs.String()
	if strings.Contains(out, "awt_supersecret") {
		t.Fatalf("token leaked into logs:\n%s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("expected [redacted] in logs:\n%s", out)
	}
}

func TestRequestLogger_IncludesWebhookTriggerIDFromContext(t *testing.T) {
	// Exercise the real production flow: the webhook handler resolves the
	// trigger, then calls SetWebhookTriggerID(r, ...) which mutates *r in
	// place. After the handler returns, the wrapping RequestLogger
	// middleware reads the stashed ID off the (now-updated) request
	// context. If SetWebhookTriggerID didn't mutate in place, the
	// middleware would see the old context and the trigger ID would
	// silently drop from the audit line.
	logs := withCapturedLogs(t)
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetWebhookTriggerID(r, "trigger-abc")
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/autopilots/awt_supersecret", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	out := logs.String()
	if !strings.Contains(out, "webhook_trigger_id=trigger-abc") {
		t.Fatalf("expected webhook_trigger_id in logs, got:\n%s", out)
	}
	if strings.Contains(out, "awt_supersecret") {
		t.Fatalf("token leaked into logs:\n%s", out)
	}
}

func TestIsSoftNotFound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		body string
		want bool
	}{
		{`{"error":"runtime not found"}`, true},
		{`{"error":"task not found"}`, true},
		{`{"error":"Runtime Not Found"}`, true},
		{`{"error":"not found"}`, false},
		{`{"error":"workspace not found"}`, false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isSoftNotFound([]byte(tc.body)); got != tc.want {
			t.Errorf("isSoftNotFound(%q) = %v, want %v", tc.body, got, tc.want)
		}
	}
}
