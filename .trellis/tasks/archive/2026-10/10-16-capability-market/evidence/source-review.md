# Capability market final source review

Task: `.trellis/tasks/10-16-capability-market`

Baseline: `5049e0f6a`

Scope: task application diff, current curated check context and AC1–AC10. Parent owns full unit execution and production Web/Electron acceptance. No commit or task-state changes were made by this reviewer.

## Findings (fixed)

1. `packages/views/mcp/mcp-agent-discovery.tsx:243`
   - The extracted catalog calls `onCustom(preset, finalFocus)`, but contextual discovery still passed `setPreset` directly and discarded the live resolver. Ordinary close could use Base UI's default return target; withdrawing the opening template during custom editing caused closing to miss catalog search.
   - Retain the resolver in the contextual controller and pass it into `McpServerDialog`, matching `McpMarketPanel`. The regression in `mcp-agent-discovery.test.tsx` withdraws the template while the real custom dialog is open and verifies search focus without creating a server.
   - Reproduction: `contextual-handoff-red.log`, exit 1, 5 passed / 1 failed. Fix verification: `contextual-handoff-green.log`, exit 0, 3 files / 56 passed. No console-warning claim is made; the verified failure was the withdrawn-opener fallback.
   - Parent synced the exact callback/focus contract in `.trellis/spec/views/frontend/mcp-market.md:189`.

2. `packages/views/settings/components/provider-settings-recovery.test.tsx:45`
   - The full Views suite exposed two consumer regressions before MCP mounting: its paths mock lacked the newly consumed `useWorkspacePaths` hook.
   - Add the hook using the actual pure `paths.workspace("acme")` builder. Existing rename-retention and confirmed-access-loss assertions remain intact.
   - `provider-recovery-green.log`: exit 0, 23 passed. Narrow ESLint and Views typecheck both exit 0 after this test-only change.

## Findings (not fixed)

No remaining task-introduced code issue was identified after the two fixes above. Full Views rerun and production Web/native Desktop evidence remain parent/acceptance-owned gates at this handoff.

Pre-existing, nonblocking credential-lifecycle observations, intentionally outside this navigation task per parent direction:

- `packages/views/mcp/mcp-agent-discovery.tsx:269` awaits custom creation without resetting the settled creation mutation. The same callback at baseline `5049e0f6a` contains only `await create.mutateAsync({ name, config })`. While that controller remains mounted, mutation variables can retain custom config in memory after the form is disposed. Recommendation for a separate bounded change: reset the settled mutation in the controller while preserving failed local drafts.
- `packages/views/settings/components/mcp-tab.tsx:163` submits replacement config through `useUpdateWorkspaceMcpServer`; `packages/core/workspace/mutations.ts:165` has no credential-specific gcTime/reset, as also shown by `git show 5049e0f6a:packages/core/workspace/mutations.ts`. New market creation correctly uses `gcTime: 0` and `finally create.reset()`; its real-query regression passed. Existing replacement handling is not changed here.

## AC source audit

| AC | Source review conclusion |
| --- | --- |
| AC1 | Central paths, workspace-page registry/icons, sidebar, command keywords, diagnostic bucketing, editor prefixes, Web thin route and Desktop session route agree. URL controls type; other query values and fragment survive type pushes. Browser/native acceptance is separate. |
| AC2 | Domain helpers, source/category/search, provenance and related-resource projections are reused. Catalog failures remain independent. |
| AC3 | Workspace Skills and MCP management no longer mount their market wrappers; explicit corresponding market links remain available, including empty inventories. Skills owns its virtualized scroller directly. |
| AC4 | Market callback stays on catalog with a toast action; ordinary creation and explicitly opened recovery candidates retain detail navigation. Dirty/unknown-result handling remains in the original session. Intentional-close focus marker is preserved. |
| AC5 | Original save/assignment state machine remains; saved identity survives inventory invalidation and catalog changes. Retry selects unfinished failed IDs; core additive assignment leaves existing disabled bindings unchanged. |
| AC6 | MCP creation remains owner/admin gated; members can browse. Skill creation gains no admin gate. Contextual target/runtime data and reuse path remain intact. |
| AC7 | Controllers key by workspace ID. Visited type panels remain mounted, hidden/inert and explicitly inactive. Localized tab names are present. Native responsive/focus/history evidence remains acceptance-owned. |
| AC8 | Inactive catalog/member/inventory/picker observers are disabled, polling uses foreground-only options, and portaled dialogs are explicitly inactive. Cached failure states, explicit MCP reload and Skill preview-withdrawal/editor independence remain. |
| AC9 | Removed libraryView API/automatic selection; persisted retired fields are dropped by merge, with source/category preferences retained and workspace defaults rehydrated. |
| AC10 | No API, DB migration or dependency was introduced. Trusted recipe identity and write-only summary use are preserved; the new market custom-create mutation is reset and collected. Pre-existing legacy observations are listed separately above. |

## Verification

| Command | Result | Evidence |
| --- | --- | --- |
| `pnpm lint --force` after final application edit | Exit 0; 6/6 tasks; 0 cached; 31 existing warnings (3 UI, 1 Desktop, 27 Views), none from task files | `lint-final.log` |
| `pnpm typecheck --force` after final application edit | Exit 0; 9/9 tasks; 0 cached | `typecheck-final.log` |
| Three MCP suites, maxWorkers=2 | Exit 0; 3 files / 56 tests | `contextual-handoff-green.log` |
| Full provider-settings-recovery suite, maxWorkers=1 | Exit 0; 23 tests | `provider-recovery-green.log` |
| ESLint on changed provider-recovery fixture | Exit 0 | `provider-recovery-lint.log` |
| Views typecheck after fixture edit | Exit 0 | `provider-recovery-types.log` |
| `git diff --check` | Exit 0 | Direct execution |
| Ordinary `pnpm knip` | Exit 1; includes `.artifacts/release-v0.6.6/windows-source` duplicate-source findings (1840 unlisted dependencies); no unrelated cleanup attempted | `knip.log` |

Parent's isolated original-config Knip comparison is the introduced-issue gate; this ordinary-root diagnostic is not a clean static-analysis claim. Parent also owns reporting the fresh full Core/Views/Web/Desktop suite totals and the production Web/Electron result.
