# Linux descriptor repair: independent executed-evidence review

**PASS for the focused real Linux regression and final test-cleanup revision.** The parent independently read the original command/result/stdout records and hashed the retained source, patch and executable files. This support review launched no new tests, builds, containers, database or Provider.

## Observed results

| Run | Actual named result | What it establishes |
|---|---|---|
| Helper replaced by no-op | FAIL,0.82s,exit1 | FD64/65 survive and the parent pipe times out waiting for EOF while the leaf remains alive. |
| Actual Linux helper | PASS,0.06s,exit0 | Same named regression excludes unrelated inherited descriptors while preserving parent writeability, exact managed stdin, stdout/stderr and explicit ExtraFiles. |
| Final fixture cleanup revision | PASS,0.07s,exit0 | Same assertions pass after registering immediate child Wait/cleanup and preventing double consumption. |

All three executables match their recorded SHA-256 and are ELF64 little-endian AArch64. Commands use the same pinned Linux image, network none, read-only root filesystem, private tmpfs state, one anchored test and count1. The baseline overlay replaces only daemon_fd_linux.go with a no-op helper; it does not remove assertions or alter the product caller. Early wrong-host/wrong-directory preparation failures remain in the original evidence and are not counted as tests passing.

## Source and cleanup review

The three production files retain the hashes reviewed in static-review.md. Final test hash is `3b01449bbe6c2c2776f6d7f6fb0631c11052d4fcc06752b69420112de3d81e6c`; complete four-file and patch hashes are in runtime-evidence-review.json.

The previously noted PID-file-publication cleanup gap is closed: after child.Start, the bridge immediately creates one Wait goroutine and installs deferred kill/drain before writing child-pid. Both success and timeout paths set consumed after reading the channel. Thus a failing PID publication or intermediate assertion does not leave this test branch without a wait/kill owner. This is source-reviewed and exercised by the final ordinary regression; a separately injected PID-write failure was not run.

## Remaining acceptance boundary

This confirms the inheritance mechanism and the focused repair. It does not yet prove the original packaged Electron restart/close workflow, self-update end-to-end behavior, or macOS/Windows runtime behavior. The core owner must bind a rebuilt fixed CLI into a separately identified package variant, preserve the prior package and r7 failure, then execute the original full Main test with its business/Stop/Resume/recovery assertions intact. Only that result can close the remaining Linux full-flow gate.

User-accepted2155.025ms performance deviation is unrelated and remains nonblocking for this recorded candidate. No P95 work was reopened.
