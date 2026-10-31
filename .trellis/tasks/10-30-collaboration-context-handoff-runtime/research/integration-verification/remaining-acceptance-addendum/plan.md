# Remaining deterministic acceptance boundary

The gap is evidence for composed behavior: different squad runtimes must enforce their own preparation budgets; a stale memory writer must refresh and explicitly merge without losing either author's fact; ordinary task/plugin/IM credentials must not enter the new private runtime endpoints.

The behavior lives in service admission/claim preparation, service memory versioned writes/reads, and the narrow runtime handler credential gates. Tests will reuse real database fixtures and service operations. Handler tests will preserve the production route boundary: the private runtime endpoints authenticate their own bearer capabilities, outside ordinary `Auth` middleware.

Owned changes:

- `server/internal/service/collaboration_squad_budget_matrix_test.go`: per-runtime file-count, per-file, total-input and workspace budget cases, mandatory references, optional selections, and pre-claim/reservation refusals.
- `server/internal/service/collaboration_memory_merge_acceptance_test.go`: conflicting writes, unchanged predecessor, current-version reread, new-intent merged replacement, exact author/version history and original refusal replay.
- `server/internal/handler/collaboration_runtime_credential_matrix_test.go`: task/plugin/IM and wrong-purpose capabilities against handoff/checkpoint/recall/content/seal handlers, no private response data or mutations, and valid content-free termination metadata where permitted.
- This evidence directory: test inventory, source hashes, commands, raw/JSON results and a final summary.

No production edits, shared fixture changes, migrations, provider execution, commits, or unrelated broad gates. Any discovered product defect will be reported before widening scope. No refactor is planned.

Use only `multica_runtime_final2_858f011e` through its existing private environment file; never record credentials. Run the targeted service suite and then the handler suite sequentially. Require named nonempty tests with no skips, format the new files, and run relevant package vet.
