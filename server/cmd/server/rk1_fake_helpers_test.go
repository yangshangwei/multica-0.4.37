//go:build agentintegration && !windows

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var rk1MemberCode = regexp.MustCompile(`MEMBER-[A-Z0-9]{6}`)

// rk1FakeSquad answers from the startup handoff; full, when set, replaces the
// visible prefix with the source's full text read through the CLI.
func rk1FakeSquad(frame []byte, full func(string) (string, error)) (string, error) {
	envelope, _, err := rk1PromptEnvelope(frame)
	if err != nil {
		return "", err
	}
	if envelope.Assignment.Role != "continuation" {
		return "", fmt.Errorf("squad answer requested in a %q turn", envelope.Assignment.Role)
	}
	var member, code string
	for _, item := range envelope.Results.Items {
		if item.Role != "member" {
			continue
		}
		for _, source := range envelope.Sources {
			if source.RunID == item.RunID && source.Kind == "provider_result" {
				member, code = item.RunID, rk1MemberCode.FindString(source.Content)
			}
		}
	}
	if full != nil && member != "" {
		if code, err = full(member); err != nil {
			return "", err
		}
	}
	if member == "" || code == "" || len(envelope.WakeupCauses.Items) == 0 {
		return "", errors.New("member result or wake cause missing from the handoff")
	}
	cause := envelope.WakeupCauses.Items[0]
	return fmt.Sprintf("MEMBER_RUN %s\nMEMBER_CODE %s\nWOKEN_BY %s %s", member, code, cause.SourceKind, cause.SourceRunID), nil
}

// rk1FakeOldest follows the documented runtime pages until next_cursor is null.
func rk1FakeOldest(issue, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	args := []string{"issue", "comment", "list", issue, "--limit", "100", "--max-bytes", "65536"}
	found := ""
	for range 20 {
		raw, err := rk1CLI(append(args, "--output", "json")...)
		if err != nil {
			return "", err
		}
		var page struct {
			Items []struct {
				Content string `json:"content"`
			} `json:"items"`
			NextCursor *string `json:"next_cursor"`
		}
		if err = json.Unmarshal(raw, &page); err != nil {
			return "", err
		}
		for _, item := range page.Items {
			if m := re.FindStringSubmatch(item.Content); len(m) == 2 {
				found = m[1]
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			if found == "" {
				return "", errors.New("binding decision not found after the last page")
			}
			return found, nil
		}
		args = []string{"issue", "comment", "list", issue, "--limit", "100", "--max-bytes", "65536", "--cursor", *page.NextCursor}
	}
	return "", errors.New("comment pages did not end")
}

func rk1LastMatch(raw, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	matches := re.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 || len(matches[len(matches)-1]) != 2 {
		return "", errors.New("pattern not found in the read")
	}
	return matches[len(matches)-1][1], nil
}

// rk1LaunchDir is this launch's private capture directory (set by the wrapper).
var rk1LaunchDir string

// rk1ToolFrames, when set, emits each CLI call as a Claude tool_use/tool_result
// pair, so the recorded stream shows what the model was shown. rk1PreviewChars
// > 0 models Claude's inline cap: the fake sees and reports only that prefix.
var (
	rk1ToolFrames   func(args []string, shown string)
	rk1PreviewChars int
)

const rk1ClaudeInlineChars = 30000

func rk1CLI(args ...string) ([]byte, error) { return rk1CLIStdin("", args...) }

// rk1RecordCLI keeps the size of everything a CLI call printed. The real
// Provider shows a model only a preview of long tool output, so the scorer
// must know whether a cursor could have been inline at all.
func rk1RecordCLI(args []string, out []byte) {
	if rk1LaunchDir == "" {
		return
	}
	name := args
	if len(name) > 3 {
		name = name[:3]
	}
	raw, _ := json.Marshal(map[string]any{"command": strings.Join(name, " "), "stdout_bytes": len(out), "stdout_chars": utf8.RuneCount(out),
		"next_cursor_char_offset": bytes.Index(out, []byte(`"next_cursor"`)), "next_offset_char_offset": bytes.Index(out, []byte(`"next_offset"`))})
	f, err := os.OpenFile(filepath.Join(rk1LaunchDir, "cli-calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(raw, '\n'))
	_ = f.Close()
}

// rk1FakeSquadTail reads a truncated member result through the documented
// source command, chunk by chunk, and answers from the full text.
func rk1FakeSquadTail(frame []byte, issue, pattern string) (string, error) {
	envelope, _, err := rk1PromptEnvelope(frame)
	if err != nil {
		return "", err
	}
	for _, source := range envelope.Sources {
		if source.Kind != "provider_result" || !source.Truncated {
			continue
		}
		var full strings.Builder
		offset := 0
		for range 64 {
			raw, err := rk1CLI("collaboration", "handoff", "source", issue, source.ID, "--kind", source.Kind, "--source-run-id", source.RunID,
				"--version", source.Version, "--sha256", source.SHA256, "--offset", strconv.Itoa(offset))
			if err != nil {
				return "", err
			}
			var page struct {
				Source struct {
					Content string `json:"content"`
				} `json:"source"`
				NextOffset *int `json:"next_offset"`
			}
			if err = json.Unmarshal(raw, &page); err != nil {
				return "", err
			}
			full.WriteString(page.Source.Content)
			if page.NextOffset == nil {
				return rk1LastMatch(full.String(), pattern)
			}
			offset = *page.NextOffset
		}
		return "", errors.New("source chunks did not end")
	}
	return "", errors.New("no truncated member result in the handoff")
}

func rk1CLIStdin(stdin string, args ...string) ([]byte, error) {
	path, err := exec.LookPath("multica")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Stdin = rk1ChildEnv(), strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	rk1RecordCLI(args, out)
	if rk1PreviewChars > 0 && utf8.RuneCount(out) > rk1PreviewChars {
		out = []byte(string([]rune(string(out))[:rk1PreviewChars]))
	}
	if rk1ToolFrames != nil {
		rk1ToolFrames(args, string(out))
	}
	if err != nil {
		return nil, fmt.Errorf("multica %s: %w: %s", strings.Join(args[:2], " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func rk1ReadFrame(r *bufio.Reader) ([]byte, error) {
	var frame []byte
	for {
		chunk, err := r.ReadSlice('\n')
		frame = append(frame, chunk...)
		if len(frame) > rk1MaxPromptFrame {
			return nil, errors.New("prompt frame over limit")
		}
		if err == nil {
			return frame, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func rk1WriteFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return errors.Join(err, f.Sync(), f.Close())
}

func rk1WriteJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return rk1WriteFile(path, append(raw, '\n'))
}

func rk1ReadPlan(path string) (rk1Plan, error) {
	var plan rk1Plan
	raw, err := os.ReadFile(path)
	if err != nil {
		return plan, err
	}
	if err = json.Unmarshal(raw, &plan); err != nil || plan.Mode != "fake" && plan.Mode != "real" {
		return plan, errors.Join(err, errors.New("invalid plan"))
	}
	return plan, nil
}

// Each physical launch of a run gets its own numbered directory, so a fallback
// launch can never overwrite the first launch's stdin capture.
func rk1NextLaunchDir(root string) (string, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	for i := 1; i <= 8; i++ {
		dir := filepath.Join(root, "launch-"+strconv.Itoa(i))
		if err := os.Mkdir(dir, 0700); err == nil {
			return dir, nil
		} else if !os.IsExist(err) {
			return "", err
		}
	}
	return "", errors.New("launch limit")
}
