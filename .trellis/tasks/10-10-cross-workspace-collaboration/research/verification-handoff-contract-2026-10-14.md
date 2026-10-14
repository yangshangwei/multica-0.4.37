# D04 handoff confirmations and core edit transport

Date: 2026-10-14. This is bounded implementation evidence, not P1–P8 completion
or permission to enable collaboration creation/execution.

## Implemented contract

- `GET /api/collaboration/issues/{id}/status-options` now accepts
  `request_information`, `return_information`, and `submit_acceptance`. Each
  projects exactly the prescribed source status under current action authority.
- All three directed writes require the original `expected_status` and
  `status_options_version`. Return also requires its original
  `information_request_id`. Missing confirmation values are refused; no client
  method fetches fresh versions or retries automatically.
- The digest includes source identity, action, current/default status inputs,
  relevant catalog key/name/category/archive/update facts, and return request
  ID/requester/recipient/parent/saved status plus fallback selection. Renaming or
  recategorizing a relevant entry can invalidate a confirmation even when its
  resulting target key is unchanged.
- Writes recompute under the existing catalog fence and locked catalog rows in
  the workflow transaction. A mismatch is a durable `status_options_changed`
  refusal, accompanied only by current authorized options. Restoring the old
  catalog does not make that refused operation execute on replay.
- Archived current blocked/review keys cannot remain selected. Returns retain
  the stored requester and nested chain; removed/disabled return targets and a
  revoked creator acceptance target are not enrolled automatically.
- Ordinary transfers keep status and execution unchanged. The HTTP handler
  rejects supplied directed status fields, including explicit null; the core
  transfer method rejects supplied directed confirmation properties rather than
  silently ignoring them.
- Core `editCollaborationIssue` uses the existing root `PATCH`, four exact
  decimal version strings, and the original idempotency key. Its explicit
  fields are `title`, `description`, `startDate`, and `dueDate`. Omitted fields
  remain omitted; empty text stays empty; null/empty dates are sent as explicit
  clears; date-only and UTC-midnight RFC3339 write strings are preserved.
- Core detail parsing accepts strict optional/null date-only `start_date` and
  `due_date`. Missing older-server dates stay absent. Malformed present dates
  make the protected projection unavailable without logging payload values.
  Account/server generation checks, abort signals, redacted errors and the
  transport-option whitelist remain in use.

The older `return_status_changed` response is replaced by the common
`status_options_changed` contract for directed confirmation mismatches. This is
an unreleased foundation protocol repair; it deliberately does not accept an
older client's missing status digest.

## Files changed in this implementation lane

- `server/internal/service/collaboration_handoff.go`
- `server/internal/service/collaboration_lifecycle.go`
- `server/internal/service/collaboration_workflow.go` (catalog-action selection
  and receipt projection call only; preserved the editor's role/recovery changes)
- `server/internal/service/collaboration_handoff_test.go`
- `server/internal/service/collaboration_handoff_status_test.go`
- `server/internal/handler/collaboration_workflow.go`
- `server/internal/handler/collaboration_handoff_test.go`
- `server/cmd/server/collaboration_handoff_routes_test.go`
- `packages/core/collaboration/schema.ts`
- `packages/core/collaboration/index.ts`
- `packages/core/collaboration/lifecycle-schema.test.ts`
- `packages/core/collaboration/edit-schema.test.ts`
- `packages/core/api/client.ts`
- `packages/core/api/collaboration-workflow.test.ts`
- `packages/core/api/collaboration-lifecycle.test.ts`
- This evidence note.

The existing status-option generator, strict workflow decoder, durable operation
owner and core operation transport were extended. The redundant catalog-action
predicate was removed. No dependency, migration, generated SQL, router startup,
or provider execution change was needed for this lane.

## Verification

All database runs sourced `/tmp/multica-collab-20261014-yOusnI/review.env` and
asserted that `DATABASE_URL` names
`multica_collab_20261014_review_1791943635066`. No credential values were printed.
Go commands ran through `scripts/go-test-with-agent-cli-guard.sh`.

Passed:

```text
go test ./internal/service ./internal/handler ./cmd/server \
  -run '^TestCollaboration(Information|Acceptance|Directed|ReturnDigest|TransferRejects|HandoffRoutes|Lifecycle|Status)' \
  -count=1
pnpm --filter @multica/core test collaboration
pnpm --filter @multica/core typecheck
pnpm --filter @multica/core lint
go vet ./internal/service ./internal/handler ./cmd/server
go test -race ./internal/service \
  -run '^TestCollaboration(WorkflowInviteTransferPreservesExecution|WorkflowTransferRecordsIterationAndSupersedesInformation|InformationRequestRechecksCatalogAfterLockWait)$' \
  -count=1
git diff --check -- <the changed source/test paths above>
```

The core run passed 250 tests across 7 files. Focused Go suites passed in
service, handler and production-router packages. Their evidence covers nested
request restoration/identity, unchanged command/run/session state, rollback,
same-key catalog metadata/category edits, missing critical digests, refusal
replay, target revocation, and a deterministic catalog-lock wait. The router
test covers explicit action-options reads followed by all three directed writes.
The race-enabled run also passed the ordinary invitation/transfer execution and
iteration-history controls plus the new catalog-lock wait regression.

Core date-schema tests were observed failing before their implementation
(`handoff-core-red.log`). The initial Go regression could not compile until the
main session generated the concurrent reconciliation SQL; subsequent compilation
and all focused tests passed. That initial compile failure is not a behavioral
red-test claim.

Logs are in `/tmp/multica-collab-20261014-yOusnI/`:
`handoff-compile.log`, `handoff-go.log`, `handoff-core.log`,
`handoff-core-typecheck.log`, `handoff-core-lint.log`, `handoff-go-vet.log`, and
`handoff-go-race.log`.

## Limits and integration ownership

- The metadata editor owns the unified root PATCH handler and backend edit
  behavior; the main session owns router wiring and Go detail date projection.
  This note claims their core transport/schema coverage only.
- Full Web/Desktop E2E, native containment/provider proofs, and production
  deployment were not run. No real agent CLI or paid provider was invoked.
- The task still requires its remaining integration/acceptance work. Main-session
  review should update the shared collaboration foundation spec's older
  expected-status-only description to this D04 contract.
- Concurrent dirty changes were preserved; no commit or deployment was made.
