# Memory runtime gate closure independent review

Verdict: **pass-with-notes** for code `5dec88f98`, with no blocking finding in the bounded nested-close repair. Product source remains `639bf6ef8`; this does not close whole-candidate acceptance or reclassify the third complete check.

The code changes only the canonical memory-runtime spec. Before a new PG client, `hostEnvironment()` validates explicit loopback services, non-default task database and explicit fake binaries. The initial read verifies actual database identity before either seed. Final verification compares the original enabled/creation/Host booleans exactly and records, rather than rolls back, the legitimately advancing epoch.

Closing the nested second fixture before the first matches each fixture's captured gate lease. Nested finally attempts first cleanup even if second close rejects, and attempts browser context cleanup even if either close rejects. Gate evidence is attached before the final flag assertion. The original business body is byte-identical except deletion of the never-invoked `read` closure; memory v1/v2/v3, authors, inputs, private workdirs/profiles, actual run-local CLI, authority and settlement checks remain. Product guards, defaults, dependencies, timeouts and retries are unchanged.

| Actual evidence | RED old order | GREEN LIFO | GREEN then Leave |
| --- | --- | --- | --- |
| Named result | 1 fail, 0 skip/retry | 1 pass, 0 skip/retry | 1 pass, 0 skip/retry |
| Actual gate flags | T/T/F → T/T/T | T/T/F → T/T/F | Same GREEN task DB/API/config |
| Epoch | 3 → 6 | 3 → 5 | No rollback/reset introduced |
| Memory / cleanup | Reached v3; both daemons exited0 | Reached v3; both daemons exited0 | Protocol Leave succeeds |

The reviewer independently checked original execution/results/log hashes and actual attachment hashes, named statuses, single execution with retry0, both distinct runtime/actor identities and v1→v2→v3 metadata. Both normal daemon closures retain proof-acknowledged ledgers with zero errors/stderr. The RED failure is the new final Host-flag assertion after the original business journey completed; RED remains retained and failed.

GREEN memory finished before Leave started. Both runs have the same DB, API PID/start time, source-env hash and production Web build ID; the guarded runner contains only read-only database identity SQL and no gate reset. Source, diff, preserved RED source, all indexed evidence and static-check logs match their recorded hashes. Actual controlled binary bytes match the run records. RED/GREEN deployment credentials, databases, Redis/catalogs are isolated, with actual task pool bounds max5/min1. No private env values or memory bodies were copied.

MG-1: This is fixture closure proof bound to product639/test5dec. Integration, rebinding and final checks remain parent-owned. It is separate from the legacy browser PG-capacity incident and does not relabel the old final3 Leave409.

MG-2: Exception coverage for daemon close is source control-flow proof; no injected cleanup-failure test was run. The unchanged early `browser.newContext()` allocation remains outside the cleanup try, so a rejection there is outside this bounded nested-close proof and remains a follow-up concern.

The reviewer only read source/git/JSON/raw evidence and wrote this directory; no test, browser, DB, service, container, CLI, Provider, child or source/git mutation was performed.
