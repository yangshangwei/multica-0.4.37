# Common icon rail browser and native journey

Owned changes: `e2e/sidebar-icon-rail.spec.ts` only, plus this research note.
Existing `global-scope-navigation.spec.ts` and fixtures remain unchanged.

Two original journeys run the same shared product behavior in production Web
and the existing actual Electron renderer/native-IPC fixture. Main supplies the
stable build and isolated API/database; this authoring slice starts no runtime.

The tests exercise:

- Painted 64px icon and spacer widths, remembered 300px expanded width,
  current destination, per-control hit testing and accessible action names.
- Native toolbar no-drag controls, active-tab clearance and the remaining
  136px main-header padding beside the 64px rail.
- Common chat and agent navigation, including a closed full AI directory;
  actual Tab-driven tooltip focus, More keyboard dismissal and focus return.
- Native More submenus and MCP navigation, with current-location feedback;
  disabled Iterations/Triage do not gain an overflow shortcut.
- Three retained notification rows read by the real protected account endpoint,
  notification-page access, account-to-workspace ordinary creation, and
  collaboration dialog source and visible More focus restoration.
- Workspace selection without forcing full navigation, viewport auto-collapse,
  manual reopening in its band and restored custom expanded width.
- Full compact Sheet while large-screen state is collapsed, account query-only
  dismissal, actual 200% native zoom, and screenshots in light/dark, Chinese,
  expanded-menu and compact presentations.

Setup uses `TestApiClient` with isolated synthetic accounts and two workspaces.
Only the specific retained notification/participation/session facts are seeded
directly in the task database, matching the existing original scope journey;
they are removed by exact IDs before normal workspace deletion. No grant,
provider, runtime, daemon, command or installed-agent access is created.

Required environment:

- `COLLABORATION_E2E_API_URL`, `NEXT_PUBLIC_API_URL`, `DATABASE_URL` and
  `PLAYWRIGHT_BASE_URL` bound to the task-owned local API/database/production Web.
- For Desktop: `COLLABORATION_ELECTRON_RENDERER_URL`,
  `COLLABORATION_ELECTRON_SOURCE_ROOT` and its matching built preload/rendered
  output; API CORS/WS allowlists include that loopback origin.
- Existing password mode is available through `E2E_PASSWORD_AUTH=1`; otherwise
  the standard task-only legacy fixture login is used.

Run with one worker and zero retries alongside the unchanged original scope
journey. Screenshots and geometry are attachments on each original test. This
proves browser/rendered native layout and interaction, not packaged Main,
installers or provider execution. No-workspace recovery and full capability
matrices remain in the canonical shared unit tests; do not duplicate those
matrices through this browser journey.

The approved scope retains route-owned open-state behavior. Web account and
workspace shells can remount different SidebarProviders, so these transitions
explicitly use the normal visible opener before collapsing again; they verify
the durable 300px expanded width, not a new persisted collapsed preference.
Desktop retains its existing single shell provider. This distinction was
confirmed by the main session while authoring the journey.

## Authoring verification

- `pnpm exec playwright test e2e/sidebar-icon-rail.spec.ts --list` passed:
  exactly two originals, one Web and one Desktop.
- `pnpm exec tsc --noEmit --strict --skipLibCheck --module esnext
  --moduleResolution bundler --target es2022 --jsx react-jsx --esModuleInterop
  --types node e2e/sidebar-icon-rail.spec.ts` passed.
- Scoped ESLint through its Node API, with the repository's
  `packages/eslint-config/base.js` and root cwd, passed with no warnings/errors.
  A first package-cwd invocation ignored the file outside its base path; that
  result was not counted as lint evidence.
- `git diff --check` for the owned files passed.

Runtime acceptance is pending the main session's source-bound production build
and isolated services. No browser success is claimed by parse/static checks.

## First production run correction

The main session's r1 production run passed the unchanged original Web/Desktop
scope journeys. Both new rail tests stopped at the tooltip assertion because
Base UI retained the outgoing New Issue tooltip with `data-closed` during its
exit transition while the current AI Chat tooltip had `data-open`; both
legitimately included the workspace name. The corrected selector requires the
actual open popup and the full AI Chat label plus workspace text. This changes
only the test selector, with no product change, arbitrary delay or tooltip
lifecycle modification.

The disabled-feature absence checks now first require a known Resources &
connections menuitem to be visible, proving that the More menu has opened before
asserting omitted feature entries. Real rerun remains the main session's gate.

The originals now also measure a 1100×650 large-screen rail with actual CDP
touch emulation. They require 44px painted controls, collect 16px SVG centering
and scrolling/footer geometry, and attach evidence before the final single
centering assertion. This preserves later compact/Chinese captures even when
an icon is misaligned. The emulation is removed before compact testing.
Opening More and activating the real visible sidebar toggle with Enter must
restore 300px and leave no stale More popup. This stays within the two original
journeys; there is no new browser matrix or product workaround.

## Disabled-feature fixture correction

The r2 Desktop original reached the disabled-feature assertion and correctly
showed Work because fresh workspaces currently enable iterations by default
(migration 567 and `TestIterationSettingsDefaultAndEnableWithoutDeploymentFlag`).
The initial rail fixture explicitly disabled Triage but incorrectly relied on
an old iteration default. The fixture now calls the existing canonical
`iterationOptOut(api, second)` administrator preview/operation flow and reads
back both the actual iteration-capabilities response and Triage settings as
disabled. Both facts are checked again before the menu absence assertions.
No product API or guard is changed, and absent Work still requires both
underlying features to be truly disabled.

## Native readiness and failure evidence

The r3 Web original completed all behavior and capture steps before the final
coarse icon-centering assertion exposed a real six-pixel product offset. That
assertion is retained for the geometry implementation to correct.
The r3 Desktop original missed the initial source navigation: `native.start()`
only waits for the document, whereas the renderer installs its navigation
handler when the shell mounts. The test now follows the unchanged original
scope journey and waits for the actual application navigation to be visible
before dispatching the source event, including after the Chinese reload.
No delay or increased timeout replaces readiness.

On any failure, the test now attaches the original error, live page errors,
current body/URL, and an immediate Web/native screenshot before the owned
window/profile/fixture cleanup. Evidence capture cannot replace the original
assertion error. Runtime rerun remains the main session's gate.
