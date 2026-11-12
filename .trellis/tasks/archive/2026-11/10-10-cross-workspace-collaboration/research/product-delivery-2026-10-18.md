# Public creation and controlled attachment delivery

This lane continues the approved task at `4dd643126`. It owns server creation,
private attachment storage and HTTP access. Resource forms and account realtime
are being changed by other agents; their files are not replaced.

## Boundary before implementation

`CollaborationService.CreateInTx` already commits a source issue, participants,
paused session, event, notification intent and actor-scoped creation receipt.
The account router has recovery/read but no public POST. Attachment tables exist
without writers or readers. Ordinary storage publishes local `/uploads` or S3/CDN
URLs and cannot carry collaboration plaintext safely.

The implementation reuses this transaction owner, the existing content mutation
owner, `Storage`, `secretbox`, authorization helpers and authenticated account
routes. Each uploaded object has a random encryption key stored only in its
private database row. The storage backend receives opaque ciphertext under the
private collaboration prefix and no original filename. Local public serving also
rejects that prefix. No ordinary attachment row, object URL or presigned storage
link is created.

Expected files: `service/collaboration.go`, new `collaboration_attachments*.go`
and creation options service; focused comment input/reference integration;
new creation/attachment handlers; `pkg/db/queries/collaboration_attachments.sql`;
727–729 migration range if required; storage private helpers; narrow router and
index recovery registration; DB-backed and storage regressions.

This lane does not change websocket delivery, client state or forms, provider
execution, production enablement, existing ordinary attachments or hard-delete
semantics. Public creation remains guarded by the default-disabled persisted gate.

## Contract being implemented

- `GET /api/collaboration/creation-options`: only the caller's source spaces and
  gate state; an explicit source query validates membership before options.
- `POST /api/collaboration/issues`: explicit source, title/description, optional
  handler (self by default), source project/parent, draft and selected attachment
  IDs. `Idempotency-Key` is preserved; successful creation returns the existing
  collaboration detail DTO. `GET /creations/{requestId}` remains its recovery path.
- `POST /api/collaboration/attachments/drafts`: multipart single file, draft UUID,
  and exactly one source-workspace or intended-issue UUID. Original upload key
  supports retry/recovery; upload responses never contain storage URLs or secrets.
- Private draft metadata/read/discard require the uploader and current authority.
  Delivered attachments are immutable and cannot be discarded as drafts.
- `POST /issues/{id}/attachments/deliver` atomically binds selected same-context
  drafts. List, metadata, content and download use the issue's current read basis.
- `POST /issues/{id}/attachments/{attachmentId}/capabilities` returns a short
  native URL. Redemption binds the actor, issue, file version and authorization
  epochs, rechecks current access, and proxies bytes without redirects.
- Comment display references can be removed while permanent issue delivery and
  immutable command references remain. Binding a different issue's attachment
  always fails before any comment/input write.

Upload is bounded to the existing 100 MiB file limit. Ciphertext and plaintext
buffers are bounded; Range uses the decrypted in-memory reader, never a public
backend redirect. Preview retains the existing text limit and sandbox headers.
All metadata/byte/capability responses use no-store and no-referrer. Native URL
credentials and payloads are excluded from request logs.

External upload occurs before the short authorized database transaction. Each
attempt uses its own random storage key, so duplicate upload attempts cannot
overwrite one another. Definite rollback/replay removes only the losing attempt's
object. Unknown commit does not delete an object that may have been delivered;
the original upload key resolves the stored winner. Any orphan remains encrypted
and never enters a public/plaintext path.

## Verification

Database: `postgres://postgres@127.0.0.1:51974/cwc_product_delivery_nebz79rl?sslmode=disable`.
Evidence directory: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.
Use the repository agent-CLI guard for all Go test commands. No ambient or paid
AI executable is used. Exact results will be appended after execution.

## Implemented wire contract for client integration

All paths below are relative to `/api/collaboration`. Authenticated paths accept
the existing human bearer/cookie credentials and ignore ambient workspace headers.
UUIDs are canonical strings, new versions are positive decimal strings. Every
response, including byte errors, is `Cache-Control: no-store` and
`Referrer-Policy: no-referrer`.

`GET /creation-options[?source_workspace_id=UUID]` returns
`{creation_enabled:boolean,sources:[{id,name,slug}],source:{id,name,slug}|null,max_attachment_bytes:104857600}`.
Only actual source memberships are returned, including while the gate is closed.
An explicitly selected source without membership is 403. This is a source picker,
not a new project/member/iteration discovery endpoint.

`POST /issues` takes `Idempotency-Key` (1–128 chars) and
`{source_workspace_id,title,description?,handler_user_id?,project_id?,parent_issue_id?,draft_id?,attachment_ids?}`.
Handler defaults to the authenticated creator. It returns 201 and the same
`CollaborationIssueResponse` as GET detail; attachments are read separately.
Status and priority currently default to `todo`/`none`, matching the original
service. Full R49/R50 field forms and iteration creation are separate follow-ups.
Recovery remains `GET /creations/{requestId}?source_workspace_id=UUID`.

