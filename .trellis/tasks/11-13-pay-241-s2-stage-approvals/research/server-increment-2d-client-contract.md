# S2 2d collaboration completion preparation and activity transport

Frozen mechanical F06 handoff, 2026-11-14. These routes are **not served until2d**. Server implementation follows the2c independent gate; Core/Views may implement this closed boundary now. Ordinary M1 routes and their DTO semantics remain unchanged.

## Collaboration root and exact ordinary-child source

Normal account routes discard ambient workspace headers:

- `GET /api/collaboration/issues/{id}/delivery-evidence/candidates?source_issue_id=<canonicalUUID>&limit=20&cursor=<opaque>`
- `POST /api/collaboration/issues/{id}/delivery-evidence/completion-preview`

R38 routes use the existing authenticated password-administration group and its current disabled-creator scope:

- `GET /api/admin/collaboration/issues/{id}/delivery-evidence/candidates?source_issue_id=<canonicalUUID>&limit=20&cursor=<opaque>`
- `POST /api/admin/collaboration/issues/{id}/delivery-evidence/completion-preview`

All are human-only, `Cache-Control:no-store`, non-mutating reads with the existing five-second bounded M1 read transaction. They intersect current root access/actual R38 authority with the existing M1 source-artifact read authority. An outside initiator or administrator does not acquire private source evidence merely by owning the completion action. Missing source membership/artifact authority is a content-free denial; source-authorized humans may separately issue existing M1 authorizations for the exact completion consumer. No synthetic human actor or broader artifact permission is introduced.

`source_issue_id` is explicitly selected from an eligible ordinary child. It must currently belong directly to this collaboration root, same workspace and exact current project, as prescribed by M1 collaboration evidence. A completed child may supply its retained artifact; its own completed status must not short-circuit a preview of the **root's** new completion. The root and child are locked/rechecked together. These routes do not call the child's ordinary completion-preview endpoint and relabel its binding.

## Candidate response

The response retains existing `DeliveryEvidenceCandidates` fields, with two required additive fields:

```json
{
 "protocol_version":1,
 "issue_id":"<collaboration-rootUUID>","issue_revision":42,
 "source_issue_id":"<selected-ordinary-childUUID>","source_issue_revision":17,
 "project_id":"<same-projectUUID>","checked_at":"<UTC>",
 "items":[{"approval_id":"<UUID>","execution_id":"<UUID>","source_revision":"<fullcommit>",
 "spec_digest":"<hex64>","artifact_digest":"<hex64>","receipt_digest":"<hex64>",
 "created_at":"<UTC>","expires_at":"<UTC>","summary":"<existing readable M1 summary>",
 "status":"<existing artifact status>","result_code":null,"selectable":true}],
 "next_cursor":null,"has_more":false
}
```

`issue_revision` is the current **numeric ordinary issue-row revision of the root**, not `collaboration_revision`; `source_issue_revision` is the corresponding child-row revision. Both use the existing M1 integer contract. This endpoint supplies the root revision missing from the older collaboration detail; no fabricated conversion from its string collaboration revision is allowed. Item metadata, limits1..50/default20, bounded hidden-source scanning and statuses use existing M1 candidate semantics. The cursor additionally binds root/source/viewer/revisions and cannot grant authority after revocation. No total count or hidden artifact count is returned.

## Preview request and response

POST uses JSON, strict unknown/duplicate/case-alias/trailing-field rejection and a64KiB body limit:

```json
{"source_issue_id":"<ordinary-childUUID>","approval_id":"<selected-artifact-approvalUUID>",
 "target": {"kind":"collaboration_submission | collaboration_self | collaboration_admin", "...":"existing exact DeliveryCompletionTarget variant"}}
```

`target` is the existing closed M1 target union, with its exact required fields/nulls. Normal routes accept only submission/self; administrative routes accept only admin. `target.issue_id` equals the root path; its `expected_issue_revision` is the numeric candidate-response root revision. Collaboration/lifecycle/participant/account revisions, status-options version and exact submission/self-result/attachments/admin reason come from the current action draft and authoritative reads. No client `consumer`, policy, source snapshot, expiry, operation ID or authorization ID is accepted by this preview.

The service derives the exact consumer: current root initiator for normal collaboration completion, or the actual current R38 administrator and organization for admin. This preserves source issuer versus completion consumer separation. Preview cannot sign on either person's behalf. Self-result validation is read-only: it never binds/uploads attachments or creates a submission.

