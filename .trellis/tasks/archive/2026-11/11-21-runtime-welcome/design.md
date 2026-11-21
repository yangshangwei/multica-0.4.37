# Technical design

Surface scope: the existing runtime-management empty state. Visitor mode:
Operate. Audience/job: a workspace member connecting the first execution
computer and then returning to everyday runtime management.

## Boundary

The gap lives in the shared RuntimesPage empty branch. Add focused RuntimeWelcome and scoped CSS, with `onConnectRemote`, shared i18n, ProviderLogo, Button/AppLink and `daemonRuntimesDocsHref(useWorkspaceSlug())`. Read the existing validated `desktopDownloadUrl` from useConfigStore; show a safe new-tab download anchor when configured, otherwise show the local guide. Two presentational examples describe local desktop discovery and remote setup; show only illustrative `multica setup`, never public shell installers or claims about a fixed command count. Manual slide index is the only new local state.

Preserve buildRuntimeMachines, queries, rows, navigation, dialogs and platform wiring. Retain existing machine/profile/bootstrap/local gate and additionally require successful inventory reads for welcome. If an inventory read fails with no usable inventory, render localized error/retry; cached nonempty inventory remains usable. Keep the existing loading skeleton.

## Direction contract

THESIS: Explain the first runtime inside management, following the supplied restrained introduction and two concrete connection examples.

OWN-WORLD: Inherit AgentWelcome and the application: semantic neutral surfaces, brand-tinted dotted example stages, role-based type, Button/AppLink and ProviderLogo. This extends an established surface without inventing a visual identity.

STORY: Understand where agents execute; choose this computer's desktop discovery or another computer's setup; use the real dialog or bundled deployment-local guide.

FIRST VIEWPORT: Desktop has a generous left intro/headline/explanation with adjacent primary/help actions; right has a full example card and muted next-card glimpse, with dots/arrows below. Narrow containers stack intro and a fitted card and scroll normally. Shell/header/cloud entries remain.

FORM: Precisely specified reference extension, no concept tournament/seed. Code-led native UI examples, explicitly marked as examples. Signature: manual two-card carousel with retained focus and no autoplay.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance. For this ordinary extension, document incumbent-system evidence and preserve global design files rather than creating/replacing DESIGN.md. No new shipping rasters.

## Files and compatibility

- `runtimes-page.tsx`: targeted welcome/error branch.
- `runtime-welcome.tsx`, `.module.css`, `.module.css.d.ts`: isolated shared UI.
- Colocated welcome/page tests: canonical interaction and gate regressions.
- `locales/{en,zh-Hans}/runtimes.json`: additive keys only.
- `e2e/runtime-welcome.spec.ts`: production screenshots and actual dialog/help/list-transition acceptance.

No core/UI API, endpoints, dependencies, migrations or platform contracts change. Rollback removes only these additions and the localized page branch.

## Incumbent system evidence

- The welcome uses the existing RuntimesPage shell, header actions and connection
  dialog. The inventory branch still uses the machine list.
  [Task PRD](prd.md) · [RuntimesPage](../../../../../packages/views/runtimes/components/runtimes-page.tsx)
- Light and dark themes supply the neutral canvas, card surfaces, borders, muted
  text and focus color. The filled action uses `--primary`. The dotted example
  stage mixes `--brand` into the surface (5%) and dots (15%). The cards and inset
  examples use shared radius and shadow tokens.
  [Shared tokens](../../../../../packages/ui/styles/tokens.css) · [AgentWelcome styles](../../../../../packages/views/agents/components/agent-welcome.module.css)
- The intro and examples follow AgentWelcome's two columns at a 60rem container
  width and stack in narrower containers. The shared type roles distinguish the
  display headline (36px), card titles (16px), body copy (14–15px) and example
  labels (11–12px).
  [AgentWelcome](../../../../../packages/views/agents/components/agent-welcome.tsx) · [RuntimeWelcome styles](../../../../../packages/views/runtimes/components/runtime-welcome.module.css)
- RuntimeWelcome uses the default `Button` for Add runtime, ghost styling for
  the download or `AppLink` guide, and outline buttons for the arrows. The
  examples use HTML, CSS, Lucide icons and existing `ProviderLogo` marks. No new
  raster assets ship, so no new asset provenance is needed.
  [RuntimeWelcome](../../../../../packages/views/runtimes/components/runtime-welcome.tsx) · [Button](../../../../../packages/ui/components/ui/button.tsx) · [ProviderLogo](../../../../../packages/views/runtimes/components/provider-logo.tsx)
- The two cards advance only on input. Named dots and arrows show a keyboard
  focus ring, and the endpoint arrow stays focused when disabled. A live status
  announces the selected position and the inactive card is inert. Reduced motion
  disables track, card and dot transitions.
  [Browser checks](../../../../../e2e/runtime-welcome.spec.ts)

The finish packet records production Chromium captures in English and Simplified
Chinese, in both themes, at 320, 390, 1024 and 1440px. The contact sheet includes
the remote example and populated list. See
`.omx/private/qa-work/runtime-welcome-20261121/resume-review-packet.json`.
