# S3 current-client authoring admission — 2026-11-18

**ADMITTED_FOR_BOUNDED_AUTHORING; zero open authoring blockers. Runtime acceptance is NOT_RUN.** The corrected current-client plan follows the actual password, workflow, ordinary-version and WebSocket contracts. This admits the assigned new contention/batch harness; it does not accept that harness's eventual execution, TC-16, R01–R09, full S3 or PAY-241.

Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`; independent reviewer: `/root/s3_clients_check`. This reviewer opened the saved native-hook output and read the task artifacts, selected current plan, accepted reports and actual admission-related code. It wrote only this report pair and private supports under `independent-check/admission-r01`. It did not run a test body, connect to a database/API/browser, control a process, change a product/fixture, stage or commit. Main remains the sole environment, build, SQL and integration owner.

## Findings (fixed)

No product code was changed. Two initial plan ambiguities were resolved by the coordinator after source evidence was sent to both main and the implementer. The final plan bytes are hash-bound in the sibling JSON.

| Initial wording | Actual source and final admission predicate |
| --- | --- |
| An `auth_ack` before issue subscription on both platforms | `server/internal/realtime/hub.go:881` authenticates a Cookie and current password/account context before Upgrade. The `auth_ack` write at line 953 is inside the first-message-token branch only. `packages/core/api/ws-client.ts:107` marks Cookie mode authenticated on open, while token mode waits for `auth_ack`. Web therefore requires a closed old physical socket, a distinct successful 101 replacement upgrade and an authorized exact-issue `subscribe_ack`, followed by actual read recovery. Electron token mode requires type-only auth send, received `auth_ack` before outbound issue subscription, then that issue's acknowledgement. No synthetic acknowledgement or token capture is permitted. |
| Numeric ordinary-child CAS could be read as a request built by the field picker | Ordinary priority and assignee pickers send their field values only (`priority-picker.tsx:102`, `assignee-picker.tsx:206`). `issue-detail.tsx:712`, `issues/mutations.ts:138` and `api/client.ts:3429` forward those values unchanged. `UpdateIssueRequest.expected_revision` is optional; handler `issue.go:4232` enforces it when supplied and line 4435 maps the atomic stale result to numeric `revision_conflict`. The final plan separately records unmodified UI compatibility, explicitly supplied numeric API CAS in platform contexts and the resulting UI projection. It forbids insertion of a version into an intercepted picker request and any claim that ordinary controls force CAS. |

These are test-plan corrections to existing behavior, not newly reproduced product defects or a weakened recovery requirement.

## Source and input verification

Independent byte/mode hashing of the accepted R04 source confirms **5,058 files and zero drift**, manifest SHA256 `1275d9709b94d55586d155cfec42886d14663ed9090d0da34cb7388d49e62dd3`. ROOT differs at exactly four `_test.go` paths and lacks two private diagnostic `_test.go` paths. All production paths represented by that manifest match ROOT. The test differences and old diagnostic absence are retained, not overwritten.

The entry detailed worktree receipt has **388 file entries**; the directory-collapsed view has 350. The entry index was empty and remained empty at this review's read-only reconciliation. Owner classifications in the entry record are preservation labels, not permission to include other tasks in a commit.

A final comparison of those 388 entry files observed two concurrent F3-document changes: `11-16-f3-parent-budget-root/design.md` and `research/step4-binding-check.md`. This reviewer did not edit either and reported the drift to main for peer attribution. Every other preexisting entry matched its recorded bytes/mode at that comparison; neither F3 document is an S3 source or commit input.

The new `source-r01` independently matches the supplied **5,058 backend and 3,612 frontend files**, with zero observed byte/mode drift at the admission capture. Its dependency inventory lists 2,010 edges; complete effective dependency/export and generated-build identity checks belong to the later candidate check. This current frontend is explicitly a new ROOT snapshot, rather than an assertion that every unrelated WIP was previously accepted.

`allocation-r01.json` binds one browser database, `multica_pay241_s3c_20261118_x4c5mjep_r01`, and API/Web/renderer ports **64930/64931/64932**. All five supplied flat configurations still match their allocation hashes and have mode 0600. Their public API/WS/renderer source inputs are consistent; database-bearing configurations name the allocated loopback database. API configuration selects password authentication, the single-process limiter and max/min pool 6/0. Model secret inputs inspected for this admission are empty. These are saved configuration/source checks; this reviewer has not freshly probed a listener, database, gate or browser.

The coordinator's allocation records 853 migrations and the original accepted ledger hash. This saved prerequisite is not a reviewer database execution or a running deployment. Build outputs, live health/source/PID proof, served production Web/renderer bytes, effective browser addresses and final restoration still must be checked before and after runtime acceptance.

## Required harness evidence

1. **Real collaboration contention.** Build both actual UI requests from the same observed collaboration revision before forwarding either. Route control may hold/forward the real requests but must not fabricate a success or refusal. In both orders verify the exact winning field/revision/event and unchanged losing field. A former noncreator handler loses edit authority with forbidden403 after transfer; an authorized old-revision transfer loses with stale409 after priority. Release and join every held request on every failure path. This composed ordering supplements the accepted actual PostgreSQL lock/PID races; it does not pretend to be a new DB-barrier experiment.
2. **Original history and deliberate continuation.** Save the original request keys, payloads and success/refusal receipts as produced. An actor's workflow GET remains content-free after losing write/participation authority (`collaboration_workflow.go:690`), with stored refusal status distinguished from HTTP200 transport. Old POST replay does not restore write authority. A deliberate fresh authorized continuation must leave the earlier receipts and retained execution inputs/runs/events unchanged except for its own documented effect. Recovery alone submits zero new command, upload, comment, delivery or Resume.
3. **Ordinary denominators.** Keep the genuine unmodified UI path separate from explicitly supplied numeric API CAS. CAS must assert the exact `revision_conflict` resource/ID and numeric expected/actual revisions in both orders. A subsequent actual UI projection proves rendering, not that its earlier picker request supplied a revision. Do not infer stale-field refusal for all ordinary clients from these API cases.
4. **Actual offline window.** Observe the old socket's physical close and keep the observer HTTP context offline too. The comment queries poll every 15 seconds; merely closing a socket leaves another path able to read during the alleged offline window. `setBlocked()` in the native fixture controls server switching/issue windows and is not a transport disconnect. Close the recorded native WebSocket without clearing the application's real `onclose`/reconnect handling; reauthentication must come from the replacement connection after writes quiesce.
5. **Complete 55-comment recall.** The shared query sends `limit=50,max_bytes=65536`; handler `collaboration_comments.go:76` selects `RecallComments`. The recall total includes deleted rows, reads newest-first `(created_at,id)` and reverses each emitted page into chronological order. Its byte budget can use more than two pages and its first oversized row can be an explicit placeholder. Retain all 55 original keys/IDs/revisions and enumerate any additional control/reply comment separately. Walk the actual Load more control through a null cursor, retain every raw page and compare the full quiescent persistent ID/tombstone/reply/version/body set. Do not assume exactly 50+5, drop a tombstone, call omitted content complete or concatenate newest-page plus older-page as global chronological order.
6. **Raw page versus UI identity.** `collaboration-detail.tsx:79` merges by ID; a duplicate API row can therefore disappear in the DOM. Check page-chain progress, exact unique IDs and overlap independently of the final rendered rows. Record refetch/polling chains separately from the completed final pass. Reconnect again without new intent and compare persisted counts/identities and zero recovery writes.
7. **Current password/native scope.** The Web actor must receive both real login cookies, verify HttpOnly auth and the readable CSRF cookie, and issue control requests with the actual signed CSRF path. Cookie/CSRF values and auth frames stay out of attachments. Native uses the real password-issued token in its independent profile and the matching built renderer/preload. This existing native harness does not establish installed Desktop Main or real Provider behavior.

Planned top-level cases, actual attempts, ordering subcases, UI compatibility and explicit API-CAS subcases must have separate honest inventories. Save partial progress and failures as they happen; a later complete replay uses a distinct run/source/output identity. These inventories supplement, rather than add to, the unchanged parent 24-case denominator.

## Findings (not fixed)

No open authoring-design finding remains after the two coordinator corrections. The following execution gates remain deliberately unaccepted:

- New harness implementation and its lint/type-check; later final candidate source/dependency/build checks and actual complete Web/Electron case execution.
- Fresh live API/config/database/listener/origin/source identity and final source/config/gate/connection/process restoration. The allocation records alone do not prove them.
- Preserve and attribute the two separately reported concurrent F3-document changes; do not restore their entry bytes or include them in an S3 commit. This is unrelated peer bookkeeping, not an authoring/product blocker.
- Full notification, upload, revoked indirect reads and structured blocking/repair/history. The accepted v2/r05 document is a design contract; the current selected source does not thereby implement its new fields/routes.
- Privacy recovery distinguishes content-free own workflow/comment results from content-bearing upload metadata. `GetAttachmentUpload` checks current attachment authority (`collaboration_attachments.go:324`); it must deny after all read bases are lost. `GetCommentOperation` may return completion identity after departure while dropping targets/attachment IDs (`collaboration_comments_edit.go:146`). Historical receipts grant no current file/runtime/content permission.
- Complete S3, real S4, final S5 and business signatures remain open. No passing prior narrow test or synthetic account changes those requirements.

## Verification

- **Admission/source/configuration: PASS for bounded authoring.** Final corrected plan and actual source predicates agree; accepted/new declared source files and configuration hashes/modes match the saved inputs.
- **Lint: NOT_RUN.** No implementation/static execution was included in this read-only assignment.
- **TypeCheck: NOT_RUN.** Reserved for the completed harness/candidate follow-up.
- **Tests / database / API / browser / Provider: NOT_RUN, zero reviewer executions.** Existing service/handler normal/race evidence was read and preserved, not rerun or broadened.
- **Full S3 / TC-16 / parent acceptance: NOT_ACCEPTED by this report.** Keep the task in progress and current matrix counts until actual bounded evidence is independently checked.

Private evidence: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/independent-check/admission-r01/`. Sibling JSON binds the final plan, relevant code/reports, two source/config reconciliation receipts and the explicit execution exclusions. Main owns candidate execution and integration; this admission slice has no remaining reviewer work.
