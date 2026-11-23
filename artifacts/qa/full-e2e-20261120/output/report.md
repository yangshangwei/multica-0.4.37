# Controlled Agent output completion

The final Web and native Desktop output journeys both passed against a fresh task-owned password API and native arm64 PostgreSQL. Final evidence is `browser-final.json`, `browser-final.log` and `browser-final-results/`; `summary.json` contains the exact two test identities and per-platform metrics. The earlier two passing executions are retained separately and do not increase the unique-case count.

| Final check | Result |
| --- | --- |
| Web controlled output | Passed, 35.071 seconds |
| Desktop controlled output | Passed, 39.625 seconds |
| Browser totals | 2 passed; 0 failed, skipped, flaky, not run or retried |
| Guard regression tests | 20 passed; 0 failed or skipped on macOS |
| Fixture ESLint | Passed for all three output fixture files |
| New guard JavaScript type check | Passed with checkJs/strict; implicit JavaScript parameters permitted |
| Independent guard review | Passed; `../stage/output-guard-independent-review.md` |
| Cleanup | Passed; owned services stopped, original gate booleans restored |

Each browser case completed all eight business subcases and retained 175 checkpoints: original lost-reply CLI recovery, same-key concurrent replay, changed-content/metadata refusal, frozen failed output, explicit fresh success, Stop, author withdrawal and grant withdrawal. Each observed five physical provider processes, six output CLI invocations, five original permit identities and zero recovery launches. Five output attachments were retained; two runs recorded results and three stopped/withdrawn runs did not. All daemon, observer, transport, session and gate cleanup steps fulfilled.

## Fix and review

The original fixture admitted only a historical database prefix and absolute private storage directory. `guard-red.log` retains its rejection of the valid current allocation. The fixture now calls a read-only allocation guard before gate mutation or registration. The explicit allocation binds the exact active task, checkout, owner0700 external state/storage, owner0600 record, upload directory, service origins, deployment ID and connection URL digest. Partial explicit configuration fails without historical fallback. No historical resources were accessed.

Independent review identified that `pg.Client` honors URL query host/port overrides despite a loopback `URL.hostname`. The final guard refuses query strings and fragments before connecting. Regression tests prove actual effective connection parameters without opening a connection and reject those overrides in both paths. Final browser execution followed that correction. The historical exact predicate is otherwise retained as requested.

Changed source/spec files:

- `e2e/fixtures/collaboration-runtime-output.ts`
- `e2e/fixtures/collaboration-runtime-output-allocation.mjs`
- `e2e/fixtures/collaboration-runtime-output-allocation.node-test.mjs`
- `.trellis/spec/web/frontend/e2e-run-environment.md`

`fixture-and-spec-final.diff` contains the complete bounded change. No production behavior, assertion deadlines, browser assertions or dependencies changed. The inline historical admission logic moved into the guard so the same entry point owns the current and historical checks.

The initial workspace-relative ESLint call ignored root E2E files; a first root configuration then exposed fixture package-resolution mismatches. Those outputs remain in `lint-initial.log` and `lint-root.log`. The final task-only configuration applies repository base rules from the root, explicitly recognizes E2E dev dependencies and resolves the existing shared package manifests. All three files pass in `lint-final.log`; no repository lint configuration or package dependency was changed. Broad fresh forced workspace lint/type checks are independently recorded in `../stage/review-static.log`.

## Runtime and cleanup

Owned allocation: `check-20261120013547-41191`, state directory `/Users/artisan/.multica/dev/envs/check-20261120013547-41191`, database `multica_qa_20261120_output_18581`. Final recorded API/Web/renderer ports were 18581/13501/5675. The initial and final allocation manifests contain connection digests only. Docker reassigned its ephemeral PostgreSQL port on restart; `allocation-before-container-restart.json`, `allocation-final.json` and `effective-pg-configuration.json` preserve the rebinding and actual driver destination.

The final run used production Web build `ppJO9I3pKqFza7qn0WOdt`, the corresponding Desktop renderer/preload and an explicitly built test daemon/provider/CLI. `source-artifact-bindings-final-before.json` and `source-artifact-bindings-final-after.json` show no bound-byte changes during execution. API/Web/native listener provenance is retained in the `*-final.running.json` records.

`cleanup.log` and `cleanup.exit` prove normal shutdown. All three ports are free; the owned PostgreSQL container is stopped and `owned-processes-after.json` has no remaining processes. Gate booleans match the allocation baseline exactly: creation=true, execution=false, host execution=false; epoch increased from 2 to 10 across both executions. The stopped database and upload evidence remain private for inspection.

This establishes controlled local daemon/CLI output plus Web/native-renderer behavior with a deterministic provider. It does not establish commercial model quality, authenticated provider accounts, packaged Main, installers, mobile or updater acceptance. Concurrent unrelated security locale changes were outside the retained shared client build; the root session owns their subsequent rebuild and targeted replay.
