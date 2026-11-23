# AC74/AC88 content verification — 2026-10-21

Two bounded S6 evidence gaps now have current database-backed race checks:
authorized resolve/unresolve succeeds for both independent comment read bases,
and edit versus delete is serialized in both controlled commit orderings. All
13 focused/adjacent top-level tests pass, with zero failures or skips. This lane
changes only two new test files and evidence artifacts; product source and
existing test helpers remain unchanged.

## AC88: successful authorized unresolve

New canonical identity:
`internal/service::TestCollaborationDiscussionResolveUnresolveUsesCurrentReadBasis`.

Its `participant` and `source_member` cases use
`newCollaborationContentFixture`. The external participant must have no source
membership. The source-only member must have no issue participation. A real
external human comment creates an immutable AI command, and the existing
synthetic persisted group/run fixture establishes nonempty execution state.
This fixture never launches a provider or claims native termination evidence.

Both cases call public `ChangeDiscussion` to resolve then unresolve the exact
comment. Assertions require successful `comment_resolved` /
`comment_unresolved` receipts, explicit nil emoji, the real resolver identity,
one resolved comment, and cleared `resolved_at`, `resolved_by_type`, and
`resolved_by_id` after unresolve. The content stays at revision 1 with its
original body and unchanged public `CommentHistory`.

Original-key replay and `GetDiscussionOperation` recover the exact action and
receipt. Replaying the old resolve after unresolve must not resolve again;
replaying old unresolve after a later creator resolve must not erase that later
resolution. Only a fresh explicit unresolve clears it. Revoking participation
or source membership denies both old-key replay and a new unresolve request,
without reserving an operation. The actor may still recover its content-free
own receipt. Machine credentials return `forbidden`.

An ordered snapshot stays identical after every successful action, replay and
refusal, excluding only the defined resolution metadata and event sequence.
It contains full comment content metadata, issue/collaboration versions,
session/channel controls, immutable commands, retained groups/runs,
non-discussion audit events, collaboration notifications, source inbox and
legacy task queue. Exactly four discussion events are produced by the four
fresh successful state changes; replays/refusals add none. The source inbox
remains empty even though the original comment author is external.

AC88 clauses supported by this new test: both participant/source-only positive
resolve/unresolve, real actor and one record, original-key recovery/replay,
current authority, machine denial, unchanged content/history/AI/other audit,
no R52 notification or source inbox side effect. Own issue/comment reaction
add/remove, concurrent one-thread resolution, deleted-root behavior and required
write rollback are covered by the adjacent canonical discussion tests in the
same final run. The already-recorded handler tests cover the private routes and
legacy/machine boundary; they were not rerun by this lane.

## AC74: deterministic edit/delete concurrency

New canonical identity:
`internal/service::TestCollaborationCommentEditDeleteBothCommitOrdersRetainHistory`.

Both cases use a real private encrypted upload, public comment delivery with
issue/comment references, a frozen AI command and synthetic persisted
group/input/dependency/run records. A reply points at that original comment.

The leading public `EditComment` or `DeleteComment` call runs with existing
`recipientCommitStarter`. It pauses immediately before commit while database
locks remain held. The following public writer starts before release;
`waitFIFOBlocked` observes its actual PostgreSQL wait via
`pg_stat_activity`/`pg_blocking_pids`. There is no timed sleep or guessed order.

| Controlled order | Required committed result |
| --- | --- |
| Edit first | Edit commits revision 2; original delete returns HTTP 409 `stale_comment_revision`. Its old key keeps that refusal. An explicitly new delete key at revision 2 creates revision 3 tombstone. |
| Delete first | Delete commits revision 2 tombstone; the original edit returns HTTP 409 `comment_deleted`. Its old key keeps that refusal and cannot recreate content. |

Success/refusal receipts are compared by exact JSON for both POST retry and own
operation GET, including identity/revision/event/request fields. Empty
`AttachmentIDs` may be nil or allocated empty in memory because `omitempty`
produces the same persisted JSON; that internal representation is not a product
contract. Both original keys are replayed again after the final tombstone and
must retain their earlier outcomes. A fresh edit at the tombstone revision is
also refused.

Public `GetComment` must retain a deletion timestamp, empty display body,
expected revision and no edit affordance/current attachment display. Public
`CommentHistory` preserves exactly the create/edit/delete sequence appropriate
to the ordering, original before/after bodies, exact attachment versions and
real actor/time/event sequence. The deletion event must point at the exact
successful original delete key, including the explicitly fresh key in edit-first.

An identical ordered snapshot preserves issue/collaboration versions, session
and channel controls, immutable AI command/group/run/input/dependency records,
all unrelated audit events, notifications, ordinary task queue, delivered
attachment rows, non-comment attachment references and the reply row. Display
references are tombstoned, never delivery itself. The external participant
still reads the exact original private attachment bytes after both races and
all retries.

