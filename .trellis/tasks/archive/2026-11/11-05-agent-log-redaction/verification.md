# Redaction verification — 2026-11-05

Implementation base: `f1d35b66af1013b6147c0e167500cacae203b7a8` in `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`.

## Changed behavior

- `InputMap` masks recognized credential fields at all supported map/array levels, including non-string sensitive values. Empty strings/null remain absent values; input data is copied. Matching covers delimiter/camelCase names, documented credential suffixes and `secret_access_key`; metadata such as `token_count` and public fields remains unchanged.
- `Text` replaces sensitive JSON value spans rather than reserializing the log. Escaped keys, duplicate keys, nested/array values, plain JSON embedded in logs and exact numeric formatting are covered. A value decode has a 1 MiB input budget; structural depth is bounded at 32. Malformed, truncated or over-budget recognized sensitive values discard the remaining text instead of exposing it.
- Exact first-party token shapes for PAT/daemon/task, plugin tokens, collaboration capabilities and installation metadata proofs are masked with boundary checks that preserve adjacent tokens and base64url trailing hyphens.
- The ordinary daemon tool-result path redacts the complete output before calculating its UTF-8-safe 8192-byte preview.

## Owned changed files

- `server/pkg/redact/redact.go`
- `server/pkg/redact/redact_test.go`
- `server/internal/daemon/daemon.go`
- `server/internal/daemon/tool_output_preview_test.go`
- `server/pkg/agent/codex_test.go`: main explicitly approved widening scope to one existing test expectation, from vendor-specific `[REDACTED API KEY]` to semantic-field `[REDACTED CREDENTIAL]`. All existing assertions otherwise remain intact; provider production code is unchanged.

## Environment

All tests use `GOMAXPROCS=2`, `go -p 1`, and an exclusive temporary root outside any Git checkout, verified with a failing `git rev-parse --show-toplevel`. Its path is recorded in `research/state-path.txt`. The launcher removes inherited `DATABASE_URL` and `PG*`. No HOME/CODEX_HOME override was used. Daemon/provider tests run through `scripts/go-test-with-agent-cli-guard.sh` and select fake-backend/synthetic-event tests only. No database/service/provider was started, no real credential read or model quota used.

## Red before implementation

- `go -C server test -p 1 ./pkg/redact -count=1`: exit 1 with new structured-field, JSON and native-token regressions. Preserved in `research/red-pkg-redact.log`.
- Guarded `go -C server test -p 1 ./internal/daemon -run '^TestExecuteAndDrain_ToolOutputRedactedBeforePreview$' -count=1 -timeout=2m`: exit 1; all four new subcases failed (plain JSON, JSON crossing preview boundary, native task token crossing boundary, PEM crossing boundary). Preserved in `research/red-daemon-output.log`.

## Green checks

| Command | Actual named test events | Result |
| --- | --- | --- |
| `go -C server test -json -p 1 ./pkg/redact -count=1` | 74 run / 74 pass / 0 fail / 0 skip | PASS |
| `go -C server test -race -json -p 1 ./pkg/redact -count=1` | 74 run / 74 pass / 0 fail / 0 skip | PASS |
| Guarded daemon subset `^(TestToolOutputPreview.*|TestExecuteAndDrain_ToolOutput.*|TestExecuteAndDrain_RedactsNestedToolInputBeforeSending)$` | 37 run / 37 pass / 0 fail / 0 skip | PASS |
| Guarded provider subset `^(TestCodexPatchApply.*|TestCodexRawItemMCPToolCall.*)$` | 16 run / 16 pass / 0 fail / 0 skip | PASS |
| `go -C server vet -p 1 ./pkg/redact` | Static check | PASS |
| `go -C server vet -p 1 ./internal/daemon ./pkg/agent` | Static check | PASS |
| `gofmt -l` on all five changed Go files | Empty output | PASS |
| `git diff --check` on owned files | Empty output | PASS |

Counts include named subtests. Full JSON/log evidence is under `research/green-redact.jsonl`, `green-redact-race.log`, `green-daemon.jsonl` and `green-agent-final.jsonl`; command summaries are in `green-summary.json` and `checks-summary.json`.

The initial provider check in `research/green-agent.jsonl` failed 1 of 16 named tests solely because the existing MCP assertion pinned the previous vendor placeholder. The approved one-line assertion update yields the 16/16 final result above; the failed run is retained, not overwritten.

## Bounded performance observation

`go -C server test -p 1 ./pkg/redact -run '^$' -bench '^BenchmarkText$' -benchtime=100ms -count=1` passed. On this Apple M4/Go environment, observations were approximately 3.34 ms for ordinary 8 KiB, 28.94 ms for ordinary 300 KiB, and 1.64 ms for 100 small credential-bearing JSON records. These are single diagnostic measurements of the entire redactor, with no before baseline and no performance-improvement claim. Evidence: `research/bench-redact.log`.

## Limits

No cross-event streaming or arbitrary encoded/encrypted payload recognition, dynamic secret registry, historic transcript cleanup, or externally issued `mcn_` format guess was added. On malformed sensitive JSON, discarding the remainder intentionally reduces diagnostic detail. Ordinary task execution still depends on its external host/credential boundaries; this patch does not create a sandbox. No complete repository or deployed-provider acceptance was run. Product/API/DB schemas and dependencies are unchanged.

## Independent-review correction

The reviewer reproduced a regression in legacy mcb_/mct_ word boundaries next to hyphens, restored the original capability regex, and added the missing regression. Final independently executed pkg/redact normal and race inventories are **77 named pass / 0 fail / 0 skip each**, with vet and format checks passing. The earlier74-case runs above remain their original evidence. See independent-review.md and research/independent-reviewed-sources.json for final source identity and the retained synthetic failure. Daemon37/provider16 retained runs were unchanged by this boundary-only correction.
