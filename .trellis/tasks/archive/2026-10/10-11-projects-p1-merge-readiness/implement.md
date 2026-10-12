# Execution plan

## Approved scope

User-approved remediation and integration from the prior assessment; task
creation separately approved. Planning review found no unresolved user decision.

## Ordered checklist

- [x] Inspect branch relationship, CI configuration and concrete failures.
- [x] Write and review PRD, design and execution boundaries.
- [x] R1 lane: write failing identifier regressions, apply minimal fix, run
  focused Vitest/Go tests and core lint/typecheck.
- [x] R2 lane: write interrupted-publication regression, implement atomic
  exclusive publication, run credential tests and cross-platform compilation.
- [x] R3 lane: compress only 13 over-budget PNG files, check dimensions and
  visual readability, rerun bitmap gate.
- [x] Review integrated corrections independently and update executable specs.
- [x] Reproduce the draft failure with unchanged browser assertions, confirm
  the deferred-create event order, and fix it with deterministic regressions.
- [x] Run local lint/type checks, TS tests, isolated Go tests/vet, production
  build and relevant E2E via project-supported verification commands.
- [x] Record any remaining environment limitations and resolve all actual
  in-scope failures before committing.
- [x] Commit corrections in logical Lore-format commits, refresh remote refs,
  push source branch, create/update PR with full scope and upgrade notes.
- [x] Watch all applicable PR CI; diagnose/fix failures and repeat affected checks.
- [x] Merge PR with commit history preserved after green CI, verify main and
  post-merge checks, synchronize local main safely.
- [x] Record final evidence, archive task and finish with a clean tree.

## Handover notes (2026-10-11)

The Codex session stopped at 20:36 on a 402 quota error after the three lanes
finished; its background `make check` then failed on a views test flake. A
Claude session continued from that state:

- Committed R1/R2/R3 as 10da158b8, 4a4705ebf, 3480ad3b8, plus b129d2d80 for
  the offline-delivery iteration-defaults wording.
- 8ec89c773: `iteration-settings-tab.test.tsx` raced a skipped refetch on a
  disabled query (3/6 isolated passes); now waits for the recovery read (12/12).
- 2fe51e76b: two `cmd/server` route tests still assumed a missing settings row
  means disabled (stale since 86d8c2b41); they now seed an explicit opt-out.
- `docs/offline-upgrade.zh-CN.md:450` still says new workspaces default off.
  That file has uncommitted v0.6.5 release edits in the original checkout, so
  it is left to that work rather than edited here.
- `make check` at 2fe51e76b: static, 6 TS packages, 70 Go packages (race) and
  vet green; Playwright 247 passed / 11 failed / 70 skipped. All 11 failures
  trace to three same-day product commits that skipped live E2E: 86d8c2b41
  (triage default on: 2 specs), db5c4b28e (`aria-label="Issue actions"` breaks a
  `:not([aria-label])` locator: 4 specs), 7724f7ca6 (progress details collapsed
  into `<details>`, restyled risk cards: 5 P1 specs). Spec-only repair in
  progress; product code unchanged.
- `TestIterationWriterServerLockWait` (internal/handler, on main since
  bc5a2929b) failed once under the full `-race` suite (sampler saw 1 sample in
  62 ms) and passed 10/10 plain and 15/15 `-race` in isolation: load flake.
- c3c1e4883 + 3ffe835ac: 11 make-check specs plus 16/19 gated I1 cases were
  stale for the same reasons (plus 2bcba583e, 49d822f92, f79662a56).
  aa6c61ef5: real zh-Hans bug, `pages.scopeChanges` still 范围变化 after
  2bcba583e moved the sibling labels to 范围变更; ISB Web/Electron now green.
- Independent review of the five original commits: no blockers. Should-fix
  (1) renameat2 needs host kernel/filesystem support, documented in 9e24e12f1
  and the PR; (2) `docs/offline-upgrade.zh-CN.md` still says new workspaces
  default off, owned by the uncommitted v0.6.5 release edits in the original
  checkout. Nits: `parseQueryNumber` ignores the workspace's own prefix, so
  `2026-10` also matches issue 10 (additive, same as `covid-19` before).
- `make check` at aa6c61ef5: stages 1–5 green; Playwright 256 passed /
  2 failed / 70 skipped (P1-L14, P1-P05: draft not persisted right after
  fill, both green in the previous full run). Flake rate being measured.

## Verification commands

- `pnpm --filter @multica/core exec vitest run iterations/candidates.test.ts`
- `go test ./internal/handler -run TestParseQueryNumber` (from server)
- `go test ./cmd/server -run TestPlatformAdminCredential -count=1` (from server)
- Cross-compile narrow credential tests for Linux/Windows as applicable.
- `node scripts/check-image-budget.mjs --base origin/main`
- `PATH=/opt/homebrew/opt/libpq/bin:$PATH make check` (isolated environment;
  does not target existing services; use scoped follow-up on failure)
- Mobile validation where required by changed core paths, plus current GitHub CI.

## Lane ownership

- Identifier implementer: core reference parsing/tests and backend issue parser/tests.
- Credential implementer: server/cmd/server credential publication files/tests.
- Image implementer: the 13 over-budget PNG assets, no source-code changes.
- Main: task artifacts, specs, integration review, verification, commits, PR and merge.

All agents preserve one another's edits. No source branch checkout, commit or
push occurs while implementers are writing. Final reviewer owns only narrow
mechanical corrections after implementation lanes finish.
