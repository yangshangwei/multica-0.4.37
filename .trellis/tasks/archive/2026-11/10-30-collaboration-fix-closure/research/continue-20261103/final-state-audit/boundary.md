# Final-state identity audit boundary

The registered v11 launcher verifies identities before Make and records its exit
without checking those identities again. A task-private, read-only audit will
run after the complete command and reject changed source, tools, workspace links,
CLI or desktop artifacts before root accepts that result.

Files owned by this slice are this directory's `final_state_audit.py`, private
regression test, and their evidence/documentation. The candidate, registered
launcher/configuration/bindings, previous evidence and all other task files are
read-only. No services, SQL, Provider, account, dependency or commit action runs.

The auditor reuses the exact registered `verify_owned_bindings` function. Only
`apps/web/next-env.d.ts` may differ by the exact generated route-reference line
`.next/dev/types/routes.d.ts` -> `.next/types/routes.d.ts`. Its regular-file type,
mode, registered Git blob and raw bytes are checked before a path-specific,
in-memory hash normalization. Every other guard remains intact. The audit records
the raw and normalized Next hashes and product SourceIDs separately.

Root retains actual live API/Web/database/renderer/Redis ownership and result
inventory checks. A passing final identity audit is not a passing whole check,
password journey or real-model experiment.

Regressions reuse the existing credential-free v11 fake fixture, mutate only
new temporary files, and exercise actual filesystem guards. Git/Node metadata
commands are mocked; unknown subprocesses are rejected. The red run precedes the
new auditor implementation. Final reports use exclusive creation under this
directory and never overwrite previous evidence.
