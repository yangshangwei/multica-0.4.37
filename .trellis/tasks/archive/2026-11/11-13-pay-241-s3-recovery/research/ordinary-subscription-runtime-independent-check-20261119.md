# Ordinary subscription runtime independent check — 2026-11-19

**Current disposition: R04 NOT_ACCEPTED; R01–R03 remain retained failures.** R04 has two genuine failed tops, zero passes and zero retries, and stops at the new composer locator before its first comment. The original R03 partial facts below remain independently reconciled: both platforms reached manual subscription, first-comment delivery, original notice read/anchor, real reconnect/reload and a persisted this-issue unsubscribe tombstone. Those facts do not combine with R04 to satisfy the complete two-comment journey. The R04 follow-up at the end of this report supersedes the original pending-R04 status without relabeling any failed round.

- Reviewer: `/root/notification_runtime_check`, already-dispatched native `trellis-check`; no spawning.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Owned writes: this report and adjacent evidence map only. Existing ordinary-code review and NR-P01 closure remain intact.
- Private evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly` (`B` below).
- R03 execution uses `B/source/client-harness-r04`, fixture SHA-256 `0cbc6d4fd7a4a6612021a55bd080cc815aca8c8dbefba58f638902c0a287fab1`; its labels must not be confused with forthcoming runtime R04.
- Main owns all configuration, API/browser/database execution and process control. This reviewer used only retained filesystem/source/raw evidence; no source/config/DB/API/browser/Provider or metadata/spec/commit mutation was performed.

## Findings (fixed)

None applied by this reviewer. R01/R02 source/configuration corrections are retained under their original failures, not presented as new completed journey results. The next owner fixes will be independently dispositioned after their frozen source and fresh complete round are available.

## Findings (not fixed)

### OS-P01 — second-comment Send locator is ambiguous in both clients

- Frozen R03 fixture: `e2e/fixtures/collaboration-recovery-subscriptions.ts:380`.
- Actual R03 error: `getByRole('button', { name: 'Send', exact: true })` resolves to two real controls, including a thread-list Send button and the main composer Send button. It occurs at `second_comment_after_unsubscribe` on Web and Desktop. There is no second-comment POST/201/ID to accept.
- The uniquely selected main editor had already been filled. Accepted R08 `packages/views/issues/components/comment-input.tsx:198`–`:243` contains that editor under `div.rounded-lg.bg-card`; its own SubmitButton is at `:287`. A source-backed, unique composer containing that original editor is a bounded way to select its Send action and wait for its enabled state, without force-clicking or modifying the product.
- Main has authorized the owner correction and a new frozen source/whole round. No prospective source or previous partial success is treated as runtime acceptance here.

### OS-P02 — final mutation equality counts read-only table POSTs as writes

**Definite upcoming assertion mismatch, independently reported before R04 execution.**

- R03 fixture `:294` records every non-GET `/api/` request into the ordinary transport inventory. The final assertion at `:531`–`:533` compares the complete watcher inventory with only Subscribe, read and Unsubscribe.
- Actual Desktop R03 watcher inventory contains **12 POSTs**: those three mutations plus **seven `/api/issues/table/rows` and two `/api/issues/table/facets`** requests caused by ordinary navigation. Web has only the three mutations in this round. The broad inventory is useful and must remain retained.
- Frozen final server routes: `cmd/server/router.go:2259` binds POST `/table/rows` to `Handler.ListIssueTableRows`; `:2260` binds POST `/table/facets` to `Handler.ListIssueTableFacets`.
- `internal/handler/issue_table_rows.go:221` decodes `issueTableRowsRequest` and calls `beginIssueTableSnapshot` at `:233`. `issue_table_facets.go:340` decodes `issueTableFacetsRequest`, validates its facet count and calls the same helper at `:355`.
- `internal/handler/issue_table_query.go:35`–`:50` explicitly executes `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY` and binds the snapshot queries to that transaction. The handlers perform authorized queries/scans and commit their read snapshots (`rows:480`, `facets:437`); they do not mutate business state.
- Accepted R08 Core signatures are `listIssueTableRows(params: IssueTableRowsRequest): Promise<IssueTableRowsResponse>` at `packages/core/api/client.ts:3078` and `listIssueTableFacets(params: IssueTableFacetsRequest): Promise<IssueTableFacetsResponse>` at `:3091`. Both send their JSON query through POST.
- Main approved classifying **only POST plus either exact confirmed pathname** as query reads for strict business-operation equality. Preserve every original transport fact/hash/ID and all other mutation, audit and no-extra-write checks. No blanket table-POST exemption, unknown endpoint or mutation route is justified by this evidence.

No production notification/subscription defect was established by these two harness failures. OS-P02 has not been disguised as an actual failure reached in R03; execution stopped earlier at OS-P01.

## Retained rounds and exact failure identities

| Whole round | Raw stdout SHA-256 | Complete result and stopping point |
| --- | --- | --- |
| `subscriptions-runtime-r01` | `169960dee0400912770c4a35c72b5d544743a758a079e7eb43735233c69ba8c7` | 0 PASS / 2 FAIL, retry 0. Web baseURL absent at relative URL construction; Desktop exact text assertion did not account for canonical Markdown escaping |
| `subscriptions-runtime-r02` | `a4552ce47c4554f4ec6e41b81cb77699ba4ce86036e1b353f4ecc28c918d19e9` | 0 PASS / 2 FAIL, retry 0. Both clients display actual comment body; fixture incorrectly demanded literal “New comment” text |
| `subscriptions-runtime-r03` | `cf8fee69beff956315388e797c3c30e8e9a8155d673a89dd1687a27920a481c5` | 0 PASS / 2 FAIL, retry 0. Both stop at second-comment ambiguous Send |

All three commands exit 1 with zero recorded source/config/tool drift. Each raw Playwright report contains both expected top names, one failed attempt per top, zero skipped/flaky results and no global reporter error. The final R03 round inventory records both `FAILED_OR_INCOMPLETE` entries with null results. Its Web attachment correctly reflects the earlier point when Desktop was still NOT_RUN; the later final round inventory supplies the whole denominator. Do not splice an earlier successful checkpoint into another round's top result.

The R03 attached per-platform JSON and final local evidence have identical checkpoints, sessions and common values. The latter additionally records successful evidence-attachment outcomes and `primary_failure_preserved=true`; the attachment copy predates those final bookkeeping fields. The exact SQL, bindings, rows and hashes are present in both attachments.

## Independently reconciled R03 partial steps

| Platform | Original child ID | Original first comment ID | Original inbox ID | Actual recipient |
| --- | --- | --- | --- | --- |
| Web | `01a1f15a-200c-701f-a24b-9d0cb980b0bb` | `01a1f15a-308e-77a5-a2f3-7ac19df6c4df` | `073dcfdf-7016-51ea-9301-47ce52f72cd5` | `a1234d48-4815-46c0-b7a4-783cd0178814` |
| Desktop | `01a1f15a-4534-7d69-8f3c-ed9046261c57` | `01a1f15a-5da1-76f1-9401-a30328eb6776` | `2c42e72e-fa3f-50aa-831f-8ff1cc2a5018` | `45271955-489b-48f2-836c-2068ce0071e6` |

Each platform uses four distinct, publicly registered/onboarded password actors and a separate real source workspace. Share-link joins establish three actual member rows; the watcher is a source member with no fabricated collaboration participation. The parent is a collaboration issue; the child is ordinary with its exact parent relationship and no Agent assignment. The pre-existing optout control explicitly subscribes/unsubscribes through API and retains its tombstone. Nonwatcher has no subscriber row.

1. **Actual Subscribe:** the watcher clicks the real control; POST returns 200, and its persisted row is `reason=manual`, current member ID, null `unsubscribed_at` and null scope. The actor/child/source tuple matches every later phase.
2. **Actual first comment:** a distinct author submits through the unmodified ordinary composer; real POST returns 201 with no Idempotency-Key. The saved comment is revision 1, member-authored, unmentioned, attachment-free, not a reply and not a task result. Its exact content hash matches the SQL row.
3. **Complete first delivery:** all four actors' own inbox API reads and the full bounded SQL inventory identify exactly one member `new_comment` for the watcher. Author, optout and nonwatcher receive none. The persisted comment UUID, actor, source, issue and notice ID all agree; there is no UI-only deduplication inference.
4. **Original read/anchor:** the real inbox selection sends POST `/api/inbox/{original ID}/read` and returns 200. Only `read` changes to true; archived remains false. The actual renderer targets `#comment-body-{original comment ID}`, inside the viewport with exact original content and the same `data-inbox-selection` ID.
5. **Real reconnect/reload:** the original workspace socket closes during deliberate offline control, a distinct authenticated workspace socket appears, then a reload creates another distinct authenticated workspace socket. The resulting API/SQL notice retains ID, read state, body/details/title hashes and timestamp. All 17 audited relation sets equal the pre-reconnect read snapshot, and no additional mutation is recorded during that recovery interval.
6. **Actual this-issue Unsubscribe:** the main Unsubscribe control POST returns 200. The original subscriber row retains its `created_at` and member identity, sets a real timestamp and `opt_out_scope=issue`, and retains `reason=manual`. The original read notice and first comment are unchanged. This establishes the tombstone; it does not establish nondelivery for the unsubmitted second comment.

