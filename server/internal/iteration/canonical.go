package iteration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CanonicalDraftJSON validates the wire draft and normalizes intent before
// hashing or decoding into the shared Draft DTO. Authorization, affected-set
// completeness and state-dependent eligibility remain transaction checks.
func CanonicalDraftJSON(raw []byte) ([]byte, error) {
	value, err := readCanonicalJSON(raw)
	if err != nil {
		return nil, err
	}
	d, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("draft must be an object")
	}
	if err := onlyKeys(d, "operation", "iteration_id", "expected_iteration_revision", "expected_scope_revision", "expected_settings_revision", "reason", "moves", "start"); err != nil {
		return nil, err
	}
	for _, key := range []string{"iteration_id", "expected_iteration_revision", "expected_scope_revision", "reason", "start"} {
		if _, ok := d[key]; !ok {
			d[key] = nil
		}
	}
	if _, ok := d["moves"]; !ok {
		d["moves"] = []any{}
	}
	if _, err := normalizeCanonical(d, ""); err != nil {
		return nil, err
	}
	operation, ok := d["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation is required")
	}
	switch operation {
	case "start", "move", "end", "cancel", "disable", "delete", "handoff":
	default:
		return nil, fmt.Errorf("unknown operation %q", operation)
	}
	if err := revisionField(d, "expected_settings_revision", false); err != nil {
		return nil, err
	}
	for _, key := range []string{"expected_iteration_revision", "expected_scope_revision"} {
		if err := revisionField(d, key, true); err != nil {
			return nil, err
		}
	}
	if err := uuidField(d, "iteration_id", true); err != nil {
		return nil, err
	}
	if d["reason"] != nil {
		if _, ok := d["reason"].(string); !ok {
			return nil, fmt.Errorf("reason must be a string or null")
		}
	}
	moves, ok := d["moves"].([]any)
	if !ok {
		return nil, fmt.Errorf("moves must be an array")
	}
	for _, value := range moves {
		move, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("move must be an object")
		}
		if err := onlyKeys(move, "issue_id", "expected_issue_revision", "expected_source_id", "target_id", "allow_completed"); err != nil {
			return nil, err
		}
		for _, key := range []string{"expected_source_id", "target_id"} {
			if _, ok := move[key]; !ok {
				move[key] = nil
			}
		}
		if err := uuidField(move, "issue_id", false); err != nil {
			return nil, err
		}
		for _, key := range []string{"expected_source_id", "target_id"} {
			if err := uuidField(move, key, true); err != nil {
				return nil, err
			}
		}
		if err := revisionField(move, "expected_issue_revision", false); err != nil {
			return nil, err
		}
		if _, ok := move["allow_completed"].(bool); !ok {
			return nil, fmt.Errorf("allow_completed must be a boolean")
		}
	}
	if err := sortUniqueIssueObjects(moves); err != nil {
		return nil, err
	}
	if operation == "start" || operation == "handoff" {
		start, ok := d["start"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("start is required for %s", operation)
		}
		if err := onlyKeys(start, "target_id", "mode", "terminal_choices"); err != nil {
			return nil, err
		}
		if err := uuidField(start, "target_id", false); err != nil {
			return nil, err
		}
		if start["mode"] != "scheduled" && start["mode"] != "today" {
			return nil, fmt.Errorf("invalid start mode")
		}
		choices, ok := start["terminal_choices"].([]any)
		if !ok {
			return nil, fmt.Errorf("terminal_choices must be an array")
		}
		for _, value := range choices {
			choice, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("terminal choice must be an object")
			}
			if err := onlyKeys(choice, "issue_id", "retain"); err != nil {
				return nil, err
			}
			if err := uuidField(choice, "issue_id", false); err != nil {
				return nil, err
			}
			if _, ok := choice["retain"].(bool); !ok {
				return nil, fmt.Errorf("retain must be a boolean")
			}
		}
		if err := sortUniqueIssueObjects(choices); err != nil {
			return nil, err
		}
	} else if d["start"] != nil {
		return nil, fmt.Errorf("start does not apply to %s", operation)
	}
	if (operation == "start" || operation == "disable" || operation == "delete") && len(moves) > 0 {
		return nil, fmt.Errorf("moves do not apply to %s", operation)
	}
	if operation == "disable" {
		if d["iteration_id"] != nil || d["expected_iteration_revision"] != nil || d["expected_scope_revision"] != nil {
			return nil, fmt.Errorf("disable is workspace scoped")
		}
	} else if operation != "move" && (d["iteration_id"] == nil || d["expected_iteration_revision"] == nil || d["expected_scope_revision"] == nil) {
		return nil, fmt.Errorf("iteration identity and revisions are required")
	}
	return json.Marshal(d)
}

