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

## Fresh rerun requested 2026-11-20

The user requests another complete current-tree run, core flows first, with autonomous reproduction and repair. Reuse this existing task and its approved scope; previous evidence remains historical. New evidence: `artifacts/qa/full-e2e-20261120-r2/`.

- [x] Allocate fresh legacy/API databases, rebuild current Web/Desktop and run core scenarios first.
- [x] Run all ordinary browser cases and locally available conditional authentication/admin cases.
- [x] Fresh static/unit/script and isolated backend checks.
- [x] Reproduce each new failure, repair its owning boundary, retain red/green evidence and repeat impacted journeys.
- [x] Reconcile current-run counts and coverage gaps; stop task-owned services.

### R2 final evidence

- Fresh canonical browser inventory496/496 latestPASS,0fail/skip,24originally failing cases retained; no historical pass counted.
- Initial core76/76; concurrent final quick-create source changes validated with fresh production core76/76 and last hint-placement refresh23/23; owning unit67/67. Final build`-Rh1o8Up4vFfJhbqZQGG3`, source stable through verification.
- Main TS14714unique+4newcases=14718current identities,0unresolved; initial10failures retained and whole-file73/73 replay passed.
- Backend84packages,default23691namedPASS+9SKIP; supplemented23795namedPASS,0missing/unresolved; narrowraces andknowledgecomposites pass.
- Performance100participants/10000propagations/100runwritebacks pass. I1closure2060.17ms exceeds2000ms once; independent unchangedprotocol1088.14ms pass. Retain variability as a limit, no speculativeproductfix.
- Seven test-only repairs independentlyreviewed; two specsupdated. Fresh private ASAR corruption diagnosed/frozen dependency rebuild integritychecked; actualpackagedLinuxMain2/2pass.
- All27ownedlistenerportsfree,allownedcontainersstopped; originaluserenvironmentsretained. Report`artifacts/qa/full-e2e-20261120-r2/REPORT.md`.
- Keep task directory at the existing explicit allocation path required by runtime-output/detail fixtures; markcompletedinplace rather thanbreakthetestcontractbyarchiving.

## R3 fresh collaboration-first regression, 2026-11-20

The user explicitly requests autonomous comprehensive collaboration/business E2E and all available functions, reproducing and repairing new defects. Reuse this existing scope and task; do not create another task or count old passes. New evidence: `artifacts/qa/full-e2e-20261120-r3/`.

- [x] Capture current source and isolate production legacy/password environments.
- [x] Fresh collaboration-first and core business browser tests, then complete canonical inventory.
- [x] Fresh static/unit/script/backend verification in independent owned lanes.
- [x] Enable available controlled runtime/native/knowledge/stage/cross-client paths.
- [x] Reproduce failures, minimally repair owning boundaries with red/green evidence, review.
- [x] Reconcile every case, current-source drift and honest remaining gaps; clean owned resources.

Existing pending changes belong to other sessions; preserve them. Build production assets once, pin/copy them before independent lanes; avoid competing rebuilds. One browser worker, no retries; no real-provider account or external publication required.

R3 browser514/514 latest admitted PASS; original17failureidentities retained. Product/tooling fixes committedcbcb86b94,37e2faa0f,6f51d385f,ddc80d5af. Backend all local/race/Linux/synthetic andquietperformancepassed; GitHuboriginaltestafterresetpassed andfinalPG/Rediscleanup verified. Allroot44portsfree8containersstopped.
