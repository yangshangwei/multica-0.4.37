package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func tokenCount(value int64) *int64 { return &value }

func TestGenerateJSONResultUsagePresence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage string
		want  JSONUsage
	}{
		{"actual zero", `,"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}`,
			JSONUsage{Status: UsageKnown, InputTokens: tokenCount(0), OutputTokens: tokenCount(0), TotalTokens: tokenCount(0), CachedInputTokens: tokenCount(0), ReasoningTokens: tokenCount(0)}},
		{"complete primary counts", `,"usage":{"prompt_tokens":20,"completion_tokens":5,"total_tokens":25}`,
			JSONUsage{Status: UsageKnown, InputTokens: tokenCount(20), OutputTokens: tokenCount(5), TotalTokens: tokenCount(25)}},
		{"missing usage", "", JSONUsage{Status: UsageUnknown}},
		{"null usage", `,"usage":null`, JSONUsage{Status: UsageUnknown}},
		{"empty usage", `,"usage":{}`, JSONUsage{Status: UsageUnknown}},
		{"partial counts do not infer total", `,"usage":{"prompt_tokens":20,"completion_tokens":5}`,
			JSONUsage{Status: UsagePartial, InputTokens: tokenCount(20), OutputTokens: tokenCount(5)}},
		{"only breakdown counts", `,"usage":{"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":2}}`,
			JSONUsage{Status: UsagePartial, CachedInputTokens: tokenCount(4), ReasoningTokens: tokenCount(2)}},
		{"invalid and null are not zero", `,"usage":{"prompt_tokens":null,"completion_tokens":0,"total_tokens":"unknown","prompt_tokens_details":{"cached_tokens":-1}}`,
			JSONUsage{Status: UsagePartial, OutputTokens: tokenCount(0)}},
		{"negative counts are unknown", `,"usage":{"prompt_tokens":-1,"completion_tokens":-1,"total_tokens":-2}`, JSONUsage{Status: UsageUnknown}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requestedModel string
			srv := stubUpstream(t, func(w http.ResponseWriter, body map[string]any) {
				requestedModel, _ = body["model"].(string)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"result","object":"chat.completion","model":" served-model ","choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]`+tc.usage+`}`)
			})
			client := New(Config{BaseURL: srv.URL, DefaultModel: "configured-model", MaxRetries: retries(0)})
			result, err := client.GenerateJSONResult(context.Background(), "", "Return JSON.", "Test input", 0, 100)
			if err != nil {
				t.Fatalf("GenerateJSONResult: %v", err)
			}
			if requestedModel != "configured-model" || result.Model != "served-model" || result.Text != "{}" {
				t.Fatalf("request/result model or text mismatch: request=%q result=%+v", requestedModel, result)
			}
			if !reflect.DeepEqual(result.Usage, tc.want) {
				t.Fatalf("usage=%+v, want %+v", result.Usage, tc.want)
			}
		})
	}
}

func TestGenerateJSONResultDoesNotInventResponseModel(t *testing.T) {
	srv := stubUpstream(t, func(w http.ResponseWriter, _ map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"result","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
	})
	client := New(Config{BaseURL: srv.URL, DefaultModel: "configured-model", MaxRetries: retries(0)})
	result, err := client.GenerateJSONResult(context.Background(), "request-model", "Return JSON.", "Test input", 0, 100)
	if err != nil || result.Model != "" || result.Usage.Status != UsageUnknown {
		t.Fatalf("missing provider model/usage must remain unknown: result=%+v err=%v", result, err)
	}
}

func TestGenerateJSONResultRetainsUsageWhenContentIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name    string
		choices string
		wantErr string
	}{
		{"no choices", `[]`, "no choices"},
		{"length", `[{"index":0,"message":{"role":"assistant","content":"{broken"},"finish_reason":"length"}]`, "max completion token limit"},
		{"empty", `[{"index":0,"message":{"role":"assistant","content":" "},"finish_reason":"stop"}]`, "empty JSON content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := stubUpstream(t, func(w http.ResponseWriter, _ map[string]any) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"result","object":"chat.completion","model":"served-model","choices":`+tc.choices+`,"usage":{"prompt_tokens":7,"completion_tokens":4,"total_tokens":11}}`)
			})
			client := New(Config{BaseURL: srv.URL, MaxRetries: retries(0)})
			result, err := client.GenerateJSONResult(context.Background(), "", "Return JSON.", "Test input", 0, 100)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want %q", err, tc.wantErr)
			}
			want := JSONUsage{Status: UsageKnown, InputTokens: tokenCount(7), OutputTokens: tokenCount(4), TotalTokens: tokenCount(11)}
			if result.Text != "" || result.Model != "served-model" || !reflect.DeepEqual(result.Usage, want) {
				t.Fatalf("rejected content lost provider facts or became usable text: %+v", result)
			}
			legacy, legacyErr := client.GenerateJSON(context.Background(), "", "Return JSON.", "Test input", 0, 100)
			if legacy != "" || legacyErr == nil || legacyErr.Error() != err.Error() {
				t.Fatalf("legacy error contract changed: text=%q err=%v", legacy, legacyErr)
			}
		})
	}
}

