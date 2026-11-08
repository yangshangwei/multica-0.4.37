# Remaining test / E2E source decisions

Pinned target: `e541cfb436fed9c6733174479718b651e2cf030d`. Audited: 2026-11-08T10:01:42.126418+08:00.

No additional product/test integration is recommended from this slice. The supposedly absent supplemental files are already integrated under canonical names, the context-audit regressions remain with stronger assertions, and the standalone macOS diagnostic adds network logging to an existing acceptance test.

## Coverage

Excludes the three Runtime sources, two Knowledge sources, recipient integration, two context-guard sources assigned to separate lanes, and the root checkout's existing dirty work. Sources were read only. No DB-mutating commands or tests were executed.

Reviewed 217 source/path comparisons:
- archive-generated-environment: 28.
- superseded-exact-product-history: 46.
- superseded-current-behavior: 9.
- archive-one-off-diagnostic: 1.
- already-integrated-renamed: 6.
- identical-to-target: 127.
- Additional task/research/local artifact entries retained in original archives: 149.

## Supplemental files are already present

| Original source | Current target | Proof |
| --- | --- | --- |
| `e2e/fixtures/supplemental-support.ts` | `e2e/fixtures/lane-guards.ts` | Entire helper matches current lane-guards.ts except its opening descriptive comment; guards, setup, SQL and restore logic are identical. |
| `e2e/supplemental-admin-collaboration-switches.spec.ts` | `e2e/admin-collaboration-switches.spec.ts` | Every line/assertion matches target after the fixture import rename; CLI spec additionally uses its canonical isolated profile name. No extra scenario to integrate. |
| `e2e/supplemental-builtin-agent-concurrency.spec.ts` | `e2e/builtin-agent-concurrency.spec.ts` | Every line/assertion matches target after the fixture import rename; CLI spec additionally uses its canonical isolated profile name. No extra scenario to integrate. |
| `e2e/supplemental-cli-forbidden-reason.spec.ts` | `e2e/cli-forbidden-reason.spec.ts` | Every line/assertion matches target after the fixture import rename; CLI spec additionally uses its canonical isolated profile name. No extra scenario to integrate. |
| `e2e/supplemental-collaboration-creation-default.spec.ts` | `e2e/collaboration-creation-default.spec.ts` | Every line/assertion matches target after the fixture import rename; CLI spec additionally uses its canonical isolated profile name. No extra scenario to integrate. |
| `e2e/supplemental-quick-create-autonomy.spec.ts` | `e2e/quick-create-autonomy.spec.ts` | Every line/assertion matches target after the fixture import rename; CLI spec additionally uses its canonical isolated profile name. No extra scenario to integrate. |

The five spec bodies are byte-identical after changing `./fixtures/supplemental-support` to `./fixtures/lane-guards`; the CLI case also changes only the isolated profile name from `supplemental` to `forbidden-reason`. The helper differs only in its opening comment. All test assertions are retained; copying these files under their old names would duplicate test execution.

## Context audit and other nonidentical tests

### mc-collaboration-final-owned-20261103: e2e/fixtures/collaboration-editing.ts

Current fixture already opens the portaled Parent issue picker and selects the exact identifier+title row, then checks saved parent/stage via API. Source variant 04daad uses parent.name instead of current title; f395 adds search indirection whose search/reset/UUID selection behavior is covered canonically by field-inputs and creation-form tests. Neither should replace the newer fixture.

- `e2e/fixtures/collaboration-editing.ts:114` at `e541cfb43` — const parentOption =
- `packages/views/collaboration/field-inputs.test.tsx:31` at `e541cfb43` — opens recent parent tasks without searching

### mc-collaboration-final3-20261102: e2e/mac-installed-network-diagnostic.spec.ts

All acceptance assertions are an exact copy of canonical installed-host test at 9de37277d; four additional lines only collect responses/request failures and attach network telemetry. No unique behavior assertion. Duplicating a 480-second installed-client lifecycle test for those observations would add another opt-in full run; keep this debugging harness in source archive.

