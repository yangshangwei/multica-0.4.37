# Desktop connection recovery acceptance

Status: complete. Independent review has no unresolved task-code findings.

## Delivered behavior and files

- `packages/core/api/{client,request-lifecycle}.ts`, `auth/{store,recovery}.ts`,
  `platform/auth-initializer.tsx`, and workspace hooks/query: selected boot
  requests have a 10-second deadline through body parsing; obsolete attempts
  cannot publish credentials or unauthorized side effects. Transient failures
  preserve credentials; manual retry keeps the recovery surface and deduplicates.
- Desktop App, IssueWindow, recovery page/boundary/workspace hook: show the
  endpoint, actual retry state/countdown, localized sanitized details and copy
  feedback. Main and child-window workspace failures use Query-owned recovery.
- Existing endpoint editor and server-switch coordinator are reused. Main owns
  saving; child windows request an authorized, queued UI handoff. Failed probes
  leave the old configuration/session intact.
- Core locale adapter/UserLocaleSync plus desktop locale adapter: an optional
  platform refresh callback lets desktop defer language reload while editing.
  Recovered preferences persist; the current editor/input stays live until Back.
  No persisted draft, global UI store, new context or dependency was added.
- English/Chinese `auth.json`, owning-layer regressions, native acceptance
  scripts and connection-recovery specs accompany the code.

## Verification

| Check | Result |
| --- | --- |
| Core affected auth/API/initializer/workspace plus full i18n suite | 44 files, 932 tests passed |
| Complete desktop suite | 93 files, 1,034 tests passed |
| Translation parity | 1 file, 64 tests passed |
| Core/desktop lint | Exit 0; desktop retains its existing unrelated warning |
| Core/desktop/views/web TypeScript | Exit 0 |
| Final desktop main/preload/renderer build | Exit 0 |
| Impeccable detector | `[]`, exit 0 |
| Final visual verdict | 93/100, pass; normal/narrow, dark, long address and editor captures |
| Isolated native acceptance | Six scenario groups passed, exit 0; zero page errors/agent starts |
| Task diff whitespace check | Exit 0 |

The 2,030 tests above are the related suites, not a claim about every repository
test. The existing desktop lint warning is
`components/tab-content.tsx:54` (`group` effect dependency), outside this task.

## Acceptance criteria

| Criterion | Evidence |
| --- | --- |
| A1: hanging/rejected boot becomes actionable; credential retained; recovery succeeds | Native identity/config/device deadline scenarios; core request and initializer suites |
| A2: prefilled editor, failed validation/probe/save, original session retained | Native invalid URL and unreachable loopback probe/save; editor/boundary tests |
| A3: coordinated valid switch; no old token to replacement; child handoff | Native production coordinator/probe/config helpers and queued authorized handoff; child destroyed, one reset, saved new URL, old bearer absent from replacement requests |
| A4: unsaved editor survives background authentication | Real UserLocaleSync/editor regression plus native English boot returning a Chinese preference; textbox/value and current language preserved until Back |
| A5: retry stays visible, no overlap; workspace failure preserves verified auth | Native duplicate manual retry and initial-list automatic recovery; auth/workspace-hook regressions |
| A6: allowlisted details, copy feedback, keyboard, locales, narrow layout | Native secret exclusion, HTTP 503 details, clipboard success/failure and Enter activation; Chinese/dark and 420×640 long-address captures, no horizontal overflow |
| A7: package checks, native/visual acceptance and independent review | Passed checks above; independent review in task research |

## Native isolation and reproducibility

The fixture runs real built renderer/preload and Chromium windows/storage against
two loopback HTTP services. It uses temporary `userData` and `desktop.json`, blocks
external requests, and replaces unrelated daemon/updater/native services. It
reuses production probe, server-switch, config parse/load/save, cookie cleanup,
message queue and editor authorization helpers. Temporary profiles and listeners
are closed after the run; no personal configuration or installed agent is used.

From the repository root, after archiving:

```sh
pnpm --filter @multica/desktop exec electron-vite build
node .trellis/tasks/archive/2026-10/10-12-desktop-connection-recovery/evidence/run.mjs
```

Artifacts are in `.omx/state/desktop-connection-recovery/`: `acceptance.json`,
`electron-acceptance.log`, package check logs, `review.md`, `impeccable.json`,
`ralph-progress.json`, screenshots and layout measurements. The `locale-red-*`
artifacts record the reproduced language-refresh regression before its fix.

Verification ran on macOS with the installed Electron 39 binary. Windows/Linux
native windows, production daemon processes and packaged installers were outside
this acceptance run. No backend, migration, packaging or deployment changed.
