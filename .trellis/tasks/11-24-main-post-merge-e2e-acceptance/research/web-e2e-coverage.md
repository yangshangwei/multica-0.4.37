# Research: Web and shared-business E2E coverage

- Query: What does current main cover through browser E2E, what requires separate acceptance lanes, and what must a complete post-merge plan add?
- Scope: Internal repository inspection; no tests or services executed and no product/test code changed.
- Date: 2026-11-24
- Task: `.trellis/tasks/11-24-main-post-merge-e2e-acceptance`

## Findings

### 1. Current inventory and collection caveat

Static TypeScript AST inspection of the **187 top-level `e2e/*.spec.ts` files** found **465 `test(...)` declaration sites**, **122 `test.skip(...)` call sites**, **0 `test.only`**, **0 `test.fixme`**, and **104 page/context route interception call sites**. These are source counts, **not executed cases or expanded Playwright case counts**: loops over platforms, languages, themes, roles and boundary values create additional cases; suite-level `skip` affects multiple declarations. A subsequent fresh read-only collector expanded these to **538 cases across 187 files**, with **468 declaration-time expected-pass and 70 declaration-time skips**, zero collection errors and zero test bodies executed (section 7). Runtime skip checks inside bodies still require a compatible actual run; reconcile every collected identity against the final reports.

The standard configuration is deliberately **Chromium only**, **one worker**, **zero retries**, 60-second default test timeout and headless browser. Several explicit journeys extend their own timeout. It starts **no server**, and resolves Web origin from `PLAYWRIGHT_BASE_URL`, `FRONTEND_ORIGIN`, then localhost:3000 (`playwright.config.ts:1-21`). A green command against an arbitrary preexisting port does not bind the result to main.

Approximate source grouping (one owning category per filename):

| Area | Spec files | Static test declarations | Skip call sites | Existing behavior examples |
| --- | ---: | ---: | ---: | --- |
| Authentication / onboarding | 12 | 30 | 6 | Email-code login, redirect/logout, password registration and recovery, account binding, device identity, onboarding questionnaire |
| Issues / comments / core business | 13 | 49 | 3 | Manual CRUD, status/priority optimistic rollback, batch edit/delete, board drag, assignees, comments, live inbox and simulated execution lifecycle |
| Projects / iterations | 19 | 75 | 1 | Goals, overview, risk/health, progress publication/correction, planning timezone, start/close/carryover, frozen history, scope audit and desktop parity |
| Triage / inbox | 8 | 30 | 1 | Intake/accept/reject/snooze/duplicate/disable/reopen, stale revision and idempotency, personal inbox snooze and cross-device wakeups |
| Agents / squads / automation | 20 | 79 | 9 | Role provenance, autonomy ladder, staffing, category/discovery, defaults, quick create, AI refinement/recommendations |
| Capabilities / runtime | 13 | 20 | 2 | Skill/MCP market, templates, assignment, welcome/recovery, runtime connection discovery |
| Collaboration (including knowledge) | 66 | 103 | 83 | Workbench, source/account views, creation, resources, human delivery/approval, mentions, private records, runtime controls, offline/recovery, multi-account and knowledge |
| Administration | 9 | 11 | 9 | Synthetic super-admin initialization, accounts, resource publishing, installations, observability, execution controls and feature switches |
| Plugin / iframe embed | 3 | 11 | 0 | Guest port handshake, redirect refusal, frame isolation, document sanitization/scroll |
| Navigation / settings / remaining | 24 | 57 | 8 | Workspace access/defaults/naming, sidebar/history, global/account scope, preferences, locale migration, changelog and chat |
| **Total** | **187** | **465** | **122** | Loops expand actual collection |

### 2. Real integration boundaries