- `e2e/collaboration-host-installed.spec.ts` at `9de37277d`
- `e2e/collaboration-host-installed.spec.ts:36` at `e541cfb43` — Packaged full Main controls a real native host run

### mc-collaboration-fixture-repair-20261103: e2e/fixtures/collaboration-editing.ts

Current fixture already opens the portaled Parent issue picker and selects the exact identifier+title row, then checks saved parent/stage via API. Source variant 04daad uses parent.name instead of current title; f395 adds search indirection whose search/reset/UUID selection behavior is covered canonically by field-inputs and creation-form tests. Neither should replace the newer fixture.

- `e2e/fixtures/collaboration-editing.ts:114` at `e541cfb43` — const parentOption =
- `packages/views/collaboration/field-inputs.test.tsx:31` at `e541cfb43` — opens recent parent tasks without searching

### mc-context-audit: packages/views/collaboration/workflow.test.tsx

Current same-named information-request test retains question/requester assertions and adds selected return status, accessible description, and absent fallback warning. Source adds no uncovered behavior.

- `packages/views/collaboration/workflow.test.tsx:415` at `e541cfb43` — shows the open information request to the person returning it

### mc-context-audit: server/cmd/multica/cmd_collaboration_test.go

Current same-named test preserves requests==0 and requires the precise runtime-local-path refusal; old source only checks the substring local and has stale bug commentary.

- `server/cmd/multica/cmd_collaboration_test.go:258` at `e541cfb43` — TestCollaborationCLICommentAddRejectsLocalPathLinksBeforeHTTP

### mc-context-audit: server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go

Current same-named shared-context authorization test retains the valid permit case and adds refusal of a token from an unissued permit; later query negotiation tests strengthen coverage.

- `server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go:18` at `e541cfb43` — TestHostProviderAccountSmokeAuthorizesSharedContextBase

### mc-context-audit: server/internal/daemon/collaborationrun/host_prompt_role_test.go

Current same-named member prompt test preserves role/step assertions, additionally requires dispatch_comment guidance, and verifies a leader does not get member guidance.

- `server/internal/daemon/collaborationrun/host_prompt_role_test.go:13` at `e541cfb43` — TestHostPromptTellsAMemberRunItsRoleAndStep

### mc-context-audit: server/internal/handler/collaboration_runtime_comment_size_test.go

Current same-named comment limit test exercises the production dedicated comment decoder, retains the 49600-byte HTML snippet, and adds worst-case comment/activity escaping and oversized refusal. Source calls generic runtime decoder, the old bug location.

- `server/internal/handler/collaboration_runtime_comment_size_test.go:19` at `e541cfb43` — TestCollaborationRuntimeCommentBodyAcceptsCodeWithinTheContentLimit

### mc-context-audit: server/internal/service/collaboration_runtime_preparation_test.go

Current same-named long-file-history regression retains promotion/claim after 33 deliveries, additionally verifies exactly the newest 32 files survive; adjacent referenced-file precedence regression preserves intended selection.

- `server/internal/service/collaboration_runtime_preparation_test.go:594` at `e541cfb43` — TestCollaborationRuntimePreparationIssueFileHistoryBeyondManifestLimitStillRuns

### mc-context-exp: server/internal/daemon/collaborationrun/host_context_experiment_test.go

One-off source explicitly says not for commit and predates preregistration. Current named harness retains scenarios while adding one-scenario authorization, fixed trial contracts, provider/model/framework provenance, durable per-trial evidence, and error/behavior scoring. Old loose trial iteration and truncated string records must stay archived; context-guard stop regressions are handled in a separate integration slice.

- `server/internal/daemon/collaborationrun/host_context_experiment_test.go:120` at `e541cfb43` — func TestHostContextExperiment

## Complete file decisions

The JSON companion retains source identity, actual blob, source/current and nearest-history diffs, exact-history commits and concrete semantic evidence. All references used for behavior reconciliation were verified against the pinned committed target, not a mutable working tree.

