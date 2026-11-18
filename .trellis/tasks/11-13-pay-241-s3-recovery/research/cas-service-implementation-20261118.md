# S3 service priority CAS implementation — 2026-11-18

Status: **ACCEPTED SERVICE SLICE.** The owned code, static checks, coordinator-run PostgreSQL normal/race inventories, independent raw audits and final source applicability are accepted within this test-only S3 R04 / TC-16 service increment. Full S3, TC-16, parent24, native execution and current API/Web/Desktop acceptance remain open. The combined check still awaits handler-race completion at this handoff. The pre-correction r01 snapshot remains `NOT_RUN_PRE_EXECUTION_TEST_FIXTURE_REVIEW`.

Owner: `/root/s3_service_cas_implement`. Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.

## Changed files and source

The only code addition is `server/internal/service/collaboration_recovery_cas_test.go` in the private source `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/source-r01`.

- Current formatted code SHA256: `bc87efccf85c2ac0f8a7c2c74a91e102b21198938992c84272dc9abb00076b29`.
- Root reports owned by this implementer: this Markdown file and `cas-service-implementation-20261118.json`.
- Current static artifacts: `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/service-static-r02/`. The original r01 artifacts and report snapshots are retained under `service-static-r01/`.

The admitted input is final accepted R33, as recorded in [s3-source-binding-20261118.md](s3-source-binding-20261118.md). The coordinator verified 5,056 existing files and their modes before creating this source. This owner read code and fixtures exclusively from that private source, wrote no ROOT server file, and changed no existing test, production implementation, SQL/DDL, dependency, specification, plan, task state or Git data. The later final source contains one independently checked handler TestMain fixture delta owned elsewhere; its service applicability is recorded below. The coordinator retains ownership of combined source and final preservation audits.

## Implemented inventory

Exactly two top-level tests, five subtests, seven named identities:

1. `TestCollaborationRecoveryPriorityConcurrentIdentity`:
   - `same_key_same_priority`: two real concurrent priority-only workflows with the same key/payload converge on one success, operation and edit event.
   - `same_key_different_priority`: the original high-priority request wins; the same key with low priority returns409 `idempotency_conflict` and retains the original payload hash/result/effect identity. The conflicting payload has no separate stored receipt.
   - `different_keys_same_revision`: distinct high/low priority inputs at the same observed revision produce one success and one durable409 `stale_revision`. The winner and loser retain distinct operation IDs/hashes; only the winner owns a business event.
2. `TestCollaborationRecoveryPriorityTransferCommitOrders`:
   - `priority_first`: priority commits; the original noncreator handler remains current at the competing transfer, whose old revision produces durable409 `stale_revision`. Responsibility stays with that handler until an explicitly fresh transfer.
   - `transfer_first`: transfer commits to the creator; the old noncreator handler's competing priority edit produces durable403 `forbidden`. Priority stays unchanged until a fresh creator-authorized edit.

Each identity case then submits a fresh observed low-priority request and proves original successes/refusals never recompute. Each transfer case rereads the actual current actor/version/epochs, submits a new authorized key to apply the remaining field, and recovers the original handler-owned successes/refusals through `GetWorkflowOperation`. After responsibility moves, old handler POSTs remain forbidden; GET recovery preserves their original status/code/version and exact stored result JSON. A creator cannot recover another actor's own operation key.

## Assertions and reuse

