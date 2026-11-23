# Final bounded independent integration check

> Final takeover update (2026-11-20): the previous pending full Views r4 run completed with exit0,608 files/7441 tests. Original report text below records its earlier review cut. The213-file source manifest was independently verified unchanged before the documentation/comment closeout; all selected normal/race evidence is retained. Final shared-branch integration has separate evidence in `codex-takeover-20261120.md` and the final delivery record.


Reviewed on 2026-11-20 in `/Volumes/artisan/code/2026/mc-f3-completion-20261120`, against `86fea651cc5d09360327b6cf0f8357c1f360070d`. Reviewer authored7a, so this pass concentrated on the other implementation slices and their integration. Used prior independent6a/5c/daemon/caller reviews, then inspected final production callers, router order, retained SQL, migration registry/readiness, spec and actual test events. No DB command or product edit during this review. The only tracked edit was an explicitly authorized fix to two existing F2 test counters; ignored dependency links were later aligned for a valid Views run.

## Verdict

**Production integration: PASS; no unresolved functional blocker found in the reviewed paths.** The previous missing lifecycle-follow-up/triage task provenance and upstream membership-error findings are fixed and exercised. The evidence is sufficient for the implemented observe-only behavior, subject to the final acceptance conditions below. The exact matrix supplements and91-name before/after B54 proof are now inspected and passing. The only outstanding acceptance result in this review is the full Views r4 run; final source-map/commit/archive binding remains coordinator bookkeeping.

## Verified production paths

1. `router.go:1551–1555`: daemon usage POST and original-key GET are inside DaemonAuth, with no-store before credential validation. They reach retained attempt/workspace/runtime authority; no task/issue lookup is needed to accept late evidence.
2. `router.go:1645–1652`: no-store retains original deny-only actor hints before Auth; the narrow human allow-list precedes TaskCapabilityGuard. Reporter/Reviewer/custom task credentials therefore cannot enter a generic guard branch or get its different body. Ordinary JWT/PAT continue to the workspace gate.
3. `router.go:2233` and issue subroutes register the exact eight management endpoints. The workspace wrapper delegates unchanged on every non-budget path and translates only budget membership failures. Handler role/body/issue checks and transaction membership recheck preserve403-before-body and replay revocation. No new root-listing, attempt mutation, correction, settlement or enforcement route exists.
4. Both single and batch daemon claim handlers pass negotiated protocol into `FinalizeTaskClaim`, expose `budget_attempt_id` only after commit, and requeue a busy binding as an empty/omitted claim. The shared WS RPC capability source is preserved. Finalize writes budget last after existing runtime/task/capability/token/comment checks; its commit alone publishes appended state.
5. Ordinary issue create, lifecycle follow-up and triage creation all derive `VerifiedTaskID` from `auth.VerifiedTaskTokenFromContext`, scoped to the creation workspace. That private field survives through `IssueCreateParams` to `BindIssueBudgetOnCreate`; JSON origin and actor headers do not select attribution. The two formerly missed callers are now at `lifecycle_handoff_transaction.go:400` and `triage.go:609`, with named normal/race regressions.
6. The central create seam binds to the actual parent, then checks the verified task’s latest retained attempt across states. Ordinary attempts and same-root creations emit nothing; a quick-create mismatch goes to the execution root, never transfers spend, and is suppressed while off. Human, comment-manual, collaboration and stage callers retain their prior authority boundaries.
7. Ordinary and collaboration reparent paths call the saved-binding seam only on a changed parent. Delete marks member tombstones inside the issue deletion transaction. Child detachment preserves the original member. Workspace deletion classifies all six tables as retained audit and its regression checks row versions and locks unchanged. The static writer inventory remains present and ran.
8. `main.go:774` starts the separate lifecycle reconciler once, using the pool. Its root-lock-only transactions turn ended/deleted/superseded attempts into holds, never release; off/unreadable roots stay retained. Root-level failure is isolated from liveness health. Source scheduling selection is unchanged.
9. Management resume creates its permanent observation_off gap in the same policy/epoch transaction; original-key replay happens before that callback. Bindings persist while off. Claim, usage and lifecycle all check mode at their documented serialization points.
10.6a and6b remain connected by the frozen protocol: claim identity → durable invocation journal → immutable original batch → atomic scoped ledger/receipt → human projection. The actual router integration proves claim, production-evidence usage, replay, receipt and show; the daemon uses fake transport/process coverage rather than unauthorized provider execution.

