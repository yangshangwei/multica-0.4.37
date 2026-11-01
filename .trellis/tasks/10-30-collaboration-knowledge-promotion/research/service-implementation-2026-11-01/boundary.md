# A2 implementation boundary

Gap: A1 provides inert candidate storage and pure contracts, but no authorized human writer/read projection or trusted consumer resolver. Add transaction-owned propose/correct/approve/reject/withdraw and own-receipt recovery; all private human projections share current source availability and a finite paged lease. Add transaction-borrowing consumer resolution, exact manifest revalidation and bounded metadata lock hints for C.

Owned: new service/collaboration_knowledge*.go/tests; new handler/collaboration_knowledge.go/tests; new query file only after A1 review release, with sole sqlc generation. Router hookup needs explicit main handoff. Do not edit A1 reviewed files before release.

Main confirmed split: F3 source scan producers, worker/drainer, relay and existing purge/delete/ACL/target-deletion hooks belong to another owner. A2 only emits direct target hints atomically with its human publications. No runtime/task/daemon/agent/Core/views edits, no producers injecting knowledge, no new dependencies, no commits or Providers.

Reuse existing actor authentication, source access, source lock order, operation ledger, strict protocol and original records/versions. Tests use dbfx on the exclusive b3c35f9c DB, service then handler suites sequentially. Preserve existing base context and original uncertain request keys.

## Approved shared-file additions

Main granted sequential router ownership for seven human account routes and project/workspace knowledge GETs; C2 daemon routes were preserved and the router window returned. Main also approved server/sqlc.yaml reserved-column json:- overrides, generated model/result protections and serialization inventory tests. Column-name wildcards are required because sqlc CTE-derived task rows lose the original table name.

Current PAT provenance was missing from cached principals. Main approved auth/principal.go + middleware/auth.go to carry a nonserialized PATTokenHash, new auth/pat_principal.go and a narrow ResolveIssueAccess check for reused private source readers, and knowledge-specific fresh read/recovery/locked-write checks. JWT semantics and global cache behavior remain unchanged. One fabricated PAT recipient fixture now creates a real token.

C2 discovered already-unavailable source candidates blocking fresh preparation. Metadata-only potential-availability probes now omit those fresh candidates, exact histories remain strict, and ValidateKnowledgeResolutionLocks prevents a source absent from discovery from later becoming an unfenced contribution. A source disappearing after discovery still refuses; mandatory consumer fences remain C2-owned.

No formal PRD/design/task status edits were made. All database tests explicitly load the owned b3c35f9c environment. The failed source-project race was a test-only barrier placed after a shared actor lock; it now interleaves at the reachable actor boundary and verifies real PostgreSQL blockers in the opposite ordering.
