# Content-based creator recommendations: service research

Research-only, 2026-09-30, original checkout `/Volumes/artisan/code/2026/multica-0.4.37`. No product code, running services, preferences or global state changed. Existing tests were inspected, not executed.

## Finding and recommended scope

No existing actor/creator/assignee recommendation endpoint was found in `server/cmd/server/router.go`, `packages/core/api/client.ts`, or the handler/service searches. Existing “suggestions” are chat follow-up quick actions, not actor routing (`server/internal/service/chat_quick_actions_generate.go`; `packages/core/api/client.ts:3605`). Do not reuse those because their input, authorization and outputs represent a different action.

Add one read-only, explicitly requested endpoint, provisionally `POST /api/issues/recommend-creators`, next to `/optimize-description` at `server/cmd/server/router.go:1949`. It returns zero to three eligible typed actor references with grounded reasons, never writes the current draft/assignee/default/recent history, and never enqueues a task. One validated JSON response is enough; three short candidates do not justify provisional streaming.

Use the deployment's existing `h.LLM` via `GenerateJSON`, no agent runtime, agent CLI, new SDK, model picker or new model credentials. Current quick-create submission keeps all its own authorization/readiness checks.

## Existing reusable infrastructure

| Existing path/function | Reuse and constraints |
| --- | --- |
| `server/internal/handler/issue_description_assist.go:60` / `OptimizeIssueDescription` | Membership gate before provider call; strict request decoding; bounded body, text and timeout; process admission slots; sanitized machine-readable errors; no writes/daemon path. |
| `issue_description_assist.go:23` | Existing limits: 128 KiB body, 20000 input runes, 45 seconds, eight process-wide slots (line 30). Use comparable or tighter limits for recommendations. |
| `issue_description_assist.go:172` / `parseDescriptionAssistResponse` | `DisallowUnknownFields`, trailing-token rejection, output size and semantic validation. GenerateJSON does not perform caller schema validation. |
| `server/pkg/llm/client.go:384` / `GenerateJSON` | Configured model when model argument is empty; JSON-object response mode; bounded compatibility negotiation; rejects truncated/empty completion. No tools enabled. |
| `server/pkg/llm/client.go:115` / `llm.Config` | API key, base URL, configured model, retry override and HTTP-client test seam. Disabled with neither API key nor base URL. |
| `server/internal/handler/handler.go:507` | Existing configured shared LLM client creation. No new service wiring is required to call it from a handler. |
| `server/cmd/server/router.go:440` | Reads existing `MULTICA_LLM_API_KEY`, `MULTICA_LLM_BASE_URL`, `MULTICA_LLM_DEFAULT_MODEL`; retries are separately validated in `main.go:159`. |
| `packages/core/api/client.ts:578` / `workspaceRequestInit` | Captures explicit workspace UUID, clears ambient slug because slug resolves before UUID, propagates AbortSignal. Essential for requests surviving desktop workspace switches. |
| `packages/core/api/client.ts:1226` / `optimizeIssueDescription` | API method pattern: `unknown` response, `parseWithFallback(..., null, {redact:true})`, malformed response becomes typed error rather than empty success. |
| `packages/core/issues/mutations.ts:1146` / `useOptimizeIssueDescription(wsId)` | Explicit mutation, workspace key, `retry:false`, `gcTime:0`, no draft/cache writes. Add a sibling `useRecommendIssueCreators(wsId)` with this contract. |
| `packages/views/modals/issue-description-assist.tsx:57` | AbortController identity, latest-value ref, cleanup on workspace/mode/unmount, live editor snapshot after flush. Reuse this lifecycle pattern; do not copy the text-assist auto-adoption behavior at line 133. |

`GenerateJSON` is JSON-object mode, **not** strict JSON-schema constrained decoding (`server/pkg/llm/client.go:366`). The new handler must validate every output field independently of prompting. A zero temperature argument leaves the upstream default in place (`client.go:382`); deterministic tests require a fake provider, not a claim of deterministic model output.

