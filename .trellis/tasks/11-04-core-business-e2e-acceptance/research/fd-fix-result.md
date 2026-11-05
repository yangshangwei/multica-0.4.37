# Linux daemon inherited-descriptor fix

The isolated product candidate remains based on1a32ca36c. Only the background daemon launch and self-update successor launch boundary changes: a Linux-specific helper reads /proc/self/fd, marks open descriptors >=3 close-on-exec, and refuses unexpected syscall errors. Already-closed descriptors are ignored. No parent descriptor is closed; intended child standard and ExtraFiles mappings are still explicit os/exec mappings. Non-Linux builds preserve existing behavior. No new dependencies.

A real Linuxarm64 subprocess test intentionally inherits a pipe and file at fd64/65. An intermediate process uses the same helper and os/exec mapping, forwards exact synthetic managedstdin, and keeps the leaf alive. The outer test requires pipeEOF while the leaf is alive and checks explicit ExtraFiles/stdout/stderr, plus parent writeability.

RED replaces only the new helper with a noop through a private Go overlay: exact named test failed with fd64/65 retained and pipeEOF timeout. GREEN passed in0.06s; after fixing fixture cleanup to wait exactly once, finalGREENpassed0.07s. No DB, network, real provider or external account access. Original helper-noop overlay and all logs/hashes retained.

Preparation errors retained: a LinuxELF was accidentally attempted on the macOS host (no test body), and a later compile from wrong cwd produced binaries without the new test (explicit no-tests-to-run rejected, never countedPASS). Corrected final binaries built from the exact isolatedworktree, with exactnamedtest events verified.

Both production spawn sites call the helper before Start; the existing Windows breakaway retry also calls the platform no-op. /proc enumeration avoids a new minimumkernel requirement for close_range. Go-created descriptors useCLOEXEC already; only intentionally inheritable parent descriptors change. This is a process-wide flag change before spawn; parent access remains usable and failure stops daemon launch.

Remaining verification: independentreview, broadfocusedcmd/multicacompile/test as appropriate, then package/currentoriginalLinuxE2E to prove r7 close completes. This patch does not yet establish full installedMain acceptance.
