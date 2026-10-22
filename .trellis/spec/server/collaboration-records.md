# Collaboration children, recorded runs and result notifications

## 1. Scope / Trigger

Apply to ordinary children under a collaboration issue, source/account links,
source-owned run records and telemetry, and participant result notifications.
These contracts extend the existing workbench and execution protocol v5. General
provider adapters, new containment platforms and full-product rollout remain
separate work.

## 2. Signatures

- Issue detail optionally adds `ordinary_children: {items: [{id, identifier,
  title, can_open}], source_workspace_slug: string | null}`. Omission means older
  server support is unavailable; an explicit empty array means no ordinary children.
- `collaboration:children_changed` has exactly `{issue_id}` and no revision,
  content or authority epoch.
- `GET /api/collaboration/issues/{id}/runs/{runId}/records?cursor&limit` returns
  protocol 1, existing run identity/state, allowlisted resource provenance,
  recording coverage, nullable exact usage, discriminated items and `next_cursor`.
  Limit defaults to 50 and is 1–100; cursor is bounded to 1,024 characters.
- Runtime outcome keeps `{outcome, body}` and adds optional `record: {version: 1,
  completeness: "complete" | "partial", messages, usage?}`. The supported static
  runner, narrow HTTP capability and retained event use the same validation.
- Migration 729 adds nullable `collaboration_notification.run_id`. Public result
  notifications retain `source_kind: "issue", source_id: null` and add optional
  `run_id`; old installed clients can still open the issue.
- `accountPath({view: "participating", issueId, runId})` produces
  `/account?view=participating&issue=<UUID>#run-<UUID>`.

## 3. Contracts

### Ordinary children and old entries

Read the complete, number-ordered same-source formal ordinary-child set inside
the existing repeatable-read issue snapshot. Exclude independently shared
children. Current source membership permits opening a child and supplies the
current source slug; outside participants receive only id/identifier/title,
`can_open:false`, and a null slug. Do not render a hidden href, preview, prefetch
or source-directory query for that outside projection.

Create/attach/detach/rename/delete remain ordinary child operations with their
existing transaction and AI owners. Do not advance the parent's collaboration
revision, edit participants/handler, or give the child shared execution grants.
After commit, publish a content-free hint for each affected old/new shared
parent. Hints are transient; polling/focus/reconnect recover missed changes.
Receiving one invalidates reads without changing access epochs or clearing drafts.

Shared source/search/menu links use the account destination. Preserve only a
validated anchor for that same issue. Mixed collaboration batch actions reject
before ordinary AI confirmation or transport, including toolbar fallback paths;
ordinary issue behavior remains with its existing owner.

### Outbound plugin and channel boundaries

`PluginEventDispatcher.run` must classify the event's issue through
`IsCollaborationIssue` before installation lookup, callback-token minting or
HTTP. Withhold collaboration events and failed identity lookups. Actual
issue/comment/task producer shapes are part of this boundary; malformed
synthetic payload shapes require separate evidence rather than ordinary-event
assumptions. The local TLS transport regression proves withheld private body,
comment, file and record sentinels and a signed ordinary-child positive.

Collaboration human/runtime writes persist their private events/records without
ordinary task/chat/inbox events. A modern collaboration run has no ordinary
`agent_task_queue` task identity. IM consumers retain their ordinary immutable
`channel_task_delivery` and input-origin checks; knowing a shared run or child
ID and having a live chat binding does not grant delivery or private context.
The actual subscriber regressions deny missing/failed delivery before account,
credential, sender or storage lookup. Slack, Lark, DingTalk, Telegram and Wecom
retain ordinary-child transport positives; direct/stream/failure/file-only
selectors must verify their actual package paths independently.

