# Runtime handoff / recording source adjudication

Reviewed at 2026-11-08T09:53:34.120350+08:00. Target: `e541cfb436fed9c6733174479718b651e2cf030d`.

## Decision

No production or test patch is recommended from these three Runtime sources. This conclusion is based on actual working-file bytes, same-path committed history, and named current behavior/tests. It does not use the previously rejected rule that an older source HEAD or an existing path makes a dirty delta obsolete.

The audit covers **163 source/path comparisons across 91 distinct paths**:
- 54 are byte-identical to target HEAD.
- 55 differ from target but match an exact blob at the same product path in target ancestry.
- 54 differ from every inspected product-path snapshot and were reconciled semantically. These include seven blobs that the initial provenance scan called reachable only because research snapshots retained them.

Source worktrees and integration worktree were read only. Only this Markdown report and its JSON companion were written. Tests were inspected, not executed; these decisions do not claim a new quality-gate pass.

## Sources

| Source | Identical | Exact prior product revision | Semantically superseded |
| --- | ---: | ---: | ---: |
| `/Volumes/artisan/code/2026/mc-handoff-cursor-20261101` | 20 | 21 | 19 |
| `/Volumes/artisan/code/2026/mc-handoff-launch-20261031` | 3 | 7 | 9 |
| `/Volumes/artisan/code/2026/mc-runtime-recording-recovery-20261101` | 31 | 27 | 26 |

## Why global blob reachability was insufficient

Seven source blobs exist under `.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/integration-verification/late-reconciliation/late-main/`, rather than at the same product path in history: `execution-panel.test.tsx`, `client_host.go`, `client_recording_ack_test.go`, `host_checkpoint_ledger.go`, `host_checkpoints.go`, `host_runner.go`, and `host_recording_recovery_test.go`. They were not accepted as already integrated on this basis.

The prior [late reconciliation decisions](../../10-30-collaboration-context-handoff-runtime/research/integration-verification/late-reconciliation/decisions.json) preserve their original intent and exact alternatives. Current committed source was independently checked against that mapping. In particular:

- `TestHostRecordingRecoveryBlockedUploadCannotPreventStopAndProof` remains at `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:233`.
- `TestHostRecordingRecoveryMalformedSealACKKeepsOriginalPending` remains at the same file, line 332.
- Restart without new execution is covered by `TestHostHandoffCheckpointRestartRecoversWithoutExecution`; unchanged checkpoint bytes/key by `TestHostCheckpointsKeepOriginalPayloadAcrossLostACK`.
- Proof priority and total recovery deadline are covered by `TestHostRecordingRecoverySharesBacklogDeadlineAndPrioritizesProof`.
- Original seal replay before monotonic advancement is covered by `TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement`. Requiring a separate second Runner pass is an obsolete implementation expectation, not a lost product contract.
- Client ACK truth, missing/null/count matrices and exact request binding are covered by the compact `client_recording_ack_test.go` suite and the canonical `collaboration_recording_retention_test.go` protocol suite.

## Semantically reconciled paths

### server/internal/daemon/collaborationrun/host_runner.go

Both older variants lack current recoverHost recording/seal ordering or use a ten-second report window; current recoverHost gives phaseProof priority and shares hostRecordingRecoveryContext with metadata-after-proof sealing. The late stable-snapshot/input-binding omissions must not replace current negotiated startup.

Current committed evidence:

- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:49` — `TestHostRecordingRecoverySharesBacklogDeadlineAndPrioritizesProof`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:233` — `TestHostRecordingRecoveryBlockedUploadCannotPreventStopAndProof`.
- `server/internal/daemon/collaborationrun/host_handoff_checkpoint_lifecycle_test.go:193` — `TestHostHandoffCheckpointRestartRecoversWithoutExecution`.

### server/pkg/db/generated/collaboration_records.sql.go

Old LIMIT 258 would omit later seal metadata. Current LIMIT 290 retains the complete bounded checkpoint and seal history; regression writes the maximum history and checks original-key replay.

Current committed evidence:

- `server/internal/service/collaboration_checkpoints_test.go:19` — `TestCollaborationRecordingSealRetainsMaximumCheckpointHistory`.
- `server/pkg/db/queries/collaboration_records.sql:46` — `ORDER BY seq LIMIT 290`.

### server/pkg/db/queries/collaboration_records.sql

Old LIMIT 258 would omit later seal metadata. Current LIMIT 290 retains the complete bounded checkpoint and seal history; regression writes the maximum history and checks original-key replay.

Current committed evidence:

- `server/internal/service/collaboration_checkpoints_test.go:19` — `TestCollaborationRecordingSealRetainsMaximumCheckpointHistory`.
- `server/pkg/db/queries/collaboration_records.sql:46` — `ORDER BY seq LIMIT 290`.

### server/internal/daemon/collaborationrun/host_checkpoint_ledger.go

Current finishHostRecording persists the explicit Drained fact; persistHostRecording validates authorized nonce and bounds; checkpoint terminal ACKs cannot flip accepted/rejected. The late hostRecordingSealAdvancePending condition is implemented by hostRecordingPending/hostRecordingChunksPending plus coalescing in sealHostRecording; old closeHostRecording would lose unknown-tail truth.

Current committed evidence:

