# Source editing contracts review — 2026-10-19

Baseline: `5f5fb9138`. The first pass reviewed source projection/filter SQL and handlers, field/initial-creation/iteration services, core API schemas and original-request recovery, and comment/discussion UI. The final pass extended this to source UI guards and marker retention, ordinary iteration entry points, operation/WebSocket invalidation, redaction, and the coordinated corrections below. Product edits during integration runs were limited to the main session's explicitly coordinated SourceAction fix. This is an independent review of the implemented continuation, not full P1–P8 acceptance or release approval.

Loaded the complete saved SubagentStart context, current `check.jsonl`, PRD/design/implementation plan, `CLAUDE.md`, security boundaries, collaboration foundation/content/workbench contracts, and relevant iteration transaction/recovery guidelines. No SQL regeneration, task-status changes, new dependencies, or commits were performed.

## Findings (fixed)

- File: `packages/views/collaboration/comment-list.tsx:22`; regression: `packages/views/collaboration/comment-list.test.tsx:78`.
- Issue: After recovering a conflict on a comment outside the loaded page, loading that comment's page unmounted `PendingComment` and mounted a fresh `CommentCard`. Recovery had correctly retired the original operation, so the new card could not restore the unsent editor and discarded its text. The new assertion failed with the edit textbox absent.
- Fix: Both individually restored and paged comments now render through one keyed `CommentItem` list. The card keeps its component identity and editor while its data source changes. The existing regression now checks that the draft and explicit version-review action survive paging. Focused comment lint and all seven comment list/composer tests pass.

### Coordinated integration corrections

1. **Mixed source response privacy — fixed by main.** `packages/core/api/client.ts` now redacts mixed detail/list/query/group/table/search/child schema failures, strips private query values from transport logs, uses bounded error-log text, and replaces JSON syntax errors that could quote response content. `api/issue-source-redaction.test.ts` exercises these paths with private sentinels; its 12 tests pass. The final code no longer logs the collaboration body on malformed marker responses.

2. **Original emoji correlation — fixed by comment owner.** `server/internal/service/collaboration_discussion.go` persists/projects the exact emoji, with explicit null for resolution. Missing or malformed stored emoji remains unavailable. `discussion-schema.ts`, `discussion.ts` and `ApiClient` validate the original issue/comment/request/action/emoji for immediate and HTTP 200 GET recovery. Impossible `comment_deleted` refusals for unresolve/remove are rejected. The original request stays pending for missing or mismatched outcomes.

3. **UTF-8 comment edit limits and durable refusal — fixed by comment owner.** New edits validate the actual 65,536 UTF-8 byte limit before saving/sending and keep oversized input editable. Existing oversized originals remain readable and can only replay their exact saved payload; the server seals a content-free refusal in the authorized original operation without changing content/history/files/notifications/AI. Retirement requires the correlated HTTP 200 own-result GET. Comment PATCH alone accepts a 512 KiB JSON envelope for escaping and old drafts; the accepted content limit and other workflow envelope limits are unchanged. Core, HTTP and service tests cover exact byte boundaries, oversized originals, 202/missing results and retained drafts.

4. **Source action drafts on background failure — fixed by reviewer with main coordination.** `packages/views/collaboration/source-bridge.tsx` previously replaced `CollaborationWorkflow` whenever the detail query had an error, including a background 500 with cached authorized data. The new guard keeps the workflow mounted and adds a retry notice for transient failures; authoritative denial still removes it. `source-bridge.test.tsx` failed before the fix and now preserves dirty form state across 500 while removing it on 403; all six tests and scoped ESLint pass.

