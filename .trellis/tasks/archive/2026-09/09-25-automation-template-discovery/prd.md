# Automation template discovery

The user approved the recommendation in `.omx/plans/automation-template-ui-proposal.md` and requested implementation.

## Requirements
- Empty workspaces show the template gallery as the page body; existing automations keep the management list as the primary surface.
- Use one gallery in the empty page and the template creation flow, with three scenario filters, readable cards, schedules, output hints and a recommended starting point.
- Keep all server-provided templates discoverable, including unknown templates, and preserve server ordering.
- Keep blank creation accessible, preserve configuration defaults and template provenance, and label final submission as create and enable.
- Retain loading, retry, empty and permission behavior; do not mistake filtered lists or errors for an empty workspace.
- Share the implementation across web/desktop and all four locales.

## Acceptance
- Regression tests cover empty/nonempty/error page states, filtering and unknown templates, template navigation, and the unchanged creation contract.
- Relevant tests, typecheck, lint and static boundary checks pass.
- Browser checks confirm responsive cards, light/dark appearance and the selection flow.
- No changes to backend APIs, dependencies, or unrelated squads work.
