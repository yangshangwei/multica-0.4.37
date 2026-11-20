# Research: Conditional M1 candidate/R4 read-coexistence test plan

- Query: Which real tests and opt-in read boundaries are required if the candidate409 is caused by concurrent read locks?
- Scope: internal; production service/SQL/handler and existing real fixtures. Only this report and its JSON were written. No tests, source edits, API, SQL, builds, runtime actions or Git operations.
- Date: 2026-11-17, Asia/Shanghai.
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

### Current gate

This is an executable **test plan**, not a RED result or product-repair permit. R24 candidates409 lacks its original body, SQLSTATE and holder. Main reports that R25 stopped earlier at QA: **1 PASS /1 FAIL /2 NOT_RUN**, result `4e4b2718edd844f086d77edd7bac0654e4707ab0b8549e835f78aae705ea3030`. Its fourth case and candidate capture were not reached. Do not repeat random full-four runs to manufacture a lock conclusion. A deterministic owning-layer regression can prove an independent reachable read boundary; it still does not identify either historical holder.

### Initial ownership and inventory

Initial write scope: **one new service test file only**, proposed `server/internal/service/collaboration_completion_candidates_read_concurrency_test.go`, in main's new bound private source. Existing helpers are reused; product/SQL/generated/handler files remain unchanged for RED. Main owns fresh task-specific `./internal/service` databases, normal/race allocation and source/command permissions.

| Proposed top-level test | Exact required leaf identities |
| --- | --- |
| `TestCollaborationCompletionCandidatesR4Coexistence` | `r4_first`, `candidates_first` |
| `TestCollaborationCompletionCandidatesSiblingReads` | `same_creator`, `different_current_managers` |
| `TestCollaborationCompletionCandidatesWriterFences` | `writer_first`, `reader_first` |
| `TestCollaborationCompletionCandidatesCurrentAuthority` | `wrong_workspace_child`, `outside_participant`, `disabled_account`, `expired_credential`, `revoked_artifact_manager` |

Inventory is **4 top-level /11 leaf cases**. Top-level + child records are observations, not15 additional business cases. Name/leaf inventories must be captured before execution and matched in RED/GREEN; no skip counts as proof. Same-child sibling reads catch approval/execution exclusives left after a root-only repair.

### Exact fixture, actor and credential

Use `stageArtifactConcurrentReadFixture(t)` (`collaboration_stage_evidence_read_concurrency_test.go:103`), which calls `newStageReviewFixture`, `stageArtifactReadFixture`, actual code-candidate creation and actual stage advance to code_review. It gives:

- root: `f.issue.Issue.ID`, source workspace: `util.MustParseUUID(f.workspace)`, project: `util.MustParseUUID(f.project)`;
- ordinary child: `util.MustParseUUID(f.child)`, actual parent/root and same project/workspace; no fabricated collaboration child;
- candidate: returned exact `CollaborationStageCandidateRef`, artifact approval: `f.artifact.ApprovalID`;
- creator `f.creator` with `f.ctx`: actual source owner, actual original artifact initiator/approver/runtime owner;
- reviewer `f.reviewer` with `f.reviewerCtx`: explicit current participant, current source member and artifact manager/admin. Distinct from creator;
- ordinary candidate reader: `f.ctx` and `completionRequestCredential(f.ctx)`. Same-actor control uses that same current creator on two transactions; different-manager control uses reviewer with their own exact credential;
- `org := pgtype.UUID{}`: normal human path. Do not supply an invented organization or administrator to bypass access. R38 requires its existing separate real fixture and password/admin controls;
- requested page: `limit=20`, `cursor=""`, current root/source/approval identities. First make a real unheld baseline read and require exactly the actual selectable approval, valid digests/receipt, root/child numeric revisions and no private-count invention.

Use `stageExitServiceM1(t, f.collaborationStageFixture, "enforce")` to seed only the same existing test policy row when the candidate test should exercise enforced schema readiness. It is fixture setup, not an acceptance action. The concurrent methods then perform every real read/authority/schema check. Do not change stage roles, turn off enforcement or mutate result/receipt metadata to obtain a baseline.