- `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go:104` — `func hostRecordingPending`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:176` — `TestHostRecordingRecoveredUnflushedTailRemainsUnknown`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:203` — `TestHostRecordingLedgerRejectsSealWithUnissuedNonce`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:121` — `TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement`.

### server/internal/daemon/collaborationrun/host_checkpoints.go

The variants omit Drained completion and stagingFailed, use per-request or two-second replay limits, or lack monotonic seal advancement. Current shared one-second recovery context, original-seal-first replay, coalesced same-budget advance, and stagingFailed latch cover valid old behavior while retaining stronger no-gap semantics.

Current committed evidence:

- `server/internal/daemon/collaborationrun/host_checkpoints.go:192` — `func hostRecordingRecoveryContext`.
- `server/internal/daemon/collaborationrun/host_checkpoint_gap_test.go:21` — `TestHostCheckpointLocalFailureCannotRebaseQueuedMessages`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:121` — `TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:49` — `TestHostRecordingRecoverySharesBacklogDeadlineAndPrioritizesProof`.

### server/internal/daemon/collaborationrun/host_handoff.go

Cursor/launch variant only substitutes the obsolete 256-KiB raw response cap. Recording variant also generates a random snapshot request, weakens memory/identity validation, and lacks stable fallback/input binding. Current uses separate 2-MiB wire cap and stable run-derived request identity; copying either variant regresses documented startup behavior.

Current committed evidence:

- `server/internal/daemon/collaborationrun/host_handoff.go:60` — `multica:handoff-snapshot:v1`.
- `server/internal/daemon/collaborationrun/host_handoff_budget_test.go:69` — `TestHostHandoffBudgetEnforcesRawHTTPBoundaryBeforeLaunch`.
- `server/internal/daemon/collaborationrun/host_handoff_checkpoint_test.go:20` — `TestHostHandoffCheckpointFailureCleanup`.

### server/internal/handler/collaboration_comments_recall_test.go

Source has no added cases: it only omits TestCollaborationRecallHTTPRejectsAmbientCredentials and its uuid import. Current keeps its budget/full-comment case and adds authority refusal.

Current committed evidence:

- `server/internal/handler/collaboration_comments_recall_test.go:14` — `TestCollaborationRecallHTTPRejectsAmbientCredentials`.
- `server/internal/handler/collaboration_comments_recall_test.go:27` — `TestCollaborationRecallHTTPBudgetAndLegacyFullComment`.

### server/internal/service/collaboration_checkpoints.go

Cursor variant only permits a single seal and omits outcome identity checks; recording variant duplicates a weaker local validateRecordingSealAdvance. Current shared protocol validator also binds Identity and validates initial/final bounds, while SealRuntimeRecording keeps per-sequence original keys and permits monotonic receipt advancement.

Current committed evidence:

- `server/internal/service/collaboration_checkpoints.go:75` — `protocol.ValidateCollaborationRecordingSealAdvance`.
- `server/pkg/protocol/collaboration_checkpoint.go:139` — `func ValidateCollaborationRecordingSealAdvance`.
- `server/internal/service/collaboration_checkpoints_test.go:19` — `TestCollaborationRecordingSealRetainsMaximumCheckpointHistory`.

### server/internal/service/collaboration_checkpoints_test.go

Source tests are a strict earlier subset (recording variant deletes 54 lines; cursor also omits Sequence=1 in a seal fixture). Current retains original checkpoint-stop/outcome tests and adds maximum-checkpoint/multiple-seal/original-replay checks.

Current committed evidence:

- `server/internal/service/collaboration_checkpoints_test.go:19` — `TestCollaborationRecordingSealRetainsMaximumCheckpointHistory`.
- `server/internal/service/collaboration_checkpoints_test.go:72` — `TestCollaborationCheckpointStopKeepsOriginalPrefixAndContentFreeACK`.
- `server/internal/service/collaboration_checkpoints_test.go:142` — `TestCollaborationCheckpointOutcomeOverlapAndInternalEventsStayReadable`.

### server/internal/service/collaboration_handoff_snapshot.go

Source omits boundCollaborationHandoff when storing/restoring snapshots and accepts run/version fields on handoff sources. Current preserves bounded source prefixes and full admitted delegation while refusing malformed source binding.

Current committed evidence:

- `server/internal/service/collaboration_handoff_snapshot.go:117` — `return out, boundCollaborationHandoff`.
- `server/internal/service/collaboration_runtime_handoff_budget_test.go:16` — `TestCollaborationHandoffSnapshotPreservesMaximumAssignment`.
- `server/internal/service/collaboration_runtime_handoff_budget_test.go:71` — `TestCollaborationHandoffBudgetKeepsAssignmentAndSourceReferences`.

### server/internal/service/collaboration_runtime_handoff.go

Source variants lose information-request event sequence and boundCollaborationHandoff. Cursor variant also omits recording retention and nil PromptSnapshot handling; recording variant weakens strict cursor field/run binding to json.Unmarshal with prefilled RunID. Current keeps actual event provenance, complete required delegation, explicit truncation, recording state and strict cursors.

Current committed evidence:

- `server/internal/service/collaboration_runtime_handoff.go:201` — `GetCollaborationInformationRequestEventSequence`.
- `server/internal/service/collaboration_runtime_handoff.go:466` — `func boundCollaborationHandoff`.
- `server/internal/service/collaboration_handoff_cursor_test.go:61` — `TestCollaborationHandoffCursorRejectsMalformedBindings`.
- `server/internal/service/collaboration_runtime_handoff_budget_test.go:71` — `TestCollaborationHandoffBudgetKeepsAssignmentAndSourceReferences`.

### server/internal/service/collaboration_runtime_handoff_test.go

Source adds no test lines relative to 0e1ee8e1e; it omits the information-request provenance/replay scenarios and later live-memory-base coverage. Existing frozen assignment, merged wake causes and purged-memory tests retain all source cases.

Current committed evidence:

