# P0 isolated execution environment and default regression

2026-11-14. This records implementation-phase tests after approval of
`2026-11-14-v1`; it does not replace the earlier planning-only evidence.

## Scope and ownership

The environment agent changed only the existing router startup configuration
test and task-local evidence tooling. The production router, Compose and `.env`
defaults are unchanged; their planned change remains P5. Existing work in the
shared checkout was preserved.

The test now separately checks unset, empty, literal `true`, literal `false`, and
invalid nonempty `1`. It exercises `NewRouterWithOptions` and the resulting
router-owned `TaskService.KnowledgeContextEnabled`, rather than duplicating the
configuration expression in a test helper.

## Allocation and execution contract

`run-owned-go.py` is a task-private launcher. It reads only `DATABASE_URL` from
the checkout's `.env` as literal data, rejects interpolation/control characters
and nonlocal hosts, and uses the explicit `postgres` administrative database
only to create new uniquely named task databases. It never drops or reuses a
database. Every run verifies `current_database()` before migration and again
before tests. Each package gets a fresh database with all current migrations.

- PostgreSQL client: `/opt/homebrew/opt/libpq/bin/psql` (keg-only libpq18.6).
- Verified cluster: local PostgreSQL17.11, pgvector PostgreSQL17 image,
  x86_64 server on the macOS arm64 host; 100 maximum connections.
- Initial read-only cluster probe: 30 connections, database creation permitted.
- Go: `go1.27.0 darwin/arm64`.
- State root: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-knowledge-automation-20261114`.
  The launcher verifies this exists outside every Git checkout. Each run has
  its own mode0700 HOME and temporary directories; normal Go cache/module paths
  remain explicitly pinned.
- Environment is constructed from an allowlist. Model/provider keys, mail,
  publishing, inherited PostgreSQL, account and task-root configuration are not
  forwarded. The synthetic signing key and explicit test knowledge/integration
  switches contain no user credentials. The default-value test overrides only
  its own knowledge setting, including actually unsetting it.
- Repository agent-CLI guard wraps each test process. Only explicit package
  selection is accepted; model smoke tags and arbitrary extra Go arguments are
  not supported. An empty test run is refused.
- A nonblocking task lock permits one database suite at a time. Each allocation
  requires at least30 free cluster connections. It never changes shared server
  limits or terminates another connection.
- Raw migration/test logs and the private environment JSON stay under the
  external state directory. Public task evidence contains database identity,
  exact command, migration ledger, named run/pass/fail/skip inventory, source
  fingerprints and post-run owned connection count; no database credentials.

Runnable examples, executed serially:

```sh
python3 .trellis/tasks/11-14-project-knowledge-automation/research/run-owned-go.py \
  --package ./cmd/server --run '^TestKnowledgeContextDefaultEnabled$' \
  --label p5-default-green

python3 .trellis/tasks/11-14-project-knowledge-automation/research/run-owned-go.py \
  --package ./internal/service --run '^TestKnowledgeAutomation' \
  --label p3-automation --race --timeout 40m
```

Allowed package permits are `./cmd/server`, `./internal/handler`,
`./internal/service`, `./internal/migrations` and `./internal/testutil`. Every
invocation creates a new run ID, database and evidence JSON. The caller must
inspect the named test inventory and source-stability flag; process exit alone
does not establish final acceptance. This launcher is for focused iterations,
not a claim that the repository-wide `make check` gate has run.

## Default regression: expected red

Evidence: `p0-default-red-c10050f1.json`.

- Database: `mca_knowledge_router_ca044f4c7bd6`.
- Migrations:831 ledger entries, latest843
  `collaboration_stage_execution_ready`; migration exit0.
- Six named events: the parent and five cases. Three passing cases
  (`enabled`, `disabled`, `noncanonical`); two failing cases (`unset`, `empty`)
  plus their failing parent; zero skips.
- Both failures are exactly `startup knowledge gate = false, want true`.
- Source fingerprint stable across this execution; HEAD
  `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b`.
- Zero connections to the owned database after the test process exited.

This is intentional P0 failure evidence. AC1 is not implemented or passing.
No real model, user account, API deployment or production environment was used.

## Existing service regression baseline

Evidence: `p0-service-baseline-cc390f65.json`; fresh database
`mca_knowledge_service_4f76f467f235`, same831-entry migration ledger through843.
The following four top-level tests and all their subcases passed:19 named runs,
19 passes, zero failures/skips, stable source fingerprint, and zero owned
connections after completion.

- `TestKnowledgeApprovalMatrixPendingWithdrawalBothCommitOrders`
- `TestKnowledgeApprovalMatrixRecoveryRejectsOtherCandidateAndKinds`
- `TestKnowledgePreparationSourceRevocationBothCommitOrders`
- `TestKnowledgeServerFailureLogsUsePersistedState`

These preserve existing human decision ordering, original-operation identity,
source revocation ordering and private failure-log behavior. They do not test
the planned automation, v2 protocol or reacceptance behavior.
