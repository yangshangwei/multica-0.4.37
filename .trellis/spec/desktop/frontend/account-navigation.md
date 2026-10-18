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

## 7. Wrong vs Correct

Wrong: preserve account content across viewer changes, treat account entry as
workspace bootstrap, or load `@multica/core` through sandboxed preload require.

Correct: keep identity-scoped account state, independent workspace recovery and
bundled pure validators within the existing native protocol.
