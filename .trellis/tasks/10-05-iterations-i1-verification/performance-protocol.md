# I1 performance acceptance protocol

Status: passed after revalidation with frozen priority/labels and label-write fences. The frozen thresholds are unchanged. Full raw observations, query plans and source hashes
are in [performance-evidence.json](performance-evidence.json).

The local reference host is Apple M4 (10 logical CPUs), 16 GiB RAM, macOS 26.2,
with PostgreSQL 17.11 aarch64 in the existing local VM. Use an exclusive migrated
database and the agent CLI guard. Record Go version, database version, query
plans, raw timings, PostgreSQL lock samples, and concurrent host load.

## Frozen thresholds (2026-10-07, before final acceptance samples)

| Operation | Population | Warm P95 limit |
| --- | --- | --- |
| Detail | 1,000 current tasks and actual start/history facts | 2,000 ms |
| End preview | Same complete 1,000-task population | 2,000 ms |
| End with carryover | 1,000 tasks, real snapshot and membership writes | 2,000 ms |
| Whole disable preview/apply | 1,530 tasks distributed over 51 plans | 3,000 ms each |

These are explicit engineering targets, not a claim that the PRD's suggested
two seconds already passed. The batched discovery run's final thirteen closure
observations were 1.20–1.99 seconds after host memory pressure subsided; the
two-second target is therefore plausible but still needs independent samples.
The larger disable target scales the supported population by approximately 1.5.
Do not raise these thresholds after observing a failed acceptance run.

Each scenario runs five identified warmup operations followed by thirty measured
operations. Preserve every raw value, including warmups, errors, and outliers.
P95 uses nearest rank (29th sorted observation of thirty). Each close/disable
must execute a fresh request and real transaction, not replay or roll back a
benchmark transaction. Fixture setup is excluded from endpoint timing.

No agent-owned build or other product test runs concurrently. A human-owned
development server may remain; record its load without stopping it. Do not call
the first observation a cold-disk measurement: the OS/PG caches are not flushed.
Query plans and a separate ordinary-writer benchmark supplement endpoint P95.

## Discovery observations (not accepted)

- Unbatched, concurrent development load: closure median 3.17 s, P95 14.28 s.
- Batched, after stopping the agent-owned 9.6 GiB Next dev process: closure
  median 2.77 s, P95 8.52 s; detail P95 151 ms; preview P95 794 ms.
- That batched run still began while the host recovered from memory pressure.
  Its 51-plan disable apply P95 was 6.02 s. Neither run meets acceptance.
- The batched run sampled one PostgreSQL relation-extension wait (~9 ms);
  endpoint time must not be described as lock time.

Raw discovery logs: `/tmp/i1-p95-initial.log`, `/tmp/i1-p95-batched.log`.
Final raw observations and source identifiers will be recorded alongside the
verification evidence after the final implementation is tested.

## Acceptance observations

Five warmups plus thirty measured real operations per scenario, no concurrent
agent build/test jobs, same exclusive migrated DB. Both opt-in tests passed;
the frozen limits were enforced by assertions after all raw samples were logged.

| Operation | Measured P95 | Frozen limit |
| --- | --- | --- |
| Detail, 1,000 tasks | 57.992 ms | 2,000 ms |
| End preview, 1,000 tasks | 324.154 ms | 2,000 ms |
| End with carryover, 1,000 tasks | 1,099.838 ms | 2,000 ms |
| Disable preview, 51 plans / 1,530 tasks | 459.041 ms | 3,000 ms |
| Disable apply, 51 plans / 1,530 tasks | 957.511 ms | 3,000 ms |

Command: isolated `DATABASE_URL` and `MULTICA_RUN_I1_P95=1`, then agent CLI guard
around `go -C server test ./internal/handler -run '^TestIterationI1(Disable)?P95$'
-count=1 -timeout 30m -v`. Total package time 229.720 s. Raw log:
`.omx/logs/i1-verification/p95-history-final.log`. These are local warm endpoint measurements, not a
production SLA or proof about cold caches / other hardware.

Final rerun includes priority and label capture: 1,000 tasks across ten projects,
five priorities, owner assignee and three of six labels per task. Every measured
closure verifies all 1,000 frozen rows retain priority and three labels. Disable
uses one active and fifty planned periods, with three labels per task. Membership,
events and label query plans are retained in the evidence. No agent-owned build or
other product test ran concurrently. All warmups and outliers remain recorded.
The previous passing pre-metadata evidence is retained in
`.omx/logs/i1-verification/performance-before-history.json`.
