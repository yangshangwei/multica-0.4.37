# Research: S2 remaining commit readiness

- Query: Which remaining PAY-241 S2 changes can be committed and pushed after the accepted S3/UI commits, without absorbing concurrent Core/UI work?
- Scope: internal; selected dirty source/spec files, accepted R33 manifests/reports and read-only Git
- Date: 2026-11-19

## Findings

**All 50 selected S2 code/test/E2E paths have accepted-source provenance.** Forty-seven match R33 byte-for-byte. Three preserve already committed S3 changes while their current HEAD→ROOT delta is exactly the accepted S2 change: `collaboration_workflow.go`'s one account read-lock line and six known refusal labels in each collaboration locale. Those three are now safe whole-file commit candidates because the S3 portions are already in HEAD. No selected product delta is unproved by source mismatch.

The recommended three commit groups contain **54 paths total: 34 server code/test/query files + 3 server specs; 9 stage frontend/test/locale files; 7 E2E helper/spec/test files + 1 E2E spec document**. Exact machine-readable `groups.*.code_paths`, `spec_paths`, `commit_paths`, `all_code_paths` and `all_commit_paths` are in the adjacent JSON. Main owns staged-source checks, commits and push. S2 archival is not requested or necessary for pushing these qualified commits.

Inspected HEAD: `7ad863a08` (following `81fc27ac8`, `f6703b7e4` and `2a46ff823`). The active Core/UI work and unrelated E2E changes are explicitly excluded. The four spec diffs describe precisely these S2 contracts; their prose was reviewed against the accepted implementation. Specs are not claimed to be executable-source manifest entries.

### Group 1 — compatible evidence/preview/receipt reads, idle claim and M1 policy wait

```text
server/internal/handler/delivery_policy.go
server/internal/service/collaboration_completion_preview.go
server/internal/service/collaboration_runtime_preparation_capabilities.go
server/internal/service/collaboration_stage_completion.go
server/internal/service/collaboration_stage_evidence.go
server/internal/service/collaboration_stage_evidence_read.go
server/internal/service/collaboration_stage_execution_approvals.go
server/internal/service/collaboration_stage_qa.go
server/internal/service/collaboration_workflow.go
server/internal/service/delivery_completion_authorization.go
server/internal/service/delivery_completion_collaboration.go
server/internal/service/delivery_completion_preview.go
server/internal/service/delivery_evidence.go
server/internal/service/managed_runtime.go
server/internal/service/task.go
server/pkg/db/generated/agent.sql.go
server/pkg/db/generated/delivery_policy.sql.go
server/pkg/db/queries/agent.sql
server/pkg/db/queries/delivery_policy.sql
server/internal/handler/delivery_policy_project_wait_test.go
server/internal/service/collaboration_completion_candidates_read_concurrency_test.go
server/internal/service/collaboration_completion_preview_admin_submission_test.go
server/internal/service/collaboration_completion_preview_managed_submission_test.go
server/internal/service/collaboration_completion_preview_read_concurrency_test.go
server/internal/service/collaboration_runtime_capabilities_read_concurrency_test.go
server/internal/service/collaboration_stage_evidence_read_concurrency_test.go
server/internal/service/collaboration_stage_execution_read_concurrency_test.go
server/internal/service/collaboration_stage_execution_run_read_fences_test.go
server/internal/service/collaboration_workflow_read_concurrency_test.go
server/internal/service/delivery_completion_consumed_read_concurrency_test.go
server/internal/service/task_idle_claim_preflight_failure_test.go
server/internal/service/task_idle_claim_read_concurrency_test.go
server/pkg/db/generated/delivery_evidence_read.sql.go
server/pkg/db/queries/delivery_evidence_read.sql
.trellis/spec/server/collaboration-stage-approvals.md
.trellis/spec/server/delivery-completion-enforcement.md
.trellis/spec/server/delivery-policy-evaluations.md
```

These files form one backend dependency closure. The shared evidence SQL and generated scanners must accompany the private read wrappers and the original-writer controls. Preview paths require the managed submission/UTC fix plus shared nested issuer/run/state reads. The receipt reader must preserve its current authority/immutable receipt checks. Capability GET uses its explicitly private shared account mode while original registration/claim writers retain `false`. The idle batch probe skips only a reliable negative reclaim check; its query/getter and error test are necessary companions. The M1 handler's waiting query/getter/test stay together; stage NOWAIT and the existing 250 ms budget are unchanged. Splitting out only tests or generated files is not a complete candidate.

`server/internal/service/collaboration_workflow.go` is the only nonidentical R33 whole file in this group. Its remaining diff is exactly:

```diff
- users, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{actor})
+ users, err := q.LockArtifactUsers(ctx, []pgtype.UUID{actor})
```

The accepted R33 source contains the new line. Reversing only that line in current ROOT reproduces current HEAD exactly, proving the already committed S3 fields/transaction/repair behavior remain unchanged. Do not copy R33's old whole file over ROOT.