- Both normal fields and collaboration versions advance exactly once for the race winner and once more for the explicit fresh continuation. The losing priority/handler field remains unchanged. Full issue comparison permits only the selected priority/handler, revision and real update timestamp; lifecycle, account/participant epochs, information pointer, AI policy and session control remain checked separately.
- Edit history binds the winning event ID, sequence, actor, original request, actual persisted edit time, receipt and before/after priority. Its other metadata values are identical. Transfer history binds the original event and handoff ID, actual actor/from/to/note/prior status and its exact saved receipt. The one transfer notification references that same event/handoff and the creator recipient.
- Per-key operation snapshots include IDs, scope/actor/request/kind, complete request hash, result code, raw stored JSON and created/completed identity. Before/after GET comparison preserves the complete saved row. Own GET JSON contains only status, code and decimal collaboration revision for these operations.
- Every fixture seeds a synthetic retained run with one permitted-attempt prerequisite and a separate waiting input through existing queue helpers. Complete execution and input snapshots check commands, grants, channels, targets, groups, runs, frozen inputs and related control state; the ordinary queue stays empty. These seeded rows do not prove native or Provider execution.
- `outbox_state` is explicitly omitted from event business equality and event history snapshots. Asynchronous delivery markers are not classified as losing business writes. This increment does not run or certify an outbox delivery journey.
- Existing `newCollaborationQueueFixture`, `workflowRequest`, `fieldJSON`, `mustAccess`, `sourceRoleBusinessSnapshot`, `sourceRoleExecutionSnapshot`, `sourceAuthorityExecutionState`, `lifecycleCommitBarrierStarter` and `waitWorkflowBlocked` are reused. No alternate transaction wrapper, fake CAS response, sleep barrier, open-coded INSERT/cleanup pattern or production abstraction was added.
- At each queue-fixture creation, exact-session cleanup registers `collaboration_channel` followed by `collaboration_batch` before creating inputs/runs. These are the existing `newCollaborationContentFixture` registrations from `collaboration_comments_test.go:140–141`. LIFO cleanup removes the batch, then the channel, after later run/input fixture cleanup; it leaves unrelated sources and protected history alone.
- The small orchestration helper starts the actual leading/following services, observes the real PostgreSQL blocking PID before release, and defers release, context cancellation and joining both goroutines. An early failure, including `Fatal` inside the wait helper, cannot leave either writer behind the commit barrier. This failure-path safety was inspected; no new failure-path matrix is claimed.

## Verification performed

The coordinator's static permit supplied the pinned Node/Go binaries, `helpers/run-nodb.mjs` and `configs/nodb.json`. The runner removes inherited database/account settings and admits a no-database fixture configuration. Configuration values were never sourced as shell code or printed.

| Check | Result | Evidence |
| --- | --- | --- |
| `/opt/homebrew/bin/gofmt -w <owned new file>` | exit0; 0.008s | `service-static-r02/gofmt.json`, empty stdout/stderr |
| `/opt/homebrew/bin/go test -c -o <service-static-r02/service.test> ./internal/service`, through the approved no-database runner | exit0; 17.092s | `service-static-r02/compile.json`, empty stdout/stderr, compiled binary SHA recorded |
| `/opt/homebrew/bin/go vet ./internal/service`, through the same runner | exit0; 1.992s | `service-static-r02/vet.json`, empty stdout/stderr |

Both Go commands ran in the private `source-r01/server` directory. Runner, configuration, actual pinned tool and owned source hashes match before/after each command. Compilation and vet executed no test body or `TestMain`. No Go database suite, API, migration, browser, account/Provider, payment, deployment or Git workflow/change was executed by this owner. The approved static runner performs its existing read-only Git probe of the private HOME boundary.

The coordinator subsequently ran the exact inventory once per normal/race profile, serially, from `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/source-verification-r03/server`. Both commands used the recorded `helpers/run.mjs` wrapper and separate fresh database/configuration profiles:

```text
go test -json -count=1 -run '^TestCollaborationRecoveryPriority(ConcurrentIdentity|TransferCommitOrders)$' ./internal/service
go test -json -count=1 -race -run '^TestCollaborationRecoveryPriority(ConcurrentIdentity|TransferCommitOrders)$' ./internal/service
```

| Profile | Actual inventory and exit | Result receipt SHA256 | Independent raw audit SHA256 |
| --- | --- | --- | --- |
| `verification/service-normal` | 2 tops / 7 named / 5 leaves; all PASS; exit0 | `115ed297f15d978eac92bd0b964c0f8a96135c7bf7f5bc0ac6d16227e7a7e908` | `d4e216b88a68023bcc550ce5ba0b4507acc5712d6093794bb03a1d9c3dc13704` |
| `verification/service-race` | 2 tops / 7 named / 5 leaves; all PASS; exit0 | `a3881c40342af19438d2e17bd26b363f7c641a1afd4c94f9a15053a828e30cf9` | `82b0cd93727a923363d9b32ae40999a0e6a57931adec7c912fad45edb2a2edff` |

Each row binds `result.json` and `independent-check/raw-inventory-service-{normal|race}-r01.json` under the evidence root. The independent audits verify one start and one PASS terminal for every expected name, identical normal/race sets, and no failed, skipped, duplicate, missing, unexpected, unfinished or unstarted identity. Raw stdout hashes are respectively `57a5ae04dd043998e11aae0cc0b48e964ccde6f56d559193d4d47903481af70c` and `e0194a57afc517277045652a5ea3f847d94802c938ebcef51ed51e2afd26b788`; both stderr files are empty.

