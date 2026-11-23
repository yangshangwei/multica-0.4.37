# Research: Codex compatibility for real knowledge acceptance

- Query: Can the completed knowledge fake framework run the planned E0b/K1/K2/K3 real acceptance with the user's local Codex, and what is the smallest correct implementation boundary?
- Scope: internal, read-only source/spec analysis; no tests, database operations, Agent executions, credential reads or git operations
- Date: 2026-11-04

## Findings

**The current product supports managed knowledge only for built-in local Claude. Codex is supported for ordinary/host execution in general, but not for managed-knowledge negotiation or exact-session exposure. Using Codex for the full acceptance requires a product adapter extension as well as harness work. Merely replacing the executable, or disguising Codex as Claude, would not verify the intended product.**

The user now authorizes local Codex execution under the proposed 15 measured-launch plan. This authorization changes the operational approval, but does not make the existing Claude-only implementation compatible. The original fake proof remains valid for its original source and provider; it cannot certify the new Codex path.

### Existing reusable seams

- `server/pkg/agent/codex.go:324`: built-in Codex backend uses `codex app-server --listen stdio://` with JSON-RPC, not `codex exec` and not Claude stream-json.
- `server/pkg/agent/launch_lifecycle.go:153`: generic host lifecycle already supports local Claude/Codex/OpenCode, including observed physical process lifecycle. This is intentionally a broader allowlist than knowledge support.
- `server/pkg/agent/knowledge_launch.go:294`: `observedLaunch.observeNativeSession` already binds raw native IDs to the exact physical attempt, requires synchronous observation ACK, and refuses mismatched identities without transferring old exposure. It can be reused once Codex supplies correct raw observations.
- `server/internal/daemon/knowledge_context.go:327`: ordinary preparation already resolves the actual claim, prepares the bundle, hashes the complete prompt and wires `NewOrdinaryLaunchLifecycle`. Existing server services own source authorization, revision/exposure checks, and fences.
- `server/internal/daemon/collaborationrun/host_knowledge.go:72`: host prepare/authorize already uses actual run identity, source-qualified contributions, prompt hash and original attempt nonce. Retain these rather than inventing an alternative injection endpoint.
- `server/internal/daemon/collaborationrun/host_preparation.go:309`: generic Codex host preparation supplies an isolated `CODEX_HOME`; host spec describes existing account/config helpers. Session-home isolation alone does not satisfy the knowledge non-persistence contract.

### Product blockers and why they matter

| Boundary | Current source | Required Codex work |
| --- | --- | --- |
| Provider admission | `knowledge_launch.go:95` returns true only for built-in local Claude; argument validation at `:170` also refuses every other provider | Explicit provider-specific Codex admission with controlled arguments, account/config namespace and exact effective invocation. Do not just broaden the allowlist. |
| Invocation fingerprint | `knowledge_launch.go:208` hashes Claude-oriented options/settings; `withKnowledgeClaudeLifecycle` at `:270` checks actual invocation against the expected fingerprint | Include Codex's actual RPC/argv behavior, service tier, managed config and persistence mode where relevant; verify at the real adapter boundary. |
| Lifecycle gate | `launch_lifecycle.go:167` additionally requires `provider == "claude"` for ordinary knowledge | Admit Codex only when its corresponding fingerprint/session checks and synchronous native observation are implemented. |
| Claim/history validation | `service/task_knowledge.go:95,353,626` and `task_knowledge_prepare.go:394` hardcode Claude; supported invocation at `task_knowledge.go:317` calls the provider allowlist | Update all matching retained/new claim validation paths consistently; preserve history and unsupported-client protections, not just fresh claims. |
| Daemon configuration identity | `daemon/knowledge_context.go:85–96` fingerprints Claude args and Claude/cloud environment keys; `knowledgeInvocation` at `:100` calls the restrictive allowlist | Add Codex's real configuration/account-home identity rather than binding a Codex process to Claude configuration provenance. Do not copy credential values into evidence. |
| Native session evidence | Codex `Execute` at `codex.go:902` only calls generic lifecycle; thread creation at `:1512` proceeds directly to turn input without the knowledge observer | Feed raw thread-start/resume identity into the exact process observer and require positive server ACK before pin/publication/turn input. Preserve mismatch and missing-ACK failures. |
| Resume semantics | `codex.go:1882–1943` falls back from rejected/missing-ID resume to `thread/start`; `Execute` at `:961–1000` can retry model-catalog failures after clearing the resume pointer | Knowledge-bound execution must not silently replace the exact session or replay input. Keep ordinary legacy recovery unchanged outside knowledge mode. |
| Host positive negotiation | `collaborationrun/host_knowledge.go:14,24,65` and `service/collaboration_knowledge_runtime.go:65,72,354` all constrain provider to Claude | Add Codex support to both sides only after establishing its host persistence and option contract. |
| Host persistence | `host_runner.go:195–201` injects Claude `--no-session-persistence`; `host_knowledge.go:65` requires that exact flag in the fingerprint; `host_runner.go:265` clears native pointers | Implement a verified equivalent Codex app-server nonpersistent-thread policy before K3 root/member/continuation. A different cwd or clearing the Multica pointer alone is insufficient. |

Codex currently starts threads with `persistExtendedHistory: true` (`codex.go:1952–1966`). This field is not evidence of an ephemeral/nonpersistent mode. The installed app-server schema and current official documentation must establish the supported nonpersistent control; no such control was verified in this source-only research.

### Harness blockers

