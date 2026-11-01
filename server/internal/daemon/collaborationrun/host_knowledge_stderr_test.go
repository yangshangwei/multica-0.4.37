package collaborationrun

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The compiled test binary is the fixture executable. This branch runs before
// TestMain, only in its child process; it never resolves an installed CLI.
func init() {
	mode := os.Getenv("COLLAB_KNOWLEDGE_STDERR_FIXTURE")
	if mode == "" {
		return
	}
	input, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		os.Exit(41)
	}
	if err := os.WriteFile(os.Getenv("COLLAB_KNOWLEDGE_STDERR_INPUT"), []byte(input), 0o600); err != nil {
		os.Exit(42)
	}
	_, _ = fmt.Fprintln(os.Stderr, "stderr-head-not-retained", strings.Repeat("x", 4096), input, "stderr-tail-retained")
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"system","session_id":"`+hostFixtureSession+`"}`)
	if mode == "provider_failure" {
		_, _ = fmt.Fprintln(os.Stdout, `{"type":"result","is_error":true,"result":"provider rejected the turn"}`)
		os.Exit(1)
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"type":"result","is_error":false,"session_id":"`+hostFixtureSession+`","result":"finished collaboration turn"}`)
	os.Exit(0)
}

func TestHostKnowledgeStderrRemainsPrivateAndRetainsFailureEvidence(t *testing.T) {
	for _, outcome := range []string{"success", "provider_failure"} {
		t.Run(outcome, func(t *testing.T) {
			f := newHostKnowledgeFixture(t)
			var logs bytes.Buffer
			f.runner.Host.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			inputPath := f.capture + ".stderr-input"
			t.Setenv("COLLAB_KNOWLEDGE_STDERR_FIXTURE", outcome)
			t.Setenv("COLLAB_KNOWLEDGE_STDERR_INPUT", inputPath)
			if err := f.runner.Run(t.Context(), f.claim.Identity.RunID, f.request); err != nil {
				t.Fatal(err)
			}
			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			var frame struct {
				Message struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			}
			if err := json.Unmarshal(input, &frame); err != nil || len(frame.Message.Content) != 1 || !strings.Contains(frame.Message.Content[0].Text, f.contribution.Content) {
				t.Fatal("fake executable did not receive approved private knowledge")
			}
			if strings.Contains(logs.String(), f.contribution.Content) {
				t.Logf("captured generic provider log:\n%s", logs.String())
				t.Error("private stderr reached the generic host logger")
			}
			if !strings.Contains(logs.String(), "claude started") || !strings.Contains(logs.String(), "claude finished") {
				t.Error("provider lifecycle metadata logs were discarded")
			}
			if len(f.outcomes) != 1 {
				t.Fatalf("expected one outcome, got %d", len(f.outcomes))
			}
			result := f.outcomes[0]
			if outcome == "success" {
				if result.Outcome != protocol.CollaborationBusinessOutcomeSuccess || result.Body != "finished collaboration turn" {
					t.Error("stderr changed successful final output")
				}
			} else {
				if result.Outcome != protocol.CollaborationBusinessOutcomeFailure {
					t.Error("provider failure became success")
				}
				if !strings.Contains(result.Body, f.contribution.Content) || !strings.Contains(result.Body, "stderr-tail-retained") {
					t.Error("bounded stderr was lost from authorized failure outcome")
				}
				if strings.Contains(result.Body, "stderr-head-not-retained") || len(result.Body) > 4096 {
					t.Error("failure diagnostic evidence is not bounded")
				}
			}
		})
	}
}
