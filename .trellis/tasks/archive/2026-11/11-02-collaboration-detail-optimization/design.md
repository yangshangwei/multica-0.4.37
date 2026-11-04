# Collaboration Detail Technical Design

**Goal:** Persist truthful delivery/decisions and expose one clear human next step.
**Architecture:** Extend the locked collaboration workflow; project immutable
handoff/event facts through strict Core schemas; render shared Views at both
existing entry points. Query owns all server state.
**Tech Stack:** Go/Chi/pgx/sqlc/PostgreSQL, TypeScript/React/TanStack Query,
existing scoped client drafts and encrypted uploaded files, Base UI primitives,
Vitest and Playwright/Electron.

## Change boundary

The gap lives in the collaboration service, not only the detail component:
submission currently records optional prose; lifecycle has no exact acceptance
precondition or revision decision. The smallest complete solution adds explicit
delivery facts and decisions to existing events/handoffs, a task AI policy, and
the shared presentation/send wiring. No replacement task identity/store,
ordinary-issue workflow, platform router, runtime redesign or unrelated source
picker change is part of this design.

## Storage and identity

1. Reuse `issue_handoff` (`purpose=acceptance`, immutable note/actor/from/to/time)
   as the formal delivery identity. Note holds the result snapshot.
2. The exactly linked `acceptance_submitted` event stores a versioned `delivery`
   payload: basis, selected attachment IDs, observed lifecycle epoch and optional
   source-comment identity. Events already store exact handoff IDs; do not join
   by timestamp or parse conclusion-like comments.
3. Acceptance/revision/cancellation events store a versioned `delivery_decision`
   referencing the exact submission ID, actor/time from the event, decision kind,
   reason, and revision recipient when applicable. A revision uses the existing
   transfer handoff atomically and a distinct `acceptance_returned` business event.
4. Self completion inserts a result handoff record without updating responsibility
   or presenting a self-transfer, and writes result plus acceptance in the same
   lifecycle transaction. Its submission identity is equally stable.
5. Add `issue_collaboration.ai_policy TEXT NOT NULL DEFAULT 'assisted'`, constrained
   to manual/assisted, through one additive reversible migration. No new indexes
   or foreign keys. Preserve default behavior on upgrade and existing gates.
6. Evidence binds through `bindCollaborationAttachments`; newly delivered files
   receive the existing retained issue reference. Store exact file identities in
   the delivery event. Comment edit/delete cannot alter the handoff/event or
   remove the retained issue reference. Text-only submissions use an empty list.

Latest/current submission selection uses issue-scoped event sequence and exact
handoff/decision identities under the existing issue-collaboration lock. A closed
or reopened lifecycle supersedes its active acceptance; prior accepted/rejected
records remain in history. Do not infer decision facts from status. Validate
payload version, actor/handoff/issue identity, lifecycle and file list before
projecting evidence as available. Old rows without new payload have an explicit
legacy/unavailable evidence state; they cannot satisfy final acceptance.

## API contracts

Existing account APIs remain Auth-only, discard workspace headers, no-store,
strict JSON, current UUID/role/version checks and original Idempotency-Key.

| Endpoint | Contract change |
| --- | --- |
| `GET /api/collaboration/issues/{id}` | Add optional `ai_policy` and delivery summary containing current/latest submission and exact decision facts. Core treats absent fields as unknown. |
| `POST .../acceptance-submissions` | Retain existing note/status observations; require nonblank result (`note`) and `basis`; optional original `draft_id`, `attachment_ids`, `source_comment_id`. Existing handoff ownership and transfer remain atomic. |
| `GET .../acceptance-submissions?limit&cursor` | Authorized bounded delivery history with exact immutable submission/decision metadata, descending sequence and scope-bound cursor. |
| `POST .../acceptance-returns` | New `return_acceptance` operation: exact `submission_id`, nonblank note, explicit target user ID, directed in-progress status/version. Initiator-only; refuses changed/nonpending submission or unavailable recipient. |
| `POST .../lifecycle` | Complete requires exact `submission_id` for pending externally delivered work, or nonblank note/basis for self-handled work. Optional decision note; cancellation is distinct. Reopen retains history. |
| `GET .../status-options?action=...` | Add directed `return_acceptance`; remove review from generic changes/reopen choices. Preserve catalog digest/custom status semantics. |
| `POST .../ai-policy` | Initiator-only versioned original-key operation for manual/assisted; manual stops/pauses through existing controls; assisted never auto-resumes. |
| Creation contract | Optional policy with assisted default, persisted atomically with creation; add the small corresponding shared creation input while preserving other pending creation changes. |

Result and basis are each nonblank and at most 64 KiB decoded UTF-8 (use existing
comment byte-count semantics); combined request limit must accommodate both plus
attachments/version metadata. Revision reason is nonblank and bounded. Do not
trim stored prose beyond nonblank validation. Duplicate/malformed file IDs are
refused by existing attachment validators.

Errors are fixed nonprivate codes: result/basis required, delivery required,
submission changed/not pending, revision reason required, acceptance pending,
task manual, unavailable recipient and ordinary stale authority/catalog errors.
Every known workflow refusal remains an original-key receipt. Core must settle
the explicitly recognized, validated refusal set, preserve its unsent form, and retry only unchanged
original input when outcome is unknown. A changed baseline is a separate explicit
action. Receipt/schema failures never imply success.

New result success/own recovery also confirms the narrow tuple
`delivery_confirmation: { submission_id, attachment_ids }`. It contains no result
text, file metadata, signed URLs or participant/source data. Verify selected file
identity before retiring an upload bundle, including after unmount; a missing or
mismatched confirmation leaves the original intent uncertain. Own recovery may
return this original-write tuple after access loss, while protected issue/result
reads remain denied. Preserve legacy minimal fields for older consumers.

