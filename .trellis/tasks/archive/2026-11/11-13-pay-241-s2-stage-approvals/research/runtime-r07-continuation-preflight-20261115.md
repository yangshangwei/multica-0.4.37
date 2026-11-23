# Research: S2 r07 runtime continuation prerequisites

- Query: Make the four existing Web/Desktop scenarios executable after P2 r07 acceptance, reusing retained helpers and build artifacts.
- Scope: internal, read-only source/configuration projection and filesystem evidence. Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
- Date: 2026-11-15. Researcher: `/root/s2_runtime_preflight`.
- Result: **Final runtime binding remains required. No Go test, database connection, API request, browser, daemon, model, payment or deployment ran in this research.**

## Findings

### 1. Four registered scenarios

The complete denominator is four. Each body retains its 480,000 ms deadline, with one worker and zero retries. The existing E2E4 review approves authoring only; it records zero executed bodies.

| Entry | Scenario | Specific prerequisites |
| --- | --- | --- |
| `e2e/collaboration-stage-approvals.spec.ts:27` | `stage decisions and prepared test reports retain human and runtime history across Web and Desktop` | Ready P2, password actors, stage roles, real local Git preparation and run-local CLI reports, matching native assets and host fixture. |
| `e2e/collaboration-stage-execution.spec.ts:233` | `release and rollback keep exact builds, human decisions and controlled execution separate on Web and Desktop` | Above plus the private controlled execution/metric bridge and actual local artifact package output. |
| `e2e/collaboration-stage-execution.spec.ts:349` | `a late authenticated deployment stays historical and a fresh approval cannot endorse it` | Same retained controlled source, original execution identity and late authenticated observation. |
| `e2e/collaboration-stage-execution.spec.ts:398` | `stage evidence gates real submission, M1 completion and administrative or project exits` | Same actual stage/M1 writer integration plus initialized platform-admin credentials. Shared Web issues the authorization; Desktop consumes that exact intent; Web recovers both original receipts. |

`stageEnvironment()` (`e2e/fixtures/collaboration-stage-approvals.ts:24`) requires `COLLABORATION_STAGE_ACCEPTANCE=1`, `E2E_PASSWORD_AUTH=1`, explicit loopback API/Web/native origins, deployment ID, source root and database-name/DSN agreement. `StageJourney` (`stage-execution.spec.ts:44`) additionally requires the private socket and refuses multiple workers/retries. `stageArtifactEnvironment()` (`stage-execution.ts:96`) requires canonical owned staging/output roots in modes 0500/0700 and exactly eight distinct generation UUIDs.

### 2. Retained frontend and current source boundaries

The retained production binding is:

`/Users/artisan/.multica/acceptance/pay241-s2-20261114-r01/frontend-build-r01/input-binding-production.json`

SHA-256: `97aa70bcdd96703177e530ba38f861598bd99fe857b448b95f205a877c4f1967`. Its source is the sibling `source/`. It binds 3,127 files, 1,961 dependency links, 47 Core/Views `stage*` files and seven stage E2E/fixture files. The six inspected primary stage spec/fixture/Host-runtime/Provider files match current ROOT exactly. The old frontend build report binds 1,586 outputs and 23 traced files.

| Purpose | Exact retained path under the frontend source |
| --- | --- |
| Production Web working directory | `apps/web/` |
| Next start entry | `apps/web/node_modules/next/dist/bin/next` |
| Next production output | `apps/web/.next/` |
| Existing build ID file | `apps/web/.next/BUILD_ID`; retained value `SaRnyLTvwkVOxOqh_KuHE` |
| Compiled proxy routes | `apps/web/.next/routes-manifest.json` |
| Native renderer | `apps/desktop/out/renderer/index.html` |
| Native preload | `apps/desktop/out/preload/index.js` |
| Renderer serving helper | `scripts/check-desktop-renderer.mjs` |
| Native fixture Main | `e2e/fixtures/collaboration-account-electron.mjs`, selected by `collaboration-native.ts:18` |

The proxy record at `resume-r05/capacity-r01/evidence/production-web-proxy-binding-r01.json` in the external acceptance root binds all six rewrites to `http://127.0.0.1:64922`. This is compiled proxy evidence, not proof of a current listener or rendered API/WS identity.

The two approved P6 fixture paths are `e2e/fixtures/collaboration-detail-journey.ts` and `e2e/fixtures/project-knowledge-automation.ts`. Their current ROOT hashes match `p6-root-narrow-fixture-binding-b7dc8f78.json`: `baafa7c5…` and `e4f3831c…`. Neither file is a member of the 3,127-file frontend binding. The four PAY specs use `closureActor()`/`createHostActor()` and their native fixture; they do not directly import these P6 fixture files.

