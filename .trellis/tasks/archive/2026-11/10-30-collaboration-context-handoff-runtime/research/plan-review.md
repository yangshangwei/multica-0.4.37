# Research: Independent review of the handoff implementation plan

- Query: Review the PRD, design, execution plan and checkpoint/recall research for actionable contract gaps before implementation.
- Scope: internal; planning review only
- Date: 2026-10-31
- Reviewed: `prd.md`, `design.md`, `implement.md`, `research/checkpoints.md`, `research/recall.md`; targeted source checks for batch limits and actual launch lifecycle.

## Verdict

The revised plan is reviewable. Four material gaps found during this review were corrected in the planning artifacts, and legacy context behavior was made explicit. This verdict is not implementation approval, a product test result, a predecessor-gate pass, or a release recommendation. No product code was edited and no runtime tests or real-provider experiments were run.

## Actionable findings and verified planning resolutions

| ID | Finding and evidence | Resolution checked in revised plan |
| --- | --- | --- |
| P1-1 | A 256 KiB cap including all mandatory claim inputs would reject previously valid work. Existing `server/internal/service/collaboration_batches.go:19-20` permits 1024 inputs and `(2 MiB - 32 KiB)`; the previous design put those inputs under the new 256 KiB budget. | `design.md:70` now limits only added handoff material, references claim inputs without duplicating them, preserves existing input limits, and requires a near-limit regression. `implement.md:49` carries that test. |
| P1-2 | Reading a launch snapshot only before `Backend.Execute` cannot bind the true attempt for Git preparation or provider-internal retries. `host_runner.go:89-105` may initially authorize preparation; `BeforeLaunch` authorizes the actual provider attempt at `:277-299`; `host_recording.go:97` passes an already constructed prompt into `Execute`. | `design.md:76-82` freezes a logical prompt first, then binds its full actual prompt hash under the real provider permit before nonce burn/start. Git preparation is excluded; every internal retry binds independently; outer fresh-session fallback creates a fresh logical snapshot. `implement.md:50` names all these cases. |
| P1-3 | Dropping a sender chunk and later appending higher ordinals would make a single `retained_through` value claim a nonexistent continuous prefix. A `partial` flag alone does not identify the hole. | `design.md:37` now seals message append on the first unrecoverable loss, continues only the retained queued prefix, prohibits later offsets from pretending to fill the gap, and keeps independent authorized final-result delivery. `implement.md:32` includes this regression. |
| P1-4 | Storing only source IDs/version/hash cannot reproduce mutable server facts such as handler/status, input states and reader membership. A later live projection could differ while retaining the original snapshot identity. | `design.md:74` now freezes the non-body server projection and exact source/reader sets; purgeable text remains referenced. Original expected hash survives a purge, with changed availability separate from the historical expected payload. |
| P2-5 | A new 50-comment/budgeted default must not silently replace the old RuntimeContext 200-comment array; preserving only JSON field names would not preserve behavior. Current runtime query is `server/pkg/db/queries/collaboration_squad.sql:42-49`. | `design.md:94` now explicitly preserves old `/context` behavior. New bounded handoff/recall reads are explicit and do not duplicate the legacy large array in a new launch. |

## Narrow implementation contract checks

These checks make the revised design's existing guarantees precise; they require no feature-scope expansion.

1. **Revalidate source revision/hash as well as purge status at launch binding.** Ordinary comments have a current revision, while an arbitrary historical comment revision is not guaranteed reconstructible. A comment edited between logical snapshot creation and launch binding must not be silently reconstructed from its latest text. If the exact referenced content cannot be recovered, return an explicit stale/missing source and prevent that launch. A new logical snapshot may be built only as a new documented preparation action. The parent coordinator was notified to make this explicit.
2. **Use server-accepted checkpoint facts for the server retention watermark.** The daemon's ACKed offset is its local knowledge. A lost ACK does not erase the checkpoint already stored by the server, and a claimed ACK in a termination seal cannot invent stored messages. The server derives `retained_through` from contiguous accepted events; `observed_through` and the sender ACK watermark remain attributed reports. Test both accepted-but-ACK-lost and a seal reporting an offset not present in server records. The parent coordinator was notified to make this explicit.

## Reviewed boundaries that need to remain in implementation

- New business content still requires exact issued business capability, current admission/reservation and matching attempt; late identical ACK is content-free. Termination seals add only bounded metadata, never body/attachments/results or stronger proof/capacity facts.
- New internal handoff/wakeup events are excluded from the public record-item parser. Checkpoint messages use existing item kinds and stable stream/ordinal IDs; immutable fixed-watermark pagination cannot suddenly gain duplicate items when an outcome arrives.
- Ordinary comments remain data even when their author currently has SubmitCommand. Historical admission references do not grant permission to re-execute a command. Immutable delegation text comes from the retained source event rather than edited comment content.
- Oversized comments produce explicit referenced placeholders and revision-bound full-text chunks; page and full-text cursors have distinct completeness. Budget calculations include UTF-8 JSON escaping and metadata. Human tombstones and runtime visibility have different total filters.
- Legacy clients retain old defaults and item shapes; declared-but-malformed new support fails closed. New support does not alter consent/configuration digests. Rollback keeps readers capable of retained new events and never deletes history to satisfy an old binary.
- Snapshot creation, launch binding, process start, input delivery and model understanding are separate evidence claims. A bound but unstarted process remains unstarted. A provider error or generic historical result is not a proven final answer.
- The predecessor gate remains unresolved until its evidence is checked. `fix-closure/task.json` still records `environment_upstream_502` and `E1-E6_gated_until_E0_pass`; this review does not change that record. `implement.md:10` preserves the entry check, and later fake-provider tests cannot discharge real-provider or native-platform acceptance.

## Related specs and source checks

- `CLAUDE.md`: compatibility, transaction boundaries, testing evidence and no new dependencies.
- `.trellis/spec/server/collaboration-squads.md`: immutable readers, leader order, coalesced continuation and proof boundaries.
- `.trellis/spec/server/collaboration-host-execution.md`: actual launch lifecycle, old/new readers, memory and termination semantics.
- `server/internal/service/collaboration_batches.go:19-20`: existing accepted input limits.
- `server/internal/daemon/collaborationrun/host_runner.go:89-105`, `:277-319`: preparation vs provider permit and burn ordering.
- `server/internal/daemon/collaborationrun/host_recording.go:94-103`: immutable prompt is passed before lifecycle starts the physical process.
- `server/pkg/protocol/collaboration_record.go:78-91`: current recording contract; checkpoint-aware versioning must preserve legacy outcome-only writes.

## External references

None; the reviewed contracts are repository-owned and the review proposes no external dependency.

## Caveats / Not Found

- This was a bounded plan review, not a complete source audit or proof that the proposed algorithms are implemented correctly.
- SQL query shape, protocol DTOs, sender/ledger failure recovery and real database locking still need implementation and the named regression evidence.
- No claims are made about model comprehension, native Desktop delivery, production availability or upstream provider recovery.
- Only this task's `research/plan-review.md` was written; external `fix-closure/research/2026-10-31` files were not modified.

## Coordinator follow-up

Both narrow contract checks above were incorporated into `design.md` and `implement.md` after this review: launch binding explicitly checks source revision/hash and rejects unreconstructible stale content; server retention derives from committed checkpoints independently of the daemon's received ACKs. Task artifact/manifest validation and whitespace checks passed. Product implementation, product tests and predecessor acceptance remain unperformed by this planning session.
