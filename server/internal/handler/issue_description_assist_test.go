package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/llm"
)

func descriptionAssistHandler(t *testing.T, status int, content string) (*Handler, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["tools"] != nil {
			t.Error("assist must not enable tools")
		}
		if request["model"] != "assist-test-model" {
			t.Errorf("assist must use deployment model: %v", request["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}, "finish_reason": "stop"}}}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	retries, err := llm.Retries(0)
	if err != nil {
		t.Fatal(err)
	}
	return &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{APIKey: "test", BaseURL: srv.URL, DefaultModel: "assist-test-model", MaxRetries: retries})}, calls
}

func TestOptimizeIssueDescription(t *testing.T) {
	for _, mode := range []string{"manual", "agent"} {
		t.Run(mode, func(t *testing.T) {
			h, calls := descriptionAssistHandler(t, http.StatusOK, `{"text":"保留手机号登录，同时支持微信登录。","questions":["是否绑定已有账号？"]}`)
			var response map[string]any
			testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "微信登录也要手机号", "title": "登录", "mode": mode})).Want(http.StatusOK).JSON(&response)
			if response["text"] != "保留手机号登录，同时支持微信登录。" || calls.Load() != 1 {
				t.Fatalf("unexpected result: %+v, calls %d", response, calls.Load())
			}
		})
	}
}

func TestOptimizeIssueDescriptionDraftingResponses(t *testing.T) {
	for _, tc := range []struct {
		name      string
		text      string
		questions []string
	}{
		{"actionable single sentence", "将登录按钮文案改为登录。", []string{}},
		{"blocking questions", "修复登录失败的问题。", []string{"哪个登录方式失败？", "失败时显示什么错误？"}},
		// Keep older provider responses usable; the prompt asks for at most two.
		{"legacy question count", "完善登录流程。", []string{"登录方式？", "目标平台？", "账号绑定？", "失败提示？", "验收方式？"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := json.Marshal(optimizeIssueDescriptionResponse{Text: tc.text, Questions: tc.questions})
			if err != nil {
				t.Fatal(err)
			}
			h, _ := descriptionAssistHandler(t, http.StatusOK, string(output))
			var response optimizeIssueDescriptionResponse
			testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": tc.text, "mode": "manual"})).Want(http.StatusOK).JSON(&response)
			if response.Text != tc.text || response.Questions == nil || len(response.Questions) != len(tc.questions) {
				t.Fatalf("draft or questions changed: %+v", response)
			}
			for i, question := range tc.questions {
				if response.Questions[i] != question {
					t.Fatalf("question %d = %q, want %q", i, response.Questions[i], question)
				}
			}
		})
	}
}

func TestOptimizeIssueDescriptionRejectsInvalidRequest(t *testing.T) {
	h, calls := descriptionAssistHandler(t, http.StatusOK, `{"text":"ok","questions":[]}`)
	for _, body := range []any{
		map[string]any{"text": " ", "mode": "manual"},
		map[string]any{"text": "ok", "mode": "other"},
		map[string]any{"text": strings.Repeat("中", 20001), "mode": "manual"},
		map[string]any{"text": "ok", "title": strings.Repeat("a", 501), "mode": "manual"},
	} {
		testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", body)).Want(http.StatusBadRequest)
	}
	for _, body := range []string{`{"text":"ok","mode":"manual"} {}`, strings.Repeat(" ", 128*1024) + `{}`} {
		req := testutil.WithHeaders(testutil.JSONRequest(http.MethodPost, "/api/issues/optimize-description", body), "X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID)
		testutil.Call(t, h.OptimizeIssueDescription, req).Want(http.StatusBadRequest)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input reached provider")
	}
}

func TestOptimizeIssueDescriptionAuthorization(t *testing.T) {
	h, calls := descriptionAssistHandler(t, http.StatusOK, `{"text":"ok","questions":[]}`)
	for _, tc := range []struct {
		user, ws string
		status   int
	}{
		{"", testWorkspaceID, http.StatusUnauthorized},
		{testUserID, "00000000-0000-0000-0000-000000000001", http.StatusNotFound},
	} {
		req := newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "ok", "mode": "manual"})
		req.Header.Set("X-User-ID", tc.user)
		req.Header.Set("X-Workspace-ID", tc.ws)
		testutil.Call(t, h.OptimizeIssueDescription, req).Want(tc.status)
	}
	if calls.Load() != 0 {
		t.Fatal("unauthorized request reached provider")
	}
}

func TestOptimizeIssueDescriptionProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name, output, code string
		upstream, status   int
	}{
		{"provider", `{}`, "ai_generation_failed", http.StatusBadGateway, http.StatusBadGateway},
		{"invalid json", `not JSON`, "ai_invalid_output", http.StatusOK, http.StatusBadGateway},
		{"empty text", `{"text":" ","questions":[]}`, "ai_invalid_output", http.StatusOK, http.StatusBadGateway},
		{"missing questions", `{"text":"ok"}`, "ai_invalid_output", http.StatusOK, http.StatusBadGateway},
		{"wrong questions", `{"text":"ok","questions":[4]}`, "ai_invalid_output", http.StatusOK, http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := descriptionAssistHandler(t, tc.upstream, tc.output)
			response := testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "ok", "mode": "manual"})).Want(tc.status).Map()
			if response["code"] != tc.code {
				t.Fatalf("unexpected error: %+v", response)
			}
		})
	}
	h := &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{})}
	testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "ok", "mode": "manual"})).Want(http.StatusServiceUnavailable)
}

