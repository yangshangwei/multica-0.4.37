# Follow-up: preserve recording gaps

The final review reproduced a locally rejected checkpoint batch followed by queued messages rebased onto the lost batch offsets. This is distinct from recoverable HTTP uncertainty: the latter already has an immutable original payload in the ledger.

Ownership: `host_checkpoints.go` sender staging/drain only, plus new `host_checkpoint_gap_test.go`. Preserve `recoverHost`, its deadline/proof behavior, the frozen combined tests, service/protocol/retention contracts and all other lanes. No dependency, claim/launch retry, database, installed Provider or commit.

Verify the production `hostRecording.message` -> sender -> ledger -> original checkpoint/seal/proof recovery flow. Include invalid first batch, accepted prefix followed by invalid middle and queued valid suffix, uncertain prefix followed by invalid middle, and uncertain prefix with only valid queued messages. Preserve the review red in `review-probe.json` and capture canonical red before implementation.
