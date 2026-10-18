# Collaboration creation and private attachments

## 1. Scope / Trigger

Apply to human collaboration creation, draft uploads, delivery, comment file
references and native preview/download. New issue creation remains behind the
persisted creation gate. Runtime file preparation and general provider support
require separate implementation and acceptance.

## 2. Signatures

The authenticated `/api/collaboration` group owns these endpoints:

- `GET /creation-options[?source_workspace_id=UUID]`
- `POST /issues` and `GET /creations/{requestId}?source_workspace_id=UUID`
- `GET /creations/{requestId}/result?source_workspace_id=UUID`
- `POST /attachments/drafts`, `GET /attachments/drafts/{draftId}` and
  `GET /attachments/drafts/{draftId}/operations/{requestId}`
- Draft metadata/content/download and uploader-only DELETE below
  `/attachments/drafts/{draftId}/{attachmentId}`
- `POST /issues/{id}/attachments/deliver`, its original-key GET below
  `/issues/{id}/attachments/operations/{requestId}`, and the issue attachment list
- Metadata/content/download and POST `/capabilities` below
  `/issues/{id}/attachments/{attachmentId}`

The only unauthenticated file route is the signed, narrowly scoped GET
`/issues/{id}/attachments/{attachmentId}/capability?cap=...`.
Migrations 727–728 extend the retained attachment table and add the concurrent
upload-request identity index. Register every concurrent index in the runner's
recovery map; retained data must refuse destructive rollback.

## 3. Contracts

Creation uses an explicit source workspace and original `Idempotency-Key`.
`handler_user_id` defaults to the human creator. Issue, participants, paused
session, selected attachment delivery, history and operation commit together.
The gate does not authorize a source workspace and creating does not invoke AI.
The source picker lists the caller's existing memberships only; it does not
open project, iteration or account directories. Initial iteration/status field
authoring is not implied by this picker.

Creation revalidates the actor under sorted actor/recipient user locks before
replaying a completed operation. New requests also hold the gate `FOR SHARE`,
source workspace `FOR KEY SHARE` and source membership fence before reserving
the operation. Actor/source authorization failures create no operation or
account fence. A closed gate or unavailable recipient seals the original key
with a refusal. Issue, counter, account-fence, participant, session, attachment
and event writes run inside a savepoint after reservation; a recognized business
refusal rolls them back while the outer actor/source/gate locks survive until
commit. `CreateInTx` borrows its transaction; only the owner commits the refusal.
Required-write, query, savepoint and uncertain commit failures are never proof
of a durable refusal. Only an actual missing parent maps to parent-not-found;
database failures remain unavailable.

The own-result endpoint returns exactly `request_id`, `source_workspace_id`,
`actor_user_id`, `outcome` and `code`. The pairs are `created/created` or
`rejected` with one of `account_unavailable`, `collaboration_unavailable`,
`invalid_attachment_selection`, `attachment_not_found`,
`attachment_scope_mismatch`, `attachment_draft_consumed`,
`parent_issue_not_found`, `project_not_found`, `triage_review_required` or
`issue_limit_reached`. It revalidates the active human/password identity, grants
no content access, and remains readable by that actor after source departure.
Missing (`operation_not_found`), incomplete (`operation_incomplete`) and invalid
results do not confirm rejection. The original successful POST/GET still return
the current authorized issue; the original GET returns `not_found` for a refusal.

Same-key replay preserves the original outcome even after recipient/gate
restoration or a corrected request consumes the draft. Changed payloads conflict.
Clients retain the original intent on unknown outcomes. Only a correlated,
settled rejection permits an explicit resubmission with a fresh key, preserving
editable fields and unconsumed uploads; unchanged input can be resubmitted after
the refusal condition improves. Reading a refusal never automatically submits.

Multipart uploads have one file, a draft UUID and exactly one source workspace
(pre-issue) or intended issue UUID. A file is at most 100 MiB; a draft contains
at most 32 files. Each attempt uses an independent random object key and a
per-object encryption key. `Storage` receives ciphertext under
`private/collaboration/`; encryption keys and digests stay in database rows.
Do not create ordinary attachment records or expose storage URLs. The namespace
is also rejected by public local serving, ordinary metadata/content/download,
signed downloads and ordinary file cleanup.

Replays must preserve the winning upload and immutable bytes. A definite losing
attempt may delete only its own object. An uncertain database commit must retain
its object until the original request can be reconciled. An encrypted orphan is
not evidence that the operation failed; automated orphan reclamation is a
separate lifecycle concern.

