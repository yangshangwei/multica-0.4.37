# Linux daemon descriptor boundary: static review

**PASS for the production change; no blocking finding.** This is a static source review, not runtime verification. Reviewed on 2026-11-05 at 10:50 UTC against `/Volumes/artisan/code/2026/mc-core-fd-fix-20261105`, HEAD `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc` plus the four files below. Their hashes were unchanged across review and match `source-observation.json`.

| File under `server/cmd/multica/` | SHA-256 |
| --- | --- |
| `cmd_daemon.go` | `7b0edfb4dea604c6383b05e799b90541f1ac0663a52bffa594a50acfef5d9833` |
| `daemon_fd_linux.go` | `6c3321981381b34a135a98566a2ec0a6b9607a9a9010cbb5c511846a768bc2d4` |
| `daemon_fd_other.go` | `82716ee1bf0756274b1700ee4f3ee7ed182f2b1a4180a77c69a6f6c6c3c0943c` |
| `daemon_fd_linux_test.go` | `1ee7be28bee30e62b4ff5b94cc484940f112cb779f1c476ea763db88fd2ae9c2` |

## Findings (fixed)

None. Product and test source were read only; this report is the sole write.

## Findings (not fixed)

One nonblocking fixture cleanup gap: `daemon_fd_linux_test.go:83` can fail publishing `child-pid` after the leaf starts but before its wait/kill handling is installed. The outer cleanup waits for the bridge, so a bridge that has already failed can leave the detached leaf briefly running; publication failure also removes the PID-based fallback. The release file or the leaf's 12-second deadline normally ends it, but cleanup does not synchronously prove that exit on this branch. The owner can register leaf kill/wait cleanup immediately after successful Start. No source change was made because this lane is explicitly read only. This does not block the intended red/green experiment or identify a production FD defect.

## Mechanism checked

- `daemon_fd_linux.go:20` enumerates `/proc/self/fd`, skips descriptors below 3, preserves existing flags and sets `FD_CLOEXEC`. It never closes a parent descriptor. Only `EBADF` from either fcntl call is tolerated as a closed-descriptor race; enumeration and other fcntl errors propagate and prevent spawning. Linux therefore requires readable `/proc/self/fd`.
- `cmd_daemon.go:655` and `:1228` prepare descriptors immediately before initial daemon and self-update successor starts. Both retry sites also prepare. Error returns reach existing log-close/reservation cleanup. The background retry rebuilds the command and reattaches managed stdin and both log streams. The successor retains its existing removal of the consumed managed-handoff flag.
- Local Go 1.27.0 source confirms explicit stream preservation: `os/exec/exec.go:721` assembles stdin/stdout/stderr followed by `ExtraFiles`; `syscall/exec_linux.go:603` clears CLOEXEC when source equals destination, and `:614` duplicates with flags zero otherwise. Thus marking the original log/ExtraFiles descriptors does not remove intended child mappings. `forwardManagedHandoff` supplies a bytes.Reader; exec creates and forwards its stdin pipe during Start.
- The complementary `linux`/`!linux` build tags provide one implementation. The non-Linux function returns nil, preserving the existing Windows access-denied retry without breakaway and its stream setup. No new dependency, template or platform detection touch point is introduced.
- The regression deliberately duplicates inherited pipe/file descriptors to 64/65, avoiding accidental overwrite by the intended fd3 mapping. It checks their absence by resource identity, exact managed stdin, explicit ExtraFiles content, stdout/stderr content, a successful parent write after preparation, real pipe EOF, and a still-existing leaf PID before release. Removing preparation leaves both a descriptor witness and a live writer that prevents EOF. Normal assertion failures still run release/wait cleanup. This is meaningful regression design, not evidence that it has executed successfully.
- The fixture re-executes the test binary with one anchored test name and explicit small environments. Its inspected TestMain only clears environment keys and runs tests; the fixture invokes no agent CLI or daemon startup/discovery.

## Verification

- Static review: PASS, with the nonblocking fixture note above.
- Lint / TypeCheck / tests / builds: NOT_RUN, explicitly excluded from this review lane.
- No database, container, browser, network, or agent-account operations were performed.
- Actual Linux red/green, complete Main/daemon flow, self-update execution and cross-platform compilation remain the execution owner's evidence. This review makes no runtime or acceptance claim.
