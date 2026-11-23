# Supplemental controller and lifecycle waiter review

Reviewed the exact current private `password/run.py`, `controlled/run.py`, and the `e2e/collaboration-workbench-web.spec.ts` waiter change. No API, Web, renderer, database, model, Docker or browser process was started/stopped/reconfigured by this review. The only executed process fixture was a temporary Python child/grandchild used to verify interruption cleanup.

## Findings and repairs

1. Password controller did not enforce offline Go compilation. Added the same `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off` values already used by the controlled controller.
2. A restore exception in the previous `finally` block could replace the original test error and prevent `final.json` creation. Both controllers now record execution failure, restoration status and cleanup/source-audit errors in `finalize_run`. The first execution error remains authoritative; cleanup failure still fails an otherwise successful run.
3. Interruption during `Popen.wait` left a live child and a permanent `running` record, allowing the caller to restore the API while the test was still alive. Both `run` helpers now launch a separate owned process group, terminate/reap it before returning the interruption, record the interruption and cleanup result, and route SIGTERM through this path. Remaining members of that exact group receive SIGKILL after its immediate child exits. Production API processes are deliberately detached by the existing registry launcher and are handled separately by the existing owned-API restoration.

Original private files are preserved as `run.before-review.py` beside each controller. Patches and SHA-256 identities are recorded in each `review-repair.patch` and `/Users/artisan/.multica/qa/core-20261108/controller-review-result.json`.

## Verified boundaries from source

- Both controllers bind the fixed `check-20261108144845-80493` registry and verify its checkout. Existing `stop_component api` uses recorded process-group/listener ownership; neither controller requests Web/renderer stop or rebuild. Root must release its baseline API before invocation.
- Every lane creates a new uniquely named local database, verifies `current_database()`, migrates it, and checks empty user/admin-initialization counts. There is no database drop or foreign data cleanup. Each API uses max6/min1 connections.
- Lane environment keys are allowlisted; each lane uses private HOME/TMP/upload/catalog paths, a synthetic deployment UUID/JWT secret, password mode, disabled Cloud/device auth, single-process password limiter, and no inherited paid-model key.
- Ordinary admin specs register a synthetic password account, bootstrap exactly that UUID via the current server CLI, then log in again after bootstrap revokes the old session. The initialization spec skips bootstrap and receives a fresh database containing `admin_init`, matching its fixture contract.
- Web production Build ID, listener ownership, rendered API/WS origins, renderer served bytes and preload hash are checked. Root still owns the final comparison between the build's original product source and any later test-only edits; recording a current source ID alone does not prove a rebuild.
- Controlled providers are compiled from named local fixture Go sources. Host runner receives explicit fake provider/CLI paths and a fresh home. Linux fixture images must already be cached; a local-socket Docker guard forces `--pull=never` and rejects pull/build. Installed CLI is the explicitly pinned cached v0.6.7 compatibility fixture, not a user-installed agent.
- Existing fixture cleanup uses newly generated container/volume names and the configured scoped profiles. The controllers restore the original owned API/configuration in normal and failure paths while preserving failed databases and logs.

## Lifecycle waiter

The response waiter now starts immediately before `releaseOriginal()`, after the concurrent actor has completed the intended revision change. The route still holds the original request until release, so subscribing there cannot miss its server response. Request identity, stale-revision refusal, original 15-second response budget, preserved form data, and explicit retry assertions remain intact. The intentionally paused preparation interval no longer consumes the server-response budget. No additional change requested.

## Checks

- Before repair, the interruption regression demonstrated live immediate child processes after `KeyboardInterrupt` in both original helpers; test cleanup then terminated only those exact fixture PIDs. New finalization helper tests were absent before the repair.
- After repair: **6 test methods, each exercised against both controllers, PASS**. They cover original-error preservation when restoration also fails, restoration failure after success, source-audit failure, retained successful results/no mutation when API never started, SIGTERM routing, and actual local child/grandchild shutdown.
- Python compilation and both preparation-only manifest invocations passed. Lifecycle diff whitespace check passed.
- Evidence: `/Users/artisan/.multica/qa/core-20261108/controller-review-{tests.py,red.log,green.log,result.json}`. No controller runtime acceptance is claimed; actual lane tests and final live resource audit remain root-owned work.
