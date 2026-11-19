# State Management

> How state is managed in this project.

---

## Overview

<!--
Document your project's state management conventions here.

Questions to answer:
- What state management solution do you use?
- How is local vs global state decided?
- How do you handle server state?
- What are the patterns for derived state?
-->

(To be filled by the team)

---

## State Categories

<!-- Local state, global state, server state, URL state -->

(To be filled by the team)

---

## When to Use Global State

<!-- Criteria for promoting state to global -->

(To be filled by the team)

---

## Server State

<!-- How server data is cached and synchronized -->

(To be filled by the team)

---

## Common Mistakes

<!-- State management mistakes your team has made -->

(To be filled by the team)

### Administrative requests with uncertain outcomes

An unresolved control request is a client command draft, not cached server
state. Persist only its original idempotency key, non-secret payload and stable
server/actor/organization/target scope. Use an independent StorageAdapter key per
request; a shared JSON-array read/modify/write can lose another tab's command
even when both tabs successfully read back their own write. Adapters without key
enumeration fail closed before sending a command they cannot later recover.

Do not register these drafts as `workspaceScoped: false` in the workspace draft
cleanup registry. That registry removes global keys during *any* workspace
leave/delete. Administrative intents use explicit session-only cleanup instead.

On reload, query the original actor/key before enabling replacement. An empty
lookup alone never proves a delayed request cannot commit. Retire an old draft
only after a durable receipt, or an endpoint-specific rejection whose locked,
monotonic version contract makes that exact mutation impossible. Keep generic
conflicts/network failures on the original key. Server data and operation
receipts continue to use scoped React Query caches.

## Ordinary issue detail reads during optimistic updates

### 1. Scope / trigger

A predictable ordinary issue field update can overlap a detail GET started
before the write. Its old response must not replace the optimistic value while
the PUT is pending. This applies to the shared Core mutation used by Web and
Desktop; explicit bound completion keeps its separate authoritative flow.

### 2. Signatures

`useCanonicalIssue(wsId, routeId)` resolves either route form to the UUID.
`useUpdateIssue()` reads and patches `issueKeys.detail(wsId, id)` with that
resolved UUID. Preserve the currently loaded row across exact cancellation:

```ts
const detailKey = issueKeys.detail(wsId, id);
const prevDetail = qc.getQueryData<Issue>(detailKey);
qc.cancelQueries({ queryKey: detailKey, exact: true });
if (prevDetail && qc.getQueryData(detailKey) === undefined) {
  qc.setQueryData(detailKey, prevDetail);
}
```

Keep the existing synchronous `onMutate` sequence. Awaiting cancellation before
the optimistic patch yields to the event loop and changes drag/reset timing.

### 3. Contracts

The reader, mutation snapshot and optimistic patch share one QueryClient and
the same workspace/UUID detail key. Capture its currently loaded row before
canceling the exact in-flight query, then use the existing cache coordinator,
rollback and settlement invalidation. Cancellation prevents the old result from being
published; HTTP transport abortion is not required when the API promise does
not consume an AbortSignal.

An identifier can supply `initialData` after an empty UUID fetch has begun.
Query cancellation can then restore the empty pre-fetch state even though a
row was visible before cancellation. Restore that already loaded row only if
cancellation removed it; never manufacture a row when none existed. Keep the
default cancellation semantics: `{revert:false}` can dispatch a CancelledError
and change the query's read/error state.

Leave unrelated detail reads running. Route aliases are used for identity
resolution, not a second authoritative copy of mutable fields. Keep page-mount
access validation and bound-completion authorization/receipt handling intact.

### 4. Validation and error matrix

| Condition | Expected behavior |
| --- | --- |
| Old Todo detail result arrives during ordinary PUT | Pending optimistic In Progress remains visible |
| Identifier initialData arrives after an empty UUID fetch began | Exact cancellation retains the loaded row before the optimistic patch |
| Detail was empty before cancellation | Do not synthesize a detail snapshot |
| Ordinary PUT returns 503 | Existing rollback restores Todo and reports the failure |
| One explicit retry succeeds | Server read and refresh show In Progress |
| Another issue or workspace has a pending detail read | Its result can still publish normally |
| Bound completion is waiting or refused | No optimistic done projection; existing authorization governs |

### 5. Good / base / bad cases

