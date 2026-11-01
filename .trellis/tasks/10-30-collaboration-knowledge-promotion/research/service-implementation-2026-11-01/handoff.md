# A2 implementation handoff — 2026-11-01

A2 is implemented and frozen in `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101`. No commit or real Provider invocation was performed by this agent. Main owns independent review and integration.

## Delivered

- Human proposal/correction, exact reviewed approval, rejection, dual-authority withdrawal, shared-namespace idempotency and candidate-only original-actor receipt recovery.
- Authorized source/detail/history/target projections matching Core, including restricted control rows, source-unavailable suppression, required history truncation, complete version paging and a non-renewing 30-second maximum projection lease.
- Shared required-source availability, trusted-consumer resolution, exact manifest revalidation, bounded metadata lock discovery and resolution-to-lock-set validation. No private body is copied into base context or task context.
- Nine human/target production routes, strict request boundaries and actual HTTP fixtures validated by the Core schemas.
- Original PAT provenance through verified authentication, current token checks for knowledge and reused private source readers, write-side credential locking, and read-lease expiry clamping.
- Reserved task-column `json:"-"` generation policy covering base and CTE-derived results, with real serialization and generated-tag inventory regressions.
- Owner/admin publication and source-authorized withdrawal avoid the project-lead SHARE hotspot; genuine project-lead authorization retains that lock.

The exact contracts and C handoff requirements are in `api-and-consumer-seam.md`. In particular, C must call `ValidateKnowledgeResolutionLocks` before disclosure and retain its independent mandatory consumer/credential/late runtime/task fences. Fresh missing sources are omitted; exact invalid history is never replaced by the current set.

## Ownership and integration

- `owned-files.txt`: **27 exclusive A2 source/test/config files**.
- `shared-files.txt`: router and generated model/agent files with shared ownership; do not blindly stage unrelated hunks.
- `review-snapshot.json`: original review allowlist, shared-file explanations and dependency identities.
- `source-hashes.json`: final hashes after formatting and scoped test cleanup.

A2 service manifest/revision types alias **C2's `protocol/task_knowledge.go`**. F3 owns its invalidation worker, relay and existing-writer hooks, and its tests consume A2 availability. Integration therefore needs the shared protocol prerequisite or an appropriate combined integration commit. A2 coordinated sqlc output for the A2, F3 and C2 query files; semantic ownership of the latter two remains with their agents. No task/daemon/agent execution source, frontend view, formal plan or task status was edited by A2.

## Verified evidence

| Scope | Result |
| --- | --- |
| Owned checks | **44 top-level tests, 40 subtests**, zero missing expected tests/skips/failures |
| Owned service race run | **32 top-level tests, 18 subtests**, all passed |
| Related source service/handler/router suites | **59 top-level tests, 102 subtests**, all passed |
| Actual HTTP to Core schema | Six response fixtures passed the current Core Zod schemas |
| Full Go static checks | `go vet ./...` and `go build ./...` passed |
| SQL generation | sqlc **1.31.1**, **126 generated files**, zero drift; query files unchanged during generation |
| Formatting/diff | Owned/shared Go formatting and scoped whitespace checks clean |

`checks.json` retains exact commands, durations and exit codes, including historical failures rather than concealing them. `test-inventory.json` separates owned, race and related executed names; `expected-tests.json` records the 32 service tests. `sqlc-drift.json`, `wire-validation.json` and `raw/` provide the detailed evidence.

All database tests explicitly loaded `/tmp/multica_knowledge_20261101_b3c35f9c.env` and used **`multica_knowledge_20261101_b3c35f9c`**. No runtime-task database was used. The temporary Core validation harness wrote no frontend source and added no dependencies.

## Failures resolved and limits

Intentional missing-symbol/schema tests established the initial red stage. Real regressions caught generated CTE metadata tags and the unnecessary owner/admin project lock; both failed before their fixes and passed afterwards. A source-project test initially placed a synchronous competing editor after a shared actor lock, producing an unreachable test ordering; it now interleaves before that fence and the opposite ordering observes real PostgreSQL blocking. A handler test initially overwrote its chi candidate path parameter when adding the request key; the fixture now attaches both together.

The deliberate missing-source test also needed explicit cleanup of its legacy rows after deleting the issue. Five orphan fixture sources in the exclusive test database were cleaned by exact retained-candidate/missing-issue identity; candidates and new invalidation records were preserved. The cleanup regression and broad realtime/source suite passed afterwards. Temporary C2 compile errors were reported to their owner and subsequent checks passed; no C2 code was edited here.

Main's independent A2 review remains pending. G2 execution/session enablement, F3 final integration, Web/Desktop UI journeys and real-model/platform acceptance remain with the owning phases; this handoff does not claim those broader outcomes.
