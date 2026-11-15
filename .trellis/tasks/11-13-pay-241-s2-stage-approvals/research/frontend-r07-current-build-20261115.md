# S2 r07 current frontend build and scoped checks

**FRONTEND_BUILD_AND_SCOPED_CHECKS_PASS.** The current ROOT frontend has a new isolated production Web and Electron renderer/preload build. This lane did not start an API, browser, Provider or daemon, and did not execute SQL. Actual Web/Desktop stage acceptance remains 0/4; S2 stays open.

The new graph contains 3,358 source files, 1,961 pinned dependency links and 26 internal workspace links. It includes the current knowledge and comment UI plus the two already reviewed P6 fixture changes. Copy and final audits found no current ROOT input drift; all internal workspace imports resolve into the new graph. LIVE files and the old 3,127-file build were preserved. Private copies use 0600 while the original ROOT modes are recorded separately.

| Verification | Actual result |
| --- | --- |
| Core/Views/Web and Desktop node/web typechecks | Five commands, exit 0 |
| Core stage/API and Views stage/comment container lint | Two commands, exit 0 |
| Core stage/completion/comment-thread tests | 486 passing parameter records in 27 files |
| Views stage/completion/detail/comment/workflow tests | 133 passing records in 9 files |
| Production Next build | Exit 0; BUILD_ID `PTmbcBOUMtBvWXyVYvpWH` |
| Electron renderer/preload build | Exit 0; CLI bundle remains separate |
| Final product output/trace audit | 1,676 output files; 90 NFT files; no foreign workspace or missing traced target |
| Source/dependency/process audit | 3,358 inputs and 1,961 links unchanged; no recorded build PID remains |

All 619 original passing parameter records are retained, with zero failed, skipped or todo records. They contain 602 different file/fullName pairs: five existing `it.each` groups legitimately repeat their display names, accounting for 17 additional rows. The first derived audit incorrectly required display-name uniqueness. Its failure is preserved; a new parser receipt uses file-local row ordinals and explicitly lists those name groups. No test, build, product source or assertion was rerun or changed to correct this inventory.

Next rewrote only `apps/web/next-env.d.ts` from `.next/dev/types/routes.d.ts` to `.next/types/routes.d.ts`. The exact production declaration change is recorded in a new binding, preserving the initial binding. Both builds retain two existing CSS optimizer warnings about `::highlight`; no style change was made.

Final frontend descriptor: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/frontend-current-r07-r01/input-binding-production.json`, SHA256 `315a7654ef0c367e3bd6fd09274f8b377813bb35531864c2f4f700ef844ac4d5`. Final derived audit: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/frontend-current-r07-r01/final-audit-r02.json`, SHA256 `270778459494336276247fdfaa33de0b42fdef7eef182fb02918e706f34a74eb`. The adjacent JSON binds all original command/results, source, output, trace and parser-correction receipts.

ROOT can use this descriptor with its accepted P2/API/CLI/Host build. Before running the four actual scenarios, it must bind the task runtime configuration, verify the live API/WS and production frontend identities, complete task-owned administrator initialization/password change, and recheck the three assigned ports. The real payment/CI/deployment and final performance evidence remain separate.

Only this report pair and the new external build/evidence tree were written. The work reused installed dependencies and existing build commands; no dependencies, product abstractions, LIVE edits, commits or task status changes were introduced.
