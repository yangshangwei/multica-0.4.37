package redact

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRedactAWSAccessKey(t *testing.T) {
	t.Parallel()
	input := "Found key AKIAIOSFODNN7EXAMPLE in config"
	got := Text(input)
	if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("AWS key not redacted: %s", got)
	}
	if !strings.Contains(got, "[REDACTED AWS KEY]") {
		t.Fatalf("expected [REDACTED AWS KEY] placeholder, got: %s", got)
	}
}

func TestRedactAWSSecretKey(t *testing.T) {
	t.Parallel()
	input := "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	got := Text(input)
	if strings.Contains(got, "wJalrXUtnFEMI") {
		t.Fatalf("AWS secret not redacted: %s", got)
	}
}

func TestRedactPrivateKey(t *testing.T) {
	t.Parallel()
	input := "Here is the key:\n-----BEGIN RSA PRIVATE KEY-----\nMIIEow...\n-----END RSA PRIVATE KEY-----\nDone."
	got := Text(input)
	if strings.Contains(got, "MIIEow") {
		t.Fatalf("private key content not redacted: %s", got)
	}
	if !strings.Contains(got, "[REDACTED PRIVATE KEY]") {
		t.Fatalf("expected [REDACTED PRIVATE KEY] placeholder, got: %s", got)
	}
}

func TestRedactGitHubToken(t *testing.T) {
	t.Parallel()
	input := "export GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn"
	got := Text(input)
	if strings.Contains(got, "ghp_") {
		t.Fatalf("GitHub token not redacted: %s", got)
	}
}

// asm joins fragments into a credential-shaped test fixture. The token is kept
// split across fragments so the full value never appears as a contiguous
// literal in source — otherwise secret scanners (including GitHub push
// protection) flag these redaction-test fixtures as real credentials. The
// runtime string is identical, so the patterns are exercised exactly the same.
func asm(parts ...string) string { return strings.Join(parts, "") }

func TestRedactCollaborationCapabilities(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"mcb_", "mct_"} {
		capability := asm(prefix, strings.Repeat("a0", 32))
		for _, text := range []string{capability, "copied value: " + capability, `{"credential":"` + capability + `"}`} {
			if got := Text(text); strings.Contains(got, capability) || !strings.Contains(got, "[REDACTED MULTICA CAPABILITY]") {
				t.Fatal("plain collaboration capability was not redacted")
			}
		}
		for _, identifier := range []string{prefix + "resource", prefix + "1234", prefix + strings.Repeat("a", 63), prefix + strings.Repeat("a", 65)} {
			if Text(identifier) != identifier {
				t.Fatal("a non-capability identifier was changed")
			}
		}
	}
}

func TestRedactCollaborationCapabilitiesPreservesWordBoundaries(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"mcb_", "mct_"} {
		t.Run(prefix, func(t *testing.T) {
			capability := asm(prefix, strings.Repeat("a0", 32))
			for _, input := range []string{
				"value-" + capability,
				capability + "-suffix",
				capability + "-" + capability,
			} {
				got := Text(input)
				if strings.Contains(got, capability) || !strings.Contains(got, "[REDACTED MULTICA CAPABILITY]") {
					t.Fatal("hyphen-delimited capability lost its existing redaction coverage")
				}
				if Text(got) != got {
					t.Fatal("capability word-boundary redaction is not idempotent")
				}
			}
			for _, input := range []string{"label_" + capability, capability + "_id"} {
				if Text(input) != input {
					t.Fatal("embedded ordinary identifier must keep the existing word-boundary behavior")
				}
			}
		})
	}
}

// TestRedactGitHubFineGrainedToken guards the github_pat_ prefix used by
// GitHub fine-grained personal access tokens. The classic-token pattern only
// covers ghp_/gho_/ghu_/ghs_/ghr_, so before the dedicated pattern was added a
// fine-grained PAT reached the DB / WS broadcast unredacted.
func TestRedactGitHubFineGrainedToken(t *testing.T) {
	t.Parallel()
	input := "cloning with token " + asm("github_", "pat_", "11ABCDE0Q0abcdefghijkl_MNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCD")
	got := Text(input)
	if strings.Contains(got, asm("github_", "pat_", "11ABCDE0Q0")) {
		t.Fatalf("fine-grained GitHub PAT not redacted: %s", got)
	}
	if !strings.Contains(got, "[REDACTED GITHUB TOKEN]") {
		t.Fatalf("expected [REDACTED GITHUB TOKEN] placeholder, got: %s", got)
	}
}

