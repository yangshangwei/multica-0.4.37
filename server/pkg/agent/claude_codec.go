package agent

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"strconv"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

var errClaudeStream = errors.New("incomplete Claude stream result")

// ClaudeFreshStreamArguments is a pure invocation leaf. The caller owns the
// process and its isolation. There is no account discovery, resume, fallback,
// custom configuration or ordinary executor behind this function.
func ClaudeFreshStreamArguments(instructions, model, thinking string) ([]string, error) {
	if protocol.ValidateCollaborationPreparationInvocation(protocol.CollaborationPreparationInvocation{Instructions: instructions, Model: model, ThinkingLevel: thinking}) != nil ||
		thinking != "" && !IsKnownThinkingValue("claude", thinking) {
		return nil, errClaudeStream
	}
	args := buildClaudeArgs(ExecOptions{Model: model, ThinkingLevel: thinking, McpConfig: json.RawMessage(`{}`)}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Committed project settings, hooks and skills remain readable task data;
	// they cannot become runtime configuration outside the frozen invocation.
	// Keep these options mandatory: unsupported CLIs must fail without fallback.
	args = append(args, "--setting-sources=", "--disable-slash-commands", "--settings", `{"disableAllHooks":true}`,
		"--mcp-config", `{"mcpServers":{}}`)
	if instructions != "" {
		args = append(args, "--append-system-prompt", instructions)
	}
	return args, nil
}

func ClaudeStreamInput(prompt string) ([]byte, error) { return buildClaudeInput(prompt) }

// ClaudeStreamCodec has no process, environment or filesystem access. Its
// terminal/fallback and control semantics reuse the ordinary Claude leaves;
// telemetry retains raw numeric presence instead of zero-valued TokenUsage.
type ClaudeStreamCodec struct {
	state        streamTerminalState
	messages     []protocol.CollaborationRecordedMessage
	usage        *protocol.CollaborationRunUsage
	partial      bool
	invalid      bool
	messageBytes int
	sawAsync     bool
}

// Read drains the canonical stream scanner while keeping stdin open for tool
// control replies. The process owner writes the initial frame concurrently.
func (c *ClaudeStreamCodec) Read(source io.Reader, replies io.WriteCloser) error {
	scanner := newAgentStreamScanner(source)
	for scanner.Scan() {
		line := scanner.Bytes()
		if strings.TrimSpace(string(line)) == "" {
			continue
		}
		response, terminal, err := c.Consume(line)
		if err != nil {
			return err
		}
		if len(response) != 0 {
			if _, err := replies.Write(response); err != nil {
				c.invalid = true
				return errClaudeStream
			}
		}
		if terminal {
			_ = replies.Close()
		}
	}
	if scanner.Err() != nil {
		c.invalid = true
		return errClaudeStream
	}
	return nil
}

func (c *ClaudeStreamCodec) Consume(line []byte) ([]byte, bool, error) {
	if c.invalid || len(line) > protocol.CollaborationOutcomeJSONLimit {
		c.invalid = true
		return nil, false, errClaudeStream
	}
	var frame struct {
		Type           string                                `json:"type"`
		Message        json.RawMessage                       `json:"message"`
		Request        json.RawMessage                       `json:"request"`
		RequestID      string                                `json:"request_id"`
		Result         string                                `json:"result"`
		IsError        *bool                                 `json:"is_error"`
		TerminalReason string                                `json:"terminal_reason"`
		Usage          map[string]json.RawMessage            `json:"usage"`
		ModelUsage     map[string]map[string]json.RawMessage `json:"modelUsage"`
		Cost           json.RawMessage                       `json:"total_cost_usd"`
	}
	if json.Unmarshal(line, &frame) != nil || frame.Type == "" {
		c.invalid = true
		return nil, false, errClaudeStream
	}
	switch frame.Type {
	case "assistant":
		ch := make(chan Message, protocol.CollaborationRecordItemLimit)
		turn := handleClaudeAssistant(claudeSDKMessage{Message: frame.Message}, ch, make(map[string]TokenUsage))
		close(ch)
		c.state.lastAssistantText = turn.resolveFallback(c.state.lastAssistantText)
		c.partial = c.partial || !turn.understood
		for message := range ch {
			switch message.Type {
			case MessageText:
				c.record(protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: message.Content})
			case MessageToolUse:
				body, err := json.Marshal(message.Input)
				if err != nil {
					c.partial = true
					continue
				}
				c.record(protocol.CollaborationRecordedMessage{Kind: "tool_call", CallID: message.CallID, Name: message.Tool, Body: string(body)})
			}
		}
		// If the canonical nonblocking channel filled, its record is partial.
		var content struct {
			Content []json.RawMessage `json:"content"`
		}
		if json.Unmarshal(frame.Message, &content) != nil || len(content.Content) > protocol.CollaborationRecordItemLimit {
			c.partial = true
		}
	case "user":
		var content struct {
			Content []struct {
				Type      string          `json:"type"`
				ToolUseID string          `json:"tool_use_id"`
				Content   json.RawMessage `json:"content"`
				IsError   *bool           `json:"is_error"`
			} `json:"content"`
		}
		if json.Unmarshal(frame.Message, &content) != nil {
			c.partial = true
			break
		}
		for _, block := range content.Content {
			if block.Type != "tool_result" || block.IsError == nil {
				c.partial = true
				continue
			}
			c.sawAsync = c.sawAsync || claudeToolResultHasAsyncLaunch(block.Content)
			c.record(protocol.CollaborationRecordedMessage{Kind: "tool_result", CallID: block.ToolUseID, Body: string(block.Content), IsError: block.IsError})
		}
	case "control_request":
		var request claudeControlRequestPayload
		if c.state.sawResult || frame.RequestID == "" || len(frame.RequestID) > 128 || json.Unmarshal(frame.Request, &request) != nil || request.Subtype != "can_use_tool" {
			c.invalid = true
			return nil, false, errClaudeStream
		}
		response, _, _, err := buildClaudeControlResponse(claudeSDKMessage{RequestID: frame.RequestID, Request: frame.Request})
		if err != nil {
			c.invalid = true
			return nil, false, errClaudeStream
		}
		return response, false, nil
	case "result":
		if c.state.sawResult || frame.IsError == nil {
			c.invalid = true
			return nil, false, errClaudeStream
		}
		c.state.sawResult, c.state.resultIsError, c.state.finalResultText = true, *frame.IsError, frame.Result
		c.state.terminalReasonError = claudeTerminalReasonFailure(frame.TerminalReason, frame.Result)
		c.usage = c.readUsage(frame.Usage, frame.ModelUsage, frame.Cost)
		return nil, true, nil
	case "system", "log", "rate_limit_event":
		// Lifecycle/log frames are not arbitrary objects in the retained record.
	default:
		c.partial = true
	}
	return nil, false, nil
}

