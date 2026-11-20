# R27 exact submission binding: bounded implementation

The separate GREEN source contains one product field initializer and the frozen
R38 current-submission regression. Its service compile and vet both finished
with exit 0. Test execution and final acceptance remain main-owned. The initial
RED source, admin RED source and prior implementation reports are preserved.

## Real RED basis

The main's original run is
`verification/submission-red-r01/result.json`, SHA256
`6d1f4de27202c0674c0bb0273d132d443e06b929c38da12601554b5dff0acd38`.
It has three PASS and three FAIL named identities: the three negative leaves
pass; the two valid-preview leaves and their parent fail. There are no skips,
pending/missing/extra names or source drift. The actual five retained witnesses
were read without executing the suite again.

Both valid previews return `reason=stage_incomplete` and the exact blocker
`verification/stage_submission_unbound`, with the genuine saved submission
present. No PostgreSQL error caused those two failures. The main reports the
independent `ACCEPTED_R27_SUBMISSION_RED_BOUNDARY` decision. This is the real
test-first basis for the one-field product fix; it is not a replacement of the
earlier R24/R25 unknown-holder evidence or candidate-only acceptance.

## Product change and source identity

Only `source-green-r01/server/internal/service/collaboration_completion_preview.go`
changes. The existing temporary `stageMutationGuard` now initializes
`boundSubmission` with
`completionUUIDPG(*request.Target.SubmissionID)` inside the existing nonnil
submission branch, before the unchanged `checkSubmission` call.

The request target and exact current completion intent have already been
validated. Initializing this transaction-local field lets the existing check
read and compare the real immutable stage submission, original operation and
handoff. It creates no durable authorization and leaves all authority,
nomination, credential, source, operation, expiry, policy and lock helpers
unchanged. The generic initializer also applies when a valid R38 target carries
its current submission. No new pure-read marker, shared lock, writer change,
SQL, generated code or dependency is introduced.

| Identity | SHA256 |
| --- | --- |
| Original preview product, both RED sources | `07637fee75dc894a50dd32eaa2beee3254784f015f0096fdd8cd3b44410fd0fe` |
| GREEN preview product | `9272930e7d665f61eb7ad1c8bafbd412e02651aa77138c682678fa8765cdd3ba` |
| Preserved 224-line initial regression | `f1aa77c0749504ee1826942c1511e5ad751a2254c8385ac69c2b0514ebc865e7` |
| New 174-line R38 regression, identical RED/GREEN bytes | `658bb12c83611a78cb6974bc054e82bbb872a3d9a8ee51bfbb5a195db64be925` |

The complete GREEN source has 5,050 files. Against the original 5,049-file RED
manifest, 5,048 existing files are byte/mode-identical, the product has exactly
the one-field byte delta with its original mode, and the R38 test is the sole
new source path. Against the 5,050-file admin RED source, only that product
initializer differs. The original initial RED source remains unchanged.

## Separate genuine R38 regression

The new file is
`source-admin-red-r01/server/internal/service/collaboration_completion_preview_admin_submission_test.go`.
It defines exactly
`TestCollaborationCompletionPreviewAdminCurrentSubmission/admin_available`:
one top-level, one leaf and two named identities. The five original leaves are
unchanged. GREEN contains both files for two top-level, six leaf and eight named
identities, established by actual Go AST parsing of each frozen file.

The R38 leaf uses `newStageCompletionFixture` with the existing independent
artifact and product-role setup. It reaches genuine verification, selects M1
enforce, calls the actual `SubmitAcceptance`, obtains the existing real
administrator fixture, adds explicit source workspace admin membership and
disables the creator through the existing account service. Platform super-admin
status alone does not supply artifact visibility.

A real preparation-only borrowed transaction performs the unchanged artifact
transaction preparation, current `completionAdminNomination` and
`collaborationDeliverySummary`. The current submission must equal the saved
submission and retain available evidence. `completionAdminTarget` receives
that actual current submission; the target is not built from nil or cached
pre-submission state. A final credential check and commit finish preparation.

A separate actual `pool.Begin` transaction calls the preview with the real
admin JWT/password snapshot and organization ID, then performs the final
credential check and commit or rollback. Both action contexts remain five
seconds, transaction preparation remains unchanged, and the default pool,
isolation and main's ten-minute suite budget are preserved. The test does not
mint an M1 grant or call administrative `ChangeLifecycle`.

Expected success is `authorization_available` with no stage blockers, the exact
admin/org/root/submission/ordinary-child/approval/project binding, a canonical
passed artifact observation and the saved evidence/credential/release expiry.
The existing envelope helper checks false enforced/usable/skip flags. The new
witness records actual returned reason, blockers, service error, backend and
target identity before assertions, so the main's original-product admin RED
can establish the reached failure.

Before either borrowed read, durable completion authorizations, terminals and
evaluations must be empty. Existing 72-table and stage-exit snapshots cover
current workflow, credential, stage, submission, artifact and effect rows.
`adminCollaborationStateSnapshot` additionally retains root admin operation,
audit and control state; the controlled source's mutex-protected call/target/
invocation snapshot must also remain identical, including after failure. This
is not an all-table or filesystem archive-byte audit.

## Verification and handoff

Private evidence lives under
`/Volumes/artisan/.multica-acceptance/pay241-r27-preview-20261117-x12xjirm/evidence/`:

- `binding-admin-red-freeze-r01.json`, its full source manifest and real AST
  result: 5,049 protected files unchanged; only the new R38 test is added to
  the original-product admin RED source.
- `binding-green-freeze-r01.json`, its full source manifest, both real AST
  results and `binding-green-product-r01.patch`: exact bounded GREEN delta.
- `binding-green-compile-r01.json`: one
  `/opt/homebrew/bin/go test -c -o <R27root>/evidence/service-binding-green.test ./internal/service`
  from `source-green-r01/server`, exit 0, no timeout and empty stdout/stderr.
- `binding-green-vet-r01.json`: `/opt/homebrew/bin/go vet ./internal/service`,
  exit 0, no timeout and empty stdout/stderr.
- Both checks use only the decoded GREEN JSON environment, SHA256
  `37eaac0bfbf54db8c89f231e22e24a4ff6f47b5def528bada350621db7d0821d`,
  with no inherited variables and no database function called by its loader.
- The unexecuted GREEN test binary SHA256 is
  `a2ad369a96e9a7e3bc3513cbc6b0f191b6755925655fc2f7ff178c43f01a05f2`.
- `binding-green-implement-postchecks-r01.json` rechecks all three complete
  source manifests after compilation/vet, the prior RED report hashes and
  every compile/vet log and binary hash. All identities match; the exact
  one-field product delta and sole new test remain the entire source change.

This implementer executed zero test binaries, TestMain bodies, database/SQL
commands, API requests or browser actions, and spawned no subagents. ROOT
product and prior RED reports remain untouched. The main owns separate actual
admin RED, combined fresh GREEN/race and any affected original controls. Broader
preview read-lock coexistence, full S2 and parent acceptance remain separate;
historical R24/R25 holders remain unproved.
