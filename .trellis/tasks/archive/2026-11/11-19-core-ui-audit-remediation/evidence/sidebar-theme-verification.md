# Sidebar theme verification evidence

This is **static compiled-CSS and source-token evidence**. It does not measure a browser DOM, physical touch target, zoom level, screen reader, or real device.

Run from the checkout root:

```bash
node .trellis/tasks/archive/2026-11/11-19-core-ui-audit-remediation/evidence/verify-sidebar-theme.mjs
```

The script reads actual component class literals with the TypeScript AST, composes the default `CollectionPageHeaderAction` / `Button` classes with the installed `tailwind-merge`, compiles the current Web stylesheet with PostCSS/Tailwind, and resolves the compiled dimension declarations under each media condition. It assumes the standard 16px root font and no overriding ancestor/custom caller style. The full matching declarations and source hashes are recorded in `collection-header-compiled-sizing.json`.

| Scenario | Height | Width |
|---|---:|---|
| 390px, fine pointer | 44px | 44px |
| 390px, coarse pointer | 44px | 44px |
| 1280px, fine pointer | 32px | Content sized; unchanged desktop behavior |
| 1280px, coarse pointer | 44px | Content sized with 44px minimum |

The compiled spacing is `--spacing: 0.25rem`; the minimum target classes resolve to `calc(var(--spacing) * 11)` under `(width < 48rem)` and `(pointer: coarse)`. The default height remains `calc(var(--spacing) * 8)`, and desktop width resolves to `auto`.

`brand-contrast-final.json` computes WCAG 2.x contrast from current opaque OKLCH tokens, converted to clamped 8-bit sRGB. State tokens are resolved from the actual `brand` variant source utilities. This is a color calculation, not screenshot sampling.

| Theme | Default | Hover | Active | Expanded |
|---|---:|---:|---:|---:|
| Light | 4.680:1 | 5.596:1 | 6.638:1 | 5.596:1 |
| Dark | 6.123:1 | 7.171:1 | 8.123:1 | 7.171:1 |

Minimum ratio: **4.680:1**, the light default state. All eight enabled state pairs exceed the normal-text AA threshold of 4.5:1. Actual rendered hit-testing, coarse-pointer input, forced-colors, and device-specific font scaling remain separate validation work.
