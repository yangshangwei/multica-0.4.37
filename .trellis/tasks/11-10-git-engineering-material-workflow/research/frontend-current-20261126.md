# Research: Current frontend and plugin boundaries for Git engineering materials

- Query: Reuse shared project/issue views for the requirement draft → automatic publication → human review → verified Git baseline → plan-stage task workflow without conflating it with existing collaboration approvals.
- Scope: internal; source inspection only, no product files changed and no tests or external integrations executed.
- Date: 2026-11-26
- Baseline: the parent session reports branch `codex/git-engineering-material-workflow`, created from local main at `2c2010f273eddd7a02fdff95da52c0ba4ed4eb99`.

## Findings

### 1. Existing shared project entry is sufficient; Git material behavior is new

| Existing surface | Evidence | Reuse and limits |
| --- | --- | --- |
| Shared project detail | `packages/views/projects/components/project-detail.tsx:547` mounts resources and knowledge in the sidebar; `:640` mounts iterations and execution squad; `:642` switches between overview and tasks | A material configuration section can live beside resources. A project material collection/detail is new behavior but does not require a new top-level application page. |
| Both platform entry points | `apps/web/app/[workspaceSlug]/(dashboard)/projects/[id]/page.tsx:3`; `apps/desktop/src/renderer/src/pages/project-detail-page.tsx:3`; desktop route at `apps/desktop/src/renderer/src/routes.tsx:148` | Both already render the same `ProjectDetail`. In-place content can ship to both without duplicating routing. If adding a URL section, update the shared path builder and consume it through `NavigationAdapter`. |
| Resource selection | `packages/views/projects/components/project-resources-section.tsx:106`, `:178`; `packages/core/types/project.ts:92` | Resources are ordinary typed pointers. Known kinds are `github_repo` and `local_directory`; the first is used for Git URLs across providers. It does not say which repository is a material output, prove write/merge permission, or configure an allowed material root. |
| Provider naming | `packages/core/vcs/use-repo-provider.ts:13`; `packages/core/vcs/repo-provider.ts:72` | Selects presentation from workspace GitHub installations and VCS connections, with neutral Git naming for multiple/unavailable providers. This is not a connection credential selector or capability proof. |
| Resource data loading | `packages/core/projects/resource-queries.ts:12`; `packages/core/api/client.ts:6556` | Scoped Query keys, captured workspace reads, and identity validation are reusable. Existing create/update methods at `client.ts:6572` return raw typed fetches; do not copy this old parser gap into new APIs. |

`Project` currently has no material configuration (`packages/core/types/project.ts:22`). `ProjectResource` has no material-purpose field (`:140`). Searches of shared core/views found no dedicated Git material revision/baseline API or page. The word “materialized” in collaboration means child-task creation, not Git material publication.

Recommended UI minimum is two shared components: a project material configuration panel and a material workflow panel with list/selected detail. The latter needs the document content and formal baseline, not just a PR link. It can be mounted under a project-local entry or linked from a task; routing should remain shared. Do not bury long PRDs in the 320px resource sidebar.

### 2. Existing collaboration approvals are useful patterns, not the missing requirement model

`packages/views/collaboration/collaboration-detail.tsx:162` mounts `CollaborationStagePanel`, followed by delivery and discussion. The current stage contract provides:

- Human roles (`business_owner`, `implementer`, `product_approver`, `code_reviewer`, `qa_approver`, `release_approver`) and versioned role configuration: `packages/core/collaboration/stage-schema.ts:37`.
- Proposal versions, immutable content digests, author/source attribution, decisions, and created-child mappings: `stage-schema.ts:283`–`:347`.
- Separate accept, modify-and-accept, reject, and revise actions pinned to the displayed candidate: `packages/views/collaboration/stage-proposals.tsx:98`.
- Stale/unknown/source-unavailable states; a pending uncertain request blocks new writes, with an explicit original-request recovery action: `packages/views/collaboration/stage-panel.tsx:74`, `:93`.
- Captured source-workspace data and protected Query caches; normal polling is 15 seconds: `packages/core/collaboration/stage-queries.ts:16`, `:31`.
- Immutable original request bodies and `Idempotency-Key` sends; read-only receipt lookup does not trigger a write: `packages/core/api/client.ts:1978`, `:2005`.

