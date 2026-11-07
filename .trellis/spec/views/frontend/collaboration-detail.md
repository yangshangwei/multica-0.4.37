# Shared collaboration detail and next action

## 1. Scope / Trigger

Applies to `packages/views/collaboration` on account and source-workspace entries
in Web/Desktop. Preserve the established UI tokens, vocabulary, platform
navigation and restricted rich-content/file rendering. Business state derives
from [Core](../../core/frontend/collaboration-workbench.md) and
[Server delivery](../../server/collaboration-delivery.md), without a Views store
or a second workflow model.

## 2. Signatures

- `CollaborationDetail({scope, destination})` composes the current authorized
  issue, workflow, result, discussion and auxiliary panels.
- `deriveCollaborationDetailState(issue, viewerId)` supplies stage,
  primary/secondary actions, current/latest submission, `canAccept` and
  `canSelfComplete`. Every rendered action also requires current capabilities.
- `CollaborationDeliveryPanel({scope, issue})` renders immutable result/basis,
  author/time, decision and protected evidence, with lazy bounded history.
- `CollaborationWorkflow` receives explicit `resultFromComment` preparation;
  `CollaborationCommentComposer` uses Core's validated preview/send correlation.
- `collaborationHasAIActivity` and optional command-history discovery determine
  `CollaborationExecutionPanel` presence; existing Stop/recovery controls remain.

## 3. Contracts

### Hierarchy and role-specific actions

Use the title, labelled status/current handler/due date and one primary action
before the main content. Keep up to two secondary actions; the remaining current
actions live in the portaled More menu. Participants shows a compact role summary;
full permission context and the lazy roster appear only when opened.
Render requirements → delivered result/decision → discussion/progress. Source metadata/fields and auxiliary records share full-width secondary disclosures
below the main journey; empty properties belong in editing. Do not squeeze their
editors into a permanent narrow sidebar. Long titles,
names, result/basis and translated controls wrap without horizontal overflow.

Preparing a result from a comment copies its current text/file identity into a
separate editable result draft only on an explicit user action. Require basis and
an explicit formal submit; no ordinary comment or preparation performs a handoff.
Exact acceptance pins the displayed current submission. Revision requires a
reason and explicit eligible recipient, defaulting to that submission's author.
The initiator's own completion requires result/basis in the same request.

Render legacy/unavailable data factually. Missing decision actor/reason/basis
does not become inferred acceptance. Completed status plus a result without a
linked decision says “No initiator acceptance is recorded for this result.” in
both locales, preserving the R38 management exception. History cannot say there
are no earlier results while `hasNextPage` is true.

### One-action ordinary comments and explicit AI confirmation

Comment tools (mentions and attachments) are explicit compact toggles. The
attachment picker stays mounted, and retained files/uploads/errors reveal it;
hiding optional tools must never reset the composer or change send semantics.

An ordinary Publish click obtains the validated preflight and sends once if the
captured draft UUID, text, parent, selected files, suppressions and execution
observations still match. Preflight failure or edits while awaiting it preserve
the input and do not publish. Explicit AI targeting opens the impact/target
confirmation instead of automatic send. Manual policy denies new AI admission
and still permits ordinary human discussion under current authority. A lost
write response retains its original intent and offers original-key recovery.

### Auxiliary discovery, recovery and anchors

Files, children, shared summary, knowledge and detailed audit are secondary
disclosures. Keep their permissions, editing/history and pending operations.
When an upload/file delivery, knowledge operation, pending memory request or
explicit anchor needs attention, open **every enclosing details element** that
would hide its controls. Opening only an inner panel leaves recovery unreachable.
Reuse local attention/anchor effects; do not add a store or automatically resend.

Omit a large empty AI console only after supported execution and history discovery
prove there are no meaningful targets/runs/inputs, selected run, error or pending
intent. Existing execution facts skip redundant history discovery; record bodies
stay lazy. Empty current manual execution does not erase historical inputs. An
unavailable history read remains a discoverable factual error, not empty success.

Comment rows deduplicate by exact ID; the main progress stream must not repeat
the same comment. Repeated comment/run/memory navigation uses the navigation
intent key, opens enclosing disclosures, scrolls the real account-detail root and
focuses the target. Scope resume restores tab scroll without consuming a fresh
anchor; Server/viewer/issue changes remount scoped forms. Navigation goes through
the shared adapter in both clients.

### Recovery visibility and intentional form focus