Both platform attachments contain all six reached snapshots, each with 17 relation queries, exact source/parent/child bindings, row counts within the 128 bound and matching SHA-256 values. Collaboration/session/event/operation/notification rows and the parent issue remain unchanged through the reached steps. Commands, groups, runs, dispatches, both attachment families and keyed delegation receipts remain empty. The controls retain the actual member/optout/nonwatcher distinctions.

## R03 transport and native cleanup

Web mutations use real cookies/CSRF and the source workspace slug; native mutations use the real password-issued bearer with the same source binding. Recorded credentials are presence indicators only. Workspace handshake IDs, not collaboration issue `subscribe_ack`, establish ordinary transport membership. Desktop records `auth` before `auth_ack`; cookie Web has no explicit auth frame.

- Web watcher: original workspace socket `5887.297`, reconnect `5887.382`, reload `5887.713`; all five observed sockets include the initial account socket and transient reload account socket.
- Desktop watcher: original workspace socket `5956.157`, reconnect `5956.221`, reload `5956.398`; likewise five total observed identities. Do not report only the three selected workspace sockets as the entire transport inventory.
- The four session closures are fulfilled with no cleanup/page/comment-page errors. Both Web page/context close timestamps are recorded. Native author PID **5928** and watcher PID **5951** each have actual exit code 0, no signal, and recorded page/context/profile-close completion.
- Final socket `closed_at` can remain null because the helper detaches CDP before context closure. That is an unobserved terminal frame, not evidence that the socket stayed live. Actual context/native closure and owned API shutdown have separate recorded proof.
- Main's R03 API records bind launch/readiness/stopping to PID/PGID **71510**, port **64940**, final GREEN API binary SHA-256 `0c8ca9898c5b022df40bf2230849c956ac2c14ee5fe59d4d87b02f805306f999`, and one unchanged configuration hash. Health reports the actual built `b1901a48f` identity; the later local commit is not substituted into that historic response. SIGTERM stopping records `listener_absent=true`.
- Post-stop audit reports the dedicated `subscriptions_r04` database, `other_connections=0`, unchanged gate epoch 2/creation true/execution false/host false, and retained two comments/two inbox notices across the failed round. Web PID 25150/port 64941 and renderer PID 25647/port 64942 remain Main-owned held frontends; this review does not claim all services were stopped.

