# Execution plan

- [x] Read repository workflow, specs, test entry points and record user authorization.
- [x] Capture current candidate and create isolated production legacy environment.
- [x] Build Web/Desktop and run core business/button/creation/status/comment/auth/navigation tests first (one worker, no retries).
- [x] Run remaining default Playwright inventory and inspect every failed/skipped case.
- [x] Independently run fresh lint, typecheck, unit, script and isolated backend checks.
- [x] Configure password/admin and available deterministic provider/daemon/native opt-ins; run conditional journeys.
- [x] Reproduce defects, record boundaries, implement minimal fixes and add regressions via Trellis agents.
- [x] Rebuild affected runtime after product changes and rerun impacted plus core suites.
- [x] Reconcile case-level inventory, review changes/spec needs and verify owned process cleanup.
- [x] Deliver report with current evidence, failures/fixes, exact coverage and remaining external prerequisites.

Commands are recorded per lane. Baseline commands use pnpm exec playwright test --workers=1 --retries=0 --reporter=list,json; static/unit use forced Turbo runs; Go uses scripts/test-go.sh against a fresh non-serving DB. Rollback means reverting only task-owned edits, never the initial dirty candidate.


## Final result

- Canonical browser inventory:496/496 latest PASS,0fail/skip/retry. Original complete run and every replay retained.
- Concurrent security-template changes: new production API/Web/Desktop build +14/14 focused cases and20 affected UI units.
- Static:14,709 initial TS unit passes; changed Core/Views/Web checks and named regressions pass.
- Backend:23,688 named PASS+9explicit SKIP,0missing/unresolved default failures; service whole-package40m timeout preserved, casewise1720 identities covered. Additional synthetic tags/races/Linux and knowledge checks recorded.
- Product fix: distinguish obsolete action-options denial from true collaboration read loss.
- Full report/artifacts:artifacts/qa/full-e2e-20261120/REPORT.md;coverage.json;final-summary.json;backend/FINAL.md.
- All task-owned listeners/containers stopped; original shared PostgreSQL accepts connections.
- User authorized commit after verification: product 67af89db8, fixtures/specs 7817d8958. Only task-owned files staged; pre-existing work retained. No push/deploy.
- Task retained at its fixed path because current runtime-output allocation manifests bind this exact existing task directory. Archiving requires updating its explicit fixture contract; completion is recorded in place.
