# Research: step 6 adapter usage semantics

- **Query**: (team lead, step 6 of `.trellis/tasks/11-16-f3-parent-budget-root`) For every adapter the daemon can launch: **A** where the usage figures come from; **B** whether a fresh-session result can include anything outside this invocation; **C** whether a resumed-session result is this invocation's increment or the session's cumulative total; **D** whether `CostUSDTicks` is ever set and whether it is provider-stated or an estimate; **E** loss or double counting inside one invocation; **F** how model keys are formed. Second deliverable: every place under `server/internal/daemon/` that starts a provider process or provider session run on behalf of a task, and whether its usage reaches the usage-entry assembly.
- **Scope**: mixed. Repository code, tests and fixtures for all adapters; the vendor's official documentation for claude; the vendor's own source repository for codex (flagged in 0.2, because it is not the vendor's docs domain).
- **Date**: 2026-11-17 (UTC). Code state and file hashes were last checked at 2026-11-17T23:13Z (section 7).
- **Code read**: worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`, branch `codex/f3-budget-root-20261116`, HEAD `86fea651cc5d09360327b6cf0f8357c1f360070d`. `git status --porcelain -- server/pkg/agent server/internal/daemon` was empty at that time, so every line number below is a line of that commit. Section 7 lists the SHA-256 of each file.
- **Method**: reading only. No agent CLI was run (not even `--version` or `--help`), no provider or model API was called, no test was run, no database, container or dev environment was touched, and nothing was written in the worktree. Vendor pages were fetched with plain GETs from a generic client (`curl/8.0`); the requests carried nothing about this project, and the responses were piped to stdout without being saved.

This report describes what exists. It makes no recommendations.

## 0. How to read this report

### 0.1 Abbreviations

- `agent/` = `server/pkg/agent/`; `daemon/` = `server/internal/daemon/`.
- "Invocation" = one call to `executeAndDrain` (`daemon/daemon.go:8486`), which calls `backend.Execute` exactly once (`daemon/daemon.go:8501`).
- "The assembly" = the loop that turns `agent.Result.Usage` into `[]TaskUsageEntry` (`daemon/daemon.go:8041-8056`).
- `design.md` = `.trellis/tasks/11-16-f3-parent-budget-root/design.md` in the main checkout. It is cited by section number only: the file was edited while this research ran (section 7 records both hashes), so its line numbers are not stable.

### 0.2 Standard of proof

Each piece of evidence carries a class tag.

| Class | What it is | Can it carry a `proven` verdict? |
|---|---|---|
| **V** | A sentence in the vendor's official documentation, quoted with URL and fetch date | Yes |
| **F** | A captured real-CLI artifact stored in the repository: a file under `agent/testdata/`, or a payload that a test states was captured from a live CLI | Yes, for exactly what the artifact shows |
| **K** | Adapter code that enforces the bound itself: a filter that discards out-of-scope records, or a baseline subtraction | Yes, for *which records are counted*; not for *what a provider's number means* |
| **S** | Structure only: the adapter starts its own process, reads only that process's stream, and asked for a new session | No |
| **R** | A repository comment that asserts observed real-CLI behaviour, with no artifact | No |
| **T** | A test that feeds an inline string, a constructed file or a fake shell script | No. It shows what the adapter does with an input, not what a provider emits |
| **VS** | The vendor's own source repository at a pinned commit (codex only; the vendor's docs domain refused the request) | Carries no verdict in this report. It is set out separately in 2.2 so that its admissibility can be ruled on |

A verdict is `proven …` only when V, F or K covers every path by which the adapter can produce its usage map. Where only S, R or T exists the verdict is `unknown`, and the cell names the classes that are present.

### 0.3 What "fresh" and "resumed" mean in the code

- The daemon signals resume only by setting `ExecOptions.ResumeSessionID`. Each adapter turns it into a CLI flag or a protocol request (cited per adapter in section 2). All 24 backend implementations have a resume path; none is resume-less.
- An empty `ResumeSessionID` does not by itself make the provider session new. Custom arguments pass through `filterCustomArgs` (`agent/claude.go:1067`) against a per-adapter blocked map, and several maps contain no session-selecting flag: claude (`agent/claude.go:777-790`), codebuddy (`agent/codebuddy.go:27-36`), cursor (`agent/cursor.go:968-972`), opencode (`agent/opencode.go:36-41`), codearts (`agent/codearts.go:38-45`), deveco (`agent/deveco.go:61-66`). pi/omp block `--session` but pass `--continue`, `-c`, `--resume`, `-r`, `--session-id`, `--fork`, `--no-session` and `--session-dir` through (`agent/pi.go:601-607`, `723-774`). dsh has no blocked map. Session-selecting flags are blocked by antigravity (`agent/antigravity.go:423-437`), copilot (`--resume`, `agent/copilot.go:618-629`), grok (`agent/grok.go:21-52`), openclaw (`--session-id`, `agent/openclaw.go:39-46`) and qwen (`agent/qwen.go:28-48`). codex and the ACP adapters select the session through a protocol request, not a flag.
- A non-empty `ResumeSessionID` does not by itself make the provider session a continued one either. `resumeRejectionUndetectable` (`agent/agent.go:384-400`) lists six adapters (antigravity, copilot, cursor, deveco, opencode, codearts) that "have no rejection detection: no phrase match, no structured error code, no internal restart". codex falls back from `thread/resume` to `thread/start` inside the same attempt and then treats the run as not resumed (`agent/codex.go:1988-2048`, `2089`, `2114`). ACP adapters continue with a different session id when the agent returns one (for example `agent/traecli.go:285-293`).

### 0.4 Which adapters the daemon can launch

- `SupportedTypes` (`agent/agent.go:342`) lists 25 provider keys; `New()` (`agent/agent.go:410`) is the constructor switch.
- `BuiltinRuntimes` (`agent/builtin_runtimes.go:87`) adds one key, `omp`, whose `ProtocolFamily` is `pi` (`:90`); `ResolveBackend` (`:154`) maps it to the pi backend (`:177`).
- The daemon resolves its backend with `agent.ResolveBackend(provider, backendConfig)` (`daemon/daemon.go:7770`).
- Result: 26 provider keys over 24 backend implementations (`pi`/`omp` share one; `qoder`/`qoderclicn` share one). The table below has one row per implementation.

## 1. Summary table

Verdict vocabulary: B and C use `proven per-invocation` / `proven NOT per-invocation` / `unknown`; D uses `provider-stated` / `estimate` / `never set` / `unknown`. The bracket after a verdict is the evidence class from 0.2. "cost-tracking.md" is `https://code.claude.com/docs/en/agent-sdk/cost-tracking.md` and "changelog" is `https://code.claude.com/docs/en/changelog.md`, both fetched 2026-11-17; `L` numbers are lines of the raw markdown as served.

| Adapter (provider key) | A. Source of the figures | B. Fresh session | C. Resumed session | D. Cost |
|---|---|---|---|---|
| `claude` | stdout stream-json. `result.modelUsage` per model, taken as-is and replacing what was summed before (`agent/claude.go:276-278`, `702-741`); else `result.usage` under one key; else Σ assistant `message.usage` (`447-454`) | **proven per-invocation** [V]: "Independent calls, with no `resume` or `continue` option: each result covers only its own call" (cost-tracking.md L222). Condition: the CLI session really is new (0.3) | **proven NOT per-invocation** on Claude Code ≥ v2.1.277 [V]: "`total_cost_usd` and `modelUsage` … carry the running total for the whole call so far, plus any spend restored when the call resumed a session" (L61). **proven per-invocation** below v2.1.277 [V]: "Before v2.1.277 … each call's results covered only that call" (L223). The adapter subtracts nothing and does not branch on CLI version. The changelog entry for 2.1.277 calls the restored figures "the session's cost and usage totals" (changelog L4475, quoted in 2.1). Residuals in 5.1: no sentence names the token counters inside `modelUsage` word for word; totals are saved only "when the process exits normally" (L223); the Result does not carry the CLI version | **never set**: no assignment in `agent/claude.go`; `claudeResultModelUsage` has no cost field (`agent/claude.go:663-668`) |
| `codebuddy` | Same shape as claude: result usage replaces assistant sums (`agent/codebuddy.go:260-262`, `569`); else Σ assistant usage (`:371`) | **unknown** [S]: no artifact, no usage test | **unknown**: nothing is subtracted; `--resume` at `agent/codebuddy.go:89` | **never set**: no reference in `agent/codebuddy.go` (list in 2.0) |
| `codex` | (1) JSON-RPC `turn/completed` `params.turn.usage` (`agent/codex.go:3437`) or legacy `task_complete` (`:3359`), summed (`3602-3627`); (2) only when (1) left uncached input and output at 0: the thread's rollout file, deltas of `token_count.info.total_token_usage` (`1901-1909`, `3859-3937`) | **unknown** overall. Rollout source is bound by adapter code to this thread's file [K] (`3668-3705`). Notification source: [S] only | **unknown** overall. Rollout source: the adapter subtracts the last pre-start cumulative total [K] (`agent/codex.go:3900-3911`), which relies on two provider properties stated only in a repo comment [R] (`3662-3667`, `3853-3858`). Notification source: no evidence | **never set**: no reference in `agent/codex.go` |
| `copilot` | Three stdout sources, used exclusively, never summed together (`agent/copilot.go:75-86`): `session.shutdown` per-model metrics (non-resumed runs only), else Σ `assistant.usage`, else Σ `assistant.message` output tokens | **unknown** [S+R+T]: semantics asserted by comment `agent/copilot.go:59-74` | **unknown** [K partial + R + T]: the session-total source is skipped on resume by code (`agent/copilot.go:76`); that the remaining source is per model call rests on the comment (`59-74`) | **never set** |
| `cursor` | Σ `result` usage (`agent/cursor.go:265`, `765-789`); if no result carried usage, Σ `step_finish` (`292-302`, `336`) | **unknown** [S + F partial]: the capture has usage on one line only (`agent/testdata/cursor-agent-2026.07.20-stream-json.jsonl` L23), with nothing to reconcile it against | **unknown** [R]: comment `agent/cursor.go:137-140` calls result usage "authoritative session totals"; no resumed capture | **never set**: comment `agent/cursor.go:818-824`; no reference |
| `qwen` | `result.usage` replaces (`agent/qwen.go:318-320`); else the last assistant usage per model, assigned not summed (`337-339`) | **unknown** [F partial], one fact short: in `agent/testdata/qwen-code-0.20.0-stream-json.jsonl` the result usage (L6) equals the sum of that run's two model calls (L3 + L5); the capture does not say whether the session was fresh or resumed | **unknown**: same capture, same missing fact | **never set** |
| `opencode` | Σ `step_finish` `part.tokens` (`agent/opencode.go:445-452`) | **unknown** [S+T] (`agent/opencode_test.go:1123`, inline) | **unknown** [S]: per-step events are summed and no total field is read; whether a resumed run re-emits earlier steps is not evidenced | **never set**: `part.cost` is read only as a liveness signal (`agent/opencode.go:521`, `703-706`) |
| `codearts` | Σ `step_finish` tokens (`agent/codearts.go:446-453`) | **unknown** [S]: no usage test | **unknown** [S] | **never set** |
| `deveco` | Σ `step_finish` tokens (`agent/deveco.go:345-352`) | **unknown** [S+T] (`agent/deveco_test.go:147`, inline) | **unknown** [S] | **never set** |
| `openclaw` | One result blob: `meta.agentMeta.usage` taken as-is (`agent/openclaw.go:663`, `684`); NDJSON fallback Σ `step_finish` (`508-515`), replaced by a final blob's usage (`531-534`) | **unknown** [F partial]: `agent/testdata/openclaw-2026.5.5-stdout.json` is a one-call run where `usage` equals `lastCallUsage` (L17, L23), so it cannot separate per-run from per-session | **unknown**: `usage` is taken as-is; no resumed capture | **never set** |
| `pi`, `omp` | Σ `turn_end` `message.usage` (`agent/pi.go:381-400`) | **unknown** [S]: no usage test, no capture | **unknown** [S] | **never set** |
| `dsh` | Σ `usage` frames (`agent/dsh.go:408-420`); frames with a different non-empty `request_id` are dropped (`383-385`) | **unknown** [S + K partial]: frames without a request id are accepted; no protocol document in the repo | **unknown** | **never set**: `dshFrame` has no cost field (`agent/dsh.go:74-101`) |
| `antigravity` | None. Always returns an empty map (`agent/antigravity.go:222-225`) | n/a (no usage returned) | n/a | **never set** |
| `hermes` | Shared ACP accumulator: per-bucket maximum over `usage_update` snapshots and prompt-result usage (`agent/acp_usage.go:55-88`; `agent/hermes.go:1480-1507`, `2059-2079`) | **unknown** [S + K partial]: a gate drops updates outside the prompt window (`agent/hermes.go:1563-1565`, `642`, `723`); what a snapshot counts is not evidenced | **unknown**: a snapshot that is cumulative for the session would be taken whole (maximum, no baseline) | **unknown**: passthrough of `costUsdTicks` if the agent sends it (`agent/acp_usage.go:205-208`); no captured payload |
| `grok` | Same accumulator; prompt-result `_meta.usage` (`agent/acp_usage.go:222-236`; `agent/hermes.go:1480-1507`) | **unknown** [F partial + S]: one captured prompt result, no stream, session state not stated (`agent/hermes_test.go:1681-1752`) | **unknown** [T]: the resume test uses a fake script (`agent/grok_test.go:633-675`) | **unknown**, flagged [F partial + R]: the captured payload carries `costUsdTicks 75360000` and comments call it xAI's own price (`agent/grok.go:459-462`); nothing shows whether the service or the CLI computed it |
| `kimi` | ACP accumulator when it holds tokens; else the session wire log, Σ `usage.record` lines (`agent/kimi.go:482-494`, `625-649`, `697-728`) | **unknown** [K + R]: the wire log is bound by session id and `time >= startTime` (`agent/kimi.go:738-746`); that a record is one model call's increment rests on comments (`613-624`) | **unknown** [K + R + T]: the same time filter excludes earlier runs' records; constructed-file test `agent/kimi_test.go:1152` | **unknown**: passthrough only (`agent/kimi.go:499-506`); comment says 0.33.0 sends none |
| `reasonix` | ACP accumulator; if it holds nothing, the latest `status_update` `status.usage.turn` snapshot (`agent/reasonix.go:483-487`, `738-790`) | **unknown** [S + K partial] | **unknown** | **estimate** on the status source: wire field `estimatedCost`, a float converted to ticks (`agent/reasonix.go:753`, `779`). **unknown** on the ACP passthrough source. The Result does not say which source filled it |
| `kiro` | ACP accumulator (`agent/kiro.go:441`, `470`) | **unknown** [S + K partial] | **unknown** | **unknown**: passthrough; the only cost in tests is a fake script (`agent/kiro_test.go:169`) |
| `qoder`, `qoderclicn` | ACP accumulator (`agent/qoder.go:386`, `431`) | **unknown** [S + K partial] | **unknown** | **unknown**: passthrough |
| `traecli` | ACP accumulator (`agent/traecli.go:389`, `430`) | **unknown** [S + K partial]; a test comment says the live capture's prompt result is `{stopReason:end_turn}`, with no usage (`agent/traecli_test.go:26-30`) | **unknown** | **unknown**: passthrough |
| `zeroclaw` | ACP accumulator (`agent/zeroclaw.go:541`, `580`) | **unknown** [S + K partial + R]: comment says 0.8.4 "reports no token counts over ACP" (`agent/zeroclaw.go:582-586`) | **unknown** | **unknown**: passthrough |
| `mcode` | ACP accumulator (`agent/mcode.go:329`, `357-358`) | **unknown** [S + K partial]: no usage test | **unknown** | **unknown**: passthrough |
| `dim` | ACP accumulator, no notification gate (`agent/dim.go:613`, `662`) | **unknown** [S]: no usage test | **unknown**: an update sent during `session/load` would be merged | **unknown**: passthrough |
| `qwenpaw` | ACP accumulator, no notification gate (`agent/qwenpaw.go:332`, `352`) | **unknown** [S+T] | **unknown**: an update sent during `session/load` would be merged | **unknown**: passthrough |

Counts over the 24 implementations (26 provider keys):

- **B** proven for 1 (`claude`); n/a for 1 (`antigravity`); unknown for 22.
- **C** proven for 1 (`claude`, in both directions depending on the CLI version); n/a for 1; unknown for 22.
- **D** `never set` proven for 13 implementations / 14 keys (claude, codebuddy, codex, copilot, cursor, qwen, opencode, codearts, deveco, openclaw, pi + omp, dsh, antigravity); `estimate` proven for one of reasonix's two sources; `provider-stated` proven for none; `unknown` for the other 10 ACP implementations / 11 keys and for reasonix's ACP source.

## 2. Per-adapter detail

### 2.0 Facts shared by all adapters

- **Types.** `agent.Backend.Execute(ctx, prompt, ExecOptions) (*Session, error)`; `Session.Result` delivers one `agent.Result` (`agent/agent.go:208`) whose `Usage` is `map[string]TokenUsage // keyed by model name` (`agent/agent.go:230`). `TokenUsage` (`agent/agent.go:185-200`) has `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CacheWriteTokens`, `CostUSDTicks`. `CostUSDTicksPerUSD = 10_000_000_000` (`agent/agent.go:205`).
- **No provenance in the Result.** `agent.Result` has no field naming the source of the usage (result event, per-step events, log file) and no field for the CLI version that produced it. `Config.CLIVersion` is documented as "observation only, never used to choose behavior" (`agent/agent.go:282`), and no adapter branches on it. `MinVersions` (`agent/version.go:13-22`) sets floors for eight adapters only: claude `2.0.0`, codex `0.100.0`, copilot `1.0.0`, grok `0.2.89`, qwen `0.20.0`, dim `0.3.10`, mcode `0.1.2`, zeroclaw `0.8.0`.
- **Cost references.** Every non-test line under `agent/` that contains `CostUSDTicks` (the search also matches the constant `CostUSDTicksPerUSD`): `agent.go:190,199,202,205` (type, constant and comments); `acp_usage.go:82-83` (accumulator maximum), `135-136` (`withFallback`), `:206` (parser, block `205-207`); `hermes.go:1515` (`acpUsagePresent`); `kimi.go:499,501`; `reasonix.go:778-779,808`. No other file in the package contains it. That list is the basis of every `never set` verdict. The type comment (`agent/agent.go:190-198`) reads: "CostUSDTicks is the provider's own statement of what this usage cost, in ticks of 1e-10 USD. Zero means "not reported" — only a few agents return it (xAI Grok Build does, via `_meta.usage.costUsdTicks`)."
- **Shared helpers without usage handling.** `agent/run_collect.go`, `agent/run_collect_quiet.go` and `agent/stream_json_result.go` (`finalizeStreamResult`, `:89-162`, decides status and output text only).
- **Captured artifacts.** `agent/testdata/` holds exactly eight files: three claude (one context-exhausted run, two `list-models` captures), one cursor, one openclaw, three qwen. Every other usage test input in the package is an inline string, a constructed file or a fake shell script (class T). Real-CLI material bearing on usage that I found outside `testdata/`: one grok prompt-result payload quoted in a test (`agent/hermes_test.go:1681-1752`), and the *shape* of a kimi wire-log record (`agent/kimi_test.go:853-869`: the comment calls the line verbatim, the helper is a `fmt.Sprintf` template filled with synthetic values).

### 2.1 claude (`agent/claude.go`)

**A. Source and parsing, line by line**

1. Launch. `buildClaudeArgs` (`agent/claude.go:792`): `-p --output-format stream-json --input-format stream-json --verbose --permission-mode bypassPermissions --disallowedTools AskUserQuestion`, optionally `--strict-mcp-config`, `--model`, `--effort`, `--max-turns`; `--resume <id>` when `opts.ResumeSessionID != ""`; then the filtered `ExtraArgs` and `CustomArgs`. The blocked map (`agent/claude.go:777-790`) does not contain `--resume` or `--continue`.
2. Input. Exactly one user message is written to stdin, and stdin is kept open (comment `162-166`, goroutine `167-174`) until the first `result` event closes it (`closeStdin()`, `:285`).
3. Accumulator. `usage := make(map[string]TokenUsage)` (`:192`).
4. `assistant` event (`249-256`) → `handleAssistant` (`:251`) → `handleClaudeAssistant` (`:435`). At `447-454`, when `message.usage` is present and `message.model` is non-empty, it **adds** `message.usage.input_tokens`, `output_tokens`, `cache_read_input_tokens` and `cache_creation_input_tokens` to the four buckets of `usage[message.model]`. There is no de-duplication by message id.
5. `result` event (`271-285`). `claudeResultUsage(msg, opts.Model)` (`702-741`) is called; when it returns a non-empty map the accumulator is **replaced** by it (`276-278`).
   - If `modelUsage` has entries: each `modelUsage[<model>]` contributes `inputTokens`, `outputTokens`, `cacheReadInputTokens`, `cacheCreationInputTokens` **as-is** (type at `663-668`); an entry with an empty key or four zero counters is skipped (loop from `:705`).
   - Otherwise: one entry from the top-level `usage` object (`input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`; type at `:656`), keyed by the result's `model`, else by the configured model.
   - Nothing is subtracted. `claude.go` contains no reference to `total_cost_usd` or `costUSD`.
6. After the first result the loop keeps reading until stdout closes. An `assistant` event that arrives after a result adds onto the map taken from that result; a later `result` replaces the map again.
7. `Usage: usage` (`:423`).

The map can therefore come from three paths: **P1** `result.modelUsage` (taken), **P2** `result.usage` (taken), **P3** Σ assistant `message.usage` (summed; in effect when no result arrives or the result yields nothing). The Result does not record which path produced it.

**Tests and fixtures, line by line**

- `TestClaudeExecuteRecordsResultModelUsage` (`agent/claude_test.go:1226-1273`) [T]: a fake script emits a result with `"modelUsage":{"zhipu/coding-plan":{"inputTokens":123,"outputTokens":45,"cacheReadInputTokens":7,"cacheCreationInputTokens":11,"costUSD":0.01}}`; the test asserts 123 / 45 / 7 / 11 under that key. Fresh run. Nothing is asserted about cost.
- The resume tests set `ResumeSessionID: "sess-old"` (`agent/claude_test.go:1134`, `:1199`) and assert nothing about usage. No claude test asserts usage on a resumed run.
- `agent/testdata/claude-code-2.1.220-context-exhausted-resume.jsonl` [F] (5 lines; loaded at `agent/claude_context_exhausted_test.go:95`): `claude_code_version "2.1.220"` (L1); an assistant message with `model "<synthetic>"` and four zero counters (L4); a result with `total_cost_usd 0`, zero `usage` and `"modelUsage":{}` (L5). The run consumed nothing and its CLI version is below 2.1.277, so the file carries no information on B or C. It does show that a `<synthetic>` model key with zero usage can appear on the stream.
- The two other claude files under `testdata/` are `list-models` captures (2.1.246, 2.1.258) and contain no usage.

**Vendor evidence [V]**

Source 1: `https://code.claude.com/docs/en/agent-sdk/cost-tracking.md` — HTTP 200, 26965 bytes, fetched 2026-11-17 (UTC). `L` = line of the raw markdown as served.

- L15: "The `total_cost_usd` and `costUSD` fields are client-side estimates, not authoritative billing data. The SDK computes them locally from a price table bundled at build time, unless a `modelPricing` table is in effect. They can drift from what you are actually billed when:" (link markup around `modelPricing` omitted)
- L23: "Use these fields for development insight and approximate budgeting. … Don't bill end users or trigger financial decisions from these fields."
- L29: "**TypeScript** provides per-step token breakdowns on each assistant message (`message.message.id`, `message.message.usage`), per-model cost via `modelUsage` on the result message, and a cumulative total on the result message."
- L36: "…Each call produces one `result` message at the end, except in streaming input mode, where one `query()` call carries multiple user turns and each turn emits its own `result` message."
- L38: "**Session:** a series of `query()` calls linked by a session ID through the `resume` option. A resumed call's results report the session's whole spend, not just that call's own."
- L46: "…When Claude uses multiple tools in one turn, all messages in that turn share the same ID, so deduplicate by ID to avoid double-counting."
- L52: "If you make multiple independent `query()` calls, each result reflects only the cost of that individual call. A call that resumes a session also counts the session's earlier spend."
- L58: "In streaming input mode, one `query()` call carries multiple user turns and each turn emits its own result message. The result fields differ in scope:"
- L60: "**`usage`**: covers only that turn, and within it only the main agent loop, not any subagents it ran."
- L61: "**`total_cost_usd` and `modelUsage`, or `model_usage` in Python**: carry the running total for the whole call so far, plus any spend restored when the call resumed a session."
- L75: "`maxBudgetUsd` (TypeScript) or `max_budget_usd` (Python) counts only the call's own spend: totals restored from a resumed session don't count against it, and a `/clear` starts the budget over."
- L79: "The result message, typed as `SDKResultMessage` in TypeScript and `ResultMessage` in Python, marks the end of the agent loop for a `query()` call. It includes `total_cost_usd`, the cumulative estimated cost across all steps in that call. A call that resumes a session also counts the session's earlier spend. Two caveats apply when you read the value:" (link markup omitted)
- L90–92 (table): `usage` — "Excluded. Counts only the top-level agent loop, so tokens consumed inside subagents are not added"; `total_cost_usd` — "Included. Counts subagent requests alongside the top-level loop"; `modelUsage` / `model_usage` — "Included. Counts subagent requests alongside the top-level loop, broken down by model".
- L94: "In single message input mode, Claude Code waits for background subagents still running at the end of the final turn before emitting the result… The result's `total_cost_usd`, `duration_api_ms`, and `modelUsage`, or `model_usage` in Python, include the work done during that wait." (link markup omitted)
- L144: "…When Claude uses tools in parallel, multiple messages share the same `id` with identical usage data. Track which IDs you've already counted and skip duplicates to avoid inflated totals."
- L148: "The deduplicated per-step values are accurate for input and cache tokens. Per-step `output_tokens` is a placeholder, so read output tokens from the result message."
- L189: "The result message includes `modelUsage`, a map of model name to per-model token counts and cost. This is useful when you run multiple models (for example, Haiku for subagents and Opus for the main agent) and want to see where tokens are going." (link markup omitted)
- L220 (the sentence that introduces the two bullets L222 and L223): "Each `query()` call returns `total_cost_usd` on its results. How you combine the values depends on whether the calls share a session:"
- L222: "**Independent calls, with no `resume` or `continue` option**: each result covers only its own call, so add the totals yourself, as the examples below do."
- L223: "**Calls that resume the same session**: Claude Code saves the session's totals to its transcript when the process exits normally and restores them when a later call resumes or forks the session. Each result already includes the session's earlier spend. Read the latest result for the session total; summing results double-counts the restored spend. Before v2.1.277, a session that you resumed through the SDK or `claude -p` started its totals at zero, so each call's results covered only that call." (link markup omitted)
- L302: "Claude Code builds each assistant message from the usage the API reported when the response began, so the message's `output_tokens` is only the count the API had reported at `message_start`, before the response was generated. One API response can produce several assistant messages, and every one of them carries that same placeholder."
- L304: "The API reports the real output count at the end of the response, and Claude Code adds it to the result message. Read output tokens from the result's `usage`, or from `modelUsage` for a per-model breakdown."
- L312: "**`error_during_execution` after a session crash**: every cost field may be zeroed."
- L313: "**`error_max_budget_usd`**: `usage` leaves out the response that crossed the budget, while `total_cost_usd` and `modelUsage` include it."
- L319: "When the Claude Code process crashes, it emits a final `error_during_execution` result and exits… That result may carry zeroed `usage`, `total_cost_usd`, and `modelUsage`, so recover the call's totals from what arrived before it…"

Source 2: `https://code.claude.com/docs/en/headless.md` — HTTP 200, 38900 bytes, fetched 2026-11-17 (UTC).

- L136: "With `--output-format json`, the response payload includes `total_cost_usd` and a per-model cost breakdown… When you continue an earlier conversation with `--continue` or `--resume`, the run reports the conversation's whole total, earlier runs' spend included. Both figures are client-side estimates and can differ from your actual bill."

Source 3: `https://code.claude.com/docs/en/changelog.md` — HTTP 200, 1324706 bytes, fetched 2026-11-17 (UTC). The page describes itself as "generated from the CHANGELOG.md on GitHub" (L9). Each quoted entry is given with the version heading it stands under.

- Under `**2.1.277**` (heading L4390, dated "September 18, 2026" at L4392), L4475: "Fixed a headless resume (`claude -p --resume`, the SDK, a VS Code extension window reload) starting the session's cost and usage totals at zero; headless sessions now save their totals at exit"
- Under `**2.1.302**` (heading L2096), L2151: "Fixed a cloud session's cost and usage restarting from zero after the session resumed in a new process"
- Under `**2.1.285**`, L3676: "Fixed `/cost` and SDK `modelUsage` reporting a turn under the wrong model when the server answered a refusal with a different fallback model than the client expected"
- Under `**2.1.214**`, L6984: "Fixed session cost and token telemetry double-counting on streams that emit multiple cumulative `message_delta` frames"

**B. Fresh session — proven per-invocation [V]**

- P1: L222 and the first sentence of L52 state that a call made without `resume` or `continue` reports only its own call. L61 states that `modelUsage` carries "the running total for the whole call so far", and adds restored spend only "when the call resumed a session".
- P2: L60 states that `usage` "covers only that turn, and within it only the main agent loop".
- P3: L29 and L302 describe the usage on an assistant message as the usage of one API response of the call.
- Conditions attached to this verdict: (1) the CLI session really is new, which `ResumeSessionID == ""` does not guarantee, because `--resume` and `--continue` are not in the blocked map (0.3; both flags are named in headless.md L136); (2) the statements describe the documentation as served on 2026-11-17.

**C. Resumed session — depends on the CLI version [V]**

- P1 on Claude Code ≥ v2.1.277: **proven NOT per-invocation**. L223: "Claude Code saves the session's totals to its transcript when the process exits normally and restores them when a later call resumes or forks the session. Each result already includes the session's earlier spend. Read the latest result for the session total; summing results double-counts the restored spend." L61 names `modelUsage` as one of the two fields that carry the restored spend. L38 and headless.md L136 say the same of a resumed call and of `--continue` / `--resume`. The changelog entry for 2.1.277 describes the change from the other side: until then a headless resume, `claude -p --resume` included, started "the session's cost and usage totals at zero" (changelog L4475).
- P1 below v2.1.277: **proven per-invocation**. L223: "Before v2.1.277, a session that you resumed through the SDK or `claude -p` started its totals at zero, so each call's results covered only that call."
- P2 (any version): L60 scopes the top-level `usage` to "only that turn", and the L90 table row says it excludes subagents. The adapter uses this path only when `modelUsage` yields no entry.
- P3 on a resumed run: **unknown**. The pages describe assistant-message usage per API response; they do not say whether a resumed call re-emits earlier assistant messages on stdout.
- What the adapter does: nothing is subtracted (`agent/claude.go:702-741`), and no code branches on the CLI version (`agent/agent.go:282`). `MinVersions` admits Claude Code from `2.0.0` (`agent/version.go:14`). The captures in the repository are from 2.1.220, 2.1.246 and 2.1.258, all below 2.1.277.
- Where the version sits in the design: the evidence table is keyed by (`adapter`, `normalizer_version`) (design.md §11.6), and `adapter_version` is an optional field of the reported record (design.md §11.5), which the design's example record fills with a CLI version (`"adapter": "claude"`, `"adapter_version": "2.1.34"`). The v2.1.277 boundary is a boundary on that field's value; the table key does not contain the field.
- How the restored amount is tied to the token counters the adapter reads: L38, L52 and L223 speak of "spend" and "totals"; L61 names `modelUsage` as a field that carries the restored spend; L189 defines that field as "a map of model name to per-model token counts and cost"; the changelog entry for 2.1.277 names what is no longer started at zero as "the session's cost and usage totals" (changelog L4475). No sentence says in so many words that the token counters inside `modelUsage` include earlier calls. Listed in 5.1.
- The restore is conditional in the vendor's own words: totals are saved "when the process exits normally" (L223). What a resumed call reports after the earlier process was killed is not stated.

**D. Cost — never set**

- `agent/claude.go` contains no assignment to `CostUSDTicks` (reference list in 2.0). The per-model struct read from the result has no cost field (`agent/claude.go:663-668`), so the `costUSD` of a `modelUsage` entry is not parsed.
- For context: the vendor classes the CLI's own cost figures as estimates (L15, L23 above).

**E.** See 3.2.

**F. Model keys**

- P1: the keys of `modelUsage`, which are provider-reported model names. Several models per invocation are possible (L189). An empty key is skipped (`agent/claude.go:705` loop). The vendor records one case, fixed in 2.1.285, in which `modelUsage` reported "a turn under the wrong model" (changelog L3676).
- P2: one key, the result's `model`, else the configured model.
- P3: `message.model` of each assistant event. `<synthetic>` can appear (fixture L4), with zero usage in the one captured case; the assembly drops rows whose four token counters are zero (`daemon/daemon.go:8044-8046`).

The collaboration contained path parses the same stream with different code (`agent/claude_codec.go`); see 4.3.

### 2.2 codex (`agent/codex.go`)

**A. Source and parsing, line by line**

1. Launch. `codex app-server --listen stdio://` (`agent/codex.go:331`) plus filtered extra and custom arguments (`356-357`); only `--listen` is blocked (`32-34`). The thread is chosen by JSON-RPC, not by a flag.
2. Attempts. `Execute` (`:911`) loops `for attempt := 1; attempt <= 2; attempt++` (`:941`). Each attempt starts its own app-server process and runs `executeOnce` (`:1029`). A second attempt follows when the first ended with a retry-safe reason, `initialize` or `model_catalog_refresh` (`982-988`), and the run is neither a knowledge run nor an ephemeral one (`989-993`). When a retry follows, the first attempt's Result, including any usage in it, is not forwarded; it is forwarded only when `ctx.Done()` fires during the backoff (`1016-1020`). After a `model_catalog_refresh` failure the retry drops the resume id and starts a new thread (`1007-1013`).
3. Inside `executeOnce`: the notification gate is wired at `:1213`; `startTime := time.Now()` at `:1481` (per attempt, before thread setup); `startOrResumeThread` is called at `:1537`; the gate is armed at `:1657`; `turn/start` is sent at `:1661`.
4. Thread. `startOrResumeThread` (`:1983`): with a resume id it sends `thread/resume` (`:2017`) and returns `resumed=true` on success (`:2039`). When the response has no thread id, or the error is not a transport error, it falls through (`2041-2048`) to `thread/start` (`:2089`) and returns `resumed=false` (`:2114`); a transport error is returned without a fallback (`2043-2046`). For private threads (knowledge or ephemeral, `:1984`) a failed `thread/resume` is returned as an error with no fallback (`2018-2025`), an ephemeral run that carries a resume id is refused (`1985-1987`), and `ephemeral` and `persistExtendedHistory` are set explicitly (`2072-2075`).
5. **Notification source.** `handleNotification` (`2884-2941`) drops a notification that carries a different `threadId` (`2900-2902`; `isNotificationFromOtherThread`, `3482-3485`, applies only when the notification has a thread id and the client's own is set), applies the turn gate (`2903-2908`), then routes legacy `codex/event` messages to `handleEvent` (`2914-2927`) and raw v2 notifications to `handleRawNotification` (`2929-2940`).
   - `handleEvent` (`:3287`), `case "task_complete"` (`:3357`): `c.extractUsageFromMap(msg)` (`:3359`). There is no completion guard on this branch.
   - `handleRawNotification` (`:3370`), `turn/completed` (`:3399`): returns early when a turn was already completed (`3418-3420`), then `c.extractUsageFromMap(turn)` (`:3437`; comment at `:3435`: "Extract usage from turn/completed if present (e.g. params.turn.usage).").
   - `extractUsageFromMap` (`3602-3627`) takes the first object present among `usage`, `token_usage`, `tokens`. It reads `input_tokens` (aliases `input`, `prompt_tokens`), `cached_input_tokens` (aliases `cache_read_tokens`, `cache_read_input_tokens`), `output_tokens` (aliases `output`, `completion_tokens`) and `cache_write_tokens` (alias `cache_creation_input_tokens`), and **adds** (`+=`) to `c.usage`: uncached input (`input − cached`, floored at 0, `3629-3635`), output, cache read, cache write. `codexInt64` (`3638-3652`) returns the first non-zero alias and converts a JSON number with `int64(v)`.
   - There is no handler for `thread/tokenUsage/updated`. `codex.go` contains no reference to `tokenUsage`, `turn_token_usage`, `thread_token_usage`, `token_usage_record` or `cache_write_input_tokens`.
   - The gate (`2414-2452`) is closed until armed just before `turn/start`. Once armed, every legacy `codex/event` message is accepted.
6. **Rollout source** (`1890-1917`). It is consulted only when the notification source left `InputTokens == 0 && OutputTokens == 0` (`:1901`). Then `scanCodexSessionUsage(startTime, taskCodexHome, threadID, resumed)` is called and its result **replaces** `u` wholesale (`:1904`). The comment at `:1891` reads "First check JSON-RPC notifications (often empty for Codex)."
   - `scanCodexSessionUsage` (`3668-3705`), `findCodexSessionRollouts` (`:3715`), `codexSessionRoot` (`:3802`): the scan looks under the per-task `CODEX_HOME`, then the ambient `CODEX_HOME`, then `~/.codex/sessions`; it selects rollout files bound to this thread id (by filename or by the first `session_meta` item); it skips a file whose mtime is before `startTime` (`:3681`); it parses the latest candidate (`:3699`).
   - `parseCodexSessionFileSince` (`3859-3937`), for each `token_count` event that has `info.total_token_usage`: when the event is "after start" it adds `current − previousTotal` to a running sum (each counter is handled independently, and a counter that went down contributes its new value, `3939-3976`); in every case it then sets `previousTotal = current`. An event is "after start" when `startTime` is zero, or its timestamp is after `startTime`, or it has no timestamp and either the run is not resumed or a post-start timestamp has already been seen. A `token_count` event that has only `info.last_token_usage` and is "after start" **assigns** that value to the result, replacing the running sum.
   - Fields read from the rollout: `input_tokens`, `output_tokens`, `cached_input_tokens`, `cache_read_input_tokens`, `reasoning_output_tokens` (`3826-3831`). Output: `InputTokens = input − cached` (floored at 0), `OutputTokens = output + reasoning_output`, `CacheReadTokens = cached`; no cache write (`3930-3935`).
7. Usage map (`1911-1917`): one entry when any bucket is non-zero. `Usage: usageMap` (`:1925`).
8. Ephemeral threads. `CodexEphemeral` is set only by the collaboration host runner (`daemon/collaborationrun/host_runner.go:200`); `daemon/daemon.go` never sets it. Such a thread must come back without a `path` (`agent/codex_knowledge.go:165-185`), so no rollout file exists for the scan on those runs. Knowledge runs also pass notifications through a stricter filter first (`agent/codex.go:2892-2895`; `agent/codex_knowledge.go:55-103`).

**Tests, line by line** — all class T; the repository holds no captured Codex stream and no captured rollout file.

- `TestCodexRawTurnCompletedWithoutIDDeduplicatesLifecycleAndUsage` (`agent/codex_test.go:606-629`): the same `turn/completed` carrying `"turn":{"status":"completed","usage":{"input_tokens":100,"output_tokens":10}}` is delivered twice; the test expects 100 / 10 (the second delivery stops at the completion guard).
- `TestParseCodexSessionFileSubtractsCachedInput` (`:898`); `TestScanCodexSessionUsageReadsPerTaskHome` (`:951`).
- `TestScanCodexSessionUsageSubtractsResumeBaseline` (`985-1015`): a constructed rollout whose `total_token_usage` (input / cached / output / reasoning) is 1000/700/50/10 at 00:00:05, 1300/900/70/15 at 00:00:11 and 1800/1400/100/25 at 00:00:12; `startTime` 00:00:10; `resumed=true`; expected `TokenUsage{InputTokens: 100, OutputTokens: 65, CacheReadTokens: 700}`. Its comment: "total_token_usage is cumulative for the resumed Codex session. This task should report only the delta after startTime, not the whole session total."
- Further constructed-file tests: `…ResumeEdgeCases` (`:1017`), `…AllMissingTimestamps` (`:1114`), `…FindsCrossDayResumeWithCacheOnlyDelta` (`:1136`), `…FollowsLinkedSessionsRoot` (`:1168`), `…SelectsCurrentThread` (`:1213`), `…UsesSessionMetadataWhenFilenameDrifts` (`:1247`).

**Vendor evidence**

- Vendor documentation: **no vendor evidence obtained.** On 2026-11-17 (UTC) four plain GETs with `curl/8.0` to the vendor's docs domain (`https://developers.openai.com/codex/app-server`, `https://developers.openai.com/codex/app-server.md`, `https://developers.openai.com/llms.txt`, `https://developers.openai.com/robots.txt`) each returned HTTP 403 with a 59-byte "Forbidden" body. No other client identity was tried.
- Vendor source repository [VS]. First-party, but not the docs domain, so it carries no verdict in this report. `github.com/openai/codex` at commit `8b5223dd7bb74bd486a6d126e388c1566f8a8ae7`, fetched 2026-11-17 (UTC) with plain GETs from `https://raw.githubusercontent.com/openai/codex/8b5223dd7bb74bd486a6d126e388c1566f8a8ae7/codex-rs/<path>`:
  - `app-server-protocol/schema/typescript/v2/TurnCompletedNotification.ts`: `export type TurnCompletedNotification = { threadId: string, turn: Turn, };`
  - `app-server-protocol/schema/typescript/v2/Turn.ts`: fields `id`, `rootTurnId`, `items`, `itemsView`, `status`, `error`, `startedAt`, `completedAt`, `durationMs`. There is no usage or token field.
  - `app-server-protocol/schema/typescript/v2/ThreadTokenUsageUpdatedNotification.ts`: `{ threadId: string, turnId: string, tokenUsage: ThreadTokenUsage, }`. `ThreadTokenUsage.ts`: `{ total: TokenUsageBreakdown, last: TokenUsageBreakdown, modelContextWindow: number | null, }`. `TokenUsageBreakdown.ts`: `totalTokens`, `inputTokens`, `cachedInputTokens`, `cacheWriteInputTokens`, `outputTokens`, `reasoningOutputTokens`. `app-server-protocol/schema/typescript/ServerNotification.ts` names the method `thread/tokenUsage/updated`.
  - `protocol/src/protocol.rs`: L1610–1613 "Agent has completed all actions. v1 wire format uses `task_complete`; accept `turn_complete` for v2 interop." (`TurnCompleteEvent`, L2460 onward, has no usage field); L1615–1617 "Usage update for the current session, including totals and last turn." (`TokenCount(TokenCountEvent)`); `TokenUsageInfo` (L2597–2607) has `total_token_usage` and `last_token_usage`; `new_or_append` (L2610–2632) starts from `TokenUsage::default()` when there is no prior info and executes `info.total_token_usage.add_assign(last); info.last_token_usage = last.clone();` (L2625–2626); `fill_to_context_window` (L2634–2648) replaces `total_token_usage` with `TokenUsage { total_tokens: context_window, ..TokenUsage::default() }`.
  - `app-server/src/request_processors/token_usage_replay.rs`: L1 "Replays persisted token usage snapshots when a client attaches to an existing thread."; L28–34 "Sends a restored token usage update to the connection that attached to a thread. This is lifecycle replay rather than a model event: the rollout already contains the original `TokenCount`, and emitting through `send_event` here would duplicate persisted usage records. …"
  - Read earlier the same day and not re-fetched in the final pass: `history/src/lib.rs` `RolloutLine` (L458–464) has a `timestamp: String` field; `core/tests/suite/token_usage_rollout.rs` has a test `observed_response_usage_accumulates_per_turn_and_thread` asserting thread totals 120, 200, 230 across a resume, for `TokenUsageRecord` items, which are a different item type from the `token_count` events the adapter parses; `app-server/README.md` and `docs/protocol_v1.md` contain no token-usage statement.
- What the VS material shows, for that one commit: `turn/completed` carries no usage; the app-server's usage notification is `thread/tokenUsage/updated`, which the adapter does not handle; `total_token_usage` is built by adding each `last` usage to a running total that starts at zero when there is no prior info; that total can be replaced by a context-window fill, after which every counter except `total_tokens` is zero.
- What it does not show: which Codex versions the commit corresponds to (the adapter admits ≥ `0.100.0`, `agent/version.go:15`); what `turn.usage` or the legacy `task_complete` usage meant in versions that had them; that a resumed thread appends to its original rollout file.

**B. Fresh session — unknown**

- Rollout source: *which records are counted* is fixed by adapter code [K]. The scan is bound to this thread's rollout (`agent/codex.go:3668-3705`; comment `3662-3667`: "Binding the scan to that ID prevents a concurrently-created rollout (for example a Codex subagent) from being billed to this task"), and the thread was created by this invocation's own `thread/start` (`:2089`). An event with a timestamp before `startTime` is baseline-only. What is not proven is the meaning of the number: that a new thread's `total_token_usage` starts at zero rests on S (and on VS).
- Notification source: [S] only. The other-thread filter and the turn gate bound which notifications are read; nothing in the repository or in vendor documentation says what `usage`, `token_usage` or `tokens` on those events count.
- The Result does not say which source produced the map, so the adapter-level verdict is the weaker of the two.

**C. Resumed session — unknown**

- Rollout source: the adapter computes an increment itself [K]. It sums the deltas of `total_token_usage` between consecutive events after `startTime`, with the last total before `startTime` as the baseline (`agent/codex.go:3859-3937`, subtraction at `3900-3911`). The constructed-file test at `agent/codex_test.go:985-1015` [T] exercises exactly this.
- The increment is right only if two provider properties hold, and the repository supports both by comment only [R]: (i) `total_token_usage` is cumulative for the resumed session (`3853-3858`: "Codex reports total_token_usage cumulatively for the whole resumed session, so the last total before startTime is subtracted from the final total"); (ii) a resumed thread keeps writing to its original rollout file, so that the pre-start events are in the file that is scanned (`3662-3667`: "A resumed rollout keeps its original date directory"; the same comment names Codex 0.144.4 as the version in which the thread id is embedded in the rollout filename and in the first `session_meta` item). Where no pre-start event is in the scanned file, no baseline exists and the first total is counted whole (`previousTotalFound` false, `delta := current`).
- Untimed events: in a resumed rollout an event without a timestamp is baseline-only until an event with a post-start timestamp has been seen; in a fresh rollout it is counted (comment `3853-3858`).
- Notification source: no evidence. The gate comment names a case the code cannot separate: "A replay emitted after arm but before a start event cannot be distinguished from a valid legacy current-turn event without breaking those older streams" (`2414-2425`).
- No captured rollout from a real resumed thread exists in the repository.

**D. Cost — never set.** `agent/codex.go` contains no reference to `CostUSDTicks`.

**E.** See 3.3.

**F. Model keys.** At most one key per invocation: the configured model (`opts.Model`); else the rollout's model, adopted only when no model was configured (`agent/codex.go:1905-1907`; read from `turn_context` `payload.model` at `3893-3895` and from `token_count` `payload.info.model` at `3921-3922`); else the literal `"unknown"` (`1913-1915`). The notification source carries no model. A run that used several models is reported under the one key.

### 2.3 codebuddy (`agent/codebuddy.go`)

- **A.** Same stream-json shape and the same three paths as claude. Accumulator at `:210`. An `assistant` event adds `input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens` per `message.model` (`369-376`). A `result` event replaces the map when `codebuddyResultUsage` (`569-608`) returns a non-empty one (`260-262`): `modelUsage` entries as-is (empty key or four zeros skipped), else one entry from the top-level `usage`, keyed by the result's `model` or the configured model. `Usage` at `:349`.
- **B.** `unknown` [S]. The repository holds no captured CodeBuddy output and no codebuddy usage test.
- **C.** `unknown`. `--resume <id>` is appended at `:89`; nothing is subtracted. The vendor statements quoted in 2.1 are about Claude Code and are not evidence for this CLI.
- **D.** `never set` (reference list in 2.0).
- **F.** As claude: `modelUsage` keys, else the result's `model` or the configured model, else `message.model`. Several models per invocation are possible on the `modelUsage` and assistant paths.

### 2.4 qwen (`agent/qwen.go`)

- **A.** `handleQwenEvent` (`:285`) remembers the model from the init event (`289-291`) and from assistant messages (`297-300`). A `result` event with usage **replaces** the state (`318-320`; `qwenResultUsage`, `389-394`, returns nothing when usage is absent, no model has been seen, or all three counters are zero). An `assistant` event **assigns** `usage[message.model] = qwenTokenUsage(message.usage)` (`337-339`), so only the last assistant usage per model survives on this path. `qwenTokenUsage` (`385-387`) maps `input_tokens`, `output_tokens`, `cache_read_input_tokens`, subtracts nothing and has no cache-write bucket. `Usage: state.usage` (`:235`). `--resume` at `:68`.
- **Fixture [F]** `agent/testdata/qwen-code-0.20.0-stream-json.jsonl` (6 lines; cited by the adapter comment at `agent/qwen.go:15-17`):
  - L1 init: `model "qwen3.8-max-preview"`, `qwen_code_version "0.20.0"`, `session_id "session-redacted"`.
  - L2 assistant, usage 0 / 0. L3 assistant: `input_tokens 23066`, `output_tokens 119`, `cache_read_input_tokens 18840`, `total_tokens 23185`. L4 user. L5 assistant: `23473` / `40` / `23060` / `23513`.
  - L6 result: `num_turns 2`, `usage {input_tokens 46539, output_tokens 159, cache_read_input_tokens 41900, total_tokens 46698}`; no model, no cost.
  - Arithmetic: 23066 + 23473 = 46539; 119 + 40 = 159; 18840 + 23060 = 41900. The result equals the sum of the run's two model calls on all three counters. On every line `total_tokens = input_tokens + output_tokens`, so the cache-read count is contained in the input count.
  - The file does not record whether the session was fresh or resumed. Its git history is a single commit (`a81eb9918`, 2026-09-01, "first commit").
- **Tests.** `TestQwenCode020FixtureParses` (`agent/qwen_test.go:319-340`) asserts only `InputTokens == 46539` under `qwen3.8-max-preview`. `TestQwenBackendPreservesSuccessfulResumeSession` (`176-189`) asserts nothing about usage.
- **B and C.** Both `unknown`, one fact short. The capture shows that in one real run of Qwen Code 0.20.0 the result usage was exactly the sum of that run's own model calls. If that run was a fresh session, this settles B for the result path on that version; if it was a resumed session, it settles C. The file does not say which.
- **D.** `never set`.
- **F.** Result path: one key, the model last seen on the stream. Assistant path: `message.model`.

### 2.5 cursor (`agent/cursor.go`)

- **A.** Two maps. Comment `137-140`: "resultUsage holds authoritative session totals from "result" events. If the result event includes usage, we use resultUsage exclusively; otherwise we fall back to stepUsage." A `result` event (`:253`) calls `accumulateResultUsage` (`:265`; `765-789`), which **adds** the top-level camelCase counters, or the nested `usage` object when those are absent. A `step_finish` event adds input, output and cache read to `stepUsage` (`292-302`). When no result carried usage, `resultUsage = stepUsage` (`:336`). `Usage: resultUsage` (`:452`). `--resume` at `:1009`.
- **Fixture [F]** `agent/testdata/cursor-agent-2026.07.20-stream-json.jsonl` (23 lines; event types assistant, result, system, thinking, tool_call, user): L1 init has `model "Auto"`; usage appears only on L23, the result: `{inputTokens 8830, outputTokens 185, cacheReadTokens 44800, cacheWriteTokens 0}`; the result has no model and no cost; the file has no `step_finish` event.
- **Tests [T]**, inline: `agent/cursor_test.go:279`, `:303`, `:322`, `:414` (`TestCursorUsageOnlyFromResult`), `:464` (`TestCursorUsageNoDoubleCount`), `:689`.
- **B.** `unknown`. The capture has a single usage figure and nothing on the stream to compare it with; its session state is not stated.
- **C.** `unknown`. The comment calls the figure "session totals" [R]; no resumed capture exists; nothing is subtracted. cursor is in `resumeRejectionUndetectable` (`agent/agent.go:393-400`).
- **D.** `never set`. Comment `818-824`: "The result event reports token counts only; cursor-agent's stream-json carries no per-turn cost … verified against the real CLI (2026.07.20, both stream-json and json output) and Cursor's CLI docs. … Cursor spend is estimated downstream from the static rate table, never carried through here."
- **F.** `cursorUsageModel` (`755-763`): the event's model, else the configured model (`:107`), else the literal `"cursor"`. The init event's model (`"Auto"` in the capture) is not used for keys. In the capture the result has no model.

### 2.6 copilot (`agent/copilot.go`)

- **A.** `resolveUsage` (`75-86`) picks exactly one source: when the run is not resumed and `session.shutdown` reported tokens, the per-model `session.shutdown` metrics (`:245`); else Σ `assistant.usage` (`:228`); else Σ `assistant.message` output tokens (`:184`, `212-214`); else nothing. Comment `44-51`: "They must never be summed together: all three describe the same tokens." `addUsage` (`109-123`) subtracts cache read and cache write from input. `usage := st.resolveUsage()` (`:442`). `--resume` at `:646`; the `resumed` flag is `opts.ResumeSessionID != ""` (`:393`).
- **Comments [R].** `59-74`: "…It is unusable on a resumed run: the CLI restores its accumulators from a checkpoint, so it also carries every earlier turn's tokens… Under-reporting beats double-billing… assistant.usage is per model call, so it measures exactly this run either way." `523-529`: "NOTE (Copilot CLI 1.0.77): the CLI's JSONL writer drops a fixed set of event types before writing stdout, and both "assistant.usage" and "session.shutdown" are on that list — so on a current CLI neither reaches us and Copilot runs report no tokens at all."
- **Tests [T]**, inline strings: `TestCopilotShutdownUsageSkippedOnResume` (`agent/copilot_test.go:659-671`); `TestCopilotResumeStillReportsPerCallUsage` (`673-687`: 250 from the per-call event, not the session total of 900); `TestCopilotModernStreamReportsNoUsage` (`709-730`).
- **B.** `unknown` [S+R+T]. No captured Copilot stream exists in the repository.
- **C.** `unknown`. The code skips the session-total source on a resumed run [K]; that the source which remains is per model call is asserted by comment and exercised by inline strings only.
- **D.** `never set`.
- **F.** `assistant.usage` carries a model; `""` or `"unknown"` falls back to the active model; the seed model is the literal `"copilot"` when none is configured (`389-393`); `session.shutdown` is keyed per model. Several models per invocation are possible.

### 2.7 opencode, codearts, deveco

Three separate files with the same accounting.

- **A.** Each `step_finish` event adds `part.tokens.input`, `part.tokens.output`, `part.tokens.cache.read`, `part.tokens.cache.write` to one running total: `agent/opencode.go:445-452`, `agent/codearts.go:446-453`, `agent/deveco.go:345-352`. No total or cumulative field is read. Resume flag: `--session` (`agent/opencode.go:103`, `agent/codearts.go:103`, `agent/deveco.go:122`). `Usage` at `agent/opencode.go:321`, `agent/codearts.go:312`, `agent/deveco.go:240`.
- **B and C.** `unknown` [S; T for opencode and deveco]. Per-step events are summed, so the figure is the sum of whatever `step_finish` events the process writes during the run. Whether a resumed run writes `step_finish` events for earlier steps is not evidenced. Inline tests only: `agent/opencode_test.go:1123`, `agent/deveco_test.go:147`; codearts has no usage test. No captured output exists for any of the three. All three are in `resumeRejectionUndetectable`.
- **D.** `never set`. opencode parses `part.cost` as a `float64` (`agent/opencode.go:703-706`: "Read only as round-trip evidence by stepReportedUsage; opencode's billing figures come from the token counters above") and uses it only in `stepReportedUsage` (`:521`).
- **F.** One key per invocation: the configured model, else the literal `"unknown"` (`agent/opencode.go:302-311`, with the comment "OpenCode doesn't report model per-step, so we attribute all usage to the configured model (or "unknown")"; `agent/codearts.go:301`; `agent/deveco.go:229`).

### 2.8 openclaw (`agent/openclaw.go`)

- **A.** `processOutput` (`:419`) first tries to parse the whole stdout buffer as one result blob (`425-434`); `meta.agentMeta.usage` is then taken as-is through `parseOpenclawUsage` (`:663`, `684-691`). Otherwise it scans NDJSON lines: `step_finish` adds (`508-515`), and a final result blob assigns `usage = u` whenever the blob's usage has tokens (`531-534`; the comment on `:530` reads "Prefer usage from the final result if no streaming events reported it."). `meta.agentMeta.lastCallUsage` is never read. `Usage` at `:224`. A non-resumed run passes a newly generated session id `multica-%d` (`:78`); `--session-id` is blocked in custom args.
- **Fixture [F]** `agent/testdata/openclaw-2026.5.5-stdout.json`: `agentMeta.usage {input 34620, output 6, cacheWrite 46482, total 81108}` (L17); `agentMeta.lastCallUsage {input 34620, output 6, cacheRead 0, cacheWrite 46482, total 81108}` (L23); `promptTokens 81102`; `agentMeta.sessionId 4dcd853a-…` (L11) and `systemPromptReport.sessionId aa49a251-…` (L36), `sessionKey "agent:main:main"` (L37); `provider openrouter` (L13), `model anthropic/claude-opus-4.7` (L14); one attempt in `executionTrace`.
- **Tests [T]**, inline: `agent/openclaw_test.go:435`, `:690`, `:712`, `:738`, `:764`, `:791`.
- **B and C.** `unknown`. The capture is a run with one model call in which `usage` equals `lastCallUsage`, so it cannot tell a per-run figure from a per-session one, and it does not state its session state.
- **D.** `never set`.
- **F.** One key (`199-216`): `meta.agentMeta.model`, else `opts.Model` (which the comment calls "the agent name passed via `--agent`, not a real model identifier"), else `"unknown"`.

### 2.9 pi and omp (`agent/pi.go`)

- **A.** Each `turn_end` event adds `message.usage.input`, `output`, `cacheRead`, `cacheWrite` (`381-400`; type at `:555`). `totalTokens` is parsed and not used. The session is a file path: a new path for a fresh run, the given one for a resume (`221-230`, `:686`, `:789`). `Usage` at `:491`.
- **B and C.** `unknown` [S]. No usage test and no captured output exist for pi or omp. The comment at `778-780` refers to a "usage scanner (package usage)"; no Go file under `server/` declares `package usage`, and `PiSessionDir` (`:813`) has no caller outside `pi.go`.
- **D.** `never set`.
- **F.** `message.model`, else the configured model, else `"unknown"` (`386-393`). Several models per invocation are possible.

### 2.10 dsh (`agent/dsh.go`)

- **A.** Launch `--profile multica --stdio` (`:117`); the request carries `resume_session_id` (`:57`). Each `usage` frame adds `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_write_tokens` (`408-420`). A frame whose `request_id` is non-empty and different from this request's is dropped (`383-385`); a frame without a `request_id` is accepted. `Usage: state.usage` (`:430`).
- **B and C.** `unknown` [S + K partial]. The repository holds no dsh protocol document and no captured frames; `agent/dsh_test.go:76` is a fake-script frame.
- **D.** `never set`. `dshFrame` (`74-101`) has no cost field.
- **F.** `provider/model` when the frame has a provider, else `model`; a usage frame with neither is dropped (`409-413`). Several keys per invocation are possible.

### 2.11 antigravity (`agent/antigravity.go`)

- **A to D.** The Result always carries `Usage: map[string]TokenUsage{}` (`:225`). Comment `222-224`: "The Antigravity CLI doesn't surface per-turn token usage today; leave Usage empty rather than report misleading zeros under a guessed model name." Resume is supported (`--conversation`, `:477`); it has no bearing on usage.

### 2.12 Shared ACP code (`agent/hermes.go`, `agent/acp_usage.go`)

Eleven backend implementations drive their agent through the same `hermesClient`: hermes, grok, kimi, reasonix, kiro, qoder (also serving `qoderclicn`), traecli, zeroclaw, mcode, dim, qwenpaw. What follows applies to all of them; 2.13 to 2.23 list only what differs.

- **Two sources, one accumulator.**
  - `usage_update` session notifications: `handleNotification` (`agent/hermes.go:1540`), `case "usage_update"` (`:1579`) → `handleUsageUpdate` (`2059-2067`) → `c.mergeUsage(parseACPTokenUsageSnapshot(msg.Usage))` (`:2066`).
  - The `session/prompt` result, or a `turn_end` update (`:1581`): `extractPromptResult` (`1480-1507`) reads the top-level `usage`, then fills gaps from `_meta` (`parseACPTokenUsageSnapshotFromMeta`, `agent/acp_usage.go:222-236`: nested `_meta.usage` first, flat `_meta` counters as fallback), and takes `modelID` from `_meta`. The adapter then calls `c.mergeUsage(pr.usage)`.
  - `acpUsageAccumulator.merge` (`agent/acp_usage.go:55-88`) never adds. For output, cache read, cache write and cost it keeps the **maximum** value seen across all snapshots. For input it keeps two candidates (from snapshots whose `totalTokens` proved the input shape, the one with the largest total; from the others, the largest raw input) and `resolveInput` (`94-110`) chooses between them; the comment there (`101-103`) calls a normalized record below the other candidate's floor "a per-call delta" that "must not replace the larger cumulative stream". Type comment (`36-43`): "acpUsageAccumulator reconciles the two metering paths shared by ACP backends: cumulative usage_update snapshots and terminal prompt usage. Per-bucket maxima deduplicate equivalent snapshots while retaining buckets omitted from one path."
- **Fields read** (`agent/acp_usage.go:167-216`): `inputTokens|input_tokens`; `outputTokens|output_tokens`; `cachedReadTokens|cacheReadTokens|cached_input_tokens|cache_read_tokens|cache_read_input_tokens`; `cachedWriteTokens|cacheWriteTokens|cache_write_tokens|cache_creation_input_tokens`; `costUsdTicks|cost_usd_ticks` (`205-208`); `totalTokens|total_tokens` (`209-212`).
- **Input normalisation** (`250-259`): cached reads are subtracted from input only when `totalTokens == inputTokens + outputTokens`. Comment `242-249`: "ACP does not specify whether cached reads are counted inside inputTokens. Grok Build counts them inside: totalTokens == inputTokens + outputTokens. The re-bucketing only happens when totalTokens proves that shape."
- **Cost.** A plain integer passthrough of `costUsdTicks` / `cost_usd_ticks` with the maximum rule. Comment `79-81`: "Provider cost is a cumulative turn total on the ACP backends that expose it. Max consolidation preserves a late/richer report without charging a duplicate prompt-result and usage_update twice." `acpUsageInt64` (`266-291`) accepts a JSON integer, else a float converted with `int64(value)`, else a numeric string. No arithmetic is applied by this code.
- **Notification gate.** `acceptNotification` is consulted before a session update is dispatched (`agent/hermes.go:1563-1565`). Nine files set it to "the prompt is in flight": grok `:214`, hermes `:390`, kimi `:152`, kiro `:153`, mcode `:157`, qoder `:170`, reasonix `:166`, traecli `:182`, zeroclaw `:324`. **dim and qwenpaw do not set it** (`agent/dim.go:259`, `agent/qwenpaw.go:142`), so a `usage_update` that arrives during `session/load` or `session/new` is merged for those two.
- **No session filter.** `handleNotification` parses `params.sessionId` (`1551-1557`) and never compares it with the client's own session id.
- **Resume.** `session/load` or `session/resume`, depending on the adapter. Nothing is subtracted on resume; the accumulator starts empty in every process.
- **Tests**, all class T (inline payloads): `agent/acp_usage_test.go:8`, `:116`, `:147`, `:208`; `agent/hermes_test.go:1256`, `:1285`, `:1581`, `:1605` (`TestHermesClientHandleUsageUpdateCumulative`: snapshots 100/50 then 300/120 give 300/120), `:1659`, `:1757` (`TestParseACPTokenUsageCostIsOptional`), `:2265`, `:2313`. The one test with a real payload is the grok capture at `:1685` (2.14).
- **What this establishes for B, C and D.**
  - The accumulator treats each snapshot as a cumulative figure and keeps the largest. Whether an agent's snapshot is cumulative for the turn, for the process or for the stored session is not evidenced for any ACP agent. On a resumed session a snapshot that continues from the session's stored totals would be taken whole.
  - Gated adapters drop updates outside the prompt window, which bounds *when* an update is read, not *what* it counts.
  - The cost field's origin (provider bill, provider-side computation, or CLI-side computation) is not evidenced for any agent other than what 2.14 and 2.16 record.
  - No vendor documentation was consulted for ACP agents; the brief limited vendor lookups to claude and codex.

### 2.13 hermes (`agent/hermes.go`)

- Gate `:390`; resume `session/resume` `:513`; new `session/new` `:538`; gate opened `:642`; prompt-result merge `:690`; gate closed `:723`; `accumulatedUsage()` `:771`; `Usage` `:789`.
- **F.** One key: the configured model (`:467`), else the session's current model (`534-535`, `554-555`), else `"unknown"` (`:777`).
- **Evidence.** No captured hermes usage payload. `agent/hermes_integration_test.go` (`TestHermesRealACPUsageSmoke`) is behind the `agentintegration` build tag and was not run.
- **B, C, D.** `unknown` (2.12).

### 2.14 grok (`agent/grok.go`)

- Gate `:214`; resume `session/load` `:332`; new `session/new` `:356`; gate opened `:418`; merge `:463`; gate closed `:487`; `accumulatedUsage()` `:497`; `Usage` `:515`.
- **F.** One key: the configured model (`:278`), else the session's current model (`352-353`, `373-374`), else the prompt result's `_meta.modelId` (`456-458`), else `"unknown"` (`:503`).
- **Captured payload [F]**, `agent/hermes_test.go:1681-1752` (`TestHermesClientExtractPromptResultMetaUsage`). The comment says "Captured from grok 0.2.106 against a live `grok agent stdio` session" and "Minimal real-world payload (ids truncated)". The payload is one prompt result: `_meta.modelId "grok-4.5"`, `_meta.usage {inputTokens 12929, outputTokens 29, totalTokens 12958, cachedReadTokens 10880, reasoningTokens 24, modelCalls 1, costUsdTicks 75360000}`, with flat mirrors on `_meta`. The test expects input 2049 (12929 − 10880), output 29, cache read 10880, cost 75360000 ticks.
  - It shows that grok 0.2.106 emits `costUsdTicks` as an integer and counts cached reads inside `inputTokens` (12958 = 12929 + 29).
  - It does not show the session state of the capture, the stream around it (whether `usage_update` notifications came, and with what values), or a run with more than one model call (`modelCalls 1`).
- **B.** `unknown` [F partial + S].
- **C.** `unknown`. `TestGrokAttributesUsageOnResumeWithoutConfiguredModel` (`agent/grok_test.go:633-675`) [T] runs a fake script that returns a fixed prompt result; it asserts the key (`grok-4.6`), 100/30/20/5 and `CostUSDTicks 98765`, and says nothing about what a real resumed session reports.
- **D.** `unknown`, flagged for a ruling.
  - Passthrough only: the adapter does no arithmetic and no float conversion on this path.
  - Repo comments call the figure the provider's own [R]: `agent/grok.go:459-462` "xAI prices the turn itself and reports the result here. Carrying it through is the only way the ≥200K long-context surcharge reaches the bill — token counts alone cannot reconstruct which tier a request hit."; `agent/hermes_test.go:1743-1745` "xAI's own price for the turn, in ticks of 1e-10 USD."; `agent/agent.go:190-198`.
  - The captured payload's cost equals the token counts multiplied by list rates, as the test comment itself computes (`agent/hermes_test.go:1723-1727`): "2049*$2 + 10880*$0.30 + 29*$6 per 1M = $0.007536, which is exactly the costUsdTicks the same payload reports". That equality is what either a service-side or a CLI-side computation would produce at list rates.
  - No artifact or vendor document in hand states whether the xAI service or the `grok` CLI computes the field.

### 2.15 kimi (`agent/kimi.go`)

- Gate `:152`; `startTime` `:216`; resume `session/resume` `:260`; new `session/new` `:281`; gate opened `:390`; merge `:434`; gate closed `:450`; `accumulatedUsage()` `:464`; `Usage` `:516`.
- **A.** Two sources, chosen at `482-494`:
  - **ACP source**: when the accumulator holds any token count, one entry under the configured model, else `"unknown"` (`477-480`, `:483`). Comment `429-433`: "kimi-code 0.33.0 sends no usage at all on this path, so this is inert today…"
  - **Wire-log source**: otherwise `scanKimiSessionUsage` (`625-649`) reads `<session root>/<workspace>/<session id>/agents/<agent>/wire.jsonl` for this session id (`660-690`), skips a file whose mtime is before `startTime` truncated to the second (`640-642`), and for each line of type `usage.record` that passes `kimiWireRecordInTurn` **adds** `usage.inputOther`, `usage.output`, `usage.inputCacheRead`, `usage.inputCacheCreation` under the record's `model` (`697-728`). `kimiWireRecordInTurn` (`738-746`): a record with no `time` counts only when the run is not resumed; otherwise it counts when `time >= startTime`.
  - An ACP-reported cost is attached to the fallback-model row of the wire-log map (`499-506`).
- **Comments [R].** `466-476`: "kimi-code 0.33.0 exports no token counters over ACP. Its session/prompt result carries only stopReason, and its `usage_update` notification carries only {used,size} — context window occupancy, not billing. … Bound by sessionID so a concurrent kimi session is never billed to this task." `570-572`: the sibling `step.end` event "repeats the same numbers, so only this type may be summed". `586-589`: "kimi appends to the same wire log across resumes, so the file still holds the previous task's records." `613-624`: "Records are per-LLM-call and incremental, never cumulative: a verified two-step turn logged one record per `llm.request` … Subagent turns get their own `agents/<name>/wire.jsonl` and may run a different model".
- **Tests [T].** `agent/kimi_test.go:853-869`: the comment calls the helper "a verbatim `usage.record` line as kimi-code 0.33.0 writes it, captured from a real session"; the helper is a `fmt.Sprintf` template filled with test values, so only the record's shape (`type`, `model`, `usage.*`, `usageScope`, `time`) is captured. `usageScope` is not parsed by the adapter. Constructed-file tests: `:898`, `:932`, `:949`, `:964`, `:979`, `:1152` (`…SkipsPriorRunOnResume`), `:1179`, `:1207`, `:1235`; `:1062` (`TestKimiBackendPropagatesACPPromptUsage`), `:1095` (`TestKimiBackendReportsUsageFromWireLog`). `agent/kimi_integration_test.go` is build-tagged and was not run.
- **B.** `unknown`. Wire-log source: *which records are counted* is fixed by adapter code [K] (this session id's logs, written at or after `startTime`). That each record is one model call's increment rests on the comments. ACP source: 2.12.
- **C.** `unknown`. Wire-log source: the same time filter [K] leaves out records written by earlier runs of the session, on two properties stated only in comments: records are incremental, and `time` is the record's write time. ACP source: 2.12.
- **D.** `unknown`. Passthrough only; by the comment above 0.33.0 sends no cost.
- **F.** ACP source: one key, the configured model or `"unknown"`. Wire-log source: the record's `model`, else the fallback model; several keys are possible, including subagent models.

### 2.16 reasonix (`agent/reasonix.go`)

- Gate `166-168`; `onNotification` hook `179-183` (feeds the status tracker only while the prompt is in flight); resume `session/resume` `:293`; new `session/new` `:325`; gate opened `:404`; merge `:444`; gate closed `:462`; `accumulatedUsage()` `:483`; `Usage` `:508`.
- **A.** Two sources. The ACP accumulator is used when it holds anything; otherwise the status snapshot (`483-487`). The status tracker (`observe`, `738-790`) handles method `_reasonix.io/session/status_update` with `schemaVersion == 1` and reads `status.usage.turn.{promptTokens, completionTokens, cacheHitTokens, cacheMissTokens, estimatedCost, currency}`: input = `cacheMissTokens`, or `promptTokens − cacheHitTokens` when that is zero; output = `completionTokens`; cache read = `cacheHitTokens`. The stored snapshot is **replaced** by each update whose `sequence` is not lower than the last (`782-789`); snapshots are not summed.
- **B and C.** `unknown`. The field is named `turn` in the adapter's struct; nothing in the repository shows what a real status update counts, or what it holds after a resume. Tests are inline [T]: `agent/reasonix_test.go:127`; `agent/reasonix_execute_unix_test.go:41`.
- **D.** Two sources with different verdicts.
  - Status source: **estimate**. The only cost field read is `estimatedCost` (`EstimatedCost *float64`, `:753`). It is converted with `u.CostUSDTicks = int64(math.Round(*turn.EstimatedCost * float64(CostUSDTicksPerUSD)))` (`:779`), and only when the currency is USD (`isReasonixUSDCurrency`, `798-805`) and the value is non-negative, finite and within range. This is a float-to-integer conversion in adapter code: a `float64` number of dollars multiplied by 1e10 and rounded to the nearest tick. The inline test at `agent/reasonix_execute_unix_test.go:41` maps `estimatedCost 0.001` to `10_000_000` ticks.
  - ACP source: `unknown` (passthrough, 2.12).
  - Which source filled a given Result is not recorded. `reasonixUsagePresent` (`807-809`) counts a cost-only accumulator as present, so a cost that arrives over ACP without tokens selects the ACP source.
- **F.** One key: the configured model (`:491`), else the model named by the status update (`:493`), else `"unknown"` (`:496`).

### 2.17 kiro (`agent/kiro.go`)

- Launch `acp --trust-all-tools` (`:69`); gate `:153`; resume `session/load` `:270`; new `session/new` `:302`; gate opened `:371` (no later `Store(false)` in the file); merge `:441`; `accumulatedUsage()` `:470`; `Usage` `:488`.
- **F.** One key: the configured model (`:241`), else the session's current model (`298-299`, `319-320`), else `"unknown"` (`:476`).
- **Evidence.** `agent/kiro_test.go:169` is a fake script (it emits `costUsdTicks 900`). The "live kiro-cli 2.12.3" comment in that file (`606-607`) is about tool-call frames, not usage.
- **B, C, D.** `unknown` (2.12).

### 2.18 qoder and qoderclicn (`agent/qoder.go`)

- Gate `:170`; resume `session/resume` `:261`; new `session/new` `:285`; gate opened `:349`; merge `:386`; gate closed `:420`; `accumulatedUsage()` `:431`; `Usage` `:449`.
- **F.** One key: the configured model (`:232`), else the session's current model (`281-282`, `302-303`), else `"unknown"` (`:437`).
- **Evidence.** `agent/qoder_test.go:865`, inline [T].
- **B, C, D.** `unknown` (2.12).

### 2.19 traecli (`agent/traecli.go`)

- Launch `acp serve --yolo` (`115-118`); gate `182-184`; resume `session/load` `:274`; new `session/new` `:298`; gate opened `:356`; merge `:389`; gate closed `:420`; `accumulatedUsage()` `:430`; `Usage` `:448`.
- **F.** One key: the configured model (`:244`), else the session's current model (`294-295`, `315-316`), else `"unknown"` (`:436`).
- **Evidence.** Adapter comment `47-50`: "The `initialize` capabilities above were captured from the real traecli v0.120.42 binary". The fake script's comment (`agent/traecli_test.go:25-30`) describes the live capture as "session/prompt returning {stopReason:end_turn}", that is, without usage. The file has no usage test function.
- **B, C, D.** `unknown` (2.12).

### 2.20 zeroclaw (`agent/zeroclaw.go`)

- Gate `:324`; resume `session/resume` `:434`; new `session/new` `:473`; gate opened `:507`; merge `:541`; gate closed `:570`; `accumulatedUsage()` `:580`; `Usage` `:603`.
- **F.** One key: the prompt result's `_meta.modelId` (`:388`, `:540`), else `"unknown"` (`:591`). The configured model is not used as a key.
- **Evidence [R].** Comment `582-586`: "ZeroClaw 0.8.4 reports no token counts over ACP — its prompt result is {sessionId, stopReason, content} — so usageMap stays nil today…". Comment `:156`: "Verified against a real ZeroClaw 0.8.4 binary driven over stdio." `agent/zeroclaw_test.go:137`, inline [T].
- **B, C, D.** `unknown` (2.12).

### 2.21 mcode (`agent/mcode.go`)

- Gate `157-159`; resume `session/load` `:264`; new `session/new` `:289`; gate opened `:313`; merge `:329`; gate closed `:341`; usage `357-358`; `Usage` `:367`.
- **F.** A fixed key, `"unknown"` (`:358`).
- **Evidence.** No usage test.
- **B, C, D.** `unknown` (2.12).

### 2.22 dim (`agent/dim.go`)

- No notification gate (client built at `:259`); resume `session/load` `:412`, inside a retry loop; new `session/new` `:479`; merge `:613`; `accumulatedUsage()` `:662`; `Usage` `:680`.
- **F.** One key: the configured model (`:342`), else the session's current model (`474-475`, `505-506`), else the prompt result's `_meta.modelId` (`610-611`), else `"unknown"` (`:668`).
- **Evidence.** No usage test.
- **B, C, D.** `unknown` (2.12). Without a gate, an update sent during session setup is merged.

### 2.23 qwenpaw (`agent/qwenpaw.go`)

- No notification gate (client built at `:142`); resume `session/load` `:231`; new `session/new` `:261`; merge `:332`; `accumulatedUsage()` `:352`; `Usage` `:369`.
- **F.** A fixed key, `"unknown"` (`:359`). Comment `356-358`: "QwenPaw's model selection is unsupported — the backend never sends opts.Model to the agent. Always attribute usage to "unknown" rather than a model that was never applied."
- **Evidence.** `agent/qwenpaw_test.go:351`, `:392`, inline [T].
- **B, C, D.** `unknown` (2.12). Without a gate, an update sent during session setup is merged.

## 3. Loss and double counting

This section lists, for one invocation, every mechanism in the code by which a provider figure can end up counted twice, replaced, or not carried forward. Each item states what the code does. Whether a provider actually emits the input that triggers it is, unless a citation says otherwise, not evidenced.

### 3.1 Between `executeAndDrain` and the usage report (daemon)

1. **Cost-only rows are dropped by the assembly.** A model entry whose four token counters are all zero is skipped, whatever its `CostUSDTicks` (`daemon/daemon.go:8044-8046`). Adapters can produce such an entry: the ACP adapters build a row when `acpUsagePresent` is true, which includes "cost > 0 with no tokens" (`agent/hermes.go:1514-1516`); kimi attaches an ACP-only cost to its fallback-model row (`agent/kimi.go:499-506`); reasonix uses `reasonixUsagePresent` (`agent/reasonix.go:807-809`).
2. **Two invocations are summed into one map before the assembly.** On the fresh-session retry the per-invocation figures exist as `firstUsage` (`daemon/daemon.go:7965`) and `retryResult.Usage` (returned at `:8005`); `reconcileFreshRetryResult` (`8415-8443`) merges them with `mergeUsage` (`9042-9063`), which adds the four token counters and `CostUSDTicks` per model key (`:9059`). The assembly then sees one map. The comment on the function says "Usage is merged across both attempts in every branch so billing is complete" (`:8414`); two of its six return paths return the first attempt's usage alone: `first.KnowledgeFailure != ""` (`8416-8418`) and `retryErr != nil` (`8430-8432`, where the second invocation returned an error and no Result). The function has one caller (`:8020`), inside the guard `execOpts.Knowledge == nil` (`:7963`); whether `first.KnowledgeFailure` can be non-empty on that path was not traced.
3. **`executeAndDrain` returns no usage when its `drainCtx.Done()` case is selected** (`daemon/daemon.go:8893-8939`). That case is taken when the upstream context is cancelled, when the idle watchdog cancels the agent context, or when the drain deadline (`Timeout + 30s`, `8528-8533`) expires before a Result arrives. For an ordinary run the backend's Result is then awaited only when the context carries a cancellation execution, for at most 10 seconds, and only its `ProcessExited` flag is used (`:8908`; `daemon/cancel_receipts.go:400-412`); the function returns a synthesized Result that has no `Usage` — `idle_watchdog` (`8918-8923`), `cancelled` (`8929-8934`) or `timeout` (`8935-8938`). For a knowledge run the awaited Result is returned when it arrives within 10 seconds (`8897-8901`), else a synthesized failure without usage (`:8905`). When the Result case is selected instead (`8874-8892`), the backend's usage is kept, including when the status is re-tagged `idle_watchdog` (`8881-8891`). The caller's comment reads: "Report usage before any early return — the agent accumulates tokens whether the task completes, errors, or is cancelled mid-run by the poll goroutine. Both claude.go and codex.go populate result.Usage even when runCtx is cancelled, so dropping this on the cancelled path silently under-reports billing." (`daemon/daemon.go:5252-5256`).
4. **A failed start returns nothing.** When `backend.Execute` returns an error, `executeAndDrain` returns `agent.Result{}` and the error (`8501-8513`), and on the first run `runTask` returns `TaskResult{}` (`7950-7952`). No provider turn ran in that case.
5. **The usage report is a single POST.** `handleTask` calls `ReportTaskUsage` once, when the returned usage is non-empty (`daemon/daemon.go:5257-5261`); it posts with `postJSON` (`daemon/client.go:550-557`, `1151-1153`), which makes one attempt, while task completion uses `postJSONWithRetry` (`daemon/client.go:547`). A failure is logged as a warning (`daemon/daemon.go:5259`).
6. **Zero-token model rows disappear.** The same skip as in item 1 removes rows such as a `<synthetic>` claude entry with zero usage.

### 3.2 claude

1. **Result figures replace the per-message sums**, so the two are not added together (`agent/claude.go:276-278`).
2. **Events after a result add onto the result's figures.** The loop continues after the first `result`; a later `assistant` event adds to the map taken from that result (`:251` with the map assigned at `:277`), and a later `result` replaces the map again. The adapter sends one user message and closes stdin at the first result (`:285`).
3. **The per-message path (P3) sums without message-id de-duplication** (`447-454`). Vendor statements about those messages: "When Claude uses tools in parallel, multiple messages share the same `id` with identical usage data. Track which IDs you've already counted and skip duplicates to avoid inflated totals." (cost-tracking.md L144) and "Per-step `output_tokens` is a placeholder, so read output tokens from the result message." (L148; mechanism at L302). P3 is in effect when no result arrives, or when the result yields no entry.
4. **A result can yield no entry.** `claudeResultUsage` returns nothing when `modelUsage` is empty or all-zero and the top-level `usage` is zero (`702-741`); the vendor says a crash result "may carry zeroed `usage`, `total_cost_usd`, and `modelUsage`" (L319). The per-message sums then stay as the Result's usage.
5. **The top-level `usage` path (P2) leaves out subagents** by the vendor's definition (L60, L90 table). The adapter takes P2 only when `modelUsage` yields no entry.
6. **Model entries with four zero counters are skipped** (`:706`); an empty model key is skipped.
7. **A CLI-side double count named by the vendor.** The changelog entry under 2.1.214 reads "Fixed session cost and token telemetry double-counting on streams that emit multiple cumulative `message_delta` frames" (changelog L6984). The entry does not say whether the result's `modelUsage` was affected. The adapter admits Claude Code from `2.0.0` (`agent/version.go:14`).

### 3.3 codex

1. **The first attempt's usage is not forwarded when the adapter retries internally** (`agent/codex.go:989-993`); it is forwarded only if the context ends during the backoff (`1016-1020`). Each attempt has its own process, accumulator and `startTime` (`:1481`).
2. **The rollout figure replaces the notification figure wholesale** when the notification source left uncached input and output at zero (`1901-1904`). A notification figure that holds only cache-read or cache-write tokens is replaced when the scan finds something, and kept when it does not.
3. **Legacy completions add on every occurrence.** `task_complete` calls `extractUsageFromMap` with no completion guard (`3357-3359`); the raw `turn/completed` path adds once (`3418-3421`, `:3437`). Once a legacy `codex/event` has been seen, raw notifications are no longer handled (`2914-2916`, `2930-2940`); raw notifications handled before that are kept.
4. **Reasoning tokens are counted on one source only.** The rollout source reports `output_tokens + reasoning_output_tokens` (`:3933`); the notification source reads `output_tokens` alone (`:3624`, inside `extractUsageFromMap`, `3602-3627`).
5. **Cache writes are never read from the rollout** (`codexRawTokenUsage`, `3826-3831`; result at `3930-3935`).
6. **A `last_token_usage`-only event replaces the running sum** instead of adding to it (`3915-3919`).
7. **A counter that decreases contributes its new value** (`nonNegativeTokenDelta`, `3970-3976`: "A counter reset means the final value already belongs to the new span."). [VS] the vendor's `fill_to_context_window` replaces `total_token_usage` with a value whose counters other than `total_tokens` are zero.
8. **Only the latest matching rollout file is parsed** (`:3699`), and a file whose mtime is before `startTime` is skipped (`:3681`).
9. **Child threads.** Notifications from another thread id are dropped (`2900-2902`) and rollouts of other threads are excluded by the thread binding (`3662-3667`). Whether the parent thread's own total includes a child thread's tokens is not evidenced.
10. **No rollout exists for ephemeral threads** (`agent/codex_knowledge.go:165-185`), so those runs have the notification source only.
11. **Number conversion.** `codexInt64` converts a JSON number with `int64(v)` (`3638-3652`).

### 3.4 Other stream and JSON adapters

- **codebuddy.** Result figures replace per-message sums (`agent/codebuddy.go:260-262`); the per-message path sums without de-duplication (`369-376`).
- **qwen.** Result usage replaces the state (`agent/qwen.go:318-320`). The per-message path keeps only the last assistant usage per model (`337-339`), so with several model calls and no result usage the earlier calls are not counted; in the fixture the result (L6) is the sum of both calls and the last assistant (L5) is one of them. The input and cache-read buckets overlap: the fixture has `total_tokens = input_tokens + output_tokens` with a non-zero `cache_read_input_tokens`, and `qwenTokenUsage` subtracts nothing (`385-387`). A result with usage and no model seen yields nothing (`389-394`).
- **cursor.** Result usage is used exclusively when any result carried usage (`agent/cursor.go:137-140`, `:336`). Several `result` events would be added together (`765-789`). The `step_finish` fallback has no cache-write bucket (`292-302`).
- **copilot.** One source is chosen and the others are ignored (`agent/copilot.go:75-86`). On a resumed run the `session.shutdown` source is skipped; if `assistant.usage` events are absent the run reports output tokens only, or nothing. Per the comment at `523-529`, Copilot CLI 1.0.77 writes neither `assistant.usage` nor `session.shutdown` to stdout. Cache read and cache write are subtracted from input (`109-123`).
- **opencode, codearts, deveco.** `reasoning` and `total` counters of a step are not added to any bucket (`agent/opencode.go:516-519`). All steps go under one model key.
- **openclaw.** In the NDJSON path a final blob's usage replaces the step sums (`agent/openclaw.go:531-534`). In the whole-buffer path only `agentMeta.usage` is read (`:663`). `openclawInt64` converts with `int64(n)` (`706-719`).
- **pi, omp.** `turn_end` usages are added (`agent/pi.go:381-400`); `totalTokens` is not used.
- **dsh.** A `usage` frame with neither model nor provider is dropped (`agent/dsh.go:409-413`). A frame without a `request_id` is accepted whatever request it belongs to (`383-385`).
- **antigravity.** Reports nothing (`agent/antigravity.go:222-225`).

### 3.5 ACP adapters

1. **Maximum, not sum** (`agent/acp_usage.go:55-88`). If an agent's snapshots were per-call increments rather than cumulative figures, only the largest one per bucket would be kept. The buckets of the final figure can come from different snapshots.
2. **Cost follows the same maximum rule** (`79-84`), and `withFallback` takes the larger of the two representations' costs (`:135`).
3. **Notifications are not filtered by session id** (`agent/hermes.go:1551-1565`).
4. **dim and qwenpaw have no notification gate** (2.12), so a `usage_update` emitted during `session/load` (for example a replayed one) is merged.
5. **A late update can be missed or included depending on timing.** Gated adapters close the gate after a quiescence wait (for example `agent/traecli.go:392`, `:420`); updates after that are dropped.
6. **Number conversion.** `acpUsageInt64` converts a non-integer JSON number with `int64(value)` (`agent/acp_usage.go:266-291`).
7. **reasonix.** The status snapshot is replaced, not summed (`agent/reasonix.go:782-789`), and is used only when the ACP accumulator holds nothing (`485-487`); the two sources are not combined.
8. **kimi.** Only `usage.record` lines are summed; `step.end` is not (`agent/kimi.go:570-573`). Subagent wire logs under the same session id are included (`660-690`). An ACP accumulator that holds tokens suppresses the wire-log scan entirely (`482-484`). An untimed record is counted on a fresh run and dropped on a resumed one (`738-746`).
9. **One model key per invocation** for every ACP adapter except kimi's wire-log source: tokens of several models are reported under the single key (see F in 2.13 to 2.23).

### 3.6 Collaboration paths

- **Host mode.** `mergeHostUsage` adds both attempts per model, including cost (`daemon/collaborationrun/host_recording.go:167-185`). `hostUsage` then adds all models into one total with no model key (`187-212`); it emits the cost only when the summed ticks are positive, formatted as an integer part and ten decimal places without floating point (`205-209`).
- **Contained mode.** `readUsage` (`agent/claude_codec.go:226-278`) takes each token field from the result's top-level `usage` when present, else the sum over `modelUsage`; the cost is the result's `total_cost_usd` when present, else the sum of per-model `costUSD`. Numbers are handled as exact decimals (comment `280-281`: "Float64 is never an intermediate"). Per-message usage is parsed into a throwaway map (`:106`).

## 4. Provider launch sites

Scope: non-test Go files under `server/internal/daemon/` at the HEAD named in the header. The list is built from four searches over that whole tree.

1. **Backend starts**, pattern `\.Execute\(|executeAndDrain\(|ResolveBackend\(|agent\.New\(|\.runTask\(|runner\.run\(`. Hits: `daemon/daemon.go:5250`, `:7770`, `:7949`, `:8005`, `:8486` (declaration), `:8501`; `daemon/rk1k_driver_agentintegration.go:75`; `daemon/collaborationrun/host_runner.go:169`; `daemon/collaborationrun/host_recording.go:108`. `backend.Execute` is therefore called in exactly two places: `daemon/daemon.go:8501` and `daemon/collaborationrun/host_recording.go:108`.
2. **Direct subprocess starts**, pattern `exec\.Command(Context)?\(`: 27 hits, classified in 4.3 and 4.4. `os.StartProcess`, `syscall.Exec` and `syscall.ForkExec` have no hit.
3. **Process starts inside the `agent` package that the daemon reaches through a function other than `Execute`**: `agent.DetectVersion` (hook `detectAgentVersion`, `daemon/daemon.go:257`), `agent.ListModels` (hook `listModels`, `:263`) and `agent.RunCollectQuiet` (`daemon/execenv/openclaw_config.go:1335`). Classified in 4.4. The package's comment on its command constructor names the same three kinds: "Everything the daemon launches or probes — task execution, `--version` detection, model discovery — routes through here or through execVia" (`agent/launch.go:108-111`).
4. **Subprocess starts through the launch-lifecycle helper**, pattern `\.CommandContext\(` other than `exec.CommandContext`: one hit, `daemon/collaborationrun/host_preparation.go:242` (git; listed in 4.4).

"Reaches the assembly" means: the `agent.Result.Usage` of that launch is an input of the loop at `daemon/daemon.go:8041-8056`. The loop's output is returned by `runTask` as `TaskResult.Usage` and posted by `handleTask` with `ReportTaskUsage` (`daemon/daemon.go:5257-5258`), which is the only non-test caller of `ReportTaskUsage`.

### 4.1 The normal claim path: two session launch sites

The chain, link by link:

- **Claim.** The poll loop calls `d.ClaimTasksWSFirst(pollerCtx, d.cfg.DaemonID, runtimeIDs, len(slots))` (`daemon/daemon.go:4892`). `ClaimTasksWSFirst` (`daemon/wsrpc.go:316`) tries the WebSocket first and falls back to HTTP `ClaimTasks` (`:371`; `daemon/client.go:301`). For a managed daemon it goes through `claimManagedTasksWSFirst` (`daemon/wsrpc.go:318`; `daemon/managed_wakeup.go:115`) and `claimManagedTasks` (`daemon/managed_transport.go:243`, one `ClaimTasks` per workspace at `:253`). Every variant returns its tasks to the one call at `daemon/daemon.go:4892`.
- **Dispatch.** `d.handleTask(parentCtx, t, slot)` (`daemon/daemon.go:4941`), the only caller of `handleTask` (`:5122`).
- **Run.** `d.runner.run(runCtx, task, provider, slot, taskLog)` (`:5250`), the only call; `d.runner = taskRunnerFunc(d.runTask)` (`:686`).
- **Backend.** `runTask` (`:6856`) resolves it with `agent.ResolveBackend(provider, backendConfig)` (`:7770`).
- **Launch.** `executeAndDrain` (`:8486`) calls `backend.Execute(agentCtx, prompt, opts)` (`:8501`).

Issue, chat, autopilot and quick-create tasks are kinds of the same claimed task (`gcMetaForTask`, `daemon/daemon.go:5761-5773`) and all run through this one `runTask`. A knowledge-bound task is the same path with `execOpts.Knowledge != nil`. Search 1 finds no separate launcher for any of them.

| # | Site | Trigger | Runs under a task claimed through the normal claim path | Does its usage reach the assembly |
|---|---|---|---|---|
| 1 | `daemon/daemon.go:7949` — `d.executeAndDrain(ctx, backend, prompt, execOpts, …)` | The first run of every claimed task. It is a resume when the task carries a prior session id, otherwise a fresh session | Yes | **Yes**, when `executeAndDrain` hands back the backend's own Result (`8874-8892`; knowledge variant `8897-8901`). **No** in the cases listed below the table |
| 2 | `daemon/daemon.go:8005` — `d.executeAndDrain(ctx, backend, freshPrompt, execOpts, …)` | The fresh-session retry, at most once per `runTask`. Guard at `:7963`: `execOpts.Knowledge == nil && shouldRetryWithFreshSession(result, task.PriorSessionID, tools, provider)` (function at `:8310`). The resume id is cleared first (`:7990`) | Yes: the same claimed task and the same `runTask` call as site 1 | **Yes, but merged with site 1, not as a record of its own.** `reconcileFreshRetryResult` (`:8020`, function `8415-8443`) returns one Result, and the assembly sees one map |

How the two results become one map (`daemon/daemon.go:8415-8443`):

- First Result carries a knowledge failure: the first Result is returned unchanged, without the retry's usage (`8416-8418`).
- Retry Result carries a knowledge failure: retry Result with `mergeUsage(firstUsage, retry.Usage)` (`8422-8427`).
- `retryErr != nil` (the second `Execute` did not start): first Result with the first usage only (`8430-8432`).
- Retry established a session id, or completed: retry Result with `mergeUsage(firstUsage, retry.Usage)` (`8433-8438`).
- Otherwise: first Result with `mergeUsage(firstUsage, retry.Usage)` (`8439-8441`).
- `mergeUsage` (`9042-9063`) adds the four token buckets and the cost per model key (cost at `:9059`).

So at this HEAD the two `executeAndDrain` results are not kept apart after `:8020`: the assembly has one merged map as its only input, and nothing downstream of it can tell which invocation a row came from. (design.md §11.10 describes per-invocation usage as coming from each `executeAndDrain` result rather than from `mergeUsage`.)

Cases in which a launch at site 1 or site 2 contributes nothing to the assembly:

- **`backend.Execute` returns an error.** `executeAndDrain` returns `agent.Result{}` (`8502-8513`). At site 1 `runTask` then returns `TaskResult{}` (`7950-7952`). At site 2 the first Result is kept with the first usage only (`8430-8432`).
- **`drainCtx.Done()` wins the `select`** (`8893-8939`). The function returns a Result that it builds itself, with no `Usage`: `idle_watchdog` (`8918-8923`), `cancelled` (`8929-8934`), `timeout` (`8935-8938`), or, for a knowledge run, `failed` with knowledge unavailable (`:8905`). Before building it, the function waits for the backend's Result only when the context carries a cancellation execution, and then for at most 10 s (`:8908`); of that Result only `ProcessExited` is used (`daemon/cancel_receipts.go:400-412`).
- **The Result carries a cost and no tokens.** The assembly skips a row whose four token counters are all zero (`8044-8046`).

After the assembly:

- No `return` lies between site 1 and the assembly (`7953-8040`), and every later return of `runTask` carries `Usage: usageEntries` (`8081`, `8098`, `8118`, `8128`, `8154`, `8173`, `8187`, `8266`).
- The deferred functions of `runTask` that write to the named result do not clear `Usage` after a launch. The prepare-timeout reset `taskResult = TaskResult{}` (`6879-6888`) applies only while `prepareComplete` is false, and that flag is set at `:7593`, before site 1.
- `handleTask` posts the entries whenever `len(result.Usage) > 0`. It does so before the cancellation check and whatever the run error is (`5252-5261`). `ReportTaskUsage` (`daemon/client.go:550-557`) goes through `postJSON` (`1151-1153`), a single attempt; task completion goes through `postJSONWithRetry` (`:547`, `:1118`).

### 4.2 Other callers of `runTask`, and extra processes inside one invocation

| Site | Trigger | Normal claim path | Reaches the assembly |
|---|---|---|---|
| `daemon/rk1k_driver_agentintegration.go:75` — `x.d.runTask(ctx, task, x.provider, 0, x.d.logger)` | Compiled only under the build tag on line 1, `//go:build agentintegration && !windows`. A test driver claims with `x.d.client.ClaimTask` (`:71`), calls `runTask` and then `reportTaskResult` (`:79`) | No. It claims one task with `ClaimTask` and bypasses `handleTask` | The assembly runs, because it is inside `runTask`. The usage is never posted: `ReportTaskUsage` is called only from `handleTask` |
| `agent/codex.go:941-1023` — a second `codex app-server` process inside one `Execute` | The codex adapter loops over at most two attempts. A second attempt follows a retry-safe `initialize` or `model_catalog_refresh` failure (`982-988`), unless the run is a knowledge run or an ephemeral one (`:989`) | Same as the enclosing site (1 or 2) | Only one Result leaves `Execute`. The first attempt's Result, with any usage in it, is not forwarded when a retry follows (`989-993`); it is forwarded only when the context ends during the backoff (`1016-1020`) |
| `agent/openclaw.go:69` → `:317` — `openclaw --version` | Runs inside `Execute`, before the session process is started | Same as the enclosing site | Not a session; no usage is read from it |
| `agent/codex.go:1815` → `2293-2299` — `codex --version` | Runs inside `executeOnce` when a timeout diagnostic is assembled | Same as the enclosing site | Not a session; no usage is read from it |
| `agent/acp_terminal.go:156-177` — a command requested by the agent through ACP `terminal/create` | Inside an ACP session, on the agent's request | Same as the enclosing site | Not a provider process; no usage |

One `executeAndDrain` is therefore one call to `backend.Execute`, but not always one provider process: codex can run two app-server processes in it, and openclaw and codex can run a `--version` process next to the session process.

### 4.3 Collaboration runs: not the normal claim path, and never the assembly

Collaboration runs are dispatched from the same poll loop, before the ordinary claim: `d.collaboration.dispatch(pollerCtx, parentCtx, slots[0], sem, wakeup, taskWG)` (`daemon/daemon.go:4885`). `dispatch` (`daemon/collaboration.go:255`) lists candidates with `client.List(pollCtx, request.RuntimeID)` (`:303`), takes a slot from the shared semaphore, counts the run in `p.daemon.activeTasks` (`:334`), builds a `collaborationrun.Runner` (`:337`) and calls `runner.Run(runCtx, runID, request)` (`:338`). The Runner's own comment reads: "Runner is the single-attempt daemon adapter. Ordinary providers, task tokens, automatic fallback and squad delegation are intentionally absent here." (`daemon/collaborationrun/runner.go:22-23`).

The claim is `r.Client.Claim` on the collaboration client (`daemon/collaborationrun/host_runner.go:54` in host mode, `daemon/collaborationrun/runner.go:79` in contained mode), not `ClaimTasksWSFirst`. There is no `Task`, no `handleTask`, no `runTask`, no `TaskUsageEntry` and no `ReportTaskUsage` on these paths. Usage leaves the daemon inside the run's outcome record as `protocol.CollaborationRunUsage`.

| Site | What starts | Trigger | Normal claim path | Reaches the assembly |
|---|---|---|---|---|
| `daemon/collaborationrun/host_runner.go:225` → `HostExecutor.execute` (`daemon/collaborationrun/host_recording.go:105`) → `backend.Execute(executionCtx, prompt, opts)` (`:108`) | Host mode, first launch. The backend comes from `agent.ResolveBackend(r.Host.Config.Provider, …)` (`host_runner.go:169`). Admitted providers: `claude`, `codex`, `opencode` (`agent/launch_lifecycle.go:154-164`; checked by `HostConfig.Validate`, `daemon/collaborationrun/host_config.go:46-56`) | A collaboration run claimed in host mode (`daemon/collaborationrun/runner.go:51-52`). The launch resumes a stored session when `resume != ""` (`host_runner.go:183`) | No | **No.** `agent.Result.Usage` → `hostUsage` (`host_recording.go:187-212`, called at `:233`) → `Record.Usage` (`:244`) → `r.Client.Outcome` (`host_runner.go:533`) |
| `daemon/collaborationrun/host_runner.go:245` → the same `execute` | Host mode, fresh-session retry | Guard at `:226`: a Result arrived, a resume id was used, the status is not `completed`, `result.ResumeRejected` or `result.ResumeRejectedTransient` is set, and neither the execution context nor the supervisor lifecycle carries an error. The resume id is cleared at `:231`. The launch happens only if the five preparation steps at `232-242` all succeed | No | **No.** `result.Usage = mergeHostUsage(firstUsage, result.Usage)` (`:246`; function `host_recording.go:167-185`), then the same route as above |
| `daemon/collaborationrun/runner.go:148` — `r.Controller.Start(ctx, input)` → namespace helper `exec.CommandContext(ctx, helper, NamespaceHelperArg)` (`daemon/collaborationrun/controller.go:218`) → inside the namespace one of: `exec.Command("/agent", request.Arguments...)` (`daemon/collaborationrun/controller_linux.go:121`); adapter `static_json_v1`, `exec.Command(execution.Entrypoint, execution.Arguments...)` (`daemon/collaborationrun/preparation_linux.go:96-100`); adapter `claude_stream_json_v1`, `executeContainedClaude` (`:101-107`, function `340-405`, process start at `:368`) | Contained mode (Linux namespace), one launch per run | A collaboration run claimed in contained mode (`runner.go:79`); the launch follows the durable burn (`:144`) | No | **No.** The helper's stdout must decode strictly as a `protocol.CollaborationProviderResult` (`runner.go:173`); its `Record` is forwarded with `r.Client.Outcome` (`:175`). For `claude_stream_json_v1` the record is built by `ClaudeStreamCodec.Result` (`agent/claude_codec.go:192-210`) with the usage that `readUsage` (`226-278`, called at `:170`) took from the result frame |

Details that bear on scope and cost for these paths:

- **Host mode uses the same adapters as ordinary tasks**, so the per-adapter findings of section 2 apply to its `agent.Result.Usage`. It differs in two launch options: for a run with a knowledge context it sets `opts.CodexEphemeral = true` for codex (`host_runner.go:200`; the only place in the daemon where the flag is set) and adds `--no-session-persistence` for the other providers (`201-202`). `HostConfig.Validate` rejects session-selecting extra arguments (`--resume`, `-r`, `--continue`, `--session`, `--session-id`, `--fork-session` and others, `host_config.go:57-69`).
- **Host mode has no model key and no per-invocation split on the wire.** `hostUsage` adds every model's counters into one total and emits the cost only when the summed ticks are positive (`host_recording.go:191-210`). After a retry the two launches are already merged (`host_runner.go:246`).
- **Host mode, missing Result.** `execute` returns `gotResult == false` when `Execute` fails (`host_recording.go:109-111`) or when the stop deadline passes before a Result arrives (`155-157`); the recording is then marked partial (`:156`).
- **Contained claude is fresh only.** `ClaudeFreshStreamArguments` (`agent/claude_codec.go:20-35`) builds the argument list without a resume id; the comment above it says "There is no account discovery, resume, fallback, custom configuration or ordinary executor behind this function." (`17-19`).
- **Contained claude forwards a cost.** `readUsage` takes `total_cost_usd`, else the sum of per-model `costUSD` (see 3.6). The vendor describes both fields as "client-side estimates, not authoritative billing data" (cost-tracking.md L15, quoted in 2.1). Its token fields come from the result's top-level `usage` when present, which the vendor scopes to the main agent loop without subagents (L60, L90–92).
- **`static_json_v1` and `/agent`.** The outcome record is whatever the executable wrote to stdout; the daemon applies no usage logic of its own to it.
- **Proof recovery launches nothing.** `recoverProofs` (`daemon/collaboration.go:178`) builds a Runner with a Ledger and a Client only (`:196`). With an existing ledger record `Run` converges without a launch (`runner.go:54-72`; host records go to `recoverHost`, `host_runner.go:35-43`); without one it returns `ErrUnsupported` (`runner.go:73-75`; `host_runner.go:45-47`).

### 4.4 Provider binaries started for something other than a session, and non-provider subprocesses

None of the sites below yields an `agent.Result`, and no code reads usage from them.

**Provider binaries, no session run**

| Site | What runs | When |
|---|---|---|
| `daemon/daemon.go:7832` → `resolveTaskModelSelection` (`:6725`) → `listModels(ctx, provider, runtimeCmd)` (`:6742`) | `agent.ListModels` (`agent/models.go:168`): a model-catalog discovery subprocess of the task's provider. Checked variants: claude sends one `list_models` control request and no user message (`agent/claude_models.go:274-295`) and can also run `claude --help` (`agent/thinking.go:177-180`); codex runs `--version` and then `debug models --bundled` (`agent/thinking.go:318-362`); the shared ACP routine sends `initialize` and `session/new` and no `session/prompt` (`agent/models.go:1906`, `:2036`, `:2070`). Other variants start `opencode models --verbose` (`:763`), `pi --list-models` (`:1211`), `omp models --json` (`:1345`) | **Inside `runTask`, so on behalf of a claimed task, before site 1.** The function comment says the catalog is read "at most once per task — and not at all when nothing needs it" (`daemon/daemon.go:6700-6724`). Results are cached (`cachedDiscovery`, `agent/models.go:507`) |
| `daemon/execenv/openclaw_config.go:1335` (`execOpenclawCLI`, through `agent.RunCollectQuiet`) | `openclaw config file`, `config validate --json`, `config get --json`, `config get agents.list --json`, `agents list --json` (`:748`, `:785`, `:1114`, `:1155`, `:1208`) | `prepareOpenclawConfig` (`:349`), called while a task's execution environment is prepared or reused (`daemon/execenv/execenv.go:728` in `Prepare`, `:1022` in `Reuse`; called from `daemon/daemon.go:6633`, `:6644`) |
| `daemon/daemon.go:4165` (`handleModelList`, `:4130`) | `agent.ListModels` | A model-list request for a runtime; no task |
| `daemon/daemon.go:1117` (`adoptAgentPath`), `:2372` (`probeBuiltinRuntime`), `:2883` (`appendProfileRuntimes`) | `agent.DetectVersion` | Runtime detection and registration; no task |
| `daemon/agents_probe.go:312` (`probeDshMulticaProfile`, called at `:228`) | `<dsh> --profile multica --probe` | Agent discovery; no task |

**Not provider binaries** (the remaining hits of search 2)

- git: `daemon/gc.go:1498`; `daemon/local_directory.go:447`; `daemon/repocache/cache.go:75`; `daemon/execenv/git.go:18`, `:25`, `:36`, `:48`, `:60`, `:67`, `:93`, `:104`, `:112`, `:123`; `daemon/execenv/local_worktree.go:1929`, `:1966`; inside the collaboration namespace `daemon/collaborationrun/preparation_linux.go:275` (`executePreparedGit`, `:263`); and, from search 4, host-mode preparation `daemon/collaborationrun/host_preparation.go:242` (`prepareGit`, `:208`, through `LaunchLifecycle.CommandContext`, `agent/launch.go:120-123`).
- Helper processes: the execution-environment preparation helper (`daemon/execenv/isolation.go:137`, `runPreparationProcess`); the collaboration preparation helper (`daemon/collaborationrun/preparation_linux.go:192`).
- Others: the login shell used to resolve paths (`daemon/config.go:1059`); `<path> --version` of the daemon's own binary for self-update (`daemon/auto_update.go:23-26`); the OS identity command (`daemon/managed_process.go:37-40`); `cmd /c mklink /J` (`daemon/execenv/codex_home_link_windows.go:18`).

Count check for search 2: 4 collaboration provider launches (4.3), 1 probe, 22 non-provider sites; 27 in total. Search 4 adds one non-provider site.

## 5. What I could not determine

Every `unknown` in section 1 is listed here with the evidence that would settle it, followed by what was not done. Two terms are used throughout:

- **Capture** = the raw output of a real CLI run, stored as a file together with the CLI version and the session state (fresh or resumed). This is class F in 0.2. No capture was made for this report, because running an agent CLI was outside the brief.
- **Vendor sentence** = a sentence in the vendor's official documentation, quoted with URL and date (class V).

### 5.1 claude: what the proven verdicts do not cover

The B and C verdicts for claude are `proven` on the vendor sentences quoted in 2.1. The rows below are the points those sentences leave open. The table item in design.md §11.6 is "does the usage contain only this call"; where a row bears on that item, the row says so.

| # | Not determined | What is known | Evidence that would settle it |
|---|---|---|---|
| 1 | Whether, on a resumed call on Claude Code ≥ 2.1.277, the four **token counters** the adapter reads from a `modelUsage` entry include earlier calls' tokens, as distinct from the cost figure of that entry | L61 names `modelUsage` as a field that carries "any spend restored when the call resumed a session"; L189 defines the field as "a map of model name to per-model token counts and cost"; changelog L4475 says a headless resume used to start "the session's cost and usage totals at zero". L79 and L220 frame their passages around `total_cost_usd`. No sentence names the token counters word for word. Under either reading that item is not proven as "only this call" for ≥ 2.1.277 | Two consecutive captured `result` events of one session on a CLI ≥ 2.1.277 (a first call, then a `--resume` call), compared counter by counter; or a vendor sentence that names the token counters |
| 2 | Path P3 (sum of assistant `message.usage`) on a resumed call: whether a resumed call writes earlier assistant messages to stdout again | The vendor pages describe assistant-message usage per API response (L29, L302). P3 is the Result's usage only when no `result` arrives or the `result` yields no entry (3.2 item 4) | A captured stream of a resumed run, or a vendor sentence |
| 3 | What a resumed call reports after the earlier process did not exit normally | Totals are saved "when the process exits normally" (L223); "headless sessions now save their totals at exit" (changelog L4475). On a normal run the adapter closes stdin at the first `result` and waits for the process to exit (`agent/claude.go:285`, `:309`). On cancellation or timeout it closes stdin, sends SIGTERM to the process group and, if the group is still alive after a grace period, SIGKILL (`agent/claude.go:208-230`). No vendor sentence says whether an exit after SIGTERM is a normal exit | A vendor sentence, or a capture of a `--resume` call that follows a run ended by signal |
| 4 | Which side of v2.1.277 a given Result is on | `agent.Result` has no version field (`agent/agent.go:208-277`). The stream's init event carries `claude_code_version` (fixture L1); no non-test Go file under `server/` refers to that field. The daemon holds a version at the launch site and passes it as `Config.CLIVersion` (`daemon/daemon.go:7760`): the last-detected version for the provider when the pinned path is still present (`992-994`, `720-726`; it can be empty), the version paired with a re-resolved path otherwise (`986-990`, `1003-1012`; `967-978` where the entry point is an installer junction), or the profile's version for a custom runtime (`:6921`). The adapter copies it into one diagnostic log record only (`agent/claude.go:357`). The evidence table's key is (`adapter`, `normalizer_version`); `adapter_version` is an optional field of the reported record (design.md §11.6, §11.5) | Not a question of provider behaviour: the value exists at the launch site and on the stream, and is not carried in the Result at this HEAD |
| 5 | Which of the three paths (P1 `modelUsage`, P2 top-level `usage`, P3 assistant sums) produced a given Result | Not recorded (2.1 A). The three paths differ in scope by the vendor's own definitions: P1 includes subagents and restored totals (L61, L90–92), P2 covers "only that turn" and excludes subagents (L60), P3 is per API response with shared message ids and placeholder output counts (L144, L148) | Not a question of provider behaviour: the adapter knows the path at `agent/claude.go:276-278` and `702-741`, and does not record it |
| 6 | Whether the CLI session is new when `ResumeSessionID` is empty | The blocked map has no `--resume` or `--continue` (`agent/claude.go:777-790`); `ExtraArgs` and `CustomArgs` are appended after the adapter's own arguments (`846-847`); they are the daemon-wide default arguments and the agent's own `custom_args` (`agent/agent.go:100-101`; `daemon/daemon.go:7818`, `7864-7865`). headless.md L136 names `--continue` and `--resume` as the flags that continue an earlier conversation | Not a question of provider behaviour: the arguments of each launch are known to the daemon at the launch site |
| 7 | Whether the sentences quoted from the cost-tracking page, which documents the Agent SDK's result message, apply word for word to the `--output-format stream-json` stream the adapter reads | L223 and changelog L4475 name `claude -p` explicitly. headless.md L136 speaks of `--output-format json`. No quoted sentence names `--output-format stream-json`. The field names the adapter reads (`modelUsage`, `usage`, `total_cost_usd` on a `result` event) are the ones the page documents, and they appear in the 2.1.220 capture (fixture L5) | A vendor sentence that names the stream-json output, or the capture described in row 1 |
| 8 | Whether the figures of CLI versions the adapter admits but the quoted pages do not describe behave as quoted | The adapter admits Claude Code from `2.0.0` (`agent/version.go:14`). The changelog records a double-count fix under 2.1.214 (L6984) and a wrong-model fix under 2.1.285 (L3676); neither entry says which result fields were affected before the fix. The pages were read as served on 2026-11-17 | A capture per version of interest; the pages are not versioned |

The D verdict (`never set`) has no open point: it rests on the absence of any assignment in `agent/claude.go` (2.0, 2.1 D).

### 5.2 codex: B and C are unknown

| # | Not determined | What is known | Evidence that would settle it |
|---|---|---|---|
| 1 | Notification source: what the `usage`, `token_usage` or `tokens` object on `turn/completed` `params.turn`, or on a legacy `task_complete`, counts, on any admitted version (≥ `0.100.0`, `agent/version.go:15`) | The adapter adds whatever it finds (`agent/codex.go:3602-3627`). The repo comment at `:1891` calls this source "often empty for Codex". [VS] at the pinned commit `Turn` has no usage field. No vendor documentation was obtained (the docs domain returned HTTP 403) | A vendor sentence on the app-server protocol for the versions concerned, or a captured app-server stream of a fresh thread and of a resumed thread |
| 2 | Rollout source, fresh thread: whether a new thread's `total_token_usage` starts at zero | Which records are counted is fixed by adapter code (thread binding and `startTime`, `3668-3705`, `3859-3937`). [VS] `new_or_append` starts from `TokenUsage::default()` when there is no prior info, at one commit | A captured rollout of a fresh thread in which the first `token_count` total equals the first model call's usage, or a vendor sentence |
| 3 | Rollout source, resumed thread: whether `total_token_usage` continues cumulatively across `thread/resume`, and whether the resumed thread appends timestamped events to its original rollout file | Both properties are asserted by repo comments only (`3853-3858`; `3662-3667`, a comment that names Codex 0.144.4); the test is a constructed file (`agent/codex_test.go:985-1015`). Where the scanned file holds no pre-start event, the first total is counted whole (2.2 C) | A captured rollout from a real resumed thread, with the pre-resume and post-resume `token_count` events in it |
| 4 | Whether `total_token_usage` decreases in real rollouts (a context-window fill or another reset), and what the figure means afterwards | The adapter counts a decreased counter at its new value (`3970-3976`). [VS] `fill_to_context_window` replaces the total with a value whose counters other than `total_tokens` are zero | A captured rollout that contains such an event |
| 5 | Whether a parent thread's total includes tokens of child threads | Notifications of other threads are dropped (`2900-2902`); rollouts of other threads are excluded (`3662-3667`) | Captured parent and child rollouts of one run, or a vendor sentence |
| 6 | Which source (notification or rollout) filled a given Result | Not recorded. The rollout source is used only when the notification source left uncached input and output at zero (`:1901`); it then replaces the figure wholesale (`:1904`) | Not a question of provider behaviour: the adapter knows the source at `1901-1909` and does not record it |
| 7 | Ephemeral threads (collaboration host runs, `daemon/collaborationrun/host_runner.go:200`) | No rollout exists for them (`agent/codex_knowledge.go:165-185`), so their usage is the notification source alone and rests entirely on row 1 | As row 1 |
| 8 | Which Codex releases the pinned vendor-source commit corresponds to | The commit id only (2.2). The adapter admits ≥ `0.100.0`; one repo comment names 0.144.4 | The vendor's release tags for that commit; not looked up |
| 9 | Whether the vendor-source material [VS] is admissible as evidence | The brief admitted vendor official documentation only. The vendor's source repository is first-party but is not the docs domain, so this report rests no verdict on it (0.2) | A ruling by the main session |

The D verdict (`never set`) has no open point: `agent/codex.go` contains no reference to `CostUSDTicks`.

### 5.3 Other adapters: every remaining `unknown`

`antigravity` has no unknown (it returns no usage). Every other adapter is listed once.

| Adapter | Unknown items | What is in hand | Evidence that would settle it |
|---|---|---|---|
| `qwen` | B, C | One capture (Qwen Code 0.20.0) in which the result usage equals the sum of the run's two model calls; the file does not state its session state (2.4) | The session state of that capture: fresh settles B, resumed settles C, each for the result path on that version. Or a captured pair: a fresh run and a `--resume` run of the same session. The assistant-only path (no result usage) is a separate question; it keeps the last usage per model |
| `cursor` | B, C | One capture (cursor-agent 2026.07.20) with usage on the result line only; a repo comment that calls result usage "authoritative session totals" (2.5) | B: a capture of a known-fresh run with per-step or per-call figures to reconcile the result against, or a vendor sentence on what the `result` usage counts. C: a captured `--resume` run of a session whose earlier usage is known |
| `copilot` | B, C | Repo comments and inline tests only. By the comment at `agent/copilot.go:523-529`, Copilot CLI 1.0.77 writes neither `assistant.usage` nor `session.shutdown` to stdout (2.6) | Captured streams of a fresh and a resumed run from a CLI version that writes those events, or a vendor sentence on the two events |
| `openclaw` | B, C | One capture (openclaw 2026.5.5) of a one-call run in which `usage` equals `lastCallUsage` (2.8) | A capture of a run with more than one model call (separates a run total from the last call), and a capture of a second run on the same session id (separates per-run from per-session) |
| `codebuddy` | B, C | Structure only; no capture and no usage test (2.3). The Claude Code sentences in 2.1 are about a different CLI | A captured stream of a fresh run and of a `--resume` run, or CodeBuddy's own documentation of its result fields |
| `opencode`, `codearts`, `deveco` | B, C | Per-step sums; inline tests for opencode and deveco, none for codearts; all three are in `resumeRejectionUndetectable` (2.7) | For each CLI: a captured `step_finish` stream of a fresh run and of a `--session` run, showing whether `part.tokens` is per step and whether a resumed run writes `step_finish` events for earlier steps |
| `pi`, `omp` | B, C | Per-`turn_end` sums; no test and no capture (2.9) | A captured stream of a fresh run and of a run on an existing session file, showing whether `message.usage` is per turn and whether `turn_end` events are written for earlier turns |
| `dsh` | B, C | Per-frame sums; no protocol document and no captured frames (2.10) | The definition of a `usage` frame in the dsh stdio protocol, or captured frames of a fresh and of a resumed request |
| `hermes`, `kiro`, `qoder` / `qoderclicn`, `traecli`, `zeroclaw`, `mcode`, `dim`, `qwenpaw` | B, C, D | The shared accumulator keeps the per-bucket maximum (2.12). No captured usage payload exists for any of the eight. By repo comments, zeroclaw 0.8.4 sends no token counts and the traecli live capture's prompt result has no usage (2.19, 2.20) | Per agent, captured `usage_update` notifications and prompt results over a fresh session and over a loaded or resumed session, showing (a) what one snapshot counts: the turn, the process, or the stored session; (b) whether `session/load` or `session/resume` replays usage updates; (c) whether prompt-result usage is per prompt. For D: who computes `costUsdTicks` — a vendor sentence, or a capture that can be set against the provider's bill |
| `grok` | B, C, D | One captured prompt result (grok 0.2.106) with `modelCalls 1`, no surrounding stream and no stated session state; a cost equal to the token counts multiplied by list rates; repo comments that call the cost xAI's own (2.14) | B, C: the session state of that capture and the stream around it; a captured resumed run. D: a vendor sentence on who computes `costUsdTicks`, or a capture in which the figure differs from tokens multiplied by list rates (the repo comment names a long-context surcharge as such a case) |
| `kimi` | B, C, D | The wire-log scan is bound by session id and time in code; what a record means is stated in comments; the test's "verbatim" line is a template (2.15) | A real `wire.jsonl` that spans a first run and a resumed run of one session, showing that records are per model call and what `time` records. For the ACP source and for D: as the ACP row above (by repo comment, 0.33.0 sends neither usage nor cost over ACP) |
| `reasonix` | B, C; D on the ACP source | The status snapshot is replaced by the latest update; the field is named `turn`; inline tests only (2.16) | A captured `status_update` sequence over a fresh and over a resumed session, showing what `status.usage.turn` counts and whether it starts again with each prompt. D on the ACP source: as the ACP row above |

### 5.4 What an `agent.Result` does not carry at this HEAD

These are not unknowns about provider behaviour. They are facts the code has at some point and does not put in the Result (`agent/agent.go:208-277`), so they cannot be read back from a reported usage map.

- **Which source filled the usage map.** Not recorded by any adapter that has more than one: claude and codebuddy (three paths), codex (notifications or rollout), copilot (three sources), cursor (result or step sums), qwen (result or last assistant), openclaw (blob or step sums), kimi (ACP or wire log), reasonix (ACP or status snapshot), and the ACP accumulator itself (notifications and prompt result, merged by maximum).
- **The CLI version that produced it.** See 5.1 row 4. `Config.CLIVersion` is passed to every backend and is described as "observation only, never used to choose behavior" (`agent/agent.go:282`).
- **Whether the provider session was really new or really continued.** The Result has `SessionID`, `ResumeRejected` and `ResumeRejectedTransient`, and no field that says "this run continued an existing session". 0.3 lists the three ways the daemon's intent and the provider's session can differ: session-selecting custom arguments, the six adapters without rejection detection, and in-adapter fallbacks.

### 5.5 Not done

- No agent CLI was run and no capture was made. No test was run, including the eleven files behind the `agentintegration` build tag (`agent/*_integration_test.go`). A search of those eleven files for `usage`, `Usage`, `CostUSDTicks` and `Resume` found two usage smoke tests, `TestHermesRealACPUsageSmoke` and `TestKimiRealACPUsageSmoke`, which require a non-empty usage map and log it; three files with a resume step (dim, dsh, qwenpaw) that assert nothing about usage; and no stored output of a real run.
- Vendor documentation was consulted for claude only. The codex docs domain refused the request (2.2). For every other adapter no vendor documentation was consulted, as the brief limited vendor lookups to claude and codex.
- Vendor statements describe the pages as served on 2026-11-17. The pages are not versioned, and the changelog page states that it is generated from the vendor's `CHANGELOG.md`.
- Server-side handling of reported usage (the handler, SQL, hourly rollup, metrics and pricing) was not read.
- Not traced: whether `first.KnowledgeFailure` can be non-empty on the path that reaches `reconcileFreshRetryResult` (3.1 item 2).
- Nothing was built or executed. Platform-specific files were read as text only: the Linux-only collaboration code (`daemon/collaborationrun/controller_linux.go`, `daemon/collaborationrun/preparation_linux.go`) and the Windows-only file cited in 4.4.

## 6. Checked claims from the earlier source map

The earlier map is `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-usage-source-map-20261113.md` (dated 2026-11-13, written against HEAD `feb87aee75fa6b9dd051a4e35bf7df8b101edb75`). Its section numbers are written `§n` below.

**Same bytes.** The map ends with a SHA-256 table of the files it read. For all 16 files of that table that lie under `server/pkg/agent/` or `server/internal/daemon/` (`agent.go`, `acp_usage.go`, `acp_usage_test.go`, `hermes.go`, `reasonix.go`, `claude.go`, `codex.go`, `copilot.go`, `cursor.go`, `pi.go`, `opencode.go`, `openclaw.go`, `kimi.go`; `daemon.go`, `client.go`, `types.go`), the recorded hash equals the hash of the file at the HEAD read for this report. The map's line numbers for those files therefore refer to the same lines.

**Scope of this check.** Only the map's statements about `server/pkg/agent/` and `server/internal/daemon/` were checked. Its statements about the server handler, SQL, the hourly rollup, metrics, pricing and the front end were not.

### 6.1 Contradicted by the code

| Map statement | What the code does |
|---|---|
| §4, row "Pi / OpenCode / OpenClaw": "`openclaw.go:509/530` stream 相加，无 stream 时用 final" (stream events are summed; the final blob is used when there was no stream) | In the NDJSON path a final result blob's usage is **assigned** whenever any of its four counters is non-zero, with no test of whether `step_finish` events had already reported usage (`agent/openclaw.go:531-534`). Step sums gathered before it are replaced, not kept. The map's wording follows the code comment on `:530` ("Prefer usage from the final result if no streaming events reported it."), not the condition on `:532`. The row also leaves out the path that is tried first: the whole stdout buffer parsed as one result blob, in which only the blob's usage is read (`425-434`) |

### 6.2 Not contradicted, but less precise than the code

| Map statement | What the code does |
|---|---|
| §4, row "Copilot": "fresh 可用 session.shutdown 总量，resume 则禁用该历史累计；优先 per-call usage，再 message output fallback" | The order in `resolveUsage` is fixed (`agent/copilot.go:75-86`): on a run that is not resumed, `session.shutdown` wins whenever it holds tokens, ahead of per-call usage; per-call usage comes second; message output tokens come third. Read as "per-call usage is preferred", the sentence holds only for resumed runs and for fresh runs without `session.shutdown` tokens |
| §3, row `model`: "Result map key，多数来自 producer 或配置 fallback" (the key mostly comes from the producer or a configuration fallback) | True as a disjunction for every adapter except the two that use a fixed key. The split is roughly even (F in section 2): the producer's model comes first for claude, codebuddy, copilot, cursor, qwen, pi/omp, dsh, openclaw, zeroclaw and kimi's wire-log source; the configured model comes first for codex, opencode, codearts, deveco, hermes, grok, reasonix, kiro, qoder, traecli, dim and kimi's ACP source; mcode and qwenpaw always use the literal `"unknown"` |
| §1 item 5 and §5 first bullet: the fresh-session fallback adds the two runs' usage | True for four of the six return paths of `reconcileFreshRetryResult` (`daemon/daemon.go:8422-8428`, `8433-8435`, `8436-8438`, `8439-8441`). The other two return the first run's usage alone: `retryErr != nil` (`8430-8432`), which the map notes in §5, and `first.KnowledgeFailure != ""` (`8416-8418`), which it does not mention (see 3.1 item 2) |

### 6.3 Checked and confirmed

Each of these was re-read at the cited lines. File names are written as the map writes them, without a directory: `daemon.go`, `client.go` and `types.go` are the files directly under `daemon/`; every other bare name is a file under `agent/`.

- §1 item 3 (reasonix `estimatedCost` enters the cost field): `agent/reasonix.go:776-779`. §1 item 4 (a row with a positive cost and four zero token counters is skipped): `daemon/daemon.go:8044-8046`.
- §2 rows: `agent.go:184` and `:207` are the doc-comment lines of `TokenUsage` and `Result` (types at `:185` and `:208`); `acp_usage.go:20` (doc comment of `acpUsageSnapshot`, type at `:23`), `:55`, `:167`; `hermes.go:1480`, `:2059`, `:2075`; `claude.go:702`, `codex.go:3602`, `reasonix.go:738`; `daemon.go:8041` (provider taken from the run's selection, `:8048`); `daemon.go:5252`, `client.go:550`; `types.go:365`; `client.go:1151`, `:1166`.
- §3: codex subtracts cached input and the ACP parser subtracts only when `totalTokens` proves the shape (`agent/codex.go:3629-3635`; `agent/acp_usage.go:250-259`); the codex rollout path adds `reasoning_output_tokens` to output and the notification path does not (3.3 item 4).
- §4 rows: ACP maxima and `withFallback` (`acp_usage.go:55`, `:117`, `:250`, `:266`); hermes (`690`, `771`, `1480`, `2066`, `2075`); reasonix (`483`, `738`, `776`, `784`: a lower sequence is ignored and an equal one overwrites); claude (`271`, `702`); codex notifications (`3357`, `3418`, `3435`, `3602`) and rollout (`1897`, `3668`, `3859`); copilot's warning at `:448`; cursor (`137`, `265`, `333`, `765`); pi (`386`); opencode (`441`; cost read only as a liveness signal, `520`, `706`); kimi (`429`, `464`, `489`).
- §5: the merge chain `daemon.go:8005`, `8020`, `8415`, `9042`; `:8430`; the usage POST uses the outer context (`daemon.go:5258`) and a single attempt (`client.go:550`, `1151`, `1199`), while complete and fail use the retry variant (`client.go:547`, `586`, `1118`); a nil response body is discarded (`client.go:1210-1212`).
- §8: `codex_test.go:606`, `973`, `1005`; `copilot_test.go:673`, `709`; `daemon/daemon_test.go:1615` (`TestMergeUsage`).
- "Related specs": the version notes for Codex 0.144.4 (`codex.go:3663`), Kimi 0.33.0 and Reasonix `schemaVersion == 1`.

### 6.4 Outside the map

The map says of itself that it did not go through every backend's parser, the Kimi wire scan, or the collaboration usage protocol (its "Caveats" section). The following are in this report and not in the map: the adapters codebuddy, qwen, dsh, deveco, codearts, zeroclaw, mcode, kiro, qoder, traecli, dim, qwenpaw, antigravity and the `omp` key; the codex notification gate, the codex two-attempt loop and ephemeral threads; the ACP notification gate and its absence in dim and qwenpaw; the `drainCtx.Done()` exits of `executeAndDrain` that return no usage (3.1 item 3); the launch-site inventory (section 4); and all vendor evidence (2.1, 2.2).

## 7. SHA-256 of files relied on

**Code state at 2026-11-17T23:13Z.** Worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`, branch `codex/f3-budget-root-20261116`, HEAD `86fea651cc5d09360327b6cf0f8357c1f360070d`.

- `git status --porcelain -- server/pkg/agent server/internal/daemon` printed nothing, and neither did the same command for the earlier map's path.
- Every hash in 7.1 to 7.5, and the map's hash in 7.6, was produced with `shasum -a 256` on the file in the worktree (123 files). Each was also compared with the SHA-256 of the same path's blob at HEAD (`git show HEAD:<path>`): all 123 are equal, so the tables describe the commit as well as the working files.
- The worktree as a whole had 52 uncommitted paths at that time. All of them are under `server/` and none is under the two directories above; they lie in `server/cmd/migrate`, `server/internal/budget`, `server/internal/handler`, `server/internal/migrations`, `server/internal/service`, `server/migrations`, `server/pkg/db` and `server/sqlc.yaml`. None of them is cited in this report.
- The earlier map carries its own SHA-256 table. Its 16 rows for files under `server/pkg/agent/` and `server/internal/daemon/` equal the hashes below (section 6).

**What the hashes cover.**

- A statement with a `file:line` citation is tied to that file's hash below.
- A statement about a whole directory was made by searching `server/pkg/agent` or `server/internal/daemon`: the four searches of section 4, the list of cost references in 2.0 on which every `never set` verdict rests, and sentences such as "the only place in the daemon where the flag is set". Such a statement also depends on files that are not listed here, so it is tied to the commit id together with the empty status of those two directories, not to the per-file hashes. The 2.0 list was run again at the final check, in the working tree and at HEAD (`git grep`), with identical results. An absence stated for one named file is tied to that file's hash.
- One search covered all of `server/`: "no non-test Go file under `server/` refers to `claude_code_version`" (5.1 row 4). It has no hit in the commit (`git grep -l claude_code_version HEAD -- 'server/*.go'`, test files set aside) and no hit in the working tree, uncommitted paths included.
- Vendor pages were piped to stdout and not saved, so they have no hash. 2.1 gives each page's URL, fetch date and the byte size served. The codex vendor source is identified by its commit id (2.2).
- Paths in 7.1 to 7.5 are relative to the worktree root.

### 7.1 Adapter and shared code, `server/pkg/agent/` (41 files)

| Path | SHA-256 |
|---|---|
| `server/pkg/agent/acp_terminal.go` | `e41f078b2d8b55150ab1bcc207171c513e58100bc926f6552b982637de9470ce` |
| `server/pkg/agent/acp_usage.go` | `ab8c945101309d98f41eb5077574b790a86000d1bd62f72a450b169ea8afa57e` |
| `server/pkg/agent/agent.go` | `d44ad98c4b7b6ac42a3c5039762fb4ebca50fdacf8f123f8d1bc74a0f4d4915e` |
| `server/pkg/agent/antigravity.go` | `980950f14894eb346b43f0bf6d96dbe4e4fc6a85a5290cb744be5a20a77bcd44` |
| `server/pkg/agent/builtin_runtimes.go` | `3974c0a29242bb38201fef501c692a31a5c2425f1d0c7d42c8339a7d367acf41` |
| `server/pkg/agent/claude.go` | `027200a87a76dcb2c2c67a3543bb1591cd169b35609af2e3eeec2a31bd65dec3` |
| `server/pkg/agent/claude_codec.go` | `db79d78ff7d666dbf9e2030433246ac7412867abf6ed7f82d15d70b397ded490` |
| `server/pkg/agent/claude_models.go` | `199979b464d847854c870537bc2164f6d5f9d369d5769df8f61f4ef5bebda9c3` |
| `server/pkg/agent/codearts.go` | `e155090197f4e1b0a91955085701e0211540d33310f19c869715e299fb253b6c` |
| `server/pkg/agent/codebuddy.go` | `1fc572d504d47b5ed4ff65bfbe83c9a8a0d84b3ae6ea1db74118fbcb2d3af653` |
| `server/pkg/agent/codex.go` | `77423e30b59f7e1fcd2d93532d4c5baa9040e4318b16102cb85e8c320af865f1` |
| `server/pkg/agent/codex_knowledge.go` | `bf244f4675fdea9f9e304bfad32c33982ec7186cc8f6130ccbb7e1673da301cc` |
| `server/pkg/agent/copilot.go` | `8acc130d025529664019d9539868c47a0905158a8bda42e84b7a8b8b8943c03e` |
| `server/pkg/agent/cursor.go` | `dd178d2bc2e55a3f8c73f7fc78b970327aa4c0ed02e2cc8faadf70403233f03e` |
| `server/pkg/agent/deveco.go` | `ca1a51a062da6d1d62f7ade1d6cda8064919337219dfb63ab024403a93ca9fce` |
| `server/pkg/agent/dim.go` | `4739239e1494e9f9299559ebb8fedeb264f8b5c106d0d3ff488448af3dd17dca` |
| `server/pkg/agent/dsh.go` | `5facfc95577936222974f2103fa17bb950c732201c3c88928b1b6595f9d66706` |
| `server/pkg/agent/grok.go` | `d9ef8421580b78be0883defab2f881237588ef7ebcbcfc429051241137a8c8c0` |
| `server/pkg/agent/hermes.go` | `b77d58ebfb9ed2df3d8e5cec42f403b2dd8f6f8f874e2a85914904e24e116115` |
| `server/pkg/agent/kimi.go` | `451aecdd59d7f45115c2fe9481b06f18fc2921243ca846fee763e9aa7df2124c` |
| `server/pkg/agent/kiro.go` | `9da94bc11dc76c8ddb43f9a78069782f86b56e4fc84c5262bb8b1ab076c727d1` |
| `server/pkg/agent/knowledge_launch.go` | `7f8ccc92938b63fcc2b65d412bbb83b46f87e922c415ba5e57c9ddb84fb6ecdb` |
| `server/pkg/agent/launch.go` | `8dc1b8b836e5572e9f58d1f4497a863d09f9231b4b151b114487db50457aa16c` |
| `server/pkg/agent/launch_lifecycle.go` | `723efd4247436a5f808019f4d62ab2883b9ad90867ad257ee92d8218c760b86d` |
| `server/pkg/agent/mcode.go` | `f12820324fb89afcab34dc3f180d35793c26fa6d00606b66f5dd07030c903fb5` |
| `server/pkg/agent/models.go` | `5715cfd461cdedd8ab322d3bc7b2a296e22dcbb8b5af51eb2812e43eab5acaf5` |
| `server/pkg/agent/openclaw.go` | `c37a85d29b96d6d2240abf45629237ae69678126660e9d36c755155e2ff2179c` |
| `server/pkg/agent/openclaw_stdout.go` | `9ab538789db056dedd042e2c697dbe6c8608e5b39b1be2790b8cb82b20c9dcb2` |
| `server/pkg/agent/opencode.go` | `bc052e5e028bcde28aed1aaa1c97a762c85832316de9ddf3859601783d00db23` |
| `server/pkg/agent/pi.go` | `555b1bdcb551c31850506762728b9d803bbd1d3773654f563a539740127f68b9` |
| `server/pkg/agent/qoder.go` | `6feb9f820e333c57d9e223afd68973d77952e14894a1abdf7154cd935d00f50d` |
| `server/pkg/agent/qwen.go` | `985b559d6615b07b2c5b81b6088af7d5968c513ee89d3ad9fda9411550544355` |
| `server/pkg/agent/qwenpaw.go` | `2f98de86af405349587b1143558e2290f8dd1c4bb151f1d49fa85faa970f1810` |
| `server/pkg/agent/reasonix.go` | `a533cce69c41cbe757a6be3209e9f37847bcf07afe312faf3e685aef278fff32` |
| `server/pkg/agent/run_collect.go` | `95202c2084f64aeae35efe5a77491dbbd8503e937a6750ae81faf37d589b7296` |
| `server/pkg/agent/run_collect_quiet.go` | `bbbf545d26123ac28d2bd9ab5307ca712dd6d10dc74c2baf867bc6a73396d6e8` |
| `server/pkg/agent/stream_json_result.go` | `d4437cb41a3d7e439e4404305a82fdcf2958c2049f2f9f29a2c85e4c6424ac84` |
| `server/pkg/agent/thinking.go` | `35da0574bcc9e348936d1501bc6d344f910e6dc34efb34e869fd3295a7fc2ed4` |
| `server/pkg/agent/traecli.go` | `f9ed437f8901361be26919d70068b6cd800cd018bccb22890b4e31b8f40f66b5` |
| `server/pkg/agent/version.go` | `5abbd5d5de155033fb4e697fe6ada94bcbdd68c733fa4b35b129f3231f3e30b3` |
| `server/pkg/agent/zeroclaw.go` | `78999689afaa3f2aebba984dc00465e624bd1a3418da749dc2585ea3beb55d42` |

### 7.2 Agent tests that are not behind the `agentintegration` build tag (29 files)

| Path | SHA-256 |
|---|---|
| `server/pkg/agent/acp_usage_test.go` | `e8a1830a4d042838bb47f466143a02c2b22ff3fce4632770bfc98401400a8515` |
| `server/pkg/agent/claude_codec_test.go` | `60aa1093fab94a9ba83510a9f2417981425b857b64e3331121d7f5bb524bdffb` |
| `server/pkg/agent/claude_context_exhausted_test.go` | `12d580b278d60bf5f94e9bf98dab503315b91ea87ee994f5ecbe4dfdd8cb14f1` |
| `server/pkg/agent/claude_test.go` | `58394d73138c089d364e23cbbab2d3d04c4b74b115f307360bbfbc351d2e5dc7` |
| `server/pkg/agent/codebuddy_test.go` | `6383eaf7935dc9c3c5f83b441b1b58b23c29529a2d6f6dd591236de4d35b221c` |
| `server/pkg/agent/codex_test.go` | `6e40db603baa72774e4ec79ece49348f7459e2446629bf4c340137f860208b72` |
| `server/pkg/agent/copilot_test.go` | `ecfcf94537903f2f97e422bc9fa8426087703637c1500a00b4beeb2d78146999` |
| `server/pkg/agent/cursor_stream_fixture_unix_test.go` | `b8daa9e38d5732f057507de04a251c871b477dcdeddf97961ce8723dbd5f9757` |
| `server/pkg/agent/cursor_test.go` | `2389fe5150ff1a80ddaba15cfd910afd52902c39e9b6195b3fb3e7eed171d264` |
| `server/pkg/agent/deveco_test.go` | `effa8d853c312cb3f044089c027a6bed20f117555174aedf66fe3fb22f48d764` |
| `server/pkg/agent/dim_test.go` | `f7fb46abbb0b6edbfb4ab5dd7303f3d2f7837740989058e58cc6fab0c2f04265` |
| `server/pkg/agent/dsh_test.go` | `e290020302d23a812bdb6a33fcfeb884b0719393aec2fd1665b6e3c827a69e9c` |
| `server/pkg/agent/grok_test.go` | `0cdd45fab1a7f3eef0689c1ac25a2333d1bbc2c507b2afa1d917512111adf8d8` |
| `server/pkg/agent/hermes_resume_test.go` | `24967ef511f8537cc82a944664f58ed5fd146bae45727176a5edce2dbb81db17` |
| `server/pkg/agent/hermes_test.go` | `3bef6d7dccded79941e4b1ea1f939b1c394193e5917f72b420297a394ad647d2` |
| `server/pkg/agent/kimi_test.go` | `6e967f5d767def8b48a88d0a3bd902dfedc9bd61a45dd327d598ffb74e0d0c99` |
| `server/pkg/agent/kiro_test.go` | `58fba0f76dbe94cc217adc0cb62cca613163bea2d269711a30b9dc2399b143f3` |
| `server/pkg/agent/mcode_test.go` | `b81f1baecf6bce67f0f8b23c104b2424e21b9e1f73235c83abe5846acf2fabc9` |
| `server/pkg/agent/omp_test.go` | `5dad1439a08000d7f13d388a4a0f4c12e35ddb3c57f4b2754741e54b08bfdb68` |
| `server/pkg/agent/openclaw_test.go` | `aa3227ee4c9ab1f515358b5b1c1f41c67414bd42ecf0771e7d95f4da21165183` |
| `server/pkg/agent/opencode_test.go` | `24ec1659b289640cf7b33c574080a3ca559ab9d73a640e05a187742322603b54` |
| `server/pkg/agent/pi_test.go` | `1eb78506f6e533b56098e00314bf7167c771669d44ba5e40b567d6d8d749c576` |
| `server/pkg/agent/qoder_test.go` | `5c85eeb446cc270f7e86748230d5c27d78497064374ea6c49d2a3237c68d87ec` |
| `server/pkg/agent/qwen_test.go` | `7ca9179c02996f0605bf9abdd0949f475c487b1cb411eacca0c016434daa2dc9` |
| `server/pkg/agent/qwenpaw_test.go` | `b5eb44bbcff353a55bf1b1e66043a2936a5d4b5806c4c89fd1866c1d7f44cdf4` |
| `server/pkg/agent/reasonix_execute_unix_test.go` | `b86608975ce65daa713b8d7f0f91714875699b44f6d377371b7b1f78dff93c7a` |
| `server/pkg/agent/reasonix_test.go` | `704d0419b45feaa171d384435ca78ac8bc69c06dd48c7913eaf70946bb99ae86` |
| `server/pkg/agent/traecli_test.go` | `aef73684bd4064276ceebbd9d1c579b5fefdc0566f4ea6c468f3b7b73cf6826e` |
| `server/pkg/agent/zeroclaw_test.go` | `214983d8afdcbf84c9290aa5e15f0a48c0bf627d43ddbc526d38cce2e9ace321` |

### 7.3 Agent tests behind the `agentintegration` build tag (read as text, not run) (11 files)

| Path | SHA-256 |
|---|---|
| `server/pkg/agent/codearts_integration_test.go` | `fa8ec815904a2ffba0dc9e7e6527263890ecee53035631a5481ebd5b76d78e04` |
| `server/pkg/agent/cursor_integration_test.go` | `a73d9196116e4a1e4297a52884f55ea502c1378293bc1e1711fb5d88b62ae6c2` |
| `server/pkg/agent/dim_integration_test.go` | `3537123227dd2b861005f94ee26525eb7d319af58d468ffa6b1b823510ecfdb2` |
| `server/pkg/agent/dsh_integration_test.go` | `91d57a29cd41038606b146efabe4ff7f8ad0b531da3bd1ddb6c8f0436e93339a` |
| `server/pkg/agent/grok_integration_test.go` | `aef6b7bbe4355dc50893a277cda5eea308f72dc06633b1840b4155667702bf9b` |
| `server/pkg/agent/hermes_integration_test.go` | `e6ae737c2915dac0312978f022c02e21b35d69906160c2ad31ce9002ec2248bb` |
| `server/pkg/agent/kimi_integration_test.go` | `357fb11dbf57f7821d0b208005d3788905d7f3ede758bba5733ef69f4170878a` |
| `server/pkg/agent/mcode_integration_test.go` | `44b41a0d912d581234a0274f4edaf219f4f68774759760dcf4b1be31e61f6e28` |
| `server/pkg/agent/qwenpaw_integration_test.go` | `e1c7cf9eef8a3261e23b376c5f172f467cd1d4514e15cacfe3c461ee90a937ee` |
| `server/pkg/agent/real_agent_smoke_integration_test.go` | `b1b75c14921c10e98deed79cb914a209cfebc256414d155ce3e6655478f33162` |
| `server/pkg/agent/traecli_integration_test.go` | `333430ab463f10a2c30e57a8b1f6fdc37667882bc1397fe55d080e944a712fb0` |

### 7.4 Captured fixtures, `server/pkg/agent/testdata/` (8 files)

| Path | SHA-256 |
|---|---|
| `server/pkg/agent/testdata/claude-code-2.1.220-context-exhausted-resume.jsonl` | `c76a518ec81aa67572aad7227147b8d853d4ca10f67d3f50a52ca66998eabb37` |
| `server/pkg/agent/testdata/claude-code-2.1.246-list-models.jsonl` | `b800b6931896daf5404e583978ef09025d58f2626d76b026baed4221e523fd3f` |
| `server/pkg/agent/testdata/claude-code-2.1.258-list-models.jsonl` | `fc0128075aa21bb7c8763c518b98525dfcf0c05c5ac89e71f2e5aabedaf57858` |
| `server/pkg/agent/testdata/cursor-agent-2026.07.20-stream-json.jsonl` | `fcc405fc322aceb9c26e9a8f5d88daca3f8d0f39dd2fddc45830a9e18e833280` |
| `server/pkg/agent/testdata/openclaw-2026.5.5-stdout.json` | `2509b9c3e55f65d7b6eb3d1da600d147d4a93807d9765f36f3fbca7ff89e1f40` |
| `server/pkg/agent/testdata/qwen-code-0.20.0-error-stream-json.jsonl` | `c560dee4a4a7c9a0460708b2212cd0d8debf092bed07a4f0c53ff31df6d9a77c` |
| `server/pkg/agent/testdata/qwen-code-0.20.0-resume-not-found.stderr.txt` | `7b8f29a0b48d1bf0cfaf2ae52f7ca72ad40f105e261347d3c926033ba84d8aef` |
| `server/pkg/agent/testdata/qwen-code-0.20.0-stream-json.jsonl` | `9ac10e3a5929194789bf7e64ad1d65296cfcf60d788fddb91782705fd700af32` |

### 7.5 Daemon, `server/internal/daemon/` (33 files)

| Path | SHA-256 |
|---|---|
| `server/internal/daemon/agents_probe.go` | `69398065100dad2fd26d3a7479d95049588984099b71570b70835eef1e272953` |
| `server/internal/daemon/auto_update.go` | `5df0d64fe62c979cb6386074b6f108dffa4e45d78b3637a3cfd9cab2266f2b33` |
| `server/internal/daemon/cancel_receipts.go` | `cc69d2b57a21d54417de99c5b94bc24e4a00077eb55dba39f5f8ac05cc3c2d61` |
| `server/internal/daemon/client.go` | `ba1b521622c9e6ad2461271ac30ea8517da59c871ad78128421fa0ca310e83ec` |
| `server/internal/daemon/collaboration.go` | `ab92baecb348ca71319adbc499e220f0f370cb40ec1723b3f1624039b05d44e6` |
| `server/internal/daemon/collaborationrun/controller.go` | `64644a4488f66fc8abc5e05aad2c6c0e870ea1dcd04ecf6af9d852f907fe1ef6` |
| `server/internal/daemon/collaborationrun/controller_linux.go` | `e143add39dec306b28fb6508ba44611f4103ecf24e30c556af7bd49fb45d0e6e` |
| `server/internal/daemon/collaborationrun/host_config.go` | `bfd1ab2dec0c7242212b6f810074a43e880fe78677f0428f47d87b24e37a4a34` |
| `server/internal/daemon/collaborationrun/host_knowledge.go` | `94e8b3ec6f3a912fb20ac58e55a2af14513707413b6aaabaaf4dc2a4342c35a6` |
| `server/internal/daemon/collaborationrun/host_preparation.go` | `8b897ba497b013c0ca47eeb2b82658cfa425bdb3c3dcfcd90120d716879e3c2f` |
| `server/internal/daemon/collaborationrun/host_recording.go` | `d7ee88e1bb0612071f132e06884e9febfb099eb8b92eb44d3f1eb80fbc914ed9` |
| `server/internal/daemon/collaborationrun/host_runner.go` | `be033bc153dfc6ac78e10e708cdda19604939aec9855044777cb8526a8d22bb6` |
| `server/internal/daemon/collaborationrun/preparation.go` | `04d1cd9b4182ed9b648ea766570e0879c59b93f20cacbc45ee9e98d408f8bc18` |
| `server/internal/daemon/collaborationrun/preparation_linux.go` | `a14accb3d71d686cad47581a69a56de69ed3e228ab72dbf561ea149fafbb0fbf` |
| `server/internal/daemon/collaborationrun/runner.go` | `19252b5f861dc382669ef3637694acf76d9de60cb4a545332dc7eef9b31f67c7` |
| `server/internal/daemon/config.go` | `eba95a03328a95e5b6042f7b9d7b42956e40a6fb8989d1979dbf355d1887fb9b` |
| `server/internal/daemon/daemon.go` | `27a914766f0cfebf0be1f66fa6a089a27506cf442fdf597fa0d1f4740fdc7bd9` |
| `server/internal/daemon/daemon_test.go` | `7c377d4a6e3e808fb806b2f0a3e8004d74b2b6404858ad2f5b227c9aa4825c48` |
| `server/internal/daemon/execenv/codex_home_link_windows.go` | `3df01877ac80c76782ef29b356cdc433a992d4d90e7327297a20f20abf726681` |
| `server/internal/daemon/execenv/execenv.go` | `55fa26ff22aa4527a3d2acc08200c04348611eb88c30591ca8ec31287dd624f5` |
| `server/internal/daemon/execenv/git.go` | `468081b256eeb1bb15d6010395ef5a23dc41e3d1dc4e0a3609746e7117245062` |
| `server/internal/daemon/execenv/isolation.go` | `4292b0b6c2ba1c44931398f16c1ad41011830a98852ee4dea14c6b1ab56c66c8` |
| `server/internal/daemon/execenv/local_worktree.go` | `b4bda71c1667a678436ed32d0d8b4353a3a6bdb2fc7500a670569d3a9eaf96b1` |
| `server/internal/daemon/execenv/openclaw_config.go` | `8bc1ed70b9cef0bcd081c50e7682f473b0d6aab8439db6d0a57c608633d7f147` |
| `server/internal/daemon/gc.go` | `a38cd668e1ec3fddf25fb1019d23d1a81b38be8be8efb12b4ff477932b7c2340` |
| `server/internal/daemon/local_directory.go` | `76f1592747ed45706e7bcd3700ad59a7f20cc7d0d96d3369004bf122b83d83e8` |
| `server/internal/daemon/managed_process.go` | `78b1e7c98c6842aaa09b7678333918d7087eba2d11e140908ed12e58a599f838` |
| `server/internal/daemon/managed_transport.go` | `0252432eb59fbe39c270b6122e83896ede8e942aca5c1aee2dd3b643755ecf2e` |
| `server/internal/daemon/managed_wakeup.go` | `d1a3f29fc4a1bcda667e8f4a93925a90d807ea220695841c4943626a85c2ef37` |
| `server/internal/daemon/repocache/cache.go` | `1e0cd8a42ed29112afcd61e4bcfda21b80a54a136c8a6f0d89be5cb490128ad0` |
| `server/internal/daemon/rk1k_driver_agentintegration.go` | `cbd052f3126961693657c3eef10a93c59d052b08d447d5d12560b2957c69bafa` |
| `server/internal/daemon/types.go` | `5df0049ed6a19c2a6295b5c445352b35812e8e434eb875790403bdc50c54092d` |
| `server/internal/daemon/wsrpc.go` | `31ca2ceb3173f1e0278e8ae74b4ac16742af5e3fbfb22e0a0bcb53dbc1f372ab` |

### 7.6 Other files

| Path | SHA-256 | Note |
|---|---|---|
| `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-usage-source-map-20261113.md` (worktree) | `c8b26da1307eef199bd80914d0e96dbaeb3b5a144abf9fadd7d5fefe5c8c6afe` | The earlier source map; unmodified in the worktree and equal to its blob at HEAD |
| `.trellis/tasks/11-16-f3-parent-budget-root/design.md` (main checkout) | `60300449f8345032082630a688fcc18fef3feeb0f7a301145977559c1325a68f` | The version read during most of this research. It no longer exists on disk: the file is untracked and was edited afterwards, so this value cannot be re-derived |
| the same file | `7b865c86ccaec320cfeefdd0920be312a388379f4e68e47ffacf1c9209deac3b` | The version on disk at 2026-11-17T23:13Z: 716 lines, 111881 bytes, modified 2026-11-17T22:12:34Z. The passages cited from §11.5, §11.6 and §11.10 were re-read in this version and are present in it. The file is being edited by the main session and may change again |
