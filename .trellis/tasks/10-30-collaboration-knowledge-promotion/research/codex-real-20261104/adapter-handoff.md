# Codex knowledge adapter handoff

Worktree: `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104`. Source hashes: `adapter-source.json`. No real Agent/account/model or database operations. No commit.

## Changed files

- `server/pkg/agent/agent.go`: `ExecOptions.CodexEphemeral` (host-only fresh nonpersistent policy).
- `server/pkg/agent/knowledge_launch.go`: built-in local Codex admission, initially empty default/custom args only; explicit storage/config/profile/subcommand selectors rejected before unsupported fallback; provider-specific fingerprint branch preserves Claude hash.
- `server/pkg/agent/launch_lifecycle.go`: both early and actual physical-start knowledge gates admit Codex through the same builtin-local contract.
- `server/pkg/agent/codex_knowledge.go`: exact config/invocation fingerprints, instruction/config/account environment bindings, ordinary lifecycle admission and explicit thread persistence/UUID response validation.
- `server/pkg/agent/codex.go`: production JSON-RPC path authorizes process, obtains raw native identity, synchronously ACKs via Source=system before input/pin; no knowledge retry/resume fallback/continuity mutation. Host sends ephemeral=true and validates response. Knowledge failures retain private output/usage/termination separately; generic logs/errors exclude private stderr/RPC bodies. Existing MCP materialization shares a pure renderer with fingerprint computation to avoid byte drift.
- `server/pkg/agent/codex_knowledge_test.go`: native fake app-server subprocess tests for denied starts, fresh/exact resume, missing/mismatched IDs, failed ACK, failed resume, missing/ignored ephemeral or persisted path, private diagnostics, initialize no-retry, changed config/args/continuity, terminal refusal, selectors and credential/instruction binding.

## Interfaces for integration

- `KnowledgeCodexConfigFingerprint(env map[string]string) (string,error)` requires absolute CODEX_HOME. Hashes config.toml/config.json/auth.json/instructions.md/AGENTS.md/AGENTS.override.md, referenced model_instructions_file/experimental_instructions_file, configured env_key/env_http_headers credential values, fixed API environment sources. Explicit env map wins over inherited env. Hash only is returned; unreadable referenced instructions/config refuse.
- `KnowledgeInvocationFingerprint("codex", cfg, opts)` requires absolute CODEX_HOME and Cwd. Uses effective managed MCP content plus model/thinking/tier/native args/persistence/executable/CLI and project ancestor instructions/config. Claude fingerprint unchanged.
- Ordinary opts.Knowledge requires ordinary lifecycle and persisted thread response (`ephemeral:false`); Source=system ACK before turn/start. Codex raw ID must be a canonical UUID.
- Host opts.Knowledge remains nil; opts.CodexEphemeral=true, Config.PrivateInput=true, builtin runtime; no resume/default/custom args. Response ephemeral=true and path=null required. Legacy events cannot certify strict managed execution; authoritative matching raw turn/completed required.
- Nonknowledge Codex retry/fallback is retained. PrivateInput error redaction also protects existing host-private calls.

## Verification

- Initial targeted existing baseline: `go test ./pkg/agent -run 'Test(Knowledge|CodexReasoning|BuildCodexArgs)' -count=1` passed.
- Subsequent test invocations use `bash scripts/go-test-with-agent-cli-guard.sh go -C server test ...`; no ambient Agent invocation.
- Complete `TestKnowledgeCodex` normal final source: pass, 2.763s.
- Complete `TestKnowledgeCodex` race after diagnostic redaction: pass, 23.200s (`adapter-race.log`). Final credential env source change adds a pure helper/test, checked by targeted race separately.
- Broad relevant existing Codex/Knowledge/lifecycle/args/MCP suite normal with `-parallel=1`: pass,119.934s (`adapter-normal-serial.log`).
- Earlier broad parallel attempt had two existing cleanup timing failures; original `adapter-normal.log` retained. Exact unchanged timing tests passed serially in4.484s (`adapter-timing-recheck.log`); no timeout/threshold relaxation.
- `go vet ./pkg/agent` passed; final repeat after credential helper passed. gofmt and git diff --check passed.

## Limitations / next verification

Integration B/C and independent main review still own daemon/server/host wiring and full fake proof. Installed CLI runtime semantics and real account connectivity remain unverified. Ephemeral response/absence of path is required; full real nonpersistence proof belongs to host acceptance. Trusted operator configuration is bound rather than silently stripped; final real experiment config must be isolated by coordinator. Source config fingerprints bind current bytes, not arbitrary external mutations during a running process. General untrusted host filesystem security is outside this change. No model version rollout is claimed by static schema or fake tests.

Final config/instruction/selector/fingerprint race control: ok  	github.com/multica-ai/multica/server/pkg/agent	1.853s

## Independent review A1/A2 repair

Reviewer identified that a same-thread stale turn could complete a new input because turn/start response identity was ignored; unsupported status was logged verbatim. Both issues reproduced before edits with seven failing controls (`adapter-review-red.log`).

Strict execution now buffers early raw events without publishing output/usage/errors/completion; the exact matching turn/start RPC response binds turn.id before the stdout reader releases only matching thread/turn events. Missing/invalid response turn.id, unmatched/missing RPC response IDs, RPC errors with queued successful notifications, and buffer exhaustion cannot establish successful execution. Buffer is bounded to256 events/4MiB; overflow fails closed. Provenance is checked before any legacy-gate mutation; nonknowledge notification behavior remains unchanged.

Unknown private terminal status is normalized for generic logs, with details retained only in private result evidence. Private turn identifiers in diagnostics are hashed; unexpected server method names, permission-field names and semantic item descriptions no longer leak arbitrary provider values to generic logs. Correct early notifications still complete after response binding. Added12 full subprocess correlation cases and a bounded-buffer byte control.

Final relevant normal/strict + existing response/notification tests: ok  	github.com/multica-ai/multica/server/pkg/agent	5.482s
Final matching race: ok  	github.com/multica-ai/multica/server/pkg/agent	47.416s
`go vet ./pkg/agent` and `git diff --check -- server/pkg/agent` exit0. Final source hashes updated; prior manifest preserved in adapter-source-before-a1-a2.json. No broader repeat suite or paid execution.

## A2-R residual turn identity logs

Independent recheck found two remaining raw c.turnID generic log fields. Added success and semantic-inactivity subprocess sentinel-turn controls, both failed before correction (`adapter-a2r-red.log`). Both fields now use existing codexDiagnosticID; all generic turn_id log fields use that helper. No product behavior/authorization changes. Normal privacy+turn correlation: ok  	github.com/multica-ai/multica/server/pkg/agent	4.171s. Narrow privacy race: ok  	github.com/multica-ai/multica/server/pkg/agent	8.417s. Vet/diff-check exit0. New hashes in adapter-source.json; prior frozen manifest preserved as adapter-source-before-a2r.json. Edits occurred after C normal02 completed/released its source window.
