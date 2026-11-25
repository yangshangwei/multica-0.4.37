# Independent comment-authority probe review

Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

Current status: **interface repair verified offline; fresh execution clearance requires root review**. Current program SHA-256: `cd75fad8ac3c0b0cf3be90bee427f1f93a3125c96070893ef0964ac3dfde06e9`. The accepted sidecar still binds the previous program hash and intentionally refuses the repaired program. This reviewer did not self-sign a replacement sidecar and made **0 live HTTP calls** across both review turns.

The original technical review accepted program `3d3811f4144944e7b2f6b920672eca9b700f29cd1d64c5e9f28f2f6654f04d7e`. Its first actual lane, `comment-authority-legacy-01`, was blocked before HTTP by an untested context-path mismatch and produced no probe report. That lane, contexts, logs, and original review metadata remain preserved. Neither the initial review nor this repair is a product permission PASS or a completed E2E run.

## Interface repair after the first actual lane

Root cause: `runtime_lib.py` defines the campaign runtime roots as `run.qa/runtime` and `run.secure/runtime`. `run_runtime.py` emits an owned mode 0600 `probe-context.private.json` under `secure/runtime/<label>/`, and a fresh `probe-result.json` under `qa/runtime/<label>/`. The old probe required its context under `run.qa`. The old preflight fixture placed both files under QA, so its positive test did not match the actual producer.

Before edits, current probe, original and independent selftests, accepted sidecar, and pending template bytes were retained under private `probes/interface-v01/`. Their hashes and modes are recorded in `artifacts/qa/main-post-merge-e2e-20261124-080504/probes/interface-v01-preservation.json`.

The repair changes only the program's `inputs()` function. It now requires:

- Context exactly at `run.secure/runtime/<label>/probe-context.private.json`.
- Output exactly at `run.qa/runtime/<label>/probe-result.json`, canonical and fresh.
- The same bounded `[a-z0-9_-]{1,38}` label on both sides, matching the pinned runtime.
- Canonical, current-user-owned mode 0700 secure campaign, secure runtime, execution leaf, and output leaf directories; the context remains a canonical owned regular mode 0600 file.

The context payload has no label field; the exact directory hierarchy establishes label correspondence. There is no QA-context compatibility fallback or arbitrary-path allowance. Authentication, machine identity, comment claims, upload observations, positive-control stops, report precedence, and sidecar enforcement are unchanged.

The original preflight fixture now mirrors the emitted hierarchy and payload. The existing independent test scratch was moved into the permitted private probe directory. Thirteen new interface tests cover the retained actual emitted context, a matching synthetic runtime hierarchy, wrong campaign or label, the rejected old QA-context location, wrong context/output basenames, wrong output branch, label bounds, directory/file permissions, and symlinked execution paths. All original product assertions remain unchanged; AST comparison also confirms that no program function other than `inputs()` changed.

New evidence under `artifacts/qa/main-post-merge-e2e-20261124-080504/probes/`:

- `interface-runtime-red-01.json` and `.log`: unchanged old program with corrected fixtures and new interface tests; 43 tests, exit 1, three failures and one error. The actual and synthetic valid private contexts were refused; the old QA-context fallback was incorrectly accepted. The original positive case reached the same mismatch.
- `interface-runtime-green-01.json` and `.log`: **43/43 PASS**, exit 0, with `ResourceWarning` treated as an error. AST, compilation, unused-import, and whitespace checks PASS for all four Python files.
- `interface-actual-context-preflight-01.json`: direct pure `inputs()` validation of the retained actual `comment-authority-legacy-01` context and reconstructed emitter environment. Context/config bytes remain unchanged; output remains absent; **0 HTTP calls and 0 subprocesses**. `main()` and the runtime were not invoked.
- `interface-code-diff-01.patch`: exact reviewable differences from the preserved version, including fixture changes and the new interface suite.

Preparation metadata is private `probes/comment_authority.interface-review-pending.json`, with `verdict: pending`, `blocking_findings_open: null`, the new program hash, and evidence references. It does not grant execution clearance. The original accepted sidecar is byte-for-byte unchanged, SHA-256 `aa700883e7e264f9b48e2f6ae2b5ed22262cc0b406fdf523aaceb5980848baf2`, and does not match current code. Root must independently review the diff and dry preflight, issue a matching sidecar, and choose a fresh lane label before execution.

