//go:build agentintegration && !windows

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// rk1Trace wraps the real router. It sees both the daemon's own calls (no
// X-Client-Platform) and every call the run's `multica` CLI makes ("cli"), so
// the exposure evidence comes from what the server actually returned.
type rk1Call struct {
	Seq        int             `json:"seq"`
	At         string          `json:"at"`
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	Query      string          `json:"query,omitempty"`
	Platform   string          `json:"platform,omitempty"`
	Credential string          `json:"credential"`
	Status     int             `json:"status"`
	Request    json.RawMessage `json:"request,omitempty"`
	Response   json.RawMessage `json:"response,omitempty"`
	raw        []byte
}

type rk1Trace struct {
	mu       sync.Mutex
	finished sync.WaitGroup
	next     http.Handler
	calls    []rk1Call
}

type rk1Recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *rk1Recorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *rk1Recorder) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_, _ = w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func (x *rk1Trace) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/collaboration/runtime/") {
		x.next.ServeHTTP(w, r)
		return
	}
	x.finished.Add(1)
	defer x.finished.Done()
	var request []byte
	if r.Body != nil {
		request, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(request))
	}
	rec := &rk1Recorder{ResponseWriter: w}
	x.next.ServeHTTP(rec, r)
	credential, _, _ := strings.Cut(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "_")
	call := rk1Call{At: time.Now().UTC().Format(time.RFC3339Nano), Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
		Platform: r.Header.Get("X-Client-Platform"), Credential: credential, Status: rec.status,
		Request: rk1Scrub(request), Response: rk1Scrub(rec.body.Bytes()), raw: append([]byte(nil), rec.body.Bytes()...)}
	x.mu.Lock()
	call.Seq = len(x.calls) + 1
	x.calls = append(x.calls, call)
	x.mu.Unlock()
}

func (x *rk1Trace) snapshot() []rk1Call {
	// Called after Runner/Provider operations stop. A client can consume a
	// Content-Length response before the handler appends the trace; join those
	// already-started handlers before reading either the trace or DB events.
	x.finished.Wait()
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]rk1Call(nil), x.calls...)
}

// Multica runtime/daemon/agent tokens, Anthropic-style keys and bearer values.
var rk1Secret = regexp.MustCompile(`\b(mcb|mct|mdt|mat|mul)_[A-Za-z0-9_\-]{8,}|\bsk-[A-Za-z0-9_\-]{16,}|(?i)bearer\s+[A-Za-z0-9._\-]{16,}`)

// rk1Scrub removes every credential the runtime protocol carries in bodies
// (claim termination token, permit business token) before evidence is kept.
func rk1Scrub(raw []byte) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		text := rk1Secret.ReplaceAllString(string(raw), "[credential]")
		out, _ := json.Marshal(map[string]string{"non_json": text})
		return out
	}
	out, _ := json.Marshal(rk1ScrubValue("", value))
	return out
}

func rk1ScrubValue(key string, v any) any {
	if strings.HasSuffix(key, "_token") || key == "token" || key == "authorization" {
		return "[credential]"
	}
	switch value := v.(type) {
	case map[string]any:
		for k, child := range value {
			value[k] = rk1ScrubValue(k, child)
		}
		return value
	case []any:
		for i, child := range value {
			value[i] = rk1ScrubValue("", child)
		}
		return value
	case string:
		return rk1Secret.ReplaceAllString(value, "[credential]")
	}
	return v
}
