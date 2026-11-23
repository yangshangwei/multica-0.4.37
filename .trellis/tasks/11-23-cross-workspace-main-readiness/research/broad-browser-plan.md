# Fresh browser acceptance plan

## Scope and source

Read-only inventory at private candidate `95c8b12dc619488e050e96dba8f97a527e2f4a4c`, before the parent's pending marketplace integration. No services, builds or browser tests were started for this planning slice.

Playwright `--list --reporter=json` currently collects **538 case identities in 187 files**. This is neither the old 514-case R3 inventory nor the historical 245-case core inventory. The historical core file selection now collects **248 cases**, and 13 recent UI spec files add **19 distinct cases** (267 total for that explicit selection). Exact file groups and counts are in `broad-browser-inventory.json`.

List-only results are not execution evidence. With no fixture environment, collection marks 70 cases skipped. With a representative default legacy/renderer environment it marks 45 skipped; body-level `test.skip()` predicates have not run, so the remaining 493 must not be described as runnable or passed.

Raw collection manifests live in private `core/browser-{inventory,cases,default-collection,default-cases}.json`. Recollect after final source integration because the marketplace spec may change the count.

## Recommended order

1. Parent finishes TS/static and current integration, then freezes latest source. Rebuild API with exact commit metadata and the regenerated embedded docs; rebuild production Web/Desktop from the same product tree. Retain initial cb192 originals as their own evidence.
2. Run the **entire default legacy + built renderer suite once** with one worker, no automatic retries, no real Provider credentials. Current harness accepts no spec positional arguments to select all files. This covers all applicable default cases and emits explicit skip reasons for opt-ins.
3. Collect case results by `(project,file,title)`, split actual failure versus skipped/unrun, and replay only concrete failures after diagnosis. A full-suite result can be partially red and still serve the inventory; never stop after an arbitrary number of failures or hide them by filtering.
4. Run password namespace and deterministic host lanes on fresh independent databases, plus feasible deterministic-provider/CLI/admin lanes. Run memory/provider/packaged/performance allocations only when their actual required fixtures are created and verified. Report the remainder as not executed, not passed.
5. Compare latest HEAD/base and build inputs again. Changed final bytes require affected rebuild/replay; final notification/sidebar/marketplace originals are required after all incoming UI commits.

Avoid running S1/host multi-account cases beside TS or heavy Go test compilation. The observed S1 failure correlated with Docker API calls timing out and system compression/swap; its unchanged fresh run passed in 34.4s after pressure recovered. Keep original test deadlines and record any recurrence.

## Reuse existing owned harness

Harness: `.omx/private/qa-work/main-readiness-20261123/core/core-run.py`.

It already provides UUID-owned PostgreSQL, a new database per label, clean environment/private HOME, source/binary/fixture hashes, runtime API/Web binding, built renderer, owned process groups, gate restore and closed-port receipts. Labels are unique and immutable; do not overwrite a previous attempt.

Before the broad run, make these **task-harness-only** additions, after parent approval of this plan:

- Set `TASK_DATABASE_NAME` to the actual generated `db`. Required by `collaboration-notifications-page.spec.ts:26` and verified against the connection by `collaboration-notifications.ts:49–73`; never hardcode another lane's database.
- Allow a curated list of conditional env overrides and record nonsecret effective flags; current `clean_env` deliberately drops arbitrary inherited variables. Simply exporting them in the parent shell currently does nothing.
- Set optional `CHANGELOG_ELECTRON_RENDERER_URL` / `CHANGELOG_E2E_API_URL` to this lane's actual renderer/API where those ordinary native fixtures are included; a changelog-feed fixture remains separate.
- Capture source hashes for *all selected specs and relevant fixture imports*, plus renderer/preload hashes, and a start/end product source identity rather than only the initial six target specs.
- Provide deterministic recommendation/assist Provider startup only for its dedicated lane, using explicit new ports and fake keys. No user-installed CLI lookup or vendor account calls.
- Add synthetic admin bootstrap for a dedicated password database if the two historical admin cases (and optionally the full admin group) are admitted. Each admin suite needs its own actor/database where a test revokes role or consumes reauth budget.

No repository fixture gate should be relaxed to fit the coordinator. A task-owned configuration record is the appropriate place to satisfy existing opt-ins.

Example commands **after rebuilding and the small harness extensions above**:

```sh
python3 core-run.py run --mode legacy --native --label final-default
python3 core-run.py run --mode control --label final-password e2e/password-registration.spec.ts e2e/password-branches.spec.ts
python3 core-run.py run --mode control --label final-controls e2e/collaboration-handoff-chain-web.spec.ts e2e/collaboration-handoff-chain-desktop.spec.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts e2e/collaboration-live-revoke-web.spec.ts e2e/collaboration-terminal-reopen-web.spec.ts e2e/collaboration-leave-web.spec.ts
python3 core-run.py run --mode host --label final-host e2e/collaboration-business-scenarios.spec.ts e2e/collaboration-host-squad.spec.ts e2e/collaboration-host-runtime.spec.ts e2e/collaboration-host-switch.spec.ts e2e/collaboration-order-export-20261109.spec.ts e2e/collaboration-order-export-leave-20261109.spec.ts
```

Split the host spec list into independent fresh labels if process/DB pressure appears; that is isolation, not retry-based masking. Preserve failed attempts and use the same original assertions.

## Historical core and latest UI coverage

`broad-browser-inventory.json` contains the exact historical groups:

