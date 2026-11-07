# Locale reload anchor-wait boundary

The canonical-final1 Electron detail failure at A17 is an incorrect fixture expectation added during the previous session: `switchJourneyLocale` expected both clients to focus their retained comment anchor after `page.reload()`.

Desktop reload rehydrates its persisted account tab. `mergePersistedTabs` retains the tab memento and returns `isScopeResume: true` (`apps/desktop/src/renderer/src/stores/tab-store.ts:1228`). The Desktop adapter forwards that value. Shared detail deliberately skips explicit anchor focus for a scope resume (`packages/views/collaboration/collaboration-detail.tsx:50`) and restores the saved offset after issue/comments readiness. A comment remaining inactive after native reload is consistent with that contract.

Web rebuilds its navigation provider on document reload with `committedIntent.resume: false` (`apps/web/platform/navigation.tsx:121`), so the URL comment is an explicit anchor visit. Issue data can arrive before comments and the deferred drawer-close callback; Web must await real comment focus before screenshot positioning starts.

The fixture now parses the browser URL and waits for the comment only on Web. Removed the native localStorage tab-shape inspection. No product code, 620 expectation, screenshot geometry checks, timeouts, retries or explicit anchor assertions were changed. The original A16 repeated-link focus assertions remain in the complete journey for both clients.

Verification: `git diff --check -- e2e/fixtures/collaboration-detail-journey.ts` passed. An exploratory ESLint run using the Web application config reports eight errors in this E2E fixture; feeding the exact HEAD file through the same config reports the identical eight errors (four dependency-policy and four escaped-string errors). This is not a lint pass, and no lint rules were disabled. The repository's prior 15-task static check remains separate; root E2E is not its configured application lint scope. Evidence is `.omx/private/acceptance-repair-20261107/checks/locale-wait-boundary/`.

The parent owns the new canonical Web/Electron run. No browser tests were launched by this subtask; canonical-final1 and its native focus failure remain preserved.
