# Core UI audit remediation

## Problem

The 2026-11-19 Impeccable audit found core Web/Electron UI defects in draft preservation, hidden focus, loading/error recovery, picker and editor semantics, dark-theme contrast, disclosure/tab state, read-only rendering, touch targets, and reduced-motion behavior.

## Scope

Fix the confirmed F01-F11 findings from `.impeccable/audit/2026-11-19-core/report.md` while preserving existing product copy, navigation, API contracts, and current desktop/web design tokens. Add focused regression coverage for changed behavior.

## Acceptance criteria

- Squad instruction edits survive a remote squad invalidation after debounce until the user saves or explicitly discards them.
- Closing a right detail sidebar removes its content from the accessibility tree and keyboard tab order; reopening restores the content without stealing focus unexpectedly.
- Squad detail/create surfaces distinguish pending, initial error, cached refresh error, not-found/forbidden, and authoritative empty states; retry preserves existing input.
- Core property pickers and squad-member selection expose selected/pressed state and keyboard active-option semantics.
- Issue description, comment, and reply content editors have stable localized accessible names in the mounted editor.
- Brand-filled controls meet WCAG AA normal-text contrast in both themes and interaction states.
- Issue detail disclosure controls expose expanded state and stable panel relationships; squad detail tabs expose tab semantics and selected panel association.
- Read-only squad instructions render as read-only content; assignee cold failures expose recovery instead of false empty results.
- Shared collection header actions provide at least 44px coarse-pointer hit targets while retaining desktop density.
- Reduced-motion preference disables both panel geometry and content motion.
- Relevant unit/component tests, typecheck/lint for changed packages, and the Impeccable detector complete with evidence.

## Out of scope

- Backend/API changes, mobile UI, unconfirmed narrow-screen geometry candidates, unrelated detector hits in tests/Markdown, and broad visual redesign.
