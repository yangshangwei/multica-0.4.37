# Independent P3 collaboration and common-union review

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`.

## Verdict

**PASS_COLLAB_UNION_FIXES_AND_TARGETED_REGRESSIONS.** Three defects have verified
corrections. The reviewer handler run has 28 named results passing, the pure
retained-receipt contract has 18 passing, and the rebuilt actual NewRouter round
has 11 passing, all with zero failures/skips. The NewRouter round binds current
sources including the third later change; earlier rounds remain separate.

This is a bounded code and targeted-handler/router verdict. It does not replace
full normal/race compatibility, complete all-writer public P4 activation,
full gate or M1 acceptance. Root retains those gates.

## Findings (fixed)

### C-R1: retained completion receipt lost exact binding validation

The union extension removed the earlier authorization-ID, exact before-revision
and strictly advancing after-revision checks from `completionReceipt`. It also
accepted contradictory actor and variant fields as a successful minimal receipt.
Database field presence alone does not prove those relationships.

`collab-review-receipt-red01` retained 18 named failures: 16 malformed scenarios
and their two parent tests. The fix restores exact authorization/operation/before/
after/status/mutation binding and validates real human/task/integration actor
identity plus ordinary, self, submission and administrative terminal shapes.
An administrative terminal cannot fabricate a submission, and submission receipt
identity must match the authorized exact submission.

`collab-review-receipt-green01` passes all 18 named results. Each of the seven
closed variants also has a valid positive before its corrupted variant is
rejected, so the fix is not an unconditional refusal. These are pure projection
contract tests, not proof of a complete database workflow.

### C-R2: R38 completion omitted the actual pending submission

R38's locked access snapshot did not copy `collaboration_session.event_seq`.
Authorization creation correctly read the live pending submission, while the
actual administrative completion read a summary through event 0 and constructed
an absence intent. A valid exact authorization therefore failed to complete.

`p3-review-admin-pending-red01` retained the actual expected-200/received-409
failure. Its setup creates the ordinary child and real tar/receipt using the
existing fixture, submits an external result through authenticated middleware,
disables the creator, signs the exact pending-submission source authorization,
then calls the real R38 administration route.

The fix copies the locked session sequence and uses `currentDelivery` when an
explicit completion binding requires exact intent. With no binding, the common
gate checks current policy: enforce refuses missing authorization; off/observe
retain administrative disposition without inventing initiator acceptance or
making damaged delivery history an additional prerequisite.

`p3-review-admin-green01` passes 28 named results on a new database: the previous
22 collaboration cases, five compatibility results and the new genuine pending
R38 case. It verifies status/one admin terminal and zero fabricated `accepted`
decisions for that new positive.

### C-R3: current issuer revocation surfaced as an infrastructure failure

After the targeted review, `collab-auth-router03` exercised real NewRouter
authentication and retained 9 passes/2 failures (revoked issuer plus parent).
Completion was correctly refused, but missing current authority propagated as
`pgx.ErrNoRows` to the workflow's 503 fallback.

The narrow correction maps only `pgx.ErrNoRows` to content-free 404
`delivery_completion_not_found` in `deliveryCompletionError`. Actual database
errors still propagate. A terminal INSERT returning no row is already converted
to a specific completion conflict before this mapping. Rebuilt
`collab-auth-router04` passes 11 named results, zero failures/skips, on the new
`enf_p3_collab_auth04` database. Its compile04 snapshot matches all 12 current
reviewed files. This source change does not inherit the earlier 28-case handler
result; it has that later actual authenticated route proof.

## Investigated hypothesis that was not a defect

The initial concern was that reading malformed delivery history would already
regress off/observe R38 completion. `p3-review-admin-red01` actually passed all
five named results before production edits; it is a positive baseline despite
its prospective run name. Investigation found that event 0 made the old call
ignore the history. The genuine bug was the valid-pending-submission mismatch
above, subsequently demonstrated by a separate real failing test.

The positive no-project/off/observe/corrupt-history controls remain unchanged and
pass after the fix, while enforce without authorization returns 428 and writes
no completion terminal. No passing baseline is recast as behavioral red.

## Findings (not fixed)

No actionable defect remains in this bounded review. Full ordinary/race
verification remains root-owned acceptance work. The producer's prior 22-result
binary and original 12 source hashes are preserved as history, not rewritten to
pretend they cover the two corrected functions.

## Reviewed authority and atomicity

- Source evidence remains an exact formal ordinary child in the same source
  workspace/project with current parent and revision. The artifact still belongs
  to that child. Current issuer membership, target-read authority and original
  artifact authority are independently checked; the observer is reused once as
  the common algorithm rather than replaced by a private evaluation key.
- Outside consumers use current collaboration participation/account identity;
  ordinary collaborative completion still belongs to the initiator. R38 keeps
  its real administrator, complete password session, platform role, active source
  organization and disabled-creator rules. Stored issuer identity is not installed
  into the consumer authentication context.
- Target lifecycle/collaboration/participant/account versions and status catalog
  are rechecked. Pending submission and prospective self/admin intents bind
  exact content, source comment and attachment identity/version/digest. Attachment
  validation is read-only before gate admission; actual delivery/handoff writes
  occur afterward in the original transaction.
- Finish checks the actual state, real operation/decision/submission and current
  source evidence. Its error propagates to rollback rather than becoming a
  committed ordinary refusal after self-result writes. Status, Stop/queue,
  decision, original operation and authorization terminal remain atomic. Existing
  original-key workflow/admin replay precedes new mutation and never consumes a
  second time.
- Consumer GET returns minimal completion facts without source-child body,
  artifact details, private observation or credential metadata. Administrative
  configuration gates its own consumer surface; independently authorized source
  evaluators retain their permitted historical read. The explicit issuer helper
  remains separate from the actual consumer's current credential.
- Source-project editing invokes the D4b checker inside its original transaction
  and intersects owner/admin restrictions with existing source-member and
  creator/handler authority. No ordinary delegation expands collaboration or
  R38 permissions.

## Reviewer modifications and verification

Five product/test files were changed by this review:

- `service/delivery_completion.go`: narrow current-authority missing-row mapping.
- `service/delivery_completion_authorization.go`: the receipt projection helper.
- `service/admin_collaboration.go`: exact pending-intent selection and the locked
  access event sequence.
- New `service/delivery_completion_receipt_test.go`: retained-fact and closed
  variant positives/negatives.
- New `handler/delivery_completion_admin_compat_test.go`: original administrative
  compatibility and genuine pending-submission R38 completion.

Paths above are relative to `server/internal/`. Exact original/current hashes and
all 26 reviewed run/log identities are in `p3-collab-independent-review.json`.
The corrected handler binary's compilation had stable scope source. Its original
snapshot predates C-R3; the later NewRouter compile04 binds all current 12 scope
files, while the two added reviewer tests remain unchanged. All reviewed logs have
matching hashes, inventories and one terminal per started named test.

Go formatting and compilation passed. Pure receipt contracts pass 18/0/0; the
actual targeted handler run passes 28/0/0, and current NewRouter passes 11/0/0.
The producer's full backend vet passed
before these fixes; root must perform the final current whole build/vet/static
checks. There is no claim that pure checks or injected handler principals prove
all current public authentication behavior. The new R38 positive specifically
uses real submission and administrative middleware; the later targeted NewRouter
round covers its listed authenticated scenarios, not all product routes.

Only the exact assigned fresh databases `enf_p3_review_red01`,
`enf_p3_review_admin_pending_red01` and `enf_p3_review_admin_green01` were used.
No database allocation/drop, historical reuse, external network/provider,
dependency, commit, push, deployment or task archival. Failed source lookups of
guessed projection/admin-query paths were resolved by `rg --files`; they were
read-only lookup errors, not product red.

Root may continue P4 within its required acceptance sequence. This report does
not declare public enforce, the complete delivery gate, M1 or six businesses done.
