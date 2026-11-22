# Sidebar Start Work Implementation Plan

> Execute in this session with native `trellis-implement`, then native `trellis-check`. The user's approved design and explicit implementation request are the execution approval.

**Goal:** Make ordinary task creation, collaboration creation and AI Chat visible and usable across sidebar scopes.

**Architecture:** Shared Views owns the sidebar composition. Core's existing modal store owns the nonpersisted target-bound creation intent; the actual workspace route consumes it once. Existing forms, navigation adapters, protected queries and unread helpers retain their business contracts.

**Tech Stack:** React/TypeScript, Zustand, TanStack Query, shared Base UI sidebar/dialog primitives, Vitest and Playwright with production Next.js and actual Electron.

## 1. Lock behavior and establish the route handoff

Files: `packages/core/modals/store.ts`, focused neighboring tests, `packages/views/layout/app-sidebar.test.tsx`, and the smallest workspace-only existing host required by research.

1. Add regressions for delayed target admission, one-shot consumption, navigation/owner/reset cancellation and superseding intents. Add sidebar regressions showing three direct entries and the account-to-selected-workspace request.
2. Run each new failing regression before production changes; retain the expected failures in task evidence.
3. Implement the smallest existing-store extension and actual-workspace consumer. Preserve direct active-workspace preference-backed opening and prevent preview-only forms/reads.
4. Run the focused store/sidebar suites until passing. Check that existing scope/unread/draft tests remain meaningful rather than mirroring the implementation.

## 2. Compose the approved shared header

Files: `packages/views/layout/app-sidebar.tsx`, its canonical tests, `packages/views/locales/en/layout.json`, `packages/views/locales/zh-Hans/layout.json`, and both `chat.json` files.

1. Replace SidebarCreation's New dropdown with the visible named Start work group in approved order.
2. Preserve primary/secondary hierarchy using existing semantic tokens and components, add full-name scope hints, and retain Beta/collaboration focus return.
3. Move AI Chat's native link, active state and workspace-only unread observer to the header; remove duplicate personalNav entry and obsolete count plumbing.
4. Verify no-space explanations, local draft marker ownership, long-name clipping, coarse 44px controls, locale parity and same behavior across Web/Desktop.

## 3. Synchronize browser journeys and documentation

Files: `e2e/fixtures/sidebar-navigation.ts`, `e2e/global-scope-navigation.spec.ts`, selector-only affected E2E files, `apps/docs/content/docs/collaboration.zh.mdx`, and `.trellis/spec/core/frontend/global-scope-navigation.md`.

1. Update shared helpers to click visible actions directly, preserving compact sidebar transition checks.
2. Replace old dropdown assertions with visible-action/space-label checks and account-to-workspace creation/AI Chat checks. Keep original detailed scope restoration, header isolation and focus assertions.
3. Update affected exact Chat selectors for the approved AI Chat label; do not rewrite unrelated journeys.
4. Update the documented direct collaboration entry, source preselection and disclosure behavior. Main session owns authoritative spec synchronization.
5. Production acceptance exposed the existing detail's anchored Back/Forward
   omission. In `collaboration-detail.test.tsx`, first reproduce that history
   traversal restores the memento and must not replay the old anchor. Correct
   only the matching guards/sibling wiring in `collaboration-detail.tsx`; retain
   the existing 800px browser assertion, then rerun the detail suite and original
   Web/Electron scope journeys.

## 4. Quality and visual gate

1. Run focused Vitest suites for sidebar, modal handoff, creation mode, navigation groups/global shortcuts and locale parity.
2. Run lint/typecheck for Core, Views, Web and Desktop as relevant to the final diff; run the repository UI export/static checks and inspect `git diff --check`.
3. Build production Web and Desktop under an isolated task-owned API/renderer allocation using `scripts/check.sh` helpers. Run `e2e/global-scope-navigation.spec.ts` and focused creation/chat journeys with one worker and zero retries.
4. Inspect desktop and compact captures in both locales/themes as a batch. Persist the visual-verdict evidence. Fix the bounded batch once, then confirm at most once.
5. Dispatch `trellis-check` over the final task-scoped diff and evidence. Resolve material findings, rerunning only affected checks.
6. Record acceptance evidence, update specs, commit only task-owned paths using Lore trailers, archive this task and record the journal. Preserve all unrelated dirty paths.

## Validation commands

- `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx layout/sidebar-navigation-group.test.tsx layout/global-shortcuts.chat.test.tsx locales/parity.test.ts --maxWorkers=2`
- `pnpm --filter @multica/core exec vitest run modals issues/stores/create-mode-store.test.ts navigation --maxWorkers=2`
- `pnpm exec turbo run lint typecheck --filter=@multica/core --filter=@multica/views --filter=@multica/web --filter=@multica/desktop --concurrency=1`
- `pnpm check:ui-exports`
- Isolated production run: `pnpm exec playwright test e2e/global-scope-navigation.spec.ts e2e/collaboration-creation-default.spec.ts e2e/settings-preferences.spec.ts --workers=1 --retries=0` with task-allocated URLs/API/native renderer exported by the existing verification helpers.
- `/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json <changed UI targets>` once after the final UI edit.
- `git diff --check`.