5. **R50 ordinary iteration move bypass — fixed by field owner.** The ordinary iteration preview/apply path previously checked only source membership and could change a collaboration issue without its creator/handler authority, collaboration version or event. `server/internal/service/iteration_lifecycle_preview.go` now rejects an explicit `move` containing any collaboration issue in the shared preparation path, before reference capture and writes. Apply rechecks current locked identity even after an older ordinary preview. A mixed selection is fully refused; ordinary moves and whole-period closure/rollover retain their original behavior. Service and HTTP `iteration_collaboration_boundary_test.go` cover join/switch/remove, old previews, rollback and automatic shared rollover; the final Go log records passes.

6. **Duplicate comment DOM during refetch — fixed by comment owner.** `CollaborationDetail` gave the comment list and composer the same sibling key. Real Web verification exposed accumulated lists and duplicate comment anchors. Their keys now use distinct `comments:` and `composer:` prefixes. The detail regression failed before the fix and passes after loading/polling/first-comment updates, preserves the composer draft and resets it only on issue change. The original reviewer paging fix remains intact.

7. **Parent-search character limit — fixed by field owner.** The UI permits 200 characters, while the initial handler/service limited the query to 200 UTF-8 bytes; long Chinese input could make the creation search fail. Both boundaries now use `utf8.RuneCountInString` with the same 200-character bound. The reviewer inspected both checks and the new service/HTTP cases for 200 Chinese/ASCII characters and rejection at 201. The source spec matches the revised read-query bound; comment content remains byte-limited. The owner's focused Go run was still in progress at this report's final freeze, so its outcome belongs to the final verification record.

## Findings (not fixed)

No unresolved code-review findings remain in the implemented source/editing/discussion slice after these corrections. The approved optional-marker limitation below remains explicit. Final production/native acceptance and the last character-limit test result are verification work owned by main, not evidence of a code defect or completion of the parent product.

## Clarified protocol limitation

The initial missing-marker finding is withdrawn as an unfinished defect. The main session clarified the approved backward-compatible wire contract: absent/null `collaboration` means ordinary; a present malformed marker fails parsing. Rejecting every absent marker would break ordinary responses from older backends and would require an unapproved classification protocol.

On reassessment, no current server read path in this review was found to emit an ordinary row for an existing collaboration relationship. Search, open/regular/grouped lists, detail, child lists, structured table rows/groups and the permitted reorder response call the source relationship projection. `collaboration_source.go:33` returns unavailable on query failure; lines 39–42 do the same for a missing creator/handler account. The relationship cannot be inferred from a cold legacy response that genuinely omits the optional field; that remains a known protocol limitation, not a release-blocking defect from this pass. The detail boundary now consults `findSourceIssue` to retain an already-known collaboration marker across partial responses. No blanket absence rejection or new wire discriminator was introduced.

## Reviewed invariants

- Source machine private filters are excluded before list counts/windows; structured table queries exclude collaboration rows before their membership CTEs, counts and facets. The external-participant filter remains effective after responsibility returns to a source member and stops matching revoked participants.
- Generic collaboration writers remain rejected except the existing source-only position operation. New source account summaries omit email/username.
- Field edits retain the workflow transaction, source membership/role checks and reference validation. Structure plus content uses one ordinary revision increment; failed validation precedes business writes. Source catalogs are emitted only for source editors, while external readers receive selected references.
- Iteration changes borrow `PrepareMembershipChange`/`CommitMembershipChange`, lock source/target before the issue, require versions/reasons, preserve queue state and use durable workflow receipts. Creation keeps selected fields inside the existing savepoint and original-key operation.
- Discussion writes share the issue lock, retain comment content revisions, serialize thread resolution, modify only the actor's reactions and emit no collaboration/source inbox notifications or AI inputs. Existing edit/delete history, tombstones, replies and delivered attachment references remain intact.
- Core comment changes verify the original issue/comment/request/action and retain uncertain operations. Access-epoch protection wraps both successful and error responses. Dedicated collaboration parsers remain redacted.
- Shared operations and account/issue events invalidate source issue/project/iteration windows rather than copying account DTOs into ordinary caches. The source workspace outbox publishes only an empty `collaboration:workbench_changed` tick with stable `:source` delivery identity; failed source relay retains the event. The workspace client accepts only an empty payload and refreshes its currently authorized source queries.
- Ordinary label/property/reaction/move writes still enter the guarded issue loader. The only ordinary collaboration mutation remains the source-only position operation; the separately discovered iteration-maintenance bypass is now guarded above.

