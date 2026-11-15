# Collaboration stage authority and evidence

Scope/code-review and source-denial boundaries have focused normal/race and independent review evidence. Full stage activation, later execution facts and joint Web/Desktop acceptance remain separate gates. Do not enable project stage policy until every real completion/submission/project-exit writer consumes its final guard.

## 1. Scope / trigger

Read this before adding a stage action, changing a source-evidence prerequisite, consuming a human approval, or displaying private stage evidence. A collaboration participant's issue access and their membership in the source workspace are different authorities. Locking a user does not lock that user's source membership.

The fixed business stages do not replace collaboration delivery status or ordinary child execution grouping. Stage approval records authorize precise versions; they do not dispatch work or establish that a deployment occurred.

## 2. Signatures

- `collaborationStageSourceMemberKind(kind string) bool` in `service/collaboration_stage.go` selects stage actions whose workflow transaction retains the existing source-member fence. It includes `stageKindConfirmScope`.
- `CollaborationService.workflowInTx` in `service/collaboration_workflow.go` acquires that fence before its final access read. Adding a source prerequisite inside an action without joining this lock path is insufficient.
- `CollaborationService.ConfirmStageScope` consumes current source authority for a nonempty child selection. `StageView` projects the corresponding child identities.
- `checkStageFinalAuthority(ctx, tx, access, state, decisions) (time.Time, string, error)` in `service/collaboration_stage_signer.go` checks retained issuers, the current caller and the final database clock. `AdvanceStage` calls it after artifact observations and uses its returned time for the CI horizon.
- `CollaborationService.readCollaborationStageEvidence(ctx, issueID, read)` in `service/collaboration_stage_evidence_read.go` shares the repeatable-read transaction used by R4 and T4. It permits M1 authority row locks; it is not a `READ ONLY` transaction.
- `testReportFrozenSourceRead(ctx, tx, access, frozen)` checks the original run subject's exact artifact through `lockDeliveryArtifactInTx`. `testReportSourceRead` also verifies that a retained report agrees with that subject; QA references use the same caller transaction.
- `collaborationStageViewOptions(client, scope, issueId, workspaceId)` in `packages/core/collaboration/stage-queries.ts` requires the actual issue's `source_workspace.id`, not the current navigation workspace. It connects R1 to the existing private-evidence source fence.
- `CollaborationStageCodeCandidateSchema` in `packages/core/collaboration/stage-schema.ts` validates the bounded R4 execution group and overlapping intent/approval/fact identities before shared UI consumes it.
- `cmd/migrate/main.go` owns `concurrentIndexCleanups`; every new concurrent index needs its actual migration stem and index name registered there.

## 3. Contracts

### Source authority and scope confirmation

A nonempty A5 selection requires current source membership as well as the existing product-role, participant, lifecycle and version checks. Hold the member-row fence through scope recording and commit. A cached child digest does not confer permission after membership is removed. The source fence must not itself require a member row for an otherwise-authorized empty external scope confirmation or reason-only return.

R1 child fields have an exact conditional contract:

```json
{"can_open": false, "content_digest": null}
```

These are excerpts, not complete response objects. `can_open:true` requires the existing hex64 digest; `can_open:false` requires explicit null. Keep the permitted child identifier/title projection. Do not substitute a dummy digest or disclose the unsalted title/description hash: a known title makes a short private description guessable offline.

A5 is an immediate confirmation. Verify its human credential when writing, but do not subsequently expire scope merely because that JWT/PAT expires. Scope applicability follows the recorded lifecycle, enabled-policy period, participant/account and role revisions. Removing and restoring a role must not revive an old scope.

### Standing approvals and the final clock

A7 standing approvals retain verified original credential metadata, finite decision expiry and the original source-member row identity. Never persist a bearer token. On consumption, recheck the original signer; the consuming user's valid credential cannot stand in for an expired or revoked issuer. Removing and re-adding source membership permits a fresh signature, not revival of the old one.

Run potentially slow artifact/evidence observations before the final authority check. The helper checks current issuer/caller credentials under the existing locks, reads the database clock again after those checks, and compares every retained decision/issuer expiry against that final time. Use the same returned time to check evidence horizons before the write. An earlier `now` or an expiry check before an observer is not the final consumption check.

Code-review CI requires a complete, nonempty current-head check set, an observation no later than the database time and within the 30-minute horizon, and every check explicitly completed/success. Missing, unknown, skipped, neutral, running or unsupported repository-less VCS status cannot satisfy it. The stable CI digest excludes refresh time. Equal provider timestamps with different heads invalidate snapshot completeness rather than selecting a winner.

### Historical report reads and artifact authority

