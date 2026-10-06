# Source-context cleanup regression found during I1 verification

The preceding creation regression exposed a real pre-existing query defect,
not merely a flaky fixture. A later passing run did not resolve that defect.

## Root cause and real RED

`ClaimSourceContextObjectIntentForCleanup` used UPDATE FROM a LIMIT1 / FOR UPDATE
SKIP LOCKED subquery. When statistics estimated one intent but three existed,
PostgreSQL17 selected an outer intent scan plus an inner candidate subquery.
That inner Limit/LockRows ran3 times. One UPDATE leased all3 rows using the same
lease token, while sqlc's QueryRow consumer processed only the first. The other
2 rows retained live leases, so the cleaner then saw no eligible row and
reported cleaned=1. After ANALYZE, a Hash Join could select once and pass.

The default-GUC reproduction uses a transaction-local copy of the real table:
insert1, ANALYZE, insert2 more. No live rows or planner configuration are changed.
The independent SQL/EXPLAIN evidence is `/tmp/i1-claim-minimal-repro-and-fix.sql`
and `.log`: old UPDATE returns3, inner Limit loops3; MATERIALIZED returns1.

The permanent `TestSourceContextClaimKeepsSingleLeaseWithStaleStatistics` calls
real generated insertion/claim queries against that temporary table and asserts
one row per lease plus no duplicate claim across three attempts. Its real RED
was `leased=3 total=3 after claim1`, `/tmp/i1-source-claim-red.jsonl`.

## Fix and final checks

The candidate SELECT is now `WITH due AS MATERIALIZED`, evaluated exactly once.
Ordering, eligibility, SKIP LOCKED, lease duration and the service attempt budget
are unchanged. No migration, dependency or global planner switch is required.
Temporary diagnostics were removed from the existing lifecycle test.

- Guarded service claim + existing cleanup/budget regressions:3 passed,0 failed
  or skipped, with race detection; `/tmp/i1-source-claim-green.jsonl`.
- Original CommentSourceContextLifecycle and I1 deletion regression selection,
  repeated3 times:48 passed,0 failures/skips, with race detection;
  `/tmp/i1-source-cleanup-final.jsonl`.
- Full Go build/vet exited0; `/tmp/i1-source-claim-final-{build,vet}.log`.
- sqlc regeneration and generated-file stability verified by root before commit.

Commands used task-only local DATABASE_URL and the agent CLI guard. The temporary
SQL probes roll back and shadow the table per connection, avoiding shared data
mutation. This narrow regression fix does not satisfy remaining I1 FG gates.