func (c *ClaudeStreamCodec) record(message protocol.CollaborationRecordedMessage) {
	encoded, err := json.Marshal(message)
	if err != nil || len(message.Body) > protocol.CollaborationRecordTextLimit || len(c.messages) >= protocol.CollaborationRecordItemLimit ||
		c.messageBytes+len(encoded) > protocol.CollaborationRecordJSONLimit/2 ||
		protocol.ValidateCollaborationRunRecord(&protocol.CollaborationRunRecord{Version: 1, Completeness: "complete", Messages: []protocol.CollaborationRecordedMessage{message}}) != nil {
		c.partial = true
		return
	}
	c.messageBytes += len(encoded)
	c.messages = append(c.messages, message)
}

func (c *ClaudeStreamCodec) Result(exitErr error) (protocol.CollaborationProviderResult, error) {
	var empty protocol.CollaborationProviderResult
	if c.invalid || !c.state.sawResult || exitErr != nil && !c.state.resultIsError && c.state.terminalReasonError == "" {
		return empty, errClaudeStream
	}
	guard := ""
	if c.sawAsync {
		guard = "claude launched an async background task; Multica-managed runs require foreground execution"
	}
	status, output, detail := finalizeStreamResult("claude", 0, nil, nil, nil, "", c.state, guard)
	result := protocol.CollaborationProviderResult{Outcome: protocol.CollaborationBusinessOutcomeSuccess, Body: output}
	if status != "completed" {
		result.Outcome, result.Body = protocol.CollaborationBusinessOutcomeFailure, detail
	}
	messages := c.messages
	if messages == nil {
		messages = []protocol.CollaborationRecordedMessage{}
	}
	result.Record = &protocol.CollaborationRunRecord{Version: 1, Completeness: "complete", Messages: messages, Usage: c.usage}
	if c.partial {
		result.Record.Completeness = "partial"
	}
	if protocol.ValidateCollaborationProviderResult(result) != nil {
		// Keep the actual terminal text if the optional transcript exhausts the
		// envelope, without presenting a truncated transcript as complete.
		result.Record.Messages = []protocol.CollaborationRecordedMessage{}
		result.Record.Completeness = "partial"
		if protocol.ValidateCollaborationProviderResult(result) != nil {
			return empty, errClaudeStream
		}
	}
	return result, nil
}

