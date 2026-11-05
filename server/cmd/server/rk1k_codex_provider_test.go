//go:build agentintegration && !windows

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const rk1kCodexProviderName = "rk1k-codex-provider"
const rk1kCodexFakeVersion = "0.165.0"

type rk1kCodexWire struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type rk1kCodexObservation struct {
	Prompt, Answer, Native, Turn, Model string
	Resumed                             bool
	Ephemeral                           bool
	Complete                            bool
	Error                               bool
}

// Observe the native RPC exchange; a terminal answer alone is not a turn proof.
func rk1kObserveCodex(input, output []byte) (rk1kCodexObservation, error) {
	var o rk1kCodexObservation
	fail := func() (rk1kCodexObservation, error) {
		return o, errors.New("incomplete or conflicting Codex RPC identity")
	}
	requests := map[string]rk1kCodexWire{}
	var threadReq, turnReq rk1kCodexWire
	for _, line := range bytes.Split(bytes.TrimSpace(input), []byte{'\n'}) {
		var m rk1kCodexWire
		if json.Unmarshal(line, &m) != nil {
			return fail()
		}
		if len(m.ID) > 0 {
			key := string(m.ID)
			if _, exists := requests[key]; exists {
				return fail()
			}
			requests[key] = m
		}
		if m.Method == "thread/start" || m.Method == "thread/resume" {
			if threadReq.Method != "" {
				return fail()
			}
			threadReq = m
			o.Resumed = m.Method == "thread/resume"
		}
		if m.Method == "turn/start" {
			if turnReq.Method != "" {
				return fail()
			}
			turnReq = m
		}
	}
	if len(threadReq.ID) == 0 || len(turnReq.ID) == 0 {
		return fail()
	}
	var params struct {
		ThreadID string                        `json:"threadId"`
		Input    []struct{ Type, Text string } `json:"input"`
	}
	if json.Unmarshal(turnReq.Params, &params) != nil || len(params.Input) != 1 || params.Input[0].Type != "text" || !rk1kUUID(params.ThreadID) {
		return fail()
	}
	o.Native, o.Prompt = params.ThreadID, params.Input[0].Text
	if o.Resumed {
		var resume struct {
			ThreadID string `json:"threadId"`
		}
		if json.Unmarshal(threadReq.Params, &resume) != nil || resume.ThreadID != o.Native {
			return fail()
		}
	}
	responses := map[string]rk1kCodexWire{}
	events := []rk1kCodexWire{}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		var m rk1kCodexWire
		if json.Unmarshal(line, &m) != nil {
			return fail()
		}
		if len(m.ID) > 0 && m.Method == "" {
			key := string(m.ID)
			if _, exists := responses[key]; exists {
				return fail()
			}
			if _, exists := requests[key]; !exists {
				return fail()
			}
			responses[key] = m
		}
		if m.Method != "" {
			events = append(events, m)
		}
	}
	threadResponse, ok := responses[string(threadReq.ID)]
	if !ok || len(threadResponse.Error) > 0 && string(threadResponse.Error) != "null" {
		return fail()
	}
	var thread struct {
		Model  string `json:"model"`
		Thread struct {
			ID        string
			Ephemeral *bool
		} `json:"thread"`
	}
	if json.Unmarshal(threadResponse.Result, &thread) != nil || thread.Thread.ID != o.Native || thread.Thread.Ephemeral == nil {
		return fail()
	}
	o.Ephemeral, o.Model = *thread.Thread.Ephemeral, thread.Model
	turnResponse, ok := responses[string(turnReq.ID)]
	if !ok || len(turnResponse.Error) > 0 && string(turnResponse.Error) != "null" {
		return fail()
	}
	var turn struct {
		Turn struct{ ID string } `json:"turn"`
	}
	if json.Unmarshal(turnResponse.Result, &turn) != nil || turn.Turn.ID == "" {
		return fail()
	}
	o.Turn = turn.Turn.ID
	terminal := false
	for _, m := range events {
		if m.Method == "error" {
			o.Error = true
		}
		if m.Method == "item/completed" {
			if terminal {
				return fail()
			}
			var p struct {
				ThreadID string                      `json:"threadId"`
				TurnID   string                      `json:"turnId"`
				Item     struct{ Type, Text string } `json:"item"`
			}
			if json.Unmarshal(m.Params, &p) != nil || p.ThreadID != o.Native || p.TurnID != o.Turn {
				return fail()
			}
			if p.Item.Type == "agentMessage" {
				o.Answer = p.Item.Text
			}
		}
		if m.Method == "turn/started" || m.Method == "turn/completed" {
			if terminal {
				return fail()
			}
			var p struct {
				ThreadID string `json:"threadId"`
				Turn     struct {
					ID, Status string
					Error      json.RawMessage
				} `json:"turn"`
			}
			if json.Unmarshal(m.Params, &p) != nil || p.ThreadID != o.Native || p.Turn.ID != o.Turn {
				return fail()
			}
			if m.Method == "turn/completed" {
				if terminal {
					return fail()
				}
				terminal = true
				o.Complete = true
				o.Error = o.Error || p.Turn.Status != "completed" || len(p.Turn.Error) > 0 && string(p.Turn.Error) != "null"
			}
		}
	}
	return o, nil
}

