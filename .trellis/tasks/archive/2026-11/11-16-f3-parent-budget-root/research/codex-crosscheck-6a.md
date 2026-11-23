# Independent cross-check of step 6a usage ingress

Reviewed in `/Volumes/artisan/code/2026/mc-f3-completion-20261120` on 2026-11-20 by the management API implementer. Read-only review of sibling-owned implementation; no usage files changed.

## Verdict

No confirmed blocking product findings in the reviewed slice. This is a source and test review; final integrated normal/race runs remain the leader’s evidence.

## Contract checks

- `issue_budget_usage_decode.go` matches design 11.5: exact-case field tags, duplicate/missing/null/type rejection before unmarshal, top-level raw UTF-8 validation, decoded Unicode Cc rejection, trailing-content rejection, canonical int64 decimal strings and lowercase invocation UUIDs, required arrays, 32 invocation/256 item/256 KiB bounds, unique invocation/model identities, positive optional cost, RFC3339 timestamps, and empty usage for unknown outcomes. No currency, price, overlap, sequence, correction or proof controls accepted.
- `issue_budget_usage_normalize.go` matches 11.6–11.8: production evidence is only claude/1 fresh own usage, no reported-cost proof. Unproven overlap is quarantined even when a cost/snapshot exists. Supplied cost is never re-estimated. Model-based estimates freeze exact rate snapshots and rate-derived versions. Missing/invalid/overflowing price conversion yields absent cost. Invocation/model identities are scoped to the retained attempt. Empty and unknown calls make distinct durable gaps. No production settlement, correction or release.
- `issue_budget_usage.go` matches 11.4 and 11.9: independent READ COMMITTED transaction, only root lock, original-key digest and prior-batch check before off/unavailable checks, one atomic ledger application and operation row, publication only after confirmed commit. Original receipt replay works after off/corruption; second batch key conflicts; query is unlocked and returns unknown for pending absence. Public receipts contain no root, totals, amount or revision.
- The append extension retains explicit attempt provenance for unreserved usage and gap entries, verifies conflicting attempt/reservation IDs before writes, and persists the complete envelope and frozen snapshot. Valid year-one timestamp values are not dropped as absent.
- `daemon_budget_usage.go` follows the exact 11.3 authority order: UUID → retained attempt → workspace → runtime → bounded body/decode → immutable protocol check → transaction. Deleted task/issue rows are never needed. Reads distinguish missing attempt/runtime from backend failure. Existing daemon helpers and DaemonAuth remain the authority boundary.

## Meaningful test evidence inspected

- Decoder frozen fixtures, required-field/null matrices, scalar and aggregate bounds; independent fixed intent and price-version digests.
- Production evidence outcomes and injected reported-cost path; separate overlap/cost axes; largest legal stored entry.
- DB original-key replay/conflicts/off/unavailable, partial and overflow atomicity, retained late usage after terminal/deleted/superseded states, frozen prices/envelope, unexpected first-delivery entry replay rollback.
- Concurrent duplicate/finalize/switch commits, pending query before commit, postcommit publication and unchanged projections.
- HTTP authorization-before-body, transient runtime read failure, task credential refusal through DaemonAuth, late hold and revoked workspace, legacy task_usage independence.

Reviewed run records: `cx6a-svc-03` 91 pass/0 fail/0 skip and `cx6a-hdl-02` 10 pass/0 fail/0 skip. Two newer HTTP cases and expanded decoder cases still require the final run; do not substitute these older inventories for it.

## Limits retained

- The production evidence table intentionally yields no actual-cost A; supported fresh Claude input may yield E, all unproven scopes remain unknown.
- Shared daemon access helpers retain their existing membership cache/managed-binding behavior, as explicitly required by 11.3. This review does not assert stronger revocation timing than those helpers provide.
- No provider call, installed agent execution, DB mutation, new dependency or broader refactor was performed for this review.