However, current proposal content is `{summary, children:[{title,description,priority}]}`, not a PRD with Git repository/path/head/baseline. Its stages start at `development`; approval can materialize ordinary children. Current project **stage policy defaults to disabled**, whereas D8's new **start plan after requirement merge** setting defaults to enabled. Reusing the existing policy boolean would change unrelated semantics (`stage-schema.ts:59`–`:70`).

The stage authority spec also explicitly keeps stage signatures separate from dispatch and warns against enabling policy before every final writer consumes its guard: `.trellis/spec/server/collaboration-stage-approvals.md:1`. Reuse patterns and downstream interfaces only after the backend chooses a deliberate join. Adding `plan` to the existing enum or accepting a task proposal is not an implementation of D7/D8.

`CollaborationDeliveryPanel` shows human deliverable submission/decision and attachment evidence (`packages/views/collaboration/delivery-panel.tsx:20`). It does not prove PRD publication or requirement merge. Ordinary PR rows (`packages/views/issues/components/pull-request-list.tsx:46`) are external links/status summaries, not an in-Multica document review surface.

### 3. The plugin target requires a real host extension

The SDK documents credential-free sandboxed surfaces (`packages/plugin-sdk/index.ts:1`). It only exposes context, issue title/description, comments, storage and hooks (`:199`). The host allowlist is correspondingly limited (`packages/views/plugins/surface-bridge.ts:24`). It rejects unknown paths before calling `api.callPluginAction`.

The server Action API checks enabled installation, granted scope and current caller authority (`server/internal/handler/plugin_action.go:26`). Human-session and plugin-token identities differ; installation/event tokens act as a plugin, while UI/manual callback grants may carry a human (`:49`, `:160`). A material approval endpoint must not accept a caller-supplied reviewer or let an event callback manufacture a human approval. The server must define whether human review requires an actual interactive session instead of generic `requireMember` alone.

`PluginPanelSection` only mounts enabled `issue_panel` surfaces, filtered by platform (`packages/views/plugins/plugin-panel-section.tsx:21`). In ordinary `IssueDetail`, its mount is gated by `executionAllowed` (`packages/views/issues/components/issue-detail.tsx:2623`). Collaboration issues return early into `SourceCollaborationDetail` (`:1205`), whose detail does not mount this plugin section. Therefore “put it in an existing plugin panel” does not automatically make it available to product reviewers or collaboration issues.

Two valid delivery choices require an explicit implementation boundary:

1. Host-owned shared material views + narrow host APIs first, with plugin exposure treated as a separate incomplete acceptance slice; this is the smallest usable product increment.
2. Preserve plugin delivery in the first increment by adding matching SDK operations, host allowlist, Action API scopes/authorization, installation manifest and a concrete surface. The host must still own durable workflow state, actor identity, recovery and external effects.

Plugin storage is not an authoritative substitute for server material revisions/approvals. A frontend callback must not itself decide that a draft is semantically ready or run provider actions. New bridge paths alone would not authorize the backend behavior.

### 4. Minimum frontend-facing contract to freeze with the backend

These are proposed narrow contracts, not claims that endpoints already exist. Names should follow the implementation chosen by the owner. Prefer a separate `packages/core/materials/` domain with schema/queries/mutations, exported through the core package, while reusing project identity and access protection.

