# Unified sidebar continuation quality review

- Date: 2026-11-21
- Reviewer: `sidebar_quality_review` (`trellis-check`, direct review; no descendants)
- Scope: task-owned Core preferences, shared sidebar/groups/creation/notifications/workbench/feedback host, Web scope/account/dashboard wiring, Desktop shell/account wiring and their canonical tests.
- Preserved all pre-existing dirty changes. No staging, commits, builds or browser/native claims were made in this lane.

## Finding fixed

- File: `packages/views/layout/sidebar-navigation-group.tsx`
- Issue: a manual disclosure override was retained indefinitely against its original full path. Collapsing AI Team at `/acme/agents`, navigating to `/acme/squads`, then returning to `/acme/agents` revived the obsolete collapse. That violated active-child reveal on re-entry.
- Fix: discard the local manual override when the full path changes. The current render already derives automatic reveal from `active`; the reset prevents a later history visit reviving an earlier override. Durable boolean preferences remain untouched by automatic navigation.
- Regression: extended the existing canonical active-child test with the return navigation. Against the original component, `pnpm --filter @multica/views exec vitest run layout/sidebar-navigation-group.test.tsx` failed at the added `aria-expanded="true"` assertion (1 failed / 1 passed). The final full focused Views run passed.
- Changed in this review: only `sidebar-navigation-group.tsx`, `sidebar-navigation-group.test.tsx`, and this report. No dependencies, interfaces, markup or styles were added.

## Findings not fixed

None identified within this review's scope. The existing account feedback host is correctly separate from workspace `ModalRegistry`; account workspace previews do not mount workspace services. Platform tests cover empty/pending/failed directories and mirror clearing. Native hit regions, compiled CSS and browser restoration evidence remain the coordinating session's separate checks.

## Verification

All commands below completed successfully after the fix. Package-manager deprecation notices concerning the root `pnpm` field were warnings only.

### Tests: 21 files / 275 tests passed

```sh
pnpm --filter @multica/core exec vitest run navigation/sidebar-preferences.test.ts navigation/scope-history.test.ts
```

Result: 2 files / 7 tests.

```sh
pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx layout/sidebar-navigation-group.test.tsx collaboration/notifications.test.tsx collaboration/account-workbench-page.test.tsx collaboration/creation-form.test.tsx modals/feedback-host.test.tsx locales/parity.test.ts
```

Result: 7 files / 157 tests, including locale parity and the new history regression.

```sh
pnpm --filter @multica/web exec vitest run platform/scope-shell.test.tsx platform/account-shell.test.tsx platform/account-workbench-route.test.tsx platform/account-route.test.tsx platform/navigation.test.tsx platform/scroll-restoration.test.tsx 'app/[workspaceSlug]/layout.test.tsx'
```

Result: 7 files / 51 tests.

```sh
pnpm -C apps/desktop exec vitest run src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx src/renderer/src/platform/account-workbench-route.test.tsx src/renderer/src/platform/navigation.test.tsx src/renderer/src/stores/account-tab.test.ts
```

Result: 5 files / 60 tests.

### Type checks: all four affected packages passed

```sh
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/web typecheck
pnpm -C apps/desktop typecheck
```

Desktop ran both node and renderer TypeScript configurations.

### Lint: all selected task-owned files passed

```sh
pnpm --filter @multica/core exec eslint navigation/sidebar-preferences.ts navigation/sidebar-preferences.test.ts navigation/index.ts
pnpm --filter @multica/views exec eslint layout/app-sidebar.tsx layout/app-sidebar.test.tsx layout/sidebar-navigation-group.tsx layout/sidebar-navigation-group.test.tsx layout/dashboard-layout.tsx collaboration/notifications.tsx collaboration/notifications.test.tsx collaboration/creation-form.tsx collaboration/creation-form.test.tsx collaboration/account-workbench-page.tsx collaboration/account-workbench-page.test.tsx modals/registry.tsx modals/feedback-host.test.tsx
pnpm --filter @multica/web exec eslint platform/scope-shell.tsx platform/scope-shell.test.tsx platform/account-shell.tsx platform/account-shell.test.tsx platform/account-workbench-route.tsx platform/account-workbench-route.test.tsx app/account/layout.tsx 'app/[workspaceSlug]/(dashboard)/layout.tsx'
pnpm -C apps/desktop exec eslint src/renderer/src/components/desktop-layout.tsx src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx src/renderer/src/platform/account-workbench-route.tsx src/renderer/src/platform/account-workbench-route.test.tsx
```

Scoped `git diff --check` also passed for Core navigation, shared layout/collaboration/modals, Web platform/account/dashboard and Desktop shell/platform paths.

## Integration handoff

The disclosure production fix postdates the earlier R2 browser artifacts. Rebuild and rerun the affected runtime path before binding final acceptance to the fixed source; the coordinating session was notified as soon as Views lint/typecheck/tests passed.

Final source SHA-256 values at handoff:

```text
a9d2a14186b2427194153664b417a2de7380d05be0769b57972d6b1da19c8f4d  packages/views/layout/sidebar-navigation-group.tsx
bfa809946c68c135decd7189a31371b176b3a8d575c3171e54bf15a231fcfc79  packages/views/layout/sidebar-navigation-group.test.tsx
```

The existing global-scope-navigation spec already requires active-child reveal. If adding an explicit example during final spec sync, include child A collapse → child B → Back to child A, so a path-keyed override is never treated as persistent visit history.
