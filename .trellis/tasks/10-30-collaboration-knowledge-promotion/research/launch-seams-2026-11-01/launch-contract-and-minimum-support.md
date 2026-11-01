# Research: Physical launch and native-session contract for G2

- Query: Can existing ExecOptions/LaunchObserver truthfully bind every physical launch and actual native session, including retries; what is the smallest supportable increment?
- Scope: internal source plus an unsuccessful primary-document lookup; no Provider calls.
- Date: 2026-11-01
- Checkout: `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`.

## Findings

### Existing lifecycle is a process contract, not native-session identity

`agent.ExecOptions.Lifecycle` (`server/pkg/agent/agent.go:29`) is currently collaboration-only. `LaunchInfo` contains **Provider, Kind only** (`launch_lifecycle.go:42`). `BeforeLaunch` returns closures for Started/Terminated plus environment (:75–88). The lifecycle serializes physical starts, obtains permission synchronously before native creation, and observes original native handles/Wait (:268 onward). A process instance ID is not a Provider session ID.

`HostLifecycleSupported` (:134) allows only local/empty mode for claude, codex, opencode. This is an adapter allowlist, explicitly not OS/version rollout evidence. All other providers/modes are unsupported for this lifecycle today. Do not equate this existing allowlist with managed-knowledge support.

Critical environment incompatibility: `observedLaunch.start` calls `collaborationLaunchEnv(s.cmd.Environ())` before start and rejects callback replacement of non-collaboration MULTICA variables (at :337–355); the filter at the end of the file retains only five collaboration variables. Ordinary daemon execution supplies task-scoped `MULTICA_TOKEN` and identity/context in `daemon.go:7608–7645`. Passing an ordinary knowledge observer to the existing constructor unchanged strips those credentials. A new **explicit ordinary environment policy** must retain the exact already-built ordinary task credentials; observer-supplied environment must not substitute a PAT/daemon token or broaden authority. Preserve current collaboration constructor/filter behavior and its tests.

### Supported local adapters: actual observations

| Adapter | Physical launches / native selection | Current evidence loss | Initial knowledge decision |
| --- | --- | --- | --- |
| Claude local stream-json | `claude.go:42` installs lifecycle; one command per Execute. `buildClaudeArgs` at :735 passes requested `--resume` at :773. Parser sees raw native IDs on system/result at :248,256. | Status is sent through lossy `trySend`; final `resolveSessionID` at :888 intentionally erases rejected IDs. Neither message/result identifies its physical attempt. A successful differing ID is not rejected by `resumeWasRejected` because that helper checks failed status first (:865). | Smallest first supported combination **after** direct per-command observation callback and strict invocation checks. Unknown fresh ID remains non-reusable until real observation binds it. No support unchanged. |
| OpenCode local JSON | `opencode.go:49` installs lifecycle; one command per Execute; requested `--session` at :103. Scanner sees each `event.SessionID` at :410. | Scanner overwrites one string, sends step-start status without ID (:434), returns final ID only (:320). Multiple observed IDs are not retained or tied to a launch. | Leave unsupported initially. Later direct callback from scanner for every native-ID change is a bounded enhancement; never call current final-only result sufficient. |
| Codex local app-server | `codex.go:902` installs lifecycle once; Execute can call executeOnce twice (:909,924). First process initialization retry can retain requested resume. Catalog-refresh retry clears resume (:981–991). `startOrResumeThread` (:1889) can resume then start fresh **inside one already-running process**; actual thread chosen at :1512 before turn/start. | Early pins held/discarded at :928–966; context LaunchInfo does not change with retry opts. Same-process selection/fallback has no observer event. Returned native IDs can be lost on failed attempt. Prompt changes through `codexTurnInput` after selection (:1528). | Unsupported initially. Enable only after synchronous selected-thread callback before turn/start, complete per-process request/actual mapping, and internal retry/fallback tests. Do not report Codex supported merely because BeforeLaunch runs twice. |
| Other local adapters, gateways, remote/custom binaries | May return IDs via Message/Result, but `HostLifecycleSupported` rejects lifecycle. Custom protocol-family commands are selected at `daemon.go:6880–6916`. | No verified common physical-start/session contract. | Ordinary base execution continues when fresh can be proven; no private bundle and no managed-session resume. |

