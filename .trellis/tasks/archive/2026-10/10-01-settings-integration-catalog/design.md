# Settings integration catalog implementation design

The approved visual reference is `preview.html` and its screenshots. The 2026-10-01 user request authorizes production implementation; the previous design-only scope is superseded.

## Shared surface

Keep the existing SettingsPage and NavigationAdapter. Integrations defaults to a wide categorized catalog; a selected provider renders its existing Tab in a narrower detail view, with a branded heading and an All integrations link. Categories stay code hosting, task management, communication. ONES, Plane, Kaneo and Fuxin are inert planned cards. GitHub always appears; self-hosted Git and messaging respect the existing deployment switches. Existing enabled messaging providers remain reachable through cards in communication. Preserve the enabled/configured Composio entry separately, without adding the removed applications/tools category or changing its business form.

## Navigation and data

Use `?tab=integrations&integration=github` (and other existing provider IDs). Card navigation pushes; switching the main settings tab replaces and clears integration. Preserve unrelated query parameters and fragments. Resolve the backend GitHub installation callback `?tab=github` to GitHub detail, and the existing Lark callback to Lark detail when available. Unknown, planned or disabled providers render the directory and a localized unavailable notice, never their form. Retain the requested URL because deployment configuration may arrive after authentication; do not redirect away before it can enable the provider. URL state is authoritative; refs may retain the last visited card for focus restoration.

Reuse React Query options, cache and workspace membership. Catalog errors show status unavailable, loading shows loading, a disconnected state requires successful data. GitHub connection and master-switch state are independent. No API, backend, new global state or dependencies. Existing provider forms retain their permissions and connection behavior.

## Accessibility and layout

Use shared semantic tokens, existing marks and Lucide symbols for planned placeholders. Whole live cards are AppLinks; planned cards have no links, controls or arrows. Two columns only when the content area is wide enough, otherwise one column. Wrap long descriptions and account names; maintain focus indicators and restore focus to the prior card on returning. Detail is headed at h2, provider sections at h3.

## Verification

Canonical route/visibility matrices use node tests; shared component tests cover real navigation wiring, focus, no eager forms, data states and permission/master-switch regressions. Verify shared package lint, typecheck and tests plus web/desktop typecheck. Browser checks exercise application UI, desktop adapter behavior where feasible, responsive layouts, locale parity, navigation and mocked API interactions without real OAuth grants.
