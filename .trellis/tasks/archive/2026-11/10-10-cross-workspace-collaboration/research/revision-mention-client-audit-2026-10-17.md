# Research: Revised typed mentions and client protocol audit

- Query: Compare implemented comment/mention/notification paths and shared clients with revised AC90–AC97; identify safe reusable pieces and the next independent core protocol slice.
- Scope: internal; static source audit of core, shared views, Web/Desktop routing, collaboration HTTP/service boundaries and existing tests.
- Date: 2026-10-17
- Baseline: The existing task was explicitly resumed by the user. Historical planning/approval notes are not a new permission requirement. This report describes code inspected before the concurrent implementation lanes complete.

## Findings

### 1. The implemented foundation does not yet expose the revised product flow

The human workflow, field editing, account workbench query and durable operation machinery exist. There is no collaboration comment/preview/target/control/notification HTTP family registered in the inspected router. The shared collaboration page, account routes and multi-target client DTOs are absent. AC90–AC97 therefore remain unverified product requirements, not minor adjustments to an already delivered mention flow.

| Area | Existing evidence | Required adjustment |
| --- | --- | --- |
| HTTP registration | `server/cmd/server/router.go:1606` registers workbench; `:1609`–`:1622` register detail, creation receipt, recipients, participants, handoffs, field/status edits and lifecycle. Public creation remains explicitly unregistered at `:1607`. | Add dedicated collaboration endpoints after backend contracts exist; do not send collaboration comments through ordinary issue routes. |
| Core detail | `packages/core/collaboration/schema.ts:136` describes the issue; `:159` only has session ID, queue state and control version. | Add an explicit supported execution protocol and target/group control projection. The present shape cannot describe AC92/AC95. |
| Core HTTP | `packages/core/api/client.ts:899`–`:937` cover detail/creation receipt/recipient/workbench; `:940` builds versioned human operations. | New mention-target, preview, comment, target-consent, control and notification methods are required. |
| Internal queue | `server/internal/service/collaboration_queue.go:35` takes only `Content`; `:46` observes a single run/fence; `:60` returns a single command/run and session control state; `:97` allocates the session-wide sequence. | Bind commands to immutable typed targets, preserve one comment to multiple commands, and expose per-target outcomes. Do not reinterpret the old session queue as the multi-target protocol. |
| Shared UI | Searching `packages/views` finds no collaboration business page or consumer of the collaboration API; no `packages/views/collaboration/` files exist in the inspected tree. | Implement the account workbench/detail/composer in shared views and Query hooks in core. |
| Web/Desktop | No Web `/account` page was found. `apps/desktop/src/renderer/src/routes.tsx:93` explicitly assumes every tab is workspace scoped; `:114` mounts `:workspaceSlug`. | Add the persistent account destination and tab-group scope migration described in D10; a new overlay alone cannot satisfy the requirement. |

### 2. Ordinary mention dispatch is incompatible with AC92, despite useful UI components

The ordinary comment pipeline already supports typed mention syntax, mixed-target outcomes and preview. Its execution policy intentionally differs from the revised collaboration policy:

- `server/internal/handler/comment.go:2632` documents deduplication by executing agent. `:3024`–`:3041` coalesce `@agent A` with `@squad S` led by A, upgrading the single dispatch to the squad-leader role. Two squads sharing a leader also coalesce. AC92 requires separate typed roles/commands while sharing real capacity; do not reuse this resolver unchanged.
- The separate outcome list does deduplicate by `type:id` at `comment.go:3043`–`:3054`. That display identity pattern is reusable, but it does not make execution independent.
- `comment.go:2666`–`:2680` already prioritizes explicit agent/squad mentions over `@all` and member-only suppression. Preserve this mixed-mention ordering in a new resolver.
- `comment.go:2676` onward retains ordinary assignee/thread/conversation fallbacks. `:2990` explicitly allows ordinary mention execution on done/cancelled issues. Collaboration comments must not inherit either behavior: no explicit valid target means no root execution; terminal collaboration issues reject AI submission.
- Existing tests deliberately protect the old behavior: `server/internal/handler/comment_trigger_outcomes_test.go:138` (`TestCreateComment_AgentAndSameLeaderSquad`) and `:211` (`TestCreateComment_TwoSquadsSharingLeaderCoalescesNonWinner`). Keep those ordinary-task tests; add separate collaboration tests rather than changing the existing global rule.

