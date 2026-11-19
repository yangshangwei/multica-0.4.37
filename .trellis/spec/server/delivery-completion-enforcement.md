# Transactional delivery completion enforcement

## Scope and activation

`delivery-completion-enforcement-v1` joins every real new completion transaction.
It reuses the single `delivery-artifact-observe-v1` observer and existing controlled
local-synthetic archives. It proves database-observed PR head identity, not a live
remote Git fetch, deployment success or general quality. Human acceptance remains
explicit. This increment does not complete the wider M1 roadmap or model gates.

Projects default to off. Current workspace owner/admin may append off/observe/
enforce via the existing reason + policy-revision CAS API. Project leadership
alone grants no policy authority. Enforce activation checks the exact artifact,
policy, authorization/terminal, project-adjustment and integration-trust schema;
missing or unknown state is not off. Same-value writes create a new revision.
Never enable while older writer processes lacking these gates still serve writes.

Current observe/enforce revisions permit fresh private evaluations and explicit
hints; neither is authorization. All successful hints retain enforced=false and
usable_for_completion=false: these describe the hint action. Old observations,
expiry, failure and unknown facts never change on recovery. Policy changes make
old unconsumed authorizations stale. Off/observe rollback is explicit and retains
all history; it cannot disable other authority checks or approve dangerous work.

## Explicit authorization API

Auth-only `/api/delivery-completion-authorizations` exposes human POST, own
GET by authorization ID, original-human GET by-request-key with source workspace,
and human POST revocations. Conflicting workspace selectors refuse. JSON rejects
unknown/duplicate/case-alias fields, missing required values, invalid UUIDs and
variant ambiguity. Task credentials cannot sign human acceptance.

The immutable request binds contract/source workspace, original operation UUID,
target issue/revision/expected and target status, project/revision/policy revision,
exact evidence issue/approval/source/spec/artifact and consumer, reason and UTC
microsecond expiry. A human signs a new current observation, never another
person's private evaluation key. The requested expiry must not exceed 24 hours, the evidence horizon or finite
issuer-credential expiry. An over-limit request is rejected, not silently shortened.

Target variants are ordinary, collaboration_submission, collaboration_self and
collaboration_admin. Consumers are human, exact ordinary task generation,
GitHub installation, VCS connection or real R38 administrator as applicable.
The retained issuer credential metadata is server-only and never contains bearer
secrets. Consumption explicitly checks that snapshot and the current actual
consumer separately; never synthesize an issuer authentication context.

One authorization has at most one append-only terminal fact: consumed or revoked.
No persistent pre-consumption claim exists. An unconsumed authorization may be
revoked by its currently authorized original issuer or source owner/admin with a
reason and original key. Consumption and revocation serialize on the same locked
authorization row. Neither expiry nor recovery renews or rewrites the grant.

Own minimal GET and original-key creation recovery recheck current access but do
not repeat completion or disclose observations, source-child contents, issuer
credential metadata or private evaluation keys. A consumed receipt binds exact
authorization/operation/before/after, actual actor and variant-specific original
workflow/admin/submission facts. After must advance before. Current issue state
may differ after reopen; a historical receipt is never projected as current done.

## Minimal authorization receipt reads

### 1. Scope / trigger

An authorized issuer reads a consumed grant while Web/Desktop refresh the same
artifact-bound R4 or another GET reads that grant. The GET must retain the exact
terminal receipt and current source authority throughout one borrowed transaction.

### 2. Signatures

`GetDeliveryCompletionAuthorizationInTx(ctx, tx, access, id)` holds the actual
id-only fence before the existing generated getter:

```sql
SELECT id FROM delivery_completion_authorization WHERE id=$1 FOR SHARE NOWAIT
```

Only GetByID selects `readCompletionAuthority`. Its issuer branch selects
`readCompletionIssuer` and `readCompletionSourceIssue`, using the existing shared
issue/access/observer helpers. All calls keep the caller's actual `pgx.Tx`.

### 3. Contracts

