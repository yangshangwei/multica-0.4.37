# M1-S1 client implementation resume

Date: 2026-11-12. Scope: Core API/client, ordinary status-only gate, shared Web/Desktop authorization modal, focused tests and locales. No CLI, provider, deployment or release work.

## Changes in this lane

- Completion hint reads can pin an explicit workspace and clear the ambient slug. The request body remains the exact three-field non-authorizing hint contract.
- Enforce/done status-only actions open the shared explicit evidence/authorization flow. The gate reads the human identity only after the enforce response, avoids duplicate mutation callbacks, settles the waiting caller on cancel/unmount, and retains the captured workspace.
- The modal displays visible candidate status/result code and distinguishes access denial, read failure, empty evidence and expired evidence. No private evaluation key is requested or displayed.
- The modal reads the small local draft set on store revision changes without a redundant memo dependency.
- The ordinary surface controller test now mocks `useModalStore` with the callable Zustand selector shape required by the package contract.

## Red/green records

| Case | Red evidence | Green evidence |
|---|---|---|
| Explicit workspace on hint read | `pnpm --filter @multica/core exec vitest run api/delivery-completion-hint.test.ts -t 'pins an explicitly scoped hint read to its workspace'` failed because `X-Workspace-ID` and cleared `X-Workspace-Slug` were absent. | Same command passed after applying `workspaceRequestInit(options)`. |
| Authorization cancel settles caller | `pnpm --filter @multica/views exec vitest run issues/actions/completion-hint-gate.test.tsx -t 'settles the waiting caller when the authorization flow is cancelled'` failed: `onSettled` received zero calls. | Same command passed after cancel used the one-shot settlement callback. |
| Distinguish access denial | `pnpm --filter @multica/views exec vitest run modals/issue-completion-authorization.test.tsx -t 'distinguishes denied evidence reads from ordinary read failures'` failed: permission denial rendered the generic read-failure text. | Same command passed after mapping 401/403/404 to the controlled access-denied message. |
| Expired candidate display | `pnpm --filter @multica/views exec vitest run modals/issue-completion-authorization.test.tsx -t 'keeps expired evidence visible but unavailable for authorization'` failed when the `Expired` label was temporarily removed. | Same command passed after showing expired state while keeping the candidate disabled. |
| Auth store is optional for ordinary surface initialization | `pnpm --filter @multica/views exec vitest run issues/actions/completion-hint-gate.test.tsx -t 'does not require the auth store for ordinary surface initialization'` failed under the eager selector with `Auth store not initialised`. The original full views run also showed this stack in surface controller, status-catalog and surface-action tests. | The new test passes after moving `useAuthStore.getState()` into the enforce/done response branch. The affected surface suites pass as recorded below. |

## Verification

- Core focused completion tests: 4 files, 21 tests passed.
- Core focused session/auth guard tests: 5 files, 78 tests passed (`session-cleanup`, `auth-initializer`, `missing-user-session`, `auth-redirects`, `skills/stores/view-store`).
- Full Core suite: 338 files, 4,546 tests passed.
- Core lint and typecheck passed.
- Focused M1 and affected surface tests: 5 files, 84 tests passed (`completion-hint-gate`, authorization modal, surface controller, surface action collaboration, surface catalog).
- Views typecheck passed. Full views lint passed with 27 existing warnings in other files and zero errors; scoped lint for changed M1 files passed.
- The 8 non-M1 files that timed out in the broad views run passed when run alone: 319 tests across `collaboration/creation-form.test.tsx`, `collaboration/knowledge-panel.test.tsx`, `iterations/iteration-events.test.tsx`, `iterations/iteration-navigation.test.tsx`, `modals/quick-create-actor-picker.test.tsx`, `search/search-command.test.tsx`, `skills/components/create-skill-template-flow.test.tsx`, and `settings/components/mcp-tab.test.tsx`.
- Desktop `daemon-reauth.test.ts`: 8 tests passed.
- `git diff --check` passed for this lane's files.

## Broad views run and remaining cache reports

The earlier broad command `pnpm --filter @multica/views test` completed in 307.63s with 602 files, 7,253 tests passing and 78 failing. Sixty-three surface tests failed during the eager auth-store lookup (54 controller, 4 catalog, 5 surface-action collaboration); the controller mock was also corrected to the documented callable-store shape. The remaining 15 failures were 5-second timeouts or delayed interaction assertions in the 8 non-M1 files listed above. All 5 affected surface/M1 files and all 8 timeout files passed in focused reruns. The entire views package was not rerun after those focused fixes.

The coordinator later reported a separate Vite cache containing `authority-selection.test.tsx` and an external `/tmp/.../authority-selection.test.tsx`. Neither file is owned by or present in this client lane, and this lane did not execute or classify those external results.

## Post-fix full-suite and renderer handoff

After the focused fixes, the full package suite was rerun from the current
checkout: `pnpm --filter @multica/views test` passed 602 files / 7,332 tests in
208.94s. The command log and classification are in
`research/m1-views-full-suite-20261113.md` and
`research/m1-views-full-suite-20261113.log`.

The coordinator then completed the real task-owned Web and Electron renderer
flow. Both surfaces used the shared modal, keyboard activation, a fresh
local-synthetic receipt and a separate current issue read; both produced one
authorization and one bound completion write, then reported `consumed` plus
current `done` revision 2. The visual artifacts are linked from
`research/m1-renderer-evidence-20261113/`; this client lane still owns no
production deployment or Provider evidence.

## File identity

SHA-256 values are for the current full files, which can contain earlier in-progress M1 work as well as this lane's edits.

| File | SHA-256 |
|---|---|
| `packages/core/api/client.ts` | `b1c8daf81d24d8759bd94b07dca1d8e21883a2baac919b443da8071f0065bfb2` |
| `packages/core/api/delivery-completion-hint.test.ts` | `357a63e6ff5ba202955484d0fdaec146642d8e0b23fd61c6015310fc27b87ce4` |
| `packages/views/issues/actions/completion-hint-gate.ts` | `2d6dcf754fd14319276e6a180d45804bafbe9d0cd7cd54210c29f77ce1cf105b` |
| `packages/views/issues/actions/completion-hint-gate.test.tsx` | `5c92b853404ed2d961e19b6a3845562f0cdce8f7372bb3525bedfdf943d30d3f` |
| `packages/views/modals/issue-completion-authorization.tsx` | `69205c58e7c6a0e426fed8c726ac621328ee927de360bd916df6fce579898343` |
| `packages/views/modals/issue-completion-authorization.test.tsx` | `c43ffe4d7684b1c26ec72a04cdb5c241c612769f3e26783e5cb5eb5d860842c4` |
| `packages/views/issues/surface/use-issue-surface-controller.test.tsx` | `62cc0b643a24e7e68a486b9c71f31bd42f750d185cb9cd17fad6e9b271e9acff` |
| `packages/views/locales/en/issues.json` | `f1dafcf0f82866657357f312db87ec652cdffcd83563b911df2742ce4e6bb997` |
| `packages/views/locales/zh-Hans/issues.json` | `eb698666df3ba626974abec9688fa92e492030b1166d410ec49dd62578e5da4e` |

No screenshots, live Web/Desktop renderer checks, CLI checks or backend acceptance are claimed by this lane; those remain with the coordinator and their respective owners.