### 3. Existing legacy fences must remain in place

`server/internal/handler/collaboration_boundary.go:61` rejects non-GET/HEAD legacy issue access, and `:88` rejects legacy collaboration writes. The ordinary preview enters through `loadIssueForUser` at `comment.go:1576`; ordinary comment edit/delete/resolve also have explicit collaboration rejection paths.

`server/internal/handler/collaboration_boundary_test.go:90` verifies ordinary create-comment, rerun, edit, delete and resolve return conflict; it also asserts no ordinary `agent_task_queue` row at `:112`. The dedicated new comment/preview route should authorize through the collaboration service and retain these exclusions. Merely allowing the existing comment endpoint would introduce implicit execution, wrong terminal-state behavior and agent/squad role coalescing.

The parent has assigned the wider backend foundation and legacy execution-fence audit/implementation to separate lanes. This bounded report does not claim every background writer or plugin listener has been audited.

### 4. Core compatibility is the safest next independent slice

`CollaborationIssueSchema` is a normal Zod object with no execution protocol discriminator. Extra future `protocol_version`, `targets` or `groups` fields are not validated by the current shape; a multi-target response can still look like a valid old single-session projection. There is no control UI using this projection yet, so this is a required gate before enabling new controls, not evidence of a presently exposed execution bypass.

Reuse these existing protections:

- `packages/core/api/client.ts:874` exposes endpoint/credential session scope; `:878`–`:895` reject late responses after endpoint/credential/auth changes and redact malformed JSON through unavailable results.
- `client.ts:899`–`:902` checks returned issue identity against the requested resource.
- `client.ts:940`–`:964` preserves the original `Idempotency-Key` and expected collaboration/lifecycle/participant/account versions.
- `packages/core/collaboration/schema.ts:309` and subsequent parsers use `parseWithFallback(..., { redact: true })`; invalid critical fields return null rather than usable defaults.

Recommended independent ownership: `packages/core/collaboration/schema.ts`, `packages/core/collaboration/index.ts`, `packages/core/collaboration/*schema.test.ts`, `packages/core/api/client.ts` and new/updated `packages/core/api/collaboration-*.test.ts`. Coordinate the exact wire names/version with the backend owner first; `api/client.ts` is a shared integration file and should have one writer.

The execution DTO should validate an explicit supported protocol version; issue/session identity; issue control and target-set versions; typed target identity and configuration epoch; target control versions; groups/runs and their same-issue/target relationships; per-target accepted/waiting/blocked results; explicit `scope=target|issue` control inputs. Versions remain decimal strings, as in the current collaboration schema. Unknown protocol/critical enum/malformed collection must be unavailable or explicitly read-only, never empty/idle or ordinary-executable. Optional presentation metadata may degrade independently.

Do not replace the existing generic human-operation response with a permissive union that loses per-target receipt information. New submission retry identity must cover one comment plus every target outcome and fixed human recipient set; retries must reuse the original payload/key, not newly fetched authority versions.

### 5. Preview and candidate reuse needs strict boundaries

| Reusable component/helper | Existing behavior and required adaptation |
| --- | --- |
| `packages/views/editor/extensions/mention-extension.ts`, `mention-view.tsx` | Existing typed mention rendering and round-trip components. Reuse the syntax/rendering after supplying issue-authorized data. |
| `packages/views/editor/extensions/mention-suggestion.tsx:70` | `MentionItem` supports member/agent/squad/all identities and presentation. Reuse rows/keyboard popup patterns, not its ambient candidate source. |
| `mention-suggestion.tsx:706`–`:718` | Candidate builder reads current workspace members, agents, squads and issue cache. This violates the collaboration catalogue boundary if wired directly. |
| `mention-suggestion.tsx:811`–`:815` | Existing `mode: context` is insufficient: nonempty search merges injected context candidates with the ordinary workspace catalogue. `MentionList` also invokes workspace issue/project search at `:305`, `:311`, `:325`. Add a dedicated restricted source/search mode that cannot expand beyond issue-authorized participants/targets. |
| `packages/core/issues/comment-trigger-outcomes.ts` | Existing typed mention parsing/label helpers can be reused as pure utilities, subject to collaboration-specific validation and target suppression identity. |
| `packages/views/issues/hooks/use-comment-trigger-preview.ts:95` | Existing Query/debounce pattern and stale-time-zero behavior are useful. The hook calls the ordinary endpoint, has workspace issue keys and hides loading/errors as empty at `:114`–`:122`; use account-scoped keys, abort/access epochs and explicit unavailable/loading state instead. |
| `packages/views/issues/components/comment-trigger-chips.tsx:35` | Existing popup/toggle presentation is useful. It suppresses by actual agent UUID (`suppressedAgentIds`, `onToggle(agentId)`), whereas collaboration needs typed target IDs so cancelling A does not silently cancel squad S. |
| `comment-trigger-chips.tsx:82`–`:88` | Presence is fetched through current workspace and is rendered as starts-now/starts-when-online. Collaboration must display server-derived waiting, paused, stop-unconfirmed and invalid-authority outcomes; online presence cannot prove permission or capacity. |
| `server/internal/handler/comment.go:1636`–`:1642` | Preview and submit share the same normalization (including NUL stripping), preventing text normalization from changing triggers only at submit time. Preserve this invariant in the new dedicated resolver. |