The retained database has eight synthetic users/members, two workspaces, four issues, two ordinary comments and two notices. Their retention is intentional failure evidence. No reviewer cleanup or speculative database repair was performed. Actual runtime B, vendor models, installed full Main, uploads and full S3 remain unexecuted by this review.

## Source/static checks and next acceptance boundary

R03's 257-entry harness manifests match before/after and current byte/mode inspection. Its matched strict type-check, nonignored targeted ESLint and exact two-top list receipts all exit 0. The runtime body still exits 1; statics do not relabel it.

The next round must use its own frozen fixture/config/source and fresh database, both exact tops, workers 1/retries 0 and all original/final inventory records. It must complete both unkeyed UI comment POSTs, retain the first notice/read state after real reconnect/reload and unsubscribe, prove no notice for the distinct second comment, preserve the optout/nonwatcher/parent controls and complete every client/API/native/catalog/connection closure. OS-P02 must retain the full transport inventory while distinguishing only the two confirmed query POST routes.

Literal original-event replay remains accepted separately at the Go listener layer. This browser slice does not add automatic composer idempotency, generic-event deduplication, mixed old/new writer safety or frozen ordinary late-subscriber decisions. Future runtime notification/token withdrawal proof and NR-P02 are separate; neither this partial round nor an eventual ordinary subscription PASS closes full S3.

## Verification

- R03 raw/result/inventory/attachment inspection: complete, **0 PASS / 2 FAIL / retry 0** preserved.
- R03 reached SQL hashes/bounds/identities and before/after relationships: independently reconciled.
- R03 session/native/API stop/connection evidence: independently inspected; exact observation limits above retained.
- Lint/type-check: Main's matching R03 static receipts pass; no checks or browser bodies were rerun by this reviewer.
- Source/product changes by reviewer: none. Owned report/evidence only.
- Full ordinary runtime acceptance: **pending fresh R04**, with OS-P01/OS-P02 owner fixes and complete final evidence required. Parent task/spec/matrix status unchanged by this review.