Keep one authority, issuer and source validator body. Their original entry points
select constant `false`; their private read entries select `true`. The original
by-request getter retains its UPDATE fence and default authority; Create-key
replay and Revoke retain the original issuer path. Prepare/Finish, current
consumer authority and R38 configuration/advisory checks retain their existing
logic and locks. No request field, context flag or marker chooses the mode.

The source helper's `exact` argument controls structural/version exactness.
`exact=false` still needs a physical source-child fence. Its separate private
read mode chooses `LockDeliveryEvidenceIssueRead`; the original source entry
continues to choose `LockLifecycleIssue`. Preserve the ordinary-target return,
supported-kind check, ordinary child check and exact parent/project/revision
predicates. The first R4/GET conflict can name the source child even though the
receipt names the completed root; compare the actual UUID arguments.

Schema, current credentials/workspace/source, retained producer/execution and
immutable request/terminal consistency remain checked. Terminal reads preserve
original before/after and minimal privacy after reopen; they do not repeat target
admission, renew a grant or perform completion. The handler retains no-store,
final credential checks and commit/outcome-unknown handling.

### 4. Validation / error matrix

| Condition | Required result |
| --- | --- |
| Same authorized issuer, R4 or same-grant GET held | Second read succeeds before first release, exact original receipt |
| Real Revoke holds authorization UPDATE | GET returns409 `delivery_completion_conflict` with actual55P03/grant/PID, zero partial view |
| GET holds authorization SHARE | Revoke fails at its original UPDATE before terminal/effects; after release the original consumed refusal remains |
| Different collaboration actor |404 `delivery_completion_not_found`, zero private view |
| Original password revoked or source member removed |403 `delivery_completion_denied`, zero private view |

### 5. Good / base / bad cases

Good: explicit GetByID read dispatches preserve every validator and compatible
grant/root/source-child/observer fences. Base: original writer and by-request
defaults keep their reached SQL and behavior. Bad: remove a fence, confuse
`exact` or consumer `newAdmission` with a read switch, infer mode from a marked
transaction, or duplicate validators to avoid the owning helper.

### 6. Required tests

`delivery_completion_consumed_read_concurrency_test.go` builds one genuine
same-issuer/consumer submission, grant and completion per leaf before registering
its no-effect baseline. Its3top/10leaf/13named cover both R4 orders, grant siblings,
both operation-read controls, actual Revoke orders and three current-authority
denials. Retain distinct backend PIDs, canonical SQL/hash/UUID/55P03, final
credential/commit and unchanged five-second reads/250ms lock bounds. Pin Revoke's
original UPDATE; a GET fence witness must permit its actual shared SQL name.

Require fresh normal/race with the four original service receipt/M1/consumer
tops (combined7top/54leaf/76named). When GetByID's outer fences cover all variants,
also run `TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay`,
`TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery` and
`TestDeliveryCompletionR38AuthorizationHonorsCurrentConfiguration`
(3top/5leaf/6named per mode).
Per-leaf72-relation equality proves read effects. Real fixture completion creates
retained handoff/job/invalidation history, so whole-suite DB counts can increase.

### 7. Wrong vs correct

Wrong: a generic HTTP409 or nearby R4 response identifies its physical holder.
Correct: retain that original failure, then bind a new deterministic two-backend
regression to actual source-child/grant SQL and IDs. PAY-241 S2 R29 records real
read RED and normal/race GREEN; the historical R28 holder remains unproved.

## Borrowed transaction contract

Complete service validators accept the actual `pgx.Tx` and derive queries from
it. They do not Begin/Commit/Rollback/retry or accept a pool query alongside it.
Prepare returns an explicit not_applicable/replayed/prepared state. A prepared
value is private and bound to that transaction; unknown/nil results cannot allow.
Finish checks the actual persisted after-state, current policy, issuer, consumer,
source and original receipt again. SQL itself fences consumed insertion by current
DB time for grant, evidence, issuer and actual consumer expiry.

