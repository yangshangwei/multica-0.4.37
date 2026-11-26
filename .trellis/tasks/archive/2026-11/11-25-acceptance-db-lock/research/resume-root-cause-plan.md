# Service continuation: bounded discriminating plan

Base: 9ffa37281184ae605bcbc0bc7a09caf3df60d180. Historical failed runs remain immutable in the previous worktree.

## Evidence and separate hypotheses

1. Original SHARE NOWAIT 55P03 holder remains unknown. Later autovacuum witness is not contemporaneous and is not attribution; no Go DATA RACE was present.
2. The first knowledge invalidation test in final-race-01 spent 428.96 s. newKnowledgeInvalidationFixture globally drains all prior preserved scan and hint jobs before starting its own assertions. The worker performs one candidate per transaction; all_sources jobs multiply retained candidate history. This is a concrete cross-test work amplifier, independent of a lock/deadlock claim.
3. Original purge/fanout 2 s checks and checkpoint/stage failures require separate timing evidence. A test message saying it waited for a target does not establish a database lock wait. Query call duration, server execution duration, transport gaps and actual blockers distinguish these paths.

## Experiments and sequence

- Fresh allocated DB: run checkpoint commit-order, stage source-revocation, knowledge purge/fanout and original wrong_consumer groups, unchanged guards/budgets, supported Go 1.26.11 and CLI guard. Capture actual named results, transaction/query timings when available, actual wait state and ambient load.
- Fresh bounded comparison: same knowledge fixture with zero versus seeded unrelated retained jobs/candidates. Count scan transactions and capture original row fingerprints before/after. This tests work amplification without rerunning the whole package.
- If confirmed, isolate only pre-existing unrelated pending outbox jobs in the test fixture using existing SQL lock/worker skip-locked semantics; do not delete/alter retained history, change production worker selection, or remove multi-fixture/fairness tests. Verify an unrelated pending scan and hint remain byte-identical while current fixture work completes and still produces the required target event. Consider candidate-history work separately.
- Only after the focused repair is established, run complete service normal and race sequentially, fresh DB per run, original 40 m, p=1, parallel=2, GOMAXPROCS=2, supported pinned tool, dedicated Redis. Preserve any new failure.

## Boundary

Expected implementation owner is service tests and minimal proven service fixes only. CLI request instrumentation belongs to sibling lane. Parent allocates resources and coordinates full-run budget. No commits/push, permission caching, NOWAIT relaxation, skip, enlarged timeout, autovacuum changes, or old launcher execution. A focused success never closes incomplete original complete-package runs.
