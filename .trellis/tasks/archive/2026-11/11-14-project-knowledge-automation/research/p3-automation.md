# P3 server automation worker and settlement

Implementation under approved plan2026-11-14-v1. Model/rule helpers and their
unit tests belong to `p0_core_contracts`; source/validity/consumer helpers belong
to `p0_server_contracts`. This lane integrates the durable worker and settlement.

## Runtime shape

`NewKnowledgeAutomationWorker(service, model)` creates one API-owned worker with
two active-batch slots. `Run(ctx)` drives the same `ProcessNext(ctx)` used by
integration tests. Each claim leases one job for180s and processes at most one
fixed batch under120s, with each model stage bounded to60s. Source authority is
derived from the retained actual acceptance-reader UUID, never a synthetic
Principal or a human approval endpoint. Integration tests call ProcessNext with
`context.Background()` without authentication.

The first lease freezes all paused approved automated predecessors, including
expired records, into up to6 batches of3 and one create slot6. Stable batch IDs
and all batch results survive recovery. New independent candidates use the
execution policy's1–3 limit; review cardinality is independent of that limit.

Each stage locks current user/parents/policy/target/sources/candidates, resolves
the exact accepted result and complete authorized comparison, and records its
immutable attempt before an outbound call. Generation sees only allowed source
material and same-source predecessor text. Checking receives the complete
comparison. Real IDs remain internal. Current-execution successful stages are
reused only if request/configuration/source/target/policy bindings match.
Transport uncertainty or a lost running lease is retained, not automatically
regenerated. Explicit retry appends a new execution; bounded DB retries retain
the already completed model stages.

`collaboration_knowledge_automation_snapshot.go` reuses source/validity locking.
Positive support dependencies stay within the candidate's source issue; other
comparison issues appear only in the complete private input closure. Every
comparison's current source Read and availability is checked before use.
Expired predecessors remain explicit review outcomes rather than disappearing.

`collaboration_knowledge_automation_settle.go` holds the same current-fact locks
and rechecks complete snapshot hashes after the model. It fixes target, source,
scope, provenance, exact predecessor and capacity itself. All five existing
lawful project consumer surfaces are included; existing audience/provider gates
still apply. Every accepted new/refreshed statement needs the new accepted
delivery among its support references. Exact duplicates are skipped; malformed
output stays at job level; valid unsupported/conflicting/full-capacity content
is persisted as a pending candidate for human review.

The stable admissible subset, pending outcomes, evaluations, candidate changes,
validity heads, one target revision and hint, and batch receipt commit together.
A failure rolls back the entire batch. Revalidation changes only the head;
revision uses the same exact lineage and supersedes only after the replacement
passes. Human dispositions are checked even for newly allocated candidate IDs.
Previously exposed manifests are never rewritten by this worker.

Public job status reads only current execution usage metadata. Free-text model
reason and full input-source closure remain private; `related_refs` cannot
reduce the recorded closure. Provider unknown/partial/known token presence is
retained, with no invented bill or zero usage on unknown calls.

## Evidence so far

`p3-worker-first-a2a64b2e.json`:6/6 named integration tests passed with stable
source, zero skips/failures and zero owned DB connections. All upstream traffic
used a local fake HTTP server with retries disabled. This proves:

- Actual accepted-job processing saves3 candidates with two model phases.
- A second acceptance restores3 and adds1; a third reacceptance completely
  reviews4 across two review batches plus create, and all4 heads validate.
- Unconfigured work has zero attempts; zero suggestions use one call; uncertain
  failure is retained and is not automatically regenerated.
- An injected failure on the second approval rolls back every candidate and
  evaluation from the batch; retry reuses saved model stages with no new calls.

The later expanded tests cover revision, manual fresh-ID withdrawal protection,
policy/reopen changes during model calls, current-policy explicit retry,
partial capacity, complete private comparison closure/current revocation and
two-batch concurrency. Their subsequent run must be recorded separately; the
first passing run is not evidence for those added tests.

These fake-provider proofs establish orchestration and deterministic boundaries,
not real model semantic reliability or deployed Web/Desktop/daemon acceptance.
Main still owns activation, final integration and platform journeys.
