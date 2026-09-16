# Recent-change E2E regression coverage

Owned files: `e2e/settings-preferences.spec.ts` and `e2e/localized-template-defaults.spec.ts`.

Five representative scenarios exercise the production Web build, real Go API and task-isolated workspace fixtures. Runtime fixtures have no daemon, model process, credentials or account attached.

- Reach grouped settings through the sidebar and visit workspace, issue-model and connection pages. Disable and enable floating chat, navigate and reload, and verify the actual launcher and dedicated Chat route.
- Configure different priority-field visibility for quick/manual issue creation. Reload, verify each dialog, set a hidden priority through its overflow menu, and verify the created issue's stored priority and visible detail.
- Read Chinese/English role pickers through actual `zh-Hans`/`en` app locales. Create agents through the real API without sending `name`, and verify persisted names in the browser.
- Staff a Chinese discovery squad without an explicit name, customize an agent, then staff in English. Verify localized squad names, complete roster identity and preservation of reused agent names/instructions.
- Refuse Chinese staffing when a manually-created agent already owns a localized role name. Verify no partial agents, squads or role skills survive; English staffing remains possible with its own default names.

## Current evidence

- Initial focused run: `.omx/reports/release-v0.4.46/new-settings-localized-1.result.json`, exit 1. All five tests stopped at `TestApiClient.login` because `/auth/send-code` returned 429; no business assertion ran. The release owner subsequently restarted the task-only API with higher auth quotas. No production quota was changed.
- Second focused run: `new-settings-localized-2.result.json`. The bilingual role-default case passed. Both squad cases reproduced a real response-parsing defect described below. Two settings assertions were corrected against the existing product contracts: issue URLs use the returned human-readable identifier, and an unselected dedicated Chat page shows its neutral selection prompt.
- Settings rerun: `new-settings-3.result.json`, exit 0, 2/2 passed in 11.4 seconds against API 18446 and production Web 13446. Every settings behavior listed above ran against the real app.
- Final candidate run: `new-settings-localized-final.result.json`, exit 0, **5/5 passed, zero skips, 20.6 seconds**. API 18446 (candidate binary, PID 27764) and production Web 13446 (rebuilt bundle, PID 27771) were restarted before this run. The run verified all five scenarios, including workspace cleanup.
- Final narrow TypeScript check passed: `new-regression-typecheck-3.result.json`.
- Final shared ESLint config check passed: `new-regression-lint-4.result.json`.
- Earlier failure evidence is retained. The squad navigation and raw HTTP array assertions passed after the product fix without being relaxed.

## Confirmed squad response defect

When staffing creates every agent, `POST /api/squads/from-template` returns a valid squad with HTTP 201 but `reused_agent_ids: null`. `StaffedSquadSchema` accepts arrays or absent fields, not null, so `parseWithFallback` substitutes `EMPTY_STAFFED_SQUAD` for the entire response. The UI loses the successful squad ID and stays on the list instead of opening the created squad. All-reuse staffing can produce the corresponding null `created_agent_ids`.

Evidence: `new-regression-results-2/localized-template-default-44812-preserves-customized-agents-chromium/trace.zip` contains the real 201 body and the `API response failed schema validation: POST /api/squads/from-template` warning. The collision case correctly returned 409 and left no partial squads, agents or role skills; its subsequent English creation hit the same successful-response defect. Product repair was completed by the release owner / review agent outside these E2E files: canonical server arrays plus defensive client parsing. Both previously failing squad cases now pass, including first-create and all-reuse navigation and preservation of customized agent names/instructions.

## Finding outside this agent's write scope

`packages/core/chat/store.ts` intentionally defaults floating chat to enabled, but the English settings hint originally said "Off by default". Reported to the release owner, who corrected the hint to "On by default" while preserving stored preference behavior. The current source was rechecked after the final E2E run.

## Final maintenance of existing acceptance tests

Additional assigned ownership: `e2e/onboarding-smoke.spec.ts` and `e2e/changelog.spec.ts` only. No application behavior changed in this pass.

- The onboarding smoke test still expected three rail labels. The approved implementation intentionally displays the three persisted steps plus the inert `First project` exit row. The repaired test asserts all four labels, verifies the exit has no link/button or current-step state, and clicks its label to prove it does not advance the form. Existing source-question absence assertions remain. The test now completes the actual runtime `Skip for now` action, verifies the successful completion API call and the new workspace's `/projects` route before dismissing the welcome dialog, and cleans up its created workspace.
- The dynamic changelog test previously called `publish(first)` before authentication and before its `try/finally`. The first auth-limited suite therefore left its v0.0.1 fixture in the shared task feed; the following reader and Electron failures were contamination, not changed changelog behavior. Setup, authentication, real Git-history generation and publication now share one guarded lifetime. Restoration, optional workspace deletion and temporary-directory removal all get their cleanup opportunity. Existing real generation, hot-publication, unchanged-server/reader identity, fresh-client and stale-content recovery assertions remain unchanged.

Verification:

- `onboarding-changelog-repaired.result.json`: **6/6 passed, zero skips**, covering `onboarding-smoke.spec.ts`, `changelog.spec.ts` and unchanged `changelog-desktop.spec.ts`; total command duration 74.54 seconds.
- `onboarding-changelog-typecheck.result.json` and `onboarding-changelog-lint.result.json`: exit 0. The shared base ESLint configuration applies TypeScript/import rules without treating Playwright's `useLocalApi` helper as a React hook.
- `changelog-auth-failure-cleanup-evidence.json`: a controlled loopback server returned 429 for setup's `/auth/send-code`. The dynamic test produced its expected setup failure, while the verification harness passed: feed SHA-256 remained `9cffa341bbbd64c41dec42eeb070d4b9acbf00220266e636c3f770b58a28b6b9`, and no new `multica-changelog-e2e-*` temporary directories remained. This is an intentional fault-injection run, separate from the six passing real-business cases.
- `git diff --check` passed for both maintained specs. The task feed is restored to its two-history seed and all test/probe processes have finished.
