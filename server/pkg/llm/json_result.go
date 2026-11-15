package llm

import (
	"context"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/respjson"
)

// UsageStatus describes the presence of provider-returned token counts, not a
// billing estimate or verification of the provider's accounting.
type UsageStatus string

const (
	UsageUnknown UsageStatus = "unknown"
	UsagePartial UsageStatus = "partial"
	UsageKnown   UsageStatus = "known"
)

// JSONUsage retains missing counts as nil and actual zero as a non-nil pointer.
// Known means all three primary totals were returned; optional breakdowns may
// still be absent. No missing total is inferred from the other counts.
type JSONUsage struct {
	Status            UsageStatus `json:"status"`
	InputTokens       *int64      `json:"input_tokens"`
	OutputTokens      *int64      `json:"output_tokens"`
	TotalTokens       *int64      `json:"total_tokens"`
	CachedInputTokens *int64      `json:"cached_input_tokens"`
	ReasoningTokens   *int64      `json:"reasoning_tokens"`
}

// JSONResult carries private, unparsed output and provider-returned accounting.
// Model is empty when the response did not identify it; the requested model is
// never substituted. Callers must preserve Usage even when generation fails.
type JSONResult struct {
	Text  string    `json:"text"`
	Model string    `json:"model"`
	Usage JSONUsage `json:"usage"`
}

// GenerateJSONResult is the accounting-aware form of GenerateJSON.
func (c *Client) GenerateJSONResult(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64, maxCompletionTokens int64) (JSONResult, error) {
	return c.generateJSONResult(ctx, model, systemPrompt, userPrompt, temperature, maxCompletionTokens)
}

func jsonUsage(completion *openai.ChatCompletion) JSONUsage {
	result := JSONUsage{Status: UsageUnknown}
	if !completion.JSON.Usage.Valid() {
		return result
	}
	usage := completion.Usage
	result.InputTokens = returnedTokenCount(usage.PromptTokens, usage.JSON.PromptTokens)
	result.OutputTokens = returnedTokenCount(usage.CompletionTokens, usage.JSON.CompletionTokens)
	result.TotalTokens = returnedTokenCount(usage.TotalTokens, usage.JSON.TotalTokens)
	if usage.JSON.PromptTokensDetails.Valid() {
		result.CachedInputTokens = returnedTokenCount(usage.PromptTokensDetails.CachedTokens, usage.PromptTokensDetails.JSON.CachedTokens)
	}
	if usage.JSON.CompletionTokensDetails.Valid() {
		result.ReasoningTokens = returnedTokenCount(usage.CompletionTokensDetails.ReasoningTokens, usage.CompletionTokensDetails.JSON.ReasoningTokens)
	}
	if result.InputTokens != nil && result.OutputTokens != nil && result.TotalTokens != nil {
		result.Status = UsageKnown
	} else if result.InputTokens != nil || result.OutputTokens != nil || result.TotalTokens != nil || result.CachedInputTokens != nil || result.ReasoningTokens != nil {
		result.Status = UsagePartial
	}
	return result
}

func returnedTokenCount(value int64, field respjson.Field) *int64 {
	if !field.Valid() || value < 0 {
		return nil
	}
	return &value
}
