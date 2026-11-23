# Core FIFO and Switch boundary

## Scope and implementation plan

This child owns the core execution DTO/parser and its pure regression coverage.
The main session approved execution read protocol **4**; runtime/control protocol
2 is unchanged. Backend mutation, migration, projection and integration evidence
belong to their respective owners.

The existing parser treats each target's sequence and current channel as the
authority for all history. That rejects a settled group after a leader Switch
and cannot represent a proven never-launched input returning to waiting.

1. Move `next_command_seq` from targets to channels, add nullable command `group_id` and
   immutable `groups[].inputs[]` membership. Keep exact decimal parsing and
   complete same-issue collections. Initial consumed membership requires a
   matching run; unread M11 membership is outside this slice.
2. Validate current waiting/in-flight associations separately from settled
   historical membership. Waiting follows the target's current channel;
   requeue requires the previous consuming run's never-launched proof.
3. Retain uniqueness, channel control mirrors, typed target identity, group
   generation, sealed settlement and run ancestry checks. Add boundary tests
   before implementation, including malformed/private responses and transport
   wiring. Preserve disabled execution affordances.
4. Run focused Vitest and core lint/typecheck; record the actual results below.

Expected files: `execution-schema.ts`, `test-fixtures.ts`,
`execution-schema.test.ts`, one focused FIFO/Switch parser test file, and the
existing private API read regression. No stores, visual UI, dependencies,
generated files or backend edits are part of this child.

## Baseline

- `pnpm --filter @multica/core test collaboration/execution-schema.test.ts`:
  56 tests passed on the initial implementation, before edits.

## Results

The core boundary and matching Go read projection are implemented. Recovery
work extended ownership to the Go projection, legal consumption/history fixtures,
and the HTTP/Linux route assertions; the initial no-backend-write scope above
was superseded by that explicit handoff.

- Core full suite: 3,532 tests passed; focused collaboration suite: 476 tests.
- Core and workspace lint/typecheck passed. Old v3 execution is explicitly
  unavailable; runtime/control protocol 2 is unchanged.
- The Go projection shares the settled-proof predicate with Stop and late proof.
  Independent tests reject started evidence, missing seals and stale attempt
  boundaries. Pure read and HTTP/routing race suites passed.
- Actual ARM64 Linux route tests passed, including original input requeue and
  a new real run after explicit Resume. The held-S case preserves the public
  `squad_execution_unavailable` refusal and stages its authorized internal input;
  only A reaches the real test provider. This is not public squad execution.
- Final combined verification and source hashes are recorded by the main
  session in the task's FIFO continuation and verification summary.
