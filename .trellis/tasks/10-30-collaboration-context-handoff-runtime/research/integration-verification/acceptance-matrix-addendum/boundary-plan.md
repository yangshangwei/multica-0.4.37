# Deterministic acceptance matrix addendum

Scope: runtime AC1 and AC5 only. Product baseline `0e1ee8e1e`; prior integration evidence `2b861347e`. This work adds tests, not production behavior.

## Gaps and owning boundaries

- `RecordRuntimeCheckpoint` shares authority locks with `StopQueue`, `RevokeGrant`, and input-author `Leave`. Existing tests cover append followed by Stop, but not competing transactions in both orders. Hold the first real service transaction at commit with the existing `recipientCommitStarter`, observe the second transaction blocked through PostgreSQL, release, and check both returned results and retained records. Test the three authority changes in both orders, exact replay versus changed/new content, metadata-only sealing, original cleanup proof, and reservation release only after proof. Use bounded contexts so lock regressions fail rather than hang.
- Squad comments, proof wakeups, and joined inputs are allocated under the group locks. Add a two-member service scenario retaining exact source identities and result provenance. Freeze a claimed continuation, then submit late comment/input/proof causes and prove they belong to one subsequent continuation without widening the first claim's inputs or snapshot. Ordinary human discussion remains data-only and is not counted as an admitted input.

## Write boundary

- `server/internal/service/collaboration_checkpoint_authority_matrix_test.go`: the three-by-two authority ordering matrix and narrowly scoped test helpers.
- `server/internal/service/collaboration_wakeup_boundary_test.go`: two-member cause coalescing and claim-boundary regression.
- This evidence directory: plan, commands, logs/JSON inventory, source hashes, and final results.

No production or existing test edits, migrations, dependency changes, commits, provider execution, other acceptance matrices, or task/parent ledgers. A production defect is reported to the parent before widening scope. Reuse existing `dbfx` and service fixtures; no fixture-row insert/cleanup boilerplate or production test hooks.

## Verification

Use only the assigned disposable database `multica_runtime_integrate_20261031_f2478725`, loading its existing private environment file without displaying credentials. Service DB tests run exclusively. Run the new named tests with `-count=1 -json`, then `-race -count=1` if practical, plus gofmt, diff whitespace checks, and service vet. Assert exact expected top-level/subtest inventory and zero skips; do not count a no-test run as success. Preserve first failures and follow-up logs. Record source HEAD, the two added source hashes, exact commands, platform, database identity, and exit status. Prior full-suite passes are not re-run or relabeled as this addendum.