The real writer commits status, ordinary iteration or collaboration/Stop facts,
authorization consumption and original receipt together. Late errors roll back
all writes. Commit ambiguity is unknown/non-success, never automatically retried.
Recovery of the same semantic operation precedes ordinary done-to-done editing;
no second notification, queue mutation, status write or consumption follows.

Keep original workspace/user/member/catalog/W01/iteration/issue lock prefixes.
New source/target projects and exact source child use deterministic NOWAIT locks;
all current credentials, retained producer/approver/runtime identities and schema
are checked. The observer's four PR/link SHARE NOWAIT table locks remain through
the writer commit. Contention is refusal, not a passing snapshot or a retry of
uncertain effects. Caller-provided metadata cannot substitute for these facts.

## Writer-specific rules

- Ordinary update/move and each batch item enter the same actual writer. A new
  completion is any non-done to done-category transition, including reopen then
  complete. Custom done is determined by category. Enforce requires exact binding:
  authorization_id, operation_id, expected_issue_revision. Missing binding is428.
- Enforce completion permits only actual status and optional exact explicit
  position changes. Save content/attachments/assignee/structure changes first,
  then obtain acceptance. Omitted position still permits existing automatic
  status-column ordering; replays never recompute it as another write.
- Done-to-done, custom done switches and non-project editing do not create a new
  acceptance or consume a grant. Original acceptance proves only its frozen
  version. Existing status/action/iteration permissions remain in force.
- CreateInTx resolves the actual project, including parent inheritance, before
  policy checks. Enforce rejects create-as-done, including comment-to-child and
  custom done. It does not silently choose todo or accept a parent artifact.
- Moving out of enforce/clearing its project requires current owner/admin in
  person and a retained reason, intersected with the original edit authority.
  If either side is enforce, a done task must first reopen; move and completion
  cannot occur in one request. Target policy governs later acceptance; old grants
  do not rebind. Delete requires prior explicit policy-off and checks it again
  in its transaction. Existing collaboration-project deletion refusal remains.

## Collaboration and R38

Source policy and original exact submission acceptance both apply. Ordinary
collaboration is still completed by its current human initiator; general proxy
or machine completion is not added. Outside participation gives no source
workspace membership or private evaluation visibility.

Collaboration artifacts explicitly come from one real ordinary child under that
collaboration, in the same workspace and actual project. Freeze and recheck child
ID/revision/current parent plus target issue/collaboration/lifecycle epochs and
exact submission, self-result or administrative intent. The artifact remains the
child's artifact; do not fabricate ordinary tasks for collaboration runtime runs.

Self-result and attachments have a read-only validation phase before the gate.
Only after admission may the original writer bind attachments, generate the real
submission, record acceptance and close the queue. Late failures roll everything
back. Exact current result/participant/account epochs and optional source-comment
and attachment versions remain part of the binding.

R38 retains the real password administrator, organization and disabled-creator
rules, plus exact source evidence/authorization under enforce. It does not create
an initiator accepted decision. Current retained submission identity uses the
locked session event sequence. Off/observe administrative closure keeps its old
behavior even with legacy or malformed delivery history. Original administrator
receipt recovery remains separate from permission for a new exception write.

## GitHub and VCS

Actual signed events retain source workspace, local integration UUID, monotonic
trust revision, exact PR/repository/head/provider, event identity and body digest.
Reconnect/credential or secret rotation changes trust revision; display refresh
alone does not. Consumption locks the current installation/connection and checks
this identity plus the current signature again. The required narrow verifier
checks only the original event authentication, never an injected business policy.

The existing combined PR close aggregate is still required and is reread after
observation locks. Match exactly one applicable authorization; none or ambiguity
refuses. Actual actor remains system, with human issuer responsibility separately
retained. A202 webhook receipt only acknowledges the event, never proves done.

## Batch, errors and skip

