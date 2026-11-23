# Runtime lane controller independent review

**PASS**, zero open execution blockers for the exact artifacts below. This is a static preparation review, not a claim that any password, Host or native journey passed.

- controller_sha256: `ef71240bbc6e2def59063b79b44e3b14a88a875cdebbf031e1d7e19402af7c98`
- allocation_request_sha256: `ac7c140720ce6954b2895dcf124c480f4ddcdb2b133188e8d52113e549b72bee`
- allocated_databases_sha256: `a12c6dc82fc7ab01fe7af9701738195126f157fe79210bdd14e6de094b9bd40a`
- runtime_manifest_sha256: `6ad02fb66afac1a0b0195d16d5f078f97fa6a72605d5bb4c1bc24e9bc8737fa2`

Candidate: `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`; 43 unique database allocations, 42 unique specs (knowledge OFF/ON repeats one identity).

## Findings fixed by author during review

- Raw framework trace could expose password request bodies/session cookies in ordinary task evidence. Framework JSON/stdout/stderr/trace/artifacts stay under task-private runtime/runs with0700 parents/umask077; ordinary evidence contains safe counters/statuses only.
- Controller exceptions after API swap lacked interrupted state and owned cleanup. Interrupt records preserve original lane/phase; stop only after exact registry DB/env and API listener/PGID/source are re-proven, otherwise preserve uncertain resources for root.
- Installed legacy CLI and installed full Linux Main need namespace execution gate enabled. pw14/pw15 requested enabled=true, host=false; root allocation ledger now agrees.

## Verified controls

- Root permit hashes bind controller, request, allocated DB ledger, root allocation, base config, test env, renderer proof and runtime manifest.
- Before any registry/API mutation, previous execution record must be complete and unchanged; active Playwright records in both public and private evidence block entry.
- 43 unique local database allocations; fresh migration/user/issue/admin/role read-only probe and exact prepared gate verification.
- Existing private registry checkout/ports/env checked; stop requires live API ownership proof; start uses existing manifest/load_env/save_manifest/start_api only, never up/ensure/migrate.
- Production Web PID/build/source, renderer PGID/artifact bytes/served index and rendered API/WS destinations checked before fixtures and after execution.
- Fresh per-spec configuration, isolated catalogs and unique synthetic admin; signup/bootstrap/login capability verified without recording passwords/tokens.
- Password browser sessions remain established by existing actual-login fixtures and cookies; controller setup uses bearer API identity checks appropriately.
- Per-spec API restart isolates registration budgets while preserving account/within-spec limiter behavior.
- Explicit controlled native/namespace/memory-provider binaries and current bundled CLI bound to manifest hashes; no commercial-model calls introduced.
- Zero retries, exclusive per-run directories, framework fail/flaky/skip prevent acceptance_pass; first run remains retained.

## Remaining scope notes

- PASS approves reviewed execution preparation only. Root must issue an exact permit after legacy-full-1 completes; current review did not inspect or mutate live service state.
- Run initialization errors before ownership is proven preserve unknown resources. Ordinary exceptions after swap stop only an ownership-proven API; signal/process termination supervision and other native subprocess/container cleanup remain root responsibilities.
- Successful last API remains alive for root inspection/final cleanup. No restoration to the previous legacy database is implicit.
- Controller counters are per-spec provisional evidence. Root must reconcile exact441 inventory identities, ON/OFF configuration repetitions, package/native conditions and skipped reasons, then run final source/artifact/endpoint audit.
- Runtime binary hashes are checked at admission; final full artifact/hash audit remains root-owned.
- Packaged pw10/pw15 require independently proven current package artifacts and explicit readiness in permit; exclude them until ready. Real commercial model is user-deferred NOT_RUN.
- Some existing specs write fixed ignored .omx/reports directories in the task checkout. Preserve/snapshot those per-run auxiliary artifacts before reruns; do not publish private authentication cache files.

## Verification

Static Python AST, manifest/hash alignment,43 unique allocation identities and prepared gate ledger consistency passed. Inspected the author17 helper-check result. This reviewer did not run controller/main, tests, database probes or service operations, and did not edit source/controller files. Lint/typecheck are not applicable to this read-only code/manifest review.
