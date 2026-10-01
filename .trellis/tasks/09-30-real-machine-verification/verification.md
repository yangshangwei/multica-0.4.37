# Local verification — desktop close behavior

Date: 2026-09-30. Outcome: **local verification passed; native acceptance pending**.
The user explicitly confirmed that no target machines are available and requested
completion of local verification first. The parent and native verification task
remain `in_progress`; no production feed or release tag was changed.

## Fix and scope

- `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.tsx`:
  preserve all translated Select labels and filter only selectable items. This
  removes the conditional array expansion and fixes the raw `minimize` label
  when a previously saved preference meets an unsupported tray session.
- `apps/desktop/src/renderer/src/components/desktop-behavior-settings-tab.test.tsx`:
  regression uses the real Select and Chinese resources. Observed failure before
  the fix; afterward the translated label remains visible, unavailable minimize
  cannot be selected, and the stored preference is not rewritten.
- Updated the close-behavior spec, original-design pointer, task context manifests,
  native checklist and draft delivery instructions. Corrected D-Bus capability,
  prompt acknowledgement deadline, automatic saves, artifact collection paths,
  Linux artifact naming, CLI provenance and rollback behavior.

## Verification evidence

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/desktop test` after fix | 73 files, 827 tests passed |
| Focused close/tray/prompt/settings/daemon regression | 8 files, 94 tests passed |
| `pnpm --filter @multica/views exec vitest run locales/parity.test.ts` | 60 passed |
| `pnpm --filter @multica/desktop typecheck` | Node and renderer passed |
| `pnpm --filter @multica/desktop lint` | 0 errors; 1 pre-existing Hook warning at `tab-content.tsx:54` |
| Windows x64 NSIS and Linux x64 AppImage cross-build | Exit 0; rebuilt after the fix |
| Packaged main/preload/renderer | 401 files per platform match the current build byte-for-byte |
| Packaged fix, tray icon, IPC and executable architecture | Passed; Windows PE x64 and Linux ELF x64; CLI matches source commit |
| Metadata / size / SHA-512 | Passed for both generated update channels |
| Isolated Nginx full GET / HEAD / Range | Passed on both artifacts; Windows blockmap also matches local bytes |
| Nginx health / directory / PUT | 200 / 404 / 403 |
| `git diff --check` | Passed |

The packaging `.test.mjs` files import Vitest. An initial `node --test` attempt
used the wrong runner; the final Desktop Vitest suite above includes those files
and passes. The failed invocation is retained only in the build diagnostics.

## Candidate artifacts

Directory: `dist/release/v0.5.3/` (local, ignored by Git).

- `multica-desktop-0.5.3-windows-x64.exe`, `.blockmap`, `latest.yml`.
- `multica-desktop-0.5.3-linux-x86_64.AppImage`, `latest-linux.yml`.
- `verification-v0.5.3.json`, platform HTTP reports, SHA-256 checksums, README
  and this task's `TESTING.md`.
- `_build/` retains logs, static verifier, CLI build metadata and `source.patch`.

Source: `5fb28713bfcf548c83737fa0755b647517956f59` plus the settings fix recorded
in `_build/source.patch`. Desktop candidate version: `0.5.3`. Bundled CLI:
`v0.5.2-69-g5fb28713b-dirty`. These are local verification candidates; create an
approved release tag and rebuild before a real release so version stamps align.
No server/Web upgrade package was produced.

## Deferred native matrix

| Target | Status | Reason / remaining evidence |
| --- | --- | --- |
| Windows 10/11 x64 | Pending | No test machine; native install/upgrade, visible tray, second-instance focus, daemon exit and theme contrast |
| Ubuntu 22.04 GNOME without a tray host | Pending | No desktop session; actual unsupported UI/fallback and warning |
| Ubuntu 22.04 KDE/XFCE with a tray host | Pending | No desktop session; actual tray click/menu, focus and theme contrast |

Vitest platform mocks, binary inspection and HTTP checks do not cover these
native rows. Rerun `TESTING.md` when machines are available. Existing pnpm and
CSS optimization warnings, plus Linux's default Utility category, remain outside
this small settings fix.
