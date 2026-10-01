# Component Guidelines

> How components are built in this project.

---

## Overview

<!--
Document your project's component conventions here.

Questions to answer:
- What component patterns do you use?
- How are props defined?
- How do you handle composition?
- What accessibility standards apply?
-->

(To be filled by the team)

---

## Component Structure

<!-- Standard structure of a component file -->

(To be filled by the team)

---

## Props Conventions

<!-- How props should be defined and typed -->

(To be filled by the team)

---

## Styling Patterns

### Nested tabs

Base UI puts `data-orientation` on each tab list and tab. Orientation-dependent
styles must read that local attribute (`data-[orientation=horizontal]:...` or
`data-[orientation=vertical]:...`), rather than a named ancestor group. A vertical
Settings root may contain a horizontal MCP catalog; an ancestor group selector
matches both roots and turns the inner list vertical. Keep consumer height and
indicator overrides on the same local variants so `cn` can merge them.

Validate nested orientation with real layout geometry, not jsdom class-string
assertions. `e2e/mcp-desktop.spec.ts` checks that the two catalog tabs share the
same y-coordinate under Settings, after reproducing a 44px difference before
the fix. Its native window checks also cover the standalone and agent-dialog
contexts.

<!-- How styles are applied (CSS modules, styled-components, Tailwind, etc.) -->

(To be filled by the team)

---

## Accessibility

<!-- A11y requirements and patterns -->

(To be filled by the team)

---

## Common Mistakes

<!-- Component-related mistakes your team has made -->

(To be filled by the team)
