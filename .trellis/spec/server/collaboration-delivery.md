# Collaboration results, exact decisions and manual policy

## 1. Scope / Trigger

Applies to formal human delivery, initiator acceptance/revision, self-completion,
delivery history and task-owned AI policy. These contracts extend the existing
workflow owner, handoff/event ledger, private attachments and Stop controls.
Account and source-workspace entries resolve the same collaboration issue.
Ordinary parents retain their independent lifecycle.

The disabled-creator super-admin exception in
[collaboration administration](./collaboration-administration.md) remains scoped
to R38. An administrative completion must not fabricate an initiator decision.
This human workflow does not establish provider or kernel-containment acceptance.

## 2. Signatures

`CollaborationService` exposes:

- `SubmitAcceptance(ctx, CollaborationAcceptanceParams)` — result handoff to the
  initiator, with immutable `note`, `basis` and optional retained evidence.
- `ReturnAcceptance(ctx, CollaborationAcceptanceReturnParams)` — return the exact
  undecided `SubmissionID` with a reason and explicit eligible `TargetUserID`.
- `ChangeLifecycle(ctx, CollaborationLifecycleParams)` — accept an exact pending
  `SubmissionID`, or atomically record and accept the initiator's self-result.
- `DeliveryView(ctx, CollaborationIssueAccess)` and
  `ListAcceptanceSubmissions(ctx, issueID, CollaborationDeliveryHistoryQuery)` —
  authorized current/latest and bounded immutable history projections.
- `ChangeAIPolicy(ctx, CollaborationAIPolicyParams)` — initiator-owned
  `ai_policy: "manual" | "assisted"`, using the ordinary workflow transaction.

The Auth-only `/api/collaboration/issues/{id}` API adds:

| Route | Fields in addition to the workflow observations |
| --- | --- |
| `POST /acceptance-submissions` | `note`, `basis`, `expected_status`, `status_options_version`; optional `draft_id`, `attachment_ids`, `source_comment_id` |
| `POST /acceptance-returns` | `submission_id`, `note`, `target_user_id`, `expected_status`, `status_options_version` |
| `POST /lifecycle` | Existing `action`, `status`, `status_options_version`; exact `submission_id` for acceptance, or result/basis/evidence for self-completion |
| `GET /acceptance-submissions?limit=1..100&cursor=…` | Issue/actor-bound sequence cursor; descending submission pages |
| `POST /ai-policy` | `ai_policy` |

Writes retain `Idempotency-Key` plus positive signed-64-bit decimal strings:
`expected_collaboration_revision`, `expected_lifecycle_epoch`,
`expected_participant_epoch`, `expected_account_epoch`. Directed status catalogs
include `return_acceptance`; generic change/reopen catalogs omit review.

Migration 764 adds `issue_collaboration.ai_policy TEXT NOT NULL DEFAULT 'assisted'`
with a `manual|assisted` check. It introduces no index or foreign key. Down takes
the table lock with `NOWAIT` and refuses while any retained manual row exists;
removing the policy must not silently enable that task. Creation accepts optional
`ai_policy`; omission defaults to assisted, while a present invalid enum fails.
No new deployment environment key is required.

## 3. Contracts

### Immutable result and decision

An acceptance handoff ID is the exact submission identity. Its note freezes the
result. A version-1 `delivery` event payload binds issue, submission, author,
lifecycle, basis, retained attachment IDs and optional source-comment ID.
`delivery_decision` binds that same submission and lifecycle to `accepted`,
`returned` or `cancelled`. Projection validates the event, handoff, actor,
source workspace and retained evidence together; timestamps or current comment
text cannot reconstruct missing links.

Each result and basis is nonblank valid UTF-8, at most 65,536 bytes. Text-only
delivery needs no file. Optional files use the existing draft/source/uploader
authorization and atomic binding. Reusing already delivered files omits their
old draft ID. Editing/deleting the source comment preserves the submitted prose
and delivered evidence. File bytes remain encrypted on the Server; protected
metadata/download authority is rechecked independently of the result record.

`GET` detail adds optional `ai_policy` and `delivery` for older-client
compatibility, plus `capabilities.return_acceptance`/`change_ai_policy`.

