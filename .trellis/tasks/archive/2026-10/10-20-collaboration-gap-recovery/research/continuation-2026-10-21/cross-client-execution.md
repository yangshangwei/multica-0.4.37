# AC33 cross-client execution continuation

Status: complete combined Web/Electron journey passed after a bounded
command-history projection repair; independent final review is in progress.
This report concerns only AC33; it does not complete the parent or broaden
production/provider gates.

## Scope and source

- `e2e/collaboration-cross-client-execution.spec.ts` is one combined journey with
  two distinct synthetic password accounts on the same collaboration issue.
- `e2e/fixtures/collaboration-cross-client-execution.ts` creates all accounts,
  managed installations/bindings, private A/B runtimes, resource offers, adoption
  and owner consents through public APIs. It uses `TestApiClient` for setup and
  source-owned read assertions and the existing actual Electron harness.
- `e2e/fixtures/collaboration-cross-client-provider.go` is an explicit stdlib-only
  synthetic provider. A real Linux namespace starts it with actual frozen
  provider input. Its TCP callback observes that input and blocks until release;
  it writes only a business outcome. The real daemon Runner/supervisor produces
  and submits termination evidence.

The journey first blocks A1 from Web, then executes B1 from the actual Electron
collaborator while A1 remains live and unproven. Three later A inputs from both
collaborators must share one pending batch. Electron cancels only the middle
input. Releasing A1 must create exactly one subsequent A process with the two
remaining original-author inputs. Both actual viewers reload command history
and source-owned run records; actor/target/channel/batch/group/run/resource and
result identities, exact execution counts and original request keys are checked.

No product file, shared control/mention fixture, task lifecycle, database row,
service configuration or production gate is modified by this lane. Docker
containers are named `multica-gap-ac33-<lane>-<suffix>` and receive only explicit
test binaries via `docker cp`; they mount no host directories or credentials.
Scoped daemon credentials travel only over stdin and are excluded from evidence.

## Static verification

Executed from `/Volumes/artisan/code/2026/multica-0.4.37`:

```sh
pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts --list
pnpm exec tsc --project /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/tsconfig.json
node packages/views/node_modules/eslint/bin/eslint.js --config /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/eslint.config.mjs e2e/fixtures/collaboration-cross-client-execution.ts e2e/collaboration-cross-client-execution.spec.ts
gofmt -w e2e/fixtures/collaboration-cross-client-provider.go
GOTOOLCHAIN=go1.26.6 CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider e2e/fixtures/collaboration-cross-client-provider.go
```

Playwright registered exactly one combined test. Lint and provider build passed.
The first typecheck found one unused `fixture` argument in the new inspection
helper (`TS6133`); that unused argument was removed. The corrected typecheck,
final typecheck/lint, and Go vet all passed. Logs are under
`/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/`.

## Runtime boundary

The coordinator supplied current-source Linux/arm64 `daemon.test`, `router.test`
and `multica-cli` identities in `linux-binaries.json`. This journey uses the
existing `__multica_collaboration_router_poller` helper inside two independent
privileged Linux containers, each with `MaxConcurrentTasks=1` and a native
containment probe. The provider callback uses confirmed numeric Docker host
IPv4 `192.168.65.254`; no DNS inside a namespace is assumed.

The coordinator granted the exclusive native slot after mention/control runs
closed their apps. The production API/Web/renderer are coordinator-owned. The
explicit provider binary is
`/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider`,
SHA-256 `80c3225030ff105764e3b65309d9b3e599c223a8914def1cb37ab6f3217a9f94`.
Attempt 3 used API `http://localhost:18996`, production Web
`http://localhost:13916` (`build_id=tP4r55FoDsuyXfi1EylGw`, listener PID
`76944`), and the actual Electron renderer `http://localhost:6090`. The API
provenance at the time was source ID
`58b3fa60203026715322f15875f607df0aac1487a544f7d277b43aa137ed35ef`,
configuration ID `805fc993af4daf895e4de11a22cd372244d7845eed807b7eaa56a50f30b39cd3`,
commit `8ecb98883`, listener PID `61506`. The post-fix run must use the new
API provenance supplied by the coordinator; its Web build and renderer identity
remain separately recorded.

## Actual attempts and retained first failures

