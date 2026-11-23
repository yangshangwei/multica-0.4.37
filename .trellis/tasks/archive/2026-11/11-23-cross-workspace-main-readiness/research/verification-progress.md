# Verification progress

## Scope and candidate

The user approved execution on 2026-11-23. Work is isolated in `.omx/private/qa-work/main-readiness-20261123/src`; the root checkout belongs to concurrent work. The initial product candidate was `cb192014b`, then the private branch fast-forwarded to `95c8b12dc`. Root has now reached `73a28ed90` including marketplace `9bca2c02e`; this final increment must be integrated after active TS finishes, then affected tests and production builds rerun.

## Completed bounded checks

- Fresh sequential lint/typecheck on initial product candidate: 15/15 Turbo tasks, zero errors, 31 existing warnings; no cache hits. The earlier commentary of 13 tasks was a count mistake; raw summary is 15.
- Image checker: 11/11 Node tests (original8pass/3fail), independent5 subprocess probes, syntax/lint/checkJs and real branch enumeration. All121 offenders preserved; without explicit PR reason it correctly exits1. Proposed disposition is recorded separately, no actual PR exists.
- Generated docs: four stale embedded page bodies regenerated; independent temporary generation twice matches all157 bundle files without mutating the backend input during tests.
- SQLC1.31.1:149 emitted files byte-identical to candidate; four handwritten helpers/tests in existing generated directory are deliberately not generator outputs and remain untouched.
- UI export suite/check, reserved-slug generation parity, native evidence contract, environment check, Go wrapper self-tests, dev-environment tests, changelog/database precondition contracts, installers and selfhost configuration checks passed.
- Knip exits1 with existing unused/unlisted items (including archived test snapshots). CI currently treats knip as warning-only; no broad cleanup performed.
- Initial complete Core TS package:381 files/5,502 tests pass. Initial Views run was interrupted with owned SIGTERM because it competed with S1; partial views are not counted. Remaining complete packages restarted on `95c8b12dc` with two workers.
- Five historical originals:5/5 latest pass on initial candidate, no skip/automatic retry, independently reconciled against raw receipts. Only localized creation locator changed. Synthetic namespace control config corrected; unchanged S1 passes after Docker/resource pressure resolved. Details in `core-replay-results.md`.

## Active checks and limitations

Backend race verification runs in a separate UUID-owned native arm64 PG/Redis allocation with Go1.26.6 and CLI guard. Initial amd64 allocation and unsafe external temp ancestors were diagnosed as setup failures; original outputs remain, corrected package replays use a verified private temp chain. No product permission guard was relaxed.

Full applicable browser acceptance remains pending final integration/build. Current default collection was538 cases/187 files; count must refresh after final incoming tests. The full legacy+renderer directory and feasible local conditional fixtures will run, with unique identities deduplicated and explicit unavailable opt-ins. No real vendor accounts, native mobile UI or release installers are claimed.

Raw logs stay under `.omx/private/qa-work/main-readiness-20261123/`; do not add raw logs/traces/screenshots to Git. Only concise evidence and reproduction summaries belong in the task.

## Final integration update

Private source fast-forwarded to `73a28ed90` after the complete Views suite stopped. It had 629 passing files/7,844 passing cases and one outdated settings trigger CSS assertion (`xl:hidden` versus the approved `lg:hidden` rail contract). Independent review confirmed the1024px compact boundary; the owning full settings/page-header files are replayed after a minimal test-only correction. Latest marketplace and notification suites, consumer tests, and final static checks run on the new candidate; prior counts are reconciled by test identity rather than added indiscriminately.

## Final-candidate local checks

Private candidate `73a28ed90` includes all currently observed concurrent commits. Root and remote branch were equal and tracked root status clean at the last fetch. Final13 lint/typecheck tasks plus2MDX prerequisite generators pass (15 total),31warnings unchanged. Mobile typecheck/lint pass with7warnings and196pure-function tests.

Whole-package/file-level reconciliation, independently reviewed, establishes15,054 latest non-mobile unit assertions across the completed package runs and affected file replays. It is not a single post-fix whole Views run or a per-case machine ledger; see `frontend-verification.json` and `frontend-independent-audit.md`.

Additional actual checks pass: worktree database cleanup contracts, desktop download scripts, entrypoint signals, Makefile build naming, Agent critical path contracts, and Helm config. Helm was downloaded to the task-only tools directory from get.helm.sh (v3.19.0, published SHA256 matched), not globally installed or added as a project dependency. PowerShell/Windows checks remain unexecuted on this macOS host.

GitHub read-only observations: main reports no branch protection and no repository rulesets; no current PR or check-runs exist for73a28ed90. This does not remove the task's quality gate and does not constitute remote CI success. No remote publication or merge has occurred.