func TestRedactGoogleAPIKeyEndingWithDash(t *testing.T) {
	t.Parallel()
	input := `the config file still had "` + asm("AIza", "SyB1cD3fGhIjKlMnOpQrStUvWxYz012345-") + `" in it`
	got := Text(input)
	if strings.Contains(got, asm("AIza", "SyB1cD3f")) {
		t.Fatalf("Google API key ending with dash not redacted: %s", got)
	}
	if !strings.Contains(got, `"[REDACTED GOOGLE API KEY]" in it`) {
		t.Fatalf("expected delimiter to be preserved, got: %s", got)
	}
}

func TestRedactOpenAIKey(t *testing.T) {
	t.Parallel()
	input := "OPENAI_API_KEY=sk-proj-abc123def456ghi789jkl012mno345"
	got := Text(input)
	if strings.Contains(got, "sk-proj-abc123") {
		t.Fatalf("OpenAI key not redacted: %s", got)
	}
}

func TestRedactSlackToken(t *testing.T) {
	t.Parallel()
	input := "token: xoxb-123456789012-1234567890123-AbCdEfGhIjKl"
	got := Text(input)
	if strings.Contains(got, "xoxb-") {
		t.Fatalf("Slack token not redacted: %s", got)
	}
}

// TestRedactSlackAppAndConfigTokens guards the xapp- (app-level) and xoxe-
// (config/refresh) prefixes that the original xox[bporas]- rule did not cover.
func TestRedactSlackAppAndConfigTokens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in, leak string }{
		{"app-level xapp-", "connecting with " + asm("xa", "pp-1-A0000000000-1111111111-abcdefdeadbeefcafe") + " now", asm("xa", "pp-1-A0000000000")},
		{"config xoxe-", "refresh with " + asm("xo", "xe-1-My0abcdefghijklmnopqrstuvwx") + " now", asm("xo", "xe-1-My0abcdef")},
	} {
		got := Text(tc.in)
		if strings.Contains(got, tc.leak) {
			t.Fatalf("%s not redacted: %s", tc.name, got)
		}
		if !strings.Contains(got, "[REDACTED SLACK TOKEN]") {
			t.Fatalf("%s: expected [REDACTED SLACK TOKEN], got: %s", tc.name, got)
		}
	}
}

// TestRedactGoogleAPIKey guards the AIza-prefixed Google API key format, which
// no prior pattern covered.
func TestRedactGoogleAPIKey(t *testing.T) {
	t.Parallel()
	input := "calling gemini with " + asm("AIza", "SyD1234567890abcdefghijklmnopqrstuv") + " now"
	got := Text(input)
	if strings.Contains(got, asm("AIza", "SyD1234567890")) {
		t.Fatalf("Google API key not redacted: %s", got)
	}
	if !strings.Contains(got, "[REDACTED GOOGLE API KEY]") {
		t.Fatalf("expected [REDACTED GOOGLE API KEY], got: %s", got)
	}
}

// TestRedactStripeLiveKey guards Stripe secret/restricted live keys, which use
// an underscore (sk_live_) and so are missed by the hyphen-form sk- rule.
// Publishable keys (pk_live_) are intentionally NOT redacted — they are public.
func TestRedactStripeLiveKey(t *testing.T) {
	t.Parallel()
	if got := Text("STRIPE_SECRET_KEY=" + asm("sk_", "live_", "51Abcdef0000000000000000")); strings.Contains(got, asm("sk_", "live_51Abcdef")) {
		t.Fatalf("Stripe live key not redacted: %s", got)
	}
	if got := Text("restricted " + asm("rk_", "live_", "51Abcdef0000000000000000")); !strings.Contains(got, "[REDACTED STRIPE KEY]") {
		t.Fatalf("Stripe restricted key not redacted: %s", got)
	}
	// Publishable keys are public and must survive untouched.
	if got := Text(asm("pk_", "live_", "51Abcdef0000000000000000")); !strings.Contains(got, asm("pk_", "live_51Abcdef")) {
		t.Fatalf("publishable key should NOT be redacted: %s", got)
	}
}

func TestRedactBearerToken(t *testing.T) {
	t.Parallel()
	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123"
	got := Text(input)
	if strings.Contains(got, "eyJhbGci") {
		t.Fatalf("Bearer token not redacted: %s", got)
	}
}