```ts
type DeliverySummary = {
  current: Submission | null;
  latest: Submission | null;
  has_history: boolean;
};
type Submission = {
  id: string; issue_id: string; event_seq: string;
  lifecycle_epoch: string | null;
  author_user_id: string; author: AccountSummary | null; created_at: string;
  result: string | null; basis: string | null;
  attachment_ids: string[]; source_comment_id: string | null;
  evidence_state: "available" | "legacy" | "unavailable";
  decision: {
    kind: "accepted" | "returned" | "cancelled";
    submission_id: string; event_seq: string;
    actor_user_id: string; actor: AccountSummary | null; created_at: string;
    reason: string | null; target_user_id: string | null;
  } | null;
};
```

`current` is undecided evidence in the current open lifecycle; an information
request does not replace it. `latest` survives revision, completion and reopen.
Available evidence requires its exact lifecycle, result and basis. Legacy or
malformed facts never authorize acceptance; replacement is explicit. Missing
detail fields on an older Server remain unknown. Completed status without a
linked initiator decision remains distinct from recorded acceptance.

Only the initiator may accept/return through the ordinary workflow. Acceptance
rechecks the exact current submission, current initiator responsibility, review
status and absence of an open information request. External work requires a
delivery. An initiator currently handling their own work with no pending result
may provide result/basis and complete in one transaction, without a self-transfer.
Self-result and decision roll back together, advance collaboration revision once,
and retain original-key recovery. Return requires a nonblank bounded reason and
explicit eligible recipient; UI defaults to the real submission author, and the
writer revalidates that recipient. Generic transfer/status cannot skip a pending
acceptance. Human submission alone preserves execution; actual lifecycle closure
continues to use the existing Stop/cancel-input/proof machinery.

History returns `{issue_id, items, next_cursor}` with at most 100 descending
submission records. Queries bind exact event/handoff IDs and retained decisions;
the opaque cursor binds actor, issue and upper sequence. Every page rechecks
current issue access. No proximity join, mutable comment lookup or unbounded
all-history load may replace this projection.

### Receipts and recovery

Success and original-key result recovery carry
`delivery_confirmation: {submission_id, attachment_ids}`. It contains no prose,
file metadata, storage key or signed URL. Clients compare the exact submission
and selected file IDs before retiring the original result bundle.

The existing minimal workflow receipt retains `code`, exact
`collaboration_revision`, and optional status-options/confirmation fields. A
committed refusal includes its original body `status`; own GET returns HTTP 200
with that stored refusal status. Historical receipt access grants no fresh issue
authority. Unknown commits, absent/malformed receipts and contradictory statuses
retain the original request; recovery performs GET, never an automatic new POST.

HTTP decoders require JSON objects, reject duplicate/unknown fields and trailing
JSON, and validate UUID/version boundaries before writes. Submission/lifecycle
bodies allow 1 MiB and return bodies 512 KiB so escaped JSON can carry the prose
byte limit. Other workflow bodies retain 64 KiB; prose limits apply after decoding.

### Manual policy and existing execution facts

Policy is independent of deployment gates, owner grants, holds, queued intent and
termination proof. Transition to manual pauses an open session and all lanes,
requests Stop through the existing controls, and records policy/event/receipt in
the same transaction. A closed queue stays closed; permitted or possibly live
runs retain their original proof identities and unconfirmed termination barriers.
Returning to assisted changes no pause/hold state and never resumes automatically.

