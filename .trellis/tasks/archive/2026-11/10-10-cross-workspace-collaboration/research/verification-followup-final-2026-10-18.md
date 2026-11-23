# Final M09 regression review — 2026-10-18

**Verdict: pass after one test-fixture correction.** The late repairs preserve
the running/waiting, cancellation, identity and proof assertions and fail closed.
This verdict covers the assigned single-agent M09 repairs, not full product AC,
target hold, multiple lanes, M11, or release readiness.

Compared `collaboration_groups.go` with the saved dirty-file baseline. The other
assigned tests and documentation were absent from that manifest and were compared
with its unchanged HEAD, `d62d6169cdbaec05717cd59fd94053e1ab19e320`.

## Findings (fixed)

- File: `server/internal/service/collaboration_edit_test.go`.
- Issue: after removing the creator's source membership, `workflowGrant` selected
  that creator as resource owner. `seedCollaborationTarget` restores its owner's
  execution-workspace membership, silently turning `departed_source_creator`
  back into a source member. The green test therefore missed its named role.
- Fix: give the existing third account active participant/account-fence records
  and use it as the independent resource owner. Assert before editing that both
  external editor variants still lack source membership. Preserve every original
  metadata, waiting-input, run/proof, grant, notification and handler-membership
  assertion. No production code was changed by this review.
- Evidence: the added assertion failed for `departed_source_creator` before the
  setup correction; the final race run passes creator, external handler and
  departed creator. The intermediate owner setup correctly refused without an
  account fence; the final fixture includes that required authority record.

## Findings (not fixed)

No scoped correctness finding remains. Existing lint warnings in UI (3) and
Views (27) are outside these Go repairs; lint reports zero errors.

## Review conclusions

- `validateGroup` locks every retained delegation, including cancelled rows,
  and checks its parent against the validated group runs. Both Stop and proof
  settlement call it before reservation release. The existing orphan regression
  compares retained state, including reservations, before and after rejection.
  Reusing the existing run map removes Stop's duplicate parent scan.
- Running batches now start before subsequent waiting inputs are submitted.
  The handoff, workflow and lifecycle tests retain their cancellation and
  immutable-identity assertions instead of expecting part of a frozen batch to
  remain waiting.
- Account reconciliation still cancels obsolete waiting input in the primary
  test. Its fresh run uses a separate target. The separate same-typed-target
  regression uses real R45 cancellation before replacing consent and verifies
  that delayed invalidation spares the fresh grant/run. Neither setup consumes
  obsolete authors into a purported fresh batch.
- Lifecycle source departure uses an independent resource owner. Disabled-gate
  rejection is tested separately from a fresh, enabled-gate Resume, whose
  partial receipt retains `stop_unconfirmed` and the identical old run.
- Fault injection covers `Query`, `Exec` and `QueryRow`, including actual channel
  and target-mirroring writes, while retaining rollback assertions. Router
  fixtures supply channel/batch identities rather than weakening v3 parsing.
- The updated server spec and paired built-in skill/source map correctly keep
  runtime protocol 2 distinct from read projection 3, retain client execution
  unavailability, and defer hold/multiple-lane/M11 work to the latest contract.

## Verification

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-review-followup-20261018-iluytkmz/`.

- Lint: pass, `pnpm lint`, 6/6; `final-review-lint.log`.
- TypeCheck: pass, `pnpm typecheck`, 9/9; `final-review-typecheck.log`.
- Go static checks: `go vet ./internal/service ./cmd/server` passed; changed Go
  files are gofmt-clean and the scoped diff passes `git diff --check`.
- Reviewer race regression: `go test -race ./internal/service -run
  '^TestCollaborationEditRecordsCurrentActorAndImmutableExecution$' -count=1 -v`
  passes all three variants against the task database. See
  `final-review-editor-red.log` and `final-review-editor-green.log`.
- Reviewed root evidence: account and lifecycle/fault repair logs pass; the
  router package passes in `go-fixture-repair.jsonl`; its earlier service failures
  are superseded by the final repair logs and `go-collaboration-verified.jsonl`
  (620 tests/subtests, package pass, no failures/skips). That broad service run
  preceded this final test-only correction; the focused race run verifies the
  corrected case. No duplicate broad Go run was performed.
- Root's three final Linux logs and cleanup-registry log also pass. They provide
  the recorded Linux/controller integration evidence, not native macOS evidence
  or completion of the deferred product phases. No provider CLI was invoked.