## Authoritative candidate and authorization sources

- `requireWorkspaceMember` at `server/internal/handler/handler.go:1007` verifies authenticated workspace membership. Resolve workspace from the request; never trust a client-provided workspace embedded in content.
- `Queries.ListAgents` (`server/pkg/db/queries/agent.sql:1`) returns active, workspace-scoped `kind='user'` agents. Do not use `ListAllAgentsAnyKind`: its comment explicitly forbids system execution carriers in pickers. `Queries.ListSquads` (`server/pkg/db/queries/squad.sql:40`) returns active workspace squads.
- Current web lists exist (`packages/core/workspace/queries.ts:51`, `:103`; `/api/agents`, `/api/squads`), but the recommendation endpoint should load authoritative rows itself. Do not accept client names/descriptions, an unverified list of IDs, or client claim `assignable: true` as authority. Current `listAgents` API parsing is unvalidated (`client.ts:1633`); do not copy that pattern for the new response.
- Visibility is not invocation permission. `ListAgents` uses `memberAllowedToViewAgent` (`server/internal/handler/agent.go:1118`), which allows admin inventory access. `canInvokeAgent` and `invokeAgentDecision` (`server/internal/handler/agent_access.go:49`, `:68`) deny private-agent invocation to admins who are not the owner.
- `validateAssigneePair` (`server/internal/handler/issue.go:3710`) is the canonical final assignment gate: existing workspace entity, not archived, allowed effective invoker. Its squad branch evaluates the leader (`:3757`). It does not establish current runtime readiness.
- `QuickCreateIssue` separately resolves a squad's leader (`issue.go:2514`), calls assignment validation (`:2556`), then `service.AgentReadiness` (`:2580`) and CLI-version gates (`:2597`). Recommendation eligibility should match phase-1 visible/assignable/runtime-bound candidates, while submission continues to enforce current readiness/version. A recommendation is not a promise that the actor can run immediately.
- Agent callers use `resolveActor` + `invokeOriginatorFromRequest` (`agent_access.go:227`), not the credential owner's identity. Terminal tasks cannot lend an originator. Do not weaken this if the new endpoint accepts the same authenticated actors as the existing issue endpoints.
- A squad candidate requires an active squad and a leader in the same workspace's eligible-agent map. This also avoids depending solely on the existing squad validator's global `GetAgent` lookup. Send the squad's own description; leader description is not squad capability evidence.

### Avoid 550-row authorization N+1

`loadInvocationTargetsByAgent` (`agent_access.go:389`) already batch-loads targets through `ListAgentInvocationTargetsByAgentIDs` (`server/pkg/db/queries/agent_invocation_target.sql:9`). Use it once for the workspace agent set. `memberHitsInvocationTargets` (`agent_access.go:178`) is a pure workspace/member target matcher; `team` remains inert. Do not call `validateAssigneePair` separately for all 550 candidates because it reloads agents/targets/principal membership repeatedly.

Recommended narrow authorization boundary repair: factor the decision part of `invokeAgentDecision` into a pure helper accepting the already loaded agent/targets, effective user and proven workspace membership/internal principal flags. Keep the existing single-agent function as its query wrapper, and call the same pure predicate from the batch recommendation loader. Preserve owner-first, private/unknown-mode denial, member targets, and the explicit workspace-internal agent/system exception. This small security-sensitive extraction needs parity tests against existing permission cases. For the final maximum three selected candidates, re-run canonical validation after the model returns; this cost is bounded and catches permission/archive changes during generation. Treat database errors as failure, not “no matching creators.”

## Bound the candidate input without silently ignoring actor 551

Recommended first implementation: scan **all** eligible saved descriptions server-side, derive a deterministic content shortlist, then send a bounded shortlist to one LLM call. Do not slice the creation-order list before scoring, and do not use favorite/recent/default/name/template labels as inferred capability.

