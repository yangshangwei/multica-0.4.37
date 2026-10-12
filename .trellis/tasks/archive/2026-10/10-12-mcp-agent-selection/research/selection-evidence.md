# Research: MCP setup agent selection evidence

- Query: Which assignment behaviors must survive improvements to the post-save MCP agent picker, which public agent metadata and presentation helpers can it reuse, and which verification proves search/bulk selection does not alter assignment semantics?
- Scope: internal; shared views, core query/mutation contracts, and existing MCP tests. No catalog or backend redesign.
- Date: 2026-10-12
- Task: `.trellis/tasks/10-12-mcp-agent-selection`

## Findings

### Files found

| File | Purpose |
| --- | --- |
| `packages/views/mcp/mcp-setup-dialog.tsx` | Saved-instance transition, agent directory, local selection, partial results and retry UI. |
| `packages/views/mcp/mcp-market.test.tsx` | Canonical shared-component tests for create/choose/assign/resume/skip and failure recovery. |
| `packages/views/mcp/mcp-agent-discovery.tsx` | Existing single-agent reuse surface, including known assignment/disabled states. |
| `packages/views/mcp/mcp-agent-discovery.test.tsx` | Single-agent reuse, progress announcement, unsupported runtime and disabled binding regressions. |
| `e2e/mcp-market.spec.ts` | Real API/browser create-and-assign, resume, disabled binding, retry and member permission flows. |
| `packages/core/workspace/queries.ts` | Workspace-scoped agent/library queries and the existing agent MCP query cache identity. |
| `packages/core/workspace/mutations.ts` | Deduplicated additive assignment with fresh binding checks and per-agent confirmation. |
| `packages/core/workspace/mcp-market-mutations.test.tsx` | Retry-safe disabled bindings, duplicate IDs, invalid confirmations and invalidation. |
| `packages/core/api/mcp-market-client.test.ts` | Explicit workspace request identity despite a different current workspace. |
| `packages/core/types/agent.ts` / `types/workspace.ts` | Public agent metadata and write-only MCP server summary shape. |
| `packages/ui/components/common/actor-avatar.tsx` | Query-free avatar primitive with icon/emoji/image/fallback handling. |
| `packages/core/workspace/avatar-url.ts` | Platform API-base resolution for relative public avatar URLs. |
| `packages/views/issues/components/description-preview.ts` | Existing inert Markdown-to-plain-text description preview, limited to 300 characters. |
| `packages/views/modals/quick-create-actor-picker-model.ts` | Existing actor-description normalization, preview reuse and full-description search patterns. |

### Behaviors protected by existing tests

- **Creation and assignment remain separate.** No checkbox starts selected, no assignment occurs until requested, and saving may be followed by Skip. Evidence: `mcp-market.test.tsx:493`, particularly `:515`–`:519`, and `:683`. Real API assertions distinguish save from assignment at `e2e/mcp-market.spec.ts:286`–`:302`.
- **Focus moves to Choose agents after save.** `mcp-market.test.tsx:515` verifies the heading receives focus; implementation is `mcp-setup-dialog.tsx:145`–`:147`.
- **Partial failure retains the same saved instance and confirmed successes.** A successful checkbox becomes disabled; Retry failed sends only Ben and creation runs once. Evidence: `mcp-market.test.tsx:493`–`:536`. Real API/browser coverage at `e2e/mcp-market.spec.ts:398`–`:424` asserts one create, one successful POST and two failed-agent POST attempts.
- **Library refetch cannot unmount assignment.** `mcp-market.test.tsx:538` rerenders with the saved row and preserves the Choose agents step.
- **Resume uses saved ID, not a new create.** `mcp-market.test.tsx:603`–`:628`. Do not turn filtering/selection changes into a new creation.
- **A pending create cannot switch to another existing instance.** `mcp-market.test.tsx:630`–`:666` protects the late-create-response race.
- **Eligible choices exclude archived, system and malformed entries.** The test at `mcp-market.test.tsx:668` supplies null, Mika, archived, missing-name and valid records and expects only Ada. Do not broaden this directory by copying another feature's unrelated eligibility rules.
- **Existing disabled bindings remain disabled.** Core tests at `mcp-market-mutations.test.tsx:34` preserve them; `e2e/mcp-market.spec.ts:326`–`:332` proves the enabled flag stays false after reselection. Single-agent discovery displays Already assigned · disabled and offers no re-enable action (`mcp-agent-discovery.test.tsx:126`).
- **Member-owned agent reuse remains possible without create permission.** `e2e/mcp-market.spec.ts:431`–`:487` and `mcp-agent-discovery.test.tsx:100` own this contract. Contextual unsupported runtimes block assignment (`mcp-agent-discovery.test.tsx:170`), not catalog browsing.

### Available metadata and helper reuse

