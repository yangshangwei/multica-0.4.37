# PAY-241 TC-06 concurrent human edit and runtime reply

Continue the existing `11-13-pay-241-collaboration-acceptance` parent. The current ledger is 9 PASS /11 PARTIAL /4 NOT_RUN; TC-12's local code-revision supplement is accepted and its real payment repair remains S4. Do not repeat that chain or the accepted S2/S3 gate matrices.

## Missing stimulus and scope

TC-06 still names concurrent content preservation. Existing tests separately cover human comment edit/delete, frozen waiting inputs and runtime output identity. The selected supplement composes a live source-bound runtime reply with its human input author's edit, under both actual database commit orders. Consult `research/tc06-content-stimulus-gap-20261120.md` for the entry points and existing coverage.

Add one service test file, `server/internal/service/collaboration_concurrent_content_test.go`. Reuse `newStageRuntimeFixture`, actual comment/claim/permit/report owners, `recipientCommitStarter` and observed PostgreSQL blocking. Main owns the frozen committed-source candidate, environment, specification, task records and Git. The implementation agent owns only the new test in the private candidate. No product, dependency, migration or parallel workflow change is planned. A demonstrated product failure must be diagnosed before expanding this boundary.

The ordinary `handler/comment.go` administrator authority gap belongs to the separate `11-20-comment-admin-human-authority` planning task. This supplement exercises the dedicated collaboration comment and run-bound output owners; it does not close that independent issue or assert that every ordinary Agent comment mutation is forbidden.

## Preregistered proof

One top-level test, `TestCollaborationHumanEditRuntimeReplyBothCommitOrders`, with exactly `human_edit_first` and `runtime_reply_first`. Normal/race each execute one top and two leaves: three distinct named tests and six mode executions, supporting one internal supplement.

1. Create a typed human comment V1 against one explicit code candidate; claim and permit its actual service run, retaining original request, command, run, attempt and frozen test-subject identities. PR/CI, managed preparation/consent and execution settlement remain explicit fixtures.
2. Prepare the author's valid revision-1 edit to V2 and an admitted Agent reply with a distinct comment UUID and the original human comment as parent. Hold the leading real transaction before commit, start the follower and observe its PostgreSQL lock wait before release. Exercise both commit orders with unchanged existing deadlines; no sleep as proof of overlap.
3. Both writes succeed once. The human comment retains its author, V2 and revision2; the reply has its own Agent/run identity and original parent. The create/edit history preserves exact V1/V2, actor/time and original operations. Editing displayed prose does not alter the admitted V1 command, frozen group input, prompt or candidate/test-subject identity and does not create another command/run.
4. While the run is live, recover and replay the original human create/edit requests and runtime reply. Require exact original content-free receipts and complete business snapshots unchanged. In particular, an old create replay must not roll V2 back to V1, re-dispatch the input or duplicate replies/notifications. Runtime comment replay checks current business authority first; it must not be misrepresented as a post-settlement replay permission.
5. Record one report through the same run's production report service, linked to that frozen subject and candidate, then settle through the existing fixture. Recover the original report ACK and human receipts after settlement; require complete final business snapshots unchanged and retain the original runtime reply event/history bytes. This report is synthetic evidence, not a new executed payment regression. Record concrete identity joins and complete protected collections before fixture cleanup.

## Validation and evidence

Intake committed HEAD is `d1b25077b89af8ef72e58c0b040b9ea495261b1b`. A private `git archive` snapshot excludes all shared WIP, including another session's stage-execution clock fixture edits. Shared PostgreSQL now answers a read-only readiness probe; this run uses a fresh owned container from the already-installed pgvector image to avoid contention with active peer workloads. Current migrations produce 856 ledger entries and 663 valid/ready indexes.

Allocate one fresh service database per normal/race mode. Bind the complete source manifest, tools, exact argv, private configuration and named denominator before test execution. Compile-only uses `go test -c`; `-run '^$'` is never compile-only. Run gofmt, service vet and the selected normal/race test with an unchanged 10-minute suite deadline. Preserve every failed source/configuration/database/raw round; no relaxed assertions, timeout increases or spliced passes.

An independent native `trellis-check` reviews source and then final raw execution/evidence/records. Check complete run/terminal sets, zero FAIL/SKIP/unfinished names, source/config/tool stability, original receipts, current database catalog/gate facts and owned process termination. Existing fixture cleanup may remove transient SQL rows: retained private snapshots and actual residual database counts must be reported separately. Stop only the owned container after zero-connection checks, retaining its volume and logs.

Update only TC-06 current evidence/gap and relevant parent progress/context. TC-06 remains PARTIAL because remote branches/PR/CI and full real-system association still require S4. No new HTTP/router/browser/password-login, real daemon checkout, real model, payment, deployment, S5 or business-signature evidence is claimed. Finish with directed Lore work and record commits, fresh HEAD/index/remote verification and push to the existing origin branch. Do not archive the parent.
