package collaborationrun

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/redact"
)

type hostRecording struct {
	messages     []protocol.CollaborationRecordedMessage
	bytes        int
	partial      bool
	privatePaths []string
}

func newHostRecording(prepared *hostPreparedRun, executable string) *hostRecording {
	paths := []string{prepared.Temp, prepared.Work, prepared.Root, executable}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, home)
	}
	return &hostRecording{messages: []protocol.CollaborationRecordedMessage{}, privatePaths: paths}
}

func (r *hostRecording) text(value string, limit int) string {
	value = redact.Text(strings.ReplaceAll(value, "\x00", ""))
	for _, path := range r.privatePaths {
		if path != "" {
			value = strings.ReplaceAll(value, path, "[local]")
		}
	}
	if len(value) > limit {
		r.partial = true
		value = value[:limit]
		for !utf8.ValidString(value) && len(value) > 0 {
			value = value[:len(value)-1]
		}
	}
	return value
}

func (r *hostRecording) add(message protocol.CollaborationRecordedMessage) {
	message.Body = r.text(message.Body, protocol.CollaborationRecordTextLimit)
	if len(r.messages) >= protocol.CollaborationRecordItemLimit || r.bytes+len(message.Body) > 512<<10 {
		r.partial = true
		return
	}
	r.bytes += len(message.Body)
	r.messages = append(r.messages, message)
}

func (r *hostRecording) message(message agent.Message, activity *HostActivity) {
	activity.LastActivity.Store(time.Now().UnixNano())
	switch message.Type {
	case agent.MessageText:
		r.add(protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: message.Content})
	case agent.MessageToolUse:
		activity.InFlightTools.Add(1)
		if message.CallID == "" || message.Tool == "" {
			r.partial = true
			return
		}
		raw, err := json.Marshal(redact.InputMap(message.Input))
		if err != nil {
			r.partial = true
			return
		}
		r.add(protocol.CollaborationRecordedMessage{Kind: "tool_call", CallID: message.CallID, Name: message.Tool, Body: string(raw)})
	case agent.MessageToolResult:
		for current := activity.InFlightTools.Load(); current > 0; current = activity.InFlightTools.Load() {
			if activity.InFlightTools.CompareAndSwap(current, current-1) {
				break
			}
		}
		if message.CallID == "" {
			r.partial = true
			return
		}
		isError := message.Status == "error"
		r.add(protocol.CollaborationRecordedMessage{Kind: "tool_result", CallID: message.CallID, IsError: &isError, Body: message.Output})
	}
}

func (h *HostExecutor) execute(ctx context.Context, cancel context.CancelFunc, backend agent.Backend, prompt string, opts agent.ExecOptions, record *hostRecording) (agent.Result, bool, error) {
	executionCtx, executionCancel := context.WithCancel(ctx)
	defer executionCancel()
	session, err := backend.Execute(executionCtx, prompt, opts)
	if err != nil {
		return agent.Result{}, false, err
	}
	if session == nil || session.Result == nil {
		return agent.Result{}, false, ErrProtocol
	}
	activity := &HostActivity{}
	activity.LastActivity.Store(time.Now().UnixNano())
	if h.Watchdog != nil {
		go h.Watchdog(executionCtx, activity, cancel, session.Messages)
	}
	messages, results := session.Messages, session.Result
	var result agent.Result
	gotResult := false
	cancelled := ctx.Done()
	var deadline <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for messages != nil || results != nil {
		select {
		case message, ok := <-messages:
			if !ok {
				messages = nil
				continue
			}
			record.message(message, activity)
		case value, ok := <-results:
			results = nil
			if ok {
				result, gotResult = value, true
			}
			if messages != nil && timer == nil {
				timer = time.NewTimer(time.Second)
				deadline = timer.C
			}
		case <-cancelled:
			cancelled = nil
			opts.Lifecycle.Stop()
			if timer == nil {
				timer = time.NewTimer(hostStopGrace)
				deadline = timer.C
			}
		case <-deadline:
			record.partial = true
			return result, gotResult, errors.New("collaboration provider recording incomplete")
		}
	}
	if gotResult && activity.WatchdogFired.Load() {
		result.Status = "timeout"
		result.Error = "Execution stopped after the ordinary agent inactivity limit."
	}
	return result, gotResult, nil
}

