# Implementation and acceptance evidence

## Outcome

Settings → Daemon now retains separate runtime setup and device connection
diagnoses, with localized failure stages, safe mismatch labels, occurrence time,
optional HTTP status and ordered initial steps. Reconnect toasts use the same
English/Chinese explanation mapping. The original generic challenge error is
classified without changing accepted signatures or weakening validation.

## Final verification

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/desktop test` | PASS: 96 files, 1184 tests |
| `pnpm --filter @multica/desktop typecheck` | PASS: Node and renderer |
| `pnpm --filter @multica/desktop lint` | PASS: 0 errors, one existing warning |
| `git diff --check` | PASS |
| Locale validation | Both JSON files parse; 46 diagnostic keys match |
| Independent `trellis-check` | No important unresolved findings after follow-up |
| Isolated Chromium renderer | 7 scenarios pass; visual verdict 96/100 |

The existing lint warning is `tab-content.tsx:54`, a `react-hooks/exhaustive-deps`
warning about `group`. That file was not changed. pnpm also reports the existing
root `package.json` configuration deprecation; no dependency configuration was
changed in this task.

Final command outputs are saved in `evidence/final-tests.txt`,
`evidence/final-typecheck.txt` and `evidence/final-lint.txt`.

## Acceptance mapping

- AC1: validator tests distinguish malformed/inconsistent input, scope fields,
  expiry and excessive future expiry. The original Go/Node signature fixture
  passes; `now + 1` and `now + 300` remain valid.
- AC2: manager and Settings regressions preserve simultaneous CLI/connection
  diagnoses through polling and remount. The isolated renderer checks both
  regions and their ordered steps.
- AC3: corresponding success clears its category. Context reset and delayed
  old-server health results cannot re-publish stale diagnoses/metadata.
  Same-context diagnostic pushes retain PID/profile/external-management data.
- AC4: English/Chinese, light/dark and 960/450px widths pass without horizontal
  overflow. Reconnect feedback is localized; malformed/unknown diagnostic data
  renders a safe fallback. Existing disabled/external-management guards remain.
- AC5: original rejection checks and signed bytes remain. Tests verify session
  invalidation versus transient failures and absence of arbitrary source text
  from display diagnostics. Background handoff errors are reported internally,
  with unchanged busy/switching and no start/drain after failed preparation.
- AC6: final package checks pass as recorded above. Native evidence limits are
  explicitly recorded below.

## Review corrections

1. Preserve the complete known status when publishing a diagnostic so external
   management guards and runtime metadata are not temporarily dropped.
2. Reset metadata before server-change publication and fence adoption of late
   health responses before they affect either status or auth/profile state.
3. Require string diagnostic codes before allowlist lookup; malformed object,
   array and null categories must not coerce into keys or crash presentation.
4. Propagate background handoff preparation errors through a guarded internal
   callback because the existing transition helper intentionally returns a
   pending state after catching them. Raw errors remain outside IPC results.

## Simplifications

- Consolidated three reauthentication result type declarations into the shared
  `DaemonReauthResult` contract.
- Reused one renderer diagnosis-to-copy mapping for persistent settings and
  reconnect toasts.
- Kept CLI and connection diagnostics in Main memory and used existing status
  IPC; no new store, persisted data format, dependency or service endpoint.

## Visual evidence

The fixture imports the real Settings component, translations and desktop CSS,
with synthetic auth and in-memory daemon IPC. It starts no real daemon and
accesses no live server. The seven cases include dual failures in both locales
and themes, narrow layouts, expiry instructions, installation permissions and
unknown-data safety. JSON evidence is checked in under `evidence/`.

Local preview sources and screenshots:

- `.omx/qa/daemon-reconnect-diagnostics/{server.mjs,fixture.tsx,verify.mjs}`
- `.omx/qa/daemon-reconnect-diagnostics/reference-settings.png`
- `.omx/qa/daemon-reconnect-diagnostics/round-2/zh-light-wide.png`
- `.omx/qa/daemon-reconnect-diagnostics/round-2/zh-light-narrow.png`
- `.omx/qa/daemon-reconnect-diagnostics/round-2/en-dark-wide.png`
- `.omx/qa/daemon-reconnect-diagnostics/round-2/en-dark-narrow.png`
- `.omx/qa/daemon-reconnect-diagnostics/round-2/zh-expired.png`
- `.omx/qa/daemon-reconnect-diagnostics/round-2/zh-install-permissions.png`

## Limits

No native Windows execution, actual Windows permissions/protection software,
real preload integration or affected live server was exercised. The clock-skew
fixture establishes a possible cause of the original message, not the root
cause on the user's device. No Windows installer was rebuilt and nothing was
published, pushed, merged or deployed as part of this task.

## Changed files

The exact product/spec file inventory is stored in `changed-files.txt`.
The main areas are challenge validation/session preparation, CLI bootstrap,
daemon manager/transition, shared/preload IPC types, the Settings and runtime
reconnect callers, the shared renderer diagnostic mapping, bilingual locale
resources and their regression tests. The diagnostic contract is documented
in `.trellis/spec/desktop/frontend/daemon-diagnostics.md`.
