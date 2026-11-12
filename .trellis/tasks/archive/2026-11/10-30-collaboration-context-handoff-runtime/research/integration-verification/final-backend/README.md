# Final backend gate

Result: **failed**. See `findings.md` for the observed failures and read-only recommendations.
Frozen candidate: `e18295d0c892811870fb2f8d42572751288e26061f311ec157942553e4b6736b`.
Database: `multica_runtime_full_20261101_2392cdc0`. Service and handler ran sequentially.
Environment: Go 1.27.0, macOS arm64; PostgreSQL 17.11. All Go tests used the ambient-agent CLI guard, `-race -count=1 -p 1 -parallel 2 -timeout 40m` and `GOMAXPROCS=2`. The real-agent gate was absent.

| Suite | Exit | Top-level pass / fail / skip | Subcase pass / fail / skip | Missing named tests |
| --- | --- | --- | --- | --- |
| service | 1 | 582 / 3 / 0 | 1069 / 0 / 0 | 0 |
| handler | 0 | 163 / 0 / 0 | 192 / 0 / 0 | 0 |
| protocol | 0 | 22 / 0 / 0 | 59 / 0 / 0 | 0 |
| cli | 0 | 67 / 0 / 1 | 200 / 0 / 0 | 0 |
| cmd-multica | 0 | 434 / 0 / 0 | 623 / 0 / 0 | 0 |
| daemon-collaborationrun | 0 | 94 / 0 / 0 | 194 / 0 / 0 | 0 |
| tagged-fake-authorizer | 0 | 2 / 0 / 0 | 9 / 0 / 0 | 0 |
| go-vet | 0 | — | — | 0 |
| go-build | 0 | — | — | 0 |

Formatting: pass across 63 changed Go files.
Server source hashes unchanged: `true`; see `source-before.json` and `source-after.json`.

Exact command/environment metadata is in each `*-command.json` and `*-summary.json`. Raw Go JSONL and derived logs are retained. Every skip and failure includes its output in the per-suite JSON files. `test-inventory.json` maps expected test names to source files.

The initial inventory scanner counted `TestMain` in two unfiltered packages. The coverage summary excludes these package entry points; no Go command or raw result changed. Initial reports are retained under `initial-inventory-reports/`; see `inventory-correction.json`. The service failures independently fail the backend gate. An earlier libpq environment preflight failed before test execution and is preserved in `preflight-diagnostic.json`.

Frontend lint/type checks and production Web/native Desktop verification belong to the parent gate. No real-provider smoke or model behavior trial ran here. The recorded toolchain is Go 1.27.0; this run does not establish the repository CI Go 1.26 toolchain result.
