# Server integration fixes

All edits are in `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`. No commit, task-state, main-workspace, rollout, migration or provider change was made. `owned-files.txt` lists the 16 source/test files touched by this lane; imported WIP is retained. See `before.json`, `check-source.json`, `check-source.diff`, `after.json` and `checks.json` for source and command evidence.

## Source identity

SSA1: runtime access now takes `EventSequence` and `InformationRequestID` from the locked session/issue aggregate. The active information request resolves its exact original `information_requested` event through its own `handoff_id`. Nested request/return restores the parent event, separately from the latest return handoff. An unlinked legacy request emits `event_seq="0"` with the explicit `missing` entry `information_request_event_unknown:<request UUID>`. Live runtime, human projection, new snapshot and frozen replay are tested through actual authorized service entry points. Existing admission is unchanged.

SSA3: a `handoff` source rejects nonempty `RunID` or `Version` with `invalid_handoff_source` before loading the note. A valid same-issue note/hash succeeds with canonical empty attributes; a foreign issue reference is still refused. This fixes attribution, not a new content authority.

SSA2 remains assigned to the independent cursor patch. This lane did not alter `parseHandoffCursor`.

## Material and wire limits

`protocol.CollaborationHandoffMaxBytes` remains 256 KiB of material. `CollaborationHandoffMaxJSONBytes` is 2 MiB of JSON wire bytes. `CollaborationHandoffSizes` computes wire as `len(json.Marshal(envelope)) + 1` for the HTTP newline, then computes material by subtracting only escape expansion from each `Sources[*].Content`. All source references, flags, metadata, quotes, JSON framing and the newline still consume material. Validation checks both budgets; the envelope decoder rejects oversized raw JSON, and the daemon snapshot HTTP reader uses the common wire limit.

A legal 64 KiB assignment can require almost 384 KiB when `<`, `&` or U+0001 is JSON-escaped. The old one-limit contract rejected that already admitted work. The new wire allowance preserves all admitted text without changing the old claim input budget. Optional source text is allocated deterministically within remaining material space: retain a UTF-8 prefix, the original hash/reference, and its own `truncated` marker. Never shorten the mandatory delegation. Metadata plus mandatory assignment that cannot fit fails closed. Snapshot framing is included before saving; rehydrating the sources applies the identical bound and reproduces the frozen bytes/hash.

Tests cover max-length `<`, `&`, U+0001 and Chinese assignments through database-backed snapshots; source hash and replay stability; precise prefix markers and source references; exact material boundary with metadata; raw wire overflow; actual handler JSON/newline byte agreement. Root owns the corresponding Core schema/spec updates and the separate daemon client/fake-provider budget test. The existing 1024-input fake-launch test remains part of the final gate.

## Seal transition

Moved the imported seal transition rule to `protocol.ValidateCollaborationRecordingSealAdvance(prior, next)`, reused by service reads/writes and available to the daemon. Each seal validates; first sequence is 1, next is exactly +1, maximum 32; full run identity, stream and reason remain fixed; acknowledged count and a known observed bound cannot decrease. A known bound cannot become unknown. Attempt/nonce are authenticated separately by the existing service gate. Exact original-key replay is checked separately, not treated as another advance.

The service derives retained bytes/message count from its committed record prefix, never the sender ACK. The 256-checkpoint/32-seal regression passed, including stale original seal replay after advancement, original-key conflict, end-of-limit refusal and termination-token denial at the checkpoint writer. The strict handler still rejects body/messages/unknown/duplicate fields on seals. No terminal proof, reservation, current-authority or token-purpose rule was expanded.

`make sqlc` used pinned v1.31.1 once. It generated the exact request-event query and corrected the imported recording query drift from 258 to 290 rows (256 checkpoints + 32 seals + one outcome + one overflow row). No unrelated generated files changed.

## Verification

Intentional red evidence: source test reported runtime watermark 0 versus locked 11 and accepted forged run/version; budget tests rejected escaped legal assignments and accepted oversized raw JSON. Final focused results: 14 service cases, 11 handler cases, 26 protocol cases; all passed, zero skips. Service and handler used the exclusive live database sequentially. Affected-package `go vet` and diff whitespace checks passed. Exact names/timestamps/commands are in `checks.json` and JSONL logs.

Full repository, native platforms and real providers were not run by this lane; root owns the final integrated gate. No claim of model understanding, product rollout or AC1–AC10 completion is made here.