| Information | Existing source | Safe use in this pass |
| --- | --- | --- |
| Name, ID, description | `Agent` at `types/agent.ts:479`–`:497` | Show name plus saved responsibility description; normalize missing/malformed optional descriptions to empty. Search full description. |
| Avatar | `Agent.avatar_url` at `types/agent.ts:520` | Use UI `ActorAvatar` with `isAgent`, `resolvePublicFileUrl`, and the existing initials pattern from `agents/components/agent-profile-card.tsx:74`–`:96`. |
| Runtime binding/liveness | `runtime_id`, `runtime_bound`, `runtime_availability` at `types/agent.ts:491`–`:495` | Public but unnecessary for this narrow picker pass; binding and temporary offline state are different. |
| Agent task status | `Agent.status` at `types/agent.ts:594`; union at `:4` | Not an MCP connection check. Do not paint assignment success as runtime/tool connectivity. |
| Provider and runtime display label | Join `runtimeListOptions(wsId)` by `agent.runtime_id`; helpers at `runtimes/display.ts:8`, `:24`, `:60` | **Not fields on Agent.** Requires a separate runtime query and may be unavailable for private runtimes; excluded from this pass. |
| Existing assignment/enabled | Agent-scoped `WorkspaceMcpServer[]`; `types/workspace.ts:30`–`:42` | Not present in agent directory or workspace MCP library. Known single-agent discovery uses `agentMcpServersOptions` (`mcp-agent-discovery.tsx:48`, `:169`). Do not add per-agent assignment queries in this pass. |
| Current operation's success/failure | Dialog `succeeded`/`failures` at `mcp-setup-dialog.tsx:131`–`:134` | Reuse these local outcomes for feedback; they are not a full historic assignment inventory. |

Presentation details:

- The **UI avatar primitive** is query-free (`packages/ui/components/common/actor-avatar.tsx:17`–`:37`) and already covers semantic icon URLs, special emoji, image errors and the agent fallback (`:40`–`:51`, `:71`–`:89`). `resolvePublicFileUrl` handles relative paths while retaining `icon:`/`emoji:` values (`core/workspace/avatar-url.ts:3`–`:11`).
- The **views ActorAvatar wrapper** reads member, agent and squad directories through `useActorName` (`core/workspace/hooks.ts:94`–`:98`) and defaults to a profile link (`views/common/actor-avatar.tsx:117`). A direct UI avatar avoids extra subscriptions and navigation within the checkbox row.
- Reuse **descriptionPreview** at `views/issues/components/description-preview.ts:19`. The actor picker already calls it on normalized agent descriptions at `views/modals/quick-create-actor-picker-model.ts:28`–`:29`. It is an inert display snippet, not the search source: full-description search is demonstrated at `:55`–`:64` and required by the quick-create actor spec.
- Keep each checkbox's accessible name equal to the agent name. Existing browser tests use exact names (`e2e/mcp-market.spec.ts:297`, `:415`). Place description/status outside its naming label or wire separate `aria-labelledby` and `aria-describedby` IDs; a verbose new label would break both accessibility clarity and canonical tests.

### Identity, cache and duplicate/stale assignment constraints

- Agent directory key is `["workspaces", wsId, "agents"]` (`core/workspace/queries.ts:12`, `:53`–`:57`). It explicitly requests archived records, so view filtering must remain. The query may refresh every 30 seconds (`:60`–`:68`); selected agents can become archived/removed while the dialog is open.
- MCP library key is `["workspaces", wsId, "mcp-servers"]` (`queries.ts:26`, `:258`). Agent assignment query uses the existing `["agents", agentId, "mcp-servers"]` key (`:267`–`:271`), also used by mutation invalidation (`mutations.ts:155`–`:158`). Do not introduce a divergent key merely for picker presentation.
- The dialog currently intersects selected IDs with the **full eligible choices**, excluding successes (`mcp-setup-dialog.tsx:197`–`:199`). Preserve this when adding search: filtered visible rows must be a second projection, not a replacement for the eligibility directory used by `pendingIds`.
- Assignment mutation deduplicates input IDs, re-reads each agent's binding list, matches **both server ID and captured workspace**, and skips existing disabled bindings (`core/workspace/mutations.ts:133`–`:141`). It verifies that every POST response contains that same identity (`:142`) before claiming success; empty/foreign confirmation regressions are covered at `mcp-market-mutations.test.tsx:64`.
- Request identity is explicit `{ workspaceId: wsId }` for fresh reads and POSTs; do not replace it with a current-workspace singleton. `core/api/mcp-market-client.test.ts:81`–`:92` intentionally changes the singleton to another workspace and still requires captured-workspace headers.
- Successful/failed agent cache entries are invalidated on settle. Selection counts must reflect eligible unfinished IDs rather than blindly using `selected.size`; that set also contains successful rows and may retain IDs removed by later query refreshes.

### Concrete gaps and recommended bounded improvement

