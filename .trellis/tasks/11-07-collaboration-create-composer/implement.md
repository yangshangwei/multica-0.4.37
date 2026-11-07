# Implementation plan

1. Re-read the collaboration creation spec and current form tests; record the
   current file boundary and do not touch unrelated uncommitted work.
2. Add the composer shell classes and locale copy. Keep the existing Dialog
   lifecycle, title/description semantics, source lock, and constrained body
   scrolling intact.
3. Implement approved option C: title/description first, then compact recipient/AI, attachment details and advanced fields in the scrolling body. Keep only the attachment entry, saved-draft notice and cancel/create in the fixed footer. Retain one upload owner and normal-flow actions.
4. Add optional compact account-picker and attachment-picker presentations.
   Preserve their default markup and existing accessible labels for all other
   callers. Keep upload/recovery, selected-file checkboxes, cancellation, and
   draft persistence unchanged. Compact mode returns lost focus to its visible attachment trigger; default mode preserves native input recovery. Full recipient IDs wrap and selected-name/AI/sharing help remain visible.
5. Update both collaboration locale files and add focused regressions for
   compact presentation semantics, retained title/description submission,
   advanced-field opening/summary, attachment upload/focus, Enter/IME behavior and default picker compatibility. Run existing regressions before the layout change to establish the baseline.
6. Run focused views tests, then views typecheck and lint. Run locale parity and
   `impeccable detect --json packages/views/collaboration`.
7. Run the broader applicable checks (`pnpm --filter @multica/views test` if
   focused execution is insufficient, plus repo typecheck/lint as practical),
   inspect the diff, and perform the Trellis quality check.
8. Build a production component preview using existing fixture infrastructure and current shared code/CSS. Inspect desktop and 390×740/320×568 together in both themes, including long identities, many attachments, recovery and focus. Persist visual-verdict JSON before any correction batch; fix the findings together and confirm once. Stop all preview-only servers. Do not widen into API or mobile work.

## Validation commands

```bash
pnpm --filter @multica/views exec vitest run collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx collaboration/attachment-picker.test.tsx collaboration/focus.test.tsx locales/parity.test.ts
pnpm --filter @multica/views exec vitest run collaboration locales/parity.test.ts --maxWorkers=2
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/collaboration/creation-form.tsx packages/views/collaboration/account-picker.tsx packages/views/collaboration/attachment-picker.tsx
git diff --check
```

## Risk and rollback points

- Highest risk: changing the constrained modal's flex/scroll ownership. Verify
  header, scrolling body, and footer rectangles at desktop, 390×740 and 320×568 in both themes before
  considering the work complete.
- Medium risk: compact controls changing accessible names or test selectors.
  Verify both compact creation callers and default account/property callers.
- Medium risk: moving attachment markup changing upload/recovery reachability.
  Re-run the existing lost-response and quota-preservation tests.
- Rollback is a product-file-only revert; no persistent data or API migration is
  involved.
