Server integration fix boundary

- Fill runtime source projection from locked issue/session state and correlate the active information request to its own original event; missing legacy provenance remains explicit.
- Reject inapplicable run/version attributes for handoff-note source reads. Keep current business authorization unchanged.
- Preserve full admitted assignment while enforcing actual handoff serialization budgets; budget design is being coordinated with the root before changing limits.
- Share the imported bounded monotonic seal validator with daemon code; verify original-key replay, immutable prior payloads and retained server prefix.
- Own service/handler/protocol tests and sqlc generation only; no daemon/parser/frontend/task-state edits, no provider execution. Preserve all imported WIP.
- Use the exclusive task database sequentially; retain red and green evidence, source hashes and generated drift.
