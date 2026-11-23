# Independent scripted-simulation review — 2026-10-27

Reviewed the seven `pilot/simulation/` files,
`simulation-plan-20261027.md` and the exported simulation records.
Only offline validation, its local tests and README changed. The provider,
wrapper, Dockerfile, gateway and captured evidence were not modified.

## Findings (fixed)

- File: `simulation/validate_evidence.py`
  Issue: mutations could remove a product comment's `source_task_id`, change
  the task workspace or diagnostic audit actor, or make a leader return
  imprecise/direct/unrelated to its evidence comment without invalidating the
  reported success. A nonzero `real_model_calls` value was also ignored.
  Fix: bind marker, product comment, task, issue/workspace and audit identity;
  require precise comment-based delegation for returns; bind the final genuine
  leader task to the last member return; reject nonzero model calls. Duplicate
  task/comment IDs and malformed result stages also fail validation.
- File: `simulation/test_provider.py`
  Issue: no regression coverage for these offline acceptance checks.
  Fix: retain the recorded evidence as a passing control and mutate only
  temporary copies. Twelve negative cases now reject wrong actors, direct
  member/leader execution, missing returns/source-task IDs, imprecise or
  unrelated return evidence, wrong terminal activity, nonleader final turns,
  wrong workspace/audit identity and nonzero model calls. Seven of these
  mutations reproduced false acceptance before the fixes.
- File: `simulation/README.md`
  Fix: document the offline snapshot dependency and stronger evidence checks.

## Findings (not fixed)

No remaining blocking defect was identified in this bounded simulation review.

The real execution provider is still the deterministic Python double. Security
judgments, role choices and patches are scripted; this is not autonomous AI
review, production remediation or completion of the original real-model gate.
The shared UID and lab do not isolate roles from one another.

The reviewer did not start a daemon, mutate the API, inspect credential files
or invoke a real agent/model. Network/profile isolation and cleanup have captured
operator evidence; live re-verification and final cleanup remain with the main
session. The validator's `real_runtime_invoked: false` means no real AI provider;
the Multica daemon and application workflow exercised by the recorded run are real.

## Boundary review

- The wrapper explicitly executes `/usr/bin/python3` on `provider.py` in both
  UID paths. The Dockerfile installs no Codex/OpenCode CLI. The fake version
  and `model/list` responses are labelled protocol fixtures.
- Provider HTTP reads use the task token and fixed configured API. Writes use
  the Multica CLI's inherited task configuration. Human tokens fail the harness
  check before networking; server authorization remains the actual authority
  boundary. The gateway preserves incoming API Authorization and has only a
  fixed development API upstream; model/unknown paths are denied.
- Stage lists are finite, the coordinator dispatches one member then returns,
  and task-count overflow fails visibly. Captured records demonstrate distinct
  implementation, diagnosis and QA actors, precise member returns, `action`
  after dispatch and final `no_action` at `in_review`.
- The provider SHA-256 still matches the recorded run. A credential-pattern
  scan of 20 exported text/JSON/log/patch files found no matching task/PAT/model
  tokens, bearer material or JWTs; this is a bounded pattern check.

## Verification

- Lint/syntax: pass — shell `sh -n`, Python AST parsing and whitespace checks.
- TypeCheck: not applicable; no configured checker for these task-local
  Python/shell/nginx/Markdown artifacts. Full product suites were not rerun.
- Tests: **14 passed**, 0.343 seconds:

  ```sh
  python3 -B .trellis/tasks/10-26-security-remediation-squad/pilot/simulation/test_provider.py
  ```

The strengthened validator reproduces the existing exported validation JSON
without changing the records:

| Case | Successful tasks | Retained failed attempts | Verified member stages |
| --- | ---: | ---: | ---: |
| Supply chain | 3 | 1 | 1 |
| Dependency | 14 | 1 | 6 |
| Authorization | 11 | 0 | 5 |
| Escalation drill | 1 | 0 | 0 |

All four captured issues are `in_review`; all 31 captured tasks are terminal.
The two failed attempts remain explicitly acknowledged with matching product
errors. QA records retain 3 and 6 passes; the diagnostic retains its expected
failure before the separate implementation step.

Independently checked the captured handler test stream: exactly 24 expected
top-level tests ran and passed, with zero skips or failures. Those database
tests were executed by the main session, not rerun by this reviewer.
