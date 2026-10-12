# MCP agent-selection design

## Boundary

The gap lives in the saved-configuration branch of `McpSetupDialog`, not in the catalog or backend. The parent dialog continues owning create/assign mutations, explicit selected IDs, confirmed successes, failures, and busy guards. A task-local `McpAgentPicker` may own search and selected-only view state plus the list presentation. Do not turn this into a general actor-selector framework.

Expected application changes: `mcp-setup-dialog.tsx` for integration and scrolling/footer composition; a focused picker component and adjacent interaction tests if separating presentation improves clarity; the existing market regression file for wiring; and English/Chinese `settings.json` keys for the new controls. Existing core/UI primitives and contracts remain unchanged.

## Data and selection

- Use the existing agent list and its eligibility filter, preserving the contextual single-agent flow. Full agent records supply optional description/avatar metadata; contextual ID/name-only data must still render correctly.
- Reuse the shared UI actor avatar directly with available name/avatar data to avoid unrelated directory/profile/navigation queries. Search covers name and full description without adding runtime joins.
- Derive filtered results and counts. Keep explicit selected IDs in the dialog, independent of the current results. Counts intersect with eligible choices and distinguish unfinished selection from confirmed success where necessary.
- Bulk selection unions only visible eligible unfinished IDs into current selection. Deselect/clear only unfinished IDs; confirmed assignments remain checked and disabled.
- Selected-only review may include confirmed rows, but cannot make them actionable. Search controls never mutate assignment results or configuration identity.
- Preserve the existing assignment mutation and server-confirmed success semantics. Retry sends only failed unfinished selected IDs. Permission and busy guards cover all new selection controls as well as submission.

## Presentation and accessibility

Retain the current dialog identity and semantic tokens. Use a compact bordered list with avatar, readable name, optional two-line description, and explicit confirmation/failure text. Expose full description for assistive reading and search. Make a whole row's label the click target while keeping one canonical checkbox, avoiding nested interactive/profile controls. Selected state must remain recognizable on hover.

For the assignment step, separate the search/selection toolbar from the scrolling list. Header and action footer stay fixed within the viewport-bounded dialog. The setup form retains its existing scrolling owner. Stable count/result feedback uses a persistent live status region, and errors remain visible independently of filtering. The existing focus handoff to the assignment heading stays intact.

## Choices

Chosen: local search + scoped bulk controls + selected review, using existing records and primitives. A search-only change leaves repetitive selection and hidden selections unclear; a full runtime-grouped catalog introduces new data joins and product semantics beyond the reported problem. No automatic selection or extra stores are required.

## Verification and rollback

Protect existing behavior with the 41-test baseline and meaningful new filtering/submission/partial-retry tests. Verify the real shared dialog and tokens through an isolated Vite/Playwright fixture with synthetic cached agents; record that this is component-browser validation rather than real API/native Electron acceptance. The existing environment's ownership mismatch prevents safe reuse. Review wide/narrow, light/dark, both languages and long text in one batched pass, then at most one confirmation pass if fixes are required.

Rollback is limited to the task-owned view/localization files; no migration or API compatibility rollout is involved.