### Group 2 — stage blocker decoding, source denial and labels

```text
packages/core/collaboration/stage-code-presentation.test.ts
packages/core/collaboration/stage-schema.test.ts
packages/core/collaboration/stage-schema.ts
packages/views/collaboration/stage-code-panel.test.tsx
packages/views/collaboration/stage-code-panel.tsx
packages/views/collaboration/stage-labels.ts
packages/views/locales/en/collaboration.json
packages/views/locales/zh-Hans/collaboration.json
packages/views/collaboration/stage-labels.test.ts
```

The known-code schema, stored-409 classification, label switch, both real catalogs and their tests must remain synchronized. `stage-code-panel.tsx` keeps source-denial feedback visible after private candidate cache clearing. Its `collaborationStageSourceDenied` dependency and `stage-code-presentation` export are already committed and clean; they are not an additional dirty path to collect.

For both locale files, the remaining six lines exactly equal R33 values. Removing only those six lines from current ROOT reproduces HEAD byte-for-byte. The committed S3 `information` subtree and activity kind remain untouched. The six keys are `stage_execution_unavailable`, `stage_execution_target_changed`, `stage_execution_pending`, `stage_execution_conflict`, `stage_rollback_observation_unavailable` and `stage_rollback_observation_changed`.

### Group 3 — exact actual-stage harness behavior

```text
e2e/collaboration-stage-execution.spec.ts
e2e/fixtures/collaboration-host-runtime.node-test.mjs
e2e/fixtures/collaboration-host-runtime.ts
e2e/fixtures/collaboration-stage-control.node-test.mjs
e2e/fixtures/collaboration-stage-control.ts
e2e/fixtures/collaboration-stage-execution.ts
e2e/fixtures/collaboration-stage-execution.node-test.mjs
.trellis/spec/web/frontend/e2e-run-environment.md
```

All seven code/test files equal the R33 consumed frontend source exactly. They preserve scoped host-target selection and actual response correlation, provider opaque conditional state within the existing 256-byte bound, the original 120-second metric horizon, and direct selection of the already visible Submit result toolbar action. The corresponding node tests and source-bound original four-case acceptance remain distinct evidence. No production simulator or alternate business writer is introduced.

## Accepted source binding and evidence

- Final R33 acceptance: `r33-full-s2-a01-a12-independent-check-20261118.json`, SHA `cb5120ea32a92ad4cd11445103ea2b6af3322476b311d382ae647e09c34e69f7`, accepts all **12 A / 18 W / 12 Q** within agreed local S2 scope. Its 18 linked report hashes were checked here: zero missing/drift.
- Backend source: `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-green-r01`, exact manifest `build-final-r01/source-after.json` (5,056 files). Every selected backend source file is manifest-bound; 33/34 current files are byte-exact, with the one workflow composition explained above.
- Frontend source: `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/frontend-base-r01`, selected by R33 `runtime/current-consuming-source-before-four-r01.json` (3,359 files). All selected source files match their accepted manifest; current files are byte-exact except the two known committed-S3 locale compositions.
- Source mode qualification: private frontend copies are generally 0600 and ROOT source remains 0644; the retained workflow regression likewise has private 0600/ROOT 0644. These are recorded non-executable source-mode differences, not code drift or an instruction to chmod ROOT. Other selected modes match.
- R33 ROOT preparation (`7a96fa83…`) binds its four integration paths, scoped gofmt/vet/compile and five actual successful program builds. Its earlier pending runtime label is superseded by the full R33 report, not rewritten.
- R33 runtime-after (`c65b98d2…`) accepts the original four complete journeys **4/4, retry 0**, matching production Web/native renderer/preload/API/CLI, original receipts, 76 settled runs/62 complete artifacts, restored gate epoch 90 and owned API/bridge stop/zero connections. Real external systems and installed production Main remain outside that scope.

### Relevant retained verification, not new executions