## Schema, generated code and documentation

- F3 files now use869–885; shared checkout’s866–868 are collaboration migrations. No old866/867/868 F3 stems or obsolete ready names remain in production/server tests. The existing checkout migration files were not overwritten.
- Six new tables have no FK/cascade/implicit primary or unique indexes. All15 indexes have independent concurrent up/down statements and matching interruption-cleanup registrations. The final readiness migration validates table/name, uniqueness, key order, predicates, btree, validity/readiness/liveness and no INCLUDE/expression drift. Populated-budget rollback refuses before index removal and again before table drops.
- All34 F3 query declarations have generated methods (29 central +2 usage +2 paging +1 attribution); retained generated models hide raw rows via json:-. Renumbering changes no SQL column/type contract. Migration catalog/readiness/rollback tests passed normal and race.
- `.trellis/spec/server/issue-budgets.md` is indexed and describes authority/locking, integer units, unknown/unavailable projections, producer limits, old-daemon coverage, original-byte recovery, no settlement/enforcement, and S1 retention/lifecycle limits. Built-in skill and source map now point to actual handlers/tests and explicitly forbid agent use of the human CLI. Core schema/log redaction is covered by the caller review.
- Query-plan evidence at the30k-row fixture is bounded but not an index-only claim: attempt first/middle7.568/4.494ms,674 shared-hit blocks; observations13.502/11.867ms,1255 hits; operation lookup0.018ms/2 hits via the scoped unique index. Pages use top-N sort and still scan many rows. This manual-only path is a documented scaling follow-up, not proof of bounded database work at arbitrary size.

## Evidence inspected

`final-r1-source.json` contains211 changed/new files. Every listed file matched its recorded SHA-256 at this review; the authorized F2 test-only file is outside that original manifest and has its own rerun hash. Upcoming matrix supplement files require inclusion in the final manifest.

| Evidence | Normal | Race |
|---|---:|---:|
| Service selected F3+B54, fresh package DB |364 pass|364 pass|
| Handler selected F3+B54, fresh package DB |159 pass|159 pass|
| Router selected F3+B54, fresh package DB |54 pass|54 pass|
| Migration static/catalog package |4 pass|4 pass|
| Migration runner/readiness package |6 pass|6 pass|
| Pure budget |112 pass|112 pass|
| Selected CLI F3+B54 after counter fix |993 pass|993 pass|
| Middleware |8 pass|8 pass|
| Report snapshot pure package |28 pass|28 pass|
| Daemon |298 pass|298 pass|

Counts include named subtests. Accepted rows above have zero failure/skip and no missing expected canonical name. DB records are `final-r1-{normal,race}-*.summary.json`; pure/middleware/daemon are `final-r{1,2,3}-nodb-summary.json`; accepted CLI records are `cx7a-cli-final-summary.json`. The91 canonical Go B54 tests are file-derived in `acceptance-inventory.json`, not inferred from F3 prefixes. F2 exact GET/first-read authority, human capture, terminal withdrawal and completion previews are present in the logs.

Full Core:366 files/5172 tests passed. Views completion:3 files/22 tests passed. Core typecheck/lint and overall Go build/vet/gofmt/diffcheck were reported green by main; this pass independently confirmed diffcheck and inspected the test outputs. Full Views initial failures were environment-only duplicate React identities: Core and its hook libraries resolved the7b dependency store while Views/UI resolved the shared store. With main authorization, replaced only ignored candidate root/Core dependency links with local mirrors of the existing shared store; @multica links all resolve to candidate sources. All18 React-resolution probes now agree, and the previously failing chat-input suite passed49/0. The root-launched r2 run exposed two relative-CSS fixture reads requiring package cwd; it was interrupted and retained. The corrected package-cwd r3 run showed widespread5-second timeouts under unbounded workers; main requested stopping it. The full run with two workers and unchanged per-test timeouts is `views-tests-r4.txt`; until complete, no full-Views pass is claimed. All earlier failure logs remain retained.

