# Runtime lane report — permissible execution complete

## Result
Runtime source is stable and ready for independent parent integration review. Complete original order-export and postreview memory-runtime journeys both PASS on frozen03, built with supported Go1.26.11. Both owning packages pass complete normal and race suites. Nine original namespace identities are environment/policy BLOCKED; their historical Probe cause remains UNKNOWN. Original failures and every new intermediate failure remain retained.

This lane adds bounded diagnostics. It does not establish or claim a scheduling/provider/root-cause product fix. The new host runs still log broad503 and poolpressure symptoms; all ten actual launch permits returned200 and the original pre-provider failure did not reproduce. No timeouts, retries, permissions, capability admission, dependencies, commits, pushes or Docker security policies were relaxed.

## Changed files
1. `server/internal/daemon/collaboration_host_router_test.go`: sanitized transport-error/context labels, monotonic request intervals, child stdin EOF/read-failed and shutdown events. Status0 stays distinct from received503. No raw URL, transport error or body enters diagnostics.
2. `server/internal/daemon/collaboration_host_router_boundary_test.go`: real HTTP canceled/deadline/503 controls; private error exclusion; EOF versus read-failure control.
3. `e2e/fixtures/collaboration-host-runtime.ts`: parent spawn/ready/visit-wait/stdin-end/process-close monotonic observations in existing evidence. Separate parent/child clock origins permit bounded alignment only.
4. `server/internal/daemon/collaborationrun/controller.go`: safe stage/code/errno/exit/context namespace refusal diagnostics, nil-safe unwrap and bounded helper parsing; captures context before cleanup and preserves concurrent cancellation separately from no reply, malformed reply and explicit refusal.
5. `server/internal/daemon/collaborationrun/controller_test.go`: safe refusal/cause propagation, allowlisted helper codes, nil unwrap and concurrent canceled-read controls.
6. `server/internal/daemon/collaboration.go`: logs only the typed safe namespace diagnostic; fail-closed admission unchanged.

Simplification: diagnostics extend existing transport and summary paths; no new service, retry mechanism, policy fallback or framework. Final six-file SHA256 manifest is `artifacts/qa/main-acceptance-fixes-20261125/runtime/source-ready.json`; every file equals frozen03.

## Original evidence → hypotheses → discriminating validation
Original host memory/order: claim200, client permit status0, server503, never-authorized/unlaunched ledger and never-launched proof. Competing explanations were API delay followed by test-cleanup cancellation, earlier transport/request deadline, or provider setup error. The ledger contradicted provider launch as the failing boundary. New actual HTTP controls prove canceled, deadline and received503 produce distinct safe facts. New full journeys show ten permits200 (46–824ms) with transport/context `none` and successful real provider/CLI handoffs. Earlier frozen01 memory also passed. Historical attribution is still UNKNOWN: the earlier run lacked the necessary cancellation/transport timeline, and later success cannot reconstruct it.

Broad API503s recur in final03: order320 with3 poolpressure log lines; memory116 with2. Safe route counts, complete API logs and permit summaries are retained. These counts do not identify the cause of each503 or prove a pool-size fix. No pool limit was changed (max12/min1).

Original namespace cross-client/lane-close/lifecycle/offline-notifications: nine failures at readiness before execution assertions. `namespace-policy-blocked.json` preserves their exact original IDs, titles, inventory hashes and current disposition. Canonical cross-client fixture and original reviewed adapter require `--privileged`, `seccomp=unconfined`, `apparmor=unconfined`; parent confirmed those conflict with current no-security-weakening authorization. None were executed. Read-only Docker observation reports built-in seccomp/cgroup namespace policy. Do not import backend EPERM or the distinct new experiment as the historical cause.

A separate default-policy probe04 actually invoked the frozen controller: `helper_start`, errno1, context`none`, exit2 (46ms), under default seccomp, unprivileged container, no network and all capabilities dropped. It refused before helper launch. This applies only to that exact new configuration. Probe01 read-only-copy,02 unavailable bind-source,03 tmpfs execution setup failures remain recorded as NOT_RUN; they are not tests or historical reproductions. All three actually created containers and their anonymous volumes were removed and verified absent. `default-policy-probe-reconciliation.json` distinguishes setup from actual Probe.

## Verification
Final evidence root: `artifacts/qa/main-acceptance-fixes-20261125/runtime/final03/`. Each run has exact argv/cwd, source/config/tool/runner hashes, raw stdout/stderr, real exit, before/after checks and named inventory.

