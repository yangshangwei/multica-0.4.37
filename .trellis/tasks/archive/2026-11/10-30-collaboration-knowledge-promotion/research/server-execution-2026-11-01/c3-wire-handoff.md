# C2 → C3 wire and invocation handoff

Server implementation is still under test/review. No C3 files were edited. `TaskService.KnowledgeContextEnabled` is default false and has no production enabling configuration in this batch. Historical-session safety stays active independently of that gate.

## Claim capability and descriptor

`protocol.TaskKnowledgeInvocation` is the per-runtime capability declaration:

```json
{"protocol_version":1,"provider":"claude","mode":"local","builtin":true,"configuration_id":"opaque stable local runtime namespace","extra_args":[]}
```

Only actual built-in local Claude is supported. Custom profile runtimes, other families/modes and PAT/JWT/cloud fallback credentials do not negotiate private knowledge. The daemon should advertise only when it can satisfy C1; the server also checks its runtime/agent facts and default arguments. Agent custom args are read from the server's agent row; there is no client `custom_args` capability field.

- Per-runtime HTTP `POST /api/daemon/runtimes/{runtimeId}/tasks/claim`: optional body `{ "knowledge_context": <capability> }`.
- Batch HTTP `POST /api/daemon/tasks/claim` and its existing `/api/daemon/claim` alias: existing daemon/runtime/max-task fields plus `"knowledge_context": { "<runtime UUID>": <capability> }`.
- WS `tasks.claim`: exactly the same batch body. Existing `rpcClaimTasks` forwards the body to the batch handler and retains the authenticated connection scope.
- Managed/scoped transport uses these same paths and bodies. New private endpoints require the existing current `mdt_` credential; no PAT fallback.

An admitted claim response has optional `task.knowledge_context` (`TaskKnowledgeClaimDescriptor`): `protocol_version`, `task_id`, `runtime_id`, decimal-string `claim_generation`, exact `dispatched_at`, `provider`, `mode`, `configuration_id`, `preparation_required`, and `session`. This descriptor is written by the existing finalization transaction before return, including non-comment tasks. It contains no candidate/source IDs, private hashes/counts, summary or body. `prior_session_id` is the post-check safe selection; omitted-version clients go through the same history safety check.

`session` (`TaskKnowledgeSessionBinding`) has `provider`, `runtime_id`, `agent_id`, `configuration_id`, `scope_kind` (`issue|chat|quick_create|autopilot`), `scope_id`, server-issued UUID `generation`, and optional `native_session_id`. An absent native ID means a fresh, not-yet-observed session. Scope generation is separate from decimal claim generation. Do not synthesize either or infer the native ID from a requested value/result fallback.

`configuration_id` is a bounded client-local opaque **stable runtime namespace**, not C1's exact per-task invocation fingerprint. It should change when the managed executable/provider/account-source/local-default configuration changes, but not on ordinary token refresh or task cwd creation. Server metadata separately retains a digest of current agent/runtime configuration (owner, provider/mode/profile, instructions, runtime config/custom args/env/MCP/model/thinking/etc). That digest is private metadata and is not the claim field. The server compares it again and starts fresh for incompatible historical configuration.

Private claim metadata also retains the actual runtime owner and daemon identity. Legacy/unsupported selection checks these provider namespaces across task scopes and runtime rewrites; a different custom runtime UUID is not proof that its shared Provider history is empty. Valid empty/claimed-only histories remain distinct from private exposure. Ordinary v1 invocation rejects `--no-session-persistence`, even though the shared leaf now permits it for C4's explicitly non-reusable collaboration policy.

## After finalized claim, before disclosure

Build the real ordinary execution environment and all argv-affecting options first: actual cwd, model/thinking, default/custom args, MCP data and settings path. Compute `agent.KnowledgeInvocationFingerprint(provider,cfg,opts)` on those real values. Do not invent a cwd or call a static capability digest an exact invocation fingerprint. If that validation rejects, settle the current input as a typed refusal; do not prepare private text or strip user args.

`POST /api/daemon/tasks/{taskId}/knowledge-context/prepare` takes:

```json
{"protocol_version":1,"request_id":"UUID","claim_generation":"1","dispatched_at":"exact claim value","session":{},"invocation_fingerprint":"64 lowercase hex"}
```

Use the descriptor's complete `session`. Response (`TaskKnowledgePrepareResponse`): `protocol_version`, `prepare_id`, `invocation_fingerprint`, `manifest`, `manifest_sha256`, `contributions`, `targets`, `session`. Manifest entries contain candidate/version/hash/source/target identity; contribution DTOs are the existing collaboration knowledge types. `targets` contains target and `knowledge_revision`. All private material is disclosed only after the independent authorization transaction commits. Full response material/wire budgets include metadata and the newline. Same-key replay reauthorizes the original manifest; it cannot substitute a new bundle.

Keep the exact response for this logical input. Append its contributions only to the per-turn prompt, using the common BuildPrompt seam, never stable runtime brief/project/workspace files. Finish the actual prompt and its SHA256. C1 options use `NewOrdinaryLaunchLifecycle`, `KnowledgeLaunchIntent { ConfigurationID, InvocationFingerprint, Session {Kind,Generation,NativeSessionID} }`; kind is fresh/resume according to the admitted native ID, and `opts.ResumeSessionID` must match it.

## Per-command authorization and observations

