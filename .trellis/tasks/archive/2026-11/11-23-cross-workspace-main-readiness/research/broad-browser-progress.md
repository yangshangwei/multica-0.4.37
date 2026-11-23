# Broad browser execution in progress

Final source commit: `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`, including the latest notifications and marketplace increments plus task-owned uncommitted fixes. Production Web build `qAqAzf40GL0apHFFr12jn`; complete source digest `5ed08fcf45aa4a94af56fd35f5b84706174c31235ea8b242419bb9bfaa889a18`.

Final API/Web/Desktop rebuilds exited 0. The current full default run is `core/final-default`, all 538 identities / 187 files, legacy + actual built renderer + deterministic assist Provider + explicit task CLI + private MCP catalog + changelog feed. Collection marked 31 early skips and 507 default expectations; body-level skip outcomes remain pending, so this is not a pass count.

The new marketplace desktop original and role-template cases have progressed without failure markers in the live runner, but only terminal JSON is final evidence. All original failures will be retained. Conditional password/control/host/admin/recommendation lanes are queued after the default run, not concurrently.

The harness admits no inherited real-provider credentials; new per-attempt DBs and private HOME keep normal user environments untouched. Completion receipts and source-end hashes will be required before final reporting.

## Frozen full-run findings awaiting allowed fixture edits

1. `auth.spec.ts` logout fails at `fixtures/sidebar-navigation.ts:56`: helper seeks product-brand button `Multica`, but the account footer trigger now intentionally exposes the actual user name. Snapshot shows `E2E User`; parent approved fixing only the owning helper after the frozen fullrun. All three helper consumers (`auth-qa`, `auth`, `main-supplemental-qa`) need replay.
2. `capability-market.spec.ts` workspace isolation fails after clicking the current workspace-name button at line196. That button now returns to the workspace; the adjacent exact `Switch workspace` button owns the menu. Parent approved replacing both same-case trigger occurrences after fullrun, preserving destination/prefs/draft assertions. The notification test's name-button click is intentional navigation and must remain unchanged.

No fixture source will change while the full run is active.

3. `collaboration-account-desktop.spec.ts:856` expects the removed generic `Refresh` action after workspace-list500. Current named action is `Retry loading workspaces`. Focused replay must first establish actual error state and retain account-content availability, then exercise the correct retry; parent approved scoped fixture diagnosis.
4. `collaboration-recipient-picker-web.spec.ts:123` loses the entire creation dialog after width changes to375 despite earlier draft and Escape assertions. Source trace shows AppSidebar nests `CollaborationCreationEntry` inside the responsive Sidebar, whose compact branch moves children into a closed SheetContent. This is a potential product mount-lifetime defect, not grounds to weaken the test by resizing first. Parent requested independent trace and isolated original replay before product edits.

## Additional frozen findings (parent-authorized fixture repairs pending terminal)

- Iterations helper / native audit / navigation require current exact `Settings` label instead of historical `Workspace settings`, preserving real route and business assertions. The existing success toast `Workspace settings saved` is unrelated and remains valid.
- MCP/Skills welcome Desktop tests enter a collapsed `Resources & connections` directory and cannot see their links; explicitly expand the actual group before navigation, without modifying stored product preferences.
- Skill market empty inventory now presents the intentional Skill welcome. Verify real empty API inventory and welcome region while retaining no inline-template-tab and From template keyboard entry expectations.
- Workspace-defaults no-runtime scenario still expects old squad Workspace/Templates tabs. Follow actual Browse templates -> squad marketplace, preserve no automatic squad creation and needs-runtime saved project semantics.
- One iteration picker test failed on its second Enter selection; current screenshot may be post-cleanup onboarding. No cause or repair assumed until isolated replay.
- Workbench account-detail test lost visible Back to list after resize390; snapshot is list. Parent approved isolated original post-modal replay with route observation, no assertion weakening.

At case432/538,19 original failure identities have been retained. Full suite still running on frozen source, no browser-owned source changes made.

