# Workspace deletion manifest fix

Updated only `server/internal/handler/workspace_delete_manifest_test.go` and `.trellis/spec/server/collaboration-knowledge.md`. The existing teardown implementation was already correct; the table ownership manifest omitted the three knowledge tables.

- Candidate: **keep** immutable source-owned versions and recovery routing.
- Invalidation: **keep** pending and completed scan/hint history, including after target deletion.
- Target: **delete** counters only, through existing `DeleteKnowledgeTargetsInTx` after final hints are retained under the parent delete fence.

The original schema coverage failure was reproduced on guarded9eee DB. Classification coverage, the new actual workspace-deletion history regression, and existing all-target final-hint regression passed normally and under race (three tests each, no skips). Handler `go vet` and scoped whitespace checks passed. The new test proves candidate/pending-scan/delivered-hint bytes remain unchanged, one final revision-2 hint survives counter removal, and source workspace deletion remains refused.

No production handler/query/schema/sqlc/cascade or deletion behavior changed. Raw red/green/race logs, commands, exact hashes and patch are retained here. Main owns independent review and restaging the previously staged spec. No more source edits planned; no commit or Provider call.
