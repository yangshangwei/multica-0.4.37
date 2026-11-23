# Browser acceptance results

## Result

The current inventory contains **538 distinct Chromium-project case identities**. Reconciliation of the full initial run and admitted later executions gives **464 passed, 74 skipped, zero remaining failed**, with **zero automatic retries**. All22 failures found in the initial full run are closed. Two additional denied-notification failures discovered in a password lane required a real product fix and are also closed.

Source baseline is `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`, with task-owned repairs recorded by per-run source and artifact hashes. Final production Web build is `VLKTO9SF0CdGqBRibRIvB`. Latest runtime source digest is `a0c856da7308e0ed3e6bc441d5d3be2fbb2ef8734edb39200934c570ecde387e`.

This was a full initial default suite followed by complete affected spec files and explicitly configured conditional lanes. **Earlier unaffected passes remain attached to their original product/build hashes. They were not all re-executed against the final build.** Every completed attempt records its own source start/end identity, and failed attempts remain available. `browser-case-results.json` is the compact identity ledger; `broad-browser-final-summary.json` links the full private ledger and cleanup receipt.

## Meaningful fixes and verification

- Responsive collaboration creation lost its mounted form when the sidebar changed between desktop and compact Sheet. The parent integrated a stable modal boundary and focus behavior. Both existing Web/actual Electron rail tests were extended with a real draft/source preserved across the breakpoint and compact Escape returning focus; they failed against old built product and passed after rebuild. The original recipient-picker test also passed through its narrow-width interaction and actual creation.
- An archived notification absent from the current list subscribed to an inaccessible issue, received a forbidden event, cleared the parent list and repeatedly remounted/re-subscribed. The core realtime fix now treats repeated denial idempotently while retaining privacy purge and legitimate recovery. Both original offline-handoff journeys pass with the original generic denial stable for a one-second observation and private title/discussion absent. In the final archived-target observation window, Web recorded4notification200 and2issue404 responses; Electron recorded1notification200 and1issue404. Neither window contained503. This is **not** a whole-journey claim of zero503.
- Stale E2E navigation assumptions were repaired: actual user account trigger; explicit workspace switch control; current Settings label and native absolute href compatibility; resource-directory disclosure before native Skills/MCP entry; current welcome/market entry semantics; reselecting an issue after sidebar navigation clears selection. Their full owning scenarios retain business assertions and cleanup.
- The localized specialist case incorrectly treated the approved four-entry/two-column market directory as a single64px row. It now checks all four target sizes and containment while preserving workspace control height, dialog containment, overflow and localization/copy checks.
- No product change was made for the host stateful-resume failure: the combined S1 lane enabled knowledge, whose documented no-session-persistence policy intentionally creates fresh run namespaces. The unchanged stateful original passes in a separate knowledge-off lane; S1 retains its knowledge-on pass.

## Execution highlights

| Run | Result | Scope |
| --- | --- | --- |
| Initial full default |386pass /130skip /22fail,36m7s | Production Web + built native renderer; assist Provider, CLI, MCP catalog and feed configured |
| Post-modal focus |5pass /2fixture failures | Real Web/native draft preservation; original recipient and iteration-keyboard flow; identified remaining fixture assumptions |
| Owning-file repair replay |53pass /6fixture failures |17additional failing files; recipient already passed separately, union18initial failing files |
| Final navigation replay |40/40pass | Remaining fixture closures, native marketplace, global navigation, notifications, strengthened rails, recipient and core journeys |
| Password authentication |14/14pass | Registration/recovery branches |
| Password namespace controls |9/9pass | Both native/Web control flows, handoff, live revoke, terminal reopen and leave |
| Deterministic recommendation |22/22pass | Includes overlaps with default; ledger deduplicates |
| Host business |7pass /1configuration mismatch; stateful replay1/1pass | Eight distinct host cases, actual task-built daemon/CLI and fake Provider |
| Administrator |9/9pass | Each spec owns a fresh database and synthetic administrator |
| Additional password human flows |8pass /3fail, then source CLI1pass and final offline2pass | Exact source-CLI prerequisite supplied; real notification-loop fix |
| Post-denial regressions |23/23pass | Notification/account/global/mention/workbench permission and recovery surfaces |
| Native MCP |Original stale-entry failure; replay1/1pass | Full16recipe/copy/assignment/catalog/native flow preserved |

Run counts overlap and must not be added. The final464/74 ledger deduplicates by `(project,file,title)` and retains every failed/configuration attempt.

## Final checks and exact post-runtime change

All16 changed E2E files pass repository-base ESLint with devDependencies allowed only for the actual `**/e2e/**` test scope; whitespace checks pass. Unconfigured root-base lint incorrectly classified existing fixture imports as production dependencies, and an unchanged baseline reproduced those errors. No repository dependency or lint rule was changed.

After the last MCP runtime pass, its existing empty callback destructuring (`{}`) was replaced by the unused primitive `browserName` fixture binding to satisfy `no-empty-pattern`. The test body, assertions and runtime product bytes did not change; collection still finds exactly the same one case and scoped lint passes. No duplicate browser run was performed for this mechanical change. Current handoff source digest is `8222d487253da58bf90b77d8b3802dab3a0781f49c73da14faaf53d67c2657f8`; this differs from the final runtime manifest **only** at `e2e/mcp-desktop.spec.ts`, as checked in `core/final-handoff-source.json`. Runtime hashes were not relabelled.

The account-error fixture retains its original scope: denied workspace list remains readable alongside account detail and exposes the correctly named retry affordance. An initially added click-recovery assertion raced the intentional automatic retry and was removed; no original assertion was removed, and no manual retry-click execution is claimed.

## Remaining coverage limits

`remaining-browser-prerequisites.json` lists every one of74skipped identities across33files. They require separately allocated PAY241/detail/inbox/M1 contexts, cross-client Linux/installed Main artifacts, capacity preregistration, alternate memory/closure/timeout Provider fixtures, device-auth or migration/initialization configurations, project-knowledge runtime or resource-publishing storage. These are explicit unexecuted cases, not passes. No commercial Provider, user-installed agent account, native mobile device, Windows/macOS installer/updater/signing or production deployment was exercised.

The browser result does not establish remote GitHub CI status, approve an image-budget exemption, or authorize a merge/release. Those remain parent-owned merge-readiness decisions.

## Cleanup and retained evidence

All32 task-owned runtime allocations report application/provider ports closed and original gate booleans restored. The exact UUID-labelled PostgreSQL container was stopped only after verifying zero other client connections; its port is free. The earlier mistakenly selected amd64 task container was also stopped before counted browser execution. No normal database, unrelated process or other session's workspace was modified.

Raw traces, synthetic credentials and detailed reports remain in mode-protected private task runtime under `.omx/private/qa-work/main-readiness-20261123/core/`. Source snapshots, API/config identity, binary hashes, process groups, test JSON and cleanup receipts are retained there. Only concise result material is written into Trellis research.

Runtime and source are now released to the parent for final integration review. No commit, push or merge was performed by this agent.
