# Form control boundaries and fills

`packages/ui/styles/tokens.css` owns two distinct semantic tokens:

- `--input` / `border-input`: the visible form-control boundary.
- `--input-fill` / `bg-input-fill`: muted interior, disabled and inactive fills.

Keep the tokens separate when changing contrast. Inputs and textareas use
`border-input` while their dark and disabled backgrounds use `bg-input-fill`
with the existing opacity modifiers. Inactive switches, outline-button dark
fills and decorative table separators retain the fill tone; increasing the
boundary contrast must not turn these into stronger surfaces.

Use `--ring` for focus styling. A token's raw contrast is not proof that an
alpha-modified focus halo meets a threshold on every background. The Web
`text-contrast.test.ts` explicitly excludes input and ring colors. Validate
actual compiled styles and rendered light/dark controls separately.

`apps/web/app/brand-variant-cascade.test.ts` verifies that outline/background
variants do not override brand colors in either Web or Desktop stylesheets.
Before changing these tokens, compare actual input, textarea, checkbox, switch
and outline/brand button rendering in both themes, including focus and disabled
states. Preserve geometry and existing fill colors when only boundaries change.

## Filled brand controls and touch targets

Keep text-brand ink separate from a filled control's foreground. The brand button
variant pairs `--brand-foreground` with solid `--brand`, `--brand-hover` and
`--brand-active` surfaces. Alpha-only hover/active fills can fail normal-text
contrast even when default passes. Both themes and forced-light landing copies
must preserve >=4.5:1, verified by `apps/web/app/text-contrast.test.ts` plus the
compiled brand cascade suite. Use `variant="brand"` for active filters instead
of layering bg-brand/text-white over outline/default classes.

Collection header actions retain dense wide-screen sizing but use a 44px minimum
in narrow/coarse-pointer conditions. Verify compiled sizing or actual geometry;
a class-name assertion alone is not a hit-target measurement.
