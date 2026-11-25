# Research: Independent security lane acceptance review

- Query: Review the security report, actual dependency/skill changes, official remediation choices and all seven source/eight binary scan outcomes; distinguish fresh public-import evidence from the historical failure.
- Scope: mixed; local source/evidence audit and current official advisory verification. Read-only final-check preparation.
- Date: 2026-11-25
- Parent task: `11-25-main-post-merge-acceptance-fixes`.
- Worktree: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`.

## Findings

**ACCEPT the security implementation and its D03 scan remediation for the frozen security candidate. No new actionable code or evidence finding. D10 remains FAIL for the external public import. This does not grant integrated product/release acceptance.**

The report's distinction is supported: patched supported-toolchain baseline retains one reachable WebSocket finding; the selected dependency upgrades clear the reviewed scans. The newly observed public403 has primary quota evidence, while the historical403 remains unclassified. The report preserves the failed scanner invocations, initial public200 observation and later import403 rather than replacing them with a single success.

### Independently checked evidence

| Check | Result |
| --- | --- |
| Evidence manifest | All 201 listed files rehashed; zero mismatches |
| Raw command receipts | All 50 stdout/stderr pairs match their receipt hashes |
| Original evidence | All four original referenced files still match their recorded hashes, including the original failing vulnerability log |
| Frozen candidate | All 5,368 files match their recorded bytes and modes; no extra files found |
| Owned source | `go.mod`, `go.sum`, `skill.go`, new diagnostic tests match both final source record and current worktree |
| Toolchain | Download archive size/checksum match official metadata; extracted Go executable matches the archive member and recorded hash |
| Scanner | Binary hash and independently read embedded build metadata match the retained provenance; native darwin/arm64 govulncheck1.6.0, built by Go1.26.11 |
| Source scans | Seven complete logs: each scans86 root packages, exit0, empty stderr, no advisory findings, final “No vulnerabilities found.” |
| Target bindings | Reconstructed all seven GOOS/GOARCH/CGO=0 environments; each digest matches its receipt |
| Binary scans | Eight actual retained executables: exit0 and no findings in every raw scan log; all binary size/hash and independently read build metadata match |
| Dependency inventory | 123 module identities before and after; no additions/removals; ten existing versions updated, matching the reported graph delta |
| Diagnostic test baseline | Raw Go JSON:13 started/13 failed named tests, complete; exit1 |
| Focused normal/race | Each raw Go JSON:16 started/16 passed, no skips, complete; each exit0 |
| Public import | Raw Go JSON:one started/one failed named test; exit1; real importer reports primary quota exhaustion |

The seven source targets exactly match `.goreleaser.yml:17–30`: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64, windows/386. The eight binaries are the seven target CLI executables plus the native API executable. All embed Go1.26.11 and the patched WebSocket/x/net versions. These are candidate cross-built executables, not proof of native foreign-platform behavior or published release artifacts.

The final scans/build/vet/public import use `security-source-green02`. Focused normal/race use green01. Independent file comparison found exactly the reported deletion of two explanatory comments in `skill.go`; executable code is unchanged. The final integrated handler/module regressions still belong to the parent's stable candidate, as the lane report already states.

### Official remediation choices

- `github.com/gorilla/websocket` **v1.5.4-0.20240701034025-d67f41855da4** is the fixed boundary for the affected v1.5.3 interval in the current [GO-2026-6278 advisory](https://pkg.go.dev/vuln/GO-2026-6278). The unusual pseudo-version is supported by the advisory, not an arbitrary upgrade.
- `golang.org/x/net` **v0.62.0** is justified by both [GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866) and [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860). The earlier v0.60.0 proposal would not cover these records. Their baseline presence is module-level evidence; the report does not claim an exploited production path.
- **Go1.26.11** is a published patch on the project's selected1.26 line and includes security fixes. [Official release history](https://go.dev/doc/devel/release), [download metadata](https://go.dev/dl/?mode=json), and [GO-2026-6859](https://vuln.go.dev/ID/GO-2026-6859.json) support the choice. The retained archive metadata exactly matches the live official metadata.
- [GO-2026-6604](https://vuln.go.dev/ID/GO-2026-6604.json) is Windows-qualified. The report correctly avoids calling it a local macOS exploitation result.
- Independently fetched five relevant OSV records and compared parsed contents with the retained copies: all match. Other retained original advisories were integrity-checked through the manifest rather than individually refetched.
- Recorded MVS edges explain the updates to existing crypto/sys/term/text/tools/mod/sync/telemetry versions. The `go.mod`/`go.sum` diff adds no module identity, no replacement directive and no new tool. CI/release already select1.26.x. The report correctly warns that the raised minimum directive does not patch an ambient Go1.27.0 installation.

### Skill error handling and the public403 boundary

`server/internal/handler/skill.go:1409` now routes non200 tree responses through the fixed classification helper at `:1869`. It retains HTTP status, never incorporates raw header values/body/cookies into the new message, leaves a bare403 unconfirmed, and preserves `errImportSourceUnavailable` at `:1275`. Auth forwarding, ref-resolution fallback, retries and import failure behavior are unchanged. The new tests cover11 status/header cases and one complete failed import; existing auth forwarding, fallback and tree-failure regressions also ran.

The real `public-import/stdout.log` reports403 with the new primary-quota label. In the reviewed implementation that label requires the actual response's `X-RateLimit-Remaining` to equal0. A **separate later** anonymous Go header probe at `2026-11-25T10:11:26.987361Z` also records403, remaining0, limit60. GitHub documents403/429 plus remaining0 for primary quota exhaustion. [GitHub rate-limit documentation](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)

The later probe is corroboration, not the exact import response and not the old response. The original missing-header403 remains UNKNOWN in cause; the new integration remains FAIL in result. No token substitution or successful-import claim is warranted. The report observes these boundaries.

## Files found

- `.trellis/tasks/11-25-acceptance-security/report.md` — scope-limited lane conclusion, regression table and retained failures.
- `server/{go.mod,go.sum}` — patched supported toolchain floor and existing-module upgrades.
- `server/internal/handler/{skill.go,skill_github_error_test.go}` — response classification and focused regressions.
- `artifacts/qa/main-acceptance-fixes-20261125/security/` — manifest, official JSON, raw source/binary logs, receipts, inventories and provenance.
- `.omx/private/acceptance-fixes/security/{run-final.py,matrix2.py,tools/}` — inspected launcher/scanner/toolchain inputs; no launcher was executed by this reviewer.
- `/private/tmp/acceptance-fixes-4b9d3494a2/security-source-green02` — independently rehashed frozen candidate.
- `security-review-audit.json` in this research directory — detailed independent hashes, source modes, raw-result reconciliation, target environment reconstruction, binary metadata checks and module delta.
- `security-official-review.json` in this research directory — independently fetched official advisory/download comparisons and source URLs.

## Related specs and previous findings

- `CLAUDE.md`: dependency ownership, test ownership, truthful verification and default Agent CLI protection.
- `.trellis/spec/server/database-test-isolation.md` and `verification-environment.md`: owned databases, process environments and immutable evidence identity.
- `.trellis/spec/server/agent-output-redaction.md`: diagnostic disclosure boundaries.
- `runtime-client-early-review.md`: no new security/skill finding here; C1/R1/R2 belong to other lanes and require closure verification in their own final review. This security review does not silently close them.

## Caveats / Not Found

- No source edits, scanner reruns, tests, services, SQL/Docker commands or Git operations were performed. Reviewer execution was limited to read-only parsing/hashing, official HTTP reads and `go version -m` inspection of retained executables. Only research files were written.
- Resource receipt reports zero other database sessions and zero handler fixture workspaces. Its hash is verified; live database/container ownership and final integrated cleanup were not re-probed here. The parent still owns those final checks. External source/build/cache retention is explicitly reported rather than called deletion.
- A clean known-vulnerability scan is not proof of absence of application authorization defects, unknown vulnerabilities, or runtime platform defects. D03 acceptance applies to the exact reviewed candidate/configurations; final integrated artifacts need their own required checks.
- D10 remains an external FAIL. Full integrated normal/race and remaining product journeys are pending outside this lane.
