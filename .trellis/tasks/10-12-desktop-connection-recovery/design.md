# Design

## Boundaries
Core owns boot request lifecycle and sanitized recovery metadata. Desktop owns presentation, endpoint editing and window routing. The existing server-switch coordinator remains authoritative.

## Shared contract
Add ephemeral auth-store recovery: AuthRecoveryState | null, exported from @multica/core/auth.
- ConnectionFailure: { kind: "timeout" | "network" | "server" | "unknown"; occurredAt: number; status?: number }
- AuthRecoveryState: { isRetrying: boolean; nextRetryAt: number | null; failure: ConnectionFailure | null }
- Export classifyConnectionFailure(error: unknown, occurredAt?: number): ConnectionFailure for desktop's workspace fallback.
- First failed boot enters recovering; later retries keep that surface and update isRetrying. Success, logout and definitive rejection clear recovery. Countdown derives from nextRetryAt.
- No raw body/message/token in metadata. Error categories describe only known facts.

Bound recovery HTTP calls to 10 seconds via abort signals, preserving endpoint and credential epochs through body parsing. Cancel obsolete attempts on disposal/supersession/endpoint switch; cancelled device login must not publish. Do not apply a generic timeout to uploads or long-lived requests.

Workspace bootstrap also needs bounded listWorkspaces. The implementation lanes agreed on `ApiRequestOptions = { signal?: AbortSignal; timeoutMs?: number }`: `getMe(options?)`, `getConfig(options?)`, `listWorkspaces(options?)` and a third options argument to `deviceLogin` / `loginWithDevice`. The initializer and workspace query pass `10_000` milliseconds; other callers keep their current lifetime. `useWorkspaceList` adds `error` / `errorUpdatedAt` to its existing return shape. Desktop owns additional initial-error scheduling; the global workspace retry policy stays authoritative.

## Desktop interaction
Keep centered layout and first-child DragStrip. Show endpoint, localized status, real retry timing, Retry, Change server, collapsed details with allowlisted copy. Wrap long endpoints/actions in small windows.

Editing state lives above conditional recovery returns in a dedicated boundary. It remains mounted when auth succeeds. Reuse DesktopEndpointSetupPage and minimally extend optional props for notice/save label. Back clears editing and renders latest state. Child windows call `desktopAPI.openRuntimeConfigEditor()` (`runtime-config:open-editor`); main queues `runtime-config:edit` until the main renderer subscribes through `onRuntimeConfigEditorOpen`. Only the main window or a registered live issue window can request this UI handoff; it never saves or clears anything.

Recovered server language preferences can otherwise force a whole-renderer reload
through `UserLocaleSync`, bypassing the editor boundary. `LocaleAdapter` therefore
adds optional `reload(): void`; core persists the preference and delegates refresh
to that platform callback, retaining the browser fallback when absent. Desktop
defers locale-triggered reload while editing and applies it on Back. This is part
of A4 input preservation; it adds no persisted editor draft or shared UI state.
The existing native server-switch reload remains authoritative.

## Compatibility and scope
No API response contracts, routes, dependencies or migrations change. Metadata is not persisted. Core improvements also cover web; cookie/device/StrictMode/stale-response regressions stay green. Candidate validation precedes old-session mutation. Auth and workspace request ownership stay separate.

## Verification and rollback
Tests follow their owning layer. Isolated Electron profile and loopback fixture endpoints only; no personal credentials or installed agents. Revert task code to restore prior behavior; no migration needed.