Both runs bind the 5,058-file r03 source manifest SHA256 `992ff44a33f70b83914a3fe49f99533413b35dd416f3b80644b2f3e2a96d5b42`. Their source-before/source-after receipt SHA256 is identical: `543b4ce85eb00f2ec45f71479553f718f4e9d63f287347e68e99921a6f1d4187`. Configuration hashes are `34a290ff01ffb74c4c178ff9794f7faf743e4d7670dc9f92716610bd081b2ad3` for normal and `3ad487908e083a4967a335c075f40ad3b8786f940f709f43193fa45ccd0255bc` for race, with no admitted drift.

The normal database was `multica_pay241_s3_20261118_hxjmd77_service_normal`; race used `multica_pay241_s3_20261118_hxjmd77_service_race`. All 30 audited fixture table counts are zero before, after tests and at final audit; final other connections are zero. Their 853-entry migration ledgers remain identical. Fixture cleanup temporarily closed creation, and the coordinator restored the original gate flags with the epoch advancing2→3 in each database; execution and host admission remained closed. No protected history was deleted to obtain these passes.

This report-only handoff reads and binds those completed receipts. It executes no new test, database, Go, API, Git or agent operation and makes no matrix/task-state change.

## Final source applicability without another service run

After the accepted service runs, the final r04 source changed only `server/internal/handler/handler_test.go`: an independently checked five-line dependent-target fixture cleanup in that package's TestMain. The final manifest is `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/source-verification-r04-manifest.json`, SHA256 `1275d9709b94d55586d155cfec42886d14663ed9090d0da34cb7388d49e62dd3`. It has 5,055 unchanged R33 base files, one allowed fixture delta and the same two test additions; all 5,056 base paths are retained.

`independent-check/source-static-audit-r03.json`, SHA256 `5ce577fcffcbe7710863c39364084e90fdacfd2fe4481f218c9cde015ffb6aa7`, is PASS with zero errors. It confirms every other file/mode is equal across r03/r04, including this service file at `bc87efccf85c2ac0f8a7c2c74a91e102b21198938992c84272dc9abb00076b29`; the common 5,057-entry fingerprint is `a6d7449609ed6bb8f2613ea557d3e801ce3847657787c0e5bc6e16df2741b2ef`.

Go compiles a package's test files only for that package. The changed handler `_test.go` is not a service compilation input, and these selected service tests call service/database fixtures directly without invoking handler TestMain, handler test bodies or a handler subprocess. The audit therefore accepts the unchanged service proof on the final source. **No service run on source-verification-r04 is claimed or repeated.** This applicability result does not complete the combined handler verification or native/product acceptance.

## Retained r01 and pre-execution cleanup correction

The original formatted r01 source SHA was `ec6c67b6ad847e62be5698d7c88f5e9dfb0b917aa650e8101859c1753290ae6d`. Its gofmt, compile-only16.550s and vet2.284s all passed with empty diagnostics. Its exact command/configuration/source/tool evidence remains unchanged in `service-static-r01/`; the original report Markdown/JSON are also retained there as `implementation-report-r01.md/json`.

Before any database test or `TestMain`, source inspection found that `newCollaborationQueueFixture` does not register generated channel/batch cleanup, although the existing content fixture does. The coordinator preserved the first combined snapshot as `NOT_RUN_PRE_EXECUTION_TEST_FIXTURE_REVIEW`, confirmed all four databases remained unused, and authorized only the two existing exact-session registrations at each new service fixture. No product defect or executed failure is attributed to this observation.

The r02 delta is exactly four added cleanup lines: channel/batch registrations at the two fixture locations. Removing those lines recreates the exact r01 SHA. `service-static-r02/source-delta.json` records that comparison and both registration/teardown orders. Formatting changed no further bytes; no existing helper, original snapshot, product file, database or migration was edited by this owner. This pre-execution correction preceded the source-verification-r03 used for the accepted service runs; the original r01 remains NOT_RUN.

## Remaining work and limits

The owned service slice is accepted: both current PostgreSQL modes passed, raw inventories/resource preservation were independently checked, and the final source audit establishes continued service applicability. The added tests reproduced no product defect, and no product fix was made. The combined check still awaits the separately owned handler-race increment at this handoff; full native Trellis/product acceptance remains open.

This increment supplies no API/production Web/Desktop recovery acceptance. Full S3 R01–R09, the remaining TC-16 delivery assertions, 55-comment reconnect/backfill, mixed mentions/notifications, current-authority content recovery, structured blocking/repair and S4/S5 remain open. Parent24 remains2 PASS /11 PARTIAL /11 NOT_RUN until the coordinator updates only the supported subassertions.
