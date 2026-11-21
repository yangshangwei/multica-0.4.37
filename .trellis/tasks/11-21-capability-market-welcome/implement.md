# Capability market welcome implementation plan

**Goal:** Let people choose a capability from a short introduction and a practical example without delaying catalog browsing.

**Architecture:** Shared React views, Base UI tab semantics, local CSS, i18n, and existing navigation. Follow `design.md` and use the existing per-category scroll roots.

- [x] Add a meaningful test in `packages/views/marketplace/marketplace-page.test.tsx` for the selected case and committed URL (including history). Run the focused market suites and retain URL, history, draft, and inactive-panel coverage.
- [x] Write `marketplace-welcome.tsx` and adjacent CSS module/types. Wire into `marketplace-page.tsx`, add bilingual copy in `packages/views/locales/{en,zh-Hans}/layout.json`, and pass the compact case through a leading slot in `agent-templates-market-panel.tsx`. Keep all existing modes and routing contracts.
- [x] Run scoped Vitest for the market, panel, and locale parity; `pnpm --filter @multica/views typecheck`; and ESLint on changed TS/TSX. Review both catalogs’ inherited entry actions and scroll restoration.
- [x] Add a focused Playwright case for compiled layout and disclosure, keeping existing market navigation/draft acceptance and agent-template tests. The production browser run is blocked by the local environment missing `psql`; the test still records wide/narrow, locale/theme, focus, reduced-motion, wrapping, and scroll assertions. Impeccable detection returned no findings.
- [x] Review the scope against the pre-existing working tree. Record the welcome/selection contract in the capability market spec and verify the final task diff without absorbing unrelated sidebar and collaboration work.
