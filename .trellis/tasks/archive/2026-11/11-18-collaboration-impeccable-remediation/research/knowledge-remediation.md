# C04/C08: knowledge input continuity and hidden reads

Research date: 2026-11-18. This is a planning artifact only. No implementation,
test, or specification files were changed by this research lane.

## Applicable contracts

- `.trellis/spec/core/frontend/collaboration-knowledge.md`: private projections
  live only in Query; request-start monotonic leases, erase-before-refresh,
  cancellation rollback erasure, exact reviewed tuples, and reference-counted
  lifecycle/realtime bindings remain authoritative.
- `.trellis/spec/views/frontend/collaboration-knowledge.md`: human-authored input
  survives an unchanged renewal; another tuple resets reason and confirmation;
  restricted/source-unavailable projections expose only current granted actions.
- `.trellis/spec/views/frontend/collaboration-detail.md`: auxiliary forms stay
  mounted across close/category changes; inactive content is hidden/inert; root
  counts, errors and original-request recovery remain discoverable.
- The task PRD forbids keeping an old candidate, quoted server reason, or cached
  capability in component state as an editor-preservation mechanism.

## Current paths and verified causes

| Path | Current behavior | Consequence |
| --- | --- | --- |
| `packages/views/collaboration/knowledge-review.tsx:30-34,85` | `decisionKey` retains the last displayed protocol/candidate/revision/availability/hash/version tuple while Query data is erased. | The decision component instance survives ordinary renewal, but its DOM currently does not. |
| `knowledge-review.tsx:93-118` | `KnowledgeDecision` returns `null` without a candidate; reason Textarea is inside the current-grant branch. | Every renewal removes/recreates the active editor and loses focus/selection, despite retaining its state value. |
| `packages/core/collaboration/knowledge-cache.ts:61-64` | A new root/detail read erases the old projection and Query rollback snapshot before fetching. | Retaining old server material to stabilize the editor would violate the lease contract. |
| `packages/core/collaboration/knowledge-queries.ts:20-24,74-76` | Candidate hooks renew at 15 seconds and enable solely for a nonempty viewer ID. | A mounted hidden review remains an active reader. |
| `packages/views/collaboration/detail-auxiliary.tsx:85,109-110` | Dialog uses `keepMounted`; memory receives `active`, knowledge does not. | Close/category changes preserve instances but do not stop optional reads. |
| `packages/views/collaboration/memory-panel.tsx:60-72` | Root read is enabled by read authority. Historical pages and selected version reads do not consult `active`. | Hidden historical pages keep renewing; selected immutable versions can reread on focus/reconnect. |
| `packages/views/collaboration/knowledge-source-picker.tsx:17-21` | Delivery candidate and source-history observers are enabled by selected source kind alone. | An open retained proposal can keep the same candidate/history key active even after the review's observer is disabled. |

The current source has no `hasOpened` gate. Actual installed Base UI 1.3.0 ESM
`dialog/portal/DialogPortal.js:29` renders when `mounted || keepMounted`. Do not
add a mount/unmount gate to solve reads; preserve the established lifetime.

## C04: minimal decision-editor change

Keep the existing exact-tuple key and local `reason`, `reviewed`, and field-error
state. Split client-owned draft rendering from current-projection action
rendering within `KnowledgeDecision`; no new store or candidate snapshot is
needed.

1. Remove the early whole-component `return null` for an erased candidate.
   Compute approve/reject/withdraw permission only from the current candidate,
   treating absence as false. All decision buttons and the approval confirmation
   remain behind current-projection/current-grant checks.
2. Give the reason Textarea a stable location/ref. During a transient renewal,
   keep that same element rendered if it contains client-authored text or is the
   user's current editing target. The latter can be tracked by a small local
   focus ref updated by `onFocus`/`onBlur`; it is client interaction state, not a
   retained capability. This also covers an empty but focused reason field.
   Keep the element branch/location stable; do not reparent it between a
   temporary editor and the normal form.
3. Do not set `disabled={!candidate}` or disable an enclosing fieldset for a
   renewal. A disabled input can blur even when its DOM identity survives.
   Client-only composition can continue while the protected body is absent;
   retain the existing `locked` rule for an actual pending/uncertain write.
4. Keep the loading/unavailable/private-body branches unchanged. No old summary,
   conclusion, audit reason, grant, or candidate object may be copied to local
   state. The existing UTF-8 field-limit feedback may still describe the human
   draft.
5. Add an explicit missing-candidate/inactive guard to decision submission;
   `prepareKnowledgeDecision` must receive only the current exact candidate.
   Keeping a draft editor visible grants no write ability.
6. On an explicit authority denial, suppress the draft/control presentation;
   do not treat denial as a normal renewal. Source/viewer/scope replacement
   retains the existing keyed parent lifetime, so another scope never inherits
   client input. An available `source_unavailable` or `restricted` candidate is
   a distinct current tuple, not necessarily a total denial: preserve its
   lawful content-free rejection/withdrawal behavior.

