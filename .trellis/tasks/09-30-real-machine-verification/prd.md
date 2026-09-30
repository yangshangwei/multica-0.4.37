# Child 4: real-machine-verification

## Scope

Real-machine verification on Windows 10/11 and Ubuntu 22.04 GNOME + KDE /
XFCE. Packaging through the existing intranet release flow. Depends on
children 1-3.

## Deliverables

1. **Windows 10 or 11 verification**
   - Install the packaged `.exe` or run the unpacked build.
   - Walk through every AC from the parent PRD.
   - Confirm tray icon is selectable / focusable, Quit from the tray kills
     the background daemon-child correctly (cross-check with the daemon
     `autoStop` pref).
   - Confirm second instance launch (`multica.exe` re-launch from Start
     menu) shows the existing window rather than failing on the single
     instance lock.

2. **Ubuntu 22.04 GNOME (default session, Wayland)** verification
   - Confirm the degraded path: no tray, settings option disabled, closing
     performs a real close.
   - Confirm the one-line warn appears in stdout when launched from a
     terminal (`multica-desktop &` then look for `[close-behavior]`).

3. **Ubuntu 22.04 KDE Plasma or XFCE (X11)** verification
   - Confirm tray appears, click works, Quit works.
   - Confirm theme-contrast of the tray icon (light / dark system themes).

4. **Existing intranet release flow** — replicate the steps in
   `intranet-upgrade-packaging/` (auto-memory). Bump the desktop tag
   (server version bump is N/A here). Drop the packaged binaries into
   `dist/release/<new-version>/`.

5. **Smoke README** — append a short section to the existing release notes
   (or produce a `TESTING.md` if one doesn't exist) with the exact
   verification matrix from this PRD so the next person can rerun it.

## Out of scope

- macOS verification (no behavior change).
- Anything server-side.
- Unit test authoring (already covered in children 1-3).

## Acceptance Criteria

- A/C 4.1: Each row in the verification matrix has a ✅ or a written
  justification for deviation.
- A/C 4.2: Packaged artifacts land in `dist/release/vX.Y.Z/` per the
  existing intranet layout (`intranet-upgrade-packaging.md`).
- A/C 4.3: A short note in the appropriate `TESTING.md` (or wherever
  verification checklists live) records the steps + OS / WM combos tested.

## Notes

- Pay attention to auto-memory: `desktop-first-run-needs-a-packaged-build`
  (dev mode skips the setup page) and `offline-upgrade-smoke-containerd-*`.
- Docker cannot bind-mount agent worktrees — so for any containerized smoke
  test, follow the existing workaround (streaming a tar into a pinned
  image) instead of trying to mount.
