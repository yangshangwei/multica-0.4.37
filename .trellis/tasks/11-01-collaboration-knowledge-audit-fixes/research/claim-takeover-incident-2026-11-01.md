# Claim takeover environment incident (2026-11-01)

Status: actual isolation violation; preserved, no rollback attempted.

At 23:14:00–23:14:02 +0800 the claim implementer ran `go run ./cmd/migrate up` from `mc-kfix-lane-claim/server` without setting `DATABASE_URL`. The inherited environment did not contain that variable. `server/cmd/migrate/main.go:867–869` falls back to the local database named `multica`. This was an avoidable command construction error, not an authorized test database operation.

The command exited 0. Its output showed historical skips and forward migrations through `763_collaboration_knowledge_delivery_history`, including the three new history indexes. By the time the missing environment was detected, the command had already completed. No follow-up destructive command, down migration, test suite or cleanup was run against this database.

A bounded read-only check through the existing PostgreSQL container found `max(version)=763_collaboration_knowledge_delivery_history`, with latest `applied_at=2026-11-01 15:14:02.393819+00`. No pre-command schema snapshot was captured, so the exact initial ledger and preexisting data cannot be reconstructed from that check. The tool transcript retains command/stdout; it reported earlier already-applied versions and newly applied versions, but there is no independent pre-migration backup collected by this task.

The coordinator was notified immediately through the agent channel. Channel payloads received by this child are empty, so this report also serves as a durable handoff. The real-model budget is unaffected; no Provider was called.

Recovery boundary: preserve the upgraded database and migration ledger. Do not try speculative reverse migrations or restore business rows. Further verification runs must use explicit task-owned database names and record identity before launching commands. The separately created `kfix_claim_svc_green_20261101` database is an untouched clone of `kfix_template_20261101` as of this incident.
