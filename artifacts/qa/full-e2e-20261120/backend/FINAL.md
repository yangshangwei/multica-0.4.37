# Backend verification — final evidence

The backend lane is complete for the executed scope. The original 84 package commands have 83 zero exits: 74 packages ran named tests successfully and nine contain no tests. The original service package command timed out after 40 minutes and remains failed. Fresh case groups and focused replays cover all 1,709 original service/added-regression identities successfully; the newer security-remediation change adds 11 passing service identities, for 1,720 covered current service identities. This is casewise coverage, not a successful monolithic service run.

## Source boundaries

- The original default package campaign used candidate `c56e2194ee45236b8c634843579b7f3e492b0db4` plus its recorded dirty working tree. Original streams and source manifests remain unchanged.
- The resumed supplemental runs used `d1b25077b89af8ef72e58c0b040b9ea495261b1b` plus the fixture repair. Focused service/handler/router tests covered the newer security-remediation template changes. The 5,097-file server manifest, including embedded instructions/docs and file modes, was unchanged between supplemental admission and final cleanup.
- `resume-current-source.json`, `resume-final-source.json`, `stage-fixture-boundary.json` and the exact per-command receipts preserve these boundaries. No new complete current-HEAD package run is claimed.

## Exact inventory

| Inventory | Pass | Skip | Fail / missing |
| --- | ---: | ---: | ---: |
| Current default top-level identities, selected casewise across the recorded runs | 11,233 | 9 | 0 / 0 |
| Current default named identities, including parent/subtest records | 23,688 | 9 | 0 / 0 |
| Current default leaf identities | 21,487 | 9 | 0 / 0 |
| Original service plus clock/code-revision regressions | 1,709 | 0 | 0 / 0 |
| New security-remediation service identities | 11 | 0 | 0 / 0 |

Each selected top-level case and its named subtree comes from one recorded execution. Parent/subtest counts are separate from leaf counts. Failed original executions are not erased or added to the pass count. Nine no-test packages are not described as tested behavior.

Additional Linux and `knowledgeintegration` runs cover 41 top-level identities excluded by the default Darwin build, with 100 named passes. The `agentintegration` synthetic subset produced 46 top-level PASS terminals and 327 named PASS terminals: 45 top-level cases were fully selected; `TestRK1KnowledgeCodexRegistrationIntegrity` was partial, with 17 passing children and `missing_real_version_binding` deliberately excluded because it sets the real-agent flag. The parent PASS is not evidence that the excluded child ran.

The remaining build-excluded inventory contains 91 Windows top-level tests, 22 real-provider/account/model tests, two unexecuted synthetic controls, and the one partial top-level case above. The two synthetic exclusions are `TestContextExperimentRejectsTrialConfigurationBeforeProviderProbe` and `TestRK1RuntimeContinuationControls`; both set `MULTICA_RUN_REAL_AGENT_SMOKE` internally, and they additionally require clean-source/archival operator-registration conditions. They are scope exclusions, not product passes or claims that all tagged tests need paid accounts. Exact identities, source files and build conditions are in `backend-final-excluded-tests.json`; `tagged-control-admission.json` separates real-provider entries from synthetic selections.

## Supplemental validation

| Run | Top-level pass | Named pass | Result |
| --- | ---: | ---: | --- |
| Redis service replay | 22 | 22 | All 18 prior Redis skips covered |
| Iteration I1 P95 | 2 | 2 | Passed frozen budgets |
| Triage scale | 1 | 1 | 10,000 formal + 2,000 pending issues |
| Public skills import | 1 | 1 | Actual public skills.sh/GitHub HTTP fetch |
| Auth race | 53 | 117 | Passed |
| Handler race selection | 136 | 240 | Passed |
| Service race selection | 98 | 247 | Passed |
| Current template service/handler/router | 114 | 266 | Passed; source-bound to newer candidate |
| Knowledge daemon composite | 14 | 27 | Synthetic provider through real server/daemon paths |
| Knowledge router composite | 10 | 28 | Synthetic physical process, correction/withdrawal/recovery |
| Linux agent/root replay | 3 | 6 | Fake executables, no network or host mounts |
| Linux descriptor tests | 2 | 2 | Real Linux descriptor/pipe behavior |
| Tagged synthetic controls and fake business fixtures | 46 terminal / 45 complete | 327 | One explicitly partial top-level selection |

The earlier fresh Linux namespace/runtime, router and daemon runs also passed: 15/43, 6/6 and 1/1 top-level/named respectively. No installed provider or authenticated external model was executed. The public-skills test fetched public documentation; it sent no external message.

