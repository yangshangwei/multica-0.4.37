# T1 technical design review

Reviewed 2026-10-04 against the complete T1 source PRD, shared `WM-AC` rules, task PRD/design/API/implementation plan, all three research maps, and current transaction/authority code. This is a design review, not implementation or test evidence.

## Verdict

**PASS — ready for implementation.** Re-read the amended `design.md`, `api-contract.md` and `verification.md`: shared lock order, user UUID identity, deleted-result replay tombstones and FR-only evidence obligations are now explicit. The Issue-plus-admission model, candidate separation, default-off rollout, authoritative execution guards, and durable action/task identity fit T1. No unresolved design blocker remains; the tests and interaction checks below remain implementation acceptance obligations, not completed evidence.

## Findings and required resolutions

| Priority | Finding | Required resolution / evidence |
| --- | --- | --- |
| High — resolved in current design/API | An Item-only history query loses old completed rounds after reopen and cannot represent CSV batch summaries. This misses FR-17, FR-26 and the history page. | Added global `/triage/history` with immutable action entries and stable import summaries. Include explicit round/source/result information in historical DTOs or snapshots; test filtering an old rejection after the issue is reopened and accepted, plus a batch with no currently finalized items. |
| High — resolved in current design/API | The batch commit accepted unrestricted `ActionInput`, despite preview only allowing four actions. A direct request could reach batch execute/duplicate/reopen. | Both routes now explicitly enforce accept/reject/snooze/assign_reviewer. Test forged commit directly, without preview. |
| High — resolved in current design | Retrying accepted execution against a newly changed assignee/project could spend resources under a different decision. | Persist authorized assignee, resolved squad leader, project/resource and execution-context identity; compare before preparation and again at enqueue. Context drift conflicts and requires a fresh explicit normal execution decision. Known task IDs remain terminal replay results, never a new enqueue. |
| High — resolved in current design | A current-state-only comment check lets a delayed pending-era mention execute after acceptance. | Persist admission-at-comment-time dispatch eligibility and guard all dispatch/recovery paths. Test event delivery after ordinary acceptance, including existing planned-input/merge retries. |
| High — resolved in current design | Machine-credential checks alone miss legacy resolved agent actors carried by a human PAT. | Review/config/import must check both credential class and resolved actor, then current member/role in the transaction. Test PAT plus legacy agent headers as well as task/cloud credentials and ordinary human control. |
| High — resolved in current design | The former general order said issue then action, while execution prose could be read as action then issue; member locks were not ordered against revocation. | The amended shared order now preserves workspace/settings fences, subscriber/revoke advisory-before-member order, and agent-before-sorted-issue ordering. Action/import results follow owners. NOWAIT/sorted-owner bounded retries apply only to definitively rolled-back contention, never uncertain commits. Still prove execute/delete, execute/revoke and concurrent retry with barriers. |
| Medium — resolved in current design/API | `responsibility_member_id` and `reviewer_id` previously did not distinguish user UUID from membership-row UUID. | Both are now explicitly user UUIDs, validated through current workspace membership. Use distinct user/member IDs in fixtures and prove departed-member rendering. |
| High — resolved in current design/API | Generic issue cleanup could erase successful import-row/request identity and let a retry recreate a deleted input. | Consumed request tombstones and successful import-row/external-ID provenance now survive issue deletion. Deleted-result replay returns a conflict/reference without recreation; workspace deletion removes all workspace-owned rows. Still prove manual-create retry, imported-row retry and duplicate target deletion separately. |
| Medium — resolved policy; implementation tests required | Header-keyed CSV mappings need deterministic rejection of ambiguous inputs. | Leader-approved policy: after removing the file BOM, reject empty or duplicate trimmed headers at file level; preserve the exact distinct header strings as mapping keys; reject mapping multiple source columns to the same non-ignore canonical field. Keep preview immutable, reject changed content/mapping under a reused request ID, retain per-row success identity, and test racing external IDs. Escape CSV correctly and neutralize formula-capable cells in downloadable failures. |
| Medium — expanded verification plan | The 37 acceptance rows alone do not enumerate every T1 functional requirement. | `verification.md` now adds FR-only obligations for filters/sort, snooze presets/reset, notification modes, batch allowlist, historical rounds/import summaries, deleted-result replay and delayed dispatch. The full-scope integration audit also retains disabled history, explicit include-triage search and compact scroll/focus checks from the source requirements. |

## Source-backed lock notes

- `server/internal/handler/lifecycle_handoff_transaction.go` resolves/locks squad and agent before sorting and locking issue rows; it rechecks assignment after locking.
- `server/pkg/db/queries/lifecycle.sql` uses NOWAIT owner locks to avoid established inverse orders. `EnqueuePreparedIssueTaskInTx` locks the agent/runtime and leaves commit/finalize to its caller; a caller cannot replace its preconditions with a stale Issue struct.
- `server/internal/handler/workspace_revoke.go` takes `LockSubscriberWrites` before membership-related writes; `LockActiveMember` holds `FOR SHARE` only after the appropriate serialization boundary.
- The existing prepared enqueue helper only deduplicates compatible pending tasks. The new action-to-task record must close the completed-task replay window in the same insertion transaction, as the design now requires.

## Scope and validation

No code changes or tests were performed by this review. The reviewer wrote only this artifact. The leader owns final design amendments, implementation, the complete requirement ledger, live PostgreSQL proof (not skipped DB suites), Web/Desktop browser evidence and full repository checks.