// TestRedactBearerMCPToken is a regression guard for the Composio MCP session
// headers (MUL-3720): the SDK attaches the project key as `Bearer mcp_...` on
// some MCP transports, so the generic Bearer pattern must mask it before it can
// reach a log line or WS broadcast.
func TestRedactBearerMCPToken(t *testing.T) {
	t.Parallel()
	input := "connecting with Authorization: Bearer mcp_AbCdEf0123456789-_token"
	got := Text(input)
	if strings.Contains(got, "mcp_AbCdEf0123456789") {
		t.Fatalf("Bearer mcp_ token not redacted: %s", got)
	}
	if !strings.Contains(got, "Bearer [REDACTED]") {
		t.Fatalf("expected Bearer [REDACTED] placeholder, got: %s", got)
	}
}

func TestRedactGenericCredentials(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
	}{
		{"API_KEY", "API_KEY=mysupersecretkey123"},
		{"DATABASE_URL", "DATABASE_URL=postgres://user:pass@host/db"},
		{"DB_PASSWORD", "DB_PASSWORD: hunter2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Text(tc.input)
			if !strings.Contains(got, "[REDACTED CREDENTIAL]") {
				t.Fatalf("expected credential redaction for %s, got: %s", tc.name, got)
			}
		})
	}
}

func TestNoFalsePositivesOnNormalText(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"This is a normal commit message about fixing a bug",
		"The function returns skip-navigation as the class name",
		"Created PR #42 for the authentication feature",
		"Running tests in /tmp/test-workspace/project",
		"The API endpoint /api/issues/123 was updated",
	}
	for _, input := range inputs {
		got := Text(input)
		if got != input {
			t.Fatalf("false positive redaction:\n  input:  %s\n  output: %s", input, got)
		}
	}
}

func TestRedactGitLabToken(t *testing.T) {
	t.Parallel()
	input := "GITLAB_TOKEN=glpat-AbCdEfGhIjKlMnOpQrStUvWx"
	got := Text(input)
	if strings.Contains(got, "glpat-") {
		t.Fatalf("GitLab token not redacted: %s", got)
	}
}

func TestRedactJWT(t *testing.T) {
	t.Parallel()
	input := "token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	got := Text(input)
	if strings.Contains(got, "eyJhbGci") {
		t.Fatalf("JWT not redacted: %s", got)
	}
}

func TestRedactConnectionString(t *testing.T) {
	t.Parallel()
	input := "connecting to postgres://admin:s3cret@db.example.com:5432/mydb"
	got := Text(input)
	if strings.Contains(got, "s3cret") {
		t.Fatalf("connection string password not redacted: %s", got)
	}
}

func TestRedactPasswordEnvVar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
	}{
		{"PASSWORD", "PASSWORD=hunter2"},
		{"SECRET", "SECRET=mysecretvalue"},
		{"TOKEN", "TOKEN=abc123xyz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Text(tc.input)
			if !strings.Contains(got, "[REDACTED CREDENTIAL]") {
				t.Fatalf("expected credential redaction for %s, got: %s", tc.name, got)
			}
		})
	}
}

func TestInputMap(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"command":   "echo sk-proj-abc123def456ghi789jkl012mno345",
		"file_path": "/tmp/test.txt",
		"count":     42,
	}
	got := InputMap(m)
	if s, ok := got["command"].(string); ok {
		if strings.Contains(s, "sk-proj") {
			t.Fatalf("API key in input map not redacted: %s", s)
		}
	}
	// Non-string values preserved
	if got["count"] != 42 {
		t.Fatalf("non-string value altered: %v", got["count"])
	}
	// Clean strings unchanged
	if got["file_path"] != "/tmp/test.txt" {
		t.Fatalf("clean string altered: %v", got["file_path"])
	}
}

func TestInputMapNil(t *testing.T) {
	t.Parallel()
	if got := InputMap(nil); got != nil {
		t.Fatalf("expected nil, got: %v", got)
	}
}

