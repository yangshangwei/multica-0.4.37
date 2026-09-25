# Squad page implementation plan

Goal: apply the user-approved audit with a small shared web/desktop change.
Architecture: shared views + existing navigation and client preferences; no backend changes.
Tech stack: React, Base UI, TanStack Query, Zustand, Vitest, Playwright.

1. Add failing page regressions for distinct tabs, existing creation paths, native links with one navigation, accessible scope/filter controls, and failed-query recovery.
2. Independently update builtin-squad-catalog.tsx and its focused tests; localize shared labels and curated template use-case summaries in all four squads.json files.
3. Update squads-page.tsx to compose tabs, a creation menu, accessible list cells, bounded columns and discoverable controls. Change only the creator/date default columns in core.
4. Run focused views/core tests and locale parity. Fix failures, then run views/core type checks, targeted lint, and whitespace/static boundary checks.
5. Capture and inspect real rendered screenshots at wide/narrow widths, verify keyboard and navigation behavior, and persist the visual verdict. Re-review before additional visual edits.
6. Review the integrated diff, record verification and remaining limitations, and finish task bookkeeping.

Cleanup boundaries: remove the redundant catalog placement and nested clear-filter action. Reuse primitives and routes; do not alter shared catalog behavior used by other pages. Preserve existing behavior with tests before changing it.

## Completion

All six implementation and verification steps are complete. See `verification.md` for executed checks, preserved behavior, environment provenance, screenshots and remaining verification limits. Product commit: `f1038361f65802785429df3f3ad181510322b64c`.
