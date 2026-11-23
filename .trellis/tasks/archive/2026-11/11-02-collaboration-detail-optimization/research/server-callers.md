# Existing Server completion callers

Adapted 17 existing Server test files to require real formal delivery and exact initiator acceptance in successful completion fixtures. This lane changed no production, Core, UI, E2E, spec, dependency or commit files. Root owns those surfaces and the final whole-package / real-client acceptance.

`deliveredCompletionParams` is an explicit positive fixture helper: it submits through the real `SubmitAcceptance` service with the actual current handler and observed epochs, then captures the submission ID and resulting revision. Self-handled fixtures supply result/basis atomically. The generic `lifecycleParams` is unchanged, so canonical missing-delivery, stale and role refusals retain their original inputs.

Existing run/proof/control/notification/parent/version/idempotency assertions remain. Catalog and iteration fixtures use an allowed non-review category, with explicit generic-review refusal retained. Parent HTTP receipts check the new narrow delivery confirmation while retaining internal-parent non-disclosure. Router and resource-offer fixtures preserve former-handler denial and recover/read their exact original results through the authorized actor.

R38 actual forced-completion/cancellation behavior is retained. Its dedicated `TestAdminCollaborationLifecyclePreservesCreatorAndRealActor` source is byte-identical to baseline. Root approved creator cancellation for separate closed-issue administration setup, preserving the external handler and every disabled-role/successor/reopen/audit assertion. No raw handler reassignment or administrator substitution was introduced.

## Verification

All commands used the exact serial task-owned package runner; no database allocation, migration, drop, switch or package-wide execution was performed by this lane. Red evidence is retained. Final caller coverage is **198 unique service named tests across 47 top-level tests, 28 handler named tests, and 2 router named tests**, all under `-race`, with no remaining failures or skips. Service coverage combines the first broad named run with the targeted correction of its one resource-offer fixture; it is not represented as a second full-run result.

| Run label | Exec handle | Exit | Pass | Fail | Skip | Exact report |
| --- | --- | --- | --- | --- | --- | --- |
| `callers-existing-red-v2` | `71397` | 1 | 92 | 101 | 0 | [callers-existing-red-v2-20261103-095809.json](./continuation-20261103/callers-existing-red-v2-20261103-095809.json) |
| `callers-host-red-v2` | `33239` | 1 | 3 | 2 | 0 | [callers-host-red-v2-20261103-102559.json](./continuation-20261103/callers-host-red-v2-20261103-102559.json) |
| `callers-parent-http-red-v2` | `77643` | 1 | 7 | 21 | 0 | [callers-parent-http-red-v2-20261103-102700.json](./continuation-20261103/callers-parent-http-red-v2-20261103-102700.json) |
| `callers-router-red-v2` | `65939` | 1 | 0 | 2 | 0 | [callers-router-red-v2-20261103-105703.json](./continuation-20261103/callers-router-red-v2-20261103-105703.json) |
| `callers-service-green-v2` | `32111` | 1 | 196 | 2 | 0 | [callers-service-green-v2-20261103-105956.json](./continuation-20261103/callers-service-green-v2-20261103-105956.json) |
| `callers-offer-green-v2` | `36652` | 0 | 7 | 0 | 0 | [callers-offer-green-v2-20261103-110701.json](./continuation-20261103/callers-offer-green-v2-20261103-110701.json) |
| `callers-parent-http-green-v2` | `4855` | 0 | 28 | 0 | 0 | [callers-parent-http-green-v2-20261103-110915.json](./continuation-20261103/callers-parent-http-green-v2-20261103-110915.json) |
| `callers-router-green-v2` | `42520` | 0 | 2 | 0 | 0 | [callers-router-green-v2-20261103-111126.json](./continuation-20261103/callers-router-green-v2-20261103-111126.json) |

The first service green attempt passed 196 named tests and failed the resource-offer close/reopen case: real delivery had made its old handler a former handler, so `forbidden` correctly preceded `stale_offer`. The correction retains that former-handler denial and checks the immutable stale offer with the current handler read from `ResolveIssueAccess`. Its seven-case rerun passed. No production finding was concealed by the fixture update.

Compile-only service and handler test binaries passed (handles `61869`, `50872`), without `TestMain` execution. `go -C server vet ./internal/service ./internal/handler ./cmd/server` passed after the final correction (handle `2301`); all 17 files pass `gofmt -l` and scoped `git diff --check`. All test runner and static process handles are closed, and the shared DB slot is released.

