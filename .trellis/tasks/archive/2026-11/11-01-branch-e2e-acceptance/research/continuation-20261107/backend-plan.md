# Current candidate backend acceptance

Candidate: `4c817dee9397a42ac903f419af7804b11d2d863e`; isolated checkout `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107`. No source changes.

The selected tests exercise startup memory attribution and immutable bases, recording/checkpoint replay, Stop and Resume authority, human knowledge approval of exact versions, source purge/revocation, task creation/batch/status/deletion semantics, and current task-capability authentication/resource fences. Native runner tests use task-owned fake providers under the repository agent CLI guard; they do not call a commercial Provider.

Each package uses an exact anchored test-name filter generated from the named current source files. `backend-inventory.json` records all names. The runner executes race detection, count1, p1, parallel2 and a 20-minute package timeout. No retries or assertion changes. Initial cluster headroom:37 connections of100.

| Package | Selected top-level tests | Database |
|---|---:|---|
| `./internal/service` | 88 | fresh owned database |
| `./internal/handler` | 89 | fresh owned database |
| `./internal/auth` | 1 | none |
| `./internal/middleware` | 5 | fresh owned database |
| `./cmd/server` | 8 | fresh owned database |
| `./internal/daemon/collaborationrun` | 59 | none |

API pool environment overrides are absent from ordinary test processes, per the verification spec. GOMAXPROCS is2; DB packages are serial. Every execution has a package-specific HOME/TMP outside Git, with explicit existing Go caches. The migration runner must confirm the selected fresh database before any migration.

Full `go build -p2 ./...` and `go vet -p2 ./...` are separate compile/static gates. These focused regressions do not replace all82 backend packages, complete browser acceptance, real Provider ingestion, or target-OS packaging. Lint and TypeScript checks are coordinated separately. Raw logs and credentials remain under the ignored private backend directory.
