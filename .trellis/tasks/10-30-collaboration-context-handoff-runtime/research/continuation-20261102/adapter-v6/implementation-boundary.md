# v6 verification adapter boundary

The existing full-check launcher exports API pool limits of 10/1. The copied task
environment also carries those keys into Go tests. The v5 adapter forwards them
to both newly allocated package children and validated recursive children, which
changes the defaults exercised by `server/cmd/server/dbstats_test.go`.

The independent v5 review also demonstrated shell substitution in the launcher's
`bash source` loader using a harmless, owned fixture. Merely changing the initial
loader would leave downstream `make check` able to source the same raw file.

This repair changes only newly versioned task-private verification files:

- `check-bin/go`: remove the two API-only pool overrides at the executable Go
  test boundary after non-test and compile-only forwarding. Existing package
  allocation, recursive allocation validation, arguments and evidence remain.
- `check-bin/pnpm`: preserve the existing fixture/guard wiring byte-for-byte.
- `check-go-isolation-test.py`: retain the original eleven contracts and add
  observed child-environment regression cases.
- `run-make-check.py`: provide a coordinator-installed launcher template with
  literal environment parsing, a safely quoted downstream environment copy,
  actual adapter/launcher/source hash checks, and the existing candidate/review,
  temporary-directory and evidence gates.
- Parser/launcher tests and an implementation report record the red/green runs.

API processes keep their task-local 10/1 limits. Tests may set their own pool and
HOME values after startup. Production source, frozen test assertions, earlier
adapter versions, earlier evidence, shared task records and database contents
are outside this implementer's write scope. Verification is limited to fake
executables, local syntax checks and connection-free pool-sizing tests; the
coordinator owns real full-check execution and independent clearance.
