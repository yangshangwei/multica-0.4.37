# TC-13 post-QA Agent protection — implementation 2026-11-19

**Test-only source frozen; service normal/race pass, actual-router runtime pending.** No product defect or TC-13 acceptance is asserted here. Main and the independent checker own execution evidence, resource reconciliation, parent-case disposition and integration.

## Scope and source

Private source is `/Volumes/artisan/.multica-acceptance/pay241-tc13-20261119-_ugzeeke/source/server`, allocated from committed `50b50a7c9`. Only two new files were written:

| File | Lines | Frozen SHA-256 |
| --- | ---: | --- |
| `internal/service/collaboration_stage_qa_protection_test.go` | 172 | `090e46545b770b5bae4d958a8060a306f59821321f29c16cc47db16296bb8d3a` |
| `cmd/server/collaboration_stage_qa_protection_test.go` | 280 | `985acd0d76b54d3a487db5e6ff5ad30e2181354893edaec52ce12e46659e0e90` |

Existing fixtures, product code, SQL, migrations, shared tests and ROOT implementation files remain unchanged. ROOT writes are this report and its adjacent JSON. No dependency or runtime protocol was added; no installed Agent CLI, model account or external provider was invoked.

Main approved the finite layer split before authoring: two service tops cover QA return and QA approval using a real frozen passing report and settled run. One actual production-router top covers a human QA return, actual claim-minted generic/restricted task credentials and six precise override/revocation refusals. This avoids duplicating the full managed runtime/report fixture into the router package. It does not claim a fresh router QA approval or browser run.

## Service proof

The two tops are `TestStageQAReturnProtectedFromAgentOverride` and `TestStageQAApprovalProtectedFromAgentOverride`. Each has four named children:

- `task_principal/opposite_decision`
- `task_principal/exact_revocation`
- `human_with_agent_actor/opposite_decision`
- `human_with_agent_actor/exact_revocation`

Each top reuses `newStageRuntimeFixture`, `startStageReportRuntime`, actual report recording and `finishSquadRun`. The report contains a passing case and is bound to the real candidate/run/attempt; its outcome settles successfully. The fixture remains in QA. A real designated human issues either a reason-only return or an approval with the exact report, code-review dependency and bounded future expiry.

The test checks original signer, time, decision identity and current QA head, then forms current observed opposite-decision and exact-revocation requests. Machine principal and the same QA JWT marked AgentActor both require `forbidden`. Each refusal preserves 17 complete persisted relation snapshots, current stage/candidate/report/decision projection, and the original human operation receipt; the refused key must create no operation. Only evidence `CheckedAt` and signability `MaxExpiresAt` are read-time observations excluded from projection equality; persisted decision expiry and all business fields remain exact.

After all four refusals, a distinct legitimate human opposite decision must succeed. This verifies the attempted envelope was not stale, malformed or lacking usable approval evidence. The prior human decision row must remain byte-equivalent after that new human action.

## Actual production-router proof

`TestStageQAReturnProtectedThroughClaimedTaskRouter` uses `testServer.Config.Handler.ServeHTTP`, the package's actual `NewRouter` construction. Testutil builds source accounts/rows, while public endpoints create policy, roles, scope, code candidate, human code-review approval, QA stage and valid reason-only human QA return. No fake decision row is inserted.

Two owned ordinary tasks are then claimed through the actual daemon endpoint using the owner's JWT. One custom Agent receives an unrestricted task credential; one Code Reviewer v2 receives the frozen restricted capability. Both tokens must have `mat_` prefix and exact persisted task/Agent/user/workspace binding. The restricted capability also retains the expected resource/runtime/generation. Each same token must successfully GET its allowed ordinary issue before the refusal baseline.

The six named children combine `generic_claimed_task`, `restricted_claimed_task`, `human_with_agent_hint` with `opposite_decision` and `exact_revocation`. Generic task and JWT+Agent hint require the precise human-only `403 forbidden`; restricted task requires the exact TaskCapabilityGuard refusal. Each attempt preserves 17 complete workspace relation inventories, stage/evidence projection, original human receipt and zero new operation for the attempt key.

The router's opposite approval is a syntactically complete current candidate/dependency/expiry envelope **without a runtime report**. Its later human negative control must reach `409 stage_test_report_unusable`, distinguishing business validation from the Agent's earlier human-only refusal. Full opposite-approval admissibility belongs to the service proof above. No `mcb_` authentication failure is used as Agent authorization evidence, and JWT+hint remains a separate deny-only control.

## Checks and ownership

The implementer ran gofmt and compile-only `go test -c` through Main's closed no-DB `compile.json` and source recorder. Service compile R01 and router compile R01/R02 all exit 0 without source/configuration/tool drift. Router R01 compiled a draft negative-control error expectation; a source check corrected it to the existing `stage_test_report_unusable` before any runtime, followed by clean R02 compile. Both original compile records remain.

Main separately cloned service source to `service-frozen-r01` and executed `tc13-service-normal-r01` and `tc13-service-race-r01` against separate allocated databases. Both results contain **2 tops / 10 named PASS**, no failed/skipped/duplicate/unfinished record or drift. These are attributed Main executions, not tests run by this implementer. Main records the existing fixture's gate cleanup/restoration and retained internal organization/configuration/knowledge facts separately.

Router source is frozen at the hash above for Main's new normal/race runs. All exact command/result hashes and planned names are in the adjacent JSON. Final runtime outcome, cleanup and independent acceptance remain pending; no matrix, task status, archive or Git change was made by this lane.