Default browser538 identities are now executing on rebuilt API/Web/Desktop with explicit local fixtures. Early confirmed failures are old account-menu and workspace-switch fixture triggers; allowed repair ownership is with `core_replay` after the frozen run completes. Original output stays immutable. Backend full guarded race run has completed46/86 packages including Handler, with no unresolved completed-package failures after documented environment replays; still running.

## Newly reproduced browser defects and fixture drift

The full frozen browser run has one confirmed product-level regression: the sidebar-owned collaboration creation entry is unmounted when the viewport crosses the compact breakpoint, losing the visible dialog/draft. Independent source/retained browser review confirms modal ownership belongs outside Sidebar's responsive Sheet subtree. A second task-owned sparse worktree `modal-fix` at73a28ed90 isolates test-first implementation from the ongoing frozen browser run; only app-sidebar component/tests are assigned. No change to store/API or creation-form semantics is planned. See `responsive-dialog-diagnosis.md`.

Confirmed fixture drift additionally includes account menu identity (user name replacesMultica), explicit workspace-switch trigger, sidebar settings label, and account error retry affordance. Their originals remain in the full-run report. Core replay owns minimal scoped fixture corrections only after that run ends, followed by replay of complete owning specs. No timeout extension, `.first()` ambiguity suppression or replacement of original business assertions is authorized.

## Full default browser terminal

First complete final-candidate default run finished:538 identities,386pass,130skip,22fail,0automaticretry. Source-start andsource-end digests match. Provider/API/Web/renderer task ports are closed and gate booleans restored. Failure list: private `core/final-default/failures.json`; raw reports remain immutable. This is a diagnostic full run, not a passed gate.

The modal product fix passed126owningunit tests in its separate checkout. Before integration, the existing Web/Desktop icon-rail originals are extended with draft-preserving breakpoint crossings and compact Escape focus; they run against the old product to prove regression. Only after thatred run closes can its5source/testfiles be integrated, followed by production rebuild and owning-spec replays.

BackendService full race completed1888top/4613namedPASS. Storage test's inherited077umask and an over-broad dbidAST import rule were discovered later; environment correction and narrowtest-contract review respectively are in progress. Do not call all backend tests passed until those and theagenttail/vet/scan are complete.

## Post-full-run repair integration

The two extended original rail cases reproduced the responsive creation failure on bothWeb/Electron against the old product (2fail,24.9s, unchangedstart/end source, closedports). The five reviewed modal source/test files were copied from modal-fix only after that freeze ended, verifying every destination still matched73a28ed90 beforehand. Rebuild and affected live replay now pending.

Approved fixture repairs are current semantics: footer account trigger, explicitSwitchworkspace, exactSettingsroute, realworkspacelist retry, expandedResourcesdisclosure and currentwelcome/catalogroutes. Their original security, persistence, no-write and recovery assertions remain. Unknown iterationspicker/workbench resize failures remain diagnosisitems, not assumed fixed by fixturechanges. The specialist market geometry failure is beingchecked against intentional2columnwelcome control layout before any assertion update.

Backend85regular+Agent have run. Vetpassed; currentGo1.26.11govulncheck exits0 with no reachablevulnerability (older1.26.6advisories retained). RemainingdbidAST failure is narrowcontractreview; no productionIDgeneration changes authorized. Privateenvironmenterrors were repaired onlyinharness, originalsretained.

## Backend terminal and post-modal focused result

All86backend packages have latestvalid full-packagePASS after documented corrections:25,564namedPASS/24SKIP,11,741topPASS/23SKIP,0fail/missing/unfinished. Fullgo vet passed. CurrentGo1.26.11govulncheck passed with0reachablevulnerability; default race suites usedGo1.26.6. Redis shared-passwordopt-in supplemental test passed separately. Package source/hash audit and exactownedcontainer cleanup are finalizing; no productbackend logic changed (onlydbidASTtestcontract).

Post-modal focused browser run:7originals,5PASS/2FAIL/0SKIP/0retry,49.5s. The actualWeb/Electron extendedrail cases and originalrecipientcreation pass; originaliterationpicker nowpasses withoutchange. Remainingworkbench failure traced to sidebarParticipating clearingissueURL before resize; owningfixture must reopen its exact task. Remainingmarketgeometry failure is the oldsingle-rowtablist height assumption versuscurrenttwo-columnwelcome selector; measuredper-tabcontainment/targetcontract repair pending. No furtherproductbug claimed.

## Conditional browser progress

