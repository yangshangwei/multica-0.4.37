# Sidebar common icon rail implementation plan

**Goal:** Deliver the already approved A composition on Web and Desktop.
**Architecture:** Shared `AppSidebar` chooses common versus full presentation;
the generic Sidebar supplies icon geometry and Desktop adapts its existing
toolbar/tab clearances. Keep queries, actions and routing under their original
authority.
**Tech stack:** React, TypeScript, Base UI, semantic Tailwind tokens, Vitest,
Playwright and the existing isolated native Electron harness.

## Execution lanes

1. Save baseline hashes/status for owned files and record existing user changes.
   Task PRD/design/manifests preserve the previous A approval and no-commit rule.
2. Shared sidebar lane (`trellis-implement`): add meaningful failing shared tests
   for collapsed common links, More, notification/counts, capabilities and compact
   full navigation; implement the smallest shared UI change and bilingual copy.
   Own `packages/views/layout/app-sidebar*`, directly needed adjacent layout or
   search/help components, and `packages/views/locales/{en,zh-Hans}/layout.json`.
3. Geometry lane (`trellis-implement`): first lock persistent icon widths,
   disabled collapsed resize and Desktop offsets with focused regressions;
   implement in `packages/ui/components/ui/sidebar.tsx`, matching shared
   primitive tests under `packages/views/layout/`, and Desktop layout/tests.
   Exclude `app-sidebar*` and locale files to prevent shared writes.
4. Integrate lanes and run scoped lint/typecheck/tests. Fix failures at their
   owning layer; coordinate shared interfaces before widening files.
5. Add an original focused rail journey in `e2e/sidebar-icon-rail.spec.ts` using
   existing API/native fixtures. Exercise common links, More/current state,
   creation and notifications, tooltip/keyboard/focus, width restoration,
   auto-collapse, compact sheet, feature switches and native toolbar geometry.
   Keep `e2e/global-scope-navigation.spec.ts` original restoration assertions.
6. Build production Web and Desktop from stable task-owned source (or the
   verified checkout if no competing source edits). Use an isolated API/database
   and loopback origins. Record source/config/listener/build identities and
   preserve generated baseline files.
7. Run both E2E files with one worker and zero retries. Inspect batched Web/native
   captures; persist visual verdict before any correction. Fix demonstrated
   defects, then run one confirmation round. Retain failed evidence honestly.
8. Dispatch independent `trellis-check` over the task diff and evidence. Resolve
   real spec/lint/type/test gaps, then update the existing navigation spec and
   task acceptance record with concrete results and verification limits.
9. Leave changes uncommitted. Record session progress and keep task artifacts
   available for review; do not stage, commit, push, release or include unrelated
   work. Stop owned verification processes and retain useful evidence.

## Useful commands

```bash
pnpm -C packages/views exec vitest run layout/app-sidebar.test.tsx layout/sidebar-resize.test.tsx layout/sidebar-auto-collapse.test.tsx
pnpm -C apps/desktop exec vitest run src/renderer/src/components/desktop-layout.test.tsx
pnpm --filter @multica/ui --filter @multica/views --filter @multica/web --filter @multica/desktop typecheck
pnpm --filter @multica/ui --filter @multica/views --filter @multica/web --filter @multica/desktop lint
pnpm exec playwright test e2e/sidebar-icon-rail.spec.ts e2e/global-scope-navigation.spec.ts --workers=1 --retries=0
git diff --check
```

Use current package scripts and fixture configuration for exact build/runtime
commands. Do not run `next dev` acceptance or resolve installed agent CLIs.
Rollback is limited to task-owned files; never reset the dirty checkout.

## Authorized delivery

The initial no-commit instruction governed implementation and acceptance. After
reviewing the completed result, the user explicitly requested local commits.
Deliver the verified work, archive this task and record the journal; do not push
or include unrelated changes.
