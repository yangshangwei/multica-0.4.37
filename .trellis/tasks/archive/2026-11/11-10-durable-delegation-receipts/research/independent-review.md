# Independent review — durable delegation receipts

**PASS — zero remaining blocking findings** for the opt-in, attachment-free ordinary comment delegation increment. This covers explicit agent/squad targets and durable recovery. It does not complete all comment writers, actual provider execution, the full M1 delivery gate, M2/M3, or the six-business roadmap.

The reviewer made no product edits, database writes or model calls. Fixes below were made by implementation owners; the reviewer traced their final code paths and independently verified source and evidence.

## Findings resolved

- Unknown receipts could previously be redispatched by task-completion reconciliation, cancellation survivor repair and the propagation fallback. One exact-comment permit now belongs only to the committed original create or an explicit authorized edit. Automatic paths require retained recorded admission for the exact original target type/UUID and executing agent, and refusal cannot fall through to propagation. Tests assert committed completion, actual zero new tasks and unchanged unknown receipts; real deletion exercises survivor repair.
- An agent shared with a previously refused squad cannot transfer its admission to that squad after a leader change. Historical covering-task squad identity remains evidence, rather than future routing authority: a direct-agent deferred input on an existing squad task still receives its intended direct follow-up. The new covering-squad regression has a genuine failing run before the fix.
- Restricted task credentials could write an inert keyed comment but could not read its receipt. The route allowlist now permits only the exact receipt GET, with current capability/resource checks; the original comment remains nondispatchable.
- A reporter source issue and a historical receipt target may differ. Reading target project before locking target allowed a concurrent move to expose an out-of-scope receipt. The old-code regression actually committed that move and returned200. The fixed handler locks target first; the concurrent move returns55P03, and a later committed move makes recovery return403.
- Empty Idempotency-Key no longer silently selects the legacy path. Finalization failure cannot mutate the initial unknown target slice. Current credentials, actor/originator namespace, original source task and issue scope are rechecked; supplied human actor headers cannot impersonate a durable agent.
- Offline runtimes retain existing queued waiting semantics. Authorized admission records a nullable status/time observation from the exact returned task runtime, without exposing runtime IDs or private rejection metadata. Reads, status changes and agent rebinding do not rewrite it; unavailable/unusable targets retain explicit refusal. This is a sample after admission, not continuous liveness.
- Concurrent index cleanup registration was corrected. Migrations retain evidence, use separate concurrent indexes and refuse populated or busy rollback; operational downgrade requires maintenance across all steps.

## Verification

Independently rehashed all36 frozen source files: zero drift, including after sqlc regeneration. Independently ran gofmt over all frozen Go files and git diff --check: clean. A reviewer compile passed; final whole-backend build/vet also passed on the final source.

Every one of the13 final run records was checked against the actual raw log SHA-256 and independently parsed named test terminal inventory. All15 indexed failed raw logs also retain matching hashes. The205 selected handler top-level names exactly match both final runs, with no missing or unexpected test names.

| Final check | Ordinary | Race |
| --- | ---: | ---: |
| Handler admission, replay, legacy and capability coverage |342|342|
| Real authenticated router |20|20|
| Database migrations/rollback |5|5|

All above have0fail/0skip. Counts include subtests and are separate runs, not sums of independent coverage. Additional final checks: CLI98, skill/runtime guidance33, migration contract1 and runtime lookup scan1 all pass; whole-backend build/vet and sqlc generation exit0. Generated source remained identical to the frozen manifest. Exact source/log hashes and commands are in independent-review.json.

## Evidence interpretation

The initial completion-replay failure runs used a fixture that did not prove the completing task transitioned. They remain retained but are not called definitive behavioral-red proof. Corrected accepted tests assert the persisted completed state. The covering-squad and reporter-project race reds independently demonstrate their respective bugs. The initial zero-selected CLI run is not coverage. Earlier341/20 checks preceded the final reporter lock fix and are superseded by the03 runs. Environment, fixture, compile, guidance and migration failures remain preserved.

## Remaining limits and handoff

This is optional explicit-target delegation. Keyed no-mention comments may use implicit routing, but their empty receipt does not identify or guarantee implicit follow-up. Keyed attachments are refused; ordinary no-key creates, edits and quick actions remain legacy behavior. Explicit authorized edits are new mutations and never rewrite the original receipt. Unknown requires investigation and grants no automatic retry. Receipt admission is not execution/completion proof.

No browser/frontend/full-repository or performance claim follows; PostgreSQL ran as an amd64 image on an ARM64 host for functional tests. No real model budget was consumed or historical stop rule relaxed. AC43/E5/H3a/knowledge remain separate. Coordinator cleanup record reports the dedicated container stopped after0connections, with23databases and logs retained and0databases deleted. Local Lore commits, protected integration and archive remain the coordinator's work.
