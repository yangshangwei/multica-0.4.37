# R33 policy-wait RED test boundary independent check

**ACCEPTED_R33_POLICY_WAIT_RED_TEST_BOUNDARY_ONLY_READY.** No blocking issue remains in the frozen new handler test. Exact source boundary and ROOT's saved gofmt/vet/compile-only checks pass. This accepts authored regression readiness only; genuine RED execution and its independent acceptance must precede the future three-file M1 product repair.

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; reviewer `/root/r32_after_check`. Source: `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-red-r01`. Only this new report pair was written/sealed0444. Saved source/manifest/static evidence review only; no reviewer source/config change, Go/build/test/vet/lint/type-check, DB/SQL/Redis/API/browser/process/Git execution or agent spawn. Peers' files and old successful completions are preserved.

## Findings (fixed)

- File: `server/internal/handler/delivery_policy_project_wait_test.go:346`.
- Issue: The provisional original-NOWAIT path could complete before the outer select, leaving both buffered response and query-entry channels ready. Selecting the response could misclassify a real entry as a missing barrier.
- Fix: The author corrected the new test before freeze: the response branch drains the already-buffered query entry and retains that response. The final18812-byte/458-line source includes this correction. No reviewer source write occurred.

## Findings (not fixed)

No unresolved defect was found within this test boundary. Actual RED results remain pending and are not fabricated from source/static checks. The short-reader target must produce its genuine old-source failure; competing-writer stale-CAS may also fail the old immediate-busy path and must be retained as its own RED outcome. The R32 physical holder remains unknown, and this deterministic fixture does not identify it. No product/whole-S2 acceptance is granted.

## Verification

### Frozen source boundary

Boundary `096f3069487cebb570a4d91503f040e015a7650f367255cc894f3be5a3ea7000` and author receipt `b83ebbd139656af613fe561540ab6b068583c1c15a6d2a8d981beec11213fe1e` bind one new file:

| Private path | SHA256 | Bytes/lines/mode |
| --- | --- | --- |
| `server/internal/handler/delivery_policy_project_wait_test.go` |`aed914a8acd64465219a8fcb07541af3caed74e899c54796b873a6deaa8340a0`|18812/458/0644 |

Independent current hashes/modes matched all5056 regular files. Exactly this test is added; all5055 protected originals remain byte/mode-identical, no removed/nonregular path or product/query/generated/config/existing-test change. Final path-sorted manifest is `2fe7a44b273bff9627cc00b8cbe21b4b2db281c1c567013d3a8cfa2ce0d7b5b8`; protected manifest is `26a4ad38ecf668f5fd665c05aa860dee948cfbfdd0af8b97035afadfc1c0f0e3`. Complete records remain in the author JSON rather than duplicated here.

Original delivery-policy handler/SQL/generated code, stage-policy caller,250ms transaction preparation and reused fixture/helper bytes match accepted R32. The original query still uses FOR UPDATE NOWAIT; no new waiting query or product caller is installed. This test recognizes only original `LockDeliveryPolicyProject` and proposed `LockDeliveryPolicyProjectWait`, with the same project/workspace argument expectations, so identical test source can later exercise both versions.

### Real operations, boundaries and evidence

`newDeliveryPolicyFixture(t,false)` supplies real fixture-owned project/issue/artifact state and a current human JWT. Initial actual GET confirms virtual revision0/off. First actual GET uses the original handler's `LockProjectForAssociationNowait` SHARE fence; the observer holds only after its real `GetDeliveryPolicy` Scan, including original ErrNoRows. Reused `progressObservedRow.Scan` forwards original destinations/error before the callback and returns the unchanged error; service then produces virtual0/off. No arbitrary raw read lock substitutes for the GET.