| Contract | Minimum data / behavior |
| --- | --- |
| Project material settings read/write | `workspace_id`, `project_id`, configuration revision, existing repository/connection identity, provider/instance, target branch, allowed stable root, integration capability/availability, configured reviewers, explicit `auto_start_plan`. Never return a credential. Save with expected revision and await confirmation before showing the new value as effective. |
| Material list/detail | Stable UUID/ID, linked task, project, kind, path, title, current candidate and formal baseline as distinct objects; independent publication, review, merge, plan-task states; actionable blockers; server-projected capabilities. Preserve unknown enum states as unavailable for action. |
| Candidate revision | Immutable revision ID/version, Markdown body, content digest, engineering-source references with full commits, readiness/blocking questions, previous revision reference; PR/MR identity and exact head only after observed publication. No client self-asserted approval. |
| Review action | Frozen candidate ID/digest/head and configuration revision; approve or request changes with reason; idempotency key. Actor comes from authenticated server identity. Return a saved decision separately from eventual merge outcome. |
| Review history / comments | Exact revision binding, immutable original decision, server actor and timestamp, validity/staleness, synchronized Git note status. Existing ordinary comments may remain discussion; a generic comment with no revision binding is not the required review record. |
| Verified baseline | Repository identity, actual merge commit, stable path, approved content digest and full fixed-version content, approval reference, actual merge actor/time; absent until verified. Squash merge SHA may differ from reviewed head. |
| Plan task | Stable task ID and baseline reference, `waiting_manual`/queued/running/blocked/awaiting-review distinction, authorized manual-start capability. Starting reuses this task. A task link does not prove execution. |
| Operation lookup | Durable operation identity and real phase, read-only GET recovery; pending/unknown outcome must survive reload. Retry retains original intent rather than generating a new revision or PR. |

Publishing should be driven by a server-accepted reviewable candidate, not a React effect watching draft changes. D2 does not require a “submit for review” button. The draft editor can save incomplete work without starting publication. The state labels must distinguish `approved` from `merged/verified` and from requirement acceptance/completion.

D8's form should state that disabling keeps plan tasks in To do without starting agents. Re-enabling should not silently start a backlog unless explicitly selected by the product owner; the planning documents recommend future stages only. Do not infer this policy from existing stage policy, normal task status or client-side disabled buttons.

### 5. UI, safety and compatibility details

- Present “正式需求 / Approved baseline” and “本次修订 / Candidate revision” separately, with source commits and a readable comparison. Use the existing sanitized Markdown renderer in `packages/ui/markdown/Markdown.tsx` (or safe plain-text versions initially), not raw HTML from a repository. The existing stage-proposal viewer renders plain text intentionally (`stage-proposals.tsx:21`); it is not a Markdown diff editor.
- Primary review wording should explain that approving authorizes the system to merge this requirement version when repository checks pass. No extra manual “merge” button is required by D7. Show waiting native checks/conflicts/permission failures as concrete blockers.
- Pass `wsId` and `projectId` explicitly to headless hooks. Keep server data in React Query, with keys under `projectKeys.detail(wsId, projectId)` where project access cleanup can remove it. `protectProjectRequest` fences authority/session changes (`packages/core/projects/access.ts:108`).
- Use `workspaceRequestInit({workspaceId,signal})` for every scoped request, including writes. Validate returned workspace/project/material identity after schema parsing; `client.ts:6556` is a useful list example.
- For new responses use `parseWithFallback` with a `null`/unavailable result and `redact:true` for private PRD text (`packages/core/api/schema.ts:13`). Do not parse malformed responses as an empty successful collection, approved state, default-enabled setting, or passed gate.
- Await review/start/configuration writes. Network uncertainty must not optimistically show a Git merge or an agent run. Persisted draft text is client state; editor identity should be keyed by account/workspace/project/material, not by a polling revision. Project cached-navigation tests demonstrate why (`packages/views/projects/components/project-detail.test.tsx:669`).
- The new component must handle load/error/denied/unsupported separately, long documents and paths, narrow layouts, labelled controls, pending actions, focus return and source changes during an open review dialog. Existing unknown-state and pinned-candidate patterns in `stage-panel.test.tsx` are directly relevant.
- Chinese product entity `issue` is **任务**, workspace is **工作空间**; a QA defect can be 问题/缺陷 as domain copy without renaming the product task (`apps/docs/content/docs/developers/conventions.zh.mdx:123`, `:178`). Candidate terms: 工程物料, 需求草稿, 待审核, 已退回, 已批准，等待合并, 正式需求已生效, 方案待办. Do not use “已完成” to mean “已合并”.
- Add matching English and Simplified Chinese keys. Reuse `projects.json` for project configuration and a focused material namespace if the feature grows; if adding a namespace, register it in `packages/views/locales/index.ts`. `locales/parity.test.ts:18` detects unregistered files as well as missing translations.

