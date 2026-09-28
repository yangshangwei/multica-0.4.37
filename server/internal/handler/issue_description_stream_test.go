package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/llm"
)

func TestDescriptionTextStream(t *testing.T) {
	for _, raw := range []string{
		`{"text":"中文 \\ \"quoted\"\nnext \uD83D\uDE80","questions":[]}`,
		`{"questions":["text","ignore",{"text":"nested"}],"text":"real"}`,
		`{"text":"\u4e2d\u6587\tline\b\f\r\/"}`,
		`{"te\u0078t":"\ud800x\udc00 \ud800\ud800\udc00"}`,
	} {
		var want struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(raw), &want); err != nil {
			t.Fatal(err)
		}
		for split := 1; split <= len(raw); split++ {
			var extractor descriptionTextStream
			var got strings.Builder
			for start := 0; start < len(raw); start += split {
				end := min(start+split, len(raw))
				delta, err := extractor.Push(raw[start:end])
				if err != nil {
					t.Fatal(err)
				}
				got.WriteString(delta)
			}
			if got.String() != want.Text {
				t.Fatalf("split %d: got %q want %q", split, got.String(), want.Text)
			}
		}
	}
}

func TestDescriptionTextStreamRejectsInvalidOrOversized(t *testing.T) {
	for _, raw := range []string{
		`{"text":"one","text":"two"}`,
		`{"text":"bad\q"}`,
		`{"text":"` + strings.Repeat("a", 30001),
		strings.Repeat(" ", 180001),
	} {
		var extractor descriptionTextStream
		if _, err := extractor.Push(raw); err == nil {
			t.Fatalf("expected rejection for %.50q", raw)
		}
	}
}

func assistStreamChunk(w http.ResponseWriter, text, finish string) {
	raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": text}, "finish_reason": finish}}})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
	w.(http.Flusher).Flush()
}

func TestOptimizeIssueDescriptionStreamsBeforeUpstreamCompletes(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		assistStreamChunk(w, `{"text":"First`, "")
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		assistStreamChunk(w, ` second","questions":["Clarify?"]}`, "stop")
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	h := &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{APIKey: "test", BaseURL: upstream.URL})}
	apiServer := httptest.NewServer(http.HandlerFunc(h.OptimizeIssueDescription))
	t.Cleanup(apiServer.Close)
	req, err := http.NewRequest(http.MethodPost, apiServer.URL, strings.NewReader(`{"text":"original","mode":"manual"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected response %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(response.Body)
	event, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	data, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if event != "event: text_delta\n" || !strings.Contains(data, `"text":"First"`) {
		t.Fatalf("first event %q %q", event, data)
	}
	close(release)
	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rest), "event: done\n") || !strings.Contains(string(rest), `"text":"First second"`) {
		t.Fatalf("missing final completion: %s", rest)
	}
}

func TestOptimizeIssueDescriptionStreamRejectsInvalidFinalResult(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		assistStreamChunk(w, `{"text":"dropped link","questions":[]}`, "stop")
	}))
	t.Cleanup(upstream.Close)
	h := &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{APIKey: "test", BaseURL: upstream.URL})}
	req := newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "[docs](https://example.test)", "mode": "agent"})
	req.Header.Set("Accept", "text/event-stream")
	body := testutil.Call(t, h.OptimizeIssueDescription, req).Want(http.StatusOK).Text()
	if !strings.Contains(body, "event: text_delta") || !strings.Contains(body, `"code":"ai_invalid_output"`) || strings.Contains(body, "event: done") {
		t.Fatalf("unexpected terminal state: %s", body)
	}
}

func TestOptimizeIssueDescriptionStreamFailures(t *testing.T) {
	for _, tc := range []struct {
		name, raw, finish, code string
		status                  int
	}{
		{"provider", "", "", "ai_generation_failed", http.StatusBadRequest},
		{"disconnect", `{"text":"partial`, "", "ai_generation_failed", http.StatusOK},
		{"token limit", `{"text":"partial`, "length", "ai_generation_failed", http.StatusOK},
		{"invalid final", `{"text":"partial","questions":false}`, "stop", "ai_invalid_output", http.StatusOK},
		{"raw limit", strings.Repeat(" ", 180001), "stop", "ai_invalid_output", http.StatusOK},
		{"text limit", `{"text":"` + strings.Repeat("a", 30001), "stop", "ai_invalid_output", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.status != http.StatusOK {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, `{"error":{"message":"private provider diagnostics"}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				assistStreamChunk(w, tc.raw, tc.finish)
			}))
			t.Cleanup(upstream.Close)
			h := &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{APIKey: "test", BaseURL: upstream.URL})}
			req := newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "original", "mode": "manual"})
			req.Header.Set("Accept", "text/event-stream")
			body := testutil.Call(t, h.OptimizeIssueDescription, req).Want(http.StatusOK).Text()
			if !strings.Contains(body, `"code":"`+tc.code+`"`) || strings.Contains(body, "event: done") || strings.Contains(body, "private provider") {
				t.Fatalf("unexpected error stream: %.500s", body)
			}
		})
	}
}

func TestOptimizeIssueDescriptionStreamCancellationAndDeadline(t *testing.T) {
	for _, cancelClient := range []bool{false, true} {
		t.Run(fmt.Sprintf("client_cancel_%t", cancelClient), func(t *testing.T) {
			upstreamCanceled := make(chan struct{})
			release := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				assistStreamChunk(w, `{"text":"partial`, "")
				select {
				case <-r.Context().Done():
					close(upstreamCanceled)
				case <-release:
				}
			}))
			t.Cleanup(upstream.Close)
			t.Cleanup(func() { close(release) })
			h := &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{APIKey: "test", BaseURL: upstream.URL})}
			handlerDone := make(chan struct{})
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
				defer cancel()
				h.OptimizeIssueDescription(w, r.WithContext(ctx))
				close(handlerDone)
			}))
			t.Cleanup(apiServer.Close)
			req, err := http.NewRequest(http.MethodPost, apiServer.URL, strings.NewReader(`{"text":"original","mode":"manual"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-User-ID", testUserID)
			req.Header.Set("X-Workspace-ID", testWorkspaceID)
			req.Header.Set("Accept", "text/event-stream")
			response, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			if cancelClient {
				_ = response.Body.Close()
			} else {
				body, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !strings.Contains(string(body), `"code":"ai_timeout"`) || strings.Contains(string(body), "event: done") {
					t.Fatalf("unexpected timeout body: %s", body)
				}
			}
			select {
			case <-upstreamCanceled:
			case <-time.After(time.Second):
				t.Fatal("upstream request not canceled")
			}
			select {
			case <-handlerDone:
			case <-time.After(time.Second):
				t.Fatal("handler did not finish")
			}
			if len(descriptionAssistSlots) != 0 {
				t.Fatal("request leaked concurrency slot")
			}
		})
	}
}
