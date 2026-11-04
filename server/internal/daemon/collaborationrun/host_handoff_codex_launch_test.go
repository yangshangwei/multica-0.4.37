package collaborationrun

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func init() {
	if mode := os.Getenv("COLLAB_HANDOFF_CODEX_MODE"); mode != "" {
		os.Exit(runLaunchHandoffCodexFixture(mode))
	}
}

// A test-owned copy of the test binary speaks the ordinary app-server RPC
// protocol. It never discovers a Codex CLI or opens a real account.
func runLaunchHandoffCodexFixture(mode string) int {
	countPath := os.Getenv("COLLAB_HANDOFF_CODEX_COUNTER")
	raw, _ := os.ReadFile(countPath)
	count, _ := strconv.Atoi(string(raw))
	count++
	if os.WriteFile(countPath, []byte(strconv.Itoa(count)), 0600) != nil {
		return 51
	}
	f, err := os.OpenFile(os.Getenv("COLLAB_HOST_TEST_CAPTURE"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 52
	}
	defer f.Close()
	output := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 3<<20)
	for scanner.Scan() {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			return 53
		}
		if len(request.ID) == 0 {
			continue
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{}}
		switch request.Method {
		case "thread/resume":
			if mode == "resume_rejected" {
				delete(response, "result")
				response["error"] = map[string]any{"code": -32602, "message": "unknown thread"}
			} else {
				response["result"] = map[string]any{"thread": map[string]any{"id": hostFixtureSession}}
			}
		case "thread/start":
			thread := map[string]any{"id": hostFixtureSession}
			if strings.HasPrefix(mode, "knowledge_") {
				var params map[string]any
				if json.Unmarshal(request.Params, &params) != nil || params["ephemeral"] != true {
					return 62
				}
				if mode != "knowledge_missing" {
					thread["ephemeral"] = mode != "knowledge_persistent"
				}
				if os.WriteFile(os.Getenv("COLLAB_KNOWLEDGE_CODEX_THREAD"), request.Params, 0600) != nil {
					return 63
				}
			}
			response["result"] = map[string]any{"thread": thread}
		case "turn/start":
			var params struct {
				ThreadID string `json:"threadId"`
				Input    []struct {
					Text string `json:"text"`
				} `json:"input"`
			}
			if json.Unmarshal(request.Params, &params) != nil || len(params.Input) != 1 {
				return 54
			}
			if json.NewEncoder(f).Encode(map[string]any{"method": request.Method, "prompt": params.Input[0].Text, "attempt": count}) != nil {
				return 55
			}
			if output.Encode(response) != nil {
				return 56
			}
			turn := "turn-" + strconv.Itoa(count)
			if output.Encode(map[string]any{"jsonrpc": "2.0", "method": "turn/started", "params": map[string]any{"threadId": params.ThreadID, "turn": map[string]any{"id": turn}}}) != nil {
				return 57
			}
			if mode == "catalog_retry" && count == 1 {
				_, _ = fmt.Fprintln(os.Stderr, "ERROR codex_models_manager::manager: failed to refresh available models: timeout waiting for child process to exit")
				continue
			}
			if output.Encode(map[string]any{"jsonrpc": "2.0", "method": "item/completed", "params": map[string]any{"threadId": params.ThreadID, "item": map[string]any{"type": "agentMessage", "id": "message-1", "text": "Verified the assigned database."}}}) != nil {
				return 58
			}
			if output.Encode(map[string]any{"jsonrpc": "2.0", "method": "turn/completed", "params": map[string]any{"threadId": params.ThreadID, "turn": map[string]any{"id": turn, "status": "completed"}}}) != nil {
				return 59
			}
			continue
		}
		if output.Encode(response) != nil {
			return 60
		}
	}
	if scanner.Err() != nil {
		return 61
	}
	return 0
}

func TestHostHandoffLaunchCodexBindsActualTurnAcrossInternalFallback(t *testing.T) {
	for _, mode := range []string{"resume_rejected", "catalog_retry"} {
		t.Run(mode, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			// The production helper seeds a private Codex home from this
			// disposable configuration source, never the user's account.
			t.Setenv("CODEX_HOME", t.TempDir())
			t.Setenv("COLLAB_HOST_TEST_MODE", "")
			t.Setenv("COLLAB_HANDOFF_CODEX_MODE", mode)
			t.Setenv("COLLAB_HANDOFF_CODEX_COUNTER", filepath.Join(t.TempDir(), "attempts"))
			h.runner.Host.Config.Provider = "codex"
			h.runner.Host.Options.SemanticInactivityTimeout = 100 * time.Millisecond
			h.runner.Host.Options.HandshakeTimeout = time.Second
			caps, err := h.runner.Host.Config.Capabilities()
			if err != nil {
				t.Fatal(err)
			}
			digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
			if err != nil {
				t.Fatal(err)
			}
			h.request.CapabilityDigest = digest
			h.claim.Preparation.Host, h.claim.Preparation.Provider, h.claim.Preparation.CapabilityDigest = caps.Host, caps.Provider, digest
			if err := protocol.SealCollaborationPreparation(h.claim.Preparation); err != nil {
				t.Fatal(err)
			}
			seedHostResume(t, h.hostRunFixture)
			key, err := hostSessionKey(h.claim)
			if err != nil {
				t.Fatal(err)
			}
			sessions := filepath.Join(h.runner.Host.Directory, key, "codex-home", "sessions")
			if err := os.MkdirAll(sessions, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sessions, "rollout-2026-10-31T00-00-00-"+hostFixtureSession+".jsonl"), []byte("{}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
				t.Fatal(err)
			}
			turns := h.starts(t)
			want := 1
			if mode == "catalog_retry" {
				want = 2
			}
			if len(turns) != want || len(h.bindings) != want || len(h.snapshots) != 1 {
				t.Fatalf("turns=%d bindings=%d snapshots=%d", len(turns), len(h.bindings), len(h.snapshots))
			}
			for i, turn := range turns {
				prompt, ok := turn["prompt"].(string)
				if !ok || h.bindings[i].PromptSHA256 != protocol.CollaborationHandoffHash(prompt) {
					t.Fatalf("launch %d bound hash differs from actual Codex turn/start input", i+1)
				}
				if !strings.Contains(prompt, hostHandoffContinuityGuidance) || strings.HasPrefix(prompt, hostContinuityNotice) {
					t.Fatal("continuity was appended after binding or falsely declared before live resume failure")
				}
				if !strings.Contains(prompt, h.envelope.Sources[1].Content) {
					t.Fatal("Codex did not receive its assignment")
				}
			}
			if want == 2 && (h.bindings[0].AttemptNonce == h.bindings[1].AttemptNonce || h.bindings[0].SnapshotID != h.bindings[1].SnapshotID) {
				t.Fatal("internal retry lost separate launch identity or reused a new snapshot")
			}
			if len(h.proofs) != 1 || len(h.proofs[0].Launches) != want {
				t.Fatal("internal retries lost original termination proof")
			}
		})
	}
}
