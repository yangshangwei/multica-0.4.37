# Resumed merge-readiness review — 2026-10-12

## Conclusion

No concrete commit-blocking findings in the reviewed corrections. Reviewed the
44-file committed diff against `origin/main` (`9929aa9a3`) at `6a892c3d0`, then the
shared-editor implementation/test changes and the two current spec edits. This
is a static review conclusion; the final full verification and merge remain with
the parent session and the `full_verification` agent.

## Findings (fixed by the main session)

- `.trellis/spec/core/frontend/iteration-operations.md:5` still described
  default-disabled workspaces. It now matches `ReadIterationSettings` in
  `server/internal/service/iteration.go:106`: missing settings start enabled,
  and saved opt-outs remain disabled. The formerly stale offline-upgrade guide
  is already corrected by current main at `docs/offline-upgrade.zh-CN.md:444`.

## Findings (not fixed)

None requiring a further code change. No tracked files were changed by this
reviewer. No new dependency, runtime redesign or test weakening is recommended.

## Review evidence

- Identifier parsing: the bounded alphanumeric prefix matches workspace
  validation and the existing project/triage pickers. UUID recognition,
  case-insensitive deduplication, order, the 50-reference cap, exact lookup,
  workspace filtering and non-404 error propagation remain in place. Backend
  number extraction retains positive-integer semantics and parameterized search.
- Credential publication: Linux and macOS use exclusive atomic rename against
  the pinned directory. The staging name is not unlinked after successful
  publication. No-follow, owner, mode and single-link checks remain; file and
  directory sync precede account creation/commit at
  `server/cmd/server/platform_admin_initialization.go:281`. The new child test
  exits immediately after the real syscall, before cleanup and outside TestMain,
  then checks unchanged retry bytes and absence of staging names. Existing
  no-clobber and unsafe-file tests retain their assertions.
- E2E repairs follow current product behavior: explicit opt-out setup, named
  Issue actions, disclosure expansion before hidden-detail assertions, actual
  settings revisions and current table/metric markup. The P1-L14 and P1-P05
  steps/assertions have no additional uncommitted changes.
- All 13 modified PNGs retain their original dimensions and are below 307200
  bytes; the largest is 292531 bytes. Direct visual inspection of the dark agent
  list, project detail, virtualized list edge and iteration-event captures found
  readable text. This does not substitute for the final bitmap gate.

## Editor state-flow conclusion

`ContentEditor.onCreate` now initializes `lastEmittedRef` only without a pending
update. It preserves `pendingBaseRef` and the original document baseline, so
both the debounce and opted-in unmount path can deliver the early edit once.
The two new deterministic tests exercise update → deferred create → autosave
and update → deferred create → unmount, with the original empty baseline and
no duplicate callback after the timer window.

`repairEmptyListItems` does not itself arm that debounce: its content rebuild
uses `setContent(repaired, { emitUpdate: false })`, and its surrounding
transactions only restore selection (`packages/views/editor/utils/repair-list-items.ts`).
An ordinary create therefore still normalizes its initial baseline without a
spurious save. The onBlur callback and external-content synchronization branches
are unchanged. Default unmount still discards pending updates at
`content-editor.tsx:765`; `flushPendingOnUnmount` remains opt-in. No additional
blocking issue was identified in these paths.

## Target-fork main CI failure

Read using:

```sh
gh run view 38179638001 --repo yangshangwei/multica-0.4.37 --job 114589314507 --log-failed
```

The 1523-line failed-step output contains exactly two named test failures:

- `TestIterationLifecycleRoutesRespectScopeAndClosedRelease`:
  `POST iterations=409 want422`, with `iteration_preview_stale` /
  `Planning timezone changed`. The missing-row default now enables iterations,
  so the old disabled-workspace fixture reached the UTC/Asia-Shanghai validation.
- `TestIterationSettingsRoutesRequireWorkspaceOptIn`: settings returned
  `enabled:true`, revision 1, while the old fixture expected disabled.

Existing correction `2fe51e76b` addresses both by inserting an explicit disabled
settings row. Current fixture locations are
`server/cmd/server/iteration_lifecycle_routes_test.go:16` and
`server/cmd/server/iteration_settings_route_test.go:17`; the latter also checks
that unauthorized writes leave that row unchanged. Only `server/cmd/server`
failed in the package summary. No additional backend fix is indicated by this
run. This is evidence about `yangshangwei/multica-0.4.37`, not the upstream repo.
The next proof is the new candidate's full verification and fork PR CI; the old
main run itself remains failed. Raw logs and synthetic identity values were not
copied into this record.

## Verification and handoff

- Lint/typecheck/tests: not rerun by this reviewer. The implementation lane's
  `research/p1-editor-draft-recovery.md` records 44/44 focused tests, targeted
  ESLint, views typecheck and diff whitespace checks passing. Full make check
  is running independently; this review does not claim it passed.
- The original P1-P05 browser baseline reproduced the race; P1-L14 did not fail
  in that baseline. Browser validation against the repaired production build
  remains part of the complete verification.
- Follow the task's production-server provenance checks. Parent reports that
  `make up`, `make status` and `make env-exec` need explicit `MULTICA_ARGS=` to
  prevent ARGS from changing the API configuration hash.
- All PR/CI commands must explicitly select
  `--repo yangshangwei/multica-0.4.37`.
- Parent owns final source snapshot, commits, full verification acceptance,
  task status, PR and merge. The reviewer authorizes no external action.
