// Package redact provides functions for detecting and masking secrets
// in agent output before it reaches the database or WebSocket broadcast.
package redact

import (
	"encoding/json"
	"regexp"
	"strings"
)

// secretPattern pairs a compiled regex with its replacement text.
type secretPattern struct {
	re          *regexp.Regexp
	replacement string
}

// Patterns are checked in order; first match wins per position.
var patterns = []secretPattern{
	// AWS access key IDs (always start with AKIA)
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[REDACTED AWS KEY]"},

	// AWS secret access keys (40 char base64-ish, preceded by a common separator)
	{regexp.MustCompile(`(?i)(?:aws_secret_access_key|secret_?access_?key)\s*[=:]\s*[A-Za-z0-9/+=]{40}`), "[REDACTED AWS SECRET]"},

	// PEM private keys (multi-line)
	{regexp.MustCompile(`(?s)-----BEGIN[A-Z\s]*PRIVATE KEY-----.*?-----END[A-Z\s]*PRIVATE KEY-----`), "[REDACTED PRIVATE KEY]"},

	// GitHub tokens (classic PAT, OAuth, user-to-server, server-to-server, refresh)
	{regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}\b`), "[REDACTED GITHUB TOKEN]"},

	// GitHub fine-grained personal access tokens use the github_pat_ prefix,
	// which the classic ghp_/gho_/... pattern above does not cover. Without
	// this line a fine-grained PAT emitted in agent output leaks unredacted
	// to the database and WebSocket broadcast.
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,255}\b`), "[REDACTED GITHUB TOKEN]"},

	// OpenAI / Anthropic API keys
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`), "[REDACTED API KEY]"},

	// Slack bot/user/legacy tokens. The char class includes 'e' so the
	// newer xoxe- config/refresh tokens are covered alongside xoxb/p/o/r/a/s.
	{regexp.MustCompile(`\bxox[bporase]-[A-Za-z0-9\-]{10,}\b`), "[REDACTED SLACK TOKEN]"},

	// Slack app-level tokens use the xapp- prefix, which the xox*- rule above
	// does not match. Without this an app-level token echoed in agent output
	// leaks unredacted to the DB / WebSocket broadcast.
	{regexp.MustCompile(`\bxapp-[A-Za-z0-9-]{10,}\b`), "[REDACTED SLACK TOKEN]"},

	// GitLab personal access tokens
	{regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`), "[REDACTED GITLAB TOKEN]"},

	// Google API keys always start with the AIza prefix and are 39 chars total
	// (AIza + 35). Capture and restore the trailing delimiter so keys ending in
	// a non-word character such as '-' are still redacted.
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}([^0-9A-Za-z_-]|$)`), "[REDACTED GOOGLE API KEY]$1"},

	// Stripe secret / restricted live keys (sk_live_ / rk_live_). The sk-
	// rule above only matches the hyphen form used by OpenAI/Anthropic; Stripe
	// uses an underscore, so live keys are not covered without this. Publishable
	// keys (pk_live_) are intentionally excluded — they are not secret.
	{regexp.MustCompile(`\b(?:sk|rk)_live_[0-9A-Za-z]{16,}\b`), "[REDACTED STRIPE KEY]"},

	// JWT tokens (three base64url segments)
	{regexp.MustCompile(`\bey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), "[REDACTED JWT]"},

	// Collaboration capabilities use word boundaries so a capability adjacent
	// to a hyphen in a log label is still redacted. Keep this established rule
	// separate from the first-party token scanner, whose stricter boundary is
	// needed for ordinary identifiers and base64url tokens.
	{regexp.MustCompile(`\b(?:mcb|mct)_[a-f0-9]{64}\b`), "[REDACTED MULTICA CAPABILITY]"},

	// Generic "Bearer <token>" in output
	{regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-._~+/]+=*\b`), "Bearer [REDACTED]"},

	// Connection strings with embedded passwords
	{regexp.MustCompile(`(?i)(?:postgres|mysql|mongodb|redis|amqp)(?:ql)?://[^:\s]+:[^@\s]+@`), "[REDACTED CONNECTION STRING]@"},

	// Generic key=value patterns for common secret env var names
	{regexp.MustCompile(`(?i)(?:API_KEY|API_SECRET|SECRET_KEY|SECRET|ACCESS_TOKEN|AUTH_TOKEN|PRIVATE_KEY|DATABASE_URL|DB_PASSWORD|DB_URL|REDIS_URL|PASSWORD|TOKEN)\s*[=:]\s*\S+`), "[REDACTED CREDENTIAL]"},
}

