# Current-source browser/runtime preparation

Status: **ready_for_review**, not runtime acceptance. Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

## Ownership and changed files

All orchestration is task-private under `run-context.private/runtime`:

- `runtime_lib.py`: source/tool/build/cache/dependency identity, private environment isolation, execution clearance and named Playwright reconciliation.
- `run_runtime.py`: preparation, owned PostgreSQL setup, compatible auth/native/provider lanes, reviewed HTTP probe hook, final audit and owned database stop.
- `runtime_selftest.py`: counterexamples for source drift, the exact generated Next import, private state, artifact redirection, secrets, duplicate/missing/retried test terminals, optional profiles, probe review and cleanup failure propagation.
- `inputs.json`, `resource-plan.json`, `credentials.private.json`, `profiles.json`, `prepared.json`, `clearance-template.json`: newly generated input/configuration/resource manifests. Credentials are private and must not be printed.

Existing binaries were read and pinned, not overwritten. No product source, existing test, assertion, retry/timeout, dependency, historical allocation or historical result was changed. The only state permission correction changed the owned Git-external `secure/runtime` leaf from 0755 to 0700; shared ancestors were untouched.

## Current identity and evidence

- Inputs SHA-256: `cd92f469855948075b7f0c6e8147ff439befdbde1593ece9cc57abbdf58737de`.
- Tracked source fingerprint: `3ee5bc8f9244d76f142e0fb39cdf5db45446f67f76f8893e82ad1abfbd75297f`; all 32,724 tracked paths, bytes/kinds/modes checked, with only the exact Next generated import normalization permitted.
- Web BUILD_ID: `fG-g1btzMcBigSJYBdIJ0`.
- Current production output bindings: 1,590 Web JS/CSS/map/config/build files; 495 Desktop output files.
- Eight current-source Go binaries are pinned to `qa/runtime/binary-builds.json`.
- Node is explicitly the existing nvm Node22.23.2, including actual child PATH priority. Installed Playwright packages are all 1.58.2; Chromium and headless-shell revision1208 are explicitly bound to the existing cache, with 341 and 19 file bindings respectively. No browser was installed or launched.
- Eight actual workspace dependency edges resolve to this checkout.
- The root-approved six `.png`/`.md` supporting files under `packages/views/.impeccable/` are independently pinned through `qa/source-support-observed.json`. This is current supplemental evidence, not a claim that the first tracked manifest proved their historical bytes. Any changed support byte/mode/path/UID/size, additional path or untracked executable/application source still rejects entry.

Final script SHA-256:

| File | SHA-256 |
| --- | --- |
| runtime_lib.py | `414bb94b13ec670a6e56eaa83c0ba4e20414825ed31dfdecbe952aa59a2339c5` |
| run_runtime.py | `410222d0c8b117abb1b87dbc388c1866a1bacbd56b19699dec6ce24218d028c9` |
| runtime_selftest.py | `617268c5638b2971086e974f9d6f28d8f7790ec50f5ad7069090b145a3c7a898` |

## Checks performed and failures preserved

- Current Python source syntax checked.
- Safety selftest03: **26/26 passed**. It executes only owned temporary Git/filesystem fixtures and fake process/parser/probe boundaries, with no product suite, database, Docker service, browser or production Main.
- Read-only prepare04: exit0; current source, completed Web/Desktop receipts, binary hashes, actual tools/cache/installed image and workspace edges bound. `prepared.json` records tests0/services0/SQL0.
- Original private selftest syntax rejection, prepare01 state-mode refusal and prepare02 unregistered-support refusal remain in separate QA files. Prepare03's successful earlier input generation is retained under `private/runtime/preparation-v03` after the final canonical path/UID/mode guards were registered. Earlier receipt bytes were not overwritten.

## Independent review and exact execution entry

The main session must independently inspect current scripts/manifests and issue `private/runtime/clearance.json` from the exact current `clearance-template.json` only after accepting the real review. The template still says `REVIEW_REQUIRED`; this implementer did not grant execution clearance.

Required clearance fields: `candidate_sha`, `inputs_sha256`, `source_fingerprint`, `resource_plan_sha256`, `source_env_sha256`, `bindings_sha256`, `verdict: accepted`, `blocking_findings_open: 0`. `setup`, `run`, `audit`, and `cleanup` refuse a missing/mismatching clearance or changed source/tools/code/config/build/cache/dependency bindings.

