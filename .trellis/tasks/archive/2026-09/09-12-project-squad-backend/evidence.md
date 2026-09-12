# Backend implementation evidence

Worktree: `/Volumes/artisan/code/2026/multica-workspace-defaults`
Branch: `feat/workspace-defaults`
Completed: 2026-09-12

## Delivered behavior

- Added `project.execution_squad` as JSONB, with no new foreign keys, indexes, or dependencies. Existing rows normalize to `state=none`.
- `POST /api/projects` accepts an optional selection and preserves the resource creation echo. `PUT /api/projects/{id}/execution-squad` returns a complete project, including issue, done, and resource counts. GET/list/search expose the same additive field.
- Requests choose `template_key` or `squad_id`; `runtime_id` only accompanies a template. Empty selection clears. A template without a runtime remains `needs_runtime`.
- Invalid shapes, UUIDs, templates, foreign-workspace references, and denied runtime/leader access are rejected before creating a project. Once project creation commits, preparation failures keep the project and retry input and return 201 Project; failed PUT preparation returns 200 Project.
- A server-generated, non-public selection revision protects the create/preparation gap. Project row locking serializes preparation, configuration, and deletion. Savepoints roll back all newly prepared roster/skill rows on failure; the final reference and new rows commit together.
- Repeated configuration reuses an invocable, unarchived squad and leaves `updated_at` unchanged when the saved configuration is already valid. Clear/change does not delete shared resources. Reads never recover stale targets implicitly.
- Reused roles and skills keep their customized content and runtime bindings. Both actual invocation and wiring permission are checked. Machine actors retain Coordinator and may-grant autonomy checks; invocation uses the task's human originator.
- Every actual squad agent binding is checked. Requested-runtime mismatches fail safely instead of rebinding roles. Projects with local directories need a workspace-scoped matching resource for each actual machine. An offline but bound runtime may still be configured.
- The explicit `/api/squads/from-template` endpoint retains its existing materialization semantics. Only its transaction body and lock helper were extracted for reuse.
- Updated the two approved built-in project skill/source-map files with this API contract; no CLI flags were added.

Safe preparation error codes: `agent_name_conflict`, `agent_access_denied`, `agent_unavailable`, `runtime_unavailable`, `runtime_mismatch`, `squad_unavailable`, `preparation_failed`.

## Regression-first execution

The cleanup/extraction plan was to keep the existing template behavior protected, extract only its transaction-owned materialization and lock body, and add a project-only validation callback. Existing role and skill reuse logic remains unchanged.

Before implementation, new POST regressions failed because the server ignored the selection, accepted invalid choices, and failed to preserve retry state. This was observed in `/tmp/multica-project-execution-red.log`.

Further regression runs caught malformed stored UUIDs being exposed as configured and a foreign-workspace resource row affecting machine validation. The fixes respectively normalize invalid stored IDs and use `ListProjectResourcesInWorkspace`. Their observed failures are recorded in `/tmp/multica-project-execution-expanded-red.log` and `/tmp/multica-project-execution-final-matrix-red.log`.

The final matrix covers no selection, template without runtime, existing squad, invalid/foreign/private input, retries, unchanged timestamps, shared customized resources, concurrent preparation, stale selection revision, delete-before-preparation, SQL/link rollback, actual runtime/directory mismatch, archived/deleted/unbound targets, machine autonomy/originator rights, a single-connection pool, and complete GET/list/search/PUT responses.

## Environment and verification

Inspected `scripts/init-worktree-env.sh` and `scripts/ensure-postgres.sh` before setup. Verified that the existing Docker PostgreSQL listener owns localhost:5432, then created only this checkout's previously absent database, `multica_multica_workspace_defaults_916`. No other database was reset or dropped. The generated `.env.worktree` is ignored by Git. No environment secret values were printed.

All Go commands ran from the worktree's `server/` directory with:

```sh
set -a
source ../.env.worktree
set +a
```

| Verification | Result |
| --- | --- |
| `go run ./cmd/migrate up` | Passed through migration 456 on the isolated database. |
| `make sqlc` from the worktree root | Passed using pinned `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`. |
| `bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/handler ./cmd/server -run 'TestProjectExecutionSquad\|TestCreateSquadFromTemplate' -count=1` | Passed, including existing explicit template creation regressions. |
| `bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/handler ./cmd/server -run '^TestProjectExecutionSquad' -count=1 -v` | Passed: 24 top-level handler/router tests; no skipped project execution tests. |
| `go vet ./internal/handler ./internal/service ./cmd/server ./cmd/migrate ./pkg/db/generated` | Passed. |
| Full `scripts/test-go.sh`, with the subprocess environment adjustment below | Passed: 65 packages, plus 9 packages with no tests. The guarded `pkg/agent` fake-subprocess suite also passed. |
| `bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/handler -run '^TestProjectExecutionSquad_' -count=1` | Passed, 5.517 seconds; no race reports. |
| `git diff --check -- server` | Passed. |

The initial full run encountered an existing test-harness assumption in `internal/daemon/repocache/TestGitEnv`: it expects no pre-existing counted Git config, while the Codex process supplies `GIT_CONFIG_COUNT` and indexed config entries. The implementation of `gitEnv` deliberately preserves those entries. No unrelated source was changed. The full run passed with those variables removed only from its test subprocess:

```sh
python3 - <<'PY'
import os
import subprocess
import sys
child_env = {
    key: value for key, value in os.environ.items()
    if key != 'GIT_CONFIG_COUNT'
    and not key.startswith(('GIT_CONFIG_KEY_', 'GIT_CONFIG_VALUE_'))
}
sys.exit(subprocess.run(['bash', '../scripts/test-go.sh'], env=child_env).returncode)
PY
```

Logs:

- `/tmp/multica-project-execution-matrix-green.log`
- `/tmp/multica-project-execution-focused.log`
- `/tmp/multica-project-execution-vet.log`
- `/tmp/multica-workspace-defaults-go-tests-clean-env.log`
- `/tmp/multica-project-execution-race.log`
- `/tmp/multica-project-execution-sqlc.log`
- `/tmp/multica-project-execution-migrate-456.log`

## Changed files

- `server/migrations/456_project_execution_squad.{up,down}.sql`
- `server/pkg/db/queries/{project,runtime,squad}.sql`
- `server/pkg/db/generated/{models.go,project.sql.go,runtime.sql.go,squad.sql.go}`
- `server/internal/handler/{project.go,project_execution_squad.go,squad_template.go}`
- `server/internal/handler/project_execution_squad_test.go`
- `server/cmd/server/{router.go,project_execution_squad_test.go}`
- `server/internal/service/builtin_skills/multica-projects-and-resources/SKILL.md`
- `server/internal/service/builtin_skills/multica-projects-and-resources/references/projects-and-resources-source-map.md`

## Review handoff and limits

Backend source and approved built-in docs are frozen for leader review. No commits, pushes, deployments, real-agent smoke tests, or user-installed agent CLI execution were performed. Configured records intentionally require current squad/agent/runtime availability checks in the UI; workspace resources may subsequently be edited or archived through their existing management paths. The leader owns frontend integration, Trellis spec updates, and final commits.
