# Research: Knowledge acceptance contracts and continuation plan

- Query: Reconcile the existing #6/#7 acceptance contracts and propose an ordered continuation without recreating tasks or replacing retained results.
- Scope: internal; contract and planning review only. Runtime v11 evidence and framework source/process audits are assigned to separate check agents.
- Date: 2026-11-03, read at approximately 09:16 UTC.
- Write ownership: this new research file only, explicitly granted by the coordinator. No existing task document, code, status, process, database, budget or workflow run was changed.

## Findings

### 1. The two completion decisions are independent

#7 may close after its own nine ACs and the later single-owner complete-check prerequisite are satisfied. Its PRD expressly excludes real Provider acceptance, enablement and release (`../11-01-collaboration-knowledge-audit-fixes/prd.md:40`). The current user instruction confirms that independence. Do not add #6 framework/model/installer/account gates to #7.

The #7 complete-check condition is stronger than a collection of partial passes: `advance-plan-2026-11-02.md:52` requires `make check` exit 0 and actual required knowledge results, with conditional skips identified. Current `.trellis/spec/server/verification-environment.md:71` additionally requires a final identity audit and complete named inventory:

```text
accepted = make_exit == 0 && final_identity_audit.status == pass && inventory_complete
```

The final identity audit checks the actual candidate, tracked bytes/modes, tools, workspace links/exports and CLI/Desktop artifacts against the reviewed run. Initial clearance or SSR success does not replace it. The only declared normalization is the exact permitted Next generated import rewrite. Root still separately establishes live resource ownership and API/database identity; the filesystem audit does not do that.

#6 retains the already merged deterministic implementation and G0/G1/G2 evidence in its original scope. `implement.md:180` explicitly keeps #6 in progress for independent real-model and release tracking. Neither the new framework compiling nor its future fake proof closes those gates.

### 2. #7 acceptance matrix to reconcile against the selected final source

| Contract | Final review requirement |
| --- | --- |
| AC1 | All 21 original audit claims remain mapped to disposition, code and independent review. Reproduced defects retain actual red/green evidence; L13 uses terminology/parity; L7 is not reproduced and test covered. |
| AC2 | Safe OFF ordinary claims keep nil descriptor/ordinary behavior; unsafe session selectors still refuse without silent argument removal. |
| AC3 | Long benign histories do not permanently refuse; deep exposure/corruption still fail closed. |
| AC4 | Transient claim failures requeue; namespace contention gets bounded whole-transaction retry. The accepted 1.5s exhaustion behavior remains terminal; do not promise indefinite retries or replay of business input. |
| AC5 | Unreachable previous workdir can bind a fresh generation only through the reviewed preparation/authorization/observation path. |
| AC6 | Inputs survive unchanged refresh, lease erasure and unrelated settlement; identity changes reset decisions; private server bodies still erase promptly. |
| AC7 | No FK, single-statement concurrent migration indexes, protected down behavior and no unexpected sqlc drift. |
| AC8 | Affected build/vet; full DB-backed package results with separate fresh databases; affected concurrency race evidence; core/views typecheck/lint/Vitest; the later shared whole-check/final-audit/inventory prerequisite. |
| AC9 | Changed behavior remains synchronized with specs and built-in skill/source-map documentation. |

The L7 exception is already in the current PRD (`prd.md:61`), so it is not a pending approval or missing red-test requirement. Retain the three final named regressions: `TestKnowledgeClaudeStartedCallbackFailureStillReportsTerminated`, `TestKnowledgeStartedObservationRefusalStillConfirmsProcessExit`, and `TestHostLifecycleStartedFailureStillWaitsAndBlocksFallback`.

The retained closure-readiness report also reconciles CCG1 actual approval/input/correction/withdrawal and CCG2 deployed OFF/ON evidence. Rebind unchanged owning bytes; if those bytes changed, run the affected required gate on the new candidate. Do not rerun unchanged passing evidence just to create another review, and do not turn an old failed full run into a pass.

### 3. Six-file framework ownership and complete fake gate

