# Client acceptance repairs — final lane report

## Result and source scope
| Full original group | Final label | Result | Source |
|---|---|---|---|
| Manual detail Web + Electron | manual-c1-final03 | 2 PASS, exit0 | New C1 production builds + synchronized capture fixture |
| Inbox snooze Web + Electron | snooze-final03 | 6 PASS, exit0 | New C1 production builds + sealed observer |
| PAY creation Web + Desktop | pay-final02 | 2 PASS, exit0 | Earlier frozen frontend, before C1 feature |
| Administrator resource publication Web/native | resource-final01 | 3 PASS, exit0 | Earlier frozen frontend, before C1 feature |
| Protected delayed reads Web/Desktop | delayed-final01 | 2 PASS, exit0 | Earlier frozen frontend, before C1 feature |

15 distinct original cases have complete passing runs. The8 C1/C2 cases use the latest sidebar change; the earlier7 retain their exact prior-source scope and are not relabeled latest-candidate acceptance. Actual packaged Main/oldCLI are owned by Hooke and are not implied by renderer/native-fixture evidence. Parent final backend binding may supersede this lane's Go1.26.11 API snapshot.

All final runs retain original case/action budgets, workers1/retries0, no added skips and unchanged business assertions. Frozen source/dependency/build audits match before/after. Each final group uses its own parent-migrated database. Current native protocol set/default exactly restored; no client Electron processes or listeners remain. Independent C2 closure accepted in parent research/c2-snooze-closure.md. C1 source semantics accepted and final raw audit is present; final C1 narrative closure pending.

## Product change C1
User explicitly approved restoring a remembered account return operation. AppSidebar exposes one Return to previous collaboration / 返回上次协作 button outside the existing disclosure, including icon/collapsed navigation. It is disabled in account scope. Disclosure, explicit list links and drafts keep their semantics.
Web uses current-identity resolveScopePath(account) + useWebScopeResume and immediately invalidates pending workspace lookup. A deferred-lookup regression failed before the counter fix. Desktop delegates to resumeDesktopScope(account) without a path. No new store, backend change or mobile change.
Files: packages/views/layout/app-sidebar.tsx and test; en/zh-Hans collaboration.json; apps/web/platform/scope-shell.tsx and test; Desktop desktop-layout.tsx and workspace-gate test; global-scope-navigation.md. Canonical manual fixture clicks the realbutton, never history or issue reselection.
Full manual evidence: both clients restore the original selectedissue and exactscroll620→620; singleaccounttab retained; each completes19 captures across EN/ZH and1280/900/430 widths plus full information/delivery/revision/resubmission/acceptance/self-complete/reopen path.

## Fixture repairs and C2
- PAY expected heading Participating matches actual account destination.
- Resource publication opens Resources & connections before Skills and accepts a task-specific output directory.
- Snooze uses Switch workspace and waits for actual target Inbox href before click.
- Delayed-read denial scopes to direct detail-scroll-root alert; protected bodies/dialog/composer/storage/source-loss checks preserved.
- Main fixture requires exact Current handling for fresh zero-workspace login. No installed-clients edits here.
- Manual JourneyAction excludes unsupported repair_information (type-only). Locale capture waits for actual successful comment rendering before native restoration can overwrite its position. Capture waits for exact geometry stability under existing5s polling budget before positioning; exact before/after target/scroll/containment assertions retained.
- Snooze observation lifecycle explicitly seals future request admission, retains every already-admitted request/response reader through drain, closes client, then asserts errors again. New post-boundary requests are explicitly counted; missing bodies/schema failures are never ignored. New helper e2e/fixtures/observed-request-lifecycle.ts and deterministic Node controls under e2e/test/observed-request-lifecycle.node-test.mjs.

C2 rawfinal03 evidence has zero traffic_observation_failed and9sealed/9traffic_closed records. Three deterministiccontrols include actualSnoozeTraffic received200/unread-summary bodyclient_closed afterseal, which must reject finalCheck and retain failurefact. The oldfinal02 authentic6PASS terminal and its lateclient_closed remain untouched; its claimed error-free observation was withdrawn.