Central workflow admission rejects new command/Resume, registration,
resource-offer/adoption/consent and Switch mutations with `task_manual`. Typed AI
comment inputs, continuation, promotion, Claim and Permit also recheck the
persisted policy. Existing recovery, human reads/discussion/workflow, Stop,
waiting cancellation and withdrawal remain available under their usual authority.
Do not enforce this only in the UI or by clearing grants/history.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Blank/invalid or oversized result/basis | 400 `result_required`, `basis_required`, `result_too_large`, `basis_too_large` |
| External completion without a delivery | 409 `delivery_required` |
| Wrong exact pending submission | 409 `submission_changed` or `submission_not_pending`; no acceptance |
| Missing/legacy/malformed retained evidence | 409 `delivery_unavailable`; explicit replacement required |
| Blank/oversized revision reason | 400 `revision_reason_required` / `revision_reason_too_large` |
| Pending result bypass through transfer/status/submission | 409 `acceptance_pending` |
| Changed status catalog/authority/revision | Existing durable stale/refusal receipt; no adopted newer intent |
| Unavailable return recipient | 409 `target_unavailable`; no implicit enrollment |
| Manual task requests new AI admission or Resume | 409 `task_manual`; existing proof/history preserved |
| Invalid policy value | 400 `invalid_ai_policy`; no policy/control side effects |
| Source-comment/file identity belongs elsewhere | Existing not-found/scope refusal; no evidence disclosure |
| Write/storage/commit fault | Roll back known failure; unknown commit keeps original-key recovery |
| Retained manual row on downgrade | Migration refuses; no silent assisted fallback |

## 5. Good / Base / Bad Cases

- Good: handler submits text result/basis, initiator returns that ID to its author
  with a reason, author resubmits, and initiator accepts the new ID. Both versions
  and the exact decisions remain readable after reopen.
- Good: permitted execution is running when the initiator selects manual; policy
  and Stop persist while the UI keeps termination unconfirmed until original proof.
- Base: information chains, plain comments, encrypted evidence and ordinary
  parent lifecycle preserve their own identities and permissions.
- Bad: infer acceptance from review/done, a successful AI run, nearby comments or
  an administrative lifecycle event; accept a stale ID; Resume on assisted change.

## 6. Tests Required

- `collaboration_delivery_test.go`: prose/byte bounds, generic-review bypass,
  external delivery requirement, exact return/resubmit/accept, legacy/malformed
  replacement, atomic self-result, retained comment/encrypted evidence and parent
  independence. Assert original receipts and no partial mutation on refusals.
- `collaboration_delivery_projection_test.go`: exact handoff/author/initiator,
  revision target and source-comment/file workspace identity. Deny cross-source
  retained evidence rather than infer a valid snapshot.
- `collaboration_ai_policy_test.go`: initiator authority, immutable original keys,
  required-write rollback, permitted-run Stop/proof preservation, explicit input,
  Resume, continuation, promotion, Claim and Permit fences.
- Handler `collaboration_delivery_test.go`: strict bodies, escaped byte limits,
  optional UUID/file validation, committed refusal status, exact confirmation,
  explicit manual creation/default assisted and present-invalid enum rejection.
- `internal/migrations/collaboration_ai_policy_test.go`: execute the actual 764
  up/down files, preserve existing rows/defaults, reject invalid/null policy,
  retain manual state on refused downgrade, reject concurrent writers with
  `NOWAIT`, and verify explicit-maintenance downgrade/re-upgrade defaults.
- Existing lifecycle/admin/queue/runtime/caller suites: prepare actual formal
  delivery and exact acceptance while preserving R38, waiting-input history,
  unconfirmed proof and independently scoped controls. Refused own GET asserts
  stored status separately from successful minimal receipt assertions.
- Core delivery-result/history/result-operation/presentation suites: malformed
  identities, bounded page chains, optional history denial, confirmation matching,
  exact bundle retirement and unknown/refused intent. Shared Views owns wiring,
  both locales, factual unaccepted completion and enclosing recovery disclosures.
- `e2e/collaboration-detail-journey.spec.ts`: production Web and actual Electron
  information → submit → revise → resubmit → exact acceptance, source identity,
  independent parent, one original POST/lost-response GET, Chinese self-result,
  repeated anchors and restored scroll. Captures must prove the intended target
  is inside the actual viewport. Renderer/preload evidence is not installed Main,
  provider-account or native kernel-proof evidence.

## 7. Wrong vs Correct

Wrong: `ChangeLifecycle(action: "complete")` accepts external work without a
submission, or a test sets review and treats it as delivered evidence.

Correct: submit immutable result/basis first, then complete with that exact
`submission_id` and current observations; self-handled work supplies its own
result/basis in the atomic lifecycle request.

Wrong: clear input/grants or force a run settled to implement manual mode; restore
assisted and silently Resume.

Correct: persist policy through the workflow owner, request Stop with retained
proof identity, fence every new admission path, and require a fresh explicit
Resume after assisted is restored.
