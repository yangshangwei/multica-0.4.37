# AC33/95 control journey continuation

Date: 2026-10-21. Existing task and approved implementation plan are unchanged.

## Change boundary

The existing control fixture has two independent empty agent lanes. It proves a
real grant-refusal partial Resume and original-key recovery, but has no squad,
selected follow-up after reconsent, or pending-proof UI explanation.

This continuation owns only `e2e/fixtures/collaboration-control-journey.ts`,
`e2e/collaboration-control-journey-web.spec.ts`,
`e2e/collaboration-control-journey-desktop.spec.ts`, and this report. The shared
fixture will add a distinct squad through real offer/adopt/all-resource owner
consent, verify agent/squad controls leave every unselected lane and hold
unchanged, and reauthorize/resume the remaining refused target explicitly.

A separate scenario uses the enrolled synthetic remote Linux daemon identity to
submit one command and call the real public List, Claim and Permit endpoints.
The server creates its own group, run, attempt and pending termination barrier.
The fixture starts no provider or supervisor process and never posts an outcome
or termination proof. Web/Electron controls can therefore demonstrate a genuine
server `stop_unconfirmed` receipt without claiming kernel execution or proof.
Business/termination capabilities and nonces stay out of report attachments.

No product code, shared schema/locale, dependencies, build, environment, gate or
database writer is modified. The coordinator owns actual browser/native runs and
commits. AC33's real concurrent A/B execution, coalescing, cancellation and
attribution, and AC95's real proof arrival/fresh Resume remain separate runtime
integration requirements; offline or simulated-protocol controls cannot close
those clauses.

## Verification

Initial scoped verification passed: Playwright discovers four tests in two specs;
strict TypeScript and ESLint report no diagnostics. The temporary config extends
the repository React-library/base ESLint settings and includes only the three
owned E2E entry files (plus their transitive imports).

The coordinator released the task-only password/managed production environment
and serialized Electron slot. The first Web attempts are retained:

- `continuation-controls-web-v1.json`: 0 passed / 2 failed / 0 skipped in 3.2s.
  Both fixtures stopped before product controls: the pre-existing
  `markUserOnboarded` helper requires legacy email state that `registerPassword`
  does not initialize. Replaced that call only in the owned fixture with the
  real public `POST /api/me/onboarding/complete`; no database write was added.
- `continuation-controls-web-v2.json`: 0 passed / 2 failed / 0 skipped in 8.5s.
  Resource setup, all consent and UI reads succeeded, but both first control
  POSTs received `403 {"error":"CSRF validation failed"}`. The original browser
  fixture inserted only `multica_auth`. `password_auth.go:31` and
  `auth/cookie.go:221` require the signed `X-CSRF-Token` for cookie-authenticated
  writes; the core client obtains it from the missing `multica_csrf` cookie.
  The fixture now uses real `POST /auth/login` and checks both server-issued
  cookies. Account passwords stay in a private function closure and are never
  attached to evidence. Authentication product code/configuration is unchanged.

These attempts are fixture failures, not passing acceptance or product failures.
Their JSON and traces remain under
`.omx/reports/cwc-gap-recovery-20261021/`.

Further retained runs:

- Web v3: 1 passed / 1 failed / 0 skipped in 13.2s. The complete offline
  agent/squad/grant-recovery journey passed. The new protocol setup misspelled
  `suppressed_target_ids` and correctly received HTTP 400 from the strict public
  comment endpoint. The owned fixture now uses the canonical field and explicit
  `CollaborationCommentInput` type.
- Web v4: 0 passed / 2 failed / 0 skipped in 18.5s. The grant-recovery fixture
  submitted a view's stale hold version after external reconsent, whose visible
  hold reason remains unchanged. An explicit reload now obtains the fresh
  authoritative version before the selected Resume. The protocol journey
  successfully obtained its real Claim/Permit and `stop_requested` receipt with
  one unsettled run, then expected the wrong exact UI label. The canonical
  `stop_requested` label is “Stop accepted; termination remains unconfirmed”;
  `stop_unconfirmed` has the shorter “Termination unconfirmed” label. Both exact
  distinctions are now asserted.
- **Web v5: 2 passed / 0 failed / 0 skipped / 0 flaky in 15.0s.** Actual wide
  (1280×1000) and narrow (390×844) captures assert PNG/viewport dimensions, focus
  bounds and no document overflow. Manual inspection of the narrow proof-partial
  screenshot confirms “2 lanes resumed; 1 remain paused”, the exact unconfirmed
  A lane, two active lanes, and a still-paused A control. Zero target holds and
  one paused lane remain visibly separate facts.
- Desktop v1: 0 passed / 2 failed / 0 skipped in 15.8s. Both windows stayed
  blank and issued no API requests. A direct reachability check immediately
  confirmed `curl http://localhost:6090/` exit 7 / HTTP 000. The coordinator owns
  restoring that production renderer; this is an environment prerequisite
  failure, not evidence against the control implementation.
