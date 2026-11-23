# Manual policy implementation

Owned scope: new Server policy service/handler and their tests;
`collaboration_queue.go`, `collaboration_batches.go`,
`collaboration_runtime.go`, `collaboration_control.go`,
`collaboration_authority.go`, and `collaboration_mentions.go`.
Root approved this exact ownership on 2026-11-03. Other sessions own shared
workflow/lifecycle/delivery, generated SQL, routes, creation and access wiring.

The gap is persisted manual policy without new-input/runtime admission fences
or an initiator-owned writer. Reuse the existing workflow transaction and
whole-issue Stop helpers; preserve waiting input, grants, native identity and
unconfirmed termination evidence. Restoring assisted must leave controls paused.

Verification starts with canonical service regressions for manual input,
Resume, runtime promotion/Claim/Permit, followed by the actual policy writer,
original-key recovery, authority, atomic failures and Stop/proof behavior.
All database runs use the coordinator's exact `run-package.py --lane service`
permit and are serialized with root. No provider accounts or native execution
are part of this slice.

## Evidence

Pending narrow red/green execution.
