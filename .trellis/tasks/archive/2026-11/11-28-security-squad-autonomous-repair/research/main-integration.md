# Main branch integration

The user requested synchronizing this feature to main on 2026-11-28.

## Scope and identity
- Main baseline: `de00c4c98`.
- Source feature: `c4a04f458`; main cherry-pick: `3912372df`.
- Source task evidence: `374587ac5`; main cherry-pick: `3cc95b12b`.
- Existing main worktree: `/Users/artisan/.codex/worktrees/main-navigation-picks/multica-0.4.37`.
- Original feature workspace and its unrelated uncommitted edits were preserved.
- Local integration only; no remote push or deployment.

## Conflict resolution and independent review
`task.go` and `task_start.go` conflicted where the source branch had unrelated
material-claim/context logic absent from main. Retained main's existing admission
flow and added only `validateSecurityReviewReturnTask` at credential finalization
and execution start. All44 paths transferred;42 files match the original feature
commit byte-for-byte. Exact added/removed lines for both conflict files match the
original feature, excluding surrounding base context. No material changes imported.
An independent review found no integration issues; scoped Go vet and diff checks
passed.

## Verification on main
- Full Go build passed.
- Scoped Go vet passed, independently repeated by reviewer.
- Pure template/role/skill service tests passed.
- Catalog component:12 tests passed.
- Fresh dedicated service/handler databases allocated, current_database checked,
  and migrations applied from main. Credentials retained privately, never committed.
- Service race checks:29 passed,0 failed/skipped (`service-1795841050348.log`).
- Handler security flows/capabilities/template checks:116 passed,0 failed/skipped
  (`handler-1795841142380.log`), including all-agent code/dependency flows with
  zero human release comments and final `in_review`.

Private run directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-security-main-dOM4mC`.
Databases: `multica_security_main_cb5094e4_service` and
`multica_security_main_cb5094e4_handler`. No application database modified.

The existing deployed squad still requires backend deployment and explicit saved
instruction updates; this Git integration does not alter its configuration.