// Match the token and installation-proof formats minted by auth and
// plugin_token. Check boundaries separately so a separator is not consumed
// when two credentials are adjacent, including base64url tokens ending in '-'.
var multicaToken = regexp.MustCompile(`(?:mul|mdt|mat)_[a-f0-9]{40}|(?:mpi|mpc)_[A-Za-z0-9_-]{43}|mip_ey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)

const credentialPlaceholder = "[REDACTED CREDENTIAL]"

// sensitiveKey recognizes complete credential names and delimited credential
// suffixes. In particular, token_count, token_usage and token_id are metadata,
// and public_key/public_token are deliberately not credentials.
func sensitiveKey(key string) bool {
	var normalized strings.Builder
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c == '-' {
			c = '_'
		}
		if c >= 'A' && c <= 'Z' {
			if i > 0 && ((key[i-1] >= 'a' && key[i-1] <= 'z') ||
				(key[i-1] >= '0' && key[i-1] <= '9') ||
				(key[i-1] >= 'A' && key[i-1] <= 'Z' && i+1 < len(key) && key[i+1] >= 'a' && key[i+1] <= 'z')) {
				normalized.WriteByte('_')
			}
			c += 'a' - 'A'
		}
		normalized.WriteByte(c)
	}
	key = normalized.String()
	for _, public := range []string{"public_key", "public_token"} {
		if key == public || strings.HasSuffix(key, "_"+public) {
			return false
		}
	}
	for _, name := range []string{
		"password", "passwd", "passphrase", "api_key", "api_secret", "secret", "secret_key",
		"token", "private_key", "secret_access_key", "authorization", "database_url", "db_url", "redis_url",
	} {
		if key == name || strings.HasSuffix(key, "_"+name) {
			return true
		}
	}
	return false
}

// maxRedactDepth bounds the walk in redactValue. Tool inputs are decoded from
// daemon-supplied JSON, so nesting depth is attacker-influenced; without a
// bound a pathologically nested payload would recurse until the stack blows and
// take the process down. Real tool inputs nest a handful of levels at most, so
// this only ever trips on abuse.
const maxRedactDepth = 32

// depthLimitPlaceholder replaces anything below maxRedactDepth. Returning the
// raw value there would hand back an unscrubbed string, which is exactly what
// this package exists to prevent, so the fail-safe direction is to drop it.
const depthLimitPlaceholder = "[REDACTED DEPTH LIMIT]"

// InputMap returns a copy of m with credential fields masked and every other
// string value passed through Text, including nested maps and slices.
//
// The nested walk is load-bearing, not defensive tidying: providers record
// structured tool inputs, and Codex records a file edit as
// changes[]{path, diff, content}. A top-level-only pass leaves a credential
// inside a patch body — or the full contents of a deleted .env — untouched on
// its way to the database and the WebSocket broadcast.
func InputMap(m map[string]any) map[string]any {
	return redactMap(m, 0)
}

func redactMap(m map[string]any, depth int) map[string]any {
	if m == nil {
		return nil
	}
	if depth >= maxRedactDepth {
		return map[string]any{"_": depthLimitPlaceholder}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sensitiveKey(k) && v != nil && v != "" {
			out[k] = credentialPlaceholder
		} else {
			out[k] = redactValue(v, depth+1)
		}
	}
	return out
}

// redactValue scrubs a single decoded JSON value, recursing through the
// composite shapes json.Unmarshal produces plus []string, which providers use
// for argv-style inputs.
//
// Composites are copied rather than scrubbed in place: the caller still holds
// the original map and keeps using it after redaction (the daemon handler logs
// and re-reads it), so mutating through the shared reference would be a
// surprise at a distance.
func redactValue(v any, depth int) any {
	if depth >= maxRedactDepth {
		return depthLimitPlaceholder
	}
	switch t := v.(type) {
	case string:
		return Text(t)
	case map[string]any:
		return redactMap(t, depth)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = redactValue(e, depth+1)
		}
		return out
	case []string:
		out := make([]string, len(t))
		for i, e := range t {
			out[i] = Text(e)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(t))
		for k, e := range t {
			if sensitiveKey(k) && e != "" {
				out[k] = credentialPlaceholder
			} else {
				out[k] = Text(e)
			}
		}
		return out
	default:
		return v
	}
}

// Text scans the input string for known secret patterns and replaces
// matches with safe placeholders.
//
// It deliberately does NOT mask the local home directory. That masking used to
// live here, but it was never a boundary: anyone who can read a transcript
// already sees repository paths, file contents, commands and diffs, so hiding
// one path segment protected nothing while making paths unusable for copy-paste
// and debugging. It was also incoherent about whose home directory it hid —
// Content/Output are redacted in the server's ingest handler, so a hosted
// deployment matched them against the *server's* home, never the machine
// running the agent. Transcript visibility is an authorization concern and is
// handled at that layer, not by string replacement here.
func Text(s string) string {
	s = redactJSONMembers(s)
	s = redactMulticaTokens(s)
	for _, p := range patterns {
		s = p.re.ReplaceAllString(s, p.replacement)
	}
	return s
}

func redactMulticaTokens(s string) string {
	var out strings.Builder
	copied := 0
	for _, match := range multicaToken.FindAllStringIndex(s, -1) {
		start, end := match[0], match[1]
		if (start > 0 && tokenByte(s[start-1])) || (end < len(s) && tokenByte(s[end])) {
			continue
		}
		out.WriteString(s[copied:start])
		switch s[start : start+4] {
		case "mip_":
			out.WriteString("[REDACTED MULTICA PROOF]")
		default:
			out.WriteString("[REDACTED MULTICA TOKEN]")
		}
		copied = end
	}
	if copied == 0 {
		return s
	}
	out.WriteString(s[copied:])
	return out.String()
}

func tokenByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// A sensitive JSON value may itself be a large object. Bound the decoder's
// allocation and the structural scan; invalid or over-budget values lose the
// remaining text rather than falling back to disclosing the secret.
const maxJSONValueBytes = 1 << 20

// redactJSONMembers replaces only value spans, preserving formatting, duplicate
// keys and exact numeric literals elsewhere. It also handles JSON embedded in
// log text without requiring the entire log line to be a JSON document.
func redactJSONMembers(s string) string {
	var out strings.Builder
	copied := 0
	for scan := 0; scan < len(s); {
		q := strings.IndexByte(s[scan:], '"')
		if q < 0 {
			break
		}
		start := scan + q
		scan = start + 1
		backslashes := 0
		for i := start - 1; i >= 0 && s[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 != 0 {
			continue
		}
		end := jsonStringEnd(s, start)
		if end < 0 {
			break
		}
		colon := skipJSONSpace(s, end)
		if colon == len(s) || s[colon] != ':' {
			continue
		}
		var key string
		if json.Unmarshal([]byte(s[start:end]), &key) != nil || !sensitiveKey(key) {
			continue
		}
		valueStart := skipJSONSpace(s, colon+1)
		decoder := json.NewDecoder(strings.NewReader(s[valueStart:min(len(s), valueStart+maxJSONValueBytes)]))
		var value json.RawMessage
		err := decoder.Decode(&value)
		valueEnd := valueStart + int(decoder.InputOffset())
		next := skipJSONSpace(s, valueEnd)
		if err != nil || !boundedJSONDepth(value) ||
			(next < len(s) && s[next] != ',' && s[next] != '}' && s[next] != ']') {
			out.WriteString(s[copied:valueStart])
			out.WriteString(`"` + credentialPlaceholder + `"`)
			return out.String()
		}
		scan = valueEnd
		if string(value) == `null` || string(value) == `""` {
			continue
		}
		out.WriteString(s[copied:valueStart])
		out.WriteString(`"` + credentialPlaceholder + `"`)
		copied = valueEnd
	}
	if copied == 0 {
		return s
	}
	out.WriteString(s[copied:])
	return out.String()
}

func jsonStringEnd(s string, start int) int {
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

func skipJSONSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == '\t') {
		i++
	}
	return i
}

func boundedJSONDepth(raw []byte) bool {
	s := string(raw)
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			i = jsonStringEnd(s, i) - 1 // The decoder already validated the string.
		case '{', '[':
			depth++
			if depth > maxRedactDepth {
				return false
			}
		case '}', ']':
			depth--
		}
	}
	return true
}
