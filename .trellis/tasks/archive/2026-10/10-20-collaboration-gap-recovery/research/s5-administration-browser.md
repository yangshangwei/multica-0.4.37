# S5 production Web and native administration acceptance

Status: both bounded production Web and actual macOS Electron journeys passed
on 2026-10-21. No product fix was required. Full parent acceptance and packaged
application/provider coverage remain with the coordinator.

## Change boundary

S5 already has backend, shared form and platform wiring coverage. The remaining
gap in this lane is proof that password-authenticated production Web and the real
Electron renderer use those paths for a disabled creator's lifecycle changes,
explicit double-disabled successor selection, and retained operation recovery.

- `e2e/fixtures/collaboration-administration.ts` owns password-only synthetic
  account/API setup, the shared UI journey and narrowly scoped read-only ledger
  evidence. It reuses `TestApiClient`; all lifecycle/account changes use real
  endpoints.
- `e2e/collaboration-administration-web.spec.ts` owns the real username/password
  form, HttpOnly session check, one committed-but-lost response and responsive
  captures.
- `e2e/collaboration-administration-desktop.spec.ts` owns isolated native profile
  setup and account navigation. It imports the existing `collaboration-native`
  harness and compositor/zoom capture helpers.
- This report records exact checks, executed evidence and remaining limits.

The coordinator owns product fixes, builds, services, the isolated database,
bootstrap credentials and rollout gates. This lane does not alter those or any
other fixture. No new dependency, real account/provider, installer, deployment or
commit is involved. Existing DB/unit matrices remain the canonical authority,
race and unknown-proof coverage; this journey validates platform integration.

## Executed real journey

Each platform registered its own synthetic creator P, handler H and participant
C through password APIs, created the collaboration through the public creation
endpoint and invited C before disabling either existing account. The injected
synthetic administrator was already bootstrapped by the coordinator. Setup
verified the running password configuration, current super-admin identity and
organization, and zero administrator workspaces.

Both UI journeys verified:

- An active creator produces `409 creator_not_disabled` and no action controls.
- The existing administration account API disables P with its current
  `expected_auth_version` and the administrator's current password. Metadata
  refresh then exposes only the collaboration identifier, source, people,
  lifecycle/control state and exact organization scope.
- Complete, reopen with healthy H, and cancel each send one new UI request key;
  the healthy-handler reopen sends no successor field.
- After the account API disables H, the reopen form lists the already-invited C
  with no selected radio. A reason-only submit displays the focused validation
  error and sends no POST. Explicitly choosing C permits exactly one reopen.
- Each result matches the actual administrator, immutable original creator,
  previous handler, chosen successor, observed status/revision and retained
  operation. Every original GET equals its corresponding POST result.
- Read-only, exact-issue PostgreSQL checks find one `admin_reopen` handoff and
  one precisely correlated administration audit/history entry, four lifecycle
  operations, the original three participants, and the reopened paused issue.
  C's real current-workbench response contains the task; its issue response has
  zero commands/groups/runs. The administrator remains outside the source and
  participant roster. Ordinary issue/detail/comment requests return 404 before
  and after administration.
- With the final receipt retained, the account API restores P. A new, correctly
  captured lifecycle request refuses with `409 creator_not_disabled`. Remounting
  returns the same detail refusal while recovering the original receipt by GET;
  the UI still has exactly four lifecycle POSTs and no new-action/retry controls.
  The committed handoff/audit/operation snapshot remains identical.

Web signs in through the actual username/password form, verifies the HttpOnly
`multica_auth` cookie and absence of a localStorage token, and drops one real,
already-committed final response. The client immediately recovers through its
existing original-key GET; no response body is fabricated. Native uses the
current password JWT only inside its temporary profile, the actual account
deep-link/Main/preload/renderer path, and the existing native harness. It records
zero daemon starts and no ordinary collaboration issue reads from the renderer.
The temporary native profile was removed after the app closed.

Successful JSON evidence contains synthetic identifiers, original operation
keys, scope, lifecycle state, counts and geometry. Authentication traces are
disabled; credentials and the private issue title/body are not attached. The
synthetic database rows remain for coordinator inspection and scoped cleanup.

## Environment and exact executions

The coordinator supplied a fixed `verification-admin-source` snapshot with API
`http://localhost:18996` (listener PID 76036), production Web
`http://localhost:13916`, Web build ID `tP4r55FoDsuyXfi1EylGw`, and matching native
preload/renderer at `http://localhost:6090`. The independent database is
`multica_gap_api_admin`. The private `admin-test.py` wrapper supplies the matching
URLs, database connection, password/admin flags, synthetic credentials and
`COLLABORATION_ELECTRON_SOURCE_ROOT`. This lane did not read a credentials file,
bootstrap an administrator, start services, rebuild, or change either gate.

Required environment names: `E2E_PLATFORM_ADMIN=1`, `E2E_PASSWORD_AUTH=1`,
`E2E_PLATFORM_ADMIN_USERNAME`, `E2E_PLATFORM_ADMIN_PASSWORD`, `DATABASE_URL`,
`COLLABORATION_E2E_API_URL`, `NEXT_PUBLIC_API_URL`, `PLAYWRIGHT_BASE_URL`; native
also requires `COLLABORATION_ELECTRON_RENDERER_URL` and the matching source root.
The fixture rejects non-loopback services, default database names and mismatched
fixture/browser API origins before account creation.

Commands ran from the repository root. All browser runs used one Chromium
worker and zero retries. Logs below live in
`/tmp/multica-gap-recovery-20261021-h6phxwo9/`.

