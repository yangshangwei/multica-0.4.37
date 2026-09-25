# Project execution squads

## Contract and boundaries

Project execution choices are stored as an ordered array in the existing server-owned `project.execution_squad` JSONB column; no new migration is needed. Readers accept legacy singleton objects and normalize malformed storage to an empty list. Project GET/list/search/create/configure responses expose `execution_squads: []`; the legacy `execution_squad` is the first item or `{state: "none"}`. The response states are none, needs_runtime, configured and failed; configured describes setup, not current liveness. template_key/squad_id/runtime_id/error_code are optional. The internal selection revision is never public.

POST /api/projects optionally accepts `execution_squads` or legacy `execution_squad`, never both. Each choice takes template_key XOR squad_id, optional runtime_id for a template and a label language. No input means an empty list. PUT /api/projects/{id}/execution-squads accepts `{squads: [...]}` and replaces the entire ordered list; `[]` clears. Missing/null arrays, empty entries and duplicate choices are invalid. The legacy singular PUT keeps its whole-list replacement semantics; `{}` clears. Template and existing-instance aliases resolving to the same configured squad are collapsed, keeping the first choice. Bad shape/UUID/template/foreign resources or denied access are rejected before project creation. Once a project commits, failed preparation returns a successful Project response with state=failed, retaining the project and selection. Arbitrary project POSTs are not idempotent; the configuration operation is.

## Atomic materialization

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

The explicit Hand to squad action opens ordinary issue creation with project/squad/todo, using the server trigger preview. Agent quick-create files an issue and is not an execution shortcut.

Catalog templates remain separate from workspace instances and instance counts. Skill provenance is informational, never authorization. Automation templates are only materialized on explicit Enable; project navigation prefill must not overwrite user edits.

## Regression owners

- server/internal/handler/project_execution_squad_test.go, project_execution_squads_test.go and cmd/server/project_execution_squad_test.go: authority, retry, rollback, concurrency and full response contract.
- packages/core/api/project-execution.test.ts and projects/execution-*.test.*: parsing, workspace capture, defaults and runtime/roster readiness.
- packages/views/projects/components/project-squad-*.test.tsx and modals/create-project*.test.tsx: flow, stale recovery and future task defaults.
- e2e/workspace-defaults.spec.ts: real configuration, fake-runtime enqueue and explicit automation enable; never run an installed agent CLI.
