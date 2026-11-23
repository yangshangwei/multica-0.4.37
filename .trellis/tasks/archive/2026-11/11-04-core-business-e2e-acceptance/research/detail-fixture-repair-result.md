# Current fixture repair handoff

Root-only test harness edits are ready as `private runtime/detail-proposal/current-ui-harness-bundle.patch`; its companion JSON records every exact frozen before/root after hash. Twelve files are included: three already-committed root handoff/offline fixes reused, plus nine focused fixture/spec changes. No product source/dependency changes. The frozen runtime tree was not changed by this agent.

- Current task/allocation provenance: original actual exported D01 failed as expected; applied root fixture35/35 contracts pass. Existing source/artifact/PID/DB/manual-only guards remain.
- Existing root handoff changes: use Submit result/Result/Basis and exact submission-bound acceptance, preserving status/history/permission/no-AI assertions.
- Installed Main: workflow discovery opens More actions; result forms require explicit basis and delivery identity. Initial metadata still requires autoStart=false and stopped; after a real successful managed login, running requires matching task-private profile PID/config server/deployment/user. Fixture close stops only this verified daemon through real IPC.
- Close/reopen: handler first submits result through actual UI; execution/session facts must remain identical; creator accepts exact submission before original stop/proof assertions. Cancellation and original pending-run/notification/history assertions remain.
- Leave/invite: explicitly discover More actions without changing role denial assertions. Offline notifications include mandatory basis and verify exact retained submission; retained attachment flows explicitly open Attachments while keeping bytes/hash/history/input assertions.

Final verification:22 cases in12 spec files collected successfully (no browser run);7 existing Node controller/layout tests pass;node syntax and git diff--check pass. First collection duplicate-name error and second wrong-Playwright-singleton error are retained, then corrected. Pure collection does not replace actual E2E replay.

## Provenance builder

Prepared at `.omx/private/core-e2e-20261104/runtime/detail-proposal/build-current-provenance.py`; AST check only. Root must first apply the exact fixture to the frozen checkout, rebind source/build/API proof, restore explicit legacy auth and manual-only gates, and create an allocated record under the private task root with `{task, source_root, database, api_origin, web_origin, renderer_origin}`. Run `python3 <builder> --allocation <exact-record> --output <new-private-provenance.json>`. It imports the hash-bound existing controller for read-only identity checks, validates actual DB/manual gates and clients, and writes a new permit plus environment JSON. It never starts services, mutates the database or creates accounts. Runtime execution requires root review and fresh proofs; no builder live checks ran in this slice.

## Failure evidence and cleanup

pw09 trace showed visible Submit result, proving stale Submit for acceptance locator; desktop companion covered only information request/return. pw10 boot evidence showed correct isolated home and initially stopped daemon, then real login triggered managed reconciliation before navigate metadata's stale stopped assertion. Task-owned profile daemon72132 was proven by exact argv/pidfile/listener/health/config and terminated; port19604 became free. Other daemons were untouched.

Remaining: independent bundle review, fresh-database full original flows, source/artifact final audit, and any new failures. Do not mark old failed runs green from this preparation.

## Independent-review follow-up: managed restart launch guard

The reviewer found a second unconditional stopped assertion in `launch()`, reached by restart before metadata validation. Root authorized a minimal correction and synchronization to the frozen fixture. Initial unauthenticated launch still requires stopped. A retained authenticated managed session now requires running and the same exact private profile/PID/server/deployment/user identity check already used by metadata. No product or lifecycle API changed. Incremental patch `runtime/detail-proposal/installed-restart-guard.patch` (SHA8c2dfd0ead5d2449cecc9e57cf14a2f8454f2973c004f42eadd69e22846e3b83), final fixture SHAaa0f1ff6375dc9ca462a1361e5c3324fde58835c1d75b952c39e33eda2b2e749. Original12-file bundle is retained unchanged; apply this delta after it. Node7/syntax/diff checks passed; actual restart remains root-owned E2E work. Source provenance must be reissued after this frozen harness update.

## Live replay follow-up: outer Host installed restart assertion

ui-r2-02 reached the actual controlled Host stop/display/capture stage and then restart, where the outer spec still required stopped despite the verified post-login managed daemon being running. Root-only `collaboration-host-installed.spec.ts` now expects running after restart, explicitly relying on the controller's exact ownership checks. The next assertions still require the shared execution projection to equal the paused snapshot and exactly two Provider visits before explicit Resume. All initial/cleanup stopped guards remain. Two installed specs collect successfully; diff check passes; this delta has not been applied by this agent to the active frozen checkout. Live full completion remains unverified. Incremental patch SHA715a8b3515c69d900e82669d247341161a1130f5156ea5b89b551e8cbe0a4df0.