The exact JSON reports, stderr, traces, screenshots and error-context snapshots
remain under `/tmp/multica-gap-recovery-20261021-h6phxwo9/`. The three actual
invocations were:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env COLLABORATION_CROSS_CLIENT_DAEMON=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/daemon.test COLLABORATION_CROSS_CLIENT_PROVIDER=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider COLLABORATION_CROSS_CLIENT_HOST_IPV4=192.168.65.254 pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts --workers=1 --retries=0 --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run1 > /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run1.json 2> /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run1.stderr
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env COLLABORATION_CROSS_CLIENT_DAEMON=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/daemon.test COLLABORATION_CROSS_CLIENT_PROVIDER=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider COLLABORATION_CROSS_CLIENT_HOST_IPV4=192.168.65.254 pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts --workers=1 --retries=0 --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run2 > /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run2.json 2> /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run2.stderr
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env COLLABORATION_CROSS_CLIENT_DAEMON=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/daemon.test COLLABORATION_CROSS_CLIENT_PROVIDER=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider COLLABORATION_CROSS_CLIENT_HOST_IPV4=192.168.65.254 pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts --workers=1 --retries=0 --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run3 > /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run3.json 2> /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run3.stderr
```

| Attempt | Actual result | Observation and correction |
| --- | --- | --- |
| `cross-client-run1`, 2.393 s | 0 passed / 1 failed / 0 skipped | Fixture sent additive challenge fields to strict enrollment and got HTTP 400 before native/provider launch. Fixed by whitelisting the original four proof fields. |
| `cross-client-run2`, 16.586 s | 0 passed / 1 failed / 0 skipped | Real A/B overlap, B success, pending batch, exact cancellation and A follow-up passed. The test incorrectly required a sequential namespace inode never to be reused; Linux legally reused a destroyed inode. Removed only that assumption while retaining concurrent namespace and unique run/group assertions. |
| `cross-client-run3`, 23.276 s | 0 passed / 1 failed / 0 skipped | All runtime and final API assertions passed. Web command history was unavailable because successful entries exposed `reason: ""`; the client schema requires a nonempty reason or null. The UI assertion remains unchanged. |

Attempt 3 retained issue `01a15d01-a3c6-7781-81d2-f11e3f12eb14`, Web actor
`21496f5e-a654-49ea-86f0-385f9617f683`, and distinct Electron actor
`1a7b010c-b7e5-46ff-80ed-ddba9280f864`. Its source-owned final state has five
immutable commands (four succeeded and one manually cancelled), three succeeded
groups/runs with `exited_proven` and business `success`, and active channels with
no holds. Actual daemon summaries report A claims = 2, B claims = 1, both
`invalid_capacity=false`, one available slot, zero active tasks and exit 0; no
task-scoped containers remained.

The actual callback observations in attempt 3 were:

| Execution | Run ID | Namespace while live | Start / release (monotonic ms) |
| --- | --- | --- | --- |
| A1, Web actor A | `dee84853-81ae-4e1f-a52a-1aa820a3d16c` | `pid:[4026535233]` | 3255.671 / 7037.449 |
| B1, Electron actor B | `75dcc3a8-b1bb-4eca-9686-da5c5cddf851` | `pid:[4026535238]` | 4647.481 / 5064.626 |
| A follow-up, A2/B author + A4/A author | `fa1c6653-56a2-4b4d-a768-81f763e254c6` | `pid:[4026535233]` (legal later reuse) | 7958.679 / 8924.554 |

B's namespace closed at 5065.419 ms before A1 was released. A's subsequent
process consumed exactly sequences 2 and 4 once; sequence 3 remains a cancelled
tombstone. Full resource, target, channel, batch, group, run, body and reader
membership assertions are retained in the `cross-client-execution-identities`
attachment.

## Command-history defect and read-only isolation

`server/internal/service/collaboration_command_history.go:150` maps
`Reason: util.TextToPtr(c.Reason)`. Successful command rows can contain valid
PostgreSQL text `""`; the view therefore emits a non-null empty string.
`packages/core/collaboration/command-history-schema.ts` accepts only a nonempty
string or null. Existing waiting/cancel-only journeys did not exercise this
post-success response.

The actual attempt-3 history was decoded with the current schema in a local
read-only reproduction. It failed exactly four paths, `items[0|2|3|4].reason`,
with `too_small`. In-memory normalization of only those empty reasons to null
made the exact response pass. The result is retained at
`/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/history-schema-reproduction.json`.

The coordinator accepted a bounded server view fix and regression. It will run
its tests and rebuild/restart only the owned API, then provide updated source and
PID provenance. Both real UI history assertions remain mandatory in the complete
rerun; this report does not mark AC33 passed on partial execution.

## Remaining boundaries

This synthetic execution does not use a provider account or quota, prove general
provider/OS support, run a packaged/full Main lifecycle, test squad execution,
or claim full AC95/97 acceptance. Those remain separate evidence scopes.

## Final current-source execution — run 4

The coordinator refreshed only the task-owned API snapshot after confirming the
only product difference was `collaboration_command_history.go`. Original snapshot
files remain backed up outside the repository. New API listener PID **35969**
started at `2026-10-21T08:02:51Z`; its health, source/configuration fingerprints,
launch command and verified history hash are recorded in
`gap-recovery-admin-20261021-api-after-history.running.json` and
`ac33-rerun-preflight.json`. The snapshot's recorded base commit remains
`8ecb98883`, and the manifest also records the current workspace HEAD; those
commit labels alone do not identify the uncommitted repair.

Production Web build `tP4r55FoDsuyXfi1EylGw` and the exact built Electron renderer
index were retained after application-source equality checks. An inherited
renderer helper had exited; the coordinator verified port 6090 was empty,
restarted the known task helper, and compared HTTP 200 bytes to the built index
before launching Electron. No production rollout or provider account was changed.

The exact command ran from the repository root:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env COLLABORATION_CROSS_CLIENT_DAEMON=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/daemon.test COLLABORATION_CROSS_CLIENT_PROVIDER=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider COLLABORATION_CROSS_CLIENT_HOST_IPV4=192.168.65.254 pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts --workers=1 --retries=0 --project=chromium --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run4
```