Wecom `deliverRelayed(ctx, relayFrame)` is a separate delivery boundary from
its publisher. For `kind: reply`, revalidate the immutable delivery's task,
installation, channel, chat and chat type; task session and channel-ingested
input origin; and current active installation/workspace before text or media.
A live socket, shared run UUID, stale frame or private source parent is not
permission. `TaskID`, `SessionID` and `WorkspaceID` must be valid exact IDs.
Missing queries/rows, malformed or mismatched facts and inactive authority end
the reply without wire/storage. A temporary read error before wire releases
the claim through `outcomeProvablyNotSent` for the existing bounded original
event retry. Log only a fixed lookup stage, never the underlying private error.
Keep attempted-write/ack-timeout outcomes conservative and inbox pushes with
their existing separate delivery semantics.

`TestCollaborationOutboundWecomRelayReadFaultRetainsRetryWithoutPrivateWire`
fails each delivery/task/origin/installation query, asserts zero wire/storage
and private diagnostic leakage, recovers the same event once and deduplicates
its replay. Ordinary relay wire/Redis fixtures must retain actual workspace,
task/session, delivery and provenance; nil queries or an empty workspace do not
prove the sender positive. Real DB/media/socket regressions remain separate
from commercial IM endpoints or provider accounts.

### Existing IM cancellation and attachment-field exits

Lark/DingTalk/Telegram completion paths are text/card only; their query/client
interfaces have no outbound attachment reader or binary upload. Inbound media
capabilities do not imply an outbound file API. Run attachment-bearing/file-only
maps through their actual subscribers and assert no account/credential/wire
access on protected identities, with exact ordinary text positives. Do not
invent an absent binary-send implementation to fill a test matrix.

Lark cancellation removes only a previously tracked ordinary typing reaction.
Telegram cancellation clears its tracked stream and sends no replacement text.
Slack chat-done/failure/cancellation subscribers likewise remove only their
tracked ordinary reaction once. Untracked shared/private identities preserve
unrelated ordinary state and touch no installation, credentials or local wire.
Real loopback transport and private sentinel assertions are required; these
state-only exits have no private business-content lookup surface.

### Recorded data and paging

Authorize current trusted-human issue access on every page. After issue access
succeeds, a missing or mismatched run is `run_not_found`, not evidence of lost
parent access. Records are private, no-store and never fetched through generic
engine-task messages. Missing optional routes are unavailable, not empty history.

Inputs come from the run's frozen prompt, and comment text comes from its retained
event. Never rebuild an old run from the current batch or live issue description.
Resource identities and the one-time provider/model/account-source snapshot are
source-owned; engine/resource deletion must not erase them. Capture live runtime
data through `s.collaborationRuntimeLookup(q)` with the production collector and
the same transaction-bound query object. A nil-recorder wrapper is not equivalent.
Do not return installation secrets, raw runtime configuration, credentials or
private directories.

Items have stable IDs and explicit input/message/tool_call/tool_result/comment/
squad_activity/result shapes. Cursor fields bind issue, run, prompt hash and
sequence/within-event positions. Decode caller cursors into a fresh value and
require every field before comparing bindings; decoding `{}` into populated
trusted defaults must fail. Input/event paging cannot silently omit entries.

Core validates the assembled page chain before cache writes: reject cursor cycles
and conflicting copies of one immutable record. Preserve identical overlap and
consistent refreshed snapshots. Keep the read under the existing account/access
epoch guard, including error paths, and clear all private pages on real denial.
Validation errors remain generic and redacted.

Counters are nullable nonnegative signed-int64 decimal strings. USD cost is
recorded decimal text (1–19 integer digits, up to 10 fractional digits); do not
round through JavaScript Number or infer missing totals/costs. Missing telemetry
is not zero. Persisted `recording_incomplete` makes coverage partial even when
there is no typed record. Coverage, business outcome, forced disposition and
termination proof remain separate facts.

Typed recordings have at most 256 entries and 65,536 UTF-8 bytes per entry body.
The combined outcome/record JSON is at most 1 MiB; the runtime envelope is at
most 2 MiB. Unknown/duplicate fields and cross-kind tool fields refuse ingestion.
Recorded stdout and a prior squad activity may agree once; they cannot overwrite
an earlier or conflicting outcome. Invalid/missing recording or denied upload
must not prevent a legitimate content-free sealed proof.

