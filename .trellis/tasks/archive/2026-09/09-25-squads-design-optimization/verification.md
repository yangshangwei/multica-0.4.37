# Verification — squad discovery and management

Completed on 2026-09-25 against the approved PRD and design.

## Result

- Workspace squads are the default view; `?view=templates` opens the separate catalog and survives refresh/detail navigation/Back.
- A single header creation menu retains template staffing and custom creation. The empty view exposes both paths.
- Template browsing uses one scrolling pane, a responsive catalog, concise use-case summaries, full role rosters, and native links to every matching active instance.
- Native squad links, selected scope semantics, named icon buttons, separate clear-filter controls, and isolated row actions work with keyboard input.
- Wide list columns keep identity and metadata together. Creator/date columns are opt-in for fresh preferences; existing saved arrays survive workspace rehydration.
- Custom squad text and the original template configuration payload remain intact. Creator/admin permission gates and query retry flows are preserved.
- All four locale structures match. Chinese labels follow the AI小队 glossary; concise Japanese tabs fit at 390px.

## Executed checks

| Check | Result |
| --- | --- |
| Views: squads, project-configuration dialog, create-squad modal, locale parity | 208 tests passed, 5 files |
| Core: squad view preferences and workspace rehydration | 6 tests passed |
| Production Chromium: discovery/keyboard/preferences/project setup; existing no-runtime workflow | 2 passed, 0 retries |
| Core and views TypeScript | Passed; views rerun after final test edits |
| E2E TypeScript (`tsc --noEmit`, bundler resolution) | Passed |
| Core and views ESLint | 0 errors; 28 existing warnings outside the squad changes |
| Changed squad components/tests ESLint | 0 errors, 0 warnings |
| UI wildcard exports and `git diff --check` | Passed |
| Impeccable detector on both changed components | No findings |
| Production Next.js build | Passed |
| Visual review at 1920, 1440, 768 and 390px | 94/100; see `evidence/visual-verdict.json` |

Commands used for the focused tests:

```sh
pnpm --filter @multica/views exec vitest run squads projects/components/use-squad-for-project-dialog.test.tsx modals/create-squad.test.tsx locales/parity.test.ts
pnpm --filter @multica/core exec vitest run squads/stores/view-store.test.ts
make env-exec ARGS="-- env PLAYWRIGHT_BASE_URL=http://localhost:13593 pnpm exec playwright test e2e/squads-design.spec.ts e2e/workspace-defaults.spec.ts --grep 'squad discovery|keeps built-in' --workers=1 --retries=0"
```

## Environment and evidence

The working checkout also contained ongoing automation-template changes. The shared launcher's source-consistency guard correctly refused two builds whose input changed during compilation. Production browser verification therefore used a fixed copy of the web app and shared packages under `.omx/squads-design-validation/source`; all seven changed squad product files were hash-checked against the working tree. The checkout's normal Web service was restored to development mode.

The isolated Web preview used the existing same-origin proxy with `REMOTE_API_URL=http://localhost:18572`. Direct browser requests initially failed because preview port 13593 was outside the local API's CORS allowlist. No product code, shared API configuration, or browser security setting was changed to resolve that environment mismatch.

The new E2E fixture initially used PATCH for squad editing; the existing API uses PUT. The fixture was corrected after the 405 response. Narrow screenshots now await the selected tab, avoiding a false readiness signal from an instance link visible in both views.

Production build identity and product-source hashes are in `evidence/provenance.json`. Six representative screenshots are stored alongside it. Full temporary logs and the source snapshot remain under `.omx/squads-design-validation/`.

## Review and limits

Independent review found no blocking regression. Its sort-tooltip inconsistency and Chinese glossary issue were fixed. Permission and nested-action coverage was added; all 13 page tests pass.

No new dependencies, backend APIs, or migrations were introduced. Native Electron interaction and the Go test suite were not rerun; shared components and adapters were exercised in unit tests and production Chromium. Existing lint warnings and CSS optimizer warnings for the browser's `::highlight` API remain outside this task.

The shared E2E file also contains an automation-button wording change from the other task. Only its squad-tab migration belongs in this task's commit.
