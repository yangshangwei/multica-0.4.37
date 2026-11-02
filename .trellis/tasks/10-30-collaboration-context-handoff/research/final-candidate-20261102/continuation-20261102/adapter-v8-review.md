# Independent frozen v8 adapter review

Verdict: **changes-required**. `blocking_findings_open: 1`. Reviewer: `/root/adapter_v8_review` (`trellis-check`). This applies to frozen Go `0d8d62797c887440d23b129f2e0d86eab4bbea88dcffc463de6478ea1bc65330` and pnpm `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48`. It creates no launcher clearance.

The direct-test package/context and scanner witnesses are corrected. A valid global `--` prefix still bypasses test dispatch before those guards. The shared execution owner is handling that bounded integration condition on its separate candidate; this report does not certify that candidate.

## Findings (fixed)

No reviewer source edits. Only the two assigned reports and new ignored independent evidence were written.

The guarded paths close the original regressions:

- **AV5-1:** canonical `allocation.package`, same cwd/`-C`/default `.` resolution, and exact single-package clone binding. Cross-package, wildcard, multiple selectors and malformed package identities refuse.
- **AV5-2:** ordinary mode1 requires nonblank explicit Go, SQL and evidence context before discovery/SQL; mode0 requires explicit Go/evidence and its allocation record.
- **AV7-1 scanner:** the actual Go1.27.0 table matches **70 value flags / 26 Boolean flags / 29 registered test aliases**, including coverage aliases and `buildvcs`/`v` Boolean exceptions. Double-dash spelling, final real true/false `c`, package/flag/positional tails, `-args`/`--args`/`--`, and unknown-option refusal pass. Known flag values resembling `-c` do not grant compilation.
- Mode0 ordinary tests reject actual `modfile`/`overlay` options before listing/execution, while source-shaped regex values and opaque binary arguments remain raw. Genuine compile-only calls remain exempt. Ordinary tests remove inherited API pool variables; recorded non-test forwarding preserves them. The normal recorded allocator and unchanged pnpm paths retain their behavior.

These guarded-path closures do not override AV8-1.

## Findings (not fixed)

### AV8-1 — high, blocking, open integration condition

File: [frozen Go adapter](/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v8-knowledge-advance-20261102/check-bin/go:25); raw forwarding at line42. The loop stops at a global `--` without recognizing the later `test` command, then enters the non-test `execv` branch. This precedes compile classification, ordinary context/package checks and pool-variable removal.

| Actual argv after `go` | Context | Exact v8 observation |
| --- | --- | --- |
| `-- test example/first` | No private test context or explicit Go | Fallback Go exec recorded instead of exit2 |
| `-- test -c=false example/second` | Mode0 clone allocated to `example/first` | Other package exec recorded with zero list/binding calls |
| `-C=first -- test` | No private context; pool10/1 inherited | Raw fallback forwarding, with pool overrides retained |

Both named global-separator negative tests fail, with zero errors. The standalone observer records three unsafe forwards. All Go/SQL execution boundaries were intercepted; no real tests or database writes were performed. Cached, network-denied `go -- list ./first` and `go -C <owned-first> -- version` both exit0, confirming this is a valid Go prefix rather than an invalid-command assumption.

Not fixed: v8 is frozen and this reviewer has read-only source ownership. The sole owner `thread01a19c0d-cce1-73e3-99ee-2c3c9e9239ba` reported a small integrated prefix correction in progress. Handle the global terminator during command identification, preserve raw argv, and recheck missing context, mode0 cross-package/final-false refusal and genuine compile positives on the **new exact integrated hash**. That owner’s independent zero-blocker review is required before launcher clearance.

Evidence: `independent-negatives.{json,log}`, `global-dispatch-witness.{json,log}` and `metadata-oracle.json` in the private review directory. This finding was reported promptly to the coordinator.

## Verification

Review dimensions ran sequentially: task/spec behavior with the supplied replay, then code quality/current-source audit and independently constructed negatives.

| Check | Actual result | Exit |
| --- | --- | --- |
| Supplied binding/scanner | **38 methods pass**, 658 observations, 0 failures/errors | 0 |
| Inherited fake-executable suite | **14 methods pass**, 26 recorded wrapper invocations, 0 failures/errors | 0 |
| Independent negative suite | **12 methods: 10 pass / 2 fail**, 187 observations, 0 errors; both failures are AV8-1 | 1 |
| Unchanged pnpm proof, reused | **5 methods pass**, exact wrapper hash and preserved raw proof; not rerun | recorded0 |
| Actual cached Go metadata | **12 calls pass**: 3 version, 7 list, 2 help; four owned fixture sources unchanged | 0 each |

