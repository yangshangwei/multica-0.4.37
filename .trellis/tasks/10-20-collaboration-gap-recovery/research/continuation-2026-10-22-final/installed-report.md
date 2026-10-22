# Current old packaged-CLI regression — 2026-10-22

The focused current Playwright run passes **1/1**, zero skip/fail/flaky. It
executes the exact previously delivered local offline OCI `/app/multica`
`v0.6.7`, commit `9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`, image digest
`sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104`.
This is actual packaged prior-version byte evidence, not a historical source
build. Local offline delivery and published registry/signature trust are
separate; the original release record says no GitHub publication.

The exact ordinary legacy `PUT /api/issues/<uuid>` succeeds200. For the same
shared issue, old CLI status update, comment add and rerun each return409 in
three states: paused with an original waiting input; Stop requested with a
server-issued permitted run and no proof; and author removed with its waiting
input cancelled. Every request checks the actual old CLI version/platform/OS
headers and compares the entire protected state/history/participant/resource
snapshot before and after, including original creator/handler, command/lane/
batch/run IDs and all control/hold versions. There are **9 shared refusals**,
no ordinary task enqueue, no proof fabricated and no queue recovery.

The removed author retains explicit source membership solely to reach the real
old route boundary instead of tenant middleware. Its removed collaboration
input remains `cancelled/participant_removed`; removing participation is not
loss of that independent source read basis. Its own operation GET equals the
original content-free comment receipt and contains no comment content. The
creator cannot read another actor's own operation key; the earlier404 correctly
exposed the fixture's mistaken actor assumption. Original Stop/Resume receipts
are recovered exactly by the original actor without writes.

The new stdlib Go relay runs inside the no-network readonly Docker container.
Only exact expected method/path leaves private Docker stdio and reaches the
existing task-owned loopback API. Relay bytes enter a fresh named volume via a
stopped stage and are readonly in the runtime; no host file bind or HOME change
is necessary. All temporary containers/volumes are gone. Auth values appear
only on private stdin/in-process environment, and sanitized artifact scanning
finds no credential/token patterns.

## Verification and evidence

- `installed-old-cli-final-command.json`/`-result.json` and Playwright JSON:
  1 passed,0 failed/skipped/flaky, source unchanged;15.6s Playwright duration.
- `installed-old-cli-final-evidence.json`: exact package version, all10 observed
  calls, protected snapshots and original command/control operation identities.
- `installed-canonical-race-*`: Go `-race -p1 -parallel1 -timeout=30m` over
  handler,daemon,cli,cmd/multica against `service_s5` (migration737), **15 top-level /
  47 including subtests passed**, zero failed/skipped. Includes legacy mutation
  boundaries, current paging/restart/proof recovery and narrow CLI guards.
- `installed-typescript-final-*`, `installed-lint-final-*` and `installed-go-vet-*`:
  exit0; strict noUnused/noUnchecked tsc and repository ESLint for the two TS files,
  stdlib relay Go vet; Linux relay build exit0.
- `installed-evidence-integrity.json`: source hashes, credential-pattern absence,
  no owned container/volume leftovers and diff whitespace exit0.

All prior attempts are retained: collection parameter, readonly-rootfs copy,
omitted zero exit frame, wrong history path, wrong operation actor and invented
redaction fields were corrected in the fixture. None establishes a product
failure. See `installed-report.json` for exact attempt/result references.

## Changed files and remaining scope

Only `e2e/collaboration-installed-clients.spec.ts`, its TS fixture and test-only
stdlib Go relay changed. `installed-*` reports/logs were added under this task.
No new dependency, product path API, user/global config, shared launcher,
production gate, ledger/spec or commit was changed by this lane.

The actual old CLI byte case plus current production daemon reconnect/natural
Linux task evidence can be assessed by the coordinator's independent review
against AC32's exact P5 clause. This lane does not promote that criterion on its
own. The new case itself does not start any daemon/provider/process proof.
Complete installed current full Main, released old Desktop UI, signed publisher
release/installer trust and other OS/provider account matrices are not certified.
AC31 remains the coordinator's separate full Main environment/journey work.
