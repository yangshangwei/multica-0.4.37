# Final knowledge integration acceptance review — 2026-11-01

**Passed within the deterministic knowledge-integration scope. No outstanding confirmed product finding blocks this scope.** The original acceptance audit's A–D gaps are closed by direct regressions or the explicit layered proofs below. This verdict does not enable production or close real-model, wider platform, installer or account/quota gates.

The reviewed tree is `53a166258` plus the retained final source manifests. The reviewer read source/evidence, recomputed raw inventories, checked the Go dependency graph and four original native command records. Only this report and its JSON companion were written; no tests, database calls, Provider calls, product edits, commits or task-status changes were performed by the reviewer.

## Acceptance evidence

| Boundary | Independently checked result |
| --- | --- |
| Full default Go scope | Latest result per package from `rest` → `affected-rest` → `db-clean`: **82 packages**, 73 passing and 9 with no tests; **10,284 top-level passes / 20,557 pass events / 18,797 passing leaves**, zero failures. **21 explicit test skips** are OS/root, optional live/performance/export/catalog or preexisting documented gaps; Redis cases ran. |
| Frontend | **1,063 files / 13,060 tests**, lint and typecheck passed; all **3,408** recorded frontend source hashes still match. |
| sqlc/static | **129 generated files, zero drift**; final build and full vet passed, including the final test cleanup source. |
| Current-source fake-input composites | Ordinary **14 top-level / 27 events / 23 leaves** and Host **8 / 16 / 13**, under race, zero failures/skips, stable production sources including final guidance. |
| Approval/recovery/deletion matrix | Service **8 / 27 / 20**, handler **3 / 11 / 10**, each normal and race; **30 distinct leaves**, no failure/skip. |
| Preparation/writer/authority matrix | Handler **4 / 12 / 9**, service **2 / 4 / 3**, each normal and race; **12 distinct leaves**, no failure/skip. Later teardown-only changes are verified separately below. |
| UI/platform | The same **8 independent cases** (7 production Web, 1 native macOS Electron) pass on the combined UI snapshot, with no retry/skip. The prior **30-image / 18-group** visual verdict remains applicable to unchanged frontend source. |

Pass events include parent containers; reruns and overlapping suites are not added as extra independent cases. Exact names, command exits, hashes and skip reasons remain in the referenced inventories.

## Original gap closure

- **A:** Real approval versus actor disable, source memory purge/comment edit/delete, target member/role/lead change, withdrawal/correction and project deletion is exercised in both commit orders. PostgreSQL waits or bounded NOWAIT refusals establish the schedule. Parallel correction allocation retains the exact predecessor and contiguous versions. Actual preparation under held issue-create/description transactions and held approval versus unrelated create/association/triage/iteration writers are covered.
- **B:** Recovery rejects a different valid candidate paired with the original key, and real memory/command operation UUIDs. Results are content-free, with no new candidate/operation/hint. HTTP wiring and original-actor recovery remain separately covered.
- **C:** A genuinely eligible external source loses Read before disclosure in both orders; original-manifest replay is refused after revocation. Source authority changes between claim construction and finalization without leaking material. Real deleted comments suppress all human projections. The legacy absent-source fixture first proves the public source-delete guard still refuses deletion.
- **D:** A real service squad negotiates KnowledgeVersion 1 across root/member/continuation, preserving exact approved content, authors, delegation/readers, capacity and original-key identities. Two member comments plus proof form one continuation. Its process observations are explicitly synthetic; separate actual fake-member stdin/argv and real server/DB/HostExecutor composites cover physical launch. Public ordinary comment coalescing also checks every current author, including a non-originator author disabled before disclosure.

Human UI plus separate runtime composites is accepted as **layered deterministic proof**. No single UI-to-Provider journey on each platform, independent native target-only-admin journey or physical full-squad cross-product is claimed. Web UI6 and shared component tests cover target-only administration; native UI8 covers platform wiring and source/project/workspace flows.

## Corrections independently reviewed

The actual description/preparation test found an overly strong project metadata lock. `LockProjectForUpdate` now uses `NO KEY UPDATE` only for metadata edits; revision, lead/association `SHARE`, deletion and existing squad/resource create-update fences remain. Its independent review and 238 passing race leaves close the defect.

Six runtime reads now use the existing metrics lookup wrapper with the exact same transaction query, runtime ID, row and error. The receiver-only exposure helper adjustment preserves authorization/session behavior. All 13 reverse-dependent packages were identified independently; their required reruns passed. RR1/RR2 private diagnostic log findings remain independently closed.

The built-in guidance was condensed to its existing template limits. Mandatory workflow references preserve human-only decisions, source access, data-only recall, exact request keys, no ordinary fallback and no automatic skills. The template regression is green. The workspace deletion manifest now classifies candidate and invalidation history as retained, and target counters as removed after final hints; actual deletion retention tests pass. No production teardown semantics changed.

A synthetic historical target fixture now owns a distinct workspace instead of poisoning the shared handler target. Its ordered reproduction, knowledge race tests and full handler evidence verify the fix without weakening product guards.

## Reused database failures and final source binding

The five service failures on retained `e7059940` were explained before accepting the fresh run. Read-only timestamp reconstruction proves at least **12,720 scan transactions** were needed, exceeding a fixture's **10,000-step** global warmup. All 98 jobs pending at that test start subsequently completed. The other four failures came from orphan test-created collaboration/event rows at the old dispatcher queue head; actual production source deletion remains guarded.

The failed database was preserved unchanged. The whole database gate then passed serially on fresh migrated `multica_knowledge_final_clean_20261101_80a134db` with the task-owned Redis. A fresh fixture environment is a declared prerequisite; the fixed warmup is not claimed to cover arbitrarily accumulated protected history.

Two test files gained exact-source teardown cleanup while that full run was in flight. Therefore its raw **`source_unchanged=false` remains true evidence**: only `task_knowledge_logging_test.go` and `task_knowledge_preparation_boundary_test.go` changed, with no production or assertion change. An ordered old-fixture → dispatcher reproduction fails in all four downstream tests; corrected normal and race sequences pass **11 fixture leaves then 4 dispatcher tests**. Orphan pending events drop from 42 to zero; all 57 owned candidate records and protected invalidation history remain. Cleanup of historical test orphans was confined to the separate `7d95` database, with protected before/after hashes equal.

This explicit cleanup-only delta, final full vet and ordered proof close the test-source difference. All **4,437** current server source hashes match the final post-run/vet snapshots, and all 3,408 frontend hashes match. The final composites include the current production bytes. UI's later server delta is only the independently reviewed metrics attribution and equivalent embedded-guidance condensation; its frontend is unchanged.

## Remaining boundaries

Main owns committing/merging these exact reviewed bytes and synchronizing the task ledger. Any later product change requires a scoped recheck. The default knowledge switch remains false, and this verdict is not a release authorization.

Real Provider/model comprehension must bind its own new-runtime version. The original E2a trial 3 / remaining cap 22 series, Windows and wider native/installer combinations, and multi-account/quota provenance remain separate. The earlier default-database incident also retains its honest uncertainty: the later absence of reserved fixtures cannot prove earlier matching data was untouched. Successful isolated tests do not erase that incident.

Detailed hashes, correction diffs, command provenance and raw counts are in [the JSON companion](final-acceptance-review-2026-11-01.json). The original failed runs and earlier audit remain retained rather than relabeled as passing.
