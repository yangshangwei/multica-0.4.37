# Compact agent batch toolbar repair

## Scope and evidence

This incremental implementation addresses the remaining `86 / revise` finding
in `evidence/visual-verdict.json`. The English and Chinese 450px effective-width
captures show a centered batch overlay covering the list header; its selected
count also wraps while its action buttons remain single-line. These are real
Electron captures, not a jsdom layout claim.

The source cause is the absolute wrapper at `left: 50%` with an automatic
shrink-to-fit width, combined with nonshrinking action buttons and a shrinking
count group. The compact `above-chat-launcher` clearance moves that oversized
overlay into filters and headers in a 350px effective-height window.

## Incremental changes

- `packages/views/agents/components/agent-batch-toolbar.tsx`: one toolbar DOM
  remains in the existing page tail. Below `md`, it participates in normal flex
  flow and reserves inline chat-launcher clearance. At `md` and above, the
  centered overlay uses explicit max-content width bounded by the page width.
  The count group cannot shrink or wrap; the action group can wrap without
  changing action order, labels, permissions, confirmation or mutations. Clear
  selection uses the shared ghost Button and its visible keyboard focus.
- `packages/views/agents/components/agents-page.tsx`: compact pages can scroll
  vertically. While selection is nonempty, the existing list scroller retains
  enough height for its `h-9` header, an `h-9` group heading and one agent row.
  The row portion comes from the existing `ROW_HEIGHT` (64px), rather than a
  duplicated row-size constant. This prevents a wrapped footer from leaving a
  scroll area too short for a keyboard-reachable checkbox. Wide layout keeps
  the existing `min-h-0` single list scroller.
- `packages/views/agents/components/agent-batch-toolbar.test.tsx`: four additional
  behavior cases cover Space/Enter clearing exactly once without API writes,
  mixed lifecycle action tab order, restoring only archived agents, and the
  keyboard confirmation gate before archiving only active agents. No CSS-class
  or synthetic jsdom rectangle tests were added.

Other working-copy changes were preserved. No dependencies, translations,
shared tokens, dialogs, API behavior, staging or commits were changed by this
incremental implementation.

## Verification run

| Check | Result | Record |
| --- | --- | --- |
| Original toolbar + AgentsPage suites before edits | 34 tests passed, 2 files | `evidence/compact-toolbar-before-tests.log` |
| Behavior regressions added before the presentation change | 38 tests passed, 2 files | `evidence/compact-toolbar-regressions.log` |
| Same suites after the presentation change | 38 tests passed, 2 files | `evidence/compact-toolbar-after-tests.log` |
| Scoped source and test ESLint | Exit 0; no ESLint findings | `evidence/compact-toolbar-lint.log` |
| Final toolbar suite after the typing correction | 12 tests passed, 1 file | `evidence/compact-toolbar-final-tests.log` |
| Final scoped source and test ESLint | Exit 0; no ESLint findings | `evidence/compact-toolbar-final-lint.log` |
| Final views package typecheck | Exit 0; `tsc --noEmit` passed | `evidence/compact-toolbar-views-typecheck.log` |
| Scoped `git diff --check` | Exit 0; no output | Implementer command result |
| Automatic Impeccable design hook on toolbar source/test edits | No deterministic design-quality findings | Tool-hook output |

Test command:

```sh
pnpm --filter @multica/views exec vitest run agents/components/agent-batch-toolbar.test.tsx agents/components/agents-page.test.tsx
```

Lint command:

```sh
pnpm --filter @multica/views exec eslint agents/components/agent-batch-toolbar.tsx agents/components/agent-batch-toolbar.test.tsx agents/components/agents-page.tsx agents/components/agents-page.test.tsx
```

The existing root package-manager configuration warning appears in the logs;
it is separate from ESLint findings.

The final archive-confirmation lookup uses only `{ name: "Archive" }`.
Testing Library's `ByRoleOptions` has no `exact` option; a string `name` already
uses exact matching. The initially added invalid field was removed without
casts or suppressions, and the final package typecheck verifies the correction.

## Coordinator handoff and limits

The coordinator's real Electron confirmation is recorded in
`evidence/toolbar-browser-confirmation.json` and the `*-confirmed.png` captures.
This implementer read that record: Chinese light/dark and English light use a
static compact toolbar, a single-line count, zero header/launcher overlap and
zero document horizontal overflow. The record also confirms wide floating
presentation, keyboard action reachability and keyboard clearing.

Active and archived page scopes are mutually exclusive, so the native capture
does not claim a mixed lifecycle selection. Mixed restore/access/archive action
ordering and mutation targeting remain covered by the canonical toolbar DOM
suite. Normal compact internal vertical scrolling is intentional. The `4.5rem`
minimum viewport allowance follows the current two `h-9` headers; revisit it if
those shared/list heading heights change.

The coordinator still owns the final full-workspace typecheck and the updated
independent visual verdict. This implementation does not replace that verdict
or broaden the recorded native evidence into a general accessibility claim.
