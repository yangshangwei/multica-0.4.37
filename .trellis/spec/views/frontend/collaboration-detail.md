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
- Its optional `returnRestorationKey?: string | null` consumes matching
  notification history/scope mementos without remounting a same-issue draft;
  undefined preserves normal fresh-link behavior. The complete page contract is
  [account notifications](../../core/frontend/collaboration-notifications.md).
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
Render requirements → delivered result/decision → discussion/progress. A compact
labelled icon toolbar below the workflow opens one right-side auxiliary drawer
for issue information, files, shared context, knowledge, children and activity.
Use the existing Dialog primitive with keepMounted, fixed header/category
navigation and independently scrolling content. It is bounded to 44rem on wide
windows and full width below the small breakpoint. Empty properties belong in
editing; no secondary disclosure stack remains below discussion. Long titles,
names, result/basis and translated controls wrap without horizontal overflow.

The top-aligned resource popup must declare `WebkitAppRegion: "no-drag"` on its
`DialogContent`, matching the collaboration creation popup. Its close control
overlaps the Electron account shell's 48px native drag bar; a portal or higher
z-index does not exclude it from native window drag hit-testing. Preserve the
real `DialogClose` wiring and retained lifetime. The component test pins the
no-drag boundary, but native acceptance must click the visible X with OS screen
coordinates and verify the full popup and backdrop disappear. Accessibility
activation, DOM `.click()`, Playwright clicks and renderer input injection do
not prove OS pointer delivery; Escape/backdrop success alone is insufficient.

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

Comment lists and reply hints share `collaborationCommentAuthor`. Named Agent
comments display the current name plus a textual localized Agent badge; missing,
empty or whitespace-only names display the generic Agent label once. Human names,
avatars and external badges retain their existing behavior. Long names wrap in
both locations; same-named humans remain distinguishable. Do not fetch Agent
details or consult the display summary when granting edit/execute/accept actions.
The public projection and fallback matrix live in
[collaboration content](../../server/collaboration-content.md).

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

### Comment threads, folding and row actions

`deriveCollaborationCommentThreads` (Core, pure) groups the loaded, deduplicated
comments once in `collaboration-detail.tsx`; the header count and the list both
consume that result. A thread is a root plus every loaded descendant, matching
the Server resolution scope. It is unresolved (`pending`) when no loaded member
carries `resolved_at` and at least one member is not deleted. Replies whose
parent is not loaded group under that missing parent id, start at depth 1 and
keep the "View parent comment" link; a loaded parent removes the link.

The list stays one flat `<ol>` of `<li key=commentId>`; thread structure is
`data-depth` (indent capped at 3), a left rule and an optional fold row
(`key=thread:<id>`, `data-thread-fold`). Never nest lists or re-parent rows: a
separately restored comment (`additionalIds`) must keep its component identity
and unsent edit when it later joins a loaded thread.

Folding hides rows with `hidden`; it never unmounts them. A thread that is
resolved when first shown starts folded; an unresolved one starts open. A thread
stays open when it was resolved on this page, when any member has a retained
comment-change or discussion request (the fold button is disabled meanwhile, and
the forced expansion is recorded so the settled result stays in view), or when
its rows were already shown open before an older page re-grouped them. A
`revealCommentId` + `revealKey` intent expands the containing thread during
render, before the parent anchor effect scrolls and focuses; an intent whose
comment is not loaded yet stays armed until it arrives.

Each row has one action line: Reply, reactions and, on the thread's single
resolve host (root when resolvable, else the first resolvable member, or the
member carrying the resolution), Resolve/Reopen. Edit, Delete, history, "Use in
result", per-row resolve and the read-only authority summary live in the row
menu, whose trigger is named `commentActions.more` ("Comment actions" /
"评论操作") — it must stay distinct from the workflow "More actions" trigger
because E2E disclosure helpers select that name page-wide. A deleted comment is
one line (author, deleted notice, time, menu) and keeps its reactions.

Hand focus off from the menu's `finalFocus`, not from the item's click handler:
Chromium keeps the popup mounted for its exit animation and re-focuses it when
the chosen item is removed, so a click-time `focus()` is undone and jsdom will
not show it. Inline forms take focus there; "Use in result" runs one macrotask
after the trigger has focus so the result dialog records the trigger as its
return target. Closing a form returns focus to the row's menu trigger.

