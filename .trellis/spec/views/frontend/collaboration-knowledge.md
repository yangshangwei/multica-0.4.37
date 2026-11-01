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
copied source bodies. Verification claims and test/commit references stay clearly
attributed data. Show expiry and applicability without implying server-tested
evidence or automatic runtime consumption.

### Live exact review and audit

Read candidate detail live from Core and render only its current authorized Query
value. Approval requires full available content, current server grant, and an
explicit review checkbox. Key decision controls by candidate ID, state revision,
availability, version and hash so replacement/unavailable data resets confirmation.
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

## 4. Validation & error matrix

| Condition | UI behavior |
| --- | --- |
| Selected source project changed | Preserve text, clear effective target and request explicit selection |
| Review version/revision/hash/availability changed | Reset checkbox and decision controls |
| Source unavailable but pending rejection granted | Show safe audit/reject controls without material |
| Target-only restricted row | Withdrawal only; no hidden-source navigation |
| Unknown delivery or missing recovery result | Original intent stays unresolved; no replacement request |
| Known result but local cleanup failed | Show known outcome plus cleanup notice; no repeat POST |
| Lease/hint/access clears current Query | Remove body immediately, including an open review |

## 5. Good / base / bad cases

- Good: a nonmember current handler selects a comment/current project, then an
  authorized human reviews and approves the exact displayed version.
- Base: unsupported backend leaves the existing collaboration page usable.
- Bad: precheck approval, copy an approved body into a correction draft, silently
  follow the latest source project, or retain a cleared body in component state.

## 6. Tests required

`knowledge-panel.test.tsx` owns source/target wiring, keyboard confirmation,
original-intent recovery, changed-review reset, safe audit facts, independent
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
