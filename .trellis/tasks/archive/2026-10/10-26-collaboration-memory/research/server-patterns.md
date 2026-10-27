# Research: server patterns a collaboration `memory` resource must follow

- **Query**: access/terminal/R38, edit history + optimistic concurrency, idempotency, realtime, side-effect channels, migrations/sqlc, env config, run records, specs
- **Scope**: internal
- **Date**: 2026-10-26

Line numbers are a snapshot of this date. `git status` showed `server/` and `packages/` clean when read.

## 1. Access, R38, terminal state

- `CapabilitiesFor` (`server/internal/collaboration/access.go:79-110`): non-human principals get only `Reference` (85-87). Read goes to source members and participants (88). `ControlAI` goes to every participant with no status check (93). `Edit` is creator or handler (96-97). `Reopen` needs a terminal status and `closed` queue (108). `TerminalCategory` = done/cancelled (75-77).
- `ResolveIssueAccess` (`service/collaboration.go:65-112`) returns 404 `not_found` for non-humans (68-69) and when `!caps.Read` (104-106). The initiator is `CreatorType=="member" && CreatorID==viewer` (99). The returned access carries `Revision`, `LifecycleEpoch`, `AuthorityEpoch` and `EventSequence`.
- R38 lives in `service/admin_collaboration.go:142-255`, and only `complete|cancel|reopen` are allowed (87-98).
  - `authorize` (66-85) requires password mode, a JWT session and a platform admin in the same organization.
  - A creator who is not disabled gets 409 `creator_not_disabled` (167-170). "Disabled" means `disabled_at` or the denylist (`admin_collaboration_view.go:75-77`).
  - Reason: trimmed, at most 1000 bytes, NUL-free (`platform_admin.go:365-371`).
  - Two ledgers: `admin_operation` via `CreateOperationInTx` (171-176, `platform_admin.go:396-430`) and `collaboration_operation` with kind `admin_<action>` (207-210).
  - Audit: `recordRoleAudit` writes `admin_audit_event` with before/after (`platform_admin.go:338-344`), plus a `collaboration_event` of kind `admin_<code>` carrying reason, operation and before/after (409-439).
  - Handler: `handler/admin_collaboration.go:95-149` (UUID `Idempotency-Key` header, 104). Routes: `router.go:1743-1747`.
- Terminal rejection is 409 `issue_closed`. Human examples: Transfer (`collaboration_workflow.go:110-111`) and lifecycle (`collaboration_lifecycle.go:298-299, 318-319`). Runtime: `currentGroupAdmission` (`collaboration_runtime.go:347-348`), and settled/forced runs give `execution_stopped` (336-341).
- R41 metadata edits are deliberately allowed on closed issues (`collaboration_edit_test.go:520-532`).

## 2. Edit history and optimistic concurrency

- R41 handler (`handler/collaboration_edit.go:29-122`):
  - The body embeds `expected_collaboration_revision`, `expected_lifecycle_epoch`, `expected_participant_epoch` and `expected_account_epoch`, all decimal strings. The key comes from the `Idempotency-Key` header (`handler/collaboration_workflow.go:105-134`).
  - The parser is `collaborationPositiveVersion` (61-72).
- R41 service: `EditIssue` (`service/collaboration_edit.go:34-156`) runs through `mutateCollaborationWorkflow` (`collaboration_workflow.go:371-421`).
  - Under locks it returns 409 `stale_authority` (551-552) or 409 `stale_revision` (553-554).
  - On success it advances `issue_collaboration.revision` (`AdvanceCollaborationEditRevision`, `collaboration_edit.sql:18-20`; column at `571_collaboration_schema.up.sql:4`) and writes `collaboration_event` `issue_edited` with full before/after (139-150).
  - Serialization errors 40001/40P01/55P03 are retried 3 times, then 503 `collaboration_busy` (406-420).
- R43 comments (`service/collaboration_comments_edit.go:18-121`):
  - The request takes `expected_comment_revision` (`handler/collaboration_comments.go:145,171`). `comment.revision` was added in migration 351.
  - Stale revision gives 409 `stale_comment_revision` (38). Content is capped at `64<<10` (44).
  - The SQL guards `revision = expected` (`collaboration_comments.sql:29-44`). Delete sets `content=''` and `deleted_at`.
  - History is kept as `collaboration_event` `comment_*` rows whose payload holds before/after text (123-141, `collaboration_comments_view.go:186-242`, `ListCollaborationCommentHistory` at `collaboration_comments.sql:52-57`).
