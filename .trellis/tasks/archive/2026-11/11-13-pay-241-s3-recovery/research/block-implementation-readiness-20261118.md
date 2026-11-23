# S3 blocking/repair implementation readiness — 2026-11-18

The approved v2/r05 contract can be implemented in the existing workflow transaction. No new product decision is open. This report prepares the next single-server-owner handoff; it does not implement the contract, run a product test, allocate a database, or accept S3.

Owner: `/root/s3_block_implement`, already the native `trellis-implement`. Main continues the current-client contention/batch increment and owns environment, SQL execution, source integration and commits. The current client source and all historical accepted/failed sources remain untouched. Machine-readable file bindings, migration/tool evidence, proposed tests and dispositions are in the sibling JSON.

## Actual source gap

- `collaboration_handoff.go` has reason/recipient and the original request/return chain, but no `unblock_condition`, ETA, complete/legacy/unavailable block projection, current information actionability or repair.
- `collaboration_workflow.go` already owns sorted user locks, current account/participant checks, source/catalog/I1/issue/session locks, original operation recovery, required event/notification writes and one commit. `handoff` preserves `PreviousStatus=before.Status` and truthful from/to; `supersedeInformation` currently returns no affected count/link.
- The human `POST .../handoffs` parser accepts only `transfer|information`. It already rejects duplicate/case-alias/trailing JSON through `iteration.ValidateObjectJSON` and uses raw status fields. The new repair and metadata presence rules belong in this boundary, before Go pointer conversion.
- The three new human-history GET paths have no current route collision. Current `collaboration_handoff.sql` and `collaboration_runtime_handoff.go` serve a separate runtime/run protocol, so those readers must remain independent of the new human DTOs.
- `CreationOptions` is already RR/READ ONLY and knows only source/parent search. `IssueService.CreateInTx` derives the effective project from a validated parent before the shared project fence and before duplicate/counter/issue effects. The new blocked-root guard must use that derived project.
- **Raw pointer distinction:** `GetCollaborationAccess` selects the joined open `information.id`, not `issue_collaboration.active_information_request_id`. A dangling/non-open pointer therefore becomes an invalid `InformationRequestID`. New information state/history/repair context must read the actual pointer from `issue_collaboration` separately in the same snapshot/transaction. Preserve the old joined field and its directed capability/runtime semantics; do not relabel a dangling pointer as `record_state=none`.
- The ROOT production paths inspected here match accepted R04 bytes/modes. `collaboration_workflow.go` nevertheless has a preexisting S2 change versus Git HEAD: `GetWorkflowOperation` uses `LockArtifactUsers` instead of `LockCollaborationUsers`. Preserve that exact line and all other S2 WIP during S3 integration.

## Smallest coherent server increment

Implement the following together before publishing the new client DTO. Separating metadata writes from repair/current actionability would admit complete requests that can still have no legal return path.

1. Add the three nullable handoff columns and exact readiness/retention guard. No new table, recovery ledger, foreign key, metadata backfill or scheduler.
2. Add one narrow information-state owner that classifies stored block metadata, captures the raw pointer/current policy/source/actor facts, validates normal nested exit/pending-safe return, and produces current actionability/context. It reads current policy with the existing shared project fence in writers and plain reads in RR readers; it never ensures a stage task or consumes completion approvals.
3. Extend the existing handoff mutation/strict transport with both optional metadata and one `purpose=information_repair` variant, service operation kind `repair_information`. Keep legitimate omitted legacy fields omitted in the digest. New semantic requirements/final ETA-clock checks run only after original-operation recovery.
4. Implement repair in the existing workflow transaction: current Transfer+ChangeStatus, managed/open/blocked state, explicit target and exact context/basis; one truthful transfer handoff; real status/handler change; exact open→superseded relation/count; pointer clear; event/notification/iteration/receipt writes. Self-target records equal from/to. No implicit B2, return, Resume, command or stage/approval consumption.
5. Add current root/block and repair status-options/receipt projections; guard managed generic blocked transfer/status bypasses, preserve independent security removal/cancel/R38 paths and all unmanaged behavior. Pending submission always requires its exact submission/event/current eligible creator/in_review exit. Normal return records its actual restoration/fallback facts.
6. Add the project-aware creation read observation and the internal root-only guard at the real effective-project creation boundary, retaining the existing creation savepoint/refusal owner and original keys.
7. Add the three protected, bounded human-history reads, exact provenance/affected denominator and the history-only current-key HMAC cursor. Add only safe repair activity references and the approved missing marker to newly built runtime-v1 envelopes/snapshots.
8. Generate only from the separate work source with the already retained canonical sqlc. Freeze the public DTO/query plus exact source delta, compile/static-check, then run the allocated owning-layer normal/race and actual current-client cases. Every failure retains its source/database/raw result.

