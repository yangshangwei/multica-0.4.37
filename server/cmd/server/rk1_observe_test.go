//go:build agentintegration && !windows

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// rk1Env is one series' Provider setup. sim replaces the real CLI with the
// test-owned rk1-sim-claude, so mode "real" runs the wrapper's pass-through
// path without a paid launch.
type rk1Env struct {
	real, version, model, wrapper, cli string
	sim                                bool
}

type rk1Launch struct {
	Meta          rk1LaunchMeta
	Frame         []byte
	Prompt        string
	Models        []string // assistant message.model: what the Provider served
	InitModels    []string // system/init model: the CLI's resolved request
	ToolText      []string // tool_result content the model was shown
	ResultSeen    bool
	ResultError   bool
	ToolResults   int
	ToolMaxChars  int
	CursorInline  int
	CLICalls      []map[string]any
	EnvelopeError string
	EvidenceError string
	// Recordable counts the messages the Claude backend turns into recorded
	// messages (non-empty text, tool_use with id and name, tool_result with
	// tool_use_id); OverLimit counts those whose body exceeds the record text
	// limit, which the recorder truncates and marks partial.
	Recordable, OverLimit int
}

func (l *rk1Launch) recordable(bodyBytes int) {
	l.Recordable++
	if bodyBytes > protocol.CollaborationRecordTextLimit {
		l.OverLimit++
	}
}

// rk1Obs is everything recorded about one run: server trace, physical
// launches, DB state and event payloads. Scorers read only this.
type rk1Obs struct {
	Run, RunErr          string
	HostDiagnostics      string
	HostLogError         string
	Calls                []rk1Call
	Launches             []rk1Launch
	Progress, Proof      string
	Business             string
	RecordingIncomplete  bool
	StepKind             string
	Events               map[string]int
	Outcome              *protocol.CollaborationRunOutcome
	Envelope             *protocol.CollaborationHandoffEnvelope
	Claim                *protocol.CollaborationRunClaimResponse
	SealReason, ClaimQry string
	SealObserved         *int
	SealAcked            int
}

// toolSaw reports whether text reached the model in any tool_result.
func (o *rk1Obs) toolSaw(text string) bool {
	for _, l := range o.Launches {
		for _, shown := range l.ToolText {
			if strings.Contains(shown, text) {
				return true
			}
		}
	}
	return false
}

func (f *rk1Fixture) observe(run string, runErr error) *rk1Obs {
	t := f.t
	t.Helper()
	o := &rk1Obs{Run: run, Events: map[string]int{}}
	if raw, err := os.ReadFile(filepath.Join(f.dir, "host-"+run+".log")); err != nil {
		o.HostLogError = "host_log_unreadable"
	} else {
		o.HostDiagnostics = string(raw)
	}
	if runErr != nil {
		o.RunErr = rk1Secret.ReplaceAllString(runErr.Error(), "[credential]")
	}
	prefix := "/api/collaboration/runtime/runs/" + run + "/"
	for _, call := range f.trace.snapshot() {
		if !strings.HasPrefix(call.Path, prefix) {
			continue
		}
		o.Calls = append(o.Calls, call)
		suffix := strings.TrimPrefix(call.Path, prefix)
		switch {
		case suffix == "claim" && call.Status == 200:
			var claim protocol.CollaborationRunClaimResponse
			if json.Unmarshal(call.raw, &claim) == nil {
				o.Claim, o.ClaimQry = &claim, call.Query
			}
		case suffix == "handoff/snapshots" && call.Status == 200 && o.Envelope == nil:
			var envelope protocol.CollaborationHandoffEnvelope
			if json.Unmarshal(call.raw, &envelope) == nil {
				o.Envelope = &envelope
			}
		case suffix == "recording/seal":
			var seal struct {
				Reason   string `json:"reason"`
				Observed *int   `json:"observed_through"`
				Acked    int    `json:"acknowledged_through"`
			}
			_ = json.Unmarshal(call.Request, &seal)
			o.SealReason, o.SealObserved, o.SealAcked = seal.Reason, seal.Observed, seal.Acked
		}
	}
	state, err := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
	if err != nil {
		t.Fatal(err)
	}
	o.Progress, o.Proof, o.Business, o.RecordingIncomplete, o.StepKind = state.ProgressState, state.ProofState, state.BusinessOutcome.String, state.RecordingIncomplete, state.StepKind.String
	rows, err := f.base.managed.pool.Query(t.Context(), `SELECT kind, payload FROM collaboration_run_event WHERE run_id=$1 ORDER BY seq`, run)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind string
		var payload []byte
		if err = rows.Scan(&kind, &payload); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		o.Events[kind]++
		if kind == "outcome" {
			var outcome protocol.CollaborationRunOutcome
			if json.Unmarshal(payload, &outcome) == nil {
				o.Outcome = &outcome
			}
		}
	}
	rows.Close()
	for i := 1; ; i++ {
		dir := filepath.Join(f.plans, run, "launch-"+strconv.Itoa(i))
		if _, err := os.Stat(dir); err != nil {
			break
		}
		o.Launches = append(o.Launches, rk1ReadLaunch(dir))
	}
	return o
}

