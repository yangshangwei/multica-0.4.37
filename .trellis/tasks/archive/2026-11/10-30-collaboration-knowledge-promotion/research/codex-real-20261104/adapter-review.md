# Independent A adapter review

- Date: 2026-11-05 Asia/Shanghai.
- Worktree: `/Volumes/artisan/code/2026/mc-knowledge-codex-20261104`.
- Reviewed source: all six files in `adapter-source.json`, including the new `codex_knowledge.go` and tests; callers and docs inspected only to assess adapter contracts.
- Verdict: **CHANGES REQUIRED — two blocking findings.** The main session received both findings during review. No product edits were made.

## Findings fixed

None. Only this report was written; the active author owns repairs.

## Findings not fixed

### A1 — Strict knowledge completion is not bound to the turn/start response (P1)

Location: `server/pkg/agent/codex.go:1652`, `:2421`, `:2853`, `:3354`.

The adapter arms the notification gate and calls `c.request(..., "turn/start", ...)`, but discards the response body. In strict knowledge mode, any same-thread `turn/started` notification supplies `c.turnID`; a subsequent matching `turn/completed` is then accepted. The added strict check compares one notification against another, not either notification against the actual turn created for this input.

A reproducible source-level sequence is:

1. The adapter sends a new knowledge-bearing `turn/start` for thread S.
2. The RPC returns `turn.id = T_new` (or returns an error after notifications arrive).
3. A stale or unsolicited same-thread `turn/started` arrives with `turn.id = T_old`, followed by a final-answer item and `turn/completed` for T_old.
4. `codexTurnNotificationGate.accept` stores T_old, `handleRawNotification` stores T_old again, and the strict completion comparison passes. The adapter can publish a completed result for the wrong turn. If the RPC returns an error after that completion is queued, the error branch consumes `turnDone` and can likewise treat the result as completed.

There is also a state-mutation ordering issue: `handleNotification` calls the current-turn gate before the strict required-thread check in `handleRawNotification`. A notification missing `threadId` can mutate gate state even though it is rejected later. Strict knowledge should reject invalid provenance before any current-turn state mutation.

Required fix: correlate strict execution with the canonical thread and the exact nonempty `turn.id` returned by the `turn/start` RPC. Since valid notifications may precede the response, retain a bounded pending notification set or equivalent evidence and release only the events matching the returned turn. Missing/error RPC results cannot establish successful execution from an unrelated notification pair. Preserve the existing permissive legacy behavior only for nonknowledge execution.

Required regression controls:

- RPC T_new with pre-response or post-response started/item/completed for T_old never publishes old output or success.
- Correct T_new notifications arriving before the RPC response still complete after exact correlation.
- Missing/invalid response turn ID refuses; a request error plus queued completion cannot become success.
- A notification missing thread identity cannot poison the gate; wrong-thread and wrong-turn items/errors/completion do not affect current output or termination.
- Strict legacy `codex/event` remains excluded; ordinary nonknowledge compatibility remains unchanged.

This finding is grounded in the current source flow. No new diagnostic test was written or run by this reviewer.

### A2 — Unsupported terminal status is still logged verbatim in private execution (P1)

Location: `server/pkg/agent/codex.go:3367–3372`.

For a strict private `turn/completed`, an unknown `turn.status` correctly sets a content-free terminal error. Immediately afterward, the adapter logs the original unvalidated `status` string through `c.cfg.Logger.Info`. A malformed provider event with `status = "private-sentinel"` therefore leaks that provider-controlled value into generic runtime logs even though the public result is sanitized.

Required fix: for private execution, validate/normalize status before logging and use a fixed diagnostic category or byte count for unsupported values. Apply the same policy to other raw diagnostic fields accepted from malformed server messages; do not rely on the private `Result` sanitizer to protect logs written earlier.

Required regression: a matching thread/turn completion with an unsupported status containing a private sentinel returns a content-free failure, retains useful private evidence where appropriate, and emits no sentinel in generic logs. Existing `TestKnowledgeCodexThreadAdmission/private_error` only tests the normal `error.message`/stderr path with `status="failed"`; it does not cover this leak.

