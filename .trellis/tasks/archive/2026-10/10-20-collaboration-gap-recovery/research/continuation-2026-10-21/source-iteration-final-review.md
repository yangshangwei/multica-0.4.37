# Source/iteration final independent review — 2026-10-21

The two-file slice is verified and ready for an isolated Lore commit. **AC80 and AC81 remain `not_verified_in_this_task`** because their direct role and request/reason negative clauses are not yet completely exercised. No product defect, failing assertion or mechanical repair was found. This review preserves the existing task/plan, shared source, production gates and acceptance ledger.

## Findings (fixed)

None. Both new files are gofmt-clean and their exact source hashes match the first focused runs, current adjacency and isolated HEAD runs. Product/helpers protected by the implementer's baseline remain unchanged.

## Findings (not fixed)

These are missing exact acceptance evidence, not observed product failures. They are outside this reviewer lane's write scope; the coordinator will use a separate bounded implementation follow-up.

1. **AC80 direct structure-role negatives.** `TestCollaborationEditRejectsNonEditors` (`collaboration_edit_test.go:116`) supplies title/labels/properties and therefore enters `edit_issue`. It cannot stand in for a source-only ordinary member or other collaborator attempting project/parent edits through `edit_source_structure` (`collaboration_edit.go:34`). Existing FieldOptions rejects a source owner and former handler, but does not execute every stated source-member/other-collaborator structure/catalog case. Add direct structure requests and exact unchanged business/history assertions.
2. **AC81 direct iteration-role negatives.** The canonical ChangeIteration cases and new races reject external/departed/former handlers; they do not submit this dedicated mutation as an ordinary source-only member or other nonmanager collaborator. `TestIterationMovePreviewRejectsCollaborationForSourceMembers` is the ordinary unsupported `move` API and cannot replace `change_iteration` authority. Add the direct role/catalog matrix.
3. **AC81 missing removal reason.** `collaboration_source_fields_test.go:124` tests a switch without a reason and sealed original refusal; the remove at line150 supplies `No longer planned`. The production shared reason predicate (`collaboration_iteration.go:74`) is source support, not an executed no-reason remove assertion. Add remove refusal, unchanged membership/history and original-key recovery.
4. **AC81 mixed iteration PUT.** The PATCH+iteration negative is currently executed in `TestCollaborationEditHTTPStrictFieldsAndVersions/iteration`. `UpdateCollaborationIteration`'s strict DTO (`collaboration_source_fields.go:43`) suggests reverse rejection, but no current test sends title/project/etc. with that PUT. A successful metadata PATCH followed by separate iteration PUT is not a mixed-PUT refusal test.

AC85 remains a separate P6 client criterion. The new iteration HTTP test does not establish display name/avatar/“external” markers or the no-Unknown list/board/timeline/comment behavior and drag matrix.

## Deterministic transaction review

`sourceAuthorityCommitOrder` (`collaboration_source_authority_race_test.go:36`) intercepts the leading real writer immediately before its actual pgx commit through the tracked `recipientCommitStarter`. The following real mutation is started before release. The tracked `waitWorkflowBlocked` (`collaboration_workflow_test.go:454`) queries `pg_blocking_pids` against the **exact leading backend PID**, fails if the writer bypasses the lock or the wait never occurs, then releases the commit. Both transfer orderings and both target-iteration metadata orderings use this barrier. There are no arbitrary sleeps or fabricated response objects.

For departure-first, the test holds the actual `member` row `FOR UPDATE`, starts the source writer, proves the PostgreSQL wait, deletes that member and commits; the following source mutation returns forbidden. For source-change-first, the real member deletion must wait until the source writer commits. Production `LockCollaborationSourceMember` uses `FOR KEY SHARE`; `workflowRoleAllowed` also requires the transaction's `sourceMemberLocked` plus reloaded current-manager capability. Transfer-first and iteration-edit-first assertions verify current handler and target revision rechecks and no partial project/parent/title or membership/history writes.

The existing wait helper observes a waiter blocked by that exact leading PID rather than exporting the following transaction's PID. The actual executions use serial packages/tests and isolated task fixtures. This is real database contention evidence; no elapsed sleep is used as proof.

