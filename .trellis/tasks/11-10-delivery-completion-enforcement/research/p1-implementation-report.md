# P1 implementation report — ordinary authorization core

Date: 2026-11-11. Native implementer: `implement_completion_ledger`.
Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`.

## Implemented boundary

- Added strict, closed target/consumer/request/binding types and canonical request/mutation digests. Duplicate, alias, missing, null, wrong-variant and malformed fields refuse; revision zero is legal. Explicit skip returns `delivery_skip_not_supported`.
- Added immutable authorization and one append-only terminal fact (`consumed` or `revoked`). Authorization/operation/request identities and terminal uniqueness use independent concurrent indexes. No FK or inline uniqueness was added. Original request and observations, issuer credential metadata and PAT hashes never appear in the public response.
- Added service-owned Create/GET/by-original-request-key/Revoke operations borrowing the caller's transaction. HTTP endpoints own strict parsing, current real credential capture, workspace-selector conflict refusal, commit/error handling and `no-store`; root owns actual router registration.
- Added `PrepareDeliveryMutationInTx` and `FinishDeliveryMutationInTx`. The prepared object is bound to its exact transaction. Original same-operation consumption recovers before fresh admission; no second state/notification/dispatch is performed. Fresh completion rechecks current policy, consumer, issuer, the single existing observer, exact binding and DB expiry. Finish checks actual stored state and revalidates evidence before inserting the terminal fact.
- Added CreateInTx/project-delete policy helpers. Public policy PUT still supports only off/observe; schema can represent enforce for internal package tests. This is not enabled global enforcement.
- Unbound ordinary edits that are not new entry into done do not invoke the artifact gate. An unchanged archived custom done key uses persisted category; actual new target keys resolve under the catalog fence. Current policy/schema errors on actual new completion never become off.
- Managed task consumers freeze the task's `ExecutionBindingID/Epoch` and current runtime daemon namespace, including explicit absence. Signing/consumption checks the current binding/installation/password/org fences; token binding must match the frozen fact. No latest-token inference or issuer-context impersonation exists.

## Concrete integration API

`DeliveryCompletionBinding` uses the frozen wire fields `authorization_id`, `operation_id`, `expected_issue_revision` (strings/string/int64). `DeliveryMutationIntent` contains server-derived Before/Candidate, Completion, actual Credential, RequestedPosition, AuxiliaryChanges, optional Collaboration and Metrics.

`PrepareDeliveryMutationInTx(ctx, tx, intent)` returns explicit `DeliveryNotApplicable`, `DeliveryReplayed` or `DeliveryPrepared` plus the appropriate receipt/prepared object. `FinishDeliveryMutationInTx(ctx, tx, prepared, after, result)` consumes in that same transaction. `RequestedPosition=nil` means no explicit override and permits existing SQL default new-column ordering; a supplied value must match the authorization exactly. Auxiliary content/attachment changes cannot be hidden behind a status request.

Policy helpers: `LockDeliveryProjectPolicy`, `CheckDeliveryCreateInTx`, `CheckDeliveryProjectDeleteInTx`. The P2 project-change owner owns its separate retained adjustment checker, reason and migration789–791; this package registered its exact790 concurrent index and retain-audit table classification by root assignment.

Handler exports: `CreateDeliveryCompletionAuthorization`, `GetDeliveryCompletionAuthorization`, `GetDeliveryCompletionAuthorizationByRequestKey`, `RevokeDeliveryCompletionAuthorization`. Same-package `writeDeliveryCompletionError(w, err, commitAttempted)` preserves uncertainty and emits code/error/message. Router binding was performed by the root, not this lane.

## Files

Owned source and generated bindings are enumerated with SHA-256 in `p1-implementation-source.json` (30 paths). Principal files are:

- `server/internal/service/delivery_completion_types.go`, `delivery_completion.go`, `delivery_completion_authorization.go` and strict-contract tests.
- `server/internal/handler/delivery_completion_authorization.go` and actual artifact/transaction tests.
- Migrations781–788, `pkg/db/queries/delivery_completion.sql`, generated completion bindings/models, `sqlc.yaml` private-field tags, migration runner index registrations.
- Focused actual migration and static migration tests; explicit history-retention entries in `workspace_delete_manifest_test.go`.

No P0 helper/auth file, existing ordinary/collaboration/VCS writer, client, task/spec or historical evidence was edited by this lane. Root separately fixed the P0 issuer-snapshot password-version helper after this lane found its ambient consumer-context dependency.

## Executed checks and retained failures

All run records/log references are under `research/runs`; none was overwritten. All database execution used exact fresh database permits from root; this lane never allocated, migrated, dropped, switched or borrowed a database.

| Record | Actual result and interpretation |
| --- | --- |
| p1-types-contract-red01 | Compile failure: test-first missing type/parser definitions. Not product behavioral red. |
| p1-types-contract-green01 | Compile failure: existing ValidateKey returns two values. Corrected without changing the test. |
| p1-types-contract-green02 / green03 | Two failing top-level behavior cases: valid zero-revision request refused. The first status correction alone was insufficient; the real remaining cause was reuse of iteration CanonicalHash's positive-revision normalization. Reused existing ValidateObjectJSON for strict raw syntax instead. |
| p1-types-contract-green04 | 18 named pass, 0 fail/skip. |
| p1-sqlc01 | sqlc failure: pg_constraint catalog needed explicit pg_catalog qualification. |
| p1-sqlc02 / sqlc03 | PASS; final run also generated the parallel project-change model. |
| p1-core-build01 / http-build01 / core-build02 | All backend build PASS. |
| p1-handler-compile01 | Test fixture compile failure: shadowed f.req.ResourceID; corrected to explicit artifactFixture.req. |
| p1-handler-compile02 / compile03 | Compile-only PASS; no TestMain execution. |
| p1-handler-iter01 | 11 named pass, 0 fail/skip, fresh enf_p1_handler_iter01. Genuine package/receipt fixture plus actual PostgreSQL state+terminal transaction tests; not middleware evidence. |
| p1-handler-iter02 | 22 named pass, 0 fail/skip, fresh enf_p1_handler_iter02: 14 P1 names plus 8 root explicit-issuer regression names. No sum with previous rounds. |
| p1-static-contracts01 | 19 named pass, 0 fail/skip (18 types + one migration contract). |
| p1-migrations-compile01 / compile02 | Compile-only PASS. |
| p1-migrate-iter01 | 6 pass / 1 fail: schema-retention fixture omitted explicit id on a table that deliberately has no generated default. This was fixture failure, not a broken product admission rule. |
| p1-migrate-iter02 | 7 named pass, 0 fail/skip, fresh enf_p1_migrate_iter02. Actual upgrade, empty rollback, authorization/terminal/enforce retention, concurrent down fences, malformed-index readiness and required-null shape rejection. |
| p1-final-vet01 | Failed on concurrently edited router test's unused net/http import and ordinary writer's unkeyed external error literals. Reported to those owners; not silently fixed outside ownership and not reported as passed. |

Lookup failures were also preserved in the conversation: guessed 780 filename, a read before sqlc had generated its file, guessed agent_runtime query and auth token glob, and guessed790 filename. Each was resolved by actual file discovery. They were read-only failures, not product red or permission incidents.

## Limits and follow-ups

This is an iterative implementation snapshot, not final P1/full-gate acceptance. Root still owns coherent frozen ordinary/race, fresh middleware tests, independent code review, final vet/format/sqlc idempotence, specs/source maps and safe integration. Changes after an earlier passing run cannot inherit that run as final proof; current source bindings explicitly record the latest bytes.

Only ordinary human/task signing/consumption/recovery is enabled by this core. The schema and strict union cover collaboration submission/self/admin and webhook consumer identity, but those paths **explicitly refuse unsupported** until P3 attaches real exact intent/source/credential adapters. No synthetic ordinary task or evidence was fabricated for collaboration. No successful usable skip is offered. Public enforce remains unavailable until all actual writers are integrated and accepted.

The managed binding code now checks real task/runtime snapshots, but no new complete managed-installation positive router fixture was run by this lane; later verification must prove that path. End-to-end router credential assertions are owned by the separate router lane.

One internal API precision consideration remains for review: public HTTP parsing canonicalizes expiry to UTC microseconds; direct typed service callers must supply canonical request values or the service should normalize/reject noncanonical precision before persistence. This cannot bypass a gate (subsequent integrity comparison refuses), but could make a noncanonical internally constructed grant unrecoverable. Do not claim that path verified.

No external messages, pushes, deployment or supplier/model calls. No commit or task archive. M1 and full delivery-gate completion remain false.