## Contracts reviewed without additional blocking findings

- Physical launch still passes through the existing lifecycle gate. Ordinary knowledge obtains the raw canonical thread ID and synchronously invokes the native observation callback before `turn/start`. Failed ACK and wrong resumed native ID prevent input delivery. Fresh IDs are bound through the existing `Source="system"` category without wire-shape changes.
- The knowledge input is sent as the prepared prompt without the adapter adding a continuity notice. Knowledge resume errors do not fall back to `thread/start`; both automatic initialize and model-catalog retry paths are suppressed for ordinary knowledge and ephemeral Host runs.
- Ordinary thread responses require explicit `ephemeral:false`; Host requires explicit `ephemeral:true` and no persisted path. Host mode refuses resume/custom/default arguments. This is response enforcement, not proof of installed-vendor nonpersistence or platform readiness.
- The private result boundary retains typed knowledge failures, prior output/diagnostics in `KnowledgeOutput`, usage and process cleanup evidence. Direct asynchronous `error.message` and ordinary message/semantic logging paths have been guarded. A2 demonstrates a remaining diagnostic path rather than absence of the whole privacy boundary.
- The MCP renderer is shared by actual materialization and fingerprint normalization, so prepared and materialized config use the same transformation. Fingerprints bind structured model/thinking/tier, built arguments, cwd, persistence, executable/version, config bytes and ancestor config/instruction sources. Unknown custom/default Codex arguments are unsupported rather than silently admitted.
- **B CROSS-1 is addressed in this frozen source:** `codexKnowledgeInstructionSources` now hashes configured `env_key` and `env_http_headers` resolved values, with explicit environment overrides taking precedence over inherited values. The added test changes both credential sources and checks override precedence. Traversing all configured providers rather than only the active one is conservative and can retire a binding after an inactive-provider credential change; it does not grant broader access.
- Claude fingerprint serialization and ordinary nonknowledge recovery remain unchanged in the examined diff.

## Documentation review

The operator guide, built-in issue skill/reference/source map, and knowledge execution spec consistently describe local Claude/Codex support, default-off rollout, exact ordinary thread ACK, ephemeral Host policy and refusal without replay. They do not claim that the local Codex account, real model or installed ephemeral behavior has already passed. The experiment spec distinguishes 15 physical measured executions from model HTTP requests and three fake seeds.

The current claim that strict execution requires an authoritative matching terminal completion is the intended contract, but A1 means the implementation has not yet established its current-turn part. Keep the docs as the target contract and repair/reverify before describing this source as accepted. The operator guide's phrase “Collaboration private runs require official print-mode” follows the Claude introduction but could be made explicitly “Claude collaboration private runs” for clarity; this is a nonblocking editorial note.

## Verification and limits

- All six file hashes matched `adapter-source.json` during the initial review, including `codex.go=dfbc7da8e4b151c61fb6a41bd6e2b962ea060ccd8e7ef3e5f4721842ec5b1d73` and `codex_knowledge.go=e73ae8859a6b0b1f9262b2abdc58d1d78ede2f18d0d6fb015981bbf597dab876`.
- The full new test file was inspected. It covers start denial, exact native ACK, missing/mismatched native IDs, resume refusal, ephemeral rejection, private error/stderr, no initialize retry, config drift, termination refusal, selectors, instruction-file changes and environment credentials. It has no current-turn RPC correlation controls or unknown-status privacy control.
- Lint/type-check/tests: not rerun during this read-only review while C's integrated test lane was active. The writer's saved handoff reports passing tests and vet, but those claims do not disprove A1/A2 and are not fresh independent verification by this reviewer.
- No installed Agent, real model, database, credential or global configuration calls. No commits, source edits or test fixture additions.
- Next: A owner repairs A1/A2 with failing regression controls first, freezes new hashes, runs the narrow normal/race controls and existing relevant regression suite as justified, then requests independent recheck. C must bind its final integrated fake evidence to the repaired source.
