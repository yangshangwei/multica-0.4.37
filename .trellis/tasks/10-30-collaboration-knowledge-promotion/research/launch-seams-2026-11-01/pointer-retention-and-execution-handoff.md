# Research: Pointer retention and bounded G2 execution handoff

- Query: Locate all pointer writers and actual deletion hazards; specify minimal ownership slices, interfaces and tests for the approved ordinary-knowledge execution step.
- Scope: internal; research only.
- Date: 2026-11-01
- Checkout: `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`.

## Findings

### Pointer writers that must bind exposure atomically

| Writer | Current behavior | G2 requirement |
| --- | --- | --- |
| Mid-run pin | `handler/task_lifecycle.go:70` `PinTaskSession`; locks Chat before task, calls `UpdateAgentTaskSession` (:120), then `AdvanceCancelledChatSessionPointer` (:130), commits. SQL `agent.sql:1420` permits dispatched/running and cancelled-with-empty-slot. | Validate claim generation plus admitted attempt/session generation; bind knowledge metadata in this same transaction before the pointer can be observed. Plain task ID plus arbitrary session string is insufficient for negotiated tasks. |
| Complete | `service/task.go:4397` `CompleteTask` locks Chat then completes task (:4407), updates direct Chat pointer (:4433), clears exact retired pointer (:4448). | Integrate binding with both updates; final NULL task session must retain original resume exposure in metadata and must not imply empty history. |
| Fail | `service/task.go:4806` / `failTask`; task update :4893 preserves prior pin via SQL COALESCE. Clears poisoned/retired Chat pointers :4931/:4945, otherwise updates :4966. | Retain observed and pending-resume exposure even on failure; don't erase bindings when clearing a pointer. Preserve dedicated nonretryable knowledge reason. |
| Cancel / delayed pin after cancel | `service/task_cancellation.go:108–126` cancels then advances cancelled Chat pointer in the same transaction; late pin shares this logic. | Cancelled history is exposure history. Bind metadata before advancement; delayed observation must not overwrite a newer task/session generation. |
| Retry child | `queries/agent.sql:621–653` `CreateRetryTask` copies parent session/workdir (unless specifically unsafe), carries original input/authorization provenance. | Child NULL negotiation cannot reset prior proof; claim must validate selected history. Do not copy a preparation authorization as fresh permission. |
| Clear / retirement | `queries/chat.sql:320` clears exact session+runtime only; `retired_session_id` in complete/fail excludes old terminal rows in ordinary selectors. | Clearing one direct pointer is not retirement of every history source. Guard fallback/rerun separately; keep immutable exposure evidence. |

Daemon early pin is intentionally asynchronous and first-only: `daemon.go:8604` uses `sessionPinned.Swap(true)`, waits for Codex rollout, then posts session/workdir (:8615). It cannot serve as the new non-lossy observation channel. Negotiated executions need per-attempt observation + generation on the existing pin request (or a dedicated bounded observation request), and normal pointer exposure must wait for binding. Unnegotiated behavior can remain unchanged.

### Deletion/retention audit: avoid speculative storage redesign

Only two current production SQL statements explicitly delete queue rows: `DeleteTaskBatch` (`queries/workspace_delete.sql:197–227`) and `DeleteUnstartedQuickCreateRetryTask` (`queries/agent.sql:696–707`). Additional legacy parent FKs can cascade task deletion; the current handlers must be considered with them.

