# F3 implementation handoff

Implemented in `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`. No commit, Provider invocation, migration, broker, frontend change, or ordinary runtime injection was made by this lane. All 22 owned Go/SQL files are listed in `owned-files.txt`; exact final source SHA-256 values and HEAD are in `source-manifest.json`.

## Result

- Durable source-only intent hooks cover actual purge, comment tombstone, participant, membership, account/password and PAT transactions. Target deletion retains final routing/hints and removes independent counters while preserving existing source deletion restrictions.
- Bounded scanner persists a frozen boundary, uses ever-published immutable routing, keeps busy cursors, deduplicates one revision/hint per cause/target, and never takes source/candidate/task row locks. Live parent + missing counter is a retryable error, not empty success. Deleted parents never initialize counters.
- Existing relay delivers stable IDs and an exact three-field public target hint. Runtime scheduling and content-free backlog/retry diagnostics are registered. Physical send rechecks current human account and target workspace membership; a target-only administrator does not need source Read.
- Common purge, credential and membership seams replace duplicated special cases. Comment history remains readable through ordinary edits; deletion is the invalidating transition.

## Evidence

- **26 new top-level tests / 32 total test and subtest outcomes passed**, then the same 32 passed under `-race`. `test-inventory.json` compares each source-declared name with observed pass events: zero missing tests, skips or failures.
- **195 related existing test/subtest outcomes passed**: 122 service memory/workflow/comment/account/realtime cases, 64 member/workspace/project/device/share-link/PAT handler cases, and 9 server scope/realtime cases. Zero skips or failures. See `related-test-inventory.json`.
- Full `go vet ./...`, `go build ./...`, and `git diff --check` passed. Commands and exits are in `focused-checks.json`, `related-race-checks.json`, `static-checks.json`; raw JSONL/logs retained. SQL generation was performed only by the A2 owner; the generated F3 file contains three queries.
- Fresh isolated PostgreSQL database **multica_knowledge_invalidation_20261101_849a1c21**, schema **760**, private environment `/tmp/multica_knowledge_invalidation_20261101_849a1c21.env` (0600). `environment.json` records identity only. Service and handler suites ran sequentially in this DB. A2's b3c35f9c database was never used for task data.
- Real locks prove human/admin purge commits while a target counter is held and workers never wait on held source/candidate rows. Other regressions cover scan rollback/unknown commit, two workers, busy-job fairness, concurrent original human approval, target deletion, source maintenance deletion, source mutation rollback, credential/membership grants and removals, and exact relay wire plus a real WebSocket queued-frame revocation barrier.

## Corrections and limits

Initial tests exposed fixture assumptions: pending invalidation rows cannot be deleted by cleanup, so later tests settle prior orphan jobs using the real worker; public approval needs valid source references, not scanner-only routing fixtures; project deletion preserves its existing retryable 503 error contract. These initial failures and subsequent passes remain in the evidence folder.

The hook-by-hook matrix and unreachable/no-op paths are in `hook-inventory.md`. Existing collaboration source hard deletion remains prohibited. The synthetic source-deletion test proves replay independence, not a new delete API. Backend hints do not prove browsers consumed them; Web/Desktop live product and finite-cache-lease evidence belongs to the main/Core/view lanes. Provider/session/runtime G2 work is separate. This lane does not claim whole-task completion or wider platform acceptance.