- `TestApiClient` is in **`e2e/fixtures.ts:48`**, not a helpers subdirectory. API base is `NEXT_PUBLIC_API_URL` or the configured API port (`e2e/fixtures.ts:13`); database URL is selected separately (`:14`). Ensure both point at the same owned allocation.
- Most business fixtures use real HTTP APIs and actual PostgreSQL rows. Email-code setup reads its one verification code from the owned DB (`e2e/fixtures.ts:118`), many default tests inject the resulting session into localStorage (`e2e/helpers.ts:31-58`), and default onboarding is marked directly in SQL (`e2e/fixtures.ts:214`). Thus these tests do not all prove user-facing login or onboarding; the explicit authentication/onboarding journeys must remain in the plan.
- `main-supplemental-qa.spec.ts:70`, `:109`, `:165` cover actual email-code entry, manual edit/comment/delete and creating a second workspace through the questionnaire/runtime flow. `auth.spec.ts:6-35` and `onboarding-smoke.spec.ts:17-137` add guards, logout, answers/skip and Chinese display.
- The **G1–G8** core suites have actual browser/API/SQL/WS behavior but use **`SimulatedDaemon`**, which speaks the HTTP daemon protocol and does not run a real daemon executable, CLI or model (`e2e/fixtures/core-journeys.ts:6-9`, `:99-143`). Titles mentioning a daemon/live reply must not be reported as real-process/model acceptance.
- Some cases use `page.route` to inject one lost response, 500 or malformed/late response while keeping the underlying server path for recovery; others fully mock discovery/tool data or plugin remote documents. Record each boundary accurately. Examples: `password-branches.spec.ts:224`, `issue-table.spec.ts:453`, `quick-create-actor-picker.spec.ts:301`, `plugin-surface-security.spec.ts:6`.
- Native tests use the actual Electron executable and this checkout's built renderer/preload, but `openDesktop` launches **the fixture Main entry** (`e2e/fixtures/collaboration-native.ts:18-28`). `start` waits for the real renderer document (`:55-63`). This proves renderer/native IPC/navigation behavior, not packaged production Main, installation or updater behavior.
- `collaboration-knowledge.spec.ts:18-25` is unconditional and explicitly requires the **legacy** lane, enables human creation only and holds execution/host gates closed. Eight human-knowledge cases include proposal, exact review, correction, derivation, purge, ambiguous original-key GET recovery, target-only authority, expiry and native source/target review (`:89-293`). A broad password run would fail this beforeAll; select compatible files for each lane.
- Actual local runtime/provider/CLI fixtures exist for host and cross-client journeys; their deterministic output verifies orchestration, writeback, isolation and termination, not model reasoning or authenticated paid-account behavior. `hostEnvironment` requires explicit absolute binaries, matching API origins, password actors and an isolated DB (`e2e/fixtures/collaboration-host-runtime.ts:104-118`). Cross-client execution has separate explicit Linux fixture paths and host address (`e2e/fixtures/collaboration-cross-client-execution.ts:44-47`).

### 3. Required acceptance lanes and prerequisites

| Lane | Required binding | Coverage that otherwise skips / remains unproved |
| --- | --- | --- |
| Broad legacy production Web + native renderer | Fresh API/Go DBs, unique synthetic JWT secret, current source and production builds, exact Web/API/WS/renderer origins, built desktop preload | Ordinary workflow, human collaboration/knowledge, project/iteration/triage/capability/shared UI |
| Password production deployment | `MULTICA_AUTH_MODE=password`, device/Cloud auth disabled, `MULTICA_PASSWORD_LIMITER_MODE=single`, signup as intended, `E2E_PASSWORD_AUTH=1`, isolated actors | Password branches, registration, migration/binding/server switch, password collaboration journeys |
| Platform-admin / managed installations | Password lane plus applicable `MULTICA_PLATFORM_ADMIN_ENABLED`, `MULTICA_MANAGED_INSTALLATIONS_ENABLED`, synthetic injected admin, deployment ID, explicit admin flags | `platform-admin*.spec.ts`, `admin-collaboration-switches`, resource publishing, runtime enrollment/control |
| Device-auth | Separate API configured with device auth; `E2E_DEVICE_AUTH=1` | `auth-device.spec.ts:6`; cannot share password startup configuration |
| Inbox snooze | Explicit owned runtime permit, `INBOX_SNOOZE_E2E_RUNTIME_READY=1`, matching native renderer | Six snooze journeys (`inbox-snooze.spec.ts:66-327`) and persisted UTC deadline/snooze receipt regression |
| AI assist / actor recommendation | Explicit deterministic provider and task API configuration; `E2E_ASSIST_PROVIDER_URL`, `ACTOR_RECOMMENDATION_PROVIDER_URL` as appropriate | Refinement, cancellation/provider error, recommendation stale-draft and native adoption |
| Host / recovery / stage / knowledge execution | Matching password API/Web/renderer, separate namespace vs host gate modes, compiled fixture daemon/provider/CLI, explicit allocation/provenance and family opt-ins | Real-process stop/reconnect/timeout, multi-account closure, stage/M1 control, automatic knowledge/leases and output receipts |
| Packaged Main / installed clients | Explicit app package/native executable/image/relay plus source and artifact hashes, owned HOME/profile/update feed | `collaboration-installed-main`, `collaboration-host-installed`, installed-client compatibility; separate from renderer fixtures |
| Performance S5 | `COLLABORATION_PERF_ACCEPTANCE=1`, private external run directory/manifest, 100 synthetic password accounts, exact protocol/topology, bound fixture binaries | 100 sockets x 100 events and 100 independent writebacks; whole-directory/default runs intentionally skip |