In `BeforeLaunch`, bind the exact C1 `LaunchInfo.Attempt {Sequence,Nonce}` and copied intent to the finalized task/prepare identity. `POST .../knowledge-context/authorize` takes: `protocol_version`, stable `request_id`, `prepare_id`, `claim_generation`, `dispatched_at`, `manifest_sha256`, `prompt_sha256`, `invocation_fingerprint`, `attempt_seq`, `attempt_nonce`, `session`. It returns the same admitted attempt/digests/session in `TaskKnowledgeAuthorization`. Replay an uncertain request unchanged; never generate a new authorization identity to evade refusal. Current implementation admits the exact prepared session, never silently creates a new native scope for a replayed input.

`POST .../knowledge-context/observations` takes `TaskKnowledgeObservation`: common `protocol_version`, stable `request_id`, `prepare_id`, `claim_generation`, `dispatched_at`, `attempt_seq`, `attempt_nonce`, then:

- `kind:"started"`: `process_id`, `process_instance_id` from C1's actual process observation.
- `kind:"native_session"`: raw `native_session_id`, `source:"system"|"result"` from C1's direct callback. The server compares against the admitted native selection; mismatch is retained then refused, not relabeled as the old session's exposure.
- `kind:"terminated"`: `termination_kind:"not_started"|"managed_launch_ended"|"unknown"`, `wait_observed` and, when known, the original process fields. Process start/end remains distinct from native binding and business result.

Successful observation replies `{ "accepted": true }`. Bind native observation synchronously before exposing/pinning that ID. The endpoint writes exposure metadata and the ordinary task pointer in one transaction. Ordinary pin/complete/fail/cancel pointer writers check admitted bindings; a status event alone cannot authorize a new pointer. Original admitted observation cleanup remains available when the feature gate closes, subject to its authenticated task/runtime scope.

Decode acknowledgments with strict `protocol.TaskKnowledgeObservationAck`; only `accepted: true` is a successful acknowledgment. Missing/null, case-aliased, duplicated and extra fields are rejected.

## Ordinary mutation claim fence

When a descriptor exists, send `knowledge_fence` on StartTask, PinTaskSession, CompleteTask and FailTask, including all early or zero-native-ID failures:

```json
{"knowledge_fence":{"claim_generation":"exact descriptor value","dispatched_at":"exact descriptor value"}}
```

Shared type: `protocol.TaskKnowledgeFence`. The server compares it under each writer's existing task lock. A supplied stale fence is rejected even if current negotiation was downgraded; a missing fence cannot reinterpret retained negotiated history as legacy. Such older clients are refused at claim when they cannot safely finish the reclaimed row. Completion's context-exhaustion failure reroute retains the fence. Fence metadata is stripped from the public stored business result.

## Failure and retry

Errors use the existing `{ "error": "<code>" }` shape. Canonical nonautomatic-retry reasons: `knowledge_context_changed`, `knowledge_authorization_denied`, `knowledge_unavailable`, `knowledge_busy`, `knowledge_invocation_unsupported`. HTTP status is 409/403/503 according to the condition; malformed requests are 400. Bounded uncertainty retries reuse the original request and hashes. Exhaustion/refusal settles the current input; do not use dispatched-only requeue, fresh-session heuristics, base-only downgrade or task auto-retry.

C1 Execute errors retain `KnowledgeLaunchError`; running results retain `KnowledgeFailure`. Map these to existing ordinary FailTask `failure_reason` and the content-free error code. `KnowledgeOutput` is internal observed failure evidence: retain it through the transcript/evidence path only, deduplicating already recorded text. Do not put it in successful `Result.Output` or these metadata-only observation DTOs. Preserve prior usage, process and transcript facts if a later attempt is refused.

Remaining C2 tests/review and C3 fake-executable integration must pass before any enabling configuration is wired. No actual Provider/model understanding or broad platform support is asserted by this handoff.

## Separate C4 collaboration execution seam

AC6 also requires actual collaboration-run consumption. C2's ordinary task IDs, four ordinary scope kinds and task-token/daemon-token endpoints do not cover a collaboration run; C3 ordinary wiring will not complete that surface. A2 already resolves `Audience: collaboration`, but a separate owner must connect the existing run-scoped claim/handoff authorization and `collaborationrun.HostExecutor` input path.

C4 needs run-scoped knowledge negotiation, a bounded private manifest tied to the existing frozen handoff, exact actual prompt binding, current source/target/consumer revalidation before every physical launch, and explicit session/downgrade safety for both negotiating and older clients. A narrow initial option is an explicitly isolated **fresh namespace per knowledge run**, with no reuse of its Provider session and an effective argument policy proving fresh selection. That requires actual claim/argv/namespace/fallback tests; blanking one stored native ID is not enough. Stable handoff/source identity must survive retries without substituting a new knowledge snapshot or replaying an input after refusal.

Reuse the existing collaboration run business/termination capabilities and per-launch permit/receipt path. Keep `NewLaunchLifecycle` and its collaboration credential filter. Do not attach the ordinary knowledge constructor or ordinary task observer to a collaboration run: those are different authorization and environment contracts. If future C4 permits native-session reuse, it must add a truthful collaboration-compatible native binding contract; C1's current knowledge option is intentionally ordinary-only.

This is an outstanding required integration slice, not a declaration that collaboration is generally unsupported or that G2/AC6 is complete. Main will assign C4 after the ordinary server interfaces stabilize.
