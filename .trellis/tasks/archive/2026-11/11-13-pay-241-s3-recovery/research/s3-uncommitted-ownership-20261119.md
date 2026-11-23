# Research: S3 uncommitted source ownership

- Query: Identify a finite, source-qualified local commit boundary for accepted earlier S3 blocking/recovery work after UI commit `81fc27ac8`, without taking all other working-tree changes.
- Scope: internal, selected files and read-only Git evidence
- Date: 2026-11-19

## Findings

**A bounded S3 commit is mechanically separable now:** 63 whole block product/test paths plus S3-only hunks in three shared files (66 total paths), and a separate five-path accepted E2E/helper group. Exact paths, current/HEAD/baseline/accepted hashes and the five composition proofs are in the adjacent JSON. The main session reports the final R01–R09 check is accepted; this report addresses ownership and lifecycle only. S3 remains unarchivable until its own accepted source/evidence/spec work is committed and lifecycle handling completes. This is an **uncommitted-source/lifecycle gate**, not a newly discovered behavior defect or an S4/S5 gate.

The 66-path boundary is taken from the original block owner deltas: 37 server paths and 29 client paths. It was not inferred from directory-wide dirty status. Fifty-eight paths have HEAD identical to the recorded pre-S3 baseline and ROOT identical to the accepted final block candidate. Five others preserve already committed output-attachment changes, proven by applying the exact pre-S3→HEAD contextual delta in memory to the accepted block source and obtaining ROOT byte-for-byte. Three retain separate, preexisting S2 hunks and need selective staging.

### Group 1: blocking recovery implementation

Stage all 63 whole paths listed below, together with only the specified S3 hunks from the three shared files. These form one coherent implementation: new columns/readiness migration, generated readers, service/HTTP transaction and history, strict client parsing/recovery and shared forms/history. Committing only new files would omit required imports, route registrations, generated fields and call sites.

**Server whole-file paths**

```text
server/cmd/server/router.go
server/internal/handler/collaboration.go
server/internal/handler/collaboration_creation.go
server/internal/handler/collaboration_information.go
server/internal/handler/collaboration_information_test.go
server/internal/handler/collaboration_workflow.go
server/internal/migrations/collaboration_information_test.go
server/internal/service/collaboration.go
server/internal/service/collaboration_activity.go
server/internal/service/collaboration_creation_options.go
server/internal/service/collaboration_handoff.go
server/internal/service/collaboration_information_concurrency_test.go
server/internal/service/collaboration_information_creation_test.go
server/internal/service/collaboration_information_history.go
server/internal/service/collaboration_information_history_test.go
server/internal/service/collaboration_information_metadata_test.go
server/internal/service/collaboration_information_repair.go
server/internal/service/collaboration_information_repair_test.go
server/internal/service/collaboration_information_runtime_test.go
server/internal/service/collaboration_information_state.go
server/internal/service/collaboration_information_test.go
server/internal/service/collaboration_lifecycle.go
server/internal/service/collaboration_runtime_handoff.go
server/internal/service/issue.go
server/migrations/866_collaboration_information_metadata.down.sql
server/migrations/866_collaboration_information_metadata.up.sql
server/migrations/867_collaboration_information_ready.down.sql
server/migrations/867_collaboration_information_ready.up.sql
server/pkg/db/generated/collaboration_delivery.sql.go
server/pkg/db/generated/collaboration_handoff.sql.go
server/pkg/db/generated/collaboration_information.sql.go
server/pkg/db/generated/collaboration_stage_submission.sql.go
server/pkg/db/generated/collaboration_workflow.sql.go
server/pkg/db/generated/models.go
server/pkg/db/queries/collaboration_information.sql
server/pkg/db/queries/collaboration_workflow.sql
```

**Client whole-file paths**

```text
packages/core/admin/collaboration-schema.ts
packages/core/api/client.ts
packages/core/collaboration/action.ts
packages/core/collaboration/activity-schema.ts
packages/core/collaboration/creation-schema.ts
packages/core/collaboration/creation.ts
packages/core/collaboration/delivery-schema.ts
packages/core/collaboration/index.ts
packages/core/collaboration/information-action.test.ts
packages/core/collaboration/information-queries.test.ts
packages/core/collaboration/information-queries.ts
packages/core/collaboration/information-schema.test.ts
packages/core/collaboration/information-schema.ts
packages/core/collaboration/operation.ts
packages/core/collaboration/presentation.test.ts
packages/core/collaboration/presentation.ts
packages/core/collaboration/schema.ts
packages/views/collaboration/activity.tsx
packages/views/collaboration/creation-form.tsx
packages/views/collaboration/detail-auxiliary.tsx
packages/views/collaboration/information-block.tsx
packages/views/collaboration/information-creation.test.tsx
packages/views/collaboration/information-history.test.tsx
packages/views/collaboration/information-history.tsx
packages/views/collaboration/information-workflow.test.tsx
packages/views/collaboration/labels.ts
packages/views/collaboration/workflow.tsx
```