The source snapshot excludes only session event sequence because the authorized human write may advance it. It compares complete persisted session/control/channel/target/command/group/run/grant/input rows. These are synthetic active-channel/in-flight-input/group/run fixtures, with no permit or provider launch; the tracked run helper can retain default `preparing` progress. They prove retained AI data is unchanged, not natural process execution or kernel proof.

## Exact clause recommendations

| Criterion clause | Recommendation | Executed evidence |
| --- | --- | --- |
| AC80 initial project/parent creation and no AI | Passed | `TestCollaborationCreationInitialFieldsAreOneRecoverableIssue` |
| AC80 source-member creator/current noncreator handler changes and clear | Passed | `TestCollaborationSourceStructureAuthorityAndAtomicEdit`; new structure source-change-first race; `TestCollaborationSourceFieldsHTTPPreservesNullAndSeparatesStatus` |
| AC80 source validation, cycle and partial-write refusal | Passed for executed contract | `TestCollaborationSourceStructureRejectsCyclesAndMixedFailure`; `TestCollaborationCreationRejectionSelectionAndSourceFailuresAreAtomic` missing-parent/project/triage cases |
| AC80 retained edit event/replay and unchanged AI records | Passed | Canonical source edit/replay plus exact new persisted execution snapshots |
| AC80 external/departed handler write/catalog denial and minimal references | Passed | Canonical external/departed source cases; new departure races and external iteration detail/options |
| AC80 source-only ordinary member / other collaborator direct structure/catalog negatives | **Pending** | Generic metadata noneditor cases do not execute this branch |
| AC80 both transfer/departure commit orders, no stale or partial writes | Passed | Both `structure` orders of each new race identity |
| AC81 initial/join/switch/remove with supplied reasons, revisions and history | Passed | `TestCollaborationIterationAssignmentVersionsReasonsAndReplay`; initial creation; completed confirmation; initial stale refusal |
| AC81 switch missing reason and immutable original refusal | Passed | Canonical switch refusal/replay |
| AC81 remove missing reason | **Pending** | Remove currently supplies a reason |
| AC81 source-only ordinary member / other collaborator direct iteration/catalog negatives | **Pending** | Ordinary IterationService move and generic metadata cases are separate domains |
| AC81 external/departed handler denied writes/catalogs; bounded references | Passed | Canonical external assignment; new iteration departure/transfer races; new handler detail/read negatives |
| AC81 source-owned statistics/preview/history/rollover for an external handler, without private account fields | Passed at P1 | `TestCollaborationExternalHandlerIterationStatisticsPreviewAndHistory` |
| AC81 metadata PATCH rejects iteration membership atomically | Passed | `TestCollaborationEditHTTPStrictFieldsAndVersions/iteration` |
| AC81 iteration PUT rejects accompanying metadata atomically | **Pending** | No exact executed mixed PUT |
| AC81 transfer, source departure and target-iteration-version contention | Passed | Both `iteration` orders of each authority race and both target-revision orders |
| AC81 executed failed/accepted membership preserves history/control/AI | Passed for the tested cases | New snapshots, original-key replay and initial stale/target-revision nonpartial assertions |

The standalone JSON supplies per-clause source anchors and exact test identity/event lines. These subclause recommendations do not promote either whole row or relax the parent definitions.

## External identity privacy and Unknown boundary

The new handler journey uses an external handler with no source membership and unique private email/username sentinels. Source-authorized HTTP start/end previews and applies remain complete, current/original statistics count the same issue, current history retains its exact assignee ID/type, closure freezes destination/counter facts, events remain readable and server-owned rollover preserves responsibility. Reads/previews/apply responses tested by the journey are `no-store` and contain no email, username or private parent body. The external account receives minimal collaboration references and no SourceOptions; ordinary source iteration reads/previews return403.

The nil assignee name and `Available=false` are the existing tenant-scoped reference contract: `iteration_history.sql:19` joins member→user only within the source workspace, and `history_capture.go:101` keeps an unavailable display reference while retaining ID/type. This satisfies AC81's usable source-owned planning/history and privacy clause; AC81 does not grant source iteration maintenance to the external account. It also does not require widening that member directory.