func rk1ReadLaunch(dir string) rk1Launch {
	var l rk1Launch
	if raw, err := os.ReadFile(filepath.Join(dir, "launch.json")); err != nil || json.Unmarshal(raw, &l.Meta) != nil {
		l.EvidenceError = "launch_record_unreadable"
	}
	l.Frame, _ = os.ReadFile(filepath.Join(dir, "stdin-first.jsonl"))
	if _, prompt, err := rk1PromptEnvelope(l.Frame); prompt != "" {
		l.Prompt = prompt
		if err != nil {
			l.EnvelopeError = err.Error()
		}
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "cli-calls.jsonl")); err == nil {
		for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
			var call map[string]any
			if json.Unmarshal(line, &call) == nil {
				l.CLICalls = append(l.CLICalls, call)
			}
		}
	}
	file, err := os.Open(filepath.Join(dir, "stdout.jsonl"))
	if err != nil {
		l.EvidenceError = "stdout_capture_unreadable"
		return l
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 1<<20), 64<<20)
	for scan.Scan() {
		var frame struct {
			Type    string `json:"type"`
			Model   string `json:"model"`
			IsError bool   `json:"is_error"`
			Message struct {
				Model   string            `json:"model"`
				Content []json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scan.Bytes(), &frame) != nil {
			continue
		}
		if frame.Type == "system" && frame.Model != "" && !slices.Contains(l.InitModels, frame.Model) {
			l.InitModels = append(l.InitModels, frame.Model)
		}
		if frame.Type == "assistant" && frame.Message.Model != "" && !slices.Contains(l.Models, frame.Message.Model) {
			l.Models = append(l.Models, frame.Message.Model)
		}
		if frame.Type == "assistant" {
			for _, item := range frame.Message.Content {
				var block struct {
					Type, Text, ID, Name string
					Input                json.RawMessage
				}
				if json.Unmarshal(item, &block) != nil {
					continue
				}
				switch {
				case block.Type == "text" && block.Text != "":
					l.recordable(len(block.Text))
				case block.Type == "tool_use" && block.ID != "" && block.Name != "":
					l.recordable(len(block.Input))
				}
			}
		}
		if frame.Type == "result" {
			l.ResultSeen, l.ResultError = true, frame.IsError
		}
		if frame.Type == "user" {
			for _, item := range frame.Message.Content {
				var tool struct {
					Type      string          `json:"type"`
					ToolUseID string          `json:"tool_use_id"`
					Content   json.RawMessage `json:"content"`
				}
				if json.Unmarshal(item, &tool) != nil || tool.Type != "tool_result" {
					continue
				}
				if tool.ToolUseID != "" {
					l.recordable(len(tool.Content))
				}
				l.ToolResults++
				l.ToolText = append(l.ToolText, string(tool.Content))
				l.ToolMaxChars = max(l.ToolMaxChars, len([]rune(string(tool.Content))))
				if bytes.Contains(tool.Content, []byte("next_cursor")) || bytes.Contains(tool.Content, []byte("next_offset")) {
					l.CursorInline++
				}
			}
		}
	}
	if scan.Err() != nil {
		l.EvidenceError = "stdout_capture_unreadable"
	}
	return l
}