**Three selectively staged paths**

| Path | S3 hunks to include | Preexisting S2 hunk to leave unstaged |
| --- | --- | --- |
| `server/internal/service/collaboration_workflow.go` | Information repair confirmation/transaction fields; managed-block transfer guard; metadata handoff values; repair kind/target/replay allowlists; information-request savepoint/final ETA-clock rollback; content-free repair receipt validation/projection. | In `GetWorkflowOperation`, `LockCollaborationUsers` → `LockArtifactUsers` (current line 763). The S3 readiness report explicitly records this as preexisting S2. |
| `packages/views/locales/en/collaboration.json` | Entire top-level `information` subtree and `activity.kinds.information_repaired`. | Six `stage.reasons` execution/rollback codes around current line 1545. |
| `packages/views/locales/zh-Hans/collaboration.json` | Same two S3 key groups as English. | Same six S2 stage codes around current line 1537. |

For the workflow file, the original pre-S3 source differs from HEAD by exactly that single read-lock line, and accepted final block source equals ROOT. A selected S3 staged blob is therefore current ROOT with only that line restored to HEAD; **do not edit the working-tree file**. For each locale, the exact pre-S3→accepted block delta adds only the information subtree and one activity key. Current ROOT equals accepted source, while pre-S3 differs from HEAD only by the six S2 codes. A selected S3 staged blob must retain the original HEAD stage code set while adding those two S3 groups. Keep the working tree's S2 changes intact. This is a proposed index composition, not a staging operation performed here.

The excluded code names are `stage_execution_unavailable`, `stage_execution_target_changed`, `stage_execution_pending`, `stage_execution_conflict`, `stage_rollback_observation_unavailable` and `stage_rollback_observation_changed`. They remain owned by the already accepted S2 increment and are not needed to rename or classify the S3 repair operation. Do not take their schema/panel/spec companions into this S3 commit by inference.

**Five accepted compositions with committed output work**

| Path | Already committed output adjustment preserved | Remaining S3 delta |
| --- | --- | --- |
| `server/cmd/server/router.go` | Runtime comment attachment POST route | Three human handoff history GET routes |
| `server/internal/service/collaboration_runtime_handoff.go` | Stored-outcome decoder in result-source text | `information_block_metadata_unavailable_v1` missing marker |
| `server/pkg/db/generated/collaboration_delivery.sql.go` | Attachment Agent/run/attempt columns/scanner | Information metadata/repair columns in handoff projection/scanner |
| `server/pkg/db/generated/models.go` | Agent/run/attempt fields on `CollaborationAttachment` | Information metadata/repair fields on `IssueHandoff` |
| `packages/core/api/client.ts` | Null `draft_id` guard for Agent rows in human upload recovery | Information metadata/repair transport, human history readers, project-aware creation options |

All five in-memory compositions equal current ROOT exactly. Their current HEAD→ROOT diffs are S3-only; whole-path staging preserves the committed output changes. Replacing them with an old accepted block source would regress the output feature.

### Group 2: accepted browser proof and registration fixture

These five complete files match their original accepted source bytes and remain outside HEAD:

```text
e2e/collaboration-information-recovery.spec.ts
e2e/fixtures/collaboration-information-recovery.ts
e2e/collaboration-recovery-extras.spec.ts
e2e/fixtures/collaboration-recovery-extras.ts
e2e/fixtures.ts
```

Use the block R08 harness for information spec/fixture and `e2e/fixtures.ts`; use the Extras-A drawer-author R05 harness for the Extras spec/fixture. The later block harness contains a different *unused* Extras fixture, so it must not replace the actually accepted Extras-A bytes. The shared recovery helper is already committed by the UI increment; do not stage or reconstruct it again.

`e2e/fixtures.ts` has only the S3 opt-in registration-window change versus HEAD. R08's retained runtime/source review verifies its real one-repeat behavior with the product limit unchanged. The adjacent new `e2e/fixtures/password-registration-window.node-test.mjs` is S3-owned and its current hash equals the prior session baseline (`6ebba4ff1b7760286e71a8d8cc8842f71db56e87a18d6e9b6f6f0a0d6002a150`). The bounded read did not locate its exact original execution receipt. The main session can bind that receipt or run this finite no-DB Node test before adding it to the same group; this is a test-evidence gap, not an unowned code delta.