`make check` specifically copies the source env and forces **legacy**, managed/admin/device/knowledge execution disabled (`scripts/check.sh:87-127`). It builds/starts this checkout's Desktop renderer and production Web after static/TS/Go gates (`:182-219`). Consequently **`make check` alone is not a complete post-merge verdict**. All skipped families need an explicit run or a named unresolved prerequisite.

Synthetic namespace control requires `creation_enabled=true`, `enabled=true`, **host_execution_enabled=false**. Real host acceptance requires a separately configured host gate; sharing these families in one gate allocation can reject valid namespace fixtures (`.trellis/spec/web/frontend/e2e-run-environment.md`, Business-scenario fixture contracts).

For password actors launching managed daemon, actual API onboarding and signed enrollment precede `.desktop-user-id` creation in an owned profile; an arbitrary UUID marker or borrowed user HOME does not establish admission (same spec, Password actors and managed profile prerequisites).

### 4. Reproducible entry points (planned; not executed here)

1. Baseline gate: `make check`. Save complete stdout/stderr/exit and generated registry provenance. This sequentially forces non-mobile lint/typecheck/tests, UI export checks, script regressions, isolated Go race + vet, API migrations/start, desktop and production Web build/start, then full Chromium collection (`scripts/check.sh:165-220`). A failure at an earlier step leaves later steps NOT_RUN.
2. Owned focused legacy run: `make up C=api,web ARGS="--web-mode production"`; `make status ARGS="--json"`; `make env-exec ARGS="-- pnpm exec playwright test --workers=1 --retries=0"`. Build/serve the matching native renderer first for unconditional desktop cases. Do not use a lazy Next dev server.
3. Discovery only before each compatible lane: `pnpm exec playwright test --list --reporter=json` under that lane's selected environment/file set. Capture names and expected skips before test bodies run.
4. Focused password example, only after the owned password allocation is proved: `pnpm exec playwright test e2e/password-registration.spec.ts e2e/password-branches.spec.ts e2e/password-migration.spec.ts e2e/password-binding-desktop.spec.ts e2e/password-server-switch.spec.ts --workers=1 --retries=0 --reporter=line,json,html`. Use report env variables/output paths unique to this run; do not run the whole legacy-dependent directory in password mode.
5. Preserve four canonical agent-protocol smoke names with `node scripts/agent-critical-path.mjs --config <owned-allocation.json> --output <fresh-run-dir>`. Its validator requires exactly those four first-attempt passes, no skips/global errors/flakes (`scripts/check-agent-critical-path.mjs:6-50`); this is a valuable early check, not replacement for the broad suites. CI pins Ubuntu 24.04 + Playwright 1.58.2 and Node22 (`.github/workflows/agent-critical-path.yml:smoke`).
6. Plugin scroll standalone: `pnpm exec playwright test --config playwright.iframe.config.ts`. This self-contained config intentionally uses system Chrome and no app server (`playwright.iframe.config.ts:1-23`). Main config already includes the file under Chromium; avoid counting a replay as new functional coverage.
7. Performance phases must run in order `prepare`, `propagation`, `writeback`, each via `pnpm exec playwright test e2e/collaboration-performance.spec.ts -g '<phase>:' --workers=1 --retries=0`. Preconditions and exact denominators are in `.trellis/spec/web/frontend/collaboration-performance-qa.md:14-63`, `e2e/fixtures/collaboration-performance.ts:75-188`. Gate requires no missing/unrun samples, propagation pooled **and event slowest-observer p95 ≤2s**, writeback p95 thresholds as in the canonical spec. Do not cherry-pick surviving samples or describe a rehearsal as main acceptance.

