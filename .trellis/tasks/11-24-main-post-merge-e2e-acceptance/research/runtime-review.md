# Independent runtime execution review

Status: **ready_for_execution / PASS**. Open technical blockers: **0**. Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

This review checked and changed only the task-private runtime recorder. It executed no Docker service, SQL, live API request, browser, production Main or product test. The full saved native hook was read before the check. Current task requirements, the production-Web/native environment contract, per-package/database isolation rules and final evidence contract were applied.

## Findings fixed

- `runtime_lib.py` and `run_runtime.py`: the original production binding selected JS/CSS/maps, omitting HTML, RSC, JSON, fonts and other served payloads. The current binding includes every production file while explicitly excluding only `cache/`, old `dev/` output, `trace` and `trace-build`. It contains 1,192 current production files. All three installed Playwright package code trees are also bound, together with the browser installation trees.
- `runtime_lib.py`: empty test IDs, same-ID changes to file/title, unexpected/flaky classification and result-level errors could bypass reconciliation. Identity/record shape and original collection metadata are now checked; empty collections cannot pass.
- `run_runtime.py`: `audit` previously accepted zero runs or ignored a run directory without completion. Each attempt now records intent; final audit requires completed attempts, required original evidence and matching input/raw-evidence hashes. Browser reports are reconciled again, and cleanup/process results are compared independently.
- `run_runtime.py`: probe exit0 and an arbitrary JSON file could be accepted despite a report declaring failure. The report must explicitly declare `pass` or `passed`, have no errors/failures/blocking findings, and match the candidate when supplied. The probe owner was informed of this contract.
- `run_runtime.py`: live resource verification now includes exact container name and image, one loopback port mapping, and the expected writable named volume. Source/tool/artifact bindings are checked immediately before launch, migration and first fixture writes. The current Go executable also has to match the compile-only binary provenance.
- `runtime_selftest.py` and the new `runtime_review_checks.py`: the original 26 checks were retained. Seven independent checks cover the recording gaps and complete static-payload binding. The original six newly executed tests produced seven failed invariant observations; their raw red receipt is preserved. The final 26 + 7 checks all pass.

## Findings not fixed / scope limits

- No product behavior, existing E2E assertion, timeout, retry, skip, dependency or old campaign was changed.
- Full packaged Main/installed upgrade, exact LaunchServices restoration, commercial models, native mobile/Windows and fixed performance-family manifests remain separately governed. This clearance admits only the declared current-source coordinator and compatible lanes.
- Profiles remain empty. Adding family-specific profiles, binaries or allocation files requires a new input version and review; this review does not fabricate those prerequisites.
- There is no configured Python type checker/linter for these private files. Current Python sources passed AST compilation, syntax and whitespace checks. The parent has already retained passing same-source project lint/typecheck and mobile checks; these product checks were not repeated.

## Verification and current bindings

- Safe tests: **33/33 PASS**; no product test or service executed.
- Read-only reprepare: **exit0**. Actual `verify_bindings()` and `admit()` against the new clearance: **PASS**.
- Source: all **32,724 tracked paths/bytes/kinds/modes** remain matched, with only the exact declared Next import normalization supported. The six support PNG/Markdown files use the root's separate current-observation binding; their historical bytes are not asserted.
- Current inputs SHA-256: `a14fab1e4a4895bff757d7e41f8977c5e7ed8096071aa689cabed8e9c713592b`.
- Web BUILD_ID remains `fG-g1btzMcBigSJYBdIJ0`. Current Desktop outputs, eight compiled binaries, Node22, installed Playwright1.58.2 package/browser trees, source dotenv hashes and physical workspace edges were checked.
- JSON review: `artifacts/qa/main-post-merge-e2e-20261124-080504/runtime/independent-review.json`.
- Technical clearance: `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime/clearance.json`.
- Red evidence: `runtime/independent-review-red-01/`; final safe tests/prepare: `runtime/independent-review-green-02/`. Earlier inputs and sources remain in `private/runtime/review-before-01/` and `review-inputs-before-05/`.

## Declared execution handoff

The main session can now invoke the current private runner using absolute paths:

```text
python3 RUNTIME/run_runtime.py setup
python3 RUNTIME/run_runtime.py run --mode legacy --label legacy-broad-01 --native --cli --catalog --changelog --expect-count 538 --trace
python3 RUNTIME/run_runtime.py run --mode password --label password-registration-01 --native e2e/password-registration.spec.ts
python3 RUNTIME/run_runtime.py run --mode control --label namespace-01 --native e2e/collaboration-live-revoke-web.spec.ts e2e/collaboration-terminal-reopen-web.spec.ts
python3 RUNTIME/run_runtime.py run --mode host --label host-01 --native --knowledge on e2e/collaboration-host-runtime.spec.ts
python3 RUNTIME/run_runtime.py run --mode device --label device-01 e2e/auth-device.spec.ts
python3 RUNTIME/run_runtime.py audit --output NEW_QA_FILE.json
python3 RUNTIME/run_runtime.py cleanup
```

Run independent password/admin files in distinct lanes so their original registration/login budgets remain meaningful. `audit` is performed while the owned database is running, before `cleanup`; cleanup retains the named volume and refuses to force-disconnect clients. Probe-only runs need their own independently reviewed script and remain HTTP integration evidence. None of the commands above was executed by this reviewer.

## Display-only recovery after legacy-broad-01

The main session's first runtime lane reached the correct owned API, production Web and renderer but did not execute browser test bodies: installed dotenv printed an environment banner before the collector JSON, and strict `json.loads` correctly failed. The original lane remains `exit1 / browser_exit2 / inventory incomplete`, with closed owned processes and ports. Its files were not rewritten.

- The installed `dotenv/lib/main.js` explicitly reads `DOTENV_CONFIG_QUIET`. `runtime_lib.clean_env()` now supplies `DOTENV_CONFIG_QUIET=true` after inherited/source keys are emptied; both collector and test child receive it. `e2e/env.ts`, the strict JSON parser, test selection, assertions, retry and timeout remain unchanged.
- A real **read-only** collector under the original effective environment plus that display setting returned **538 cases / exit0**, starting with `{`, and the entire stdout parsed as strict JSON. No test body, SQL or service was executed by this recovery check.
- Original and independent private safety checks remained **33/33 PASS**. Syntax checks passed.
- Current runtime input revision is `dotenv-quiet-01`, SHA `80bc39dd1a392b93766074a752e3de24719898256732a04555558020af5191d0`. The matching clearance was independently accepted and actual `admit()` passed.
- Existing resource-plan, credentials, database allocation, readiness and setup-start bytes were checked unchanged against `private/runtime/quiet-before-01/`; container, volume and port ownership were rechecked through read-only Docker inspection. **Do not rerun setup.**
- Evidence is in `qa/runtime/dotenv-quiet-review-01/`; old source/input/clearance/allocation bytes are retained in `private/runtime/quiet-before-01/`.

The next actual command is the same original full selection with a fresh label:

```text
python3 RUNTIME/run_runtime.py run --mode legacy --label legacy-broad-02 --native --cli --catalog --changelog --expect-count 538 --trace
```

The original failed version remains visible in an all-history audit and must be classified by its source/input version and setup failure, never relabeled as a successful browser execution.
