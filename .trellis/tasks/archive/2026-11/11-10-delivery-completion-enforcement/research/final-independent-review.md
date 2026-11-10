# Final independent review: transactional completion v1

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`. Baseline: `1ce4e81223be3397fe15e2649debfac40720ddd9`.
Reviewed source: the 110-file `final-source-manifest.json` worktree freeze at
HEAD `fb0558881a973fa4018897a91c00aa656eab6d5a`; the uncommitted implementation
is bound by file hashes, not misrepresented as already tested at a future commit.

## Verdict

**PASS_V1_TRANSACTIONAL_COMPLETION_ENFORCEMENT.** The accepted v1 contract is
implemented and supported by the selected actual transaction, authentication,
signature, artifact, concurrency, migration, client and retention evidence.
No actionable defect remains within the reviewed scope. The main session may
commit and safely integrate this reviewed batch after its staged scope check.

This verdict completes the v1 delivery-gate child contract only. It does not
complete M1, M2/M3 or the six businesses. Usable skip is unavailable by the
accepted v1 empty-waiver decision. Local synthetic execution, database-observed
PR head, no deployment/provider/browser/performance/full-repository acceptance
and all historical supplier stop/budget limits remain explicit.

## Final-pass findings and corrections

- Batch binding parsing reused iteration CanonicalHash, which rejects revision
  zero. A real authorization for revision 0 succeeded, but its batch completion
  returned 400. The narrow fix uses strict JSON validation followed by the
  completion binding parser, preserving the approved nonnegative int64 contract.
- Duplicate authorization or operation IDs across different batch items were
  detected only after the first item had committed. The request boundary now
  rejects both forms before any writer. `final-batch-red01` retains the actual
  zero-revision and two committed-prefix failures, four named failures including
  the parent. Final handler ordinary/race includes the new positive and
  zero-side-effect refusal regressions.
- Actual resource-deletion proof for the new grants/terminals was missing from
  the earlier evidence. The reviewer added one test with project/issue/workspace
  subcases. It creates a real archive, signs two grants, consumes one through PUT,
  revokes the other, then calls actual deletion APIs. Complete original grant and
  consumed/revoked terminal JSON bytes retain identical SHA-256 before/after.
  Project deletion first explicitly turns policy off and permits minimal retained
  history for the surviving issue; deleted issue/workspace reads return 404.
  The additional ordinary/race runs each pass four named results.

Earlier independent corrections remain recorded in the P0/client/P1/VCS/
collaboration reviews: exact schema validation, typed expiry precision, final SQL
expiry fencing, distinct Contributor consumer authority, R38 current submission
identity, precise terminal receipts and missing-authority refusal mapping. Original
red, fixture, compilation, shared-source-window and runner failures are retained.

The first broad final runs also exposed three obsolete test assumptions. Main
corrected the terminal's retained parent relation, the deliberate missing-current-
authority 404 assertion and the old unsupported-enforce negative after enforce
became a supported mode. Product constraints were not weakened to obtain a pass.

## Requirement-to-runtime evidence

H = final handler ordinary/race; R = actual NewRouter ordinary/race;
S = service ordinary/race; M = actual migrations ordinary/race;
D = separate real deletion-retention ordinary/race. The JSON companion contains
fully qualified exact names and records, verified against the executed inventory.
These are mappings to observed tests, not proposed checks or additive coverage.

| Requirement | Actual evidence and outcome |
| --- | --- |
| G1 current policy and state transaction | H `TestDeliveryCompletionAuthorizationConsumeRevokeAndEvidenceLocks`, `TestDeliveryPolicyConcurrentCAS`; R `TestDeliveryCompletionEnforcementRouterPrivateObservationAndABA`: held facts/refusals, policy revisions and consumption remain coherent. |
| G2 current exact evidence/expiry | H `TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant`, `TestDeliveryCompletionAuthorizationTerminalInsertRechecksExpiry`; R `TestDeliveryCompletionManagedRouterLiveFences`: failures leave state and terminal unchanged. |
| G3 all actual writers/custom done | R `TestDeliveryCompletionEnforcementRouterOrdinaryAndMove`, `...BatchCommittedPrefix`, `...CreateAndCommentChild`, `TestDeliveryCompletionWebhookRouterExactGrantConsumesWithState`, `TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery`; H `TestDeliveryCompletionR38BindsActualPendingSubmission`: admitted positives and explicit refusals use real writer transactions. |
| G4 frozen history/recovery/retention | H `TestOrdinaryDeliveryActualConsumptionAndReplay`; D `TestDeliveryCompletionResourceDeletionRetainsOriginalAuthorizationAndTerminal`; M `TestDeliveryCompletionMigrationsRetainTerminalAndEnforce`: no renewal, repeated consumption or evidence deletion. |
| G5 privacy and authority intersection | R `TestDeliveryCompletionAuthorizationRouterDesignatedHumanKeepsEvaluationPrivate`, `TestDeliveryCompletionCollaborationRouterPrivateEvaluationAndAuthentication`, `...RejectsMachineAndSpoofedIdentity`: narrow receipts do not share private observations or manufacture identity. |
| G6 truthful batch/old clients | H `TestOrdinaryDeliveryBatchCommittedPrefix`, `...UnknownCommitRecovery`, `...CommittedParentNotification`, `...ZeroRevisionConsumesExactGrant`, `...DuplicateAuthorizationOrOperationRefusesBeforeWrites`; R actual prefix; frozen Core63/Views91 prove parser/cache/message contracts. |
| G7 safe activation and rollback | H `TestDeliveryCompletionActivationExplicitCASAndCapabilities`, `...RejectsPartialSchemaAllowsOff`; R `...PublicActivation`, `...RevokedGrantAndRollback`; M `TestDeliveryCompletionMigrationsRejectWeakenedSchema`: explicit current authority/readiness and retained history. |
| G8 reviewable actual proof | R `TestDeliveryCompletionAuthorizationRouterRealArtifactAndRestart`, `TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery`; D actual deletion, source/hash/inventory audit and ten distinct final databases. Local integration is still a separate main-session action. |
| G9 explicit acceptance/designated consumer | R `TestDeliveryCompletionManagedRouterDistinctIssuerAndContributor`; H `TestDeliveryCompletionConsumerDistinctContributor`, `...AuthorizationTaskAndConsumerIsolation`: actual consumer identity is separate from issuer/producer. |
| G10 finite skip governance | H `TestDeliveryCompletionActivationExplicitCASAndCapabilities`; R `TestDeliveryCompletionAuthorizationRouterSkipAndAmbiguousJSON`: v1 has no waivable checks or usable skip, rather than an untested bypass. Future governance is not claimed implemented. |
| G11 creation admission | H `TestOrdinaryDeliveryCreateDonePolicy`, `...CustomAndCommentChild`; R `...CreateAndCommentChild`: actual inherited project and custom done enforce refusal, old nonterminal/off/observe positives. |
| G12 done editing semantics | H `TestOrdinaryDeliveryActualConsumptionAndReplay`, `TestDeliveryCompletionAuthorizationNoGateForArchivedDoneEditing`; S `TestCollaborationEditDoesNotNeedOpenExecutionOrBusinessState`: later non-project edits do not manufacture new acceptance. |
| G13 project scope changes | H `TestOrdinaryDeliveryControlledProjectExit`, `...ProjectChangeBoundaries`, `...EnteringEnforceProject`, `TestDeliveryCompletionCollaborationSourceProjectMove`; D project off/delete; M `TestDeliveryProjectChangeMigrationsFenceConcurrentAdjustment`: current source/target restrictions and retained adjustment facts. |
| G14 collaboration/R38 separation | R `TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery`, `...SelfAttachmentRemainsUnboundUntilAccepted`; H `TestDeliveryCompletionR38BindsActualPendingSubmission`, `...UnenforcedLegacyEvidenceRemainsAdministrative`: exact user acceptance and real administrative disposition stay distinct. |
| G15 lawful child evidence | R `TestDeliveryCompletionCollaborationRouterCurrentIdentityAndChild`; H `TestDeliveryCompletionCollaborationCurrentBindingsRefuse`, `...ChildLockAndRollback`: exact ordinary child/parent/project/revision and current issuer/participant boundaries. |
| G16 no invented skip | R `TestDeliveryCompletionAuthorizationRouterSkipAndAmbiguousJSON`; H `TestOrdinaryDeliveryStrictCompletionControls` and explicit capabilities: unsupported skip never writes completion. |
| G17 completed object fixed first | H `TestOrdinaryDeliveryMixedCompletionRefusesActualChanges`; R ordinary/move positive: content/attachment differences refuse, actual permitted status/ordering completes. |

## Completion-entry and boundary review

Re-scanned production query callers and inline issue SQL. Actual completion
owners remain ordinary UpdateIssue, CreateInTx, WriteIssueStatus and independent
collaboration/R38 lifecycle. Move reaches the ordinary transaction; batch owns
one transaction per item; comment-child reaches creation. Project deletion and
collaboration structural edits have their separate current policy checks.

Excluded candidates retain concrete limits: triage accepts only backlog/todo;
scheduled/onboarding/channel/follow-up creation is nonterminal; ordinary task
recovery is todo; collaboration handoff/status actions reject done and creation
rejects terminal categories; iteration changes reject status co-writes; the child
barrier wakes parents without setting them done. The scan found no new unhandled
completion writer in the current sources. It is not a guarantee about future code.

The review checked current policy/credential/source bindings inside the original
transactions, exact union and retained receipt checks, late expiry SQL, revoke
serialization, real integration proof, strict non-success handling and preserved
original-key recovery. There is one observer, no service-to-handler dependency,
issuer-context impersonation, optional nil enforcement callback or unknown-as-pass
branch. No new dependency, foreign key or combined concurrent-index migration.

## Final evidence

| Scope | Ordinary | Race | Limit |
| --- | ---: | ---: | --- |
| Handler | 620 | 620 | Selected actual writer/authority and existing contracts |
| Real authenticated/signed router | 166 | 166 | Actual local route, tar, state and receipt assertions |
| Service | 272 | 272 | Common contracts and legacy collaboration compatibility |
| Actual migrations | 62 | 62 | Upgrade/down, retention, schema and concurrency |
| Additional real resource deletion | 4 | 4 | Separate supplement, not added to the 620 |

All ten rounds have exit 0, zero failed/skipped named tests, one terminal per
started name and separate new task databases. Ordinary/race and iteration rounds
are not summed as unique coverage. Core63/Views91 and both current typechecks/
scoped lint remain bound to the unchanged frontend hashes. Full Views lint has
27 existing warnings in unchanged files; none was suppressed.

Independently read and rehashed all **256** task run records and raw logs,
including **47** nonzero records. Go inventories and Vitest assertion inventories
agree; frontend logs are not counted using the Go parser's zero named count.
This is an evidence-audit count, not a test-coverage total.

Full backend build/vet passed. The 52 static contracts comprise 37 service skill/
other contracts, 11 status, three migration static and one runtime-metrics check.
134 sqlc outputs are byte-identical after generation. Current gofmt on 64 source/
test files is clean; tracked diff and new-file whitespace checks have no
diagnostics. The prior extra-LF format finding remains in its original record,
with a separate clean `final-format-after.json`.

## Source provenance and limitations

- All 110 final worktree SHA-256/byte counts match the manifest. Fifteen already
  committed unchanged files omitted by run snapshots were independently resolved
  from each recorded source_head and matched; omission is not mislabelled drift.
- The service/migration ordinary rounds preceded only the final handler batch
  parser correction. Their selected service/migration source stayed stable;
  corrected handler/router ordinary/race and service race provide later evidence.
  The service ordinary record honestly retains the concurrent handler-file change.
- The deletion supplement was added after broad suites and has separate frozen
  compiled normal/race proof. The only other final change removes one trailing LF
  from SQL source; generated SQL remains unchanged. No old binary is relabelled
  as testing later implementation bytes.
- Reviewed delivery/spec/progress/parent/six-business documents identify current
  v1 delivery and preserve M1 in_progress, M2/M3 planned and historical records.
  Expiry wording matches rejection of over-limit requests; client rollback text
  matches the actual optimistic-snapshot identity guard.
- The main session records the owned PG stopped after zero client connections,
  with 65 owned databases and raw logs retained. This review reads that record;
  it did not operate other environments or claim remote evidence archival.

No new browser, performance, whole-repository, remote provider/model or deployment
acceptance is claimed. The observer still uses stored PR facts and local-synthetic
archives. Mixed old writer binaries remain an operational bypass risk; deployment
must upgrade and drain all writers while preserving guarded history. AC43, E5,
H3a and knowledge stop/budget rules remain unchanged; provider calls are zero.

The JSON companion binds source, documents, all run/log identities, exact G1–G17
tests and limits. Main still owns staged commit scope, Lore commit, byte/mode/
existence-preserving local integration, final journal and this child task's
archival. This review does not claim those later actions already succeeded.