Batch remains sequential per-item transactions. Ordered results distinguish
committed/rejected/not_attempted/unknown; stop at first refusal or uncertainty,
retain the committed prefix, and notify parents only for committed items.
Partial results are non-2xx; unknown commit is503. `updated` counts known committed
items only. There is no full-batch durable ledger or automatic replay/resume.
See core/frontend/issue-batch-outcomes.md for schema parsing and cache recovery.

v1 has no optional quality checks that can safely be waived. Report
skip_supported=false and waivable_checks=[]; reject explicit skip requests.
Never label unknown, missing, stale, expired, revoked, corrupt or mismatched
mandatory evidence as passed. The approved limited skip governance remains a
future contract when a real independently waivable check exists, not a claim that
usable skip is implemented now.

## Migrations, verification and recovery

Migrations781–788 retain immutable authorizations/terminal facts;789–791 retain
project changes;792–793 add integration trust revisions. No foreign keys; each
new index is a separate CONCURRENTLY migration. Exact readiness checks columns,
constraints and live/valid/ready index shapes. The authorization runtime predicate
is shared verbatim with its migration readiness validation. Do not treat names or
schema_migrations alone as proof of an enforced constraint.

Resource deletion retains audit facts. Down refuses retained grants/terminals,
project-adjustment history, enforced policy history or relevant integration grants;
never delete evidence to permit rollback. Stop admission across multi-file rollback.
Prefer explicit policy off/observe while keeping guarded readers and retained data.

Require task-owned fresh databases per package and normal/race, real authentication,
actual tar bytes/SHA, final persisted status/receipt, current-authority races,
unknown commit and retained original-key recovery. Preserve every failed fixture,
compile, behavior or migration attempt with its own raw log and source identity.
Final all-writer proof, client protocol validation and independent review are
separate from baseline compatibility tests, provider/model or deployed acceptance.

## Ordinary human preparation reads (M1-S1)

### 1. Scope / trigger

These reads let the current human inspect artifact candidates and the exact
ordinary issue/project/policy binding before creating the existing human
authorization. They add no writer, durable authorization, evaluation, queue
entry, notification or provider call. Existing authorization creation,
transactional completion consumption and terminal receipts remain authoritative.

### 2. Signatures

- `GET /api/issues/{issueId}/delivery-evidence/candidates?limit=1..50&cursor=...`
  returns `{protocol_version, issue_id, issue_revision, project_id, checked_at,
  items, next_cursor, has_more}`. Each item contains only current-authority
  metadata: approval/execution ID, source revision, spec/artifact/receipt
  digests, creation/expiry times, summary, status/result code and selectable.
- `POST /api/issues/{issueId}/delivery-evidence/completion-preview` accepts
  exactly `{expected_issue_revision, target_status, approval_id}`. It returns
  current issue/status/category, project and policy revisions, checked_at, the
  current observation, optional canonical ordinary/human authorization
  binding, `max_expires_at`, and explicit non-permission fields
  `enforced:false`, `usable_for_completion:false`, `skip_supported:false`.
- Neither endpoint accepts an evaluation key. Creation remains
  `POST /api/delivery-completion-authorizations` with `Idempotency-Key`; by-key
  recovery remains scoped by query `workspace_id`; ordinary completion remains
  `PUT /api/issues/{id}` with `{status, completion}`.

### 3. Contracts

Candidate scans are issue/workspace scoped, human-only and bounded to at most
200 rows per request. Every candidate is filtered through live artifact
authority; approval-list visibility does not confer artifact visibility.
Filtered records must not affect visible counts, cursor placement or
`has_more`. If the scan bound is reached before enough visible candidates can
be proven, return unavailable rather than a false empty page. Cursors are
opaque request snapshots bound to issue revision, source workspace and viewer;
stale binding is refused.

Each read uses the existing artifact transaction preparation and lock order,
current credential fence, issue lifecycle lock and current project/policy
read. Selected preview repeats the actual artifact observer with current PR,
task, issuer and credential authority and calculates the maximum expiry from
the evidence and finite credential horizons under the existing 24-hour cap.
The response is not a grant: the human must review and call the existing
authorization endpoint, which observes and validates again. Custom active done
statuses use server category resolution; archived/unknown status is refused.
Both responses are `Cache-Control: no-store` and have zero business writes.
The routes keep the existing minimum task-capability guard; do not widen
Reporter/Reviewer credentials to use them.