// A Codex file edit arrives as changes[]{path, diff, content}. Before the
// nested walk these strings bypassed redaction entirely and reached the DB.
func TestInputMapRedactsNestedSliceOfMaps(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"changes": []any{
			map[string]any{
				"path": "cfg.go",
				"diff": "+api_key: sk-proj-abc123def456ghi789jkl012mno345",
			},
			map[string]any{
				"path":    ".env",
				"content": "GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn",
			},
		},
	}
	got := InputMap(m)

	changes, ok := got["changes"].([]any)
	if !ok || len(changes) != 2 {
		t.Fatalf("changes not preserved as a 2-element slice: %#v", got["changes"])
	}
	first, _ := changes[0].(map[string]any)
	if diff, _ := first["diff"].(string); strings.Contains(diff, "sk-proj") {
		t.Fatalf("API key nested in changes[0].diff not redacted: %s", diff)
	}
	if first["path"] != "cfg.go" {
		t.Fatalf("clean nested string altered: %v", first["path"])
	}
	second, _ := changes[1].(map[string]any)
	if content, _ := second["content"].(string); strings.Contains(content, "ghp_ABCDEFGH") {
		t.Fatalf("token nested in changes[1].content not redacted: %s", content)
	}
}

func TestInputMapRedactsDeeplyNestedMaps(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": []any{"leak sk-proj-abc123def456ghi789jkl012mno345"},
			},
		},
	}
	got := InputMap(m)
	a, _ := got["a"].(map[string]any)
	b, _ := a["b"].(map[string]any)
	c, _ := b["c"].([]any)
	if len(c) != 1 {
		t.Fatalf("nested slice not preserved: %#v", b["c"])
	}
	if s, _ := c[0].(string); strings.Contains(s, "sk-proj") {
		t.Fatalf("API key at depth 4 not redacted: %s", s)
	}
}

// Home directory paths are intentionally left intact — see Text's doc comment.
// This guards the removal: a path under $HOME must survive verbatim, so nobody
// reintroduces host-path masking as an incidental side effect of another fix.
func TestTextLeavesHomePathIntact(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory resolved in this environment")
	}
	path := home + "/secret/app.go"

	if got := Text(path); got != path {
		t.Fatalf("home path was rewritten:\n  input:  %s\n  output: %s", path, got)
	}

	m := map[string]any{"changes": []any{map[string]any{"path": path}}}
	changes, _ := InputMap(m)["changes"].([]any)
	first, _ := changes[0].(map[string]any)
	if got, _ := first["path"].(string); got != path {
		t.Fatalf("nested home path was rewritten:\n  input:  %s\n  output: %s", path, got)
	}
}

func TestInputMapRedactsStringSliceAndStringMap(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"argv": []string{"curl", "-H", "Authorization: Bearer abc123def456"},
		"env":  map[string]string{"TOKEN": "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn"},
	}
	got := InputMap(m)

	argv, ok := got["argv"].([]string)
	if !ok || len(argv) != 3 {
		t.Fatalf("argv not preserved as []string: %#v", got["argv"])
	}
	if strings.Contains(argv[2], "abc123def456") {
		t.Fatalf("bearer token in []string not redacted: %s", argv[2])
	}
	env, ok := got["env"].(map[string]string)
	if !ok {
		t.Fatalf("env not preserved as map[string]string: %#v", got["env"])
	}
	if strings.Contains(env["TOKEN"], "ghp_ABCDEFGH") {
		t.Fatalf("token in map[string]string not redacted: %s", env["TOKEN"])
	}
}

// The caller keeps using the map it passed in, so redaction must not write
// through the shared nested references.
func TestInputMapDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	secret := "sk-proj-abc123def456ghi789jkl012mno345"
	inner := map[string]any{"diff": secret}
	m := map[string]any{"changes": []any{inner}}

	_ = InputMap(m)

	if inner["diff"] != secret {
		t.Fatalf("input map was mutated in place: %v", inner["diff"])
	}
}

// Depth is attacker-influenced, so the walk must bottom out instead of
// exhausting the stack — and must not hand back a raw string when it does.
func TestInputMapBoundsRecursionDepth(t *testing.T) {
	t.Parallel()
	leaf := map[string]any{"leak": "sk-proj-abc123def456ghi789jkl012mno345"}
	cur := leaf
	for i := 0; i < 5000; i++ {
		cur = map[string]any{"next": cur}
	}

	got := InputMap(cur) // must not stack overflow

	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal redacted map: %v", err)
	}
	if strings.Contains(string(blob), "sk-proj") {
		t.Fatalf("secret past the depth limit was returned unredacted")
	}
	if !strings.Contains(string(blob), depthLimitPlaceholder) {
		t.Fatalf("expected depth-limit placeholder in output: %s", truncateForLog(string(blob)))
	}
}

func truncateForLog(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "…"
}

