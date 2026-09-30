# Agent directory presentation polish

## Approved scope

Apply the screenshot review in priority order: stable squad membership column,
less duplicated row metadata, clearer filtering and direct access from the Mika
hint. Preserve the concurrently edited category semantics and core view state.

## Implementation

- Keep the 64px virtualized row contract. Give squads a 240px desktop track shared
  by the header, rows and loading skeleton. Show two links and a keyboard-accessible
  overflow popover; nested links must stop click and auxiliary-click propagation.
- Remove repeated template provenance from row names; search still uses role
  metadata. Show a presence dot on the avatar only when the status column is hidden.
  Suppress owner badges in Mine or when the owner column is enabled. Localize the
  private visibility tooltip and allow keyboard focus.
- Keep category/squad controls aligned with the concurrent category implementation.
  Show removable selected conditions. More filters counts only its own dimensions;
  clear-all is a separate real button. Separate archive visually without changing
  the saved scope model. Link the saved Mika name directly using the navigation adapter.

## Verification

- Focused views tests cover list behavior, squad overflow navigation, filtering,
  access cells, i18n parity, keyboard clearing and Mika navigation.
- Targeted ESLint passes; layout detector reports no findings.
- Directory Playwright scenario passed against the existing Web dev server using
  a temporary API-forwarding route because its configured API origin was stale.
  Screenshots reviewed at 1440, 768 and 390px; no page-level horizontal overflow.
- Final full views typecheck passes. An earlier concurrent test error was resolved
  by its owning session; no unrelated test edits were made in this slice.

## Boundaries

No category classification changes, API changes or dependencies in this slice. Mid-width list scrolling remains intentional. Full production integration
is not established by the temporary local API-forwarding browser check.