- `server/cmd/server/rk1k_entry_test.go:38`: real admission always fails; the real test body only checks admission and does not dispatch a real scenario after it.
- `rk1k_provider_test.go:121–124`: pass-through mode admits only the exact test-created Claude stand-in. At `:149–168` it captures the first Claude input frame and forwards Claude argv; at `:170` even the stand-in is counted as fake. Real Codex needs JSON-RPC transcript capture, correct physical-start accounting and a reviewed executable binding.
- `rk1k_evidence_test.go:126`: native/result observations parse Claude `type/system/init`, `type/result`, `session_id`. Codex needs its own raw RPC identity and completed-turn proof; a translated pretend-Claude result would hide the adapter under test.
- `rk1k_fixture_test.go:84,164–165,369,383`: fixture home, runtime/host registration and daemon agents are Claude-specific. The production producer/approval/source fixtures are reusable, but provider/session storage must be real Codex.
- `server/internal/daemon/rk1k_driver_agentintegration.go:34,52,69`: the thin driver pins Claude in configuration, runtime index and runTask. Parameterize the explicit supported provider while preserving actual claim/run/report behavior.
- `rk1k_scenarios_test.go:358` implements only the fake proof schedule. Shared producers and semantic assertions can be reused, but real dispatch, private transcript material, account/model/version evidence, stopped-cursor bookkeeping, and no unexpected later starts still need implementation.

### Smallest defensible delivery plan

1. Treat this as an explicit **Codex knowledge provider extension** within the continuation design. Freeze the existing fake proof and preserve the original runtime framework/ledger. Record this changed boundary before editing product files.
2. Implement ordinary Codex support as one coherent slice: admission/args/fingerprint, daemon configuration identity, server retained/current claim checks, adapter native ACK and strict no-fallback knowledge behavior. Use fake app-server tests first, including mismatched/missing IDs, rejected resume, missing observation ACK, changed config and denial before launch.
3. Establish the Codex host non-persistence contract against the installed app-server schema; then implement both host/service admission and actual thread policy. Reuse existing Codex host handoff tests and knowledge authorization/namespace tests. Unsupported versions/options must refuse without fallback.
4. Extend the tagged six-file harness plus thin driver with a real Codex RPC capture path and provider-specific fixtures. Keep seed starts fake and label them separately. Prove actual ordinary and host producer paths using fake Codex before any model launch.
5. Independently review the full cross-layer diff and run affected non-Agent tests, dedicated-database knowledge integration tests and race checks. Product/source has changed, so bind new evidence rather than simply relabeling the previous 49-pass report.
6. Only then execute the frozen 15 measured calls: E0b1, K1 four, K2 two, K3 eight, plus three explicitly fake setup starts. Charge every real physical attempt, including incomplete ones, and preserve stop-on-incomplete. The experiment should use the production Codex backend; a standalone `codex exec` reading an exported bundle can at most prove a separately labelled semantic observation, not K2 session security or K3 production integration.

This crosses agent adapter, ordinary daemon, service claim/history, host preparation, and tagged integration harness. It is not a small six-file test-only modification. A schema migration is not obviously required: current wire provider fields are strings and session bindings are provider-namespaced. A protocol-version change still requires explicit review if field shapes or promised semantics are altered.

## Files found

- `server/pkg/agent/knowledge_launch.go`: restrictive knowledge policy, fingerprint and raw-native observation helper.
- `server/pkg/agent/launch_lifecycle.go`: generic physical lifecycle and knowledge-specific admission.
- `server/pkg/agent/codex.go`: production JSON-RPC adapter, native threads, ordinary retries/fallback.
- `server/pkg/agent/claude.go`: existing knowledge lifecycle/observation/result handling to preserve as behavior reference.
- `server/internal/service/task_knowledge.go`, `task_knowledge_prepare.go`: claim metadata, supported invocation, retained history and preparation authorization.
- `server/internal/service/collaboration_knowledge_runtime.go`: server-side host knowledge negotiation and revalidation.
- `server/internal/daemon/knowledge_context.go`: configuration namespace, prepare, prompt and observation wiring.
- `server/internal/daemon/collaborationrun/host_knowledge.go`, `host_runner.go`: host knowledge admission, fingerprint, persistence and launch authorization.
- `server/cmd/server/rk1k_{entry,provider,fixture,scenarios,evidence}_test.go`: completed Claude-shaped fake-only framework.
- `server/internal/daemon/rk1k_driver_agentintegration.go`: tagged ordinary driver, currently Claude-pinned.
- `server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go`: existing non-knowledge Codex RPC launch test seam.

## Related specs

- `.trellis/spec/server/collaboration-knowledge-execution.md:204–221`: built-in local Claude only; host non-persistence and ordinary resumable policy are intentionally different.
- `.trellis/spec/server/collaboration-host-execution.md:86`: Codex account/config helpers with a collaboration-local session home.
- `.trellis/spec/server/collaboration-context-experiments.md`: actual execution and model behavior need separate evidence; fake results cannot certify a real model.
- `.trellis/workflow.md`: update design when execution reveals a wider behavior gap; implementation/check agents own product changes.

## External references

None fetched for this source-only research. Parent should verify current official Codex app-server schema/documentation for thread non-persistence and the installed CLI before choosing its precise policy. This report does not assume `codex exec --ephemeral` is an app-server flag or can replace production RPC execution.

## Caveats / Not Found

- No real Agent/model, CLI discovery, account lookup, credential access, tests, database operations or git operations were performed.
- No evidence was found that Codex already supports managed knowledge in product code. Generic Codex host support is present and reusable.
- No claim is made about the user's account/login availability; parent owns local environment verification.
- Existing passing reports belong to the prior Claude-backed fake path. Old failures and database artifacts remain untouched.
