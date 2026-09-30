# MCP audit fixes

Implement the approved MCP audit in order: harden → clarify → adapt → layout → polish.

## Scope and preservation

- Preserve the monochrome interface, recipe identity, permissions, explicit assignment, retry semantics and shared Web/Desktop behavior.
- Reuse existing UI components and semantic tokens. No dependencies, global token changes or unrelated agent-category edits.
- Existing market, setup and discovery tests protect behavior; add focused regressions for result announcements and pending assignment before implementation.
- Simplify the card hierarchy by placing the icon beside the title and removing the fixed minimum height. Replace discovery's boolean busy state with the pending server identity instead of adding duplicate state.

## Ordered work

1. Harden: accessible card focus, persistent search result announcements, visible per-server pending assignment with completion feedback, safe long-text wrapping.
2. Clarify: distinguish custom creation from templates, expose a template action, explain name restrictions before submission, present browser tools by their existing use cases.
3. Adapt: scope 44px coarse-pointer controls to MCP surfaces, preserve single-column narrow layouts and reachable dialog actions.
4. Layout: keep two desktop columns, group icon/title, remove excess card whitespace and use a readable description role.
5. Polish: inspect desktop and narrow light/dark states together, fix discovered defects in one batch, confirm once, review diff and document contracts.

## Verification

- Focused Vitest suites: market/setup, discovery, workspace MCP and locale contracts.
- ESLint for changed frontend files, views TypeScript check, Impeccable detector.
- Browser checks on current source with isolated fixture data: keyboard focus, result filtering, configuration form, 320/390/1440px widths, light/dark and coarse pointer, plus long content.
- Save visual verdict and screenshot evidence; distinguish fixture browser checks from live backend/provider integration.

## Completion evidence

- Completed all five passes in the approved order.
- 73 focused Vitest tests passed across market/setup, discovery, workspace MCP,
  locale contracts and the custom configuration dialog. New regression assertions
  failed before the result announcements, pending feedback, field guidance and
  save-step focus handoff were implemented.
- Views TypeScript check, targeted ESLint, Go built-in MCP template tests and
  Impeccable detector passed. No dependencies or shared color tokens changed.
- Final isolated Chromium round: 65 checks passed, no page errors. Covered
  320/390/720/1440px, Chinese/English, light/dark, coarse pointer, long content,
  filtering, setup, assignment, pending state and custom configuration dialogs.
- Browser focus contrast: 19.49:1 light / 17.27:1 dark; sampled coarse-pointer
  controls reach 44px. Saving focuses the assignment heading; Tab continues to
  the first agent checkbox.
- Evidence: `.omx/state/mcp-audit-fixes/harness/round-2/browser-checks.json` and
  sibling screenshots. Visual verdict: `.omx/state/mcp-audit-fixes/ralph-progress.json`.
- Remaining verification limits: fixtures exercise real UI with in-memory data,
  not live API persistence, Electron IPC, provider execution or a real screen reader.
