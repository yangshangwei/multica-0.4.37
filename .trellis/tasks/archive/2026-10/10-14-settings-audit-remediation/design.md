# Technical design

## Boundaries

The failures live in view ownership of Query states, mutation settlement, field validation, responsive content sizing, and accessible metadata. Keep server state in TanStack Query and client form state local; do not introduce stores into views or a second request layer. PAT response parsing belongs in core API code. Desktop preference controls remain in the desktop renderer.

## Query recovery

Use the notification settings recovery contract: distinguish absence of data from an error with retained data, keep a previous error visible during Retry using query timestamps, and disable repeat Retry while fetching. Definitive 401/403 must not authorize cached management controls. Do not turn pending or failed membership into an empty member list for permission decisions.

A small settings-specific feedback component/helper is permitted because these callers share one recovery contract. The catalog lane owns it and publishes its interface before other lanes consume it. It may render loading/error/Retry and derive display state from an existing Query; it must not fetch, mirror server data, own navigation or create a generic form framework. Leave working notification code intact unless reuse is strictly behavior-preserving.

Status sections must not mount new catalog observers during a cold-error/loading cycle. Fix the owning settings flow; do not globally disable React Query retry-on-mount.

Definitive denial must survive a later transient failure and remount. Reuse the existing Gantt exact-Query eviction pattern through a small `packages/core/query-access.ts` QueryFunction wrapper: capture the Query before awaiting, clear its data/dataUpdatedAt on active 401/403, respect cancellation, and rethrow. The continuation review reproduced cancellation restoring TanStack's private rollback snapshot after `setState`; the implementation therefore uses public `setData(undefined, { manual: true, updatedAt: 0 })`, which also updates that snapshot without changing global retries. Lane A owns this wrapper/tests and its catalog/triage options; B owns memberListOptions, workspaceMcpServersOptions, invitations and share links in core/workspace/queries.ts; C owns the corresponding provider query modules, including the settings repository picker's GitHub pagination. Relative core imports avoid package export churn. This follows the existing access-loss contract, without a view-level permission store. The continuation's real-query regressions cover subordinate read denial and member-permission loss in the triage settings form.

## Writes and validation

- Await destructive/confirm actions before removing rows or clearing their target. Use a handler-level pending guard in addition to disabled buttons.
- Avatar callers must propagate failed persistence to the existing upload control.
- Invalid names retain local values and show associated errors and unsaved feedback; correcting the name saves the retained form. Do not add a new global draft subsystem.
- Lock timezone/GitHub preference controls while their shared write is pending, using current busy state/ref guards as needed. This is preferable to introducing an unneeded parallel-write merge protocol.
- Token acknowledgement governs every dismissal path. Token revocation errors retain the confirmation target.
- PAT parsing uses existing zod/parseWithFallback patterns with redacted diagnostics. Reject unusable credential responses explicitly; no automatic create retry after an ambiguous result.
- PAT create/revoke hooks live alongside the account-scoped list options in core auth. Create delivers the one-time credential through a view-local callback and returns void; neither QueryCache nor MutationCache receives the secret as data or variables. Both write hooks explicitly disable retries.

## Layout and accessibility

Use content-container breakpoints for fixed-column catalogs, retaining the existing compact stacked rows and wide presentation. Bound only the affected property dialog, with scrollable content and reachable actions; avoid global Dialog changes. Use stable label/control associations, current-value descriptions for shortcuts, semantic warning/success foregrounds and a stable updater live region.

## Ownership

| Lane | Owned implementation files / concerns |
| --- | --- |
| A — catalogs | labels, properties, quick-actions, issue-statuses, keyboard-shortcuts, triage settings; shared settings recovery helper; colocated tests. |
| B — account/workspace | account, preferences, workspace, members, repositories, use-auto-save if necessary; directly required draft/save helpers; colocated tests. |
| C — providers/MCP | integrations catalog, GitHub, VCS, Slack/Lark/DingTalk/WeCom/Telegram/Composio provider settings, MCP; colocated tests. Gated plugin/billing delivery remains deferred. |
| D — token/desktop | TokensTab and PAT schemas/client methods/core auth query and mutation hooks/tests; daemon, behavior and updater accessible semantics/tests. Runs when a slot frees. |
| Coordinator | Planning, locale integration, source snapshots, verification/evidence scripts, spec updates, final review and completion records. |

Workers do not edit shared locale JSON files directly. Each emits a task-local `research/locale-<lane>.json` mapping locale file paths to nested additions; the coordinator merges them sequentially and promptly. Existing locale keys should be reused when precise. No broad JSON reformatting.

The API client is concurrently modified outside this task. Lane D owns only PAT imports/method hunks and any new PAT schema file; retain all other content. No worker may reset, checkout, stash, commit or revert another lane's changes.

## Validation / rollback

Add behavioral regressions first in the owning package. Verify pure boundary matrices once in core, real component wiring/AX in views, and desktop-only wiring in desktop. Use native screenshots/geometry for CSS; do not write tests that merely assert class strings. Reuse the isolated audit harness and fix its instrumentation only in evidence files. Each lane reports changed paths and commands; final integration verifies the combined state. Roll back only a task-owned hunk if required, never the dirty checkout.
