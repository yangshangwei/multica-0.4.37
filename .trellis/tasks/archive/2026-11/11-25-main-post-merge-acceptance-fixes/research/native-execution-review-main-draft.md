# Mac runtime draft review

Static review is complete before package binding. Two small cleanup defects remain; neither is a platform blocker.

## Findings (fixed)

No reviewer edits to runners. Linux L1/L2/L3 are already closed and exact request f0462f15a5c09a598a6c2797d4d835316f0ab5545110fc25c70cd029243c3407 is accepted in native-execution-review-linux01-v2.json.

## Findings (not fixed)

- M1 — run_mac.py:78,89: log_close_errors are recorded but ignored by the success predicate. Make them fail,as already done for Linux.
- M2 — run_mac.py:79: the terminal path audit omits the Host poller. collaboration-host-runtime.ts:355 launches native/bin/daemon-test with only its poller selector in argv; private HOME is in env and is not searched. Use a hash-checked per-run daemon copy under state/bin,or retain exact child PID/identity for independent verification. Do not kill other lanes through a broad native/bin prefix.

Hooke owns these mechanical fixes; reviewer did not edit sources/runners.

## Reviewed safe boundary

Original480-second complete packaged Mac Main/Host Stop,restart,Resume,history,authority and proof assertions remain. Actual file renderer/password login/production IPC are used. Task HOME/TMP,26provider overrides,discovery-only fake and isolated synthetic Host provider prevent personal account/model execution. API PID/source and frontend readiness checks remain. Protocol restoration retains exact captured native baseline and enumerated owned app paths; parent releases the protocol lane before signing/runtime. Whole-task initial protocol state remains unproven.

## Binding follow-up

Replace draft package02 reference with verified signed output and bind actual binaries/tools/configuration before execution. This static review did not wait for those hashes or count them as code defects. Older source01 runs remain scoped to that candidate; latest C1/comment integration needs fresh final binding. Mac signing is already independently cleared in native-execution-review-mac-sign03.json.

## Verification

Python AST and draft Mac/helper hashes passed. Lint,typecheck and tests were not run under the read-only restriction. No services,Docker/protocol mutations,signing or recursive agents ran.
