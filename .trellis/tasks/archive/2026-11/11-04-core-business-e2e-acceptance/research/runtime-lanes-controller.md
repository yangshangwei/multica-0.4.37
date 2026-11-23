# Password conditional lane controller

Prepared only. The controller's main function has not run; no database, service, account or configuration was changed by this preparation. Root allocated 43 fresh databases independently. Private `runtime/lane-control.py` accepts selected rows from that ledger and never creates, drops, migrates or reallocates a database.

## Scope and execution order

One selected spec uses one root-allocated database and one API process lifetime on the existing API port18784. Production Web13704 and renderer5878 stay running. The controller requires the prior browser run to be complete and checks for active task browser runs before every switch.

1. Bind controller, allocation request, actual allocated DB ledger, original allocation/env/test-env, runtime binaries and renderer proof by SHA-256. Validate exact candidate, spec bytes, database name/URL and row identity. Packaged cases need explicit `external_preparation_ready=true`; namespace cases need a numeric callback IPv4.
2. Verify current owned API PID/PGID/source, Web listener/production Build ID/source, renderer PID group/artifact bytes, and the Web HTML's actual API/WS destinations. Check the new database through a read-only connection: correct name,752 migrations,zero users/issues/roles/admin initialization, and exact root-prepared collaboration gate.
3. Stop only the current owned API. Source `dev-env.sh`, load the existing registry manifest, load the new private config, update only that manifest's DB/env selection, and call `start_api`. It deliberately never calls `up`, `ensure_database` or `migrate_database`.
4. Verify exact registry DB/env binding, actual API health/configuration fingerprint/password capability/deployment UUID, database and preserved client proof.
5. For an admin spec only: register a unique synthetic account through `POST /auth/register`, invoke current compiled server `platform-admin bootstrap --user <exact-returned-UUID> --reason ...`, log in again because bootstrap revokes old credentials, and verify `/api/admin/me` returns that user as `super_admin`. Credentials remain in a private0600 JSON file and child environment, never command arguments or public evidence.
6. Execute that one spec with one worker and zero retries, capture framework stats, then audit source/client/API/gate identities. Continue after browser assertion failures; stop on setup/identity/gate-restoration failures. A mandatory skipped/flaky/empty spec is not acceptance-pass even if process exit is0.

Raw framework stdout/stderr/JSON/traces/screenshots are under `.omx/private/core-e2e-20261104/runtime/runs/<run_id>/` with umask077. Task evidence contains only sanitized counts/statuses and the private evidence path. On an exception, record interruption and terminate a running record; stop the API only after a second exact registry/health/PID/PGID/source ownership check. If ownership cannot be proved, preserve the process and report it to root. The final normally completed API remains available for root inspection/cleanup; `active-lane.json` records its actual configuration and database.

## Why a database per spec

`PlatformAdminService.Bootstrap` refuses when any effective super administrator already exists. Registering a different account per spec and repeatedly bootstrapping one shared database therefore fails by design. Each admin spec instead gets a fresh root clone and one administrator.

Password registration has a real30-requests/5-minute per-IP budget; account login has10-attempts/5-minute and login IP300/minute. Each independent spec starts its own task-owned API with the actual single-process limiter. No limiter configuration or within-spec assertion is weakened. If a single spec exceeds its own budget, preserve that failure and investigate; the controller does not retry automatically or reset its process mid-spec.

## Root permit contract

Create a0600 JSON permit under the task private control directory after baseline has finished and independent controller review is accepted. Required fields:

