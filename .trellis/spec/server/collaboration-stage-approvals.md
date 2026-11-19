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
- `collaborationStageEvidenceReadTx` marks only that private read transaction. `stageArtifactObservationAs` and `testReportFrozenSourceRead` select `evaluateDeliveryEvidenceReadInTx` and `readDeliveryArtifactInTx` only for this concrete transaction type. The public writer helpers retain their original signatures and exclusive defaults.
- `LockDeliveryEvidenceIssueRead`, `LockDeliveryArtifactPrincipalsRead`, `LockDeliveryArtifactApprovalRead`, and `LockDeliveryArtifactExecutionRead` in `queries/delivery_evidence_read.sql` retain the original predicates and return shapes with compatible `FOR SHARE ... NOWAIT` fences.
- The retained QA issuer branch of `stageExecutionApprovalChain` selects `readCompletionCollaborationAccess` and `stageQAReportReadAs` only for that concrete private read transaction. The former shares collaboration/session fences; the latter always locks the exact run with `FOR SHARE NOWAIT`, checks one row, and repeats the original scoped `GetCollaborationRecordedRun` before applicability checks.
- `completionCollaborationAccess` and `stageQAReportAs` pass constant false to their private mode helpers. A direct writer call with a private read marker still uses the original exclusive fences; the marker cannot authorize a writer to borrow the read variants.
- `testReportFrozenSourceRead(ctx, tx, access, frozen)` checks the original run subject's exact artifact through `readDeliveryArtifactInTx` for the private marked read transaction and the original `lockDeliveryArtifactInTx` for writer transactions. `testReportSourceRead` also verifies that a retained report agrees with that subject; QA references use the same caller transaction.
- `collaborationStageViewOptions(client, scope, issueId, workspaceId)` in `packages/core/collaboration/stage-queries.ts` requires the actual issue's `source_workspace.id`, not the current navigation workspace. It connects R1 to the existing private-evidence source fence.
- `CollaborationStageCodeCandidateSchema` in `packages/core/collaboration/stage-schema.ts` validates the bounded R4 execution group and overlapping intent/approval/fact identities before shared UI consumes it.
- `CollaborationStageBlockerSchema`, `collaborationStageRefusalStatus(code)` and `useCollaborationStageLabels().refusal(code)` jointly own known blocker decoding, stored-refusal HTTP status and bilingual display. Extend all three when a frozen R4/A7/A9 refusal is added.
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

Two authorized read projections of the same artifact must coexist. Mark the
transaction only at `readCollaborationStageEvidence`; preserve that same pgx
transaction through nested standing-signer and report observations. The marker
is a private concrete wrapper, not an HTTP field or context flag. Inherited
`Begin` returns an unmarked nested transaction. Current read chains do not open
nested transactions; a future change must verify that it preserves the intended
read or writer boundary.

The M1 artifact observer needs compatible shared fences at its four existing lock
points: the initial lifecycle issue, the task-principal issue join, the approval,
and the artifact execution. A valid standing QA/release chain also reaches the
retained issuer's collaboration/session fences and the exact report run fence.
All three must be shared for this read. Changing only an earlier fence leaves a
later exclusive upgrade. Keep the original task-principal joins and all
credential, member, workspace, signer, generation, artifact, receipt, report,
and applicability checks. The existing shared artifact-user locks remain in place.

For the run, keep the original id-only locking predicate and scoped recorded-run
getter. Do not replace the fence with `lock=false`, or introduce a session JOIN
or extra issue predicate merely to reuse another subsystem's query. The private
QA read entry always requests its shared fence; it does not expose an unlocked
variant. Shared row locks still conflict with current run updates.

`EvaluateDeliveryEvidenceInTx` and `lockDeliveryArtifactInTx` always select their
exclusive writer mode. The original completion access and QA report helpers do
the same even when explicitly given a marked transaction. Completion, QA,
advance, subject, artifact, token, and comment writers keep the original upfront
locks; they never acquire permission
from the private read marker. Replacing their issue lock with a shared lock can
allow simultaneous writers to attempt an unsafe lock upgrade. Do not weaken
writer locks, change RR isolation, add retries, use cached positive evidence
after a current error, or remove form locking to conceal a failed read.

