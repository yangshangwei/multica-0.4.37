# Implementation plan

## Ordered checklist

1. **Lock route and view contracts with tests.** Extend `packages/core/paths/paths.test.ts` for the new `squad-templates` market path. Update `packages/views/squads/components/squads-page.test.tsx` for a single saved-squad view and the legacy redirect, including unrelated query parameters and the hash. Extend `packages/views/marketplace/marketplace-page.test.tsx` for direct selection, URL updates, back/forward state, retained visited panels, active flags, and workspace remount behavior.
2. **Add the market category.** Expand the market type union and translations in `packages/core/paths/paths.ts` and `packages/views/locales/{en,zh-Hans}/layout.json`; wire the third URL-driven tab in `packages/views/marketplace/marketplace-page.tsx` and add its tests.
3. **Integrate the squad template catalog.** Add a market adapter that only queries saved squads while active and distinguishes pending/error/retry from a successful empty list. Add `active` handling to `BuiltinSquadCatalog`; preserve the existing template query, card copy, instance links, and project setup dialog. Add component tests for inactive query behavior and dialog lifecycle.
4. **Simplify the squad directory.** Remove the page-level workspace/templates tabs from `SquadsPage`, preserve its collection and welcome state matrix, render the agent-style description/docs header, and redirect only exact legacy `view=templates` URLs with `replace` while preserving the remaining location. Add a market entry to `SquadWelcome` and localize the page/welcome copy in both locales.
5. **Update end-to-end coverage and specs.** Change existing squad catalog journeys from the removed page tab to the market category, add a legacy-bookmark route assertion, and update `.trellis/spec/views/frontend/capability-market.md` plus the squad template navigation note in `component-guidelines.md` to describe the new owner and retained contracts.
6. **Verify.** Run targeted Core path and Views suites, then `pnpm --filter @multica/views typecheck`, `pnpm --filter @multica/core typecheck`, and `pnpm --filter @multica/views lint`. Run the focused production-Web specs for squad discovery, squad welcome, security-remediation catalog, and capability market when the required local services are available. Review both locales and narrow-screen captures; run Impeccable's changed-target detector once after UI changes.

## Expected write scope

- Core: `packages/core/paths/paths.ts` and `paths.test.ts`.
- Shared views: squad page/welcome/catalog and their tests; capability market page, new small market adapter and related test; English and Simplified Chinese `layout.json` / `squads.json`.
- Browser coverage: existing squad and capability market E2E specs that currently exercise the old tab.
- Specs: capability market and shared view component guidelines.
- Task-owned planning/evidence: this task's research and validation records only.

## Risk controls

- Do not modify `UseSquadForProjectDialog`, the template endpoint, or project mutation logic.
- Ensure squad-list errors are visible instead of rendering a false “no existing squads” state; ensure template errors retain their own retry path.
- The old route redirect must run on shared Web/Desktop navigation and preserve `hash`; keep redirect matching limited to `view=templates`.
- Inactive visited market panels must not open portaled dialogs or continue template/squad queries. Workspace identity changes must remount the market controller as before.
- Repository already has unrelated dirty paths. Stage only files created or deliberately edited for this task if commit planning later proceeds; do not include or overwrite existing workspace artifacts.

## Rollback points

- The URL category/path change and catalog adapter are isolated and can be reverted without server or database rollback.
- If the market adapter exposes a query-state defect, temporarily keep the market tab visible but gate the catalog behind authoritative squad/template states; do not treat query failure as an empty list.
- If redirect behavior fails on Desktop, retain the old category entry until the shared adapter test covers both path and hash semantics; do not move platform APIs into `packages/views`.

## Verification evidence

- Core path and tab-subject suites: 76 tests passed.
- Focused Views market/squad/project-dialog/locale-parity suites: 126 tests passed.
- Core and Views TypeScript checks passed. Targeted ESLint for changed Core/Views source and tests passed; full Views lint had 0 errors and 27 warnings in unrelated existing files.
- Playwright `--list` loaded the changed production-Web specs successfully. Browser runs were attempted against the local web server; all 7 cases stopped during `TestApiClient.login()` because the already-running API returned HTTP 403 from `/auth/send-code`, before entering page assertions. The checkout's registered API/Web ports were occupied by processes `make up` did not own, so no process was terminated. The temporary local Next dev server was stopped after the attempt.
- Impeccable changed-target detector returned `[]`.
- `git diff --check` passed.
