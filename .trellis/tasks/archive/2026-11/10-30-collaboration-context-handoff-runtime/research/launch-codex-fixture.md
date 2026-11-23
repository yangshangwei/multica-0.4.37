# Research: fake Codex app-server for startup handoff verification

- Query: Find the smallest existing fixture patterns to verify actual Codex `turn/start` input and internal resume fallback through `Runner` / `HostExecutor`, without an installed CLI or account.
- Scope: internal, read-only research; no implementation or provider calls.
- Date: 2026-10-31

## Findings

### Smallest existing patterns

| File / symbol | Reuse value |
| --- | --- |
| `server/internal/daemon/codex_reuse_fallback_test.go:43` | Minimal complete subprocess app-server handshake. Captures real working directory/config home/setup RPC and drains stdin until EOF so the process terminates cleanly. |
| `server/pkg/agent/codex_test.go:3895` — `TestCodexExecuteSemanticInactivityDoesNotAffectNormalTurnCompletion` | Adds the minimal successful `turn/started`, `item/completed` agent message and `turn/completed` notifications, yielding an observable final output. |
| `server/pkg/agent/codex_test.go:2076` — `TestCodexStartOrResumeThreadFallsBackOnResumeError` | Exact recoverable resume rejection: `thread/resume` returns JSON-RPC error code `-32602`, message `unknown thread`; backend then calls `thread/start` in the same process. |
| `server/pkg/agent/codex_test.go:3990` — `TestCodexExecuteRetryAfterCatalogFailureStartsFreshThreadForResume` | Two physical launches, per-invocation RPC capture, first catalog failure then fresh successful second process; asserts no second `thread/resume` and continuity behavior. |
| `server/pkg/agent/codex_test.go:4259` — `writeFakeCodexAppServer` | Writes a test-owned executable with optional `--version` response. It is private to the `agent` test package, so it cannot be imported by `collaborationrun` tests. |
| `server/internal/daemon/collaborationrun/host_runner_test.go:130` — `newHostRunFixture` | Existing HTTP server, managed permit/termination/proof ledger and test-binary subprocess pattern. This is the suitable outer harness for the new contract. |
| `server/internal/daemon/collaborationrun/host_runner_test.go:503` | Existing local session seeding before a run via `hostSessionKey`, `lockHostSession`, `save`, and `Close`. |

No existing Codex `Runner`/`HostExecutor` fake was found in `collaborationrun/host*test.go`; its reusable provider process currently speaks Claude stream JSON. The `agent` package's `newTestCodexClient`/`drainRPCScript` are useful protocol references but bypass native process launch and cannot prove a host binding.

For portability, adapt the small RPC dispatcher into the existing test-binary subprocess style rather than importing test-only helpers or creating a new shell dependency. A POSIX script is already accepted by nearby tests, but those tests explicitly skip Windows.

### Minimal RPC sequence for the exact-input regression

Read newline-delimited JSON, echo each request's actual `id`, and handle methods rather than assuming numeric positions. Host options forbid a thread title, so no `thread/name/set` request is needed in this fixture.

1. Receive `initialize`; return a result object `{}`.
2. Consume the `initialized` notification; it has no request ID and needs no response.
3. For the live-resume case, receive `thread/resume`; return `{ "error": { "code": -32602, "message": "unknown thread" } }` with the same ID. A transport error/EOF is not equivalent: transport failure deliberately does not take same-process fresh fallback (`codex.go:1938`).
4. Receive `thread/start`; return `{ "thread": { "id": "thread-handoff-fresh" } }`.
5. Receive `turn/start`; retain the complete `params.input` and especially its single `text` block before acknowledging with a result object `{}`.
6. Emit `turn/started` with the matching thread and a stable turn ID.
7. Emit `item/completed` with an `agentMessage`, a stable item ID and short output such as `Done`.
8. Emit `turn/completed` with matching thread/turn IDs and status `completed`.
9. Continue reading until stdin EOF, then exit. This follows the backend's normal shutdown and avoids racing an early process exit against RPC/notification consumption.