The base fixture is legacy-auth test context: real JWT principal with finite expiry, backed by actual current user/member rows. If the package selects password mode, use a real test-created `user_password_credential` and `auth.WithPasswordSession` with matching stored version/finite expiry, or the established managed fixture; a JWT without password context is invalid. Never derive credential snapshots from display labels or use `stageCredential` without matching context. Existing router password/session/revocation tests remain required controls and need no duplicate full matrix here.

### Borrowed transaction and both read orders

All candidate calls use actual `pool.Begin(ctx)` and **that same pgx.Tx**. The service neither begins nor commits a candidate call itself. Match handler default transaction isolation; do not switch it to REPEATABLE READ to resemble R4. `PrepareArtifactTransaction` retains its own250ms lock timeout/schema guard. Each reader has its own `context.WithTimeout(actualActorCtx, 5*time.Second)`; no SQL READ ONLY because current-authority row locks are required.

**R4 first**:

1. Capture baseline candidate page and baseline R4 while no transaction is held.
2. Call existing `holdStageArtifactR4(t,f,candidate)`. Its SQL-forwarding `stageProjectionBarrierStarter` pauses before actual `ListCollaborationCandidateTestReportEvents`, **after** the R4 artifact/current-authority locks. Keep its returned backend PID and release channel.
3. Begin candidate reader on another actual connection; record `tx.Conn().PgConn().PID()`, require nonzero and unequal to held R4 PID.
4. Call actual `f.svc.ListCollaborationDeliveryCandidatesInTx(ctx, tx, root, child, pgtype.UUID{}, completionRequestCredential(ctx),20,"")`. Require successful complete baseline-equivalent page **before releasing R4**. Capture any first PGerror in a SQL-forwarding test wrapper; service `deliveryCompletionError` can otherwise erase SQLSTATE into a domain409.
5. On success perform final `CheckArtifactCredentialInTx` and real `tx.Commit(ctx)` before releasing the held R4. On error roll back. Always release/cancel/drain via `sync.Once` cleanup even on assertion failure. Then require original held R4 success and exact baseline/artifact ID/digests.

**Candidates first**:

1. Begin actual candidate transaction, record PID and call the method. Complete actual candidate read plus final credential check, but **do not commit**. A service return leaves its real locks held; this is the barrier and requires no fake row/read or new production hook.
2. On a second connection invoke actual `StageCodeEvidence` through the existing `stageProjectionBarrierStarter{TxStarter:f.pool,began:...}`; require a distinct PID and success while candidate transaction still holds its locks. Validate exact currently readable baseline/artifact/decisions.
3. Only after the second reader terminal, commit/release the first; then check original page identity, no write effects and complete cleanup. Releasing first before the second returns would not prove coexistence.

Sibling controls similarly hold a completed first candidate transaction before starting a second candidate read. They catch exclusive approval/execution rows or root collaboration/session fences that an R4-first-only test may never reach. The current original may reject at an early fence; capture the first failure but do not guess deeper failures were executed.

### No-write evidence and real writer controls

Reuse `stageExitServiceSnapshot(t,f.collaborationStageFixture)` for root/child ordinary statuses/revisions/parent/project, root collaboration/session epochs/queue/control/event_seq, ordinary queue states, events, handoffs, stage submissions, attachment binding and project adjustments. Also capture the fixture-specific rows/counters for:

- `collaboration_operation`, notification/outbox, stage task/role/scope/candidate/decision/revocation;
- `delivery_completion_authorization` and `delivery_completion_terminal`;
- actual `agent_approval_request` and `agent_artifact_execution` canonical spec/request/receipt digests, claim/run/generation/status/output metadata;
- `collaboration_run`, execution intents/observations/metrics, original gate state and active reservation/lease facts when present.

