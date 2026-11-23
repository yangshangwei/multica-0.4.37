# Collaboration creation closure implementation plan

## Authorized scope

The user approved finishing the existing collaboration-creation layout and
explicitly declined a new Trellis task. Work continues as a bounded direct
follow-up; no task is created or activated. Preserve the current bottom property
toolbar, ordinary composer styling, and existing draft/source/request ownership.

## Behavior gap and ownership

- Compact recipient triggers currently hide the complete account ID in a
  tooltip and screen-reader text. Restore a directly visible, wrapping identity
  without changing default picker behavior or recipient lookup.
- Creation no longer has the independent selected-recipient status sentence or
  visible AI-participation explanation required by the existing contract. Restore
  those using existing localized copy.
- The bottom toolbar, disclosure button, and default-height growing editor need
  current-browser evidence. Preserve form identity, body-only scrolling,
  native submission, upload/AI recovery, and the local Electron no-drag boundary.

Production changes are limited to `packages/views/collaboration/account-picker.tsx`
and `creation-form.tsx`; their existing tests cover meaningful semantic and
interaction regressions. The main session owns the scoped spec update, this
plan, verification fixtures/results, and selective commit. Existing Core,
backend, docs-site, stage-approval locale additions, and other task changes are
outside this follow-up.

## Implementation and verification order

1. Save the original page diff and a dirty-path/index manifest before edits.
2. Build an isolated preview from current production components and shared CSS.
   Capture desktop/narrow baseline and record a visual verdict before UI edits.
3. Add a regression for the independent selected-recipient status and observe
   the expected failure. Browser text-range geometry, rather than jsdom text
   presence or Tailwind-string assertions, owns real identity/help visibility.
4. Restore visible identity and contextual guidance at their owning components;
   preserve the bottom toolbar and existing API/persistence contracts.
5. Inspect one batched final-build matrix: desktop plus 390x740/320x568, both
   themes and relevant Chinese/English states; long/same-name identities,
   advanced fields, long AI text, attachments, error/recovery, focus and resize.
   Independent review also owns an intermediate-width first-edit probe; add any
   confirmed responsive focus defect as a real-CSS regression before its fix.
   Persist visual verdicts. Correct measured defects in one bounded batch and
   confirm only affected states if required.
6. Run scoped collaboration/assistant/locale tests, Views typecheck and lint,
   the scoped Impeccable detector, and diff checks. Independent source review
   checks the implementation, spec and evidence.
7. Update `collaboration-creation.md` and write a factual audit report with exact
   source/build provenance and verification limits. Close owned browsers,
   Electron fixtures and loopback listeners.
8. Commit only the authorized page changes and their evidence using Lore intent
   and trailers. Keep unrelated staged/unstaged work intact. No Trellis task
   archive or journal auto-commit is run for this direct follow-up.

## Confirmed review follow-up

The 520x568 first Start-date edit was observed failing against the earlier frozen
build: count growth moved the entire control group onto another row, clipping
32px of the active date field. The final local correction adds a stable 20rem
group basis and growth, clamped to the available width. The existing 12 scenarios
plus this new regression must pass on the same final build before submission.

## Validation commands

```sh
pnpm --filter @multica/views exec vitest run collaboration modals/issue-description-assist.test.tsx locales/parity.test.ts --maxWorkers=2
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/collaboration/creation-form.tsx packages/views/collaboration/account-picker.tsx
git diff --check -- packages/views/collaboration/account-picker.tsx packages/views/collaboration/account-picker.test.tsx packages/views/collaboration/creation-form.tsx packages/views/collaboration/creation-form.test.tsx .trellis/spec/views/frontend/collaboration-creation.md docs/plans/2026-11-14-collaboration-creation-closure.md
```

## Limits and rollback

The preview uses real shared components/CSS with deterministic mock transport
and memory drafts. It proves rendering and integration, not real-model content
quality, paid quota, production submission, physical touch/IME, or spoken
screen-reader acceptance. Native input proof, if run, is explicitly distinguished
from Chromium synthetic clicks. Rollback is confined to this follow-up's page
diff and requires no data migration.
