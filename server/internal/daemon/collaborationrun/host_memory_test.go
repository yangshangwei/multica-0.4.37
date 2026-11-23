package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// hostStartPrompt decodes the stream-json frame the test-owned provider read
// and returns the prompt text plus its trailing JSON input object.
func hostStartPrompt(t *testing.T, start map[string]any) (string, map[string]json.RawMessage) {
	t.Helper()
	var frame struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	raw, _ := start["input"].(string)
	if err := json.Unmarshal([]byte(raw), &frame); err != nil || len(frame.Message.Content) != 1 {
		t.Fatalf("provider did not receive one prompt frame: %v", err)
	}
	text := frame.Message.Content[0].Text
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text[strings.LastIndex(text, "\n")+1:]), &input); err != nil {
		t.Fatalf("prompt lost its JSON input: %v", err)
	}
	return text, input
}

func TestHostRunnerInjectsSharedContextOnlyIntoThePrompt(t *testing.T) {
	const secret = "Current state: migration 739 verified on the shared clone"
	f := newHostRunFixture(t, "result")
	f.memoryVersion, f.memoryContent = "4", secret
	ledger := f.runner.Ledger.dir
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.starts(t)
	if len(starts) != 1 {
		t.Fatalf("provider started %d times", len(starts))
	}
	text, input := hostStartPrompt(t, starts[0])
	var memory struct {
		Version int64  `json:"version"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input["memory"], &memory); err != nil || memory.Version != 4 || memory.Content != secret {
		t.Fatalf("prompt did not carry the base document: %s", input["memory"])
	}
	if !strings.Contains(text, "read the collaboration memory below") {
		t.Fatal("prompt did not separate shared context from comments")
	}
	brief, err := os.ReadFile(filepath.Join(starts[0]["cwd"].(string), "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(brief), "multica collaboration memory show "+f.claim.Identity.IssueID) || strings.Contains(string(brief), secret) {
		t.Fatal("brief must explain shared context without containing its text")
	}
	_ = filepath.WalkDir(ledger, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), secret) {
			t.Errorf("shared context reached the local ledger: %s", path)
		}
		return nil
	})
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimMemoryVersion != protocol.CollaborationMemoryVersion || f.memoryReads != 1 {
		t.Fatalf("claim declared %d and read shared context %d times", f.claimMemoryVersion, f.memoryReads)
	}
	sent, err := json.Marshal([]any{f.permits, f.preparations, f.terminations, f.outcomeRequests, f.outcomes, f.proofs})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.outcomes) == 0 || len(f.proofs) == 0 || strings.Contains(string(sent), secret) {
		t.Fatal("shared context reached a permit, preparation, termination, outcome or proof payload")
	}
}

func TestHostRunnerRunsWithoutSharedContextWhenTheRunIsUnsupported(t *testing.T) {
	f := newHostRunFixture(t, "result")
	f.memoryFailures = []HTTPError{{Status: http.StatusConflict, Code: "memory_unsupported"}}
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.starts(t)
	if len(starts) != 1 {
		t.Fatalf("unsupported shared context stopped the provider: %d starts", len(starts))
	}
	text, input := hostStartPrompt(t, starts[0])
	if _, ok := input["memory"]; ok || !strings.Contains(text, "Read the issue and its comments before acting.") {
		t.Fatal("unsupported run received shared-context guidance")
	}
	brief, err := os.ReadFile(filepath.Join(starts[0]["cwd"].(string), "CLAUDE.md"))
	if err != nil || strings.Contains(string(brief), "collaboration memory") {
		t.Fatalf("unsupported run brief mentioned shared context: %v", err)
	}
}

func TestHostRunnerSharedContextFailureLeavesTheProviderUnlaunched(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failures []HTTPError
		reads    int
		starts   int
	}{
		{"refused", []HTTPError{{Status: http.StatusForbidden, Code: "business_capability_required"}}, 1, 0},
		{"exhausted", []HTTPError{{Status: http.StatusServiceUnavailable}, {Status: http.StatusServiceUnavailable}}, 2, 0},
		{"transient", []HTTPError{{Status: http.StatusServiceUnavailable}}, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHostRunFixture(t, "result")
			f.memoryFailures = tc.failures
			err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
			if starts := len(f.starts(t)); starts != tc.starts {
				t.Fatalf("provider started %d times, want %d", starts, tc.starts)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.memoryReads != tc.reads {
				t.Fatalf("shared context read %d times, want %d", f.memoryReads, tc.reads)
			}
			if tc.starts == 1 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var refusal *HTTPError
			if !errors.As(err, &refusal) || refusal.Status != tc.failures[0].Status {
				t.Fatalf("missing shared context did not fail the run: %v", err)
			}
			if termination, ok := f.terminations["1"]; !ok || termination.Kind != "not_started" || len(f.outcomes) != 0 {
				t.Fatalf("unlaunched attempt was not settled as not_started: %+v", f.terminations)
			}
		})
	}
}

func seedHostResume(t *testing.T, f *hostRunFixture) {
	t.Helper()
	key, err := hostSessionKey(f.claim)
	if err != nil {
		t.Fatal(err)
	}
	s, err := f.runner.Ledger.lockHostSession(key, f.claim.Identity.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.save(hostFixtureSession, false); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
}

// The server freezes the base (RuntimeMemoryBase); this proves only that the
// fallback reads it again and injects what the server returns.
func TestHostRunnerFreshRetryRereadsTheSharedContextBase(t *testing.T) {
	f := newHostRunFixture(t, "reject_resume")
	seedHostResume(t, f)
	f.memoryVersion, f.memoryContent = "2", "Next steps: rerun the handler suite"
	if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.starts(t)
	if len(starts) != 2 {
		t.Fatalf("fresh retry started %d times", len(starts))
	}
	for i, start := range starts {
		text, input := hostStartPrompt(t, start)
		if !strings.Contains(string(input["memory"]), `"version":2`) || i == 1 && !strings.HasPrefix(text, hostContinuityNotice) {
			t.Fatalf("launch %d lost its shared-context base: %s", i+1, input["memory"])
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.memoryReads != 2 {
		t.Fatalf("fresh retry read shared context %d times, want 2", f.memoryReads)
	}
}

func TestHostRunnerFreshRetryWithoutSharedContextNeverStarts(t *testing.T) {
	f := newHostRunFixture(t, "reject_resume")
	seedHostResume(t, f)
	f.memoryFailures = []HTTPError{{}, {Status: http.StatusConflict, Code: "execution_stopped"}}
	err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request)
	var refusal *HTTPError
	if !errors.As(err, &refusal) || refusal.Code != "execution_stopped" {
		t.Fatalf("fallback without its base did not fail: %v", err)
	}
	if starts := len(f.starts(t)); starts != 1 {
		t.Fatalf("fallback started without shared context: %d starts", starts)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, permitted := f.permits["2"]; permitted {
		t.Fatal("fallback requested a launch permit after its base was refused")
	}
}

func TestClientRejectsMalformedSharedContextBase(t *testing.T) {
	id := testIdentity()
	valid := protocol.CollaborationRunMemory{IssueID: id.IssueID, RunID: id.RunID, Version: "3", Content: "state", Bytes: 5, MaxBytes: 64 << 10}
	for _, tc := range []struct {
		name   string
		mutate func(*protocol.CollaborationRunMemory)
	}{
		{"other_run", func(m *protocol.CollaborationRunMemory) { m.RunID = id.SessionID }},
		{"other_issue", func(m *protocol.CollaborationRunMemory) { m.IssueID = id.SessionID }},
		{"noncanonical_version", func(m *protocol.CollaborationRunMemory) { m.Version = "03" }},
		{"empty_version_with_text", func(m *protocol.CollaborationRunMemory) { m.Version = "0" }},
		{"byte_count", func(m *protocol.CollaborationRunMemory) { m.Bytes = 4 }},
		// The byte count must match so only the NUL check can refuse it.
		{"nul", func(m *protocol.CollaborationRunMemory) { m.Content = "st\x00te"; m.Bytes = len(m.Content) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := valid
			tc.mutate(&document)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(document)
			}))
			defer s.Close()
			c, err := NewClient(s.URL, "mdt_test", nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.MemoryBase(context.Background(), id, "mcb_"+strings.Repeat("a", 64)); !errors.Is(err, ErrProtocol) {
				t.Fatalf("malformed shared context was accepted: %v", err)
			}
		})
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "memory_unsupported", "error": "free text is discarded"})
	}))
	defer s.Close()
	c, err := NewClient(s.URL, "mdt_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	var refusal *HTTPError
	if _, err := c.MemoryBase(context.Background(), id, "mcb_"+strings.Repeat("a", 64)); !errors.As(err, &refusal) || refusal.Status != http.StatusConflict || refusal.Code != "memory_unsupported" {
		t.Fatalf("refusal code was not preserved: %v", err)
	}
}
