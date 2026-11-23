# Verification — 2026-10-28

The selected fixed global switch plus position restoration is implemented in
Web and Desktop. Existing unrelated working-tree edits were excluded from this
task. No backend behavior, API, mobile surface or dependency was changed.

## Source checks

- Desktop implementation agent: 100 Vitest files / 1,245 tests passed; full
  Main/renderer typecheck and lint passed. One existing `tab-content.tsx` hook
  warning remains outside changed lines.
- Trellis check agent: all four touched packages pass lint/typecheck; Core
  scope-history (4), Web navigation/restoration/shell (27), and shared
  navigation/sidebar/detail/memory/workbench (53) focused tests passed.
- Main integration: Core navigation (6), Web including account guard (32 before
  the additional delayed-commit regression), and shared views/locales (120)
  passed. The final memory dialog regression suite passed all 7 tests.
- `git diff --check` passed. The Impeccable detector returned `[]` for the new
  shell/switch targets. Full Views lint has 27 pre-existing warnings; no new lint
  error was left. No Go suite was run because product changes are frontend-only.

The independent review fixed an outgoing-Next-route focus race and recorded its
regression in [research/check.md](research/check.md).

## Actual clients

`e2e/global-scope-navigation.spec.ts`: **2 passed**, zero retries or skips.

- Production Next Web and real macOS Electron both opened a task in the second
  workspace, entered Participating collaboration detail, applied search/sort,
  and restored the original task with one Workspace click.
- One Collaboration click restored the exact issue, view, search, title sort,
  list offset 180 and detail offset 620. Re-clicking the selected global item
  left the offset unchanged.
- An explicit link opened a different issue at `#description`. After scrolling
  to 800, a workspace/account roundtrip restored 800 without replaying the
  anchor. Global navigation coordinates matched between scopes.
- Web at 390px and Electron at 200% zoom retained both global labels without
  document overflow. Both English and Chinese were captured.
- Fixture cleanup removes only its exact seeded protocol rows, then uses normal
  workspace deletion. The first run completed UI assertions but failed cleanup
  because retained collaboration records correctly prevent workspace deletion;
  the final run passes both behavior and cleanup.

`e2e/collaboration-account-desktop.spec.ts`: **3 selected tests passed**, zero
retries/skips: account tab preservation across spaces/popouts/restart; cold
account without spaces plus controlled directory error; pending account login
while workspace loading stalls.

Commands ran via the existing isolated environment with explicit
`COLLABORATION_E2E_API_URL`, `COLLABORATION_ELECTRON_RENDERER_URL`,
`NEXT_PUBLIC_API_URL` and `PLAYWRIGHT_BASE_URL`. The initial API mode mismatch
was resolved by stopping only the task-owned API and loading its exact env file;
that preflight failure is not product evidence. A second running Web used the
original checkout's build directory, so Web validation used a temporary worktree
snapshot with production Next, leaving the other instance intact.

## Evidence and limits

Build identity, source hashes, API provenance and screenshot hashes are in
[evidence/provenance.json](evidence/provenance.json). Exact test summaries are
[roundtrip.txt](evidence/roundtrip.txt) and
[account-native.txt](evidence/account-native.txt). Visual verdict: 95/pass.

- [Desktop workspace](evidence/desktop-scope-workspace.png)
- [Desktop restored collaboration](evidence/desktop-scope-resumed.png)
- [Desktop Chinese at 200%](evidence/desktop-scope-compact-zh.png)
- [Web workspace](evidence/web-scope-workspace.png)
- [Web restored collaboration](evidence/web-scope-resumed.png)
- [Web Chinese compact](evidence/web-scope-compact-zh.png)

Native evidence covers the repository's actual Electron renderer/preload and
selected production Main handlers, not a newly packaged installer or Windows/
Linux desktop. Web transient history/scroll remains per browser-tab session;
full browser restart persistence is outside this task. The change adds no
server-side persistence or data migration.