| Path | Actual lifetime behavior | Safe implication / necessary work |
| --- | --- | --- |
| Queue clear/remove/edit | `handler/chat.go:1734`; `service/task.go:2923`; `task_cancellation.go:82–100` cancel rows and settle owned input. | They do not delete knowledge_delivery. No retention marker needed. Cancelled rows must remain in cumulative lookup. |
| Chat archive | Existing archive path cancels active work; Chat row/pointer/history retained. | Treat as retained exposure; no history deletion. |
| Ordinary Chat hard delete | `handler/chat.go:677–777` locks Chat, cancels tasks, removes bindings/drafts then deletes Chat. Existing migration 033 line 35 gives queue.chat_session_id ON DELETE SET NULL. | Direct reusable Chat pointer is deleted; task history normally remains. Store original scope in metadata, because nullable row link is not durable identity. Deleted scope cannot be resurrected through claim. |
| User agent removal | Current user-agent path archives (`queries/agent.sql:224`), retaining tasks and chats. Hard delete functions target system agents or workspace teardown (`:108`, `runtime.sql:384`, `workspace_delete.sql:679`). Existing Chat.agent FK cascades Chat deletion on agent hard delete. | No surviving first-party pointer to the deleted agent's managed session was found. No new standalone session registry warranted. |
| Normal runtime deletion | `queries/runtime.sql:349–360` first nulls terminal tasks' runtime IDs so history survives; user agents are unbound (:362–375); system agents/chats may be deleted. | Chat.runtime_id can still contain old runtime UUID (column introduced without FK in migration 060), but current claim requires exact runtime match. New runtime cannot use it. Immutable delivery must keep original runtime even if row column becomes NULL. |
| Legacy runtime merge | `queries/runtime.sql:415` reassigns agents and :475–480 rewrites task runtime IDs; old runtime is subsequently removed. | Concrete identity rewrite, not deletion: history lookup must notice metadata's original runtime/config mismatch and refuse that session. Filtering on new runtime then concluding no exposure would be a bug. Do not silently relabel old exposure as new runtime authority. |
| Workspace teardown | `handler/workspace.go:871` `deleteWorkspaceTasks` sweeps by agent, issue, runtime under workspace deletion fence; `DeleteTaskBatch` removes queue/history, then workspace agents/Chat/runtime/issue data are deleted transactionally. Claim builder rejects cross-workspace tasks (`handler/daemon.go:2037`). | For supported first-party scopes the reusable pointer/scope disappears with history. Cross-owner malformed legacy rows are handled broadly by cleanup but cannot have been admitted managed knowledge through the required workspace gate. Do not build a new storage system solely for hypothetical orphan pointers. |
| Issue deletion | Existing task.issue FK cascades (`migrations/001_init.up.sql:127–131`); handler `issue.go:4526` cancels then deletes under authorization. | Issue-session selectors need the deleted issue scope; no live first-party issue pointer survives. Quick-create's only LinkTaskToIssue caller is `notifyQuickCreateCompleted` (`service/task.go:7548`); it does not link Chat turns. |
| Discard unstarted quick-create retry | Deletes still-uncommitted queued/deferred child only (`agent.sql:696–707`). | Child cannot have disclosed managed body or launched; no special marker required. |
| Age-based task retention | No current production age-based queue-row purge found in handler/service/query scan. Provider workdir GC removes files, not these SQL rows. | Do not invent a task-retention blocker. If a future retention writer removes exposed rows while retaining a reusable pointer, it must atomically retire the affected pointer or retain a content-free non-resumable marker. That is conditional, not work demanded by a present path. |

Thus no actual supported first-party deletion path found that needs a new dedicated retained session table. The concrete current work is **atomic binding on writers, preservation across NULL/reclaim/runtime-merge, and fail-closed query semantics**. Source/candidate purge is different: it leaves consumer delivery history, whose current availability check must then refuse old exposure.

### Ownership and sequencing

A currently owns candidate schema/SQL/protocol and B owns Core. Do not edit their files concurrently. The following is a proposed bounded handoff for C, not a replacement for the six approved steps.

1. **A → C contract handoff, no implementation cycle.** A confirms immutable contribution/source/target DTOs and exposes source availability + resolver/revalidation service signatures. C can immediately implement independent adapter/lifecycle tests against a small fake service interface while A writes resolver internals.
2. **C1: leaf lifecycle and one adapter.** Own `pkg/agent/agent.go`, `launch_lifecycle.go`, Claude adapter and adjacent tests; preserve collaboration env default. Add explicit ordinary env policy, per-command attempt/session intent and non-lossy observation; strict effective-invocation guard. Support only admitted local Claude initially; other combinations explicitly unsupported. Other adapter edits are not prerequisites.
3. **C2: ordinary task service/SQL after A release.** Own new `service/task_knowledge.go`/tests, new task-specific SQL file, plus narrow coordinated edits to `service/task.go`, `task_cancellation.go`, `queries/agent.sql`, `queries/chat.sql`, `handler/task_lifecycle.go`. Implement exact history selection/union, generation allocation, finalized source-free descriptor, prepare/authorize orchestration and atomic binding. A remains sole sqlc/migration generator until explicit handoff; avoid parallel sqlc runs.
4. **C3: handlers and daemon integration.** Own `handler/daemon_knowledge.go`, `daemon.go`, `agent.go`, `daemon_rpc.go`, router additions; daemon `client.go`, `types.go`, `wsrpc.go`, managed transport routing, `prompt.go`, `daemon.go`, new `knowledge_context.go` helper/observer. Service method tests precede wiring; both claim transports share the same guard. Do not hijack collaboration business tokens.
5. **C4: focused integration and independent review.** Run fake executable/handler tests under the authorized isolated DB. B adds only changed wire schema/client capabilities once A/C protocol is frozen. All final G2 tests must actually execute; `[no tests to run]` is not evidence. Full checks follow the approved implement plan.

### Exact service interfaces needed (proposed, not claimed to exist)

