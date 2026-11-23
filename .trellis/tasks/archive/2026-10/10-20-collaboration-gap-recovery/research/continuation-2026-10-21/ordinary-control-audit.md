# Ordinary-control acceptance audit — 2026-10-21

Status: independent S6 audit complete. This report is read-only with respect to product source, acceptance ledgers, environments, gates and commits; it is the only file written in this lane. The audit uses the now-complete broad Go evidence rather than the earlier collaboration-only selectors.

## Evidence basis

The coordinator's current partition inventory contains 1,153 passing service-package top-level tests (109 + 354 + 672 + 18 Redis; the 18 skips in partition 3 are explicit unsupported/test-tag boundaries), all 81 Go packages accounted for, 73 supplemental Redis tests passing, and six current Linux router tests passing. The relevant raw artifact hashes are:

| Artifact | SHA-256 | Top-level result |
| --- | --- | --- |
| `go-service-partition-1.jsonl` | `908053a239dfee9f6957b2bd947049b837ff42322563cafa8476e5e03a1dd4e7` | 109 pass / 0 fail / 0 skip |
| `go-service-partition-2.jsonl` | `1a1edb5507cf4a6b4259194d2d8abfe5c1d2ebb9f56672f52353d999ef07d99d` | 354 pass / 0 fail / 0 skip |
| `go-service-partition-3.jsonl` | `876f6ca2aea52576b2d3da19c3da33abe7de67e30969af1d99af13987270623d` | 672 pass / 0 fail / 18 skip |
| `go-service-redis.jsonl` | `303a188d684786d9bf4ee282316f6448b1ccfc72ac07f86f18d5183c7fffe804` | 18 pass / 0 fail / 0 skip |
| `go-other-packages.jsonl` | `1e71354c2c8743c79c02081dba8450981af0e2b91c699afa1ded78cb470e37f8` | 6,601 pass / 0 fail / 79 skip |
| `go-other-redis.jsonl` | `880e2c33a812a4e2c8ce6a065595311f2a334a43fcc4bdbcec116d423231b54f` | 73 pass / 0 fail / 0 skip |
| `go-remaining-packages.jsonl` | `de59266def0bf98ea12d38d50339d84834f043ccc16e5ddf01339ec98f21bdd4` | 1,873 pass / 0 fail / 14 skip |

The prior final mention/control report remains the source for current dual-client AC90/91/95/97 evidence. This audit does not rerun Go or browser suites and does not reinterpret a test name without reading its assertions.

## Promotion recommendations

### AC02 — recommend `passed`

The original matrix's service identity `internal/service::TestCollaborationWorkflowNotificationsAndWorkbench` passes in `go-service-partition-2-first.jsonl` (0.23 s). The formerly missing handler identity `internal/handler::TestCollaborationWorkbenchKeepsResponsibilityParticipationAndPaginationSeparate` passes in `go-other-packages.jsonl` (0.03 s), and `TestCollaborationNotificationsLoseContentAndAnchorsWithAccess` passes in the completed service partition. The final Web and Electron offline mention journeys then provide the explicit platform clause: H's client/application closes before send, a new legacy API login recovers the same H identity, a fresh context/profile reads the durable comment notification and opens its anchor, and read/archive leaves the issue/workbench/control state unchanged. This supports persistent offline delivery and proves notification state changes do not complete business work.

Boundaries: the UI login is API-authentication followed by a fresh authenticated client; interactive login-page rendering is not claimed. Other lifecycle notification kinds remain covered by the service notification matrix, not repeated through this one comment flow.

### AC12 — recommend `passed`

Current canonical controls now pass:

- `internal/service::TestCollaborationControlOperationReplaysOnlyAuthorizedOriginalResult` (0.33 s in the adjacent final race artifact; the same identity also passed in partition 2 first), preserving actor ownership, original refusal, no state mutation and no recovery by another participant.
- `internal/service::TestCollaborationQueueStaleStopCannotReachNextRun` (0.28 s), proving an old stop cannot affect a later run.
- `internal/handler::TestCollaborationControlHTTPRecoversOriginalStructuredRefusal` (0.05 s), proving structured refusal identity and original-key replay.

The final Web/Electron control journeys show the operator-facing accepted/refused/partial/unknown states, per-lane decisions, original-key recovery and the distinction between `stop_requested`/`stop_unconfirmed` and confirmed termination. Actual Linux route tests supply real proof/result progression. The pending-Permit UI fixture is explicitly not treated as kernel proof; it is the client explanation boundary.

### AC63 — keep `not_verified_in_this_task`