The response is the existing `DeliveryCompletionPreview` shape with these required additions:

```json
{"source_issue_id":"<selected-childUUID>","source_issue_revision":17,
 "stage_blockers":[],"stage_blockers_truncated":false}
```

Its `issue_id`, `issue_revision`, current/target status/category, project and M1 policy describe the root. Its `observation` describes the actual ordinary child artifact. When current requirements allow explicit M1 authorization, `authorization_binding.target` is the exact collaboration target, `.evidence.issue_id`/`.expected_issue_revision` are the actual child identity/revision, `.project` is the shared current project, and `.consumer` is server-derived. The existing explicit authorization POST then adds a human-chosen reason, expiry and original operation ID. Preview never issues an authorization or dispatches/completes work.

`max_expires_at` is bounded by existing current source/credential horizons and all required current stage approval/evidence horizons. Stage-incomplete produces200 with `reason:"stage_incomplete"`, `usable_for_completion:false`, `authorization_binding:null`, `max_expires_at:null`, and the source-safe bounded blocker list from wire10.2. This applies independently of M1 off/observe/enforce. Other reason/policy/observation semantics remain M1's existing contract. Collaboration `skip_supported` is alwaysfalse; ordinary skip behavior is unchanged. A200 preview is still non-authorizing, and every real completion writer rechecks its private final guard.

Failures retain `{code,error}` (ordinary delivery errors may additionally carry the existing `message`):400 `invalid_delivery_completion_request`;403 `delivery_completion_denied`;404 `not_found` for missing/unreadable root/source association;409 existing M1 revision/authority/lock conflicts;503 `delivery_completion_candidates_unavailable` or `stage_schema_unavailable`. Do not convert denial or an absent old-server route into an empty successful preview. GET cursor retry and POST preview retry are reads only; neither may replace a pending final-write key.

## Safe activity extension

The existing collaboration activity item gains optional `stage` (absent on old responses; null when no valid safe stage projection). Existing IDs, event sequence, actor, created_at and paging remain unchanged. This is a closed metadata union selected by the existing activity `kind`; no prose, candidate body/hash, child list, report ID/content, private integration identity or raw receipt is copied into it.

| New admitted activity kind | Non-null `stage` fields |
| --- | --- |
| `stage_roles_configured`, `stage_roles_unchanged` | `{role_revision}` |
| `stage_proposal_created` | `{proposal_id,candidate_id,version}` |
| `stage_proposal_revised` | `{proposal_id,candidate_id,version,parent_candidate_id}` |
| `stage_proposal_accepted`, `stage_proposal_rejected` | `{proposal_id,candidate_id,version,decision_id}` |
| `stage_scope_confirmed` | `{decision_id,scope_revision,lifecycle_epoch}` |
| `stage_code_candidate_created` | `{candidate_id,version,parent_candidate_id}` (parent explicitlynull for first version) |
| `stage_decision_recorded` | `{candidate_id,decision_id,step}` where step is code_review/qa/release/rollback |
| `stage_decision_revoked` | `{decision_id,revocation_id}` |
| `stage_advanced` | `{candidate_id,current,stage_revision}` where current is a fixed business stage |
| `stage_execution_admitted`, `stage_execution_observed`, `stage_rollback_observed` | null; safe invalidation/activity hint only, actor may be null |

UUIDs/versions follow stage canonical strings. Invalid or mismatched retained metadata becomes `stage:null`/`details_available:false`; it does not synthesize a successful decision. Unknown kinds remain the existing `other` fallback. Safe identifiers are read through current collaboration activity authority; private evidence still requires R4/T4. Existing reference-only notifications and collaboration invalidation drive Query refresh—no new channel or outbox.

### Mechanical R38 context supplement

R38 candidate responses additionally require `target_context:{expected_lifecycle_epoch:<positive decimal string>,submission_id:<current acceptance submission UUID|null>}`. These come from the same locked root snapshot and let the client form the existing `collaboration_admin` target without deriving an epoch from collaboration revision. Normal candidate responses omit this R38-only field. The normal preview still validates participant/account observations of its actual completion consumer (current initiator), not those of the source issuer. An issuer's own `write_context` cannot replace another consumer's observations.

Actual completion responses retain their existing workflow/admin receipt. A client that owns a pending M1 completion authorization may GET the existing authorization afterward and require its exact consumed receipt before retiring the corresponding pending action; no new completion receipt is fabricated on the workflow envelope.
