# Agent autonomy merge readiness

## Goal

Preserve the branch's value: enforce declared agent autonomy at server write boundaries and retain recoverable Git conflict work without delivering unresolved merges. Close the seven confirmed review findings before recommending a merge into local main.

## Requirements

- Authorized squad provisioning must work with one database connection and retain the access checks on reused agents.
- Finalization must inspect the content it will commit. Resolved work, Markdown headings and ordinary diagnostic-like text must remain deliverable; actual unresolved conflict groups must remain blocked and recoverable.
- Accepted case variants of batch updates must have deterministic field-presence semantics, including explicit null unassignment.
- The HTTP policy suite must clean its own related rows and actually exercise the new autonomy ceiling with a Coordinator task credential.
- Existing agents without a declared autonomy level retain their documented behavior. Do not describe these API gates as a process or credential sandbox.
- No dependencies, migrations, automatic merge, or remote publication are required.

## Acceptance

- [x] The single-connection public-agent reuse regression passes without removing authorization.
- [x] A stale staged conflict followed by a working-tree resolution finalizes correctly.
- [x] Valid Markdown separators and whitespace diagnostics are accepted.
- [x] Unresolved staged or newly staged working-tree conflict groups remain refused and recoverable.
- [x] Mixed-case batch update objects preserve null unassignment deterministically.
- [x] E2E teardown leaves no test-owned issue-status or automation-version rows.
- [x] Coordinator-to-Operator creation is denied by the ceiling, while equal/lower creation succeeds.
- [x] Relevant regression tests demonstrate failure before the fix and success after it.
- [x] Typecheck, lint, TS tests, guarded Go race tests, Go static checks/build and the API suite pass.
- [x] Independent review finds no remaining blocking regressions in the changed paths.

## Source

.omx/reviews/agent-autonomy-gates-2026-09-06.md, reviewed HEAD 886be2898, base main 68ecd9f22.
