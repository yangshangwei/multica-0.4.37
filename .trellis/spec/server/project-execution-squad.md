# Project execution squads

## Contract and boundaries

Project execution choices are stored as an ordered array in the existing server-owned `project.execution_squad` JSONB column; no new migration is needed. Readers accept legacy singleton objects and normalize malformed storage to an empty list. Project GET/list/search/create/configure responses expose `execution_squads: []`; the legacy `execution_squad` is the first item or `{state: "none"}`. The response states are none, needs_runtime, configured and failed; configured describes setup, not current liveness. template_key/squad_id/runtime_id/error_code are optional. The internal selection revision is never public.

POST /api/projects optionally accepts `execution_squads` or legacy `execution_squad`, never both. Each choice takes template_key XOR squad_id, optional runtime_id for a template and a label language. No input means an empty list. PUT /api/projects/{id}/execution-squads accepts `{squads: [...]}` and replaces the entire ordered list; `[]` clears. Missing/null arrays, empty entries and duplicate choices are invalid. The legacy singular PUT keeps its whole-list replacement semantics; `{}` clears. Template and existing-instance aliases resolving to the same configured squad are collapsed, keeping the first choice. Bad shape/UUID/template/foreign resources or denied access are rejected before project creation. Once a project commits, failed preparation returns a successful Project response with state=failed, retaining the project and selection. Arbitrary project POSTs are not idempotent; the configuration operation is.

## Atomic materialization

New label languages are `en` and `zh`. Retired `ja`/`ko` requests and
stored deferred selections are interpreted as English. Normalize old singleton
objects and array entries in memory, preserving revision and identity; GET must
not rewrite the JSON. A configuration retry still reuses customized instances
without replacing their names, descriptions or instructions.

Use the existing role/skill/squad materializer inside the configuration transaction. Project row locks serialize configure/delete. Acquire every requested runtime in UUID order before agents, then all requested template locks in key order, then the workspace role-skill lock. Never acquire the role lock between template locks: a concurrent single-template creation can hold the next template while waiting on the role lock. Each choice owns a savepoint, so a failed item removes its partial resources while successful choices commit together with the project list. Compare the complete ordered selection, including each internal revision, across the create/preparation gap; checking only the first revision loses concurrent changes to later choices. Full-list retries match template_key/runtime_id or squad_id to preserve existing instance identity and customization.

The explicit squad from-template endpoint retains its existing creation behavior. Project preparation alone reuses a valid accessible template squad. Never rebind or overwrite reused agents, skills or squad instructions. Clearing the default does not delete shared entities. Reads do not resurrect deleted/archived defaults.

## Authority and actual execution location

Invocation and wiring are different checks: admin editing/wiring privileges do not grant private-agent invocation. Preserve agent actor Coordinator/may-grant-autonomy gates and authorizing-human provenance. Private runtimes are owner-only for binding, including workspace administrators.

Requested runtime_id is not proof of the effective machine: reused agents retain bindings. Check actual runtime/machine coverage for all squad agents. The UI must repeat availability checks against a complete current roster; initial successful setup does not prove readiness after a member is moved or archived.

## Client parsing and recovery

Use the canonical project/resource schemas. Malformed execution config is unavailable, not ready. A local_directory requires nonempty string daemon_id and local_path; malformed resource responses fail the query instead of becoming an empty restriction list.

Capture workspace identity for requests and use workspace-scoped mutation keys so an in-flight observer cannot replace callbacks with another workspace's closures. Do not navigate or clear a newer draft after a stale creation result.

Pending or failed prerequisite queries block dispatch. A deleted default's roster may fail permanently: that blocks dispatch, but must not block explicitly replacing or clearing the default. Automatic retry needs a retained template or squad; an empty request is a clear, not a retry.

## UI semantics

Project lead remains separate from execution squads. The list offers candidate squads; selecting several never broadcasts one task to all of them. Each task still has one selected assignee. Project-local new-issue defaults may inherit the first squad, but explicit grouping, saved-view assignee constraints and user overrides win. Assignee type/id must be treated as a pair when higher-priority defaults replace or clear them. Existing issues are never reassigned by changing the project default.

The project canvas shows a bounded squad summary; full candidate descriptions, runtime details and configuration actions live in the Manage squads sheet. Readiness counts and the closed-sheet default warning use the same prerequisite queries and complete-roster checks as each management row. Loading, paused and failed checks must not imply ready. Configuration writes share one pending guard; a permanently missing roster still permits replacement/removal.

