# Automation Template Discovery

`AutopilotTemplateCatalog` is shared by the empty automation list and the
template creation picker. Keep the populated management list independent of
the catalog, and do not change the generic built-in catalog for this feature.

- Detect first use from the successfully loaded, unfiltered automation list.
  Loading, errors and filtered-out instances are not an empty workspace.
- Presentation groups are local metadata keyed by template key. Preserve server
  ordering and keep unknown templates in All with conservative output wording.
- `run_only` means no issue is created up front. Some patrol templates can create
  or update follow-up issues. Do not promise that they never create tasks.
- Template adoption keeps the prompt, schedule and output mode server-owned.
  Navigation must retain project and assignee defaults; submission creates and
  enables only after the user confirms the configuration.

## Restoring a Two-Step Gallery

The picker and configure steps share a pathname. Desktop captures and replaces
plain scroll entries on every URL transition, including query changes, so
leaving configuration can erase the gallery's ordinary scroll offset.

Before selecting a template, write the gallery's group and scroll offset through
the existing platform view-state adapter. Preserve explicit All and zero values,
and restore only after template content is loaded. A URL containing stale group
or offset defaults must not override the latest saved selection on native Back.
On web, the platform adapter must key render-time reads and view-state writes
by Next's rendered pathname, not the mutable browser URL during navigation.
The empty-page entry uses the allowlisted `return_to=autopilots` marker so the
page's Back button returns to the same route that owns its saved gallery state.

`autopilot-template-catalog.test.tsx` covers restoration when the plain-scroll
adapter returns no entry. `template-create-autopilot-page.test.tsx` covers
origin routing and creation defaults. `e2e/autopilot-template.spec.ts` verifies
native and in-flow Back through the production web application.
