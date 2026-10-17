# Implementation plan

## Planning gate

- [x] Inspect source, callers, CLI resolution and relevant specs.
- [x] Obtain task-creation/planning consent.
- [x] Write converged PRD, design and execution plan.
- [x] Curate real spec/research entries in both manifests.
- [x] Present final summary and obtain its explicit approval (2026-10-17: "请开始实施").
- [x] Run `python3 .trellis/scripts/task.py start .trellis/tasks/10-17-daemon-reconnect-diagnostics` after the gate; branch `codex/daemon-reconnect-diagnostics`.

## Execution sequence

1. Dispatch one `trellis-implement` owner with active task/context. Keep cross-layer result types and locales under one owner.
2. Add failing challenge-category regressions and lock the exact signature fixture/rejection behavior before regrouping validator checks.
3. Define finite safe diagnostic types. Preserve observed CLI download/checksum/extract/install/validation failures without changing candidate selection or checksums.
4. Carry independent setup/connection diagnostics through daemon-manager and preload. Test status polling, relevant success clearing, context resets and superseded attempts.
5. Add bilingual persistent Settings summaries and ordered steps; reuse the mapping for localized reconnect feedback and preserve other callers/disabled states.
6. Run focused tests and package checks. Dispatch `trellis-check` for the complete diff; fix verified issues and rerun affected checks.
7. Update the relevant spec, record native-verification limits, and finish through the repository workflow when the approved scope is complete.

Review follow-up: the transition helper swallows handoff errors into `switching`.
Before final verification, add the guarded internal callback described in
`design.md`, covering stopped-daemon start, same-scope refresh and scope-switch
preflight. Verify unchanged pending/busy semantics and no stale callback
publication, then have `trellis-check` review the completed flow.

## Regression matrix

- `managed-installation.test.ts`: unchanged signed fixture; each scope guard; independently expired and excessive-future timestamps; safe mismatch labels; invalid canonical/envelope/body data.
- CLI/bootstrap or manager tests: fake filesystem/network/process dependencies; stage-specific failures; successful fallback clears the setup failure; no installed CLI execution.
- Manager auth/recovery tests: status reads/pushes agree; CLI and challenge failures coexist; polling preserves them; corresponding success/context switch clears correctly; stale callbacks cannot contaminate new context.
- `daemon-reauth.test.ts`: safe localized transient/unknown/IPC feedback; preserve session-invalid logout behavior.
- `daemon-settings-tab.test.tsx`: both locales, simultaneous errors, remount persistence, long/wrapping content and existing disabled controls. Do not repeat the full category matrix in DOM tests.
- Existing runtime actions tests: the second reconnect caller remains wired if its signature changes.

## Verification commands

```bash
pnpm --filter @multica/desktop exec vitest run src/main/managed-installation.test.ts src/main/daemon-manager-auth.test.ts src/main/daemon-manager-managed-recovery.test.ts src/renderer/src/platform/daemon-reauth.test.ts src/renderer/src/components/daemon-settings-tab.test.tsx src/renderer/src/components/daemon-runtime-card.test.tsx
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/desktop lint
pnpm --filter @multica/desktop test
git diff --check
```

Include new diagnostic/bootstrap suites in the focused command. Check locale JSON/key consistency and secret-free display/IPC projections. If unrelated existing checks fail, establish baseline evidence and report separately instead of widening scope.

For visual verification, use an existing isolated renderer/Electron fixture with fake daemon status, both locales and narrow width. Follow the visual-verdict gate for screenshot iterations. Never launch the user's actual daemon or agent CLI to preview settings. State any unavailable native verification.

## Review and rollback points

- Validator: every old rejection guard remains, valid signature unchanged.
- Manager: independent errors survive polls but not resolved/context-invalid states.
- Renderer: no arbitrary exception fallback or credential-bearing values.
- Revisit design before any protocol, identity reset, credential lifecycle or installer packaging change; these are outside the approved diagnostic scope.
- Code-only rollback, no data cleanup or migration.

## Completion

- [x] Implemented diagnostics, propagation and bilingual presentation with regression coverage.
- [x] Independent full-scope review and handoff-propagation follow-up resolved all important findings.
- [x] Final desktop test suite: 96 files / 1184 tests passed; typecheck and lint passed.
- [x] Seven isolated renderer scenarios and visual verdict passed.
- [x] Updated the executable diagnostic spec and recorded evidence/limits in `verification.md`.