A missing initial memory response is not an empty document. Keep the named
`memory` region mounted through loading, unavailable and failed reads, with a
protected-query retry. Render retained memory-request recovery independently
of current memory content; denial still removes protected data. Open all
ancestor disclosures when a retained operation needs attention.

`CollaborationCommentAttachments` must distinguish no attachment references
(no query/UI) from pending/failed metadata and partially missing references.
Use protected query retry and a factual missing-file notice. Active previews
must derive from currently supplied metadata so removed files cannot retain a
stale open preview. Never use workspace attachment APIs as a fallback.

Workflow actions open a named, scroll-bounded shared Dialog without moving the
underlying detail. The form heading receives focus; closing returns to the actual
button or persistent More trigger. If the original control is disabled by an
unknown receipt, return to the visible original-result recovery control instead.
Mutation feedback and original-key recovery belong inside the open dialog, and
remain outside when it closes. Source-workspace action dialogs pass `embedded`
and forward `onDismiss` / `onBusyChange` so only one modal owns focus.

Active writes/uploads prevent dismissal. Keep the upload cancellation control
enabled: the form's mutation/receipt lock must be separate from the toolbar and
dismissal upload lock. Retained unknown operations are dismissible and recoverable.
Comment preparation still requires explicit submission; result drafts survive close.
Do not repeat this on a refetch or a keystroke, and do not automatically submit
prepared content. Associate byte-limit errors with each affected reason/result/
basis field; an unrelated field must not become invalid.

Workbench search/sort apply to loaded rows. Show loaded/total and match context,
retain explicit pagination, and let Clear search restore the list without
changing the selected issue. History/comment timestamps use `useLocale()`.

## 4. Validation & Error Matrix

| Condition | UI result |
| --- | --- |
| Stale input during ordinary preflight | Keep draft; no publish |
| Explicit AI targeting | Show impact/target confirmation before send |
| Missing basis/result or revision reason | Local validation; no write |
| Exact submission/authority changed | Keep original/refused intent and show current revalidation path |
| Completed result without initiator decision | Factual unaccepted-record caption; no Accepted label |
| Current-only history page with another page | Keep More; no premature absence claim |
| Pending operation under collapsed auxiliaries | Open all ancestors; expose original recovery without POST |
| Empty manual current state with retained history/error | Keep AI entry/history/Stop/recovery accessible |
| Protected Query denial | Remove private content and use existing scoped recovery |
| Repeated explicit anchor versus scope resume | Fresh target focus versus saved scroll, respectively |

## 5. Good / Base / Bad Cases

- Good: a handler sees Submit result as primary, the initiator sees exact
  acceptance/revision, and an ordinary participant sees discussion; all use one
  shared issue identity.
- Base: encrypted evidence, knowledge source/approval scope, source picker and
  parent lifecycle keep their existing authority and recovery contracts.
- Bad: hide unresolved intent in a collapsed ancestor, show an empty manual
  console despite historical commands, publish changed text after stale preview,
  or infer acceptance from done status.

## 6. Tests Required

Views tests own actual happy-path wiring, accessible names/keyboard actions,
captured-preflight edits, distinct AI confirmation, result/basis/decision/history
presentation, participants, ancestor recovery, both locales and repeated anchors.
Use endpoint-routed mocks when history discovery adds an independent Query.
The workflow upload regression covers Cancel upload inside the modal, and the
unknown-result regression covers focus returning to recovery. Core owns pure parsing/action/page/recovery matrices; Server owns transactions
and admission. Do not multiply these matrices through DOM mounts.

`e2e/collaboration-detail-journey.spec.ts` proves the full production Web/actual
Electron human chain, original GET with one POST, source and parent identity,
Chinese self-completion, long/narrow/enlarged-text states and repeated anchor/
scroll restoration. Screenshot capture must wait for fonts/frames/animations
**after** scrolling, and assert the intended heading/content lies within the
real viewport before compositor capture. A screenshot of the previous scroll
position is not delivery evidence. Persist the batched visual verdict and source
hashes. Installed Main, provider execution and native proof require separate tests.

## 7. Wrong vs Correct

Wrong: open an upload form's inner details while its file auxiliary ancestor
remains closed, or assert only screenshot dimensions after requesting a scroll.

Correct: open every enclosing disclosure for recovery/anchors; capture after the
new scroll is painted and validate the intended target geometry.

Wrong: duplicate Server stage logic in Views or automatically submit prepared
comment text as a result.

Correct: consume Core's facts/permissions and require explicit result/basis
confirmation through the existing workflow owner.