For diagnosis, correlate the first actual pgx error with the same HTTP request,
principal, candidate, and backend PID. `55P03` from a read's mutation issue fence
is distinct from `40001` after real revocation. A QueryEnd or CommitEnd timestamp
only bounds an operation; it does not prove the exact database lock-release time
or identify a competing holder by itself. Use a deterministic two-connection
product regression to establish read/read compatibility.

QA's reason-only return remains available under its existing role/stage authority. Supplying a private report reference additionally requires current read authority, even when the report is failed or stale. This check does not turn a failed report into passing QA evidence.

The original issuer's GetByID completion-receipt read also uses compatible grant,
root, source-child and observer fences through the same borrowed transaction.
Its private read entries keep one validator body; original by-request/Create-key/
Revoke/source entries select constantfalse. Source `exact=false` controls version
exactness and still requires a row fence. Compare real failure UUIDs with the
receipt root and evidence child before attributing an issue lock. See
[Minimal authorization receipt reads](./delivery-completion-enforcement.md#minimal-authorization-receipt-reads)
for the13-name regression, 76-name service union, direct-Get handler
variants, current-authority/real-writer controls and retained fixture-history limits.

### Runtime capability GET and current account fences

**Scope and signature.** `EnsureCapabilities` reads `GET /api/collaboration/runtime/capabilities?runtime_id=...` before each dispatch. A matching registration requires no POST. `RuntimeCapabilities` makes no durable write, cursor advance or launch; its account fence must coexist with an authorized R4 read for the same account.

**Contract.** Only that GET calls `lockPreparationRuntimeAccount(..., true)`. The existing preparation and public `LockManagedRuntime` wrappers select `false` for registration, claim and other writers. The private account-read helper requires an existing PasswordSession, `Setup=false`, `Change=false`, positive version and valid user UUID. Lock exactly that user through `LockArtifactUsers` (`FOR SHARE NOWAIT`), require one matching ID, then call `CheckPasswordVersion` while holding the fence. Preserve current binding, namespace, runtime-owner and installation-admission checks and the original runtime-row lock.

**Validation.** Both capability-GET/R4 read orders must finish before the held reader is released. Wrong binding, revoked binding, disabled account and changed password remain non-authorizing and return no capability projection. A real registration writer keeps its original exclusive user fence; a conflicting R4 refuses with `55P03` and no partial evidence. Read failure is not permission to use stale cached authority.

**Good/base/bad.** Good: the current authorized GET uses the shared account fence and returns the unchanged registration/epoch with zero write effects. Base: unchanged writer callers retain their exclusive defaults. Bad: omit account locking, route registration through GET mode, or let a request field select its locking mode.

**Required tests.** `collaboration_runtime_capabilities_read_concurrency_test.go` uses actual same-QA managed/password credentials, artifact-bound R4, distinct PostgreSQL PIDs, real query barriers and the existing five-second bounds. Retain the changed-summary registration writer and current-authority controls, plus original registration no-op/epoch/metadata/unknown-commit tests in the combined race run. A changed test summary must respect `FileCount` 1–32. Count fixture runs using the persisted `collaboration_run.session_id`; task identity belongs to `collaboration_session.issue_id`. A setup error before the barriers does not establish read/read RED.

**Wrong versus correct.** A deterministic GET/R4 regression proves its own current read boundary. It does not identify the unobserved holder of an earlier runtime failure. Preserve failed and setup-blocked source/results separately, then require scoped RED/GREEN, writer/authority controls and matched product acceptance before new runtime acceptance.

Evidence: PAY-241 S2 `research/r22-capabilities-read-red-green-check-20261116.json` records accepted normal 4/8/7 and combined race 7/25/22, format/vet and matching products. Full S2 and the original R20 holder remain separate gates.

### Ordinary idle batch claims and current evidence reads

**1. Scope / trigger.** Ordinary daemon batch polling with no work must coexist
with an authorized stage evidence read by the same account. Nil or failed Redis
is a supported production fallback: the router still initializes both cache
factories, whose nil clients require PostgreSQL checks. Do not substitute a
test cache or change the QA permission fence to conceal idle claim contention.
This contract changes `ClaimTasksForRuntimes`; the singular claim path retains
its original behavior and needs a separate scope decision.

**2. Signatures.** `HasPotentialDispatchedReclaimForRuntimes(ctx,
HasPotentialDispatchedReclaimForRuntimesParams) (bool, error)` in `agent.sql`
checks the complete deduplicated runtime set and the original recovery window.
It uses `EXISTS` with dispatched/not-started, database-clock dispatch age and
absent/expired prepare-lease predicates. It has no row locks, authority joins,
`lock_issue_execution`, task payload or durable effect.

**3. Contract.** After the existing runtime validation and deferred handling,
run the probe when the reclaim schedule is due. Reliable false skips only the
Step2 reclaim transaction, then continues the original queued-work path. True
retains the exact `LockManagedRuntimes` exclusive account/binding fences,
original reclaim UPDATE and task-binding validation. The boolean never grants
authority. Keep all-runtime `MarkChecked` and retry/backstop timing after a
reliable negative or successful canonical check; a later eligible row must be
discoverable through normal polling and the existing backstop. Preserve actual
generation, lease, installation/admission and retained execution provenance.

**4. Validation / errors.** A probe error returns a wrapped error; it cannot
become empty success or proceed to a new claim. Initial validation remains
before an empty return. A true probe whose full reclaim predicates refuse the
task is still a real canonical check. Earlier deferred promotion may already
have effects, so do not promise whole-method rollback or whole-database zero
changes from a later probe failure.

**5. Good / base / bad.** Good: a valid no-work batch poll returns empty without
excluding same-account R4. Base: actual stale dispatched work still acquires
the original exclusive fence and preserves delivery history. Bad: using queued
candidate SELECT as an unlocked absence probe, sharing the real writer's user
fence, returning early before queued claims, or treating Redis/probe failure as
permission or reliable absence.

**6. Required tests.** `task_idle_claim_read_concurrency_test.go` owns the two
true-idle orders and two actual reclaim controls, with real services, borrowed
transactions, distinct PIDs and selected-runtime no-work prerequisites.
Reader-first must distinguish a real blocking `LockPasswordUser` cancellation
from NOWAIT `55P03`; preserve original bounds and snapshots after setup.
`TestOrdinaryIdlePreflightFailureDoesNotAuthorizeClaim` checks one exact
probe failure, original runtime/window arguments, no subsequent transaction or
claim and unchanged true-idle effects. Retain the original claim/binding/
admission controls, `TestManagedBatchClaimLostResponseRecovery` for dispatch
age/lease/started boundaries, and the complete-runtime/backstop Redis controls.
Those Redis tests flush DB12 and require an explicitly owned Redis server.

**7. Wrong vs correct.** Wrong: infer a missing router cache or an earlier
physical holder from nearby HTTP timestamps, then weaken an account lock.
Correct: prove the current request/query error, reproduce true-idle contention
on two connections, skip only a reliable necessary-condition negative, and
verify that actual reclaim writers keep their fences. PAY-241 R31 has a
six-name real RED and a 34-name normal/race service union, plus the temporal
handler and isolated Redis controls. These scoped checks do not close full S2
or identify the historical R30 holder.

### Completion candidate preparation reads

`ListCollaborationDeliveryCandidatesInTx` and its nested ordinary candidate read are pure preparation reads on the caller's actual `pgx.Tx`. Their private `lockCompletionPreviewRoot` and `lockDeliveryPreviewIssue` helpers are used only by candidate GET and pure preview POST entry points. Preserve the workspace, current user/credential, status catalog, root, source child and artifact lock order.

Use existing `LockDeliveryEvidenceIssueRead` for the root and both source-child preparations, `TryLockCollaborationStageRootShared` for the root collaboration row, and `readDeliveryArtifactInTx` for each selected artifact. Both issue and collaboration queries retain `FOR SHARE NOWAIT`; re-read current access after the root fences. The artifact helper keeps the existing shared principal-child, approval and execution fences with all canonical spec/receipt, current manager, producer, generation and expiry checks. No new read marker or public flag is needed for candidate enumeration.

Authorized R4/candidate reads in either order and same-child candidate siblings must finish before the held first transaction releases its locks. Real `EvaluateDeliveryEvidenceInTx` writers retain their exclusive defaults: writer-first denies the candidate without a partial page; reader-first denies the writer at its original lifecycle fence. Wrong source, outside participant, disabled/expired credential and revoked artifact manager remain hidden or non-authorizing under their existing distinctions.

`collaboration_completion_candidates_read_concurrency_test.go` is the canonical four-top/eleven-leaf regression. It uses existing real fixtures, distinct PostgreSQL PIDs and before/after rows for72 relations. Database metadata stability does not establish a new tar-byte audit when the fixture does not expose its archive path.

Good: select existing shared preparation helpers and preserve every authority predicate. Base: unchanged actual writer entry points retain exclusive locks. Bad: fix only the root lock, omit the nested child/approval/execution fences, or treat candidate GREEN as full preview acceptance. Completion preview has additional intent/session/attachment/stage/observer and managed-submission checks that require their own real regression.

Evidence: PAY-241 S2 `research/r26-candidates-read-green-independent-check-20261117.md/json` independently accepts normal/race each4top/11leaf/15named PASS and the two writer/five authority controls. This does not identify historical R24/R25 lock holders or close S2/A01–A12.

### Private stage-control conditional-version replies

**Scope and signatures.** When changing the test-only stage transport, check `parseStageControlReply(raw, expectedRequest)` and `StageControlInvocation` in `e2e/fixtures/collaboration-stage-control.ts` against the existing provider invocation contract. A successful rollback reply can carry the provider's updated state token.

**Contract.** `calls.last.conditional_version` is nonempty valid UTF-8 text, at most 256 bytes, with no NUL or lone surrogate. The controlled provider may return `"1/after/<intent-id>"`; preserve that exact text through the correlated reply. Reuse `text(..., 256)`. The separate candidate/native/configuration and initial controlled-source request versions retain the existing positive decimal signed-int64 validator, including its `9223372036854775807` maximum.

**Validation and errors.**

| Input | Required behavior |
| --- | --- |
| Correlated reply with a bounded opaque conditional token | Accept and preserve the exact invocation value |
| Empty, null, missing, non-string, NUL, lone surrogate or over-256-byte token | Refuse the reply; never replay the request |
| Wrong envelope, duplicate key, mismatched request identity or invalid digest/count | Retain the original strict refusal |
| Opaque text supplied to a decimal candidate/native/request version | Retain `Invalid stage control version` |

**Good/base/bad.** Good: decode the exact provider state using the existing bounded text helper. Base: ordinary decimal tokens remain valid text in this reply field. Bad: reuse a numeric protocol-version validator for a provider state token, loosen every version field, or replay a committed rollback because its reply was rejected.

**Required tests.** The owning `.node-test.mjs` suite must preserve an actual opaque rollback reply as offline data and prove a fresh synthetic fake-socket request sends one frame and confirms one correlated reply. Cover the 256-byte UTF-8 boundary, invalid token states and unchanged decimal request controls alongside envelope, frame, timeout and disconnection cases. Preserve genuine original-code RED and matched GREEN/type/lint evidence; a private copy needs the actual package context for dependency lint.

**Wrong versus correct.** `version(last.conditional_version)` rejects a valid provider reply after the backend has committed. `text(last.conditional_version, 256)` validates its actual contract. Keep the 20-second timer, 64 KiB frame limit and no-replay behavior; code acceptance does not relabel the retained failed runtime journey as passed.

### Client knowledge of source denial

A fresh authoritative R1 denial must clear/fence cached R4/T4 private evidence before publishing that denial. Do not wait for a second endpoint to return its own denial. Apply the existing workspace source generation to sibling issues and late responses; retain public identity and original request/recovery identity. A later positive read may restore access through fresh authorized requests, never by reviving an older response.

Derived code state, C1 test preparation and QA controls must respect that same known denial. Test comments use the existing protected mention-target `can_submit_commands` capability together with current comment/stage/review authority. Do not remove the separate global direct-command runtime gate to make C1 reachable.

`StageCodePanel` must also show its existing source-denial status from
`collaborationStageSourceDenied(view)`, independently of a private R4 record.
R1 can clear that record and supersede a later anonymous R4 response. Render the
explanation once outside the `record` fragment; keep the anonymous R4 denial
case and generic query error/Refresh UI. Pending, unsupported or failed R4 by
itself does not establish source denial. A known R1 denial hides source details,
test reports, decisions and approval actions while the explanation remains visible.

The canonical component regression starts with readable private R4 evidence,
refetches R1 with `child.can_open === false` and null private digests, and leaves
replacement R4 pending, unavailable or failed. All three states must retain the
localized status without restoring private data or submitting a write. Readable
R1 with pending/failed R4 must keep its ordinary loading/error state. Coverage
lives in `packages/views/collaboration/stage-code-panel.test.tsx`; workspace-wide
cache-generation and late-response matrices remain in the existing Core tests.

### Client execution evidence identities

The three additive R4 execution arrays must be present together or omitted as an older unavailable response. Native external operation IDs are bounded UTF-8 strings, not server UUIDs. Pending, unknown and not-started operations are not execution facts. Source-denied responses require empty private arrays.

For 2c facts, require unique intent and approval IDs as well as unique fact IDs. Cross-check every overlapping operation/fact by approval, intent and fact identity. Checking only the first matching fact lets a later contradictory row escape validation; reject the entire response instead of choosing or dropping a row. Keep the old response's omitted execution group non-authorizing. A full 50-row operation page cannot prove an absent older approval is unconsumed.

Release derives its report from the selected current QA decision and reuses the protected T4 read to verify a matching non-null artifact. Rollback uses the current anomalous metric for the exact deployment and its immediately previous artifact. An open form losing this evidence keeps its draft but cannot submit. This client contract does not establish that a production deployment adapter exists.

### Known execution blockers and current-stage actions

The frozen R4/A7/A9 execution codes are `stage_execution_unavailable`,
`stage_execution_target_changed`, `stage_execution_pending`,
`stage_execution_conflict`, `stage_rollback_observation_unavailable` and
`stage_rollback_observation_changed`. Recognize these in both `blockerCodes` and
`STORED_REFUSALS` in Core, and in the shared explicit label switch and both locale
catalogs. Their stored refusal status is 409; original-key recovery may return
HTTP200 carrying the retained `status:409` receipt. A contradictory stored status
must remain malformed.

R4 projects all four signable steps even during development. A current candidate
with matching passed CI and no bound build legitimately includes unavailable
release/rollback blockers. Recognizing those reasons does not authorize those
steps. It permits the existing current-stage authority calculation to offer
`code_review` to the source-authorized business owner. Mapping an already-known
future-step blocker to `unknown` makes the complete code-state projection fail
closed and hides that valid current action. Keep genuinely unfamiliar codes
mapped to `unknown`; do not weaken the derivation's unknown-state check or add
dummy builds/targets to make the control visible.

Human workflow observations are a separate request shape from Host execution
observations. The participant/stage/lifecycle request contains only
`expected_collaboration_revision`, `expected_lifecycle_epoch`,
`expected_participant_epoch` and `expected_account_epoch`, plus action-specific
fields. The shared E2E `workflowAuthority` selects these four fields explicitly;
`hostAuthority` retains `execution_view_version:6` for its resource/execution
consumers. Spreading the entire Host shape into a strict human workflow request
correctly produces HTTP400 `invalid_request` before an operation is recorded.

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
| Two authorized R4 reads, or R4 and T4 reads, share the same current artifact | Both complete using compatible shared read fences and the unchanged authority checks |
| Two authorized R4 reads validate a standing QA/release chain with an unknown intent | Retained issuer and exact-run shared fences coexist; both return the same immutable intent identity and unknown state |
| An original completion or QA writer is directly given a private read marker | Its constant-false writer entry retains the original exclusive locks |
| A real R4 holds its exact-run shared fence | The original run writer fails contention until the read commits, then can acquire its original lock |
| A real writer holds the original issue fence before a read | The read may fail contention with no private projection; do not retry automatically |
| A read holds its shared artifact/authority fences before a real writer | The writer retains its original upfront refusal and cannot silently upgrade or bypass the read fence |
| QA return without private evidence | Existing reason-only role/stage behavior remains available |
| Fresh R1 denial with old readable R4/T4 cache | Private content disappears, C1/QA cannot use it, late responses remain fenced |
| Two terminal facts reuse an intent or approval, or a fact disagrees with its visible operation | R4 parsing refuses the entire contradictory response; A9 remains unavailable |
| New R4 execution group omitted or a bounded page cannot establish a dependency | Unknown/unavailable; no inferred execution success or unused approval |
| Current development candidate, matching passed CI and known unavailable future execution blockers | Existing business-owner `code_review` move remains available; release/rollback approvals remain unavailable |
| Genuinely unknown blocker in any signable step | Preserve unknown state and withhold code actions |
| Known execution refusal recovered by original key | HTTP200 with retained `status:409` remains a final refusal; mismatched status is malformed |
| Host `execution_view_version` sent in a human workflow body | HTTP400 `invalid_request`; do not loosen the strict decoder |
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
- `TestStageArtifactReadProjectionConcurrentR4` and `TestStageArtifactReadProjectionConcurrentR4T4`: retain the old-source `55P03` failures, hold the first real read after its issue/authority fences, and finish the second authorized product read on a different PostgreSQL PID before releasing the first. Both must succeed after the read-only fix.
- `TestStageArtifactReadProjectionWriterFences` and `TestStageArtifactReadProjectionRevocation`: keep writer-first and reader-first refusal controls, actual current-manager demotion, and the RR revocation-first `40001` case. Failed reads must return zero private projection; public writer helper defaults and original test assertions remain unchanged.
- `TestStageExecutionReadStandingApproval`: construct a valid QA/release chain and actual unknown intent, observe the two retained-issuer locks, then hold the first real R4 at `ListStageExecutionIntents`. A different authenticated reader on a different PostgreSQL PID must finish the exact R4 projection. Empty review heads do not exercise this branch; retain its original `55P03` failure and the later run-fence failure.
- `TestStageExecutionReadWriterAccessFences` and `TestStageExecutionReadRetainedIssuerFences`: preserve plain/marked direct completion writers on both rows, current QA issuer mutation, and the RR revocation-before-authority-lock case. Reuse unchanged accepted controls by their source/raw binding rather than repeating them without a changed dependency.
- `TestStageExecutionRunReadFences`: preserve plain/marked original QA writers against an isolated run shared lock; a writer-first run lock makes actual R4 fail with zero projection, and a reader-held actual R4 prevents the original run writer until that read commits. Do not invent a run-fact invalidation assertion for a field the applicability code does not read.
- `TestStageConcurrentIndexCleanupInventory` plus `TestEveryConcurrentUpBuildHasCleanup`: retain both scoped and global results. Known unrelated failures must remain visible.
- Core source-cache/query tests and the `B01` cases in `stage-test-presentation.test.ts` and shared `stage-test-flow.test.tsx`: current R1 denial beats cached evidence, removes private text without sending a comment, fences late reads and permits fresh restoration. Preserve prepared text and exact subject/request identity.
- `packages/core/collaboration/stage-execution.test.ts`: preserve the three `C3C-C01` contradiction regressions (same intent/different approval, same approval/different intent, and a single mismatching fact for the visible operation's approval). Keep older-response compatibility, bounded-page uncertainty and original-key GET404 recovery controls.
- `stage-code-presentation.test.ts` composes R4 parsing with the real derivation: current development/CI, no artifact, and the actual four blocked signable-step shapes must offer only the business owner's `code_review` move. A genuinely unknown blocker, missing caller authority and an unrelated actor remain non-authorizing; recognized blockers never grant release or rollback.
- `stage-schema.test.ts` checks all six frozen execution/rollback codes as blockers and stored409 receipts, including HTTP200 original-key recovery and contradictory retained-status refusal. `packages/views/collaboration/stage-labels.test.ts` checks both actual locale catalogs and the unknown-reason fallback.

Use the task-owned database/package isolation and race rules in [Database test isolation](./database-test-isolation.md). These regressions are not substitutes for the actual API/CLI/production Web/Desktop journey or real external integration evidence.

## 7. Wrong vs correct

### Post-QA machine refusal regression

`collaboration_stage_qa_protection_test.go` in service owns the exact missing
post-decision stimulus: create a passing settled report, record a current human
QA return or approval, then attempt the opposite decision and revocation as a
task principal and as a human credential marked AgentActor. Preserve the exact
decision, derived review head, expiry/dependencies, original human receipt and
all issue/runtime/event/notification facts after each refusal. A distinct
successful human opposite decision proves the request was otherwise usable;
it must leave the original decision immutable. Normalize only read-time
`CheckedAt` and signability horizons when comparing projections.

The router companion uses the real production router and actual task claim to
obtain generic and restricted `mat_` credentials. First prove the same token's
supported ordinary issue GET succeeds. Generic task identity and the separate
JWT-plus-Agent-hint control must hit the human-only `403 forbidden`; restricted
capabilities stop at their closed-route guard. The router's QA-return fixture
has no runtime report, so its opposite-approval request proves authentication
ordering only; a human control reaches `409 stage_test_report_unusable` and the
service test separately proves full approval admissibility. Do not describe
that router case as a fresh approved-report or browser journey.

Wrong: send an `mcb_` runtime token to the generic human route and count its
authentication failure as an authenticated Agent denial. Correct: bind a real
issued `mat_` to its task/Agent/workspace/user, retain the positive supported
read and exact post-QA write refusal. Never print the credential itself.

The existing service fixture unconditionally clears `creation_enabled` during
cleanup. A task-owned verification run must retain its actual after-state and
restore the recorded original flag explicitly without rewriting the epoch or
deleting retained invalidation records. Business no-effect assertions belong
around each denied request, not to final cleanup row counts.

Wrong: synchronize a new execution blocker only in the schema list, leave its
stored-status or locale mapping unknown, or weaken the complete action-state
check to bypass it. Another wrong fix is carrying Host execution fields into a
strict human-workflow body.

Correct: synchronize the frozen blocker, retained-refusal status and shared
bilingual label together. Exercise the complete R4 parser-to-action path with
known future-step blockers and a genuinely unknown control. Select only the four
human-workflow observations for that request shape.

Wrong: adding `if !access.SourceMember` only inside A5, or checking a decision's expiry before artifact observation.

Correct: join the existing source-member lock path, then recheck final authority at the real consumption boundary after observers.

Wrong: call `collaborationStageViewOptions` with a route workspace, or let a cached readable R4 override a newer R1 denial.

Wrong: call the mutation artifact observer from R4/T4 without an explicit read
boundary, or change only its first issue lock while the task-principal,
approval, or execution read still takes an exclusive fence.

Wrong: validate read/read concurrency with an empty review chain, then assume
the retained QA issuer and run locks are also compatible once release is approved.

Correct: use the private read transaction to select compatible artifact,
retained-issuer and exact-run fences, preserve every authority predicate and same-transaction snapshot, and
keep public writer helpers on their original exclusive mode. Verify read/read,
reader/writer, and revocation behavior on two real connections before the
unchanged original Web/Desktop scenarios.

Correct:

```ts
collaborationStageViewOptions(queryClient, scope, issue.id, issue.source_workspace.id);
```

Pass the actual source identity, clear private evidence before the denial becomes observable, and preserve the original request separately from current evidence.

Wrong: check each visible report with the current candidate observer after paging, inside a read-only transaction.

Correct: enter `readCollaborationStageEvidence`, authorize the original frozen run source before cursor/page work, then validate each report against that subject. Use current-candidate applicability only to describe whether the historical evidence can support a new decision.