Compare exact IDs/rows and counts before/after both readers, not only status or one zero count. Queries for counters stay test setup/verification, never substitute a pool read for production's borrowed transaction. No grants/notifications/commands, status/generation changes or packager invocation are allowed during either read.

`stageArtifactReadFixture` makes a real tar but does **not expose its output path** in `stageReviewFixture`; do not guess a test temp directory from runtime naming or treat DB digest stability as a fresh tar-byte check. Initial one-file RED can validate the exact retained execution/spec/receipt/output metadata and reuse the original packager fixture evidence. A new tar before/after assertion needs main to explicitly widen test-only fixture output exposure, or retain the existing router fixture's known `archive()` check. It is not justification to clone a second artifact setup or claim an unperformed filesystem audit.

**Writer-first**: reuse `stageProjectionWriterRequest(t,f)`, begin a real separate writer and call unchanged `EvaluateDeliveryEvidenceInTx` until its actual observation is pass. This retains the writer's exclusive child/principal/approval/execution fences. Candidate read must refuse content-free, return no partial page and finish before writer release. Roll back the real writer; fresh candidate read must recover and all rows remain baseline.

**Reader-first**: hold actual successful candidate read uncommitted, then call unchanged `EvaluateDeliveryEvidenceInTx` on another actual writer connection. It must retain its original upfront exclusive `LockLifecycleIssue` refusal before downstream authority/effects. Use the existing SQL-forwarding barrier and a marker on `LockArtifactUsers` to show the rejected writer did not advance beyond its first fence. Roll back writer, commit reader, compare all effects. These are real existing observer locks; no hand-written lock row pretending to be the product writer.

Current-authority controls must distinguish: wrong workspace/parent/project source or outside participant is hidden/denied, expired/disabled current human has no page, revoked reviewer manager cannot recover that artifact (empty authorized page or defined denial, never selectable/private metadata). Capture original responses/domain distinctions instead of requiring every refusal be409. After a changed authority control establish its own baseline so expected setup mutation is not mistaken for a read write.

### Complete lock inventory: candidate GET

| Call/SQL | Current lock | Consideration for opt-in pure read |
| --- | --- | --- |
| `lockCompletionPreviewRoot:98` root `LockLifecycleIssue` | **NO KEY UPDATE NOWAIT** | Must coexist with another current reader; root writer still excludes it |
| same helper `:101` `LockIssueCollaboration` | **UPDATE**, blocking until lock_timeout | Root metadata fence; two candidates also contend |
| same helper `:111` child `LockLifecycleIssue` | **NO KEY UPDATE NOWAIT** | Conflicts with R4's shared artifact child fence |
| nested `lockDeliveryPreviewIssue:158` | repeated **NO KEY UPDATE NOWAIT** on child | Root-only shared replacement leaves this exclusive path |
| `lockDeliveryArtifactInTx` → `LockTaskCapabilityPrincipals` | **NO KEY UPDATE OF child issue**, SHARE OF queue/agent/runtime/member/users | Replace only projection's issue part using existing `LockDeliveryArtifactPrincipalsRead`; all principal checks remain |
| same nested artifact helper `LockAgentApprovalRequest` | **UPDATE NOWAIT** | Existing `LockDeliveryArtifactApprovalRead` is SHARE NOWAIT |
| same helper `LockAgentArtifactExecutionByApproval` | **UPDATE NOWAIT** | Existing `LockDeliveryArtifactExecutionRead` is SHARE NOWAIT |
| workspace `TryLockCollaborationAuthorityWorkspaces` | KEY SHARE NOWAIT | Already compatible; keep current workspace fence |
| users/member, PAT/password credentials, invocation targets/agent | SHARE NOWAIT | Already compatible; keep non-key role/session revocation checks |
| `LockDeliveryProjectPolicy` → `LockProjectForAssociationNowait` | **SHARE NOWAIT** | Despite its name this is not exclusive; do not weaken it |
| schema predicates/candidate scan/final clock | reads; bounded200-row scan, current deadline | Keep readiness, visible-set/cursor/privacy, final expiry checks |

