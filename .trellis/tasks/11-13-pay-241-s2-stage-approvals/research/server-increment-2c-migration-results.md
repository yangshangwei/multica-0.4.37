# S2 increment 2c — coordinator migration verification

Recorded 2026-11-14. **The isolated migration increment passed.** This verifies database shape and downgrade retention, not release/rollback implementation, an external invocation, or whole-S2 acceptance. Policy activation remains disabled.

## Frozen input and correction

The migration binary was built from the independently checked 4,894-file ACL source plus the server owner's declared 42-file early 2c overlay, yielding 4,928 bound files. No live implementation file was silently substituted. The original `ddl-ready.json` and source remain preserved.

Coordinator review found that the original 829 down migration dropped the three tables without repeating the retained-history check. The owner added the same deterministic `ACCESS EXCLUSIVE NOWAIT`/history refusal used by 843 before 829's table drops. The one-file `ddl-ready-r02.json` supplement binds that correction; up migrations are unchanged. This was a source-review correction before execution, not a claimed runtime red test.

Migrations 829–843 add three execution/metric history tables and 13 individually registered, single-statement concurrent indexes. Runtime and final migration readiness inspect actual index keys, predicates, uniqueness, valid/ready/live state and immutability triggers. No foreign keys were added.

## Database results

All eight recorded databases passed actual identity and zero-connection checks before migration:

- Original S2 service/handler normal and race databases.
- Fresh 2c router normal, router race, long-lived harness and migration-verification databases.

Each reached the exact 831-entry frozen ledger through `843_collaboration_stage_execution_ready`. The actual `StageExecutionSchemaReady` query returned true; all 13 new indexes were valid/ready/live with the required shape, and the new tables had zero foreign keys. Raw commands, binary/DDL/configuration hashes, before/after snapshots and direct exit files are retained in the companion evidence.

The original four package profiles remain limited to their exact service/handler packages. New router test profiles explicitly clear real mail and GitHub credentials; their original allocation/migration profiles remain unchanged. The long-lived harness and migration database remain coordinator-owned. No API or browser was launched by this migration verification.

## Downgrade and cleanup verification

Only `multica_pay241_20261114_s2_2c_migrate_r01` was used for the following checks. All source admission was stopped for this database during the multi-file downgrade.

| Check | Observed result |
| --- | --- |
| Empty 843→829 down | Exit 0; ledger returned to 828; three new tables and their indexes were absent. |
| Reapply 829–843 | Exit 0; original ledger, table counts and valid index state were restored. |
| Highest down with one retained synthetic intent | Exit 1 with `stage execution history is retained`; ledger, all indexes and the intent remained unchanged. |
| Direct 829 table-drop down with that same intent | Exit 1 with the same retention refusal; no table/index/ledger loss. |
| `TestStageConcurrentIndexCleanupInventory` | 1 named PASS; includes all current S2 concurrent index registrations. |
| `TestEveryConcurrentUpBuildHasCleanup` | 1 named FAIL for the same eight pre-existing F2 registrations: 795, 796, 797, 798, 799, 801, 802 and 803. No new S2 omission was reported. |

The retained intent is explicitly synthetic migration-test data with no corresponding human approval, deployment fact, provider call or real business issue. It remains in the dedicated migration database to preserve the guard evidence. It must not be used as an execution fixture or acceptance fact. The final database audit found zero other connections.

The global F2 failure remains failed and outside S2; neither its assertion nor its registration map was altered to hide it. No timeout, connection pool or cleanup rule was changed.

## Remaining gates

The server owner is still implementing and testing 2c's real consumers and test-only router harness. Its source has moved beyond this early DDL freeze. Full 2c independent review, 2d submission/completion/project-exit guards, legitimate activation, final Web/Desktop/CLI/Git journey and A01–A12 remain required. Actual external S4 cases remain NOT_RUN.
