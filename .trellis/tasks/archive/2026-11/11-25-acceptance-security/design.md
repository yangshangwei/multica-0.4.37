# Boundary and evidence design

Smallest gap: known vulnerable versions remain in server/go.mod; skill tree fetch errors discard diagnostic headers and suggest rate limiting without proof. Behavior lives in the Go module/build toolchain and server/internal/handler/skill.go GitHub response handling.

## Owned writes
- server/go.mod and go.sum: patch existing dependencies/minimum supported Go as official evidence warrants.
- Supported Go CI/release configuration only if current latest-patch selection is insufficient (currently all use 1.26.x; retain if already correct).
- server/internal/handler/skill.go plus focused skill error diagnostic tests: bounded safe classification, retaining failure and existing fallback semantics.
- This child task and new evidence/private security directories.

No runtime, authority, locking, fixture navigation, production credentials or other agents' files. No broad refactor. No API response schema change. New errors retain HTTP status; header values are validated/allowlisted, bodies never included. Preserve errors.Is classification where present.

## Original evidence → hypotheses → discriminating tests
1. D03 private backend/logs/whole-module-vulnerability.log (exit 3, 14 IDs). H1 stdlib caused by local 1.27.0; H2 genuine third-party vulnerable versions remain. Run unchanged module with exact Go1.26.11 then minimum patched dependency graph; compare module IDs, symbols and platforms.
2. GO-2026-6604 is Windows-qualified; cross-target source/build scans cover release darwin/linux amd64+arm64 and windows amd64+arm64+386. Source scanning is not a claim of execution on foreign platforms. Record unsupported/build errors without masking.
3. backend-diagnostics/skills-public-01/test.jsonl reports HTTP403 without headers. H1 primary rate limit; H2 secondary limit; H3 auth/permission/proxy denial. One anonymous request to the same GitHub API tree records only status and safe header fields. Old failure remains unknown if no old headers; current result is separately identified.
4. Mocked response matrix for quota exhaustion, Retry-After throttling, bare403,401,429,404,5xx and malicious header values verifies diagnostics and no disclosure without changing retries or fallback behavior.

Official sources: https://go.dev/doc/devel/release ; https://go.dev/dl/?mode=json ; https://pkg.go.dev/vuln/GO-2026-6278 ; https://pkg.go.dev/vuln/GO-2026-6859 ; https://pkg.go.dev/vuln/GO-2026-6617 . Preserve downloaded JSON and hash it.

Use task-local toolchain/cache/HOME/TMP. No server or DB needed for scanning; focused handler tests follow TestMain DB contract and use parent-supplied isolated PG if required.

## Baseline refinement (before product approval)
A frozen-source scan under checksum-verified Go1.26.11 retains only GO-2026-6278 at symbol reachability (exit3); seven additional module findings remain informational until dependency updates. The original 13 stdlib findings disappear with the supported patched toolchain. Current anonymous tree GET returns200 / remaining35 of60; it does not explain the historical403. No personal token was used. x/net0.60.0 requires updates to existing graph modules x/crypto0.57.0, x/sys0.48.0, x/term0.46.0 and x/text0.42.0; inspect MVS/tidy result, preserve no-new-module rule.

Every acceptance run consumes a fresh frozen server/scripts source copy, captures file hashes and checks for mutation after execution; mutable checkout results are diagnostic only. Baseline copy has no product edits.

Current verbose baseline additionally finds GO-2026-6866 and GO-2026-6860 in required x/net; both require v0.62.0. Therefore v0.60.0 alone is insufficient. Proposed x/net minimum is v0.62.0, subject to MVS graph review; earlier v0.60.0 research is retained as historical evidence.