## Verification

Evidence directory: `.omx/reports/cwc-source-editing-20261019T035920Z/`.

- Core targeted tests: **pass**, 5 files / 29 tests (`review-contracts-core.log`): comment actions, discussion, field actions/options and source schema/actions.
- Comment UI tests after fix: **pass**, 2 files / 7 tests (`review-comment-paging-after.log`). The added paging assertion failed before the fix (`review-comment-paging-before.log`).
- Go race: **pass**, service and handler packages, selected source structure/iteration/initial creation/discussion/comment tests (`review-contracts-go.log`). Ran serial packages against the completed comments lane's isolated `comments.env`; did not use the main integration DB or installed AI CLIs.
- Go vet: **pass** for `./internal/handler ./internal/service` (`review-contracts-vet.log`, exit 0).
- Lint: **pass** for core/UI and focused completed comment files. The broader views run failed in the active source lane's missing table icon imports (`review-contracts-lint.log`); these were reported, not treated as completed-code findings.
- TypeCheck: **pass** for core/UI. The repeat views check failed only in active-lane tests: `collaboration/field-editing.test.tsx:164` (empty mock tuple) and `issues/components/list-view.test.tsx:384` (`never` call), recorded in `review-contracts-typecheck-final.log`. Active owners/main must complete the final integrated check.
- `git diff --check -- server packages/core packages/ui packages/views/collaboration`: **pass**.
- Browser/native Electron, full repository checks and final field/source UI review remain with the main session.

Final integration evidence was inspected instead of rerunning broad suites while main owned the API/Web/Desktop verification environment:

- **Lint and TypeCheck: pass**, `static-final.log`, 15/15 uncached tasks. This supersedes the first pass's transient active-lane errors.
- **Full TypeScript tests: pass**, `typescript-final.log`, 5/5 tasks, 1,010 files and 12,162 tests.
- **Reviewer SourceAction fix: pass**, 6 tests (`review-source-action-after.log`) and scoped ESLint; the new assertion failed in `review-source-action-before.log`.
- **Comment corrections: pass**, 42 core tests (`comments-review-fixes-core.log`), 9 comment UI tests (`comments-review-fixes-views.log`), then 10 list/composer/detail tests after the duplicate-key fix (`comment-dom-unit-after.log`). The production failure evidence is retained in `comment-dom-production-before.json` and the unit failure in `comment-dom-unit-before.log`.
- **Redaction: pass**, 12 tests (`source-redaction-final.log`).
- **Go integration:** `go-iteration-final.jsonl` has passing service/handler packages and all new R50 boundary/rollover cases. `comments-review-fixes-go-final.log` records passing comment/discussion/UTF-8 service/handler race tests. The broader `go-final.jsonl` had one built-in skill template-budget failure, then `builtin-budget-final.log` confirms its focused correction passed; other recorded packages passed. These are owner-run results, not claims that this reviewer reran the broad suite.
- Final production Web/native Electron acceptance remains owned by main; this report does not mark those scenarios or parent ACs complete.

## Spec synchronization for integration

The main session added `.trellis/spec/server/collaboration-source-editing.md`, linked it from the server/core indexes and updated the built-in working-on-issues skill/source map. The contract covers the optional marker, source-only editing, guarded ordinary moves, original emoji/UTF-8 outcomes, source invalidation and paging identity. Its parent-search wording matches the final character-bound correction. The parent task and unverified broader ACs/gates remain pending.

The reviewer has finished product and review-document edits for the final build fingerprint. Subsequent integration evidence and task status are owned by main.
