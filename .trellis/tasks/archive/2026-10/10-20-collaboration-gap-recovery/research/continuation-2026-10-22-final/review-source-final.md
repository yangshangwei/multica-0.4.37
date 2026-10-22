# Independent final whole-clause AC85 / AC89 review

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`; reviewer
`/root/evidence_check` (`trellis-check`). Only this report and JSON written.
No product/test/spec/ledger/environment mutation, client/DB/provider launch,
commit or archive by reviewer. Root owns the two test assertion fixes below.

## Decision

**Recommend full AC85 and AC89 promotion.** Current strengthened production Web,
actual Electron and task-built CLI receipt satisfies the remaining clauses when
combined with canonical shared/server source and exact stage race evidence.
No blocking finding remains. Native duplication of the Web nonmanager matrix
is unnecessary for the same shared guard; its evidence scope is explicit below.

## Authoritative source receipt

- `source-clients-reviewed-final.stdout`: SHA256
  `583524a22e92cb80bd28d39ad0d64cdcf266b2a9c6368fcf4a13d809b0ed8bdc`.
  1 combined test pass, both platforms/CLI exercised; no retry, skip, unexpected,
  flaky or errors,32.144s Playwright duration. Raw and command/result hash checked.
- Final spec SHA `a2cbafc2a5e7aa8cdc13cc0844b680e3853b0442c06f9db445e39b1a0bfb8179`;
  fixture SHA `4188a8299f3cf78657853df494e8fc56ea74fc1d4974482e80b2fd4004cded64`.
  Current source equals recorded, including shared components/guards and native.
- Eight embedded JSON attachments exactly equal saved `source-clients-reviewed-final-*`
  files. Credential scan zero. Identity attachment SHA
  `1bc7877e736ebce4d568d3d5cfbee38fdb587fb11cf63b8131047e82a4f8b83b`.
- Summary SHA `23aedc15194d1346ee110aee6eba72a0b678b42bb014acd2bfdde94db61e4cb0`.
- Explicit CLI binary SHA
  `4b4c949a8bf90cf6bc0e2cb149f7c9c7c9e66d516f367d1023cf77486c326fb8`:
  matches both actual commands, current binary and94 CLI source build paths.
- Earlier `source-clients-final`1/1 (`a5030320...`) is retained as partial before
  the two assertion improvements; not falsely treated as the final current bytes.

## Findings (fixed by root)

1. Observer now includes `/api/issues/<shared>/*`, so `POST /move` cannot escape
   zero ordinary-write checks on collaboration assignee/status/terminal drags.
2. Both clients compare rendered Todo card IDs to source API numeric positions
   after board and CLI reloads. All four checks execute and prove no snapback.
   Prior assertions observed persisted server position alone.

## Full AC85 clauses

- **External name/avatar/external mark in list/board/timeline/comment; no Unknown/private account fields.** Both production Web and actual Electron inspect List row name+tag+title, Board tag+tooltip, Gantt tag+tooltip and comment visible name/avatar/tag/no Unknown. Core minimal schema+handler projection owns email/username stripping, source_identity network summary has no such keys. Blank avatar_url uses actual visible initial fallback, no image upload claim.
- **External collaborators filter.** Both clients toggle pressed filter, ordinary control disappears, shared remains; toggling off restores ordinary control. Canonical source handler covers membership/participant revocation and filter/count.
- **Cross-assignee move no optimistic card/assignee write, authorized transfer dialog.** Both clients inspect DB snapshot and actual column IDs while drag-over and after release, all ordinary/collaboration writes watched including ordinary /move subpath. Exact Transfer/recipient field visible for creator; no assignment write or optimistic column change.
- **Unauthorized source member gets clear refusal.** Actual Web observer is source member only and sees exact alert, no writer or optimistic move and same source snapshot. Same shared Board/SourceAction implementation runs both shells; source-bridge canonical denies capability/write_context. Native duplicate matrix is not required to verify a platform-neutral shared guard.
- **Nonterminal R47 checked; terminal complete/cancel denied with whole-issue prompt.** Both shells drag to In Progress and see current shared workflow/status options, no prior optimistic/write mutation. Web Confirm+restore performs public collaboration PATCH, actual status changes. Both Done drops show Complete/Cancel/Reopen explanation without write/snapshot change. Core source mapping covers cancelled/custom terminal; existing current canonical R47 owns role/catalog/terminal/refusal/concurrency semantics.
- **Ordinary drag unchanged.** Both actual clients observe ordinary column optimistic movement, then real POST /api/issues/{ordinary}/move with status and before_id:null/after_id:null, 200 and persisted target status. Shared snapshot unchanged. Internal hook test uses move_intent but actual public wire is POST /move; do not report PUT wire.

## Full AC89 clauses

- **Source board same-column and CLI order persist without snapback.** Both clients actual shared PUT contains only position+expected_revision, server position changes. Strengthened assertions after board reload compare rendered Todo DOM ID order to source API numeric positions. CLI issue reorder --before uses explicit current-built binary, then API position and fresh DOM order after reload match. All4 rendered order checks ran.
- **No collaboration version/history/notification effects; machine reorder refused.** Exact sourceSnapshot equality preserves c.revision/lifecycle/handler/status/events (excluding outbox-state delivery only)/notifications/session across board and CLI moves. Current source handler position tests machine403 and version1; canonical source projection no event; ordinary issue revision may increase, as intended.
- **Current source creator/handler sets child stage and records; other roles refused.** Current stage3/9 and exact HEAD+one file3/9 verify creator/current_handler stage-only history/actor/request/before/after/replay and exact execution preservation; unchanged source role test covers seven nonmanager/departed/external stage refusals.
- **Completion follows ordinary stage parent wake semantics.** Source handler8/31 and current23/75 repeat actual parent hook category/agent/squad/sibling-stage/parent guards, sanitized minimal child reference, task provenance, commit timing and exact replay. Wake is ordinary best-effort, not durable retry.
- **Concurrent transfer/source departure never preserves obsolete authority.** Actual PostgreSQL commit barrier and pg_blocking_pids in both stage/transfer/departure commit orders, new-key former/departed handler denial and exact source/execution snapshots.

The canonical source/parent raw8/31 and repeated source-machine-runtime23/75
remain independently verified in review-source-canonical. New stage exact
HEAD+single file3/9 now passes, raw
`5c8c7e93873586d596da23aa9996725bd1f84062cb45a15c38d63cafff7341bf`.
It closes that earlier review's isolated-run prerequisite without changing the
original stage/source role definition.

## Findings (not fixed) / explicit limits

No blocking issue. Web-only actual source nonmanager refusal and nonterminal
Confirm/restore remain bounded evidence. Both clients run the same shared Board/
SourceAction/core route and canonical guards; no further native duplicated matrix
is needed. Actual terminal drag is Done, with Cancelled/custom categories covered
by current Core/server matrices. Blank avatar_url proves visible initial fallback
and shared avatar, not uploaded image retrieval. Native private identity wire is
supplied by the same handler/Core schema; Web summary alone is explicitly recorded.
Native selects Workspaces on the account shell before sidebar/row navigation;
this is actual Electron IPC harness, not installed full Main. Parent wake is
ordinary best-effort with known crash/error/uncertain-commit loss and no replay
repair. Provider/account/quota, installed/old clients, OS packaging/containment,
Mobile and production gate rollout remain independent.

## Verification

- Strict source tsc and ESLint reviewed receipts exit0, final source hashes current.
- Source shared suites: Core1024, Views94 pass; relevant source/bridge files match
  current and fall within their collaboration selector.
- Current source1/1 and canonical/source stage receipts hash-checked/reparsed.
  No tests launched by reviewer. Diff check scoped to two test files passes.
- Write scope: this md/json only. Root owns ledger/commits; both tasks in_progress.
