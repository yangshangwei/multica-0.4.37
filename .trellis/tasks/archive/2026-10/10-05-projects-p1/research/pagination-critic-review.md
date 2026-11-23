# ADR-05 focused Critic review

**Verdict: APPROVE.** Approve option C for implementation with the revised ADR's existing disclosure, correctness and verification obligations. No additional design amendment is required. This is approval of the plan, not a claim that C is implemented or has passed performance testing.

Scope: read-only review of `pagination-adr.md`, `pagination-architect-review.md`, the risk handler and UI/query options, PRD §§9/18 and the preserved active-pagination results. No product source changes, tests, new measurements, commits or delegation were performed.

## PRD and snapshot correctness

C does not violate the original PRD. PRD §9.2 requires overview totals, breakdown and drill-down conditions to share a calculation version, and explicitly permits refreshed results after tasks change. It does not require one frozen membership snapshot to survive a sequence of HTTP pages. PRD §9.1 says pagination must not change the statistical set; §18 requires same-version agreement and prohibits using a page count as the full total.

For request snapshot t, let R(t) be the entire authorized formal risk set and a the cursor anchor. C returns the first limit IDs from `{id in R(t): id > a}`, with `total = |R(t)|` and overview/version from that same RR transaction. A page is an explicitly identified slice of that current set. Earlier pages are not added to it or advertised as belonging to its version. Therefore counts remain exact even when previously unseen low-ID tasks enter risk before the anchor.

The existing handler already computes the complete risk IDs and overview in one transaction, refuses incomplete statistics, reads only formal same-project rows, checks the fetched page length, and returns the complete risk count. The proposed change only decouples `sort.Search(last_id)` from the version-change flag (`project_health.go:213–216`). Authorization, admission and row retrieval must remain on that path. Published immutable progress snapshots are unaffected.

## Tradeoff and strongest counterargument

The strongest argument for B is valid: a bounded frozen-membership session can let a user systematically review an initial P1 worklist without missing an existing low-ID task that later becomes risky. It need not retain every private task body, although it still needs lifetime/expiry and current access/deletion handling. The revised ADR now represents this current-purpose value fairly.

C cannot promise exhaustive or exactly-once review of a changing final risk set. Reaching an empty suffix can leave new or re-entered risk members before the anchor, and repeated restarts can involve repeated work. The revised sticky disclosure and explicit fresh restart make that limitation visible; they do not pretend to provide B's guarantee. Current PRD requirements are live risk visibility and exact per-response statistics, so this is an acceptable tradeoff without adding a snapshot lifecycle to P1. If exhaustive initial-worklist review later becomes a requirement, reconsider B.

A is not an adequate fallback for the measured workload: the preserved report shows 155/362 page-two successes at 1 change/s (42.8%) and 0/599 at 10 changes/s despite HTTP P95 below 100 ms. Optimizing request latency alone would not repair that reset policy. C removes the demonstrated reset-starvation mechanism by making a nonempty next page advance strictly beyond its supplied anchor.

## R1/R2 and implementation boundaries

Both architecture amendments are satisfied in the revised ADR:

- The continuation disclosure is sticky across later `refreshed=false` responses, includes existing tasks entering/re-entering risk before the anchor, and survives a failed restart. Only a successful fresh from-start request establishes a new baseline for that traversal.
- B is presented as a viable frozen P1 worklist alternative with explicit costs, rather than being dismissed as an unrelated history feature.

The current UI still needs the planned implementation: its warning follows only `query.data.refreshed`, its empty-list message means “no risks”, and its “previous” button only changes query keys. Because first-page results can already exist in the cache, clearing the cursor alone is insufficient. Preserve the ADR's explicit network refetch, pending/failure handling, page replacement and positive-total empty-suffix wording. An initial card-to-first-page version refresh should use ordinary refreshed wording; omission/continuation wording belongs to a traversal that has passed an anchor.

When implementing, synchronize `api-contract.md` §2, which currently says a version change returns a new first page, and the corresponding UI text/tests. This is part of delivering the approved contract change, not a request to expand the feature.

## Verification sufficiency

The proposed tests and fixed budgets are sufficient for this bounded change. Keep them as implementation acceptance checks; they do not have to pass before this plan can be approved.

- DB correctness remains **100%**, independently of performance success thresholds: ordered static traversal; every nonempty continuation strictly beyond last_id; current full total/page/overview agreement; before/after-anchor insertions and risk re-entry; missing/moved/nonformal anchors; authorization, incomplete-statistics and cursor scope/version failures.
- UI tests must include a later unchanged page after a changed one, failed and successful fresh restart with a populated first-page cache, positive-total empty suffix, returned-version propagation and replacement of the displayed page. No visited/page counter may imply all current risks were reviewed.
- Repeat the same ten-minute 0/1/10 changes/s scenarios and retain original A samples. Keep HTTP P95 ≤2s, zero errors, ≥95% idle and ≥50% active page-two success thresholds. Measure advancement by IDs, not `refreshed=false`; record legitimate terminal empty suffixes separately from failed advancement. An out-of-order/nonadvancing nonempty response is a correctness failure and cannot be averaged away by the ≥50% usability threshold.
- Repeat the 30-event two-client convergence check (≤5s) on the final Web build. Retain raw waits/restarts and distinguish unfinished waits from successful latency samples.

No remaining blocker was found in the revised plan. Implementation and these measurements remain the next steps.

## 架构第二轮后确认

2026-10-05 19:27:12 CST（Asia/Shanghai）：**架构第二轮后确认 APPROVE**。已顺序核对 [pagination-architect-review-2.md](pagination-architect-review-2.md) 的 APPROVE 结论；其 R1/R2 判断与本报告对同一修订版 [pagination-adr.md](pagination-adr.md) 的批准一致，无新增条件或未决设计问题。可以开始既定代码实施；原有实现、性能和跨端收敛验收要求保持。