AC91 requires the preview to include human recipients as well as concrete typed AI targets, allow local suppression without deleting text, and produce no comment/notification/execution writes. Sending must re-resolve current authority and persist partial target results atomically: rejected A, valid B and mentioned H still mean one saved comment, one B command and one H notification.

### 6. Human notification storage is partly ready, delivery/mention semantics are not

- `server/internal/service/collaboration_workflow.go:318`–`:345` creates one collaboration event and then reference-only notifications to current recipients in the same transaction, excluding the actor. `:349`–`:352` gives a removed participant a content-free `removed` kind. This is reusable for recipient identity and transactional fanout.
- `server/pkg/db/queries/collaboration.sql:69` inserts notification references and an `is_mention` flag. `server/migrations/601_collaboration_notification_recipient_event.up.sql:1` uniquely constrains `(recipient_user_id, issue_id, event_seq)`, supporting one notification per event/recipient.
- `server/internal/service/collaboration_edit.go:122`–`:125` explicitly defers mention delivery: description/metadata edits currently create history only. AC97 requires a locked before/after effective-human-mention delta and notifications only to newly mentioned valid participants.
- The inspected collaboration router registers no account notification list/read/archive endpoints. No core collaboration notification schema or API methods were found. Stable storage alone does not provide offline notification visibility, comment/body anchoring, or independent read/archive behavior.
- Build new-comment fanout once per event, set mention presentation on the same row, and freeze recipients at commit. Do not emit both ordinary-comment and mention notifications for H, recalculate recipients during replay, notify the actor, or expand `@squad` into human membership.
- Notification consumer output needs an explicit anchor (`comment` with original UUID, or `description`/edit event) for `/account?...#comment-...` navigation. Current generic workflow notifications are not evidence that this anchoring exists.

### 7. Acceptance mapping and verification ownership

| Acceptance | Status from inspected code | Canonical next verification |
| --- | --- | --- |
| AC90 | No collaboration new-comment route or multi-target command binding. | DB-backed collaboration comment service: H+A+B+S yields one comment, independent targets, one H notification, unchanged handler/status/participants. |
| AC91 | Ordinary preview exists; collaboration preview/catalogue/partial atomic send absent. | Shared resolver tests plus one composer wiring test; mixed rejection, no preview side effects, same-key replay. |
| AC92 | Ordinary agent/squad execution coalescing conflicts with revised requirement. | New typed-target dedup tests; separate agent/squad roles, common resource capacity tested in backend scheduler. |
| AC93 | Legacy HTTP writes fenced; internal queue uses epochs, but no revised target resolver/route. | Dedicated collaboration request tests for permissions/closed/paused/unconfirmed/revoked states, keeping ordinary-route regression controls. |
| AC94 | Existing queue params/receipts still single-session/run. | Backend group scheduler and daemon integration owner; client only verifies truthful per-target state display. |
| AC95 | No scope-aware core DTO or UI. | Core protocol malformed-response matrix; backend all-or-none resume/replay; shared scope-control UI and Web/Desktop route tests. |
| AC96 | No issue-authorized target endpoint or restricted mention picker. | Backend target/grant owner matrix; restricted candidate source tests; no ambient catalogue expansion; invalid target history retains stop affordance. |
| AC97 | Recipient/event uniqueness and workflow fanout exist; comment/edit mentions and notification HTTP/client surfaces absent. | DB notification/mention-delta tests; core anchor/read/archive parsing; one account UI deep-link/relogin regression. |