Lint remains scoped Python static checks; no configured Python type analyzer was installed. Product lint/typecheck, runtime input mutation, builds, SQL, and live probe execution were outside this repair turn. The historical findings and initial verification below remain relevant, with the current interface status above taking precedence.

## Initial reviewed scope

The task-private program is `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/probes/comment_authority.py`. The initial reviewed SHA-256 was `3d3811f4144944e7b2f6b920672eca9b700f29cd1d64c5e9f28f2f6654f04d7e`, now retained in `interface-v01/`.

Review covered loopback input restrictions, secret redaction, source/review binding, public human registration and PAT creation, real task-token mint/claim/start, authenticated positive controls, negative comment edit/delete/attachment-only witnesses, upload observations, cross-workspace scope, cancelled-token refusal, and terminal report/exit semantics. The original selftests were preserved, and independent counterexamples were added separately.

Only private probe code, private offline tests, probe evidence, and this research note were changed. Product source, existing repository tests, dependencies, frozen runtime inputs, `.trellis/spec/`, and other tasks were not modified. New conventions were documented here because this is disposable campaign instrumentation rather than a production contract change.

## Findings (fixed)

- File: task-private `probes/comment_authority.py`.
  Issue: inherited HTTP proxies could route the declared loopback requests through ambient proxy configuration.
  Fix: explicitly install `urllib.request.ProxyHandler({})` with the existing redirect refusal handler. The independent test inspects configured handlers without opening a connection.
- File: task-private `probes/comment_authority.py`, `deny_result`.
  Issue: HTTP 409 revision conflicts and 503 dependency refusals were misreported as established permission failures; the same request/authentication distinctions apply to 400/401.
  Fix: an unexpected 400/401/409/503 with unchanged HTTP witnesses is BLOCKED. A state mutation remains FAIL. Explicitly expected 401 on revoked credentials still follows the declared refusal matrix.
- File: task-private `probes/comment_authority.py`, legitimate controls.
  Issue: unsuccessful allowed edit/delete/upload controls could leave the negative matrix running without establishing the endpoint and actor capability.
  Fix: record the control witness, then stop with `SetupError` unless that control is PASS. A failed control cannot be converted into valid negative permission evidence.
- File: task-private `probes/comment_authority.py`, `report`.
  Issue: an empty case collection, unrecognized or malformed case record, or duplicate case name could silently produce a clean report.
  Fix: validate recorded case names/statuses, reject empty and duplicate collections, retain the original rows, and record blocking metadata. Real FAIL retains report and exit precedence.
- File: task-private `probes/comment_authority.py`, `report`.
  Issue: declared NOT_RUN coverage could still produce top-level PASS/exit 0.
  Fix: coverage gaps or OBSERVED_RISK produce `partial`/exit 2 when there is no FAIL or blocking condition.
- File: task-private `probes/comment_authority_review_selftest.py`.
  Issue: the original 21 tests did not protect the independent boundary counterexamples above.
  Fix: add nine offline regression cases; preserve the original suite and original program bytes under `probes/review-original-01/`.

## Findings (not fixed)

