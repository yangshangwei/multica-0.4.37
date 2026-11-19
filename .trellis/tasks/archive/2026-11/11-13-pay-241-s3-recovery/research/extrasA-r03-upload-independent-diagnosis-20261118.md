# Extras-A R03 upload observation failure diagnosis

Decision: **NOT_ACCEPTED_CURRENT_EXTRAS_A_R03**. No demonstrated product defect; one observed fixture introspection failure. Reviewer `/root/s3_check_batch_block` performed source/raw reads only, with no API, database, browser, source or matrix action.

## Findings (fixed)

No source fix. The original R03 candidate and failure remain unchanged.

## Findings (not fixed)

At `harness-extrasA-r03/e2e/fixtures/collaboration-recovery-extras.ts:544`, the second upload subcase parses multipart facts from `response.request()` after the natural retry completed. `uploadFact:376` throws when either the request has no observable raw body or the reported content type is not multipart. The current error combines those conditions, so the raw trace does not establish which was unavailable.

This is the **second** subcase, `same_bytes_unknown_retry`. The first, `committed_upload_GET_reload`, already passes its complete GET/reload, attachment selection, explicit comment delivery, protected plaintext read and original-comment replay fixture assertions. The second reaches HTTP 201, strict attachment parsing and equality with the original attachment before introspection fails.

`clickResponse:172` matches the exact `POST /api/collaboration/attachments/drafts`; it cannot select a comment POST. Native writes retain the second original upload key twice, once for its lost-ACK original and once for the real retry. No reviewer claims exact retry multipart bytes from that metadata alone.

Recommend the owning implement agent capture the retry's actual multipart facts at an exact POST request route, preserve the raw observation, then use `route.continue()` without replacement. Correlate the original key/draft/file fingerprint with the actual natural HTTP 201 response. Do not fabricate missing bytes, fetch a second POST, fulfill a response, replace the original key, or weaken the fingerprint checks. Source changes belong to the new Extras owner, outside this review's client-code correction permit.

## Verification

Existing coordinator R03 selected tops: **1 PASS / 1 FAIL / 4 NOT_RUN**, retry 0. Planned eighteen subcases: **4 PASS / 1 FAIL / 13 NOT_RUN**: three notification subcases and the first upload pass; the second upload fails; the other subcases remain unexecuted. This does not accept the incomplete upload top or a full Extras-A increment. Main's object/ciphertext/database/resource audit remains separate.

Product lint/typecheck/tests: NOT_RUN by this diagnosis. No second attempt was made. The adjacent JSON binds the exact source and raw report/case evidence. Full blocking, Extras-A/B and S3 remain open; no matrix or archive change is justified here.
