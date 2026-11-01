# F3 review correction handoff

Status: **implementation and evidence ready for independent re-review**. F3-R1/P1 and F3-R2/P2 remain reviewer-owned; this report does not close either finding. The original independent report and earlier scanner/hook evidence are unchanged.

## F3-R1: original credential freshness

`Client.authorizeKnowledge` binds the original authenticated token and subject when the socket is established. Immediately before writing a validated knowledge frame, it calls `Hub.validateKnowledgeCredential` through a bounded context. Legacy PAT checks use the existing authentication database handle directly, checking hash lookup, revocation, expiry and the original subject; no PATCache lookup or eviction result is used. Password mode repeats the existing `CheckPasswordToken` current-version validation. Legacy JWT cryptographic/subject verification, original connection checks, current human/account/workspace authority and agent-hint denial remain in place. Missing callback or required database capability drops knowledge.

`accountConnectionContext` now preserves A2's `PATTokenHash` in the private trusted principal. Raw credentials remain captured only in the connection's private closure and are never serialized/logged. This WebSocket protocol supports initial authentication followed by subscribe/unsubscribe/ping, not in-place credential replacement; a reconnect creates a new binding. The regression proves a replacement PAT works on the new socket after the old one fails freshness.

The actual socket regression leaves the account and workspace member active, holds an actual knowledge frame at the final-send gate, commits PostgreSQL revocation/expiry/subject change, then deliberately fills the actual Redis PATCache with the old subject after eviction. The old cached resolver still succeeds; knowledge is dropped and unrelated marker traffic retains its existing behavior. Valid PAT, legacy JWT, password JWT and password PAT controls pass. Password-version changes close the old session. Direct tests cover unavailable DB/validator, original-subject mismatch and missing freshness capability.

## F3-R2: canonical final envelope

`canonicalKnowledgeFrame` inspects every outer type entry before selecting the knowledge contract; duplicate/aliased type keys cannot switch interpretation through last-key wins. Knowledge accepts only canonical `type`, exact validated three-field `payload`, and optional nonzero canonical `<UUID>:target` event ID. Unknown/private fields, duplicate payload/type/event IDs, aliases, invalid UTF-8, malformed/trailing JSON and invalid IDs fail closed with a fixed diagnostic.

The write pump sends newly marshaled per-send bytes built from those validated values, without mutating a buffer shared with other subscribers. Valid unrelated envelopes retain their original bytes. Tests cover 13 adversarial parser cases, outer `source_id` and duplicate private payload with existing `event_id` at an actual socket, duplicate type, malformed JSON, canonical deleted-project/relay positives and shared-buffer preservation.

## Verification

- Before the fix, actual socket tests reproduced stale-PAT leakage for revocation and expiry. A separate isolated socket for each P2 case reproduced all four outer-envelope failures. See `red-sockets.jsonl`, `red-envelopes-isolated.jsonl`, and `red-source-hashes.json`. An initial missing test helper and a later test DBTX stub-interface compilation correction are retained in raw output; they are not counted as product regressions.
- **9 new/changed top-level tests / 32 owned test-and-subtest outcomes passed**, including their race runs. Full realtime tests passed **85 cases**; touched server scope/realtime integration passed **23 cases**. Exact owned/other cases, source-declared names, zero missing tests/skips/failures, and commands/exits are in `test-inventory.json` and `final-checks.json`.
- Full `go vet ./...`, `go build ./...`, formatting and diff checks passed. Final code/dependency SHA-256 values are in `source-manifest.json`.
- Exactly **6 correction-owned files**: three updated prior realtime files, new envelope implementation/test, and new server socket test. Full F3 integration now has 25 owned files. The other **19 original files are byte-identical** to the earlier reviewed manifest, preserving all scanner/transaction/hint producer work and its evidence.
- Database remains `multica_knowledge_invalidation_20261101_849a1c21` (schema 760), private env `/tmp/multica_knowledge_invalidation_20261101_849a1c21.env`. Socket tests used the already installed Redis 7 image in an isolated temporary container; no dependency was added and no other Redis database was flushed. See `environment.json` for lifecycle details.

No auth/middleware/router/A2/C2 files were changed by this correction. A2 owns PAT provenance and API checks. No commits, Provider calls, broader product acceptance or browser-consumption claims were made. Independent review is the remaining gate.
