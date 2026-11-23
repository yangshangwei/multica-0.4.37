# ADR-05 live risk pagination: UI evidence

2026-10-05. Implements the approved C contract in `../10-05-projects-p1/research/pagination-adr.md`. Only the risk component, its project-scoped mount key, three localized messages and its canonical tests changed.

## RED preserved against A

Before editing product source, ran:

`pnpm --filter @multica/views test projects/components/project-risk-pagination.test.tsx`

Result: **5 cases, 4 failed / 1 passed**. Failures showed the missing sticky continuation disclosure, missing explicit fresh-from-start action (including the cached-first-page case and failed restart), and incorrect “No matching issues” for an empty suffix with positive total. The existing initial-card snapshot-change disclosure passed and remains protected.

The backend's original A first-page reset/starvation measurements remain unchanged in the verification lane's performance artifacts; this UI change does not reinterpret those results as passing C evidence.

## Implemented semantics

- Displayed pages replace one another; no accumulated rows imply one frozen snapshot.
- Every next request uses the cursor and snapshot version from the currently returned page.
- Once a continuation response has `refreshed=true`, its explanatory warning stays visible across subsequent `refreshed=false` pages.
- “Refresh from start” / “从头刷新” cancels a matching in-flight first-page request and calls `fetchQuery` with `staleTime:0`. A cached first page is insufficient. Only success replaces the displayed page and clears the sticky warning; failure preserves both the prior page and warning.
- Restart sends neither cursor nor old version, establishing a fresh live baseline. Low-ID newly matching or re-entered items can appear again.
- An empty suffix with positive total says there are no matches after the current position and offers restart. Total zero retains the normal no-matches wording.
- Initial card changes without a cursor retain the existing “counts and results refreshed together” notice, without the continuation or frozen-traversal implication.
- A different project remounts the risk scope, preventing a previous project's cursor or disclosure from being reused.

## GREEN and regression checks

- Canonical pagination file: **5/5 passed**, including two consecutive unchanged pages after a changed continuation; cached restart with a deferred network response; failed restart; low-ID re-entry; positive-total empty suffix; and initial-card semantics.
- Combined project regression command (pagination, management, review regressions, detail): **4 files / 37 tests passed**.
- `pnpm --filter @multica/views typecheck`: passed.
- Scoped ESLint for risk implementation and pagination tests: passed.
- `git diff --check`: passed.

Remaining: production Web/Electron verification and the parent's repeated C performance runs. No claim is made here about those unexecuted measurements.
