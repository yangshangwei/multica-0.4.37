# Linux restart: inherited descriptors keep Playwright process close pending

## Actual r7 observation

The improved timeline records command35 `restart` entering `restart.close` with no completion. While it was stuck, two independent read-only observations (`docker top`, `/proc`) showed **no Electron main or renderer processes**. Only init, Xvfb, the Node test controller and the detached Go `multica` daemon remained. The daemon had been adopted by containerPID1. Consequently an Electron `before-quit` callback still running is ruled out for this observed stall.

The Go daemon retained dozens of Electron descriptors: `icudtl.dat`, `v8_context_snapshot.bin`, `app.asar`, Chromium pak files, profile LevelDB locks, and multiple sockets. DaemonFD3/4 were sockets765999/766001 with flags02 (not close-on-exec); controllerFD23/25 held sockets765998/766000. stdout/stderr were already redirected to `daemon.err.log`, so ordinary log stdio is not the retained pipe pair. The endpoint adjacency is consistent with pair ownership, but no independent peer query completed before the container was naturally cleaned up; that exact peer relationship is not claimed. Curated bounded observations are in `linux-restart-rootcause-observation.json`; the later failed snapshot remains in `linux-restart-rootcause-proc.json`.

## Verified source mechanism

- Local Playwright1.58.2 `lib/server/utils/processLauncher.js:105` creates five stdio entries (`ignore`, `pipe`, `pipe`, `pipe`, `pipe`). Its `:153` completion handler listens to Node child-process **close**, not merely process exit. Only this handler invokes the Electron application's onExit hook.
- `lib/server/electron/electron.js:77–84` invokes real `app.quit()`, closes the Node debugging connection, then awaits the application Close event. That event depends on the processLauncher callback above. Retained descendant descriptors can therefore prevent close after the main executable has already exited.
- Frozen `server/cmd/multica/cmd_daemon.go:643–649` creates the background Go command, forwards managed handoff through stdin, redirects stdout/stderr, and sets `daemonSysProcAttr(true)`. `cmd_daemon_unix.go:19` only sets Setsid; it does not mark inherited FD>=3 close-on-exec.
- The actual Go1.27 runtime `src/syscall/exec_linux.go:625–634` explicitly documents that inheritedFD>=3 require the caller to set close-on-exec. Its file mapping preserves them unless already CLOEXEC. This matches the observed daemon fdtable.

**Confirmed defect:** the detached daemon retains unrelated Electron resources across its launch. **Strong causal explanation of r7:** those descriptors retain Playwright child stdio endpoints, so Electron itself exits while the harness still awaits child-process close. The remaining proof step is a focused regression/differential launch demonstrating EOF after the daemon spawn boundary is corrected, followed by original r7 reproduction. No signal or FD intervention was applied to the original run.

## Minimal repair proposal for root approval

Repair the **detached CLI process boundary**, not the browser assertion: before spawning/restarting the Unix background daemon, mark inherited non-stdio descriptors close-on-exec while preserving intentionally forwarded managed-handoff/stdout/stderr channels. Do not close live Go-owned descriptors in the parent; marking CLOEXEC affects only the nextexec, and Go's explicit standard-file mapping clears it on the intended child channels.

A Linux-specific helper may enumerate `/proc/self/fd` and setFD_CLOEXEC onFD>=3, with the enumeration descriptor ignored if closed; alternatively use supported `close_range(CLOEXEC)` with a deliberate compatibility contract. Select the smallest supported platform-specific implementation after checking the repository's runtime/OS support. Do not add generic retry/fallback behavior merely to hide unknown errors. Cover both normal daemon start and self-update restart if they share the same launch vulnerability. macOS and Windows behaviors remain separately considered; this observation is Linux.

Regression seam: a helper process starts with a deliberately inheritable extra pipe/file descriptor (no database, credentials or model). It calls the real production launch-preparation helper then starts a child which enumerates fd targets and reports whether the designated inherited inode survived. An independent parent reads EOF on the leaked pipe while the spawned child remains alive, and verifies intended stdin/stdout managed-handoff transfer still works. The unmodified preparation must fail this; repaired code must pass. Do not mirror the helper implementation as the assertion. Use task-owned Linux process/container with externalHOME/TMP; preserve original product/package hashes and new candidate identity if applying a product fix.

No product code was edited and no new test was run by this diagnostic slice.
