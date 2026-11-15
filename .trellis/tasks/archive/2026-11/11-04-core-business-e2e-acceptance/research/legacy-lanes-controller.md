# Legacy device and controlled Provider lanes

Prepared only; `legacy-lanes.py` main has not run. Python AST syntax parsing passed. No service, helper, API, browser or database action was performed during preparation. The existing reviewed password controller remains byte-identical (`0a7b435357482a6cbfcf95ee42ce781d16000e9848db9d433687e5cd4d2073f5`). The new runner imports its reusable identity/read-only DB/registry/client/provenance functions instead of copying them.

## Three exact variants

| Variant | Existing test | Fresh root database | Exact expected tests |
|---|---|---|---|
| device | `e2e/auth-device.spec.ts` | `multica_core1a32_20261104_device` |1|
| assist | `e2e/issue-assist-flow.spec.ts` | `multica_core1a32_20261104_assist` |8|
| recommend | `e2e/quick-create-actor-picker-phase2.spec.ts` | `multica_core1a32_20261104_recommend` |4|

Root's `runtime/legacy-allocated-private.json` contains these plus the performance database. The runner only accepts the three exact variants above and rejects performance. It performs no CREATE/DROP/migration and verifies zero users/issues/roles/admin-initialization plus752 migrations before using a row. Execution and Host gates must start disabled; all gate fields including epoch must remain unchanged afterward.

Device sets `E2E_DEVICE_AUTH=1` and starts API with legacy mode/device enabled, while clearing device default-workspace and password/managed/platform modes. Its owner still uses actual legacy email verification, as required by the existing test.

Assist starts the exact hash-bound `e2e/support/assist-provider.mjs` source on a root-selected loopback port using `E2E_ASSIST_PROVIDER_PORT`. Its `/health` must return `ok`. The API receives that local `/v1` URL and the synthetic key `e2e-local-provider`; Playwright receives `E2E_ASSIST_PROVIDER_URL`.

Recommend starts the separate exact `e2e/fixtures/creator-recommendation-provider.mjs` source using `ACTOR_RECOMMENDATION_PORT`. Its fresh `/health` must return `{calls:0}`. The API uses this helper's local `/v1`; Playwright receives `ACTOR_RECOMMENDATION_PROVIDER_URL`, plus current Changelog native API/renderer values so the actual Electron case runs. No API route or business response is mocked by this runner.

## Safety, isolation and cleanup

- Each invocation pins its own file hash, the shared controller hash, root allocation, fresh-DB ledger, original base env, test env, renderer proof, exact spec/helper bytes and initial registry manifest.
- Before each lane, verify previous-run completion/no active browser run; exact existing registry source/ports; actual API PID/PGID/source; production Web Build ID/source/listener/served API/WS addresses; actual renderer process group/artifact bytes.
- Each helper starts in a fresh owned process group with a clean environment, and its explicit port must be free and distinct from API/Web/renderer. Readiness requires the listener PID to equal this new child, PGID ownership and expected helper health. No inherited commercial credentials are supplied.
- Only the owned API stops. The existing registry manifest switches to the exact root-permitted DB/private config and `start_api` is called directly; no `up` allocation/migration path. Actual health/config, startup configuration fingerprint and DB are checked before Playwright.
- Each helper is closed in `finally` with Popen ownership and PGID checks, including readiness/test failure. API remains on its current controlled loopback configuration between sequential lanes; its helper may already be stopped during that short transition. No fallback commercial endpoint is configured.
- The outer finally stops the last API only if exact registry DB/config plus current PID/PGID/source still prove ownership. Otherwise record `ownership_unproved_preserved_for_root`; never blindly kill a changed listener. Web and renderer stay running. `active-lane.json` records the actual final DB/config and stopped/preserved state.
- Raw JSON/stdout/stderr/traces remain under private `runtime/runs/<run_id>/` with umask077. Public task evidence contains counts, helper call counts, status, private evidence path and cleanup only. Provider observation bodies are not copied; retain count plus SHA-256.
- Zero automatic retries. Process success with skipped/flaky/empty/wrong-count tests is incomplete acceptance. First failures are retained; assertion failures may continue to the next independent variant, identity/setup failures stop.

## Root permit

After independent review, create0600 JSON in the private `runtime/` directory. Required fields:

- `approved_by: "root"`, `candidate`, `runner_sha256`, `shared_controller_sha256`.
- `root_allocation_sha256`, `allocated_databases_sha256` (supplemental ledger, not password ledger).
- `source_root`, `source_id` equal current candidate/client proof; `base_env_sha256`, `test_env_sha256`, `renderer_proof_sha256`.
- `initial_registry_manifest_sha256` bound immediately before this invocation.
- `previous_run: {path,sha256}` points to an actual completed browser record accepted by shared controller (task evidence or private runtime/runs); no active browser run may exist.
- `run_id`: fresh lowercase/digit/hyphen identity.
- `rows`: selected exact ledger rows containing `variant`, `database`, `database_url`, plus `spec_sha256`; assist/recommend additionally need `helper_sha256` and an explicit unused integer `helper_port` between1024 and65535. The runner selects source paths and test counts internally; it does not accept arbitrary commands, scripts or specs.

`runtime/legacy-review-identity.json` and `research/legacy-lanes-preparation.json` contain exact prepared source hashes and spec/helper mappings without database credentials. Root chooses helper ports only once the intended execution window is available. Do not print credentials from the ledger or base env.

Actual later command:

```text
python3 .omx/private/core-e2e-20261104/runtime/legacy-lanes.py <absolute-private-runtime-permit.json>
```

The script expects the existing allocated API to be live and owned at entry. If the previous controller deliberately stopped it, root must restore its exact recorded private config first or prepare a separately reviewed stopped-entry path. Do not restart by an unrelated default `.env` or assume the original legacy DB is still current.

## Remaining validation

Only static preparation has occurred. Independent review must bind the current runner digest; actual PID/health startup, browser behavior, helper call observations and cleanup require execution. This runner verifies model transport and business handling against controlled helpers, not model reasoning or paid Provider accounts. The user explicitly deferred real-model acceptance.
