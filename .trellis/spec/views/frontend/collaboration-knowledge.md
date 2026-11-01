# Shared knowledge proposal and review UI

## 1. Scope / trigger

Apply to collaboration source proposal/review and project/workspace managed
contribution panels shared by Web and Desktop. Use the
[Core lease/recovery contract](../../core/frontend/collaboration-knowledge.md)
and [server authority contract](../../server/collaboration-knowledge.md).
These views introduce no platform route, store or knowledge-generation model.

## 2. Signatures

- `CollaborationDetail` mounts `CollaborationKnowledgePanel` after shared context.
- `ProjectDetail` and workspace `WorkspaceTab` mount `ManagedKnowledgePanel`
  outside their human description/context editors.
- `KnowledgeProposal`, `KnowledgeSourcePicker`, `KnowledgeReview` and controls
  live in [`packages/views/collaboration`](../../../../packages/views/collaboration/).
  Core owns every network/schema/action; use `AppLink` and existing account source
  anchors. Views have no runtime environment or enablement switch.

## 3. Contracts

### Source selection and destination

Show proposal only for current `can_propose === true`. Default to the source
workspace and offer its already-visible current project directly. External
current handlers may choose those destinations without any workspace/project
catalog request. **Another visible destination** uses existing normally visible
workspace/project queries; it grants no new directory access.

Retain selected project ID, not a position or label. A changed source-project
relationship preserves human draft text and requires explicit reselection; never
retarget the draft silently. Correction starts blank with the exact predecessor
and fixed target. Workspace derivation from an approved project version starts a
separate blank proposal with `derived_from_candidate_id`; it does not withdraw or
copy the project contribution.

Memory, comment and run record are independent required-source choices. No shared
memory is needed for comment/run proposals. Preserve chosen historical version or
record ID as lists advance; call `buildKnowledgeSources` to resolve exact refs at
submit. Component state owns IDs, selections and human-authored fields, never
copied source bodies. That state sits above the Query gate: lease erasure or a
refetch hides server content but never clears the draft. At submit, if a refresh
erased the list, join that read (`refetch({ cancelRefetch: false })`, bounded by
one refresh interval) and prepare the intent only from a current
`can_propose === true` view. Report an unresolvable source, lost proposal
availability and invalid input as distinct errors, keeping the draft. Verification claims and test/commit references stay clearly
attributed data. Show expiry and applicability without implying server-tested
evidence or automatic runtime consumption.

### Live exact review and audit

Read candidate detail live from Core and render only its current authorized Query
value. Approval requires full available content, current server grant, and an
explicit review checkbox. Mount one decision instance keyed by the last displayed
tuple (candidate ID, state revision, availability, version, hash) and give it only
a candidate with that exact tuple. Lease erasure or an unchanged refetch hides the
controls without clearing the checkbox or reason. A different tuple, including a
changed availability, resets the checkbox, reason and error.
Await the exact receipt; a refusal cannot display success.

Show proposer identity, version, target, expiry, original approval actor/time and
latest decision actor/time. Reuse a lawful source creator/handler name or display
the user ID; do not add a directory lookup. Deduplicate identical approval/decision
facts. Render an available decision reason once as plain text. Source-unavailable
views retain permitted audit identity/time and possible rejection, with no quoted
reason or source material. Restricted target rows expose only control identity,
state and granted withdrawal: no source link, proposer/approval facts, rejection
or approval.

Source lists page every candidate version using the exact first-page boundary.
Detail history reports `history_truncated` and selects versions by candidate ID.
Available source links use existing memory-version/comment/run account anchors.
Malformed or unsupported reads show explicit scoped failure/unavailability rather
than an empty collection. Base editors never receive a merged knowledge string.

### Field limits