func mergeHostUsage(first, second map[string]agent.TokenUsage) map[string]agent.TokenUsage {
	if len(first) == 0 {
		return second
	}
	merged := make(map[string]agent.TokenUsage, len(first)+len(second))
	for model, usage := range first {
		merged[model] = usage
	}
	for model, usage := range second {
		prior := merged[model]
		prior.InputTokens += usage.InputTokens
		prior.OutputTokens += usage.OutputTokens
		prior.CacheReadTokens += usage.CacheReadTokens
		prior.CacheWriteTokens += usage.CacheWriteTokens
		prior.CostUSDTicks += usage.CostUSDTicks
		merged[model] = prior
	}
	return merged
}

func hostUsage(usage map[string]agent.TokenUsage) (*protocol.CollaborationRunUsage, bool) {
	if len(usage) == 0 {
		return nil, true
	}
	var totals [5]int64
	for _, model := range usage {
		for i, count := range []int64{model.InputTokens, model.OutputTokens, model.CacheReadTokens, model.CacheWriteTokens, model.CostUSDTicks} {
			if count < 0 || count > math.MaxInt64-totals[i] {
				return nil, false
			}
			totals[i] += count
		}
	}
	values := [4]string{}
	for i := range values {
		values[i] = strconv.FormatInt(totals[i], 10)
	}
	out := &protocol.CollaborationRunUsage{InputTokens: &values[0], OutputTokens: &values[1], CacheReadTokens: &values[2], CacheCreationTokens: &values[3]}
	if totals[4] > 0 {
		// Fixed ten-place integer formatting avoids float rounding of cost.
		fraction := strconv.FormatInt(totals[4]%agent.CostUSDTicksPerUSD, 10)
		cost := strconv.FormatInt(totals[4]/agent.CostUSDTicksPerUSD, 10) + "." + strings.Repeat("0", 10-len(fraction)) + fraction
		out.CostUSD = &cost
	}
	return out, true
}

func (r *hostRecording) outcome(identity protocol.CollaborationRunIdentity, seq, nonce string, result agent.Result) (protocol.CollaborationRunOutcome, bool) {
	if result.BusinessOutcomeMissing {
		r.partial = true
		return protocol.CollaborationRunOutcome{}, false
	}
	kind := ""
	switch result.Status {
	case "completed":
		kind = protocol.CollaborationBusinessOutcomeSuccess
	case "failed", "aborted", "cancelled", "timeout", "idle_watchdog":
		kind = protocol.CollaborationBusinessOutcomeFailure
	default:
		return protocol.CollaborationRunOutcome{}, false
	}
	body := result.Output
	if body == "" && kind == protocol.CollaborationBusinessOutcomeFailure {
		body = result.Error
	}
	body = r.text(body, 256<<10)
	usage, valid := hostUsage(result.Usage)
	if !valid {
		r.partial = true
	}
	completeness := "complete"
	if r.partial {
		completeness = "partial"
	}
	out := protocol.CollaborationRunOutcome{ProtocolVersion: protocol.CollaborationHostRuntimeProtocolVersion, Containment: protocol.CollaborationContainmentHostProcessSupervision,
		Identity: identity, EventID: uuid.NewString(), AttemptSeq: seq, AttemptNonce: nonce, Outcome: kind, Body: body,
		Record: &protocol.CollaborationRunRecord{Version: 1, Completeness: completeness, Messages: r.messages, Usage: usage}}
	if protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: kind, Body: body, Record: out.Record}) != nil {
		return protocol.CollaborationRunOutcome{}, false
	}
	return out, true
}