At case471/538 the run retains20 failures, including the third account-menu consumer. The latest sidebar-icon-rail Web/Desktop originals progressed successfully; their existing assertions do not cover keeping the creation draft across the responsive Sidebar branch. Parent approved extending those existing two cases after the frozen run with draft retention across1024 and compact Escape/focus. A real creation adds a collaboration record that intentionally prevents workspace deletion, so successful native creation must preserve ownership/cleanup semantics rather than silently bypass that product rule.

## Initial frozen full-run terminal

`final-default`: **538 identities = 386 pass + 130 skip + 22 fail**, 36m7s, no automatic retries. Start/end source digest matches `5ed08fcf…889a18`; all owned Provider/API/Web/renderer ports closed and gate restored. Exact failures and skips are private `final-default/{failures,skipped-cases}.json`. This is the original candidate inventory, not the final acceptance verdict.

Extended existing rail originals were then run against unchanged pre-fix product: both Web/Desktop failed (24.9s), confirming the responsive creation failure on actual native UI as well as original recipient flow. Parent integrated separately reviewed modal lifetime/focus repair only after that terminal. Rebuilt production Web/Desktop both pass.

Approved fixture-only locator/entry repairs were applied after frozen source proof: account footer user trigger; actual workspace switch; current Settings labels; actual retry-loading-workspaces action with failed-state screenshot before successful retry; expand resource directory before native MCP/Skills welcome; current empty Skill/Squad welcome and market routes. Original business assertions and cleanup remain. Account helper red cases and all owning specs are scheduled for replay.

Unresolved original observations still to diagnose: second keyboard iteration selection; account workbench detail after narrow resize; obsolete one-row market-tab geometry (current intended grid has2rows). No product assertion was loosened for these.

## Post-modal seven-case focused replay

`postmodal-focus`:5pass /2fail,49.54s,0skip/retry; source-end identical and appportsclosed. Both strengthened Web/Desktop rail originals now preserve draft and source across1024, and compact Escape restores navigation/focus. Original recipient completes its real create flow. The original second-Enter iteration selection passes unchanged on fresh allocation.

Two residual failures were fixture contracts:

- Workbench trace showed `Participating` sidebar navigation changed URL from `/account?view=current&issue=<id>` to `/account?view=participating` at35719ms, before resize36563ms. The test must select the listed issue again; added exact issue-query and heading assertions before permission and narrow-view checks. The product did not drop selection during resize.
- Specialist skill visual test measured the entire category tablist at110px and assumed64px. Current approved marketplace welcome is2columns of4entries, with52px targets plus6px gap. Replaced obsolete wholelist height assumption with4target geometry checks (at least44px and contained), while retaining workspace-entry bound, page overflow, dialog bounds and all localization/copy assertions. Both baselinefull and postmodalfocus recorded this same red failure first.

All17files containing original full-run failures now replaying as59original cases (`fixture-repair-replay`), preserving complete business sequences. No product change for these2findings.

## Configured conditional lanes

- `final-password`:14/14pass,17.15s, no skip/retry; source preserved, appportsclosed.
- `final-controls`:9/9pass,76.80s, no skip/retry; includes original live revoke/terminal and both native control journeys, explicit host=false gate restored.
- `final-recommendation`:22/22pass,66.28s, no skip/retry; actual Electron recommendation adoption and real API deterministic-provider requests, no agent runtime. Overlaps18blocked-feedback/default cases; identity ledger will deduplicate.

The59case repair replay has6residual fixture failures so far: current-workspace resource directory remains collapsed after switching; extra added account retry-click races its auto-recovery (remove this unrequested new assertion and keep originalvisible-error scope);4native Settings strict relative href intersections reject Desktop absolutehrefs. Source screenshots prove correctSettings present. Correct these only after frozenreplay ends; do not count earlierpasses twice.

## Final navigation/runtime follow-ups