AC74 clauses supported by this new test: both actual database edit/delete
contention orderings, exact original-key success/refusal recovery, no
resurrection, complete history/operator/time, deletion marker/reply anchor,
attachment retention, unchanged frozen AI/input/execution/audit/notifications.
Canonical author/non-author checks, sequential history and lost-response/write
rollback tests were included in the final adjacent run. The existing broader
current-task handler/authority evidence remains the source for other revoked
author and platform UI clauses; this lane does not independently claim a new
Web/Electron content journey.

## Commands, attempts and provenance

Commands ran from
`/Volumes/artisan/code/2026/multica-0.4.37/server`, with Go 1.26.6 on
darwin/arm64. The explicit launcher injects isolated `service_s5`, whose schema
is through migration 737. Agent CLI sentinels were active. Upload storage is a
test-owned temporary directory and test `t.Setenv` values are scoped; no
application environment, production gate, browser/native slot or provider
account was changed.

Initial focused command:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^(TestCollaborationDiscussionResolveUnresolveUsesCurrentReadBasis|TestCollaborationCommentEditDeleteBothCommitOrdersRetainHistory)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

All attempts remain retained, without renaming failures as green:

| Attempt | Actual result | Classification / artifact |
| --- | --- | --- |
| First focused | Exit 1 before any test; unused `pgtype` import in the new discussion test | Build-only test authoring error; `content-unresolve-edit-delete-focused-first-20261021*` |
| Second focused | Both resolve/unresolve cases and edit-first pass; delete-first fails a Go receipt `DeepEqual` on nil versus empty attachments | Test comparison mismatch after JSON `omitempty`; exact wire fields agree. `content-unresolve-edit-delete-focused-second-20261021*`, package 2.507s |
| Final focused plus adjacent | Exit 0, 13 top-level passes (21 pass events including subtests), zero failures, zero skips | `content-unresolve-edit-delete-final-race-20261021*`, package 6.543s |

Only new test code was corrected after the initial attempts: remove unused
import; compare original receipt JSON rather than slice allocation; strengthen
the exact successful delete key assertion. No production defect was found and
no production fix or rollback was performed.

Final race command:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^(TestCollaborationDiscussion|TestCollaborationCommentAuthorEditRetainsHistoryAndDeletedReplyAnchor|TestCollaborationCommentEditsSerializeAndUnknownCommitDoesNotRepeat|TestCollaborationCommentEditFailureRollsBackBodyHistoryAndNotification|TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing|TestCollaborationCommentEditDeleteBothCommitOrdersRetainHistory)' \
  -p 1 -parallel 1 -count=1 -timeout=30m
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Vet exits 0 with empty diagnostics in 2.258s; command/result/streams are in
`content-unresolve-edit-delete-vet-20261021*`. From repository root,
`gofmt -d server/internal/service/collaboration_discussion_resume_test.go server/internal/service/collaboration_comment_edit_delete_test.go`
returns an empty diff. Final scope artifacts retain the command, source hashes
and no-trailing-whitespace checks.

Final source SHA-256:

| New test file | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_discussion_resume_test.go` | `27b0abb0614ef71f96645ab3feb4063c4d33c9e071542c7747cb105b7894d747` |
| `server/internal/service/collaboration_comment_edit_delete_test.go` | `37849f45948264b9c99257823935138d974797103be14ab3b4b48bc58aa0a3c2` |

The baseline artifact hashes eight existing product/helper files. All remain
unchanged, and the final run's six recorded source hashes stay identical during
execution. Exact commands, UTC timestamps, source/environment and return codes
are in each `-command.json` / `-result.json` companion.

## Remaining boundaries

- These two new identities close the concrete AC74 edit-vs-delete and AC88
  authorized unresolve gaps recorded in the independent ledger review. The
  coordinator must combine them with the other existing primary-owner clauses
  and independently review any acceptance promotion.
- Persisted run fixtures prove that content writes leave retained execution
  data unchanged; they do not prove native containment or paid provider behavior.
- Attachment bytes use real local encryption/storage. External object storage,
  packaged clients, new Web/Electron content journeys and production rollout
  were not tested here.
- No new full `make check`, commit, archive, spec/matrix/progress edit or new
  Trellis task was performed. Existing source and shared dirty work are preserved.

Changed files: the two new canonical service test files, this report and unique
`content-unresolve-edit-delete-*` evidence logs. The tests reuse existing
fixture/transaction barriers; no production abstraction or dependency was added.

## Coordinator independent exact-HEAD confirmation

Exact HEAD `aef77e9ce` plus only the two new content tests was archived outside
the shared checkout. The same two canonical race identities pass there with
real `service_s5` and the CLI guard, zero failures/skips; no shared S3b source,
generated SQL or new product implementation was copied. The exact argument
vector, source/archive hashes and results are in `content-isolated-head-source.json`
and `content-isolated-head-race-result.json`/JSONL/stderr.
The coordinator also inspected the full test assertions, deterministic lock
barriers, original-key/tombstone/history/attachment snapshots and both independent
comment ACL bases. Current canonical author/reaction/handler/legacy tests already
have exact PASS entries in the catalog; this closes the two specific missing
AC74/88 clauses while retaining provider/native/packaged limits above.
