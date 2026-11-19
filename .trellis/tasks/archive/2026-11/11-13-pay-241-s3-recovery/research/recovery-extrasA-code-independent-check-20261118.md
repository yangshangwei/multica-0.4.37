# Extras-A R01 independent failure and code review

- Reviewer: `/root/s3_check_batch_block`; active task `11-13-pay-241-s3-recovery`.
- Decision: **NOT_ACCEPTED_CURRENT_EXTRAS_A_R01**. Three fixture blockers; no demonstrated product defect.
- Scope: frozen source and existing raw evidence only. The coordinator owns execution and integration; the next native implement owner owns fixture changes. No product/test source, matrix, index, database, API, browser, build, process or Git operation was changed or executed by this review.
- Frozen harness: `harness-extrasA-r01`, 243 declared files; manifest SHA256 `d192aff19ceeb8299b9903f937a9ebc08a5f05671f8b18e02ebbfb65ba4e1b76`. All 243 file bytes and modes independently match. API and production client source inspected is the actual `source-r01` allocation, rather than later ROOT work.

## Findings (fixed)

No source fixes were made in this read-only ownership lane. This report, its adjacent JSON and private reconciliation record preserve the exact failure and distinguish it from the two unexecuted fixture defects.

## Findings (not fixed)

### EXA-R01-01 — navigation is incorrectly treated as Mark read

File: `harness-extrasA-r01/e2e/fixtures/collaboration-recovery-extras.ts:275`.

The actual first Web case creates its mixed comment with HTTP 201, one deduplicated Agent target and one waiting command. The input contains two identical human markers and two identical Agent markers, but one `target_versions` entry. The current preview, two original notification recipients, notification row, mention label, link and comment anchor assertions all complete before the failure. The timeout is the `read_at` poll after clicking that link, not a trigger-preview or marker-count failure.

Actual `source-r01/packages/views/collaboration/notifications.tsx:59–64` closes the popover and navigates on the link. Its separate Mark read button calls the existing notification mutation. There is no automatic read mutation on navigation. A null read timestamp is therefore expected until that explicit action.

The coordinator-approved fixture correction is to identify the exact original notification ID from the current authorized page, click that row's real Mark read button, capture one HTTP 200 `POST /api/me/collaboration-notifications/{notificationId}/read`, and parse `CollaborationNotificationReceiptSchema` with the matching ID and non-null read timestamp. After the real refetch, find the row again and click its existing link/anchor. Keep the original recipient/event/command identity and replay assertions. Do not add product auto-read behavior or weaken the timestamp check.

Not fixed here because test source belongs to the new implement owner. The complete R01 failure remains immutable.

### EXA-R01-02 — source-only membership is incorrectly granted structure editing

File: `harness-extrasA-r01/e2e/fixtures/collaboration-recovery-extras.ts:770`; **NOT_RUN**.

The newly registered actor joins the source workspace as admin but is not an issue participant, creator or handler. `source-r01/server/internal/collaboration/access.go:91–101` grants a human source member read, comment and reorder, then returns before participant/manager editing permissions when `!Participant`. Thus `write_context=null` is correct, while `edit_source_structure=true` is a false fixture assumption.

The approved correction uses actual read/comment and membership authority: read/comment true and reorder true before removal; current private reads denied and membership removed afterward; nullable participant/account epochs remain genuine. It must not invent participant epochs or expand source-admin permissions. Not fixed here because this is implement-owned fixture source, and the original case has not executed.

### EXA-R01-03 — ordinary participating survivor is incorrectly treated as manager

Files: `harness-extrasA-r01/e2e/fixtures/collaboration-recovery-extras.ts:786` and `:791`; **NOT_RUN**.

The survivor joins the source workspace and is then invited as an ordinary issue participant. No transfer makes it creator or handler. `CapabilitiesFor` computes manager as `Creator || Handler`; consequently structure editing is false both before and after source removal. `source-r01/server/internal/service/collaboration_fields.go:74–75` also returns 403 from field-options for this actor before the attempted successful parse at line 791.

The approved correction keeps independent participation intact: stable actual participant epoch, read/comment true, source reorder true then false, source workspace readable before removal then 403/404, and field-options 403 in both phases. The existing draft/content/attachment survival and fresh authorized comment/capability checks remain. A source workspace read and an editing-only field-options catalog are separate endpoints. No product permission change or implicit manager transfer is authorized by this test repair. Not fixed here because fixture implementation is assigned elsewhere.

## Checked candidates that are not defects

- Password registration and `/api/me` parsing are valid in this allocation. `UserResponse` always emits `name:string` and `email:string`; a nullable stored email becomes the empty string through `u.Email.String`. `TestApiClient.registerPassword` returns the actual `data.user` object, including name, despite its narrower TypeScript return annotation. The actual run completes both `seedExtras` and outsider registration before opening sessions, which also rules out the proposed missing-name/null-email setup failure.
- The public share-link fixture matches the current response: the join returns `workspace_id` and a member with `id`, `user_id` and `role`. The one-use admin link is a real public join, not an email or membership fabrication.
- The notification link's `#comment-{id}` locator is current and actually succeeded in R01. Typed marker counts and IDs are also source/raw confirmed; none is the observed failure.

## Verification

- Lint: author targeted **PASS**, exit 0, `verification/extras-a-author-r03/eslint-result.json`; not independently rerun in this read-only lane.
- TypeCheck: author targeted **PASS**, exit 0, `verification/extras-a-author-r03/typecheck-result.json`; not independently rerun. Exact four entry-file hashes before/after match the frozen harness; logs and configuration hashes are reconciled. This is not a claim of a fresh whole-ROOT static check.
- Tests: existing coordinator R01 **FAIL**, exit 1, retry 0. Selected six tops: **0 PASS / 1 FAIL / 5 NOT_RUN**; planned eighteen subcases: **0 PASS / 1 FAIL / 17 NOT_RUN**. No reviewer tests, database/API/browser execution or second attempt occurred.
- Current runtime evidence: the first committed comment and original IDs/receipt are retained under the failed R01 database. The four unauthenticated natural response-stage probe requests do not accept real private late-body or revocation behavior. Upload object/ciphertext reconciliation and actual denial/stamp/response-terminal evidence remain required in a future complete run.
- Source reconciliation: all 243 frozen harness entries match; existing runner source-before/after reports no drift. Private support is `independent-check/recovery-extrasA-code-r01/reconciliation-r01.json` under `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`.

The coordinator reports the failed R01 API stopped and its dedicated database preserved. This small review does not replace the final independent connection/gate/catalog/process audit. Batch acceptance remains committed separately at `72c207f72`; TC-16, remaining Extras-A/B and full structured blocking/S3 acceptance stay open. No parent matrix or archive change is justified by this failed candidate.