For an additive P6 test overlay, preserve the original source, `.next` output and production binding. A new task-owned descriptor must identify the original binding plus the two reviewed fixture files and narrow patch provenance; adding test-only files must not be described as rebuilding product assets. If a new physical test graph is required, create it separately and bind its links and new config to that graph. Do not patch an absent manifest member or rewrite the old binding in place.

The old bundle cannot certify the current ROOT comment UI. Four inspected product files differ from that bundle: `packages/views/collaboration/collaboration-detail.tsx`, `comment-list.tsx`, `comment-history.tsx`, and `discussion-actions.tsx`. A current-ROOT acceptance claim needs a newly bound product graph and corresponding production build. The retained checked3d/E2E4 bundle remains usable evidence for its own frozen scope.

### 3. Current runtime configuration is prepared, not final

The existing runtime directory is `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/runtime-r05`. Its `runtime-env.json` exists in mode0600 and selects:

- Database `multica_pay241_20261115_s2_runtime_r05`; API/Web/native ports 64922/64923/64924.
- Password authentication, single limiter, disabled device authentication, empty Cloud/mail/Redis fields and the explicit test harness/stage/password E2E flags.
- The retained frontend source and eight existing private0700 artifact generation directories.

It does **not** declare `GOTMPDIR`, `COLLABORATION_STAGE_GIT`, `E2E_PLATFORM_ADMIN` or either administrator E2E credential field. Its `GOCACHE` is still the older `/Volumes/artisan/dev-cache/go-build`, not the PAY cache required by the new final build helper. All three Host binary paths under `runtime-r05/bin/` are placeholders that do not exist. `initial-admin-credentials.json` and `browser-env.json` also do not exist.

Five old binaries do exist under `runtime-r05/build-candidate-r02/`, all with retained successful build exits. Their manifest identifies `resume-r05/combined-candidate-r05-r02/source`, manifest SHA `c12ecb4a34e1a181697f6e0b3fe4ac1c557485061ff149261b73a354453516dd`. They are not the accepted P2 r07 source and cannot be used as final matching binaries.

The allocation file records a template audit with 835 ledger entries, last exact `864_collaboration_stage_submission_ready`, bare hex64 and empty selected stage tables. Its runtime-target `actual` field records only database identity and connection count at allocation time. There is no runtime-specific 865/origin verification in that record, and the runtime `evidence/` directory is empty at inspection. This does not establish the current database state; no database was connected here.

The runtime config declares no knowledge/model configuration fields. Absence is not proof that automation or model execution is disabled. Final configuration must explicitly follow the already-reviewed source/worker configuration boundary. This research does not infer a model state from default settings or enumerate user accounts.

### 4. Reuse the existing final build and bridge mechanisms

The final builder is:

`/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/final-verification-r06/coordinator/build-runtime-r06.mjs`

SHA-256: `00109dd7d5e6b7188624d97816f6dea5ef2ee177aaac69875d1cbb1ab528646b`.

Its single argument is a final build-clearance JSON. It requires `p2_code_review_accepted=true`, `runtime_product_acceptance_complete=false`, exact source/manifest/config/frontend/launcher bindings, trusted private Data `HOME/TMPDIR/TMP/TEMP/GOTMPDIR`, and PAY-owned external `GOCACHE`. It builds `server`, `multica`, `stage-router.test`, `host-daemon.test` and the checked `host-provider`. It correctly uses the cmd/server package linker for the router test binary. It does not build a migrator.

The existing generic `runtime-r05/bridge.mjs` accepts a config-file argument (`:240`) and can be reused without changing its protocol. The config must select the newly built `stage-router.test`, final source working directory, final runtime environment and dedicated socket/ready/log paths. The persistent API harness is `TestCollaborationStageExecutionHarness`; the documented direct test-binary invocation has `-test.run '^TestCollaborationStageExecutionHarness$' -test.timeout=0`. Its controlled synthetic sources are not public simulation controls or real deployment facts.

The old `tools/prepare-runtime.mjs`, `prepare-bridge.mjs`, `authentication-preflight.mjs` and `run-frontend.mjs` hardcode `runtime-r01`; `prepare-bridge` also names old green-r03 source. Retain them. Their recipes are useful, but unmodified execution would target the wrong allocation. Root should use a separately bound new child config/helper where adaptation is necessary.

