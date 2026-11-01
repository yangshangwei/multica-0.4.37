//go:build agentintegration && !windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Fake behaviors stand in for the model with the same tools a real run has:
// the run's own `multica` binary on PATH and the exact stdin prompt. Recall and
// relay behaviors derive answers from those reads; smoke/report use explicit
// fixture answers. Faults model the stop-rule cases, not model understanding.
const rk1BlockHold = 2 * time.Minute

func rk1Fake(plan rk1Plan, frame []byte, stdout io.Writer) int {
	if plan.Fault == "crash" {
		return 3
	}
	session := uuid.NewString()
	emit := func(v any) { raw, _ := json.Marshal(v); _, _ = stdout.Write(append(raw, '\n')) }
	text := func(s string) {
		emit(map[string]any{"type": "assistant", "message": map[string]any{"model": "rk1-fake", "content": []map[string]string{{"type": "text", "text": s}}}})
	}
	emit(map[string]any{"type": "system", "subtype": "init", "session_id": session, "model": "rk1-fake"})
	calls := 0
	rk1ToolFrames = func(args []string, shown string) {
		calls++
		id := fmt.Sprintf("toolu_rk1_%d", calls)
		emit(map[string]any{"type": "assistant", "message": map[string]any{"model": "rk1-fake", "content": []map[string]any{{"type": "tool_use", "id": id, "name": "Bash",
			"input": map[string]string{"command": "multica " + strings.Join(args, " ")}}}}})
		emit(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": id, "content": shown}}}})
	}
	if plan.Fault == "preview" {
		rk1PreviewChars = rk1ClaudeInlineChars
	}
	issue := os.Getenv("MULTICA_COLLABORATION_ISSUE_ID")
	read := plan.Fault != "noread"
	answer, err := rk1FakeAnswer(plan, frame, issue, read, text)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "rk1 fake:", err)
		return 4
	}
	if plan.Fault == "wrong" {
		answer = plan.Params["wrong_answer"]
	}
	text(answer)
	emit(map[string]any{"type": "result", "is_error": plan.Fault == "provider_error", "session_id": session, "result": answer})
	return 0
}

func rk1FakeAnswer(plan rk1Plan, frame []byte, issue string, read bool, text func(string)) (string, error) {
	p := plan.Params
	switch plan.Behavior {
	case "smoke", "report":
		if read {
			if _, err := rk1CLI("issue", "get", issue, "--output", "json"); err != nil {
				return "", err
			}
		}
		return p["answer"], nil
	case "delegate":
		if _, err := rk1CLI("issue", "get", issue, "--output", "json"); err != nil {
			return "", err
		}
		if _, err := rk1CLI("issue", "comment", "add", issue, "--content", p["comment"], "--request-id", uuid.NewString(), "--output", "json"); err != nil {
			return "", err
		}
		return "Delegated", nil
	case "block":
		for i := range 20 {
			text(fmt.Sprintf("Interim analysis step %d: %s", i+1, p["decoy"]))
		}
		// Hold the launch like a Provider mid-turn: stdin EOF does not end it,
		// only Stop's process-group SIGKILL does. Bounded, so a missed kill
		// shows as a surviving process instead of a hang.
		time.Sleep(rk1BlockHold)
		return "", errors.New("not stopped within the hold")
	case "prior":
		return rk1FakePrior(frame)
	case "squad":
		return rk1FakeSquad(frame, nil)
	case "squad_tail":
		return rk1FakeSquad(frame, func(string) (string, error) { return rk1FakeSquadTail(frame, issue, p["pattern"]) })
	case "newest", "canary":
		if !read {
			return p["answer"], nil
		}
		raw, err := rk1CLI("issue", "get", issue, "--output", "json")
		if err != nil {
			return "", err
		}
		if plan.Behavior == "canary" {
			answer, err := rk1LastMatch(string(raw), `(The blocker is [^"]+?\.)`)
			if err != nil {
				return "", err
			}
			if plan.Fault == "obey" {
				_, _ = rk1CLI("issue", "comment", "add", issue, "--content", p["canary"], "--request-id", uuid.NewString(), "--output", "json")
				return answer + " " + p["canary"], nil
			}
			return answer, nil
		}
		code, err := rk1LastMatch(string(raw), p["pattern"])
		if err != nil && plan.Fault == "preview" {
			// What a model reports when the decisive comment is past the preview.
			return rk1LastMatch(string(raw), `(REL-[A-Z0-9]{6})`)
		}
		return code, err
	case "oldest":
		if !read {
			return p["answer"], nil
		}
		return rk1FakeOldest(issue, p["pattern"])
	case "relay_write":
		if _, err := rk1CLI("issue", "get", issue, "--output", "json"); err != nil {
			return "", err
		}
		_, prompt, _ := rk1PromptEnvelope(frame)
		facts, err := rk1LastMatch(prompt, `(Decision history for the job lock:(?:\\n- [^\\]+)+)`)
		if err != nil {
			return "", err
		}
		memory := "## Handoff\n" + strings.ReplaceAll(facts, `\n`, "\n") + "\n"
		if _, err := rk1CLIStdin(memory, "collaboration", "memory", "update", issue, "--base-version", "0", "--request-id", uuid.NewString(), "--content-stdin", "--output", "json"); err != nil {
			return "", err
		}
		return "DONE", nil
	case "relay_read":
		raw, err := rk1CLI("collaboration", "memory", "show", issue, "--output", "json")
		if err != nil {
			return "", err
		}
		var memory struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(raw, &memory) != nil || memory.Content == "" {
			return "", errors.New("memory unavailable")
		}
		return memory.Content, nil
	}
	return "", fmt.Errorf("unknown behavior %q", plan.Behavior)
}

func rk1PromptEnvelope(frame []byte) (protocol.CollaborationHandoffEnvelope, string, error) {
	var input struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	var out struct {
		Handoff *protocol.CollaborationHandoffEnvelope `json:"handoff"`
	}
	if json.Unmarshal(bytes.TrimSpace(frame), &input) != nil || len(input.Message.Content) != 1 {
		return protocol.CollaborationHandoffEnvelope{}, "", errors.New("stdin is not one stream-json user frame")
	}
	prompt := input.Message.Content[0].Text
	start := strings.Index(prompt, "\n{")
	if start < 0 || json.Unmarshal([]byte(prompt[start+1:]), &out) != nil || out.Handoff == nil {
		return protocol.CollaborationHandoffEnvelope{}, prompt, errors.New("prompt carries no handoff envelope")
	}
	return *out.Handoff, prompt, nil
}

func rk1FakePrior(frame []byte) (string, error) {
	envelope, _, err := rk1PromptEnvelope(frame)
	if err != nil {
		return "", err
	}
	for _, item := range envelope.Results.Items {
		if item.RunID == envelope.RunID {
			continue
		}
		record, result := "COMPLETE", "PRESENT"
		if item.RecordingIncomplete {
			record = "INCOMPLETE"
		}
		if item.ResultState != "present" {
			result = "ABSENT"
		}
		text := "NONE RECORDED"
		for _, source := range envelope.Sources {
			if source.RunID == item.RunID && source.ID == item.SourceID {
				text = source.Content
			}
		}
		return fmt.Sprintf("PRIOR_RUN %s RECORD %s RESULT %s\nRESULT_TEXT %s", item.RunID, record, result, text), nil
	}
	return "", errors.New("no earlier run in the handoff")
}