Good: an identifier entry resolves to the UUID, an older UUID GET completes
during the status PUT, and the detail remains In Progress until settlement.
Base: an ordinary status update without an overlapping read keeps its existing
optimistic/rollback behavior. Bad: cancel every issue detail, dual-write mutable
status into an identifier cache, or render a separate local status to hide the
stale server cache.

### 6. Required tests

`packages/core/issues/mutations.test.tsx` owns the real QueryClient delayed-read
matrix for UUID and identifier entry, no-overlap controls, exact issue/workspace
isolation, late identifier initialData and bound-completion waiting/refusal. Mock only the API boundary;
release the old GET while the PUT is held, then assert rollback and explicit
retry. Start an empty UUID fetch before mounting the canonical identifier hook
for the late-seed case; assert immediate In Progress before releasing either
request. A passing standalone cache patch is not evidence that an overlapping
observer was protected.

`e2e/issue-optimistic-status.spec.ts` covers real detail buttons, closed options,
independent server reads, 503 feedback, one retry and refresh in both route
forms. It holds all naturally started UUID GETs after successful cold entry
and before the first PUT, with the canonical address committed. The same GET
endpoint also feeds `issueReactionsOptions`; holding only the first matching
request does not prove that the detail observer's response was controlled.
Keep request counts diagnostic, and never force a test-only fetch or infer
Query identity/cancellation from an endpoint match alone.
Keep the original `e2e/issue-mutation-branches.spec.ts` rollback assertions and
adjacent priority case.

### 7. Wrong versus correct

Wrong: leave the earlier GET active, or cancel it and assume the visible row
must still exist before the optimistic patch. Correct: preserve the loaded UUID
row across exact cancellation, apply the synchronous optimistic patch and keep
normal settlement and authorization behavior.

## Ordinary priority and assignee observations

### 1. Scope / trigger

Ordinary detail/menu, board, table and sub-issue field pickers must send the
revision of the Issue that supplied the visible UI. A newer cache entry cannot
silently authorize an older menu or confirmation intent.

### 2. Signatures

`withObservedIssueFieldRevision(issue, updates)` in `issues/mutations.ts` adds
the existing `UpdateIssueRequest.expected_revision?: number`. Shared Views
calls it before delegating to mutation/surface owners or `runConfirmIntent`.
The existing modal `issueRevision?: number` carries that same observation.

### 3. Contracts

Preserve an explicit token. Otherwise attach only a positive safe integer from
the supplied ordinary Issue, and only for priority or assignee fields. Do not
derive an ordinary token from collaboration authority or read QueryClient to
replace the observation. Keep Core transport, optimistic metadata exclusion,
rollback and settlement invalidation in their existing owners.

Both confirmation and suppress/handoff submission retain the captured token.
A batch may attach the existing common token only when every selected rendered
Issue has the same known positive revision. Missing/heterogeneous observations
retain the versionless API contract; do not invent a maximum revision or fan
out the batch. Status/completion/drag operations keep their separate contracts.

### 4. Validation and error matrix

| Condition | Expected behavior |
| --- | --- |
| Rendered R, newer cached R+1 | Send R; server may reject with `409 revision_conflict` |
| Explicit caller token | Preserve it through every delegation and modal |
| Missing/invalid revision or collaboration projection | Do not manufacture an ordinary token |
| Two genuine requests observed at R | Winner advances once; loser receives exact conflict and cannot overwrite its field |
| User reopens the picker after authoritative refresh | New explicit intent uses R+1; no automatic resubmission |
| Batch observations differ | Preserve existing compatibility and partial-result behavior |

### 5. Good / base / bad cases

Good: a captured menu retains R even when a detail cache already holds R+1.
Base: an older backend without revision keeps its supported versionless call.
Bad: fill the token inside `onMutate` from the highest cached revision, or renew
it when a confirmation modal submits.

### 6. Required tests

`issues/field-revision.test.ts` owns the pure boundary matrix; mutation tests
own actual QueryClient rollback, transport forwarding and fresh continuation.
Views tests own rendered observations, stale menu/cache differences, child
versus parent revision, modal retention and common-revision batch behavior.
`e2e/collaboration-recovery-ordinary-ui.spec.ts` verifies genuine Web/Desktop
picker PUTs, both commit orders, exact conflicts and explicit continuation;
its versionless API control is a separate denominator.

### 7. Wrong versus correct

Wrong: `expected_revision: findSourceIssue(cache, id).revision` at send time.
Correct: `withObservedIssueFieldRevision(renderedIssue, updates)` at the field
owner, with the supplied token preserved through confirmation and transport.