- `approved_by: "root"`, `candidate`, `controller_sha256`.
- `allocation_request_sha256`, `allocated_databases_sha256`, `runtime_manifest_sha256` from `runtime/lane-review-identity.json`.
- `root_allocation_sha256`: current `allocation.json`.
- `base_env_sha256`: the original allocation's `source_env`; `test_env_sha256`: `test-env.json`; `renderer_proof_sha256`: `renderer.json`.
- `initial_registry_manifest_sha256`: exact current private registry `manifest.env`. This changes after each controller invocation; bind again when selecting a later batch.
- `source_root`: current frozen checkout; `source_id`: current full checkout source identity, equal to current API and Web proof.
- `migration_count: 752`, `run_id`: a fresh lowercase/digit/hyphen name.
- `previous_run: {path, sha256}`: an actual completed run record under the current task evidence or this controller's private runs. Failed-but-complete prior runs are acceptable and remain failed.
- `migration_cutoff` and `migration_deadline`: UTC RFC3339 strings ending inZ, with deadline after cutoff and in the future. Both are explicitly propagated for password-binding/migration cases.
- `rows`: chosen records from the exact allocation ledger, each containing `id`, `database`, `database_url`, `ready: true`, `spec_sha256`, `deployment_id` (root-selected stable UUID for that API lifetime), optional `extra_test_env` and `external_preparation_ready`.

`test-env.json`, `renderer.json`, source config and permit must have0600 permissions before their hashes are signed. No config content should be printed. The root allocation file itself has no credential and is hash-bound without requiring private file mode.

Run only after review and prerequisites:

```text
python3 .omx/private/core-e2e-20261104/runtime/lane-control.py <absolute-private-root-permit.json>
```

Do not use the older `run-browser.py` after switching: it intentionally binds the original legacy DB. Read the actual current registry / `active-lane.json` for subsequent cleanup or a new lane batch. The original `allocation.json`, `legacy.env`, `test-env.json`, Web artifact and renderer artifact are never rewritten by the controller.

## Requested databases and gates

`runtime-lanes-allocation-request.json` has42 unique password/host/native specs and43 executions because knowledge OFF/ON are separate rows. Root's v2 ledger updates only pw14/pw15 (installed CLI and installed Main) to creation=true,execution=true,host=false, matching their namespace-control fixture. Host fixtures themselves open/restore all three gates, so their starting state remains creation=true,execution=false,host=false. Admin-init is pw40, with database name containing `admin_init`, no preseeded administrator, and no bootstrap step.

Selected namespace rows additionally need `COLLABORATION_CROSS_CLIENT_HOST_IPV4`. Selected packaged rows need exact current package paths and hashes accepted by their fixtures, passed as new `COLLABORATION_*` keys in `extra_test_env`; existing bound keys cannot be overridden. All controlled native binaries are loaded from the current runtime manifest, including the dedicated memory Provider.

Device authentication and assist/recommendation Provider switching are not implemented in this controller. Root should use separate reviewed lanes for those legacy-mode configurations. Real commercial-model execution remains explicitly out of scope for this run.

## Preparation validation and remaining review

Python AST parsing passed. Seventeen credential-free helper checks passed: completed/changed/active prior records, private file/hash/mode ownership, unsafe shell-source values and keys, and status-only HTTP error reporting. They import definitions but never call `main`, execute subprocesses, access network/database, or switch services. These tests do not prove the real stop/start/registration/browser path; independent review and first actual execution remain required. Root still reconciles exact Playwright identities and owns final service/database cleanup.

## Native macOS packaged row

Only pw10 (`collaboration-host-installed.spec.ts`) may use root's native macOS extra file. Set `external_preparation_ready: true` and add `extra_env_file: {path: <absolute runtime/mac-extra-env.json>, sha256: <exact file hash>}` to that permit row. Do not also specify Docker installed-image keys. The controller reads this private0600 file without modifying it. For this exact native spec, its packaged acceptance home must be an existing directory inside the task control root and its explicit DSH path must be nonexistent inside that same root. The child gets HOME equal to this acceptance home and an explicit existing macOS Playwright browser cache; ordinary lanes retain their existing environment. All package executable/ASAR/CLI/source hashes are forwarded to the fixture, which performs the native checks. This does not establish notarization or release readiness.