The source exposes **seven exclusive acquisitions** across five conceptual row groups (root issue/collaboration, child issue with repeats, approval, execution). Exact query-name/source-frame capture decides the reached failure; table locks and clock are not evidence of a row holder.

### Caller map and eventual product write boundary

`lockCompletionPreviewRoot` has exactly two callers: collaboration candidate GET and collaboration completion-preview POST. `lockDeliveryPreviewIssue` has exactly two callers: ordinary candidate GET and ordinary completion-preview POST; collaboration candidate GET nests through the ordinary candidate method. All four are **pure reads despite previewPOST**. Handler adapters own Begin/final credential/Commit, five-second deadline and response. No actual final completion/authorization writer calls these two private root helpers.

Shared dependencies do have writers:

- `EvaluateDeliveryEvidenceInTx` is used by authorization creation/recovery, actual completion prepare/finish, stage writes and ordinary policy observation. Its original exclusive behavior must remain the default; use existing `evaluateDeliveryEvidenceReadInTx` only for selected pure projections.
- `lockDeliveryArtifactInTx` defaults to original task/approval/execution locks. Existing `readDeliveryArtifactInTx`/`lockDeliveryArtifactWithReadMode(...,true)` already preserves canonical spec/digest/generation/current principals while selecting shared SQL; do not change the default.
- `completionCollaborationAccess` and `completionAdminNomination` are used by creator/submission/self/R38 actual writers, grant creation/recheck and final receipts; only explicit read counterparts may change lock modes.
- `stageCompletionFacts` and `stageCompletionState/loadStageDetails(...,true)` are also actual final-writer gates; their default task/child locks cannot be globally made shared.

**After genuine owning-layer RED**, the smallest candidate-only product slice can initially be limited to `collaboration_completion_preview.go` (pure read root fence/marker) and `delivery_completion_preview.go` (nested pure child/artifact projection dispatch), using the already existing shared SQL/read helpers. No migration/dependency/new abstraction is needed for the candidate child/artifact fences. Preserve all original error/authority/source/expiry logic. A private unexported marker may reuse `collaborationStageEvidenceReadTx` only when its creation is confined to exact pure read entry points; nested real writers must receive the unmarked transaction. Ensure wrappers do not lose marker identity. A new externally supplied boolean, HTTP marker or persistent read token is not acceptable.

Root collaboration metadata needs a real shared row fence, e.g. existing `TryLockCollaborationStageRootShared` or the exact existing `readCompletionCollaborationAccess` path. It must still re-read current authoritative root facts. Removing the root lock entirely or selecting just its lifecycle while failing to recheck current access is insufficient. Exact choice belongs to the implementation owner after RED, not this plan's license.

### Preview POST has extra exclusive paths

Do not call candidate-only GREEN a completed M1 preview repair. Ordinary preview nests `EvaluateDeliveryEvidenceInTx` (exclusive issue/task/approval/execution), requiring its explicit read observer if in scope. Collaboration preview additionally traverses:

- `completionCollaborationIntent` → normal `completionCollaborationAccess`: root issue_collaboration and collaboration_session **UPDATE NOWAIT**; attachments in `completionIntentDigest` and self validation use `LockCollaborationAttachments` **UPDATE**;
- R38 `completionAdminNomination`: global **exclusive transaction advisory lock** `70856394211471801`, original root collaboration/session UPDATE and real admin/password/organization fences; it is an independent admin lane, not authorization to disable global serialization;
- `stageCompletionFacts` → `stageCompletionState/loadStageDetails(...,true)`: stage task **UPDATE**, child lifecycle NO KEY UPDATE; QA report's default run lock `LockCollaborationStageTestRun` **UPDATE**; retained artifact issuer checks default to exclusive unless the accepted exact read marker follows every branch;
- PR/link/check/integration `LOCK TABLE ... IN SHARE MODE NOWAIT` is already shared, as are policy/project rows, stage candidate/submission/operation/handoff snapshots and final signer member/PAT credentials; retain them.

