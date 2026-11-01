# Native fixture origin repair

Owner `/root/check_native_origins`; exclusive write scope is `mc-check-fixture-origins-20261102`, branch `codex/check-fixture-origins-20261102`, baseline `c3db64b24`. No parent/candidate worktree, process or database is touched. Incoming task Payload is empty; the exact bounded assignment is recovered from the inherited parent spawn record. Existing parent Trellis task remains in progress; no task is created.

## Confirmed root cause and boundary

`scripts/check.sh:prepare_check_environment` always writes both HTTP and WebSocket allowlists as only the allocated Web origin. An explicitly configured `COLLABORATION_ELECTRON_RENDERER_URL` or `CHANGELOG_ELECTRON_RENDERER_URL` is copied but not included. `server/internal/realtime/hub.go:checkOrigin` permits absent/same-host origins, a trusted forwarded host, or exact allowlist membership; an HTTP renderer on another loopback port has no exemption. Desktop `webSecurity:false` does not change this server-side decision. No browser/native result is claimed here.

Smallest fix lives only in check-environment construction: append canonical origins derived from these two existing explicit native-fixture URL variables to the task's allocated Web origin before API provenance/configuration IDs are calculated. Accept HTTP(S) localhost/127.0.0.1/[::1] URLs without credentials, reject invalid/nonloopback input without echoing it, and deduplicate. Keep the default Web-only allowlist and source env bytes unchanged. Never reuse arbitrary production source CORS values or wildcard access.

Expected files: `scripts/check.sh`, canonical fake-only `scripts/check.test.sh`, relevant E2E run-environment spec and this evidence/status directory. No production CORS/WebSocket change, new variable/dependency, fixture service, DB, GUI, Agent CLI or Provider operation. Add regression against the real prepare function first, confirm expected native-origin omission, then implement and run only fake script regressions and syntax/static checks. Independent review/local merge remain the coordinator's gates.

## Implementation and early results

The canonical regression ran against the original real prepare function and failed for the actual missing HTTP native origin (exit1), not a setup or missing-helper error. A minimal check-only helper now derives a deduplicated Web-plus-renderer CSV from the two existing explicit fixture variables. It rejects unsupported scheme/host/credentials/zero-port without printing raw input. No production CORS or WebSocket code changed. The first complete shell regression run passed. Final cases add shared-renderer-origin deduplication; check/dev-env/test-go fake script regressions and syntax checks are running.

Integration requirement: the coordinator must set COLLABORATION_ELECTRON_RENDERER_URL (and optional CHANGELOG_ELECTRON_RENDERER_URL) in the trusted task source env BEFORE make check prepares its environment. Supplying those only in the later private pnpm/Playwright wrapper cannot change a server that already started. This fix needs a new source-bound candidate/run after independent review, not a silent change to an active check process.

## Final verification

Final canonical test bytes copied unchanged beside baseline c3db64b24 check/dev-env/local-env scripts fail exactly at the missing native HTTP origin (expected exit1). The first scratch replay lacked the empty server directory required by the existing fake orchestration harness; that setup failure is preserved separately and excluded. Supplying that directory lets final-byte replay reach the real origin omission. Original first red is also retained.

Fixed check.test.sh passes all11 named native cases: independent/shared/deduplicated origins, IPv6 and invalid-input refusal without credential output. Existing dev-env and test-go fake script entrypoints pass. Bash syntax and diff whitespace checks pass; shellcheck is unavailable and was not added. No actual API, database, GUI, installed Agent CLI or Provider ran.

Only check.sh, its canonical fake regression, E2E environment code-spec and this evidence changed. Production HTTP/WS origin checks remain untouched. Source env stays byte-identical; unconfigured behavior remains Web-only; renderer origins are computed before API provenance. The coordinator must place explicit renderer URLs in the trusted task source env before the next make check invocation. Independent review, local integration and final same-candidate browser/WS acceptance remain pending. Exact commands and hashes are in verification.json and red-final-tests-baseline-r2.json.

## Handoff

Product/test/spec/evidence commit `b8035617de06cf164354ff75f122f1e28c990a8b`. Worktree was clean immediately after that commit. Independent coordinator review and local merge are pending; parent candidate/processes/databases were not changed. Before the next check run, put the explicit native renderer variables in the trusted source env; late Playwright-only injection is insufficient.
