# Desktop changelog integration research

Research date: 2026-09-13. Scope: desktop presentation, Help entry, updater interaction, shared fetching/i18n, and focused acceptance tests. No implementation changes were made by this research lane.

## Recommendation

Use a shared `ChangelogPage` under `packages/views/changelog/`, backed by a deployment-wide API query under `packages/core/changelog/`. Mount it at `/:workspaceSlug/changelog` as an ordinary desktop session/tab destination, matching the in-app documentation reader. Add `Help > 变更说明` with an `AppLink` and provide the same minimal web route because HelpLauncher is shared. Keep release content independent of Electron's binary-update channel.

Serve release information through the configured Multica backend. The official site is an editorial/source reference, not a runtime dependency: this fork is explicitly an intranet deployment. New installs and already-running clients with this feature must read the published server feed rather than a changelog compiled only into the desktop bundle.

## Existing integration points and constraints

| Concern | Evidence | Consequence |
| --- | --- | --- |
| Help menu | `packages/views/layout/help-launcher.tsx:23` | Shared by web and desktop; reads nullable `useWorkspaceSlug()` at line 29 and links docs through `paths.workspace(workspaceSlug).docs()` at line 53. Add a sibling changelog item inside the same workspace guard. |
| Old changelog removal | `packages/views/layout/help-launcher.tsx:45`; `packages/views/layout/help-launcher.test.tsx:145` | The public multica.ai link was deliberately removed because it is unreachable on the intranet. Replace the old test asserting absence with an assertion for the new in-app destination. Do not restore a public external link. |
| Help crash regression | `packages/views/layout/help-launcher.tsx:66`; `packages/views/layout/help-launcher.test.tsx:117` | Keep the version label inside `DropdownMenuGroup`. A bare Base UI GroupLabel previously crashed the application when Help opened. |
| Desktop presentation category | `CLAUDE.md`, Desktop Rules; `apps/desktop/src/renderer/src/routes.tsx:278` | Changelog is a durable reference/session destination just like docs. `WindowOverlay` is reserved for pre-workspace one-shot flows; no new overlay is needed. |
| Desktop routing | `apps/desktop/src/renderer/src/routes.tsx:280` | Add the route alongside docs under WorkspaceRouteLayout. The existing shell supplies toolbar/drag regions; a new full-window shell is unnecessary. |
| Shared path construction | `packages/core/paths/paths.ts:95` | Add `paths.workspace(slug).changelog(...)`; any release hash/query belongs here so callers never construct conflicting URLs. |
| Tab icon and title | `packages/core/paths/route-icons.ts:47`, `:63`, `:91`; `packages/core/paths/tab-subject.ts:125` | Register the page key, nav label key, and icon in WORKSPACE_PAGES. The generic subject fallback then recognizes it automatically. This registry does not add a normal sidebar row. If choosing a new icon name, extend `packages/views/layout/route-icon-components.tsx:35`. |
| Web wrapper | `apps/web/app/[workspaceSlug]/(dashboard)/docs/page.tsx:8` | Create the analogous small wrapper for shared Help navigation; avoid leaving a broken destination on web. |
| Existing content shell | `packages/views/docs/docs-page.tsx:47` | Reuse PageHeader, PAGE_GUTTER, semantic typography, scrolling/overflow treatment, Skeleton, and CollectionPageState. Preserve error/empty/loading distinctions. |
| Intranet contract | `.trellis/spec/docs/frontend/docs-bundle.md`, sections 2 and 5 | Docs currently ship in the Go binary and require no public site. Changelog assets/content must likewise remain available from the deployment. Never invent intranet repository URLs. |

## Fetching and freshness

- `packages/core/docs/queries.ts:16` is the appropriate domain query-options pattern. Its keys omit workspace ID because every workspace reads the same deployment content (lines 10–14). Changelog can use the same justified scope.
- **Do not copy docs cache policy.** `packages/core/docs/queries.ts:33` uses infinite stale/GC times because docs are build-immutable. The global defaults in `packages/core/query-client.ts:7` also use infinite stale time and disable window-focus refetch. A changelog that inherits either policy will stay stale in an open client.
- Explicitly choose and document a freshness bound, such as a 60-second poll while mounted, refetch on mount/focus/reconnect, and a manual refresh action. If Help should show a new-release indicator while the page is closed, a lightweight shell/Help query must remain mounted; polling only the page cannot provide that indicator. Keep this bounded polling independent of the automatic binary-updates preference.
- Keep successful cached entries visible on a later network error and show a small refresh failure/stale notice. A first-load failure needs retry; a server without the feature (404) needs a clear unsupported-server state, as in `packages/views/docs/docs-page.tsx:39`.
- Add `ApiClient.getChangelog` using `this.fetch<unknown>` and `parseWithFallback`, following `packages/core/api/client.ts:5123`. Network JSON must pass through zod before UI access. `packages/core/docs/schema.ts:16` shows defaulted/loose schemas for installed-client compatibility; `packages/core/docs/schema.test.ts:58` covers missing/unknown/malformed fields.
- Use the configured API origin, already wired into CoreProvider at `apps/desktop/src/renderer/src/App.tsx:454`. No fetch from multica.ai/GitHub is needed in renderer code. No release server data belongs in Zustand or persisted localStorage.
- Do not conflate installed app version, deployed server version, and newest published release. The current app version is already available as `window.desktopAPI.appInfo.version` (`apps/desktop/src/preload/index.d.ts:21`), while Help's server version comes from the config store (`packages/views/layout/help-launcher.tsx:25`). Inject desktop-only version data through route props rather than introducing Electron references in shared views.

