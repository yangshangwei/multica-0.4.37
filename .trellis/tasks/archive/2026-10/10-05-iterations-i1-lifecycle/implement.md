# LG lifecycle execution

2026-10-06; approved parent plan, FG passed; release flag stays off.

1. Add real PostgreSQL service tests for create/edit replay, saved timezone,
   complete move preview/atomicity, immutable start baseline, terminal choices,
   single active, membership leave/reentry, planned cancel and unused delete.
   Record RED before implementation and GREEN after each cohesive slice.
2. Reuse RunOperation and caller authorization for durable writes. Preview owns
   one RR transaction. Lock all affected iterations, issues, current grants and
   display references before the one business-time sample. Full sets never
   depend on pagination. Preview limits reject rather than truncate.
3. Add borrowed membership prepare/commit helpers without transactions/retries,
   preserving caller lock ordering, original facts and cumulative rollover.
   Parent integrates T1/create using these helpers.
4. Capture full originals at start through HG display resolution; emit one start
   marker then baseline facts. Planned activity never becomes scope growth.
5. Parent owns SQL generation, HTTP routes and integration; HG owns read/history
   projection. No edits to shared SQL/generated or other agents' files here.
6. Run guarded focused DB tests, race tests and build/vet. Report exact evidence,
   remaining CG-only operations, and integration gaps; never infer LG from mocks.

No CG end/active cancel/handoff/disable, clients or rollout changes are included.

## Ordinary issue assignment integration (parent-authorized follow-up)

1. Add real HTTP RED tests for create target confirmation, atomic membership,
   duplicate retry, stale target, pending admission and injected recorder rollback.
2. Reuse Create's current authority callback; lock settings/target before its
   reference locks and insertion. Add a borrowed after-create hook before commit.
   Preserve existing duplicate guard and ordinary assigned-agent enqueue contract.
3. Add membership-only PUT handling requiring issue expected_revision, with
   reason for actual leave/switch and allow_completed for completed active joins.
   Compound ordinary edits return 400; generic batch still requires preview.
4. Reuse W01's existing four-attempt NOWAIT owner/current authority callback.
   Lock iterations before issue, sample once after complete checks, then call
   borrowed membership helper. Explicit branch publishes after commit and never
   invokes execution. Omitted fields retain the existing fast path.
5. Validate release-off rejection, current permissions, stale revisions, terminal
   rules, original retention/reentry, running execution invariance and regression.

POST /issues retains its existing duplicate-guard retry contract and Issue DTO;
this slice does not invent a durable request_id envelope for ordinary creation.

## Integrated acceptance

2026-10-06: LG passed. Commits `1247d2728` and `4c9be7652`; see
[parent integrated evidence](../10-05-iterations-i1/lg-hg-verification.md).
Release remains off; CG is next and foundation remains open through FCG.