Concrete initial budgets to lock in the PRD/tests:

1. Request body ≤128 KiB; required task `text` ≤20000 Unicode characters, nonblank. Optional project reference only if project context is included by the approved product design; validate workspace membership of the project and resolve any project facts server-side. A task-text-only recommendation endpoint is the smallest scope.
2. Rank the full eligible catalog by description-only text evidence using simple normalized term overlap (English words and CJK bigrams, no new dependency), stable `{type,id}` tie-break. Empty descriptions cannot establish a responsibility match. This is retrieval, not a capability taxonomy.
3. Retain at most 40 positive-evidence candidates, with at most 600 Unicode characters of **relevant description excerpt** each; bound the serialized provider payload too (for example 128 KiB including task text). Extract windows around matched description terms rather than always taking the first 600 characters; the useful responsibility can occur late in a long description.
4. When all eligible described actors fit the budget, sending all of them instead of positive lexical matches is reasonable and improves synonym/cross-language recall. For larger catalogs, the bounded shortlist is necessary. Do not fill a large-catalog shortlist with arbitrary first-N actors when there is no evidence; return an honest empty result and leave browsing available.
5. `GenerateJSON` with a short response token budget (for example 2000), one request lifecycle deadline (for example 30 seconds) and bounded admission. Reuse or deliberately share the existing assist admission pool; a second independent unbounded path would double deployment load. No frontend auto-retry, no background request on every keystroke.

Tradeoff: lexical preselection cannot guarantee semantic or cross-language recall in a large directory. This limitation must be explicit, not disguised as “best three across the whole workspace.” If guaranteed semantic retrieval across 550+ heterogeneous descriptions is a hard requirement, that requires a different approved design (bounded hierarchical model passes or embeddings); it is not a free addition to this minimal endpoint. Recommendations may validly be empty or partial.

## Grounded output and reasons

Do not ask the model to produce arbitrary UUIDs. Build a per-request map of short opaque references (`c1`, `c2`, ...), send only `{ref, actor_type, description_excerpt}` plus task context. Omit actor names, avatars, editable instructions, runtime/env/MCP credentials, template capability claims and leader descriptions. Names can be resolved by the UI from the eligible actor list after the server maps references back to typed IDs.

Safest minimal internal output: `{"recommendations":[{"ref":"c4","evidence":"exact excerpt from that candidate description"}]}`. The handler validates array length 0–3, duplicate refs, known refs, nonblank bounded evidence, and exact evidence membership in the description text actually supplied. Resolve refs only through the authoritative request map. Render evidence as the reason, with a localized UI lead-in such as “Description mentions …”. This makes the reason verifiable against saved content without claiming the server can mechanically prove a free-form model rationale. Empty output means insufficient evidence, not failure.

If product requires generated natural-language reasons, permit a short `reason` plus a separately validated exact evidence quote, omit names from model inputs, and acknowledge that semantic entailment of the reason remains model-dependent. Prefer the evidence-only reason for this phase's “known descriptions only” requirement.

Proposed public success shape: `{"recommendations":[{"actor_type":"agent","actor_id":"uuid","reason":"validated description excerpt"}]}`. A strict server response parser rejects missing/null array, invalid enum, unknown/duplicate ref, >3 items, oversized/raw malformed output, blank reason or unsupported evidence. Do not silently slice five provider choices to three or substitute a default actor after validation failure. Check current eligibility/description again before returning; a changed/deleted description cannot validate a stale reason. Existing current-creation APIs independently reject invented/unauthorized IDs even if a client bypasses recommendation UI.

Errors follow existing sanitized codes: `invalid_request` 400, unauthenticated/member gate 401/404, `ai_unavailable` 503, `ai_busy` 429 + Retry-After, `ai_timeout` 504, `ai_generation_failed`/`ai_invalid_output` 502. A network/provider failure is never converted to an authoritative empty array. Do not expose raw provider errors, prompts, descriptions or credentials in logs.