Each handler copy has its own real transaction wrapper/probe. PID is cached at Begin before pool completion releases Conn. Exec/QueryRow and generated Scan results/errors are forwarded unchanged. The second real PUT supplies the same current actor/JWT/project/workspace and a distinct nonzero PID. Query-entry notification alone is insufficient: before release the test requires either its actual native55P03 or an actual `pg_blocking_pids` witness for the identified first PID. The probe records native SQL hash/name/args/error/frames; no SQL error, actor or PID is manufactured.

| Exact leaf | Authored contract |
| --- | --- |
| `TestDeliveryPolicyProjectWaitShortReader/short_reader_release` |Real GET retained; real PUT attempt witnessed; release at50ms after project-query entry and assert inside250ms; original GET observation stays equal; target200/exactly one requested revision1 append. |
| `TestDeliveryPolicyProjectWaitBoundaries/long_reader` |Same real GET held until at least350ms after entry; real PUT observed within one-second test guard, native55P03/409busy/no append; release and finish GET. |
| `TestDeliveryPolicyProjectWaitBoundaries/competing_writer` |First real PUT holds after actual CreateDeliveryPolicy Scan with exclusive project fence/uncommitted append; second same-revision PUT attempted before50ms release; winner200, loser exact409stale-CAS with zero second append and one retained winner. |
| `TestDeliveryPolicyProjectWaitBoundaries/member_denied` |Real new workspace member may GET; actual PUT403management-denied before project-writer query/append; original owner unchanged. |

Static literal inventory is2top/4leaf/6named and matches the admitted selector. The original250ms SET LOCAL succeeds and is observed; no product timeout change is made. The shared five-second fixture bound and one-second long-response guard are orchestration bounds, not claims that whole-request latency is bounded by250ms. Release channels are idempotently closed, request cancellations invoked and every started goroutine joined on failures. Buffered completed RED responses are preserved; no Want/assertion is moved into a handler goroutine.

Snapshots begin after genuine fixture/member setup, preserving non-policy issue/project/status/evaluation/artifact/approval/task/comment and authority/stage/authorization/terminal facts. Only the permitted winner policy append is excluded from protected equality; exact count/revision/mode/current-human binding is separately asserted. Existing `evidence_issue_id`/terminal fields match schema. Account snapshot payloads remain in memory; logged effects use hashes/equality, with safe query/error identity. This is fixture-owned effect coverage, not whole-database-zero-change proof.

### Saved static evidence and execution limits

ROOT static result `ef3debd5aaf9202666b9fb7ff762fff7f0145667035a8449e32d021943a59a0c` records gofmt/vet/handler compile-only exits0; all six output logs are empty and hash-verified. Source-before/after5056 manifests match the accepted final records. Explicit compile environment has no DATABASE_URL; TestMain/DB were not used.

Fresh allocation `8243a9ba1a2361e7762877fc6100fc42f75552c135d4c9f86af2b7989a93e5a4` identifies distinct RED/GREEN/race handler profiles and saved clone audits. They remain preparation, not tests run by this reviewer. Future three-file product scope is conditional on actual accepted RED; existing stage NOWAIT query/caller remain protected. The later GREEN original-control union is separate and does not change this six-name RED denominator.

- **Lint:** PASS, retained ROOT gofmt/vet exits/empty logs.
- **TypeCheck:** PASS, retained handler compile-only; no reviewer execution.
- **Tests:** NOT_RUN within this boundary acceptance. Actual RED is ROOT's next gate.
- **Source scope:** PASS, one new test5056/5055protected, product0.
- **Product/runtime/fullS2:** NOT_ACCEPTED_HERE.

The current R32 project-query55P03→HTTP409 witness is preserved in after-check `ecdd27b0…`; its unknown holder and R31's separate historical attribution remain unknown. Original3PASS/1FAIL/0NOT_RUN,retry0 and successful completed keys are unreplayed. Full A01–A12/parent24/S3–S5 remain open; parent0PASS/9PARTIAL/15NOT_RUN. Captured UTC `2026-11-18T00:18:50.271251Z`. No pending work remains within this frozen RED-test boundary review.
