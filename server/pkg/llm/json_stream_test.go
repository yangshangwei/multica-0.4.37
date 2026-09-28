package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func writeJSONStreamChunk(w http.ResponseWriter, content, finish string) {
	_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":%q}]}\n\n", content, finish)
	w.(http.Flusher).Flush()
}

func TestGenerateJSONStream(t *testing.T) {
	srv := stubUpstream(t, func(w http.ResponseWriter, body map[string]any) {
		if body["stream"] != true || body["model"] != "gpt-5.6-luna" || body["reasoning_effort"] != "none" || body["max_completion_tokens"] != float64(800) {
			t.Errorf("unexpected streaming params: %#v", body)
		}
		if body["temperature"] != nil {
			t.Error("GPT-5.6 must omit temperature")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeJSONStreamChunk(w, `{"text":"`, "")
		writeJSONStreamChunk(w, `hello","questions":[]}`, "stop")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	c := New(Config{APIKey: "test", BaseURL: srv.URL, DefaultModel: "gpt-5.6-luna"})
	var deltas []string
	out, err := c.GenerateJSONStream(context.Background(), "", "Return JSON", "input", 0.3, 800, func(delta string) error { deltas = append(deltas, delta); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 2 || out != `{"text":"hello","questions":[]}` || strings.Join(deltas, "") != out {
		t.Fatalf("out=%q deltas=%#v", out, deltas)
	}
}

func TestGenerateJSONStreamCompatibility(t *testing.T) {
	requests := 0
	srv := stubUpstream(t, func(w http.ResponseWriter, body map[string]any) {
		requests++
		parameter := ""
		if body["max_completion_tokens"] != nil {
			parameter = "max_completion_tokens"
		} else if body["reasoning_effort"] != nil {
			parameter = "reasoning_effort"
		}
		if parameter != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":{"message":"Unsupported parameter","param":%q,"code":"unsupported_parameter"}}`, parameter)
			return
		}
		if body["max_tokens"] != float64(800) {
			t.Errorf("missing legacy token budget: %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		writeJSONStreamChunk(w, `{"text":"ok"}`, "stop")
	})
	c := New(Config{APIKey: "test", BaseURL: srv.URL, MaxRetries: retries(0)})
	if _, err := c.GenerateJSONStream(context.Background(), "gpt-5.6-luna", "JSON", "input", 0, 800, nil); err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("requests=%d want 3", requests)
	}
}

func TestGenerateJSONStreamRejectsPartialAndStopsOnCallbackError(t *testing.T) {
	for _, tc := range []struct {
		name, content, finish string
		reject                bool
	}{
		{"premature EOF", `{"text":"partial`, "", false},
		{"length", `{"text":"partial`, "length", false},
		{"empty", "", "stop", false},
		{"consumer cancelled", `{"text":"partial`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			srv := stubUpstream(t, func(w http.ResponseWriter, _ map[string]any) {
				requests++
				w.Header().Set("Content-Type", "text/event-stream")
				writeJSONStreamChunk(w, tc.content, tc.finish)
			})
			c := New(Config{APIKey: "test", BaseURL: srv.URL, MaxRetries: retries(2)})
			sentinel := errors.New("consumer stop")
			_, err := c.GenerateJSONStream(context.Background(), "", "JSON", "input", 0, 800, func(string) error {
				if tc.reject {
					return sentinel
				}
				return nil
			})
			if err == nil {
				t.Fatal("expected incomplete/rejected stream error")
			}
			if tc.reject && !errors.Is(err, sentinel) {
				t.Fatalf("callback error lost: %v", err)
			}
			if requests != 1 {
				t.Fatalf("restarted partial stream: %d requests", requests)
			}
		})
	}
}
