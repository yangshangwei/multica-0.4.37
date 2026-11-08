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