- `server/internal/service/collaboration_runtime_handoff_test.go:16` — `TestCollaborationHandoffFrozenAssignmentAndSnapshot`.
- `server/internal/service/collaboration_runtime_handoff_test.go:100` — `TestCollaborationHandoffPreservesEveryMergedWakeCause`.
- `server/internal/service/collaboration_runtime_handoff_test.go:176` — `TestCollaborationHandoffSnapshotCannotResurrectPurgedMemory`.

### server/pkg/db/generated/collaboration_handoff.sql.go

Source only omits GetCollaborationInformationRequestEventSequence. Current query scopes the original request event, enabling real provenance instead of an unconditional zero sentinel.

Current committed evidence:

- `server/pkg/db/queries/collaboration_handoff.sql:53` — `GetCollaborationInformationRequestEventSequence`.
- `server/internal/service/collaboration_runtime_handoff.go:205` — `information_request_event_unknown:`.

### server/pkg/db/queries/collaboration_handoff.sql

Source only omits GetCollaborationInformationRequestEventSequence. Current query scopes the original request event, enabling real provenance instead of an unconditional zero sentinel.

Current committed evidence:

- `server/pkg/db/queries/collaboration_handoff.sql:53` — `GetCollaborationInformationRequestEventSequence`.
- `server/internal/service/collaboration_runtime_handoff.go:205` — `information_request_event_unknown:`.

### server/pkg/protocol/collaboration_checkpoint.go

Source variants omit strict retention UnmarshalJSON/required-fields checks and monotonic seal validator; earliest variant lacks seal Sequence bounds. Current centralizes these checks, retains the existing checkpoint/metadata constraints and refuses decreasing/foreign-identity seal histories.

Current committed evidence:

- `server/pkg/protocol/collaboration_checkpoint.go:79` — `func ValidateCollaborationRecordingRetention`.
- `server/pkg/protocol/collaboration_checkpoint.go:139` — `func ValidateCollaborationRecordingSealAdvance`.
- `server/pkg/protocol/collaboration_checkpoint_test.go:91` — `TestCollaborationRecordingSealAdvancesBoundedKnowledge`.
- `server/pkg/protocol/collaboration_recording_retention_test.go:13` — `TestCollaborationRecordingRetentionWireRequiresFacts`.

### server/pkg/protocol/collaboration_checkpoint_test.go

All source tests are earlier subsets. Current keeps bounded checkpoint, record compatibility and metadata-only seal cases, adds Sequence=1 to fixtures and covers monotonic sequence/ACK/observed/identity/reason limits.

Current committed evidence:

- `server/pkg/protocol/collaboration_checkpoint_test.go:13` — `TestCollaborationCheckpointBoundsAndKinds`.
- `server/pkg/protocol/collaboration_checkpoint_test.go:68` — `TestCollaborationRecordingSealHasOnlyBoundedMetadata`.
- `server/pkg/protocol/collaboration_checkpoint_test.go:91` — `TestCollaborationRecordingSealAdvancesBoundedKnowledge`.

### server/pkg/protocol/collaboration_handoff.go

Source variants use one encoded 256-KiB limit and simple decoder, omit nested required-field decoding and in the earliest variant recording retention. Current separate material/wire accounting accepts legal escaped delegation, bounds metadata, validates nested facts and retains recording projection and memory-base distinction.

Current committed evidence:

- `server/pkg/protocol/collaboration_handoff.go:452` — `func CollaborationHandoffSizes`.
- `server/pkg/protocol/collaboration_handoff_test.go:13` — `TestCollaborationHandoffPreservesEscapedAdmittedAssignment`.
- `server/pkg/protocol/collaboration_handoff_decode_test.go:53` — `TestCollaborationHandoffWireRequiresDeclaredFields`.
- `server/pkg/protocol/collaboration_recording_retention_test.go:43` — `TestCollaborationHandoffRejectsMalformedRetention`.

### server/pkg/protocol/collaboration_handoff_test.go

Source only removes escaped-assignment/raw-JSON/material-metadata tests. Its false-completeness and unknown-version case remains unchanged in current; no unique source assertion is missing.

Current committed evidence:

- `server/pkg/protocol/collaboration_handoff_test.go:13` — `TestCollaborationHandoffPreservesEscapedAdmittedAssignment`.
- `server/pkg/protocol/collaboration_handoff_test.go:63` — `TestCollaborationHandoffRejectsFalseCompletenessAndUnknownVersion`.

### server/pkg/protocol/collaboration_recall.go

Source omits required-field decode for comments, explicit nullable next_cursor presence, and truncation-reason coherence. Current retains all valid source paging/content handling and rejects aliases/missing or inconsistent wire facts.

Current committed evidence:

- `server/pkg/protocol/collaboration_recall.go:64` — `func (c *CollaborationRecallComment) UnmarshalJSON`.
- `server/pkg/protocol/collaboration_recall.go:218` — `reasons["more_comments"]`.

### packages/core/api/client.ts

Compared to nearest product ancestor source only removes raw handoff wire-size enforcement from fetchCollaboration/fetchCollaborationOptionalHistory. Current getCollaborationRunHandoff retains optional-route compatibility and identity binding while enforcing COLLABORATION_HANDOFF_MAX_JSON_BYTES; replacing the full old file would additionally remove newer Knowledge/delivery APIs.

Current committed evidence:

- `packages/core/collaboration/run-handoff-schema.test.ts:40` — `bounds the actual HTTP JSON bytes`.
- `packages/core/api/client.ts:6` — `COLLABORATION_HANDOFF_MAX_JSON_BYTES`.

### packages/views/collaboration/execution-panel.test.tsx

