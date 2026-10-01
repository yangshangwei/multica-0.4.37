# Close Behavior — Real-Machine Verification Checklist (v0.5.3)

> **Scope for this checklist**: the close-behavior feature shipped under parent
> task `09-30-desktop-close-behavior`. Verify each row before publishing
> `latest.yml` for Windows / Linux.
>
> **Out of scope**: macOS. Its dock / window semantics did not change in this
> release — see the spec at `.trellis/spec/desktop/frontend/close-behavior.md`.

## Local continuation status (2026-09-30)

The user confirmed that target machines are unavailable; this continuation
covers local regression, cross-build and artifact/download verification only.
See `verification.md` and `dist/release/v0.5.3/verification-v0.5.3.json` for
actual results. Native rows below remain **pending**, including GUI, installer,
upgrade, tray visibility, theme contrast and real second-instance activation.

The current contract is `.trellis/spec/desktop/frontend/close-behavior.md`.
Linux capability depends on the session D-Bus StatusNotifier host, **not** the
desktop name or Wayland/X11 alone. Record the result of:

```bash
gdbus call --session --dest org.kde.StatusNotifierWatcher \
  --object-path /StatusNotifierWatcher \
  --method org.freedesktop.DBus.Properties.Get \
  org.kde.StatusNotifierWatcher IsStatusNotifierHostRegistered --timeout 1
```

`(<true>,)` indicates a host. If `gdbus` is absent, the app tries
`busctl --user --timeout=1s get-property org.kde.StatusNotifierWatcher
/StatusNotifierWatcher org.kde.StatusNotifierWatcher
IsStatusNotifierHostRegistered` (expected `b true`). Missing tools, host or a
failed probe take the unsupported path. GNOME with a working extension may
support trays; KDE/XFCE without a host may not.

## Setup matrix

Every row needs both a fresh-profile test AND an upgrade-from-v0.5.2 test.
Please record OS, WM / desktop environment, and session type.

| #  | OS / Version                          | WM / DE       | Session | Notes |
|----|----------------------------------------|---------------|---------|-------|
| W1 | Windows 10 (22H2)                      | —             | —       | Any 64-bit install |
| W2 | Windows 11 (23H2 or later)             | —             | —       | Preferred primary target |
| L1 | Ubuntu 22.04 LTS (Jammy)               | GNOME         | Wayland | Test without a StatusNotifier host; also test with an extension when available |
| L2 | Ubuntu 22.04 LTS (Jammy)               | KDE Plasma    | X11     | Confirm StatusNotifier host before testing supported path |
| L3 | Ubuntu 22.04 LTS (Jammy)               | XFCE          | X11     | Confirm StatusNotifier host before testing supported path |
| L4 | (Optional) Fedora 39+                  | GNOME         | Wayland | Record actual host availability |

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
- ☐ Minimize (no remember) hides the window; the tray icon appears. (When the tray host is unavailable, Minimize is **hidden** — verify Quit and
  Cancel still work.)

### T2. Subsequent close (Setup A)

- ☐ Closing immediately after T1 (no remember) shows the prompt again.
- ☐ Choosing Quit + "Remember my choice" → next close skips the prompt
  and exits directly.
- ☐ Choosing Minimize + "Remember my choice" → next close hides the
  window without prompting.

### T3. Tray icon behavior (only when a tray host is available)

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
- ☐ Selecting Quit (saved automatically) persists across restart.
- ☐ Selecting Minimize (saved automatically, where supported) persists across restart.
- ☐ Selecting Ask (saved automatically) → next close prompts again.
- ☐ (Any unsupported session) Minimize option is hidden; helper text explains the tray
  is unavailable on this system.

### T6. Linux graceful degradation (any session without a tray host)

- ☐ With `closeBehavior = minimize` saved from a previous session,
  closing the window performs a real close (no wedging), and a single
  `[close-behavior] tray unavailable; quitting instead of hiding` is logged in stdout. Launch with `multica-desktop &` from a
  terminal to see the log.

### T7. Interop with daemon prefs

- ☐ `~/.multica/desktop_prefs.json` is untouched across every scenario.
- ☐ Minimize keeps the daemon running. Quit triggers daemon cleanup
  per its own `autoStop` pref.

### T8. Prompt lifecycle and unavailable stored preference

- ☐ Keep an acknowledged prompt open for more than five seconds; it must
  remain open. Only delivery acknowledgement has a five-second deadline.
- ☐ Close repeatedly while a prompt is open; only one prompt is active.
- ☐ Reload the renderer with a prompt open; the pending choice cancels and
  the next close can open a fresh prompt.
- ☐ In Setup C without a tray host, the selected value retains its translated
  “Minimize to tray” label (also check Chinese); the dropdown offers only Quit
  and Ask, and displays the unsupported explanation.
- ☐ Close with an auxiliary issue window open and select Quit; all application
  windows exit through daemon cleanup.

### T9. Theme / contrast (pick one Windows + one Linux)

- ☐ Tray icon is legible on a light system theme.
- ☐ Tray icon is legible on a dark system theme.

## Local candidate packaging

The candidate Desktop version is `0.5.3`. A candidate is not a published
release. No tag or update channel is changed by this local verification.
The wrapper supports cross-building; such a build proves packaging only,
not native installation or desktop-session behavior.

```bash
MULTICA_DESKTOP_VERSION=0.5.3 CSC_IDENTITY_AUTO_DISCOVERY=false \
  pnpm --filter @multica/desktop package -- --win --linux --x64 --publish never
bash scripts/desktop-updates.sh collect \
  apps/desktop/dist/win-x64 dist/release/v0.5.3/windows-x64
bash scripts/desktop-updates.sh collect \
  apps/desktop/dist/linux-x64 dist/release/v0.5.3/linux-x64
```

The combined invocation builds bundles once, scopes platform output, and
cross-builds Linux AppImage on macOS. Collect from the directory containing
installers **and** generated metadata, never from `win-unpacked` alone.
The Desktop override does not change the bundled CLI's git-derived version;
record both plus the exact source commit/diff in candidate verification.
Release packaging should use the approved tag so both versions align.

## Publication after native acceptance

Publication is a later operation after target-machine results are reviewed.
Use the existing configured intranet storage and run one platform at a time:

```bash
bash scripts/desktop-updates.sh publish /srv/incoming/desktop-0.5.3-windows-x64
bash scripts/desktop-updates.sh verify latest.yml --expected-version 0.5.3
bash scripts/desktop-updates.sh publish /srv/incoming/desktop-0.5.3-linux-x64
bash scripts/desktop-updates.sh verify latest-linux.yml --expected-version 0.5.3
```

`verify` accepts a metadata **filename**, not an absolute file path. For an
isolated smoke server use `apps/desktop/scripts/verify-updates.mjs --url
http://127.0.0.1:PORT/desktop/ --metadata latest.yml --expected-version 0.5.3`.
Do not infer GUI or updater-install success from HTTP integrity checks.

## Deliverables back to the task

- A photo or short recording of the tray icon + menu on each supported
  platform.
- The output of the final `verify` runs (already JSON-formatted).
- An updated `dist/release/v0.5.3/verification-v0.5.3.json` per the
  existing release layout (see `dist/release/v0.5.2/` for shape).
