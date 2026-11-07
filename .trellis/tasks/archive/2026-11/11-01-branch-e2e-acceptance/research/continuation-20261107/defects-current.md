# Current-candidate defects

Candidate: `4c817dee9397a42ac903f419af7804b11d2d863e`. Product code remains unchanged by this acceptance task.

## QA-20261107-001 — New squad creation routes are missing from diagnostic bucketing

- Status: OPEN, independently reproduced; S2 test-gate/observability regression. This finding alone prevents declaring all checks green. It does not demonstrate a broken squad submission or a privacy leak.
- First execution: `frontend/units-reconfigured` — core331 files,4460PASS/1FAIL; Turbo stops before remaining packages. Independent remaining-package run is separate.
- Reproduction: `pnpm --dir packages/core exec vitest run diagnostics/diagnostic-context.test.ts --maxWorkers=1` on the frozen candidate;15 tests,14PASS/1FAIL. Private JSON: `.omx/private/branch-e2e-20261107/frontend/diagnostic-route-reproduction/results.json`.
- Exact failed test: `route coverage stays in step with paths.ts > recognizes every builder — no builder falls back to the mask`.
- Expected: every canonical route builder has a known, identity-free diagnostic template.
- Actual: `/workspace-slug-value/squads/new/manual` becomes `/:slug/squads/*`.
- Root cause: commit `e1312009e` introduces `newSquadManual` and `newSquadTemplate` in `packages/core/paths/paths.ts:91` without updating `WORKSPACE_ROUTES` in `packages/core/diagnostics/diagnostic-context.ts:53`. The registry has only `squads` and `squads/:id`. The template route also lacks a structural match; chooser `/squads/new` is categorized as an ID route.
- Scope: freeze/watchdog diagnostics lose the specific creation screen. Existing masking continues to hide unknown segments; the separate privacy assertion passes.
- Suggested repair: add explicit squad chooser/manual/template route patterns at the existing diagnostic boundary, following agent-creation routes. Preserve fallback masking and the builder-parity/privacy tests. Do not weaken the assertion or edit path builders to hide the mismatch.
- No repair performed: approved acceptance R08 forbids product changes; this report is the reproducible repair handoff.

## QA-20261107-002 — Drawer resource helper cannot match numbered buttons

- Status: test-only repair independently reviewed and integrated; original failures retained.
- Candidate helper `e2e/fixtures/collaboration-disclosures.ts:18` builds a RegExp from a template string with a single backslash before `d`; JavaScript consumes it, so the resulting suffix is `d+`, not a numeric count.
- Actual original native resource journeys time out looking for Attachments/Sub-issues controls with the malformed regex. The form/API behavior after those steps is not proved by the interrupted run.
- Repair: escape the backslash inside the JavaScript template literal; preserve exact section title, count suffix, drawer accessibility assertion and every subsequent business assertion.
- Private patch, before/after hashes and source-equivalence audit: `.omx/private/branch-e2e-20261107/browser/disclosure-helper.patch`, `harness-variant.json`, `harness-product-equivalence.json`.
- Original legacy batch normalized:7PASS/2FAIL/2conditionalSKIP/42unexecuted (one interruption-skipped case plus41 with no result). Raw framework records are7PASS/2FAIL/3SKIP/41 without results. The complete repaired-harness batch recorded37PASS/7FAIL/9SKIP; subsequent bounded drawer-contract corrections proved six originally failing journeys passed. These are separate runs, not a replacement of the original outcome.

## QA-20261107-003 — Updated progress-report Skill no longer receives localized default presentation

- Status: OPEN; S2 localization/contract regression, independently reproduced.
- First execution: views598 files,7236PASS/3FAIL; full file reproduction109 tests,106PASS/3FAIL. Raw JSON: `.omx/private/branch-e2e-20261107/frontend/skill-presentation-reproduction/results.json`.
- Three failures: real source descriptions with only `en` loaded; with only `zh-Hans` loaded; default-description comparison for `multica-progress-report`.
- Expected: current built-in source descriptions match their current locale catalog and localize correctly without changing custom workspace descriptions.
- Actual: the backend Skill description starts `汇总当前项目的日报、周报时使用。`; Chinese catalog still starts `汇总工作空间或项目的日报、周报时使用。`; English catalog still describes workspace/project scope. `getDefaultDescriptionKey` sees no exact default match, so the English UI returns the Chinese source description and drops the default cross-language search description.
- Root cause: scope-hardening commit `cd95c8e40` changed `server/internal/service/builtin_role_skills/multica-progress-report/SKILL.md:3`; corresponding `packages/views/locales/{en,zh-Hans}/skills.json:47` defaults were not synchronized. `packages/views/skills/lib/skill-presentation.ts:27` deliberately preserves unrecognized/custom descriptions, which is correct protection.
- Suggested repair: align both current locale descriptions with the approved current-project scope; explicitly decide how retained shipped defaults should present, preserving customization and least-privilege semantics. Do not revert backend scope limits or broadly translate arbitrary descriptions.
- No product repair performed under acceptance R08.

## QA-20261107-004 — Account detail scroll position moves after scope restoration

- Status: OPEN, S2 state/interaction stability defect; exact shrinking element and root cause not yet established. No product change or assertion relaxation.
- Original production-Web detail journey fails saved620→actual476 after Workspaces→Collaboration. The initial fixture asserted max-scroll>700 before saving620, so this was not an unsupported initial position.
- A separate diagnostic preserves the exact620 assertion: WebPASS, DesktopFAIL at476. Desktop first restores620, changes660→620 and then476 while content height changes3265→3121. Final maximum2333 remains above620, ruling out simple short-content clamping.
- The final description position plus scrollTop is37+476=513; the earlier position plus saved scroll is-107+620=513. The description retained its document position while its viewport position changed144px. These observations support a visible restoration shift; they do not identify the specific element or browser effect responsible.
- Source: `e2e/fixtures/collaboration-detail-journey.ts:774` (scope restoration); original raw `browser/runs/detail-journey-first`, separate diagnostic `browser/runs/detail-diagnostic-1`. Independent source/trace review in `drawer-failure-analysis.*`.
- Impact: switching between workspace and collaboration contexts can move the user's reading position after restoration. A later instrumented Webpass does not close the earlier Web failure or the independently observed Desktop failure.
- Suggested next investigation: correlate layout changes and scroll writes/anchoring after restored mount, then fix the owning scroll/layout boundary with the unchanged scope-switch regression. Do not increase tolerance or treat476 as equivalent to620.

## Drawer contract adaptations and non-defects

The new approved UI intentionally uses modal resources and a visible Needs attention entry for pending operations. Closing resources before acting on the background, reopening them for resource assertions, and entering recovery through that explicit attention button are legitimate current-contract fixture changes. Six affected account/delivery/records/knowledge/memory journeys passed after these bounded adaptations. The recovery trace and passive request observer establish an original-key GET result and zero POST after reload; pending state was retained, not lost.

The original Desktop comment deletion waited for a DELETE while stale-revision protection disabled confirmation. The fixture now waits for the exact edited text before opening deletion, and the later Desktop journey passes that stage before encountering QA-20261107-004. This is not evidence of a bypassed stale-revision gate.