## Proposed production write list

| File | Necessary change |
| --- | --- |
| `server/internal/service/collaboration_information.go` (new) | Metadata validation/classification, current raw-pointer/policy/chain context and normal request/return guard; no parallel transaction owner |
| `server/internal/service/collaboration_information_repair.go` (new) | Closed repair basis/options/confirmation and one callback through existing workflow owner |
| `server/internal/service/collaboration_information_history.go` (new) | Human history/detail/affected DTO, exact tuple paging/provenance/budget and narrow HMAC cursor |
| `server/internal/service/collaboration_handoff.go` | Optional metadata params/projection, new managed request/return guards and original return transaction facts |
| `server/internal/service/collaboration_workflow.go` | Optional handoff insertion values, exact repair supersession/count, repair receipt/GET/replay allowlists; retain the S2 read-lock line |
| `server/internal/service/collaboration_lifecycle.go` | Explicit repair action/options variant with empty default only there; managed blocked generic-status protection |
| `server/internal/service/collaboration.go` | Only the internal collaboration-root create distinction passed to the existing borrowed issue create |
| `server/internal/service/issue.go` | Unexported root-only create distinction/guard after effective-project resolution and project fence, before counter/effects; ordinary children unchanged |
| `server/internal/service/collaboration_creation_options.go` | Selected project read, closed six-field policy observation and actual managed blocked-category filtering; reuse existing creation fields |
| `server/internal/service/collaboration_activity.go` | Explicit repair activity allowlist and safe ID/actor/time/status/basis projection; no note/condition/ETA in generic repair metadata |
| `server/internal/service/collaboration_runtime_handoff.go` | Add only `information_block_metadata_unavailable_v1` for newly built envelopes with an information request |
| `server/internal/handler/collaboration_workflow.go` | Strict raw metadata/repair-purpose parsing and response/own-GET confirmation; no alternate mutation route |
| `server/internal/handler/collaboration_information.go` (new) | Human history GET adapters, authority-before-query decoding, strict limit/cursor/detail rules and one five-second read deadline |
| `server/internal/handler/collaboration.go` | Additive optional `block` and `information_workflow` projections from the same authorized detail snapshot |
| `server/internal/handler/collaboration_creation.go` | Strict optional `project_id` combination/closed query handling and source/project correlation |
| `server/cmd/server/router.go` | Register only the three new human GETs beside existing POST handoffs |
| `server/pkg/db/queries/collaboration_workflow.sql` | Optional insert values and new exact counted repair supersession query; keep ordinary/security supersession unchanged |
| `server/pkg/db/queries/collaboration_information.sql` (new) | Raw current pointer/context and scoped human history/provenance/affected keysets, exact original event lookup, counts/readiness |
| Generated counterparts | `models.go`, changed handoff scanners plus the new information query output; accept only actual generator deltas |
| Two coordinator-reserved migration pairs | Metadata columns and highest-numbered readiness/retention guard below |

The generated output currently contains handoff references in `collaboration.sql.go`, `collaboration_activity.sql.go`, `collaboration_delivery.sql.go`, `collaboration_handoff.sql.go`, `collaboration_knowledge_delivery.sql.go`, `collaboration_stage_submission.sql.go`, `collaboration_workflow.sql.go` and `models.go`. Regeneration can update scanners/embedded row structs in this set, but presence of a reference does not mean every file must change. No existing generated file may be wholesale restored over another task's WIP. The exact output list is recorded after generation.

