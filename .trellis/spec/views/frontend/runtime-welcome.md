# Runtime welcome and inventory recovery

## Scope

Read before changing the shared `RuntimesPage` first-use branch or
`RuntimeWelcome`. Web and Desktop use the same component; the view extends
the existing application rather than adding a route or persisted onboarding flag.

## State contract

`RuntimesPage` derives welcome from `runtimeListOptions(wsId)` and
`runtimeProfileListOptions(wsId)`. Both reads must be successful and there must
be no grouped machines, orphan profiles, local-machine placeholder or active
bootstrap. Count offline machines as inventory. Use query `isPending` for initial
readiness so paused reads do not become empty data.

| Observation | Required presentation |
| --- | --- |
| Both inventories successfully empty | Welcome and connection examples |
| Initial runtime/profile pending | Existing page skeleton |
| Inventory error with no usable inventory | Localized recovery state; retry both reads |
| Cached nonempty inventory and refresh error | Existing machine/profile content |
| Desktop local placeholder or daemon bootstrap | Existing machine/startup presentation |
| Pending custom profiles | Existing pending profile list |
| Runtime registration/deregistration notification | Invalidate scoped runtime queries; derive presentation from the refreshed inventory |

Do not render welcome merely because defaulted `data = []` is empty. A failed
read does not prove the user's computers were removed. Do not parse event
payloads as inventory or keep a separate welcome-completed preference.

## Actions and example presentation

`RuntimeWelcome({ onConnectRemote })` opens the existing ConnectRemoteDialog
through its callback. The optional validated `desktopDownloadUrl` from
`useConfigStore` supplies a real new-tab anchor with `noopener noreferrer`.
Without a configured download, use
`daemonRuntimesDocsHref(useWorkspaceSlug())` through AppLink. Never substitute a
hardcoded public download host or send a workspace-local path through an
external anchor. Keep the deployment and system-specific connection commands in
the dialog; the welcome terminal is explicitly illustrative.

The welcome inherits AgentWelcome's semantic surfaces, typography, native
controls, provider logos and responsive container layout. Keep the two examples
manual, name controls in the current locale, expose only the active card to
assistive navigation with `aria-hidden` and `inert`, and announce its position.
Use 44px controls and remove card/track transitions under reduced motion.
Pass `focusableWhenDisabled` to the shared arrow buttons so Base UI exposes
`aria-disabled` at an endpoint and the just-activated arrow retains keyboard
focus. Use the ARIA state for disabled styling and keep the focus ring visible.
Scope the styles to the welcome CSS module; do not modify global tokens.

## Canonical verification

- `runtimes-page.test.tsx`: real QueryClient/API-mock gates, empty/error recovery,
  cache invalidation, offline list, cached-data failures and retained Desktop/
  pending-profile behavior.
- `runtime-welcome.test.tsx`: callback, workspace help, reactive configured
  download, keyboard/position controls, inactive examples and Chinese copy.
- `e2e/runtime-welcome.spec.ts`: production Web, both locales/themes,
  1440/1024/390/320 widths, computed card radii/overflow, real dialog and bundled
  guide, reduced motion, and keyboard focus retained on each disabled endpoint
  arrow after activation. Synthetic API registration/deregistration exercises
  WebSocket invalidation and the switch to a navigable machine list.

The browser fixture starts no daemon, installed agent CLI or model. Shared
Desktop consumption additionally requires typecheck; a Web capture does not
establish native Electron or installer acceptance.
