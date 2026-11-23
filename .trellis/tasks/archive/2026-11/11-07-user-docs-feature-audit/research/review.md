# Independent documentation review

Reviewed on 2026-11-07 against the current working tree, including uncommitted product implementation. This is source/semantic review; no production deployment, browser acceptance or real-model run was performed.

## Findings fixed by this reviewer

### Collaboration creation refuses an unaccepted parent, not a whole workspace triage policy

- File: `apps/docs/content/docs/collaboration.zh.mdx`, creation prerequisites.
- The initial new wording said a source workspace could require every new collaboration to pass triage, and suggested switching the source workspace.
- The actual rejection is `admission.Blocked` for the selected parent task: `server/internal/service/issue.go:455`, mapped to `triage_review_required` in `collaboration_creation_result.go:82`. Creation itself has the deployment creation gate in `collaboration.go:260`.
- Fixed wording to distinguish a closed deployment creation gate from an unaccepted parent, and direct users to accept the parent before creating its collaboration child.

## Findings closed during integration

1. **Usage counting boundary.** `usage.zh.mdx` initially described task counts and runtime as all executions in the selected range. `server/internal/handler/dashboard.go:364` and `server/pkg/db/queries/task_usage.sql` show that time/task KPIs count completed/failed/cancelled runs only when both start and finish timestamps exist, using completion time. Errors count completed/failed terminal runs without a start-time requirement; cancelled runs do not enter that failure-rate denominator. Queued expiry can therefore appear in Errors but not the time/task KPIs. **Closed:** the author updated both metric descriptions and the error-population explanation; the reviewer checked the final text against the SQL predicates.
2. **Trigger coalescing.** `concepts.zh.mdx` still said every trigger creates a new execution. New squad and triggering guides correctly distinguish queued/coalesced/deferred. **Closed:** the reviewer changed both the execution definition and relationship bullet to creation or combination into a pending execution, preserving one business task versus many executions.
3. **Quick actions coverage.** Settings → 快捷操作 and task sidebar actions were omitted from the overview/guide. Verified source: `server/internal/handler/quick_action.go:423-480,522-568`, `packages/views/issues/components/quick-actions-section.tsx`, `packages/views/issues/hooks/use-quick-action-menu.ts`. Private actions belong to the human creator; shared actions require workspace owner/admin and an all-members-invocable target. Click posts a mention comment using existing dispatch; slash insertion permits editing before send. Prompts are literal, reject template tokens and extra side-effect mentions. An admission receipt is not completion. **Closed:** the reviewer added a bounded task-guide section with creation, direct execution, slash editing, permissions, literal prompts and outcome interpretation, and linked it from the feature overview.
4. **Actual chat entry label.** Sidebar label is 聊天 in `packages/views/locales/zh-Hans/layout.json:7`. Use it for navigation instructions. Chat still selects a single agent; squad metadata only shows leadership (`quick-create-actor-picker.tsx:46,94`), so do not describe team chat. **Closed:** the coordinator updated `chat.zh.mdx`; the reviewer corrected the feature overview navigation label. It remains individual-agent chat. The overview also now matches the task page’s assigned/created/participated “我的任务” scopes.
5. **Defender certainty.** The touched desktop guide retains an unconditional “这是误报” for Wacatac-style alerts. Source provenance/unsigned binaries alone cannot establish that every user's downloaded file is safe. Qualify as possible heuristic false positive, require trusted provenance and release checksum confirmation before restore/exclusion, and direct uncertain cases to the administrator. **Closed:** the reviewer removed the unconditional false-positive claim and blanket two-directory exclusions, kept files quarantined until the administrator verifies provenance/version checksums and approves handling, and documented minimum-scope exclusions only when necessary. This is a factual documentation correction, not a product change.

No unresolved findings remain from this bounded review. Final generation/build/integration checks remain coordinator-owned.

## Verified contracts and coverage

- All five new pages are registered in `meta.zh.json`; homepage and feature overview provide discoverable links. Existing product slug constants do not need new entries because no product deep link was added.
- Collaboration source ownership versus task participation, same-server directory, explicit resource-owner consent and resume, stop-request uncertainty, shared-lane control scope, and manual versus assisted policy match `collaboration/access.go`, collaboration services and current UI labels.
- Knowledge approval requires current source read plus target membership/role. `MULTICA_KNOWLEDGE_CONTEXT_ENABLED` defaults off and runtime/provider/profile restrictions remain explicit.
- MCP library configuration is write-only; per-agent MCP and audited environment-variable reads have different rules. Save/assign/enable are distinct, and offline program/service availability is stated separately.
- Approval authority and recording gates match `agent_approval.go`; approval is not a command executor, and reported executed state is not proof of successful release.
- Saved-view privacy, batch collaboration refusal, CSV current-query scope, property administration, source-context capture and account navigation were checked against the relevant handlers/components.
- Usage price split, local custom pricing, calendar windows and restricted-agent aggregate bucket were checked against dashboard handler/SQL, shared UI constants and custom pricing store.
- Planned controlled artifact execution is not presented as delivered.

## Verification performed by reviewer

- `git diff --check -- apps/docs/content/docs`: passed.
- `pnpm --filter @multica/docs typecheck`: passed (tsc exit 0). pnpm printed an existing configuration-location warning; no dependency/configuration changes were made.
- Follow-up checks: five edited/verified pages passed frontmatter, MDX-to-directive and TOC parsing; the new overview link resolves to the generated `快捷操作` heading. Actual chat label, qualified Defender text and usage-count conditions were asserted.
- Lint: docs package has no lint script; whitespace/static semantic review used here.
- Tests/build/bundle/link/Go verification: coordinator-owned; deliberately not duplicated by this reviewer.

## Limits

No claim is made that the screenshot's installed desktop/server contains the entire working-tree feature set. Source capabilities, enabled deployment capabilities and a real AI tool completing work are separate evidence. Older unchanged guides may retain legacy English product terms outside the revised paragraphs; command identifiers and stable slugs must remain unchanged. No new tests are required for these prose changes; reuse existing compilation, bundle, links and embedding checks.
