# Visual and interaction verification matrix

Primary visual reference: `user-reference.png`, copied from the user's 722×660 screenshot. Baseline verdict is 72/revise. The previous task's 94 score is not the acceptance target: this review must explicitly judge whether empty editing space, labels, attachment chrome and footer density now address the supplied screenshot.

## New visual acceptance

- Empty Chinese draft at 722×660, long source “协作测试 11-06（保留数据）”, default self/AI, realistic full UUID, both themes. Initial-fields summary entirely visible at scrollTop=0; body has no unnecessary vertical overflow. Empty description <=72px. Sharing guidance remains while empty attachment legend is visually hidden.
- Title focus is a 2px opaque bottom border, not a ring. Measurement examines computed bottom-border style/width/color, alpha and effective opacity, composites color against actual ancestor surfaces, and calculates contrast. Screenshots remain authoritative for appearance; color presence alone is not enough.
- Empty 390×740 / 320×568, both locales/themes. Required reason visible, fixed footer controls reachable, English Cancel/Create share a row, no document overflow. Full account identity remains directly visible.
- 200% equivalent reflow with 640×450 CSS viewport at DPR2 representing a 1280×900 device-pixel canvas. This tests reduced CSS space and a doubled raster, NOT actual browser toolbar/OS zoom or physical hardware. Header/actions/advanced summary remain reachable without horizontal document scrolling.

## Retained behavior checks

Reuse actual shared Views/UI/Core source, CSS and translations with page-local memory StorageAdapter and intercepted mock API only. Keep the prior 28 contracts: desktop light/dark, coarse390/320 light/dark, 32 long files across six viewport/theme combinations, upload chooser/cancel/recover/retry focus, original request recovery, refusal, source lock, advanced summary, full-ID selected/candidate ranges, 24-step Tab traversal, native Enter/CDP composition, shared picker defaults.

## Execution and evidence discipline

Wait for implementation-stable before building. Run one combined first batch, inspect screenshots and diagnostics, persist `.omx/state/collaboration-create-polish/ralph-progress.json` before requesting a concrete correction batch. At most one correction+confirmation; no repeated aesthetic edits. Any harness calibration is against the same immutable build and clearly labeled.

```sh
node .trellis/tasks/11-07-collaboration-create-polish/research/preview/build.mjs round-1
node .trellis/tasks/11-07-collaboration-create-polish/research/preview/verify.mjs round-1
```

Every build records current product/CSS/locale/harness SHA-256 values. The verifier starts its own ephemeral loopback listener and fresh Chromium contexts, blocks external traffic, writes concise results/screenshots, and closes browser/listener in finally. No full stack, installs, real accounts/submissions or product edits by this evidence worker. Only meaningful final captures/results and scoped failure evidence should be retained.
