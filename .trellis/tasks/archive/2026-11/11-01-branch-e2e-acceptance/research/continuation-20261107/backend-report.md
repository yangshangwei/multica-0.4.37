# Backend continuation result

**390 unique top-level tests passed with race detection; 0 failures, 0 skips.** Including606 subtests,996 named terminals passed. These are focused regressions on candidate `4c817dee9397a42ac903f419af7804b11d2d863e`, not a whole-repository acceptance verdict.

Full `go build -p 2 ./...` and `go vet -p 2 ./...` both exited0. Backend/scripts source identity and pinned Go/Node/psql bytes match the initial snapshot. The concurrent declared E2E disclosure fixture correction is outside the backend fingerprint and did not change backend inputs.

| Group | Top-level PASS | Named terminals PASS |
|---|---:|---:|
| `./internal/service` | 88 | 148 |
| `./internal/handler` | 89 | 235 |
| `./internal/auth` | 1 | 1 |
| `./internal/middleware` | 5 | 24 |
| `./cmd/server` | 8 | 50 |
| `./internal/daemon/collaborationrun` | 59 | 118 |
| `./pkg/agent` | 45 | 117 |
| `./pkg/redact` | 36 | 77 |
| `./cmd/multica` | 2 | 60 |
| `./internal/daemon` | 40 | 138 |
| `./internal/daemon/execenv` | 4 | 10 |
| `./internal/migrations` | 1 | 1 |
| `./internal/service` | 7 | 7 |
| `./internal/handler` | 2 | 2 |
| `./internal/daemon/collaborationrun` | 2 | 7 |
| middleware scoped task CDN cookie refusal supplement | 1 | 1 |

Initial250 and later139 plus the separately identified CDN case are disjoint by package + test name. `backend-final.json` contains all996 unique terminal identities; the original raw runs and exact filters remain separate. Every package terminal passed, every expected top-level name has a terminal, and no unexpected name was added.

Coverage includes immutable launch memory beside live memory (ENV1), checkpoint authority/Stop/Resume and recording recovery, version-bound knowledge approval and revocation, issue status/batch/deletion contracts, scoped task authorization/current authority/real router, Codex knowledge admission and process lifecycle through fake subprocesses, output redaction, partial delegation receipts, delivery-vs-acceptance instructions and the real migration rollback fence. No commercial Provider or account was accessed.

Seven fresh task databases were explicitly allocated, verified empty and migrated; all have0 connections after execution and are retained. All package homes/temp roots live outside Git. The agent CLI guard remained active. No process command still referenced the owned state prefix at final audit. Go1.27.0 differs from CI1.26; database PostgreSQL17.11 and psql18.6 were recorded.

Remaining scope: other backend tests, Linux daemon descriptor regression, Windows/macOS installer behavior, real models and whole browser acceptance. Lint/TypeScript are coordinated by the parent session, not counted here. No product/test source edits, no dependency changes, no weakened assertions, no automatic retries.