Local handler-plus-PostgreSQL P95 measurements in milliseconds: iteration detail 96.861, preview 111.569, closure 1140.711; disable preview 165.036 and apply 384.419. Triage list/filter/action P95: 104.407 / 65.197 / 11.300. These use a bounded native ARM64 PostgreSQL tmpfs fixture with `fsync`, `full_page_writes` and `synchronous_commit` on; they are not production-storage, browser or network performance claims.

## Fixed finding

Synthetic stage-execution observations used the host clock, which was observed 14–19 ms ahead of PostgreSQL. Production correctly refused those future observations, causing otherwise valid fixture setup to retain an unknown receipt. The test fixture now reads its authoritative database clock before signing and propagates a clock-read error without returning a proof. Production validation, time budgets and security fences are unchanged.

Changed source/spec files:

- `server/internal/service/collaboration_stage_execution_fixture_test.go`
- `server/internal/service/collaboration_stage_execution_fixture_clock_test.go`
- `.trellis/spec/server/verification-environment.md`
- `.trellis/spec/server/database-test-isolation.md`

The two new clock regressions failed against the retained original implementation and passed against the repair. They verify the exact supplied timestamp, production rejection of a future observation, and clock failure without a signed proof. All 62 affected top-level cases received post-repair database replays. The first post-repair volume-backed replay still suffered infrastructure timing failures; it is retained as failed rather than described as green. Later tmpfs groups pass. No production Go source was changed by this backend lane, and no broad refactor or new dependency was introduced.

## Nine remaining default skips

| Test | Recorded reason / scope |
| --- | --- |
| `TestCommentContentBigramRequirementMatchesRealPGBigm` | The PostgreSQL image lacks `pg_bigm`; real extension/opclass integration is unverified. |
| `TestCollaborationStageExecutionHarness` | Coordinator-owned browser harness entry; not an ordinary package test. |
| `TestDeliveryCompletionM1BrowserFixture` | Opt-in browser fixture entry; not an ordinary package test. |
| `TestCLIConfig_UnknownFieldsArePreserved` | Existing documented limitation: JSON round-trip drops unknown fields. No encoder/interface redesign was made. |
| `TestIsDriveRoot` | Windows-only behavior. |
| `TestMcpMarketPublicHTTPRecipeLifecycle` | The current catalog contains no HTTP recipes. |
| `TestTriageWireFixtures` | Wire export runs only under its cross-package coordinator. |
| `TestWebhookHandler_DBErrorOnTokenLookupReturns500` | Existing skipped marker for a missing injected-query failure seam; no new proof of this branch is claimed. |
| `TestACPManagedTerminalPreservesWindowsUnicodeShellOutput` | Windows `cmd.exe` code-page behavior. |

Browser harness coverage belongs to the parent browser report; these backend skip terminals are preserved unchanged.

## Static checks, retained failures and cleanup

Fresh current-source `go vet -p 2 ./...` passed; both edited Go files are `gofmt` clean and `git diff --check` passed. Go compilation/type checking passed for the full default package selection and every targeted invocation. No TypeScript files were edited by this lane; TypeScript lint/typecheck results belong to the parent static lane. Fresh script checks passed (`test-go.test.sh`, `dev-env.test.sh`, `check.test.sh`, database-precondition checks), and the PostgreSQL seed integration suite passed all 13 tests.

`backend-retained-failure-inventory.json` indexes ten failed/interrupted streams, including the shared AMD64 PostgreSQL Rosetta crash, missing Linux Git fixture, monolithic service timeout, clock diagnostic/red runs, volume-I/O stalls, and the first tmpfs WAL-capacity failure. A separate Linux container setup failure occurred before any test because a read-only root prevented copying the test binary; the numbered corrected container run passed. Original logs and receipts remain intact.

Every successful disposable database was checked for zero connections before dropping. The final remaining template had its recorded seeded gate state `creation_enabled=true`, `enabled=false`, `host_execution_enabled=false`, and 856 migration entries. No serving API used that template. Its private dump is retained. All four backend-owned PostgreSQL/Redis containers are stopped; temporary Linux test containers were removed; no backend test runner remains. Failed native database state remains in the stopped owned container. The user's PostgreSQL and the parent's browser/native environments were not changed. Cleanup evidence is `backend-final-cleanup.json`.

Machine-readable evidence: `backend-final-summary.json`, `backend-final-default-top-level.json`, `backend-final-default-named.json`, `backend-final-excluded-tests.json`, `backend-final-runs.json`, `tagged-control-results.json`, and all referenced immutable raw streams.
