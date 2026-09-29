# Skill library separation verification

Verified on 2026-09-29. The approved option 1 is implemented in the shared Web/Desktop skills views.

## Delivered behavior

- Workspace skills and Skill templates have separate tabs and counts; the page heading no longer repeats the workspace count.
- Workspace management shows no deployment template shelf, catalog messages or template cards. From template opens the catalog and focuses its persistent tab.
- Template discovery retains source/category/search, named previews, related-copy links, independent-copy editing and focus restoration. A successful catalog copy stays in the catalog; ordinary creation still opens the new detail.
- Empty workspace initialization, explicit tab selection, unknown/error counts and mounted workspace collection behavior are preserved.
- Shelf-only imports, responsive shelf classes, compact skeleton branching, collapse state/action/persistence and locale keys are removed. Saved view/source/category and workspace list preferences survive old persisted payloads.
- English/Chinese labels, public docs and the embedded Chinese skills page are synchronized. Chinese deployment source labels agree in the catalog and preview.

## Verification evidence

| Check | Result |
| --- | --- |
| Views skills and locale parity | 26 files, 438 tests passed; rerun after final locale edit |
| Core skills view-store | 1 file, 19 tests passed, including legacy shelf preference payload |
| Docs bundle tests | 4 files, 43 tests passed |
| Embedded Go docs | `go test ./internal/docs/...` passed |
| Workspace lint/typecheck | `pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force`: 15 tasks passed, no cache |
| UI export static check | `pnpm check:ui-exports` passed, 62 files |
| Production Web build | `next build --webpack` passed |
| Docs generation | 36 pages and 58 assets generated |
| Trellis manifests | 4 valid entries in each manifest |
| Diff whitespace | `git diff --check` passed |

Four browser scenarios passed against the production Web build and local API with real authenticated workspace setup and cleanup:

1. `e2e/skill-market.spec.ts`: deployment discovery, keyboard navigation, real copy creation, retained browsing context, persistence, distinct counts and absent workspace shelf. Only the template GET is augmented with synthetic deployment templates.
2. `e2e/skill-template-creation.spec.ts`: unmodified real built-in catalog, preview/cancel without writes, edited copy creation, supporting-file preservation, dirty navigation guards, related links and focus.
3. `e2e/localized-template-defaults.spec.ts` (`specialist skills localize`): English/light and Chinese/dark, 1280/375/360px, visible From template entry and tab focus, localized lookup and unchanged stored copies.
4. `e2e/workspace-defaults.spec.ts` (`keeps built-in resources available`): fresh workspace template access without a runtime, with existing project setup behavior preserved.

Browser outputs and screenshots are under `.omx/reports/skill-library-separation-20260929/`: `final-browser/`, `remaining-browser/`, `localized-final-verified/`, and `workspace-defaults/`. The localized capture names distinguish workspace and catalog at the same viewport width. Visual review passed at 94/100; persisted verdict: `.omx/state/skill-library-separation/ralph-progress.json`.

## Findings resolved during resume

- A read-only reviewer found the old Chinese source label in the template preview; it now reads 部署提供.
- The first browser attempt requested the retired API port 18572 while the registered environment serves 18573. Starting with the registered `check.env` corrected the environment; no product workaround was introduced.
- Existing E2E expectations still looked for the deleted heading counts and old Skill market name. They now assert tab counts and Skill templates. An existing equal-name screenshot overwrite was corrected while adding bilingual evidence.
- Final independent delta review reported no blocking findings.

## Limits and remaining risks

- Existing unrelated lint warnings and Next CSS `::highlight` optimizer warnings remain; no lint/typecheck/build errors.
- Browser interaction was verified in Chromium Web, not an Electron native window. Desktop consumes the same shared views and passed its typechecks.
- This task adds no backend API, dependency, database migration or mobile UI. No production deployment or real agent/model execution was performed.
- Other active sessions are editing/committing in this checkout. Only task-owned skill files and evidence are included in this task's commits.

## Local logs

- `/tmp/skill-separation-views-final.log`, `/tmp/skill-separation-core.log`
- `/tmp/skill-separation-static-final.log`, `/tmp/skill-separation-static.log`
- `/tmp/skill-separation-docs.log`, `/tmp/skill-separation-docs-generate.log`
- `/tmp/skill-separation-browser-final.log`, `/tmp/skill-separation-browser-remaining.log`, `/tmp/skill-separation-localized-final-verified.log`, `/tmp/skill-separation-workspace-defaults.log`

The first browser log contains the initial stale-count failure; the later creation scenario passes in `browser-remaining.log`.

## Delivery

Implementation commit: `eb6b3d5afc0250ca1db8f75838f993b577d2e71f`. Changed files: shared catalog/page and tests, core view store and tests, English/Chinese skill locale bundles, four affected E2E specs, skill discovery spec/index, English/Chinese skills docs and generated embedded skills page. No unrelated files are in this commit.

A concurrent session later restarted the shared API with an origin allowlist that excludes port 13493. The final localized screenshot run used the same production build on allowed port 13492, verified its preflight response, and passed in 13.4 seconds. This was environment recovery, without changing application behavior.