The header shows the Server total (absent when not provided) and, when above
zero, the unresolved-thread count as a button that focuses the first unresolved
thread. While another page exists the count uses the "in loaded comments"
wording and the fold row omits its size; "Loaded x of y" renders only beside
Load more. Timestamps are relative within seven days, a localized date after,
with the ISO value in `dateTime` and the full localized time in `title`.
"Edited" (`updated_at` later than `created_at`, not deleted) opens the history.

### Auxiliary discovery, recovery and anchors

Keep all auxiliary panel instances mounted across drawer close and category
changes; hide/inert inactive content so uploads, pending operations and unsaved
memory/knowledge input survive without entering the keyboard/accessibility tree.
Key the entire drawer by Server/viewer/issue identity and unmount on denied access.

Pass the category's `active` state through memory history/version reads,
selected knowledge review and retained proposal source discovery. Inactive
optional observers are disabled, so interval/refocus/deadline/realtime cannot
keep rereading hidden history or candidate bodies. Keep required current-context
and source/target list root status reads for toolbar facts/recovery, and retain
all knowledge projection expiry/erasure. Reopen through current protected reads
without resetting drafts, reopening a dismissed version or refocusing a consumed
selection. Hiding one observer must not cancel another visible consumer's shared
Query.

`CollaborationPanelStatus` reports only ephemeral `{attention, count?}` to stable
callbacks. Errors and retained recovery requests appear in an actionable toolbar
status without opening a modal or taking focus. Attachment count requires a
successful current query; child count comes from the complete ordinary_children
projection. Unknown counts stay absent. Knowledge is paginated and pending
identities are account-wide: never label a page length or global request count as
this issue's total or pending count. Activity query errors also report attention.
Standalone panel disclosures retain their own ancestor-reveal effects.

File reference links open the drawer and become its return-focus target. Memory
and memoryVersion destinations route explicitly into the portaled drawer; its DOM
is outside the detail root. Repeated explicit navigation keys re-open the
requested view; scope resume and Back/Forward history traversal do not.
`CollaborationDetail` combines `navigation.isScopeResume` and
`navigation.isHistoryTraversal` for its normal scroll restoration, anchor
focus guards and the existing comment/run/submission/memory child props.
Both platform adapters deliberately mark history traversal separately from
scope resume; checking only the latter would replay an old URL anchor and
overwrite the saved memento. Notification returns retain their distinct
`returnRestorationKey` owner. Comment/run/description navigation closes the drawer and
nested previews before focusing the main target. Await modal close completion,
replace any queued callback with the latest navigation, and do not focus inert
background content. Panels with nested portals receive `active`; hidden panels
must not open previews or steal focus after async responses. A nested file preview
closes on deactivation; ordinary nested Escape closes only that preview.

Omit a large empty AI console only after supported execution and history discovery
prove there are no meaningful targets/runs/inputs, selected run, error or pending
intent. Existing execution facts skip redundant history discovery; record bodies
stay lazy. Empty current manual execution does not erase historical inputs. An
unavailable history read remains a discoverable factual error, not empty success.

Comment rows deduplicate by exact ID; the main progress stream must not repeat
the same comment. Repeated comment/run/memory navigation uses the navigation
intent key, opens enclosing disclosures, scrolls the real account-detail root and
focuses the target. Scope resume and history traversal restore tab scroll
without consuming a fresh anchor; Server/viewer/issue changes remount scoped forms. Navigation goes through
the shared adapter in both clients.

### Recovery visibility and intentional form focus

A missing initial memory response is not an empty document. Keep the named
`memory` region mounted through loading, unavailable and failed reads, with a
protected-query retry. Render retained memory-request recovery independently
of current memory content; denial still removes protected data. Report attention to the drawer toolbar; in standalone disclosure hosts, open all
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
Their optional `onApplied: () => void` callback reports a successful original
receipt, including successful original-result recovery, before dismissal. The
source bridge uses it for the success toast after its dialog unmounts. Do not
attach success feedback to `onDismiss`: cancellation also calls that callback.
Failures and unknown receipts never invoke `onApplied`. Keep tests for success,
cancel, refusal and later successful recovery in the source-bridge suite.

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

Workflow status-option failures offer `options.refetch()` through the existing
error component in the same form; show loading while the retry is pending and
keep submission gated by successful current status options. Read retry cannot
discard a typed reason/result or send an action POST. Context restore/purge
confirmation is locally bounded to the viewport, with a nonshrinking title and
footer and a scrolling description/reason/error area. Bound the growing reason
Textarea; even a legal 1,000-byte multiline reason must leave both cancel and
confirm reachable at narrow/short/enlarged-text sizes. Keep the original
snapshot/revision and permission guards when resizing the form.

## 4. Validation & Error Matrix