## Updater integration

- `apps/desktop/src/main/updater.ts:17` enables automatic download/install-on-quit. Background binary checks start after five seconds and repeat hourly (lines 54–55); users can disable them (lines 126–169). This mechanism does not satisfy changelog freshness and should remain a separate concern.
- `apps/desktop/electron-builder.yml:124` still points binary updates at GitHub `multica-ai/multica`. Replacing the binary release transport is outside this UI research scope; the changelog must work regardless of whether this public update channel can be reached.
- `apps/desktop/src/renderer/src/components/update-notification.tsx:11` currently builds a public version-anchor URL; its line 53 opens that URL externally. Route this action to the in-app release entry and retain the downloaded version identifier.
- **Provider placement trap:** UpdateNotification currently mounts at `apps/desktop/src/renderer/src/App.tsx:488`, outside CoreProvider as well as desktop navigation/workspace providers. Adding `useNavigation`, `useWorkspaceSlug`, or locale hooks there directly is unsafe. DesktopNavigationProvider and WorkspaceSlugProvider wrap `DesktopLayout` at `apps/desktop/src/renderer/src/components/desktop-layout.tsx:260` and `:270`. Move the actionable notification into the appropriate shell provider scope, or inject a platform callback with a deliberate no-workspace behavior. Avoid silently losing the notification after an event fired before that shell mounted.
- Desktop navigation operates on the tab store, not the router directly (`apps/desktop/src/renderer/src/platform/navigation.tsx:165`). Use its adapter. A release anchor must read `useNavigation().hash`, since `window.location.hash` is the packaged file URL rather than the active session's hash (same file, lines 220–224).

## Rendering and localization

- `useT("namespace")` uses typed selector calls (`packages/views/i18n/use-t.ts:7`). Add `help.changelog` and `nav.changelog` to all four layout JSON files under `packages/views/locales/{en,zh-Hans,ja,ko}/`.
- If the page has its own `changelog` namespace, add its four locale JSONs, runtime registrations in `packages/views/locales/index.ts:110`, and the type import/interface entry in `packages/views/i18n/resources-types.ts:37`. A JSON file alone will not make the namespace available at runtime or to typecheck.
- Follow `apps/docs/content/docs/developers/conventions.zh.mdx:106` and `:289`: issue → 任务, workspace → 工作区, skill stays lowercase English; simple Chinese sentences, full-width punctuation, and spaces around English product terms. Use 变更说明 as requested.
- The existing DocsContent renderer is documentation-specific: `packages/views/docs/docs-content.tsx:81` rewrites root-relative links into documentation page routes. Do not pass arbitrary release/commit links through it without accounting for that behavior. Structured sections/bullets avoid this mismatch; if markdown is needed, reuse existing markdown sanitization rather than allowing raw release HTML or inventing a new dependency.
- Use semantic styles/tokens (PageHeader already demonstrates text-body/text-caption), responsive timeline layout, accessible headings/date labels, and break long release titles/commit subjects. A new icon must be decoratively hidden from assistive technology where the adjacent text already names the action.

## Focused verification for implementation

1. **Core boundary:** a node-environment suite beside the new schema tests proper releases, missing/unknown fields, malformed envelope/entry, and HTTP 404/error behavior. Assert correct configured API path. Keep parsing matrices here rather than repeating them through DOM mounts.
2. **Live refresh behavior:** use a real QueryClient and fake transport/timers to return a second published release while the same observer stays mounted. Verify it appears within the documented bound without reinstall/reload; also verify retained content on failed refresh, reconnect/focus refresh as selected, and a fresh client's first read returning latest content. Merely snapshotting option constants does not prove this behavior.
3. **Help/page UI:** update `packages/views/layout/help-launcher.test.tsx` to assert `/<slug>/changelog`, no public external link, safe no-workspace rendering, and preserve the version-row regression. A new page test should cover normal timeline, initial loading/error/empty, refresh feedback, and release-anchor selection; keep locale fixtures tied to actual English JSON as the existing Help test does.
4. **Desktop wiring:** extend the updater notification test to prove the downloaded version opens the in-app destination and restart still delegates to updater. Test chosen no-workspace/pre-shell behavior. Verify route recognition/title/icon through existing `packages/core/paths/paths.test.ts`, `route-icons.test.ts`, `tab-subject.test.ts`, and `tab-presentation.test.ts` at their canonical layers.
5. **Locale and smoke checks:** existing `packages/views/locales/parity.test.ts` catches key mismatch. Run narrow Vitest suites and typecheck/lint for core/views/desktop plus the web wrapper. Smoke-test real Help opening, route/tab navigation, a long page scroll, zh-Hans rendering, offline/failure states, and a changed backend feed visible in an already-open client. Desktop prior builds without the new UI cannot gain a new Help entry through a data-only feed; acceptance must distinguish upgrading to this feature once from later release-content updates.

The desktop/views/core Trellis frontend specs were read; most are scaffolds. CLAUDE.md and the existing code above currently carry the actionable integration contracts. No implementation or tests were run in this research-only lane.