A's stable data-level boundary should use current DB facts, not HTTP user impersonation:

- `KnowledgeSourceAvailability(ctx, qtx, requiredSources) -> availability`: immutable required source refs; checks purge/delete/current restorable version before any derived body projection.
- `ResolveApprovedKnowledge(ctx, qtx, KnowledgeConsumerFacts) -> KnowledgeResolution`: consumer facts include originator, actual runtime owner/agent, surface, target workspace/project, private Chat creator or proven issue/workspace audience. Facts are assembled by server. Return complete bounded contributions, target revisions, immutable manifest/digest, coarse no-injection reasons. Never accept caller-supplied principal/audience as authorization.
- `RevalidateKnowledgeManifest(ctx, qtx, facts, manifest) -> eligibility/refusal`: checks the exact immutable manifest for exposure resume and launch; returns no body. It must not swap to currently available candidates or an empty bundle.
- A exposes bounded **metadata-only hints** for lock discovery and the actual NOWAIT source/target lock primitives; resolver/revalidation operate on the transaction passed by C without opening nested transactions or obtaining hidden reverse-order locks.

C owns the execution wrappers (A may implement them only by explicit transfer):

- `PrepareTaskKnowledge(ctx, authenticatedRuntime, taskID, PrepareRequest) -> PrepareResponse`: validates finalized descriptor, original key, generation and selected/fresh intent; obtains design §7.2 locks; rechecks A's resolution under locks; commits disclosure metadata before returning body.
- `AuthorizeTaskKnowledgeLaunch(ctx, authenticatedRuntime, taskID, AuthorizeRequest) -> LaunchAuthorization`: reuses §7.2 lock order; validates exact prepare/manifest/prompt hashes, attempt seq/nonce, native resume or unknown-fresh generation; writes authorization, no body.
- `BindTaskKnowledgeSession(ctx, qtx, lockedTask, observation) -> error`: validates admitted attempt and updates delivery metadata before pin/complete/fail/cancel pointer publication within the caller's existing Chat→task transaction. It must not acquire workspace/source locks there. Current authorization was a separate operation; binding records facts, it does not grant new disclosure.
- `CheckTaskKnowledgeResume(ctx, currentFacts, chosenSession) -> resumable/fresh/refusal`: queries all history for the **finally chosen** key. Unsupported/legacy client plus any managed exposure means fresh ordinary only if effective invocation proves fresh; ambiguous selector/configuration means refusal. Corrupt negotiated history never means legacy.

Lock sequence stays the approved users/credentials → ordered workspace KEY SHARE/member fences → target project KEY SHARE + separate knowledge revision SHARE → Chat/source/candidate → runtime/task NOWAIT. FinalizeTaskClaim keeps its current runtime/task-first lock graph and only adds source-free metadata. Bind-only pointer writers must not import the prepare authorization lock graph.

### Wire DTO checklist for A/C protocol owner

The inspected `pkg/protocol/collaboration_knowledge.go` currently provides candidate/source/evidence/contribution types and budgets. Add a separate ordinary-task protocol file to avoid colliding with A's stable candidate DTOs:

- `TaskKnowledgeClaimDescriptor`: version, task/runtime IDs, claim_generation, dispatched_at binding, effective provider/mode/configuration identity, preparation_required; optional source-free chosen session descriptor. No candidate/source IDs, hashes, counts, summaries or body.
- `KnowledgeSessionBinding`: provider, runtime, agent/configuration namespace, native_session_id when known, scope_kind/id, scope_generation. Tagged unknown-fresh state must not masquerade as a real native ID. Claim generation is a separate field.
- `PrepareRequest`: version, request_id, claim_generation, dispatched_at, resume_intent from finalized descriptor or fresh. `PrepareResponse`: prepare ID, exact manifest/digest, contributions using A's existing DTO, target revisions and admitted binding/generation.
- `AuthorizeRequest`: version, stable request/prepare IDs, exact manifest_sha256/prompt_sha256, claim_generation, attempt_seq/nonce, session_binding. Response echoes verified identity/digests and admitted per-attempt binding; a new fresh generation is allocated/recorded server-side and replayed under the original key.
- `KnowledgeLaunchObservation`: authorized attempt identity, Started/Terminated facts and native session observation. Extend pin/terminal report shape (or add one narrow typed subresource) without letting arbitrary session text establish authorization. Distinguish disclosure, authorized, actually started, ended, session bound.
- Server-only `KnowledgeDelivery`: bounded metadata retaining previous disclosures/attempts/resume bindings across claim generations and NULL terminal session reports. No private body, tokens, or Provider transcript. Existing 64 KiB check is only envelope validation; validate phases and union budget in Go.

### Named tests and reuse anchors

