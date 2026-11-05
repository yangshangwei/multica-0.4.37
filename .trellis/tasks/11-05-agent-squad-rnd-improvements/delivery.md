# First P1 delivery status

## Plan and implementation

The user approved task creation and first P1 implementation. Work is isolated on `codex/agent-rnd-p1-20261105`, worktree `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`, baseline `f1d35b66af1013b6147c0e167500cacae203b7a8`.

| Child | Implemented | Evidence | Review |
| --- | --- | --- | --- |
| agent-log-redaction | Credential fields, JSON values, native tokens, redaction before preview | 77 redact (also race),37 daemon,16 provider named passes | Independent check fixed existing capability boundary regression; final redact/race passed |
| agent-ci-test-integrity | Explicit release PG/Redis/migration, wrapper guard, fail-closed3 TestMain | Wrapper PASS,workflow5/5,offline binary matrix12 scenarios + parent13/13,Go vet | Independent check: no blocking findings |
| squad-handoff-contract | Correct delivery/acceptance guidance, leader6-role coverage,v3 and docs | execenv22/service61 named passes,docs embed/generation,key parity,vet | Independent check fixed eligibility wording; final docs check PASS |

Raw failed attempts are retained. Named counts include subtests and are scoped checks; they are not a full repository pass count. Original shared checkout product files were not edited.

## Verification boundary

No real DB/service/model/provider started, no real credential read, no paid model, no production changes or release executed. Database rejection matrix uses real compiled test binaries with synthetic inputs and nonexistent private sockets. Successful DB fixtures, seed fault injection, hosted Go1.26 CI/Release, browser E2E and real-model behavior remain integration follow-up. Existing instances preserve saved instructions; v3 defaults do not silently upgrade them.

## Next backlog

M1: task/resource/action capability, controlled high-risk executor, project evidence gates, effective delegation receipts and PR smoke.
M2: deterministic reporting snapshot, shared budget accounting and capability-aware scheduling.
M3: continuous NFR/recovery evidence, model behavior evaluation, supply-chain evidence and controlled template/knowledge upgrade.

See `design.md`, `implement.md`, and `docs/plans/2026-11-05-agent-squad-rnd-improvements.md` for dependencies, negative cases and rollout/rollback. Parent remains active until future milestones complete.

Independent review additionally exercised 1,000 valid JSON samples and six boundary samples. The first-party capability hyphen-boundary regression was caught and fixed before commit; original failing probe retained.

## Work commits

- `1cd9c1e60c744b8c82b56297bfe06c4ab23a8cc8` — fix(security): prevent credentials from surviving structured execution logs
- `5975edae9f42d92c30deaae734f11e6f43cc5dbf` — fix(ci): require working database setup before accepting test results
- `028e93bb04035960784a339e6789395adba92eeb` — fix(squads): align delivery guidance with acceptance and review requirements