func TestRedactMultipleSecrets(t *testing.T) {
	t.Parallel()
	input := "Keys: AKIAIOSFODNN7EXAMPLE and ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn"
	got := Text(input)
	if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatal("AWS key not redacted in multi-secret text")
	}
	if strings.Contains(got, "ghp_") {
		t.Fatal("GitHub token not redacted in multi-secret text")
	}
}

func TestInputMapSensitiveFields(t *testing.T) {
	t.Parallel()
	for _, key := range []string{
		"password", "Passwd", "passphrase", "api_key", "apiKey", "APIKey", "api-secret",
		"secret", "secretKey", "access_token", "authToken", "refresh-token", "TOKEN",
		"private_key", "clientSecret", "Authorization", "Proxy-Authorization",
		"GITHUB_TOKEN", "DB_PASSWORD", "DATABASE_URL", "service-api-key", "aws_secret_access_key", "secretAccessKey",
	} {
		t.Run(key, func(t *testing.T) {
			for _, value := range []any{"synthetic-private-value", 123456, true, []any{"synthetic-private-value"}, map[string]any{"value": "synthetic-private-value"}} {
				input := map[string]any{"items": []any{map[string]any{key: value}}}
				got := InputMap(input)
				field := got["items"].([]any)[0].(map[string]any)[key]
				if field != "[REDACTED CREDENTIAL]" {
					t.Fatalf("sensitive %s value of type %T was retained", key, value)
				}
				if !reflect.DeepEqual(input["items"].([]any)[0].(map[string]any)[key], value) {
					t.Fatal("redaction changed the original input")
				}
				if !reflect.DeepEqual(InputMap(got), got) {
					t.Fatal("field redaction is not idempotent")
				}
			}
			stringsInput := map[string]string{key: "synthetic-private-value"}
			got := InputMap(map[string]any{"env": stringsInput})
			if got["env"].(map[string]string)[key] != "[REDACTED CREDENTIAL]" || stringsInput[key] != "synthetic-private-value" {
				t.Fatal("string map must be copied and its sensitive value redacted")
			}
		})
	}
}

func TestInputMapPreservesNonSecretFields(t *testing.T) {
	t.Parallel()
	input := map[string]any{
		"token_count": 23, "token_usage": map[string]any{"input_tokens": 1, "output_tokens": 2},
		"max_tokens": 42, "public_key": "public-value", "publicToken": "public-value",
		"token_type": "synthetic-type", "token_id": "synthetic-id", "key": "ordinary-key",
		"name": "example", "path": "/a/secret/file", "password": "", "api_key": nil,
	}
	if got := InputMap(input); !reflect.DeepEqual(got, input) {
		t.Fatalf("non-secret or absent fields changed: %#v", got)
	}
}

