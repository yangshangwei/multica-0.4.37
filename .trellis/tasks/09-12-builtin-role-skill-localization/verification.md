# Verification

## Completed checks

- `pnpm --filter @multica/views test`: 421 files, 5,081 tests passed.
- `pnpm typecheck`: all 9 Turborepo tasks successful, including shared views, web, and both desktop TypeScript targets.
- `pnpm --filter @multica/views lint`: exit 0; zero errors and 26 warnings in unchanged files.
- `git diff --check`: passed.
- Impeccable detector over all changed UI source files: exit 0, no findings.
- Independent review: approved after fixing paused metadata requests and locale-dependent slash-name ranking.
- Actual-component browser harness: all 13 checks passed; no console/page errors or external/API requests. Visual verdict: 94/100, pass.

## Regressions covered

- All seven built-in names and default purpose descriptions.
- Same-name custom skills, renamed skills, missing/mismatched provenance, customized descriptions, and unknown built-ins.
- Chinese and English names/descriptions remain searchable with only the current locale loaded, matching production resource loading.
- Chinese exact-name ranking remains above description matches in an English UI with a 20-result limit.
- Offline/paused metadata never blocks existing slash suggestions; cached/raw matches do not wait for a slow refresh.
- Picker callbacks, toggle/remove mutations, and slash invocation nodes preserve original IDs and canonical names.
- Pristine and dirty detail drafts survive locale changes; save payloads retain raw properties and SKILL.md content.
- Existing source-link middle/left-click navigation regressions remain covered.

## Existing findings outside this change

`pnpm exec knip --workspace packages/views --no-progress` exits 1 for five pre-existing unused files: inspector concurrency-picker, skill-attach, visibility-picker, chat quick-agent-bar, and labels/index.ts. These paths and their orphaned references predate this change; no new unused files were reported.

At a 375 px viewport, the existing skill list grid has a 392 px internal minimum width. The outer page remains 375 px wide and all translated names fit. Grid tracks and column widths are identical to HEAD; detail and picker have no horizontal overflow.

## Browser scope and artifacts

Browser QA mounted the real shared components and real CSS/fonts with in-memory fixtures, one active locale bundle at a time. It did not verify a live backend, persisted user workspace data, or a packaged Electron application. No server files or skill body sources changed.

Local artifacts are under `.omx/qa/skill-localization/`: `browser-results.json`, `visual-verdict.json`, and desktop/narrow screenshots. Full command logs and the persisted verdict are under `.omx/state/agent-skills-localization/`.
