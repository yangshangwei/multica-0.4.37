# UI and interaction design

Mode: Operate. Preserve existing neutral surfaces and role-named typography.

## Composition
Toolbar sits after the workflow summary, before requirements. Compact ghost buttons with Lucide icons, 4–8px gaps and rounded hover fill; selected item uses foreground/weight plus muted fill. Known counts are small tabular figures, never decorative metrics. At narrow widths wrap to two rows without clipping labels.

One persistent right-aligned Dialog styled as a drawer: desktop width min(44rem, viewport); small windows full width; full viewport height; no central translation. Header contains current category, task title and localized Close. A wrap-capable category nav lets users switch without closing/reopening. The content body scrolls independently. Main layout never reflows.

## Lifetime and reporting
Keep six business panels mounted in one keepMounted Dialog portal, with inactive panels hidden/inert. A scoped callback from attachments/memory/knowledge reports presentation attention and optional confirmed count; parent stores only this ephemeral view status, not server entities. Retain existing details recovery logic for standalone uses. Drawer attention is an inline actionable toolbar reminder, never automatic focus stealing. Disabled/unavailable content keeps existing query states.

Use existing DialogContent keepMounted and positioning classes instead of modifying global Sheet APIs. Child selected version/anchor effects need visibility awareness so closed panels cannot open portals or focus hidden controls. Memory identity and repeated navigation keys remain authoritative. No compatibility fallbacks or second workflow model.

## Risks and rollback
Nested previews and source detail dialogs already use Base UI; verify Escape and focus ownership. Preserve upload lifecycle by never unmounting on drawer close. Identity change unmounts the whole auxiliary owner. Rollback only scoped task files; unrelated dirty work remains intact.