`POST /attachments/drafts` takes one multipart `file`, a `draft_id`, and exactly
one `source_workspace_id` (new issue) or `issue_id` (existing issue), plus the
original `Idempotency-Key`. File metadata is sniffed server-side. Up to 32 files
can be in one draft; a file is at most 100 MiB. Response 201 is:

```json
{
  "id": "UUID", "draft_id": "UUID", "issue_id": null,
  "source_workspace_id": "UUID", "uploader_user_id": "UUID",
  "name": "notes.txt", "content_type": "text/plain; charset=utf-8",
  "size_bytes": 24, "version": "1", "created_at": "ISO8601",
  "delivered_at": null
}
```

After delivery, `issue_id` and `delivered_at` are non-null; immutable `version`
remains `"1"`. This metadata never includes `url`, `storage_key`, encryption key,
content digest, or a native capability.

- `GET /attachments/drafts/{draftId}` → `{draft_id,attachments:[metadata]}`.
- `GET /attachments/drafts/{draftId}/operations/{requestId}` → original upload
  metadata after current authorization, including after binding to an issue.
- `GET /attachments/drafts/{draftId}/{attachmentId}` → private draft metadata.
- The same draft path with `/content` or `/download` streams authorized bytes.
- `DELETE /attachments/drafts/{draftId}/{attachmentId}` → 204; uploader only,
  undeclared/delivered IDs refused. A discarded original upload key never revives.
- `POST /issues/{id}/attachments/deliver` takes `{draft_id,attachment_ids}` and
  original key. It returns the existing content receipt envelope with
  `code:"attachments_delivered"`, `protocol_version:2`, `issue_id`, `attachment_ids`,
  `event_id/event_seq`; unused comment/target identities remain null/empty.
- `GET /issues/{id}/attachments/operations/{requestId}` recovers this receipt.
- `GET /issues/{id}/attachments` → `{issue_id,attachments:[metadata]}`.
- `GET /issues/{id}/attachments/{attachmentId}` → metadata.
- That issue attachment path with `/content` or `/download` streams bytes;
  download supports standard byte ranges. Text preview retains the 2 MiB limit.
- `POST /issues/{id}/attachments/{attachmentId}/capabilities` takes
  `{intent:"content"|"download"}` and returns
  `{issue_id,attachment_id,version,url,expires_at}`. `url` is a relative
  `/api/collaboration/issues/.../attachments/.../capability?cap=...` URL, valid for
  at most 60 seconds. The one public GET endpoint verifies its signature and
  current user/password/account/member/participant identity on every redemption.
  A native loader may use this URL without a bearer; never persist it in content.

Comment create/trigger-preview accept optional `draft_id` and `attachment_ids`.
Create atomically delivers selected issue-intended drafts and records their
comment references. Already delivered same-issue IDs can be reused without a
draft. Create preserves attachment IDs in every accepted command's immutable
snapshot and command reference. Comment responses include `attachment_ids:[]`.
Edit accepts optional `attachment_ids`: omission preserves, `[]` removes current
display references. New drafts also require `draft_id`. Delete keeps reference
tombstones and permanent issue/command delivery. Comment history now includes
`before_attachment_ids` and `after_attachment_ids` (older history may be null).

Important bounded failures include `invalid_attachment_upload`,
`invalid_attachment_selection`, `attachment_scope_mismatch`,
`attachment_already_delivered`, `attachment_discarded`, `attachment_limit`,
`attachment_too_large`, `preview_too_large`, `attachment_busy`,
`attachment_unavailable`, and the existing `idempotency_conflict/not_found`.
Known mutation failures preserve drafts; uncertain responses retain the original
operation key. 503 `attachment_busy` includes `Retry-After: 1` without buffering
another large file.

## Evidence collected so far

- `delivery-red.log`: absent creation POST returned 404; the unguarded private
  local namespace returned 200. Both baseline tests failed as intended.
- `delivery-green-initial.log`: those storage/router regressions passed.
- `delivery-comment-red.log`: comment attachment fields returned 400 before
  their dedicated binding/reference implementation.
- `delivery-legacy-route-red.log`: an ordinary row naming private storage could
  mint an old signed-download URL before the generic file guards were added.
- `delivery-service-first.log`: initial five attachment DB regression groups pass.
- `delivery-service-race.log`: attachment, creation and comment DB regressions
  pass under `go test -race` (7.640 s); includes independent comment references,
  immutable command attachment snapshots, unknown commit and concurrent uploads.

Further integration boundary: the protocol already carries `AttachmentIDs` and
this lane now preserves them. General provider/CLI attachment preparation still
needs its dedicated business-capability byte contract and native evidence; the
human attachment API and old file APIs must not become that fallback. Automatic
reclamation of encrypted objects orphaned by a truly unresolved commit is also
not introduced here. Known losing/failed attempts are cleaned immediately;
delivered objects are never classified as ordinary attachment garbage.

## Final lane verification and handoff