Result: **exit 0, 1 passed / 0 failed / 0 skipped / 0 flaky**, 29.039 seconds
including setup and 27.441 seconds for the combined journey. Both actual clients'
command-history and run-record assertions passed. The raw report hash is
`2569f54320469122d23ac80c85d1219d7ff26b0fafee7630aae666649adc1f31`;
the decoded safe identity artifact hash is
`370bd5232900fa78c0a830c38845049c76223954464ef91747709735c9b780e6`.
`ac33-run4-result.json` contains the exact argument vector, stats and artifact
hashes. A safe copy of the final JSON/stderr and two actual client screenshots is
under `.omx/reports/cwc-gap-recovery-20261021/ac33-final/`.

The original three failures remain retained. The server serialization regression
and adjacent cancellation evidence are separately recorded in
`history-empty-reason.md`: one focused natural-success pass and thirteen
adjacent top-level passes under database-backed race detection, with no skips.
The previously named `history-empty-reason-green.jsonl` is still a failed fixture
attempt and is never counted as green.

Final issue `01a15d85-fe06-73b5-bbb6-6a455f97dba5` retains two distinct authors,
five immutable commands, four successes and one exact manual cancellation.
A1's namespace stayed live while B1 completed and closed; only after A1 was
released did the A follow-up execute the original sequences 2 and 4.
The daemon summaries show A claims=2 and B claims=1, `MaxConcurrentTasks=1`,
zero active tasks, one available slot, `invalid_capacity=false` and exit 0 in
both lanes. Namespace inode reuse after A1 exited is legal; concurrent A/B
namespace identities, run/group identities and monotonic times prove the
actual overlap and subsequent processing. Both viewers' refreshes produced no
extra execution, and no Web renderer errors were recorded.

This completes the specific AC33 combined behavior, subject to independent
ledger review. It does not establish packaged/full Main startup, squad provider
execution, other-OS containment or real provider executable/version/account/quota
support; all broader criteria and rollout restrictions remain separate.

## Post-extraction confirmation — run 5

After the independently verified helper extraction and Lore commit `839a6606b`,
the task API snapshot was synchronized to the exact new files and restarted with
listener PID68800. Only the identical helper move changed product file layout;
no public behavior, frontend build or rollout configuration changed.
`ac33-api-post-extraction-source.json` and `ac33-run5-preflight.json` preserve
that equality proof and fresh API identity. The exact run4 command with output
`cross-client-run5` ran again: **exit0, 1 passed, 0 failed/skipped/flaky**, total
20.150s. Both actual clients and natural Linux daemon proof/history assertions
passed. `ac33-run5-result.json` and safe decoded run5 identities retain the final
hashes; `.omx/reports/cwc-gap-recovery-20261021/ac33-post-extraction/` contains
JSON/stderr and both screenshots. Runs1–4 remain unchanged and independently
attributed. This rerun is the current exact post-extraction client evidence.
