# M1-S1 implementation wire agreement

2026-11-12. User approved the latest plan; this records the exact P1/P2/P3 interface agreement. Runtime proof is separate.

`GET /api/issues/{id}/delivery-evidence/candidates?limit&cursor` returns:

```text
protocol_version: 1
issue_id: UUID
issue_revision: int64
project_id: UUID | null
checked_at: UTC timestamp
items: [{approval_id, execution_id, source_revision, spec_digest,
         artifact_digest: string | null, receipt_digest: string | null,
         created_at, expires_at, summary, status,
         result_code: string | null, selectable: boolean}]
next_cursor: string | null
has_more: boolean
```

`POST .../completion-preview` accepts exactly `{expected_issue_revision, target_status, approval_id}` and returns:

```text
protocol_version: 1
issue_id, issue_revision, current_status, current_category, target_status, target_category
project_id: UUID | null
project_revision: int64 | null
policy_mode, policy_revision: int64, checked_at, reason
observation: existing DeliveryObservation | null
authorization_binding: {contract_version, source_workspace_id,
                        target, project, evidence, consumer} | null
max_expires_at: UTC timestamp | null
enforced: false
usable_for_completion: false
skip_supported: false
```

The top-level reason is `authorization_available`, `target_not_done`, `already_done`, `no_project`, `policy_off`, `policy_observe` or the unchanged observation reason. Observation retains the existing fields and current-authority observer. All dates use UTC microsecond precision. Core must reject int64 values that JavaScript cannot represent exactly.

`target/project/evidence/consumer` use the existing closed ordinary/human request variants. Creation still uses header `Idempotency-Key`; by-key recovery uses query `workspace_id`; ordinary PUT uses `completion`. No evaluation key appears in any new body or operation file. This agreement adds no new durable authorization or completion service.

Core consumer agreement, exported from `@multica/core/issues`:

- `deliveryCompletionScope(userId, workspaceId, issueId)`
- `completionCandidatesOptions(scope, {limit?, cursor?})`
- `completionPreviewOptions(scope, {expected_issue_revision, target_status, approval_id})`
- `readCompletionDrafts(scope)`
- `authorizeIssueCompletion(scope, preview, {reason, expiresAt}, options?)`
- `submitIssueCompletion(scope, draft, options?)`
- `recoverIssueCompletion(scope, draft, options?)`
- `revokeIssueCompletion(scope, draft, reason, options?)`

Action result: `{outcome: authorized|consumed|revoked|refused|unknown, draft, authorization|null, issue|null, currentIssueRead:ok|denied|unknown|not_requested, reason|null}`. Creation, submission and revocation each retain their own dispatched stage. Integration consumers must consult the actual exported types as they land; material contract changes require coordination before edits.