| Verification | Result |
| --- | --- |
| Complete order-export, original600s/30s budgets, zero retries | 1 PASS; body45.550s, Playwright47.4s; exit0 |
| Complete postreview memory-runtime, original300s/30s budgets, zero retries | 1 PASS; body39.159s; exit0 |
| Full daemon normal Go1.26.11 | 2053 named PASS,2 existing platform SKIP,0 FAIL; exit0 |
| Full daemon race Go1.26.11 | 2053 named PASS,2 existing platform SKIP,0 FAIL; exit0 |
| Full collaborationrun normal Go1.26.11 | 383 named PASS,0 SKIP/FAIL; exit0 |
| Full collaborationrun race Go1.26.11 | 383 named PASS,0 SKIP/FAIL; exit0 |
| Owning-package go vet, gofmt, targeted fixture TypeScript check | All exit0; formatting output empty |
| Earlier Linux daemon/controller compile under localGo1.27 | Both exit0; compile-only, not Linux runtime |
| Distinct Linux/arm64 default-policy actual Probe built Go1.26.11 | Refused helper_start errno1; exit2; not an original journey PASS |

Package commands use `scripts/go-test-with-agent-cli-guard.sh -- <pinned-go> test [-race] -json -count=1 <owning-package>`. No database/API pool/publisher variables enter those processes. The two unchanged daemon platform skips are `TestCollaborationBatchPollerSlotsAndRunnerLinux` and `TestIsDriveRoot`; no skips were added. Full terminal inventories have no missing tests.

Host commands: `python3 .omx/private/acceptance-fixes/runtime/run_final03.py host order-export-01 .omx/private/acceptance-fixes/runtime_order01/env.json order`, then `... host memory-runtime-02 .omx/private/acceptance-fixes/runtime_memory02/env.json memory`. Migration receipts were0 before use. Actual child Playwright commands retain one worker and retries0.

Frozen03 includes current dirty/new server and fixture sources. APFS-cloned dependencies retain relative workspace links resolving inside the snapshot. Production Web/Desktop artifacts are byte-bound; UI product sources match the earlier parent-built snapshot exactly (`ui-build-source-binding.json`: no changes). Source, binaries, Go/Node tools, configurations and runner remained unchanged across both full journeys. This is lane acceptance, not a claim that parent final integrated full check is complete.

Earlier red diagnostic failure, concurrent unrelated compile failure, moving-source preliminary run, Go1.27 results and TS4058 from inherited declaration emission are retained separately. Fixture typecheck uses noEmit/declaration=false; no product type suppression. Original concrete evidence hashes still match `original-evidence-final-audit.json`.

## Resource audit
All execution HOME/TMP and frozen sources are Git-external under `/private/tmp/acceptance-fixes-4b9d3494a2/runtime-frozen01`,02,03 and separately recorded probe roots. Private manifests/scripts: `.omx/private/acceptance-fixes/runtime/`. No old runner executed or old profile modified.

Final host DBs: parent-owned `fix_runtime_order01` and `fix_runtime_memory02`, existing recorded container `22bd321d8816f54fd23328fc2adeccc1e6e553285f041d765b41baa3adaa0961`, loopback62450. Exact owner/image/volume/port inspected before SQL. Immutable derived env removes DB URL query and selects PGSSLMODE=disable; original allocated env remains untouched. Failed and successful databases retained.

Order ports64578/64579/64580 and memory51187/51188/51189 are closed; all recorded API/Web/renderer/Playwright PIDs absent, no host fixture daemon processes, DB clients0 at closure, no cleanup fallback. Original gate booleans restored, epoch2→4. Default probe containers/volumes absent; no shared service stopped. Final resources are in `final03/resource-audit.json` and per-run receipts.

## Remaining limits / parent follow-up
- Historical host cause UNKNOWN; broader503/poolpressure remains observable despite successful complete journeys. Do not claim this diagnostic increment resolves overall API availability.
- Exact nine original namespace identities BLOCKED by current security policy; historical cause UNKNOWN. Default-policy EPERM is a distinct observation only.
- Parent owns independent final R1/R2 review, full integrated cmd/server/handler/service checks, spec capture and campaign release verdict. No further lane-specific permissible execution remains pending.
- Suggested spec capture: safe Probe stage/context/no-reply contracts and separate monotonic clock domains; a new PASS never changes old outcomes.
- Commercial providers, native Windows/iOS and general Linux namespace support are not established.
