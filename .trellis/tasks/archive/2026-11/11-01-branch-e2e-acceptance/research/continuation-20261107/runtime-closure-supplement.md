# Runtime canonical closure supplement

**All6 requested canonical tests passed with race detection,0 failures and0 skips.** The six are disjoint from the prior390-case ledger. Combined results are396 top-level PASS and1014 named PASS terminals including subtests; original backend result files remain unchanged.

Candidate: `4c817dee9397a42ac903f419af7804b11d2d863e`. Backend/scripts fingerprint matches the original snapshot. Additional E2E fixture changes are deliberately outside that backend fingerprint. No product changes or real Provider calls.

| Package | Canonical test | Result |
|---|---|---|
| `./pkg/protocol` | `TestCollaborationHandoffWireBoundsMemoryBaseVersion` | PASS |
| `./internal/service` | `TestCollaborationHandoffSnapshotPreservesMaximumAssignment` | PASS |
| `./internal/service` | `TestCollaborationRuntimeCommentsCarryMemberDisplayNames` | PASS |
| `./internal/service` | `TestCollaborationRecallRuntimePagesFulltextAndCurrentAuthority` | PASS |
| `./internal/service` | `TestCollaborationRecallChunksReassembleUTF8AndRejectCrossBinding` | PASS |
| `./internal/cli` | `TestCollaborationRuntimeCommentKeepsOptionalDisplayName` | PASS |

Protocol contributes8 named terminals, service9, and CLI1. All selected top-level names ran; no unexpected names were selected. Raw commands, JSON test events, source identity and summaries remain under `.omx/private/branch-e2e-20261107/backend/runtime-closure/`.

The four service tests used a new owned database `branch_e2e_20261107_runtime_service_a3`, verified empty before migrations/tests and retained with0 connections afterward. HOME/TMP were outside Git; the agent CLI guard remained enabled. Final process scan found no command under the owned state prefix. Build/vet and the broader390 tests were not repeated.

These are canonical protocol/service/CLI regressions, not new browser, packaging, real-model or full-suite claims. The parent/evidence reviewer owns final runtime audit closure interpretation.
