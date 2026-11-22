# Shared sidebar implementation evidence

## Result

The fixed shared header now has a named Start work / 开始工作 group with directly visible New Issue / 新建任务, Start collaboration / 发起协作, and AI Chat / AI 对话 controls. Ordinary creation is the primary action. Workspace names appear as supporting captions and descriptions; collaboration shows Cross-workspace / 跨工作空间 and Beta. The old New dropdown and duplicate personal Chat link were removed.

The existing creation forms, creation-mode helper and chat unread definition were reused. Chat's unread observer and the draft marker remain under the actual workspace branch. Collaboration still defaults to the displayed workspace and returns focus to its visible direct button. Missing-workspace actions remain visible, disabled and explained.

Core's existing nonpersisted modal store now carries `WorkspaceIssueCreationIntent`. The request records API instance/session epochs, authenticated viewer, target UUID/slug and source/target paths. `reconcileWorkspaceIssueCreation` retires invalid owners/destinations, ignores obsolete observer references, and atomically consumes only the admitted real target. Actual `ModalRegistry` validates the real workspace provider and live platform mirror before invoking the existing preference-backed helper. Sidebar navigation, another modal action, and session teardown retire pending intent. Both reconciliation effects read the live auth store at application, while their selectors retain reactive ownership updates.

## Changed files

- `packages/core/modals/store.ts`, `index.ts`, new `store.test.ts`: transient target-bound handoff and its canonical state/ownership matrix.
- `packages/core/platform/session-cleanup.ts`, `session-cleanup.test.ts`: clear pending handoff during existing session teardown.
- `packages/views/modals/registry.tsx`, new `registry-workspace-creation.test.tsx`: actual workspace admission, delayed mirror, host remount, StrictMode consumption and live-viewer regression.
- `packages/views/layout/app-sidebar.tsx`, `app-sidebar.test.tsx`: direct header actions, scope descriptions, moved unread observer, cancellation, no-workspace presentation and existing navigation/feedback behavior.
- `packages/views/locales/{en,zh-Hans}/{layout,chat}.json`: direct-action copy and consistent AI Chat naming.
- Parent-approved assertion-only changes in `packages/views/chat/chat-page.test.tsx`, `packages/views/chat/components/chat-window.test.tsx`, `packages/views/layout/tab-presentation.test.tsx`: existing page/window/tab assertions use the intentional AI Chat label.

No platform adapter, existing creation-mode implementation, backend, dependencies or persisted-state format needed modification. Existing source file metadata is preserved, including `store.ts`'s HEAD mode 0755. The concurrent footer Settings-label update was retained.

## RED evidence

All initial tests were run before their corresponding production implementation.

1. `pnpm --filter @multica/core exec vitest run modals/store.test.ts`: **12 failures** because the pending queue/reconcile API did not exist.
2. `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx -t 'offers all three start-work|opens ordinary creation immediately'`: **2 failures** because no named Start work group/direct controls existed.
3. After the store implementation, `pnpm --filter @multica/core exec vitest run modals/store.test.ts platform/session-cleanup.test.ts -t 'workspace issue creation handoff|pending workspace creation handoff'`: **12 passed, 1 failed** because session cleanup retained the pending descriptor.
4. `pnpm --filter @multica/views exec vitest run modals/registry-workspace-creation.test.tsx`: **3 failures** because the registry neither consumed an admitted request nor retired a changed destination.
5. `pnpm --filter @multica/views exec vitest run modals/registry-workspace-creation.test.tsx -t 'validates the live viewer'`: **1 failure** showing a form opened with an earlier render's viewer even though the live viewer had changed. Both effects now read live identity.
6. The three existing Chat-label suites produced **3 failures, 48 passed** for hardcoded old Chat labels after the intentional locale change; only those label assertions were updated with parent approval.

Temporary full command logs are under `/tmp/sidebar-start-work-*.log`; this note retains the meaningful results without requiring those temporary files for acceptance.

## Final GREEN verification

- `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx modals/registry-workspace-creation.test.tsx modals/registry-project-prefill.test.tsx layout/sidebar-navigation-group.test.tsx layout/global-shortcuts.chat.test.tsx chat/chat-page.test.tsx chat/components/chat-window.test.tsx layout/tab-presentation.test.tsx locales/parity.test.ts --maxWorkers=2`: **9 files, 186 tests passed**, exit 0.
- `pnpm --filter @multica/core exec vitest run modals/store.test.ts platform/session-cleanup.test.ts issues/stores/create-mode-store.test.ts --maxWorkers=2`: **3 files, 22 tests passed**, exit 0.
- `pnpm --filter @multica/core typecheck` and `pnpm --filter @multica/views typecheck`: **passed**, exit 0. An earlier Views run caught unsupported Testing Library `exact` options in new tests; those were removed and both final checks reran successfully.
- Full Core and Views package lint: **exit 0**. The existing Views lint inventory has 27 warnings outside this task's files; no unrelated fixes were made.
- Explicit eslint over all changed Core/Views TypeScript and test files: **passed with no findings**, exit 0.
- Task-scoped `git diff --check`: **passed**, exit 0.

## Main-session handoff

The owned source files are frozen after these checks. The main session owns final cross-platform checks, the isolated production Web/Desktop build, actual browser navigation and visual verification, authoritative spec synchronization, commits, task JSON and archive/journal. No browser/native or physical-input success is claimed by this implementation lane.
