# Progress Reporter and Daily Report Implementation Plan

> Execute in this session using bounded native subagents, test-first changes, and an independent final review. The user has authorized implementation and release.

**Goal:** Ship one built-in Progress Reporter and an adjacent daily report automation in v0.4.45, with verified intranet and Windows artifacts.

**Architecture:** Add entries and canonical content to the existing role/role-skill and automation registries. Reuse template creation, shared catalogs, task comments and review-triggered automation completion.

**Tech Stack:** Go/Chi/sqlc, React/Next/Electron, Vitest, Playwright, pnpm, Docker, electron-builder.

## 1. Role lane

- Own `server/internal/service/builtin_agent_templates_roster.go`, `builtin_agent_templates/progress-reporter/INSTRUCTIONS.md`, `builtin_role_skills/multica-progress-report/`, registry/role-skill tests and agent-template handler regression tests.
- Add a failing test for the ninth role, its Contributor/default skill contract and report-only boundaries. Run the focused test and record the expected failure.
- Add the role and its skill using the existing embedded registry; update count/mapping expectations and preserve all existing role mappings.
- Run focused Go tests. Do not change automation files, release scripts or shared docs owned by other lanes.

## 2. Automation lane

- Own `server/internal/service/builtin_autopilot_templates_roster.go`, daily and weekly prompt files, autopilot-template service/handler tests.
- First assert daily exists directly beside weekly with a 24-hour window, daily 18:00 schedule, dated title, correct output mode and report-only review transition.
- Implement the daily entry and update weekly default version/closeout wording, preserving stored-copy behavior.
- Run focused tests for the catalog, template creation, real dispatch/claim and no-overwrite contract.

## 3. Browser and documentation lane

- Root owns shared UI/locale wiring if needed, `.trellis/spec/server/builtin-templates.md`, relevant docs, generated docs bundle, and `e2e/progress-reporting.spec.ts` plus related existing E2E expectations.
- Use the real API/runtime fixture pattern in `e2e/autopilot-template-zh.spec.ts`; assert role catalog → preview → actual creation, localized identity/default skill, adjacent daily/weekly catalog items, each actual automation create body/persistence, schedule/timezone, run issue linkage and report closeout without modifying business tasks.
- Include empty-period/evidence-missing assertions at the appropriate contract layer. Capture desktop-sized and narrower catalog/preview screenshots.
- Run the focused browser spec, fix actual failures, then run the full E2E suite. Save results and visual verdict JSON.
- Sync all affected documentation and regenerate the in-app Chinese bundle.

## 4. Verification and review

- Establish unchanged registry baseline before implementation.
- Run `pnpm typecheck`, `pnpm test`, `pnpm lint`, `pnpm knip`, Go tests via the repo wrapper, `go vet ./...`, and the full Playwright suite against the isolated environment. Use fresh results after any fixes.
- Run relevant release/package script tests and builds. Add meaningful regression coverage if a build/version defect must be fixed.
- Independent reviewer first checks this PRD, then code/content quality, trust/permission boundaries and verification evidence. Resolve findings before publication.

## 5. Release and artifacts

- Follow the previous release runbook under the main checkout `.omx/reports/release-v0.4.42-20260913/` and `dist/release/v0.4.42/`.
- Commit only this task's files with conventional intent-first Lore messages. Integrate to main, push to origin and create new v0.4.45 tag after successful checks. Specify `--repo yangshangwei/multica-0.4.37` for GitHub operations.
- Wait for the actual release run to finish and fetch the published changelog. Build the Linux amd64 offline upgrade archive with the released source/version and changelog; smoke it with an isolated PostgreSQL volume, verify both new catalogs, UI version, migrations and offline assets.
- Build Windows x64 (pending architecture clarification) with `MULTICA_DESKTOP_VERSION=0.4.45`, `CSC_IDENTITY_AUTO_DISCOVERY=false`, `--win --x64 --publish never`; preserve outputs before another build wipes desktop dist. Verify PE/ASAR/CLI/version/blockmap/checksum. Native Windows CI is available as a fallback and must not be mistaken for GUI testing.
- Publish verified installers/archives to the fork release and write a local delivery README/checksums. Preserve the release changelog assets.
- Archive task/journal after work commits, then audit every PRD acceptance item against actual output before completing the goal.