- `final-navigation-replay`:40/40pass,314.13s; complete remaining failing fixture files plus latest global navigation, both strengthened rail originals, notification Web/native, latest native marketplace, recipient and core business journeys. Source preserved, appportsclosed.
- `final-host`:7/8pass; stateful runtime expectation failed because the combined S1 lane enabled knowledge context. Provider evidence explicitly reported `no_session_persistence=true`, matching the documented RunID namespace/no-resume contract, even for empty approved knowledge.
- `final-host-stateful`:exact unchanged runtime original1/1pass in32.05s with knowledge disabled; S1 retains its correct knowledge-on pass. No runtime product or assertion change.
- `final-admin-01` through `-09`:all9originals pass on fresh synthetic-admin databases; no shared admin identity or real credentials.
- `final-password-human`:8/11pass; two offline-handoff originals expected a generic issue-denied string after opening an archived notification, although current notification detail owns `Issue unavailable`. Corrected only that notification predicate and added absence of private title/body; direct issue404/redaction remains. The third source-surface prerequisite lacked explicit builtCLI path; task harness now supplies it. Three originals replaying.

The final coverage is intentionally a full initial default run plus complete affected files/conditional replays. Unaffected earlier passes remain tied to their original product/build source, not relabelled as finaldigest executions.

## Two real remaining failures: archived notification denial loop

The human replay closed the source-surface CLI prerequisite case, but both offline-handoff originals still fail. Changing the final generic denial wording was an incorrect hypothesis and was reverted. Current helper preserves the original generic issue-denial assertion for the archived notification fallback, plus notification heading/private-title/body absence. The earlier selected denied comment correctly expects `Issue unavailable`; these are different branches.

A single isolated Web diagnostic (14.55s) recorded correct `/account?view=notifications&filter=all&notification=<archived transfer>&issue=<id>`, visible detail with no heading, only24characters of transient text and no allowed denial message. Its actual API log contains314successful notification reads,352notification503s,84issue404s and524issue503s across the whole case, rather than a bounded denial state. No raw message body or credentials were printed. Diagnostic instrumentation was removed afterward.

Independent source review traced the loop: unknown archived notification is absent from the all-list, so fallback `readableIssueId` subscribes to its issue. WS forbidden clears both issue and account-list data, making that ID null and unbinding; account invalidation reloads the list and reselects the same fallback ID, creating a new subscription whose local denied flag is fresh. Parent assigned a product fix with real Query/binder regression. Runtime is stopped and source unfrozen for this repair; PostgreSQL remains owned for replay.

Current ledger is exactly538 identities: **461pass,75skip,2fail**. The22initial full-run failures are all closed; these two newly admitted conditional failures remain explicit blockers. No completion claim is made.

All22 completed browser allocations currently report owned application ports closed. The test PostgreSQL cluster remains deliberately running for the denied-notification repair replay. Parent confirmed the remaining prerequisite-only families stay explicit rather than implying release acceptance; one feasible native MCP catalog flow will be admitted after the core fix because the existing password/catalog harness already satisfies it.

## Denial repair runtime accepted

`final-denial-replay`:2/2pass in23.96s, no skip/retry; both Web and actual Electron retain the original generic denial element for an added1-second observation, private title and retained discussion absent, and original404/redaction assertions intact. Final archived-target response-window records show Web4notification200 +2issue404 and Electron1notification200 +1issue404, with no503 **in that bounded window**. Whole journeys still contain earlier transient503s; no global-zero claim.

`final-denial-regressions`:23/23pass in155.92s, covering notification Web/native,11native account cases, global scope navigation Web/native, six mention permission journeys and workbench conflict/transfer/revocation. Complete start/end source identity preserved and owned appportsclosed.

The additional MCPnative original first failed at a hidden MCP link in its collapsed Resources directory. Minimal actual-disclosure opening and current Settings label were applied to that one owning spec; full recipe/catalog/copy/assignment/native workflow replay is in progress. No product fix added for this fixture drift.
