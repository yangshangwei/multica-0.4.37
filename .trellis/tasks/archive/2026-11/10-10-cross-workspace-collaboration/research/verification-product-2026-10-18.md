# Product continuation independent review

Reviewer: `product_check` (`trellis-check`), 2026-10-18. Baseline:
`4dd643126f4ade9fa70e1dfa90cad0ba21350425`. Review includes tracked changes and
new files. The parent task remains `in_progress`; this review does not enable
creation or execution and does not claim complete AC01–AC97 acceptance.

## Review plan

1. Read the full saved role context, current task contracts, lane handoffs,
   root coding rules, applicable package specs and the full server security
   boundaries. Trace current code rather than treating handoff claims as proof.
2. Review public creation and controlled attachment storage, replay, binding,
   references, native capabilities and ordinary-file exclusion. Check current
   authority before and after external storage work, scope/existence masking,
   bounded parsing and the retention of unknown commits and delivered history.
3. Review resource/participant projections and original-operation recovery,
   including actual owner identity, inactive participants and strict schemas.
4. Review account/issue realtime authentication, final-send authorization,
   durable outbox handling, current read bases and client access fencing.
5. Verify migrations 727/728 and runner invalid-index recovery without changing
   the application database. Add focused regressions for confirmed local defects
   before fixing them; coordinate SQL/generated/shared in-flight files with the
   parent. Do not change product decisions or public interfaces speculatively.
6. Run focused Go race/vet and core lint/typecheck/tests, saving raw logs in the
   task evidence directory. The parent owns full-suite/static and native UI
   execution. Extend the review to creation/attachment client changes only after
   the parent sends their completed checkpoint.

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.
Reviewer database: `cwc_product_integration_nebz79rl` on loopback port 51974.
All Go test/vet commands use `scripts/go-test-with-agent-cli-guard.sh` and no
ambient or paid AI executable. No commits or process/environment changes.

## Findings fixed

1. **Whole-draft consumption and context were not enforced.**
   `bindCollaborationAttachments` consumed individual files only. Two concurrent
   creation requests could attach A and B from one draft to separate issues;
   another upload could also commit after the draft had been consumed. A draft's
   intended issue could change after its last file was discarded. The parent
   confirmed that the approved contract means the entire upload group.
   `collaboration_attachments.sql` now reads all retained current-upload
   provenance, including delivered/discarded rows. Under the existing uploader
   user-row lock, the first delivered row permanently consumes that draft and
   the original source/intended issue remains fixed. Upload checks repeat after
   external storage work; only the losing attempt's object is removed. Existing
   upload/create receipts and same-issue references without `draft_id` still work.
   No new table or migration is needed; the existing 728 index supports the
   lookup. The parent regenerated sqlc output.
   - Changed: `server/internal/service/collaboration_attachments.go`,
     `server/pkg/db/queries/collaboration_attachments.sql` and generated output.
   - Regression: `server/internal/service/collaboration_attachment_draft_test.go`
     exercises sequential/concurrent split creation, post-storage consumption,
     discarded provenance and original replay/reference behavior.
   - Evidence: `product-check-draft-red.log` reproduces the defects;
     `product-check-draft-service-green-race.log` passes after the fix.

2. **Draft metadata omitted the intended issue.** A client could not distinguish
   two still-undelivered drafts for different issues in the same source space.
   `CollaborationAttachmentView` now always exposes `intended_issue_id`, null for
   creation drafts and the fixed UUID for issue drafts. The service metadata
   regression and real upload handler assert this value. The parent delegated
   matching client schema/API tests to `delivery_client`, avoiding shared edits.
   - Changed: `server/internal/service/collaboration_attachments.go`,
     `server/internal/handler/collaboration_attachments_test.go`.

3. **Unknown WebSocket frame diagnostics logged arbitrary private input.**
   `server/internal/realtime/hub.go` logged the untrusted `type` value, even on
   the new authenticated account connection. The diagnostic now retains only
   its bounded message and user ID.
   - Regression: `server/internal/realtime/collaboration_test.go`,
     `TestAccountWebSocketUnknownFrameDoesNotLogPrivateInput`, uses a real socket
     and a following pong to prove the unknown frame was handled before checking
     logs. `product-check-ws-redaction-red.log` failed before the fix;
     `product-check-ws-redaction-green-race.log` passes afterward.

4. **New attachment rollback guards lacked a direct migration regression.**
   `server/internal/migrations/collaboration_attachment_delivery_test.go` now
   executes the actual 727/728 SQL in a private schema. It verifies a valid
   unique upload index, duplicate original-key refusal, unchanged delivered
   history after downgrade refusal, immediate refusal of an uncommitted first
   writer, and successful empty maintenance rollback. Production migration SQL
   and the existing cleanup registration required no correction.
   - Evidence: `product-check-attachment-migrations-race.log` passes.

## Findings not fixed

- General provider/runtime attachment-byte preparation and initial creation
  iteration/status fields remain outside this increment, as explicitly retained
  in the lane handoff and confirmed by the parent. Human controlled downloads
  must not become a machine-credential fallback. No public interface was extended
  for those later phases.
- The final creation/attachment client implementation and native Web/Electron
  acceptance still await the parent's completed checkpoint. This report does not
  treat the stable-core checks below as verification of in-flight client files.
  The parent has dispatched `client_check` for that frontend scope. Its review
  found a creation-only denial distinction: `account_unavailable` can name the
  selected handler, and file/recovery `not_found` can name only the file or
  operation. Those responses alone are not proof of source/issue access loss.
  Client recovery fixes stay with that lane.