`collaboration_handoff_snapshot.go`, `server/pkg/protocol/collaboration_handoff.go`, runtime-v1 sources/decoders, S2 completion/signature helpers, R38 implementation, unrelated workflow/test files, dependencies and global configuration remain read-only unless a concrete newly retained failure requires a main-approved scope addition.

## Migration and generator readiness

At this read-only capture, the highest source prefix is **865** (`865_knowledge_automation_origin`); 866+ are unused. Main must recheck and reserve the actual numbers before writing. Recommend **two new migrations / four files**, provisionally:

| Provisional prefix | Up | Down |
| --- | --- | --- |
| 866 `collaboration_information_metadata` | Idempotently add nullable `unblock_condition text`, `estimated_resolution_at timestamptz`, `superseded_by_repair_handoff_id uuid` to `issue_handoff` | Drop these supporting columns only after the higher retention gate has safely completed |
| 867 `collaboration_information_ready` | Verify actual nullable column types and required current schema; do not backfill legacy rows | Highest new downgrade guard takes the required table fences and refuses while any new metadata, repair relation or `information_repaired` event is retained; must run before 866 |

Existing `589_issue_handoff_history` already indexes `(issue_id,created_at,id)` and supports both scoped tuple reads. A new index is not required for correctness of the bounded initial increment. If actual query/volume evidence later justifies an affected-repair index, main reserves an additional single-statement `CREATE INDEX CONCURRENTLY` migration before the final guard; no inline/unique table index or FK is introduced.

Canonical generator configuration is `server/sqlc.yaml`: PostgreSQL migrations + `pkg/db/queries`, pgx/v5, JSON tags and empty slices, with existing private-column overrides. `Makefile` pins sqlc **v1.31.1**. It is absent from ambient PATH, but the following retained executable exists with mode0755 and its actual bytes match the R33 generator receipt:

`/Volumes/artisan/.multica-acceptance/pay241-r31-idle-claim-20261118-bz7ym_r1/generation-tool-r02/sqlc`

SHA256 `a3da067519c3fbe11be1c96ac019d7f44527fb1e3aaa26c2644c66433e28a8a6`. Its retained build result binds the cached `github.com/sqlc-dev/sqlc@v1.31.1` module and exit0; R33's subsequent `generate` receipt binds the same hash. This turn did **not** invoke it, use versioned `go run`, install dependencies, or assume the ambient shell has sqlc. Main can copy/hash-bind that exact executable into the separate work environment.

## Lock and recovery contracts to preserve

- Follow `workflowInTx`'s current sorted user/account set, source workspace, catalog/I1/iteration, issue/collaboration/session/channel/target fences. Re-read the user set under the issue lock and restart the entire transaction on a changed set. Resolve new target eligibility using the already locked users.
- Managed applicability uses current project association/policy, even with no `collaboration_stage_task`. Borrow `LockProjectForAssociationNowait` + supported policy/schema reads under the workflow root locks, matching accepted `loadStage`/`stageCompletionState` ordering. `prepareStageMutation` is an approval-consumption owner and is **not** an applicability helper for information repair.
- RR/READ ONLY creation/current/history reads use plain supported policy/project reads; do not call a row-locking writer helper from them. Source-only authority fingerprint includes the real member row ID/created-at and nullable participant/account epochs. Verified credential generation stays private.
- Add repair to the existing directed replay class and own GET allowlist. The existing current-account check remains; a prior success/refusal is not reinterpreted through the new managed/blocked/ETA requirements. Nil optional metadata keeps old `json:",omitempty"` hashes.
- Validate the final persistable UTC microsecond ETA against the final DB clock for a **new** write; retain the original timezone string in its request hash. Reject sub-microsecond precision. An old ETA becoming past neither schedules work nor changes a receipt.
- Preserve exact pending delivery/submission/attachment/stage/M1 facts. A legal normal nested blocked→blocked return reactivates its exact complete parent; a pending nonblocked exit cannot fall back into ordinary work or another acceptance handler.
- Repair updates only genuinely open affected information rows. Already returned/superseded rows and links remain unchanged; security-removal supersession is never attributed to a later repair. Count with signed-64-bit precision and retain every affected ID, including legacy/unavailable provenance.