The existing external Playwright config is `resume-r05/p2-verification-r07-r01/runtime-runner-preparation/playwright.config.mjs`. Runtime execution requires `PAY241_FINAL_RUNTIME_BINDING`. Its JSON must have `status=READY_FOR_FOUR_S2_SCENARIOS`, accepted P2, the exact hardcoded retained `frontend_source`, hash-bound `required_files` and a fresh external output directory. It selects precisely the two spec files, one worker and zero retries. A different physical frontend/test graph requires a new separately bound config; do not mutate this retained config.

### 5. Minimal coordinator sequence

1. Accept P2 r07 normal/race and final source verdict. Retain original RED and the old r03/r05 records. Bind the chosen frontend scope separately from current ROOT.
2. Create a new mode0600 runtime child config from the retained allocation. Bind final five binary paths, explicit `/usr/bin/git`, all trusted Data temporary keys and PAY cache. Keep password/device/Cloud/mail/integration isolation and reviewed model/automation choices explicit. Verify actual runtime schema/gates before any fixture write; retained allocation metadata is insufficient for 865/origin.
3. Invoke the already bound builder with the final clearance, for example `node <build-runtime-r06.mjs> <final-build-clearance.json>`. Audit outputs; compile a matching migrator separately only if a reviewed runtime migration is required.
4. Run the matching standard server binary's real administrator initializer through the existing environment runner: `platform-admin initialize --credentials-file <fresh absolute private path> --reason <nonempty reason>`. `server/cmd/server/platform_admin_initialization.go:34` defines the flags; password mode, `MULTICA_PLATFORM_ADMIN_ENABLED=true` and deployment identity are prerequisites. Do not insert administrator credentials with SQL.
5. Start the source-bound generic bridge and production Web/native renderer with their owned configs. Use actual listener/health/config identities. The concrete frontend commands are Node plus `apps/web/node_modules/next/dist/bin/next start --hostname 127.0.0.1 --port 64923`, and Node plus `scripts/check-desktop-renderer.mjs <out/renderer> 64924 <new provenance path>`, executed under the selected frontend graph.
6. Authenticate through `/auth/login`, change the initial password through `/api/me/password/change`, verify the same user ID, then use the new password for a fresh login. Store only private credential references and a new private browser config. The fourth case requires `E2E_PLATFORM_ADMIN=1`, `E2E_PLATFORM_ADMIN_USERNAME` and `E2E_PLATFORM_ADMIN_PASSWORD` (`stage-execution.spec.ts:401`). Preserve actual onboarding/current-account guards.
7. Write the actual ready runtime binding, then invoke the existing Playwright CLI with its external config and `--workers=1 --retries=0`. Do not use discovery output as a body result. Bind compiled proxy/live HTML/API/WS and native assets to the same runtime.
8. Reconcile all four bodies and A01–A12, close owned resources, restore allocation gate booleans with monotonic epochs, and retain every failed run/configuration/database. Final external S4 and parent acceptance remain independent.

## Files found and related specs

- `research/joint-e2e-increment-4-check.md/json`: approved four-scenario authoring; exact shared M1 cross-client recovery and zero bodies executed.
- `research/joint-build-preparation-20261114.md/json`: isolated production Web/native build and original build/audit failures.
- `research/server-increment-2c-control-transport.md`: test-only JSONL bridge, original request identity and persistent-harness invocation.
- `research/coordination-freeze-r05.json`: source-bound references to current R06/P2/runtime/proxy preparation.
- `.trellis/tasks/11-14-project-knowledge-automation/research/p6-root-narrow-fixture-binding-b7dc8f78.json` and its two patches: exact reviewed ROOT fixture delta provenance.
- `.trellis/workflow.md:363`: persisted research and role-isolated context.
- `.trellis/spec/web/frontend/e2e-run-environment.md:76`, `:104`, `:758`: password startup, actual actors/managed profile and production/native inputs.
- `.trellis/spec/server/database-test-isolation.md:80`, `:359`: actual registry identity and trusted toolchain-selected temporary roots.
- `CLAUDE.md:206`, `:236`: same-layer test ownership and evidence before acceptance.

## Caveats / Not Found

- This is filesystem/configuration/source inspection, not runtime verification. It did not inspect current database ledger, live listeners, service health, actual model state, current business permissions or remote integrations.
- The inspected artifact files exist; the complete 1,586-output frontend manifest was not rehashed by this researcher. Root's retained audits and final binding own that proof.
- Current ROOT product differences and P6 files must not be silently injected into an old freeze or represented by its existing bundle. The two test-only P6 additions alone do not certify the current comment UI.
- No source, test, migration, dependency, existing configuration, old report, task state or Git state was changed. Only this research Markdown and its JSON companion were written.
