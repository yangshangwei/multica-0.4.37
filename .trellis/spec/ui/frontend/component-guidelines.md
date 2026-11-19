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

### Preserve native popup visibility defaults

Base UI merges caller props after its own lifecycle props, including keys whose
value is `undefined`. For a retained `DialogContent keepMounted`, Popup and
Backdrop own `hidden: !mounted`. A wrapper must omit its visibility overrides
while `PopupVisibilityProvider` is visible; forwarding `hidden: undefined`
replaces native `hidden: true` and leaves a closed drawer and backdrop onscreen.

Preserve caller props first, then add suppression only while the enclosing
surface is hidden:

```tsx
<DialogPrimitive.Popup
  {...props}
  {...(!visible ? { hidden: true, inert: true, "aria-hidden": true } : {})}
/>
```

Use the same omission rule for Backdrop's `hidden`. Do not fix a retained
drawer by removing `keepMounted`: its editors and pending operations need that
lifetime. Preserve native open-state ownership and hidden-surface final-focus
suppression. `hidden={visible ? undefined : true}` still supplies the key and
is not equivalent to omitting it.

The real-primitive collaboration auxiliary tests must assert the full popup and
backdrop are hidden before first open and after closing, the close control leaves
the accessibility tree, focus returns, and reopening preserves the same editor
and its draft. A hidden body panel alone does not prove the drawer closed.

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
