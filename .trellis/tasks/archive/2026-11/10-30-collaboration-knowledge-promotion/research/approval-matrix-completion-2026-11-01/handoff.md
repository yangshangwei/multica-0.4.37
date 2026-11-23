# Approval/recovery acceptance matrix completion

This followup adds tests only. No production code, existing test file, SQL/query, generated file, runtime/logger implementation, task status or commit was changed. No Provider was invoked.

## Added evidence

| Approved gap | New executable scenario |
| --- | --- |
| Target role/membership/human lead | `TestKnowledgeApprovalHTTPMatrixAuthorityAndProjectDeletionBothOrders`: actual `UpdateMember`, `DeleteMember` and `UpdateProject` writers versus actual approval, both orders |
| Actor disable | `TestKnowledgeApprovalMatrixActorDisableBothCommitOrders`: actual `PlatformAdminService.ChangeAccount` versus approval, both orders |
| Required source mutation | `TestKnowledgeApprovalMatrixSourceWritesBothCommitOrders`: actual historical memory purge, comment edit and comment delete, both orders |
| Withdrawal/current review CAS | `TestKnowledgeApprovalMatrixPendingWithdrawalBothCommitOrders`: same originally pending candidate, exact observed revision retained |
| Approved withdrawal versus correction | `TestKnowledgeApprovalMatrixApprovedWithdrawalAndCorrectionBothOrders`: old approved version withdrawal and independent new correction confirmation, both orders |
| Competing correction approvals | `TestKnowledgeApprovalMatrixCompetingCorrectionsBothCommitOrders`: version 2/version 3 contention; original retry after real NOWAIT, no duplicate publication or downgrade |
| Parallel correction allocation | `TestKnowledgeApprovalMatrixConcurrentCorrectionAllocation`: two actors sharing the same observed predecessor; one contiguous next version and one stale refusal, followed by explicit latest-predecessor version 3 |
| Project deletion | HTTP matrix executes actual `DeleteProject`; source issue has no structural association with the independent target project. Winning approval history survives deletion; target/counter do not |
| Negative recovery identity | `TestKnowledgeApprovalMatrixRecoveryRejectsOtherCandidateAndKinds`: valid candidate B with A's key, and valid candidate with real memory/command UUID; all return empty receipt plus `operation_not_found`. HTTP wrong-candidate wiring also verifies content-free 404 |
| Deleted-comment human projections | `TestKnowledgeApprovalMatrixDeletedCommentSuppressesAllHumanProjections`: real deletion suppresses list/detail/history/target body, summary, refs/hash and quoted evidence/reason while retaining independent approval |
| Missing legacy source | `TestKnowledgeApprovalHTTPMatrixLegacyDeletedSourceKeepsDeletionGuard`: ordinary delete is first refused by the real guard; an explicitly test-seeded legacy missing issue then produces no source body or history through human APIs |

Actual writers are held immediately before commit. Blocking followers are observed through `pg_blocking_pids` before release. Production NOWAIT intersections are verified through actual returned PostgreSQL `55P03`, bounded `knowledge_busy`, and the same uncommitted request identity after the first writer commits. No mutation is replaced by a standalone raw lock query.

## Results and identities

The matrix comprises **30 independent leaf cases**: service **8 top-level tests / 27 passing events / 20 leaves**, handler **3 top-level tests / 11 passing events / 10 leaves**. Normal and race runs cover the same cases and must not be added together. Exact final logs, command exit codes, expected names, wait/NOWAIT observations and final source fingerprints are in the adjacent JSON artifacts.

Exclusive database: `multica_knowledge_matrix_20261101_9eee46c7`; environment `/tmp/multica_knowledge_matrix_20261101_9eee46c7.env` (0600). The connection template was used only to create this new database. Each run explicitly sourced the environment, checked its marker/current database, and ran service and handler suites sequentially. Main owns the separate final e705 database and broad gate.

Initial failures were test-fixture corrections, not product defects: unused imports; public purge correctly protecting current memory (the test now makes required v1 historical by creating v2); and a legacy command receipt omitting its request ID (the test now retains its explicit submitted UUID). All raw failures are retained. No assertion was weakened to permit a product mismatch.

Only the two files in `owned-files.txt` belong to this followup. C2's preparation versus ordinary writers, held approval versus unrelated ordinary operations, prepare/source-revoke and claim-build/finalize authority changes remain with its separate test owner. Main retains final combined review and completion accounting.
