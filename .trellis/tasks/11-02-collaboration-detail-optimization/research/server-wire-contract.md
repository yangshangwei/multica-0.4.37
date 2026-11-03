# Delivery and policy wire contract

Server-owned additive contract, frozen for Core/Views implementation on 2026-11-03.
All IDs are UUID strings; event/version/epoch fields are positive int64 decimal
strings. Account summaries use the existing `{user_id,name,avatar_url,external}`
shape. Absent new fields from an older Server mean unknown, never manual.

## Detail and history

`GET /api/collaboration/issues/{id}` adds:

```ts
ai_policy: "manual" | "assisted"
delivery: {
  current: Submission | null
  latest: Submission | null
  has_history: boolean
}
capabilities.return_acceptance: boolean
capabilities.change_ai_policy: boolean
```

`current` is the exact undecided submission of the current open lifecycle.
Information requests do not replace it. `latest` retains the most recent result
even after revision, completion or reopening. A legacy undecided submission is
factual current evidence but cannot satisfy acceptance.

```ts
type Submission = {
  id: string // exact acceptance handoff ID
  issue_id: string
  event_seq: string
  lifecycle_epoch: string | null // null for legacy/unavailable payload
  author_user_id: string
  author: AccountSummary | null
  created_at: string // ISO timestamp from immutable handoff
  result: string | null // immutable handoff note; null when unavailable/blank
  basis: string | null
  attachment_ids: string[]
  source_comment_id: string | null
  evidence_state: "available" | "legacy" | "unavailable"
  decision: Decision | null
}
type Decision = {
  kind: "accepted" | "returned" | "cancelled"
  submission_id: string
  event_seq: string
  actor_user_id: string
  actor: AccountSummary | null
  created_at: string
  reason: string | null
  target_user_id: string | null // returned only
}
```

`GET .../acceptance-submissions?limit=<1..100>&cursor=<opaque>` returns
`{issue_id,items:Submission[],next_cursor:string|null}` ordered by submission
event sequence descending. Cursor binds the issue, actor and upper event
sequence; the public endpoint rechecks current issue read authority.

## Writes

All existing original Idempotency-Key and workflow observation fields remain.

- Submit (`POST .../acceptance-submissions`): `note` (result), `basis`,
  `expected_status`, `status_options_version`, optional `draft_id`,
  `attachment_ids`, `source_comment_id`. Each prose field is nonblank and at most
  65,536 UTF-8 bytes. No file is required.
- Return (`POST .../acceptance-returns`): `submission_id`, `note` (nonblank
  revision reason), explicit `target_user_id`, `expected_status`,
  `status_options_version`. Status-options action is `return_acceptance` and its
  directed category is `in_progress`. Only the initiator may return.
- Lifecycle complete (`POST .../lifecycle`): add optional `submission_id`,
  `note`, `basis`, `draft_id`, `attachment_ids`, `source_comment_id`.
  Pending delivery requires its exact `submission_id` and review/current
  initiator responsibility/no open information request. Self-handled work with
  no pending delivery requires result `note` and `basis` and stores an atomic
  self delivery plus acceptance. Cancel/reopen retain their existing inputs.
- Policy (`POST .../ai-policy`): `ai_policy:"manual"|"assisted"` plus the four
  existing workflow observations. Operation action/kind is `change_ai_policy`.
- Creation adds optional `ai_policy`, defaulting to assisted.

Result success and own original-key recovery include
`delivery_confirmation:{submission_id:string,attachment_ids:string[]}`.
It contains no prose, file metadata or signed URL. All other legacy minimal
receipt fields remain; return/policy need no result confirmation.

## Fixed workflow outcomes

New successful codes: `acceptance_returned`, `ai_policy_changed` (and
`ai_policy_unchanged` for an explicit no-op). Existing submit/complete success
codes remain `acceptance_submitted`/`completed`.

New durable refusal codes: `result_required`, `basis_required`,
`result_too_large`, `basis_too_large`, `delivery_required`,
`submission_changed`, `submission_not_pending`, `delivery_unavailable`,
`revision_reason_required`, `revision_reason_too_large`, `acceptance_pending`,
`task_manual`, `invalid_ai_policy`. Existing `target_unavailable`,
`information_request_not_active`, `forbidden`, stale authority/catalog and
`issue_closed` outcomes retain their meaning.

Generic change/reopen catalogs omit review; generic transfer/status cannot skip
an undecided submission. Formal legacy replacement is explicit resubmission by
the actual handler, not inferred acceptance or generic completion.

Manual retains read/comment/human/Stop/recovery authority, denies new command
and Resume admission, pauses existing lanes and requests Stop through the
existing proof-aware controls. Assisted never resumes by itself.
