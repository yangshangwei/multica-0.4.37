# Research: bounded checkpoint retention

Date: 2026-10-31. Baseline: `4c833e737`. Repository inspection only; no implementation or tests run. This document consolidates the checkpoint research agent's findings and the coordinator's source inspection after the agent hit a rate limit.

## Source facts

- `server/internal/daemon/collaborationrun/host_recording.go:21`: messages live in memory. `text` uses `redact.Text`, strips NUL and masks known local paths. `message` records assistant text/tool calls/tool results, not reasoning events or provider session files.
- `host_recording.go:56`: recording is bounded to 256 messages / 512 KiB of bodies, with a 64 KiB per-message limit; overflow sets partial.
- `host_recording.go:103`: `execute` drains provider channels and handles cancellation. Network uploads cannot block this loop or delay `Lifecycle.Stop`.
- `host_recording.go:203`: all messages are attached at final outcome. An absent business outcome yields no recording upload. Failure body can be `result.Error`, not a final answer.
- `server/pkg/protocol/collaboration_record.go`: owns recorded-message types and validators. Reuse it rather than introducing another transcript parser.
- `server/internal/service/collaboration_runtime_capabilities.go:421`: outcome writes require exact issued capability/identity/attempt and current authority. `acknowledgeRecordedRuntimeOutcome` permits content-free identical replay after revoke; new content remains gated.
- `server/internal/service/collaboration_squad.go:84`: `lockRuntimeBusiness` checks current authority and reservation. Checkpoint writes need the same transaction/lock ordering as stop/revoke.
- `server/pkg/db/queries/collaboration_records.sql`: records are source-owned and read through issue authorization. `MarkCollaborationRunRecordingIncomplete` only updates unsettled runs, so late seals need a deliberate monotonic rule.
- `(run_id, producer_event_id)` already uniquely identifies events. Unknown internal kinds currently break public record projection; query/projection changes must accompany new event kinds.

## Minimal contract recommendation

1. Append immutable `record_checkpoint` events under exact run/attempt identity and stable producer keys. Chunks contain contiguous message ranges and typed, redacted content. Server independently validates/redacts; never trust a caller's redacted flag.
2. Preserve the existing run-wide ceiling: 256 messages / 512 KiB decoded bodies, including fallback launches. Initial engineering defaults: 16 messages / 64 KiB bodies per chunk; at most 256 chunks; flush every 5 seconds or on chunk fullness/completion. A full 64 KiB message fits alone. These bounds do not promise that unflushed output survives interruption.
3. One bounded sender queue/in-flight request; provider message drain and cancellation remain responsive during network failure. Keep uncertain payload/key in the existing host ledger and advance the acknowledged watermark only on receipt. On the first irrecoverable dropped chunk, close message appends for that stream and mark partial; do not send larger ordinals across the gap or describe them as a retained prefix. Already queued contiguous chunks may still be acknowledged. Recovery cannot send business text through termination credentials.
4. Exact accepted retries can receive a content-free ACK after revoke; changed body/key/attempt or new content fails. Transaction order determines whether a racing checkpoint was admitted before authority loss.
5. A typed metadata-only recording seal can report last observed/ACKed offsets and an enumerated reason, never arbitrary text/files/paths/results. Original termination identity must validate; metadata is monotonic and cannot weaken proof or capacity barriers. Without a seal, report a known retained range and unknown tail.
6. New writers identify recording stream/message offsets. Project checkpoint prefixes and an outcome suffix without duplicate text, using immutable stream/offset item identity. Validate the final payload against accepted checkpoints; reject conflicts rather than replace history. Old outcome-only writers keep their path.
7. Prefer existing event storage and indexes. Any necessary new index must have its own concurrent migration. Legacy runs explicitly lack checkpoint completeness.

## Implementation and verification locations

- `server/pkg/protocol/collaboration_record.go`: checkpoint/seal types and malformed/budget/offset tests.
- Runtime service/handler and `client_host.go`: authority, exact retry, stop/revoke/author departure lock ordering, metadata-only seal and original-identity tests.
- `host_recording.go`, `host_runner.go`, `host_ledger.go`: fake-provider tests for periodic flush, lost ACK/restart, partial tail, backpressure, responsive stop and final delivery.
- `collaboration_records.sql`, `collaboration_records.go`: stable stream IDs, prefix/suffix validation, fixed-watermark paging, internal-event filtering, completeness and no duplicate display.
- Reuse fixtures behind `TestCollaborationRuntimeMemoryRejectsStoppedCancelledAndRevokedRuns`, `TestCollaborationHostStopRejectsFallbackAndAcceptsOriginalCleanup` and `TestCollaborationHostIntegrationStoppedHistoryUsesOriginalIdentity`; those tests do not themselves prove the new checkpoint behavior.

## Limits

No runtime verification was performed. The design does not guarantee retention of unflushed output or detection of every arbitrary secret. Local ledger/provider files have a separate retention scope. Missing results cannot be reconstructed into a final answer. Detailed sender/ledger recovery requires implementation-time fixture validation.
