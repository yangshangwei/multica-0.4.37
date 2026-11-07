# Implementation and checks

1. Read approved option C contracts and the user screenshot; record baseline visual verdict before edits.
2. Apply the bounded presentation changes in the three scoped components. Reuse existing tokens and copy, keep default callers unchanged.
3. Run focused collaboration/attachment/account/focus tests plus locale parity and Views typecheck/lint. No tests that merely assert Tailwind strings.
4. Adapt the archived production component preview into this task, retaining actual current components/CSS and mock API/memory storage. Add empty screenshot-sized 722x660 captures, desktop, 390x740/320x568 both themes, English footer, long identities, attachment states and painted bottom-border focus checks. Include 200% zoom/reflow evidence with explicit method.
5. Inspect all relevant sizes in one batch and persist visual-verdict before any correction. Correct measured defects together, then confirm once. Existing native Enter/IME, initial focus and cancel/retry focus regressions must stay green.
6. Run final bounded collaboration+locale suite, typecheck/lint, scoped Impeccable detector and diff check; preserve exact final source provenance. Record limits, stop owned preview resources, sync narrow spec additions, commit only task-owned files and archive.

Commands:
```sh
pnpm --filter @multica/views exec vitest run collaboration locales/parity.test.ts --maxWorkers=2
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/collaboration/creation-form.tsx packages/views/collaboration/account-picker.tsx packages/views/collaboration/attachment-picker.tsx
git diff --check
```
