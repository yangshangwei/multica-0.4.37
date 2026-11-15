# P1 durable storage implementation

2026-11-14; implementation of the approved `2026-11-14-v1` storage contract.
This report covers schema and query primitives, not completed automation, model
quality or production deployment.

## Schema and bounded normalization

Migration844 adds the four conceptual records from the design and two normalized
children. Migrations845–859 each create exactly one concurrent index. No new
foreign keys, cascading actions, inline primary keys or inline unique constraints
were added.

| Table | Contract |
| --- | --- |
| `project_knowledge_policy` | Absent row means the built-in default; explicit row holds workspace/project, positive revision, enabled,1–3 maximum candidates, built-in rule version and the actual human modifier. |
| `collaboration_knowledge_job` | Unique source issue plus accepted event; immutable source/project/submission/delivery/acceptance/closed epoch/source-reader/policy binding. Fixed batch manifest, execution revision, lease generation/token/deadline, bounded DB retries and durable result. |
| `collaboration_knowledge_batch` | Normalized child of job, unique job/slot. Slots0–5 contain1–3 fixed predecessors each; slot6 is the only create batch. This preserves all16 predecessors across at most6 review batches plus1 create batch. |
| `collaboration_knowledge_attempt` | Normalized child of batch. Each execution revision/stage/attempt has immutable input hashes, target/policy revision, configuration, requested model and complete input-source closure. Running transitions once to succeeded/uncertain/failed; every terminal attempt is immutable. |
| `collaboration_knowledge_evaluation` | Append-only per-check/candidate result with exact candidate tuple, intent/predecessor, accepted identity, source/target snapshots, rule and policy revision, full input closure, support dependencies, model result and actual returned usage. |
| `collaboration_knowledge_validity` | Retained per-candidate head. CAS advances to a new evaluation and strictly newer accepted event/closed epoch. Initial creation requires expected revision0. Same-job replay reads the original head instead of advancing it. |

The two child tables are an implementation refinement of the approved fixed
batch/stage-result model, confirmed by the main session. They are not a generic
workflow engine. Job/model result JSON is bounded in storage; the service owns
the stricter approved decoded-text, wire, source-count and model-stage budgets.

No retained automation table allows row deletion. Completed/skipped/cancelled
jobs and batches are immutable. `ReleaseKnowledgeAutomationJob` commits terminal
result and lease release in one UPDATE; `UpdateKnowledgeAutomationJob` permits
only nonterminal phases. Explicit human retry increments execution revision and
resets only unsettled batches; successful or uncertain attempt history remains.

`source_reader_user_id` is the actual acceptance actor used as the future source
Read basis. It is never copied into a candidate's proposer or approver column.
The accepted policy snapshot is immutable; a current-policy check still belongs
in every service stage and settlement transaction.

## Candidate attribution and legacy history

The old candidate's body/source/lineage/state contracts remain. Added fields:

- `proposal_origin`: immutable `human|automation`, default human.
- `automation_job_id`, `automation_policy_revision`, `automation_rule_version`:
  immutable original automated proposal provenance. Required for automation,
  absent for human proposals; automated proposer UUID is null.
- `approval_origin`, `approval_job_id`: immutable original approval provenance
  after approval. Human approval requires a human UUID; system approval requires
  a job and a null human UUID.
- `decision_origin`, `last_decision_job_id`: latest decision provenance. Human
  rejection/withdrawal/supersession updates this without changing original
  automatic approval facts.

The new migration replaces only the existing attribution checks6 and9. The
named expiry check `collaboration_knowledge_candidate_check5` remains unchanged.
New columns default old rows to human without rewriting retained rows or their
original timestamps. Human decision SQL explicitly sets human latest provenance
and clears the latest system job.

`KnowledgeAutomationHasHumanDisposition` is an in-database EXISTS predicate over
the same source/project and exact normalized content/applicability. It finds
human withdrawn/rejected/superseded prior automated versions, preventing a new
candidate ID from evading a manual disposition. Only line endings normalize;
spaces, negation, case and numbers remain meaningful. The worker must route such
new content to review; it must not send rejected historical prose to the model.
Revalidation must also check the latest lineage version so a pending human
correction wins; that service check is not replaced by this predicate.

## Public/history compatibility and deletion

`agent_task_knowledge_metadata` now permits versions1 and2, preserving the object
and byte-budget constraint. `SetTaskKnowledgeDeliveryParams` adds
`KnowledgeContextVersion pgtype.Int4`; callers must send the decoded delivery's
actual version with `Valid:true`. The SQL argument intentionally uses column
type inference: casting it to integer produced an unprotected generated JSON
tag, while the inferred field retains the existing `json:"-"` contract.

`collaboration_operation.scope_kind` additionally admits `project`. Policy
writes reuse the existing retained actor/request namespace so older receipts
remain recoverable after further policy changes; the policy row has no
last-request-only shortcut.

All fields of generated job/batch/attempt/evaluation/validity rows, including
source-lock hints, have `json:"-"`. Public endpoints must construct separately
authorized projections. A model's `related_refs` cannot reduce the stored input
closure: evaluation INSERT selects the closure from the exact successful check
attempt, independent of model output. Support dependencies are separate and
available through `ListKnowledgeValidityLockHints` for consumption locking.

