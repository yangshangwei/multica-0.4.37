# Independent full-scope implementation check — 2026-11-27

Reviewed the runtime-only material changes against PRD, design, contracts and
check.jsonl, including tracked changes and new files in implementation-scope.json.
Review covered server/domain/SQL/migrations, claim/start/receipt/permit boundaries,
daemon/CLI/read-only providers, core schemas/recovery, shared UI/locales, fixtures
and built-in documentation. Existing unrelated work was preserved. No real model,
account or remote repository operation was invoked.

## Findings fixed

1. **Missing review note could never recover.** A runtime terminating before its
   audit-note write left a permanent unknown result: readback returned
   `decision_note_missing`, and the service had no note-absence transition.
   `daemon/material_observer.go` now admits that result only for an explicit
   read-only review-sync reconciliation, after complete note pagination and full
   review/file readback. `service/material_operation.go` records `no_effect`;
   another deliberate retry retains the original decision/key. A missing note
   during mutation, conflicting note, incomplete read or changed bytes still
   refuses. Owning-layer regressions failed before the fix and pass afterward.
2. **Recovered note did not advance to merge.** Successful `observe/review_sync`
   retained action `observe`; dispatch checked only `review_sync` and stopped the
   approved job. Dispatch now uses the effective reconciled action. The regression
   writes the note, loses its observation, completes read-only recovery, then
   reaches an approved merge and verified baseline through the normal service
   state machine. No extra note write is needed.
3. **Running plan could adopt a refreshed configuration.** Plan token authorization
   checked the current carrier/settings but not the original operation snapshot.
   `service/material_plan.go` now binds task/agent/runtime/originator, original
   configuration version and runtime digest while preserving legitimate read/replay
   after submission moves the material's current revision. Both stale-config refusal
   and post-submission authorized access have regression coverage.
4. **Disabling required a live replacement runtime.** Public schema allows a
   disabled configuration with no runtime, but Configure still resolved/created
   an execution carrier unconditionally. `service/material.go` retains target,
   reviewer and UUID validation while skipping execution binding for disabled
   settings. Tests cover null runtime, deleted runtime, exact request replay, no
   dispatch, and re-enable refusal until a valid explicit runtime is selected.

Added canonical regression files:

- `server/internal/service/material_recovery_check_test.go`
- `server/internal/daemon/material_recovery_check_test.go`

## Verification

Independent evidence root:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/material-independent-57lsjktl`.
Private allocation files retain credentials outside the repository. Fresh database
identities were checked before migrations; neither backend nor browser lane's active
databases were reused.

| Check | Result |
| --- | --- |
| Review-sync absence, same-carrier plan config, disabled runtime, recovered note transition regressions | Recorded red before fixes, green with race detection after fixes |
| Final service materials plus ordinary capacity/batch claim/finalization/iteration-start | 50 top-level tests passed, `-race`, fresh `multica_material_check_034dcc3c` |
| Final tagged real queue → fake native runtime → independent provider reads → server observation | GitHub and GitLab passed, `-race`, fresh `multica_material_runtime_0b7cd044` |
| `pnpm lint` | Passed, 6 tasks; existing warnings, no errors |
| `pnpm typecheck` | Passed, 9 tasks |
| `go -C server build ./...` | Passed |
| `go vet` affected service/handler/daemon/material/materialgit/CLI/router/protocol packages | Passed; service/daemon repeated after final transition fix |
| gofmt and `git diff --check` | Passed |

Logs: `red.log`, `red-disable.log`, `red-recovered-note.log`, targeted green logs,
`service-final.jsonl`, `runtime-final.jsonl`; source hashes in `reviewed-source.json`.
The first transition-test replay failed because its next scheduler tick was not
yet eligible. The test now uses the existing fixture's scheduler-tick helper;
no product timeout or final-state write was introduced. Earlier passing suites
before the transition fix are retained as `*-before-transition.*`, not mislabeled
as final source evidence. Independent frontend command logs are
`/tmp/material-independent-lint.log` and `/tmp/material-independent-typecheck.log`.

The coordinator additionally ran the complete default daemon/CLI/material/
materialgit/protocol packages (1,623 top-level passes, three intentional platform/
helper skips); the backend lane owns router/handler/migration evidence. Those
results are complementary, not independently rerun here. Production Web/Desktop
final source replays belong to the frontend lane after this review is complete.

## Remaining findings and limits

No unresolved blocking code finding remains in this reviewed scope. GitHub
Enterprise and GitLab URL-prefix instances remain explicitly unsupported by the
host CLI transport. The configured daemon/host is trusted; phase authorization
does not isolate arbitrary host shell commands or prove an honest host owner.
Full-repository Go acceptance and live GitHub/GitLab/model execution remain the
separate follow-ups stated in the task. Deterministic fixtures do not prove real
installed CLI credentials, native provider policy compatibility or model behavior.
