package iteration

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidateObjectJSON preserves raw intent before typed decoding: duplicate
// keys (including aliases that encoding/json would match case-insensitively),
// excessive nesting and trailing values must not disappear during decoding.
func ValidateObjectJSON(raw []byte) error {
	value, err := readCanonicalJSON(raw)
	if err != nil {
		return err
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("request must be an object")
	}
	return validateObjectAliases(value)
}
func validateObjectAliases(value any) error {
	switch value := value.(type) {
	case map[string]any:
		seen := map[string]bool{}
		for key, child := range value {
			folded := foldJSONField(key)
			if seen[folded] {
				return fmt.Errorf("duplicate field alias %q", key)
			}
			seen[folded] = true
			if err := validateObjectAliases(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := validateObjectAliases(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// Match encoding/json's Unicode simple-fold aliases, including long s and
// Kelvin sign, rather than only ASCII or lowercase conversion.
func foldJSONField(key string) string {
	var folded strings.Builder
	for _, r := range key {
		smallest := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < smallest {
				smallest = next
			}
		}
		folded.WriteRune(smallest)
	}
	return folded.String()
}
