# Persistent Desktop account navigation

## 1. Scope / Trigger

Apply to `/account`, the existing tab store/coordinator, auth bootstrap, deep
links and issue windows. Account routes are persistent destinations; one-shot
workspace creation/invitation flows remain `WindowOverlay` state.

## 2. Signatures

`platform/account-workbench-route.tsx` wires shared `AccountWorkbenchPage`.
The tab store has account/workspace/none scope and version-5 persistence.
The existing issue-window context is tagged account/workspace; Main, preload
and renderer validate that same context. Shared account path validators are
bundled into Main/preload rather than externalized as raw core TypeScript.

## 3. Contracts

One account tab holds account view/issue selection and its navigation history.
Persist the actual history index, scroll, Server/viewer identity and existing
workspace pins/MRU. Canonicalization must not select the last occurrence of a
repeated URL. Identity changes discard prior account state and close its popouts.

Clear `setCurrentWorkspace(null, null)` before publishing account navigation;
outgoing workspace layouts must not write the old mirror back. Account reads
must carry no ambient workspace header. The sole coordinator owns route changes.
Route APIs stay in platform modules; shared views contain no router imports.

Authenticated account entry remains available with no workspaces, a pending list
or list failure. Login completion does not await another workspace fetch; the
existing Query owns that list. Actual login/setup/password guards remain blocking.

Detailed account links may remain in an account popout; account list navigation
returns to the main account tab. Preserve native Cmd+W and existing workspace
issue windows. With no workspace sidebar, reserve native toolbar clearance,
omit its unavailable toggle and show the localized account tab/window title.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Workspace list zero/pending/500 | Account remains usable; workspace recovery is separate |
| Fresh session (no restored tab), list ready with workspaces | Open the first workspace; account stays one click away in the navigation bar |
| Login/setup/password restriction | Existing guard blocks account content |
| Account navigation from a workspace | Clear mirror before subscribers observe the new tab |
| Repeated persisted URL | Restore saved index, not the last equal URL |
| Server/viewer changes | Discard previous account state and close popouts |
| Invalid/foreign native context | Reject; no guessed account/workspace scope |
| Raw core require in sandboxed preload | Build defect; bundle the pure validators |

## 5. Good / Base / Bad Cases

- Good: A → account → B preserves both workspace tabs and one account history;
  restart restores the selected account issue and scroll.
- Base: existing workspace window, tab and pin behavior remains intact.
- Bad: waiting on workspace loading during account login, or publishing account
  navigation before clearing workspace headers.

## 6. Tests Required

Store/coordinator/navigation/boot/shared-window tests own their boundary matrices.
`e2e/collaboration-account-desktop.spec.ts` runs actual Electron with isolated
profiles and actual API/WS; only named failure/guard cases use a loopback proxy.
Use `BrowserWindow.sendInputEvent` for genuine native shortcuts and
`webContents.capturePage` for Retina-safe zoom capture. Verify 200% zoom/overflow.
Before reruns, verify the static renderer URL is reachable; an exited helper
process is not evidence of a product failure. Rebuild after shared-core changes.
Native-service stubs do not prove packaged full Main startup or other OSes.

`e2e/collaboration-offline-handoff.spec.ts` closes the recipient client before
public invitation/assignment, then performs fresh password authentication into
a new Web context or actual Electron profile. Invitation changes participation
only; assignment moves responsibility and current workbench counts atomically,
with no acceptance step. After transfer away, notification read/archive and a
second client restart preserve the participating entry/history while the issue
is absent from current work. Check exact list IDs/counts, not only page labels.

After removal, retained notification links and the old account detail must lose
their private title/comment and refuse entry; the independent creator retains
the issue. Participation never creates source membership or personal runtime
consent. Original POST replay is subject to current write authority; a former
handler recovers its transfer receipt by GET instead. Fresh API authentication
plus actual clients covers login-after-offline; login-page UX and installed full
Main are separate evidence scopes.

## 7. Wrong vs Correct

Wrong: preserve account content across viewer changes, treat account entry as
workspace bootstrap, or load `@multica/core` through sandboxed preload require.

Correct: keep identity-scoped account state, independent workspace recovery and
bundled pure validators within the existing native protocol.

## Run anchors and native evidence

Source/account links preserve validated `#run-<UUID>` through the navigation
adapter, including native MemoryRouter and popouts; `window.location.hash` is
not the route authority. Hiding the currently linked run clears only its own
validated fragment so the same notification can reopen and refocus it.
Popover final-focus handoff must permit navigation while Escape still returns
focus to its trigger.

Native copy tests wait for the exact absolute link from the existing configured
share policy; a completed click is not proof that asynchronous clipboard writes
have settled. Use test sentinels and avoid printing prior clipboard data. Keep
clipboard preservation outside the product. For screenshots, await closed
popovers, committed scroll geometry and renderer frames before `capturePage`;
assert that usage and long-body captures show their intended, distinct states.


## Source surface native navigation evidence

A fresh native session may legitimately restore the account shell while its
workspace list loads. Source E2E uses the fixed global `Workspaces` button and
then the contextual workspace menu before using workspace sidebar links; a renderer URL cannot select the tab-owned
MemoryRouter. List/board/Gantt/comment and relative layout CLI evidence reuse
the existing source fixture, with actual native sort transforms complete before
pointer release. This harness exercises renderer/preload and named Main hooks;
full installed Main/installer remains separate acceptance.


## Current full packaged Main and cross-client continuation

`e2e/collaboration-installed-main.spec.ts` starts the exact current unsigned
Linux-arm64 electron-builder directory package under Xvfb, with full production
Main/preload/renderer and `app.isPackaged=true`. A disposable container's real
OS-default `/root` owns runtime config, daemon profile, updater and password
session; do not override the user's home or replace native IPC services.
Disable automatic daemon/update through their actual fresh profile files, verify
bundled CLI version/bootstrap, and perform the real password login form.

The same issue UUID passes Web→installed Main→Web handoff with unchanged creator,
original session/channel/command identities and current capabilities. Use the
actual per-lane resource scope when canceling: sequence1 can exist in multiple
channels. Read cancelled input history after reload and full process restart;
unknown Stop proof remains a barrier in both clients, with explicit partial
Resume preserving it. Service-issued pending Permit is protocol simulation,
not a native provider run or proof. Exact current package hashes and separate
actual Linux natural-proof/reconnect evidence must retain their own scopes.


The same installed-Main journey submits Request information, Return information
and Submit for acceptance through their real forms on both Web and Desktop.
Each side uses the one prescribed status option/current version and exact
request identity; both clients observe the current handler/request and the
same session/execution state. Before another intent, await actual post-navigation
GET completion and read through the authenticated renderer: Electron can retire
reload response bodies from CDP, even when HTTP200 has completed. A stale CDP
body is no evidence of an unavailable issue and must not cause a write retry.
Keep independent original receipts and actual rendered handler/status checks.
