package agent

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestClaudeFreshStreamArgumentsKeepFrozenInvocation(t *testing.T) {
	args, err := ClaudeFreshStreamArguments("fixed system instructions", "fixture-model", "high")
	if err != nil {
		t.Fatal(err)
	}
	for flag, want := range map[string]string{"--model": "fixture-model", "--effort": "high", "--append-system-prompt": "fixed system instructions", "--mcp-config": `{"mcpServers":{}}`} {
		found := false
		for i, arg := range args {
			if arg == flag && i+1 < len(args) && args[i+1] == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing frozen invocation field %s", flag)
		}
	}
	for _, arg := range args {
		if arg == "--resume" || arg == "--continue" {
			t.Fatal("fresh invocation acquired a session pointer")
		}
	}
	if _, err := ClaudeFreshStreamArguments("", "", "invented-effort"); err == nil {
		t.Fatal("unsupported effort silently inherited a default")
	}
	wire, err := ClaudeStreamInput("actual immutable prompt")
	if err != nil || !strings.Contains(string(wire), "actual immutable prompt") || !strings.HasSuffix(string(wire), "\n") {
		t.Fatal("input did not use the canonical stream-json user frame")
	}
}

func TestClaudeFreshStreamArgumentsIsolateRepositoryConfiguration(t *testing.T) {
	args, err := ClaudeFreshStreamArguments("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--setting-sources=", "--disable-slash-commands", "--strict-mcp-config"} {
		if !slices.Contains(args, flag) {
			t.Errorf("fresh collaboration invocation permits implicit repository configuration without %s", flag)
		}
	}
	settings := slices.Index(args, "--settings")
	if settings == -1 || settings+1 >= len(args) || args[settings+1] != `{"disableAllHooks":true}` {
		t.Error("fresh collaboration invocation does not explicitly disable repository hooks")
	}
	for _, flag := range []string{"--resume", "--continue", "--plugin-dir"} {
		if slices.Contains(args, flag) {
			t.Errorf("fresh collaboration invocation acquired %s", flag)
		}
	}
}

func TestClaudeStreamCodecKeepsExactPresenceAndStructuredTerminal(t *testing.T) {
	var codec ClaudeStreamCodec
	if _, _, err := codec.Consume([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"earlier assistant text"}]}}`)); err != nil {
		t.Fatal(err)
	}
	_, terminal, err := codec.Consume([]byte(`{"type":"result","subtype":"success","is_error":false,"result":"explicit result","usage":{"input_tokens":9007199254740993,"output_tokens":0},"total_cost_usd":1e-10}`))
	if err != nil || !terminal {
		t.Fatal("explicit terminal result was not observed")
	}
	result, err := codec.Result(nil)
	if err != nil || result.Outcome != "success" || result.Body != "explicit result" || result.Record == nil || result.Record.Usage == nil {
		t.Fatalf("result unavailable: %v", err)
	}
	usage := result.Record.Usage
	if usage.InputTokens == nil || *usage.InputTokens != "9007199254740993" || usage.OutputTokens == nil || *usage.OutputTokens != "0" ||
		usage.CostUSD == nil || *usage.CostUSD != "0.0000000001" || usage.CacheReadTokens != nil || usage.TotalTokens != nil {
		t.Fatal("explicit zero/large integer/cost or missing usage was changed")
	}
}

func TestClaudeStreamCodecRejectsExitOnlyAndIncompleteResults(t *testing.T) {
	for _, kind := range []string{"no_result", "missing_error_flag", "truncated", "two_results", "process_error", "prompt_too_long"} {
		t.Run(kind, func(t *testing.T) {
			var codec ClaudeStreamCodec
			line := `{"type":"result","is_error":false,"result":"answer"}`
			switch kind {
			case "no_result":
				line = `{"type":"assistant","message":{"content":[{"type":"text","text":"partial"}]}}`
			case "missing_error_flag":
				line = `{"type":"result","result":"answer"}`
			case "truncated":
				line = `{"type":"result","is_error":false,"result":`
			case "prompt_too_long":
				line = `{"type":"result","is_error":false,"terminal_reason":"prompt_too_long","result":""}`
			}
			_, _, _ = codec.Consume([]byte(line))
			if kind == "two_results" {
				_, _, _ = codec.Consume([]byte(line))
			}
			var exitErr error
			if kind == "process_error" {
				exitErr = errors.New("fixture process error")
			}
			result, err := codec.Result(exitErr)
			if kind == "prompt_too_long" {
				if err != nil || result.Outcome != "failure" || !strings.Contains(result.Body, "prompt_too_long") {
					t.Fatal("canonical structured failure was lost")
				}
			} else if err == nil {
				t.Fatalf("%s acquired a business result", kind)
			}
		})
	}
}

func TestClaudeStreamCodecControlKeepsExactInputAndForcesForeground(t *testing.T) {
	var codec ClaudeStreamCodec
	response, terminal, err := codec.Consume([]byte(`{"type":"control_request","request_id":"fixture-control","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"run_in_background":true,"count":9007199254740993}}}`))
	if err != nil || terminal || !strings.Contains(string(response), "9007199254740993") || !strings.Contains(string(response), `"run_in_background":false`) {
		t.Fatalf("control response lost exact/foreground input: %v", err)
	}
	var wire map[string]json.RawMessage
	if json.Unmarshal(response, &wire) != nil || string(wire["type"]) != `"control_response"` {
		t.Fatal("not a canonical control frame")
	}
}

func TestClaudeStreamCodecUsesCanonicalEmptyResultFallback(t *testing.T) {
	for _, tool := range []bool{false, true} {
		var codec ClaudeStreamCodec
		_, _, _ = codec.Consume([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"possible final text"}]}}`))
		if tool {
			_, _, _ = codec.Consume([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"call-1","name":"read","input":{}}]}}`))
		}
		_, _, _ = codec.Consume([]byte(`{"type":"result","is_error":false,"result":""}`))
		result, err := codec.Result(nil)
		if err != nil || (result.Body == "") != tool {
			t.Fatal("tool narration became an empty-result final answer")
		}
	}
}

func TestClaudeStreamCodecAggregatesOnlyFullyObservedUsage(t *testing.T) {
	var codec ClaudeStreamCodec
	_, _, err := codec.Consume([]byte(`{"type":"result","is_error":false,"result":"","modelUsage":{"one":{"inputTokens":9007199254740993,"outputTokens":0,"costUSD":"0.0000000001"},"two":{"inputTokens":2,"costUSD":"0.0000000002"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := codec.Result(nil)
	if err != nil {
		t.Fatal(err)
	}
	usage := result.Record.Usage
	if usage == nil || usage.InputTokens == nil || *usage.InputTokens != "9007199254740995" || usage.OutputTokens != nil || usage.CostUSD == nil || *usage.CostUSD != "0.0000000003" {
		t.Fatal("per-model presence/precision was invented or rounded")
	}
	if len(result.Record.Messages) != 0 {
		t.Fatal("usage produced synthetic messages")
	}
}