The safest initial rollout is the actual built-in local Claude executable with an admitted, tested invocation/configuration fingerprint. First support is conditional on implementing the contract below and passing fake-executable tests; this document does not certify a Provider version. Supporting OpenCode/Codex is a separate bounded increment, not a prerequisite to implement truthful unsupported behavior.

Do not manufacture a fresh native ID from task ID, process PID, generation UUID, or arbitrary latest task. The existing code does not pass fresh Claude `--session-id`. That flag could simplify future preselected native identity, but the official docs request to `https://code.claude.com/docs/en/cli-reference.md` returned HTTP 403 on this research date; its current/version-specific semantics were not independently verified here. A blacklist mentioning the flag is not evidence that the adapter already uses it. The minimum plan uses returned stream IDs and explicit unknown-fresh state.

### Minimal contract delta (proposed names, no code written)

1. Add an immutable **per-command session intent** to lifecycle context/LaunchInfo: `fresh` with server-issued scope generation and no native ID, or `resume` with full exact binding. Include opaque effective invocation/configuration identity. This must be computed from the opts actually used by that physical command, not copied once from outer Execute and left stale on internal retry.
2. Give each admitted physical command an opaque attempt handle (seq/nonce or equivalent) retained by its callbacks. Add a direct callback such as `NativeSessionObserved(NativeSessionObservation) error` on that handle, with actual native ID and whether this is a confirmed resume/new selection. Call from raw adapter observations, before lossy public status conversion. Never derive an observed session from requested arguments alone.
3. For Claude, system/result native IDs both reach that callback; duplicate equal IDs are idempotent, changed IDs are explicit facts. A mismatch cannot silently replace the binding. Persist requested S as possibly exposed and actual T as observed under that launch as warranted; do not label S's historical manifest as T's history. Stop/refuse further execution on an unapproved transition, and never publish an unbound pointer.
4. For future Codex support, call the selected-session callback after `thread/start`/`thread/resume` response and **before turn/start**. Model-catalog retry needs an explicit fresh selection/new generation. A callback after a streamed task_started pin is too late and can be suppressed by the outer retry loop. Reauthorize the new binding using the existing prepared manifest, without fetching a replacement snapshot or replaying an input after knowledge refusal.
5. Add explicit lifecycle environment policy: existing collaboration filtering stays default for collaboration; ordinary preserves the original task environment and forbids replacement credentials. Ordinary knowledge observer should not need Environment overrides at all.
6. Bridge observations to extended pin/terminal DTOs or a bounded typed observation subresource. Every report carries task/runtime/claim generation, prepare/authorization identity, attempt seq/nonce and scope generation. Service validates the admitted attempt before atomically binding metadata plus pointer. Started/Terminated are distinct durable facts; no fabricated network ACK or native observation.

No private body belongs in the lifecycle observer or its logs. It needs IDs/digests plus the fixed assembled prompt hash. An unknown fresh ID can be authorized as a generation, but cannot become resumable until a real native observation is stored with its exposure. If the observation callback or persistence fails, stop/seal and preserve uncertain exposure; do not turn it into an ordinary recoverable transport error.

### Fresh must be an effective invocation property

Current ordinary argument filters are insufficient:

- `claudeBlockedArgs` (`claude.go:719`) protects protocol flags and effort, but does not block `--continue`, `-c`, `--resume`, `-r`, `--session-id`, or `--fork-session`. Both ExtraArgs and CustomArgs are appended (:787–788).
- `opencodeBlockedArgs` (:36) lacks `--session`, `--continue`, `--attach` and related remote selectors; CustomArgs append after the controlled session option (:105).
- Daemon custom runtime profiles substitute path and fixed args under the same provider family (`daemon.go:6880–6916`); local defaults are appended separately (:7790 onward).
- Existing `collaborationrun/host_config.go:47–73` is a useful validation pattern: exact local provider, bounded args, and rejection of session/remote/cwd selectors. Reuse policy primitives where possible; do not import collaboration's broader authority or silently strip ordinary user intent.

