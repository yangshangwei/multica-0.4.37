# Research: Approved collaboration resume UI semantics

- Query: Independently review the explicit account-resume UI/platform patch against the approved C1 behavior without redesigning it.
- Scope: internal; focused source and retained unit/platform log review.
- Date: 2026-11-25

## Findings

**Source semantics accepted; no new actionable issue found. C1 remains open until the new bound build and both complete original manual-detail journeys pass.**

- `packages/views/layout/app-sidebar.tsx:861` adds a separately named Return to previous collaboration button outside the existing disclosure. It remains available when the group is collapsed and in icon navigation, has localized accessible text/tooltip and coarse-pointer sizing, and is disabled in the current account destination. The heading still only toggles disclosure; explicit list links remain unchanged.
- `apps/web/platform/scope-shell.tsx:74` resolves the account destination through the existing owner-scoped history store and calls `useWebScopeResume`, preserving query/fragment semantics. The increment before account no-op/route commit invalidates a prior pending workspace lookup; it does not introduce another history/store or manufacture an issue URL. Existing Web layout prop spreads carry the callback to the sidebar.
- Desktop layout passes only `resumeDesktopScope("account")`; the existing adapter clears the workspace mirror and calls `switchAccount()` without an explicit path. Existing account tab/history/scroll semantics remain the owner. No shared view imports router APIs and no state is relocated outside core/platform boundaries.
- Shared tests cover one callback in expanded/icon states, disclosure independence and Chinese/current-account inertness. Web tests cover full remembered query/fragment, identity change/default and deferred workspace lookup cancellation. Desktop wiring tests require the no-path account resume call. These tests follow their owning layers.
- The canonical manual helper now clicks the actual resume button; it does not use Back/Forward, an explicit issue link, or issue reselection. Original selected title/issue, saved scroll and single account-tab assertions remain. This restores the test's intended operation rather than substituting another path.

Retained logs inspected: latest shared3-file run88PASS, corrected Web46PASS, Desktop51PASS and scroll7PASS; the deferred-workspace control retains its prior1FAIL/11PASS RED. These counts describe their individual selections and are not summed as unique coverage. This is source/unit evidence, not final visual or complete manual acceptance. Exact inspected hashes/log summaries are in `c1-ui-source-review-audit.json`.

## Files found / related specs

- Shared sidebar and test; English/zh-Hans collaboration locale — explicit accessible action with existing design-system primitives.
- Web scope shell and test — owner-bound resume and invalidated pending lookup.
- Desktop layout/workspace-gate test and existing platform navigation — no-path account switch.
- `e2e/fixtures/collaboration-detail-journey.ts` — retained original restoration assertions using the real affordance.
- `.trellis/spec/core/frontend/global-scope-navigation.md` — updated contract matches the inspected implementation.
- `CLAUDE.md` — package/state/platform boundaries and canonical test ownership respected.

## Caveats / Not Found

- No product edits, builds, tests, UI operations, services or Git commands executed by reviewer. Research outputs only.
- Earlier13 client PASS cases remain bound to their earlier frontend artifacts. They do not establish the new C1 candidate; use fresh build bindings and scoped replay as already planned.
- Existing C2 late snooze observation qualification remains; this navigation review does not close it.
- CLI03/Linux Main execution review was reassigned to another checker. No duplicate admission decision was issued here.
- No external references needed.
