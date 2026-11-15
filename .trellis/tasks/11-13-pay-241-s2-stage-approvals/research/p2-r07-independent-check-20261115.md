# PAY-241 S2 P2 r07 independent check

**NOT_ACCEPTABLE_R07_GREEN_R01.** The sealed 4,968-file r07 candidate has eight passing normal profiles and one failing canonical K06 profile. Nine race profiles remain NOT_RUN because execution stopped at that failure. No product defect was established by the failed assertion; the new test expects an internal response field that the public success DTO deliberately omits. A separate test-only owner is preparing a new candidate. This report does not accept LIVE activation, S2, or Web/Desktop runtime acceptance.

## Source and scope

The reviewer independently rehashed all 4,968 candidate files, finding exactly the three permitted handler/router differences and no drift in 4,965 protected files, with zero missing or extra paths. The complete manifest SHA256 is `18b3f4bdf484a6ae6174c6087ba4616bb90f570f2747fb22ec38eae2a96518bb`; the independent source audit is `26a5011854d7e19279a87aad57d84f8f1f8a55f8ba93d73cb90ebacd655d6501`. The R06 source and original false/503 RED remain preserved. The reviewer changed no product, test, migration, original report, configuration or LIVE path.

## Results

Every executed profile used ROOT's exact argv, original runner and separate admitted database, with one PAY suite at a time. Before and after each command, the reviewer rechecked all candidate bytes/modes, the private configuration and runner, exact database identity, ledger/catalog and connections. Go1.27 build events were kept separate from named test inventory. There were no retries, skipped tests, missing/extra top names, duplicate terminals, unfinished names or parser errors.

| Profile | Result | Top/named/leaf records |
| --- | --- | --- |
| p2_service_closure_normal_r01 | PASS | 7/11/9 |
| p2_handler_ready_normal_r01 | PASS | 1/1/1 |
| p2_router_ready_normal_r01 | PASS | 1/1/1 |
| p2_handler_missing_864_ledger_normal_r01 | PASS | 1/2/1 |
| p2_handler_submission_id_index_normal_r01 | PASS | 1/2/1 |
| p2_handler_execution_approval_index_normal_r01 | PASS | 1/2/1 |
| p2_handler_foundation_policy_index_normal_r01 | PASS | 1/2/1 |
| p2_handler_rollback_schema_normal_r01 | PASS | 1/1/1 |
| p2_service_closure_race_r01 | NOT_RUN | — |
| p2_handler_ready_race_r01 | NOT_RUN | — |
| p2_router_ready_race_r01 | NOT_RUN | — |
| p2_handler_missing_864_ledger_race_r01 | NOT_RUN | — |
| p2_handler_submission_id_index_race_r01 | NOT_RUN | — |
| p2_handler_execution_approval_index_race_r01 | NOT_RUN | — |
| p2_handler_foundation_policy_index_race_r01 | NOT_RUN | — |
| p2_handler_rollback_schema_race_r01 | NOT_RUN | — |
| p2_router_consumed_history_normal_r01 | FAIL | 1/1/1 |
| p2_router_consumed_history_race_r01 | NOT_RUN | — |

The four native negative controls each reached the production service with the original `*pgxpool.Tx`: exact864 removal or the single specified index removal, one Begin, actual catalog/ledger difference, HTTP503 `stage_schema_unavailable`, no revision, and complete transaction rollback. The missing-submission disable case returned HTTP200, retained exact original-key history, rejected fresh enable, and restored the original single-statement migration862 index. Its observed OID was **15980467 → 0 → 16217550**; before/after logical index definitions, constraints and ledger agreed. Handler and real-JWT router activation both returned actual HTTP200 while leaving roles, scope, candidates, dispatch and other covered business facts unchanged.

## Finding not fixed by this reviewer

**R07-K06-F01 — test public-response contract mismatch.** The canonical test failed at `server/cmd/server/collaboration_stage_routes_test.go:250` after its real lifecycle POST, workflow GET and completion-authorization GET each returned HTTP200. At line249 it required `originalWorkflow.Status == 200` after decoding the public success body into the internal `CollaborationWorkflowOperationReceipt`.

The actual public `collaborationOperationResponse.Status` has `json:"status,omitempty"` (`handler/collaboration_workflow.go:80`). `GetCollaborationOperation` only sets that field for a retained failure with status≥400 (lines410–417), so a successful HTTP200 body legitimately omits it. The internal int therefore defaults to zero. The test owner should retain HTTP200 and compare the exact public code/revision/confirmation/body, including the omitted-status contract. Fresh refusal still requires its explicit HTTP409 and body status409. Product serialization and completion authority must remain unchanged.

The original failed raw SHA256 is `7adaeadb1a7c7d6da4ee38951699d792a35cf5f60c2748f5dd89d6311f10484d`. A bounded READ ONLY snapshot of the retained failed database proves the issued revision1 → consumed revision2/statusdone, matching operation and actual external consumer, nonempty collaboration operation/submission, and a distinct unused authorization with no terminal. All database-side identities in the compound assertion agree. Original successful HTTP bodies were not separately captured; this report does not invent a capture or claim every local variable was logged. ROOT assigned `/root/p2_k06_repair` to change only the test on a new private source.

## Audit boundary

The original recorder's generic postflight asserted that all M1/quota tables were empty. It wrote the canonical after-snapshot and then raised because the existing fixture retains immutable M1 facts: evaluation1, delivery-policy2, authorizations2 and consumed-terminal1. This is a separate audit assertion, preserved alongside the actual Go exit1. Those records must not be deleted to obtain a green audit. New canonical acceptance should compare exact retained identities, counts and fingerprints, no extra consumption/audit rewrite, stable catalog and zero owned processes/connections. Initial freshness remains required.

## Verification and remaining work

- Formatting: pass; `gofmt -l` on the three changed files returned no output.
- Go static analysis/type checking: pass; `go vet ./internal/handler ./cmd/server` returned exit0 with no output in the credential-free metadata environment. No TypeScript changed in this candidate.
- Normal tests: eight profile PASS; original K06 FAIL. Race: nine profiles NOT_RUN.
- All nine actual before/after source and configuration audits found no drift. The failed canonical database remains healthy with zero other connections; its historical M1 records are retained.
- No API/CLI/Host/browser service was manually started, no external model/funds/production system was used, and no Git commit or source integration was performed.

The new test-only candidate requires its own exact source binding and fresh canonical normal/race databases. Unused original race profiles can proceed only after ROOT supplies new descriptors. The full S2/A01–A12, four actual Web/Desktop scenarios and parent24-case acceptance remain open. The JSON companion binds commands, raw results, database/source/configuration snapshots and the retained identity read.
