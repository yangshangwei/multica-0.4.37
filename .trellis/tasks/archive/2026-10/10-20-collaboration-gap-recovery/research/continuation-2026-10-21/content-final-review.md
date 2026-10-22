# Independent AC74/88 content and commit review — 2026-10-21

Decision: **approve the two narrow content tests and the complete P3 AC74/88 behavior**, combining their current race executions with the already catalogued author/handler/reaction/legacy clauses. No product defect was found. Only this report is written by this reviewer; source, index, acceptance matrix/catalog and environment remain read-only.

## AC74: edit/delete concurrency and history

`collaboration_comment_edit_delete_test.go` exercises both public service writers with one immutable comment, a real private upload/delivery, an existing reply and nonempty synthetic command/group/run/input/dependency records. `recipientCommitStarter` reports the leading transaction's actual PostgreSQL PID immediately before commit; `waitFIFOBlocked` queries `pg_stat_activity` and `pg_blocking_pids` before releasing it. A successful second writer cannot satisfy that wait, and no sleep chooses the order.

- Edit-first commits revision 2, then the original delete returns exact `stale_comment_revision` 409. Only a new delete key with revision 2 creates revision 3 tombstone.
- Delete-first commits revision 2 tombstone, then the original edit returns exact `comment_deleted` 409. The earlier body cannot be restored by retry or by a new edit at the current tombstone revision.
- Original success/refusal receipts are replayed and read through the own-operation getter before and after final deletion. JSON comparison retains every wire field while correctly ignoring nil-versus-empty `omitempty` slice allocation.
- Public history retains exact creation/edit/deletion order, before/after body and attachment IDs, real actor, nonzero timestamps and increasing event sequence. Deletion history correlates the precise committed original/fresh key.
- Full protected-state snapshots keep issue/collaboration/session/channel versions, immutable commands/groups/runs/readers/dependencies, other audit, notifications, task queue, delivered attachment rows, frozen references and the reply unchanged. Only comment display references retire; an external participant still reads the original encrypted attachment bytes.

The unchanged `changeCollaborationComment` path first requires current `Capabilities.Comment`, then the real member author and comment type; it checks deleted state before revision and performs body/reference/history/operation writes in the shared transaction. Existing current author/non-author, independent source/participant authority, sequential author history, stale edit, lost-commit and write-failure tests retain their distinct assertions. The new race test supplies the formerly absent edit-versus-delete matrix rather than replacing them.

## AC88: current authority and explicit unresolve

`collaboration_discussion_resume_test.go` tests an external participant with no source membership and a source-only member with no participation. Both resolve and successfully unresolve the exact comment through `ChangeDiscussion`. Receipts contain explicit null emoji and correct status/action/key; one actual resolved record has the true resolver, and unresolve clears all three resolution columns. Original comment body/revision and `CommentHistory` remain unchanged.

Old resolve replay after unresolve does not resolve again. Old unresolve replay after a later creator resolve does not erase that new record. Only a fresh explicit unresolve changes it. After participation or membership revocation, both the old write key and a fresh write are denied without an operation reservation; current account-only original receipt recovery remains content-free. Machine credentials are refused.

The protected snapshot contains full comment metadata except resolution columns, issue/collaboration/session/channel state, commands/groups/runs, nondiscussion audit, notifications, source inbox and ordinary queue. Exactly four fresh discussion events appear; retries/refusals add none. Its original comment author is external, and source inbox stays empty. Existing current tests separately cover comment/issue own reaction add/remove, others' reaction preservation, actual concurrent one-thread resolution, deleted-root anchors, required-write rollback, private HTTP routes and legacy/machine denial. Thus successful unresolve closes the missing positive clause without manufacturing AI or notification evidence.

## Verified current execution and source

No full suite or native/browser run was repeated here. Parsed actual final JSONL and companions:

| Run | Exact result | Raw SHA-256 |
| --- | --- | --- |
| `content-unresolve-edit-delete-final-race-20261021.jsonl` | 13 top-level / 21 including subtests PASS, no fail/skip; package 6.543s | `4db35acd3bbdddee2de22edeccc1972ff674595bd463cc93fbf554baf358146e` |
| `content-isolated-head-race.jsonl` | 2 top-level / 6 including subtests PASS, no fail/skip; package 3.730s | `b5d9e167ca3baa7a898fb6c1d06b756a316997568cac66dbe1c9271f09e93982` |

