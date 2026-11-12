# CCG2 deployed probe independent review

Verdict: **pass-with-notes**. No blocking finding for test code `5bd1ecf13` with E6 product `e6e7fe07d`; actual OFF and ON both have one passed, zero failed/skipped/retried test and one physical fake launch. This closes the deployed-probe scope, not final integrated candidate acceptance.

## Source and parser boundary

The four E2E/fixture file hashes independently match `5bd1ecf13`. No product/dependency/timeout change is introduced. The parser decodes the first frozen assignment through its actual decoder offset, then allows only whitespace or the exact managed-knowledge marker and one validated array ending at EOF. It retains admitted issue/input and CLI/credential checks. Actual content is hashed before safe contribution identity is reported. The optional observation preserves the separate memory fixture.

Original raw red evidence confirms 1 run / 1 fail. Recorded normal and race logs each independently confirm 24 run / 24 pass / 0 fail / 0 skip, with exact raw hashes. Recorded strict TypeScript, fixture-scoped base ESLint, Go file-list vet and one-test collection log hashes also match; collection alone is not execution.

## Actual deployed evidence

The reviewer independently read both raw execution/results records, decoded and hashed their evidence attachments, and checked the actual named test outcomes. Root's safe public aggregate matches the originals. OFF completed before ON began; both commands use the repository Agent CLI guard, one worker and zero retries.

Both services bind E6 with the same product source fingerprint, separate database/deployment/JWT/Redis/catalog configuration, distinct API/Web configuration fingerprints and production Web build IDs. Private source-env hashes and literal knowledge flags match the recorded execution; no private values were printed or copied. The only generated source difference in service checkouts is the allowed Next route-types import. Actual daemon/provider/CLI bytes match build and attachment hashes; renderer identity is recorded by the guarded root runner.

| Observation | OFF | ON |
| --- | --- | --- |
| Actual test / physical fake launch | 1 pass / 1 launch | 1 pass / 1 launch |
| Actual input knowledge section | Absent | Exact approved candidate/version/source/content hash |
| Actual no-session-persistence flag | False | True |
| Claim / preparation / authorization events | 0 / 0 / 0 | 1 / 1 / 1 |
| Settlement / proof | Success / host_exit_observed | Success / host_exit_observed |
| Ledger | Sealed; authorized, observed and reaped | Sealed; authorized, observed and reaped |

Each attached lifecycle summary has one claimed run, zero remaining active tasks and successful permit/termination/proof calls for that exact run. Each daemon exited zero with no stderr/errors. Reviewer JSON records raw result/log/attachment hashes, actual prompt hashes and canonical allowlisted-ledger/lifecycle-summary hashes. Those ledger hashes identify retained safe summaries, not credential-bearing original ledger files.

## Notes and boundaries

DP-1: This evidence binds E6 product plus the exact reviewed harness and task environments. Full harness diffs during execution contain ongoing root/reviewer documentation; all four code hashes remained stable. Final integrated candidate source still needs its own binding and actual required checks. The earlier failed complete check stays failed.

DP-2: The journey proves deployment flag → real API → test-owned actual Host stdin. Public exact human approval is used, while human approval UI, CCG1, real-model interpretation, installers and different-account acceptance remain separate. No paid Provider or external operation occurred.

Root owns service/DB execution and integration. The reviewer ran only read-only source/JSON/hash/git inspections and wrote this directory; no tests, DB commands, browsers, services, containers, installed CLI, Provider calls, children or git mutations were performed by the reviewer.
