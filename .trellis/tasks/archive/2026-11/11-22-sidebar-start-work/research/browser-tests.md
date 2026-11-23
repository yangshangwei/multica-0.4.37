# Sidebar browser-test and documentation slice

## Owned changes

- `e2e/fixtures/sidebar-navigation.ts`: retain the exported `sidebarNewButton` helper name, now targeting the direct New Issue / 新建任务 button. Ordinary/collaboration helpers each perform one direct action click after existing compact-navigation admission checks.
- `e2e/global-scope-navigation.spec.ts`: replace obsolete dropdown assertions with the named Start work group, visible ordered action rows, workspace descriptions, Cross-workspace/Beta guidance and one native AI Chat link. The header remains available after Collaboration is collapsed. Fixture workspaces have realistic long names. Existing Web and actual Electron journeys now open ordinary creation from account scope, verify the target workspace route and real read-only `/api/issues/table/rows` POST header after a cold reload, observe exactly one popup DOM identity, close it without replay, and open AI Chat in the same target workspace. The original source detail and account list are restored before the existing filter/history/scroll assertions. A batched light/dark header capture uses the actual command-palette ThemeProvider setter and restores the original requested theme mode before continuing. Compact English keyboard order and compact Chinese captions/direct-entry visibility are checked, alongside real control/text Range geometry through clipping ancestors. Coarse-mode targets require 44px when that existing input mode is supported. The existing 180-second test budget remains unchanged.
- `e2e/settings-preferences.spec.ts`: replace only the dedicated Chat link/heading names with AI Chat.
- `e2e/collaboration-creation-default.spec.ts`: correct the entry-point comment to the visible Start collaboration action; keep creation-gate assertions intact.
- `apps/docs/content/docs/collaboration.zh.mdx`: document direct Start work → 发起协作 entry and sidebar workspace source preselection. Correct Collaboration-title behavior to disclosure, preserving workspace-name resume and all remaining content.

No other E2E/product files were modified by this lane. In particular, the pre-existing dirty `e2e/agent-template-marketplace-desktop.spec.ts` was not edited. No commits, environment launches or real model execution were performed.

## Verification performed

1. Read the full saved native Trellis hook output and task artifacts; read `conventions.zh.mdx` before editing Chinese documentation.
2. Confirmed existing actual ordinary-dialog names from `packages/views/locales/en/modals.json`: `sr_manual` = `New Issue`, `sr_agent` = `Quick create issue`. The browser selector `/^(New Issue|Quick create issue)$/` matches those existing names.
3. Confirmed the shared Dialog popup carries `data-slot="dialog-content"`. A task-local `MutationObserver` records unique new popup nodes across the account→workspace remount, excluding nodes present before the action. Its observer and JSHandle are disconnected/disposed in `finally`; no product hook is introduced.
4. `pnpm exec playwright test e2e/global-scope-navigation.spec.ts e2e/collaboration-creation-default.spec.ts e2e/settings-preferences.spec.ts --list` — exit 0, six tests discovered in three files, including both Web and Desktop scope journeys.
5. `pnpm exec tsc --noEmit --module esnext --moduleResolution bundler --target es2022 --esModuleInterop --skipLibCheck --types node --jsx react-jsx e2e/fixtures/sidebar-navigation.ts e2e/global-scope-navigation.spec.ts e2e/settings-preferences.spec.ts e2e/collaboration-creation-default.spec.ts` — exit 0.
6. `git diff --check -- e2e/fixtures/sidebar-navigation.ts e2e/global-scope-navigation.spec.ts e2e/settings-preferences.spec.ts e2e/collaboration-creation-default.spec.ts apps/docs/content/docs/collaboration.zh.mdx` — exit 0.
7. Searched E2E TypeScript for obsolete exact New/Chat and New-menu selectors; none remain outside the deliberate assertion that the generic New button is absent.
8. Follow-up theme coverage reuses the actual `SearchCommand` commands verified against `search.json` (`Switch to Light Theme`, `Switch to Dark Theme`, `Use System Theme`) and the existing `setSnoozeTheme` acceptance pattern. No raw theme-class mutation is used. Long workspace captions are allowed to ellipsize as designed; the full name remains covered by accessible-description assertions, while the primary action text must fit its actual clipping ancestors.
9. Re-ran the scoped TypeScript check, Playwright discovery and task-scoped `git diff --check` after the theme/geometry follow-up — all exit 0; discovery still contains six tests in three files. The primary-text geometry reads the expected visible action labels, so it does not require a particular `aria-label` attribute implementation.

The local pnpm version prints its existing package.json pnpm-field deprecation warning; it does not fail either command and no package/dependency changes are included.

## Main-session acceptance handoff

Main owns the fresh production Web/Desktop build and task-owned API/renderer allocation, then runs the six listed tests with one worker and zero retries. This lane's static pass is not a claim that browser acceptance passed.

New artifacts emitted by the scope journeys include `*-scope-start-work-light.png`, `*-scope-start-work-dark.png`, `*-scope-start-work-creation.png`, `*-scope-start-work-chat.png` and the existing wide/compact English/Chinese captures. `start-work-compact-{en,zh}-geometry.json` records the three actual control rectangles, visible primary-label text ranges, clipping verdicts and existing pointer mode. `scope-restoration.json` additionally records the intended workspace UUID, the observed workspace-slug request header and the single ordinary-popup count alongside the original memento evidence.

If the actual run exposes a genuine route-admission failure, report it to the product implementer. If it exposes an obsolete locator, repair only this lane's selector. Do not weaken scope isolation, popup count or saved-position assertions to obtain a pass.
