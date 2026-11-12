# Frozen-candidate backend findings

## Records legacy corruption assertion fails

- Test: `TestCollaborationRunRecordsCurrentAuthorityAndLegacyAbsence`.
- Assertion: `server/internal/service/collaboration_records_test.go:186`, "unknown stored payload became a complete recording or escaped redaction".
- Raw evidence: `service-first-failure.json`, `service.jsonl`, and the final service failures/summary files.
- The test inserts an event with `kind='unknown_private'` and a private marker, then requires `RunRecords` to return an error without exposing that marker.
- The candidate query `ListCollaborationRunRecordEvents` in `server/pkg/db/queries/collaboration_records.sql:29` now selects only `outcome`, `comment`, `squad_activity`, and `record_checkpoint`. `unknown_private` is excluded before record parsing.
- The task design explicitly requires an allowlist of displayable events so new internal metadata cannot break existing record readers. The old error expectation therefore appears inconsistent with that change. This failure alone is not evidence that the private marker was exposed.
- Recommendation: the source owner should reconcile the legacy unknown-event assertion with the approved allowlist contract, retain explicit non-disclosure and pagination assertions, and verify the affected records suite on a new source snapshot.
- No product source was modified during this gate.

## Derived preparation scheduling tests fail

- `TestCollaborationRuntimePreparationDerivedLateDeliveryDoesNotStarveHealthyRoot` fails at `server/internal/service/collaboration_runtime_preparation_derived_test.go:51`: "unclaimed derived preparation kept the pending-derived fence forever: healthy=false refused=map[continuation:true member:true]".
- `TestCollaborationRuntimePreparationJoinBeforeFirstClaimReleasesOnlyUnclaimedWork` fails at the same file, line 238: "an unread pre-claim continuation stranded the reserved unsupported root".
- Raw evidence: `service-live-failures.json`, `service.jsonl`, and the final service failures/summary files.
- Both tests ran against the exclusive, migrated backend gate database in the same sequential service process. No handler process or browser fixture used that database during the run.
- Root cause has not been established. Recommendation: inspect derived refusal/continuation release and healthy-root admission after this frozen gate, preserving these results; do not treat either failure as an accepted skip or a provider/environment failure.
- No product source was modified during this gate.
