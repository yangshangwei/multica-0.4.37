# Agent discovery technical design

## Approved direction

The user approved the prior business review and template/member treatment, then asked to create and progress this task. Execute the first identification iteration, preserving the current restrained Operate UI, list virtualization and platform navigation. No additional product decision is open.

## Directory

Keep saved agents as the default page and remove BuiltinAgentCatalog, including the duplicate error-state placement. The existing New agent → Use template route owns the catalog.

Group rows by role kind (coordinator, specialist, other) by default, with a Display setting to return to an ungrouped list. Group headers carry counts; sorting remains available within groups. Visible role narrowing and a squad selector expose the business model without requiring a new taxonomy. Search matches the saved name/description plus known localized role-template title and associated squad names. Unknown or custom agents remain in Other and All; no capability inference from editable names, emoji or autonomy policy.

Resolve specialist role metadata from useRoleTemplates(), and coordinating roles from useSquadTemplates().leader. The label identifies template provenance, not enforced current capability; retain user-authored descriptions. Membership chips show squad names and actual leader relationship, with real AppLinks using rowLinkInteractiveProps. No extra hidden agent reads.

Fresh preferences show identity, status and recent activity; owner/access/runtime/runs/model/created remain opt-in. Existing hiddenColumns must be retained exactly. Display must be named explicitly, preserve selection visibility and provide grouping/sorting/columns. Clarify 30-day statistics. Scope and filters compose: setScope never clears filters; filters never implicitly change scope.

## Full squad membership contract

Add optional Squad.agent_member_ids to the existing API response and client schema. Current servers return the complete list (including leader, agent members only), collected from the already-batched member preview query before preview truncation. Preserve the three-person preview. No SQL migration, extra per-squad server query or permission changes.

Absence/null/invalid arrays remain undefined, not an asserted empty list. Older servers: show only confirmed leader relationships until complete data is available; when a squad is selected and its full array is absent, load its member endpoint through squadMembersOptions and the existing workspace-scoped query key. Gate results while needed memberships are pending, and show retry on error instead of false empty results. Parse the member endpoint; malformed payloads fail rather than becoming empty. Existing squad realtime invalidations cover the cache subtree.

Core API:
- AgentRoleKind = coordinator | specialist | other.
- buildAgentRoleMetadata(roles, squadTemplates), resolveAgentRole(agent, metadata).
- buildAgentSquadMemberships(squads, resolvedMembers?) returns byAgent and incompleteSquadIds.
- AgentListFilters adds roles and squads, empty arrays inactive.
- AgentsViewState adds groupBy: role | none and setGroupBy.

## Template picker

Keep the existing two-step ?template= route and configure/submit flow. Query workspace agents only in the picker. Match active instances by template_key; display all renamed instances, omit archived ones. Use non-interactive card containers with separate real AppLinks for existing members and explicit creation. Opening a member never creates an agent or adds a squad member. Keep squad query parameters on creation links.

Render instance loading/error distinctly from zero instances and offer retry. Keep template failure/empty recovery. Preserve gallery scroll using existing restored-view-state and view-state-writer adapters before opening configuration or existing members, restoring after real content loads. Remove the now-unused simple BuiltinAgentCatalog and dead catalog locale keys.

## Boundaries and ownership

Data lane: server/internal/handler/squad.go and canonical tests; core squad types/schema/client/query and relevant tests; core agents/discovery.ts and view-store with tests/exports.
Template lane: views/agents/create/template-create-agent-page.tsx + tests, new picker-specific helper/component if needed, e2e/agent-role-template.spec.ts. No shared locale file edits.
Directory lane: views/agents/components/agents-page.tsx, agent-list-toolbar.tsx and canonical tests; remove obsolete catalog; new directory-only view component if justified. No core or locale edits.
Leader: locale additions in all four languages, docs/spec/task artifacts, end-to-end environment and integration verification. All lanes must preserve others' edits and report scope changes.

## Validation and rollback

Add meaningful regressions before behavior changes. Test full memberships beyond preview, custom/unknown/renamed roles, shared specialists, filter/scope composition, old preferences, template instance matrix and non-mutating navigation. Use existing Query and navigation providers rather than duplicating business matrices in DOM tests.

Run scoped Vitest, Go handler tests, impacted package lint/typecheck, API/schema tests and source static checks. Build an owned production web environment before Playwright. Record source identity, screenshots at 1440/768/390 where applicable, errors, and a visual verdict. An independent check agent reviews the integrated changes and fixes verified findings. Revert the task's atomic commit to roll back; no data migration or entity rewrites occur.
