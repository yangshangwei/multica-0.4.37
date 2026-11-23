# ADR-05: Risk pagination under live changes

Status: approved after sequential Architect and Critic re-review, 2026-10-05. This supersedes only ADR-01's cross-request first-page reset if approved. Product scope, live formal membership, authorization, and per-request snapshot correctness remain unchanged.

## Evidence and decision drivers

The original A implementation was tested for ten minutes at each of 0/1/10 changes per second on 10,000 formal issues. At 1/s, reaching page two succeeded 155/362 times (42.8%), with P95 user wait 7.1 seconds; at 10/s it succeeded 0/599 times. All HTTP and count-consistency checks passed. See the verification task's performance-report.md and preserved original samples. Fast requests did not make pagination usable.

Drivers: users must be able to advance while others work; each rendered page and count must describe a current authorized formal set; avoid retaining private obsolete task content or adding a snapshot lifecycle when a simpler live-cursor contract suffices.

## Alternatives

- A (current): reset to the beginning on every project snapshot change. It tightly couples sequential requests but demonstrably starves later pages; rejected for measured usability.
- B: retain server snapshots of risk membership/content with expiry. It can reproduce the original full traversal, but requires storage, cleanup, stale-content UI and current permission/deletion revalidation. It is a viable P1 choice when users need to freeze the initial work list while traversing it, not merely a future history feature. It costs persistence/expiry/permission revalidation and requires a clear distinction between frozen membership and current task facts.
- C (proposed): retain the stable ID anchor across changes and recompute the current page/count in one RR transaction. No new dependency, table, background cleanup or cached protected rows. Concurrent changes can introduce rows before the anchor; this is explicitly disclosed with a restart action. Never present the concatenation of earlier pages as one current snapshot.

Choose C; focused re-review approved R1/R2 in pagination-architect-review-2.md and pagination-critic-review.md. B remains a fair alternative for frozen P1 work-list traversal. C is selected because the product can provide explicit live traversal semantics without adding snapshot storage; the cost is that changes before the anchor require a restart.

## Exact contract

1. Every request still checks current workspace/project membership and calculates the full formal set (`not_required`/`accepted` only), categories, current risk membership and overview in one RR transaction. Unknown/incomplete statistics continue to refuse precise risk results.
2. Validate cursor workspace/project/signal/version/last_id exactly as before. Client-provided snapshot_version must equal the cursor's version; mismatched cursors remain 400. A cursor is not an authorization token.
3. With no cursor, return the first page; if the card snapshot changed, return the new overview/count and `refreshed=true`.
4. With a valid cursor, select current risk IDs strictly greater than its last_id, even if the old snapshot differs. Set `refreshed=true` when it differs. A missing/deleted/moved/nonformal anchor is still a numeric ordering boundary; it need not remain a member. Every returned row is a current formal member.
5. `snapshot_version`, `overview`, `total`, and rows describe the new request's snapshot. next_cursor binds the new version and final returned ID. UI sends that version on the next request, replaces the displayed page, and never accumulates pages under a false shared snapshot claim.
6. `refreshed` means the data snapshot changed, not that the server moved back to page one. Show: data changed, continued after the previous position; restart to include new or re-entered matching items before that position. Keep this disclosure sticky for the traversal even when a later response has refreshed=false. Only a successful explicit from-start refresh establishes a new baseline and clears it; a failed refresh must not clear the disclosure. The restart action clears the cursor and fetches a fresh first page, not a stale React Query cache entry.
7. If the current suffix is empty but total>0, say there are no more results after this position, not that the project has no risk. Offer restart. total=0 may show no matching risk.
8. No page number or count of previously visited rows implies a complete traversal of a concurrently changing final dataset. Permission loss still clears protected content; source membership changes never expose historical removed rows.

## Verification before accepting the change

- Current A tests become behavioral regression evidence, not assertions to silently weaken: preserve the original first-page reset and starvation results in performance artifacts.
- DB tests: unchanged cursor exactness; changed snapshot advances past last_id; inserted-before/after-anchor rows, deleted anchor, removed/pending old members; empty suffix/positive total; foreign scope/signal and explicit version mismatch rejection; current total/page consistency.
- Core/UI: next request uses returned version; sticky refreshed-continuation warning across later refreshed=false pages, low-ID items re-entering risk, successful/failed explicit from-start refetch, and empty-suffix wording; page replacement only. Two-client updates retain current authorization.
- Repeat 0/1/10 changes/s for ten minutes each against the updated API. Use neutral ID-based advancement (first returned ID > requested last_id) rather than refreshed=false as the success metric. Under A both metrics were equivalent; under C refreshed=true is legitimate progress. Keep the original >=95% idle and >=50% active second-page success budgets and P95<=2s HTTP budget unchanged. Record waits/restarts and all samples.
- Re-run the 30-event two-client convergence check (<=5s) on the final Web build. No success claim until the new measurements pass.

## Consequences

C trades a frozen multi-page traversal for live, monotonic progress with explicit change disclosure. New items before the anchor require restart. This is acceptable for current risk triage and must be stated in the UI and API contract. The initial card and each page remain independently exact; immutable published project progress snapshots are unaffected.