func init() {
	if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == rk1kCodexProviderName {
		dir := os.Getenv(rk1kPlanDirEnv)
		if !filepath.IsAbs(dir) {
			os.Exit(26)
		}
		os.Exit(rk1kCodexProviderMain(dir))
	}
}

func rk1kCodexProviderMain(dir string) int {
	if slices.Contains(os.Args[1:], "--version") {
		version, err := rk1kCodexReportedVersion(dir)
		if err != nil {
			return 41
		}
		fmt.Println("codex-cli " + version)
		return 0
	}
	if len(os.Args) < 2 || os.Args[1] != "app-server" {
		return 26
	}
	id, err := rk1kProviderIdentity()
	if err != nil {
		return 27
	}
	plan, err := rk1ReadPlan(filepath.Join(dir, id.key()+".json"))
	if err != nil {
		return 28
	}
	launch, err := rk1NextLaunchDir(filepath.Join(dir, id.key()))
	if err != nil {
		return 29
	}
	rk1LaunchDir = launch
	cwd, err := os.Getwd()
	if err != nil {
		return 30
	}
	purpose := "mcb"
	if id.Kind == "task" {
		purpose = "mat"
	}
	meta := rk1kLaunchMeta{rk1LaunchMeta: rk1LaunchMeta{Args: os.Args[1:], Cwd: cwd, Mode: plan.Mode, Behavior: plan.Behavior, Fault: plan.Fault, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), EvidenceState: "started"}, Identity: id, CredentialPurpose: purpose, Seed: plan.Params["seed"] == "true"}
	if rk1WriteJSON(filepath.Join(launch, "launch.json"), meta) != nil {
		return 30
	}
	in, err := os.OpenFile(filepath.Join(launch, "stdin.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 30
	}
	out, err := os.OpenFile(filepath.Join(launch, "stdout.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		in.Close()
		return 30
	}
	var input io.Reader = io.TeeReader(os.Stdin, in)
	output := io.MultiWriter(out, os.Stdout)
	code := 0
	var reservation *rk1kCodexReservation
	if plan.Mode == "real" {
		reservation, err = rk1kReserveCodex(plan, id, launch)
		if err != nil {
			in.Close()
			out.Close()
			return 41
		}
		if rk1WriteJSON(filepath.Join(launch, "launch.json"), meta) != nil {
			in.Close()
			out.Close()
			return 42
		}
		var started bool
		code, started = rk1kExecObserved(plan.Real, os.Args[1:], rk1kChildEnv(), input, output, reservation.started)
		if !started {
			code = 43
		} else {
			meta.PaidStarts = 1
		}
	} else if plan.Mode == "fake" {
		meta.FakeStarts = 1
		code = rk1kCodexFake(plan, id, input, output)
	} else {
		code = 44
	}
	finishErr := errors.Join(in.Sync(), in.Close(), out.Sync(), out.Close())
	rawIn, inErr := os.ReadFile(filepath.Join(launch, "stdin.jsonl"))
	rawOut, outErr := os.ReadFile(filepath.Join(launch, "stdout.jsonl"))
	observation, observeErr := rk1kObserveCodex(rawIn, rawOut)
	meta.NativeID, meta.ResultSeen = observation.Native, observation.Complete && !observation.Error
	if reservation != nil && observation.Model != reservation.model {
		observeErr = errors.Join(observeErr, errors.New("observed Codex model differs from registration"))
	}
	meta.PromptBytes = len(observation.Prompt)
	meta.ExitCode = &code
	meta.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	meta.EvidenceState = "complete"
	if errors.Join(finishErr, inErr, outErr, observeErr) != nil {
		meta.EvidenceState = "incomplete"
		code = 45
	}
	if rk1WriteJSON(filepath.Join(launch, "launch.json"), meta) != nil {
		code = 46
	}
	if reservation != nil && reservation.finish(code == 0 && meta.ResultSeen && meta.EvidenceState == "complete") != nil {
		code = 47
	}
	return code
}

// The fake endpoint speaks the production Codex app-server protocol. Business
// effects use the same scoped Multica CLI as the real agent will use.
func rk1kCodexFake(plan rk1Plan, id rk1kIdentity, input io.Reader, output io.Writer) int {
	var mu sync.Mutex
	emit := func(v any) error { mu.Lock(); defer mu.Unlock(); return json.NewEncoder(output).Encode(v) }
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1<<20), 4<<20)
	native, ephemeral := "", false
	for scanner.Scan() {
		var m rk1kCodexWire
		if json.Unmarshal(scanner.Bytes(), &m) != nil {
			return 50
		}
		respond := func(v any) error { return emit(map[string]any{"id": m.ID, "result": v}) }
		switch m.Method {
		case "initialize":
			if respond(map[string]any{"userAgent": "rk1k-codex-fake"}) != nil {
				return 51
			}
		case "initialized":
		case "thread/start", "thread/resume":
			var p struct {
				ThreadID  string `json:"threadId"`
				Ephemeral bool   `json:"ephemeral"`
			}
			if json.Unmarshal(m.Params, &p) != nil {
				return 52
			}
			ephemeral = p.Ephemeral
			native = p.ThreadID
			if m.Method == "thread/start" {
				native = plan.Params["native_id"]
				if native == "" {
					native = uuid.NewString()
				}
			}
			if !rk1kUUID(native) {
				return 53
			}
			store := filepath.Join(os.Getenv("CODEX_HOME"), "sessions")
			path := filepath.Join(store, "rollout-rk1k-"+native+".jsonl")
			if m.Method == "thread/resume" {
				if _, err := os.Stat(path); err != nil {
					return 54
				}
			}
			if !ephemeral {
				if os.MkdirAll(store, 0700) != nil || rk1WriteFile(path, []byte("{\"type\":\"session_meta\"}\n")) != nil {
					return 55
				}
			}
			responseNative := native
			if plan.Fault == "missing_native" {
				responseNative = ""
			}
			if plan.Fault == "changed_native" {
				responseNative = uuid.NewString()
			}
			if respond(map[string]any{"thread": map[string]any{"id": responseNative, "ephemeral": ephemeral, "path": nil}, "model": "rk1k-requested-model"}) != nil {
				return 56
			}
		case "turn/start":
			var p struct {
				ThreadID string                        `json:"threadId"`
				Input    []struct{ Type, Text string } `json:"input"`
			}
			if json.Unmarshal(m.Params, &p) != nil || p.ThreadID != native || len(p.Input) != 1 || p.Input[0].Type != "text" {
				return 57
			}
			turn := uuid.NewString()
			if respond(map[string]any{"turn": map[string]any{"id": turn, "status": "inProgress"}}) != nil {
				return 58
			}
			if emit(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": native, "turn": map[string]any{"id": turn, "status": "inProgress"}}}) != nil {
				return 58
			}
			answer, err := rk1kCodexBusiness(plan, id, p.Input[0].Text, func(args []string, shown string) error {
				item := map[string]any{"id": uuid.NewString(), "type": "commandExecution", "command": "multica " + strings.Join(args, " "), "status": "completed", "exitCode": 0, "aggregatedOutput": shown}
				return emit(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": native, "turnId": turn, "item": item}})
			})
			if err != nil {
				return 59
			}
			if plan.Fault == "wrong" {
				answer = "WELL_FORMED_WRONG"
			}
			if emit(map[string]any{"method": "item/completed", "params": map[string]any{"threadId": native, "turnId": turn, "item": map[string]any{"id": uuid.NewString(), "type": "agentMessage", "text": answer}}}) != nil {
				return 60
			}
			if plan.Fault == "missing_result" {
				return 0
			}
			if emit(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": native, "turn": map[string]any{"id": turn, "status": "completed"}}}) != nil {
				return 60
			}
		default:
			if len(m.ID) > 0 {
				if respond(map[string]any{}) != nil {
					return 61
				}
			}
		}
	}
	if scanner.Err() != nil {
		return 62
	}
	return 0
}