The missing identity `internal/handler::TestWorkspaceWithCollaborationHistoryCannotBeDeleted` passes in `go-other-packages.jsonl` (0.04 s across `todo`, `done`, `cancelled`), and `TestDeleteWorkspace_OwnerSucceeds` plus `TestDeleteWorkspace_PreservesOtherWorkspaceData` also pass. The refusal test snapshots workspace, issue, collaboration, session, participants, comments and events and proves no partial delete; the ordinary positive test proves an owner can still delete a workspace without collaboration records.

That is still narrower than AC63's exact contract: the refused fixture does not include a collaboration attachment or explicitly assert every permission relation before/after. The deletion manifest is static schema coverage, not a transaction snapshot containing those rows. Do not promote until a current test binds an attachment and permission relation into the refusal snapshot, or an equivalent source-backed proof is accepted by the primary owner.

### AC68 — recommend `passed` at the backend boundary

The current broad run closes the previously missing `internal/handler::TestLegacyMixedBatchRefusesBeforeAnyCollaborationSideEffects` (0.01 s). The legacy boundary suite also passes:

- `TestLegacyIssueWritesCannotBypassCollaborationLifecycle` and `TestLegacyCollaborationCommentAndRerunPathsStayInert` (legacy status/assignee/delete/comment/rerun refusal).
- `TestLegacyExecutionSQLBoundaryAndOrdinaryChild` (ordinary and ordinary-child queue insert/deferred/claim/start/retry/promote/reclaim/link paths remain executable; collaboration rows are rejected).
- `TestLegacyExecutionQuickCreateReferenceAndCapturedSource` and `TestLegacyExecutionUnrelatedChatAndQuickCreateStillStart` (ordinary quick-create/capture and chat remain independent; collaboration capture is rejected).
- Collaboration attachment HTTP/service tests cover scoped upload/delivery, revocation, display removal retention and ordinary capability rejection.

The tests read actual assertion bodies: ordinary child execution is allowed without a collaboration queue identity, legacy writes cannot mutate shared lifecycle, mixed legacy batches refuse before effects, and private collaboration attachments cannot be used through ordinary capabilities. This covers the backend R39 contract, including old API/client-shaped paths. Packaged old-client binaries and provider smoke are not part of AC68's primary P1 backend proof and remain general parent boundaries.

### AC81 — keep `not_verified_in_this_task`

Current canonical identities all pass: `TestCollaborationIterationAssignmentVersionsReasonsAndReplay` (0.25 s), `TestCollaborationIterationCompletedJoinRequiresExplicitConfirmation` (0.23 s), `TestCollaborationCreationInitialIterationRefusalNeverPartiallyCreates` (0.17 s), `TestCollaborationSourceStructureAuthorityAndAtomicEdit` (0.25 s), `TestCollaborationSourceStructureRejectsCyclesAndMixedFailure` (0.20 s), plus parent source-membership/iteration boundary tests.

The assertions prove source-authority edits, reasons, revisions, replay refusal, no partial initial create, external-handler source-catalog redaction, and ordinary stage/parent constraints. They do **not** establish the complete explicit clause for an external current handler's iteration statistics, preview and history projection without account leakage, nor every concurrent transfer/leave/iteration-version race in one current matrix. `TestIterationPeriodClosureStillRollsOverCollaboration` uses the collaboration creator as handler, so it is not evidence for the external-handler variant. Keep AC81 open until that external-handler projection/race boundary is current.

### AC82 — recommend `passed` at the backend boundary

The current broad run passes `TestCollaborationOrdinaryChildrenCompleteAuthorizedProjection` (0.29 s), `TestCollaborationOrdinaryChildWritesKeepParentStateAndEmitCommittedHints` (0.08 s), `TestSourceCollaborationTableAndChildMachineBoundary` (0.03 s), and the legacy execution ordinary-child matrix above. Assertion coverage includes: child references are bounded and non-openable for external readers; parent/handler/participants/version and ordinary execution remain unchanged; move/detach/delete-like writes publish only content-free committed hints; foreign-workspace attachment is rejected; machine credentials see references without body/assignee/private collaboration fields; and ordinary child execution remains outside the collaboration queue/resources.

The ordinary-child write test snapshots both collaboration parents, exercises attach, move, rename, detach and batch deletion, and asserts no collaboration event/notification side effects. This is the exact P1 contract; no DOM duplication is necessary.

### AC84 — keep `not_verified_in_this_task`