Policy deletion belongs in the existing parent transactions. Main-session
integration calls `DeleteProjectKnowledgePolicy` after project target cleanup
and `DeleteWorkspaceKnowledgePolicies` after workspace target cleanup. The
deletion manifest classifies policy as delete and all five retained automation
tables as keep. These queries do not delete jobs or evaluations.

Every new down migration first locks the affected tables with NOWAIT and refuses
any automation/policy/project-operation/v2 state. Empty automation can downgrade
while preserving legacy human candidates. New retained data cannot be erased to
make an old binary appear compatible.

## Callable service primitives

Source SQL: `server/pkg/db/queries/collaboration_knowledge_automation.sql`.
Generated exact parameter types:
`server/pkg/db/generated/collaboration_knowledge_automation.sql.go`.

- Policy: Get/Lock/LockForRead/Create/UpdateProjectKnowledgePolicy; separate
  create-if-absent and revision-CAS update preserve concurrent original intent.
- Jobs: Create/Get/GetForAcceptance/List/Lock/ClaimKnowledgeAutomationJob;
  FreezeKnowledgeAutomationBatches; Update/Release/RetryKnowledgeAutomationJob.
- Batches: Create/List/Get/UpdateKnowledgeAutomationBatch;
  ResetKnowledgeAutomationBatchForRetry.
- Attempts: Create/Get/List/FinishKnowledgeModelAttempt;
  MarkKnowledgeAbandonedAttemptsUncertain. A new lease owner must handle old
  running requests as uncertain rather than assume zero cost and replay them.
- Evaluation: Create/GetKnowledgeEvaluation; ListKnowledgeCandidateEvaluations.
- Validity: Set/Get/LockKnowledgeValidity; ListKnowledgeValidityLockHints.
- Candidates: Create/Approve/SupersedeAutomatedKnowledgeCandidate;
  ListKnowledgeAutomationRevalidationCandidates (no expiry filter or LIMIT3),
  ListKnowledgeApprovedTargetCandidates (complete approved set), and
  KnowledgeAutomationHasHumanDisposition.

All settlement primitives borrow the caller's transaction. Service code still
must hold the approved lock order, recheck current source/project/policy/reader,
perform exact schema/quote checks, enforce complete comparison and capacity, and
commit candidates/proofs/target revision/hints/batch receipt together.

## Verification

- `sqlc` regeneration succeeds using pinnedv1.31.1.
- `go -C server build ./...` passed after generation.
- Generated metadata privacy tests passed, including every field of five new
  retained row types and validity lock hints.
- Initial fresh-DB migration/query run: `p1-storage-4478f7e7.json`,8/8 named
  tests passed, zero skips, stable source fingerprint, zero owned connections.
  Four new suites cover legacy upgrade/down preservation, version boundaries,
  explicit policy false/revision/recovery, complete16-predecessor batching,
  immutable input/usage/evaluations, original system attribution, human withdrawal
  priority, exact validity CAS, terminal lease/result atomicity and retained
  uncertainty. Four existing generated-query tests verify the old human path
  against the upgraded schema.

The original747-only migration tests remain unchanged and continue testing their
historical schema. Current generated-query fixtures apply the new844+ migrations
after the old schema, because generated candidate rows now include attribution.
Supplemental manual-disposition and real handler deletion assertions were added
after the initial run; their separate evidence follows below. No real model or
deployment was used.

### Final storage and deletion evidence

- `p1-storage-supplement-a1190f35.json`:5/5 tests passed, but another lane changed
  server source during this run. It remains intermediate evidence only.
- `p1-storage-final-292241aa.json`:9/9 tests passed on a new database, zero skips,
  stable source and zero remaining owned connections. This adds the human
  disposition check and complete revalidation inventory containing four expired
  predecessors, while rerunning all prior automation and generated-query cases.
- `p1-retention-handler-697dc62a.json`:2/4 passed on stable source. The new project
  fixture first assumed a project with live collaboration sources was deletable;
  the existing correct handler refused409. The fixture was corrected to assert
  that refusal and policy-cleanup rollback, then detach the live source relation
  before testing the permitted deletion. The other failure was preexisting
  schema-manifest drift: `report_snapshot` and `report_snapshot_control` are
  unclassified, outside this task's new tables; reported to the main session.
- `p1-retention-handler-replay-d0e65dd4.json`: the corrected project retention test,
  workspace policy cleanup test and existing knowledge-history deletion test all
  passed:3/3, zero skips, stable source and zero remaining owned connections.
- `go vet ./pkg/db/generated ./internal/migrations` and the repeated generated
  metadata privacy tests passed after the final storage query additions.

The independent P1 checker subsequently identified a missing downgrade refusal
for pure human v2 delivery references and v2 collaboration fixed-slot history.
Migration844's guard and separate rollback regressions were handed to
`p1_foundation_check` after all runs above. These green results do not prove those
newly identified cases; the checker owns their red/green proof and final guard.

One unrelated generated diff appeared during sqlc regeneration:
`collaboration_stage_execution.sql.go` gained `ListStageExecutionFacts` from an
existing concurrently edited source query. It was preserved and reported to the
main session, and is not claimed as this task's work.