For a known managed-exposed or unprovable negotiated scope/session, an unsupported/old client or ambiguous effective invocation must not resume via a custom/default flag after the server blanks PriorSessionID. Reject the ambiguous invocation rather than silently modifying user intent. Safe legacy sessions with proven no managed exposure retain existing behavior. Claims cannot validate unknown local argv by a provider string; negotiating clients must report a bounded source-free invocation capability/fingerprint and execution must match it. Arbitrary manual access to private Provider history is outside application-controlled resume, and this work does not erase such history.

### Failure and retry exclusions

`StartTask` already runs at `daemon.go:7569`, before final BuildPrompt and provider invocation. `executeAndDrain` at :8434 returns Execute errors directly (:8444–8452); runTask returns early at :7926. Ordinary initial error handling classifies failures and calls normal FailTask. Introduce typed knowledge refusal classification at this boundary, and preserve it through Result/string terminal serialization.

- `knowledge_context_changed`, `knowledge_authorization_denied`, and exhausted `knowledge_busy/unavailable` are terminal for this input; bounded transport retries reuse the same authorization identity only.
- `shouldRetryWithFreshSession` (:8277) must explicitly reject knowledge failure before ResumeRejected/error-text heuristics. Its current tool-count=0 guard is not enough.
- `reconcileFreshRetryResult` (:8379 onward) and caller :7981 can replace a failed second launch with the first result. Preserve a knowledge refusal and first-attempt usage/output/process facts instead of letting it become a retriable network/poison error.
- `TaskService.retryEligible` (:5347), `MaybeRetryFailedTask` (:5399), and FailTask's in-transaction retry use `retryableReasons`. Knowledge reasons must not enter that set. Ensure normalization preserves the dedicated reason. The orphan sweep must not override a durably settled knowledge refusal.
- `RequeueTaskAfterClaimFailure` (:3856; SQL :946) is dispatched-only, unstarted-claim recovery. Never call it for preparation/launch refusal after finalized delivery or StartTask.
- Codex's internal retry needs a refusal check if/when supported; lifecycle failure must remain sealed and no fresh session should be created to bypass it.

When refusal is on first launch, record zero physical starts and fail the existing task. If a later launch is refused, retain prior usage/transcript/termination and side effects. The user's explicit rerun is the existing way to request a new logical input; automatic base-only downgrade after private preparation is forbidden.

## Files found / related specs

- `server/pkg/agent/agent.go`, `launch_lifecycle.go` — options, observer, process observations and credential filtering.
- `server/pkg/agent/claude.go`, `opencode.go`, `codex.go` — actual native observations and retries.
- `server/internal/daemon/daemon.go`, `poisoned.go`; `server/pkg/taskfailure/failure.go` — drain, fallbacks, error classification.
- `server/internal/daemon/collaborationrun/host_config.go`, `host_runner.go` — bounded invocation validation and closure-based attempt registration patterns.
- `.trellis/spec/server/collaboration-host-execution.md` §2–3 — observed launch contract and unchanged ordinary behavior.
- `.trellis/spec/server/collaboration-startup-handoff.md` — exact input hashing; managed prompt must preinclude continuity notice and suppress adapter prefix duplication.

## Caveats / Not Found

- No currently supported adapter satisfies G2 unchanged; fake-input evidence would prove application delivery, not Provider understanding.
- No real binaries, authenticated accounts, model discovery or tests were executed. Official CLI documentation lookup failed as above; do not treat fresh `--session-id` as verified support.
- Current scope is a minimum contract, not a full adapter rewrite or expanded global plan. Parent owns rollout/enablement decisions.
