# S3 block implementation staged check — 2026-11-19

**COMMIT READY; no blocking finding.** At HEAD
`81fc27ac847c6b59782b734eaba70d99f14eed67`, the index contains exactly the
66-path accepted block ownership set plus the two intended server spec files:
**68 paths = 63 whole product/test paths + 3 selective paths + 2 specs**.

The recomputed `git diff --cached --binary` SHA-256 is
`b54c3a9424e9b42f89779c647b3e0b38b3ef3c41fee85658cb6214e277784957`,
exactly matching the private `intake/s3-block-staging-r01.json` receipt.
`git diff --cached --check` passes. No additional path is staged.

## Source and selective-index verification

The 66 paths are the exact set in
`research/s3-uncommitted-ownership-20261119.json`; its original owner deltas and
five accepted compositions remain the provenance basis. Every current
worktree hash still matches that inventory, except the explicit one-byte EOF
cleanup below. All 63 whole-file index blobs equal the current worktree.

Within those 66 paths, the only worktree→index differences are:

| Path | Preserved unstaged S2 change |
| --- | --- |
| `server/internal/service/collaboration_workflow.go` | One `GetWorkflowOperation` line: worktree uses `LockArtifactUsers`, index retains HEAD's `LockCollaborationUsers`. All other bytes match. |
| `packages/views/locales/en/collaboration.json` | Six execution/rollback refusal keys remain only in the worktree. |
| `packages/views/locales/zh-Hans/collaboration.json` | The same six refusal keys remain only in the worktree. |

The exact locale container is `stage.refusals`; the ownership report's prose
called it `stage.reasons`. This label typo did not affect staging. Removing
only the six named S2 keys from each parsed worktree object produces the
complete index object exactly; the index's stage key set equals HEAD. The
S3 `information` subtree and `activity.kinds.information_repaired` match the
accepted block candidate. The six retained keys are:

`stage_execution_unavailable`, `stage_execution_target_changed`,
`stage_execution_pending`, `stage_execution_conflict`,
`stage_rollback_observation_unavailable`, `stage_rollback_observation_changed`.

The five files composed with already committed Agent-output changes retain
their recorded accepted compositions; no old block file replaced the output
route, stored-outcome decoder, attachment fields/scanners or nullable upload
guard.

## Explicit formatting exception

`packages/views/collaboration/information-workflow.test.tsx` differs from its
accepted source only by deletion of its final extra LF. The original ends in
two newline bytes; the staged/current file equals `accepted_bytes[:-1]` and
still ends in one newline. No test statement, assertion or other byte changed.

- Accepted SHA: `61c40203fcd0e8480c86d9739aae8d5ad19246763bd82a31b36574c80f0a442d`.
- Staged/current SHA: `85a6e427efd81bb69fcefe6629398ce3395eadd548176e8cc56544826f0a2df8`.

This is an explicit formatting exception to literal accepted-source identity;
it does not require replaying 280 unchanged UI tests or old runtime bodies.

## Spec review

The staged `collaboration-information-recovery.md` and its server index link
match their working copies. The spec accurately describes the inspected
accepted implementation: timezone-bearing zero-to-six fractional-digit ETA,
UTC year/microsecond checks, original-operation recovery before the final
new-write clock rule, managed required metadata, atomic repair and pending
submission constraints, actual saved return status, nullable legacy
provenance, credential-sensitive context versions, bounded keyset history,
verified-key HMAC cursors and the 5-second/50-item/2,048-cursor/65,536-response
limits. Migrations 866/867 add the three nullable fields/readiness and guarded
downgrade; they add no foreign key, cascade or index.

The spec keeps runtime-v1 metadata limitations, original failure history and
the finite registration retry contract explicit. The separate E2E/helper
batch is not part of these 68 staged paths. Main's later registration Node
test result and that six-path batch are outside this staged check's scope.

## Verification and limits

- Exact staged path set/hash, index-versus-worktree selection and preserved
  accepted compositions: **PASS**.
- Staged whitespace check: **PASS**.
- Spec/contract consistency: **PASS**.
- Product tests/typecheck/lint: not rerun here; the immutable owning-layer and
  actual R08 evidence plus the just-passed ROOT full typecheck/lint remain the
  accepted verification basis. Only the stated EOF byte was changed later.
- Reviewer writes: this report only; no source/index/config/DB/API/process
  changes.

The initial read-only locale checker used the prose's `stage.reasons` name and
raised `KeyError`; inspection located the actual `stage.refusals` container,
and the corrected complete check passed. No file or index content was changed
to satisfy the checker.

Main may commit this bounded S3 block batch now. Its tested working candidate
includes the separately accepted S2 foundation whose exact three hunks remain
unstaged; this report does not claim the selected index reconstructs that
entire runtime candidate from clean HEAD. Later E2E commit, task bookkeeping
and archival remain Main-owned lifecycle work.
