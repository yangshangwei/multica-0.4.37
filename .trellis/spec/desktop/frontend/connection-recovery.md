# Desktop connection recovery

## 1. Scope / Trigger

Use this contract when changing boot authentication, initial workspace loading,
the connection recovery page, or the server editor. An unreachable server must
leave users with an actionable page; editing its address must survive background
authentication recovery. Credential and endpoint isolation still follow
[ApiClient auth epochs](../../core/frontend/api-client-auth-epoch.md).

## 2. Signatures

Core exports these ephemeral types through `@multica/core/auth`:

```ts
interface ConnectionFailure {
  kind: "timeout" | "network" | "server" | "unknown";
  occurredAt: number;
  status?: number;
}
interface AuthRecoveryState {
  isRetrying: boolean;
  nextRetryAt: number | null;
  failure: ConnectionFailure | null;
}
// AuthState.recovery: AuthRecoveryState | null
declare function classifyConnectionFailure(error: unknown, occurredAt?: number): ConnectionFailure;
```

`ApiRequestOptions` is `{ signal?: AbortSignal; timeoutMs?: number }`.
`ApiClient.getMe`, `getConfig`, and `listWorkspaces` accept optional options;
`deviceLogin` and the auth store's `loginWithDevice` accept them as their third
argument. `useWorkspaceList` exposes `error` and `errorUpdatedAt` beside
`workspaces`, `ready`, `unavailable`, `isFetching`, and `refetch`.

`LocaleAdapter.reload?(): void` lets a platform own the refresh requested after
server language synchronization. Without that callback, `UserLocaleSync` retains
its normal browser reload; persistence still happens before refresh.

Preload exposes `openRuntimeConfigEditor(): Promise<{ ok: boolean }>` and
`onRuntimeConfigEditorOpen(callback: () => void): () => void`. The former invokes
`runtime-config:open-editor`; the latter subscribes to `runtime-config:edit`.

## 3. Contracts

- Initializer identity/config/device calls and workspace queries pass the shared
  `RECOVERY_REQUEST_TIMEOUT_MS` (`10_000`). A deadline covers response body parsing
  as well as waiting for headers. Other operations choose their own lifetime.
- Caller cancellation and endpoint cancellation remain active through parsing.
  Disposed or superseded attempts cannot rotate credentials or publish a user,
  configuration, recovery metadata, or a delayed unauthorized callback.
- Auth recovery metadata stays in memory. A manual attempt keeps status
  `recovering`, clears its pending retry timestamp, and sets `isRetrying`.
  Success, explicit logout, and terminal rejection clear recovery metadata.
- TanStack Query owns the workspace list. Initial errors have no authoritative
  data; cached data remains usable after a background error. Desktop schedules
  additional initial-error recovery without changing the shared retry policy.
- Render a countdown only from a real scheduled `nextRetryAt`. Disable duplicate
  retry attempts while keeping Change server usable.
- The persistent editor boundary wraps the changing app/recovery children.
  Its state belongs above those branches. Back renders the latest children.
- A restored user language or background OS-language update may request a full
  reload. Desktop defers that locale reload while the editor is open and applies
  it on explicit Back. An unmount/StrictMode cleanup must not flush the pending
  request. Native server-switch reload still runs through its own coordinator.
- Main accepts editor-opening requests only from its own window or a registered
  living issue window. Queue `runtime-config:edit` until the main renderer has
  installed its listener. This handoff only opens UI.
- Saving remains main-only and uses the existing server-switch coordinator.
  Candidate probing precedes teardown, config writing, and renderer reload.
- Display/copy only endpoint, client version, failure time, localized category,
  and optional HTTP status. No raw error message, body, token, or stack.
  Copy must expose success or failure feedback.

## 4. Validation & Error Matrix

| Condition | Required outcome |
| --- | --- |
| Deadline expires | Timeout recovery; keep a retained credential |
| Network rejection | Network recovery; schedule a real retry |
| HTTP 5xx | Server recovery with status; omit response text |
| Definitive auth rejection | Existing terminal teardown/device handoff rules |
| New login supersedes old attempt | Cancel old work; preserve the new session |
| Initial workspace list fails | Keep verified auth; show actionable list recovery |
| Candidate address/probe fails | Keep original configuration and session |
| Auth succeeds with editor open | Keep editor/input; Back shows recovered app |
| Recovered language differs while editing | Persist preference; defer locale reload until Back |
| Unknown/unregistered window opens editor | `{ ok: false }`; no UI or config mutation |

## 5. Good / Base / Bad Cases

- Good: the user corrects a mistyped server while automatic retries continue;
  their input remains intact when identity recovery finishes.
- Base: first boot has no token. The page describes a connection problem without
  claiming that a previous login was saved.
- Bad: each retry swaps to a loading screen, or an auth success unmounts the
  editor. A generic timeout applied to uploads is also outside this contract.

## 6. Tests Required

- Core `api/recovery-requests.test.ts`: hanging headers/body, deadline disposal,
  endpoint/caller abort and no late unauthorized/device side effects.
- Core auth store/classifier/initializer and real CoreProvider tests: retained
  credential, manual retry deduplication, replacement login and StrictMode handoff.
- Core locale sync tests: platform callback after persistence, browser fallback,
  and no refresh for an already matching preference. Desktop integration must
  keep unsaved editor input when a recovered server preference differs.
- Workspace hook tests: authoritative-data distinction and query cancellation.
- Desktop recovery/boundary/workspace-hook tests: truthful countdown, safe details,
  copy feedback, real-editor persistence, current-child restoration and retries.
- Native helper/message-queue tests: sender authorization and deferred delivery.
- Isolated Electron acceptance: real renderer/preload, temporary profile/config,
  loopback fixtures, failed/successful switch, child handoff, keyboard/copy,
  both locales, dark/narrow windows and long endpoint text. Never launch an
  installed daemon or agent as part of the default acceptance fixture.

## 7. Wrong vs Correct

```tsx
// Wrong: recovery success destroys the user's editor and its unsaved input.
if (authStatus === "recovering") return editing ? <Editor /> : <Recovery />;
return <AppShell />;

// Correct: editing is owned above the changing authentication branches.
return <DesktopConnectionRecoveryBoundary recovery={recovery} onRetry={retry}>
  {currentAppState}
</DesktopConnectionRecoveryBoundary>;
```