The task's "never cache capability" constraint rules out a local
`lastCanReject`/`lastCanWithdraw` cache. A focus ref plus the existing reason value
is enough to decide whether a client editor needs continuity during absence.

### Immediate candidate-switch fence

`KnowledgeCollection` currently reuses `KnowledgeReview` across `candidateId`
changes. During the new candidate's initial pending read, `identity` is null and
the previous `decisionKey` still exists. Once erased-candidate drafts remain
visible, this could display the previous candidate's draft under the new
selection.

Key `KnowledgeReview` by the selected candidate ID at its caller, in addition to
the existing scope-keyed collection and exact-tuple-keyed decision. This is a
small lifetime fence consistent with the existing requirement that candidate
changes reset input. It removes the old client editor immediately on selection
change, rather than waiting for the new response. Existing rapid-switch and
overlapping-query recovery tests must remain passing.

### Active propagation and focus

Pass `active` (default true for standalone/managed hosts) through:

`CollaborationDetailAuxiliary` -> `CollaborationKnowledgePanel` ->
`SourceKnowledge` -> `KnowledgeCollection` -> `KnowledgeReview`.

Guard review's selection-focus effect with active visibility. Track consumption
of the selected ID separately from activation: a new visible selection may
focus its review, while toggling `active` for the same selected ID must not steal
focus from the drawer heading/category button. A selection established by an
async receipt while hidden is consumed without focus; reopening is not a new
selection. Same-tuple renewal must never run a focus effect; preserving the same
editable element preserves the native selection directly.

## C08: gate optional observers, retain required roots

Keep the root memory observer and root source/target knowledge lists on their
existing schedules. Their toolbar error/status/count/recovery behavior is an
explicit preservation requirement. This task does not remove the nominal root
memory/list fanout of two normal reads per 15 seconds.

For optional reads:

- Add an optional `enabled = true` argument to
  `useCollaborationKnowledgeCandidate(scope, candidateId, enabled?)`. Its Query
  option becomes `enabled && scope.userId !== ""`. Keep
  `useKnowledgeLifecycle(scope)` and its bindings active while the component is
  mounted; disabling a reader must not disable erasure.
- `KnowledgeReview` passes its `active` to that hook. Managed knowledge defaults
  remain active; auxiliary knowledge derives it from drawer open/selected state.
- Memory history uses `active && !!memory && historyOpen && !denied`; immutable
  selected-version reads also use `active`. Preserve `historyOpen`,
  `selectedVersion`, editor text, original request identity, and existing explicit
  navigation-consumption rules.
- Thread `active` through `KnowledgeProposalForm` to `KnowledgeSourcePicker`.
  Gate its delivery-candidate, historical memory-page, comment-page and recorded
  run-page discovery observers by `active`. Leaving the delivery observer enabled
  can defeat the review gate when both observe the same key. Shared current
  memory/root reads are still kept by their required owners; do not infer a
  duplicate request count from multiple observers of the same key.
- Preserve explicit source-building/mutation/recovery operations that were
  already initiated by the user. Hiding a panel is not cancellation of the
  immutable original intent. An async completion while hidden can update its
  scoped client selection/feedback, but cannot focus or open a nested preview.

### Why interval suppression alone is insufficient

Installed TanStack Query 5.96.2 confirms:

- `src/queryObserver.ts:395-409`: disabled observers do not install a refetch
  interval.
- `src/query.ts:272-280`: a query with only disabled observers is inactive and
  disabled.
- `src/queryClient.ts:315-326`: refetch excludes disabled queries.
- `knowledge-cache.ts:39-47,71-76`: focus/online transitions and lease deadline
  erase first, then refetch active queries.

Setting only `refetchInterval: false` would leave the observer active, allowing
deadline/focus/reconnect/realtime to continue hidden GETs. `enabled: false`
stops those automatic reads while the independent lease timer still erases
private data. Re-enabling a stale-time-zero observer obtains a current read,
which uses the existing erase-before-read path.

### In-flight and shared-query boundaries

Disabling an observer does not cancel an already dispatched GET. Do not add an
unconditional `cancelQueries(candidateKey)` effect on hide: another visible host
can observe that same candidate and needs its read.

The minimal accepted boundary is: no new optional automatic reads while hidden;
an already-started read still obeys Core's epoch, cancellation signal and
monotonic deadline. If it becomes stale, existing erasure/cancellation must
prevent it from restoring private material. If it returns while still valid,
its hidden Query value remains bounded by the same expiry timer. Reopening
requires the current normal authorized read. No delayed response may restore
focus or a stale nested popup.

Do not add an extra cancellation helper unless the acceptance explicitly requires
aborting valid in-flight GETs on hide. Such a helper would need to inspect all
enabled observers for the exact key, preserve another visible owner, and use
non-reverting cancellation; it would broaden this task unnecessarily.

