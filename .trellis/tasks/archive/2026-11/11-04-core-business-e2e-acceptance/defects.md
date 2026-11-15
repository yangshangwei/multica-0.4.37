# Current-run failures and blockers

## ENV-001: Changelog feed outside the fixture-owned live directory
- Original run: evidence/legacy-full-1, e2e/changelog.spec.ts:99.
- Actual result: fixture refused before publication, `Refusing to replace a feed outside the task-owned acceptance directory`.
- Cause: coordinator configured task-private feed under original checkout .omx/private, while this test deliberately requires isolated checkout `.omx/state/desktop-changelog/live/`. Guard is correct; no product assertion was reached.
- Next: preserve original full run; after completion copy only owned feed to accepted isolated live directory, update owned API configuration, verify identity, and replay changelog spec in a new run. Do not weaken guard.

## TEST-001: Non-Git local project mode unit test timed out
- Original run: frontend nonmobile full suite; packages/views/modals/create-project-local-mode.test.tsx:307.
- Actual result: 5000ms test timeout, no business assertion mismatch reported.
- Root cause: not established. Several adjacent tests were slow; resource contention remains a hypothesis.
- Next: one independent whole-file replay without timeout changes, preserve first failure and report stability limits.

## TEST-002: Detail journey fixtures coupled to an old task
- Two current cases require another task identity/allocation path.
- Current status: BLOCKED pending reviewed narrow fixture patch and real listener/allocation proof.
- Private patch prepared; no source changes applied during original run.

## Conditional scope
- Real commercial-model acceptance: explicitly deferred by user; NOT_RUN, no paid calls.
- iOS UI: local Xcode/simctl absent; BLOCKED. Native Windows/Linux desktop and distribution trust require actual target platform evidence.

## E2E-001: Native English duplicate picker remains open after keyboard selection
- First full run: fixture triage-duplicate-picker.ts:185 expected eligible-listbox count0, got1.
- Evidence: synthetic query unexpectedly became `Resolved UI/UX upload failure e`; option still selected, input focused/list expanded. Framework trace records ArrowDown/Enter but lacks native context network/key/composition events.
- Same-run desktop Chinese and both Web variants passed. Cause not established. Next: one original-case replay after quietwindow, then evidence-driven native input instrumentation if needed; no click substitution or timing increase.

## E2E-002: Member assignment workflow never emits expected write
- First full run: G-E1 timeout150000ms while waiting for PUT /api/issues/<id>.
- Trace: member directory200 contains target; second click waits for target locator and never resolves; no task write emitted. Assignee menu appears briefly then closes; trigger instance identity changes while frame canonicalizes UUID route to CORE-1.
- Hypothesis: route-normalization remount races initial picker click, separate from API/method failure. Reproduce original case first; compare explicit canonical-route readiness only if needed, and distinguish harness readiness from actual product interaction risk.

### One unchanged replay of E2E-001/E2E-002
`diagnosis-original-1` ran exactly the two original cases with unchanged code/assertions/timeouts and zero automatic retries:2PASS/0FAIL,27.7s; source unchanged. Latest case state isPASS with initial instability preserved. This is not a product fix, proof of load causality, or a clean first full run. Original navigation/input risks remain documented; no retries-to-green loop.

## ENV-002: Idle production Web listener disappeared during coordinated pause
- changelog-feed-replay-1 preflight refused absent13704listener before tests; no business write occurred.
- Root will restore exact owned production services and record new listener proof; preserve failed preflight.

ENV-001 resolved by selecting fixture-required owned feed location. changelog-feed-replay-2 PASS, actual live publication/cached-reader recovery verified; no product or assertion changes. Original ENV-001 and ENV-002 remain in history.

## ENV-003: Password migration fixture generated a future-issued legacy token
- password-phase1-1 pw34 legacy setup and pw32 Desktop binding failed before setup capability.
- Root permit selected cutoff2026-11-05T00:00:00Z while execution was2026-11-04T17UTC. Both fixtures mint iat=cutoff-60s, therefore future23:59UTC.
- jwt.WithIssuedAt and password_session.go issued.After(now) correctly reject it. This is coordinator configuration, not a product auth bypass/defect.
- Correct only cutoff to2026-11-04T00:00:00Z in a new numbered run with fresh DBs. Keep future deadline and all old-token/identity/history assertions. Preserve first failures.

## ENV-004: Test temporary directories inherited original repository Git ancestry
- Backend default execenv full package had5 failures with HOME/TMP under original root .omx/private. The non-Git-directory fixture found original repository as its Git ancestor.
- Original `.cursor/mcp.json` is now untracked122bytes, observed hashfb40aae928dc3a6ec111763e3d52cd76ce83c03dd78c792d7285ccb21eb7a606,mtime2026-11-05T03:04:15UTC. No pre-run snapshot establishes previous existence/content. Preserve it; no guessed restore/delete.
- New v4 executor uses /Volumes/artisan/e2e-private/core1a32-20261105/backend and rejects any Git ancestor; fresh execenv DB; first5failures retained. Other suites audited by exact test behavior; no blanket claim of safe original temp placement.

