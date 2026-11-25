# Security lane implementation and verification

## Result
D03 is remediated for the frozen security candidate: seven release-target source scans and eight actual-binary scans exit0 with no vulnerabilities. D10 remains an external integration FAIL: the real anonymous import received403 with confirmed primary quota exhaustion in this new execution. The historical403 cause remains unknown. This is lane evidence, not integrated release acceptance.

## Files changed and simplification
- `server/go.mod`, `server/go.sum`: require Go1.26.11; upgrade gorilla/websocket1.5.3 to1.5.4-0.20240701034025-d67f41855da4 and x/net0.57.0 to0.62.0. Existing transitive modules move only as required by minimum version selection. No new or removed module identity (module-graph-diff.json).
- `server/internal/handler/skill.go`: classify failed GitHub tree requests by status and rate-limit headers; remove unsupported blanket token/rate-limit advice. Do not expose body/cookie/arbitrary header content. Existing errImportSourceUnavailable, fail-closed behavior, ref fallback, auth forwarding, budgets and request counts remain unchanged.
- `server/internal/handler/skill_github_error_test.go`:11 response cases and one full failed-import regression, plus existing blocked/ref/token tests used for regression.
- Child planning, manifests, research and report; new public evidence folder only. No CI/release configuration edits: both already choose latest1.26.x. No commits/push/deployment.

## Original evidence and discrimination
Original private scan SHA256 `20f757c80d3f828b5aa3b1b992df2be990800ef326895371f4909743243192cf` is preserved and rehashed (original-evidence.json). It exited3 under local Go1.27.0 with14 reachable findings. The supported Go1.26.11 baseline exited3 with one reachable finding, GO-2026-6278, and seven module-only x/net findings. Thus the13 stdlib reports disappear under the patched supported toolchain, while the dependency issue remains real. GO-2026-6604 is Windows-specific per official OSV metadata; no exploitation is claimed.

Official JSON records are retained for all14 original IDs plus GO-2026-6866/6860. Current official records require x/net0.62.0, rather than only the0.60.0 HTTP/2 fix. Sources: https://go.dev/doc/devel/release ; https://go.dev/dl/?mode=json ; https://pkg.go.dev/vuln/GO-2026-6278 ; https://pkg.go.dev/vuln/GO-2026-6866 ; https://pkg.go.dev/vuln/GO-2026-6860 .

MVS edges retained in repaired-graph/stdout.log: x/net0.62.0 selects x/crypto0.58.0, x/sys0.49.0, x/term0.47.0, x/text0.43.0. x/text0.43.0 selects x/tools0.51.0, x/mod0.41.0, x/sync0.24.0; x/tools selects the newer telemetry version. These are existing graph modules, not new dependencies.

## Toolchain and frozen source
Stable supported Go path: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37/.omx/private/acceptance-fixes/security/tools/go/bin/go`.
Go executable SHA256:`20e07ffb4f08faaf125e413db024b6e988d3ec2799dcb3568a720d14be0ad133`; official archive checksum verified. Native scanner path:`/private/tmp/acceptance-fixes-4b9d3494a2/security-govulncheck`, v1.6.0 compiled by Go1.26.11. scanner-provenance.json binds binary SHA256 and full go version -m, including resolved scanner dependencies. Installed global Go1.27.0 was neither replaced nor reconfigured; a minimum1.26.11 directive alone does not make an ambient1.27.0 compiler patched.

Baseline/red runs inside the new private workspace are retained as preliminary diagnostics. Final source is frozen outside Git at `/private/tmp/acceptance-fixes-4b9d3494a2/security-source-green02`; source hashes/modes and exact owned-file hashes match final worktree files. Every final command has its own external HOME/TMP and a cleared explicit environment, without personal tokens/config or API pool/publisher overrides. green01 normal/race regressions precede only deletion of two explanatory comments; green02 contains identical executable code and is the source used by final scans/build/vet/live import. Parent owns final integrated full-handler normal/race against its final source.

## Commands and results
Exact argv, cwd, times, source changes, tool hash, environment hash and output hashes are in each named `receipt.json`; no exit status was discarded. Per-test started/terminal inventories are retained and complete.

| Run | Command core | Exit / result |
|---|---|---|
| baseline-vuln | Go1.26.11 tool govulncheck ./... |3, one reachable dependency finding |
| baseline-vuln-details | tool govulncheck -show=verbose ./... |3, seven additional module-only findings |
| diagnostics-red | guarded go test -json -count=1, two new top-level tests |1,13 named FAIL; proves former diagnostics insufficient |
| diagnostics-green | guarded go test -json -count=1, five focused top-level tests |0,16 named PASS,0 SKIP |
| diagnostics-race | same exact five tests with -race |0,16 named PASS,0 SKIP |
| module-verify | go mod verify |0 |
| native2-build | go build ./... |0 |
| native2-vet | go vet ./... |0 |
| api-binary-build | go build -trimpath -o owned-api ./cmd/server |0 |
| scan2-<target> | native govulncheck -show=verbose ./..., GOOS/GOARCH target, CGO_ENABLED=0 |all seven exit0, no vulnerabilities |
| build2-<target> | go build -trimpath -o owned-binary ./cmd/multica, target env |all seven exit0 |
| binary-scan-<binary> | native govulncheck -mode=binary actual-binary |all eight exit0 |
| public-import | guarded go test -json -count=1 -run ^TestFetchFromSkillsSh_AnthropicPptxIntegration$, MULTICA_RUN_SKILLS_SH_INTEGRATION=1 |1, one named FAIL;403 primary rate limit |
| github-go-headers | isolated Go HTTP GET,30s existing client timeout, no authentication |0 transport;HTTP403 and remaining0/limit60 |
| format-check | git diff --check and gofmt -l owned Go files |0; no unformatted files |

Release matrix: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64, windows/386. Cross builds/scans prove static target results, not runtime platform acceptance. binary-provenance.json records full build info, actual paths, sizes and SHA256 for all eight candidate binaries.

## Retained failed diagnostics
- Module-only scanner invocation with a package pattern exited2; documented tool rejects that syntax. Verbose source scan provided the required module detail.
- First Linux/Windows `go tool govulncheck` attempts exited1 because target GOOS cross-built the scanner and macOS could not execute it. All five original failures remain in scan-<target>. Corrected matrix2 uses a native scanner binary with target environment for analysis. This was a harness correction, not a skipped target or product workaround.
- Initial standalone Python GitHub request returned200/remaining35. Later real Go importer returned403/primary quota exhausted; separate matching Go transport headers record remaining0. Those observations are time/transport-specific and neither explains the historical missing-header403.

## Resources and remaining work
Parent owns `fix_security_handler01`; verified migration exit0 and current_database. Final audit sees0 other database sessions and0 handler fixture workspaces. No API/daemon/container started by this lane; no accounts or tokens used. Owned external source, HOME/TMP and binaries are retained under `/private/tmp/acceptance-fixes-4b9d3494a2/security*`. Task-local compiler/scanner provenance remains available to parent. Owned generated cache was moved to `/Volumes/artisan/.multica-qa/acceptance-fixes-4b9d3494a2/security/cache` with its original private path retained as a symlink (cache-relocation.json). No unowned cache or old evidence was removed.

D10 stays FAIL pending normal anonymous public-service availability; no retry, personal token, deadline increase or forced success was introduced. Parent's comment lane owns full handler normal/race and root owns full integration. Suggested spec follow-up: distinguish status403 from rate-limit proof; use host-built govulncheck for cross-target scanning. No spec files outside this lane were edited.