### 6. Test and file boundaries

Suggested production write scope, after backend contracts are frozen:

- `packages/core/materials/{schema,queries,mutations,index}.ts` and package exports, plus narrow API client methods.
- `packages/views/projects/components/project-material-settings.tsx` and shared material collection/detail components, plus the one existing mount in `project-detail.tsx` (or a separately agreed issue/collaboration mount).
- Matching locale files and contract tests. Platform files only if introducing a destination that the existing project route cannot express.
- If first-release plugin delivery is selected: `packages/plugin-sdk/index.ts`, host `surface-bridge.ts`, Action API implementation/scopes and concrete plugin surface. Keep these changes coordinated; do not add an unrestricted API proxy.

Verification ownership:

1. Core node tests own schemas/unknown enums/identity mismatches/default-vs-explicit-false/config CAS/recovery and state derivation. Reference `packages/core/collaboration/stage-schema.test.ts:1` and API response tests. Pure suites use `// @vitest-environment node`.
2. Shared DOM tests own saving settings, revision-pinned approval/return, post-response state, full PRD readability, comment/revision binding, plan waiting/manual start and key authority-loss regressions. Use QueryClient + mocked API/fetch + `NavigationProvider` + `renderWithI18n`; no Next or react-router mocks. `stage-panel.test.tsx:16` documents the canonical-layer split.
3. Add a `ProjectDetail` wiring test, not a duplicate full material behavior matrix. Existing test at `project-detail.test.tsx:479` checks panel placement.
4. Focused production-Web/Electron journey should cover both languages, a narrow viewport and both automatic/manual plan paths with controlled server/provider data. Do not claim real GitLab/GitHub integration from mocks.
5. Run affected package lint/typecheck/tests and locale parity, then repository-mandated broader checks. No tests were run by this research-only agent.

## Related specs

- `CLAUDE.md`: package boundaries, server/client state, compatibility, comments and testing ownership.
- `.trellis/spec/core/frontend/index.md`, `type-safety.md`: response normalization and active project/collaboration contracts; several generic entries are still placeholders.
- `.trellis/spec/views/frontend/component-guidelines.md`: shared settings/navigation, accessibility, project editor lifetime.
- `.trellis/spec/views/frontend/settings-recovery.md`: awaited writes, access-loss cleanup, credential handling.
- `.trellis/spec/server/collaboration-stage-approvals.md`: source authority, exact approvals, no implied dispatch.
- `.trellis/spec/plugin-sdk/frontend/index.md`: currently placeholder guidance; actual SDK/bridge/Action API code is more informative.
- Current task `prd.md`, `design.md`, `implement.md`: D1–D10 and PAC1–PAC15 remain the product source; older “planning only” statements need the main session's current-scope amendment before implementation.

## Caveats / Not Found

- No Git material frontend/API schema, project output binding, automatic-publication UI, approved PRD baseline or D8 switch was found in the inspected shared source.
- Existing collaboration stage functionality is substantial but is not a substitute for material publication, and its activation gate is explicitly separate. No test execution or deployed availability was inferred from source presence.
- No new dependency is required for the minimal shared panels. A sophisticated Markdown diff library is unnecessary for an initial full-content side-by-side comparison.
- Product decisions still required in the implementation artifact: first-release plugin surface vs host UI, reviewer roles, provider capabilities, re-enable backlog policy, and how the new verified baseline enters existing proposal/plan execution. QA ingestion/repair authority must stay separate from D7/D8 requirements flow.