The isolated run contains PASS for both `TestCollaborationCommentEditDeleteBothCommitOrdersRetainHistory/edit_first` (0.54s) and `/delete_first` (0.47s), plus `TestCollaborationDiscussionResolveUnresolveUsesCurrentReadBasis/participant` (0.39s) and `/source_member` (0.33s). Their parents pass. The current adjacent run passes the same cases plus all selected canonical discussion/content neighbors. `-race -p 1 -parallel 1 -count=1 -timeout=30m`, Go 1.26.6, isolated `service_s5` migration 737 and agent CLI guard are explicit in command/results; synthetic persisted runs are never called native containment.

| Commit candidate | Current and isolated SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_discussion_resume_test.go` | `27b0abb0614ef71f96645ab3feb4063c4d33c9e071542c7747cb105b7894d747` |
| `server/internal/service/collaboration_comment_edit_delete_test.go` | `37849f45948264b9c99257823935138d974797103be14ab3b4b48bc58aa0a3c2` |

All six source hashes in the final race result match current files. The current original author/comment/discussion/handler identities in `content-independent-coordinator-review.json.base_clause_checks` resolve to real PASS entries in `continuation-service-2`, `continuation-go-other` and `continuation-content-race`; their assertion boundaries were inspected, not inferred from counts.

## Exact HEAD isolation / commit boundary

`content-isolated-head-source.json` uses HEAD `aef77e9ce62b2f665f6a99676a79244b49fde497`, outside checkout at `/tmp/multica-gap-recovery-20261021-h6phxwo9/content-acceptance-head-bjg3a90e`. Read-only comparison with `git show HEAD:path` confirmed **all 4056 tracked server files are identical to that HEAD**; the only added Go sources are the two tests above, neither of which exists in HEAD. Their bytes match the worktree and manifest. No uncommitted product/helper/generated SQL was copied into the isolated execution.

The recorded argument vector runs exactly those two tests against `service_s5`; returncode 0 and raw package PASS agree. The pair is independently reviewable for an atomic test commit and does not depend on including the broad dirty collaboration implementation. At inspection the shared staging index was empty; this reviewer did not stage or commit anything.

## Findings not fixed by this report-only reviewer

One mechanical evidence linkage needs root synchronization: `continuation-content-race.artifacts` records `content-independent-coordinator-review.json` SHA `aedf94a9ba817247980e263b5ff35f94d912ee602bb9fffce0c65e62cbbebda6`, while the file after its canonical-clause addendum hashes `12364cb7133141fa8d154f4b77eba8a46fe0b6226cb620027632124404a931cd`. Its `remaining` still says isolated HEAD validation is pending, but `content-isolated-head-race-result.json` and JSONL prove it completed. Root was notified to update the wording and recorded hash together. This mismatch does not invalidate the hash-matching raw runs or the isolated source verification above; the authoritative catalog remains root-owned.

Other evidence limits remain: encrypted bytes use local task storage, groups/runs are synthetic persisted fixtures, and this lane adds no new Web/Electron content journey, real-provider/account/quota, S3/fault environment, other-OS or installed Main/installer proof. Those general acceptance boundaries are not hidden or promoted by the P3 checks.

## Reviewer verification

Commands actually run from the repository root / server:

```sh
gofmt -d server/internal/service/collaboration_discussion_resume_test.go server/internal/service/collaboration_comment_edit_delete_test.go
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Both exit 0 with empty output. Read-only Python checks confirm JSONL PASS/fail/skip counts, all current/isolated source hashes, exact isolated tracked-file equality and canonical catalog entries. Task context and report whitespace are checked after writing. Go has no TypeScript surface in this report-only slice; no frontend lint/typecheck rerun is claimed.

Changed file: this report only. Simplification: none; production/tests were not modified. Recommended outcome: keep AC74/88's reviewed P3 pass, reconcile the catalog JSON hash/wording, and commit only the verified two-test slice under the Lore protocol. Parent/child remain in progress; no rollout, environment, archive or commit action was performed here.
