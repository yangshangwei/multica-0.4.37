# Agent output redaction

## 1. Scope / trigger

Apply to provider tool inputs/results, daemon transcript previews and server
ingest. Redact before a payload crosses a process boundary or is truncated;
the server repeats redaction before storage and WebSocket publication.

## 2. Signatures

- `redact.InputMap(map[string]any) map[string]any`
- `redact.Text(string) string`
- Daemon tool-result path: `toolOutputPreview(redact.Text(msg.Output))`
- Server ingest: `Content`/`Output` use `Text`; structured `Input` uses `InputMap`.

## 3. Contracts

Treat recognized credential field names as sensitive even when their values do
not match a vendor token pattern. Nested maps, string maps and arrays retain the
same rule. Match explicit names and credential suffixes, never any occurrence
of `token` or `key`: `token_count`, `token_usage`, `public_key` and ordinary
identifiers must remain useful. Sensitive non-string values cannot bypass
masking. Preserve the original input map and bound recursive work.

JSON text redaction replaces sensitive value spans without reserializing the
whole document: normal whitespace, key order, duplicate members and numeric
literals remain intact. Recognized sensitive values that are malformed,
truncated or over a parsing bound must not fall back to exposing their raw
bytes. Preserve ordinary text that contains no secret and make repeated
redaction stable.

Native credential patterns must follow their actual issuers. Human PAT,
daemon and task tokens use `mul_`/`mdt_`/`mat_` plus 40 lowercase hex digits;
plugin tokens use `mpi_`/`mpc_` plus 43 base64url characters. Installation
metadata proofs prefix a JWT with `mip_`. Keep existing collaboration capability
word-boundary coverage, including hyphen-delimited labels; do not reuse the
stricter base64url identifier boundary for those existing capabilities. Do not guess a format for a prefix whose issuer is external.

## 4. Validation / error matrix

| Input | Result |
| --- | --- |
| Nested ordinary password under a recognized key | Masked without mutating input |
| Non-sensitive usage/count/public fields | Preserved |
| Complete JSON credential with escaping/duplicate keys | Every sensitive value masked; other bytes retained |
| Recognized malformed/over-budget sensitive JSON value | Conservative masking, no raw-secret fallback |
| Exact native token / adjacent tokens | Mask each token with correct token boundaries |
| Short/long lookalike ordinary identifier | Do not treat it as the exact native format |
| Secret crossing the preview cut point | Redact full result before calculating preview |

## 5. Good / base / bad cases

Good: the fake provider emits a credential-bearing tool result and the recorder
receives a bounded, UTF-8-valid scrubbed preview. Base: ordinary build output is
unchanged. Bad: truncate a JSON/private-key payload first, destroying the shape
that the redactor needs, or rely only on a remote server to remove credentials.

## 6. Required tests

Use synthetic values only. Cover structured fields, nested arrays/maps, escaped
JSON keys/values, non-string sensitive values, duplicates, numeric preservation,
malformed/large/deep input, exact token boundaries, input immutability and
idempotence. Include the actual fake `executeAndDrain`→recorder path rather than
only composing two helpers in a unit test. Keep existing provider-side patch and
MCP output regressions. Test process state belongs outside Git and must not
invoke ambient agent CLIs or accounts.

## 7. Wrong versus correct

Wrong: a passing vendor-prefix regex test proves that all logs are secret-free.
Correct: verify the structured and textual boundary shapes and their transport
ordering, and state coverage limits. Cross-event streaming, encrypted/encoded
payload recognition, task-scoped dynamic secret registries and historical log
cleanup require separate designs; this contract does not promise those features.
