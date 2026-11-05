# UX live acceptance preflight

Reviewed 2026-11-05. **PASS — static preflight for the five selected cases and the final frozen runner, after the corrections below.** No blocking source finding remains. This is not runtime acceptance. The parent will resume only after the actual knowledge-window release and owns live client identity and final resource audits.

Candidate: `49e99115610fad4c88447d9580cac24498d49a1a`, isolated checkout `/Volumes/artisan/code/2026/mc-ux-e2e-49e991156-20261105`. The parent owns builds, services, database allocation, test execution, final identity checks and cleanup. No tests, browser, service or database command was executed by this reviewer. The parent subsequently assigned static review of `/Volumes/artisan/e2e-private/ux-acceptance-20261105/{run-ux.py,db-control.mjs}`.

The injected task requirements, design, execution plan, database/environment specs, parent acceptance plan, fixture-impact report and recipient static review were read. A later user priority gave knowledge one short retry window; all UX tests remain NOT_STARTED at this review. A static clearance does not supersede that runtime handoff or authorize this reviewer to start services/tests.

## Findings (fixed)

- File: isolated `e2e/collaboration-delivery-web.spec.ts:149`.
- Issue: both a file-bearing comment and the issue-level attachment panel now expose a region named Attachments. The page-wide locator could target both sets of file controls.
- Fix: applied only the reviewed single-hunk `delivery-attachment-locator-proposal.patch`, filtering by the issue panel's own Attachments heading. Comment attachment regions have no such heading. Preview, range bytes, full download bytes, retained delivery, account WebSocket revocation and former-capability denial assertions are unchanged.
- Verified proposal SHA-256 `39d901efbea055f422c82118638d3b4b2c1afe66387ca2d4a140bd49b2dd649a` and base SHA-256 `2abe357d3dda931edae450548183a9501d00daec98c124c28227f23b4342f110` against the parent's recorded proposal. Base bytes equalled frozen HEAD; `git apply --check` and subsequent `git diff --check` passed. The original-root delivery source was unchanged.

## Reviewed spec identities

| File under isolated `e2e/` | Tests | SHA-256 after the authorized fix |
| --- | ---: | --- |
| `collaboration-recipient-picker-web.spec.ts` | 1 | `4975f7e6198cac880216138b71b0bdfa0099c636f20416e808caae21de0b12a4` |
| `collaboration-delivery-web.spec.ts` | 2 | `11be1b7e11c08cd5212ffc0c1d345f5b28b50ed0b56325f1f0d36e916b63ca32` |
| `collaboration-administration-web.spec.ts` | 1 | `c21ac3ab9b0454577b06138295f30ae739d8f663101917c54cd48d2b0cd85a0c` |
| `collaboration-administration-desktop.spec.ts` | 1 | `b04bf8e9d3be24695c72feb16ac731dca0e9cc5e5582adfefcd507b497a3b79b` |

There are **five canonical cases in four files**. The two delivery cases respectively cover uncertain upload/create recovery with real file revocation, and explicit correction after a retained typed creation refusal. Do not report a four-case total.

## Fixture and product compatibility

- Recipient and delivery use `TestApiClient.login`, real legacy email-code endpoints and direct access to the same task database. They need matching explicit loopback API/browser API origins, production Web, signup and creation enabled. Their retained accounts/issues require a disposable task database, not generic workspace deletion. The delivery correction case intentionally changes only its synthetic recipient's `disabled_at` inside that database.
- Administration requires password mode, device/Cloud auth disabled, single-process password limiter, platform administration enabled, `E2E_PLATFORM_ADMIN=1`, `E2E_PASSWORD_AUTH=1`, and an injected synthetic super-admin with no workspace membership. Each admin case registers three additional synthetic users. Distinct admin databases/accounts preserve real reauthentication and signup limits.
- Admin Web uses the real password form and real cookies. Its explicit trace/screenshot defaults remain off; only later content-safe captures are saved. Native admin uses a bearer token from the real API password login, a private Electron profile and the account administration route. It does not claim a native password-form interaction.
- `collaboration-native.ts` launches the explicit checkout's Electron executable, `collaboration-account-electron.mjs`, built preload and configured renderer URL. The harness compiles selected production Main routing/IPC functions with existing TypeScript/esbuild dependencies. It stubs runtime discovery/daemon lifecycle, rejects actual start/restart/install, and blocks non-loopback native network destinations. No Docker, provider, paid model or installed Main prerequisite was found in the selected import/call paths. Native dependencies and actual renderer/preload identity still need runtime admission by the parent.
- Recipient pending-state assertions correctly require a visible popup/search plus an attached busy listbox; an empty listbox may have zero height. The held response comes from the real API, is fulfilled unchanged, and the distinct name substring is an uncached query. The current-response waiter is installed immediately before release. Pending Enter requires no active option, no enabled row, unchanged self identity and zero creation POSTs. The explicit create action observes click/response together, then checks exact recipient UUID in the request, response and reads by both authenticated actors.
- Delivery's exact-account helper opens the current Recipient picker and uses its explicit full-ID lookup; importing that helper does not invoke its separate password-only workflow. The current comment composer still exposes Preview delivery and Send comment, matching this spec despite also supporting the newer one-action ordinary flow.
- Admin metadata, controls, successor radio group, original receipt recovery and denied ordinary-content reads match the current shared components and service contract. Native captures keep the two-frame layout stability, frozen offsets, compositor and full coverage assertions.