The final binding check maps another uploader's private draft, another issue's
delivered file, and a foreign source file to the same 404
`attachment_not_found` as an unknown ID. This was first reproduced through the
DB-backed HTTP handler (`delivery-existence-handler-red.log`, 409 before fix),
then passed with the full attachment handler group. A creator reusing their own
already-delivered creation draft still receives the explicit scope conflict.

All Go tests below ran from `server/`, with the isolated `DATABASE_URL` above,
through `bash ../scripts/go-test-with-agent-cli-guard.sh`. Router tests used the
owned `delivery-uploads` directory, never application upload storage.

| Command after the wrapper | Result / evidence |
| --- | --- |
| `go test -race ./internal/service -run '^TestCollaboration(Attachment\|Create\|Creation\|Comment)' -count=1` | PASS 7.939 s, `delivery-service-final-race.log` |
| `go test -race ./internal/handler -run '^Test(Collaboration\|Attachment\|DownloadAttachment\|GetAttachment\|ServeLocal\|UploadFile\|DeleteAttachment\|ListAttachments)' -count=1` | PASS 6.373 s, `delivery-handler-race.log` |
| `go test -race ./internal/handler -run '^TestCollaboration(Attachment\|CommentAttachment\|PrivateObjects)' -count=1` | Final PASS 3.995 s, `delivery-final-handler-race.log` |
| `go test -race ./cmd/server -run '^TestCollaboration(PublicCreation\|Routes)' -count=1` | PASS 2.579 s, `delivery-routes-race.log` |
| `go test -race ./internal/storage -count=1` | PASS 1.805 s, `delivery-storage-race.log` |
| `go test -race ./internal/middleware -run '^TestCollaborationAttachmentRequestLogOmitsCapabilityAndContent$' -count=1` | PASS 1.790 s, `delivery-logging-race.log` |

`go vet ./internal/service ./internal/handler ./internal/storage ./internal/middleware ./cmd/server`
passed with empty output (`delivery-vet-final.log`). Its earlier run was blocked
by the resource lane's in-progress `Participants` test, not a vet finding; that
method and query generation are now present. `gofmt` and `git diff --check` passed.

Behavior evidence includes: encrypted stored bytes; same-key uploads converging
without overwriting or deleting the winner; successful commit with lost reply
recovering the same intact file; source membership lost after storage success
causing definite cleanup; required creation event failure rolling back the issue,
bindings and operation; same-issue file scoping; participant removal/rejoin and
source membership identity changes invalidating native links; direct read retained
through the still-valid alternate read basis; comment reference tombstones and
independent command/other-comment references; proxy byte ranges; an actual streamed
100 MiB + 1 upload refused before storage; cancellation; revocation during a slow
storage read; old file/signed-download/cleanup refusal; capability/body log redaction.

No commits were made. Creation/execution gates remain default-disabled. The main
session owns SQL generation and migration verification, client schema/forms,
Web/Electron acceptance, independent review, final specs and commits. This lane
does not claim complete P1–P8 acceptance, a real S3 deployment check, or runtime
attachment-byte preparation support.

Exact owned changes (generated files also contain other lanes' shared generation):

```text
.trellis/tasks/10-10-cross-workspace-collaboration/research/product-delivery-2026-10-18.md
server/cmd/migrate/main.go
server/cmd/server/router.go
server/cmd/server/collaboration_routes_test.go
server/cmd/server/collaboration_delivery_routes_test.go
server/internal/handler/collaboration_attachment_capability.go
server/internal/handler/collaboration_attachments.go
server/internal/handler/collaboration_attachments_test.go
server/internal/handler/collaboration_creation.go
server/internal/handler/collaboration_comments.go
server/internal/handler/collaboration_mentions.go
server/internal/handler/file.go
server/internal/middleware/collaboration_attachment_logging_test.go
server/internal/service/collaboration.go
server/internal/service/collaboration_attachment_access.go
server/internal/service/collaboration_attachments.go
server/internal/service/collaboration_attachments_test.go
server/internal/service/collaboration_creation_options.go
server/internal/service/collaboration_batches.go
server/internal/service/collaboration_comments.go
server/internal/service/collaboration_comments_create.go
server/internal/service/collaboration_comments_edit.go
server/internal/service/collaboration_comments_view.go
server/internal/service/collaboration_mentions.go
server/internal/service/collaboration_queue.go
server/internal/storage/collaboration_private.go
server/internal/storage/collaboration_private_test.go
server/internal/storage/local.go
server/migrations/727_collaboration_attachment_delivery.up.sql
server/migrations/727_collaboration_attachment_delivery.down.sql
server/migrations/728_collaboration_attachment_upload_request.up.sql
server/migrations/728_collaboration_attachment_upload_request.down.sql
server/pkg/db/queries/collaboration.sql
server/pkg/db/queries/collaboration_attachments.sql
server/pkg/db/generated/collaboration.sql.go
server/pkg/db/generated/collaboration_attachments.sql.go
server/pkg/db/generated/models.go
```