### 5. High-value plan additions / explicit gap disposition

- **Mode breadth:** give legacy, password, device, managed/admin, host-enabled and namespace-only modes separate allocations. Map each skip reason to a compatible lane; count skipped cases separately from pass/fail.
- **Current merge focus:** retain complete human delivery/revision/reacceptance, live grant withdrawal/stop, original-key ambiguous recovery, private run paging, UTC outcome/snooze round-trip, actual output/attachment retention, knowledge source authority and source/target lease invalidation journeys.
- **Permissions:** include owner/admin/member/nonmember, external collaborator without a workspace, resource owner vs source reader, ordinary PAT vs machine credentials and agent autonomy. Assert both visible UI affordance and valid authenticated API refusal plus unchanged state; a malformed 400 or absence behind a collapsed/modal panel does not prove 403 authority.
- **WS/state:** two real clients on different active workspaces; same-account fresh profile/relogin; offline/reconnect/missed event; cross-client revision contention, old cursors, status and inbox count; revocation while an editor/preview is open. Existing examples are `workspace-access-branches.spec.ts:204`, `projects-p1-lifecycle.spec.ts:287`, `collaboration-recovery.spec.ts:36-52`, `global-scope-navigation.spec.ts:179`.
- **Data breadth:** empty/normal/long Unicode descriptions and names, many pages/rows, attachments, repeated clicks/shortcuts, UTC+Shanghai date boundaries, expired/withdrawn versions, zero-result/partial batch mutations and stale enums/malformed response. Existing table tests cover 1,001 rows (`issue-table.spec.ts:87`) and actor mocked stress covers 550 rows (`quick-create-actor-picker.spec.ts:301`); distinguish real server volume from mocked UI volume.
- **Authentication:** run actual user-facing login, questionnaire/skip, reload/logout and second-device identity rather than relying only on injected fixtures. Add session expiry/stale-401, failed logout retaining drafts and server/viewer switch checks through the canonical layer and production journeys.
- **Accessibility / responsive:** English+zh-Hans, dark/light, keyboard/pointer/touch, desktop/narrow viewport/200% zoom, focus/error/recovery, large text/scroll/portal navigation. Existing suites cover several matrices; browser phone-width cases do not establish Expo/iOS app acceptance.
- **Browser breadth:** the default suite does not establish Firefox or WebKit/Safari. A focused core/auth/attachment/history smoke per installed engine should be explicitly planned or recorded NOT_RUN with browser prerequisite; do not add dependencies just to manufacture a universal label.
- **Platform breadth:** Renderer fixture pass does not prove production Electron Main, packages/installer, macOS updater/automatic restart or Windows Job Object behavior. Local macOS cannot substitute for actual Windows/Linux/CI native results. Mobile is excluded from root commands (`package.json:build/typecheck/test/lint`); owner must plan explicit mobile commands and device/simulator acceptance separately.
- **Operational breadth:** fresh migrations, representative existing-ledger upgrade/idempotency, configuration/startup refusal, actual listener/source hash identity, upload/download authorization, fault recovery and owned-resource cleanup. Installer/updater/offline docs/site navigation should have their own evidence rows.
- **Evidence:** every run records main commit/source snapshot, build hashes/IDs, API listener/health identity, actual auth/gate configuration, owned DB/runtime/profile paths, named test inventory, full exit and report, request/WS IDs and screenshots/traces as enabled. Preserve first failures and separate repaired reruns. Complete zero-retry selected journeys must remain intact.

### 6. Critical allocation identity issue

`e2e/fixtures/collaboration-runtime-output-allocation.mjs:5-6` pins the task identity to **`full-e2e-regression-20261120`** and only allows that task's actual active or `archive/YYYY-MM/` path. `assertRuntimeOutputAllocation` also checks task.json id/name, private external 0700 state, 0600 allocation file, digest, current checkout, nondefault loopback DB **without query/fragment**, upload storage and exact service origins before any mutation (`:49-101`).