## Public DTO/query freeze for the client owner

The approved v2/r05 wire is the freeze source; this report does not rename fields:

- New human `block` is the strict `protocol_version:1` complete/legacy/unavailable union. The containing handoff retains original note/recipient/actor/time and IDs. Root `information_workflow` has exact raw `active_request_id`, managed/context/version/record state and closed request/return/repair availability+code entries.
- Repair POST uses `purpose:"information_repair"`, explicit target/note/status/catalog digest, all four ordinary workflow observations and required repair context. Active request is present UUID-or-null. Basis is exactly human_selected, historical_information(request+event), or pending_submission(submission+event); unexpected/partial/null branch fields refuse.
- Repair status-options adds the exact `information_repair_context`; only this valid variant has `default_status:""`. Old nonempty/in-options defaults stay strict. Success/own GET add only content-free `information_repair_confirmation` with exact handoff/pointer/status/handler/superseded count.
- Creation GET observation is the six-field not_selected/known union, selected-project Query-correlated, virtual policy revision0/off supported. Observation never enters creation POST/hash. Effective managed blocked creation uses stored `status_unavailable` refusal.
- Human history is newest-first actual handoff `(created_at,id)` with upper tuple and changed revision/lifecycle/authority rejection. Request/return/repair have exact closed provenance; legacy event IDs remain null and rows remain in the denominator. Affected detail count is canonical nonnegative int64 decimal, including0; no page-length count.
- Every complete encoded history/detail envelope plus escaping/newline is ≤65,536 bytes, fixed limit1..50/cursor≤2048, whole read≤5s. Reserve the real next cursor before selecting the prefix; a first oversized full item is503, never an omitted row or false completion.
- History cursor is narrow v1 HMAC-SHA256/base64url with the approved domain, actual validated cached `auth.JWTSecret()` and constant-time verification. No new secret/token/capability. Invalid syntax/MAC is400, valid stale binding409, unavailable key/schema503. A key rotation requires an actual new process.
- Human history is the only new structured runtime-related reader. Runtime v1 typed fields/source kinds/note bytes remain unchanged. Recovered old snapshots receive no new marker or fields; new envelopes/snapshots may add only the existing-array missing code named above.

## Meaningful owning-layer test inventory

Proposed new files and names below are planning identities; **zero have run or been authored by this readiness turn**. Table subcases retain independent run/terminal identities when implemented. Existing ordinary/unmanaged/handoff tests are reused as compatibility regressions, not copied into DOM suites.

| Canonical owning suite | Proposed top tests / V2 coverage |
| --- | --- |
| `server/internal/service/collaboration_information_test.go` | `TestCollaborationInformationManagedMetadataAndLegacyRecovery` (14); `TestCollaborationInformationCompleteNestedReturns` (6); `TestCollaborationInformationCurrentActionabilityAndRawPointer` (13) |
| `server/internal/service/collaboration_information_repair_test.go` | `TestCollaborationInformationRepairRestoresReturnableContext` (A/B/C/D/E,1–5); `TestCollaborationInformationRepairPreservesPendingSubmission` (G/H,7–8); `TestCollaborationInformationRepairOriginalKeysAndUnknownCommit` (9–10); `TestCollaborationInformationRepairRequiredWritesRollback` (11); `TestCollaborationInformationRepairAuthorityCommitOrders` (12); `TestCollaborationInformationRepairPreservesExecution` (18) |
| `server/internal/service/collaboration_information_history_test.go` | `TestCollaborationInformationHistoryExactProvenanceAndAffectedPaging` (13/15;0 and51+ with legacy rows); `TestCollaborationInformationHistoryCursorBindingAndBudget` (13/15; read-only authority, actual encoded envelope/continuity/HMAC/current key) |
| `server/internal/service/collaboration_information_creation_test.go` | `TestCollaborationInformationCreationEffectiveProjectAndOriginalKeys` (17; no/off/on/parent inheritance/policy commit orders/refusal replay) |
| `server/internal/service/collaboration_information_runtime_test.go` | `TestCollaborationInformationRuntimeV1SourcesAndSavedSnapshots` (16; exact existing raw bytes and new-marker boundary) |
| `server/internal/handler/collaboration_information_test.go` | `TestCollaborationInformationMetadataWirePresence`; `TestCollaborationInformationRepairWireClosedBasis`; `TestCollaborationInformationHistoryWireAuthorityAndQueries`; `TestCollaborationInformationCreationWireProjectObservation` (14/13/17; actual HTTP body/query/serializer boundaries, no duplicated service transaction matrix) |
| `server/cmd/server/collaboration_information_routes_test.go` | `TestCollaborationInformationRoutesUseCurrentPasswordPrincipal` (real router/CSRF/machine/foreign identity + new GET wiring; ordinary underlying transaction matrices stay service-owned) |
| `server/cmd/migrate/collaboration_information_recovery_test.go` | `TestCollaborationInformationMetadataMigrationAndRetention` (15; actual new-schema shape and retained facts refusing down before supporting column removal) |
| Existing protocol and compatibility suites | Existing handoff request/return, pending-delivery and source/parent/ordinary creation tests; strict protocol decoder and saved-source tests remain unchanged, with exact named inventory declared in the final execution permit |

