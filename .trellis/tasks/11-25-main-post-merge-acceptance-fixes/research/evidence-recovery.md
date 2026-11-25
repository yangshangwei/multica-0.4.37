# Evidence recovery and original identity mapping

2026-11-25: original checkout remains on main at a7bf6ba70fcc3af8171d6a406e0fae6a1b0c29e1. Repair checkout is /Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37, branch codex/main-post-merge-acceptance-fixes. Unrelated original untracked files were not moved or deleted.

All 88 files in the original publication-manifest.json matched recorded SHA256. Original QA directory measures891MiB; .omx/private/qa-work exists and measures44GiB across campaigns. Original memory api.log exists. Each lane must still hash its actual referenced raw evidence; directory presence is not completeness.

All46 raw report/completion references attached to the latest21 failed browser identities are present on the original machine. This presence check does not establish completeness of nested logs/traces; lane-specific hash checks remain required.

Read report.md, defects.md, test-plan.md, coverage-continuation-review.md, host-visit-failure-review.md, backend-continuation-results.md, QA README/manifest and snapshot24. Original selected snapshot JSON remains immutable. Its known incomplete historical attempts (comment-authority-legacy01, legacy-broad01, native-response01, stage01 and business-scenarios01) are not repaired retrospectively.

## Original latest failed browser identities

| Defect | Canonical spec | Latest failed identities | Repair owner / complete closure |
|---|---|---:|---|
| D07 | admin-resource-publishing.spec.ts | 1 | clients: all uploads, market copies, revision/recovery/withdrawal |
| D05 | collaboration-cross-client-execution.spec.ts | 1 | runtime: actual namespace probe then complete A/B/follow-up |
| D07 | collaboration-detail-journey.spec.ts | 2 | clients: complete Web and Desktop manual journey |
| D06 | collaboration-host-installed.spec.ts | 1 | clients: macOS packaged Main stop/restart/resume/history |
| D11 | collaboration-installed-clients.spec.ts | 1 | clients: all10 original operations within240s and canonical cleanup |
| D06 | collaboration-installed-main.spec.ts | 1 | clients: Linux actual packaged Main same-issue handoff/authority/history |
| D05 | collaboration-lane-close-controls.spec.ts | 4 | runtime: full6 cases, preserving the original2 passes separately |
| D05 | collaboration-lifecycle-controls.spec.ts | 2 | runtime: Web and Desktop removal/reconnect/original proof |
| D04 | collaboration-memory-runtime.spec.ts | 1 | runtime: two profiles, real run-local CLI, complete shared context |
| D05 | collaboration-offline-result-notifications.spec.ts | 2 | runtime: complete two clients, source privacy/dedup |
| D04 | collaboration-order-export-20261109.spec.ts | 1 | runtime: complete queues, ownership transfer and AI preservation |
| D07 | collaboration-pay-creation.spec.ts | 2 | clients: full creation and immutable creation identity |
| D09 | collaboration-recovery-extras.spec.ts | 1 | clients: all Desktop subscenarios after natural protected-response hold |
| D08 | inbox-snooze.spec.ts | 1 | clients: complete mention/assignment wakeup and cross-workspace unread |

Total21 original latest failures. D01 comment HTTP probe, D02 service race and D03/D10 security/public-source checks are additional non-browser boundaries. These are not21 independent product defects. Default400PASS/138SKIP, latest original skips117PASS/21FAIL and all-history114PASS/24FAIL remain historical only.

## New infrastructure observations

- Offline frozen-lockfile pnpm install succeeded, Node22.23.2/pnpm10.28.2. Original worktree dependencies were not linked as a substitute.
- Fresh PostgreSQL owner acceptance-fixes-4b9d3494a2, pinned pgvector image cd4ccfdaf62c…; loopback62450, default connection limit100, lock waits logged. Credentials private. Original failed databases/containers remain untouched.
- Separate fresh migrated DBs: fix_comment_red01, fix_comment_red02, fix_service_diag01, fix_security_handler01, fix_runtime_api01, fix_clients_api01. Every allocation checks current_database before migration and records exit0. No API/pool/publisher fields enter ordinary Go environments.
- Initial Web and Desktop production builds exit0 with no source changes during either build. These are preliminary build receipts, not integrated candidate acceptance. Full tool/source/build/resource binding follows final source integration.
- One aggregate cluster check observed4 client backends; this is a point-in-time capacity observation, not final cleanup.

Initial allocation and build HOME directories were task-private but Git-nested; their receipts remain preliminary. New allocator HOME is outside Git under /private/tmp/acceptance-fixes-4b9d3494a2; lanes derive immutable corrected environments for acceptance. Allocator now checks exact container ID/running state/owner/image/volume/loopback binding before database writes. Live ownership and five mismatch-refusal controls passed. Recorder now preserves initial and terminal disposition: intentional absent executable produced launch_failed/unknown child exit and runner2; intentional child7 remained completed/exit7. These are recorder controls, not product tests.

## Execution controls

Internal parent review accepted with explicit corrections. Comment, database-lock, security and runtime plans independently read and accepted by parent; client plan also read and accepted after preparation. Every actual acceptance run needs frozen source/tool/config/resources and final audit. New failures get new run labels. MacOS package preparation must preserve and restore system-wide protocol state. Parent owns final integration and independent review; no commit/push authorized.
