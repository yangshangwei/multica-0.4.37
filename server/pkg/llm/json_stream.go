package llm

import (
	"context"
	"errors"
	"strings"
)

// GenerateJSONStream is GenerateJSON's streaming sibling. onDelta receives raw
// JSON fragments, not parsed fields, and may return an error to stop generation
// (for example when a caller's response-size limit is reached). The assembled
// result is usable only when this method returns without an error. Callers own
// JSON/schema validation. No stream is restarted after content has been seen.
func (c *Client) GenerateJSONStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64, maxCompletionTokens int64, onDelta func(string) error) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	params := c.jsonCompletionParams(model, systemPrompt, userPrompt, temperature, maxCompletionTokens)
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()
	for compatibilityRetries := 0; ; compatibilityRetries++ {
		stream, err := c.ChatStream(ctx, params)
		if err != nil {
			return "", err
		}
		var content strings.Builder
		finish := ""
		for stream.Next() {
			chunk := stream.Current()
			for _, choice := range chunk.Choices {
				if choice.Index != 0 {
					continue
				}
				if delta := choice.Delta.Content; delta != "" {
					if onDelta != nil {
						if err := onDelta(delta); err != nil {
							_ = stream.Close()
							return "", err
						}
					}
					content.WriteString(delta)
				}
				if choice.FinishReason != "" {
					finish = choice.FinishReason
				}
			}
		}
		err = stream.Err()
		closeErr := stream.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			if content.Len() == 0 && compatibilityRetries < 2 && negotiateJSONParameters(&params, err) {
				continue
			}
			return "", err
		}
		if finish != "stop" {
			return "", errors.New("llm: upstream stream ended without a complete JSON response")
		}
		if strings.TrimSpace(content.String()) == "" {
			return "", errors.New("llm: upstream returned empty JSON content")
		}
		return content.String(), nil
	}
}
