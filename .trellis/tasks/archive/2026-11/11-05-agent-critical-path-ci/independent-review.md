# Independent implementation review: critical-path CI

Reviewer: `trellis-check` / `ci_plan_review`. Scope: eight CI/launcher/config/test files, their existing dev-env consumers, task contracts and proposed private allocation. No application, database, browser or model was started by this review.

## Final conclusion

**PASS — local four-path product smoke and independent review accepted.** The actual-run section below and `evidence/actual-run-review.json` bind the final evidence. Hosted CI and broader product acceptance remain separate.

## Pre-run conclusion (recorded before execution)

**PASS for the recorded owned local smoke, after staging the new CI source files and freezing source/task/spec writes.** No remaining implementation blocker was found in this bounded check. Actual four-path acceptance is still pending. The root coordinator owns allocation and will execute the launcher; this is not a hosted-CI or product-pass claim.

The private allocation was inspected without printing credentials: owner matches, regular file mode0600, database `rnd_m1_smoke01`, explicit loopback port51908, and explicit existing Playwright cache. No database query was run by the reviewer. The launcher's current_database comparison remains mandatory before migration. Browser ownership/install validation runs before service startup; an unsafe or unavailable cache must fail as a retained prerequisite error rather than installing or borrowing another browser.

## Findings (fixed)

- `scripts/agent-critical-path.sh`: `smoke_identity` previously embedded three fingerprint calls inside `printf`. A failed source/config command could produce an empty field while printf returned0; repeated failures could therefore accept matching empty identities. Changed it to checked assignments, propagated the original exit, and rejected empty values.
- `scripts/agent-critical-path.test.mjs`: added a failed-source-identity process-boundary regression. Before the fix it observed exit0 instead of43; after the fix it stops before migration/API/Web/Playwright and records the failure. The original red output is preserved.

## Findings (not fixed / operational constraints)

- Existing `dev-env.sh` fingerprints untracked `apps`, `packages`, `server` and `scripts`, but not a newly untracked root Playwright config. The coordinator will stage the eight CI source files before the run; `git diff HEAD` then includes the config without needing a commit or changing the shared fingerprint algorithm. Do not claim this run's source binding if the config remains untracked. All covered writes, including tracked task/spec documentation, must stay frozen through the final identity check.
- This launcher verifies declared allocation plus actual database identity; it cannot independently establish who originally created a local database. Root's retained allocation record supplies that ownership evidence. CI owns its dedicated service database.
- Retained traces contain only synthetic fixture sessions. They are deliberately inside the artifact allowlist; private env files, database credentials, registry manifests and the signing key are excluded/redacted. No production credential/account is part of the proposed environment.
- Hosted workflow execution, remote branch protection, actual provider/daemon process behavior, all other E2E cases and NFR/model behavior remain untested and outside this child.

## Checked boundaries

- JSON config rejects default/system databases, mismatched names, non-loopback/implicit ports and connection overrides; shell uses the exact URL and compares current_database before migrations/fixtures. No cmd_up, ensure_database, GC, DB destroy or Make path exists in this launcher.
- Child environment is rebuilt, root/Web dotenv is rejected, account/provider/integration settings do not survive, and runtime state/upload/home are private. Browser cache is explicitly selected and checked against the locked registry.
- Existing process ownership/start/stop primitives are reused with cleanup flags set before startup. Fake boundary tests prove partial failures clean owned components and a busy unowned listener receives no kill/adoption. Cleanup failure remains a failed run.
- Fixed selection and JSON validation bind exact four original source/title pairs and Chromium, one worker/result, retry0, expected passed and no global/test errors/skips/duplicates. Playwright and inventory exits both gate acceptance. The real installed reporter-schema fixture validates format compatibility; the real list-only report is correctly rejected.
- Workflow targets both requested PR bases, uses read-only permissions and synthetic PG17/pgvector configuration, installs locked dependencies/matching Chromium, runs contract tests, and uploads only evidence with always/error-on-empty behavior. It does not claim to configure required branch protection.
- Reviewed `.trellis/spec/web/frontend/agent-critical-path-ci.md`; it matches implementation and preserves the real/fake/hosted verification boundaries.

## Verification

- Independent initial Node contract suites: **71 passed, 0 failed/skipped**.
- Added identity regression, red: **32 passed, 1 failed** in the launcher suite; actual0 versus expected43.
- Final independent three Node suites: **72 passed, 0 failed/skipped**.
- Node syntax checks for launcher, validator and Playwright config: pass. Bash syntax and scoped diff whitespace check: pass. There is no TypeScript source change in this child; no whole-repository typecheck is claimed.
- Installed real JSON reporter schema: four synthetic empty-body cases accepted as schema evidence only; real list-only report rejected. No product test body was executed by this reviewer.
- Exact current source and raw evidence hashes: `evidence/independent-manifest.json` (includes the two review fixes). Implementer's earlier `evidence/manifest.json` remains historical and must not be presented as the final fixed source manifest.

## Actual-run evidence — accepted 2026-11-06

The coordinator ran the exact cleared launcher with the private `smoke-config01.json` allocation into a new `smoke-run01` directory. **The four actual product browser paths passed on production Web: G1-01, G1-02, G6 and G8, Chromium worker1, retry0.** Raw and exported Playwright reports and the inventory agree: expected4, skipped0, unexpected0, flaky0, one passed result per case; total reported browser duration15.1s. This is the real original-spec run, not the earlier empty-body reporter probe.

The API served committed base `cd95c8e40` plus the reviewed staged CI source. Web ran `next start` with production BUILD_ID `AWqXuPHM26R3H96ZXZgsU`. Both process manifests reference the exact same source fingerprint `760af74f212020ec16ac9d156903866b676d7f7eaee401f143ec38e60faa4e03`; pre/post source/API/Web fingerprints match. The new root Playwright config was staged, and all eight current source and staged hashes match the clearance manifest. Rendered browser configuration points to the owned API18251 and Web13171. The explicit allocation is `rnd_m1_smoke01` on the owned loopback PG17 container port51908; the launcher's successful identity checks precede migration and fixtures.

Launcher, Playwright, inventory and cleanup all exited0. Independent read-only process inspection after completion found both recorded launcher PIDs11888/12346 and listener PIDs11950/12371 absent, and no listener on either API18251 or Web13171. The reviewer sent no kill signal. Root retains database ownership and later cleanup responsibility.

All16 exported evidence files were compared in memory with the private database URL/password (raw and decoded) and the generated JWT signing key: **zero matches**, no exported symlinks, no env or private registry manifest. Browser manifest checksum matches the launcher result. Raw files and sanitized exports remain separate. There were no failure traces to inspect in this passing run; the retained artifact's test-results directory contains only Playwright's last-run record.

After the successful final identity audit, root restored only the known generated `next-env.d.ts` route-types import to HEAD. The existing source-fingerprint contract normalizes exactly that rewrite; it does not alter the product-source acceptance above. No product edits or new test/service/database runs were made in this follow-up review.

Compact source/run/provenance/result/cleanup/confidentiality evidence and raw/export file hashes are in `evidence/actual-run-review.json`. Local M1-C2 acceptance is complete. Hosted GitHub execution, branch protection, real daemon/provider/model behavior and complete E2E/NFR remain untested; parent M1–M3 completion is not implied.