Summary, conclusion, evidence note and decision reason count UTF-8 bytes against
Core's `KNOWLEDGE_*_MAX_BYTES` with `collaborationTextByteLength`, never DOM
`maxLength` (UTF-16 units, silent paste truncation). Over a limit, keep the text,
mark the field `aria-invalid`, describe it with an inline used/max byte alert and
block submit before source resolution or any request. Approval sends no reason,
so an over-limit reason blocks only rejection and withdrawal. The sources plus
evidence count and the metadata budget have no field of their own; their schema
refusal shows the input message.

### Action outcomes

An uncertain in-memory intent locks its original fields and offers **Check
original result** / **Retry same request** with the same Core intent. On reload,
recover content-free stored identities by GET only. Do not reconstruct payloads,
automatically resend or mint replacement IDs.

A `KnowledgeUnusedIntentError` unlocks human editing/new intent creation;
`cleanupFailed` is a separate notice. A `KnowledgeReceiptCleanupError` applies its
known receipt, releases the completed intent and reports cleanup trouble without
another POST. Keep these feedback paths distinct from transport uncertainty and
known server refusal. Authorization expiry, purge and failed refresh remove body
through Core even while review is open.

The caller passes receipt ownership to the panel; never infer it from a render-time
intent closure. A receipt is owned when it matches the panel's in-memory intent,
or when a recovered stored identity names a candidate in the panel's current list
(decided before the request). Only an owned success changes the selection. An
unowned success leaves selection alone and tells the human to open the candidate
from its own knowledge list. Only a proposal receipt for the panel's in-memory
intent closes the proposal form; a decision receipt or a recovered stored
identity leaves an open draft unchanged.

## 4. Validation & error matrix

| Condition | UI behavior |
| --- | --- |
| Selected source project changed | Preserve text, clear effective target and request explicit selection |
| Source list refreshing at proposal submit | Wait for the current read; post only with current `can_propose`; keep the draft on failure |
| Field over its UTF-8 byte limit | Inline used/max error on that field; submit blocked; no source read or request |
| Local schema refusal after sources resolve | Input message, not source-unavailable |
| Review candidate/version/revision/hash/availability changed | Reset checkbox and decision controls |
| Lease erasure or unchanged refetch during review | Hide body and controls; restore the same checkbox/reason when the same tuple returns |
| Source unavailable but pending rejection granted | Show safe audit/reject controls without material |
| Target-only restricted row | Withdrawal only; no hidden-source navigation |
| Unknown delivery or missing recovery result | Original intent stays unresolved; no replacement request |
| Known result but local cleanup failed | Show known outcome plus cleanup notice; no repeat POST |
| Lease/hint/access clears current Query | Remove body immediately, including an open review |
| Decision or recovered older request settles while a draft is open | Keep the draft; select only an owned candidate, otherwise point to its own list |

## 5. Good / base / bad cases

- Good: a nonmember current handler selects a comment/current project, then an
  authorized human reviews and approves the exact displayed version.
- Base: unsupported backend leaves the existing collaboration page usable.
- Bad: precheck approval, copy an approved body into a correction draft, silently
  follow the latest source project, or retain a cleared body in component state.

## 6. Tests required

`knowledge-panel.test.tsx` owns source/target wiring, keyboard confirmation,
original-intent recovery, changed-review reset, unchanged-refresh retention of
decision input, draft isolation from decision and recovery receipts, recovery
ownership, submit during list refresh with staged error attribution, byte-limit
attribution for proposal fields and decision reason, safe audit facts, independent
source choices, external-handler catalog avoidance, correction/derivation and
restricted withdrawal. Existing page tests assert mount identities; locale
parity covers both dictionaries. Core owns exhaustive schema/hash/cache matrices.

`e2e/collaboration-knowledge.spec.ts` covers production Web and native Electron
journeys. Keep actual renderer/routing/IPC evidence, viewport geometry and source
hashes with each run; fixture records and fake execution are not proof of runtime
ingestion, packaged installation, real Provider behavior or all-platform release.

## 7. Wrong vs correct

Wrong: read a cached list summary, approve its latest ID and update the UI before
the server responds.

Correct: open current full detail, require review of its exact tuple, await the
matching receipt, and let current authorized Query reads determine visible state.