Existing fake executable/process evidence anchors: `pkg/agent/launch_lifecycle_test.go`, `launch_lifecycle_unix_test.go`, `launch_lifecycle_windows_test.go`; `daemon/collaborationrun/host_handoff_launch_test.go` and `host_handoff_codex_launch_test.go`. Existing claim/cancel anchors: `handler/daemon_chat_resume_fallback_test.go`, `daemon_claim_cancelled_session_test.go`, `daemon_batch_claim_finalize_test.go`, `service/task_batch_claim_test.go`. Default tests use only test-owned executables; never installed CLIs.

Minimum new C leaf tests (names proposed):

- `TestOrdinaryKnowledgeLifecyclePreservesTaskCredentials` and `TestCollaborationLifecycleStillRejectsOrdinaryCredentials`.
- `TestKnowledgeClaudeNativeObservationIsBoundToPhysicalAttempt`, `TestKnowledgeUnknownFreshSessionCannotBePinnedWithoutObservation`, `TestKnowledgeChangedNativeSessionDoesNotRelabelHistory`.
- `TestKnowledgeEffectiveArgsCannotOverrideSessionSelection`, `TestKnowledgeAmbiguousLegacyContinueRefused`, `TestKnowledgeCustomProtocolFamilyIsNotBuiltinSupport`.
- `TestOrdinaryKnowledgeFirstLaunchRefusalIsTerminal`, `TestOrdinaryKnowledgeLaterLaunchRefusalDoesNotReplay` (approved K3 names): actual executable start count, exact stdin, usage/transcript preservation, zero automatic requeue/retry.
- `TestOrdinaryUnsupportedKnowledgeBackendRunsBaseTask`: unsupported safe configuration executes original base task with zero private preparation requests; never restores a managed session.

Claim/service matrix must retain approved names in implement §4, especially:

- `TestKnowledgeResumeDirectChatPointerUsesExactExposure`, `TestKnowledgeResumeOlderSessionFallbackUsesChosenKey`, `TestKnowledgeResumeSameSessionUnionsAllTurns`, `TestKnowledgeResumeCancelledTaskRetainsExposure`, `TestKnowledgeExplicitRerunUsesSourceSessionExposure`.
- `TestKnowledgeLegacyClientCannotResumeManagedSession`, `TestKnowledgeUnsupportedBackendCannotResumeManagedSession`, `TestKnowledgeDowngradeDoesNotTreatNullCurrentTaskAsEmptyHistory`, `TestKnowledgeProvenLegacySessionRemainsResumable`.
- `TestKnowledgeSessionPointerAndExposureBindAtomically`, `TestKnowledgeClaimFinalizationDisclosesNoPrivateFields`, `TestKnowledgePreparationRejectsOldClaimGeneration`, `TestKnowledgePreparationLostResponseReauthorizes`.
- Add `TestKnowledgeReclaimedTaskRetainsPriorDisclosureBindings` and `TestKnowledgeRuntimeMergeDoesNotRelabelExposureAsLegacy` for concrete mutations above.
- `TestKnowledgeAllFirstPartyClaimSurfaces` with per-runtime, HTTP batch, WS batch, managed/scoped fallback; table includes assignment, mention/coalesced comment, direct/fallback Chat, quick-create/source-context retry, autopilot manual/trigger. Capture private sentinels only after prepare and in actual fake stdin; assert absence from stable config/claim response.

If Codex/OpenCode are later enabled, add `TestKnowledgeCodexSameProcessFreshFallbackBindsBeforeTurn`, `TestKnowledgeCodexInternalRetryUsesActualGeneration`, and `TestKnowledgeOpenCodeAllNativeIDsObserved`; until then test their unsupported path rather than writing passing-but-unconnected support assertions.

## Files found / related specs

- Pointer/deletion/service/SQL files are cited in the tables above with one-line responsibilities.
- `design.md` §7, `implement.md` §4 — approved execution contract and mandatory regressions.
- `.trellis/spec/server/security-boundaries.md`, `managed-installations.md` — trusted originator/runtime and workspace transport.
- `.trellis/spec/server/collaboration-host-execution.md`, `collaboration-startup-handoff.md` — environment separation, per-start observation and prompt hash semantics.
- `CLAUDE.md` — no new FKs, one concurrent index per migration, test-owned executables, dbfx/testutil fixtures.

## Caveats / Not Found

- This report proposes interfaces; it does not claim resolver/preparation/authorize implementations already exist or require C to wait for them before independent leaf work.
- No general task-retention job or concrete supported-scope pointer-survives-history-deletion path found. Reclaim and runtime merge are actual mutation hazards and must be tested.
- No production files, specs, tests, DB rows or git state were modified by this research. External references: repository source; no newly verified vendor API/version contract.