The late blanket mock of CollaborationRunHandoff isolates record queues by removing real component wiring. Current endpoint-aware fetch fixture handles /handoff and issue endpoints while keeping the rendered handoff behavior; source mock should not be restored.

Current committed evidence:

- `packages/views/collaboration/execution-panel.test.tsx:73` — `if (path.endsWith("/handoff"))`.

### packages/views/collaboration/run-records.tsx

Source handoff/retention additions already exist in current. Actual byte diff against current only removes passing the issue handler into RunRecords and CollaborationRunHandoff, which would lose correct human identity labels.

Current committed evidence:

- `packages/views/collaboration/run-records.tsx:101` — `handler={handler}`.
- `packages/views/collaboration/run-handoff.test.tsx:32` — `does not label a historical handler`.

### server/internal/daemon/collaborationrun/client_host.go

Late raw embedded-retention DTO wrapper is unnecessary and conflicts with promoted UnmarshalJSON. Current shared CollaborationRecordingRetention decoder enforces field presence/nullability/counts; RecordingSeal adds request-bound stream/reason/sealed validation. Exact checkpoint ACK end-count behavior remains.

Current committed evidence:

- `server/internal/daemon/collaborationrun/client_recording_ack_test.go:17` — `TestClientCheckpointACKMatchesOriginalRequestBound`.
- `server/internal/daemon/collaborationrun/client_recording_ack_test.go:48` — `TestClientRecordingSealRejectsInvalidACK`.
- `server/pkg/protocol/collaboration_recording_retention_test.go:13` — `TestCollaborationRecordingRetentionWireRequiresFacts`.

### packages/core/collaboration/run-handoff-queries.ts

Only difference is source uses raw runId in queryKey; current lowercases it, preserving case-insensitive resource identity and avoiding duplicate cache entries. No missing source behavior.

Current committed evidence:

- `packages/core/collaboration/run-handoff-queries.ts:7` — `runId.toLowerCase()`.

### packages/core/collaboration/run-handoff-schema.test.ts

Source is a strict test subset: no new assertions; current adds legal escaped assignment, material/wire boundary, unknown metadata and launch-base/snapshot-mode checks.

Current committed evidence:

- `packages/core/collaboration/run-handoff-schema.test.ts:50` — `preserves absence and exact handoff identity`.
- `packages/core/collaboration/run-handoff-schema.test.ts:40` — `bounds the actual HTTP JSON bytes`.

### packages/core/collaboration/run-handoff-schema.ts

Source single JSON.stringify byte check and stripped-object counting are superseded by separate material/wire accounting on the original envelope, explicit snapshot id and live/base memory validation. All original identity/page/source checks remain.

Current committed evidence:

- `packages/core/collaboration/run-handoff-schema.test.ts:19` — `accepts a legal 64 KiB source`.
- `packages/core/collaboration/run-handoff-schema.test.ts:36` — `counts additional response metadata`.
- `packages/core/collaboration/run-handoff-schema.test.ts:63` — `keeps a live read`.

### packages/views/collaboration/run-handoff.test.tsx

Current keeps both source tests with stronger handler assertions and adds wrong-handler and live-memory-base regressions. Source offers no unique assertions absent from current.

Current committed evidence:

- `packages/views/collaboration/run-handoff.test.tsx:32` — `does not label a historical handler`.
- `packages/views/collaboration/run-handoff.test.tsx:53` — `labels a live read`.

### packages/views/collaboration/run-handoff.tsx

Source shows raw handler UUID and labels latest live memory as launch base. Current retains source display and source links, adds identity-matched handler name and HandoffMemory separating live latest from recorded launch base.

Current committed evidence:

- `packages/views/collaboration/run-handoff.test.tsx:32` — `does not label a historical handler`.
- `packages/views/collaboration/run-handoff.test.tsx:53` — `labels a live read`.

### server/internal/daemon/collaborationrun/client_recording_ack_test.go

All late test intent is retained: malformed/missing/negative bounds are canonical protocol decoder cases plus HTTP wiring checks; truthful empty/known-missing/unknown and later-count original-seal ACK cases are in current compact suite. Original full matrix need not be duplicated through HTTP.

Current committed evidence:

- `server/internal/daemon/collaborationrun/client_recording_ack_test.go:48` — `TestClientRecordingSealRejectsInvalidACK`.
- `server/internal/daemon/collaborationrun/client_recording_ack_test.go:79` — `TestClientRecordingSealAcceptsTruthfulRetention`.
- `server/pkg/protocol/collaboration_recording_retention_test.go:13` — `TestCollaborationRecordingRetentionWireRequiresFacts`.
- `server/pkg/protocol/collaboration_recording_retention_test.go:79` — `TestCollaborationRecordingRetentionPreservesKnownAndUnknownBounds`.

### server/internal/daemon/collaborationrun/host_recording_recovery_test.go

Six late scenarios reconciled: restart+original bytes -> HandoffCheckpointRestart and CheckpointsKeepOriginalPayload; proof priority+whole backlog deadline -> SharesBacklogDeadlineAndPrioritizesProof; old seal before advancement -> SealReplaysBeforeAdvancingAcknowledgement; unique blocked Stop and malformed ACK after reopening remain as same-named current regressions. No source scenario lost; current adds unknown-tail, nonce, cancelled backlog coverage.

Current committed evidence:

