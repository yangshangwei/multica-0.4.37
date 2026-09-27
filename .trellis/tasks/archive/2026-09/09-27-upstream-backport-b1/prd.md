# Upstream backports: batch 1

## Authorization and goal
The user explicitly approved executing batch 1, making an independent commit per feature, recording an upstream backport ledger, and merging the verified batch into local main. Later batches, remote push, tags and deployment are outside this task.

## Requirements
- Preserve the local main baseline and its custom two-language UI, templates/copies, approvals/RCA, device identity and intranet release workflow.
- Add CLI skill label management using only the six approved CLI implementation/test files plus necessary current documentation.
- Add CLI comment updates through the existing author/admin and revision API.
- Correct cache hit rate to include cache writes in the denominator.
- No database migrations, API schema changes, package dependencies or restored ja/ko/fr resources.
- Preserve unrelated original-checkout .impeccable files byte-for-byte.
- Record upstream/local commit identity, adaptation and real verification in docs/upstream-backports.md.

## Acceptance
- Each feature has a reproduced missing/broken behavior, focused passing tests, and its own Lore commit with upstream SHA.
- CLI label resource-type, workspace and permission handling remains intact.
- Comment updates preserve Unicode/input modes, require a positive expected revision, and reject stale/unauthorized updates through the existing backend.
- Both cache-hit-rate views agree for mixed cache writes, full hits, no input and near-100-percent rounding.
- Batch checks and scoped Web/Desktop acceptance pass with fresh evidence before merging to main.
- No changes to migration files, dependency manifests, retired locale registries or protected backend authorization.
