# S3 block fixture cleanup and URL context repair — 2026-11-18

The R02 service normal/race rounds remain **NOT ACCEPTED** despite all 43 named
functional results passing in each mode. Both retain five `issue_handoff` rows
after the owning issues are removed. The separately retained handler round has
three named tests, two PASS and one FAIL: its history read returned400
`invalid issue_id`. These are reproduced test infrastructure failures, not
evidence requiring a production workflow or authorization change.

Owner: `/root/s3_block_fixture_r03`, already the native Trellis implementer.
Main owns databases, process/build integration, matrix and commits. This owner
will edit exactly two test files in the new private
`source-block-work-r03/server`. ROOT, R01/R02 source, runtime builds, failure
databases and raw evidence remain untouched. R03 admission is the main-owned
`block-fixture-owner-r03-admission.json` under the current private evidence root.

## Actual failure relationships

`newStageCompletionFixture` borrows `newStageExecutionFixture` →
`newStageRuntimeFixture` → `newManagedCollaborationFixture` and its queue/base
fixture. This path registers the managed/stage and parent records, but does not
register the newly created acceptance/information/repair handoffs. It does not
use the other stage fixture constructor's handoff teardown. The two new tests
are the owners of the additional rows:

- `TestCollaborationInformationRepairPreservesPendingSubmission` creates one
  acceptance row, an information request and one repair transfer: three rows.
- `TestCollaborationInformationRepairRefusesUnavailablePendingCreator` creates
  one acceptance and one information request: two rows.

Main's retained five-row audit preserves the exact issue IDs, row IDs, purposes,
actors, prior status, metadata and repair relation. A deleted parent is not
proof that these no-FK relationships were removed. No cleanup of that failed
database belongs to this repair.

The history test invokes `withURLParam(req,"id",issue)` followed by
`withURLParam(req,"handoffID",handoff)`. The existing helper creates a new Chi
context each time, so the second call drops `id`. Production reads the correct
`id` parameter and the router uses that parameter. The canonical
`testutil.WithURLParams` already attaches an even list of pairs in one context;
use it once on the fresh JSON request rather than changing shared helpers or
the UUID parser.

## Cleanup plan and exact write boundary

1. Verify the entire R03 input matches the 5,077-file frozen R02 manifest and
   retain this before-state. Keep all previous sources and failed evidence.
2. In `server/internal/service/collaboration_information_repair_test.go`,
   register one parameterized `DELETE FROM issue_handoff WHERE issue_id=$1`
   immediately after `newStageCompletionFixture` in each of the two pending
   tests, before submission or information mutation. Run it only at test exit,
   before previously registered parent/pool cleanups. Keep the seeded facts
   available for every assertion and nested test.
3. The existing `Fixture.Cleanup` intentionally ignores SQL errors. Use
   `t.Cleanup` with the existing checked `Fixture.Exec` for these two
   registrations, so a failed SQL teardown is not silently accepted. Add no
   fixture utility, broad deletion, cascade, FK, dependency or production rule.
4. In `server/internal/handler/collaboration_information_test.go`, replace the
   two single-param helper calls with one `testutil.WithURLParams` carrying both
   `id` and `handoffID`. Preserve all private-query, provenance, current-principal,
   cache-control and refusal assertions.
5. Verify the exact R02→R03 delta contains only these two `_test.go` files.
   Production, public DTO/wire, generated SQL/scanners, migrations866/867 and
   protected runtime-v1/source/snapshot bytes must remain identical.

## Verification and acceptance boundary

This owner runs targeted two-file gofmt, compile-only service and handler
`go test -c`, and combined two-package `go vet` through the already hash-bound
no-database wrapper and main-owned noDB configuration. Keep absolute commands,
tools, source/config before/after, raw outputs and binary hashes in newly
numbered private receipts. No TestMain or database-backed body is authorized for
the implementer; do not use `-run '^$'` as a compile probe.

The DB inventory stays16 service tops/43 names and3 handler names. Unchanged
pure and migration tests retain their original owning-layer evidence. Main must
use fresh normal/race package databases and independently audit the actual
handoff relation and complete declared cleanup catalog after the new runs.
Passing compile/vet does not establish database cleanup or full S3 acceptance.
The former R02 service PASS results and handler failure remain their original
not-accepted dispositions. No test outcome, failure denominator or old bytes is
rewritten by this two-file repair.