## Runner findings corrected by the parent

The private runner is parent-owned; no runner bytes were edited by this reviewer. All reported blockers were corrected and the final bytes were re-read and hash-verified:

1. Source identity now checks HEAD, all tracked file bytes/modes, initial staged and unstaged changes against the two allowed paths, the exact reviewed delivery hash, and rejects nonignored untracked application source. Next declaration normalization is checked against the candidate's original blob, preserving all other content. Every existing app/package `@multica` workspace link is bound to this checkout; the reviewer independently inspected all 22 current links. Server/migrate binaries, Web build ID/manifests/server/static output and all Desktop output are fingerprinted before each lane, afterward and at final audit.
2. API and Playwright run in owned process groups; stopping no longer assumes leader exit proves group exit. Installed Playwright `1.58.2` explicitly launches Electron/browser processes detached, so the parent also added descendant observation every 250ms, exact observed PID/command plus private-path cleanup, and per-lane/final remaining-process counts. Final checks retain database connection counts on ordinary failure paths. The parent separately owns the actual Web/renderer listeners and post-run resource audit.
3. Exact selected-file counts, single `passed` terminal, one worker and zero retries gate each lane. Final `acceptance_status` requires successful rows, unchanged source/artifacts, zero database connections and zero remaining browser processes. The last exit guard now directly requires `acceptance_status == 'PASS'`, including a failure in final browser cleanup.
4. The DB helper rejects names longer than 63 ASCII bytes and existing databases, accepts only the task prefix, and clones only the designated template. Both probes require the exact requested database identity. Pre-fixture checks require 752 migrations, zero users/issues/admin roles and creation on/execution off/host off. Setup rows precede clone/start, preserving early failure identity. Retained DBs are not dropped by this runner.
5. The process environment is allowlisted, then configured explicitly per auth lane, with fresh task-external HOME/TMPDIR, bounded API pools, disabled Cloud/device/knowledge execution and no inherited provider credentials. The directory tree and new raw logs/config/credentials use private permissions. Separate synthetic admin accounts are bootstrapped and verified through the real API; the selected native harness has no provider or container dependency.

Final frozen inputs:

- `run-ux.py`: `96d9e32e9c9ad943b08f62d7d35c84c76ddf05a5bcd86c9aabc5db3195980998`.
- `db-control.mjs`: `d8c944102ce5030c40e6cdf025c559d5edc5d2f6483d69fa9d74b0246e60e5ee`.

## Findings (not fixed) and limits

No remaining blocking finding in the assigned static scope. These limits remain:

- Existing admin cases run English controls and real privilege/receipt journeys; they do not add bilingual status assertions. Prior component/localization evidence must retain its own identity. These runs also do not close the separate OtherTarget directory-failure branch or installed Main/model acceptance.
- Earlier response waiters in existing recipient/delivery/admin steps are armed before the triggering click without joint promise observation. A failing click can leave a waiter until timeout/disposal; the lost-response admin route is also removed only on its successful recovery path. The reviewed new recipient hold/create sequence improves those local boundaries, but no experimentally verified fault-cleanup claim is made for all older steps. Unrelated helper edits were outside this reviewer's authorized write scope.
- Runner cleanup and source checks were inspected and parsed, not experimentally fault-injected. The parent must evaluate actual lane results and final process/database/listener evidence; static review does not establish that any test, timeout recovery or cleanup ran successfully. The parent reports the production client PID/cwd and rendered API/WS proof; this reviewer did not execute those live probes.
- These test-only compatibility changes do not introduce a new product contract; no `.trellis/spec/` change is required. The known attachment-region distinction is recorded here and in the parent's attachment-selector follow-up.

## Verification

- Authorized patch base/hash/apply and whitespace checks: PASS.
- Syntax-only parse: PASS, zero diagnostics across the four specs and seven directly relevant helpers, using TypeScript `5.9.3` resolved inside the isolated checkout. No test module was imported/executed. Private Python runner AST parse also passed.
- Lint: NOT_RUN by this reviewer; explicitly restricted to static inspection/parse. Root E2E files are outside current workspace lint coverage, so an ignored-file success is not fixture lint evidence.
- TypeCheck: NOT_RUN by this reviewer; syntax parsing is not a type-check.
- Tests/build/browser/database/native execution: NOT_RUN by this reviewer; assigned to the parent.
- Source edits: only the isolated delivery locator above. Original-root write: this report only. The parent's generated `apps/web/next-env.d.ts` change was observed and left untouched.