- **Desktop v2: 2 passed / 0 failed / 0 skipped / 0 flaky in 14.46s.** The
  coordinator restored the owned renderer, the explicit preflight returned HTTP
  200, and the full suite ran after the mention lane released Electron. Real
  isolated Electron windows exercised both shared journeys and production
  account routing/IPC. Wide and 200% captures used the native compositor; manual
  inspection of both 200% partial receipts confirms complete readable labels and
  no horizontal clipping. The tests assert zero renderer errors, no ambient
  workspace headers on account requests, and zero daemon starts. Both native
  applications closed and their isolated profiles were removed; the exclusive
  slot was released immediately afterward.

Environment provenance lives beside this report:
`managed-control-environment.json`, `source-environment-manifest.json`,
`native-artifacts.json`,
`gap-recovery-admin-20261021-api-after-managed.running.json`, and
`gap-recovery-admin-20261021-web.running.json`.

## Current test identities and commands

Each platform has two tests, sharing the real public API assertions:

| Platform | Exact identity |
| --- | --- |
| Web | `Web controls recover partial Resume, isolate agent/squad lanes and stop the whole issue` |
| Web | `Web pending termination proof keeps A paused while whole Resume releases B and squad S` |
| Electron | `Desktop controls recover partial Resume, isolate agent/squad lanes and stop the whole issue` |
| Electron | `Desktop pending termination proof keeps A paused while whole Resume releases B and squad S` |

```sh
pnpm exec playwright test e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts --list
pnpm exec tsc --project /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-control-static-7qc45i4p/tsconfig.json
node packages/views/node_modules/eslint/bin/eslint.js --config /var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-control-static-7qc45i4p/eslint.config.mjs e2e/fixtures/collaboration-control-journey.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py pnpm exec playwright test e2e/collaboration-control-journey-web.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-controls-web-v5
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py pnpm exec playwright test e2e/collaboration-control-journey-desktop.spec.ts --project=chromium --workers=1 --retries=0 --reporter=json --output=.omx/reports/cwc-gap-recovery-20261021/continuation-controls-desktop-v2
```

Each run's JSON reporter output is saved as the sibling `<output>.json` file.
The final strict TypeScript and ESLint reruns after the Web repairs both exited
0. The pnpm package-field migration warning is retained and unrelated to tests.

The final passing Web/native runs share these unchanged source hashes:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-control-journey.ts` | `1e6b3536b6ba030242320635e9892ee4df86dd63e10f5e81fbd84d63c8e4f512` |
| `e2e/collaboration-control-journey-web.spec.ts` | `97189fd859b7d8a524b6686bb19f23f478600362f9eb9eed46b627acb886dc9b` |
| `e2e/collaboration-control-journey-desktop.spec.ts` | `705b5648f4a675ab43283688e0145dbf6b7f6544eb38fed64e0f2e7214f62c01` |

All three source files and this new report pass the untracked-file whitespace
check (`git diff --no-index --check /dev/null <file>`; exit 1 with empty output
means only the expected new-file difference). Shared consent and control-request
observation helpers serve both journeys; Web/Electron wrappers share the same
behavior assertions. No dependency or product abstraction was added. The
coordinator owns any Lore commit and the acceptance-ledger/spec integration.

## Runtime evidence boundary and remaining inputs

No native Linux runtime test is run by this lane. Canonical source identities
for real execution/proof behavior are:

- `server/cmd/server/collaboration_hold_routes_test.go`:
  `TestCollaborationPartialResumeRoutesLinux` (two real namespace runners,
  independent resources, original proof barrier, later B execution and fresh A
  Resume after proof).
- `server/cmd/server/collaboration_single_agent_routes_test.go`:
  `TestCollaborationSingleAgentRuntimeRoutesLinux` (actual immutable input
  consumption, coalesced later batch, Stop, delayed proof and fresh Resume).
- `server/cmd/server/collaboration_daemon_cli_routes_test.go`:
  `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux` (actual CLI/daemon
  poller and capacity-one squad chain).
- `server/internal/service/collaboration_control_managed_test.go` and
  `packages/views/collaboration/execution-panel.test.tsx` retain canonical
  service boundary matrices and receipt rendering checks.

The Linux route tests require actual Linux, the isolated migrated database,
`MULTICA_RUN_COLLAB_RUNTIME_E2E=1`, static test executables (`CGO_ENABLED=0`) and
working PID/mount/network namespaces. The CLI/daemon chain also requires explicit
test-built `MULTICA_TEST_COLLAB_CLI` and `MULTICA_TEST_COLLAB_DAEMON`; the latter
uses `__multica_collaboration_router_poller` with server-issued scoped binding
credentials supplied privately over stdin. Ordinary installed providers and
accounts are never discovered by these tests.

Joining those native runners to this GUI fixture requires a real Linux harness
that consumes this API deployment's enrolled binding, fixed resource/run identity
and test-owned executable, can reach the owned API, retains its private ledger,
and delays only the real sealed proof's transport. Existing Go route helpers use
their own httptest router and isolated rows, so their assertions cannot be
relabeled as a Web/Electron execution journey. No such cross-client/process
handoff or successful proof arrival is claimed here.
