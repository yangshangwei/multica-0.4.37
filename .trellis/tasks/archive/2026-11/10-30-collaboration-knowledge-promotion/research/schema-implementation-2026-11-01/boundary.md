# Schema/protocol batch boundary

Existing runtime has no candidate entity or managed knowledge contract. Add only immutable candidate storage, independent target counters, content-free scan/target hints, and nullable ordinary-task negotiation metadata; expose pure typed validation/permissions and transaction-bound SQL. Pending proposals have no producer that can affect context.

Owned files: new pkg/protocol/collaboration_knowledge.go and focused tests; internal/collaboration pure knowledge policy/tests; migrations 747 onward (verified baseline maximum 746); new pkg/db/queries/collaboration_knowledge.sql and sqlc output; focused new migration/query tests and this evidence directory. Concurrent index registration in cmd/migrate/main.go is required and has been escalated to the main owner.

No HTTP/service/daemon/core/UI producers, no enablement, no Provider, no dependencies, no existing task metadata/planning edits, no commits. Use a dedicated disposable database, private-schema rehearsals and repository migration tooling. Test immutable content/routing, CAS and approved uniqueness, independent counters, bounded initialization, invalidation routing and rollback guards. Capture a fail-first stage, then fresh/upgrade/reapply/down rehearsals and pinned sqlc drift/static checks.