- Version helpers:
  - TS: `positiveVersion`/`nonnegativeVersion` in `packages/core/collaboration/version.ts` (signed-int64 decimal strings).
  - Go: `collaborationPositiveVersion` (handler), `collaborationRuntimeVersion` (`collaboration_runtime_capabilities.go:28`), and `json:",string"` tags on int64 fields.

## 3. Idempotent writes

- Table `collaboration_operation` (`571:123-136`): `scope_kind IN ('create','issue')`, `request_id` 1-128 characters. It has a unique index on (scope_kind, scope_id, actor_user_id, request_id) (migration 598).
- Queries: `CreateCollaborationOperation ... ON CONFLICT DO NOTHING` (`collaboration.sql:32-36`) and `CompleteCollaborationOperation` (195-200).
- Replay detection (`collaboration_workflow.go:518-532`): the stored row's `kind` and `payload_hash` must match, where the hash is sha256 of `json.Marshal(params)` (383-388). A mismatch gives 409 `idempotency_conflict`. An incomplete row gives 409 `operation_incomplete` (590-602). Refusals are sealed with the operation (562-568). The content variant is at `collaboration_comments.go:257-272`.
- Recovery GETs use a kind allowlist: `GetWorkflowOperation` (`collaboration_workflow.go:607-653`, list at 639-644) and comments (`collaboration_comments_edit.go:153-187`). Routes: `router.go:1665`, `1696`, `1680`, `1691`, `1693`.
- Runtime (agent) writes are idempotent on `collaboration_run_event (run_id, producer_event_id)` (migration 596).
  - `CreateRuntimeComment` compares the saved input and returns 409 `comment_conflict` on mismatch (`collaboration_squad.go:219-230`).
  - The CLI requires `--request-id` and sends it as `comment_id`/`event_id` (`cmd/multica/cmd_collaboration.go:56-59, 90, 104`).

## 4. Realtime

- The outbox is `collaboration_event.outbox_state` (`571:145`). Every event takes a seq from `AdvanceCollaborationEventSequence` (`collaboration.sql:62-63`).
- `CollaborationRealtime.deliverNext` (`service/collaboration_realtime.go:94-161`) locks any pending kind (`collaboration_realtime.sql:3-12`) and publishes:
  - an empty `collaboration:workbench_changed` to participants and that seq's notification recipients (131-140), and to the source workspace (148);
  - `collaboration:changed` `{issue_id, event_seq, collaboration_revision, authority_epoch}` to the issue scope (57-62, 151);
  - `access_changed` only for participant_removed/left.
- Runtime facts use `recordCollaborationExecutionChange` with no actor (165-179).
- The Hub revalidates at final send (`realtime/collaboration.go:62-89`).
- Frontend (`packages/core/realtime/collaboration.ts:72-76`): a strict `changeSchema` (12) leads to `refreshIssue` (53-58), which invalidates `collaborationKeys.issue(scope,id)` = `["collaboration",apiScope,userId,"issue",id]` (`access.ts:19-24`) plus source views (`invalidation.ts:18-37`). The event union is in `types/events.ts:15-19`.

## 5. Side-effect channels

- Activity: `ListCollaborationActivityEvents` reads every `collaboration_event` kind (`collaboration_activity.sql:3-9`). Unknown kinds project as `"other"` (`collaboration_activity.go:70-79, 99`) and render "Activity recorded" (`views/locales/en/collaboration.json:487`, `activity.tsx:22-26`). Any new `collaboration_event` kind therefore appears in the activity feed and triggers realtime.
- Notifications are written only by explicit calls: `writeCollaborationCommentNotifications` and `notifyAddedCollaborationMentions` (`collaboration_notifications.go:55, 68`; called from `collaboration_edit.go:154` and `collaboration_comments_edit.go:117`), plus `collaboration_result_notifications.go`.
- Search: `buildSearchQuery` (`handler/issue.go:643`) matches only `issue` title/description and `comment.content` (around 707, 718). A new table is not picked up.
- Plugins: the bridge subscribes only to the issue/comment/task bus events (`plugin_event_bridge.go:34-58`). Collaboration issues are withheld (`plugin_event_dispatch.go:237-239, 266-272`). Collaboration writes do not use `events.Bus` (`collaboration_outbound_boundary_test.go:171`); the only Bus publish is `PublishCollaborationChildrenChanged` (`collaboration_children.go:17-33`).

## 6. Migrations and sqlc

