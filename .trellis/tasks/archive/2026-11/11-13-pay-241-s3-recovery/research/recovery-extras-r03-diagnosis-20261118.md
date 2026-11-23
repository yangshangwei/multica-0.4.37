# Extras-A R03 multipart observation diagnosis and R04 fixture repair

**Disposition: TEST_ONLY_REPAIR_STATIC_PASS_RUNTIME_PENDING.** R03 remains not accepted: 1 PASS / 1 FAIL / 4 NOT_RUN tops and 4 PASS / 1 FAIL / 13 NOT_RUN subcases, retry 0. This repair author ran no database, API, browser, Provider or TestMain body. The coordinator owns the fresh runtime round, database/object audit, matrix and commit.

## Proven failure and remaining uncertainty

The actual R03 failure is the second upload subcase, `same_bytes_unknown_retry`. Its stack points to the late `uploadFact(response.request())` call at frozen fixture line 544. Execution reached that call after the exact upload POST waiter, HTTP 201 assertion, strict attachment parsing and equality with the original attachment. The retry response body was not separately checkpointed before the failure. Request metadata records the same second-original idempotency key for the lost-ACK POST and its genuine retry; this metadata does not establish exact retry file bytes.

The old `uploadFact:376` error combines missing observable raw body and absent/nonmultipart Content-Type. The failed run does not identify which condition occurred, and no product upload defect is established. The first upload subcase, `committed_upload_GET_reload`, completed its GET/reload, selection, explicit comment delivery, protected plaintext read and original-comment replay assertions. Notification's three subcases completed. Unstarted cases stay NOT_RUN.

The frozen R03 fixture and nine original failure/source/report artifacts were copied before edits and rehashed unchanged after the repair. The retained failed allocation and all prior rounds remain coordinator-owned; this child performed no process or database action.

## Focused change

Only `e2e/fixtures/collaboration-recovery-extras.ts` changed. The existing `LostUpload` barrier becomes `HeldUpload` and gains a natural continuation branch. Both retry forms and `normalUpload` in revocation preparation reuse this one barrier. The latter had the same post-response multipart introspection and is within the exclusive fixture scope; its reuse was reported to the coordinator before editing.

At the intercepted exact POST, the fixture copies the actual `postDataBuffer()` before asynchronous header observation. It records body availability, actual MIME, byte count and multipart SHA, then strictly parses the original key, draft, intended issue and file name/type/size/SHA. Authentication values and file bytes are not written into the evidence ledger. Missing body and missing multipart Content-Type now have separate diagnostic errors.

The genuine request stays held until its original tuple/file fingerprint has been validated. Natural retry and ordinary upload call unchanged `route.continue()` exactly once. A waiter installed before continuation matches that exact Playwright Request object, joins the continuation and response, parses the actual attachment and checkpoints the actual status/body. HTTP 201, canonical attachment equality, original tuple/file equality and POST counts remain required. The deliberate first committed ACK loss still uses real upstream fetch followed by abort.

The wrapper releases/joins the held route and UI gesture in `finally`; continuation and response waiters are joined with `Promise.allSettled`. No key substitution, storage clearing, forced control, fabricated response, second artificial POST, timeout increase, inventory change or product source change was introduced.

## Verification and evidence

No-database strict TypeScript on the two E2E roots and their 639-file source graph: exit 0. Explicit ESLint on exactly the extras spec and fixture with `--no-ignore`: exit 0. Both raw stdout/stderr logs are retained and empty. Source and recorded tool/config hashes match before/after; the final source graph still matches the successful checks.

The first static tool manifest omitted the imported `packages/eslint-config/react.js` and `base.js` exports. A separate same-candidate two-file ESLint pass seals those imports, package mapping, node/tool and source inputs before/after: exit 0, 15 inputs, zero drift. This adds configuration provenance without changing the candidate or running a business body.

The new candidate fixture SHA-256 is `c86229ef94f3444118affe0a9d0138a4586e1fc73d166ff8e41800bbd984ae1f`. The extras spec, shared recovery helper and base spec keep their previous hashes. The denominator is still **6 selected tops / 18 selected subcases / retry 0**.

Private complete evidence: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/verification/extras-a-fix-r04`. The adjacent JSON binds the repair plan, exact before/after source copies, diff, original R03 failure hashes, compiler/ESLint argv/config/tool inputs and raw logs. Completion seal: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/verification/extras-a-fix-r04/repair-completion-r04.json`.

## Coordinator follow-up

Freeze this new fixture into a fresh R04 harness and run all six tops/eighteen subcases with the current API, production Web and matching Desktop. If multipart bytes remain unavailable at interception, preserve the new availability/MIME diagnostic and obtain a justified request-body observation; do not invent bytes or weaken checks. The full runtime result, ciphertext/object/draft/request audit, independent acceptance, blocking increment and complete S3 acceptance remain open. No matrix or archival change follows from this static repair.
