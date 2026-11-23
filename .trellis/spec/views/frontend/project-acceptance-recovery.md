# Project acceptance description recovery

## 1. Scope / trigger

Apply when `ProjectUpdateComposer` previews or publishes a new acceptance and
the project description has changed. The draft must survive and the author must
explicitly review the current description before changing its expected revision.

Realtime project events may replace an active detail request more than once.
An imperative `fetchQuery` promise is not a durable subscription: the installed
Query implementation follows one cancelled fetch onto its replacement, but a
second replacement can reject that promise while the cache later receives data.

## 2. Signatures

- `POST /api/projects/{id}/updates/preview` and acceptance publication consume
  `ProjectUpdateDraft.expected_description_revision`.
- A stale description returns HTTP409 with `code: project_description_conflict`.
- `projectDetailOptions(wsId, projectId)` owns the protected detail read and
  `projectKeys.detail(wsId, projectId)` query identity.
- `ProjectUpdateComposer` in `packages/views/projects/components/project-progress.tsx`
  owns comparison, explicit adoption and read-error retry.

## 3. Contracts

- Enable the existing project-detail Query observer only while a description
  conflict needs review for `operation === "create"` and `kind === "acceptance"`.
  Do not copy a one-time read result into local server-state storage.
- Derive comparison content from that observer. Missing or unchanged description
  revisions must not expose adoption. Disable adoption while fetching, even if a
  newer cached revision exists; a failed read must not offer cached adoption.
- Retry retries the description read, preserving the draft and without repeating
  preview or publication. A successful read does not publish or silently advance
  `expected_description_revision`.
- Only the explicit adoption action updates the draft's expected revision and
  original-description baseline. A new preview remains required before publish.
- Keep `projectDetailOptions` and `useProjectAccessGuard` so session/access epochs,
  workspace denial, protected-query cleanup and draft erasure remain authoritative.
  Do not reinterpret access rejection as a transient conflict read failure.

## 4. Validation and error matrix

| Observation | Required behavior |
| --- | --- |
| Preview409 with retained body/scope/explanation | Preserve input and read current description |
| One or repeated realtime request replacements | Eventual current Query data supplies comparison |
| Cached description has the draft's expected revision | No adoption action |
| Cached newer description while refetch is pending | Adoption disabled |
| Detail read fails transiently | Visible error and read-only Retry; draft remains |
| Detail read succeeds after Retry | Current comparison returns; preview call count unchanged |
| User adopts current description | Update expected revision only through this action |
| Protected read rejects access | Hide protected UI and erase scoped drafts through existing guard |

## 5. Good / base / bad cases

- Good: two realtime invalidations replace conflict reads, the latest description
  appears, and a second preview sends the explicitly adopted revision with the
  original acceptance text.
- Base: an ordinary conflict/read/adopt/preview flow behaves identically.
- Bad: `.catch(() => undefined)` drops a cancelled one-time result, leaving the
  sidebar current but the composer permanently unable to review the description.

## 6. Required tests

`project-management.test.tsx` is the canonical mounted suite. Keep real
QueryClient behavior and controlled abortable reads; do not mock Query internals.
Assert repeated replacement recovery, retained draft text, explicit adoption and
the next preview's exact revision. Also assert read-error retry does not resend
preview, cached adoption is disabled during fetching, and denial erases drafts.

`e2e/projects-p1-progress.spec.ts` P1-P10 verifies the real API409, latest-description
comparison, explicit adoption, publication and persisted acceptance revision.
Run its full progress spec after rebuilding affected production clients. The
shared component applies to Web and Desktop; relevant native detail journeys
remain a separate browser gate.

## 7. Wrong vs correct

```tsx
// Wrong: later replacements can cancel this promise without ever updating local UI.
const current = await qc.fetchQuery(options).catch(() => undefined);
if (current) setDescriptionConflict(current);

// Correct: keep recovery attached to the protected query's current state.
const descriptionQuery = useQuery({
  ...projectDetailOptions(wsId, projectId),
  enabled: needsDescriptionReview,
  staleTime: 0,
});
// Derive comparison and guard explicit adoption from data/error/isFetching.
```