func TestGenerateJSONResultUnconfiguredAndTimeoutUsageUnknown(t *testing.T) {
	transport := &countingHTTPClient{}
	disabled := New(Config{HTTPClient: transport, DefaultModel: "configured-only"})
	result, err := disabled.GenerateJSONResult(context.Background(), "", "Return JSON.", "Private input", 0, 100)
	if !errors.Is(err, ErrNotConfigured) || transport.requests.Load() != 0 || result.Model != "" ||
		!reflect.DeepEqual(result.Usage, JSONUsage{Status: UsageUnknown}) {
		t.Fatalf("disabled call did not retain unknown facts without a request: result=%+v err=%v requests=%d", result, err, transport.requests.Load())
	}
	var requests atomic.Int64
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	client := New(Config{BaseURL: srv.URL, MaxRetries: retries(0)})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result, err = client.GenerateJSONResult(ctx, "", "Return JSON.", "Private input", 0, 100)
	if !errors.Is(err, context.DeadlineExceeded) || requests.Load() != 1 || result.Text != "" || result.Model != "" ||
		!reflect.DeepEqual(result.Usage, JSONUsage{Status: UsageUnknown}) {
		t.Fatalf("timeout invented response facts: result=%+v err=%v requests=%d", result, err, requests.Load())
	}
}

func TestGenerateJSONResultSharesParameterNegotiation(t *testing.T) {
	var bodies []map[string]any
	srv := stubUpstream(t, func(w http.ResponseWriter, body map[string]any) {
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		switch len(bodies) % 3 {
		case 1:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Unsupported parameter: max_completion_tokens","type":"invalid_request_error","param":"max_completion_tokens","code":"unsupported_parameter"}}`)
		case 2:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Unsupported parameter: reasoning_effort","type":"invalid_request_error","param":"reasoning_effort","code":"unsupported_parameter"}}`)
		default:
			_, _ = io.WriteString(w, `{"id":"result","object":"chat.completion","model":"served-model","choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}],"usage":{"total_tokens":3}}`)
		}
	})
	client := New(Config{BaseURL: srv.URL, MaxRetries: retries(0)})
	result, err := client.GenerateJSONResult(context.Background(), "gpt-5.6-luna", "Return JSON.", "Test input", 0.3, 800)
	if err != nil || result.Model != "served-model" || result.Usage.Status != UsagePartial || result.Usage.TotalTokens == nil || *result.Usage.TotalTokens != 3 {
		t.Fatalf("typed negotiation: result=%+v err=%v", result, err)
	}
	if _, err = client.GenerateJSON(context.Background(), "gpt-5.6-luna", "Return JSON.", "Test input", 0.3, 800); err != nil {
		t.Fatalf("legacy negotiation: %v", err)
	}
	if len(bodies) != 6 || !reflect.DeepEqual(bodies[:3], bodies[3:]) {
		t.Fatalf("typed and legacy parameter negotiation differ: %#v", bodies)
	}
}

func TestGenerateJSONResultKeepsTransportRetryAndTemperature(t *testing.T) {
	requests := 0
	srv := stubUpstream(t, func(w http.ResponseWriter, body map[string]any) {
		requests++
		if body["temperature"] != float64(0.25) || body["model"] != "gateway-model" || body["max_completion_tokens"] != float64(80) {
			t.Errorf("non-GPT request policy changed: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"message":"retry","type":"server_error"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"result","object":"chat.completion","model":"served-model","choices":[{"index":0,"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`)
	})
	client := New(Config{BaseURL: srv.URL, DefaultModel: "gateway-model", MaxRetries: retries(1)})
	result, err := client.GenerateJSONResult(context.Background(), "", "Return JSON.", "Test input", 0.25, 80)
	if err != nil || requests != 2 || result.Text != "{}" {
		t.Fatalf("transport retry: result=%+v err=%v requests=%d", result, err, requests)
	}
}