func rk1kCodexBusiness(plan rk1Plan, id rk1kIdentity, prompt string, tool func([]string, string) error) (string, error) {
	previous := rk1kToolFrames
	rk1kToolFrames = tool
	defer func() { rk1kToolFrames = previous }()
	if plan.Behavior != "guard" {
		commands := [][]string{}
		if id.Kind == "task" {
			commands = append(commands, []string{"project", "get", plan.Params["project"], "--output", "json"})
			if plan.Behavior == "quick_create" {
				commands = append(commands, []string{"issue", "create", "--title", "RK1K " + id.TaskID, "--description", "Created by actual quick-create task", "--project", plan.Params["project"], "--status", "backlog", "--output", "json"})
			}
		} else {
			issue := os.Getenv("MULTICA_COLLABORATION_ISSUE_ID")
			commands = append(commands, []string{"collaboration", "memory", "show", issue, "--output", "json"}, []string{"collaboration", "handoff", "show", issue, "--output", "json"})
			if plan.Behavior == "delegate" {
				commands = append(commands, []string{"issue", "comment", "add", issue, "--content", plan.Params["comment"], "--request-id", uuid.NewString(), "--output", "json"})
			}
		}
		for _, args := range commands {
			if _, err := rk1kCLI(plan.Params["cli"], args...); err != nil {
				return "", err
			}
		}
	}
	items, err := rk1kContributions(prompt)
	if err != nil {
		return "", err
	}
	bodies := []string{}
	for _, item := range items {
		bodies = append(bodies, item.Content)
	}
	if len(bodies) == 0 {
		return "NONE", nil
	}
	return strings.Join(bodies, "\n"), nil
}