Prompt treats task text and saved descriptions as untrusted data, forbids following embedded instructions, selecting outside the candidate set, inventing capabilities and generating code/actions. There are no tools or writes. Exact ID/evidence checks are the hard boundary; prompting alone is insufficient.

## Core/UI request ownership and explicit adoption

- Add request/response types near `packages/core/types/issue.ts:222`, a bounded `RecommendIssueCreatorsResponseSchema` near `packages/core/api/schemas.ts:3801`, and an API client method using `workspaceRequestInit({workspaceId,signal})`. Validate `actor_type` enum, UUID format, max three, duplicate typed IDs and nonblank bounded reasons. `parseWithFallback` uses `null` and `redact:true`; malformed output fails, not `[]`. Preserve future harmless fields per existing API compatibility conventions.
- `useRecommendIssueCreators(wsId)` is a TanStack mutation with no cache invalidation, `retry:false`, `gcTime:0`. Recommendation data is ephemeral; it must not be persisted in Zustand or mixed into the actor catalog.
- Explicit “Recommend” activation reads `ContentEditor.getMarkdown()` after `flushPendingUpdate()`, not just debounced prompt state. Snapshot workspace, draft/session generation, mode, task text, optional project context and actor choice generation. Abort any previous request; assign a new AbortController/request identity.
- Completion may expose candidates only while request identity still matches, signal is not aborted, dialog/session/mode/workspace still match, prompt/context is unchanged, and the current selection has not been manually superseded. Clearing text, selecting another actor, closing, switching mode/workspace, starting a new continuous-create draft, Cancel and retry invalidate ownership. Abort cancels network where possible; identity/snapshot checks still reject late completions when transport ignores cancellation.
- Never call `setActor` when requesting/completing a recommendation. User explicitly chooses one candidate; immediately re-check that its `{type,id}` remains in the current eligible set before routing through the same phase-1 `onPick` path. That writes only current actor/draft, not default or recent history. Successful create remains the sole recent-use writer.
- Stale/failed/unavailable recommendations do not clear the selected actor, draft or editable task and do not disable ordinary Create or manual actor search. Disable adoption while submitting or when content/context no longer matches. Do not silently regenerate on content edits.

## Exact implementation boundaries

| File | Expected change |
| --- | --- |
| `server/internal/handler/issue_creator_recommendation.go` (new) | Request gate, server catalog/eligibility, bounded shortlist, prompt, strict output/evidence parsing and revalidation; no writes. Split pure shortlist/parser into a sibling file only if necessary for clarity. |
| `server/internal/handler/agent_access.go` | Narrow shared pure invoke decision extraction for batch reuse; preserve existing wrappers/semantics. |
| `server/cmd/server/router.go` | Register dedicated endpoint before `/{id}`. |
| `packages/core/types/issue.ts`, `packages/core/api/schemas.ts`, `packages/core/api/client.ts`, `packages/core/issues/mutations.ts` | Typed request/response, safe parsing, explicit workspace and abort propagation, ephemeral mutation. |
| `packages/views/modals/quick-create-issue.tsx` and phase-2 recommendation component/hook | Live editor snapshot/request ownership and explicit candidate adoption, without altering unrelated create behavior. |
| `server/pkg/llm/client.go`, `server/pkg/llm/outbound_contract_test.go` | Add documented consumer and precise transmitted-data contract. The consumer allowlist at `outbound_contract_test.go:130` will fail if the new call site is omitted. |
| `.env.example`, `apps/docs/content/docs/environment-variables.mdx`, `apps/docs/content/docs/environment-variables.zh.mdx` | Update assist-layer disclosure: explicit task text and bounded eligible saved-description excerpts; no attachments fetched, no agent execution. Existing disclosure is `.env.example:131` and English docs line 227. |
| `.trellis/spec/views/frontend/` and/or a focused server spec | Record recommendation-only semantics, evidence/eligibility bounds and cancellation contract after implementation. |