## Final owned source hashes

Baseline copies and SHA-256 values: `/tmp/multica-server-callers-v2-20261103/baseline.json`. Structured final inventory / protected-function hashes: `/tmp/multica-server-callers-v2-20261103/final-report.json`. Only these existing Go files changed; this report is the sole additional authored task artifact.

| File | Final SHA-256 |
| --- | --- |
| `server/cmd/server/collaboration_handoff_routes_test.go` | `b9b853bd0fa21a4c3ef6418dd4166b71943a0947efaec3be8bca863af7f917f3` |
| `server/cmd/server/collaboration_lifecycle_routes_test.go` | `bca30ed480ed54d76afd583e02d0e07ed2b395feeee9dcfcd1738800341218cc` |
| `server/internal/handler/collaboration_parent_wakeup_test.go` | `dcbe7b070390c3aec4baa2c5eab4befaa836849f79bb56efbe7fa94e00a21b81` |
| `server/internal/service/admin_collaboration_authority_test.go` | `d761a884e7bb1ea8347d2b780104976afb9a93c332d16db1461adedf62e3f4f0` |
| `server/internal/service/admin_collaboration_test.go` | `678d28ed1d196939c40099a597186004d25b6c70d5f074b5c6be71623c76dfe5` |
| `server/internal/service/admin_collaboration_transaction_test.go` | `8725924936f749870071b0ac9267df2acc9e56a32811d7fb176357212673f310` |
| `server/internal/service/collaboration_command_history_execution_test.go` | `cccf467d700a068bfcaee3660d8704631416bf8a9c571f08d340e66cb329e9ef` |
| `server/internal/service/collaboration_command_history_test.go` | `8363cc5253c589d9c77a61cc8c95eac3fe97fd43d19dbbed8738dd3b6c59021c` |
| `server/internal/service/collaboration_handoff_execution_boundary_test.go` | `c3cf580ea20cebff0ab18ee7b101385e035b01f16ed2f96f7fef170a1d02f290` |
| `server/internal/service/collaboration_host_integration_test.go` | `4edc3f20eef3e2ae4905f90a84d8467633c498b545d61acd3a079999b4ecccf2` |
| `server/internal/service/collaboration_lifecycle_test.go` | `a892d2c1d464b76920359594c8195c7403fee360454e801dca6d28bd6586b92a` |
| `server/internal/service/collaboration_memory_test.go` | `5e7270a1c861f2b541f151bd3e27245ae79be9441b689e08cd26ea6fd78e3b0b` |
| `server/internal/service/collaboration_mention_admission_states_test.go` | `49d1b9415592b213ba00bbf67105bc908be45ed85a3e8db28656d6e1c1b36e78` |
| `server/internal/service/collaboration_parent_wakeup_test.go` | `fa888124a8b2835411113161307daead27b8e369799ddd0740036827109bb679` |
| `server/internal/service/collaboration_resource_offers_test.go` | `6faaad804080af9b4c91d8eb457b918b6a7b7dc7d98b7c9539d8dabe67ca7d69` |
| `server/internal/service/collaboration_targets_review_test.go` | `65afed94d24411fc860ea32d321c6c9654648d53f4c6693b7019c3c54768d78b` |
| `server/internal/service/collaboration_targets_test.go` | `e976fa2346d56dd7bfaa950da82f957c2a9d563c1bcded5cf330de33c4f4e4a7` |

Protected function hashes:

- `lifecycleParams` — `451c0938696cb753f559e9d9e1795b0ec1f05558189b4de4149faef793617386`
- `TestAdminCollaborationLifecyclePreservesCreatorAndRealActor` — `895ba5613bd32b60a0866aba589d18702cb91f82f4e01d70b6624b754f27f275`

## Remaining scope

No pending caller changes or known local failures. Root still owns fresh complete-package verification and actual production-Web/Electron acceptance; these named runs establish database protocol and HTTP fixture behavior. No commit was made.

## Fresh full-router recovery follow-up — 2026-11-03

The fresh full-router run `full-router-race-final-2-20261103-122205.jsonl` exposed one obsolete test expectation: a committed `handler_must_transfer` own receipt recovered after participant removal was passed through the two-field success verifier. The approved `GetCollaborationOperation` contract returns HTTP 200 with the original body `status: 409` for refusals; `CollaborationOperationSchema` permits that status, and canonical `TestCollaborationDeliveryHTTPRefusalHasCommittedStatusAndOriginalKey` requires it. This is a content-free failed operation receipt, independent of current issue read rights.