| Source | Path | Decision | Proof |
| --- | --- | --- | --- |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `e2e/fixtures/collaboration-detail-journey.ts` | identical-to-target | `e2e/fixtures/collaboration-detail-journey.ts` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `e2e/global-scope-navigation.spec.ts` | identical-to-target | `e2e/global-scope-navigation.spec.ts` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/core/diagnostics/diagnostic-context.test.ts` | identical-to-target | `packages/core/diagnostics/diagnostic-context.test.ts` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/core/diagnostics/diagnostic-context.ts` | identical-to-target | `packages/core/diagnostics/diagnostic-context.ts` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/views/collaboration/account-workbench-page.tsx` | identical-to-target | `packages/views/collaboration/account-workbench-page.tsx` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/views/locales/en/skills.json` | identical-to-target | `packages/views/locales/en/skills.json` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/views/locales/zh-Hans/skills.json` | identical-to-target | `packages/views/locales/zh-Hans/skills.json` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/views/skills/lib/skill-presentation.test.ts` | identical-to-target | `packages/views/skills/lib/skill-presentation.test.ts` |
| `/Volumes/artisan/code/2026/mc-acceptance-repair-20261107` | `packages/views/skills/lib/skill-presentation.ts` | identical-to-target | `packages/views/skills/lib/skill-presentation.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-account-desktop.spec.ts` | identical-to-target | `e2e/collaboration-account-desktop.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-boundaries-web.spec.ts` | identical-to-target | `e2e/collaboration-boundaries-web.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-delivery-web.spec.ts` | identical-to-target | `e2e/collaboration-delivery-web.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-knowledge.spec.ts` | identical-to-target | `e2e/collaboration-knowledge.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-leave-web.spec.ts` | identical-to-target | `e2e/collaboration-leave-web.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-memory-web.spec.ts` | identical-to-target | `e2e/collaboration-memory-web.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-records-web.spec.ts` | identical-to-target | `e2e/collaboration-records-web.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/collaboration-workbench-web.spec.ts` | identical-to-target | `e2e/collaboration-workbench-web.spec.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-control-gaps.ts` | identical-to-target | `e2e/fixtures/collaboration-control-gaps.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-detail-journey.ts` | superseded-exact-product-history | same product path at `c697b3477` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-disclosures.ts` | identical-to-target | `e2e/fixtures/collaboration-disclosures.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-knowledge.ts` | identical-to-target | `e2e/fixtures/collaboration-knowledge.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-mention-journeys.ts` | identical-to-target | `e2e/fixtures/collaboration-mention-journeys.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-resource-offers.ts` | identical-to-target | `e2e/fixtures/collaboration-resource-offers.ts` |
| `/Volumes/artisan/code/2026/mc-branch-e2e-4c817dee9-20261107` | `e2e/fixtures/collaboration-retained-attachments.ts` | identical-to-target | `e2e/fixtures/collaboration-retained-attachments.ts` |
| `/Volumes/artisan/code/2026/mc-check-20261026` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-candidate-c3db64b24-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103` | `e2e/collaboration-handoff-chain-web.spec.ts` | superseded-exact-product-history | same product path at `f429823b5` |
| `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103` | `e2e/fixtures/collaboration-handoff-chain.ts` | identical-to-target | `e2e/fixtures/collaboration-handoff-chain.ts` |
| `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103` | `e2e/fixtures/collaboration-offline-handoff.ts` | superseded-exact-product-history | same product path at `f429823b5` |
| `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/triage-focus-diagnostic-v15/runtime` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/triage-focus-diagnostic-v15/runtime` | `e2e/fixtures/triage-duplicate-picker.ts` | identical-to-target | `e2e/fixtures/triage-duplicate-picker.ts` |
| `/Volumes/artisan/code/2026/mc-collaboration-e2e-repair-20261103` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-final-knowledge-on-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103` | `e2e/collaboration-delivery-web.spec.ts` | superseded-exact-product-history | same product path at `b6c74033f` |
| `/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103` | `e2e/fixtures/collaboration-editing.ts` | superseded-current-behavior | `e2e/fixtures/collaboration-editing.ts:114` |
| `/Volumes/artisan/code/2026/mc-collaboration-final-password-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-final3-20261102` | `e2e/mac-installed-network-diagnostic.spec.ts` | archive-one-off-diagnostic | `e2e/collaboration-host-installed.spec.ts` |
| `/Volumes/artisan/code/2026/mc-collaboration-final4-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-collaboration-fixture-repair-20261103` | `e2e/fixtures/collaboration-editing.ts` | superseded-current-behavior | `e2e/fixtures/collaboration-editing.ts:114` |
| `/Volumes/artisan/code/2026/mc-collaboration-password-c3db64b24-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-context-audit` | `packages/views/collaboration/workflow.test.tsx` | superseded-current-behavior | `packages/views/collaboration/workflow.test.tsx:415` |
| `/Volumes/artisan/code/2026/mc-context-audit` | `server/cmd/multica/cmd_collaboration_test.go` | superseded-current-behavior | `server/cmd/multica/cmd_collaboration_test.go:258` |
| `/Volumes/artisan/code/2026/mc-context-audit` | `server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go` | superseded-current-behavior | `server/internal/daemon/collaborationrun/host_agentintegration_authorizer_test.go:18` |
| `/Volumes/artisan/code/2026/mc-context-audit` | `server/internal/daemon/collaborationrun/host_prompt_role_test.go` | superseded-current-behavior | `server/internal/daemon/collaborationrun/host_prompt_role_test.go:13` |
| `/Volumes/artisan/code/2026/mc-context-audit` | `server/internal/handler/collaboration_runtime_comment_size_test.go` | superseded-current-behavior | `server/internal/handler/collaboration_runtime_comment_size_test.go:19` |
| `/Volumes/artisan/code/2026/mc-context-audit` | `server/internal/service/collaboration_runtime_preparation_test.go` | superseded-current-behavior | `server/internal/service/collaboration_runtime_preparation_test.go:594` |
| `/Volumes/artisan/code/2026/mc-context-baseline-20261031` | `server/internal/daemon/collaborationrun/host_context_experiment_evidence_test.go` | identical-to-target | `server/internal/daemon/collaborationrun/host_context_experiment_evidence_test.go` |
| `/Volumes/artisan/code/2026/mc-context-baseline-20261031` | `server/internal/daemon/collaborationrun/host_context_experiment_fixture_test.go` | identical-to-target | `server/internal/daemon/collaborationrun/host_context_experiment_fixture_test.go` |
| `/Volumes/artisan/code/2026/mc-context-baseline-20261031` | `server/internal/daemon/collaborationrun/host_context_experiment_test.go` | identical-to-target | `server/internal/daemon/collaborationrun/host_context_experiment_test.go` |
| `/Volumes/artisan/code/2026/mc-context-exp` | `server/internal/daemon/collaborationrun/host_context_experiment_test.go` | superseded-current-behavior | `server/internal/daemon/collaborationrun/host_context_experiment_test.go:120` |
| `/Volumes/artisan/code/2026/mc-context-resume-20261031` | `server/internal/daemon/collaborationrun/host_context_experiment_fixture_test.go` | identical-to-target | `server/internal/daemon/collaborationrun/host_context_experiment_fixture_test.go` |
| `/Volumes/artisan/code/2026/mc-context-resume-20261031` | `server/internal/daemon/collaborationrun/host_context_experiment_test.go` | identical-to-target | `server/internal/daemon/collaborationrun/host_context_experiment_test.go` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/collaboration-handoff-chain-web.spec.ts` | superseded-exact-product-history | same product path at `f429823b5` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/collaboration-host-installed.spec.ts` | identical-to-target | `e2e/collaboration-host-installed.spec.ts` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/collaboration-installed-main.spec.ts` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/collaboration-leave-web.spec.ts` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/collaboration-terminal-reopen-web.spec.ts` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-control-gaps.ts` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-detail-journey.ts` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-handoff-chain.ts` | identical-to-target | `e2e/fixtures/collaboration-handoff-chain.ts` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-installed-main.mjs` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-installed-main.node-test.mjs` | identical-to-target | `e2e/fixtures/collaboration-installed-main.node-test.mjs` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-installed-main.ts` | identical-to-target | `e2e/fixtures/collaboration-installed-main.ts` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-lane-close-controls.ts` | identical-to-target | `e2e/fixtures/collaboration-lane-close-controls.ts` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-offline-handoff.ts` | superseded-exact-product-history | same product path at `f429823b5` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-offline-result-notifications.ts` | identical-to-target | `e2e/fixtures/collaboration-offline-result-notifications.ts` |
| `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104` | `e2e/fixtures/collaboration-retained-attachments.ts` | superseded-exact-product-history | same product path at `76836fa8d` |
| `/Volumes/artisan/code/2026/mc-core-fd-fix-20261105` | `server/cmd/multica/cmd_daemon.go` | identical-to-target | `server/cmd/multica/cmd_daemon.go` |
| `/Volumes/artisan/code/2026/mc-core-fd-fix-20261105` | `server/cmd/multica/daemon_fd_linux.go` | identical-to-target | `server/cmd/multica/daemon_fd_linux.go` |
| `/Volumes/artisan/code/2026/mc-core-fd-fix-20261105` | `server/cmd/multica/daemon_fd_linux_test.go` | identical-to-target | `server/cmd/multica/daemon_fd_linux_test.go` |
| `/Volumes/artisan/code/2026/mc-core-fd-fix-20261105` | `server/cmd/multica/daemon_fd_other.go` | identical-to-target | `server/cmd/multica/daemon_fd_other.go` |
| `/Volumes/artisan/code/2026/mc-final-legacy-diagnostic-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102` | `e2e/collaboration-installed-main.spec.ts` | superseded-exact-product-history | same product path at `b6c74033f` |
| `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102` | `e2e/collaboration-memory-web.spec.ts` | superseded-exact-product-history | same product path at `b6c74033f` |
| `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102` | `e2e/fixtures/collaboration-installed-main.mjs` | superseded-exact-product-history | same product path at `b6c74033f` |
| `/Volumes/artisan/code/2026/mc-functional-7af9ab990-20261102` | `e2e/fixtures/collaboration-recipient-picker.mjs` | identical-to-target | `e2e/fixtures/collaboration-recipient-picker.mjs` |
| `/Volumes/artisan/code/2026/mc-functional-go-7af9ab990-20261102` | `server/internal/handler/daemon_test.go` | identical-to-target | `server/internal/handler/daemon_test.go` |
| `/Volumes/artisan/code/2026/mc-h5-quote-20261105` | `server/cmd/server/rk1_entry_test.go` | identical-to-target | `server/cmd/server/rk1_entry_test.go` |
| `/Volumes/artisan/code/2026/mc-h5-quote-20261105` | `server/cmd/server/rk1_fake_test.go` | identical-to-target | `server/cmd/server/rk1_fake_test.go` |
| `/Volumes/artisan/code/2026/mc-memory-gate-green-env-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-memory-gate-red-env-20261102` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-push-check-20261026` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-runtime-integration-20261031` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-ux-e2e-49e991156-20261105` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/mc-ux-e2e-49e991156-20261105` | `e2e/collaboration-delivery-web.spec.ts` | superseded-exact-product-history | same product path at `88d25e6a2` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/collaboration-acceptance-current-20261026/collaboration-source` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/collaboration-acceptance-current-20261026/collaboration-source` | `e2e/collaboration-host-installed.spec.ts` | superseded-exact-product-history | same product path at `4d9f47928` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/collaboration-acceptance-current-20261026/collaboration-source` | `e2e/fixtures/collaboration-installed-main.mjs` | superseded-exact-product-history | same product path at `4d9f47928` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/collaboration-acceptance-current-20261026/collaboration-source` | `e2e/fixtures/collaboration-installed-main.ts` | superseded-exact-product-history | same product path at `4d9f47928` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/e2e-collab-8f80b8700/source` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/e2e-collab-8f80b8700/source-password` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/baseline-a8e374bb9` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `apps/docs/content/docs/agents.mdx` | identical-to-target | `apps/docs/content/docs/agents.mdx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `apps/docs/content/docs/agents.zh.mdx` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `apps/web/platform/scope-shell.test.tsx` | identical-to-target | `apps/web/platform/scope-shell.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `apps/web/platform/scope-shell.tsx` | superseded-exact-product-history | same product path at `55694c8e3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/admin-collaboration-switches.spec.ts` | identical-to-target | `e2e/admin-collaboration-switches.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/builtin-agent-concurrency.spec.ts` | identical-to-target | `e2e/builtin-agent-concurrency.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/cli-forbidden-reason.spec.ts` | identical-to-target | `e2e/cli-forbidden-reason.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/collaboration-creation-default.spec.ts` | identical-to-target | `e2e/collaboration-creation-default.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/collaboration-selection-boundary.spec.ts` | identical-to-target | `e2e/collaboration-selection-boundary.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/fixtures/lane-guards.ts` | identical-to-target | `e2e/fixtures/lane-guards.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/localized-template-defaults.spec.ts` | superseded-exact-product-history | same product path at `37e1829a9` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/progress-reporting.spec.ts` | identical-to-target | `e2e/progress-reporting.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `e2e/quick-create-autonomy.spec.ts` | identical-to-target | `e2e/quick-create-autonomy.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/core/agents/autonomy.test.ts` | identical-to-target | `packages/core/agents/autonomy.test.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/core/agents/autonomy.ts` | identical-to-target | `packages/core/agents/autonomy.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/core/agents/index.ts` | identical-to-target | `packages/core/agents/index.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/core/issues/creator-selection.test.ts` | identical-to-target | `packages/core/issues/creator-selection.test.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/core/issues/creator-selection.ts` | identical-to-target | `packages/core/issues/creator-selection.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/chat/chat-page.test.tsx` | identical-to-target | `packages/views/chat/chat-page.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/chat/chat-page.tsx` | identical-to-target | `packages/views/chat/chat-page.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/inbox/components/inbox-page.test.tsx` | identical-to-target | `packages/views/inbox/components/inbox-page.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/inbox/components/inbox-page.tsx` | identical-to-target | `packages/views/inbox/components/inbox-page.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/locales/en/inbox.json` | identical-to-target | `packages/views/locales/en/inbox.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/locales/en/modals.json` | identical-to-target | `packages/views/locales/en/modals.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/locales/zh-Hans/inbox.json` | identical-to-target | `packages/views/locales/zh-Hans/inbox.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/locales/zh-Hans/modals.json` | identical-to-target | `packages/views/locales/zh-Hans/modals.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/modals/quick-create-actor-picker.test.tsx` | identical-to-target | `packages/views/modals/quick-create-actor-picker.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/modals/quick-create-actor-picker.tsx` | identical-to-target | `packages/views/modals/quick-create-actor-picker.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/modals/quick-create-issue.test.tsx` | identical-to-target | `packages/views/modals/quick-create-issue.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `packages/views/modals/quick-create-issue.tsx` | identical-to-target | `packages/views/modals/quick-create-issue.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/docs/content/pages/agents.json` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/handler/issue.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/handler/quick_create_autonomy_test.go` | identical-to-target | `server/internal/handler/quick_create_autonomy_test.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/handler/source_context.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/handler/task_lifecycle.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/integrations/slack/slash_command.go` | identical-to-target | `server/internal/integrations/slack/slash_command.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/integrations/slack/slash_command_test.go` | identical-to-target | `server/internal/integrations/slack/slash_command_test.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/service/builtin_agent_autonomy_test.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source` | `server/internal/service/task.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `apps/docs/content/docs/agents.mdx` | identical-to-target | `apps/docs/content/docs/agents.mdx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `apps/docs/content/docs/agents.zh.mdx` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `apps/web/platform/scope-shell.test.tsx` | identical-to-target | `apps/web/platform/scope-shell.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `apps/web/platform/scope-shell.tsx` | superseded-exact-product-history | same product path at `55694c8e3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/admin-collaboration-switches.spec.ts` | identical-to-target | `e2e/admin-collaboration-switches.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/builtin-agent-concurrency.spec.ts` | identical-to-target | `e2e/builtin-agent-concurrency.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/cli-forbidden-reason.spec.ts` | identical-to-target | `e2e/cli-forbidden-reason.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/collaboration-creation-default.spec.ts` | identical-to-target | `e2e/collaboration-creation-default.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/collaboration-selection-boundary.spec.ts` | identical-to-target | `e2e/collaboration-selection-boundary.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/fixtures/lane-guards.ts` | identical-to-target | `e2e/fixtures/lane-guards.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/localized-template-defaults.spec.ts` | superseded-exact-product-history | same product path at `37e1829a9` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/progress-reporting.spec.ts` | identical-to-target | `e2e/progress-reporting.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `e2e/quick-create-autonomy.spec.ts` | identical-to-target | `e2e/quick-create-autonomy.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/core/agents/autonomy.test.ts` | identical-to-target | `packages/core/agents/autonomy.test.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/core/agents/autonomy.ts` | identical-to-target | `packages/core/agents/autonomy.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/core/agents/index.ts` | identical-to-target | `packages/core/agents/index.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/core/issues/creator-selection.test.ts` | identical-to-target | `packages/core/issues/creator-selection.test.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/core/issues/creator-selection.ts` | identical-to-target | `packages/core/issues/creator-selection.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/chat/chat-page.test.tsx` | identical-to-target | `packages/views/chat/chat-page.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/chat/chat-page.tsx` | identical-to-target | `packages/views/chat/chat-page.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/inbox/components/inbox-page.test.tsx` | identical-to-target | `packages/views/inbox/components/inbox-page.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/inbox/components/inbox-page.tsx` | identical-to-target | `packages/views/inbox/components/inbox-page.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/locales/en/inbox.json` | identical-to-target | `packages/views/locales/en/inbox.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/locales/en/modals.json` | identical-to-target | `packages/views/locales/en/modals.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/locales/zh-Hans/inbox.json` | identical-to-target | `packages/views/locales/zh-Hans/inbox.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/locales/zh-Hans/modals.json` | identical-to-target | `packages/views/locales/zh-Hans/modals.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/modals/quick-create-actor-picker.test.tsx` | identical-to-target | `packages/views/modals/quick-create-actor-picker.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/modals/quick-create-actor-picker.tsx` | identical-to-target | `packages/views/modals/quick-create-actor-picker.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/modals/quick-create-issue.test.tsx` | identical-to-target | `packages/views/modals/quick-create-issue.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `packages/views/modals/quick-create-issue.tsx` | identical-to-target | `packages/views/modals/quick-create-issue.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/docs/content/pages/agents.json` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/handler/issue.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/handler/quick_create_autonomy_test.go` | identical-to-target | `server/internal/handler/quick_create_autonomy_test.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/handler/source_context.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/handler/task_lifecycle.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/integrations/slack/slash_command.go` | identical-to-target | `server/internal/integrations/slack/slash_command.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/integrations/slack/slash_command_test.go` | identical-to-target | `server/internal/integrations/slack/slash_command_test.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/service/builtin_agent_autonomy_test.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-password` | `server/internal/service/task.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `apps/docs/content/docs/agents.mdx` | identical-to-target | `apps/docs/content/docs/agents.mdx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `apps/docs/content/docs/agents.zh.mdx` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `apps/web/platform/scope-shell.test.tsx` | identical-to-target | `apps/web/platform/scope-shell.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `apps/web/platform/scope-shell.tsx` | superseded-exact-product-history | same product path at `55694c8e3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/collaboration-selection-boundary.spec.ts` | identical-to-target | `e2e/collaboration-selection-boundary.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/fixtures/supplemental-support.ts` | already-integrated-renamed | `e2e/fixtures/lane-guards.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/supplemental-admin-collaboration-switches.spec.ts` | already-integrated-renamed | `e2e/admin-collaboration-switches.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/supplemental-builtin-agent-concurrency.spec.ts` | already-integrated-renamed | `e2e/builtin-agent-concurrency.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/supplemental-cli-forbidden-reason.spec.ts` | already-integrated-renamed | `e2e/cli-forbidden-reason.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/supplemental-collaboration-creation-default.spec.ts` | already-integrated-renamed | `e2e/collaboration-creation-default.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `e2e/supplemental-quick-create-autonomy.spec.ts` | already-integrated-renamed | `e2e/quick-create-autonomy.spec.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/core/agents/autonomy.test.ts` | identical-to-target | `packages/core/agents/autonomy.test.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/core/agents/autonomy.ts` | identical-to-target | `packages/core/agents/autonomy.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/core/agents/index.ts` | identical-to-target | `packages/core/agents/index.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/core/issues/creator-selection.test.ts` | identical-to-target | `packages/core/issues/creator-selection.test.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/core/issues/creator-selection.ts` | identical-to-target | `packages/core/issues/creator-selection.ts` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/chat/chat-page.test.tsx` | identical-to-target | `packages/views/chat/chat-page.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/chat/chat-page.tsx` | identical-to-target | `packages/views/chat/chat-page.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/inbox/components/inbox-page.test.tsx` | identical-to-target | `packages/views/inbox/components/inbox-page.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/inbox/components/inbox-page.tsx` | identical-to-target | `packages/views/inbox/components/inbox-page.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/locales/en/inbox.json` | identical-to-target | `packages/views/locales/en/inbox.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/locales/en/modals.json` | identical-to-target | `packages/views/locales/en/modals.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/locales/zh-Hans/inbox.json` | identical-to-target | `packages/views/locales/zh-Hans/inbox.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/locales/zh-Hans/modals.json` | identical-to-target | `packages/views/locales/zh-Hans/modals.json` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/modals/quick-create-actor-picker.test.tsx` | identical-to-target | `packages/views/modals/quick-create-actor-picker.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/modals/quick-create-actor-picker.tsx` | identical-to-target | `packages/views/modals/quick-create-actor-picker.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/modals/quick-create-issue.test.tsx` | identical-to-target | `packages/views/modals/quick-create-issue.test.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `packages/views/modals/quick-create-issue.tsx` | identical-to-target | `packages/views/modals/quick-create-issue.tsx` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/docs/content/pages/agents.json` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/handler/issue.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/handler/quick_create_autonomy_test.go` | identical-to-target | `server/internal/handler/quick_create_autonomy_test.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/handler/source_context.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/handler/task_lifecycle.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/integrations/slack/slash_command.go` | identical-to-target | `server/internal/integrations/slack/slash_command.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/integrations/slack/slash_command_test.go` | identical-to-target | `server/internal/integrations/slack/slash_command_test.go` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/service/builtin_agent_autonomy_test.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/full-e2e-20261029/source-supplemental` | `server/internal/service/task.go` | superseded-exact-product-history | same product path at `c32977cc3` |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/recent-commits-e2e-20261027/native-source` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |
| `/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/recent-commits-e2e-20261027/password-source` | `apps/web/next-env.d.ts` | archive-generated-environment | Next.js generated ambient route-types import varies with local dev/build output; not a product change. |

## Handoff

No production/test patch requested. Keep unmerged debugging/evidence files in the existing all-source archives. Normalized identity is proof of duplicate assertions, not a claim that tests passed in this session. Coordinating session still owns targeted and full quality gates for the actually integrated changes.
