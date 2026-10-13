# Desktop core audit remediation

Nine audited keyboard, accessibility and recovery gaps are fixed in the shared
desktop views. This preserves the current layout, routing and data ownership.

- Filter inputs keep text editing and IME composition out of menu typeahead.
- Agent row actions reveal keyboard focus. Board/member controls and project
  property drawers have contextual accessible names.
- Label foregrounds use WCAG relative luminance while retaining valid fills.
- Inbox selection uses native buttons and current-item semantics, including
  focus handoff across virtual row boundaries.
- Status ordering reuses adjacent controls. Showing archived rows disables
  ordering because server position normalization cannot preserve those visible
  boundaries; the interface explains how to enable it again.
- Gantt, notification preferences and runtime usage distinguish failures from
  successful empty data. Cached content survives transient refresh failures.
- Definitive Gantt access failures erase the cached snapshot. Cancelled older
  requests cannot erase newer successful data.
- Project disclosures expose expanded state and retain the editor/draft while
  closing hidden editor portals.

## Verification before landing

The development checkout passed 307 tests across 20 targeted files, views/core/
desktop type checks, scoped ESLint, whitespace validation, and the Impeccable
scan. Ten Electron checks exercised typing, visible focus, both themes, 200%
zoom, an 80-notification virtual list, ordering and read recovery. Measured label
contrast was at least 4.71:1 in both themes.

The clean-main integration checkout re-runs the targeted suites and type checks
before merge. This isolates the remediation from unrelated collaboration and
release work in the development checkout.

Known verification limits: Windows/Linux, actual screen readers, OS IME candidate
windows, production performance and service writes were not exercised. The local
Gantt backend returned 503 during recovery verification; final successful recovery
used an isolated API fixture. Existing unrelated lint warnings remain unchanged.

## Maintenance contracts

See [desktop core accessibility](../../.trellis/spec/views/frontend/desktop-core-accessibility.md)
for the keyboard, query cancellation, archived-ordering and retained-editor rules.
Detailed local audit artifacts remain under `.impeccable/audit/2026-10-13-core/`.