func (c *ClaudeStreamCodec) readUsage(usage map[string]json.RawMessage, models map[string]map[string]json.RawMessage, cost json.RawMessage) *protocol.CollaborationRunUsage {
	if len(usage) == 0 && len(models) == 0 && (len(cost) == 0 || string(cost) == "null") {
		return nil
	}
	read := func(key, modelKey string, costField bool) *string {
		raw := usage[key]
		if costField {
			raw = cost
		}
		if len(raw) > 0 && string(raw) != "null" {
			value, ok := exactClaudeNumber(raw, costField)
			if !ok {
				c.partial = true
				return nil
			}
			return &value
		}
		if len(models) == 0 || len(models) > protocol.CollaborationRecordItemLimit {
			return nil
		}
		sum := new(big.Rat)
		for _, fields := range models {
			raw := fields[modelKey]
			if len(raw) == 0 || string(raw) == "null" {
				return nil
			}
			value, ok := exactClaudeNumber(raw, costField)
			if !ok {
				c.partial = true
				return nil
			}
			number, ok := new(big.Rat).SetString(value)
			if !ok {
				c.partial = true
				return nil
			}
			sum.Add(sum, number)
		}
		scale := 0
		if costField {
			scale = 10
		}
		value, ok := exactClaudeNumber(json.RawMessage(sum.FloatString(scale)), costField)
		if !ok {
			c.partial = true
			return nil
		}
		return &value
	}
	return &protocol.CollaborationRunUsage{InputTokens: read("input_tokens", "inputTokens", false), OutputTokens: read("output_tokens", "outputTokens", false),
		CacheReadTokens: read("cache_read_input_tokens", "cacheReadInputTokens", false), CacheCreationTokens: read("cache_creation_input_tokens", "cacheCreationInputTokens", false),
		TotalTokens: read("total_tokens", "totalTokens", false), CostUSD: read("", "costUSD", true)}
}

// Normalize only exact decimal arithmetic. Float64 is never an intermediate;
// an unrepresentable field remains unknown rather than rounded or estimated.
func exactClaudeNumber(raw json.RawMessage, cost bool) (string, bool) {
	value := string(raw)
	if len(value) == 0 || len(value) > 128 {
		return "", false
	}
	if value[0] == '"' && json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	if !json.Valid([]byte(value)) || value == "" || value[0] < '0' || value[0] > '9' {
		return "", false
	}
	exponent := 0
	if index := strings.IndexAny(value, "eE"); index >= 0 {
		var err error
		exponent, err = strconv.Atoi(value[index+1:])
		if err != nil || exponent < -100 || exponent > 100 {
			return "", false
		}
		value = value[:index]
	}
	whole, fraction, _ := strings.Cut(value, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "0", true
	}
	scale := len(fraction) - exponent
	for scale > 0 && strings.HasSuffix(digits, "0") {
		digits, scale = strings.TrimSuffix(digits, "0"), scale-1
	}
	if scale <= 0 {
		digits += strings.Repeat("0", -scale)
		scale = 0
	} else if !cost || scale > 10 {
		return "", false
	}
	if scale > 0 {
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale+1-len(digits)) + digits
		}
		digits = digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	}
	if cost {
		integer, _, _ := strings.Cut(digits, ".")
		return digits, len(integer) <= 19
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	return digits, err == nil && n >= 0
}