Current choices render as bordered checkbox/name rows with local success/failure indicators (`mcp-setup-dialog.tsx:548`–`:599`). There is no directory search, responsibility preview, bulk-result control, selected-only view or selected count. With many agents, all identification and selection work is manual scrolling.

Recommend a shared-dialog improvement using the existing agent query only:

1. Add accessible local search matching agent name and full saved description. Show a separate no-matching-results state with Clear search, retaining the existing true-empty/loading/error states.
2. Show query-free avatars and inert description previews beside names. Keep exact checkbox naming and long-name/description wrapping.
3. Add **Select results** and **Clear selection** for editable choices, plus a visible/live selected count and optionally a Selected view. No automatic initial selection.
4. Keep the full eligible directory, filtered rows, editable selected IDs and success/failure outcomes distinct. Select results unions unfinished visible IDs with existing selection; changing search never deselects hidden IDs. Clear selection changes editable selection without erasing success/failure bookkeeping. Refreshed archived/removed IDs never enter submitted IDs or selected counts. Bulk controls obey busy/permission/unsupported gates.
5. Preserve saved-instance identity, heading focus, pinned assignment step, canonical assignment mutation and retry-failed behavior. Keep actions reachable at narrow sizes and with long lists; selected rows must remain visually selected under hover.

Meaningful new regression cases: selection survives filtering away and returning; selecting current results preserves hidden selections; Clear/Selected view behaves with no matches; description match beyond preview limit is found; equal names retain ID identity; partial failure plus search/clear/select never repeats confirmed successes; refreshed archived/deleted IDs are excluded from counts/submission. Keep pure model boundary cases in one canonical `.test.ts` if a helper is introduced; component tests own control/focus/wiring.

### Verification commands

From the repository root, after implementation:

```bash
pnpm --filter @multica/views exec vitest run mcp/mcp-market.test.tsx mcp/mcp-agent-discovery.test.tsx
pnpm --filter @multica/core exec vitest run workspace/mcp-market-mutations.test.tsx api/mcp-market-client.test.ts
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec eslint mcp/mcp-setup-dialog.tsx mcp/mcp-market.test.tsx
pnpm exec playwright test e2e/mcp-market.spec.ts --project=chromium
```

Include any new picker helper/test files in focused lint/test commands. Run the existing description-preview tests only if that helper changes. Static source checks can confirm no added direct API calls, stores, runtime-list query or per-agent assignment prefetch inside this picker. Existing root lint/typecheck/test commands remain available for broader verification.

`playwright.config.ts:19` requires servers already running; use the checkout's configured API/frontend/database environment rather than assuming port 3000. `e2e/mcp-market.spec.ts:83` supports device and `E2E_PASSWORD_AUTH=1` fixtures. `e2e/fixtures.ts:410` seeds a process-free Codex runtime; these tests do not execute a real provider CLI or MCP tool. Add a many-agent/filter/bulk interaction case to the browser suite and capture wide/narrow screenshots; unit tests alone do not prove footer visibility or long-list layout.

### External references and related specs

- No external research was needed; implementation uses existing React/TanStack Query/UI primitives. Repo tooling is pnpm `10.28.2`, Vitest via package scripts, and Playwright `^1.58.2` from root `package.json`.
- `.trellis/spec/views/frontend/mcp-market.md:81`–`:103`: explicit assignment, saved-ID retry, disabled bindings, permission and eligible-agent contracts.
- `.trellis/spec/views/frontend/mcp-market.md:105`–`:130`: loading/error separation, scrolling, focus, progress and coarse-pointer targets.
- `.trellis/spec/views/frontend/quick-create-actor-picker.md:29`–`:31`: full-description search and inert preview precedent, without borrowing its unrelated favorites/permission/storage semantics.
- `CLAUDE.md:218`–`:225`: canonical testing layer and behavior-before-implementation tests; package/state/UI boundaries also apply.

## Caveats / Not Found

- This research ran no verification tests and edited no product code. Line citations describe the inspected baseline; concurrent implementation may shift lines.
- The market component tests mock agents with only ID/name (`mcp-market.test.tsx:127`–`:129`) and mock assignment mutations. Presentation must tolerate missing optional metadata; core/API/browser suites prove actual request identity and duplicate-write behavior.
- Existing agent directory parsing normalizes malformed network lists to `[]` (`core/api/client.ts:2149`; `api/schema.ts:40`–`:55`). Therefore the dialog's non-array guard (`mcp-setup-dialog.tsx:154`) cannot identify all malformed real API responses, despite the MCP spec's error-not-empty preference. This is a pre-existing boundary limitation, not justification for a broad client rewrite in this picker pass.
- MCP assignment list parsing similarly has an empty-list fallback (`core/api/client.ts:3518`). Keep the authoritative mutation confirmation path and do not infer historic binding absence from a new cached/failed presentation query.
- Provider/runtime labels and existing-assignment badges are not available from the current agent list alone. The parent confirmed these extra joins/queries are outside the continuation's scope.
