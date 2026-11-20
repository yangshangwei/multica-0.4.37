# Ordinary comment notification GREEN independent check — 2026-11-19

**Accepted for the bounded ordinary `new_comment` delivery increment. Blocking code findings: 0.** The final normal and race rounds independently contain the same complete 35-top / 57-name PASS set. The corrected tests reproduce the original duplicate-delivery defect against the unchanged producer, then pass against exactly three product changes. This accepts persisted comment/recipient deduplication and its ordinary recipient controls; Web/Desktop subscription delivery, controlled runtime results and full S3 remain separate.

- Reviewer: `/root/notification_runtime_check`, already-dispatched native `trellis-check`; no child dispatch.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Private evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly` (`B` below).
- Final product/test source: `B/source/server-green-r02`; counterfactual: `B/source/server-red-r02`.
- Owned writes: this report, its evidence map and the reviewer's existing runtime-preflight report. No product, test, configuration, specification, task metadata or Git state was changed by this reviewer.
- Main executed the allocated commands and database audits. The reviewer inspected the retained raw records and current source, and ran only read-only `gofmt -l`. No DB/API/browser/Provider body, service control, package install or suite rerun was performed here.

## Findings (fixed)

### ON-F01 — the test observer's restored default logger could deadlock later tests

- File: `cmd/server/notification_comment_replay_test.go:356`–`:372`.
- Original issue: the old observer wrapped `previous.Handler()` and restored only `slog.Default`. `slog.SetDefault` also changed the standard `log` writer. Restoring the previous default handler did not restore that writer, so a later unchanged notification log reentered the observer/default-handler bridge.
- Actual original evidence: `verification/notification-normal-r01` failed at the retained default 10-minute package timeout in `TestNotification_StatusChange_ArchivesStaleTaskFailed`. Its raw stack includes standard `log` output, `slog.defaultHandler.Handle`, the copied test handler and `archiveStaleTaskFailed`. It has 46 completed PASS names, one unfinished name and ten unstarted names from the final 57-name selection. It is **NOT_ACCEPTED**, even though all 29 new replay names had passed before the hang.
- The exact copied helper/default-restoration probe at `logging-probe-red-r01` reports `RESTORED_LOGGING_BLOCKED` and exits 1. The corrected probe at `logging-probe-green-r02` exits 0, observes the error control and emits both restored slog and standard-log messages.
- Owner fix: use an independent `slog.NewTextHandler(io.Discard, nil)` sink while retaining the error collector; save/restore the previous slog default, standard-log writer and flags. The later-registered cleanup releases and joins both publishers before restoring logger state. The observer still enables and records error-level failures/panics; the fix does not silence the assertion that rejects swallowed writer errors.
- Verification: final RED and GREEN use the same corrected 862-line test, SHA-256 `e157ac71ab1eac61315c932d8351750d4ceaa88d218971eeb613723a626db001`. The full selected normal/race rounds finish with the original legacy archive tests passing. This reviewer verified the owner's fix; it was not applied during this review.

### ON-F02 — runtime withdrawal closure evidence was overwritten between subcases

The separately owned `source/client-harness-r02` fix is statically accepted in [the updated runtime preflight](notification-runtime-preflight-check-20261119.md#r02-retention-follow-up--same-independent-reviewer). It awaits each `withSessions` cleanup and deep-clones a per-subcase checkpoint in `finally`, preserving success/failure/zero-client records before the next scope. The global recovery helper is unchanged. This closes NR-P01 for R02 source only; actual runtime acceptance and NR-P02 provenance reconciliation remain open.

## Findings (not fixed)

No blocking production, query, generated-code or test-validity issue was found in the final ordinary increment. No design or public-interface change was made by this reviewer.

The deliberate limits remain:

- Deduplication requires a valid nonzero original comment UUID and applies to this `new_comment` member delivery tuple under current fixed writers. Empty, invalid or zero synthetic IDs retain random-v7 behavior.
- Other generic events and ordinary direct `mentioned` notices retain their existing writers. This is not a bus-wide event identity or replay guarantee.
- A previously committed matching legacy random-ID notice prevents a new notice. A concurrently running old random-ID server is not fenced by the new stable ID; mixed-version writers need separate scope and review.
- Existing duplicate history is preserved. The increment neither deletes duplicates nor updates/reopens old read or archived records.
- Ordinary delivery still resolves current active subscribers. A late subscriber can receive an old re-emitted comment once; collaboration terminal-result frozen recipients are a separate contract.
- A crash after insert but before `inbox:new` is not given a new durable outbox by this change. API-keyed comment recovery, composer behavior, eventual delivery and actual client reauthentication remain separate claims.
- The selected package union is 35 tops; it is not the complete server package/repository gate. TC-22 gains only the server-side subassertions proved here. Parent TC-22 and S3 are not promoted to full PASS by this report.

After acceptance, Main owns the server-spec update for the stable-ID exception, retained legacy rows, current-recipient rule and these limits. Current task design/intake already describe the same contract; this reviewer was explicitly excluded from specification edits.

## Source and generated-output authenticity

The two full 5,078-entry RED/GREEN source manifests differ in exactly these three existing product files. Their modes remain `0755`; no path addition/removal or other byte/mode change exists between the pair. The new test is present and byte-identical in both sources.

| Source file | Final SHA-256 | Change |
| --- | --- | --- |
| `cmd/server/notification_listeners.go` | `8308a58cfcb9217da8cf313b112b1c434e1245b3d2e2a2af05e48e8652dbab42` | Select stable persisted ID only for a valid original `new_comment`; skip broadcast when no row was inserted |
| `pkg/db/queries/inbox.sql` | `44c0fefd774151a341cef5f024e9b9e79ab8d1f1fdb27d41f66a66ddbe6b3bfe` | Add dedicated retained-row/PK-conflict insertion |
| `pkg/db/generated/inbox.sql.go` | `47a619be9ae342496caf6f8abcdf79b21fdab354f134996a055c7bdb634fafd5` | Canonical sqlc query, parameters and scanner |
| `cmd/server/notification_comment_replay_test.go` | `e157ac71ab1eac61315c932d8351750d4ceaa88d218971eeb613723a626db001` | Seven tops / 22 leaves; corrected observer shared by RED and GREEN |

`verification/notification-sqlc-generate-r01` ran the explicit owned sqlc executable against `source/sqlc-work-r01` and exited 0. Its actual before/after delta is only `pkg/db/generated/inbox.sql.go`. The canonical install receipt identifies the same generated SHA used by final GREEN. The other 143 generator-owned Go outputs remain unchanged; the four handwritten row/test files in that directory also remain unchanged. All 147 other directory entries and every other source path are stable. No fresh generation was substituted for an unavailable original receipt.

The final normal, race, vet and build receipts record no source/config/tool drift. Current recorded files were independently compared to their manifests, including the final test and canonical generated file. `go.mod`, `go.sum`, migrations, subscriber writers, event bus, other notification producers and frontend product source are unchanged. There is no new table, foreign key, index or dependency.

## Actual producer/query behavior

The existing `comment:created` adapter reads the real comment UUID from the typed HTTP response or map payload, places it in `details.comment_id`, and keeps the system-comment guard (`notification_listeners.go:902`–`:958`). Subscriber/mute/delegated/actor filtering still happens before the insertion (`:373`–`:432`).

At `:380`–`:393`, only `notifType == "new_comment"` with successfully parsed, nonzero UUID details enters the durable path. `commentInboxDeliveryID` (`:493`–`:498`) uses that original UUID as namespace and binds a versioned domain plus fixed-width source workspace, target issue and member-recipient UUIDs. It is a stable UUIDv5 delivery key using the already-declared UUID dependency. Case-equivalent UUIDs resolve to the same identity. Generic inserts retain their `dbid.NewV7()` path and database fallback.

`CreateCommentInboxItem` (`inbox.sql:100`) uses a parameterized `NOT EXISTS` comparison across workspace, issue, member recipient, `new_comment` and canonical lowercased comment UUID text. It includes all read/archived history and does not cast untrusted historical JSON to UUID. It neither overwrites nor deletes an existing row. `ON CONFLICT (id) DO NOTHING RETURNING *` lets competing fixed writers converge even if both initial snapshots see no row. The generated parameter ordering/scanner matches the canonical query.

`pgx.ErrNoRows` means this writer inserted nothing and skips `inbox:new` (`notification_listeners.go:450`). Every other insert error retains the existing error log. A returned row is the only path that publishes the new notice (`:475`–`:484`). Thus a new body/title under an old comment ID cannot mutate its first delivery, while a new original comment or recipient obtains its own delivery.

The change preserves the ordinary recipient semantics rather than importing collaboration result freezing. That difference is explicitly exercised: unsubscribe the original watcher, add a late watcher, replay the original event and retain one notice for each historical/current recipient; a distinct event after both unsubscribe delivers neither.

## Test validity and retained RED

The new fixture uses `internal/testutil` rows, reads the saved comment and issue through generated queries, and republishes the same genuine event through the actual notification listener. It does not create equal-text HTTP comments as a substitute for event replay or register unrelated subscriber-mutation listeners in its concurrency test. Broadcast assertions compare actual returned IDs and complete recipient/source/type tuples to persisted rows.

| New top | Leaves | Tested behavior |
| --- | --- | --- |
| `TestNotificationCommentReplaySequential` | 4 | Identical event, fresh bus, UUID case equivalence and actual map payload |
| `TestNotificationCommentReplayConcurrent` | 2 | Shared and independent buses; two genuine publishers held at the real insert boundary |
| `TestNotificationCommentReplayRetainedState` | 3 | Read, archived and both; exact original row survives saved content/title changes and replay |
| `TestNotificationCommentReplayLegacyRows` | 4 | Original v4/v7 IDs, uppercase history and retained existing duplicates |
| `TestNotificationCommentReplayDistinctTuples` | 3 | Distinct comments/recipients/workspaces; unrelated retained tuple fields cannot suppress a valid delivery |
| `TestNotificationCommentReplayRecipientControls` | 2 | Actual actor, Agent, mute, delegated, nonwatcher, parent-only and unsubscribe controls; current late subscriber |
| `TestNotificationCommentReplayLegacyFallback` | 4 | Empty/invalid/zero identities and unchanged generic status UUIDv7 writes |

The concurrency seam forwards the original SQL, arguments and generated scanner. Its first insert blocks each publisher, so both reach the write boundary before release. Cleanup releases the barrier and joins publishers even on failure. The collector is mutex-protected, no publisher calls `testing.T`, and the error observer rejects logged SQL failures or recovered listener panics. This is genuine concurrent DB-backed delivery rather than a fabricated successful response.

The fixed-observer RED round `notification-red-r02` uses the unchanged producer/query/generated source and the exact final test. Its 35 tops / 57 names contain **34 PASS / 23 FAIL**, with six new top failures and 17 failing new leaves; all 28 legacy tops pass. Raw failures show two original recipients acquiring four retained rows/four broadcasts on the same event, retained history duplicates, and wrong counts after fresh-bus/concurrent replay. These are the target product failures, not a setup-only RED.

`notification-red-r01` remains preserved: seven new tops / 29 names, 6 PASS / 23 FAIL. The original `notification-normal-r01` timeout, original sources/configuration, log probes and failed database remain distinct. No test threshold was increased and no original row history was erased to obtain final GREEN.

## Final raw inventory and database reconciliation

Main's final commands, each `-count=1` with explicit allocated configuration, are:

```text
go test -json -mod=readonly -count=1 -run '^Test(Notification|Subscriber)' ./cmd/server
go test -json -race -mod=readonly -count=1 -run '^Test(Notification|Subscriber)' ./cmd/server
```

The reviewer parsed every raw JSON event, matched the expected package and all run/terminal identities, and compared both complete name sets. Each has 35 tops / 57 names: seven new tops / 22 new leaves plus 28 legacy tops. Every name has one run and one PASS terminal; failure, skip, malformed JSON, duplicate, unfinished and race-diagnostic counts are zero. Both package terminal events are PASS and both commands exit 0.

| Retained round | Raw stdout SHA-256 | Result |
| --- | --- | --- |
| `notification-red-r02` | `3e47117edbcc58dc800a4e6bd0a234d425f938ceb6bac38da270f64be193ed44` | Genuine target RED, retained |
| `notification-normal-r01` | `e472f87981a32f77c67be32f410ec7d4d6c62c1d7c2bb876fb943ff418611637` | Timeout/incomplete, retained and not accepted |
| `notification-normal-r02` | `c381e722b3c42c504acbfaa306efa63c89e8fcef5067d85895dffcde8327f0fa` | 35 tops / 57 names PASS |
| `notification-race-r02` | `7bf7e23d3e397397e8d4787931bae8b1bd6074e2a037c6121668475fe94ae91a` | Same 35 / 57 PASS set, real `-race` command |

Final normal/race command configuration hashes match the new `database-fixed-logger-allocation-r03.json` profiles, their actual private files and the admitted/after database names. They use distinct fresh `normal_r03` and `race_r03` databases; the older failed `normal_r02` database is retained.

The admitted and final Main audits reconcile all 211 table keys, the exact 855-entry migration ledger, and all 663 ready/valid index definitions. Gate state is identical: epoch 2, creation true, execution false, host false, no freeze. User/workspace/member/issue/comment/inbox/subscriber/task-queue and other transient test counts return to zero, with final `other_connections=0`.

There is one explicit expected count delta: `collaboration_config_invalidation` is **0 at admission → 3 retained after the package** in RED, normal and race. These are retained fixture-produced history; they are not unchanged pre-existing counts. No cleanup was performed by this reviewer, and count-only audits do not prove unrecorded row-content hash equality. The failed original normal database still contains its actual fixture/history residue rather than a fabricated zero cleanup result.

`audit/notification-normal-race-reconciliation-r02.json` and both final database audits agree with these raw inventories and boundaries. This is independent reconciliation of Main's retained audit evidence; the reviewer did not establish those facts through a new database connection or alter shared cluster state.

## Verification

- **Go formatting: PASS, independently run read-only** on the producer, canonical generated file and corrected new test with `/opt/homebrew/bin/gofmt -l`; no output or source mutation.
- **Lint/static analysis: Main's `notification-vet-r02` exit-0 receipt inspected**, empty raw stdout/stderr and no recorded source/config/tool drift. Four-file R02 client ESLint also has zero errors/warnings/suppressed messages.
- **Type/compile: Main's API, daemon-test-binary and CLI R02 build receipts inspected: all exit 0**, bound to final GREEN; daemon `go test -c` executes no TestMain/body. Strict four-file client type-check also exits 0. These build checks were not rerun by this reviewer.
- **Tests: final Main normal/race PASS independently reconciled**, each 35 tops / 57 names; same exact complete set and zero failures/skips/incomplete identities. Original RED and timeout/probe failures remain retained.
- **sqlc: canonical actual generation/install receipts reconciled**; only the intended generated output changes.
- **Runtime/client acceptance: NOT_RUN in this review.** R02 lists exactly four runtime plus two ordinary subscription tops; definition/static success is not browser, result notification, original-token withdrawal, PID/proof/ledger/gate closure or full S3 acceptance.
- Fixed findings: owner observer fix ON-F01 verified; R02 NR-P01 retention fix verified separately. Blocking ordinary product/test findings remaining: **0**. NR-P02 and actual Web/Desktop/runtime acceptance remain open.