## Retained failures and permitted test repair

- First CLI normal run failed because an ignored `.omx/state` directory had appeared in the immutable fixture directory. Main moved that runtime directory to scratch; all26 frozen JSON files were unchanged. The rerun passed. The guard was not weakened to ignore unexpected files.
- `final-r2-race-multica` reported real **test data races** on the request counters in `TestIssueReportSnapshotCaptureResponseLossNeverRetries` and `TestIssueReportSnapshotWithdrawResponseLossNeverRetries` (writes387/830, reads396/839 in the pre-fix file). No product race was in those stacks.
- Main authorized the minimal correction: `int`→`atomic.Int32`, increment→Add, read once with Load. The original assertions still require exactly one request, no output, an error and the original-request-key instruction. No production code or request behavior changed.
- `cx7a-f2-counter-race`:2/0/0; `cx7a-cli-final-normal` and `cx7a-cli-final-race`:993/0/0 each, every expected F3+B54 name present. Changed file SHA256 `3570cd2ec179dfa9e7ee4d74f5c26c3759b9065bf3a104ce31c529ef3be06d53`. Original failed run retained. Every command used the agent CLI guard and explicit unreachable DATABASE_URL; no DB package ran here.

## Acceptance conditions still requiring final evidence

- G1/G2/G3 supplemental assertions have now been independently inspected in `issue_budget_usage_acceptance_test.go`: `TestIssueBudgetUsageAcceptanceRetryKeepsActualAndLimit` uses real CreateRetryTask and40+25=65; `...UsageAndCancellationBothOrders` queues both root-lock orders and proves one hold/receipt withA30/H70; `...MovesAndParentDeletionKeepOriginalRoot` mutates/detaches then finalizes/reports into the retained root; `...ConcurrentReservationsPastLimit` proves exactL100/A90/twoR10 with only the second refusal and unchanged execution fields. Their normal and race runs (`cx6a-supp-{normal,race}-01.summary.json`) each passed all6 named tests/subtests with zero failures/skips on fresh separate DBs. G1/G2/G3 are closed for this reviewed source.
- Bind the completed row-by-row map, E1–E4 gates and all final runs to the final source manifest, including supplementary tests and the F2 counter fix. E01–E06/contractE5 are future enforcement, excluded.
- B54’s plan asks for before/after comparison as well as final pass. The91-name before/after proof is now inspected at `/tmp/f3-codex-5c/b54/canonical-before-after.json`; baseline source verification pins86fea651c across5039 files with zero mismatches. Baseline DB summaries are service9, handler50, router42 passes, with the remaining caller/pure names in baseline-pure-summary. Canonical baseline/current normal/current race maps contain only pass outcomes. Final Core and completion Views are green; only full Views r4 remains pending.
- This check covers the isolated feature branch. Merging it with the independently changing shared branch requires a clean overlap review and resulting source/build identity. A completed local feature commit can be described as such; it is not proof the shared branch was merged/tested.

## Accepted S1 limitations, not blockers

- Production evidence has no reported-cost row: actualA stays0; only supported fresh Claude usage contributes estimates. Unproven scopes remain unknown. Pure/injected-actual tests must not be presented as provider/billing proof.
- No production settlement/release, correction authority, hard cap, cost-based execution refusal, new UI, deployment, provider smoke or F4 feature. Closed/off/deleted states retain economic history and unknown holds.
- Seven-day journal retention, same-server account changes and persistence failure may lose telemetry. Lifecycle scans can repeatedly encounter unreadable/oversized roots and delay later ones. These are explicitly documented before S2.
- Issue loader can collapse a query error to404; shared helper behavior was retained by design. `InstallationMetadata` may reject an otherwise human request carrying invalid installation proof before budget handlers, using its own credential-error envelope; main accepted this as the upstream authentication exception analogous to401/CSRF. It still gets no-store, discloses no root, performs no budget write and produces a conservative CLI unknown outcome.

STATUS: implementation/integration review passed with no functional blocker; G1–G3 and91-name B54 before/after proof resolved; only full Views r4 acceptance result pending