func TestTextRedactsJSONMembersWithoutReformatting(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, input, want string }{
		{"format", "result: {\n  \"password\" : \"private phrase\", \"count\":9007199254740993\n}\n", "result: {\n  \"password\" : \"[REDACTED CREDENTIAL]\", \"count\":9007199254740993\n}\n"},
		{"escapes", `{"apiKey":"private\"phrase\\tail","ok":true}`, `{"apiKey":"[REDACTED CREDENTIAL]","ok":true}`},
		{"escaped key and duplicate", `{"\u0070assword":"first","password":"second","password":null}`, `{"\u0070assword":"[REDACTED CREDENTIAL]","password":"[REDACTED CREDENTIAL]","password":null}`},
		{"nested", `{"items":[{"clientSecret":"value","Authorization":"Basic opaque"}],"count":2}`, `{"items":[{"clientSecret":"[REDACTED CREDENTIAL]","Authorization":"[REDACTED CREDENTIAL]"}],"count":2}`},
		{"composite", `{"password":{"wrapped":["private"]},"api_key":["value"],"ok":true}`, `{"password":"[REDACTED CREDENTIAL]","api_key":"[REDACTED CREDENTIAL]","ok":true}`},
		{"scalar", `{"password":123456,"client_secret":false,"token":true}`, `{"password":"[REDACTED CREDENTIAL]","client_secret":"[REDACTED CREDENTIAL]","token":"[REDACTED CREDENTIAL]"}`},
		{"empty", `{"password":"","api_key":null,"public_token":"public","token_count":12}`, `{"password":"","api_key":null,"public_token":"public","token_count":12}`},
		{"normal", "{\"message\":\"say \\\"password\\\" in the docs\",\"id\":9007199254740993}", "{\"message\":\"say \\\"password\\\" in the docs\",\"id\":9007199254740993}"},
		{"adjacent records", `{"password":"one"} {"TOKEN":"two"}`, `{"password":"[REDACTED CREDENTIAL]"} {"TOKEN":"[REDACTED CREDENTIAL]"}`},
		{"aws secret", `{"aws_secret_access_key":"ordinary-private-value","secretAccessKey":"other-value"}`, `{"aws_secret_access_key":"[REDACTED CREDENTIAL]","secretAccessKey":"[REDACTED CREDENTIAL]"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.input); got != tt.want {
				t.Fatalf("unexpected redaction:\n got: %s\nwant: %s", got, tt.want)
			}
			if got := Text(tt.want); got != tt.want {
				t.Fatalf("redaction is not idempotent: %s", got)
			}
		})
	}
}

func TestTextLargeJSONKeysAndUnmatchedQuotes(t *testing.T) {
	t.Parallel()
	largeKey := strings.Repeat("ordinary", 150000)
	for _, input := range []string{
		`{"` + largeKey + `":"normal value","count":1}`,
		`{"message":"` + strings.Repeat(`escaped\"quote `, 100000),
		strings.Repeat(`"`, 100000),
	} {
		if Text(input) != input {
			t.Fatal("large non-sensitive text changed")
		}
	}
	input := `{"` + largeKey + `_password":"synthetic-secret"}`
	if got := Text(input); strings.Contains(got, "synthetic-secret") {
		t.Fatal("a large sensitive key bypassed redaction")
	}
}

func BenchmarkText(b *testing.B) {
	for _, tt := range []struct{ name, input string }{
		{"8KiBText", strings.Repeat("ordinary log line\n", 512)[:8192]},
		{"300KiBText", strings.Repeat("中", 100*1024)},
		{"JSON", strings.Repeat(`{"password":"synthetic-private-value","token_count":12}`+"\n", 100)},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.SetBytes(int64(len(tt.input)))
			b.ReportAllocs()
			for b.Loop() {
				Text(tt.input)
			}
		})
	}
}

func TestTextRedactsInvalidOrOversizedSensitiveJSON(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`{"password":"synthetic-secret`,
		`{"password":synthetic-secret}`,
		`{"password":123synthetic-secret}`,
		`{"password":"synthetic-secret` + strings.Repeat("x", 1024*1024) + `","ok":true}`,
		`{"password":` + strings.Repeat("[", 64) + `"synthetic-secret"` + strings.Repeat("]", 64) + `}`,
	} {
		got := Text(input)
		if strings.Contains(got, "synthetic-secret") || !strings.Contains(got, "[REDACTED CREDENTIAL]") {
			t.Fatalf("invalid or oversized sensitive value retained: %s", truncateForLog(got))
		}
		if Text(got) != got {
			t.Fatal("fail-closed redaction is not idempotent")
		}
	}
	for _, input := range []string{`plain log: {unfinished`, `{"count":oops`, `print("ordinary text")`, `{"token_count":12}`} {
		if Text(input) != input {
			t.Fatalf("unrelated text changed: %s", input)
		}
	}
}

func TestRedactMulticaTokens(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"mul_", "mdt_", "mat_", "mpi_", "mpc_"} {
		body := strings.Repeat("a0", 20)
		if prefix == "mpi_" || prefix == "mpc_" {
			body = strings.Repeat("a0_", 14) + "-"
		}
		token := asm(prefix, body)
		for _, input := range []string{token, "value " + token + " end", `{"value":"` + token + `"}`, token + "," + token} {
			got := Text(input)
			if strings.Contains(got, token) || !strings.Contains(got, "[REDACTED MULTICA TOKEN]") {
				t.Fatalf("%s credential was not redacted", prefix)
			}
			if Text(got) != got {
				t.Fatal("token redaction is not idempotent")
			}
		}
		for _, input := range []string{token[:len(token)-1], token + "a", "label_" + token, token + "_id", "label-" + token} {
			if Text(input) != input {
				t.Fatalf("credential-like identifier changed: %s", input)
			}
		}
	}
	proof := asm("mip_", "eyJ", strings.Repeat("a", 16), ".", strings.Repeat("b", 20), ".", strings.Repeat("c", 20))
	if got := Text(proof); got != "[REDACTED MULTICA PROOF]" || Text(got) != got {
		t.Fatalf("installation attribution proof not redacted: %s", got)
	}
}
