# R23 integrated codec check

**Verdict: INTEGRATED_CODEC_CHECK_ACCEPTED. Blocking findings: 0.** Fresh ROOT-source checks pass. The original R22 four scenarios remain **1 PASS / 1 FAIL / 2 NOT_RUN**; full S2/A01–A12 and parent acceptance remain open.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`. Reviewer: `/root/r23_integrated_codec_check`. Observed at `2026-11-17T03:32:44.736416+00:00`.

## Findings (fixed)

No product or test code fix was needed. Main's two ROOT files and isolated `frontend-base-r01` copies match accepted `codec-green-r01` bytes. ROOT modes0644 and isolated modes0600 are preserved separately. The initial integration guard failure remains recorded; no source had been written at that failure.

The sole decoder delta at `e2e/fixtures/collaboration-stage-control.ts:148` is `version(last.conditional_version)` → existing `text(last.conditional_version, 256)`. The remainder of the codec is byte-identical. Provider conditional state remains nonempty valid UTF-8 of at most256 bytes, without NUL or lone surrogates, matching `stageBoundedText` and the existing controlled provider's `/after/<intent>` state. Candidate/native/configuration revisions retain positive canonical signed-int64 validation. Closed envelopes, UUID/digest/count validation,64KiB frames,20-second timer, private socket checks, one-send and no-replay semantics remain unchanged.

An independent source AST comparison verifies all41 original Node statements and all11 original test bodies remain byte-identical and ordered. The seven added statements are two data constants and five new parser/socket regressions; the suite now has16 tests.

## Findings (not fixed)

No unresolved scoped findings. No spec change is required: the test-only reply decoder now follows the existing server contract, and the distinction is already preserved in the R23 diagnosis.

Main owns fresh source/runtime binding, original four-scenario execution, A01–A12 and final resource/lifecycle checks. This local acceptance does not change the original R22 failure or complete S2/S3/S4/S5/parent acceptance. Original rollback request61b was never sent or replayed.

## Verification

All checks below are **new actual ROOT executions**, with explicit argv/cwd, a dedicated Git-external HOME/TMP, trusted ancestor checks and no inherited account environment. The socket tests preserve their original privately owned `/private/tmp` fixtures. No dependency was installed.

| Check | Actual result | Result SHA256 |
| --- | --- | --- |
| Node22 protocol/socket suite | **16 PASS**, exit0,0 FAIL/SKIP/cancel/todo, exact16 ordered TAP terminals | `5583da6abb7b8e2abc1c568e53b2f0de5a067f8e4a750f14e03507e5669383ae` |
| Strict TypeScript | **PASS**, noEmit/strict ES2022/NodeNext, exit0, empty stdout/stderr | `77e790d5784ac84ce0285277f0a0ff75ba6291576375e1d8bf760d2285ba3ae5` |
| Actual two-file ESLint | **PASS**, exit0, exact ROOT TS+MJS files,0 errors/warnings/messages/suppressed messages | `ae16e1a099ff4c9d82156b583d9eb5e64ca9fbc4fcc891e85215364c213a9777` |
| Offline AST preservation | **PASS**,41 original statements/11 original tests preserved,5 new tests | `7fdc855d9d599d00e621101adf5104cc40e94ab19d223f916b2b864fedd45a80` |

`--skipLibCheck` excludes dependency declarations; MJS is exercised by Node and actual lint. Lint retains the accepted repository base rules and import dependency rule at error severity with real ROOT package context. Both exact lint file entries have empty messages; ignored-file success is not counted.

Ten inputs, including four ROOT/private copies, the actual adjacent completion-fault helper, package/lint context and selected tool entrypoints, were rehashed with permissions before/after every run and at sealing: **drift0**. All retained handoff raw-result/command/stdout/stderr references and the original R22 result were rechecked. HOME is empty; TMP retains only an explicitly inventoried owned Node compile cache, created by the installed TypeScript and ESLint `enableCompileCache` shims. An initial report-only empty-TMP assertion failed before any report/source write; its receipt and the correct cache classification are retained, without rerunning or changing tests.

The controlled-provider source lookup was corrected from an absent service test path to the existing `server/cmd/server/collaboration_stage_execution_source_test.go` before reading; this had no code/test effect.

Detailed [JSON](r23-integrated-codec-check-20261117.json), SHA256 `21eb5d978f19302be3d1ca7cefb17eaebdf2accdc94eb292345f01431ae43990`. Fresh raw/input records are in `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r23-control-opaque-version-r01/integrated-codec-check-current-r01`; summary SHA256 `68384142fbc9cccffc32fc60d7fc66b66621174f1ddad691ecd86b57999e8ab8`.

Only two new research reports and the task-private verifier/log/cache inventory were written. ROOT code, configs, previous evidence, DB/API, deployments and Git were unchanged by this reviewer.