## TEST-003: Collaboration workflow browser fixtures target previous UI contract
- pw09 expects Submit for acceptance, current UI uses Submit result with mandatory Result/Basis and exact submission acceptance.
- pw15 expects Add participant directly; current entry is under More actions.
- pw18/pw19 expect direct completion/cancel/transfer/leave buttons. Current completion requires submission by handler before creator acceptance.
- Fix only test interaction to current contract, preserving permissions/history/no-execution/Stop-resume assertions and adding exact submission binding. ROOT product untouched; frozen original failures kept.

## TEST-004: Packaged Main metadata asserts stopped daemon after authenticated managed login
- Initial boot is correctly task-isolated,autoStartfalse,daemonstopped. Login triggers current managed-session reconcile to running; reused metadata incorrectly requires stopped again.
- Keep initial stopped assertion; after login assert owned profile/server identity rather than arbitrary running. Originaltaskdaemon72132/19604 verified and stopped by owned cleanup.

## ENV-005: Source-surface acceptance CLI variable omitted
Originalpw29 failed its explicit COLLABORATION_SOURCE_CLI prerequisite. Currentcandidate CLI existed, but runner configured only MULTICA_E2E_CLI_BINARY. Newreplay row explicitly pins samecandidate server/bin/darwin-arm64/multica; no sourceassertionchanges.

## PERF-001: Currentcandidate iteration closure P95 exceeds frozen target
Current1a32candidate actualTestIterationI1P95 reports30measured/5warmup: closure2155.025ms>2000msFAIL; detail78.280ms andpreview396.391msPASS. No race/automaticretry/thresholdchange. Originalhistorical2171ms retained separately; repeatclass doesnotproveproductrootcause. Thisremainsopenperformancegate; overallcompleteacceptancecannotbePASS. Fullrawrun includesdisable+triage resultspending.

## ENV-006: Shared PostgreSQL crashed under Rosetta during UI replay
DockerState exited133/OOMKilledfalse at2026-11-05T03:32:23.957Z; logs:rosetta unexpected signal during sigreturn thenpostmasterexit. UIlaneclose Webcancel read503 and subsequent3native registrations503; postrunDBprobe failed. Root restarted samecontainer preserving volumes, no datareset. Originalrun retained; newfreshDBrequired for affectedcase replay. Host/containerarchitecture x86_64 PG onarm64host was recorded beforetesting. No proofthisis applicationrootcause.

## ENV-007: Installed Linux acceptance image lacks required runtime host fixtures
Original registration failedENOENT because /etc/machine-id absent; actualsameimage identityfunction RED→GREEN when per-container syntheticreadonlyID added. Next fullrun proves installationpresent/server/usertrue, butdaemonexits. Captured269byteerrlog SHA5bebebc71450b1800468c26113f4df22b039b1fedd7d4dd67e434d689a7aa525 exactlymatches frozenconfig.go noagentCLI installed refusal. This is separate from machineidentity. Nexttest-onlyrepair provides explicitlycontainedversion-onlyfakeCLI, rejects/records anyexecution and requires noexecutionmarker; actualtaskexecution remains originalnamespaceprotocol/controlledHost scopes, realmodeldeferred. Productfailclosedguardsandrunningassertion retained.

## OPEN-LINUX-001: Final installed Main workflow fails during close; primary error not retained
- linux-delivery-replay-1/linux-delivery-r6: one originalcaseFAIL, zero retries, new DB, source_matchestrue.
- Final error Installed Main close exceeded30000ms. Spec traceoff, catchstatus fails and finallyclose replaces the preceding exception, so raw data cannot establish the first business error.
- API timeline reacheshandoffback200/commandsGET200; nextcodecommand restart120s plusstatus20s/close30s/cleanup roughlymatches214.9s. Treat restartblock as hypothesis only.
- Do not claim businesscomplete or onlycleanupfailed. No confirmedproductfix; preserveFAIL. Next repair testdiagnostics toretain primaryandcleanup errors/no-payloadcommandtimeline, thenone targetedreproduction.

## Final cleanup
Allownedtestprocesses stopped; API18784/Web13704/renderer5878/Redis60284 closed; noownedcontainers.161taskdatabases retained with0connections. No unrelated DB/container/process stopped. SharedPostgres retainedrunning. Currentproductsource unchanged; rawprivateevidenceexcludedfromcommit.

## CLOSED-LINUX-FD-001: Background daemon inherited Electron descriptors
Improvedpayload-freetimelineprovedrestart.closeblockedafterElectronexited; actualdaemonretainednon-CLOEXECFD3/4andChromiumfiles. LinuxlaunchboundarynowmarksFD>=3CLOEXECwithoutclosingparentFDs; bothbackgroundstart/selfupdatesuccessorcovered. TrueLinuxpipeEOFregressionREDfd64/65+timeout→GREEN preservingstdin/stdout/stderr/ExtraFiles andparentFD. OriginalpackagedMain withonlyfixedbundledCLI passed47.268s,0retry/skip;restart.close52.387ms,allrestartphasescompletedandfullbusinessassertionspassed. Sourcevariantbase1a32+FDpatchd6f4ceeb,CLI8ad7a9ac;original1a32FAILnotrewritten.

## ACCEPTED-PERF-001
User“这个2155可以接受”acceptsrecorded1a32closureP952155.025ms deviation only. Original2000msassertion/FAILpersist; notglobal/futurethresholdchange, notsame-run performancePASS. No furtherperformanceworkrequired.
