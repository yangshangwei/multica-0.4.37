# Handoff: collaboration gap recovery continuation

Date: 2026-10-21
Branch: `codex/cross-workspace-collaboration`
Task: `.trellis/tasks/10-20-collaboration-gap-recovery`
Parent: `10-10-cross-workspace-collaboration`
Status: `in_progress`

Continue this existing task. Do not create a task, replan, archive the parent, reset, clean, or discard the shared dirty workspace.

## Start

```bash
cd /Volumes/artisan/code/2026/multica-0.4.37
python3 ./.trellis/scripts/get_context.py
python3 ./.trellis/scripts/task.py validate .trellis/tasks/10-20-collaboration-gap-recovery
git status --short --branch
```

Read the original handoff, `progress.md`, `implement.md`, `research/s6-acceptance-audit.md`, then these continuation reports:

- `research/continuation-2026-10-21/pipeline-verification.md`
- `research/continuation-2026-10-21/cross-client-execution.md`
- `research/continuation-2026-10-21/mention-control-final-review.md`
- `research/continuation-2026-10-21/ordinary-control-audit.md`

## Current state

- Authoritative `acceptance-matrix.json` is now **70 passed / 27 not verified / 0 failed** after independent full-clause review and current follow-ups. All97 protected definitions retain SHAe29ead58ba9e2c45a4099a52611c55e12dcae3b735136e058aafc87674cf217e.
- Original `acceptance-matrix.next.json`64/33 remains historical proposal. Six of its automatic promotions were rejected; subsequent different criteria pass. The collector now preserves statuses/gaps and writes only a fresh output directory.
- Verified Lore commits already present: `b67c8253f` (hold commit-fault test repair) and `7c77bf157` (mixed mention/offline notification Web + Electron journeys).
- The workspace still contains broad shared collaboration implementation and task artifacts. Preserve all of it.
- Full service race evidence is partitioned and green: 1153 distinct service top-level passes; all 81 Go packages accounted for; 18 Redis service supplements and 73 other Redis tests pass; six actual Linux router/daemon tests pass.
- Final mixed mention journey: Web 3/3 and Electron 3/3. Final control journey: Web 2/2 and Electron 2/2. Password/admin S5 journey: Web 1/1 and Electron 1/1. Reports and hashes are in the continuation directory and `.omx/reports/cwc-gap-recovery-20261021/`.
- Production creation/execution gates remain closed. No real provider account/quota smoke was run.

## Prior AC33 blocker — resolved by current evidence

The real AC33 cross-client journey reached the intended behavior: A1 stayed busy while B1 executed; A2/A3/A4 coalesced; only A3 was cancelled; a follow-up consumed A2+A4; real namespace/claim/proof/capacity and actor/target/channel/batch/run checks passed.

Run 3 failed only when the successful command history was parsed. The service emits `reason: ""` from `server/internal/service/collaboration_command_history.go:150` (`util.TextToPtr(c.Reason)`), while the strict client schema accepts only a nonempty string or `null`. Read-only reproduction: `/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/history-schema-reproduction.json`; exactly four empty-reason items fail, and empty-to-null normalization passes.

Bounded fix ownership:

- `server/internal/service/collaboration_command_history.go`
- `server/internal/service/collaboration_command_history_test.go`
- `research/continuation-2026-10-21/history-empty-reason.md`

Change only the service projection so absent/empty reason becomes JSON `null`; preserve nonempty failure/cancellation reasons and generic `util.TextToPtr`. Add a failure-first DB-backed regression and run focused race/vet. Do not weaken the client schema or the AC33 history assertion.

After the fix, rebuild/restart only the task-owned password/managed API, record new PID/source/configuration provenance, verify Web/renderer identity, and rerun the exact cross-client Playwright command. Preserve run1/run2/run3 failures. Final AC33 pass requires the complete Web/Electron history view, not only runtime proof.

AC33 runtime artifacts:

- API/Web/native environment was formerly API `18996`, Web `13916`, renderer `6090`.
- Synthetic provider: `/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider`, expected SHA-256 `80c3225030ff105764e3b65309d9b3e599c223a8914def1cb37ab6f3217a9f94`; verify before use.
- Docker host IPv4: `192.168.65.254`.
- Use `docker create`, `docker cp`, `docker start --attach`; no host/home mounts or credential files.
- Native slot is currently owned by the offline-handoff lane. Run Electron serially and probe exact renderer HTTP200/index bytes before launch.

## Ledger after AC33

Review, do not blindly apply, `research/continuation-2026-10-21/acceptance-matrix.next.json` and `ledger-proposal.json`.

Ordinary audit recommends promoting AC02, AC12, AC68 and AC82 after exact evidence review. Keep these open:

- AC63: refusal snapshot lacks collaboration attachment and complete permission relation.
- AC81: external-handler iteration statistics/preview/history and full race boundary are missing.
- AC84: collaboration-specific IM/channel outbound redaction is missing.
- AC87: dual-end collaboration selection must hide the create-subtask UI.
- AC89: collaboration issue board drag and CLI `issue reorder` positive paths are missing.

