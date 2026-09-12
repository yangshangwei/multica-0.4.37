# Workspace defaults technical design

Worktree: /Volumes/artisan/code/2026/multica-workspace-defaults
Branch: feat/workspace-defaults; base fe38a164b (feat/agent-skills-localization).
User authorized the presented UX and implementation. This document fixes contracts before code.

## Current behavior
Workspace creation seeds owner/statuses; onboarding creates Mika. Template GETs are pure catalogs. provisionSquadTemplate transacts role skills, reuse and membership but always inserts a squad. Project resources support atomic creation. Ordinary issue creation supports squad assignment and trigger preview. Agent quick-create files an issue and is not the execute shortcut. Template autopilots activate on creation.

## Project execution contract
Add project.execution_squad JSONB NOT NULL DEFAULT '{}'. No new FK/index. Parse into concrete Go/TS structs; never persist arbitrary client JSON.

Project response adds optional execution_squad with:
- state: none | needs_runtime | configured | failed
- template_key, squad_id, runtime_id, error_code: optional nullable strings
configured means an instance was assigned, not that its machine is online. UI combines this with current squad/leader/runtime availability. Empty/legacy/malformed values normalize safely.

ConfigureProjectSquadRequest: optional template_key, squad_id, runtime_id, language (en/zh/ja/ko). At most one of template_key/squad_id. Empty object clears. runtime_id only accompanies a template choice. Template without runtime persists needs_runtime. Absent request in legacy project creation preserves current behavior.

POST /api/projects accepts optional execution_squad: ConfigureProjectSquadRequest.
PUT /api/projects/{id}/execution-squad accepts that request, returns complete Project. Repeating the same selection is idempotent; retry resends retained template/runtime. Invalid shape/UUID/template/foreign-workspace input is 4xx. Failed preparation retains project and choice with safe error_code, not raw SQL errors.

## Transactions, ownership and recovery
1. Validate caller and selection using existing project/member/runtime/agent gates, scoped to the workspace.
2. Commit project/resources/validated selection atomically. Then synchronously prepare database configuration; no agent run or new job framework.
3. Lock project row and reuse current workspace/template and role-skill locks. Extract a transaction-aware materializer from provisionSquadTemplate without changing explicit template-creation semantics.
4. Return an already assigned valid squad unchanged. Reuse an accessible, unarchived matching template squad where possible. Never adopt a private leader the caller cannot wire or rebind reused roles to another runtime.
5. Commit new squad/roles/skills and final project reference together. Roll back preparation failures; retain a retryable choice. Serialize project mutations so an older failed attempt cannot overwrite newer configuration.
6. Clear/change does not delete shared resources. Project deletion serializes with preparation. Missing/archived/inaccessible defaults show unavailable; recovery is explicit, never a GET side effect.
7. Preserve template autonomy and private-runtime boundaries. Built-in defaults confer no new permission.

## Shared core
Own types, zod boundary parsing, API client, query keys, configuration mutation, draft choice fields and pure default/readiness helpers in packages/core. Project.execution_squad stays optional for existing fixtures/servers. Mutation awaits server and invalidates project, squads, agents and skills. No server data in Zustand. Request captures wsId to avoid stale-workspace writes.

Exports: ProjectExecutionSquad, ConfigureProjectSquadRequest; api.configureProjectSquad(id,data); useConfigureProjectSquad(wsId); helpers in projects/execution-squad.ts. API route uses PUT.

## UI contracts
- create-project modal accepts data.squad_template_key or data.squad_id. Default uses the actual feature-delivery registry key.
- Add ProjectSquadPicker for template/existing/none and authorized runtime selection. Prefer eligible Mika runtime or sole suitable runtime, matching a chosen local_directory daemon. Otherwise require a choice; do not guess across machines.
- ProjectSquadSection sits above IssueSurface. Explicit execute opens create-issue with project_id, assignee_type=squad, assignee_id, status=todo; server preview determines whether it starts. Show preparing (mutation), needs connection, configured/online, failed/retry, and stale target.
- Runtime connection uses existing runtime page; return to project with selection retained. Changing default affects future task defaults only.
- New-workspace completion gains projects destination, wired through shared type and Web/Desktop callers. Desktop remains a WindowOverlay transition.
- Catalog sections sit above instance filters. Use registry queries and existing presentation/editor paths. Catalog key differs from instance UUID; use real origin/template fields. A UseSquadForProjectDialog selects a project or opens create-project with prefill.
- Automation template page accepts initialProjectId/initialAssigneeType/initialAssigneeId props via platform routes. Seed once, validate against workspace/permissions, preserve user edits. ProjectAutomationsSection sits below resources and links to this flow. Final Enable CTA alone creates automation/trigger. No paused seed instances.

## Parallel ownership
Backend: server migrations/queries/generated, project/squad handler and router, Go tests.
Leader/core: core API/types/query/draft/helpers; new workspace-defaults locale namespace/provider registration; integration/specs.
Project lane: create-project, project selection/section/detail, project list and onboarding platform callers. Own project-detail.tsx mounting.
Catalog lane: squads/agents/skills list/entry components and UseSquadForProjectDialog. Do not edit core/create-project/project-detail.
Automation lane: autopilot list/template pages and their platform routes, ProjectAutomationsSection. Do not edit project-detail.
Locale ownership: project lane owns projects/modals/onboarding; catalog lane owns squads/agents/skills; automation lane owns autopilots, in all four locales. Reuse these existing namespaces; coordinate any shared-file change.

## Validation
Backend regression: no choice/template without runtime/existing/invalid/foreign/private/retry/concurrency/rollback/clear/customized/deleted cases. Core: old/malformed API, default selection, stale workspace, invalidation. UI: default/prefill, setup recovery, task dispatch inputs, catalog identity, automation no write before enable and seed-once. Browser checks use fake runtimes, never real installed agent commands. Run relevant tests first then typecheck/lint/Go tests+vet and targeted E2E. Capture desktop/narrow screenshots and visual verdict before a visual fix.

## Rollback
Additive column and optional response fields permit reverting feature use without deleting user resources. No production deployment is part of this task.

## Contract review corrections (accepted before implementation)
- Once project/resources commit, preparation failure returns 201 Project (PUT returns 200 Project) with state=failed and safe error_code. Do not turn it into a generic 5xx that invites duplicate project POSTs. Idempotency covers configuration/materialization, not arbitrary repeated project creation.
- Protect the create/preparation gap with an internal server-generated selection revision, persisted with the choice and compared under the project row lock. Stale preparation/failure writes return current state instead of replacing a newer selection. PUT may prepare under one transaction/savepoint while persisting its chosen failure state atomically.
- Check actual leader invocation permission as well as wiring permission; admin wiring rights do not grant private-agent invocation. Retain coordinator/autonomy grant gates for agent actors through both new entry points.
- runtime_id records the requested binding. Read actual leader/member runtime IDs for execution readiness; do not rebind reused agents. A project local directory must be usable by the actual execution machine. Surface mismatches as unavailable with a recovery action.
- Update handwritten project search SELECT/Scan along with generated queries. Preserve bundled project-create resources response echoes and standard project counts.