## Validation
- Shared sidebar/disclosure/scroll Vitest88PASS; Web scope/navigation/account-shell46PASS; Desktop layout/navigation51PASS. Total185 relevant product regressions;3newsharedtests and deferredlookup regression verified RED→GREEN.
- Observed lifecycle Node tests3PASS, including actual observer late-reader rejection.
- Product typechecks views/Web/Desktop and targeted E2E typecheck exit0.
- Changed productfile lint and new observer helper/test lint exit0. Existing E2E fixture lint still reports pre-existing dependency declarations/no-useless-escape/no-unsafe-finally; before/current diagnostics unchanged. No rule disabled/newdependency added.
- Fresh Web and Desktop production builds exit0; sourcechanges=[] through builds. Web rebuilt after reviewed counter fix; unchanged Desktop source/artifact equivalence separately hashed.
- Visual-verdict score94PASS retained under public c1-visual-verdict.json and .omx/state/acceptance-clients-c1/ralph-progress.json. Actual EN/ZH wide/narrow screenshots inspected, no product visual redesign.
- git diff --check owned files exit0.

Exact commands, exits, source manifests, tools and artifact hashes are under artifacts/qa/main-acceptance-fixes-20261125/clients; rawprivate evidence .omx/private/acceptance-fixes/clients. Final manual command: CLIENTS_DATABASE_LABEL=clients_manual_final03 python3 .omx/private/acceptance-fixes/clients/run-c2-second.py manual manual-c1-final03. Strict snooze: CLIENTS_DATABASE_LABEL=clients_snooze_final03 python3 .omx/private/acceptance-fixes/clients/run-c2.py snooze snooze-final03. Full private manifests c2-freeze02.json/c2-freeze.json bind external physical trees and copied production assets. All execution HOME/TMP outside Git; no personal credentials, model execution or newdependencies.

## Preserved failures and cleanup
Original400PASS/138SKIP,117PASS/21FAIL and114PASS/24FAIL unchanged. All preliminary/sharedDB, setup and failed run records retained. C1manual01WebPASS/DesktopFAIL on late locale restoration; manual02WebPASS/DesktopFAIL on1.3px native layoutdrift; manual03bothPASS. C2final02sixPASS with lateobserverfailure retained; strictfinal03sixPASS separately. Other retained failures include initial manualgate mismatch, snooze oldlink race/bodyresourceunavailable, MDX timestamp integrityfailure, refused duplicateDB creates, frozen-copy setup refusal and automaticVCSstamping failure; none silently relabeled.
Latest final manual+snooze cleanup audits prove ownAPI/Web/rendererPIDs absent, assignedportsclosed, DBotherclients0; DBs/uploads/profiles kept as taskevidence. Full macOSprotocolbaseline was captured before C1preparation and immediately before launch. Cleanup unregistered only exact added com.github.Electron bundles under clients/c1-final02,c2-final01,c2-final02. c1-protocol-restoration.json statuspass and exact_set_and_default=true; native lane released through c1-native-released.json. No further native runs planned here.

## Follow-ups
Independent C1/C2 final audit and parent integrated candidate/runtime/native outcomes remain separate. No commit,push,deploy,release performed. Do not infer release approval from this lane or combine partial case steps with successful cases.

Review clarification: protocol equality is relative to the captured C1 pre-preparation baseline. Whole-task initial protocol provenance predating earlier preliminary native activity remains unproved, as recorded by the parent reviewer; no earlier-campaign default was assumed.
The C2 deterministic test moved from e2e/fixtures/*.node-test.mjs to e2e/test/*.node-test.mjs after the bound snooze run to follow existing lint test-path classification; imports updated and explicitnode:url used. Lifecycle/observer/businessspec bytes are unchanged; latestthreecontrols and newfilelint pass.

Independent review evidence finalized in independent-review-evidence.json: C1source semantics accepted; C1finalraw audit96fileszero mismatches, actual2PASS, currentimplementation matchesfrozen, protocolC1baselineequality; C2closure explicitlyCLOSED. Parent final narrative/integration/native outcomes remain outside this child. No further implementation or native execution pending in this lane.
