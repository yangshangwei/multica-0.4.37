# Coordinator manual review

This is a manual review, not a completed independent agent gate.

- Host proof priority remains before telemetry on phaseProof; phaseAcknowledged recovers only existing reports and never reclaims work or starts a Provider.
- All recording requests retain original issued credentials, identity, attempt/nonce and immutable payload/key. Deadline contexts stay under the caller and report window.
- A copied seal value is staged only after the old seal ACK; original pointers are not mutated. Metadata advances are deferred until every chunk resolves, bounding revision use.
- Checkpoint ACK semantics remain exact per-request offsets. Seal retention may reflect later committed facts. Supported nullable bounds must be explicit; valid zero/null and known missing tails are preserved.
- Client validators do not turn recording metadata into termination proof or business authorization. Negative, contradictory, missing and null mandatory values remain errors and preserve recovery.
- The execution-panel mock preserves all records behavior assertions and leaves handoff behavior with its own canonical component suite.
- Real fake-executable Stop and on-disk fresh Runner recovery are demonstrated. Full daemon crash, real accounts/models and final installed/platform acceptance remain separate gates.
