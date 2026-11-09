# Legacy supplement and detail provenance review

Reviewed private `legacy/run.py`, `legacy/detail.py`, current fixture consumers, registry record shapes, and the two final test diffs. This review did not execute API/DB/provider/browser services or product tests and did not stop or reconfigure the running baseline. The only executed regression processes were temporary Python children belonging to the review.

## Result

The legacy/detail preparation is suitable for root's serialized execution after the baseline releases its API. Actual lane outcomes and final live ownership audit remain unproven until that execution. Two private runner defects were reproduced and repaired; no product files or frozen password/controlled scripts changed.

### Private runner repairs

- Failed `Popen` previously left a permanent `running` record. The local missing-executable regression failed before repair and now records terminal failure and error type.
- An immediate child exiting on SIGTERM could leave its SIGTERM-ignoring descendant alive while API restoration began. A real local child/grandchild regression failed before repair and now passes. The legacy runner reuses the already-reviewed owned-process-group `stop_owned_process`/`run` implementation from the frozen password controller, including escalation of remaining group members and interruption recording.
- Original bytes were saved as `legacy/run.before-review.py`; diff and hashes are in `legacy/run.review-repair.patch` and private `legacy-controller-review-result.json`. `detail.py` is byte-identical to its before-review backup.

## Source/runtime contracts reviewed

- Device, Assist and Recommendation lanes are separate complete specs on fresh local databases, with per-lane private HOME/TMP/upload/catalog paths and max6/min1 API pools. Only the existing fixed owned API is switched; neither Web nor renderer is stopped/rebuilt.
- Source env keys are allowlisted. Device auth is enabled only in its lane. Other lanes disable device auth, and all disable Cloud, managed installation, platform administration and knowledge execution. New deployment/JWT identities are synthetic. There is no inherited commercial-model endpoint or account.
- Provider helpers are named local source files, request port0, and bind only `127.0.0.1`. Startup obtains the actual port from that exact child PID and checks `/health`; Assist uses its own protocol and Recommendation uses its distinct helper. Only those recorded helper processes are stopped. Go compilation uses local/offline mode.
- Existing `switch_api`/registry guards verify checkout and owned listeners. Fresh database identity is checked before migration; API configuration and rebound registry are verified afterward. Failure cleanup retains original error, helper-stop/API-restore/source-audit errors and results separately, and reports failed cleanup as nonzero.
- Detail preflight only reads registry, hashes, listener identity, HTTP health/config/client bytes and a read-only PostgreSQL gate query. It never changes API configuration. `--execute` selects exactly the two detail journeys with zero retries; the fixture independently revalidates allocation, current product hashes, build ID, listener PIDs, loopback origins, legacy auth and manual-only gates before login.
- Detail record fields match the existing `api.running.json`, `web.running.json`, renderer record and `DetailJourneyProvenance` consumer. Read-only file verification during review confirmed that the initial 8,377-file snapshot differs only at `e2e/collaboration-workbench-web.spec.ts` and `packages/core/platform/core-provider.test.tsx`. This is the two-test-only drift allowed by the detail helper, not permission to accept future product changes without a new build/binding.

## Final test diffs

- Lifecycle waiter remains subscribed before the explicitly held request is released. Its 15-second response budget excludes intentional concurrent-actor setup, while stale409, exact request identity, preserved form data and explicit retry assertions are unchanged. This repair is consistent with the root's controlled red/green evidence.
- CoreProvider loading now runs in awaited per-case setup and the mount is synchronous. Singleton resets, 5000ms behavioral budget, StrictMode identity counts and authentication assertions remain intact. The earlier 6-second cold-import red/green and 331-file/4464-test core pass are the evidence. This reviewer authored that repair earlier; this note is a final diff check, not a separate independent author review.

## Validation

Two credential-free helper regressions: **before 2 FAIL; after 2 PASS**. Python compilation and preparation-only legacy manifest invocation also passed. The detail command was not executed, because even default mode probes live services; only its source/schema and existing record files were reviewed.

Private evidence: `/Users/artisan/.multica/qa/core-20261108/legacy-controller-review-{tests.py,red.log,green.log,result.json}`. Frozen password and controlled runner hashes still match the prior review. No further work performed after this review.
