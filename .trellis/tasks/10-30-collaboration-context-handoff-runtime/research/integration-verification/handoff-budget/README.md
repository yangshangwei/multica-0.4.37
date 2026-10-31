# Daemon handoff budget verification

- Worktree: `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`
- Source HEAD: `e3620721efacb820b89709c8db9d326d9cf2b00b` with the integration worktree dirty; see before/after status and tracked server diff files.
- Changed source: `server/internal/daemon/collaborationrun/host_handoff_budget_test.go` only. No production edits, commits, task-state edits, DB access, installed agent CLI lookup, or real-provider access.
- Platform: macOS; local HTTP fixtures and the test-created Go executable used by `newLaunchHandoffFixture`.
- Source SHA-256: `3740d24bf0956f4ab23fb2a0218f051cf7627e2f2716a9103728cf282aa6cbea`. `before-source.json` and `after-source.json` cover all tracked/untracked server Go files and module inputs. No server source hash changed during verification.

## Results

All commands in `verification.json` exited 0. Exact argv, UTC timestamps, working directory, and status are recorded there. `focused-tests.log` contains the actual test names and subtests; there were no skips.

- `TestHostHandoffBudgetEscapedAssignmentReachesProviderIntact`: four 65,536-byte assignments (`<`, `&`, U+0001, and mixed escapes/Chinese/emoji) produced wire sizes 396,232 / 396,232 / 396,232 / 275,617 bytes. Each remained within the material budget, reached actual fake-provider stdin with the complete source unchanged, and matched the prompt hash and real attempt binding.
- `TestHostHandoffBudgetEnforcesRawHTTPBoundaryBeforeLaunch`: valid JSON plus framing whitespace and final newline was accepted at exactly 2,097,152 bytes and refused at 2,097,153 bytes. The over-limit response caused no binding or provider start and retained original unlaunched proof cleanup. Padding isolates the HTTP limit from decoded material size.
- `TestHostHandoffBudgetRejectsInvalidMaterialBeforeLaunch`: source text totaling the entire material budget plus metadata, excessive metadata, and a malformed source hash were all rejected before binding or process start, without snapshot retries. Material/wire sizes logged in these cases describe the fixture envelope before its request-specific snapshot ID is added.
- Existing `TestHostHandoffLaunchKeepsMaximumAdmittedInputsBeyondMetadataPage` passed unchanged; the maximum-count matrix was not duplicated.
- Package `go vet` and `go build` passed; `gofmt -l` returned no files.

## Scope and remaining work

No production defect was found in the verified boundaries. The shared material/wire correction was supplied by the server owner before this lane ran. This evidence verifies daemon delivery and rejection, not real-model comprehension, server authorization/database behavior, or full repository acceptance.

Recommended spec sync for the parent owner: `.trellis/spec/server/collaboration-startup-handoff.md` should state the 256 KiB material budget (JSON metadata/framing plus decoded UTF-8 source content), separate 2 MiB HTTP wire cap including newline, and this test file under required tests. The spec file is outside this lane's write scope.
