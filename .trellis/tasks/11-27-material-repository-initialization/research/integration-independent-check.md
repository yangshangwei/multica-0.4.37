# Independent integrated lifecycle check — 2026-11-27

Reviewed the complete task change set against PRD, design, contracts, check.jsonl
and the server/shared-interface specifications, in the dedicated initialization
worktree only. The review includes kernel, registry, runtime, queue, permission,
observation, maintenance, SQL, handler, shared UI and protected pending-request
boundaries. No real model, account or remote write was performed.

## Findings fixed

- Repeated checks before dispatch created competing inspection revisions;
  checks after successful publication abandoned the existing open PR. Checks
  now retain queued inspections and unsettled candidates. Repeated maintenance
  likewise retains pending work instead of replacing its revision.
- Retry before an operation existed failed with no rows when an offline runtime
  blocked dispatch. It now wakes the original inspection job. Retry after a
  changed root targets the current revision, and successful retries settle jobs
  instead of retaining a scheduler lease forever.
- Recovery of a failed inspection used `observe/inspect_repository`, but the
  daemon tried to discover a PR. It now independently reads the base tree.
- Any protected-tree verification error was treated as proven drift. Malformed
  or incomplete evidence now returns unverified; only a complete actual rule
  mismatch produces the accepted negative receipt and durable blocked state.
  Maintenance drift previously rolled back its blocker with the original error;
  complete negative maintenance observations now persist and acknowledge it.
- Only one bundle existed, making nonempty maintenance unreachable. The
  coordinator approved an actual packaged1.0.1 lifecycle-rule update while
  retaining exact1.0.0 assets. The runtime lane implemented the registry and
  real queue upgrade test; this check wired registered targets and stored the
  actually adopted prior bundle instead of falsely labeling it current.
- Recheck/maintenance proof replacement discarded the old proof projection.
  Old proofs now enter informational history before replacement.
- An already running plan could adopt a newly accepted repository proof.
  Plan-token authorization now compares the original frozen proof as well as
  current settings/runtime authority.
- Same-binding settings changes stranded an open initialization review. New
  decisions now freeze settings version and binding generation. Old attempts
  and decisions remain invalid; explicit fresh human approval can authorize a
  new attempt against the same candidate/review/branch. Policy-only edits and
  disabled→enabled recovery have regression tests. Old history is unchanged.
  Queued execution is serialized by existing operation/task checks; reawakening
  a logical job does not authorize a concurrent mutation.
- The prelaunch rules check changed the established missing-baseline error
  precedence. The baseline check remains first, while both guards must pass.

New canonical regression files are
`server/internal/service/material_repository_check_test.go` and
`server/internal/daemon/material_repository_check_test.go`.

## Evidence

Private evidence root:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/material-init-independent-kbtr481r`.
Fresh databases were identity-checked before migration; no active backend or
frontend database was reused. Dedicated disposable Redis covers ordinary claim
cache tests that the backend's earlier run skipped. Its owned container is
stopped after each check, with name/port retained in private `redis.json`.

Recorded failures before fixes are in `red.log` and `reauthorization.log`.
Targeted race checks subsequently passed in `green.log`, `new-regressions.log`,
`reauthorization-green.log`, `material-regression.log`, and
`final-local-regressions.log`. The complete material service regression passed
after the baseline-order correction. Intermediate passing whole affected runs
are retained as `service-intermediate.*` / `runtime-intermediate.*` because later
state-machine fixes changed the source.

Independent `pnpm lint` and `pnpm typecheck` passed (6 and9 tasks, cached), in
addition to the frontend lane's forced checks. Logs are
`/tmp/material-init-independent-lint.log` and
`/tmp/material-init-independent-typecheck.log`. `make sqlc` regenerated the
updated queue query; its log is `/tmp/material-init-independent-sqlc.log`.

Final fresh service/Redis and both-provider runtime/maintenance checks passed:

| Check | Final result |
| --- | --- |
| Material/init/recovery plus ordinary claim, finalization and iteration start, race |64 top-level passes, zero failures, zero skips; fresh `multica_init_check_cadc90a5` |
| Actual queue/native fixture/independent readback: new initialization + ordinary/manual/drift | GitHub79.17s, GitLab38.89s; both passed |
| Actual supported1.0.0→1.0.1 maintenance, human review and merged proof/history, race | GitHub17.83s, GitLab14.77s; both passed |
| Final Go build ./... and affected service/daemon/material/materialrepo vet | Passed |
| Root lint/typecheck, generated SQL, gofmt, diff-check | Passed |

Runtime suite total152.665s, zero skips/failures. Final logs are
`service-final.jsonl` / `runtime-final.jsonl`, with separate private allocation
records. All recorded source/test hashes in `source.json` remained unchanged
through final checks. The dedicated Redis container was stopped afterward.

No unresolved blocking code finding remains in the reviewed integrated scope.
The final production UI matrix is deliberately run after this completed review,
so frontend evidence must bind these final source bytes independently.

## Final fixture-only follow-up review

Reviewed the subsequent GitHub request-body fix: buffering raw chunks, checking
the same byte limit, and decoding once after `Buffer.concat` preserves UTF-8
codepoints split across network chunks. The regression deliberately splits
`证` within its encoded bytes and checks the retained content SHA-256 before
continuing the existing immutable-file, exact-head merge and history assertions.
No product authority, byte comparison or validation was weakened.

The runner's optional `--grep` is passed as a separate process argument from its
private allocation record. Per-case fresh database/remote fixtures preserve the
existing test deadlines and assertions while avoiding accumulated-tree cost.
Reviewed these changes with no additional finding. The coordinator reran both
provider fixture node tests successfully and reports all eight final production
Web/Desktop initialization/maintenance cases passed on isolated case fixtures.
Those frontend evidence manifests own exact source/build/run provenance; no
product code changed after the completed independent check above.

## Scope limits

GitHub Enterprise and GitLab URL-prefix instances remain explicitly unsupported.
A truly empty repository still needs an existing base branch; initialization
does not silently create a default branch outside review. Existing unsupported
custom rule content is preserved and blocked. Multica's stage grants do not
confine arbitrary host shell commands. Final production Web/Desktop replay and
live provider/model acceptance are distinct evidence stages; local test success
does not certify real credentials or model behavior.
