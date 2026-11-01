package daemon

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestKnowledgeStderrRemainsPrivateAndRetainsFailureEvidence(t *testing.T) {
	for _, outcome := range []string{"success", "provider_failure", "knowledge_refusal"} {
		t.Run(outcome, func(t *testing.T) {
			f := newKnowledgeDaemonFixture(t)
			// Echo the actual application input, including its approved contribution,
			// to stderr. The old leading diagnostic must fall outside the tail budget.
			script := "#!/bin/sh\nIFS= read -r payload\nprintf '%s' \"$payload\" > \"$CAPTURE_INPUT\"\n" +
				"printf '%s\\n' 'stderr-head-not-retained' '" + strings.Repeat("x", 4096) + "' \"$payload\" 'stderr-tail-retained' >&2\n" +
				"printf '{\"type\":\"system\",\"session_id\":\"%s\"}\\n' \"$FAKE_NATIVE\"\n"
			if outcome == "provider_failure" {
				script += "printf '%s\\n' '{\"type\":\"result\",\"is_error\":true,\"result\":\"provider rejected the turn\"}'\nexit 1\n"
			} else {
				script += "printf '{\"type\":\"result\",\"is_error\":false,\"session_id\":\"%s\",\"result\":\"finished ordinary turn\"}\\n' \"$FAKE_NATIVE\"\n"
			}
			if err := os.WriteFile(f.bin, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			if outcome == "knowledge_refusal" {
				f.observationRefusal = "terminated"
			}
			result := f.run(t)
			input, err := os.ReadFile(f.capture)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(input, []byte(privateKnowledgeTestBody)) {
				t.Fatal("fake executable did not receive the approved private input")
			}
			if strings.Contains(f.logs.String(), privateKnowledgeTestBody) {
				t.Logf("captured generic provider log:\n%s", f.logs.String())
				t.Error("private stderr reached the generic daemon logger")
			}
			if !strings.Contains(f.logs.String(), "claude started") || !strings.Contains(f.logs.String(), "claude finished") {
				t.Error("provider lifecycle metadata logs were discarded")
			}
			if outcome == "success" {
				if result.Status != "completed" || result.Comment != "finished ordinary turn" {
					t.Errorf("unexpected success outcome: %s", result.Status)
				}
				return
			}
			if result.Status != "blocked" {
				t.Errorf("failure was reported as %s", result.Status)
			}
			evidence := result.Comment
			if outcome == "knowledge_refusal" {
				if result.FailureReason != agent.KnowledgeContextChanged || result.Comment != agent.KnowledgeContextChanged {
					t.Error("knowledge refusal lost its content-free terminal failure")
				}
				var recorded strings.Builder
				for _, call := range f.callsFor("messages") {
					var body struct {
						Messages []TaskMessageData `json:"messages"`
					}
					if err := json.Unmarshal(call.body, &body); err != nil {
						t.Fatal(err)
					}
					for _, message := range body.Messages {
						if message.Type == "text" {
							recorded.WriteString(message.Content)
						}
					}
				}
				evidence = recorded.String()
			}
			if !strings.Contains(evidence, privateKnowledgeTestBody) || !strings.Contains(evidence, "stderr-tail-retained") {
				t.Error("bounded stderr was lost from authorized failed evidence")
			}
			if strings.Contains(evidence, "stderr-head-not-retained") || len(evidence) > 4096 {
				t.Error("failed stderr evidence is not bounded")
			}
		})
	}
}