A runtime-output test cannot directly use this new task path. Plan an explicit compliant archived-task allocation subrun with a fresh record and current source (retaining linkage in this task), or a separately reviewed narrow fixture change to admit the new owner. Never fake the old task metadata, use a symlink/empty task, weaken the guard, or reuse retained allocation/report bytes. This is a test-entry limitation, not yet an observed product bug.

## Files found

- `playwright.config.ts` — standard Chromium-only existing-server collection.
- `playwright.agent-critical-path.config.mjs` / `scripts/check-agent-critical-path.mjs` — exact four-name early protocol smoke and report validation.
- `playwright.iframe.config.ts` — independent system-Chrome iframe scroll harness.
- `scripts/check.sh` — source-preserving allocated gate, non-mobile checks, separate Go/API DBs, production Web/native renderer.
- `e2e/env.ts` — dotenv precedence from `.env.worktree` before `.env`.
- `e2e/fixtures.ts` / `e2e/helpers.ts` — TestApiClient API+SQL setup, session injection, fixture cleanup.
- `e2e/fixtures/lane-guards.ts` — loopback/nondefault DB and legacy/admin compatibility guards, gate writes/restore.
- `e2e/fixtures/core-journeys.ts` — actual APIs with SimulatedDaemon, not a real provider process.
- `e2e/fixtures/collaboration-native.ts` — actual Electron/renderer with fixture Main and isolated profile.
- `e2e/fixtures/collaboration-detail-journey.ts` — generic current-task provenance and actual hash/listener admission.
- `e2e/fixtures/collaboration-runtime-output-allocation.mjs` — historical/current-old-task exact allocation limitation.
- `e2e/fixtures/collaboration-host-runtime.ts` / `collaboration-cross-client-execution.ts` — explicit controlled real-process prerequisites.
- `e2e/fixtures/collaboration-performance.ts` — fixed-protocol external allocation and exact denominator guard.
- `.github/workflows/agent-critical-path.yml` — four-path PR smoke and supported pinned browser runner.
- `.github/workflows/ci.yml` — root split frontend/backend/native/installer verification, not a universal E2E run.

## Related specs and external references

- `.trellis/spec/web/frontend/e2e-run-environment.md` — production server, mode allocation, renderer source binding, password actor/managed owner, output/task archival and updater evidence.
- `.trellis/spec/web/frontend/collaboration-closure-qa.md` — multi-owner attribution, timeout/termination/cleanup, honest deterministic scope.
- `.trellis/spec/web/frontend/collaboration-recovery-qa.md` — original receipts, full paging, protected recovery, subscriptions and controlled output.
- `.trellis/spec/web/frontend/collaboration-performance-qa.md` — preregistered 100-user sampling and thresholds.
- `.trellis/spec/core/frontend/index.md` / `.trellis/spec/views/frontend/index.md` — canonical shared contracts and quality routing; some generic quality sections remain placeholders.
- Repository configured tools: pnpm 10.28.2, Node >=22, Playwright ^1.58.2, Web Next ^16.2.5, Docs Next ^15.5.16 (`package.json`, consuming package manifests). No external retrieval was needed for inventory; actual installed/lock resolutions must be retained by execution owner.

## Caveats / Not Found

- **No test bodies, build, services, DB mutation or browser launch ran in this research.** Source AST and a subsequently authorized read-only Playwright collector established inventory only; no PASS claim follows.
- The fresh expanded count is 538; actual runtime skip count per compatible lane, flake/time budget and installed browser/runtime availability require execution by owner.
- Root Playwright has no Firefox/WebKit projects, mobile-app driver or automatic server startup.
- Root `pnpm` gates omit mobile; default `make check` omits password/admin/device/real-host/packaged/performance acceptance unless separately arranged.
- Route interception count is a proxy for mixed evidence, not a classification of every fixture interception. Detailed final results must label actual mock/HTTP/native/controlled-real-process boundaries per case.


### 7. Fresh read-only Playwright collector

