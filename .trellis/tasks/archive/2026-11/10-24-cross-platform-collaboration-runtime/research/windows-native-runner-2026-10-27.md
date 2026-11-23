# Research: Windows native runner and failure matrix

- Query: Determine how to execute the prepared Windows native collaboration verification and what evidence it can establish.
- Scope: internal repository and remote-readiness inspection
- Date: 2026-10-27

## Findings

The verification snapshot is local branch `codex/verify-collaboration-host-runtime-20261026` at commit `92bb6d86b6305de37e926078deb703d977d2ee93`. Its workflow is intentionally isolated from product branches and triggers only on a push to that exact branch (`.github/workflows/collaboration-host-native-verification.yml:1-8`). The matrix runs `ubuntu-latest`, `macos-latest`, and `windows-latest` with Go 1.26.x and Node 22 (`.github/workflows/collaboration-host-native-verification.yml:13-36`).

On Windows, the workflow runs the selected native packages with `go -C server test -json -p 1 -parallel 2 -count=1 -timeout=5m ./pkg/agent ./internal/daemon/collaborationrun ./internal/daemon ./internal/cli -run '^(TestHost|TestCollaborationHost)'`; Unix runners additionally use the repository's agent-CLI guard (`.github/workflows/collaboration-host-native-verification.yml:42-53`). Each runner retains `go version`, `go env GOOS GOARCH`, and JSONL test evidence as an Actions artifact (`.github/workflows/collaboration-host-native-verification.yml:54-59`).

The native evidence checker requires 11 cross-platform host cases. On Windows it adds five Windows-specific cases: `TestHostWindowsLedgerProtectsFilesAndLocksAcrossProcesses`, `TestHostWindowsLedgerRejectsSharedACL`, `TestHostWindowsLedgerSupportsLongPaths`, `TestHostLifecycleWindowsWrapperRequiresJobBeforeResume`, and `TestHostLifecycleWindowsWrapperWaitRequiresManagedChildEnd` (`scripts/check-collaboration-native-tests.mjs:9-30`). It rejects both failed cases and skipped required parent/child cases, because a Go parent can pass while required nested cases skip (`scripts/check-collaboration-native-tests.mjs:31-53`).

The Windows test source confirms the failure matrix is native-only: it exercises process identity from the original handle, Job attachment refusal before resume, live managed-child detection retaining an unknown barrier, ACL/file protection, cross-process locking, and long paths. The key Job tests are guarded by `//go:build windows` and include explicit `ErrLaunchUnsupported`/`ErrLaunchUnconfirmed` assertions (`server/pkg/agent/launch_lifecycle_windows_test.go:1-5,39-109,111-165`). Cross-compilation cannot establish these behaviors.

Remote readiness is sufficient for a later explicit push: `origin` is `git@github.com:yangshangwei/multica-0.4.37.git`, `upstream` is `https://github.com/multica-ai/multica.git`, GitHub CLI is authenticated as `yangshangwei` with `repo` and `workflow` scopes, and the snapshot branch is absent from both remotes (`git ls-remote --heads ...` returned no ref). No remote mutation was performed.

The current task documentation has a stale phrase in `verification.md:100-105`: it still lists a separately authorized real-Provider smoke as pending, while the S6 evidence records the Claude Code 2.1.310 macOS arm64 smoke as passed. The remaining account gate is the complete installed-delivery/account journey, not the initial smoke itself. The platform table's `Authenticated Provider/version` cells remain `Not run` because full admission evidence is still absent.

## Recommended execution sequence

1. Finish and verify the macOS packaged fixture rerun locally; keep production admission closed.
2. After the local fixture changes pass narrow checks, push the snapshot branch to `origin` only with explicit remote-side-effect authorization; the push automatically triggers the three-OS workflow.
3. Treat the Windows job as passing only if all 16 required cases (11 shared + 5 Windows-specific) report `pass` and no required descendant reports `skip`; retain the uploaded `native-collaboration` artifact and record OS/arch from `runtime.txt`.
4. Do not claim Windows arm64 support from this workflow: `windows-latest` is one hosted runner/architecture. Windows arm64 and Windows packaged installers remain separate gates.
5. Update `verification.md`, `progress.md`, and the S6 matrix with the actual workflow URL, job results, runtime metadata, and artifact hashes; correct the stale smoke wording before final Trellis check.

## Related specs

- `.trellis/spec/server/collaboration-host-execution.md` — native lifecycle, Job/process ownership, unknown barriers, and default-closed admission.
- `.trellis/spec/web/frontend/e2e-run-environment.md` — task-owned production Web/API environment and evidence provenance.
- `.trellis/tasks/10-24-cross-platform-collaboration-runtime/verification.md` — platform/provider matrix and remaining gates.
- `.trellis/tasks/10-24-cross-platform-collaboration-runtime/research/s6-delivery-matrix-2026-10-26.md` — current S6 candidate combinations.

## Caveats / Not Found

- No Windows runner or hosted workflow has executed yet; no native Windows result is claimed.
- The workflow's Windows matrix covers the hosted runner's architecture only, not both amd64 and arm64.
- Pushing the verification branch and triggering Actions are external state changes and were intentionally not performed by this research agent.