No migration, new dependency, new general LLM proxy, skill installation or daemon changes are needed. Keep project/default preference implementation in its separate lane.

## Deterministic test contract

Existing test seams:

- `server/internal/handler/issue_description_assist_test.go:18` creates an `httptest.NewServer` OpenAI-compatible provider and configures `llm.New` with fake key/model and `llm.Retries(0)`. It asserts no tools and deployment-model use. Reuse this pattern for a separate recommendation fixture.
- Same file covers authorization without upstream calls (line 110), malformed/provider failures (129), canceled request (152), saturated slots (163), propagated timeout and released slot (179). Use test-controlled channels and short request contexts instead of sleeping for the product timeout.
- `server/pkg/llm/outbound_contract_test.go` checks disabled clients make zero outbound calls and restricts SDK/client use to documented consumers.
- Permission matrix is protected by `agent_permission_test.go:152`, `:261`, `:429`; private/view distinctions by `agent_access_test.go:80`, `:410`; run these when extracting the pure invoke predicate.
- Core `packages/core/api/issue-description-assist.test.ts:23` covers explicit workspace/blank slug, signal not serialized, malformed response failure, redacted logs and cancellation.
- `e2e/support/assist-provider.mjs:10` is an existing isolated fake provider with `/requests`, task markers, delayed/invalid responses, no real models. Extend it by a clearly separate recommendation request marker/shape or create a sibling fixture; do not accidentally change optimization responses.

Required new tests:

1. Pure Go shortlist/parser: >550 actors with the only relevant description beyond index 550; stable order/ties; Chinese and English text; description evidence late in a long field; empty descriptions; no name/template-only match; input/payload/rune bounds; at most40 provider candidates and at most3 results; valid empty result.
2. DB-backed handler with `testutil`/`dbfx`: workspace isolation, archived/system/unbound agents excluded, member allowlist/public workspace/owner-only/private admin distinction, unknown permission denied, squads with missing/foreign/archived/uninvocable leader excluded, squad descriptions independent of leader, source context principal rules if endpoint permits agent callers. Assert excluded descriptions never enter fake provider input.
3. Provider adversarial matrix: invented ref/UUID, wrong type, duplicate choice, four choices, missing/null array, blank/oversized reason, evidence from another candidate, unsupported evidence, valid ref with injected reason/instructions, malformed/trailing JSON, truncated completion. No writes/issues/queue rows/default/recent mutations; no raw output logs.
4. Race: revoke assignment rights, archive/delete actor or change description while provider is blocked; returned candidates must be revalidated. Timeout/cancel releases admission and cancels upstream. Unconfigured/busy/invalid request causes zero provider requests.
5. Core boundary: workspace and signal propagation, schema drift/malformed array, unknown actor type, >3, duplicate typed ref, whitespace reason, sanitized ApiError codes, redacted parse logs; retry disabled and no draft/cache writes.
6. Real component wiring: Recommend never changes actor; explicit candidate selection changes actor once; task edits/mode/project/workspace/manual selection/close/reopen/continuous create invalidate results; late canceled promise cannot repopulate; failure/empty/unavailable leaves manual creation functional.
7. One isolated browser flow with TestApiClient + fake provider: select from a real recommended list, preserve typed draft, create through real API without invoking an agent CLI; close/cancel stale response; long reason/three candidates at narrow width; keyboard explicit adoption. Assert provider request count and contents through the fixture, not mocked recommendation endpoint success.

Verification after implementation should run narrow recommendation/auth/LLM Go suites, core schema/mutation tests, shared component tests, locale parity and TypeScript/lint checks, followed by the standalone fake-provider E2E in the checkout's isolated environment. No real model quality or semantic-recall claim can be established by deterministic fixture tests; that remains a separately authorized evaluation if required.
