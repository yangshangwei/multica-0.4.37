# #7 current-candidate evidence reconciliation

Candidate: `85ef5d20578f63c7c4f879f035acbd414e80f306` in `mc-collaboration-closure-v12-20261103`. Verdict: **conditional; no reopened audit defect, but the current full gate and current CCG1/CCG2 observations remain required before closure.** This is a pre-closure source/evidence review, not the final acceptance.

All 21 original dispositions, seven independent review reports and five follow-up review reports retain matching evidence hashes. The disposition count is 19 repaired defects, one terminology repair (L13), and one not-reproduced/test-covered claim (L7). The current PRD already has the accepted L7 exception; the old readiness report's pending clarification is obsolete.

The companion `source-evidence.json` reconstructs the retained final639 race output: 20,852 named pass events, zero fail, 22 explicit skips, 82 packages including nine compile-only packages, with a distinct database per package. All executed packages used `-race`. Skip names and their original reason lines are retained. These are historical passes inside a failed whole-check attempt, not a new candidate pass.

The three exact L7 terminals are present and passed: `TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated`, `TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit`, and `TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback`. Their source files and the launch/daemon implementation are unchanged from final64. No L7 red baseline or product repair is invented, and no additional standalone L7 run is required.

## AC1–AC9

| AC | Current conclusion | Remaining condition |
| --- | --- | --- |
| AC1 | All 21 dispositions and original evidence remain verified; L7 wording reconciled | Current named result inventory is part of AC8 |
| AC2 | Safe OFF/nil descriptor and unsafe-selector code/tests unchanged; retained race passes | No duplicate lane rerun |
| AC3 | Complete-history/deep-exposure queries and three concurrent indexes unchanged | Keep O(N) history cost and no production-scale planner benchmark qualification |
| AC4 | Transient requeue and bounded 1.5-second retry code/tests unchanged | New Host policy/locking interaction needs current CCG1 normal/race; exhaustion may still be terminal busy |
| AC5 | Workdir rebind, original fence and stale refusal code/tests unchanged | No duplicate H2 standalone run |
| AC6 | Original input/recovery/erase semantics retained; panel now opens enclosing details for recovery, with updated test/spec | Current core/views lint/typecheck/Vitest plus required Web/macOS knowledge cases |
| AC7 | 761–763 up/down bytes unchanged; actual SQLC exit 0 at 2026-11-03 09:37:12 UTC generated 130 files with zero drift | SQLC run's server tree equals this candidate; retain older failed generation and migration runs |
| AC8 | Prior race and composite evidence independently reconstructed | One actual current full make-check pass, final identity audit, complete named/skip inventory, and post-full gates below |
| AC9 | Knowledge server contracts retained; recovery disclosure spec/test synchronized; built-in guidance extended for new delivery/policy | Current full template/static checks remain in the final gate |

## What can and cannot be rebound

The knowledge repair implementation, protocol, daemon, realtime and source SQL remain unchanged. The L13 `knowledge` locale subtree is byte-equivalent as parsed JSON in both locales. Core knowledge actions/cache/lease/recovery/query files remain unchanged. The API client's new delivery-receipt branch is outside its separate `fetchCollaborationKnowledge` path. These findings retain the original narrow audit/race evidence without replaying every lane.

The shared knowledge panel differs only by an attention effect that opens ancestor `details` elements; the test now verifies GET-only remount recovery through the enclosing disclosure. The original business assertions remain. The final current TS/UI gate covers the composed page; the old entire test-file hash is not treated as current.

**CCG1 and CCG2 cannot be mechanically rebound to this candidate.** Their principal harness/test files are unchanged, but exercised server dependencies changed: runtime admission, queue/batch/Resume/mention admission now check task AI policy; target input authorization additionally locks `issue_collaboration`; migration 764 supplies `ai_policy DEFAULT 'assisted'`, and generated source/access queries read it. Assisted default preserves intended behavior but does not prove the changed transaction graph. The new preceding built-in delivery/policy guidance and current detail composition also differ. Existing E6 OFF/ON deployments, binaries, configuration and browser observations retain only their original source scope.

Original ordinary composites remain 27 named events per normal/race run; Host composites remain 17 per run, including CCG1. All four raw hashes/counts were rechecked. These numbers must not be added into new independent cases, and CCG1 must not be double-counted.

## Minimum post-full-check verification

Run serially after the sole-owner full gate releases its resources. The following are focused gaps, not another complete-check launch.

1. On a fresh migrated database whose name starts with `multica_knowledge_host_composite_20261101_`, run `./cmd/server`, tag `knowledgeintegration`, exact test `TestCollaborationKnowledgeHostCompositeCorrectionReplacesActualInputAndWithdrawalRefusesNextLaunch`, `-count=1 -p=1 -parallel=2 -json -timeout=5m`, through `scripts/go-test-with-agent-cli-guard.sh` and the pinned Go. Require one named pass, zero skip/fail, two physical starts, exact corrected v2 input, and a 409 refusal with zero third start.
2. Repeat that exact test with `-race` on another fresh migrated owned database. Preserve both logs and source hashes. If a current whole Host-family binding is desired, the existing `^TestCollaborationKnowledgeHostComposite` family may replace these two invocations; it should not be run in addition merely for another count.
3. Run `e2e/collaboration-knowledge-deployed.spec.ts --workers=1 --retries=0 --reporter=line,json` once against a source-bound isolated OFF deployment and once against a separate recorded ON deployment. Explicit `COLLABORATION_KNOWLEDGE_EXPECTED=false/true` must match actual API startup configuration. Bind current test-owned Host daemon/provider/CLI, API commit/PID, database, Web origin/build and config before each run. Each lane must produce one pass and zero skips/retries with settled, reaped, sealed physical proof. OFF has zero contribution/events; ON has the exact contribution and one claim/preparation/authorization. Real Provider calls stay zero.

The old wrapper templates and environment references are listed in `review.json`. `knowledge-final-gates/run-gate.py` hardcodes final639; `run-deployed-probe.py` hardcodes E6 and old deployment paths. Preserve those originals and materialize new source-bound copies/permits; do not resume old workflow runs, reuse old failed databases, or mutate the live v13 environment. Parse current environment files as data rather than executing shell content.

After those passes and the actual full/final-state-audit result, a final independent review can close #7. It does not wait for #6's real-model/framework budget, production enablement, or parent platform/account/release gates. Historical failures and all original skip qualifications remain retained.

## Verification performed by this reviewer

Read-only Git/source comparisons, retained raw JSONL/count reconstruction, exact evidence/review hashes, SQLC log/source reconciliation. No product edits, task metadata edits, tests, database/service/browser operations or Provider calls. Only new files in this `knowledge-gate-rebind` directory were written.