Fresh named-method total is **64 run / 62 pass / 2 fail / 0 errors**. Metadata, syntax checks, the observational command and reused pnpm are not added to that total. The observational command exits0 while recording unsafe forwarding; its exit is not a safety-gate pass. The independent failed run was read and retained without rerunning it.

- **Lint:** no configured Python lint entry point for these private adapters; repository lint not run. In-memory compilation passes for five frozen and six reviewer Python sources. Both bounded `git diff --no-index --check` probes have empty diagnostics; each returns1 for an ordinary diff.
- **TypeCheck:** not applicable to this Python-only review. No product TS/Go changes or repository typecheck pass claimed.
- **Tests:** fail for exact-v8 clearance because AV8-1 remains open. The prior build-value/double-dash/source-option witnesses and independently chosen unknown-arity paths in absent/mode0/mode1 contexts pass.

All fake-suite temporary files use the reviewer’s private directory. Supplied harness copies are byte-identical; they run the original frozen wrapper. Network and real-Go execution are denied for fake subprocess suites. Metadata uses explicit offline/private HOME/cache settings and performs no Go test/TestMain. The inherited pnpm replay uses only a test-created guard/executable; reused pnpm5 covers only its unchanged bytes, not the older Go pin in that command record.

## Source and evidence binding

| Artifact | SHA-256 |
| --- | --- |
| Frozen v8 Go, unchanged before/after | `0d8d62797c887440d23b129f2e0d86eab4bbea88dcffc463de6478ea1bc65330` |
| Unchanged pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| Fresh binding raw | `83468a84bca06183edc8beb05eaade926a50dc2e6b564cd02658fb1477a3eb41` |
| Fresh inherited raw | `4b91ce11db52ae35a9268b08b5e9ac28aaa1d051f40d335b0fc5989df7ab5cc2` |
| Independent failure raw | `99151cd64f49c6949bd0e072fa1b3ab6196c8beeb1b98e4275c72f0ab01bed83` |
| Metadata summary raw | `e3383fbef668518d399b342fbca1368c716572a60191102910c975992aada6bd` |
| Static audit raw | `5a4e58fdb703f914e933cf7f15bed6cfa5d9af4d3933323cbb383ad7a6e1d6c3` |
| Cached Go executable | `71c4991041d8e44975c882e4f72005719c958013d3340dc665a3808b72ddf702` |

All ten final snapshots match. Eight cached-source excerpt/full-file checks and independently extracted registrations agree. Six incoming v7 sources, 59 recorded v7 evidence files, all 47 v8 artifact-manifest entries and 163 before/after preserved inputs match. The original v5/v7 reports and historical failures remain unchanged. Retained v7 red evidence is **8 methods / 246 failures / 0 errors / exit1**, read and hash-checked without rerunning.

The 89-line v7-to-v8 Go patch (`c3d61fa1785a61b4d78daebe336c4f8e99ca4b77f1ff10a5efe3b2c5ffa7baf5`) and 118-line preserved incoming-v6-to-v8 patch (`a1d3f34093940ba07ac6f4808ca76ac398bff5f48a5bdcf6f8c1b0ccea4210c8`) match their manifests. Live v6 is outside this review.

[Machine report](/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff/research/final-candidate-20261102/continuation-20261102/adapter-v8-review.json) includes actual argv, environments, exits, named counts, raw/result/source hashes and preserved-input checks. Private output: `.omx/private/collaboration-resume-20261101/adapter-v8-independent-review-20261102/`.

## Supported scope and remaining limits

The flag registry is pinned to Go1.27.0 and must be rechecked on toolchain/flag changes. Mode0 source overrides are deliberately unsupported for ordinary execution. Mode1 retains its existing splitter for recorded repository-check arguments; this review does not certify arbitrary Go CLI compatibility. No new Go parser, dependency or product abstraction was introduced.

No actual Go tests/TestMain, SQL/DB/migrations, services, browsers, installed Agent CLI, Provider/account calls, downloads, commits or product/adapter/launcher/spec/status edits occurred. Shared final4/sourceenv/clearance, Redis16422 and renderer19845 were untouched. Existing specs already require context before ordinary execution; no spec edit was made.

This report neither starts nor clears a complete `make check`. The coordinator must review and bind the owner’s corrected integrated source/launcher/env bytes. Product, knowledge K1–K3, native/account and release acceptance remain separate.