Finalpassword14/14 and namespacecontrols9/9 passed on currentmodalbuild withfreshownedDBs,0skip/retry,sourcebinding andclosedportreceipts. Originalreceipt gates preserved. The59casefullfailed-files replay remainsinflight; newlycorrectedlogout/workbench/recipientpass, but its own extraDesktoprecovery-click assertion racesautomaticrefetch and its Settingsrelativehref restriction excludesnativeabsolutehrefs. Those test-only repair regressions are retained and will be corrected afterfreeze. Preserve originalerror-affordance assertions without addingunneededrecovery timing; validate parsedURLpathname forbothplatforms. Defaultlatest failures are notdeclaredclosed untilreplayresultsreconcile.

## Final runtime follow-up

The59failed-file replay produced53PASS/6fixturefail, all retained. The remaining causes were corrected onlyintheir owningfixtures (realresource disclosure, remove newlyadded flakyrecoveryclick, supportnativeabsoluteSettingslinks). A40case finalnavigation/lateUI replay is runningonexactfinalmodalproduct.

Deterministic recommendation22/22 passed. Hostbatch8 produced7PASS/1FAIL; the lone stateful-resume test ran underS1's knowledge-on environment, and actualprovider evidence contains `no_session_persistence=true`. Independentcontracttrace is checking this mismatch; noresumed=trueassertion was changed. Exactoriginal will run inits appropriate ordinary-host lane ifconfirmed. S1knowledge-on passedintheactualbatch, so those remain separateguarantees.

## Original failures closed

Finalnavigation40/40PASS(314s),sourcepreserved/allownedportsclosed. All22originaldefaultfailure identities now have latestvalidPASS. Ordinaryhostruntime1/1PASS inknowledgeofflane(32s) preservesstatefulresumeassertions;S1knowledgeon alreadyPASS. Currentindependentledgercheckpoint538identities443PASS/95SKIP/0FAIL beforeadmin/extendedpasswordsupplements, whichremaininflight. No historicalR3passes substituted.

Remainingplannedwork:finishadmin9+passwordhumanextended11,finaldedupe/skipinventory,independentcase/provenance/cleanupaudit,source/base refresh,copyonlytaskownedverifiedchangesbacktorootwithpreimagechecks,preparecommitbatchandfinalreadinessreport. NoPR/push/mainmergeauthorizedorperformed.

## New live access-denial defect

FinalofflinehandoffWeb/Electron remainedfailedaftertwoincorrectwordinghypotheses. SafeactualDOM+API diagnostics showedstablecorrectURL/private-title-absent but repeatedloading:314notification200+352notification503+524issue503+84issue404 in14.55s. Independenttrace foundWSforbidden→clearallownedaccountcaches→notificationparentunmountdetail→listrefresh→sameunknownarchivedtargetremount/resubscribe→forbidden feedback. This is a realproductdefect, not merecopydrift.

A newcore realQueryClient/binder regression failedbeforefix (epoch3vs1 afterduplicateforbidden+rebind;11existingtests passed). Minimalfix in `packages/core/realtime/collaboration.ts` leaves `subscriptionDenied=true` but onlycallsrecoverAccess when issueisnotalreadydenied. Firstdenialprivatepurge, trueaccesschangeevents, accountrevalidation, HTTPmanualrecovery, ack andlaternewrevocationremainintact. Fourcanonicalcorefiles27/27passed; +1newtest, noViewsduplicateharness. Product/runtimeproof awaits rebuiltWeb/Desktop originals. Incorrectfinalnotificationwordingeditrestoredgenericdenial; privacyassertionsretained.

## Access-denial runtime closure

`final-denial-replay` originalWeb/Electronofflinehandoff2/2PASS23.96s,0skip/retry, source-start=end andallownedportsclosed. Bothkeepgenericcanonicaldenial atarchivedunknownnotificationtarget distinctfromknownredactedrow'sIssueunavailable. Same denialelement remainsvisible for1s,private title/bodyabsent. Safeactualarchived-readreceipts:Web4notification200+2issue404;Electron1notification200+1issue404;0HTTP503. This replacesonlythetwofailureidentities,notoriginalredhistory.

Oneadditionalcoretestraisesreconcilednonmobileunitcountto15,059; +196mobileunchanged. Finalconsumer types and23notification/account/global/workbench/mentionregressionsplusMCPnativeconditional remainrunningbeforefinalauditandcleanup.

## Local commit delivery

User explicitly approved the six-group local commit plan with “执行”. The five work commits are recorded in `local-commit-delivery.json`; all38 committed blobs match the verified source hashes. The sixth approved commit preserves task evidence. No push, PR creation or main merge occurred. Remote CI/image disposition remain separate delivery conditions; the task is retained in_progress rather than archived as an unconditionally mergeable branch.
