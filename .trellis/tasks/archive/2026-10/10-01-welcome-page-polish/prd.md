# Welcome page refinement

User approved the 2026-10-01 audit and requested impeccable adapt → typeset/layout → clarify → polish. Preserve the editorial serif/brand identity, six-stage copy, 24px alternating offsets, small rotations, and navigation behavior.

## Implementation plan
1. Adapt: decouple left intro height from long illustration using an intrinsic, viewport-minimum left column, self-start and desktop sticky positioning. Keep natural scrolling for short/zoomed views. Use 44px CTA heights locally.
2. Typeset/layout: center a bounded left content group with less padding at intermediate widths; use a container-relative hero scale and keep the emphasis phrase together where it fits. Preserve 20px card spacing and right margins.
3. Clarify: add a localized heading and chronological-example description, including the resolved-review/release-decision context. Retain the compact human-approval caption on narrow screens. Represent the six stages as an ordered list.
4. Polish: run views typecheck, scoped ESLint, existing welcome and locale tests, mechanical detector, and browser verification for widths 320/390/720/1024/1440/1780, short windows, English/Chinese, web/desktop, dark theme, keyboard and reduced motion.

## Acceptance
- At 1024×768 English, the primary CTA is visible without scrolling.
- At 1440px Chinese, no isolated 间。 title fragment.
- No horizontal overflow; narrow and zoomed content/actions remain reachable.
- Six historical stages and existing review/human gates retain exact content.
- All welcome actions are >=44px high and continue/cancel/skip callbacks retain behavior.
- Source scope: step-welcome.tsx plus en/zh-Hans onboarding locale files. No dependencies or platform changes.