## R04 follow-up — retained failure, no whole-round acceptance

The fresh `subscriptions-runtime-r04` uses `source/client-harness-r05`, fixture SHA-256 `f62c310bc84f02eba054c85e490bbf0c84054ad95fba332eead696bca62755b0`, a distinct `subscriptions_r05` database and its closed R05 configuration. Raw stdout SHA-256 is `ca28e1fd513db8d6e39ecad33804a939187b01a5eede0963e5f986850f25e8ec`. Both exact tops have one failed attempt at retry 0; command exit is 1, with no recorded source/config/tool drift or global reporter error.

### OS-P02 disposition: closed statically for the R04 fixture

`isIssueTableQueryRead` at new fixture `:290` admits only method POST and exact `/api/issues/table/rows` or `/api/issues/table/facets`. The observer still records every original fact and separately saves `query_read_facts`. Only the final strict watcher mutation equality filters those two confirmed read paths; every other mutation and all audit/recovery checks remain. This implements Main's bounded authorization and preserves the original 12-fact R03 inventory. R04 stops before that final equality, so the static fix is not represented as a reached runtime assertion.

### OS-P03 — the new composer selector binds no element

- New frozen fixture `:385`–`:386` filters `div.rounded-lg.bg-card` by the previously selected main editor, then requires count 1. Both real R04 clients return count 0 and fail before `uiResponse`/first-comment POST. Source stack and the two raw errors pin this exact assertion.
- Both reached the actual Subscribe control and retained its manual row; neither emitted a saved comment or inbox notice. Author transport retains its natural `comments/trigger-preview` POST while the draft is typed. That query is not a submitted comment.
- The R03 page-wide Send problem remains unresolved at complete runtime level. The R04 attempt introduced this earlier binding failure; neither failure is hidden or counted as a product notification defect.
- Read-only source/artifact inspection confirms that accepted R08 CommentInput source and both served bundles contain the exact card class and ContentEditor child. Web `apps/web/.next/static/chunks/29363-207b9ea372ef13a4.js` SHA-256 is `4b27c6fb28232f366e22a99a4ecf206bc958f2b61dc19b5d5c4f12387e5e06d9`; renderer `apps/desktop/out/renderer/assets/index-SL3fwV2N.js` SHA-256 is `36d4dd0215d85089b0849be46b8cfe6b1d8fdf8714fef24bcd551a625ac1f309`. Thus this result does not establish an obsolete product bundle or an absent product card.
- No retained HTML/error-context/trace artifact was found; only JSON and screenshots. A Main-owned content-free observation of the selected editor's actual ancestor tag/classes and Send controls is needed before selecting another structural boundary. The reviewer did not invent a new selector, change the fixture or execute a browser diagnostic.

Both R04 per-platform final records and attachments retain primary failure plus fulfilled observer/session/attachment cleanup. Web contexts/pages close. Native PIDs **26720** and **26729** each exit 0 with no signal and recorded page/context/profile completion. The final whole-round inventory remains two `FAILED_OR_INCOMPLETE` entries with null results.

Main's final R04 failure audit is now reconciled: API PID/PGID **25659**, port **64940**, the same exact GREEN binary, one configuration hash and actual `b1901a48f` health identity agree across launch/readiness/stopping. SIGTERM stopping records `listener_absent=true`. `audit/subscription-runtime-after-r04.json` SHA-256 `79c4b91e10a0fd9f19d05a5ef9225175d7c13190da8c1dc7ad8d8b0dc45a7c59` identifies `subscriptions_r05`, connections 0, unchanged gate, the exact 855-entry ledger and all 663 unchanged ready/valid index definitions. It retains eight users/members, two workspaces/four issues and six subscriber rows, but **zero comments and inbox notices** in this new failed database. Existing knowledge invalidations and fixture records are retained; none were deleted by the reviewer.

R04's corresponding `subscription-typecheck-r05`, `subscription-eslint-r05` and `subscription-list-r05` Main receipts exit 0 with matching raw hashes and unchanged 257-entry source/config/tool bindings. The actual body exits 1. These checks close source/failure-retention reconciliation, not the unfinished comment/delivery journey.

The previously documented R03 two comments/two original notices remain in their own failed database. R04 has its own source, actors, manual rows and failure evidence. No cross-round partial PASS, task/spec/matrix update, runtime B result or full S3 acceptance follows from this review.
