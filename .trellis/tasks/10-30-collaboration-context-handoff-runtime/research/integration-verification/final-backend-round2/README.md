# Corrected full backend gate

Result: **pass**.
Candidate: `5b0262606d143ca44fc2ab19247eece7025087cda2541372f67afc218b76245b`; HEAD `e3620721efacb820b89709c8db9d326d9cf2b00b` plus the recorded source diff.
Database: `multica_runtime_final2_858f011e`. Service and handler ran sequentially on this fresh dedicated database.
Environment: `go version go1.27.0 darwin/arm64`; PostgreSQL 17.11. All test commands used the repository ambient-agent CLI guard, `-race -count=1 -p 1 -parallel 2 -timeout 40m`, and `GOMAXPROCS=2`. Real-agent flags were absent.

| Suite | Exit | Top-level pass / fail / skip | Subcase pass / fail / skip | Missing named tests |
| --- | --- | --- | --- | --- |
| service | 0 | 587 / 0 / 0 | 1078 / 0 / 0 | 0 |
| handler | 0 | 163 / 0 / 0 | 192 / 0 / 0 | 0 |
| protocol | 0 | 31 / 0 / 0 | 267 / 0 / 0 | 0 |
| cli | 0 | 67 / 0 / 1 | 200 / 0 / 0 | 0 |
| cmd-multica | 0 | 434 / 0 / 0 | 623 / 0 / 0 | 0 |
| daemon-collaborationrun | 0 | 96 / 0 / 0 | 204 / 0 / 0 | 0 |
| tagged-fake-authorizer | 0 | 2 / 0 / 0 | 9 / 0 / 0 | 0 |
| go-vet | 0 | — | — | 0 |
| go-build | 0 | — | — | 0 |

Formatting: pass across 71 changed Go files.
All 4317 server file hashes unchanged: `true`. Tracked/untracked server file-set parity: `true`.

## Previously failing tests

- `TestCollaborationRunRecordsCurrentAuthorityAndLegacyAbsence`: **pass**.
- `TestCollaborationRuntimePreparationDerivedLateDeliveryDoesNotStarveHealthyRoot`: **pass**.
- `TestCollaborationRuntimePreparationJoinBeforeFirstClaimReleasesOnlyUnclaimedWork`: **pass**.

## Skips

- `TestCLIConfig_UnknownFieldsArePreserved`: === RUN   TestCLIConfig_UnknownFieldsArePreserved     config_test.go:265: documenting known limitation: encoding/json drops unknown fields on round-trip; future PR can switch to a preserving encoder --- SKIP: TestCLIConfig_UnknownFieldsArePreserved (0.00s)

Exact commands, exits, timings, environment metadata, expected test names, raw JSONL/logs, and all failure/skip output are preserved here. `source-before.json` and `source-after.json` bind the run to `final-candidate/source-round3.json`; `evidence-sha256.json` hashes the evidence files.
The first run remains untouched under `../final-backend/`. No product source changed during this run.

Frontend checks and production Web/native Desktop validation are owned by the parent. No real-provider smoke or model behavior trial ran here. This macOS Go 1.27.0 run does not establish the separate CI Go 1.26 or other-native-platform result.