func TestOptimizeIssueDescriptionCanceled(t *testing.T) {
	h, calls := descriptionAssistHandler(t, http.StatusOK, `{"text":"ok","questions":[]}`)
	req := newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "ok", "mode": "manual"})
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	testutil.Call(t, h.OptimizeIssueDescription, req.WithContext(ctx))
	if calls.Load() != 0 {
		t.Fatal("canceled request reached provider")
	}
}

func TestOptimizeIssueDescriptionBusy(t *testing.T) {
	h, calls := descriptionAssistHandler(t, http.StatusOK, `{"text":"ok","questions":[]}`)
	for i := 0; i < cap(descriptionAssistSlots); i++ {
		descriptionAssistSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(descriptionAssistSlots); i++ {
			<-descriptionAssistSlots
		}
	}()
	response := testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "ok", "mode": "manual"})).Want(http.StatusTooManyRequests).Map()
	if response["code"] != "ai_busy" || calls.Load() != 0 {
		t.Fatalf("unexpected admission: %+v", response)
	}
}

func TestOptimizeIssueDescriptionDeadline(t *testing.T) {
	release := make(chan struct{})
	upstreamCanceled := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			close(upstreamCanceled)
		case <-release:
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	h := &Handler{Queries: testHandler.Queries, LLM: llm.New(llm.Config{APIKey: "test", BaseURL: srv.URL})}
	req := newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "ok", "mode": "manual"})
	ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
	defer cancel()
	response := testutil.Call(t, h.OptimizeIssueDescription, req.WithContext(ctx)).Want(http.StatusGatewayTimeout).Map()
	if response["code"] != "ai_timeout" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(descriptionAssistSlots) != 0 {
		t.Fatal("request leaked its admission slot")
	}
	select {
	case <-upstreamCanceled:
	case <-time.After(time.Second):
		t.Fatal("deadline did not cancel the upstream HTTP request")
	}
}

func TestOptimizeIssueDescriptionRejectsChangedReferences(t *testing.T) {
	h, _ := descriptionAssistHandler(t, http.StatusOK, `{"text":"Visit the docs.","questions":[]}`)
	response := testutil.Call(t, h.OptimizeIssueDescription, newRequest(http.MethodPost, "/api/issues/optimize-description", map[string]any{"text": "Visit [docs](https://example.com)", "mode": "manual"})).Want(http.StatusBadGateway).Map()
	if response["code"] != "ai_invalid_output" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestDescriptionAssistPreservesMarkdown(t *testing.T) {
	for _, tc := range []struct {
		name, source, result string
		valid                bool
	}{
		{"prose", "rough input", "Clear input", true},
		{"link label", "[old](https://example.com/a)", "[new](https://example.com/a)", true},
		{"link lost", "[old](https://example.com/a)", "new", false},
		{"attachment changed", "![file](/api/attachments/abc)", "![file](/api/attachments/xyz)", false},
		{"file card preserved", "!file[report.pdf](/uploads/report.pdf)", "Clear description\n\n!file[report.pdf](/uploads/report.pdf)", true},
		{"file card removed", "!file[report.pdf](/uploads/report.pdf)", "[report.pdf](/uploads/report.pdf)", false},
		{"file card inline", "!file[report.pdf](/uploads/report.pdf)", "See !file[report.pdf](/uploads/report.pdf)", false},
		{"file card renamed", "!file[report.pdf](/uploads/report.pdf)", "!file[other.pdf](/uploads/report.pdf)", false},
		{"file card escaped filename", "!file[report\\[final\\].pdf](https://example.test/report.pdf)", "!file[report.pdf](https://example.test/report.pdf)", false},
		{"file card authenticated URL", "!file[report.pdf](/api/attachments/11111111-2222-3333-4444-555555555555/download)", "[report.pdf](/api/attachments/11111111-2222-3333-4444-555555555555/download)", false},
		{"file card invented", "[report.pdf](/uploads/report.pdf)", "!file[report.pdf](/uploads/report.pdf)", false},
		{"mention lost", "[@Alex](mention://member/abc)", "Alex", false},
		{"mention label swapped", "[@Alex](mention://member/abc) [@Sam](mention://member/def)", "[@Sam](mention://member/abc) [@Alex](mention://member/def)", false},
		{"inline code", "Use `foo()`", "Call `bar()`", false},
		{"fenced code", "```go\nx := 1\n```", "```go\nx := 2\n```", false},
		{"code language swapped", "```go\nx\n```\n\n```js\ny\n```", "```js\nx\n```\n\n```go\ny\n```", false},
		{"raw URL", "See https://example.com", "See https://elsewhere.com", false},
		{"new URL", "Do this", "Do [this](https://evil.example)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := descriptionMarkdownPreserved(tc.source, tc.result); got != tc.valid {
				t.Fatalf("preserved=%v want %v", got, tc.valid)
			}
		})
	}
}