Candidate preparation uses `LockDeliveryEvidenceIssueRead` and
`readDeliveryArtifactInTx` to retain compatible shared issue/principal/approval/
execution fences. The collaboration candidate path also shares its root
collaboration row and rechecks current source access. Actual authorization and
completion writers retain their existing exclusive entry points. Candidate
read/read proof does not cover preview's additional observer or stage checks;
see [Completion candidate preparation reads](./collaboration-stage-approvals.md#completion-candidate-preparation-reads).

### Managed submission preview binding and expiry

`GetCollaborationCompletionPreviewInTx` validates the exact current submission
without preparing a new completion operation or grant. Initialize its temporary
`stageMutationGuard.boundSubmission` from the already validated nonnil target ID
before calling the unchanged `checkSubmission`. That checker must still read the
immutable binding and original operation/receipt/handoff; do not remove it or
call the mutation binder to make a preview pass. The same predicate covers a
real R38 administrator's current submission, with unchanged nomination,
organization/password and source-artifact checks.

After taking the minimum of the24-hour, evidence, issuer-credential and stage
horizons, apply `expiry.UTC().Truncate(time.Microsecond)` before comparing it
with the database clock. This is the existing ordinary-preview rule. Returning
an offset timestamp violates the UTC wire contract; normalizing only after the
future-expiry comparison can return an expiry rounded down to the checked instant.
Keep `enforced`, `usable_for_completion` and `skip_supported` false; preview
availability is not completion authority.

The canonical regressions are
`collaboration_completion_preview_managed_submission_test.go` (enforce/off,
stale target, external participant and expired credential) and
`collaboration_completion_preview_admin_submission_test.go` (actual password
administrator, independent signers and disabled creator with a genuine pending
submission). They verify exact bindings/horizons and no grant, terminal,
evaluation, status, queue, notification or adapter effects. Preserve the original
submission RED and the separate UTC failure. Their normal/race acceptance is
scoped to binding/expiry; full preview/R4, sibling reads, missing bindings and
revocation require their separate owning-layer proof.

Evidence: PAY-241 S2
`research/r27-preview-binding-utc-followup-independent-check-20261117.md/json`
accepts each mode's2top/6leaf/8named and the two local product edits.

### 4. Validation / error matrix

| Condition | Result |
|---|---|
| Invalid/duplicate/unknown query or strict malformed preview body | 400 |
| Hidden workspace or issue | Existing resource-hiding 404 |
| Non-human or current artifact access denied | 403/404 per existing boundary |
| Stale issue, project, evidence, policy or cursor binding | 409 |
| Lock contention | 409; no retry |
| Bounded scan cannot classify enough visible candidates | 503 unavailable; never false empty |
| Current pass with canonical binding | `authorization_available`; still not permission |
| Off/observe, no project, already done or non-done target | Non-authorizing reason branch |

### 5. Good / base / bad

Good: the human selects one visible candidate, sees its current observation
and expiry horizon, then separately accepts the exact binding. Base: an empty
authorized list reveals no hidden row count. Bad: select the newest approval
automatically, filter only after a broad first page, or treat preview pass as
completion permission.

### 6. Required tests

Use fresh task-owned handler/router databases in normal and race runs. Assert
pagination and visible-set cursors, hidden count/has-more privacy, bounded
scan refusal, selected binding/credential horizon, custom status and stale
branches, cache-control, no evaluation/authorization/status/comment/queue
writes, legacy/password router access and Reporter/Reviewer refusal. Keep
failed allocations/logs and bind results to changed source bytes.

### 7. Wrong vs correct

Wrong: read the newest approval or infer signing permission from `pass`.
Correct: resolve only candidates visible under original artifact authority, let
a human choose one, then obtain a new issuer observation when the existing
authorization is explicitly created.
