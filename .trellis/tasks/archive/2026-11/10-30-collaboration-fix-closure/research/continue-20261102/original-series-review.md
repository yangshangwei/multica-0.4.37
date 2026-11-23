# Original-series stop guard and remainder independent review

Verdict: **pass-with-notes**, zero blocking findings for the OEP-1/OEP-2 remediation. This independent review did not change framework/source/evidence/clearance and made no installed CLI, account, DB, server or Provider call.

## Findings disposition

- **OEP-1 closed:** new append-only `original-series-run` registers only E3 after trial 2/3, E4 baseline 1–3, E5 baseline 1–3 and E6 baseline 1–3/hop1–2. The allocation stays **14 calls (2+3+3+6)**, preserving old trial IDs, slots and directories. Both frozen HEADs, exact 4-file set/hashes, complete dirty scope and fixed executable bytes are checked before Go. Output reuse is refused.
- **OEP-2 closed:** after `env.run` durably finishes an E5 `finished/fail` record, the loop returns a typed `errContextBlockingBehavior` before the next trial. The entrypoint reports this distinct behavior error. Incomplete error propagation stays unchanged and occurs before that guard. Other scenarios' complete behavior failures retain their original trial behavior.
- **OEP-3 remains a root-owned live prerequisite:** independent review, complete deterministic check, authorization and exclusive window must be bound in execution clearance before a live call. This review does not grant that clearance.

## Evidence

1. Raw red JSONL matches its record/hash: 4 named runs, 3 pass, 1 fail, 0 skip. The old canary logic completes E5 trials 1, 2, 3 with `finished/fail` and returns 3 runs with nil error. This proves the regression fails against the prior loop.
2. Each new arm's raw green JSONL matches its record/hash: **32 named run/pass, 0 fail/skip**, exit 0. The canary test verifies one actual fake launch, exactly started/finished records for trial 1, retained failure/run identity and no next trial. Safe E5 still runs 3 trials; incomplete E5 stays runtime error and stops; non-E5 behavior failures still run3. Existing cursor, six-hop relay, partial evidence and source guards all pass.
3. Both new worktrees retain their original product HEADs and identical successor framework `ff2ac795d7ec3a6ca9664276200fe61f2526fe40c496ad7bc137b42099bd08c0`. The exact reconstructed diff matches `framework-stop.diff`. Scenarios/prompts, scoring file, 3-slot series and original live 45m/per-provider limits are unchanged. Changes are limited to the stop guard, distinct entry message, fake canary mode and four regression tests.
4. Both original worktrees' 3-file framework remains `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`. Old samples and source overlays are untouched.
5. Runner SHA-256 `da39ef5ec1f0c6ae1607f16dacf82cd247bb1d70804a1880112127bd258f5042` and prerun SHA-256 `2f77e27d357f2b5cf9b2eedc386b07981e4ed50e8252c69227a51b733ad50dd1` match the recorded 7 safe contracts. This reviewer freshly repeated 4 registered pure check-only cases and 2 invalid pure cases: all expected results. The seventh bare missing-clearance refusal was reviewed from its bound original result, not re-executed as a possible live command.
6. Formal E0's 4c833→835 product-scope difference is exactly the 3 experiment files. Formal smoke test bytes and original successful log match; fixed Claude bytes remain registered 2.1.316. No additional E0/health call is required to resume the unchanged original product/account/CLI; this is not new RK1 or current health proof.

## Notes for the coordinator

Root must preserve ordered scheduling and actual quota accounting. On the first incomplete result or E5 canary, stop every later scenario; a process exit code is not semantic classification. The runner permits one selected remainder and trusts root's current owner/authorization/wholecheck/window record. Execution clearance remains absent; only root may create it with the reviewed hashes and 0 blocking findings. Future records must declare successor framework provenance and retain every old invalid sample.

## Verification

Fresh read-only source/diff/fingerprint/JSONL reconciliation, Python AST check and six pure check-only executions: **pass**. New Go tests, lint/type-check and real CLI/Provider execution: **not run** under this review's bounded permission. The original 64 green named passes and 4 red executions were validated from raw bytes. Exact commands, hashes and check results are in [original-series-review.json](original-series-review.json).

Files written: this report and its JSON only. No framework, metadata, historical sample, source, clearance or commit changed.
