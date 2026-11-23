# V14 continuation preparation

Coordinator: `01a1a186-9a5c-7b00-96f8-0430b47edb4c`.

This preparation is offline and not runnable. It preserves the original v14 templates and the v13 failure/identity evidence. The final candidate remains pending the 17-case directed green result, independent repair review and root commit. No build, database, service, package test, browser test or Provider invocation was performed by this lane.

## Prepared files

`prepared-template-manifest.json` records every original hash, new template hash and intended final path. Eleven new `.in` files relocate evidence to this directory and identify the current coordinator. Five already-installed private helpers/adapters/config files remain byte-identical. The new private launch/environment/control templates are under the candidate's `.omx/private/final-check-v14/resume-2100-templates/`.

The preserved worktree path is `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103`; its `v12` component is part of its actual filesystem identity and must not be replaced.

## Inputs before materialization

1. Root supplies the reviewed final 40-character commit SHA. Candidate status must be clean. The sole allowed generated Next import may be restored only after its exact bytes are retained and checked; do not clear another source edit.
2. Root dispatches CLI/Desktop build preparation. Materialize `.in` files to their manifest `final_path`, replacing `PENDING_ROOT_APPROVED_COMMIT_SHA` only with that SHA. Preserve all originals and use exclusive file creation. Run using `/opt/homebrew/opt/python@3.14/bin/python3.14`; system Python 3.9 lacks `hashlib.file_digest`, which is required by the retained workflow.
3. Allocate fresh current-owner seed database, Redis container/port, renderer listener/port and execution registry. Proposed seed name is `multica_closure_v14_seed_01a1a186`, not yet allocated. Replace `PENDING_ROOT_REDIS_PORT` consistently in launcher, fake controls and environment. Replace `PENDING_ROOT_RENDERER_PORT` consistently in environment, bindings and controls. Set a new private JWT secret in the source env, without printing it. Do not reuse the old `53841`/`19857` proofs or the previous owner's DB suffix.
4. Retain and revalidate the already-private v14 cache/temp allocation. The fullcheck writable directories `full-check-home-v14`, `go-packages-v14`, `dev-registry-v14`, `profiles-v14` and `workspaces-v14` must be absent before the sole launcher creates them. Do not create a placeholder fullcheck registry with writable run state.

## Ordered build and binding actions after root dispatch

Use the intended scripts in this directory after removing `.in` through explicit materialization:

1. `candidate-setup/source-snapshot.py candidate-setup/source-before.json` captures clean tracked bytes/modes and source/donor protection.
2. `candidate-setup/build-current-v14.py cli`, then `candidate-setup/build-current-v14.py desktop`. The retained sandbox denies networking, pins Node 22.23.2 and Go 1.26.6, and writes only to v14 private outputs/temp. Each build records source, output, raw log and actual exit. Do not reuse previous build-result files.
3. Root preserves the current installed `apps/desktop/out` and installs the new staged `artifacts-build/{main,preload,renderer}` exactly. This write is outside this lane's current scope. Do not touch an active directed environment's installed artifacts before the root releases its window.
4. `candidate-setup/capture-artifacts-v14.py` proves installed/staged equality, CLI hash/version and successful clean-source build records.
5. `prepare-bindings-v14.py` captures all tracked files/modes, canonical product ID, distinct development HEAD/overlay ID, exact workspace paths and actual exported graph, tool/SDK hashes, new CLI/Desktop manifests and current renderer URL. Historical v11 binding is used only as a pinned tool/SDK layout input; runtime identities are regenerated.
6. Root writes actual `resource-ownership-v14.json`: candidate identity, fresh renderer PID/command/cwd/port/index bytes, fresh Redis container/port/identity and explicit exclusive window. Initial source env/binding/ownership must agree. The root also records the actual API/database/process identities after the check starts; filesystem audit does not prove them.
7. Materialize a credential-free `private-preflight-offline.sb` that denies network and permits only new task-owned temporary/evidence paths. Run the three prepared private checks `run-private-preflight.py launcher`, `adapter-env` and `final-audit`, saving fresh logs. Run the independent reviewer only after all actual bytes and bindings exist. This preparation has run syntax parsing only, not those checks.
8. Independent reviewer writes `independent-review/review.json`; root writes the corresponding hash-bound `independent-review-clearance-v14.json` under v14 private output, including final auditor/review/resource hashes. Preserve actual reviewed script bytes and require zero blockers.
9. The designated sole executor launches `.omx/private/final-check-v14/run-make-check-v14-provenance.py`. It must not auto-retry or overlap any prior full run. Capture raw log, exit, named Go/TS/browser inventory and every explicit skip.
10. Run `final-state-audit-v14/final_state_audit.py --output <new file in audit directory>` after success or failure. Wholecheck acceptance requires make exit 0, actual final audit pass, inventory completeness and the root's live ownership validation. An identity-only pass on failed make remains a failure.

## Remaining gates

CCG1 normal/race and CCG2 startup OFF/ON follow the successful new full gate and need their own newly bound resources. Task #7 receives independent AC1–AC9/21-item closure after those gates. Task #6 deterministic fake normal/race and six-file review follow separately. All knowledge model budgets/calls remain 0/0 and production injection remains false.
