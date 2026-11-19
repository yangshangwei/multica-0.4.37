# S3 current-client field R02 independent check — 2026-11-18

**NOT_ACCEPTED_R02_CURRENT_CLIENT_FIELD_RUN.** The fresh complete four-case attempt ended **2 PASS / 1 FAIL / 1 NOT_RUN**, with eight planned ordering subcases **4 PASS / 1 FAIL / 3 NOT_RUN**, retry0. The two Web cases are retained partial evidence; no passing case from R01 is added to this round. Full current-client contention and TC-16 remain incomplete.

Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`; reviewer `/root/s3_clients_check`. This reviewer read saved R02 raw reports/ledgers, exact source/configuration manifests, one saved native image and main-owned shutdown snapshots. It wrote only this report pair and private `independent-check/client-runtime-r02` supports. It ran no DB/API/browser/TestMain/build/process or Git command and did not edit product, fixture, old evidence, matrix or peer WIP.

## Findings (fixed)

No source correction was made by this reviewer. R05's previously accepted harness cleanup repairs successfully preserved the real R02 primary failure and closed all three native sessions with recorded exit0. The old R01 outer-unroute failure remains unchanged and its original primary cause remains unknown.

## Findings (not fixed)

### S3C-R02-01 — settle the original losing UI request before new intent

The current scenario proves the losing original operation through a direct authenticated GET, then attempts a fresh Transfer on that same UI without settling its own retained original request. This is a harness journey gap under the existing recovery contract, not a demonstrated permission failure or new product defect.

The exact Desktop priority-first ledger identifies issue `01a1edba-a93c-75cc-9d86-8b201016772c`, external handler `edb48e37-b63a-4d4c-8d56-620a33571da7` and original losing Transfer key **`64908876-b0ee-44cb-81b8-70ca3a93d706`**. Both real UI requests observed collaboration revision4 before forwarding. Priority committed200/revision5; Transfer returned409 `stale_revision`/revision5. The losing own GET returned HTTP200 carrying the exact stored status409. Both original GET-to-POST comparisons were reached.

The winner's UI original-result recovery, same-key200 and changed-priority-key409 were reached. After the losing Transfer page reloaded, the primary failure occurred at `fresh_intent_open_form`: its **More actions** button remained disabled and the real click timed out after30 seconds. The case duration was38.538 seconds, not the360-second test deadline.

The saved `desktop-priority-transfer-priority-failure-1.png` directly shows a readable task/current external handler, the message **“The result is not confirmed. Keep this request and check its original outcome.”**, an enabled **Check original result** button and disabled workflow controls. All three pages were still open before owned teardown; native exits are0, all cleanup results fulfilled, and page/cleanup error lists are empty. This is the retained original request blocking a replacement, not evidence that the current actor lost task access.

The inspected product path explains the exact required action:

- `packages/views/collaboration/workflow.tsx:81` locks controls while `drafts.actions.length > 0`; line105 exposes recovery when the mutation is no longer pending and a saved action remains.
- `packages/core/collaboration/pending.ts` reads the scope-owned saved action; the test's direct `platformRequest` GET never calls `executeCollaborationAction` or removes that saved draft.
- `packages/core/collaboration/operation.ts:29` validates the current original result, retires that exact draft through `removeCollaborationDraft`, and notifies/refetches the owned scope. The real Check original result control is wired to this owner.
- For a recovered refusal, workflow `onSuccess` restores the original Transfer form and original input. It is an editable refusal, not an automatic resubmission. Dismiss that restored form with its real Close button, then prepare the new intent with current observations and a new key.

The next bounded harness revision must exercise the losing original **Check original result** when pending, observe the same GET/key/canonical refusal and zero business POST, verify that exact request is retired, dismiss the restored form, and only then submit a fresh authorized intent. Preserve both old receipts and all failure history. Do not force a disabled click, increase deadlines, clear/reset localStorage, insert a version or change product authority. No such new revision or runtime acceptance is claimed here.

### Required scope still open

Desktop transfer-first and both ordinary-child ordering paths remain unexecuted in R02 after max-failures1. Batch read/reconnect, notification/upload/revocation/blocking and full R01–R09 acceptance are separate required work. Parent TC-16 remains PARTIAL; this report does not alter matrix counts or archive S3.

## Exact run and source reconciliation

| R02 top-level case | Actual result | Duration | Ordering evidence |
| --- | --- | --- | --- |
| Web priority/handler | PASS |11.528s|priority-first and transfer-first PASS |
| Web ordinary child API CAS/UI compatibility | PASS |9.820s|priority-first and assignee-first PASS |
| Desktop priority/handler | FAIL |38.538s|priority-first FAIL; transfer-first NOT_RUN |
| Desktop ordinary child API CAS/UI compatibility | NOT_RUN |—|both orderings NOT_RUN |

The reporter's final skipped label has zero attempt results; it means the final case was not run after max-failures1. It does not represent a passed or deliberately exercised business skip. No flaky/retry attempt exists. Runner exit1, raw hashes and report agree. R01 separately remains **2 PASS / 1 FAIL / 1 NOT_RUN**, with a masked primary exception that R02 cannot retrospectively identify.

R02 used the independently accepted R05 harness: spec SHA256 `10cac566342deb24c68e90fd15cdcfd1c8a6aec50c9572116dfbb683c77b67ca`, fixture SHA256 `0188d92a565ae9c63e7d08b30b1cd87cfe0444b8701a6cf0211fab10dcc32614`. Its independent `harness-r02` manifest has **241 entries**, SHA256 `91ba4843b1aaa88d4d71be93b22a2dc50d1736c5d6a0daa16c799a38966b0019`; all declared files/modes were rehashed with zero drift. Runtime before/after source snapshots match. Browser configuration matches the actual command/result hash `d0fc8424bfe477b402114269dde0a0dbac890b8bcb2b6a0cbce5819b1a346fc2`; original stdout/stderr hashes independently match.

The command ran once from `harness-r02` with its explicit closed environment, one worker, retries0, max-failures1 and the exact field grep. Actual password API used the fresh R02 database and PID9667 at64930. Production API/Web/renderer source/builds remain the recorded `source-r01`; no rebuilt product or ROOT production edit is inferred. This source identity can accompany a failed run and does not itself pass the journey.

## Saved shutdown and retained history

Main stopped only owned API PID/PGID9667. Its saved before-stop and after-stop snapshots have the same R02 database, **all211 table counts equal**, all853 migration entries equal, matching indexes and matching actual original gate: executionfalse, creationtrue, hostfalse, epoch2. Connections decrease6→0; the API port is empty. These facts independently match main's stop receipt.

This equality covers the shutdown interval, not the whole testcase: the five begun scenario fixtures and their legitimate success/refusal/history remain in the retained R02 database. Counts are not proof that every row's bytes were unchanged, and this reviewer makes no complete-database-no-effects claim. No failed data/history was deleted or normalized to make the round green. R01's separate corrected creation-gate restoration and original failure remain retained.

## Batch scope intake

The new batch expectation of literal scope **`issue`** is correct. `server/internal/realtime/broadcaster.go` defines `ScopeIssue="issue"`; `handleSubscribe` authorizes that scope and emits `subscribe_ack` with the same scope and normalized issue ID. Core uses a literal `issue` subscription schema and calls `subscribeScope("issue", selected)` through the unchanged WSProvider. The safe CDP observer records frame types and only subscription scope/ID. No normalization or invented acknowledgement is required. Cookie Web authentication remains pre-upgrade101; token Electron's auth_ack order remains separate. This source intake is not an executed batch pass.

## Verification

- **Code/static:** retained exact R05 independent targeted TypeScript/ESLint PASS; no new static execution in this runtime review.
- **R02 source/config/raw inventory:** PASS for identity reconciliation, zero declared source drift; failed round preserved.
- **R02 runtime tests:** **NOT_ACCEPTED**, exact4 tops/8 ordering denominator, two retained Web passes, one Desktop failure and the remaining NOT_RUN items.
- **Saved shutdown audit:** PASS for owned API stop, original flags/epoch, equal ledger/index/table counts and connections0; no fresh reviewer SQL/process probe.
- **Full S3 / TC-16 / parent:** not accepted or changed. A fresh complete run follows actual original-request UI settlement; old passing subcases do not replace it.

Private supports: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/independent-check/client-runtime-r02/`. JSON binds exact raw/ledgers/source/config/shutdown/image and source predicates. Main and the exclusive implementer own the bounded harness correction and a new recorded complete run. This failed-round review is complete.