func rk1kReadCodexLaunch(dir string) rk1Launch {
	var l rk1Launch
	raw, err := os.ReadFile(filepath.Join(dir, "launch.json"))
	if err != nil || json.Unmarshal(raw, &l.Meta) != nil {
		l.EvidenceError = "codex_launch_record_missing"
	}
	in, inErr := os.ReadFile(filepath.Join(dir, "stdin.jsonl"))
	out, outErr := os.ReadFile(filepath.Join(dir, "stdout.jsonl"))
	o, err := rk1kObserveCodex(in, out)
	if errors.Join(inErr, outErr, err) != nil {
		l.EvidenceError = "codex_rpc_incomplete"
	}
	l.Prompt, l.Frame, l.ResultSeen, l.ResultError = o.Prompt, in, o.Complete, o.Error
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		var m rk1kCodexWire
		if json.Unmarshal(line, &m) != nil || m.Method != "item/completed" {
			continue
		}
		var p struct {
			Item struct{ Type, Text, AggregatedOutput string } `json:"item"`
		}
		if json.Unmarshal(m.Params, &p) != nil {
			continue
		}
		if p.Item.Type == "agentMessage" {
			l.recordable(len(p.Item.Text))
		}
		if p.Item.Type == "commandExecution" {
			l.recordable(len(p.Item.AggregatedOutput))
			l.ToolResults++
		}
	}
	return l
}