- No unresolved defect was found in the reviewed resource/participant readers,
  owner-only identity projection, original resource receipts, stable resource
  forms or account/issue final-send authorization. The parent is correcting the
  new realtime spec to name `collaborationScopeAuthorizer` and acknowledge the
  existing own-user room alongside the account room.

## Verification

All log names below are relative to the evidence directory above. Go commands
ran from `server/` through the agent-CLI guard. These are reviewer executions,
not relabeled lane evidence.

| Check | Result | Log |
| --- | --- | --- |
| `go test -race ./internal/service -run '^TestCollaboration' -count=1` | PASS, 274.274 s before draft corrections | `product-check-service-race.log` |
| `go test -race ./internal/service -run '^TestCollaboration(Attachment\|Create\|Creation\|Comment)' -count=1 -v` | PASS, 9.474 s after draft/metadata corrections | `product-check-draft-service-green-race.log` |
| `go test -race ./internal/handler -run '^TestCollaboration(Attachment\|CommentAttachment\|PrivateObjects)' -count=1 -v` | PASS, 3.954 s after metadata correction | `product-check-draft-handler-green-race.log` |
| `go test -race ./internal/realtime ./internal/storage -count=1` | PASS, 2.787 / 1.831 s | `product-check-transport-storage-race.log` |
| `go test -race ./internal/realtime -count=1` after logging fix | PASS, 1.990 s | `product-check-ws-redaction-green-race.log` |
| `go test -race ./internal/migrations -run '^TestCollaborationAttachmentDeliveryMigrationsPreserveOriginalUploads$' -count=1 -v` | PASS, 2.016 s | `product-check-attachment-migrations-race.log` |
| Stable resource/participant/Switch/realtime/WS Vitest suites | PASS, 10 files / 62 tests | `product-check-core-stable-tests.log` |
| Shared resource-panel/workflow Vitest suites | PASS, 2 files / 7 tests | `product-check-resource-views-tests.log` |
| ESLint for stable changed core resource/participant/Switch/realtime files | PASS | `product-check-core-stable-lint.log` |
| Core `tsc --noEmit` | PASS at stable checkpoint | `product-check-core-typecheck.log` |
| `go vet` across affected service/handler/storage/realtime/migration/server packages | PASS before final draft edits | `product-check-go-vet.log` |
| `go vet ./internal/service ./internal/handler ./internal/migrations ./internal/realtime` after draft/metadata corrections | PASS | `product-check-final-go-vet.log` |
| `git diff --check` | PASS at reviewed checkpoints | Direct command output |

Backend corrections are verified. Full client/static and native integration
results remain owned by the parent until final review.

## Route fixture continuation

Reviewer: `route_fixture_check` (`trellis-check`), 2026-10-18. This follow-up
addresses only the four route fixture failures from `go-affected-race.jsonl`;
the completed backend review above was not repeated.

The new `TestCollaborationPublicCreationUsesOriginalAccountKey` calls the real
creation transaction. `CollaborationService.CreateInTx` ensures a durable
`collaboration_account_fence` for the shared `TestMain` user. Per-issue cleanup
correctly leaves that account-wide row in place. The edit, handoff, lifecycle
and workflow route fixtures still unconditionally inserted a fence for the same
user, so each failed with SQLSTATE 23505 before reaching its business assertions.
Running the public creation test followed by those four tests reproduced all
four failures; this was a fixture ownership assumption, not a uniqueness defect.

The four files now use `ensureCollaborationRouteAccountFence`, located beside
the existing route fixture helper in `collaboration_lifecycle_routes_test.go`.
It uses `INSERT ... ON CONFLICT (user_id) DO NOTHING` and registers fixture
cleanup only when the statement inserted a row. Reused fences retain their
epoch and are not deleted by the borrowing fixture. All business assertions,
production queries and production uniqueness constraints remain unchanged.

Changed files under `server/cmd/server/`:

- `collaboration_edit_routes_test.go`
- `collaboration_handoff_routes_test.go`
- `collaboration_lifecycle_routes_test.go`
- `collaboration_workflow_routes_test.go`

The commands used the same dedicated integration database and agent-CLI guard
as above, with `go test -race -p 2 -parallel 2 ./cmd/server -count=1 -json`.
Focused runs additionally selected the public creation test and the four named
regressions. No ambient agent CLI, application database or services were used.

| Check | Result | Evidence |
| --- | --- | --- |
| Creation followed by four old fixtures, before repair | Reproduced all four duplicate-key failures | `route-fixture-red.jsonl` |
| Same five tests after repair | PASS, 5 tests, 3.475 s | `route-fixture-green.jsonl` |
| Entire `./cmd/server` package with race detection | PASS, 79.852 s; 353 top-level tests passed, 6 expected skips (705 passing test/subtest results) | `route-package-green-race.jsonl` |
| `go vet ./cmd/server` through the guard | PASS | `route-fixture-go-vet.log` |
| `gofmt -l` for all four changed tests; scoped `git diff --check` | PASS, no output | Direct command output |
| Go type checking/compilation | PASS through both successful test runs | Test logs above |

`route-fixture-summary.json` records the result counts and exact skipped tests.
All six skips require the explicit Linux namespace integration environment;
this macOS route run does not claim that runtime coverage. TypeScript checks
remain with the frontend owner because this repair changes Go tests only.
No unresolved route defect was found, no production/spec contract changed,
and no commits were created. The integration database is released to the parent.
