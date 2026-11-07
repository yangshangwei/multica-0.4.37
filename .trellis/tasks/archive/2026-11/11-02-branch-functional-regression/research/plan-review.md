# Independent planning and inventory review

Verdict: **PASS WITH NOTES**. No blocking finding in the current functional-test plan. This verdict reviews planning artifacts only; it does not start testing, approve the old private launcher, clear adapter-v5/RKI6 evidence gates, or establish any product pass.

## Scope and evidence inspected

- Candidate: `7af9ab9905c98c77bb091223ebe687c639475bb4` on `codex/cross-workspace-collaboration`.
- Read `CLAUDE.md`, task PRD, `test-plan.md`, `design.md`, `implement.md`, research notes, baseline and collection records, coverage CSV, and context manifests.
- Cross-checked repository package scripts, Playwright config, referenced spec files, relevant script regressions, database isolation spec, and actual API pool configuration keys.
- No tests, database connections, services, Provider commands, downloads, or task-state changes were performed. Only this review file was written.

## Verified findings

1. `coverage.csv` has **439 unique IDs and 439 unique collected test identities across 148 spec files**. Every project/spec/line/title tuple exactly matches `research/playwright-inventory.txt`; every spec exists. All rows correctly remain `NOT_RUN`.
2. Both JSONL manifests contain seven real entries; all referenced paths exist.
3. The plan covers authentication, workspace/member authorization, ordinary task lifecycle, projects/iterations, triage, collaboration workflow, Agent execution, memory/handoff/recall, knowledge, realtime/notifications, chat/files, capabilities/MCP, administration/automation, and platform behavior. Recent Transfer, nested gate restoration, immutable startup memory, actual knowledge input, time-bound authorization, native origin/focus and PG capacity risks are explicit.
4. All literal spec names and wildcard groups in the functional matrix resolve to at least one current file. Static/unit/script commands refer to existing scripts. Mobile's existing Vitest and shell steps are split correctly instead of forwarding Vitest options through the chained script.
5. The design requires fresh owned databases per DB-backed Go package, separate browser lanes, shared-cluster capacity accounting, owned pool caps, production Web, actual Electron, artifact/configuration identity, first-failure retention and skip reconciliation. The stock shared-Go-DB `make check` is accurately distinguished from campaign acceptance.
6. Historical `822c15785` and `639bf6ef8` outcomes are explicitly historical. The plan preserves the old adapter-v5/RKI6 independent-review block and does not create or authorize a clearance file. Missing native platforms/installation/real accounts remain explicit limitations; fake Provider results cannot be presented as paid vendor acceptance.

## Nonblocking execution notes

- **N1 — Include the Web scope-business entry explicitly.** `test-plan.md:34` lists `iteration-scope-business-*.spec.ts`, which matches the Desktop file but not `iteration-scope-business.spec.ts`. The complete 439-case inventory already includes the Web spec, so no case is lost. Add the exact Web filename to that domain row or use `iteration-scope-business*.spec.ts` when updating the plan.
- **N2 — Run source-mutating script regressions in the owned checkout, serially.** `scripts/check-ui-wildcard-exports.test.sh` writes temporary probe source files under `packages/ui` and `packages/views`. Its existing cleanup is appropriate for its regression contract, but it should not overlap static/type/build/source-fingerprint verification in the same tree. Prefer the detached task checkout already described in B0, and verify a clean product tree after the script completes.
- **N3 — Make the Agent CLI guard concrete in browser execution commands.** The plan states that the guard stays active, while `implement.md:63` shows the unwrapped Playwright command. The actual runner should invoke it through `bash scripts/go-test-with-agent-cli-guard.sh -- pnpm exec playwright test ...` whenever fixtures can discover Agent executables, and pin fake/missing DSH paths where those fixtures require them. Retain the existing original case assertions.

These notes do not change product scope or acceptance behavior. Apply them when preparing execution permits. User review of the concrete plan is still required by the current planning state.