Reuse `newCollaborationHandoffFixture`, `workflowRequest`, `newManagedCollaborationFixture`, `lifecycleCommitBarrierStarter`, `waitWorkflowBlocked`, the existing fault/unknown-commit starters, `handoffBoundaryExecutionSnapshot`, and `testutil` fixtures/Call/Want/JSON. Do not substitute sleeps or stage-policy stubs for actual commit/PID waits. The live execution snapshot excludes only legitimate session event sequence; retain targets/grants/commands/inputs/groups/runs/attempts/proof/capability/reservation facts and stage/M1 approvals.

The required-write test must enumerate actual reached writes: enroll/account fence/invalidation where needed; status/handler; iteration fact; handoff insert; exact supersession link/count; pointer clear; workflow/event sequence; event/notification; original operation completion. Each injected failure proves complete rollback; unknown commit is a separately retained case, not proof of no write.

Main should compile without executing TestMain first, then provide fresh package/mode databases. Intended command shapes:

```text
go test -c -o <owned-output>/service.test ./internal/service
go test -c -o <owned-output>/handler.test ./internal/handler
go test -c -o <owned-output>/router.test ./cmd/server
go vet ./internal/service ./internal/handler ./cmd/server ./cmd/migrate
go test -json -count=1 -run '^TestCollaborationInformation' ./internal/service
go test -json -race -count=1 -run '^TestCollaborationInformation' ./internal/service
go test -json -count=1 -run '^TestCollaborationInformation' ./internal/handler
go test -json -race -count=1 -run '^TestCollaborationInformation' ./internal/handler
go test -json -count=1 -run '^TestCollaborationInformation' ./cmd/server
go test -json -race -count=1 -run '^TestCollaborationInformation' ./cmd/server
go test -json -count=1 -run '^TestCollaborationInformation' ./cmd/migrate
```

These are future shapes, not execution authorization or results. Exact names/patterns, modes and command counts must be sealed after authoring. Every DB-backed package (including filtered TestMain) gets its assigned fresh database; browser API uses another. Pure cursor/protocol tests may use main's explicit no-DB permit. Required new Web/Desktop A/B/C/E/G and lost-ACK/partial progress cases follow the frozen Core/View contract, current API/binary/config/build identities and independent check.

## Current verification and remaining handoff

Read-only source/contract/tool binding checks passed: approved references exist, twenty selected production paths match R04 bytes/modes, the S2 workflow line is present, migration prefixes/types/index and exact retained generator bytes were inspected. Product lint/typecheck/tests, generator, DB/API/browser/Provider execution: **NOT_RUN / 0**. No runtime/source/product/global spec/task-matrix/status/commit edit occurred; only this report and its sibling JSON were written.

Main now owns the actual 866+ reservation, separate server-work source seal and exact write/test permits. The server owner can then implement the coherent delta directly and freeze the DTO/query for the shared client owner. Full S3, current client evidence, real S4/S5, runtime structured-v2 metadata and installed production Desktop Main remain outside this readiness conclusion.
