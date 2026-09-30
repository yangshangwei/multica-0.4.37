# Close Behavior — Real-Machine Verification Checklist (v0.5.3)

> **Scope for this checklist**: the close-behavior feature shipped under parent
> task `09-30-desktop-close-behavior`. Verify each row before publishing
> `latest.yml` for Windows / Linux.
>
> **Out of scope**: macOS. Its dock / window semantics did not change in this
> release — see the spec at `.trellis/spec/desktop/frontend/close-behavior.md`.

## Setup matrix

Every row needs both a fresh-profile test AND an upgrade-from-v0.5.2 test.
Please record OS, WM / desktop environment, and session type.

| #  | OS / Version                          | WM / DE       | Session | Notes |
|----|----------------------------------------|---------------|---------|-------|
| W1 | Windows 10 (22H2)                      | —             | —       | Any 64-bit install |
| W2 | Windows 11 (23H2 or later)             | —             | —       | Preferred primary target |
| L1 | Ubuntu 22.04 LTS (Jammy)               | GNOME         | Wayland | **Degraded path** — Tray unsupported |
| L2 | Ubuntu 22.04 LTS (Jammy)               | KDE Plasma    | X11     | Supported path |
| L3 | Ubuntu 22.04 LTS (Jammy)               | XFCE          | X11     | Supported path |
| L4 | (Optional) Fedora 39+                  | GNOME         | Wayland | Same as L1 |

For each cell, run three scenario setups:

- **Setup A**: fresh install → first close should show the prompt.
- **Setup B**: install over v0.5.2 (no stored `close-preferences.json`) — same expectation as Setup A.
- **Setup C**: install over a build where `closeBehavior` was previously
  saved as `minimize`. The first close under the new build must respect it
  without prompting (or fall back gracefully if tray unsupported).

## Test cases

For each setup + scenario, record ☐ → ☑.

### T1. First close (Setup A / Setup B)

- ☐ Closing the main window opens the AlertDialog with three options:
  Quit / Minimize to tray / Cancel + a "Remember my choice" checkbox.
- ☐ Cancel closes the dialog without applying any behavior; the window
  stays open.
- ☐ Quit (no remember) closes the app. Daemon `autoStop`'s pref decides
  whether the daemon stops.
- ☐ Minimize (no remember) hides the window; the tray icon appears. (On
  Linux GNOME+Wayland the Minimize button is **hidden** — verify Quit and
  Cancel still work.)

### T2. Subsequent close (Setup A)

- ☐ Closing immediately after T1 (no remember) shows the prompt again.
- ☐ Choosing Quit + "Remember my choice" → next close skips the prompt
  and exits directly.
- ☐ Choosing Minimize + "Remember my choice" → next close hides the
  window without prompting.

### T3. Tray icon behavior (only where supported — skip on L1/L4)

- ☐ Tray icon is clickable; left click shows the window.
- ☐ Tray menu has "Show Multica" and "Quit".
- ☐ "Show Multica" restores a minimized-to-tray window.
- ☐ "Quit" from the tray menu exits the app completely (window gone,
  process gone, daemon handled per its own pref).
- ☐ Tray icon disappears once the main window is visible again (no
  duplicate icon in the notification area).

### T4. Second instance

- ☐ While the app is minimized to tray, launching the .exe / .desktop
  entry again brings the existing window forward. A second Multica
  process must NOT spawn.

### T5. Settings tab (all setups)

- ☐ Settings → Behavior shows current value.
- ☐ Switching to Quit + saving persists across restart.
- ☐ Switching to Minimize + saving persists across restart.
- ☐ Switching to Ask + saving → next close prompts again.
- ☐ (L1 / L4) Minimize option is hidden; helper text explains the tray
  is unavailable on this system.

### T6. Linux GNOME graceful degradation (L1 / L4 only)

- ☐ With `closeBehavior = minimize` saved from a previous session,
  closing the window performs a real close (no wedging), and a single
  `[close-behavior] minimize requested but tray unsupported; closing
  instead` is logged in stdout. Launch with `multica-desktop &` from a
  terminal to see the log.

### T7. Interop with daemon prefs

- ☐ `~/.multica/desktop_prefs.json` is untouched across every scenario.
- ☐ Minimize keeps the daemon running. Quit triggers daemon cleanup
  per its own `autoStop` pref.

### T8. Theme / contrast (pick one Windows + one Linux)

- ☐ Tray icon is legible on a light system theme.
- ☐ Tray icon is legible on a dark system theme.

## Package & publish (only after every box above checks)

This release carries version `v0.5.3`.

1. Tag the commit serving as the v0.5.3 baseline:
   ```bash
   git tag v0.5.3 <commit-sha>
   git push origin v0.5.3
   ```
2. Build the desktop installers per platform **on the matching host or
   in the documented cross-compilation environment**. Do **not** build
   the Windows installer on macOS.
   ```bash
   cd apps/desktop
   pnpm package -- --win --x64
   pnpm package -- --linux --x64
   ```
3. Collect and publish via the desktop-updates script:
   ```bash
   bash scripts/desktop-updates.sh collect \
       apps/desktop/dist/win-unpacked \
       /srv/incoming/desktop-0.5.3-windows-x64

   bash scripts/desktop-updates.sh publish \
       /srv/incoming/desktop-0.5.3-windows-x64

   bash scripts/desktop-updates.sh verify \
       latest.yml --expected-version 0.5.3
   ```
4. Repeat the collect/publish/verify trio for the Linux x64 channel.

## Deliverables back to the task

- A photo or short recording of the tray icon + menu on each supported
  platform.
- The output of the final `verify` runs (already JSON-formatted).
- An updated `dist/release/v0.5.3/verification-v0.5.3.json` per the
  existing release layout (see `dist/release/v0.5.2/` for shape).