## Completion, revision and status

- Complete/cancel/reopen and policy changes keep initiator-only identity checks
  independently of current stage so successful keys replay after transitions.
- These checks govern ordinary collaboration operations. The user confirmed
  retaining R38's scoped disabled-creator administrative completion exception;
  preserve its current authorization, audit and recovery. It never fabricates
  a `delivery_decision: accepted` for the initiator.
- Acceptance additionally requires review, current handler=initiator, no open
  information request, exact pending submission and complete result/basis facts.
- An initiator who currently handles their own work and has no pending submission
  or information request may supply result/basis and complete directly.
- Pending submission keeps its identity while the initiator requests information;
  the existing return restores its recorded review stage before acceptance.
- Return for revision defaults to the actual submission author but carries an
  explicit eligible recipient, allowing a deliberate substitute if the author
  is unavailable. It records decision/reason/transfer/status together.
- Generic changes never enter review/done. A pending acceptance cannot be escaped
  by generic status or transfer; explicit revision, information workflow,
  cancellation and authority reconciliation retain their legitimate operations.
- Legacy review without linked delivery stays an ordinary factual review state,
  with repair/resubmission paths and no fabricated acceptance label.

## Manual policy and execution

Keep task policy, deployment admission, current grant, queue state and run proof
separate. `ResolveIssueAccess` suppresses new-command authority for manual tasks,
retaining read/comment/human controls and Stop/recovery. Reuse this central check
for target/resource/command writes and previews. Explicit preview targets report
the manual refusal; ordinary comments still publish.

The policy operation owns existing sorted account/source/session/channel/target
locks, updates policy, pauses the issue/targets and requests stop through existing
execution controls. Preserve pending inputs and native termination identities.
Runtime claim/launch/continuation admission must also inspect task policy so a
manual issue cannot launch through a stale queue observation. Changing to assisted
leaves lanes paused until explicit authorized Resume. Existing default-closed
host/model and knowledge-execution gates remain unchanged.

## Core and shared presentation

- Add strict optional delivery/policy schemas; bind issue/source/submission/file
  identities and reject malformed present evidence. Unknown server enum values
  retain truthful unavailable behavior. Keep private logging redacted.
- Add `return_acceptance` / `change_ai_policy` action variants and complete-result
  input; retain original draft IDs and correlation in saved action schemas.
- Reuse the existing scoped delivery form with a distinct `result` form kind and basis/
  source-comment fields. Result and ordinary comment forms must not share a
  mutable draft slot. Confirmed submission retires only its own exact result
  bundle; referencing a comment only prepares the result draft.
- Client form/operation records currently serialize JSON through the synchronous
  `StorageAdapter` to localStorage. This is existing recovery behavior, not client
  encryption. Server-side uploaded draft/file bytes retain their encryption.
- Derive human stage, primary/secondary actions and AI presence in pure Core
  helpers using both authority and factual submission/request state. Views do not
  implement a parallel workflow model.
- Header: title, factual business stage/current handler/deadline, one primary
  action, small secondary choices and More. Local management/source editing
  remains contextual; participants stay labelled and discoverable.
- Main: requested work/reference files, formal delivery with result/basis/author/
  time/evidence and exact decision, discussion/progress. Prior delivery history
  remains accessible and paged. Empty-result copy points to the authorized action.
- Wide: main plus compact information sidebar; narrow/intermediate: deliberate
  stack in DOM reading/focus order. Use semantic tokens/role type scale and
  existing breakpoints; long titles/results wrap, no fixed-height clipping.
- Source metadata shows selected meaningful values. Existing editors retain empty
  fields/catalog recovery. Participants and current permission context are
  secondary; comment headers emphasize authorship rather than current AI grants
  presented as historical execution.
- Keep detailed audit separate; main progress excludes duplicate comment events
  and cross-references only exact identities. Auxiliary working summary/knowledge/
  AI/children entries keep history, deep links, permissions and operation recovery.

## One-action ordinary comments

### Spatial thesis

Use one readable work column: requirements and reference material, then the
immutable current delivery and decision, then discussion. Title, current
responsibility and one primary action precede it. At sufficient content width,
selected source fields and labelled participants occupy a compact right column;
at narrower widths they stack after main content in DOM order. Files, children,
working summary, knowledge and audit use labelled disclosures. Workflow recovery
and unresolved outcomes remain visible. Preserve the existing Operate semantic
tokens and role-based typography.

Publish captures the current draft text/parent/files/suppression/execution input,
awaits the existing validated preview, verifies the captured intent is still
current, and sends only when no AI target requires confirmation. If AI targets
are reported, show impact preview and require an explicit confirmed send. Never
use a local mention heuristic as permission to skip preflight. Retain byte limits,
upload recovery, draft identity, epochs, request-key semantics and failure input.

## Verification and compatibility

Server tests own atomic workflow/permission/status/retention/legacy/policy matrices;
Core tests own strict parsing, stage/action derivation and immutable/recoverable
intent. Views own accessible wiring and the main journeys. Update actual callers
and relevant built-in skill contracts when the API behavior changes; do not add
compatibility bypasses allowing old clients to fabricate completion.

Use task-owned Go database and production Web/API; never the existing shared
development DB or unknown mismatched listener. Actual Desktop acceptance uses
the existing Electron fixture and a fresh isolated profile, built from current
shared Views. Record original commands, listener/source/build identities,
screenshots, receipts and cleanup. No real AI account/quota usage.

Rollback: additive policy column down migration must refuse/guard incompatible
manual rows rather than silently make them AI-enabled. Delivery events remain
retained; older clients can read old fields but cannot bypass server completion
requirements. Keep source-picker/locales baseline hunks and unrelated task files
outside owned commit selection.