| Group | Files | Current collected cases | Effective lane |
| --- | ---: | ---: | --- |
| work | 14 | 63 | legacy |
| project/iteration/progress | 11 | 69 | legacy |
| agent/squad/autopilot | 12 | 52 | legacy; recommendation Provider for one blocked-feedback case |
| collaboration human | 7 | 15 | legacy with deployment/API/DB |
| native projects/iterations/account | 4 | 22 | legacy + actual built Electron renderer |
| password | 2 | 14 | password |
| handoff/revoke/reopen | 4 | 4 | password + managed + namespace gate |
| administrator | 2 | 2 | password + synthetic admin bootstrap |
| host business/squad | 2 | 4 | password + host gate + deterministic binaries; knowledge on for S1 |
| supplemental | 1 | 3 | legacy |

Latest UI files (19 current cases):

```text
collaboration-notifications-page
 global-scope-navigation
 sidebar-icon-rail
 agent-template-marketplace-desktop
 capability-market
 capability-market-welcome
 agent-welcome
 runtime-welcome
 iteration-welcome
 mcp-welcome
 mcp-welcome-desktop
 skill-welcome
 skill-welcome-desktop
```

The full default lane will already collect these; do not add their successful executions to the total a second time. Notification and sidebar tests exercise both Web and actual Electron originals. Marketplace must use the incoming commit's spec and rebuilt clients; existing cb192 passes cannot establish that increment.

All five historical failed identities are covered by the final default localized suite, final controls' two tests and final host S1. Rerun them on final candidate when the newly integrated global UI can affect navigation. The prior five passes are preserved evidence, not substituted for those final affected checks.

## Opt-in matrix and default make-check gaps

| Family | Missing default prerequisite | Feasible follow-up / boundary |
| --- | --- | --- |
| Password registration/control/handoff (34 cases across the three proposed password groups) | `E2E_PASSWORD_AUTH=1`, password API, managed installations, actual deployment identity; controls require host=false | Existing control harness can admit. Password human extended group has exact 7 specs/11 cases in JSON. |
| Host business (8 proposed cases) | Explicit daemon-test/CLI/deterministic Provider paths, `E2E_PASSWORD_AUTH`, built renderer; S1 knowledge flag | Existing host harness, same host Provider kind. No real model. |
| Admin (9 cases) | `E2E_PLATFORM_ADMIN=1`, API platform-admin support and bootstrapped synthetic username/password | New owned admin lane; do not borrow a real admin. Initialization additionally needs its own fresh server binary/recovery configuration. |
| Quick-create recommendation / issue assist | `ACTOR_RECOMMENDATION_PROVIDER_URL` or `E2E_ASSIST_PROVIDER_URL` plus matching local API LLM config | Local repository fixture Provider; configure only relevant spec group. Full default skips at least recommendation case and all 8 assist cases. |
| CLI contracts (6 cases) | `MULTICA_E2E_CLI_BINARY` explicit built CLI | Task binary exists; use dedicated private config/home and capture actual CLI hash. |
| Notifications (2) | `TASK_DATABASE_NAME` | Add actual lane DB name; plain make check currently does not set this, so otherwise skips despite working API. |
| MCP catalog/native | `E2E_MCP_TEMPLATE_DIR` + API `MULTICA_MCP_TEMPLATE_DIR`; native MCP also password | Task-owned empty/template directory; report any undispatched case. |
| Changelog/native settings/retired languages | `CHANGELOG_E2E_API_URL`, renderer and for changelog `CHANGELOG_E2E_FEED` | Map ordinary native endpoints; feed itself must be a task fixture, not production. |
| Device auth | `E2E_DEVICE_AUTH=1` + separate API device mode | Separate config; default explicitly disables it. |
| Password migration/binding/server switch | Migration cutoff/deadline and explicit recovery binary or second server | Separate dedicated config; cannot claim covered by ordinary password tests. |
| Detail/inbox/PAY241 stage/recovery/output/creation/history (most of 47 special-allocation cases) | Exact provenance/allocation files, selected source IDs, named opt-ins, often controlled transport bridge or extra test binaries | Fresh allocations may be feasible but need their full contract; never reuse historical permits or set only an opt-in flag. |
| Cross-client namespace/installed (14) | Explicit Linux binaries, Docker/host bridge address and/or packaged Main/relay/ASAR/source hashes | Not default renderer acceptance. Separate platform scope and packaging evidence. |
| Memory/closure/timeout Provider journeys | Different fixture Provider binary kind | Do not run with generic host-provider; compile corresponding repository fixture explicitly first. |
| Project knowledge automation | `P6_RUN_DIRECTORY` and explicit controlled daemon/Provider allocation | Separate fixture-controlled integration; default skipped. |
| 100-observer performance | `COLLABORATION_PERF_ACCEPTANCE=1`, preregistered run/source manifest, capacity environment | Not an ordinary merge smoke or a default make-check pass; leave not-run unless explicitly allocated. |
| Real vendor models, mobile native UI, Windows/macOS installer/updater/signing | Actual external/platform environment | Outside this merge-readiness browser claim. No vendor quota or real-account execution authorized. |

`make check` creates legacy production API/Web and built renderer; it deliberately overrides auth to legacy and disables managed installation/platform administration/knowledge context. It runs the whole directory, so expected conditional skips remain. It does not automatically generate synthetic admins, special provenance permits, local model fixtures or packaged artifacts. A completed `make check` must therefore report its skip inventory rather than imply all 538 cases passed.

## Reporting contract

Create one identity ledger over the latest admitted executions. Store original failure and repaired replay links, source/build/config identity and skip reasons. Collection-time annotations and actual execution statuses are separate fields. The final claim is “current applicable default suite plus N configured conditional cases,” with explicit unconfigured families, not “all historical514 passed” or “538 passed” unless every actual identity has matching terminal evidence.