### Accepted sources and evidence

- Server original delta: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/intake/block-server-delta-r01.json`; all 37 before-blobs match the retained `source-r01` baseline. Final candidate: `source-block-work-r05`.
- Server acceptance: `block-package-r05-independent-check-20261118.md` binds R05 handler normal/race 15 names each and unchanged R04 service normal/race 74 names each; `block-runtime-r08-independent-check-20261118.md` binds the actual final R05 server manifest and fresh 2-top/12-case runtime. Earlier R03/source/UTC/query failures remain retained.
- Client original delta: same campaign `verification/block-client-release-r01/delta-r01.json`; all 29 before-blobs match `source-r01`. Final candidate: `source-block-client-work-r03`, also copied exactly into production `source-block-ui-run-r05`.
- Client acceptance: `block-client-code-r02-independent-check-20261118.md`, `block-client-projection-r03-independent-check-20261118.md` (Core 178/Views 94 plus type/lint), `block-ui-build-r05-20261118.md`, and R08 runtime. These sources are compared here; old raw tests are not re-parsed or rerun.
- Extras source/evidence: `verification/extras-a-drawer-author-r05/harness` and `extras-a-r05-independent-check-20261118.md` (six tops/eighteen cases). Information source/evidence: `runtime/block-information-r08/harness` and its independent report (two tops/twelve cases).

## Code patterns and hard dependencies

- `server/internal/service/collaboration_information_repair.go` and `collaboration_information_state.go` borrow the existing workflow transaction, current policy/authority and original operation owner; they depend on the workflow's S3 receipt/savepoint fields, not a parallel ledger.
- `server/migrations/866_collaboration_information_metadata.up.sql` adds the three nullable handoff fields. `867_collaboration_information_ready` supplies readiness/retention protection. The corresponding generated `IssueHandoff` fields and scanners must travel together; no new foreign key/cascade or index is introduced.
- `packages/core/admin/collaboration-schema.ts:3` switches its existing legacy consumer to `LegacyCollaborationStatusOptionsSchema`; omitting this small companion breaks the new repair union's `.safeExtend` consumer.
- `packages/core/collaboration/operation.ts` consumes exact repair confirmations and sealed semantic 503 through the existing original-intent owner; it needs the information schema, API transport and refusal-mapper changes in the same group.
- `packages/views/collaboration/workflow.tsx` reuses Core's explicit repair action while generic transfer/status capabilities remain false; the schema/presentation/form/history/i18n changes are one feature boundary.

The accepted S3 candidate was composed with a separately accepted but still partly uncommitted S2 foundation. This map does not claim that staging only S3 reconstructs the entire runtime candidate from clean HEAD, or authorize committing S2's other files. Preserve that source qualification. Any later clean-HEAD/full-branch validation belongs to the main integration checker; it is not a reason to swallow unrelated WIP here.

## Spec, evidence and archive handling

The existing dirty `.trellis/spec/server/collaboration-stage-approvals.md` and `.trellis/spec/web/frontend/e2e-run-environment.md` diffs inspected here describe S2 read-lock/blocker/toolbar work. They are not a ready S3 block-spec batch. Main should capture the accepted information block/repair/history/ETA/creation/registration contracts in a bounded S3 spec addition or exact S3 hunks and include only the corresponding accepted reports/evidence references. This researcher has not edited specs or shared task records.

The main session can now commit Group 1 and Group 2 with explicit paths and selective S3 hunks, after normal staged-diff/byte checks and the final quality gate. Then record its accepted R01–R09 disposition, exact commit links and remaining parent limitations and archive S3. S4 real inputs, final S5 and wider business sign-off remain parent work; they are not blockers to S3 behavior acceptance. The archive gate identified here is that accepted earlier S3 source is still uncommitted.

## External references

No external research or credential access. Source references are local owner deltas, retained accepted candidates/reports, current HEAD and selected `git diff`/`git show` outputs. The inspected HEAD advanced from `89740ed14` to the main session's independent UI commit `81fc27ac8` while this read ran; the selected block/E2E paths did not change.

## Caveats / Not Found

- No staging, commit, code/spec/shared-task write, test/build, DB/API/service/process command was performed. Only this report pair was written.
- The registration unit-test execution receipt was not found in the bounded evidence scope; its ownership/hash and the actual helper's R08 runtime acceptance are established separately.
- The known original legal-history-read 503 remains an unexplained historical failure; current accepted R05/R08 results do not erase it. This ownership review adds no new product finding.
- The exact 66-path map and five E2E/helper paths are finite; this is not an audit or authorization of all existing dirty files.