Use an absolute path to the new `run_runtime.py`; shorthand below assumes `RUNTIME` means that existing task-private directory, not a shell HOME override:

```text
python3 RUNTIME/run_runtime.py setup
python3 RUNTIME/run_runtime.py run --mode legacy --label legacy-broad-01 --native --cli --catalog --changelog --expect-count 538 --trace
python3 RUNTIME/run_runtime.py run --mode password --label password-registration-01 --native e2e/password-registration.spec.ts
python3 RUNTIME/run_runtime.py run --mode control --label namespace-01 --native e2e/collaboration-live-revoke-web.spec.ts e2e/collaboration-terminal-reopen-web.spec.ts
python3 RUNTIME/run_runtime.py run --mode host --label host-01 --native --knowledge on e2e/collaboration-host-runtime.spec.ts
python3 RUNTIME/run_runtime.py run --mode device --label device-01 e2e/auth-device.spec.ts
python3 RUNTIME/run_runtime.py run --mode legacy --label inbox-snooze-01 --native --snooze e2e/inbox-snooze.spec.ts
python3 RUNTIME/run_runtime.py audit --output NEW_QA_FILE.json
python3 RUNTIME/run_runtime.py cleanup
```

These are future commands, not recorded test passes. Password requires an explicit compatible file set; do not include legacy-only `collaboration-knowledge.spec.ts`. Independent password files should use separate labels/API lifetimes to preserve registration/login budgets. The runner pins one local ARM64 PostgreSQL image by digest, creates only nonce-named owner-labelled volume/container resources, proves endpoint/container/volume/database identity, and uses a fresh migrated template clone per labelled browser/probe run. Namespace gates and host gates remain separate. Database volumes and failed data are retained; cleanup never force-disconnects clients.

JSON/HTML/collection/trace and process logs go under this run's `qa/runtime/<label>`; HOME/TMP/GOTMPDIR/uploads/config go under its `secure/runtime/<label>`. Collection is saved before test execution, retries/workers remain 0/1, and absence, duplicate identities, retries, incomplete terminals, global errors and nonzero exits prevent acceptance. Skip-only output is not a passing execution. Exact source/config/tool/artifact and owned listener/gate cleanup are checked again before completion; failure changes final exit.

## Optional family profiles and coverage limits

`profiles.json` is presently empty. `--profile NAME` can read only reviewed control-free E2E/collaboration/inbox flags and owned path/loopback templates. The file itself is pinned in current inputs, so edit it only by preparing/reviewing a new input version. It cannot supply secrets, override generated resource bindings, use external URLs/paths, enable a real model account, or opt into production packaged Main. Existing family-specific provenance, output allocations, performance manifests and signed enrollment must still be genuinely prepared; this generic lane support does not manufacture them or remove existing fixture guards.

L7 full production Main and signed/automatic update behavior are not admitted by this runner. Renderer fixtures are separate evidence. Root's early NSWorkspace snapshot establishes feasibility only; a full Main subrun still requires fresh exact default/registration-set snapshots, a stable task-owned app path and exact restoration. Commercial accounts, native iOS/Windows and full 100-user performance protocols remain separately governed.

## Reviewed HTTP probe interface

```text
python3 RUNTIME/run_runtime.py run --mode legacy --label comment-probe-01 --probe ABSOLUTE_TASK_PRIVATE_SCRIPT.py --probe-only
```

The script must be a canonical regular owned `.py` under `run-context.private`, with no group/other write. Its adjacent `<script>.py.review.json` must be a regular0600 JSON containing accepted verdict, blocking0, exact candidate and `probe_sha256`. The actual invocation uses the pinned Python executable; code/review hashes are checked before invocation and recorded.

Probe environment includes only the current owned API/DB/JWT/deployment and private execution state. `MULTICA_QA_PROBE_CONTEXT` points at regular0600 JSON with candidate, api, web, renderer, database, deployment_id, mode and output; Web/renderer are null for HTTP-only scope. `MULTICA_QA_PROBE_OUTPUT` is the required regular0600 JSON report path. The probe must not include passwords, tokens or cookies in its report or stdout. Exit/report/code/source/cleanup failures affect the lane result. `--probe-only` launches only the owned API and is explicitly HTTP integration evidence, never browser/native evidence. `--probe` without `--probe-only` runs after the original browser selection and before cleanup.