Targeted existing regression commands (recommended, not executed by this research lane):

```sh
pnpm --filter @multica/core exec vitest run collaboration/schema.test.ts api/collaboration.test.ts issues/comment-trigger-outcomes.test.ts
pnpm --filter @multica/views exec vitest run editor/extensions/mention-extension.test.ts editor/extensions/mention-suggestion.test.tsx issues/hooks/use-comment-trigger-preview.test.ts issues/components/comment-trigger-chips.test.tsx
pnpm --filter @multica/core typecheck
pnpm --filter @multica/core lint
```

From `server/`, with the repository's isolated test database configured:

```sh
go test ./internal/handler -run 'Test(LegacyCollaborationCommentAndRerunPathsStayInert|CreateComment_AgentAndSameLeaderSquad|CreateComment_TwoSquadsSharingLeaderCoalescesNonWinner|PreviewCommentTriggers)' -count=1
go test ./internal/service -run 'TestCollaboration(Queue|Workflow|Edit)' -count=1
```

New contract tests should live beside the new collaboration helpers; keep pure malformed-response and target-identity matrices at core/service level rather than repeating them in DOM tests. Actual Web browser acceptance must use a production Next build/start target; Desktop account-tab behavior still needs native platform verification.

## Files Found

- `packages/core/collaboration/schema.ts` — current private detail/workbench/human-operation schemas and decimal-string versions.
- `packages/core/api/client.ts` — collaboration auth/server response fences, original-key operation transport and identity matching.
- `server/cmd/server/router.go` — current public collaboration endpoint registration.
- `server/internal/service/collaboration_queue.go` — existing internal single-session command/stop/resume contract.
- `server/internal/service/collaboration_workflow.go` — locked human operations and transactional notification fanout.
- `server/internal/service/collaboration_edit.go` — audited description/field changes with mention notifications deferred.
- `server/internal/handler/collaboration_boundary.go` — ordinary entry-point exclusions and reference-only policy.
- `server/internal/handler/comment.go` — ordinary mention parsing, preview, role coalescing and implicit execution policy.
- `packages/views/editor/extensions/mention-suggestion.tsx` — reusable typed rows and popup, but workspace-wide candidate/search behavior.
- `packages/views/issues/hooks/use-comment-trigger-preview.ts` — ordinary preview hook and empty-on-error behavior.
- `packages/views/issues/components/comment-trigger-chips.tsx` — actual-agent suppression and presence-based copy.
- `apps/desktop/src/renderer/src/routes.tsx` / `stores/tab-store.ts` — current workspace-only route/tab model.
- `server/migrations/601_collaboration_notification_recipient_event.up.sql` — recipient/event uniqueness.

## Related Specs and References

- `CLAUDE.md`: package boundaries, API compatibility/redaction, account/server state ownership, original request identity, testing and native desktop route rules.
- `.trellis/spec/server/collaboration-foundation.md:11`–`:15`: explicitly distinguishes revised multi-target planning from current foundation; do not enable ordinary dispatch or relax old indexes in place.
- `.trellis/spec/core/frontend/index.md`, `.trellis/spec/views/frontend/index.md`, `.trellis/spec/web/frontend/index.md`, `.trellis/spec/desktop/frontend/index.md`: package ownership and verification entry points.
- Task `design.md:188`–`:219`: authorized catalogue, preview/send atomicity, two-level control and explicit versioned client protocol.
- Task `design.md:227`–`:247`: account workbench, persistent Desktop destination, cache/permission epochs and unavailable behavior.
- Task `acceptance.md:102`–`:109`: AC90–AC97; task `prd.md` R09/R10/R13/R23/R24/R52/R55.
- External references: none; this audit depends on repository source and the revised task contracts, not a new SDK or external API.

## Caveats / Not Found

- No product code, specs, task metadata or git state were changed by this lane; only this research report was written.
- This is static inspection, not a passing test report. No test/build/browser/native-runtime execution was performed.
- Concurrent implementation can move line numbers or add the missing foundation. Reconcile with the backend lane's final DTO before writing core schemas; do not infer completed capability from this report's snapshot.
- The full daemon process-proof/capacity/delegation protocol, plugin egress and every background writer are owned by other lanes and were not re-audited here.
- Initial broad path lookup encountered a shell glob/missing router directory; verified the actual paths with `rg --files` and used `server/cmd/server/router.go`. This did not affect source files or the findings.