Changed only `server/cmd/server/collaboration_workflow_routes_test.go`: the strict two-field success verifier stays byte-identical, while the refusal path checks exactly three fields, immutable original code/revision/status, HTTP 200 and no-store. It also pins the initial refusal revision to 5. Responsibility, denied issue reads, original-key/foreign-actor checks, no fabricated membership, no ordinary AI and malformed-input assertions remain. Production code and all earlier caller edits are untouched.

| Run | Handle | Pass | Fail | Skip | Report |
| --- | --- | --- | --- | --- | --- |
| `callers-workflow-recovery-red-v3` | `58036` | 0 | 1 | 0 | [callers-workflow-recovery-red-v3-20261103-131455.json](./continuation-20261103/callers-workflow-recovery-red-v3-20261103-131455.json) |
| `callers-workflow-recovery-green-v3` | `69986` | 1 | 0 | 0 | [callers-workflow-recovery-green-v3-20261103-131804.json](./continuation-20261103/callers-workflow-recovery-green-v3-20261103-131804.json) |
| `callers-full-router-race-v3` | `73949` | 731 | 0 | 7 | [callers-full-router-race-v3-20261103-132325.json](./continuation-20261103/callers-full-router-race-v3-20261103-132325.json) |

The narrow red/green pair used the retained failed database `multica_detail_final2_20261103_115734_router`. Full verification used the newly allocated schema-only database `multica_detail_callers_router_20261103_132048_b2feb410`, cloned from `multica_detail_final_template_20261103_101054`; initial user/workspace/issue/operation counts were zero and the migration ledger contained 752 rows. No failed database/log was dropped, existing runtime stopped, or service/handler suite rerun. The new database has 0 remaining connections after the full suite.

Full router race result: **731 named passes, zero failures, seven skips**. Exact skip identities/reasons:

- `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux` — collaboration_daemon_cli_routes_test.go:27: requires explicit Linux namespace integration environment
- `TestCollaborationPartialResumeRoutesLinux` — collaboration_hold_routes_test.go:155: requires explicit Linux namespace integration environment
- `TestKnowledgeQueuedFrameRejectsStalePATCacheAfterRevocationOrExpiry` — collaboration_knowledge_realtime_test.go:125: requires isolated REDIS_TEST_URL
- `TestCollaborationSingleAgentRuntimeRoutesLinux` — collaboration_single_agent_routes_test.go:418: requires explicit Linux namespace integration environment
- `TestCollaborationHeldSquadAllowsDirectAgentRoutesLinux` — collaboration_single_agent_routes_test.go:585: requires explicit Linux namespace integration environment
- `TestCollaborationRuntimeProofSurvivesOwnerDisableLinux` — collaboration_single_agent_routes_test.go:688: requires explicit Linux namespace integration environment
- `TestCollaborationSquadCapacityOneRoutesLinux` — collaboration_squad_routes_test.go:173: requires explicit Linux namespace integration environment

Final source proof:

- Router test before SHA-256: `52957d795a96e7edba508745c649d80479ce6a348fae1b2dc814e054a41b10ff`.
- Router test after SHA-256: `623f3cb96e359777572ea36b53a2476c73c5b1c6b4df5f88e198f2a99d08e9f8`.
- Strict success verifier SHA-256 (unchanged): `b7a7fc7d91d3fe72a464242e4e7a027ebb0c3259359f4dfd79050e508ce39902`.
- 4417 server / guard source files have identical before/after aggregate SHA-256 `4145fcc170135243030ef9c99d3852e73d504d5248839fcbca057f73bd11a684`; no file changed during the full run.
- HEAD `952f76bb5ae46a3df45f4d3487ab5b7ac7e3fb1a`; toolchain `go version go1.27.0 darwin/arm64`.
- `go -C server vet ./cmd/server` passed (handle `29318`); gofmt and scoped diff checks passed.

Structured follow-up evidence: [callers-workflow-recovery-final-v3.json](./continuation-20261103/callers-workflow-recovery-final-v3.json). Private source manifests and baseline copy remain under `/tmp/multica-server-callers-v2-20261103/workflow-recovery/`. All follow-up process handles are closed and the DB slot is released. No commit was made. The six Linux integration skips and isolated-Redis case remain explicit environment coverage limits.
