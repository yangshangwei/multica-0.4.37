# R30 test-only read diagnostics independent check

**ACCEPTED_R30_TEST_ONLY_READ_DIAGNOSTICS_NORMAL_RACE_READY.** The private observer path extension, corrected offline fixture, fresh normal/race eleven-name tests and matching harness build are accepted. Product changes are0. This accepts diagnostic readiness only; it does not repair or attribute the original QA503, admit/start the next API or accept full S2.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`. Current source: `/Volumes/artisan/.multica-acceptance/pay241-r30-qa-read-20261118-pohc3l9t/source-green-r01`. Only this new Markdown/JSON pair was writable. The reviewer used filesystem/hash/retained parsing only: no Go/AST/TestMain/DB/API/browser/build/process/signal/Git execution, source/configuration/old-report edit or agent spawn. ROOT owns all execution and progress.

## Findings (fixed)

No reviewer source fix was made. The implementation's one fixture correction was independently verified.

- File: `server/cmd/server/pay241_read_diagnostics_scope_test.go`.
- Issue: the first actual normal run's three forwarding leaves stopped at the existing capture-parent0700 assertion before any HTTP callback. Installed Go1.27 returns a numbered TempDir leaf through os.Mkdir(...,0o777); its mode follows umask and is not a guaranteed0700 capture root. Current outer temporary roots have been cleaned, so this review does not pretend to observe the removed failed leaf.
- Fix: in a new GREEN copy, one line `parent := t.TempDir()` becomes a joined `capture` child and checked `os.Mkdir(parent,0700)` before the exclusive0600 log open. First96 and final150 source lines remain identical. Original0700 assertion, context/principal/body/header/cap tests and all eleven names are unchanged. The patch exactly reconstructs current249-line source SHA `1fa8a5ec3f709b7b98b4d53ec5a31bff3cc56b7583d1fe396b84bf6c45431c1e`,0644. Observer `326e53f9…` is byte-identical between RED and GREEN.

## Findings (not fixed)

No unresolved issue remains inside this test-observer review. Retained limits:

- Original normal-r01 SHA `59180ad8a2ef1f800420db50dac350a18843bf8e1201394d71354c262636aba6` stays **7PASS/4FAIL/0SKIP**; all six path leaves pass and all three response leaves fail during directory setup. It is not a business/observer RED or proof of the QA503 cause. Original RED source/build/results and unused original race plan are retained.
- Initial242-line malformed draft `18401cd2…` and gofmt exit2 remain embedded with exact output in the sealed implementation report. The missing chunk-loop brace was fixed before Go execution. The separate static regex false positive on go-chi is retained and required no source edit. Neither preparation failure is silently promoted to product evidence.
- The original R29 browser run remains1PASS/1FAIL/2NOT_RUN,retry0; its QA root/candidate503 body/native query/SQLSTATE/physical holder remain unproved. Offline callbacks do not authenticate or execute PostgreSQL. Real future query/response correlation still needs the bounded main runtime capture. Eligible real writers are not automatically classified as read/read bugs. Original R28 successful completion/history and3PASS/1FAIL/0NOT_RUN are preserved; no old write is replayed.
- Runtime preparation/startup, full A01–A12/A12/S2, parent24 and S3–S5 remain outside this verdict. Parent stays0PASS/9PARTIAL/15NOT_RUN. Retained package TestMain changes config-invalidation21→23 in each fresh DB; no all-database-zero-write claim is made.

## Verification

### Path/default/source boundary

Only the existing test-only `pay241DiagnosticPath` changes against accepted R29 source. Exact uppercase GET and canonical nonzero lowercase issue UUIDs remain required. Exact root len5 and R4 `stage/code-candidates/{canonical UUID}` len8 are added; original M1 `delivery-evidence/candidates` len7 is retained. Other methods/routes/segments and malformed/noncanonical/zero IDs refuse. The actual UUID helper still requires parser success, validity, nonzero bytes and canonical round-trip equality.

The seven other observer functions and the complete file remainder are byte-identical, remainder SHA `98c962086d6b6c31d12d3f94a0b186f7d72cfe415962fddafcddb6e9323094f0`. Nil/default pooling, harness-only env, absolute path/0700parent/exclusive0600sink,4096-record/1MiB-body limits, original query args/filters/source frames/PID/SQLSTATE, HTTP result/status/headers/body and inherited context are unchanged. No new writer capture, raw secret/argument collection, pool/lock/isolation/default/role/epoch/deadline change, HTTP write, dependency or new product exists.

Full initial source audit found5053 paths with exactly one existing test change plus one new test,5051 original files unchanged/modes preserved. The final correction uses the same5053 paths and changes only the new test's parent block, with5052 others exactly matching its frozen RED. Final manifest matches `43249cefbf9209eef3002c0d57a4d5b575e57e34278aa0f899032926010dfab1` (lexically path-sorted compact sorted-key JSON, no newline). The existing observer is236 lines/0600/SHA `326e53f91153e08762929c85f1b83246f35c5f6b649a0279e527402ef48a1dd6`; protected products/SQL/generated/DTO/other tests remain unchanged. No original source/default re-review was needed after the four-line fixture correction.

### Actual tests and meaningful assertions

| Top-level | Exact leaves |
| --- | --- |
| `TestPAY241ReadDiagnosticExactGETScope` | `m1_candidate`, `root`, `stage_code_candidate`, `other_method`, `invalid_identity`, `other_route` |
| `TestPAY241ReadDiagnosticResponsePreservation` | `success_passthrough`, `auth_refusal_passthrough`, `stage_refusal_passthrough` |

Fresh normal-r02 result `e344355a56e43bdb06f37b22893197f9e99468b11049b2055a852536bc23de46` and race-r02 `be7ac4457e0905c5feabe6f1f29cd626339babc9375cbae68ac7587bf103b802` each have **2top/9leaf/11named,11PASS/0FAIL/0SKIP**. Source-derived literal inventory, both complete raw run/terminal sets and permits match exactly. There are no missing/extra/duplicate/pending/parse/shape errors, unfinished output fragments, input drift or race warning.

Path tests exercise exact admission and negatives. Response tests compare direct/nil/unadmitted/admitted callbacks, preserve request pointer where appropriate, cancellation Done channel, inherited request ID/principal/password-session, URL/query/request body/downstream headers, status and complete response bytes. Stage-refusal writes an actual body larger than1MiB in two chunks: complete forwarding, captured bounded prefix, full-byte hash/size and incomplete flag remain correct. Starting at the last available record verifies4096th normal event, limit marker and ignored tail while every callback still runs. HTTP-only events explicitly require absent native PID/query/SQLSTATE/principal metadata rather than fabricating native evidence.

The unchanged query tracer may capture real request-bound query failures when real PostgreSQL executes; these offline tests prove forwarding/limits, not a real error witness. Static syntax/caller checks and actual package compile establish parse/types; no separate Go AST execution is claimed.

### Package/configuration/build evidence

r02 allocation `0adca7a0313bc8d71bfa070bd4d8b2e37e291bfc1af15353360752a32c1a0069` owns two fresh ./cmd/server databases: normal `multica_pay241_20261117_s2_qadiagrouter_normal_r30b`, race `...race_r30b`. Profiles/clone-before ledger/index/table counts equal actual suite-before; both afters have other_connections0 and unchanged ledger/index definitions. Main serial one-command/count1/90s permits, decoded profile/source cwd and raw references match; each5056 required-file set is bound to current5053 source plus its support receipts. Only package TestMain config-invalidation21→23 is retained in both before/after records.

Matched r02 build result SHA `16ad246838e75362dce36841bf183e376e97f3379b5a93b55154ab4342d9b5aa` records gofmt-l, cmd/server vet and go test-c exit0/empty logs. Compile environment SHA `e6ac5786…` contains no DATABASE_URL; TestMain/test bodies did not execute during this build. Before/after5053 source snapshots match hash `153d396c7b2d4a9a94da9a2ecbf10cedc8305784be25c3560fea5717b7002087`. Actual0755 stage-router.test is118,587,698 bytes, SHA `e45730ae3d5263e6bf833b544abfd3eb7f9fbc53280420443ee12424b5d0ee05`; it binds the corrected test source. The four other accepted R29 programs are outside this test-only change and remain available for later source/runtime binding.

- **Lint/Vet: PASS** for the retained actual two-file format and ./cmd/server vet.
- **TypeCheck: PASS** through retained matched compile-only and actual fresh package runs.
- **Tests: PASS** normal/race each eleven names; original setup7PASS/4FAIL retained separately.
- **Spec sync:** existing test-isolation/capture conventions apply; private route and fixture details are recorded here and in their owner reports. No production/public interface/spec file was changed by this reviewer; full116/A12 reruns are not implied.

Only the two authorized reports were created and sealed0444. The JSON binds this Markdown SHA256 and full preserved/new source, commands, profiles, raw results and failures. No pending work remains within this diagnostic-readiness review.
