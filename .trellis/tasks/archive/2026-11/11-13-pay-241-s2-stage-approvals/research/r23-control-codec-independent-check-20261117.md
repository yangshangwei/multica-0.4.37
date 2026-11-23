# R23 independent codec check

**Scoped verdict: ACCEPTED. Blocking findings: 0.** The frozen two-file test-control codec candidate is accepted for main-session integration. The original R22 four-case result remains **1 PASS / 1 FAIL / 2 NOT_RUN**; full S2, A01–A12, parent acceptance and real S4 integration remain open.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`. Reviewer: `/root/r23_codec_check_seal`. Sealed at `2026-11-17T00:03:29.028453+00:00`.

## Findings (fixed)

None required reviewer edits. The reviewed implementation fixes the observed decoder mismatch at `e2e/fixtures/collaboration-stage-control.ts:148` by replacing only `version(last.conditional_version)` with existing `text(last.conditional_version, 256)`. The retained RED codec and both recomputed integration patches establish the exact scope; all other codec bytes are unchanged.

The provider conditional state is nonempty valid UTF-8 text of at most 256 bytes without NUL or lone Unicode surrogates. This matches the existing server `stageBoundedText` contract. The original decimal validator remains positive canonical **signed-int64**, bounded at `9223372036854775807`; candidate, native, initial configuration conditional version and configuration revision checks retain that validator. No uint64 widening is claimed.

The closed request/reply and invocation envelopes, duplicate-key validation, UUID/digest/count validation, result shape, 64 KiB frame limit, 20-second timer, socket ownership, one explicit send and uncertainty/no-replay behavior remain byte-identical. No dependencies, platform/configuration, runtime or template changes are part of the candidate.

## Findings (not fixed)

**No unresolved scoped findings.** No code, product specification or public interface fix is required by this review. The existing diagnosis and implementation reports preserve the opaque-state distinction; the repair restores an existing provider/service contract.

Main owns integration, fresh matching test-source/runtime binding, remaining full four-case/A01–A12 acceptance and lifecycle/cleanup gates. These remain uncompleted acceptance work outside this codec review. The original R20 lock holder remains unproved. A successful local parser/socket regression or retained durable rollback record cannot close those gates.

## Verification

All check outcomes below are **independently validated retained raw executions**. This reviewer did not rerun tests, lint or type-check. The only new Node operation extracted a data literal using TypeScript AST and compared it with the retained reply; it imported no fixture, created no socket and executed no test runner.

| Verification | Retained actual result | Result SHA256 |
| --- | --- | --- |
| Original codec + new regression tests | 16 tests: **14 PASS / 2 FAIL**, exit 1; zero skip/cancel/todo | `76205e5cf02fbf9db85c7833f363476d070d2439e7d15026773878752af9bf72` |
| Initial GREEN | **16 PASS**, exit 0; zero fail/skip/cancel/todo | `03ae6f78a6623edbfe3db977b9ee03c06ef1ac62079cac44f662defabf73f3c0` |
| Final GREEN with actual package context | **16 PASS**, exit 0; zero fail/skip/cancel/todo | `d3adcd8b7e2db5d95f2afd3cfbc8beb316ebafb4d7cff392da1e9b20ef13c829` |
| Final TypeCheck | **PASS**, `--noEmit --strict`; exit 0, empty stdout/stderr | `d46651da0a50354190037db761a6fb2cd7f217bdfb1a0c2cd8d8e3b797dd21a5` |
| Final actual two-file lint | **PASS**, exactly both files; 0 errors/warnings/fatal/suppressed messages | `a2e55bf8449625ecb6f4d3f1c517e55ef1ffbb6815b95fac50e50c419fab3af4` |
| Initial lint environment failure, retained | exit **2**, null ancestor package-path error; never counted as PASS | `f69d5131e02ad4be139d29e079db7078027d516edbf43cc1f486f6ad00968df5` |

Raw TAP was reconciled against all 16 ordered source test names, terminal records, actual exit codes and summary counts. RED's only failures are the retained successful-rollback parser regression (`Invalid stage control version`) and the one-send fake-socket confirmation regression (generic unconfirmed result). GREEN retains all 11 original tests plus five added tests. Initial and final GREEN are separate observations of the same inventory, not additive counts. Source-before/after hashes agree for every recorded run.

The new tests cover the exact opaque `1/after/01a1e575-3092-7cb9-bfa2-8a28d87745ff` reply, ASCII/multibyte 256-byte boundaries, empty/null/missing/non-string/NUL/lone-surrogate/over-limit refusals, unchanged request decimal constraints and local one-send confirmation/refusal. Original wrong/duplicate/unknown envelopes, identity, count/digest, disconnect/no-replay, concurrent duplicate, private-socket, frame/UTF-8 and completion-fault controls remain. Socket tests use synthetic request `11111111-1111-4111-8111-111111111111`.

Type-check covers the exact codec TS file with ES2022/NodeNext and Node types; `--skipLibCheck` excludes dependency declarations. The owning `.mjs` is exercised by retained Node tests and included in actual lint. The lint JSON contains both exact file paths and empty messages; an ignored-file exit 0 is not used. The explicit private lint config retains existing repository base rules and the dependency rule at error severity with actual package context.

The first lint failure and unchanged config/raw outputs remain preserved. Both source roots' existing `package.json`, installed dependency and adjacent immutable completion-helper links resolve to their recorded targets; package/helper/tool/config hashes match. No package was installed and no linked target was changed. Node `22.22.0`, TypeScript `5.9.3` and ESLint `9.39.4` are the recorded installed tools. Commands retain explicit argv/cwd/owned HOME/TMP and no inherited ambient environment.

## Actual R22 reply and preserved result

The new offline AST check proves that the **complete** test literal equals the retained actual R22 reply payload. Original request `61b4733a-f70e-424c-8f4b-c337ea9c3f52`, issue/approval/intent/fact identities, rollback/succeeded result, count 2, both digests and opaque token match exactly. That request was never sent or replayed during this review, and no original sent wire-frame capture is claimed.

Retained line SHA256: `b00bc0a507bf3c9d8652964f8880da4d6e2bdf826c247b13fc25ad82719c71c6`. It still equals actual harness log line 24. The first-24-line prefix remains `30e41954b0128cc2ecc5c87416220939df193957629fa25f606d3aaf4ed2aa21`. At this review the log has 26 lines and full hash `9b852ce72bce57ddb885f789605428f71d5cc740a0707a9294e51103b677d7ed`; later log appends are separate observations and do not alter the frozen prefix.

The unchanged original four-case result hash is `a6a9b58fe105b95a5497ac014cb598d43efb1457a96dcd338eb4058a8941f968`. Its four cases still map to PASS, FAIL, NOT_RUN, NOT_RUN with retry 0. The retained after snapshot `e6fd17ce87aea0b97021b5299d321201c9ca5fe2dd5a0f7a8435e0ac6f142ff3` corroborates the exact successful rollback intent/observation. It was read offline, and is not a current DB, shutdown or full scenario success claim. Later rollback UI/receipt/screenshot assertions were not reached in R22.

## Frozen integration bounds

| ROOT path | Current pre-integration SHA256 | Accepted candidate SHA256 | Both modes |
| --- | --- | --- | --- |
| `e2e/fixtures/collaboration-stage-control.ts` | `cd3d1e26322312dd1ad01e5d86c8fb20df82334356b6a4d436293f95475adb76` | `a6e433acdb92f943cbe0c1288908ffcfe5ab667b2a0771106c7a090979e51c96` | 0644 |
| `e2e/fixtures/collaboration-stage-control.node-test.mjs` | `af4b5aa148ec5855bc068a195c93f2c25c1e29121cd5dabedbe537a2aebab966` | `479bc8b47e5d533a6cb1db728b857529c7a48b09ecf3b77b1fc03cf46334f3dc` | 0644 |

Main must recheck these two ROOT hashes/modes immediately before integration and preserve peer edits. Integration patch hash: `ef9728ee25cf87ab27983371c3cdc296157bf974388727b3bf6bde38fe043a60`; one-field codec patch: `9e71916b9454d9c6d0880b82a5f06c00d2937fdaa540bc08173d6fb2487a3a22`. All three retained source/integration patches were recomputed and match exactly. Both private source roots contain exactly the two owned regular files, plus explicitly recorded support symlinks.

Handoff SHA256: `35a205a51b2f3f04ed42be98a987e171b94985a2f0170d688c0bdd46941f2f17`. All **53** existing handoff/implementation/source/raw-static-result references were hashed independently with zero mismatches. Additional selected original R22 result/snapshot/prefix and offline AST evidence were checked separately. The detailed check artifact binds each current reference and mode: [verification-audit.json](/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/r23-control-opaque-version-r01/check-seal-r01/verification-audit.json), SHA256 `67a252c699644cc7b68b056e73dd4395bc03f2dc6180ce1af455f726ee3011c0`.

Only this report, its JSON companion and task-private `check-seal-r01` artifacts were written. ROOT product paths, runtime configs/processes, DB/API, network and Git were kept read only; no new test suite was executed. S2 stays in progress.