Source-workspace membership alone does not authorize an artifact-bound report. T4 list/detail and optional QA report references must reuse M1's current initiator/manager authority for the **original frozen subject's artifact**. Verify its child, approval, execution, spec, receipt and output identities. Do not substitute the latest candidate's artifact or require the historical candidate, PR head or failed outcome to remain usable for a new approval.

Authorize the run's frozen source before validating a cursor, reading a sequence watermark or filtering a page. Checking only returned rows leaks existence through an empty artifact-bound page. An otherwise-authorized legacy run without a frozen subject retains its empty-list behavior. Cached report IDs and cursors never confer permission.

R4/T4 use one repeatable-read snapshot with permission locks allowed. Do not call M1's locking observer from the generic `readCollaborationContent` read-only transaction. If a concurrent revocation invalidates the snapshot before the authority lock, discard the whole projection on the serialization error; never return partially assembled cases, logs or counts. A fresh denied T4 request returns only `404 {code:"not_found",error:"not_found"}`. If the read obtains the permission lock first, a competing revocation must wait or fail contention before committing.

QA's reason-only return remains available under its existing role/stage authority. Supplying a private report reference additionally requires current read authority, even when the report is failed or stale. This check does not turn a failed report into passing QA evidence.

### Client knowledge of source denial

A fresh authoritative R1 denial must clear/fence cached R4/T4 private evidence before publishing that denial. Do not wait for a second endpoint to return its own denial. Apply the existing workspace source generation to sibling issues and late responses; retain public identity and original request/recovery identity. A later positive read may restore access through fresh authorized requests, never by reviving an older response.

Derived code state, C1 test preparation and QA controls must respect that same known denial. Test comments use the existing protected mention-target `can_submit_commands` capability together with current comment/stage/review authority. Do not remove the separate global direct-command runtime gate to make C1 reachable.

### Client execution evidence identities

The three additive R4 execution arrays must be present together or omitted as an older unavailable response. Native external operation IDs are bounded UTF-8 strings, not server UUIDs. Pending, unknown and not-started operations are not execution facts. Source-denied responses require empty private arrays.

For 2c facts, require unique intent and approval IDs as well as unique fact IDs. Cross-check every overlapping operation/fact by approval, intent and fact identity. Checking only the first matching fact lets a later contradictory row escape validation; reject the entire response instead of choosing or dropping a row. Keep the old response's omitted execution group non-authorizing. A full 50-row operation page cannot prove an absent older approval is unconsumed.

Release derives its report from the selected current QA decision and reuses the protected T4 read to verify a matching non-null artifact. Rollback uses the current anomalous metric for the exact deployment and its immediately previous artifact. An open form losing this evidence keeps its draft but cannot submit. This client contract does not establish that a production deployment adapter exists.

### Migrations and evidence identity

Concurrent-index registration is part of the migration change, not optional cleanup. Put retained-history refusal in a higher-numbered readiness migration so down traversal refuses before dropping supporting indexes. Verify actual index shape/readiness; the ledger alone does not prove successful DDL.

A frozen verification manifest must include fixture adaptations caused by a public type change. For example, changing a child digest to a pointer also changes pre-existing admission/enqueue/scope tests. If a manifest omits them, preserve the failed reconstruction and add an explicit hash-bound supplement; do not silently read live implementation files into a historical check.

## 4. Validation and error matrix

| Condition | Required result |
| --- | --- |
| Source-readable child | `can_open:true`, hex64 digest; existing A5 checks still apply |
| Source-denied child | `can_open:false`, null digest; no private R4/T4 hydration |
| Nonempty cached A5 selection after source removal | `stage_scope_child_unavailable`; no scope decision |
| A5 holds its member fence before removal | The competing deletion cannot commit through that fence; serial completion is allowed |
| Empty selection by an otherwise-authorized external product approver | Remains allowed; no new blanket source-membership requirement |
| Original approval expires during a slow observation | `stage_decision_expired`; no stage advance |
| Original issuer is no longer valid | Existing signer/credential refusal; caller validity does not substitute |
| Caller expires before final consumption | `forbidden`; no stage advance |
| Rejoined source membership with an old approval | Old approval remains unusable; require a fresh signature |
| Source member without the frozen artifact's current authority | R4 withholds its source group; T4 list/detail deny without report/case/log/count data; QA rejects a supplied private report reference |
| Authorized artifact initiator/manager reading a superseded candidate's failed report | Original report stays readable; current applicability may be stale |
| Artifact-bound empty page or cached cursor after demotion | Original subject authority is checked before pagination; no existence disclosure |
| Demotion commits after the RR access snapshot but before its artifact lock | Serialization failure discards the complete response; a fresh request denies |
| QA return without private evidence | Existing reason-only role/stage behavior remains available |
| Fresh R1 denial with old readable R4/T4 cache | Private content disappears, C1/QA cannot use it, late responses remain fenced |
| Two terminal facts reuse an intent or approval, or a fact disagrees with its visible operation | R4 parsing refuses the entire contradictory response; A9 remains unavailable |
| New R4 execution group omitted or a bounded page cannot establish a dependency | Unknown/unavailable; no inferred execution success or unused approval |
| Retained stage history during downgrade | Refuse before any supporting index/ledger entry is removed |
| Missing concurrent-index cleanup registration | Invariant fails; scoped success does not turn a failing global inventory green |

