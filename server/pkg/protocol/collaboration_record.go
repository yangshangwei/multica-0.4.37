package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	CollaborationOutcomeBodyLimit = 1 << 20
	CollaborationRecordJSONLimit  = 1 << 20
	CollaborationOutcomeJSONLimit = 2 << 20
	CollaborationRecordTextLimit  = 64 << 10
	CollaborationRecordItemLimit  = 256
)

// Only explicit provider output is recorded. No console parsing or token/cost
// estimation can turn absent telemetry into an observed value.
type CollaborationRunUsage struct {
	InputTokens         *string `json:"input_tokens"`
	OutputTokens        *string `json:"output_tokens"`
	CacheReadTokens     *string `json:"cache_read_tokens"`
	CacheCreationTokens *string `json:"cache_creation_tokens"`
	TotalTokens         *string `json:"total_tokens"`
	CostUSD             *string `json:"cost_usd"`
}

// Kind determines the permitted fields; ValidateCollaborationRunRecord rejects
// cross-kind fields instead of exposing arbitrary provider-native objects.
type CollaborationRecordedMessage struct {
	Kind    string `json:"kind"`
	Body    string `json:"body"`
	Role    string `json:"role,omitempty"`
	CallID  string `json:"call_id,omitempty"`
	Name    string `json:"name,omitempty"`
	IsError *bool  `json:"is_error,omitempty"`
}

func (m *CollaborationRecordedMessage) UnmarshalJSON(data []byte) error {
	type messageWire CollaborationRecordedMessage
	var value messageWire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return errors.New("invalid collaboration message")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return errors.New("invalid collaboration message")
	}
	required := []string{"kind", "body"}
	switch value.Kind {
	case "message":
		required = append(required, "role")
	case "tool_call":
		required = append(required, "call_id", "name")
	case "tool_result":
		required = append(required, "call_id", "is_error")
	default:
		return errors.New("invalid collaboration message")
	}
	if len(fields) != len(required) {
		return errors.New("invalid collaboration message")
	}
	for _, key := range required {
		if raw, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errors.New("invalid collaboration message")
		}
	}
	*m = CollaborationRecordedMessage(value)
	return nil
}

type CollaborationRunRecord struct {
	Version      int                            `json:"version"`
	Completeness string                         `json:"completeness"`
	Messages     []CollaborationRecordedMessage `json:"messages"`
	Usage        *CollaborationRunUsage         `json:"usage,omitempty"`
}

// CollaborationProviderResult is the supported static executor's stdout wire.
// Its optional recording is additive to the original outcome/body envelope.
type CollaborationProviderResult struct {
	Outcome string                  `json:"outcome"`
	Body    string                  `json:"body"`
	Record  *CollaborationRunRecord `json:"record,omitempty"`
}

var collaborationCostPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,18})(\.[0-9]{1,10})?$`)

func validRecordedCounter(value *string) bool {
	if value == nil {
		return true
	}
	n, err := strconv.ParseInt(*value, 10, 64)
	return err == nil && n >= 0 && strconv.FormatInt(n, 10) == *value
}

func ValidateCollaborationRunRecord(record *CollaborationRunRecord) error {
	invalid := errors.New("invalid collaboration recording")
	if record == nil {
		return nil
	}
	if record.Version != 1 || (record.Completeness != "complete" && record.Completeness != "partial") || record.Messages == nil || len(record.Messages) > CollaborationRecordItemLimit {
		return invalid
	}
	for _, message := range record.Messages {
		if !utf8.ValidString(message.Body) || strings.ContainsRune(message.Body, 0) || len(message.Body) > CollaborationRecordTextLimit {
			return invalid
		}
		validName := func(value string) bool {
			return value != "" && len(value) <= 128 && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
		}
		switch message.Kind {
		case "message":
			if (message.Role != "assistant" && message.Role != "user" && message.Role != "system") || message.CallID != "" || message.Name != "" || message.IsError != nil {
				return invalid
			}
		case "tool_call":
			if !validName(message.CallID) || !validName(message.Name) || message.Role != "" || message.IsError != nil {
				return invalid
			}
		case "tool_result":
			if !validName(message.CallID) || message.Role != "" || message.Name != "" || message.IsError == nil {
				return invalid
			}
		default:
			return invalid
		}
	}
	if usage := record.Usage; usage != nil {
		for _, counter := range []*string{usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheCreationTokens, usage.TotalTokens} {
			if !validRecordedCounter(counter) {
				return invalid
			}
		}
		if usage.CostUSD != nil && !collaborationCostPattern.MatchString(*usage.CostUSD) {
			return invalid
		}
	}
	return nil
}

func ValidateCollaborationProviderResult(result CollaborationProviderResult) error {
	if (result.Outcome != CollaborationBusinessOutcomeSuccess && result.Outcome != CollaborationBusinessOutcomeFailure) || !utf8.ValidString(result.Body) || strings.ContainsRune(result.Body, 0) || len(result.Body) > CollaborationOutcomeBodyLimit {
		return errors.New("invalid collaboration outcome")
	}
	if err := ValidateCollaborationRunRecord(result.Record); err != nil {
		return err
	}
	if result.Record != nil {
		encoded, err := json.Marshal(result)
		if err != nil || len(encoded) > CollaborationRecordJSONLimit {
			return errors.New("collaboration recording too large")
		}
	}
	return nil
}