- Latest migration: `738_collaboration_host_execution`, wrapped in `BEGIN/COMMIT` with no indexes.
- Recent table pattern:
  - 732 `collaboration_resource_offer`: `CREATE TABLE IF NOT EXISTS`, CHECKs, `version bigint`, no FK.
  - 733 `CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ... (id)`.
  - 734 `(issue_id, created_at DESC, id DESC)`.
  - The down files `RAISE EXCEPTION` to refuse rollback.
- Each concurrent index must be registered in `concurrentIndexCleanups` (`server/cmd/migrate/main.go:122`, entries 222-224). This is enforced by `TestEveryConcurrentUpBuildHasCleanup` (114-118).
- `server/sqlc.yaml`: queries `pkg/db/queries/collaboration_*.sql`, generated code in `pkg/db/generated` (pgx/v5, JSON tags).

## 7. Deployment config

- Fail-startup precedent: `parseLLMMaxRetries` (`cmd/server/main.go:152-182`) followed by `os.Exit(1)` (627-634). Its comment contrasts it with the warn-and-fallback helpers.
- Fallback examples: `envNonNegativeInt` (`router.go:471`), `envDurationPositive` (`autopilot_failure_monitor.go:389-400`) and `configuredAdminRetention`, which returns nil when invalid (`admin_observability.go:47-54`).
- `AdminReadConfig` struct is in `handler/admin_settings.go:10-20`, filled at `router.go:465-468`, and serialized as the `configuration` map (37-53). Client schema: `packages/core/admin/observability-schema.ts:61-65`.
- Documentation locations:
  - `.env.example` (178, 648-652)
  - `docker-compose.selfhost.yml`, which passes each variable explicitly (124, 202)
  - `apps/docs/.../environment-variables.mdx:237-239` and `.zh.mdx:222-224`
  - the generated `server/internal/docs/content/pages/environment-variables.json` (`pnpm generate:docs-bundle`)
  - `docs/offline-upgrade.zh-CN.md:36-46`

## 8. Run records

- `collaboration_run_event` (`571:112-121`) has unique indexes (run_id, producer_event_id) and (run_id, seq) (migrations 596/597). Seq is `max+1` (`collaboration_squad.sql:36-40`, `collaboration_runtime.sql:222-225`).
- Kinds: comment, squad_activity, outcome, resource_snapshot, preparation.
- Per-run columns: `collaboration_run` (`571:82-110`) plus `prompt_snapshot`, `preparation_snapshot` (737) and `containment` (738). The prompt is frozen at claim (`collaboration_runtime_capabilities.go:279-315`). Runtime capabilities are a versioned struct (`protocol/collaboration_preparation.go:54-65`) stored in `agent_runtime.collaboration_capabilities` (737).
- Projection: `RunRecords` (`service/collaboration_records.go:143-386`).
  - Metadata kinds come from `collaboration_records.sql:8-10`.
  - Item queries exclude resource_snapshot/preparation (23-30).
  - `collaborationRecordEventItems` (388-430) turns any unknown kind into `invalidCollaborationLedger`, which fails the whole page.
  - Output is gated on execution view version 5/6 (`collaboration_host_compatibility.go:24-37`).
- Core schema (`run-records-schema.ts`): items are a closed `discriminatedUnion` (13-21). The page is a non-strict `z.object` with a refine (62-94), parsed through `parseWithFallback` (99-103).

## 9. Specs

- `collaboration-foundation.md`:
  - no FKs (82-83)
  - workflow input/receipt (245-251)
  - metadata edit atomicity (277-284)
  - machine projection (121-129)
  - `bigint` SQL arithmetic (132-134)
  - AgentActor deny-only (104-108)
- `collaboration-content.md`: concurrent index registration and refusing rollback (28-30); replay semantics (67-72).
- `collaboration-records.md`: records private/no-store (114-117); frozen inputs (119-121); explicit item kinds (128-132); add an optional field rather than a new enum value (230-231).
- `collaboration-realtime.md`: content-free frames (37-45); runtime facts in the owning transaction (46-47); invalidate, never mirror (50-55).
- `collaboration-administration.md`: real admin actor (12-13); reason bounds (40-45); two ledgers and original-key GET (59-63).

## Caveats / Not Found

- R38 delegation exists only for lifecycle actions. There is no admin path for any other collaboration write.
- TanStack prefix invalidation of sub-keys is standard default behavior. I did not test it here.
- I did not read `collaboration_result_notifications.go` in detail. Mobile was not checked.