Set as new issue default replaces the full ordered selection with that candidate first, preserving every other candidate and each template/runtime provenance value. Removing a candidate removes only its project association. Neither operation changes existing issue assignees.

The explicit New issue with this squad menu action closes and unmounts the manager before opening ordinary issue creation with project/squad/todo, using the server trigger preview. This avoids stacking modal focus traps. Agent quick-create files an issue and is not an execution shortcut. No squad dispatch happens while browsing or configuring candidates.

ProjectIssueSurface routes the single primary creation action through the canonical IssueSurface controller, preserving saved-view/grouping/explicit-assignee precedence. Authoritative `project.issue_count === 0` enables the first-issue state for table and Gantt as well as board/list. Loading and status-catalog errors take precedence; active filters, actor tabs, saved views and table searches retain their recovery/navigation controls.

Link an existing issue offers only same-workspace issues with no current project, so an association never silently moves an issue from another project. It awaits the existing issue update mutation and sends only the issue ID and new project ID; assignee/status are unchanged and normal issue/project query invalidation owns refresh. IssuePickerModal accepts synchronous or awaited selection callbacks, prevents duplicate writes/dismissal while pending, and closes only after success. A rejected/false selection remains retryable, and a stale result cannot close a newly opened picker.

Catalog templates remain separate from workspace instances and instance counts. Skill provenance is informational, never authorization. Automation templates are only materialized on explicit Enable; project navigation prefill must not overwrite user edits.

## Regression owners

- server/internal/handler/project_execution_squad_test.go, project_execution_squads_test.go and cmd/server/project_execution_squad_test.go: authority, retry, rollback, concurrency and full response contract.
- packages/core/api/project-execution.test.ts and projects/execution-*.test.*: parsing, workspace capture, defaults and runtime/roster readiness.
- packages/views/projects/components/project-squad-*.test.tsx and modals/create-project*.test.tsx: flow, stale recovery and future task defaults.
- e2e/workspace-defaults.spec.ts: real configuration, fake-runtime enqueue and explicit automation enable; never run an installed agent CLI.
- e2e/project-squad-workspace.spec.ts: compact 1/8/20-candidate presentation, keyboard sheet management, ordered defaults, explicit creation, association recovery, table/Gantt emptiness and narrow layouts with real writes and a fake runtime.

## Concurrent project-resource mutations

### Scope and signatures

`CreateProjectResource` and `UpdateProjectResource` use
`runProjectTransactionAtIsolation(..., pgx.ReadCommitted, callback)`, then
`LockProjectForExecutionSquad` (exclusive project `FOR UPDATE`). The shared
workspace/member/subscriber fences precede this project lock.
`findLocalDirectoryConflict(ctx, qtx, ...)` must use transaction-bound queries.

### Contract

Read the resource set/row in a separate query **after** obtaining the project
lock. Merge partial request fields and validate daemon uniqueness against that
fresh state on every retry. Only one local directory per project/daemon may
commit. Omitted label/ref/position fields retain the latest stored value; preserve
explicit label clears, embedded legacy labels, unknown JSON keys and old-client
rename-only worktree exemptions. Publish exactly once after successful commit.

READ COMMITTED is deliberate: a waited-for lock under REPEATABLE READ can retain
a snapshot predating the previous child insert, because resource mutations do
not update the parent tuple. The write CTE's late lock alone is insufficient.

### Validation and examples

| Condition | Result |
|---|---|
| Concurrent different paths on the same daemon | One success, one 409 |
| Concurrent resources on different daemons | Both succeed with current append positions |
| Execution-ref edit plus label/position-only edit | Both changes survive |
| Row/project deleted before the locked read | 404 |
| Write/commit failure | No success event |

Good: lock, fresh read, merge, validate, write, commit, publish. Base: sequential
legacy rename behavior stays unchanged. Bad: validate before locking or merge
from the handler's pre-transaction snapshot. Existing duplicate rows are not
automatically repaired; future direct writers must join the same invariant.

### Tests required

`project_resource_concurrency_test.go` uses distinct active users, dedicated
connections and an observed direct/transitive PostgreSQL lock dependency before
releasing the fixture lock. Same-user-only races can be serialized by the earlier
subscriber fence and hide the bug. Sleeps alone are not a concurrency barrier.
Retain existing rename, worktree capability and execution-snapshot tests.
