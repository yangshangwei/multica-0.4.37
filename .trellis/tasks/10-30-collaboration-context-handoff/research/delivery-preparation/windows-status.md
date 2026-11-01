# Windows installed collaboration preparation

Owner: native child `/root/windows_delivery`. Worktree `mc-collaboration-delivery-20261101`, branch `codex/collaboration-delivery-20261101`, takeover base `7240d8542` clean. Root's guide and native checker commits are preserved. No other worktree writes, external actions, database commands, migrations, installed Agent CLI or real Provider calls are permitted.

## Smallest behavior gap

The installed collaboration journey already uses the real packaged Main/preload/renderer and an independent native Host fixture. That native Host fixture has no Windows build exclusion and its TypeScript wrapper already selects `.exe` and Windows OS metadata. The native installed Main controller is artificially macOS-only and computes `.app/Contents/Resources` paths, a Unix absolute-home regex, arm64 architecture and `macos` metadata for every native launch.

Extend that same fixture to native Windows, with one shared layout/endpoint/home contract, exact EXE/ASAR/bundled-CLI hashes, Windows file URLs, and an owned native process shutdown path. Keep Linux Docker mounting and macOS behavior. Add an explicit candidate-only opt-in workflow path after installer lifecycle; do not run the journey against the historical baseline. A fresh task-owned server/web/DB is required and will be allocated by the runner driver, not by this child. No downloaded image or provider fallback is allowed.

Expected files: `e2e/fixtures/collaboration-installed-main.ts/.mjs`, shared installed-layout helper and Node tests, `e2e/collaboration-host-installed.spec.ts`, Windows collaboration runner script and its fixture contracts, `.github/workflows/desktop-smoke.yml`, `apps/desktop/scripts/WINDOWS_ACCEPTANCE.md`, and this task's evidence. Production UI/backend behavior is outside scope.

## Verification boundary

Only pure Node/TypeScript fixture checks, syntax/listing, compile-only Windows build and script/CI parsing locally. No GUI executable, installer, API, database or Provider is run here. Native Windows installed collaboration, signed trust, real accounts, upgrade state and final source-bound acceptance stay open until the coordinator runs an authorized native environment on the final candidate.

## Implemented preparation

- Shared native layout supports Windows installed `Multica.exe/resources` and macOS `.app/Contents/Resources`, correct native CLI suffix/OS/architecture and escaped file URLs. Relative/network native roots are refused. Existing Linux Docker transport remains its own path.
- Native controller close has a grace wait, PID-owned Windows tree termination or owned POSIX group termination, bounded completion, and no native fallthrough to Docker. Close-command errors are not swallowed.
- Windows journey requires source SHA and actual EXE/ASAR/bundled-CLI hashes; fixture binary hashes are recorded by its driver. The same existing installed Main/preload/renderer/Host journey covers Stop, paused app restart and explicit Resume. No private IPC/auth substitution.
- A separate default-false workflow input runs candidate upgrade/reinstall journeys, preserving the baseline's existing contract. A task-only API/production Web/fake Host preparation and always-run owned cleanup are wired locally. The shared Windows API helper verifies both current_database and its newly created cluster directory before migration and keeps knowledge disabled. Public reports/PNG names exclude private server/Web state and raw Playwright failures.
- Updated Windows operator docs and desktop delivery spec; no dependencies changed.

## Local evidence so far

13 Node path/configuration/lifecycle/early-refusal contracts pass with no fail or skip. The original layout test was red before Windows support. Installed controller strict typecheck passes; Playwright lists exactly one existing journey. Workflow YAML parses using the existing Core-owned YAML tool. Modified desktop scripts lint passes after correcting test-only dependency/regex lint errors; original failed lint logs retained. Windows amd64 daemon fixture cross-compiles with `go test -c` (PE machine0x8664), never executing TestMain. A standalone journey typecheck first used too-old default lib (isWellFormed baseline error); rerun uses the repository's ESNext/DOM library contract and passes for the complete installed journey.

No PowerShell binary exists in this macOS environment, so native parser/runtime/installer/GUI results remain unrun. The workflow contains actual PowerShell parse steps for the new service script. None of the local framework checks close Windows native, installed-account, quota or signed/offline trust gates. Coordinator independent review and final candidate binding remain required.

## Handoff

Product commit: `9de37277d651b4d2d39ffe0cc1d2171286744550`. Machine evidence: `windows-verification.json`, including every command/exit code and source/log/PE hashes. Product17 files; no new dependency or lockfile change. Final contract run13 pass/0fail/0skip, strict transport and whole-journey typechecks pass, selected script lint pass, YAML parse pass, one journey enumerated but not executed, Windows amd64 daemon test-c pass. The cross-built executable stays private and is not a native pass.

Coordinator next: independently review this branch against7240d8542 (especially native process cleanup, PowerShell service ownership and phase gating), then combine after runtime/knowledge/RK1 per the authorized sequence. Final candidate must rerun these local checks and the actual native workflow after external authorization. Native planned entry is Desktop Smoke with `windows_collaboration_acceptance=true`, a candidate version above the chosen installer baseline (e.g.0.6.8-rc.1 versusv0.6.6); optionally combine existing business/update acceptance. No workflow dispatch or installer command was executed here.