- `server/internal/daemon/collaborationrun/host_handoff_checkpoint_lifecycle_test.go:193` — `TestHostHandoffCheckpointRestartRecoversWithoutExecution`.
- `server/internal/daemon/collaborationrun/host_checkpoints_test.go:18` — `TestHostCheckpointsKeepOriginalPayloadAcrossLostACK`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:49` — `TestHostRecordingRecoverySharesBacklogDeadlineAndPrioritizesProof`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:121` — `TestHostRecordingSealReplaysBeforeAdvancingAcknowledgement`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:233` — `TestHostRecordingRecoveryBlockedUploadCannotPreventStopAndProof`.
- `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:332` — `TestHostRecordingRecoveryMalformedSealACKKeepsOriginalPending`.

## Complete source/path disposition

The JSON companion retains actual source blob, source HEAD, source-delta line counts, exact history match or concrete semantic references, and original comparison diffs for every differing path. The following table also includes byte-identical dirty files.

| Source | Path | Decision | Evidence |
| --- | --- | --- | --- |
| `mc-handoff-cursor-20261101` | `server/cmd/multica/cmd_collaboration.go` | superseded-exact-history | `0e1ee8e1e:server/cmd/multica/cmd_collaboration.go`; blob `6601772b0` |
| `mc-handoff-cursor-20261101` | `server/cmd/multica/cmd_collaboration_recall.go` | identical-to-target | target blob `b3f439cdb` |
| `mc-handoff-cursor-20261101` | `server/cmd/multica/cmd_collaboration_recall_test.go` | identical-to-target | target blob `272eeadda` |
| `mc-handoff-cursor-20261101` | `server/cmd/multica/cmd_issue.go` | superseded-exact-history | `0e1ee8e1e:server/cmd/multica/cmd_issue.go`; blob `e9a72089a` |
| `mc-handoff-cursor-20261101` | `server/cmd/server/router.go` | superseded-exact-history | `0e1ee8e1e:server/cmd/server/router.go`; blob `0fc0700cd` |
| `mc-handoff-cursor-20261101` | `server/internal/cli/collaboration_recall.go` | identical-to-target | target blob `f9e511735` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/client.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/client.go`; blob `61d38bdc6` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/client_context_extensions_test.go` | identical-to-target | target blob `9b5252f66` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/client_host.go` | identical-to-target | target blob `354434534` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go`; blob `4debfa3fe` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_agentintegration_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_agentintegration_test.go`; blob `34e109f55` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go:104`; `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:176` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_checkpoints.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_checkpoints.go:192`; `server/internal/daemon/collaborationrun/host_checkpoint_gap_test.go:21` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_checkpoints_test.go` | identical-to-target | target blob `7582e77b7` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_handoff.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_handoff.go:60`; `server/internal/daemon/collaborationrun/host_handoff_budget_test.go:69` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_handoff_checkpoint_lifecycle_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_handoff_checkpoint_lifecycle_test.go`; blob `17e384957` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_handoff_checkpoint_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_handoff_checkpoint_test.go`; blob `eeb2a9b94` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go`; blob `0407fa163` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_handoff_launch_test.go` | identical-to-target | target blob `002d9e14a` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_ledger.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_ledger.go`; blob `8ed815293` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_recording.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_recording.go`; blob `be94bbb81` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/host_runner.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:49`; `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:233` |
| `mc-handoff-cursor-20261101` | `server/internal/daemon/collaborationrun/ledger.go` | identical-to-target | target blob `8d557d516` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_checkpoints.go` | identical-to-target | target blob `27f17ef53` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_comments.go` | identical-to-target | target blob `8518ead6a` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_comments_recall.go` | identical-to-target | target blob `7fb34d7d5` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_comments_recall_test.go` | superseded-semantic | `server/internal/handler/collaboration_comments_recall_test.go:14`; `server/internal/handler/collaboration_comments_recall_test.go:27` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_runtime.go` | superseded-exact-history | `0e1ee8e1e:server/internal/handler/collaboration_runtime.go`; blob `5acee92f9` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_runtime_extensions_test.go` | identical-to-target | target blob `7c8ac6850` |
| `mc-handoff-cursor-20261101` | `server/internal/handler/collaboration_runtime_handoff.go` | identical-to-target | target blob `e3b0c3faf` |
| `mc-handoff-cursor-20261101` | `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md` | superseded-exact-history | `0e1ee8e1e:server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md`; blob `9bb43b8f3` |
| `mc-handoff-cursor-20261101` | `server/internal/service/builtin_skills/multica-working-on-issues/references/collaboration-workflow.md` | superseded-exact-history | `0e1ee8e1e:server/internal/service/builtin_skills/multica-working-on-issues/references/collaboration-workflow.md`; blob `7b8bdeaeb` |
| `mc-handoff-cursor-20261101` | `server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md` | superseded-exact-history | `0e1ee8e1e:server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md`; blob `e9aaca3b0` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_checkpoints.go` | superseded-semantic | `server/internal/service/collaboration_checkpoints.go:75`; `server/pkg/protocol/collaboration_checkpoint.go:139` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_checkpoints_test.go` | superseded-semantic | `server/internal/service/collaboration_checkpoints_test.go:19`; `server/internal/service/collaboration_checkpoints_test.go:72` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_comments_recall.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_comments_recall.go`; blob `59d7056ca` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_comments_recall_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_comments_recall_test.go`; blob `26faa3cbb` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_comments_view.go` | identical-to-target | target blob `a45025728` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_handoff_cursor_test.go` | identical-to-target | target blob `c3fa25d3f` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_handoff_snapshot.go` | superseded-semantic | `server/internal/service/collaboration_handoff_snapshot.go:117`; `server/internal/service/collaboration_runtime_handoff_budget_test.go:16` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_handoff_wakeup.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_handoff_wakeup.go`; blob `03dfce54a` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_records.go` | identical-to-target | target blob `61ca7566b` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_runtime_capabilities.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_runtime_capabilities.go`; blob `064aaf0cc` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_runtime_handoff.go` | superseded-semantic | `server/internal/service/collaboration_runtime_handoff.go:201`; `server/internal/service/collaboration_runtime_handoff.go:466` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_runtime_handoff_test.go` | superseded-semantic | `server/internal/service/collaboration_runtime_handoff_test.go:16`; `server/internal/service/collaboration_runtime_handoff_test.go:100` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_squad.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_squad.go`; blob `8916fc4e4` |
| `mc-handoff-cursor-20261101` | `server/internal/service/collaboration_squad_join.go` | identical-to-target | target blob `17a8f8a98` |
| `mc-handoff-cursor-20261101` | `server/pkg/db/generated/collaboration_handoff.sql.go` | superseded-semantic | `server/pkg/db/queries/collaboration_handoff.sql:53`; `server/internal/service/collaboration_runtime_handoff.go:205` |
| `mc-handoff-cursor-20261101` | `server/pkg/db/generated/collaboration_recall.sql.go` | identical-to-target | target blob `730f93c93` |
| `mc-handoff-cursor-20261101` | `server/pkg/db/generated/collaboration_records.sql.go` | superseded-semantic | `server/internal/service/collaboration_checkpoints_test.go:19`; `server/pkg/db/queries/collaboration_records.sql:46` |
| `mc-handoff-cursor-20261101` | `server/pkg/db/queries/collaboration_handoff.sql` | superseded-semantic | `server/pkg/db/queries/collaboration_handoff.sql:53`; `server/internal/service/collaboration_runtime_handoff.go:205` |
| `mc-handoff-cursor-20261101` | `server/pkg/db/queries/collaboration_recall.sql` | identical-to-target | target blob `e9b5ed463` |
| `mc-handoff-cursor-20261101` | `server/pkg/db/queries/collaboration_records.sql` | superseded-semantic | `server/internal/service/collaboration_checkpoints_test.go:19`; `server/pkg/db/queries/collaboration_records.sql:46` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration.go` | superseded-exact-history | `0e1ee8e1e:server/pkg/protocol/collaboration.go`; blob `d75089f82` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration_checkpoint.go` | superseded-semantic | `server/pkg/protocol/collaboration_checkpoint.go:79`; `server/pkg/protocol/collaboration_checkpoint.go:139` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration_checkpoint_test.go` | superseded-semantic | `server/pkg/protocol/collaboration_checkpoint_test.go:13`; `server/pkg/protocol/collaboration_checkpoint_test.go:68` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration_handoff.go` | superseded-semantic | `server/pkg/protocol/collaboration_handoff.go:452`; `server/pkg/protocol/collaboration_handoff_test.go:13` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration_handoff_test.go` | superseded-semantic | `server/pkg/protocol/collaboration_handoff_test.go:13`; `server/pkg/protocol/collaboration_handoff_test.go:63` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration_recall.go` | superseded-semantic | `server/pkg/protocol/collaboration_recall.go:64`; `server/pkg/protocol/collaboration_recall.go:218` |
| `mc-handoff-cursor-20261101` | `server/pkg/protocol/collaboration_record.go` | identical-to-target | target blob `ae5674d64` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/client.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/client.go`; blob `61d38bdc6` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/client_host.go` | identical-to-target | target blob `354434534` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go`; blob `4debfa3fe` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_agentintegration_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_agentintegration_test.go`; blob `34e109f55` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go:104`; `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:176` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_checkpoints.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_checkpoints.go:192`; `server/internal/daemon/collaborationrun/host_checkpoint_gap_test.go:21` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_handoff.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_handoff.go:60`; `server/internal/daemon/collaborationrun/host_handoff_budget_test.go:69` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_handoff_codex_launch_test.go`; blob `0407fa163` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_handoff_launch_test.go` | identical-to-target | target blob `002d9e14a` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_ledger.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_ledger.go`; blob `8ed815293` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_recording.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_recording.go`; blob `be94bbb81` |
| `mc-handoff-launch-20261031` | `server/internal/daemon/collaborationrun/host_runner.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:49`; `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:233` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration.go` | superseded-exact-history | `0e1ee8e1e:server/pkg/protocol/collaboration.go`; blob `d75089f82` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration_checkpoint.go` | superseded-semantic | `server/pkg/protocol/collaboration_checkpoint.go:79`; `server/pkg/protocol/collaboration_checkpoint.go:139` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration_checkpoint_test.go` | superseded-semantic | `server/pkg/protocol/collaboration_checkpoint_test.go:13`; `server/pkg/protocol/collaboration_checkpoint_test.go:68` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration_handoff.go` | superseded-semantic | `server/pkg/protocol/collaboration_handoff.go:452`; `server/pkg/protocol/collaboration_handoff_test.go:13` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration_handoff_test.go` | superseded-semantic | `server/pkg/protocol/collaboration_handoff_test.go:13`; `server/pkg/protocol/collaboration_handoff_test.go:63` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration_recall.go` | superseded-semantic | `server/pkg/protocol/collaboration_recall.go:64`; `server/pkg/protocol/collaboration_recall.go:218` |
| `mc-handoff-launch-20261031` | `server/pkg/protocol/collaboration_record.go` | identical-to-target | target blob `ae5674d64` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/api/client.ts` | superseded-semantic | `packages/core/collaboration/run-handoff-schema.test.ts:40`; `packages/core/api/client.ts:6` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/api/collaboration-content.test.ts` | identical-to-target | target blob `73e70e785` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/comment-recall-queries.test.ts` | identical-to-target | target blob `a08216c96` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/content-queries.ts` | identical-to-target | target blob `814891537` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/content-schema.test.ts` | identical-to-target | target blob `28a6f1bf3` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/content-schema.ts` | identical-to-target | target blob `5702da87b` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/index.ts` | superseded-exact-history | `0e1ee8e1e:packages/core/collaboration/index.ts`; blob `cdb48d098` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-handoff-queries.ts` | superseded-semantic | `packages/core/collaboration/run-handoff-queries.ts:7` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-handoff-schema.test.ts` | superseded-semantic | `packages/core/collaboration/run-handoff-schema.test.ts:50`; `packages/core/collaboration/run-handoff-schema.test.ts:40` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-handoff-schema.ts` | superseded-semantic | `packages/core/collaboration/run-handoff-schema.test.ts:19`; `packages/core/collaboration/run-handoff-schema.test.ts:36` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-records-queries.test.ts` | identical-to-target | target blob `ad3f66f89` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-records-queries.ts` | identical-to-target | target blob `2acfa09a1` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-records-schema.test.ts` | identical-to-target | target blob `a1e5060cc` |
| `mc-runtime-recording-recovery-20261101` | `packages/core/collaboration/run-records-schema.ts` | identical-to-target | target blob `d2ab5e945` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/collaboration-detail.test.tsx` | superseded-exact-history | `0e1ee8e1e:packages/views/collaboration/collaboration-detail.test.tsx`; blob `b3dd143fe` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/collaboration-detail.tsx` | superseded-exact-history | `0e1ee8e1e:packages/views/collaboration/collaboration-detail.tsx`; blob `1fc32472a` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/comment-list.test.tsx` | superseded-exact-history | `0e1ee8e1e:packages/views/collaboration/comment-list.test.tsx`; blob `038c838d4` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/comment-list.tsx` | superseded-exact-history | `0e1ee8e1e:packages/views/collaboration/comment-list.tsx`; blob `b3c1114dc` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/execution-panel.test.tsx` | superseded-semantic | `packages/views/collaboration/execution-panel.test.tsx:73` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/labels.ts` | superseded-exact-history | `0e1ee8e1e:packages/views/collaboration/labels.ts`; blob `00e4fd95d` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/run-handoff.test.tsx` | superseded-semantic | `packages/views/collaboration/run-handoff.test.tsx:32`; `packages/views/collaboration/run-handoff.test.tsx:53` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/run-handoff.tsx` | superseded-semantic | `packages/views/collaboration/run-handoff.test.tsx:32`; `packages/views/collaboration/run-handoff.test.tsx:53` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/run-records.tsx` | superseded-semantic | `packages/views/collaboration/run-records.tsx:101`; `packages/views/collaboration/run-handoff.test.tsx:32` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/collaboration/workflow.tsx` | superseded-exact-history | `0e1ee8e1e:packages/views/collaboration/workflow.tsx`; blob `b75a70c23` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/locales/en/collaboration.json` | superseded-exact-history | `0e1ee8e1e:packages/views/locales/en/collaboration.json`; blob `b01611bf7` |
| `mc-runtime-recording-recovery-20261101` | `packages/views/locales/zh-Hans/collaboration.json` | superseded-exact-history | `0e1ee8e1e:packages/views/locales/zh-Hans/collaboration.json`; blob `1561872ba` |
| `mc-runtime-recording-recovery-20261101` | `server/cmd/multica/cmd_collaboration.go` | superseded-exact-history | `0e1ee8e1e:server/cmd/multica/cmd_collaboration.go`; blob `6601772b0` |
| `mc-runtime-recording-recovery-20261101` | `server/cmd/multica/cmd_collaboration_recall.go` | identical-to-target | target blob `b3f439cdb` |
| `mc-runtime-recording-recovery-20261101` | `server/cmd/multica/cmd_collaboration_recall_test.go` | identical-to-target | target blob `272eeadda` |
| `mc-runtime-recording-recovery-20261101` | `server/cmd/multica/cmd_issue.go` | superseded-exact-history | `0e1ee8e1e:server/cmd/multica/cmd_issue.go`; blob `e9a72089a` |
| `mc-runtime-recording-recovery-20261101` | `server/cmd/server/router.go` | superseded-exact-history | `0e1ee8e1e:server/cmd/server/router.go`; blob `0fc0700cd` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/cli/collaboration_recall.go` | identical-to-target | target blob `f9e511735` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/client.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/client.go`; blob `61d38bdc6` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/client_context_extensions_test.go` | identical-to-target | target blob `9b5252f66` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/client_host.go` | superseded-semantic | `server/internal/daemon/collaborationrun/client_recording_ack_test.go:17`; `server/internal/daemon/collaborationrun/client_recording_ack_test.go:48` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/client_recording_ack_test.go` | superseded-semantic | `server/internal/daemon/collaborationrun/client_recording_ack_test.go:48`; `server/internal/daemon/collaborationrun/client_recording_ack_test.go:79` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_checkpoint_ledger.go:104`; `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:176` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_checkpoints.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_checkpoints.go:192`; `server/internal/daemon/collaborationrun/host_checkpoint_gap_test.go:21` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_checkpoints_test.go` | identical-to-target | target blob `7582e77b7` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_handoff.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_handoff.go:60`; `server/internal/daemon/collaborationrun/host_handoff_budget_test.go:69` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_ledger.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_ledger.go`; blob `8ed815293` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_recording.go` | superseded-exact-history | `0e1ee8e1e:server/internal/daemon/collaborationrun/host_recording.go`; blob `be94bbb81` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_recording_recovery_test.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_handoff_checkpoint_lifecycle_test.go:193`; `server/internal/daemon/collaborationrun/host_checkpoints_test.go:18` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/host_runner.go` | superseded-semantic | `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:49`; `server/internal/daemon/collaborationrun/host_recording_recovery_test.go:233` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/daemon/collaborationrun/ledger.go` | identical-to-target | target blob `8d557d516` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_checkpoints.go` | identical-to-target | target blob `27f17ef53` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_checkpoints_test.go` | identical-to-target | target blob `d6824cf37` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_comments.go` | identical-to-target | target blob `8518ead6a` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_comments_recall.go` | identical-to-target | target blob `7fb34d7d5` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_comments_recall_test.go` | identical-to-target | target blob `163b36e95` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_runtime.go` | superseded-exact-history | `0e1ee8e1e:server/internal/handler/collaboration_runtime.go`; blob `5acee92f9` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_runtime_extensions_test.go` | identical-to-target | target blob `7c8ac6850` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_runtime_handoff.go` | identical-to-target | target blob `e3b0c3faf` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/handler/collaboration_runtime_test.go` | identical-to-target | target blob `a6f810c4d` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md` | superseded-exact-history | `0e1ee8e1e:server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md`; blob `9bb43b8f3` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/builtin_skills/multica-working-on-issues/references/collaboration-workflow.md` | superseded-exact-history | `0e1ee8e1e:server/internal/service/builtin_skills/multica-working-on-issues/references/collaboration-workflow.md`; blob `7b8bdeaeb` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md` | superseded-exact-history | `0e1ee8e1e:server/internal/service/builtin_skills/multica-working-on-issues/references/working-on-issues-source-map.md`; blob `e9aaca3b0` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_checkpoints.go` | superseded-semantic | `server/internal/service/collaboration_checkpoints.go:75`; `server/pkg/protocol/collaboration_checkpoint.go:139` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_checkpoints_test.go` | superseded-semantic | `server/internal/service/collaboration_checkpoints_test.go:19`; `server/internal/service/collaboration_checkpoints_test.go:72` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_comments_recall.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_comments_recall.go`; blob `59d7056ca` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_comments_recall_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_comments_recall_test.go`; blob `26faa3cbb` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_comments_view.go` | identical-to-target | target blob `a45025728` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_handoff_snapshot.go` | superseded-semantic | `server/internal/service/collaboration_handoff_snapshot.go:117`; `server/internal/service/collaboration_runtime_handoff_budget_test.go:16` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_handoff_wakeup.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_handoff_wakeup.go`; blob `03dfce54a` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_records.go` | identical-to-target | target blob `61ca7566b` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_runtime_capabilities.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_runtime_capabilities.go`; blob `064aaf0cc` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_runtime_handoff.go` | superseded-semantic | `server/internal/service/collaboration_runtime_handoff.go:201`; `server/internal/service/collaboration_runtime_handoff.go:466` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_runtime_handoff_test.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_runtime_handoff_test.go`; blob `484e53339` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_squad.go` | superseded-exact-history | `0e1ee8e1e:server/internal/service/collaboration_squad.go`; blob `8916fc4e4` |
| `mc-runtime-recording-recovery-20261101` | `server/internal/service/collaboration_squad_join.go` | identical-to-target | target blob `17a8f8a98` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/db/generated/collaboration_handoff.sql.go` | superseded-semantic | `server/pkg/db/queries/collaboration_handoff.sql:53`; `server/internal/service/collaboration_runtime_handoff.go:205` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/db/generated/collaboration_recall.sql.go` | identical-to-target | target blob `730f93c93` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/db/generated/collaboration_records.sql.go` | superseded-semantic | `server/internal/service/collaboration_checkpoints_test.go:19`; `server/pkg/db/queries/collaboration_records.sql:46` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/db/queries/collaboration_handoff.sql` | superseded-semantic | `server/pkg/db/queries/collaboration_handoff.sql:53`; `server/internal/service/collaboration_runtime_handoff.go:205` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/db/queries/collaboration_recall.sql` | identical-to-target | target blob `e9b5ed463` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/db/queries/collaboration_records.sql` | identical-to-target | target blob `4084c7313` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration.go` | superseded-exact-history | `0e1ee8e1e:server/pkg/protocol/collaboration.go`; blob `d75089f82` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_checkpoint.go` | superseded-semantic | `server/pkg/protocol/collaboration_checkpoint.go:79`; `server/pkg/protocol/collaboration_checkpoint.go:139` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_checkpoint_test.go` | superseded-semantic | `server/pkg/protocol/collaboration_checkpoint_test.go:13`; `server/pkg/protocol/collaboration_checkpoint_test.go:68` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_handoff.go` | superseded-semantic | `server/pkg/protocol/collaboration_handoff.go:452`; `server/pkg/protocol/collaboration_handoff_test.go:13` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_handoff_test.go` | superseded-semantic | `server/pkg/protocol/collaboration_handoff_test.go:13`; `server/pkg/protocol/collaboration_handoff_test.go:63` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_recall.go` | superseded-exact-history | `0e1ee8e1e:server/pkg/protocol/collaboration_recall.go`; blob `b3aba5e66` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_recall_test.go` | identical-to-target | target blob `3ff40677b` |
| `mc-runtime-recording-recovery-20261101` | `server/pkg/protocol/collaboration_record.go` | identical-to-target | target blob `ae5674d64` |

## Limitations and handoff

This adjudicates preserved Runtime source intent at the pinned target SHA. It does not establish runtime correctness under every environment, nor replace the coordinating session's targeted tests and complete integration gate. No missing behavior or minimal corrective patch was found in this slice. Do not overwrite the original archived source snapshots.