| Condition | UI result |
| --- | --- |
| Stale input during ordinary preflight | Keep draft; no publish |
| Explicit AI targeting | Show impact/target confirmation before send |
| Missing basis/result or revision reason | Local validation; no write |
| Exact submission/authority changed | Keep original/refused intent and show current revalidation path |
| Completed result without initiator decision | Factual unaccepted-record caption; no Accepted label |
| Current-only history page with another page | Keep More; no premature absence claim |
| Pending operation in hidden auxiliary panel | Toolbar attention opens the correct recovery panel; no automatic POST |
| Drawer close/category switch | Preserve editor/upload lifetime; hidden content inert |
| Main anchor with nested preview open | Close all auxiliary overlays, then focus main target |
| Empty manual current state with retained history/error | Keep AI entry/history/Stop/recovery accessible |
| Protected Query denial | Remove private content and use existing scoped recovery |
| Repeated explicit anchor versus scope resume/history traversal | Fresh target focus versus saved scroll, respectively |
| Thread already resolved when first shown | One fold row; expanding shows every member and Reopen |
| Thread resolved on this page | Stays open; feedback and Reopen remain visible |
| Retained comment/discussion request inside a resolved thread | Thread open, fold button disabled, recovery controls visible |
| Anchor to a comment inside a folded thread | Thread expands, then the existing scroll/focus runs |
| Unresolved count while another comment page exists | "In loaded comments" wording; never presented as the issue total |
| All comment pages loaded | No "Loaded x of y"; fold rows show their comment count |

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

`collaboration-detail.test.tsx` owns the workbench anchored-history regression:
with `isHistoryTraversal: true` and `isScopeResume: false`, call the existing
scroll restoration with the actual detail container and do not focus the old
description anchor. Keep the repeated fresh-anchor navigation regression too.
`e2e/global-scope-navigation.spec.ts` proves a saved 800px anchored workbench
returns at 800px on production Web and actual Electron, alongside unanchored
620px and query-error recovery. A trace that proves departure at 800px and a new
returning detail explicitly focuses the description at 325px diagnoses guard
replay; do not weaken the expected position or add a delay to hide it.

Views tests own actual happy-path wiring, accessible names/keyboard actions,
captured-preflight edits, distinct AI confirmation, result/basis/decision/history
presentation, participants, ancestor recovery, both locales and repeated anchors.
Use endpoint-routed mocks when history discovery adds an independent Query.
The workflow upload regression covers Cancel upload inside the modal, and the
unknown-result regression covers focus returning to recovery. Core owns pure parsing/action/page/recovery matrices; Server owns transactions
and admission. Do not multiply these matrices through DOM mounts.

The thread grouping/pending/resolve-host matrix lives in
`packages/core/collaboration/comment-threads.test.ts`. `comment-list.test.tsx`
keeps the wiring: one action line and menu contents, fold/expand/forced-open,
hide-not-unmount, reveal, restored-row identity, and the two focus regressions
that run under `closeMenusLikeChromium` (exit animation plus focus loss on
removal). A menu-item focus test without that helper proves nothing about
Chromium. E2E reaches row actions only through
`e2e/fixtures/collaboration-comment-menu.ts`; use `expectCommentMenuWithout`
for absence so the assertion first proves the menu is open, and check a comment
row before opening another modal menu on the page.

`e2e/collaboration-detail-journey.spec.ts` proves the full production Web/actual
Electron human chain, original GET with one POST, source and parent identity,
Chinese self-completion, long/narrow/enlarged-text states and repeated anchor/
scroll restoration. Screenshot capture must wait for fonts/frames/animations
**after** scrolling, and assert the intended heading/content lies within the
real viewport before compositor capture. A screenshot of the previous scroll
position is not delivery evidence. Persist the batched visual verdict and source
hashes. Installed Main, provider execution and native proof require separate tests.

## 7. Wrong vs Correct

Wrong: unmount an auxiliary panel when switching category, or leave its portaled
preview open after hiding the panel. Correct: retain the panel instance, gate
nested overlays with active, and expose recovery through an actionable toolbar.

Capture only after fonts, finite animations and scroll painting settle; verify
main geometry and the intended visible target, not screenshot dimensions alone.

Wrong: nest reply lists, unmount a folded thread, or fold a thread that still
holds a retained request. Correct: one flat keyed list, `hidden` rows, and a
forced-open thread until the request settles.

Wrong: duplicate Server stage logic in Views or automatically submit prepared
comment text as a result.

Correct: consume Core's facts/permissions and require explicit result/basis
confirmation through the existing workflow owner.
