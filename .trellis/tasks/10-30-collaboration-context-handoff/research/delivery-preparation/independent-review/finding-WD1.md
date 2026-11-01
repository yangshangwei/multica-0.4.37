# WD-1 — Windows Host cleanup still uses POSIX resume

Blocking for the new Windows installed collaboration journey. `e2e/fixtures/collaboration-host-runtime.ts:302` unconditionally calls `child.kill("SIGCONT")` on its still-running native daemon before `child.stdin.end()`. The new Windows journey reaches `fixture.daemon.close()` unconditionally and requires exit0 plus the final claims/HTTP summary. The actual Go Host fixture emits that summary after stdin EOF cancels its poller (`server/internal/daemon/collaboration_host_router_test.go:179–186`).

SIGCONT is a POSIX resume operation, not a Windows resume operation. The installed Node22 headers' `uv/win.h` lists Windows signal definitions without SIGCONT. On a Windows unsupported-signal path, close throws before EOF; any forced-termination interpretation would also prevent the required graceful summary. The unchanged underlying fixture became newly reachable on Windows through this patch.

`host-close-probe.mjs` extracts the actual close function and invokes it with a child that rejects SIGCONT. It confirms that stdin EOF is not reached, without a process, GUI, Provider or database. This is an executable modeled portability regression, not native Windows evidence. A Node documentation fetch was attempted but the web tool returned a gateway404; no remote source is cited as verified.

Minimal repair: skip the POSIX resume signal on win32 in the normal close path, retain stdin EOF and its bounded fallback, and add a Windows-shaped cleanup regression. The exposed suspend/resume helpers also use SIGSTOP/SIGCONT and should remain explicitly out of Windows coverage unless separately implemented; the installed journey itself does not call them. Do not change the final exit/claims assertions to accept abrupt shutdown.
