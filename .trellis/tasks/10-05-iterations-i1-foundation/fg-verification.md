> 2026-10-07: FCG also passed; foundation is now completed. Final shared integration, current source identity and real recovery evidence are in [final verification](../10-05-iterations-i1-verification/verification.md). Statements below retaining foundation until FCG describe the earlier FG checkpoint. Rollout remains off; VG remote CI remains pending.

# FG foundation acceptance

Status: **passed** on 2026-10-06. Verified implementation commits:
`bc5a2929b` (current authority and write pipeline) and `aca0762a2` (durable
operations/settings), following the preserved S0–S2 foundation commits.
The server release flag remains off. Foundation stays in_progress until FCG;
LG/HG may begin and client contract work may proceed against stable APIs.

## Gate evidence

| Foundation requirement | Evidence |
|---|---|
| W01–W17 production paths integrated or explicitly exempt | Existing S0/S1, S2 public/system/batch/create/delete/start ledgers; refreshed current writer matrix below |
| Correct lock ordering, current authorization, atomic facts | Real two-connection writer tests; s3-agent-create-authorization-verification.md and s3-write-pipeline-verification.md; attachments retain their prior sequence |
| Bounded transaction conflicts without duplicated facts | Existing caller budgets preserved; new RunOperation uses three RC attempts and stable operation identity; exact request-index conflict recovery only |
| Same request replay, payload conflict, unknown commit recovery | Enable settings is the real production mutation proving the common runner; actual commit+EOF is recovered by GET or the same request; saved result and settings share one transaction |
| Protected operation reads independent of original payload hash | GET current-actor lookup, deleted-identity fixture, two-way revoke/read races and real JWT route checks |
| Workspace/recipient cleanup and authorization foundation | Workspace deletion purges all seven I1 tables in its transaction; ordinary deletion preserves history; recipient cleanup is atomic under the subscriber fence; read/retry authority is current |
| Old endpoint compatibility and inert unconfirmed writes | HTTP/Plugin omission retains membership and rollover; explicit fields return428; T1 damaged/pending associations fail closed; no hidden execution from iteration recording |
| Disabled default, shared timezone, schema/query stability | Default-off iterations_i1 flag, truthful atomic_handoff=false, no duplicate timezone API; real schema/index baseline retained;81 generated files stable after second sqlc run |
| Ordinary-write and populated-workspace regression gates | 4,800 tasks,120,000 historical events; actual server lock-wait sampling; all frozen post-optimization thresholds passed (initial failures retained) |

## Current writer matrix

| Writers | Current production boundary | Canonical evidence |
|---|---|---|
| W01 | HTTP update/batch/rank move, same-tx facts, current grant decision | s0-s1-verification.md; s3-agent-create-authorization-verification.md; s3-write-pipeline-verification.md |
| W02 | Public/Plugin content mutation | s2-verification.md |
| W03/W04 | VCS completion and failed-execution reset | s2-system-writers-verification.md |
| W05 | Removed test-only status bypass; production TaskService path | s2-verification.md |
| W06–W08 | Ordinary/source/Autopilot/onboarding creation fences, no implicit association | s2-creation-writers-verification.md; s3-agent-create-authorization-verification.md |
| W09–W11 | T1 owner fences and invalid association refusal | s2-creation-writers-verification.md |
| W12 | Single/batch deletion retains participation/original facts | s2-delete-writers-verification.md |
| W13/W14 | Squad transfer and project detach facts in owning batch | s2-batch-writers-verification.md |
| W15 | Workspace-wide explicit I1 purge | s2-verification.md |
| W16 | Actual successful task start and participation evidence together | s2-execution-start-verification.md |
| W17 | Verified analytics-only exemption, no start/scope facts | s2-verification.md |

## Final integrated validation

- Handler520 passed,2 existing opt-in skips (TriageQueueScaleBaseline and
  TriageWireFixtures),0 failures: `/tmp/i1-fg-final-handler.jsonl`.
- Supplementary grant snapshot tests9 passed; final settings suite18 passed
  (including owner/admin, task/cloud credentials, strict envelope and recovery).
- Service/channel210 passed; domain/featureflag/public API100 passed; JWT
  settings/operation routes3 passed. No failures/skips in these selections.
- Full Go build and vet exited0; all81 sqlc hashes stable; gofmt/diff checks
  and Trellis context-manifest validation passed.
- Frontend lint/typecheck15/15 tasks;10,054 unit tests across5 tasks passed
  without cache hits. Root scripts exclude Mobile; they are not E2E evidence.
- Built-in guidance checks22 passed after updating the product contracts.

Exact commands, logs, fixtures and intermediate failures are retained in
`s3-operation-settings-verification.md` and the other cited ledgers. These
counts are separate selections and should not be summed as unique tests.

## Capacity and limits

After one fewer round trip in no-attachment W01 updates:

| Four-writer scenario | Median writes/s | Median P95 ms |
|---|---:|---:|
| Original small probe |696.7|9.168|
| Populated, unassociated baseline |644.8|9.963|
| Populated, associated treatment |377.1|14.80|

Frozen absolute baseline gates are570/s and10ms; associated relative throughput,
latency, query-count and sampled-lock-cost gates also passed. The populated
baseline has0.037ms P95 headroom. This is a local regression acceptance on the
recorded M4/PostgreSQL fixture, not a production SLA or full lifecycle capacity.
Single-writer zero wait samples remain diagnostic, not evidence of zero waits.

## Later gate responsibilities

- LG supplies actual create/edit/start/move and T1 accept-and-join semantics;
  the shared durable runner is verified now, not every future operation handler.
- HG supplies real fact projections, full historical display/identifier capture
  and chart DTOs; CG supplies actual closure freeze, handoff, disable and outbox
  delivery. CG must prove recipient reauthorization, no regeneration after
  revoke, crash/retry idempotence and final closed-history stability.
- Ordinary credential checks retain the existing middleware point-in-time
  contract. No new credential-lifetime atomicity guarantee is made.
- Full Go repository tests, Web/Electron E2E, final1,000-item detail/preview/close
  performance, remote CI and release remain VG. FCG follows shared integration
  after LG/HG/CG/UG. Do not enable the rollout flag before complete acceptance.

## Independent phase-boundary review

A separate read-only acceptance review compared this determination with the
original implement/design/test-spec and the pre-continuation handoff. It found
no missing hard FG evidence. Actual outbox delivery is owned by closure after
LG/HG; requiring its implementation before FG would introduce a dependency
cycle. The recipient no-regeneration rule remains a mandatory CG constraint,
not a claimed executed worker test. The reviewer approved unlocking LG/HG while
retaining foundation in_progress until FCG and leaving rollout disabled.
