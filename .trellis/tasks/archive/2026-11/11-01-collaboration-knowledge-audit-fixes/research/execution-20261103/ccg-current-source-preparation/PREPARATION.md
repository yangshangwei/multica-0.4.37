# Current-source CCG1 / CCG2 preparation

State: **prepared for independent review; no gate executed**. Source is `85ef5d20578f63c7c4f879f035acbd414e80f306` in `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103`. Only new task-private files were written. The running `final-check-v13`, tracked source, old runners and historical results were not modified.

The current reconciliation requires four later observations because AI policy, migration 764 and the Host authorization lock graph changed. Existing final639 / E6 results retain their historical scope. No original audit defect was reopened by this preparation.

## Prepared inputs

Private root: `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/knowledge-final-gates-v13`.

- `run-ccg1.py`: retains the Host composite's exact current correction test, `knowledgeintegration`, normal/race selection, `-count=1 -p=1 -parallel=2 -json -timeout=5m`, pinned Go and repository Agent CLI guard. It requires one named pass, no fail/skip, and the original logged physical-input/withdrawal fact. It does not run the whole Host family in addition.
- `run-ccg2.py`: retains the one deployed OFF/ON probe, one worker and zero retries. It imports the frozen v13 environment parser as definitions and treats the new environment as data; the old shell sourcing is not reused. Root-bound API PID/commit, actual startup knowledge flag, production Web API/WS configuration, renderer bytes, database and explicitly built fake daemon/provider/CLI must match before Playwright starts.
- `source-inputs.json`: current source, tool and reviewed final-auditor hashes. Both runners reuse the already-reviewed final identity auditor and require its actual full-check result to be successful. No running full-check file is written.
- Four `permissions/*.json.template`: all have `ready_to_execute=false`. Root must create new real permits after the full run completes, its inventory is verified, its resources are released, and a separate execution dispatch is issued. The templates contain no database credentials or allocated resource claims.
- `deployments/{off,on}/source-values.json.template` and `deployment-proof.json.template`: unallocated startup values and proof shapes. ON changes only the separately owned test deployment; production knowledge remains default off.
- `binary-binding.json.template`: not-built current fake binary inventory. Existing reviewed current CLI bytes may be copied after full-check release and bound to their original successful build record; a user-installed Agent CLI is never selected.

All prospective commands and file hashes are in `preparation.json`. The runners themselves have not been invoked, including refusal probes. Only Python AST and JSON parsing were performed. Independent review should exercise useful credential-free admission controls before granting execution.

## Root allocation sequence after the complete gate

1. Verify the actual complete run, final identity audit and named/skip inventory, and release its resource window. A failed full gate does not become successful because a CCG run passes.
2. Allocate a new Git-free `/private/tmp/multica-ccg-v13-*` directory and task-owned caches. Each CCG1 invocation receives a separate newly created/migrated database. Its name must retain the literal `multica_knowledge_host_composite_20261101_` prefix required by the existing fixture; a unique `v13n_<nonce>` / `v13r_<nonce>` suffix keeps the old runs separate. Record actual database identity and migration 764 before issuing each permit. The runners do not create, migrate or drop a database.
3. Run CCG1 normal, then race. Keep both raw logs, source binding, exact test identity and original failure records. Require the current test's two physical starts, corrected v2 input, 409 third authorization refusal, zero third start and retained original exposure/receipts.
4. Compile the current `./internal/daemon` test binary with `go test -c` and the current `e2e/fixtures/collaboration-host-provider.go` fake. Bind build commands, current source/tool hashes, exit codes and resulting binary hashes. Copy the already-built current-source CLI into this new private binary directory and record its original build/source binding. Compilation is not deployed acceptance.
5. Allocate OFF and ON deployments serially with repository `make up C=api,web` and `--web-mode production`. Each lane needs its own `MULTICA_DEV_HOME`, profile/workspace directories, canonical private environment, fresh database/deployment identity and recorded API/Web origins. Use a new CCG-owned renderer and, if required, CCG-owned Redis; do not reuse the complete-check services.
6. Capture actual API PID/commit/startup flag and production HTML API/WS values, plus registry manifest, database and binary identities. Populate the appropriate root permit only after these observations. Execute one OFF probe, stop only its owned services while retaining its database/evidence, then separately prepare and execute ON.
7. Require one pass, zero skips/retries/failures in each deployment. Inspect the retained attachment: OFF has no knowledge section/contribution/events; ON has the exact approved contribution, session isolation and one claim/preparation/authorization. Both must settle one actual fake launch with authorized, observed, reaped and sealed proof. Preserve the raw result and attachment hashes for independent closure review.

## Environment allocation caveat found in current source

`scripts/dev-env.sh` looks up an existing environment by checkout directory before honoring a new name. Changing only `--name` within one registry therefore does not establish a separate OFF/ON deployment; use separate private registries for the same source checkout.

If the requested API/Web/renderer slot is unavailable or inconsistent, `up` rewrites ports **and the database name** to its allocated directory/offset name. That name can collide with a retained database even when the ports are now free. Root must establish a fresh intended database and matching available slot before startup, then compare the actual manifest/database identity afterward. Do not silently treat a rewritten existing database as the new allocation or delete it to make the proof pass.

The startup templates preserve the previously validated password/managed-installation test flags, including task-only platform administration, and specify max10/min1 API pooling. Secrets, exact ports, database names, deployment IDs and runtime proof fields remain unfilled. Generate canonical source files with the reviewed parser/serializer rather than shell expansion.

## Remaining conditions

- No DB, API, Web, renderer, Redis or process has been allocated or started by this preparation.
- No CCG run, Go build, Playwright collection/test, Provider call or production enablement occurred.
- Current full-check failure or source movement requires reconciliation before reusing these source-bound runners; they refuse a different candidate or changed source/tool/verifier bytes.
- Root owns allocation, service lifetime, the later execution permissions and final acceptance. A separate Trellis check agent owns review of these prepared scripts and the later actual results.
