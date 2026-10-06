# LG/HG integrated acceptance — 2026-10-06

Status: **LG passed; HG passed**, including the final independent review fix.
Implementation commits: `1247d2728` and `4c9be7652`.
Branch: `codex/projects-p1`. Starting commit: `da6c0fc65`.
Machine-readable results: [lg-hg-verification.json](lg-hg-verification.json).

The `iterations_i1` release default remains **off**; capability
`atomic_handoff` remains **false**. No release configuration was changed.
Only isolated task databases were used; all Go tests used the agent CLI guard.

## Accepted scope and evidence

| Gate | Implemented behavior | Evidence |
| --- | --- | --- |
| LG management | Real create/edit/list/detail, preview/apply start/move/planned cancel/unused delete; current human authorization; durable replay | `iteration_management_test.go`, `iteration_list_test.go`, `iteration_lifecycle_routes_test.go`, service lifecycle suites |
| LG membership | Complete sets, fixed original commitment, one active, terminal choices, leave/reentry and rollover retention, no management execution side effects | [Lifecycle ledger](../10-05-iterations-i1-lifecycle/verification.md) |
| LG ordinary entrypoints | Confirmed create, explicit completed acknowledgment, membership-only PUT, current Contributor checks, real iteration fields in all issue read builders | `issue_iteration_assignment_test.go`; final explicit-create follow-up below |
| LG T1 | Acceptance/project/iteration atomicity, stale target remains pending, held invocation grants, one action identity and one clock sample, single explicit execution | `triage_iteration_assignment_test.go`, `triage_iteration_authorization_test.go` |
| HG facts | Real LG and ordinary update/delete writers produce A–J: O8/current9/effective8/completed5/original_completed4; 62.5%/50% | `TestIterationHistoryCanonicalThroughLifecycleAndIssueWriters`; [History ledger](../10-05-iterations-i1-history/verification.md) |
| HG reads | Coherent protected RR reads, bound cursors, stable original display, complete validated immutable snapshots, later metadata audit | History/snapshot suites; actual close persistence remains CG |

## Final checks

Counts include subtests. The selections overlap; do not sum them as unique tests.

| Check | Result | Raw local log |
| --- | --- | --- |
| Broad handler regression before last create acknowledgment | 638 passed; 2 existing opt-in skips | `/tmp/i1-lg-hg-final-handler.jsonl` |
| Complete domain/util/public API packages | 240 passed, no skips | `/tmp/i1-lg-hg-final-domain.jsonl` |
| Final LG/HG/T1/creation/service/JWT integration after acknowledgment fix | handler179 + service154 + router5 passed; same 2 opt-in skips | `/tmp/i1-lg-hg-integration-accepted.jsonl` |
| Explicit completed-create real DB/race regression | 17 passed, including 9 new cases; no skips | `/tmp/i1-lg-create-completed-green.log` |
| Builtin issue guidance after final documentation synchronization | 1 passed | `/tmp/i1-lg-hg-builtin-accepted.jsonl` |
| TypeScript unit suites, cache bypassed | 10,054 passed; 5 tasks | `/tmp/i1-lg-hg-ts-test.log` |
| TypeScript lint/typecheck, cache bypassed | 15 tasks passed | `/tmp/i1-lg-hg-ts-static.log` |
| Full Go build and vet | Both exit 0 | `/tmp/i1-lg-hg-build-after-review.log`, `/tmp/i1-lg-hg-vet-after-review.log` |
| sqlc | 84 generated files stable; no migrations added | `/tmp/i1-lg-hg-final-sqlc.log` |
| Formatting/static diff | gofmt clean; `git diff --check` passed | local tool output |

The skipped tests are `TestTriageQueueScaleBaseline` and `TestTriageWireFixtures`.
Raw `/tmp` logs are local artifacts; the committed JSON records counts and
hashes. Previous RED/intermediate failures remain documented in both child
ledgers and are not represented as passing runs.

Final integration commands:

```sh
DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/multica_i1_w13_20261006?sslmode=disable' \
  bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 1 -parallel 2 \
  ./internal/handler ./internal/service ./cmd/server \
  -run '^(TestIteration|TestIssueIterationAssignment|TestTriage|TestBuiltin.*)' \
  -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test ./internal/service \
  -run '^TestWorkingOnIssuesSkillCoversIssueLoopContracts$' -count=1 -json
pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2
DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/multica_i1_w13_20261006?sslmode=disable' bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -race -p 1 -parallel 2 ./internal/handler ./cmd/server \
  -run '^(TestTriage|TestIterationLifecycleRoutes)' -count=1 -json
go -C server build -p 2 ./...
go -C server vet -p 2 ./...
git diff --check
```

## Capacity and remaining gates

Final independent review found a settings-response query borrowing from the
pool while its caller still owned a transaction. The real `MaxConns=1` RED
regression returned 500 after its 3s request deadline
(`/tmp/i1-triage-capability-red.log`). The capability query must use the caller's
transaction-bound `q`; a larger pool or post-commit query would conceal the
ownership defect. GREEN completed in 0.04s (`/tmp/i1-triage-capability-green.log`). Root independently
reran all T1 tests plus the lifecycle JWT route: **114 passed**, with the same
two existing skips (`/tmp/i1-lg-hg-pool-fix-accepted.jsonl`). Final full Go
build/vet both exited 0 after this fix (`/tmp/i1-lg-hg-*-after-review.log`).
The query now receives `q` explicitly; settings GET uses pool queries and the
settings write uses its existing transaction. No review blockers remain.

LG proves complete 1,000-item preview/move/start and rejects 2,001 atomically.
The deployment operation limit is 2,000. Recorded timings are single local
observations, not P95 or production SLA evidence.

Next is **CG**: end, active cancellation, handoff, workspace disable, actual
snapshot persistence, outbox delivery/revocation and overdue reminders. Then
UG completes Web/Desktop behavior and compatibility; FCG integrates all shared
SQL/router contracts; VG owns full acceptance, repeated capacity, full repository
Go tests, Web/Electron E2E, `make check`, and remote CI. None is claimed by LG/HG.
The foundation task remains open through FCG.

Two pre-existing untracked `handoff-2026-10-06-lg-hg*` files came from another
session and are excluded from this work's commits.
