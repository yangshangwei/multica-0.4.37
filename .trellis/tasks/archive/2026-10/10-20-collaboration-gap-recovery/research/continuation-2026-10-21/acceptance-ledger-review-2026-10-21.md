# Acceptance ledger continuation review — 2026-10-21

Status: **conservative ledger update applied; full acceptance remains false**.
This review audited the continuation artifacts against `acceptance-matrix.json`,
`s6-acceptance-audit.md`, `progress.md`, `implement.md`, and the generated
continuation proposal. Product source, runtime gates, provider accounts and
parent task lifecycle were not changed.

## Protected definitions and ledger result

The authoritative matrix now records **28 passed / 69 not verified / 0 failed /
0 skipped / 0 unsupported**. Only rows whose current assertions and platform
receipts were reviewed in this lane were promoted. The protected 97-row
criterion definition hash is unchanged:

`e29ead58ba9e2c45a4099a52611c55e12dcae3b735136e058aafc87674cf217e`

The evidence catalog now includes the continuation run records from
`s6-evidence-map.next.json`, including partitioned Go, Linux router, Web,
Electron, and password/admin journey artifacts. Every evidence `run_id` in the
matrix resolves to an entry in `research/s6-evidence-map.json`.

## Promoted rows

- **AC90** — Final Web attempt 6 and actual Electron attempt 2 each pass the
  single committed H+A+B+S comment with distinct target/channel/command/batch/
  input identities and one located H notification. Current service preview and
  commit tests pass in the final race partition.
- **AC91** — The same final Web/Electron journeys cover read-only concrete and
  unavailable previews, A refusal after owner revocation, B queue retention,
  H notification, exact original-key retry deduplication, and no second send.
  Service preview/grouping/commit tests pass.
- **AC97** — Final Web/Electron offline journeys perform fresh API login/profile
  recovery, notification anchor navigation, read/archive without completing the
  issue, and comment/body edit deltas. Only newly effective human recipients
  are notified; typed AI edits create no commands; outside squad humans remain
  uninvited.
- **AC95** — Web 2/2 and actual Electron 2/2 control journeys cover agent,
  squad and whole-issue Stop/Resume, unselected-lane preservation, pending
  proof explanations and explicit re-consent. Current Linux router tests and
  service race identities supply execution/proof and lifecycle barriers.
- **AC02** — The canonical service/handler notification/workbench tests pass;
  final Web/Electron offline journeys prove persistent delivery after fresh
  login, anchor navigation and read/archive preserving business/workbench state.
- **AC12** — Current service/handler control identities pass for authorized
  original-result replay, stale-stop fencing and structured refusal recovery.
  Web/Electron controls and Linux routes show accepted/refused/partial/unknown
  operator states and original-key recovery.
- **AC68** — Broad current Go evidence passes mixed-batch refusal, legacy issue
  write/comment/rerun boundaries, ordinary execution/quick-create paths and
  attachment capability boundaries. This is backend-primary evidence; packaged
  old clients and provider rollout remain outside this row's scope.
- **AC82** — Broad current Go evidence passes ordinary-child projection/write
  isolation, parent/history preservation, machine minimal references, foreign
  workspace rejection and ordinary execution outside collaboration resources.
  This is backend-primary evidence.

## Exact evidence references

- `.trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21/mention-journeys.md`
  — final Web attempt 6 and Electron attempt 2 commands, receipts, source
  hashes, offline login, anchors, read/archive and edit-delta assertions.
- `.trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21/control-journeys.md`
  — final Web v5 and Electron v2 control commands and boundaries.
- `.trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21/mention-control-final-review.md`
  — independent AC90/91/95/97 assessment and exact canonical service identities.
- `.trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21/ordinary-control-audit.md`
  — assertion-level AC02/12/68/82 recommendations and concrete reasons to keep
  AC63/81/84/87/89 open.
- `research/s6-evidence-map.json` continuation entries:
  `continuation-mentions-web`, `continuation-mentions-native`,
  `continuation-controls-web`, `continuation-controls-native`,
  `continuation-linux-router`, `continuation-service-1`,
  `continuation-service-2`, `continuation-go-other` and related supplements.

## Rows intentionally kept open

- **AC33** remains `not_verified_in_this_task`: the real cross-client runtime
  execution passed its overlap/cancellation assertions, but command-history
  parsing exposed `reason: ""` versus the strict client schema's nonempty-or-null
  contract. The bounded service projection fix and full rerun are still pending.
- **AC63** lacks a current refusal snapshot that includes collaboration
  attachments and every permission relation before/after deletion.
- **AC81** lacks the complete external-handler iteration statistics/preview/
  history projection and full source/iteration race boundary.
- **AC84** lacks a current collaboration-specific IM/channel outbound-redaction
  integration assertion.
- **AC87** lacks a current Web/Electron negative proving collaboration selection
  hides the create-subtask affordance while ordinary selection keeps it.
- **AC89** lacks the same-column collaboration board drag and CLI `issue reorder`
  positive paths in one current matrix.

The remaining 63 rows beyond these specifically reviewed gaps are unchanged from
this lane's prior status. The production collaboration creation/execution gates
remain closed; real provider account/quota, packaged Main/installer, Windows and
macOS containment, stock provider/version matrix, S3/Redis fault acceptance and
other parent-wide limits remain separate evidence boundaries.

## Verification

```text
python3 -m json.tool .trellis/tasks/10-20-collaboration-gap-recovery/acceptance-matrix.json
python3 -m json.tool .trellis/tasks/10-20-collaboration-gap-recovery/research/s6-evidence-map.json
python3 ./.trellis/scripts/task.py validate .trellis/tasks/10-20-collaboration-gap-recovery
```

All three commands passed. A separate reference check confirmed every matrix
`evidence[].run_id` exists in the catalog, all eight newly incorporated
continuation artifact hashes match, and the protected definition hash above is
unchanged. Two pre-existing `all-components` catalog entries have no recorded
SHA-256 (`/tmp/multica-s3-all-core-green.log` and
`/tmp/multica-s3-all-views-green.log`); they were not rewritten or treated as
new continuation evidence.
