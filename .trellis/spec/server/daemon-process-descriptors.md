# Linux daemon descriptor isolation

## Scope
Background CLI daemon start and self-update successor launch, especially when an installed Electron application launches the CLI.

## Signatures
`prepareDaemonChildFds() error` in `server/cmd/multica/daemon_fd_linux.go`; complementary `!linux` implementation preserves other platforms. Call immediately before each `exec.Cmd.Start` in both daemon launch paths.

## Contract
On Linux enumerate `/proc/self/fd`, preserve existing flags and mark open descriptors >=3 `FD_CLOEXEC`. Do not close parent descriptors. Go's explicit stdin/stdout/stderr and `ExtraFiles` mappings remain intentional child channels. Keep managed-handoff input and error-log mappings. No new kernel close_range requirement.

## Errors
Ignore only `EBADF` caused by a descriptor closing during enumeration. Other enumeration/fcntl errors prevent daemon launch. Descriptor0/1/2 are untouched.

## Cases
An Electron-launched CLI can inherit debug sockets, Chromium files and profile locks; Setsid alone does not prevent further inheritance. After the application exits, a detached daemon must not keep its parent's process-close event pending. Explicitly forwarded channels must still work.

## Verification
Run `TestDaemonInheritedDescriptorsDoNotKeepParentPipeOpen` on actual Linux. The test intentionally inherits fd64/65, requires EOF while the daemon child remains alive, and checks parent writeability plus explicit managedstdin/stdout/stderr/ExtraFiles. A noop preparation must fail; repaired preparation must pass. Confirm original packaged Main restart.close reaches completion without timeout/retry changes.

## Wrong / correct
Do not fix this by killing an otherwise valid background daemon, weakening the app-close assertion, or closing all live parent FDs. Repair inheritance at the CLI spawn boundary, preserve intentional child mappings, and retain original failed evidence.

Evidence: task `11-04-core-business-e2e-acceptance`, `research/fd-fix-result.md` and fixed-CLI original packaged Main run. Base candidate1a32 + FDpatchd6f4ceeb passed the complete flow; original unpatched failure remains distinct.
