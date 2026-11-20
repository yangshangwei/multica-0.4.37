# Design

## Surface direction

Operate mode; scoped extension of the existing Multica visual system. The user
reference pins a quiet canvas, large left-aligned title, small blue automation
badge, primary dark action, secondary document action and compact neutral icon
cards. Use existing semantic colors, role-named typography, Lucide icons and
shared buttons. No artwork, gradients, invented statistics or global styles.

The settled design was presented before the user said to create and proceed.
Impeccable context was loaded once for packages/views/autopilots. PRODUCT.md and
DESIGN.md are absent; this scoped empty-state change inherits actual tokens and
components and does not introduce a new visual identity.

## Boundary and implementation

The behavior already lives in AutopilotsPage: showEmpty uses the successful,
unfiltered automation response. Keep this branch and the populated list intact.
Extend the shared AutopilotTemplateCatalog with an explicit welcome presentation
so the restoration/focus implementation, icon metadata and schedule formatting
stay canonical. The default full catalog retains filtering and its current
creation guidance. The welcome presentation places the hero left and up to six
compact template cards right. Keep all ten templates reachable in the existing
full catalog route through Browse all templates.

Featured keys: daily-change-review, stale-pr-reminder, bug-triage,
weekly-progress-report, dependency-audit, documentation-check. Keep server order
within the preview; if fewer are supplied, fill spare preview slots using other
server templates in server order so partial/new registries remain useful. Full
catalog continues to include unknown keys. Labels, schedules, prompts and modes
remain server-owned. No template payload is rewritten.

The welcome variant must ignore saved full-catalog category filters, otherwise a
returning category can hide its featured entry. It still writes the existing
scroll/focus handoff before onPick; Back restores the clicked visible card once.
Fallback focus uses the blank action if the target is absent. Keep restoration
after loading/error and do not steal focus from another control or dialog.

Use workspace path builders and shared NavigationAdapter/AppLink for docs and
full catalog navigation. The welcome view has a single scroll owner, container
queries, bounded readable width and vertically balanced content on wide screens.
On smaller containers stack the introduction above the cards; cards become a
single column on phones. No fixed-height clipping or new persisted state.

## Alternatives resolved

- Replacing the whole creation picker would unnecessarily disturb a mature flow.
- A separate cloned catalog would duplicate schedule, icon and focus contracts.
- Showing all templates in the welcome hero would lose the reference's restrained
  six-card composition; the full catalog remains one action away.

## Expected files

- packages/views/autopilots/components/autopilots-page.tsx: route welcome actions.
- packages/views/autopilots/components/autopilot-template-catalog.tsx: welcome
  presentation sharing existing catalog mechanics; small local component only
  if clarity materially benefits.
- Matching page/catalog tests and both locales' autopilots.json.
- e2e/autopilot-template.spec.ts: keep full-gallery restoration and add appropriate
  welcome restoration/CTA/browser evidence instead of expecting welcome filters.
- e2e/progress-reporting.spec.ts and e2e/autopilot-lifecycle-branches.spec.ts:
  update the existing empty-page entry to browse all templates and the renamed
  blank-creation CTA, preserving reporting and lifecycle assertions.
- .trellis/spec/views/frontend/automation-template-discovery.md: document the
  two presentations and preserve shared lifecycle contracts.