Existing `collaborationStageEvidenceReadTx` already switches artifact source/observer, QA report run and issuer access read branches. It does **not by itself** fix `stageCompletionState`'s task lock, explicit child lifecycle, normal creator intent, attachments or R38 nomination/advisory. Preview extension may require `delivery_completion_collaboration.go`, `collaboration_stage_completion.go` and existing marker-aware paths, plus same-layer tests, but must be a separately bounded proposal after a real preview witness. Do not silently widen initial candidate RED ownership.

One separate static preview issue is recorded for follow-up: managed submission preview initializes `stageMutationGuard` without `boundSubmission` and calls `checkSubmission(nonzeroID)`, whose first predicate requires that bound ID. It statically yields `stage_submission_unbound` under that branch. This was not reached by R24/R25's candidate failure and needs its own real managed-submission regression; it must not be bundled into a speculative locking fix.

### Exact validation command shapes

Main allocates each profile/source and substitutes actual known paths; placeholders below are not approval to choose a database. Keep the same four-test selector:

```text
^(TestCollaborationCompletionCandidatesR4Coexistence|TestCollaborationCompletionCandidatesSiblingReads|TestCollaborationCompletionCandidatesWriterFences|TestCollaborationCompletionCandidatesCurrentAuthority)$
```

1. Original product + new test: normal JSON `go test -json -count=1 -timeout=10m ./internal/service -run '<selector>'`, exact fresh RED profile. Record every attempted leaf, first real PGerror/domain result and cleanup.
2. Accepted private repair + exact same test source: normal same command/selector, fresh GREEN profile.
3. Same GREEN + exact same source: `go test -race -json -count=1 -timeout=10m ./internal/service -run '<selector>'`, separate fresh race profile.
4. Focused unchanged read/writer regression selection, separately source/profile bound: `^(TestStageArtifactReadProjectionConcurrentR4|TestStageArtifactReadProjectionConcurrentR4T4|TestStageArtifactReadProjectionWriterFences|TestCollaborationWorkflowReceiptReadOriginalWriterFence)$`; reuse previously accepted results only if exact selected bytes match, otherwise run the necessary affected controls. Ordinary/preview/R38 full tests are separate if those boundaries are changed.

All commands use the existing task-owned runner, explicit package DB identity/private HOME/TMP/caches, ordinary pool defaults and offline installed dependencies. The researcher has executed **zero** commands from this plan. No broad old A/W/Q replay is required by a candidate-only test proof.

## Files found and related specs

Source-backed details above come from `collaboration_completion_preview.go`, `delivery_completion_preview.go`, `delivery_evidence.go`, `artifact_authority.go`, `delivery_completion_collaboration.go`, `collaboration_stage_completion.go`, `collaboration_stage_evidence_read.go`, `collaboration_stage_execution_approvals.go`, `collaboration_stage_qa.go`, actual SQL under `server/pkg/db/queries/`, and existing stage artifact/exit/completion test fixtures. See JSON for exact source references/hashes.

Applicable specs: `.trellis/spec/server/delivery-completion-enforcement.md`, `collaboration-stage-approvals.md`, `database-test-isolation.md`, `verification-environment.md`. No external docs or accounts were accessed.

## Caveats / Not Found

- No executed RED/GREEN, matching original409 body/SQLSTATE/holder, or proven R25 M1 failure. R25 failed earlier; original R24 and every historical result remain unchanged.
- Legacy finite JWT fixture proves actual stored source/current actor checks; password/R38 uses separate real session/admin fixtures. No fabricated org/account authority.
- Candidate-only proof is not full preview, every writer, A01–A12, parent24, S3–S5 or actual Web/Desktop acceptance.
- Initial one-file test scope does not expose a tar path; no fresh filesystem-byte audit can be claimed from retained database digests alone.
- This researcher wrote only the two new task-local research artifacts. Any product repair requires main's genuine RED and explicit bounded implementation dispatch.