There is a concrete UI boundary: `iteration-scope-details.tsx:54–59` displays its `unknownHistory` fallback for nil assignee names; `iteration-operation.tsx:377` uses `none`. No browser rendering, external display name/avatar/marker, no-Unknown or drag behavior is established here. Those limitations remain explicit under AC85 and are not silently accepted or repaired by this backend review.

## Verification

Go1.26.6, darwin/arm64, agent CLI guard and task-owned `service_s5` schema737 for new/current adjacency and isolated snapshot. Commands/results/raw events are preserved in the adjacent source-authority manifests; their exact argv, hashes, source provenance and PASS event lines are in `source-iteration-final-review.json`.

| Actual run | Result |
| --- | --- |
| First focused service race | Exit0; 3 top-level /13 including subtests passed; no fail/skip; package6.033s |
| First external-handler HTTP race | Exit0; 1 top-level passed; no fail/skip; package2.485s |
| Coordinator current canonical adjacency | Exit0; 12 top-level /22 including subtests passed; no fail/skip; wall19.35s |
| Exact HEAD archive + only the two new files | Exit0; 4 top-level /14 including subtests passed; no fail/skip; wall55.113s |
| `go vet -p 1 ./internal/service ./internal/handler` | Pass, empty diagnostics, 3.675s; implementer's exact-source artifact inspected |
| Reviewer `gofmt -d` on the two new files | Pass, exit0, no stdout/stderr |

Current partitioned canonical execution also has actual PASS events for source fields/initial creation/iteration, initial source refusal, source options, PATCH strict fields, ordinary mixed-move refusal and ordinary collaboration rollover. Relevant raw SHA-256 values are `go-service-partition-2.jsonl` = `1a1edb5507cf4a6b4259194d2d8abfe5c1d2ebb9f56672f52353d999ef07d99d`, `go-other-packages.jsonl` = `1e71354c2c8743c79c02081dba8450981af0e2b91c699afa1ded78cb470e37f8`, and `go-remaining-packages.jsonl` = `de59266def0bf98ea12d38d50339d84834f043ccc16e5ddf01339ec98f21bdd4`. Their exact case/event lines are attached; broad package counts are not acceptance.

Lint: Go formatting and vet pass. TypeCheck: actual Go package compilation in race runs passes; no TypeScript product change in this slice. Tests: all cited new/current adjacency identities pass. This reviewer ran no DB/build/Playwright/runtime suite; the coordinator owns those executions.

## Standalone commit and spec handoff

Verified outside snapshot: HEAD `5baf8659296a4a04666e6eaaa60e5965ffc70725` plus only:

- `server/internal/service/collaboration_source_authority_race_test.go` — SHA-256 `992d1afb1ee9d71e48daabffe682a6dc034fe593beac97b101ef37dbe9c0f979`.
- `server/internal/handler/collaboration_external_iteration_test.go` — SHA-256 `fe5b0bfe6ac6d377238e40c3ec1475f84cb17eb9c83983cfb3d830079b1a98f7`.

Existing queue/source-field/execution/recipient commit/waiter and HTTP iteration fixtures are tracked dependencies present in that HEAD. The isolated pass confirms no uncommitted product/helper dependency is necessary for the two-file source slice. Its environment uses schema737 and synthetic fixtures; this does not claim validation against an older schema.

The coordinator may commit these two test files and their scoped evidence using Lore, listing the actual commands as Tested and the role/reason/mixed-PUT/AC85/native/provider boundaries as Not-tested. Keep follow-up tests, shared product hunks and acceptance promotions separate. The coordinator may append this verified transaction/reference contract to `collaboration-source-editing.md`; no public interface or template change was made, and no existing spec is changed by this reviewer.

Changed files in this lane: this report and `source-iteration-final-review.json` only. Simplification: reused tracked fixtures and canonical primary-layer evidence; no production abstraction or dependency added. Remaining risks: exact pending clauses above, synthetic AI persistence scope, source-directory nullable-name UI boundary and unrun native/provider/packaging environments. Neither task nor parent is archived, and production creation/execution gates remain closed.
