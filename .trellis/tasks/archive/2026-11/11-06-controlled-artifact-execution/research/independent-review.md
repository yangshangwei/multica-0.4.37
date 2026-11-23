# Independent implementation review — final

## Verdict

**PASS, zero remaining blocking findings** for the local-synthetic controlled artifact backend child. This does not close the parent M1/M2/M3 roadmap or any real-model, deployment, desktop or UI/CLI gate. Reviewer made no product edits; owning implementation lanes resolved the findings.

## Findings resolved by the implementation owner

1. Agent detail and claimed replay now recheck originator invocation permission. A retained real task token is denied after a distinct originator loses the invoke target; authorized human audit reads remain.
2. Creation replay validates original canonical/denormalized binding before returning retained data and uses current read authority without renewing expiry.
3. Runtime lookup uses the existing transaction-bound instrumented wrapper. The canonical direct-query scanner passes.
4. Once packaging ran, finalization lock failure returns503 with explicit unknown/execution identity. The retained claim stays unknown and replay invokes nothing. Preclaim busy remains409.

## Review coverage

Inspected canonical strict decoding/hash domains, exact limits and fixed policy; trusted staging/no-follow descriptor traversal, input snapshots, inode and byte verification, exclusive USTAR publication, durable uncertainty and cleanup ownership. Inspected atomic pending approval/spec creation, original credential/principal locking, human digest decision, one unknown claim before actual Pack, nonce-only atomic finalization, restricted reads, legacy guards and retained bound approvals across real workspace deletion.

The ten acceptance areas and C1–C4/D1–D4 are mapped to concrete tests in `independent-review.json`. Actual Execute precommit barriers prove blocked revocation, zero Pack before definite commit and exactly one afterward. Production CancelTaskInTx is exercised in both orders. Actual Create/WorkspaceDelete ordering and post-deletion completion preserve original decisions and receipts without recreating deleted scope. Direct SQL lock tests remain supporting evidence, not substitutes for actual handler/adapter proof. This is targeted coverage of the contract, not every possible role permutation.

## Verification

- Independently recomputed all38 frozen product/test source hashes; no drift. Reviewer gofmt returns no files.
- Fresh handler databases: ordinary84pass and race84pass, both0fail/skip.
- Fresh router databases: ordinary23pass and race23pass, both0fail/skip; real legacy/password/task credentials and post-auth managed revocation included.
- Pure packager: macOS ordinary/race each78pass; Linux container78pass. Windows fail-closed stub cross-compiled only.
- Actual SQL race9pass; failed concurrent-index recovery/registry3pass; migration rules4pass. No row/index rollback was run against a business database.
- Auth/middleware48pass, built-in source contracts21pass, runtime lookup scanner1pass.
- Whole-server build/vet exit0. Final raw-log hashes and named terminal counts independently match every retained record. Counts include subtests and separate runs, not unique-case totals. No frontend code changed; TS checks are inapplicable.

No reviewer DB writes, service startup, real-provider calls or repeat integration runs. Final source manifest and evidence records are hash-bound in the JSON. Operational cleanup, scoped commit/integration and task archive remain the coordinator’s responsibilities.

## Limits retained

Backend-only fixed local tar policy; default-disabled without operator roots. No deployment/publication or generic host sandbox. Operator/same-UID/root actors are trusted. Source revision is declared; staging manifest proves input bytes rather than a Git checkout. Linux race/native Windows execution and client UI/CLI flows were not tested or claimed. Original failed test attempts remain historical failures.