- **Existing product authority debt, outside this review's write scope.** `server/internal/handler/comment.go` permits workspace owner/admin management of another author's comment through its role branch in `UpdateComment` and `DeleteComment`. The generic PUT/DELETE routes lack a human-only route guard. The existing `.trellis/tasks/11-20-comment-admin-human-authority/prd.md` remains a separate planning task. This probe must expose any real unauthorized success; this reviewer did not change the product or weaken expectations.
- **Ancillary severity attribution needs parent triage.** In the probe's exercise diagnostic, any FAIL for an actor with `agent_id` receives `P1_known_machine_admin_authority_debt`. That includes a generic unexpected HTTP 500 or a successful positive control with a broken state invariant. This label does not alter case status, proof, raw HTTP status, snapshots, top-level status, or exit code. It is not sufficient evidence of the known admin debt on its own. Leave the diagnostic categorization unchanged in this bounded review; the parent should classify actual failures from operation/status/before/after and reserve the debt attribution for established unauthorized management. This is a nonblocking reporting limitation, not an acceptance waiver.
- **Upload-specific permission policy is not established here.** `server/internal/handler/file.go` validates a same-workspace `comment_id` in generic upload without an observed author check. Successful upload to another author's comment is retained as OBSERVED_RISK. Deciding the intended upload policy is outside this instrumentation review.
- **Unrun identities and side effects remain explicit.** Password registration does not provide the verified email required by the current public invitation API, so password admin/member coverage is NOT_RUN. No usable local Cloud principal is available; fabricated `mcn_` is not authority proof. `mdt_` is accepted by DaemonAuth rather than generic comment Auth, and `mul_` is a human PAT. HTTP snapshots do not establish database-wide audit, notification, WebSocket, or storage cleanup behavior.

## Source and route evidence

- Reviewed ordinary comment write branches in `server/internal/handler/comment.go` (`UpdateComment` approximately lines 3296–3390; `DeleteComment` approximately lines 3556–3650), generic comment route registration, and same-workspace upload validation in `server/internal/handler/file.go` approximately lines 494–508.
- The probe mints an actual `mat_` using public daemon registration, a manually created Agent, assigned issue, claim, and start; it does not run a provider process. `/api/tokens` HTTP 403 establishes machine classification, and the bound Agent's own create/edit/delete controls establish legitimate capabilities.
- Public PAT requests use `expires_in_days`; comment edit requests use the current `content`, `content_base`, `expected_revision`, `attachment_ids`, and `suppress_agent_ids` fields.
- Simulated daemon/CLI versions match the current `e2e/fixtures/core-journeys.ts` fixture convention. This reports HTTP simulation, not real Codex execution.

## Verification

Evidence root: `artifacts/qa/main-post-merge-e2e-20261124-080504/probes/`.

- Original program SHA-256: `15a0e3c03bb3a7d52301fd8fbee07ac53216405aece6307bcab7915ad38ba5ff`.
- Independent red baseline: `independent-counterexamples-red-01.json` and `.log`; all nine independent counterexamples failed against original bytes, exit 1. No live calls.
- First green baseline: `review-regressions-green-01.json` and `.log`; 21 original plus nine independent tests PASS, exit 0.
- Final fresh verification: `review-final-offline-01.json` and `.log`; **30/30 PASS**, exit 0, with `ResourceWarning` treated as an error. AST parsing, compilation, unused-import inspection, and whitespace checks PASS for all three Python files.
- Lint: scoped static checks PASS. Repository Go/TypeScript lint was not rerun in this child; the parent owns product verification.
- TypeCheck: not applicable to the task-private Python probe; no configured Python type analyzer was installed. Repository TypeScript checks were not rerun in this child.
- Probe and selftest hashes were unchanged between the first green baseline and final verification. HEAD matched the candidate during final verification. No live product result is claimed.

## Initial parent handoff (retained history)

The initial review sidecar is `probes/comment_authority.py.review.json`, owned by the current user and mode 0600, with `verdict: accepted`, `blocking_findings_open: 0`, the exact candidate, and the old program hash. It remains preserved and unchanged, so it refuses the repaired code. The original pending template is also retained as historical preparation evidence.

The original command handed off to the parent was the following. Its label is now occupied by the preserved first blocked lane; do not rerun or overwrite it. A new command requires root's matching review sidecar and a fresh label.

```text
python3 /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/runtime/run_runtime.py run --mode legacy --label comment-authority-legacy-01 --probe /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/probes/comment_authority.py --probe-only
```

Report precedence is `fail`/exit 1 for any recorded FAIL; otherwise `blocked`/exit 2 for blocked cases, setup/runtime errors, or invalid recording; otherwise `partial`/exit 2 for OBSERVED_RISK or NOT_RUN gaps; otherwise `pass`/exit 0. Positive-control failures stop the negative matrix. The parent must retain these statuses and limitations in the campaign report.
