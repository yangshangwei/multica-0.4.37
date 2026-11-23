# Independent full Main / AC31 review

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`.
Reviewer: `full_main_check`, dispatched `trellis-check`, 2026-10-22.

Decision: **the packaged Main and cross-client continuity evidence is valid;
keep AC31 pending until the three information/acceptance UI branches and the
coordinator's final rerun are verified**. This is a concrete first-version flow
gap, not a requirement for commercial provider accounts, signed releases or
every desktop operating system. No ledger promotion, product/test/spec change,
runtime launch, build, race run, commit or archive was performed by this reviewer.

## Findings (fixed)

None. This review owns only `full-main-review-*` documents. The inspected product
and test files remain coordinator/implementer-owned and read-only here.

## Findings (not fixed)

1. **AC31 first clause: actual request-information, return-information and
   submit-acceptance form submissions are not yet evidenced in both clients.**
   R04/R18/R20 are existing first-version requirements; P6 includes their shared
   workflow forms. `packages/views/collaboration/workflow.tsx:119` separates
   recipient/status requirements, and lines134–136 build three different
   payloads, including the captured information-request ID. The inspected
   `workflow.test.tsx` exercises transfer/recovery/edit/remove but none of these
   branches. `e2e/fixtures/collaboration-offline-result-notifications.ts:151`
   invokes the three public operations through `TestApiClient`; the current
   paired offline tests prove notification consumption and authority but not
   those form submissions. The full Main spec exercises invitation, transfer,
   comments, target/issue control and waiting cancellation/history, without the
   information/acceptance branch sequence. Do not infer those UI receipts from
   canonical service PASS counts or the notification UI.

   Recommended bounded continuation: the installed handler requests information
   from the creator; the creator returns the exact current request to the
   handler; the handler submits acceptance to the creator. Exercise the same
   three shared actions on Web and Desktop, retaining exact original keys,
   status-option versions, handler/current/participating lists, same issue UUID
   and unchanged session/execution/commands. These actions must leave the issue
   open, preserve former-handler control rights and never count as completion.
   Root has acknowledged and assigned this extension within existing S6.

2. **A later paired run requires fresh diagnosis and verification.**
   `installed-pair-coordinator-final-result.json` is exit1; its actual JSON has
   0 expected /2 unexpected /0 skipped /0 flaky and retry0. The old CLI case
   lacks its explicit relay input; the full Main case reaches an assertion in
   `originalResponse` at fixture source line139. **139 is a source line, not an
   observed native process exit status.** The exact diagnostics are retained in
   `installed-pair-failure-errors.json`. This reviewer does not infer a product
   failure or fixture cause without the coordinator's diagnosis. The prior
   full Main1/1 remains a valid individually identified run, but it cannot turn
   the later paired result into a pass. Root owns the fixture repair/rerun and
   must correlate current test hashes before promotion.

## Original clause and evidence mapping

Original AC31, primary owner P6, `acceptance.md:39`:

> 首版用户流程在 Web 和 Desktop 均可完成；跨端接续处理时，当前处理人、协作权限、任务历史和队列状态保持一致，不因切换客户端产生另一份协作任务或不同的操作权限。

| Exact clause | Reviewed evidence | Assessment |
| --- | --- | --- |
| First-version Web/Desktop flows | Current dual-client typed creation/retained files3/3; offers and owner consent; mixed mentions6/6; controls4/4; removal/natural proof/reconnect2/2; same-lane/complete/cancel6/6; source surfaces/CLI1/1; offline/admin4/4; actual independent Linux execution; new packaged Main1/1 | Strong combined support; the three distinct information/acceptance UI submissions above still lack client receipts. Canonical/shared tests and fresh client wiring are identified by scope, not presented as every flow freshly exercised in installed Main. |
| Current handler remains consistent while changing clients | Public Web invite/transfer to a fresh zero-workspace password actor; same issue read in full Main; native transfer back; current list moves once, former handler remains in participating | Satisfied by exact1/1. Handler changes by the authorized operation, not by client selection. |
| Collaboration permissions remain consistent | Real password form200; full Main public control writes; former handler retains `control_ai:true` and loses `transfer`; creator retains its actual capabilities;185 account requests without workspace headers | Satisfied. Differing actor roles are not a client-dependent permission difference. No seeded token or native auth/IPC substitute. |
| Same history, queue and issue identity | Same issue UUID, creator, session and complete execution projection across transfer/read/reload/restart; exact manual B cancellation body/author/event/command; A unconfirmed barrier; individual B/S controls and whole partial Resume; no ordinary dispatch | Satisfied. Unknown A remains ending/held, whole Stop remains paused, and restart does not create a replacement issue/run or revive cancelled input. |

The aggregate evidence catalogue entries reviewed were
`web-offers`, `native-offers`, `web-records`, `web-workbench`,
`continuation-20261022-content-clients`,
`continuation-20261022-lifecycle-clients`,
`continuation-20261022-control-clients`,
`continuation-final-source-clients`,
`continuation-final-offline-notifications`,
`continuation-final-administration`, `continuation-final-mixed-mentions`,
`continuation-final-ac33-individual` and
`continuation-20261022-current-lane-close`. All88 cited artifact hashes match.
Prior independent slice reviews retain their own command/source/native limits.

## Exact packaged run and provenance

`full-main-current-third.stdout` parses as one actual expected Playwright test,
zero unexpected/skipped/flaky and one result with retry0. Suite duration
39.412293s; test duration37.973s; command elapsed42.529s. Exact identity:

`collaboration-installed-main.spec.ts:57` —
`installed Linux full Main and production Web hand off the same issue, preserve authority/history and reject unconfirmed Resume`.

Raw SHA256:
`fc03372f3252b54432d3979835ed518b71ed73dab121a46efac17999cc7dccf3`.
Sanitized journey SHA256:
`40f8921a08a69b1c181bf4fc176caa35965cae44334f002599154148ad2c47c0`.

The exact electron-builder Linuxarm64 **unsigned directory package** executes
the complete production Main/preload/renderer from `/opt/multica/resources/app.asar`,
with `app.isPackaged:true`, Electron39.8.7, app version
`0.6.5-collaboration.63c0beeb5`, default isolated OS home `/root` and default
userData `/root/.config/Multica`. It is an executable packaged application;
system installer integration/signature/public distribution are not established.
Those additional release properties are not written into original AC31.

| Artifact | Exact SHA256 |
| --- | --- |
| `multica-desktop` | `379334fb6707615062d32fcc481a8a8d3140938531378ccba14f3fa169665043` |
| `resources/app.asar` | `35690546d3148df7c702ae39a5589e684eeb379893e34e79b2438ada578b3ea5` |
| Bundled Linux CLI | `d4f9091acf534ec4fff9f3110d063ac58ead160b4badedf0607a5b989edfc732` |

`full-main-package-identity.json` records1578 current application source hashes;
the reviewer recomputed all1578 against this checkout and found zero mismatch.
Boot and restart report the same executable/ASAR/userData. Bundle/build/package
commands have exit0 and explicitly use `--publish never`; the named owned volume
is mounted read-only. The host's `HOME`/`CODEX_HOME` were not overridden.

Actual profile files set daemon `autoStart:false`, `autoStop:false` and updater
`automaticUpdates:false`. Metadata calls the real native `getStatus`, `getPrefs`,
`isCliInstalled` and updater preferences. Real bundled CLI bootstrap succeeds and
its version command runs. No alternate Main or native IPC handler replacements
exist in the new fixture. The standard renderer/native harness used for other
slice evidence still has named daemon/install stubs; its scope remains explicit.

The same UUID is `01a163d1-9ab4-7b5a-8041-a250dd2594cb`.
Native handler is `95bc232a-df44-4374-a1c7-c6c4c049c58e`; responsibility returns
to creator `dd858035-6458-4aba-bdf3-810f08de0daa`. Cancelled B command is
`04f39a6a-5086-4e89-99d1-fb9e8b497211`; exact manual cancellation event is
`fab8ec8e-fc38-44be-b4a3-df1483b03c0f`. One remote permitted run remains
`69c3b213-6c64-4aad-a63b-b62994dc224e`; it comes from the disclosed public
Claim/Permit protocol simulation, supplies no provider outcome or termination
proof and is never reported as a physical daemon termination test.

Production Web/API endpoints are13916/18996; the retained preflight identifies
the owned production Web build `tP4r55FoDsuyXfi1EylGw`, source/configuration and
listener. Current packaged source equality supplements that older embedded
commit label. Native renders its actual packaged file URL and carries its real
client version/Linux headers on writes. Real-provider accounts/quota and
production creation/execution/rollout gates remain closed.

## Verification

- Lint: retained scoped base ESLint3 files,0 errors/0 warnings, exit0.
- TypeCheck: retained strict noEmit/noUnused check for spec/TS transport, exit0.
- Syntax/discovery/whitespace: retained node syntax, one named Playwright
  identity, scoped diff check exit0. Listing is not test acceptance.
- Tests: individually audited full Main1/1 above; later paired0/2 remains
  unresolved. No reviewer test run or runtime mutation.
- Integrity: raw/safe attachment hashes verified;3 static-test source hashes
  match their command record;1578 packaged application source hashes match;
  88 aggregate catalogue artifact hashes match.

The original first stale-revision transfer and second wrong-lane cancellation
fixture attempts are retained with distinct source/receipt hashes. The first
never launched Main; the second ran the real Main but matched a lane-local
sequence globally. Their repaired third1/1 is not an automatic retry. The later
paired failure is retained separately and has not been diagnosed by this review.

The current ledger remains94 passed /3 not verified,97 original criteria,
`full_acceptance_complete:false`, task/parent `in_progress`. The review does not
approve AC43 provider usage or installation/update/signature/other-OS release
coverage, and it does not require those unrelated approvals as an AC31 substitute.