// CanonicalHash hashes a complete, explicitly selected comparison projection.
// Callers include workspace, actor and all relevant revisions/facts, exclude
// volatile preview timestamps, and sort semantically unordered arrays. Use
// json.RawMessage for bytes returned by CanonicalDraftJSON, not a byte slice.
// A hash is a comparison token, never evidence of authorization.
func CanonicalHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	decoded, err := readCanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	normalized, err := normalizeCanonical(decoded, "")
	if err != nil {
		return "", err
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func onlyKeys(object map[string]any, keys ...string) error {
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	for key := range object {
		if !allowed[key] {
			return fmt.Errorf("unknown field %q", key)
		}
	}
	return nil
}

func revisionField(object map[string]any, key string, nullable bool) error {
	value, exists := object[key]
	if nullable && exists && value == nil {
		return nil
	}
	number, ok := value.(json.Number)
	if !ok {
		return fmt.Errorf("%s must be a positive safe integer", key)
	}
	parsed, err := number.Int64()
	if err != nil || parsed <= 0 || parsed > 9007199254740991 {
		return fmt.Errorf("%s must be a positive safe integer", key)
	}
	return nil
}

func uuidField(object map[string]any, key string, nullable bool) error {
	value, exists := object[key]
	if nullable && exists && value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok || !canonicalUUID(text) {
		return fmt.Errorf("%s must be a UUID", key)
	}
	object[key] = strings.ToLower(text)
	return nil
}

func canonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func sortUniqueIssueObjects(values []any) error {
	sort.Slice(values, func(i, j int) bool {
		return values[i].(map[string]any)["issue_id"].(string) < values[j].(map[string]any)["issue_id"].(string)
	})
	for i := 1; i < len(values); i++ {
		if values[i].(map[string]any)["issue_id"] == values[i-1].(map[string]any)["issue_id"] {
			return fmt.Errorf("duplicate issue_id")
		}
	}
	return nil
}

func normalizeCanonical(value any, key string) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			normalized, err := normalizeCanonical(item, k)
			if err != nil {
				return nil, err
			}
			v[k] = normalized
		}
	case []any:
		for i, item := range v {
			normalized, err := normalizeCanonical(item, key)
			if err != nil {
				return nil, err
			}
			v[i] = normalized
		}
	case string:
		v = strings.TrimSpace(strings.ReplaceAll(v, "\r\n", "\n"))
		if (key == "id" || key == "recipients" || strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids")) && canonicalUUID(v) {
			v = strings.ToLower(v)
		}
		if key == "date" || strings.HasSuffix(key, "_date") {
			if _, err := ParseDate(v); err != nil {
				return nil, err
			}
		}
		if strings.HasSuffix(key, "_at") {
			stamp, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return nil, fmt.Errorf("invalid timestamp %s", key)
			}
			v = stamp.UTC().Format(time.RFC3339Nano)
		}
		return v, nil
	case json.Number:
		// Validate the original token before float conversion can round a
		// fractional revision into an apparently valid integer.
		if key == "revision" || strings.HasSuffix(key, "_revision") {
			i, err := v.Int64()
			if err != nil || i <= 0 || i > 9007199254740991 {
				return nil, fmt.Errorf("%s must be a positive safe integer", key)
			}
			return json.Number(strconv.FormatInt(i, 10)), nil
		}
		// Integer revisions retain exact precision; floats are needed for ratios.
		if i, err := v.Int64(); err == nil {
			return json.Number(strconv.FormatInt(i, 10)), nil
		}
		f, err := v.Float64()
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("invalid number")
		}
		return json.Number(strconv.FormatFloat(f, 'g', -1, 64)), nil
	}
	return value, nil
}

func readCanonicalJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := readCanonicalValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("expected one JSON value")
	}
	return value, nil
}

// Token decoding rejects duplicate object keys instead of silently replacing
// an earlier intent with a later field before hashing.
func readCanonicalValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, fmt.Errorf("JSON nesting is too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("invalid object key")
			}
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("duplicate field %q", key)
			}
			value, err := readCanonicalValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := readCanonicalValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter")
	}
}