`knowledge-framework-plan-2026-11-03.md:15` limits the implementation to these files in `/Volumes/artisan/code/2026/mc-knowledge-framework-20261103`:

| File | Responsibility |
| --- | --- |
| `server/cmd/server/rk1k_fixture_test.go` | Knowledge-enabled router and true producer fixtures |
| `server/cmd/server/rk1k_provider_test.go` | Knowledge wrapper and actual fake/stand-in process capture |
| `server/cmd/server/rk1k_scenarios_test.go` | Finite K1/K2/K3 schedules |
| `server/cmd/server/rk1k_evidence_test.go` | Durable evidence and negative controls |
| `server/cmd/server/rk1k_entry_test.go` | Guarded fake/real entry admission |
| `server/internal/daemon/rk1k_driver_agentintegration.go` | Thin ordinary claim/run/report bridge using the production daemon |

All use `agentintegration && !windows`. Preserve the original 17 RK1 files byte-for-byte, existing registrations/ledgers, production policy and other sessions' dirty edits. No new dependencies. A demonstrated need outside these six files requires an explicit expanded owner boundary from root; the implementer must not silently broaden scope.

Required schedules (`knowledge-framework-plan-2026-11-03.md:34`):

- K1: four independent consumers for pending, approved, corrected and withdrawn states. Absence cases cannot inherit previous lawful output.
- K2: two native turns with distinct alpha/beta exposure, then a third claim/preparation frozen before revocation; the original authorization/replay refuses with zero third process. A fresh claim made after revocation is a separate lawful control.
- K3: fixed order of issue, mention comment, private Chat, AI quick-create, manual run-only autopilot, collaboration root, actual member and actual continuation. Eight measured samples, with member/continuation fake seed starts counted separately.
- E0b remains a separate ordinary prerequisite. Native IDs/ACKs, real task/run identities, task-token CLI reads, quick-create linking/fences, original request keys, all physical starts, evidence faults and stop-on-incomplete controls must be proven.

The current plan's future command names are `TestRK1KnowledgeFakeProof`, `TestRK1KnowledgeEntryGuard`, `TestRK1KnowledgeWrapperGuard`, and `TestRK1KnowledgeEvidenceFaults`; the implementation audit must confirm their actual names and inventory before execution. Ordinary and `-race` acceptance must cover the complete accepted fake matrix and required controls, not only K1, compilation, or selected green leaves.

Each actual DB-backed invocation receives a newly allocated/migrated `multica_rk1k_*` database. `cmd/server` TestMain runs before body skips; `-run` is not isolation. Root records expected DB identity, private configuration and resource budget first, and gives each child an exact package/test/database permit. Do not use full-check API/Web/Redis/renderer or retained failed databases. Database allocation does not remove the shared PostgreSQL connection limit.

`knowledge-framework-plan-2026-11-03.md:45` requires a separate native `trellis-check` review of all six files and the complete contract. A review of only the four foundation files cannot be promoted to that result. Bind the final committed framework/product source and artifacts after integration; preserve earlier source hashes and failed K1 evidence separately.

### 4. Ordered continuation

1. Root consumes the two independent live audits, corrects the current snapshot, and contacts the recorded complete-check owner before any service/test action. Preserve v11 and earlier results. Do not resume an old workflow across sessions.
2. Prioritize remaining v11 failures. Assign each verified failure a module and narrow worktree owner; use native `trellis-implement` for the smallest reproduced repair and a different `trellis-check` for review. Reuse any already completed same-round results. The single complete-check owner decides the next numbered candidate/run after the repair and source binding are ready.
3. Collect one accepted complete-check result plus the final identity audit, required knowledge names and explicit skip inventory. Reconcile AC1–AC9 and unchanged CCG1/CCG2 bindings with an independent full-scope #7 review. Root may then complete/archive #7 without waiting for #6's framework/model/release work.
4. For #6, retain the failed framework K1 run and independently bind the later correction. Under a coordinated resource window, execute the complete ordinary and race fake matrix in separate fresh databases, address new failures, then independently review all six final files. Preserve exact 4/2/8 measured counters and separate seed counters.
5. Root integrates only owned reviewed changes, records the final source/artifact binding, and updates the knowledge real registration as prepared but not runnable. Knowledge remains allocated 0 / consumed 0 and default off. The proposed 14 knowledge calls and one separate E0b call require their own later allocation/window; do not consume original E-series remaining 14 or RK1 runtime 25.
6. Update existing task records and the relevant acceptance ledger with actual results, source, timestamps, commands, names/counts/skips, raw evidence hashes and remaining gates. Preserve historical statements as dated history. Root owns Lore commits/spec updates/finish flow; this research file performs none of those operations.

