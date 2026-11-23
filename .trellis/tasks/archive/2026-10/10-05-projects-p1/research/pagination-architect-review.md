# ADR-05 independent architecture review

Verdict: **REVISE — retain option C, with the two bounded amendments below before approval.**

Scope: read-only risk-pagination review, independent of the mobile implementation. Reviewed 2026-10-05. No source edits, implementation tests or new performance measurements were performed.

## Evidence and product fit

A needs to change. The preserved performance run reached page two in **155/362 attempts (42.8%) at 1 change/s** and **0/599 at 10 changes/s**. At 1/s the page-two wait P95 was **7,078.8 ms**; at 10/s the window ended with an unresolved wait of **598,085.7 ms**. Active HTTP P95 was below 100 ms. This is traversal starvation, not evidence that optimizing request latency alone would solve it. Source: [performance-report.md](../../10-05-projects-p1-verification/performance-report.md), active-pagination tables.

C fits the original product contract if its live-traversal limitation remains visible. [Projects PRD §9.2](../../../../docs/plans/2026-10-04-work-management-prds/projects-prd.md) requires one calculation version for overview totals, breakdown and drill-down conditions, and a refresh indication when tasks change after the click. It does **not** require retaining one frozen result set across every subsequent HTTP page. PRD §18 requires exact same-version agreement and prohibits using first-page counts as a full total.

C preserves those obligations when the current full total, suffix membership, row contents, overview and version come from one authorized RR transaction, and the client replaces the displayed page. `total` continues to mean the **whole current risk set**, not the suffix size or visited count. Earlier pages must not be represented as members of that response's snapshot.

For an anchor A and a nonempty current suffix, selecting the first IDs strictly greater than A produces a strictly greater next anchor. A concurrent version change cannot reset that request to the first page. This removes the demonstrated starvation mechanism. It is not a measured latency guarantee or a guarantee that a continuously changing dataset can be exhausted. An empty current suffix is a legitimate end of the ordering, not a failure and not proof that every current risk has been reviewed.

## Required amendments

### R1. Retain the change disclosure for the traversal

ADR contract §6 must retain `changedDuringTraversal`, or an equivalent persistent disclosure, until a **successful explicit fresh first-page restart** or a project/signal change starting a new traversal. A later `refreshed=false` response must not clear it.

Counterexample: page one ends at ID 100 in V1; an existing issue with ID 50 becomes blocked; page two continues above 100 in V2 with `refreshed=true`; nothing changes again, so page three receives V2 with `refreshed=false`. ID 50 remains absent from the traversal. The current view only shows its warning under `query.data?.refreshed` (`project-risk-issues.tsx:27`), so that disclosure would disappear even though the omission remains relevant.

Wording must include **existing issues newly entering or re-entering risk before the anchor**, not only newly created issues. Preserve the warning/restart action on a positive-total, empty-suffix page. A failed restart cannot clear the warning or imply a fresh traversal. This is client view intent, not a second server-data store.

### R2. Give B a fair current-purpose comparison

Remove the categorical implication that B is useful only for a separately requested immutable-history feature. A bounded frozen-membership session is also a legitimate **P1 triage alternative** for reviewing each member of an initial worklist once. It can persist ordered IDs/version metadata without necessarily retaining every private body; current authorization/deletion checks and unavailable-item treatment still apply.

C wins **for the stated requirement** because the PRD permits refreshed live results, C removes observed reset starvation while preserving per-response correctness, and B adds lifetime/storage/expiry and initial-versus-current membership semantics that this contract does not require. B has real present value when exhaustive initial-worklist review is required. If that becomes an acceptance condition, C does not meet it; reconsider B rather than relabeling it as a historical-reporting feature.

## Feasibility in the current implementation

The server change is localized. `server/internal/handler/project_health.go:213–216` currently lets `refreshed` disable the `sort.Search` anchor calculation. C can keep `cursor.LastID` independently of that flag without altering the existing cursor scope/version validation or RR transaction. `projecthealth.Collect` sorts IDs (`collect.go:111`); `GetProjectHealthIssueRows` returns ID order. A deleted or moved anchor remains an ordering boundary and needs no historical lookup.

The response at `project_health.go:234` already carries the current total, overview and version. Next-cursor creation already binds the current version and final returned ID. Keep the incomplete-statistics refusal and within-transaction page-row count check. No new table, dependency, secondary health source or persistent snapshot is required for C.

The view already replaces pages and sends the returned version with next_cursor (`project-risk-issues.tsx:33`). It needs R1, accurate empty-suffix wording and the ADR's truly fresh restart. Its current “previous” action merely clears the cursor/change state; `projectRiskOptions` caches by cursor/version, so that alone does **not** guarantee a fresh request. Restart must revalidate a populated first-page cache and show pending/failure honestly. Label it “From start” or “Refresh from start”, not previous page.

## Strongest counterargument and real costs

A user may interpret reaching the end as having reviewed all current blocked tasks. C can miss a low-ID issue that becomes risky after the user passed it, and continued changes before the anchor may require repeated restarts. Stable ordering solves forward-progress starvation; it does not provide exhaustive, exactly-once review of a changing risk set. Honest wording reduces misunderstanding but cannot provide B's frozen-worklist guarantee.

B offers stable initial membership and reproducible traversal, potentially a lower cognitive burden for that workflow. It costs resource/lifetime management, expiry/restart behavior, and reconciliation of frozen membership with current access/deletion/risk state. Storing IDs limits content retention but does not remove those semantics. C costs persistent change disclosure, a fresh-restart flow, potential repeated review, and the absence of a completeness claim for a changing traversal. Neither choice by itself removes the existing per-page collection cost.

## Tests and acceptance evidence to retain

1. Preserve A's original starvation samples, fixed performance budgets and static full-set SQL/API audit. Do not retroactively reinterpret A's refreshed responses as success.
2. DB: exact full ordered traversal without changes; changed snapshots advance beyond last_id; new/re-entered members before and after the anchor; deleted/moved/nonformal anchors; excluded candidates stay excluded; positive-total empty suffix versus truly empty set; current page/total/overview agreement; foreign workspace/project/signal and explicit version mismatch rejection; authorization and incomplete-statistics failures.
3. UI: next uses the **returned** version, pages replace, change disclosure survives a later `refreshed=false`, positive-total empty suffix does not say “no project risks”, and explicit restart fetches even with a populated first-page cache. Successful restart clears traversal disclosure; failed restart does not. No page/visited count promises complete review of the current changing set.
4. Repeat 0/1/10 changes/s for at least ten minutes each with the same fixed success/HTTP budgets. Measure ID advancement on nonempty suffixes; distinguish terminal empty suffixes from failed advancement; retain raw samples/waits/restarts. This review is not a performance-pass claim.
5. Retain the 30-change cross-client convergence check on the final Web build, current authorization and T1 formal-set checks. Published immutable project-update snapshots remain outside this change.

After R1 and R2 are incorporated, I recommend **C** for implementation and sequential Critic review. No other P1 scope expansion is requested.
