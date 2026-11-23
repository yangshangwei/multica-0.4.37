# M1-C2 critical-path smoke implementation verification

Status: independently reviewed and locally accepted; actual production-Web four-path run passed. Hosted execution remains untested.

## Delivered
- A separate PR workflow covers main and the integration branch, including changed E2E, business, launcher, lockfile and workflow inputs. It uses an isolated PG17/pgvector service, locked dependencies, matching Chromium, no repository secrets, and a named status. It does not modify remote branch protection.
- The private JSON launcher rebuilds the child environment, checks allocation and browser ownership, rejects ambient dotenv, verifies the actual database name before migration, and uses existing dev-env port/registry/start/stop primitives. It never invokes cmd_up, ensure_database, GC, destroy or a real daemon/provider.
- The dedicated Playwright config selects four original G1-01/G1-02/G6/G8 cases, production Web, one worker, no retries, failure trace/screenshots. The inventory gate independently checks exact file/title/project and first-attempt results.
- Source/config identity is checked before/after; partial startup cleanup is registered before launch; teardown failure remains failed. Raw private state stays outside the repository. Only sanitized logs/provenance/results and synthetic fixture traces enter the artifact allowlist.

## Executed evidence
- `node --test scripts/check-agent-critical-path.test.mjs scripts/agent-critical-path.test.mjs scripts/agent-critical-path-workflow.test.mjs`: 71 named tests passed, 0 failures/skips, contracts02.log.
- `node --check` for launcher, inventory and Playwright config; `bash -n` launcher; `git diff --check`: passed.
- Installed Playwright `--list` with a nonexistent synthetic DB port: exactly four tests in the two original files, Chromium only. This loads fixtures but executes no DB/test body. Its real JSON confirms the checker field layout. Feeding that list-only report to the checker exits1 as required.
- Baseline counterexample: a deliberately skipped temporary Playwright spec exited0 with 0 expected/1 skipped. This is the real tool behavior motivating the extra coverage gate, not a product-test failure.
- A temporary reporter-schema fixture (empty synthetic test bodies only) exercises the installed JSON reporter. Any four-pass result in that directory proves reporter/validator compatibility only, never Multica behavior.

Raw outputs, source fingerprints and evidence hashes are in `evidence/manifest.json`; private paths are outside Git. No API/Web/database/model was started by this lane for these checks.

## Run contract for coordinator
Use `node scripts/agent-critical-path.mjs --config /absolute/private/allocation.json --output /absolute/new/run-directory` after independent clearance and source freeze. Allocation is mode0600 and owned by the runner, with exactly:

```json
{"version":1,"owner":"agent-critical-path","databaseName":"recorded_smoke_database","databaseUrl":"postgres://synthetic:password@127.0.0.1:54321/recorded_smoke_database?sslmode=disable","browserPath":"/absolute/installed/playwright-cache"}
```

The database must already be allocated by its owner; the launcher only verifies/migrates it. Ensure psql is on PATH (local installation: /opt/homebrew/Cellar/libpq/18.6/bin). Output must not exist and must be outside the checkout. Private HOME, registry, uploads, temporary files and logs are derived under it. Frozen tracked docs/task files count in source identity too. The existing browser cache is selected explicitly; no runtime download fallback exists.

## Independent review and actual local acceptance
- Independent checker reproduced and fixed swallowed fingerprint errors; preserved red32pass/1fail, then all72 contract tests passed. See `independent-review.md` and `evidence/independent-manifest.json` for the final fixed source hashes. The earlier71-pass manifest remains historical evidence.
- Root executed the actual original four paths in `smoke-run01`, starting2026-11-05T16:00:16.790Z (2026-11-06 local time). G1-01/G1-02/G6/G8 each passed on its first Chromium attempt; expected4/skip0/unexpected0/flaky0, one worker, duration15.1s.
- API base commit `cd95c8e40`, production Web build `AWqXuPHM26R3H96ZXZgsU`, exact cleared staged CI source, rendered loopback API/Web URLs and unchanged pre/post source/config fingerprints verified. Root's exact generated Next import restoration occurred only after final identity acceptance.
- Launcher/test/inventory/cleanup exits all0. Independent observation confirms all four recorded API/Web launcher/listener PIDs stopped and both ports free. Root retains the owned database.
- All16 sanitized artifact files checked against private database credentials and signer key in memory:0 matches; no private env/registry manifest or symlink exported. Raw evidence retained privately.
- Detailed immutable run hashes, test names, provenance, cleanup and confidentiality checks: `evidence/actual-run-review.json`. No reviewer rerun was needed.

## Remaining verification
Hosted GitHub, branch protection, real daemon/provider/model behavior, complete E2E, desktop and NFR are not measured. The local four-path smoke and independent check are accepted; this child does not complete M1, and M2/M3 remain unaccepted.