After auditing 401 resolved relative-import modules for direct registration-time network/DB/process/write calls (none found), the owner authorized a read-only `--list` collector. Installed Playwright 1.58.2 `lib/runner/testRunner.js:378-381` selects only load/report-begin tasks for list mode, without global setup or test bodies; `lib/runner/lastRun.js` explicitly avoids writing its last-run file in list mode. Existing JSON/HTML output env variables were removed from the child to prevent overwriting retained evidence.

- Command: `pnpm exec playwright test --list --reporter=json`
- Collection summary: `{"collector_exit": 0, "files": 187, "expanded_tests": 538, "expected_statuses": {"passed": 468, "skipped": 70}, "global_errors": 0, "tests_executed": 0}`
- Result is **inventory only**, not a test PASS. Dynamic runtime-only `test.skip` calls inside bodies cannot be reflected reliably until the compatible lane actually runs.

Declaration-time skip identities collected in this current shell configuration:

| File | Case | Declared skip reason |
| --- | --- | --- |
| admin-resource-publishing.spec.ts | first resource page screenshot | Requires task-owned API, Web, database and managed storage |
| admin-resource-publishing.spec.ts | localized responsive states and deployment-disabled feedback | Requires task-owned API, Web, database and managed storage |
| admin-resource-publishing.spec.ts | real uploads reach Web and native markets; revisions, recovery and withdrawal preserve copies | Requires task-owned API, Web, database and managed storage |
| builtin-agent-concurrency.spec.ts | every role template, squad roster and 小阿孚 is created at the same cap as a blank agent | Requires the legacy email-code lane |
| builtin-agent-concurrency.spec.ts | an agent staffed from a role template in the browser shows the shared default as its concurrency | Requires the legacy email-code lane |
| cli-forbidden-reason.spec.ts | an Observer agent's status change prints the autonomy reason, not the workspace hint (en) | Requires the legacy email-code lane |
| cli-forbidden-reason.spec.ts | an Observer agent's status change prints the autonomy reason, not the workspace hint (zh) | Requires the legacy email-code lane |
| cli-forbidden-reason.spec.ts | an ordinary member editing someone else's comment sees the server's reason | Requires the legacy email-code lane |
| collaboration-creation-default.spec.ts | at the fresh-install defaults an ordinary member creates a collaboration issue while execution stays closed | Requires the legacy email-code lane |
| collaboration-creation-default.spec.ts | with new collaboration issues closed the member sees the reason and the server refuses the create | Requires the legacy email-code lane |
| inbox-snooze.spec.ts | Web group snooze survives reload, changes time, undoes and reminds the original unread group | Requires the explicit owned inbox-snooze runtime permit |
| inbox-snooze.spec.ts | Web failed save retains selection and custom input; another device invalidates the visible undo | Requires the explicit owned inbox-snooze runtime permit |
| inbox-snooze.spec.ts | Web ordinary and group mentions stay snoozed; personal mentions and assignment wake across devices while another workspace is active | Requires the explicit owned inbox-snooze runtime permit |
| inbox-snooze.spec.ts | Web server expiry returns original unread notifications near the top and never revives an archived plan | Requires the explicit owned inbox-snooze runtime permit |
| inbox-snooze.spec.ts | Web Chinese touch controls preserve keyboard focus, custom time and narrow layouts in both themes | Requires the explicit owned inbox-snooze runtime permit |
| inbox-snooze.spec.ts | Native Electron uses the shared snooze journey and keeps main, Snoozed and Archived tabs independent | Requires the explicit owned inbox-snooze runtime permit |
| issue-assist-flow.spec.ts | refines a vague feature, preserves folded answers, merges and undoes, then persists exactly once | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | handles 0 dynamic clarification questions and undo | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | handles 1 dynamic clarification questions and undo | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | preserves edits during generation, supports cancellation and real provider failure | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | creates the unchanged manual draft after a refinement failure | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | manual keeps creation reachable at desktop and phone widths with long content | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | agent keeps creation reachable at desktop and phone widths with long content | Requires the deterministic AI provider and isolated API configuration |
| issue-assist-flow.spec.ts | agent creation queues the refined prompt through the real API without executing a daemon | Requires the deterministic AI provider and isolated API configuration |
| password-branches.spec.ts | invalid username preserves registration fields and can be corrected without restarting | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | five-character password error can be corrected to the supported six-character minimum | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | overlong pasted password is rejected and accepts the corrected 128-character boundary | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | whitespace-only display name reaches server validation and keeps credentials for correction | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | duplicate username after normalization leaves the original identity and password intact | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | incorrect password preserves the login form and a corrected retry restores the same identity | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | unknown account gives a generic error and can sign in after the account is created | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | lost registration response offers sign-in recovery for the account the server already created | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | network failure during login keeps credentials and succeeds after connection recovery | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | temporary authentication service failure preserves the form and permits a real retry | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | Retry-After disables login submissions until cooldown ends without discarding credentials | Requires isolated password-mode deployment with signup enabled |
| password-branches.spec.ts | pending registration locks the form and ignores repeated native submissions | Requires isolated password-mode deployment with signup enabled |
| password-migration.spec.ts | legacy setup returns to the original identity and workspace | Requires isolated password-mode deployment |
| password-migration.spec.ts | temporary recovery returns to the original identity and workspace | Requires isolated password-mode deployment |
| password-registration.spec.ts | confirmed password registers without email and a second device keeps the same identity | Requires isolated password-mode deployment |
| password-registration.spec.ts | confirmation requires an exact match and password visibility stays independent | Requires isolated password-mode deployment |
| platform-admin-accounts.spec.ts | account controls, lost response recovery, forced password change and observer UI | Requires a task-owned password-mode production build |
| platform-admin-controls.spec.ts | admission receipts and original-key recovery survive a lost cancellation response and reload | Requires the isolated password-mode production deployment |
| platform-admin-executions.spec.ts | private metadata, filters, cursor, attempt detail and original content permissions | Requires the isolated password-mode production deployment |
| platform-admin-installations-read.spec.ts | real enrollment and binding keep client, daemon and engine status separate | Requires the isolated password-mode production deployment |
| platform-admin-observability.spec.ts | windowed metrics, active alerts, original-key recovery and observer boundaries | Requires the isolated password-mode production snapshot |
| platform-admin.spec.ts | zero-workspace cookie access, ordinary-user denial, and live role revocation | Requires a task-owned password deployment and bootstrapped administrator |
| quick-create-autonomy.spec.ts | the creator picker lists Observer-level creators disabled with the reason and seeds a Contributor | Requires the legacy email-code lane |
| quick-create-autonomy.spec.ts | an explicitly requested Observer explains the refusal and hands off to manual creation assigned to it | Requires the legacy email-code lane |
| quick-create-autonomy.spec.ts | the API refuses an Observer or an Observer-led squad before enqueueing and accepts Contributors | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | empty description remains in the Tab order, explains itself and routes blocked activation to the editor | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | clearing the editor and immediately submitting uses live Markdown | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | zero candidates has two working exits, keeps the draft and makes no recommendation request | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | loading recipients routes activation to the persistent picker | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | failed recipients routes activation to the persistent picker | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | autonomy and CLI version refusals keep their dedicated focus destinations | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | recommendation adoption submits from the restored picker focus | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | modifier Enter sends exactly once from the close | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | modifier Enter sends exactly once from the property | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | modifier Enter sends exactly once from the switch | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | modifier Enter sends exactly once from the editor | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | portal search and refinement answer inputs keep modifier Enter to themselves | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | plain Enter activates the focused Close button without handing off | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | pending submission and upload use native disabled | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | real geometry and helper contrast: en, light | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | real geometry and helper contrast: en, dark | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | real geometry and helper contrast: zh-Hans, light | Requires the legacy email-code lane |
| quick-create-blocked-feedback.spec.ts | real geometry and helper contrast: zh-Hans, dark | Requires the legacy email-code lane |
| upstream-backports-cli.spec.ts | skill labels remain typed and idempotent while issue labels still work | Requires an explicitly supplied task-owned MULTICA_E2E_CLI_BINARY |
| upstream-backports-cli.spec.ts | comment updates preserve UTF-8 files and reject stale revisions | Requires an explicitly supplied task-owned MULTICA_E2E_CLI_BINARY |
| upstream-backports-cli.spec.ts | an unrelated member and an outsider cannot update another member's comment | Requires an explicitly supplied task-owned MULTICA_E2E_CLI_BINARY |
