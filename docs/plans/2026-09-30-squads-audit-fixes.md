# Squad audit fixes and browser verification

**Goal:** Apply the approved Impeccable audit findings to squad discovery and measure actual browser behavior and performance.

**Architecture:** Preserve shared web/desktop ListGrid and navigation patterns. Fix squad-specific links and controls locally; make the shared agent profile card detail link visible. Retain stored descriptions and display preferences. Do not change global brand tokens, add dependencies, or rewrite unrelated work in this shared checkout.

**Approved direction:** Native avatar/name links, primary-colored active filter with visible condition summaries, progressive container columns, coarse-pointer target sizing, concise workspace/template labels, and full-description access without changing user-authored content.

## Execution

1. Inspect current local runtime ownership and capture a browser baseline against an isolated fixture workspace before source edits. Record environment and cache state.
2. Add failing component regression tests for actor links and filter summaries. Implement the squad page/locales changes, preserving URL and row navigation behavior.
3. Make profile-card details consistently visible with a focus indicator. Cover the behavior using real-browser keyboard navigation; do not add CSS-string mirror tests.
4. Extend real Playwright squad coverage for links, menus, filter/clear, sorting, persisted columns, new-squad dialog, locale, light/dark contrast, responsive layouts and touch target sizes. Use TestApiClient and clean up only the fixture workspace.
5. Build/run the updated application as needed without stopping other developers' processes. Capture load timing, FCP/LCP, CLS, resource transfer and interaction latency under documented conditions.
6. Inspect desktop/compact screenshots in one batch and record visual-verdict JSON. Apply any observed corrections in one batch and confirm once.
7. Run focused tests, lint, typecheck and static detector. Review the task diff; save evidence and remaining limitations under docs/audits and .omx/reports.

## Validation

- Component: `pnpm --filter @multica/views exec vitest run squads/components/squads-page.test.tsx agents/components/agent-profile-card.test.tsx locales/parity.test.ts`
- Browser: `pnpm exec playwright test e2e/squads-design.spec.ts e2e/squads-audit.spec.ts --project=chromium`
- Types: `pnpm typecheck`
- Lint: scoped ESLint for changed views files, with broader package checks if indicated.
- Static: Impeccable detector over changed UI files and `git diff --check`.

## Scope and rollback

Edits belong to the squad list/chooser/locales, shared agent profile card, browser tests, and task documentation. Rollback can revert those hunks independently. Existing user data, default-hidden provenance columns and other active frontend/backend changes remain intact.