## 5. Good / base / bad cases

Good: the workflow takes its existing member lock, performs the final access read, validates the selected child and commits while retaining that lock. The UI consumes denial from any authoritative stage projection through one existing source fence.

Base: unavailable or older response shapes remain unknown and non-authorizing; keep the permitted public task identity and original-key recovery.

Bad: check `access.SourceMember` once, perform other work, then record scope after a concurrent membership deletion. Another bad case is showing cached private report text because only R4, rather than the fresh R1, is allowed to report source loss.

## 6. Required regressions

- `TestStageScopeMemberFenceAtActualDecisionWrite`: use the canonical `DeleteMember` on a real second connection. Preserve the original double-commit failure. Scope-first must hold the lock (`55P03` in the bounded diagnostic); removal must succeed after scope commits. Revocation-first must refuse cached nonempty selection; retain the allowed empty external control.
- `TestStageSourceDeniedDigestAndCachedScopeSelection`: verify serialized null, preserved public identity and refusal of the previously readable digest.
- `TestStageAdvanceRechecksOriginalApprovalAtFinalClock`: move the DB-time seam only after earlier checks; original approval and issuer expiry must refuse, alongside an independently tested caller-expiry control. Assert the stage did not change.
- `TestStageCodeReviewSourceMembershipRejoinDoesNotReviveApproval`: retain old decisions and prove only a fresh signature works after rejoin.
- `TestStageArtifactReportReadAuthorityMatchesR4` and `TestStageArtifactReportQAReferenceUsesCurrentManager`: retain the original five incorrect-allow failures; initiator/manager positives, ordinary-member/demoted-manager refusals and reason-only return must remain distinct.
- `TestStageArtifactReportAuthorityLocksAcrossReadAndQA` and `TestStageArtifactReportRevocationBeforeAuthorityLockDiscardsSnapshot`: use a real second connection with canonical `UpdateMemberRole`, check `55P03` in the read-first order and content-free failure in the revoke-first order. The QA barrier must actually observe `CreateCollaborationStageReviewDecision`; an unhit test hook is not a product test.
- `TestStageArtifactReportCachedCursorDoesNotRestoreAuthority`, `TestStageArtifactReportEmptyListStillChecksFrozenSource`, and `TestStageTestReportUnboundRunKeepsAuthorizedEmptyList`: prove both protected pagination and the legacy empty-list control.
- `TestStageArtifactReportHTTPReadAuthorityEnvelope`: invoke the actual T4 adapters and assert `no-store`, exact error fields, and no report/case/log/count body on refusal. Fixture authentication is not an actual router/login journey.
- `TestStageConcurrentIndexCleanupInventory` plus `TestEveryConcurrentUpBuildHasCleanup`: retain both scoped and global results. Known unrelated failures must remain visible.
- Core source-cache/query tests and the `B01` cases in `stage-test-presentation.test.ts` and shared `stage-test-flow.test.tsx`: current R1 denial beats cached evidence, removes private text without sending a comment, fences late reads and permits fresh restoration. Preserve prepared text and exact subject/request identity.
- `packages/core/collaboration/stage-execution.test.ts`: preserve the three `C3C-C01` contradiction regressions (same intent/different approval, same approval/different intent, and a single mismatching fact for the visible operation's approval). Keep older-response compatibility, bounded-page uncertainty and original-key GET404 recovery controls.

Use the task-owned database/package isolation and race rules in [Database test isolation](./database-test-isolation.md). These regressions are not substitutes for the actual API/CLI/production Web/Desktop journey or real external integration evidence.

## 7. Wrong vs correct

Wrong: adding `if !access.SourceMember` only inside A5, or checking a decision's expiry before artifact observation.

Correct: join the existing source-member lock path, then recheck final authority at the real consumption boundary after observers.

Wrong: call `collaborationStageViewOptions` with a route workspace, or let a cached readable R4 override a newer R1 denial.

Correct:

```ts
collaborationStageViewOptions(queryClient, scope, issue.id, issue.source_workspace.id);
```

Pass the actual source identity, clear private evidence before the denial becomes observable, and preserve the original request separately from current evidence.

Wrong: check each visible report with the current candidate observer after paging, inside a read-only transaction.

Correct: enter `readCollaborationStageEvidence`, authorize the original frozen run source before cursor/page work, then validate each report against that subject. Use current-candidate applicability only to describe whether the historical evidence can support a new decision.