## Regression ownership and concrete checks

### Shared Views: `knowledge-panel.test.tsx`

- Strengthen the existing unchanged-renewal test and `refreshDetailHeld` helper
  at lines 216-241. Capture the real Textarea, focus it, set a nontrivial native
  selection, start a held read, and assert the same element/focus/selection and
  continued composition while private body and decision actions are absent.
  Resolve the same tuple and assert continuity again. The old assertion that
  the reason textbox disappears is the named pre-fix expectation to replace.
- Keep the existing state-revision and changed-version reset tests, plus current
  source-unavailable/restricted cases. A new candidate selection with a held
  response must not display the previous candidate's reason. The default
  managed-host path must remain functional.
- Verify an explicit denial exposes no protected body or decision action;
  viewer/scope remount removes the old draft. A transient failed renewal retains
  only human draft input and the existing manual retry, with no authorization.
- Use actual Core hooks with API spies and bounded fake time to open a review,
  hide it without unmounting, advance beyond 15 seconds and its lease deadline,
  and assert no new candidate GET while the root list still refreshes. Its old
  private Query projection must be erased at expiry. Reopen with a held GET and
  assert an authorized reread, unchanged human draft, and no unsolicited focus.
- Exercise the retained proposal's delivery-source/history observers too; a
  hidden second observer must not keep the selected detail key active.
- A real, already-initiated receipt that settles while hidden must not focus the
  new review, issue a hidden candidate read, lose the proposal draft, or create
  another POST. Existing receipt-ownership regressions remain canonical.
- Include one overlapping-host case: hiding one observer must not cancel or
  suspend another visible observer of the same candidate key.

### Shared Views: `memory-panel.test.tsx`

- Open historical pages, keep at least two pages, hide with `active=false`, and
  advance across the normal interval. Historical GET counts stay fixed while
  the required root current-memory read can still refresh. Reopen and confirm
  current authorized historical reads with retained client editor/history state.
- A selected immutable version is also disabled during hide; focus/reconnect
  must not fetch it until active again.
- Keep the existing fresh-anchor/scope-resume/dismissed-version regressions at
  lines 110-135 and 220-258. Network gating must not reset the consumed navigation
  key or reopen a dismissed preview. Existing access-revocation and stale-editor
  tests continue to own their behavior.

### Auxiliary wiring: `detail-auxiliary.test.tsx`

Its existing panel mocks can assert the passed `active` value for knowledge,
as they already do for memory. Keep the current keep-mounted, hidden/inert,
attention/count and return-focus cases. Do not use this mocked suite as proof
that actual leased queries stopped; that evidence belongs in the real panel
suites above.

### Canonical Core contracts to rerun

- `knowledge-cache.test.ts`: erase before refetch, late response, cancellation
  rollback, shorter expiry, viewer replacement, pagination boundary, lifecycle
  reference counting.
- `knowledge-lease.test.ts`: monotonic deadline/generation/replay boundary.
- `memory-queries.test.ts`: normal prefix invalidation, missing routes, and actual
  access loss clearing issue content.

Do not duplicate the whole Core matrix through DOM mounts. A focused UI hidden
deadline test proves the new enabled wiring; the existing Core suites retain
canonical private-data enforcement.

### Browser evidence

The real compiled-component Chromium probe should hold a candidate renewal
while the reason Textarea is focused, verify identical DOM identity plus native
selection, and type before the response resolves. Capture one coarse-pointer
or narrow auxiliary close/reopen case to confirm that background async reads
and selection changes do not steal focus. Fake API transport is sufficient for
these shared interactions; it is not production Web/native/physical-device
certification.

## Required specification updates after implementation

- Views knowledge spec: replace the broad "hide controls" statement during
  renewal with "hide private material and current-grant actions; preserve the
  client-owned active reason draft and focus/selection for the same tuple".
  Record the immediate candidate-selection key and denial/active gates.
- Core knowledge spec: document the optional candidate enabled argument and
  the distinction between reader activation and unconditional lease erasure.
- Views detail spec: record the optional-history/candidate active propagation,
  required root status exception, retained editor lifetime and no hidden focus.

## Rejected alternatives

| Alternative | Reason |
| --- | --- |
| Keep the old candidate/body/capability in local state | Violates Query ownership and can retain revoked private material. |
| Disable the reason editor during every renewal | Same DOM can still blur; it interrupts client composition. |
| Unmount auxiliary panels on close | Discards drafts/uploads/original-intent UI and contradicts established lifetime. |
| Only turn off `refetchInterval` | Deadline and focus/realtime active refetches remain. |
| Cancel a shared candidate key whenever one panel hides | Can cancel another visible owner and introduces unnecessary coordination. |
| Hoist editor state into a new global store or generic form layer | The existing keyed local state already owns this input; broader abstractions are unnecessary. |