After AC33/history fix, update the authoritative matrix, evidence map, audit, progress, specs and this continuation record. Preserve all 97 immutable definitions and verify the protected-definition hash before and after. Do not archive the child or parent while required evidence is missing.

## Safety and commits

- No `git reset`, `git clean`, force-push, broad revert, production gate change, or real provider account access.
- Every behavior slice gets an inline Lore commit with `Co-authored-by: OmX <omx@oh-my-codex.dev>`.
- Run the exact verification command and read its output before claiming success.
- `pnpm knip` currently reports pre-existing unused files/dependencies; record it and do not perform unrelated cleanup.

Baseline snapshots remain at `/tmp/multica-gap-recovery-20261021-h6phxwo9/` and `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-gap-continuation-20261021-d_16vnr3/`.


## Current next-step ownership and completed work

AC33 full combined run4 and exact post-extraction run5 passed Web/Electron with
real Linux daemon proof. The history helper is committed839a6606b; API snapshot
PID68800 at18996 includes the exact identical helper extraction. Web13916 retains
production buildtP4r55FoDsuyXfi1EylGw; renderer6090 uses known built index.
`ac33-run5-preflight.json` and `cross-client-execution.md` contain exact identities.

Completed verified Lore slices:

- 839a6606b: nullable successful history reason/helper and failure-first/isolated DB proof.
- aef77e9ce: evidence refresh cannot auto-approve or overwrite authoritative output.
- 67a8e5c23: real edit/delete contention and participant/source-only unresolve.
- 1918a41b9: paused-period input/empty-target and same-resource per-task grant isolation.
- abe5a9494: independently committable real encrypted attachment/permission deletion refusal.
- 5baf86592: actual Web/Electron shared/ordinary selection boundary.

AC63 original offer-path expanded function stays dirty and retained; standalone
Register/Consent regression committed independently. The raw owned.patch stays
on disk but is excluded from commit because patch context causes whitespace
check diagnostics; baseline/full function proof remains in JSON evidence.

Active next lanes within the existing plan:

- Offline public invite/assignment/transfer, fresh login/archive/client restart,
  former-handler participating/current and revoked entry: AC77/59; full83 remains
  partial unless every lifecycle/result notification clause is evidenced.
- Backend creator/current/former-handler control cycle, newly invited existing-
  grant use and active-run SubmitAcceptance/noncompletion: AC06/16/28/30.
- Source project/parent/iteration membership/transfer/version races and external-
  handler statistics/preview/history: AC80/81.

Service_s5 schema737 is shared by scoped tests; reserve it serially because
fixtures modify its isolated execution-gate row. Root coordinates DB/native
slots and commits; child scopes never touch production gates or provider accounts.
The child and parent stay in_progress. Treat earlier missing-stage paragraphs
as historical boundaries; use latest matrix/catalog/progress for current status.


## Latest continuation after local midnight — 2026-10-22

Authoritative ledger is now **79 passed /18 not verified /0 failed**, not the
older70/27 snapshot. All97 definitions retain the same protected SHA. Newly
accepted clauses in this session: AC06/16/28/30 (actual handler control/active
acceptance correlation), AC59/77 (offline workbench/archive/restart), AC93 (all
seven direct @ states), AC80/81 (all source roles/reasons/strict PUT and races).

Current verified Lore commits:

- `ccedea89e`: handler shared-grant authority and explicit whole completion.
- `28681f527`: offline responsibility/archive/restart, existing native helper.
- `7794f57d9`: real source lock-order races and external source iteration planning.
- `cef01d954`: direct typed @ state admission and ordinary fallback refusal.
- `1cb3f5073`: existing plugin private-event gate plus actual TLS/Slack transport.
- `829815775`: direct source role/reason/request separation.

AC84 stays partial for other IM private direct/failure/stream/relay/media paths.
The current catalog has189 valid artifact references. New tests continue in
three bounded lanes: retained comment attachments/typed creation body,
Web/Electron pause/removal/reconnect/whole lifecycle, and multi-target owner
exit/public staged Switch to busy destination. Main owns all matrix/catalog/
spec/commit integration, native slots and application snapshot identity.

DB slots: owner/Switch owns `service_s5` while running; root previously released
`service_s6_collab` after outbound isolated validation; `service_s6_ordinary`
is available after source-role HEAD isolation. Never run gate-mutating tests
against an allocated peer alias or the API database.

Task API/Web/native endpoints stay password18996/13916/6090. Current exact
product snapshot matches5094 product files, API PID68800. Legacy18985/13905/6079
was refreshed only for the already committed history helper and owned renderer
restored. Current creation/delivery Web/Electron2/2 and backend6/6 pass but
AC67/75 remain partial pending direct current remaining clauses. No real provider
account/quota or production gate changed. Child and parent remain in_progress.
