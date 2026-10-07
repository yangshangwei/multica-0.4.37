# R1 Desktop implementation evidence

Implemented 2026-10-07 on the shared task checkout. No commits, new dependencies,
real profiles, credentials, agent services, or external HTTP providers were used.

## Change boundary and files

The behavior gap was local-file access from untrusted iframe documents with
`webSecurity: false`. The owning boundary is Electron Main's renderer session.
No shared preview behavior or renderer protocol was changed.

- `apps/desktop/src/main/renderer-file-access.ts`: sole `onBeforeRequest` owner,
  installed once per session. Registered WebContents are removed on destruction;
  each file request must identify that live WebContents' current, attached main
  frame. Subframes, absent identities and inaccessible/disposed frames fail closed.
- `apps/desktop/src/main/index.ts`: install the policy beside the existing
  navigation guard, before the shared `loadRenderer` loads main or issue windows.
- `apps/desktop/src/main/renderer-web-preferences.ts`: correct the security
  explanation. Process sandboxing and opaque iframe origins alone do not prevent
  this file vector; the native policy is required. Existing preferences remain.
- `apps/desktop/src/main/renderer-file-access.test.ts`: five boundary/lifetime
  tests (unknown/detached/throwing/destroyed contexts, live frame replacement,
  cross-session separation, shared/recreated windows, single-listener ownership).
- `apps/desktop/scripts/verify-renderer-file-access.mjs`: explicit native
  acceptance using installed Electron and electron-vite's existing compiler.

The only production wiring addition is one import and one install call. The
existing navigation, WebSocket header and download handlers are reused unchanged.
There is no parallel HTML renderer, CSP wrapper, new session per preview, or
additional dependency.

## Native regression method

The script creates a unique temporary directory, userData profile and fake local
files, serves a loopback-only HTTP fixture, and cleans its directory on exit.
It bundles actual `HtmlBlockPreview`, `HtmlAttachmentPreview`, and
`AttachmentPreviewPage`, the real attachment API client/query, translations and
the full-page scroll bridge with an active restoration adapter. Only attachment
server responses and the surrounding navigation/scroll providers are fixtures.

It extracts the production `loadRenderer` function verbatim using TypeScript's
AST, bundles its real navigation/file-access imports, and calls the production
`createRendererWebPreferences`. This exercises the installation site without
executing index.ts's unrelated auth/daemon/application startup. The preload is a
small sandbox-safe fixture. The fixture renderer loads through the production
file-based entry path; no Electron security flag is overridden.

## Red evidence, before production changes

Command: `node apps/desktop/scripts/verify-renderer-file-access.mjs`

Electron **39.8.7**, exit **1**. Six positive control groups passed; ten security
groups failed:

- Inline, attachment and full-page paths returned `FAKE_LOCAL_FILE_ONLY` through
  fetch/XHR, executed the fake local script, loaded the fake image/style, and
  read the fake file from a nested srcdoc frame. Blob-worker file fetch was
  already denied by Chromium and is retained as a negative control.
- All three paths read the fake file after self-navigation to an HTTP document.
- All three paths navigated to and executed the fake local HTML document.
- Recreated/shared-window denial failed with the same fake-file marker.
- Trusted main/issue renderer assets, preload, dynamic import, worker and cache
  seed passed, as did all three interactive JS/HTTP preview controls and the
  native HTTP image/PDF/header-handler control.

Original session log: `/tmp/multica-desktop-file-red.log` (temporary); the above
records its substantive results without relying on preservation of that log.

During harness development, fixed fixture-only issues before recording this
red result: initialized the actual API singleton instead of assigning a proxy
property, corrected button JS quoting, and used PDFium's actual progress field.
After implementation, corrected file-navigation observation: Electron retains
the attempted URL on a failed frame. The regression now requires the native
subframe `did-fail-load` **-20 / ERR_BLOCKED_BY_CLIENT**, plus no fake script
execution, rather than inferring success/failure from that attempted URL.

## Green and verification

Final native log: `/tmp/multica-desktop-file-green.log`. Desktop check logs:
`/tmp/multica-desktop-tests.log`, `/tmp/multica-desktop-typecheck.log`, and
`/tmp/multica-desktop-lint.log`. These are session-local evidence paths; results
are retained below because temporary files are not durable task artifacts.

| Command | Result |
| --- | --- |
| `node apps/desktop/scripts/verify-renderer-file-access.mjs` | Exit 0, Electron 39.8.7, **16/16** groups pass. Final run includes PDFium progress 100, one parsed page and an actual embed. |
| `pnpm --filter @multica/desktop exec vitest run --maxWorkers 2` | Exit 0, **85 files / 962 tests** pass. |
| `pnpm --filter @multica/desktop exec vitest run src/main/renderer-file-access.test.ts --maxWorkers 2` | Exit 0, **5 tests** pass after the final test-only TypeScript annotation correction. |
| `pnpm --filter @multica/desktop typecheck` | Exit 0; node and web TypeScript projects both checked. |
| `pnpm --filter @multica/desktop lint` | Exit 0; one pre-existing `tab-content.tsx:54` hook dependency warning, zero errors. |
| `pnpm --filter @multica/desktop exec eslint src/main/renderer-file-access.ts src/main/renderer-file-access.test.ts src/main/index.ts src/main/renderer-web-preferences.ts scripts/verify-renderer-file-access.mjs` | Exit 0; changed files clean after final edits. |
| `node --check apps/desktop/scripts/verify-renderer-file-access.mjs` | Exit 0. |
| `git diff --check -- apps/desktop` | Exit 0. |

The native checks prove file-denial for all three entry paths, including a file
already fetched by the trusted main frame, HTTP and file self-navigation,
shared-session windows and recreation after the main window is destroyed.
Trusted scripts, dynamic imports, main-frame worker assets, sandboxed preload,
interactive scripts, HTTP requests/scripts, HTTP image and actual PDFium rendering
remain usable. A separately installed `onBeforeSendHeaders` fixture still stamps
every HTTP request, proving the file listener does not replace that event owner.

## Limits and follow-ups

- `webSecurity` remains disabled. This closes the demonstrated renderer local-file
  vector, and does not establish general network/CORS isolation.
- Native acceptance ran on macOS with Electron 39.8.7. Windows/Linux, installer/
  ASAR packaging, the entire application preload and real auth/daemon startup were
  not exercised; no release packaging was requested.
- Missing/destroyed/throwing native-object access is covered by the focused unit
  matrix; real Electron covers document navigation and window destruction/
  recreation. Privileged Main-process `session.fetch` is outside this renderer
  boundary.
- Keep one `onBeforeRequest` owner per renderer session; future subscribers must
  compose with this policy instead of replacing it. Future file-backed subframe
  features require explicit review. Main-frame worker entry assets are covered;
  worker requests without a verifiable frame intentionally fail closed.
- Main session owns the independent review, relevant spec update and integration
  checks. Existing unrelated workspace edits were left intact.