### Results and navigation

Only a newly proven terminal execution-group transition emits `ai_completed`,
`ai_failed` or `ai_stopped`. An interim leader/member completion is not squad
completion; accepted Stop is not confirmed Stop. Forced disposition wins over
late success, and proof without a business outcome is failed/incomplete.

Freeze effective recipients in that same transaction using the existing unique
event/recipient notification identity. System results have no proxy human actor:
the input author and resource owner are not excluded merely by attribution.
Human action events retain true-operator exclusion. Delivery retries do not
recompute recipients or create ordinary inbox/subscription rows. Revoked reads
remove title and run/comment anchors and deny opening the issue.

Link to the final allocated leader/direct representative run, not whichever
member supplied the last proof. The view displays group state beside run outcome.
Source/native routes consume NavigationAdapter.hash, including MemoryRouter;
window.location.hash is not the native route authority. Hiding a currently linked
run clears only its matching validated fragment, allowing the same notification
to reopen and refocus it. Popover focus handoff must allow that navigation while
Escape still returns focus to the trigger. Known human/result events have clear
labels; removed notifications remain content-free and unknown kinds stay generic.

## 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Outside participant reads ordinary children | Minimal references, no source slug/link/catalog |
| Old server omits child or records support | Unavailable, not a false empty set |
| Source membership disappears but participation remains | Fresh outside projection; shared access survives |
| Child write or transient hint | No parent collaboration revision or execution side effect |
| Authorized issue with a wrong run | 404 `run_not_found`; preserve valid parent state |
| Missing/mismatched cursor field | 400 `invalid_records_query` |
| Paging cycle/conflicting record copy | Reject assembled pages before cache write |
| Old/partial telemetry | Nullable counters; partial coverage when incomplete is persisted |
| Outcome without proof | No terminal group result notification |
| Stop with a live descendant | No confirmed-stop notification |
| Replayed terminal transition | Original recipients/notification only |
| Reopen the same hidden run notification | Expand, scroll and focus again |

## 5. Good / Base / Bad Cases

- Good: an outside handler reads a retained run and its exact usage, sees a
  text-only ordinary-child reference, and opens the final continuation from a
  group-failure notification.
- Base: ordinary child changes, old body-only outcomes and old installed
  notification parsers keep their existing meaning.
- Bad: returning engine logs or live resource catalogs, treating missing usage
  as zero, or announcing whole-squad completion when one member exits.

## 6. Tests Required

Server owns current authority/machine boundaries, complete child sets, ordinary
transaction effects and hints, frozen inputs/resources, retained data after
cleanup, exact telemetry/limits, cursor required fields, result/proof ordering,
recipient/replay/rollback, and migration/RuntimeLookup conformance. Linux runner
tests use the test-created static executable and require actual containment;
host skips do not count as Linux evidence.

Core owns malformed/private wire, exact decimals, notification compatibility,
page-chain integrity, optional-resource versus issue denial and stale epochs.
Shared views own source/outside rendering, safe old actions, run/group facts,
controlled content, and composed repeat-notification/Popover focus behavior.
Isolated component tests must include NavigationProvider when rendering run
disclosures. Web/native tests own real routing, clipboard, issue-scope hints,
cross-account notification/paging, revocation and narrow/200% layout. Seeded
record-read journeys do not prove runtime ingestion; keep that evidence separate.

## 7. Wrong vs Correct

Wrong: `json.Unmarshal(cursorBytes, &prepopulatedTrustedCursor)`.
Correct: decode fresh, require the complete cursor shape, then compare bindings.

Wrong: `r.q.GetAgentRuntime(ctx, id)` or a wrapper with no production recorder.
Correct: `s.collaborationRuntimeLookup(r.q).Get(ctx, id)`.

Wrong: expose a new notification `source_kind: "run"` to installed enum parsers.
Correct: retain the issue source shape and add an optional validated `run_id`.
