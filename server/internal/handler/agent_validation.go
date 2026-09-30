package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/agentconfig"
)

func normaliseAgentCategory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 50 {
		return "", fmt.Errorf("category must be 50 characters or fewer")
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return "", fmt.Errorf("category must not contain control characters")
	}
	return value, nil
}

func validateAgentMaxConcurrentTasks(value int32) error {
	if err := agentconfig.ValidateMaxConcurrentTasks(value); err != nil {
		return fmt.Errorf("max_concurrent_tasks %w", err)
	}
	return nil
}

func defaultAndValidateAgentMaxConcurrentTasks(rawFields map[string]json.RawMessage, value *int32) error {
	raw, provided := rawFields["max_concurrent_tasks"]
	if !provided || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		*value = agentconfig.DefaultMaxConcurrentTasks
		return nil
	}
	return validateAgentMaxConcurrentTasks(*value)
}
