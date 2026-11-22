# Collaboration sidebar Beta verification

## Outcome

The collaboration directory now defaults to a single collapsed heading with a neutral Beta marker. The title resumes the exact account destination; a separate arrow controls disclosure. Collaboration destinations reveal the directory, and the existing core preferences retain manual choices. The same protected notification Query keeps positive collapsed unread feedback available; collapsing removes the notification popover and child controls.

Both supported locales show the approved maturity explanation through hover/keyboard focus and the quiet workbench header. Title and creation entry descriptions provide the assistive equivalent of their decorative Beta markers. The live native preview's verification-only disclosure choice was restored to collapsed/default false.

## Evidence

- Baseline behavior suites passed before edits. Added default-collapse/legacy-preference and maturity-description regressions failed on the missing behavior before their fixes.
- Core sidebar preference tests: 4/4 passed.
- Related views sidebar/group/workbench/notifications/locale suites: 128/128 passed. The final heading-description follow-up also passed all 45 sidebar tests.
- Core, views, Web and Desktop type checks passed. Affected-file core/views/E2E lint and final whitespace checks passed.
- Impeccable detector over the four changed UI targets returned `[]`.
- Final isolated production Web and Desktop builds passed. Both canonical navigation journeys passed, zero retries, in 22.4s. They covered keyboard disclosure, notification portal removal, title resume/current-scope no-op, automatic reveal, Beta marker/context, restored search/sort/scroll, deep links, EN/ZH compact sheet navigation and native 200% zoom.
- Native source and final production screenshots were inspected. Final visual verdict: 97/pass. See `research/visual-verdict-native.json`, `research/visual/` and `research/production-journey/`.
- The final production watchdog returned exit 0 in 119.2s, below its 600s bound. Original source environment files and `apps/web/next-env.d.ts` were preserved. See `research/production-run.json`.

## Changed files and simplifications

- `packages/core/navigation/sidebar-preferences.ts` and its test: one existing preference map now includes collaboration's collapsed default and safe legacy hydration.
- `packages/views/layout/app-sidebar.tsx` and its test: one compact heading, sibling resume/disclosure controls, contextual Beta descriptions and retained unread feedback.
- `packages/views/layout/sidebar-navigation-group.tsx` and its test: extracted the existing disclosure behavior into a small shared hook; other groups keep their prior behavior.
- `packages/views/collaboration/notifications.tsx`: the existing query-owning wrapper delegates to a presentational popover, letting the sidebar retain a single query observer while unmounting hidden controls.
- `packages/views/collaboration/account-workbench-page.tsx` and its test: quiet maturity explanation in the header.
- `packages/views/locales/en/collaboration.json` and `zh-Hans/collaboration.json`: aligned Beta/disclosure copy.
- `e2e/global-scope-navigation.spec.ts`: meaningful two-client Beta/disclosure assertions; the existing identical cleanup transaction moved into an awaited helper to remove `no-unsafe-finally`.
- `.trellis/spec/core/frontend/global-scope-navigation.md`: current disclosure, notification and accessibility contracts.

## Environment recovery and verification limits

An existing live Web check on 13524 prevented rebuilding shared `.next` output. Verification used an agent-created detached sparse worktree with this task's exact reviewed files and copied installed dependencies. No existing environment was stopped or dependency installed. The failed tooltip role assertion was corrected against the actual Base UI primitive and real keyboard modality; product presentation was unchanged.

The final fixture runs the current production renderer/preload in a real Electron window with fixture Main, plus production Web and a task-owned current API/database. It does not claim packaged installer or whole collaboration execution acceptance. No new regression or unresolved task finding remains. The three unrelated original user changes were preserved.

## Acceptance mapping

AC1–AC7 are met by the canonical behavior suites, independent review, real native source inspection, final two-client production journey and screenshot checks. All in-scope work is complete.