The current identities pass: `TestCollaborationRuntimeManagedBoundaryRejectsBroadCredentials`, `TestCollaborationRuntimeNarrowHandlersNeverFallBackToOtherPurposes`, `TestCollaborationSquadRuntimeCommentAuthorityAndMentionBoundary`, `TestLegacyIssueDetailDoesNotExposeCollaborationToMachine`, `TestCollaborationMachineSearchAndListKeepOnlyReferences`, `TestCollaborationMachineReferencesDoNotRevealStatusOrPrivateOrdering`, and `TestEventDispatchWithholdsCollaborationIssues`. They prove narrow managed credentials, machine minimal references, no private fields/status/order, squad runtime authority and plugin event withholding.

AC84 explicitly includes plugin/IM outbound redaction. The current implementation/test evidence covers the plugin event dispatcher (`TestEventDispatchWithholdsCollaborationIssues`) but no current collaboration-specific IM/channel integration test was found under `server/internal/integrations`; the full Go pass therefore does not close that outbound surface. Keep the row partial until the IM path is either explicitly unsupported/documented or tested.

### AC87 — keep `not_verified_in_this_task`

All six missing server/service identities now pass: `TestSourceContextCollaborationPreviewIsDenied`, `TestSourceContextCollaborationBothModesRejectBeforeCopyOrAgentLookup`, `TestSourceContextOrdinaryChildHTTPStillCreatesOwnSubtask`, `TestSourceContextCollaborationBuildersDenyBeforeProjection`, `TestSourceContextCollaborationDirectWritersCannotUsePrebuiltCapture`, and `TestSourceContextOrdinaryChildRetainsOwnCaptureAndWorkspaceScope`. Their bodies prove manual/agent/preview refusal before attachment reads, copies, source-context rows or agent lookup, plus the ordinary child positive path.

The explicit UI clause remains unverified. Existing ordinary `issue-detail.test.tsx` tests intentionally show `Create sub-issue from here` for normal issues, and `bubble-menu.test.tsx` covers ordinary selection. The collaboration detail tests cover source-bridge rendering and ordinary-child references but do not assert that collaboration comment/body selection hides the affordance. No current production Web/Electron UI test exercises that negative. Keep AC87 open until one shared UI test or current browser journey proves the collaboration affordance is absent while the ordinary path remains available.

### AC89 — keep `not_verified_in_this_task`

Current backend identities pass: `TestCollaborationPositionUsesSourceAuthorityWithoutChangingCollaborationVersion` (0.01 s), all five child stage-wakeup identities, `TestCollaborationSourceStructureAuthorityAndAtomicEdit`, and the iteration/source race tests. They prove a source-authorized position edit leaves collaboration revision unchanged, machine position writes are rejected, source-stage edits record reasons atomically, and ordinary parent wakeup preserves stage/status/parent guards, terminal categories, and stale-parent rechecks.

The criterion still explicitly requires the same-column board drag and CLI `issue reorder` positive paths. The current CLI tests exercise generic ordinary issues, not a collaboration issue through the CLI; the position handler test is HTTP-level and not a board UI journey. Concurrent transfer/leave/source-stage races are covered only in narrower service paths, not the complete board+CLI combination. Keep AC89 open.

## Additional exact current evidence

`TestCollaborationControlOperationReplaysOnlyAuthorizedOriginalResult`, `TestCollaborationQueueStaleStopCannotReachNextRun` and `TestCollaborationControlHTTPRecoversOriginalStructuredRefusal` all pass in the current artifacts. `TestCollaborationIterationAssignmentVersionsReasonsAndReplay` and `TestCollaborationSourceStructureAuthorityAndAtomicEdit` record source reasons and revision/event assertions; `TestCollaborationChildCompletionWakesOrdinaryParentOnce`, `...PreservesParentGuards`, `...TerminalCategoriesUseOrdinaryStageWakeup` and `...WakeupRechecksParentBeforePublishing` all pass in `go-other-packages.jsonl`. These names are included here only with their assertion-level boundaries above; they are not blanket promotion of AC81/89.

## Verification and limits

This audit ran no product suite and changed no source or environment. It read source assertions and parsed current raw JSONL. The full Go inventory has no failed top-level tests; explicit skips remain visible in the inventory. Core/Views/Web/Desktop checks and the final mention/control UI evidence are inherited from current task reports. No provider account/quota, packaged installer/full Main, Windows/other-OS UI, or unknown external IM deployment was exercised.

Final recommendations: promote AC02, AC12, AC68 and AC82 if the coordinator's ledger policy permits combined primary-layer evidence; leave AC63, AC81, AC84, AC87 and AC89 `not_verified_in_this_task` with the concrete gaps above. Do not change acceptance status from this report alone.
