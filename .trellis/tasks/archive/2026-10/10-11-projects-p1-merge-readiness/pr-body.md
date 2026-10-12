## Summary

Project drafts could lose their first input when Tiptap delivered `create` after
an input update. Keep that pending update available for autosave and the opted-in
unmount flush, with deterministic regressions for both delivery paths.

- Resolve mixed and numeric workspace references such as `12AB-42` and `2026-42`
  consistently in iteration selection and backend issue search.
- Align regression fixtures with persisted opt-outs and the current UI, and use
  the same zh-Hans label for scope changes. Keep the draft E2E assertions intact.
- Retain the dimensions and references of 13 audit screenshots while bringing
  them within the bitmap budget. Update the affected executable specifications.

## Validation

- The unchanged draft browser baseline reproduced P05 failures in 4/10 runs;
  lifecycle observations confirmed `update → create` only in the failing path.
- Both new editor regressions failed before the fix. The editor, accessibility,
  and project lifetime suites then passed 44/44, with targeted ESLint and views
  type checking passing.
- Bitmap budget against current `origin/main`: passed without an exemption.
- Complete `make check`: 15 lint/typecheck tasks, 10,970 TypeScript tests, Go race
  and vet, production Web build, and 258 browser passes / 70 conditional skips.
- Dedicated I1 Web/Electron acceptance then passed all 19 originally skipped I1
  tests, with zero retries. The remaining 51 skips require separate configured
  provider/authentication/CLI environments.
- Both unchanged draft browser cases passed 10/10 on the committed production
  build: 20 executions, zero retries, failures, skips, or flaky results.
- Latest-main integration changed only the Compose overlay regression assertion;
  its focused verification passed 33 tests, with 3 environment-dependent skips.
- PR CI: all 16 checks passed on the candidate, including backend race tests,
  frontend build/tests, Mobile Verify, Windows execution checks, three-platform
  installers, vulnerability scanning, and the bitmap gate.

## Upgrade context

The target already includes migrations 567–570; this correction adds no migration.
567 enables defaults for new triage/iteration settings without changing saved
choices. 568 retains administrator initialization markers, and 569–570 create
their concurrent unique indexes. Their rollback guards reject removal after
initialization data exists.

New or missing workspace iteration/triage settings are enabled, while saved
opt-outs remain disabled. `FF_ITERATIONS_I1` is removed. Unconfigured planning
timezones use `Asia/Shanghai`; existing iterations retain their saved timezone.
Credential publication requires Linux `renameat2(RENAME_NOREPLACE)` support from
the kernel/filesystem, or macOS `renameatx_np(RENAME_EXCL)`, and fails closed when
that operation is unavailable. Use a merge commit to retain the corrective
decision history.