```bash
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --esModuleInterop --types node e2e/fixtures/collaboration-administration.ts e2e/collaboration-administration-web.spec.ts e2e/collaboration-administration-desktop.spec.ts
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true}]' e2e/fixtures/collaboration-administration.ts e2e/collaboration-administration-web.spec.ts e2e/collaboration-administration-desktop.spec.ts
pnpm exec playwright test e2e/collaboration-administration-web.spec.ts e2e/collaboration-administration-desktop.spec.ts --list --reporter=list

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py pnpm exec playwright test e2e/collaboration-administration-web.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/s5-web-first > /tmp/multica-gap-recovery-20261021-h6phxwo9/s5-administration-web-first.json 2> /tmp/multica-gap-recovery-20261021-h6phxwo9/s5-administration-web-first.stderr.log
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py pnpm exec playwright test e2e/collaboration-administration-web.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/s5-web-second > /tmp/multica-gap-recovery-20261021-h6phxwo9/s5-administration-web-second.json 2> /tmp/multica-gap-recovery-20261021-h6phxwo9/s5-administration-web-second.stderr.log
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py pnpm exec playwright test e2e/collaboration-administration-desktop.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/s5-native-first > /tmp/multica-gap-recovery-20261021-h6phxwo9/s5-administration-native-first.json 2> /tmp/multica-gap-recovery-20261021-h6phxwo9/s5-administration-native-first.stderr.log
```

| Execution | Actual result |
| --- | --- |
| Strict TypeScript and scoped ESLint | Exit 0 before execution and after the fixture correction |
| Playwright discovery | Exactly two tests in the two new specs |
| Whitespace checks for all four new files | No whitespace errors; checked added files with `git diff --no-index --check` |
| Web first, 01:36:19 UTC | 1 failed, 0 skipped; 9.650 seconds |
| Web second, 01:40:38 UTC | 1 passed, 0 failed/skipped/flaky; 5.176 seconds |
| Native first, 01:44:05 UTC | 1 passed, 0 failed/skipped/flaky; 9.070 seconds |

The first Web failure was an incorrect fixture expectation: it waited for a
manual recovery alert after the dropped response, while
`submitAdminCollaboration` had already recovered the real receipt through its
existing immediate GET. The preserved screenshot shows the correct reopened,
paused result. The fixture now observes the failed POST transport and that
automatic GET, checks its exact key/body, and retains the one-write assertion.
No application behavior changed and no timeout/reload was added to hide a
consistency failure. Its failed JSON, error context and screenshot remain in
the `s5-web-first` output. The two successful runs cover two distinct tests;
the failed first execution is not added to the passing count.

Test identities:

- `collaboration-administration-web.spec.ts`: Web administers disabled-creator
  collaborations with explicit succession and original receipt recovery.
- `collaboration-administration-desktop.spec.ts`: Desktop administers
  disabled-creator collaborations in the account shell and recovers the
  retained receipt.

Successful fixture issue IDs are
`01a15c1e-2098-7b69-8c42-80da052acee8` (Web) and
`01a15c21-4242-7dac-b87b-2b4de740e56f` (native). Their JSON attachments retain
each original operation/key plus handoff/audit/event identity for inspection.

## Visual evidence and limits

The successful Web run saves eight viewport PNGs under
`.omx/reports/cwc-gap-recovery-20261021/s5-web-second/`, covering disabled creator,
unselected successor validation and restored creator with retained receipt.
Widths are 1380 and 390 CSS pixels. PNG dimensions, browser viewport state,
document/scroll-root horizontal overflow and full focus-section coverage are
asserted, including overlapping captures for tall narrow content.

The native run saves twelve real `webContents.capturePage` PNGs under
`.omx/reports/cwc-gap-recovery-20261021/s5-native-first/`. Its actual wide viewport
is 1380×997 CSS pixels; native 200% zoom produces 450×498 CSS pixels on this
display. The existing helper proves continuous, overlapping coverage of each
long section and validates horizontal bounds. It does not pretend the requested
window height overcomes the macOS display's native bounds.

Representative wide/narrow Web and native wide/200% images were visually
inspected: the unselected radio and validation remain identifiable, the reason
and action controls wrap, and the complete receipt identifiers are readable
across the saved scroll parts. No visual product edit was made.

This is English/light-theme, Chromium production Web and actual macOS Electron
harness evidence. It does not establish packaged full-Main startup, installer or
Windows/Linux UI acceptance, a real provider invocation, or behavior of a live
unknown process. This fixture intentionally has no runs; canonical backend
tests own the unknown-proof, revocation/race and rollback matrices. Those limits
and remaining full-parent acceptance are not converted into passing coverage.

## Delivered source identity

Only the three new test files and this report changed in this lane. Shared
setup/journey reuse keeps Web and Desktop assertions aligned without copying
the native harness. SHA-256 after the successful runs:

```text
9477c31baf2cfba9eaf05fee4b694acd5b0a0cf38e5ecca06a28c548419468f3  e2e/fixtures/collaboration-administration.ts
c21ac3ab9b0454577b06138295f30ae739d8f663101917c54cd48d2b0cd85a0c  e2e/collaboration-administration-web.spec.ts
b04bf8e9d3be24695c72feb16ac731dca0e9cc5e5582adfefcd507b497a3b79b  e2e/collaboration-administration-desktop.spec.ts
```

No commit, deployment, new dependency, real account/provider access or unrelated
file edit was performed. The coordinator owns final integration and acceptance
ledger updates.
