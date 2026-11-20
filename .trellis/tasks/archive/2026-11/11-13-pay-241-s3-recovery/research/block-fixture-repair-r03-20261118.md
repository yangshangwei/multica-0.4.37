# S3 block fixture repair R03 — 2026-11-18

**Two-test repair and static checks PASS; database acceptance remains pending.**
The R03 source is frozen for main's new owning-layer normal/race runs. R02's
functional PASS with five orphan handoffs, the handler's invalid-issue failure
and every earlier source/database/raw result remain retained and not accepted.
This owner performed no database, TestMain, test-body, API, browser, Provider or
Git execution.

Owner: `/root/s3_block_fixture_r03`, the native Trellis implementer. Only two
tests in the separately assigned `source-block-work-r03/server` changed. ROOT,
R01/R02 source, current runtime, public DTO/wire, production, generated SQL,
scanners and migrations were not edited.

## Actual failures and repairs

The two new pending tests use `newStageCompletionFixture` through the managed
runtime fixture chain. Their acceptance/information/repair handoffs were not
registered for teardown, while their owning issues were removed. Main's actual
retained relation audit preserves three rows from
`TestCollaborationInformationRepairPreservesPendingSubmission` and two from
`TestCollaborationInformationRepairRefusesUnavailablePendingCreator`. Those
five rows are not a product retention or cascade failure; no old failed
database is cleaned by this repair.

In `collaboration_information_repair_test.go`, both tests now register
`t.Cleanup` immediately after fixture creation and before the first submission.
Each callback invokes the existing checked `fx.Exec` with
`DELETE FROM issue_handoff WHERE issue_id=$1` and that exact fixture issue ID.
This keeps the handoffs available throughout the assertions and runs before
the already registered parent/pool teardown. `Fixture.Cleanup` intentionally
ignores SQL errors, so this repair uses its existing checked execution method
instead of silently accepting a failed cleanup. No new helper, broad deletion,
foreign key, cascade or production rule was added. The cleanup plan was written
and hash-bound before the test edits.

The new handler history test attached `id` with `withURLParam`, then attached
`handoffID` with the same single-param helper. The second call creates a new
Chi context and loses `id`, yielding the actual400 `invalid issue_id` before
the private history read. In `collaboration_information_test.go`, it now uses
one canonical `testutil.WithURLParams` call on the fresh JSON request with both
pairs. Production UUID parsing, route naming, current-principal checks,
query-validation, no-store and exact legacy provenance assertions remain
unchanged.

## Verification

Five newly recorded noDB profiles passed under
`verification/block-fixture-repair-r03-checks`:

- `gofmt -w` for exactly the two changed tests.
- `go test -c` for `./internal/service`, without executing its binary.
- `go test -c` for `./internal/handler`, without executing TestMain or its binary.
- `go vet ./internal/service ./internal/handler`.
- `gofmt -l` for those same two tests, with empty output.

Absolute argv/cwd, trusted configuration, wrapper/recorder/tool hashes, raw
stdout/stderr and all5,077 source hashes/modes are preserved per command. Every
profile exited0 and its before/after source snapshot is identical. Both test
binaries are mode0700 task-private outputs:

| Binary | SHA256 |
| --- | --- |
| service.test | `fdc3e3b15530b361d4a2044edce4707a9c0e74272f267d0e65faf54f42607379` |
| handler.test | `e3730410b1725d5b5ca2073e0b853df3424348111155c70f1e80493729d057a5` |

No `-run '^$'` or filtered TestMain probe was used. Unchanged pure and migration
tests were not repeated; their owning-layer evidence remains separate.

## Source seal and handoff

- Freeze: `intake/block-server-freeze-r03.json`, SHA256
  `4f22e38c6ad3aacdf5ab84c7c08d4174cf369544803e3d4815b8f51d9961bd7f`.
- Manifest: `intake/block-server-candidate-manifest-r03.json`, **5,077 files**,
  SHA256 `538e4f92de2707285d285a98238260edf2acc3930512056b2f560ba01db6ad36`.
- R02→R03: **two existing `_test.go` changes, 5,075 unchanged, no new or removed
  source paths**. The precise diff is retained with its source bindings.
- Inventory: `intake/block-server-test-inventory-r03.json` and
  `block-server-expected-names-r03.json` retain the exact finite identities:
  **16 service DB tops/43 names, 3 handler names and 5 migration names**.

All files above live under the private evidence root
`/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`.
The sibling report JSON binds every plan, raw failure, compile/static receipt,
manifest, delta and inventory. R02 source and its failed evidence were rehashed
unchanged after the repair.

Main must run fresh complete service/handler normal/race package profiles and
audit the actual handoff relation plus the full cleanup catalog, original gate,
migration/index and connection state. Static PASS does not certify database
cleanup or full S3. This report updates no parent matrix, task status, archive
or commit. Full current API/Web/Desktop block/repair/history and S3 acceptance
remain with the coordinator and independent checker.