Assert the retained `turn/start.params.input[0].text` hash equals the server's recorded handoff binding hash. Also assert that `thread/resume` really occurred and was followed by `thread/start`; otherwise the fixture might only test the host's earlier missing-rollout fallback and miss the late text transformation.

### Seeding a real host resume without private history

`HostExecutor.Probe` rejects a caller-supplied `Options.ResumeSessionID` or `ResumeExpected` (`host_config.go:152`). Seed the existing local host session record instead, as the current host resume test does.

For Codex that is not sufficient by itself. `runHost` checks `CodexResumeRolloutPresent` after preparation and discards the pointer if the file is missing (`host_runner.go:128`). Seed a test-owned matching rollout under:

`<Host.Directory>/<hostSessionKey>/codex-home/sessions/rollout-<timestamp>-<prior-session-id>.jsonl`

The presence helper (`execenv/codex_home.go:783`) uses the known filename pattern; nearby tests use simple synthetic nonempty content (`codex_home_sessions_test.go:706`). Create only the test-owned home/session data. Follow the per-test account/config-home isolation in `daemon/codex_reuse_fallback_test.go`; never let home preparation inherit the real installed Codex credentials or sessions.

`PrepareCollaborationCodexHome` creates this exact local `codex-home` below the execution root (`execenv/collaboration.go:13`). If the fixture wants a successful new session pointer retained after completion, it must also create the fresh thread's matching rollout. Otherwise the host intentionally clears the unbacked result pointer; the run's output/proof can still succeed, so do not mistake that pointer clearing for a failed handoff test.

### Separate physical retry test

Same-process `thread/resume` → `thread/start` fallback is the smallest exact-text test. It does not prove that every new physical provider process gets a new binding.

For that second assertion, adapt the existing two-invocation catalog-refresh fixture (`codex_test.go:3990`):

- Maintain a test-owned invocation counter and per-invocation RPC capture file.
- First process: complete initialization/thread setup/turn start, emit `turn/started`, then emit the recognized `codex_models_manager::manager: failed to refresh available models` diagnostic on stderr without any semantic item, and wait for EOF/termination.
- Set a bounded `Host.Options.SemanticInactivityTimeout`; this option and `HandshakeTimeout` are allowed by `HostExecutor.Probe`. Backend retry occurs only with confirmed cleanup and no semantic activity.
- Second process: complete the normal sequence. It should use `thread/start`, not the stalled thread's `thread/resume`.
- Verify two distinct actual attempt nonces/bindings and retained termination receipts, with the same handoff snapshot. Capture both actual turn inputs when both are submitted.

Do not copy the `agent` test's unexported global shutdown-timeout override into production. Give the host test a reasonable bounded timeout and let normal lifecycle cleanup run. An alternative two-process fixture is an initialization timeout followed by success; that tests nonce renewal but does not exercise post-thread resume continuity.

### Why this catches the reported hash gap

`codexTurnInput` runs after process startup and can prefix `ResumeContinuityNotice` when the live RPC falls back (`codex.go:1519`, `:1874`). An ordinary fake Claude stdin capture will not see that branch. The negotiated host path should already own its continuity guidance and pass an empty backend notice; this fixture proves the actual RPC text stayed equal to the bound host input despite resume rejection.

## Related specs and external references

- `.trellis/spec/server/collaboration-host-execution.md`: every physical launch is observed; use the ordinary backend leaf; lifecycle proof and provider output are different facts.
- Runtime task `design.md`: binding occurs after authorize, before process start, with an exact prompt hash; source text stays out of brief/ledger.
- No external documentation or package was needed. All referenced fixtures are repository code and use test-created executables.

## Caveats / Not Found

- This research did not implement or run the proposed fixture.
- Existing fake shell scripts in the `agent` package are POSIX-only and use non-importable test helpers. Prefer the current host test binary when the new regression should run cross-platform.
- The two-process retry depends on the backend's cleanup-supported platform and recognized diagnostic predicate. Keep its assertion separate from the simpler same-process live resume rejection.
