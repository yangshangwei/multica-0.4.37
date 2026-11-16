# P3 prerequisite: typed model results and configuration identity

2026-11-14; native `trellis-implement` child `p0_core_contracts`. This slice adds
no automation consumer or background writer.

## Interface

```go
func (*Client) GenerateJSONResult(ctx context.Context,
    model, systemPrompt, userPrompt string,
    temperature float64, maxCompletionTokens int64) (JSONResult, error)

type JSONResult struct {
    Text string
    Model string
    Usage JSONUsage
}
type JSONUsage struct {
    Status UsageStatus // "known", "partial", "unknown"
    InputTokens, OutputTokens, TotalTokens *int64
    CachedInputTokens, ReasoningTokens *int64
}

func (*Client) ConfigurationFingerprint() string
```

`Model` is the actual returned model, trimmed; absent/invalid remains empty.
Requested configuration is available separately through `DefaultModel`. `nil`
token fields mean missing/null/invalid; a pointer to zero means actual reported
zero. Known requires all three primary counts; optional breakdowns may remain
absent. Partial preserves any available count without inferring missing totals.
Counts describe the returned completion, not a reconstructed bill for SDK retry
attempts whose usage may be unknown.

Both JSON methods use the same private request/parameter-negotiation path. Legacy
`GenerateJSON` retains its string/error behavior. No choices, token-length
termination and empty text return errors with empty `Text`, retaining actual
model/usage. Unconfigured calls and transport timeouts return unknown usage.

The fingerprint captures effective endpoint, key identity, default model, retry
count/timeout, enabled state and inherited SDK organization/project/custom headers
at construction. It normalizes equivalent URL/default/header forms and returns
only SHA256; no raw credential, header or URL accessor was added. Later environment
changes do not change an existing client's identity. A custom `HTTPClient` and
network routing behavior are outside this configuration hash; unstable pointer
addresses are deliberately excluded. Ambient `OPENAI_*` values still cannot
enable an otherwise unconfigured assist layer.

## Files and validation

- Changed `server/pkg/llm/client.go` and `outbound_contract_test.go`.
- Added `json_result.go`, `json_result_test.go`, `configuration.go` and
  `configuration_test.go` in that package.
- Existing consumer inventory remains unchanged; `GenerateJSONResult` joins the
  scanned method surface and disabled-client zero-request matrix. Register the
  real automation consumer only when it is added.
- Original package baseline passed. `p3-llm-red.log` retains the typed-result
  failures before provider facts were preserved.
- Final `go -C server test ./pkg/llm -count=1 -json`: **34 top-level tests and
  22 subtests passed, zero failures/skips**. Raw `p3-llm-tests.jsonl`.
- `go -C server vet ./pkg/llm` and `git diff --check -- server/pkg/llm`: passed.
  Vet log: `p3-llm-vet.log`.

All upstream requests used explicit local fake HTTP servers or refusing test
transports. No database, real model/account call, commit or deployment occurred.
