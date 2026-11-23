# Managed argument review follow-up

Owner: `/root/claim_managed_args`; scope recovered from the parent’s visible spawn record because the live NEW_TASK payload is empty. Worktree `mc-kfix-lane-claim`, branch `codex/kfix-lane-claim-20261101`, base `94e790d2d`. The current full service process and its evidence are left untouched. No database, migration, installed Agent CLI or Provider operation is permitted in this slice.

Root cause: `buildClaudeArgs` filters competing `--settings` values but appends the authoritative managed pair after arbitrary default/custom options. A required-value option can consume that flag or make parsing stop at a later operand. Merely joining the pair as `--settings=...` does not establish safety when a preceding required-value option consumes the whole token.

Minimal repair boundary: emit the already-owned managed settings pair before filtered default/custom options, preserving the existing competing-settings filter and all ordinary unrelated options. No new argument parser or option allowlist. The managed option is consumed before a malformed later option can consume it; malformed configuration may still fail as ordinary CLI configuration, and never silently disables the managed settings policy.

Expected files: `server/pkg/agent/claude.go`, canonical `claude_test.go`, knowledge execution spec and operator runtime doc where the ordering is described, plus named red/green evidence. Existing H3 query/migration code and the service test suite are outside ownership. Tests use a local standard-library fake option parser with required-value semantics and the repository Agent CLI guard; no installed CLI is queried. Independent review remains pending.

## Live verification

- New required-value parser regression: red exit 1, seven subcases plus parent failed because the managed setting was absent.
- Fixed construction: targeted suite exit 0, nine named tests/subtests pass, zero failure or skip.
- `go -C server build ./pkg/agent` and `go -C server vet ./pkg/agent`: exit 0.
- Targeted race18/18 passed. First full pkg/agent attempt had9 timing/handshake failures under high parallel load (private fixtures, not Provider). Root reran the same package with `-parallel=2`:1950 named passes,0 failure,4 explicit skips; no per-test timeout changed. No database operations were launched. Final independent review remains pending.