Metadata contains IDs, explicit nullable `intended_issue_id`, original display name,
sniffed content type, size, exact
decimal version and creation/delivery timestamps. It contains no object key,
encryption material, digest or capability URL. Selected files must belong to the
authorized uploader/draft/source or already be delivered to the same issue.
Missing and foreign attachment identities have the same not-found response.
A draft is one upload bundle, consumed once by a creation, comment or explicit
delivery operation. Original-operation replay is allowed; another request must
not bind remaining sibling files to another issue or upload into a consumed
draft. Same-issue references to already delivered files omit `draft_id`.
Delivered rows are the permanent consumed-bundle marker; discarded rows retain
the draft's original context. Upload, creation and delivery writers must hold
the uploader user-row `FOR UPDATE` fence through the full state check and commit.
It also fences a new upload inserted after another request began preparing its
bytes. Recheck after external storage and delete only the losing attempt's object.

Comment create/preview accept `draft_id` and `attachment_ids`. Delivery and
comment creation are one transaction. Every accepted AI input stores immutable
attachment IDs and command references; this does not grant the runtime a human
download credential. Comment edit omits `attachment_ids` to retain references
or sends `[]` to remove display references. Editing/deleting comments preserves
issue delivery, historical before/after IDs and command references.

Authenticated bytes and native capabilities use current account plus independent
source-member/participant read bases. Capabilities bind issue, attachment,
version, viewer, password/account and membership/participation identity, and an
explicit `content|download` intent; lifetime is at most 60 seconds. Every use
rechecks access, including after slow reads and before response bytes. Permission
restoration must not revive an old membership/participant capability.

No file response redirects to public/S3 storage. Standard download ranges are
served from authorized decrypted bytes; text preview retains the 2 MiB limit.
Two process-wide byte slots bound simultaneous buffering. A full slot set returns
503 `attachment_busy` with `Retry-After: 1` before buffering another request.
Responses, including failures, carry `no-store` and `no-referrer`; logs exclude
capability query values and protected contents. Already delivered bytes cannot
be recalled from a recipient.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Creation gate closed / source not authorized | Refuse before creation or binding |
| Valid actor/source with settled creation refusal | Persist only original operation; all attempted creation effects roll back |
| Old refused key after recipient/gate recovery | Replay original refusal; no creation or attachment consumption |
| Uncertain creation commit or missing own result | Keep original intent; no confirmed refusal or fresh-key retry |
| Upload key reused with another payload | `idempotency_conflict`; winner unchanged |
| Missing, foreign or wrong-issue file | Not found without existence disclosure |
| Discarded upload key replayed | Refuse; never recreate the draft file |
| File over 100 MiB / oversized text preview | `attachment_too_large` / `preview_too_large` |
| Byte capacity exhausted | 503 `attachment_busy`, bounded retry hint |
| Permission changes while reading | Recheck and refuse before writing bytes |
| Old native capability after revoke/rejoin | Refuse despite renewed access |
| Comment reference removed | Preserve issue/history/command delivery |

## 5. Good / Base / Bad Cases

- Good: recover an unknown upload, atomically create with its ID, then let the
  current external participant download through a newly authorized capability.
- Base: ordinary attachments keep their existing public contract outside the
  reserved private namespace.
- Bad: delete storage on every commit error, or let an ordinary signed-download
  endpoint proxy a fabricated attachment pointing into the private namespace.

## 6. Tests Required

Service/handler `collaboration_attachments_test.go` covers concurrent upload keys,
unknown commit, atomic rollback, same-issue identity, discarded replay, retained
comment/command references, exact size bounds and slow-read revocation.
`collaboration_delivery_routes_test.go` verifies the real route and gate.
Storage and middleware suites cover private-prefix normalization, old file
entry points and capability log redaction. Migration tests must exercise actual
upgrade SQL, concurrent index recovery and retained/uncommitted-row downgrade
refusal in private schemas. Browser acceptance uses public creation and real
uploads; directly converting a test issue does not prove this flow.

Service/handler `collaboration_creation_result_test.go` covers original-key
refusal replay, fresh-intent draft reuse, caller-owned rollback, unknown commits,
content-free correlation and current actor/password checks. Deterministic
database waiters verify recipient, source membership, workspace deletion, gate
and result readers remain fenced through the refusal commit after savepoint
rollback. Required-write fault wrappers must still observe writes inside that
savepoint; existing creation and draft-consumption regressions remain required.

## 7. Wrong vs Correct

Wrong: return `Storage.ObjectURL`, put a signed URL in a comment, or remove a
delivered file when its last current comment reference disappears.

Correct: persist immutable file IDs, authorize every byte request through the
current issue, and retain delivered history independently of display references.
