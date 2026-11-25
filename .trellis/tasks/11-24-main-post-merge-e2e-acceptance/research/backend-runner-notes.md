# Independent backend preparation review

Status: **PASS / ready_for_execution**. This is technical admission for the task-private backend runner, not a passing product, database or browser result.

## Reviewed scope and source

- Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.
- The original 32,724-file tracked source manifest and its fingerprint remain unchanged. Current source audit passed. No product or existing product-test source was edited.
- The six existing non-executable PNG/Markdown support files were separately pinned from `qa/source-support-observed.json`. This records current supplemental identity, not their historical bytes. New untracked code, scripts or environment inputs are rejected.
- Private code/config/evidence root: `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend`.
- Private fixture state: `/Users/artisan/.multica/qa/main-post-merge-e2e-20261124-080504/backend`; HOME/scratch/package homes remain outside Git on the trusted ownership/mode chain.
- All review execution was private fake-process, small Git-fixture, parser, formatting or read-only collection work. **No product TestMain/test body, SQL, Docker service or API was started.**

## Findings fixed

1. `backend_lib.py`: the original source audit ignored new untracked source. It now rejects new untracked inputs across apps/packages/server/scripts/e2e/.github and Go workspace files, while separately validating the exact six permitted support paths, hashes, sizes, ownership and modes. Counterexamples cover Go/C/header/assembly/module/script/workspace inputs.
2. `backend_lib.py`: empty named identities and package terminals before unfinished named tests, or further test events after a package terminal, could previously count as complete. These now refuse acceptance. Valid mixed Go build-output events, including empty ImportPath, remain supported. Stream readers now close explicitly rather than emitting ResourceWarning.
3. `go_guard.py`: matching an ImportPath string did not prove the recursive command used this physical checkout. Working directories must now belong to the frozen server module, and actual `go list -json` ImportPath/Dir must match the frozen package inventory. Foreign same-name packages and an empty Dir are rejected before test execution.
4. `run_backend.py` / `tools.json` / `config.json`: the saved PATH preferred Node25. The actual subprocess Node22 executable and pnpm are now pinned by path/hash, Node22 precedes inherited paths, and resolution is checked. No dependency was installed.
5. `run_backend.py`: TZ now reads the reviewed configuration. Database cleanup observations count client backends, distinguishing PostgreSQL maintenance workers from application connections.
6. Unused private Python imports were removed. The private process fixtures now model the server-module directory and JSON package metadata accurately.

## Verification evidence

Evidence is under `artifacts/qa/main-post-merge-e2e-20261124-080504/backend/`.

- Original private suite: `resume-selftest-01.json`, **25 PASS**.
- Original source/parser counterexamples: `independent-counterexamples-red-01.json`, **4 failures reproduced**. That first test entry also rediscovered imported base cases; the later entry restricts discovery to the intended suite.
- Original foreign-directory counterexamples: `independent-counterexamples-red-02.json`, **2 failures reproduced**.
- Empty package-directory counterexample: `empty-directory-red-01.json`, **1 failure reproduced**.
- Final suite: `review-regressions-final-01.json`, **33/33 PASS**, stable before/after code hashes and ResourceWarning configured as an error. The source-input case has seven filename/type subcases.
- Python syntax and unused-import review: PASS. Go AST collector formatting: PASS. No Python static typechecker/linter is installed; no such tool PASS is claimed and no new dependency was added.
- `race-collection-review-01.json`: read-only normal/race collections agree on **86 packages, 11,765 Test functions and 1 Fuzz function**. There are 13 declared benchmarks not selected by the default test command. Dynamic subtests and no-test packages will be reconciled against actual execution, not treated as discovery PASS.
- Original private bytes/input records are retained in `review-original-01/`. No previous campaign allocation, database, receipt or clearance was reused.
- `review.json`, `execution-readiness.json`, current private `inputs.json` and `clearance.json` bind the reviewed code, tools, configuration, planned resources and preserved original source.

## Execution handoff

Use the following current private runner, retaining complete output/exit and never printing credentials:

```sh
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/run_backend.py setup
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/run_backend.py execute normal
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/run_backend.py execute race
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/run_backend.py static
python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/run_backend.py audit
```

Setup first verifies current clearance/source/tool/config bytes, then creates only the planned label-owned ARM64 PostgreSQL/Redis resources and a fresh migrated template. Each normal/race package receives a distinct fresh database, Redis logical database and private HOME. Original 40-minute package deadline and parallel budget remain unchanged; default Agent CLI sentinels remain active.

The current configuration runs normal and race in Asia/Shanghai. A required UTC replay needs its own admitted configuration and fresh run; do not relabel a Shanghai result as UTC. Interrupted or previously executed mode directories refuse reuse rather than silently retain PASS.

The audit command observes owned resources; it does not stop them or establish browser/platform acceptance. The coordinator still owns actual runtime identity, complete named inventory, database/client audits, final source/tool checks and stopping only proven owned processes/resources. Go1.27/macOS arm64 evidence does not substitute for CI Go1.26 or other OS native execution.