### 5. Records that need current-state correction

| Record | Correction required after runtime audits |
| --- | --- |
| Both `progress.md` headers | Replace v10-as-current and contradictory running/prepared wording with the latest proven result; retain v6/v9/v10 as history. State #7 versus #6 remaining gates separately. |
| Both `task.json` files | Update `notes`, `progress_phase`, `next_action`, current candidate/run pointers, final result/owner timestamps, and framework status from actual evidence. Preserve historical v10 fields; do not change status to completed before its gate. |
| #7 `advance-plan-2026-11-02.md` | Current owner/candidate/launcher references are older than task metadata. Add a dated current entry and final identity audit requirement; keep old execution log and failure counts intact. |
| #6 `knowledge-framework-plan-2026-11-03.md` | Check only steps with full matching evidence. Its tail still describes v10 prepared/not run and the former owner; its original base/source remains historical, not silently replaced. |
| #6 `real-model-acceptance-plan.md` | Current status still references v6 and says framework unimplemented. Update preparation/testing stage after proof while preserving no-budget/no-call/non-runnable state. |
| #6 historical progress section | The old statement that RK1 runtime has cap 0 is superseded by the separate runtime 25 / knowledge 0 budget table. Label it historical rather than changing knowledge allocation. |
| #7 old design/progress follow-ups | M5/L12 current-page limitation and H2/L8/doc follow-ups are described as unresolved in historical sections but reconciled in `knowledge-closure-readiness.md`. Do not open duplicate repair work from those stale paragraphs. |

## Files and related specs

Read in full: #6 `prd.md`, `design.md`, `implement.md`, `progress.md`, `task.json`, `knowledge-framework-plan-2026-11-03.md`, `real-model-acceptance-plan.md`; #7 `prd.md`, `design.md`, `implement.md`, `task.json`, `advance-plan-2026-11-02.md`. The inherited main-session context also contains #7 progress. Additional closure-readiness/entry-matrix material was used only for contract interpretation; source/runtime facts are assigned elsewhere.

Relevant specs: `.trellis/spec/server/database-test-isolation.md`, `.trellis/spec/server/verification-environment.md`, server/core/views guideline indexes, and `.trellis/workflow.md:480` through its implementation/check dispatch and full-scope final-review rules. Native context injection is preferred; child loading is the fallback. Root dispatches implement/check separately; children do not recursively orchestrate.

The inspected product default is explicit: `server/cmd/server/router.go:431` enables knowledge only when `MULTICA_KNOWLEDGE_CONTEXT_ENABLED == "true"`; `server/internal/service/task_knowledge.go:376` still combines that flag with invocation support. This source read verifies the default condition, not any running deployment's configuration.

External references: none. No new SDK, dependency, API or model choice was proposed.

## Caveats / Not Found

- This is a contract/plan review, not a code review or test acceptance. No suite, provider, process or database operation was executed.
- v11 final counts, latest source identity, current process owner, framework K1 failure/correction and current complete normal/race results must come from the separately assigned actual-evidence audits. This file intentionally does not duplicate or preempt them.
- Old design K1–K5 findings and new real RK1-K1–K3 experiments reuse labels; keep them qualified. Historical deterministic G2 success does not prove the new framework or real model.
- Tests and complete checks already passed on unchanged bytes should be reused with exact binding. A source change, new failure or unresolved coverage gap justifies fresh verification; partial or skipped results never replace the complete accepted scope.