| Slice | Accepted retained result and reference |
| --- | --- |
| Original server contracts | Service 116 tops / 321 named (37/128 + 79/193), handler 99/326, router 54/208, each normal/race; eight exact raw streams are bound in the final R33 report and copied as references into this JSON. |
| Shared stage/artifact/issuer/run reads | Accepted R17–R20 read/default/writer/revocation proof carried by final R33 source bridge and original scope map. Current selected products/tests/SQL match the final source exactly. |
| Original workflow receipt GET | R21/R22 continuation accepts shared account read with original writer/current-authority controls; current 214-line regression bytes match R33. |
| Capability GET | `r22-capabilities-read-red-green-check-20261116.*`: normal 4 tops/8 names and combined race 7/25, matching products and writer/current-authority controls. |
| Candidate read | `r26-candidates-read-green-independent-check-20261117.*`: each mode 4 tops/15 names/11 leaves. |
| Managed/R38 submission binding + UTC | `r27-preview-binding-utc-followup-independent-check-20261117.*`: each mode 2 tops/8 names/6 leaves. |
| Nested submission preview read | `r28-preview-read-green-independent-check-20261117.*`: each mode 26 named checks with original writer defaults. |
| Consumed authorization reads | R29: each mode 76 service + 6 handler names, original 34-function/three-default-body/caller proof retained. |
| Idle batch claim | R31: each mode 34 service + 1 temporal handler + 2 cache controls; SQL/getter generation and error/non-authorizing behavior checked. |
| M1 policy wait | R33: each mode 10 tops/40 names/33 leaves, actual short/long reader, writer/CAS and authority controls; canonical generation preserved other outputs. |
| Stage frontend | R11/R12: 496 Core + 143 Views parameter records, five typechecks, two lints and two builds. R20 source-denial panel adds separate Core 44/Views 22 checks. Counts overlap across increments and are not added into a larger business denominator. |
| E2E helpers | R13 host-target fixture source/static proof, R23 codec 16/16 plus strict type/lint, R24 selector strict type/nonignored lint/list; current R33 four journeys give actual composed acceptance. |

The adjacent JSON retains each file's current/HEAD/R33 hash, manifest entry and classification, and references the original raw results. This lane did not rerun or reparse those accepted test bodies. A new isolated HEAD-plus-selected-files source check is a reasonable final push prerequisite owned by Main; it should not silently consume concurrent WIP or claim a new whole `make check` pass.

## Code patterns and dependency closure

- `server/internal/service/collaboration_stage_evidence_read.go:15` introduces the private transaction marker; `delivery_evidence.go:80`/`:235` retain constant-false writer entries, while read-specific branches consume `delivery_evidence_read.sql`. Current product/query/generated files and all dependent regressions are in Group 1.
- `server/internal/service/collaboration_completion_preview.go:188` selects nested read helpers only for validated submission projection, retains the original checker binding, and normalizes expiry before final comparison. Corresponding managed/admin/read-concurrency tests are included.
- `server/internal/service/delivery_completion_authorization.go:222` selects the original issuer's GetByID read path; `delivery_completion_collaboration.go:44`, `:96` and `:216` separate read entry points while preserving original defaults.
- `server/internal/service/task.go:3995` calls the new scalar necessary-condition query; its generated getter and error/real-reclaim tests are in the same group.
- `server/internal/handler/delivery_policy.go:135` is the sole M1 waiting-query call. Current stage-policy query/caller remains committed and unchanged.
- `packages/core/collaboration/stage-schema.ts:161` and `:582`, `packages/views/collaboration/stage-labels.ts:50` and both catalogs share the exact six codes. The source-denial helper/export is already in HEAD.
- `e2e/fixtures/collaboration-stage-control.ts:148`, `collaboration-stage-execution.ts:299`, and `e2e/collaboration-stage-execution.spec.ts:468` keep distinct provider-state, metric-horizon and active-surface contracts.

All 34 dirty server paths observed in the selected backend scope are included. No new migration or dependency is introduced by this remaining S2 batch. Original S2 migrations 808–843/861–864 are already committed and source-qualified by R33; preserve S3 migrations 866/867 and output migration 868 already in HEAD. The backend's new query helper symbols are resolved by the included query/generated pair; the frontend's new label/schema dependencies are resolved by Group 2.

## Explicitly excluded work

Concurrent quick-create/creator recommendations/chat actor/UI changes, `packages/core/issues` and `packages/views/modals` work, and these unrelated E2E files are outside the S2 groups: chat-actor-picker, core-business-agent-loops, issue-assist-flow, quick-create-actor-picker-phase2, quick-create-actor-picker, quick-create-autonomy and upstream-selected-fixes. Other task records, shared journals and S2 historical research are not swept into a code commit. Main may select necessary evidence/bookkeeping in a separate explicit batch.

## Related specs and external references

The four included spec diffs align with their code groups: stage read/default/current-authority contracts; consumed-receipt and managed-preview contracts; M1-only wait/CAS/budget; and toolbar versus modal-menu fixture selection. No external research or credentials were used. Version/source facts remain the local accepted R33 artifacts and current HEAD; real S4, final S5, business signatures and installed Main are separate product acceptance work, not reasons to hold these qualified source commits.

## Caveats / Not Found

- No unproved source change remains among the selected 50. This is a commit-ownership/readiness conclusion, not a fresh runtime, clean-HEAD build, CI or whole-repository verification claim.
- Original failures, old unknown lock holders, the eight historical F2 cleanup-inventory failures and all earlier failed rounds remain recorded. No passing S2 count erases them.
- This researcher staged/committed/pushed nothing and ran no DB/API/services/tests/builds. Only this report pair was written. Main owns current candidate validation and the authorized commit/push.
