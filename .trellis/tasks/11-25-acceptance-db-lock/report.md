# D02 service lock investigation — HOLD

The historical SHARE NOWAIT failure is **not reproduced; its holder remains unknown; no product fix is claimed**. The final supported-toolchain normal and race runs both hit the unchanged40minute test timeout and are incomplete. No normal03 execution is authorized yet, and it remains fresh.

## Result and boundary

This lane changed only its Trellis planning/report/context artifacts and new evidence/private diagnostic tools. It made no service product/fixture changes, no guard/deadline/retry reductions, no dependency edits, no global PostgreSQL/autovacuum changes, and no commits/push/deployment. Other agents own the handler/runtime/dependency/skill edits. The parent-fixed embedded skill budget issue is recorded separately below.

## Original evidence and actual lock observation

Original failure: 2026-11-24T02:49:02.224Z PostgreSQL55P03 at `LOCK TABLE github_pull_request, issue_pull_request, issue_vcs_pull_request, vcs_pull_request IN SHARE MODE NOWAIT`, followed1ms later by legitimate M1 grant setup failure in `TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt/wrong_consumer`. The wrong-consumer denial assertion was never reached. No Go DATA RACE warning. Original holder identity was not recorded. All original raw hashes in original-evidence.json were rechecked unchanged at final audit.

The fresh full Go1.27 diagnostic captured PID2585, backend_type=autovacuum worker, granted ShareUpdateExclusiveLock on github_pull_request at2026-11-25T10:19:21.200765Z, query `autovacuum: VACUUM ANALYZE public.github_pull_request`; same PID held issue_pull_request at10:19:21.212226Z. These are actual pg_locks/pg_stat_activity witnesses of a conflicting mode, **not correlated service-refusal evidence or attribution of the historical holder**. The original wrong_consumer passed at10:21:51.685Z, outside those samples. Its complete four-case matrix also passed in both subsequent normal runs. Final race timed out before reaching it. Preserve the production fence: existing PR writers lack a common issue-row lock, so table SHARE NOWAIT covers newly inserted links and returns conflict on contention.

## Complete commands and inventories

Every full command used the current frozen `scripts/go-test-with-agent-cli-guard.sh`, then fixed Go `test -json -count=1 -timeout=40m -p 1 -parallel 2 [-race] github.com/multica-ai/multica/server/internal/service`. Exact argv, tool/source/config/migration/resource hashes, start/end and exits are in each public `*-admission.json`/`*-receipt.json`. Independent parsing checks build/test event types, one named run/terminal, package terminal and static top-level inventory; missing tests remain gaps.

| Run | Exit | Named PASS / FAIL / SKIP | Started unfinished | Static top-level not reached |
|---|---:|---:|---:|---:|
| baseline-race-01 | 0 | 4611 / 0 / 18 | 0 | 0 |
| final-normal-01 | 1 | 4627 / 2 / 0 | 0 | 0 |
| final-normal-02b | 1 | 3293 / 6 / 0 | 16 | 590 |
| final-race-01 | 1 | 2620 / 4 / 0 | 18 | 875 |

- baseline-race-01: original Go1.27.0 full diagnostic, preliminary HOME/source inside Git (explicitly diagnostic-only), external trusted scratch;18Redis tests skipped because REDIS_TEST_URL was absent. Four dynamic recipient names differ from old run; exact identities retained. Does not satisfy final supported-toolchain coverage.
- final-normal-01: Go1.26.11, external source/HOME/TMP, Redis configured; all1,890top-level tests completed. Only failure was embedded `multica-working-on-issues` body504lines over500. Parent skill owner shortened it. `go vet -p 2 ./internal/service` exit0. Corrected exact `TestBuiltinSkillsConformToTemplate` exit0 before next full pair.
- final-normal-02b: corrected snapshot; two checkpoint revoke_grant leaf timeouts, two stage-source-revocation setup failures (`stage_evidence_unavailable` after125.23s; `stage_expiry_exceeds_limit` after171.49s), plus their parents. Full timeout in `TestIssueBudgetLifecycleRoundFindsExactlyWhatAReconciliationCloses`. No matching PR-table lock error at stage failure11:41:32Z. Full raw timeout stack retained.
- final-race-01: corrected identical snapshot, **no continuous lock polling**, by parent agreement. Checkpoint matrix that failed normal02b passed all6leaves in6.20s (revoke leaves1.07s/1.01s). Knowledge purge human/admin and fanout nonblocking checks returned context deadline errors, plus purge parent failure. Full timeout in stage M1 enforce/submission/verified. Original wrong_consumer NOT_REACHED. No DATA RACE warning. Wrapper/recorder exit0 means receipt recorded; actual Go exit1 remains authoritative.

Supported tool is Go1.26.11, verified security provenance binary SHA256 `20e07ffb4f08faaf125e413db024b6e988d3ec2799dcb3568a720d14be0ad133`. Corrected export includes9,068paths (server/scripts/examples/apps/packages and root metadata), manifest `97008c37b375a78072bd91d0e85f789a73ce7e66a389fd4478162aee03f37687`. Normal02b/race01 use original GOFLAGS=-p=2 -mod=readonly, GOMAXPROCS=2 and explicit POSTGRES_DB. All97external module source manifests, frozen source, config and Go binary checked unchanged after both failed runs. Late parent handler/skill prose changes are separately hashed in late-server-delta.json with parent-owned focused conformance/handler coverage; no full-service claim is silently rebound to later bytes.

## Bounded diagnosis and limits

Normal02b continuous monitor was50ms/max20queries/sec; process CPU2.25s over13m57s. Sampled PG state idle/ClientRead; completed query latencies were not instrumented. Three separate audit SQL roundtrips80.6–103.9ms. A host snapshot showed load42.30/VM448.5%CPU and another backend race process. These establish concurrent load, not causation. Race excludes continuous polling yet also slows and times out, so monitoring alone is not a proved cause.

Race `TestKnowledgePendingChangesDoNotSignalTarget` passed after428.96s. Its fixture intentionally drains older retained knowledge jobs through the real bounded worker. Actual read-only snapshots:54pending all_sources jobs at12:17:12Z;52pending and2done at12:19:04Z, active CompleteKnowledgeScan and no blocker. At12:29:31Z database41.4MB,244retainedcandidates,63targets,621jobs. Over56s,10,488commits/80rollbacks and zero deadlocks; these are transaction counts, not per-query counts (pg_stat_statements absent). Actual connection snapshots have no blocking PID, while sharedPG CPU192.43%includes all parent databases. No proof yet identifies the system-level slowdown or a safe query/fixture repair. Do not label every timeout a lock bug or infer that increasing resources would fix it.

No repeated full run is queued. Parent reserved normal03 but later explicitly conditioned it on a reasoned changed-condition plan. If continuing, first isolate the new knowledge/purge/fanout and checkpoint failures on a separately allocated bounded normal diagnostic with identical source/guards and recorded ambient workload. Any new full normal/race acceptance must preserve these failed runs; a focused pass cannot substitute for it.

## Harness failures retained

Initial read-only psql URL-through-PGDATABASE connection failed before mutation; explicit parsed PG host/port/user/database then verified identity. Initial standalone monitor child did not survive its short parent command; the actual full diagnostic monitor ran under the long-lived runner and returned exit0. Duplicate normal01 DB create request refused because parent had already provisioned it; no data changed. Initial Redis bind-mount startup failed before container creation (Docker host cannot see worktree path); create/copy/start succeeded with same existing pinned image. `final-normal-02` admission failed before any test because JSON manifest insertion order changed its serialization hash despite identical9,068path/byte/mode maps. Failed admission retained; corrected recorder copies original manifest bytes after equality check. `normal02b` used the still-fresh allocated DB. No test retry masked any failure.

## Resources and follow-ups

Final SQL audit: diag01, normal01, normal02, race01, reservednormal03 each zero clients, all877migration entries. Failed normal02 retains2issues/1workspace; race01 retains2issues/2workspaces due timeout cleanup interruption. Preserve those DBs as evidence; no global cleanup. normal03 has0issues/0workspaces and was not used.

Dedicated Redis (fixtures force DB12 regardless of URL): container `f6528c817c5ccc37251e069b4baca243c90d0ce8a815dd4f3375b06c16d103d2`, loopback51112, owneracceptance-fixes-4b9d3494a2/lanedb-lock, pinned existing Redis7-alpine image. Fresh random password stayed private; AUTH/PING verified before tests. Exact ID/image/owner checked before docker stop, exit0, running=false. Stopped container/config retained; sharedPG remains parent-owned. No test/monitor processes remain in this lane. Final-resource-audit.json and redis-stop-receipt.json record cleanup.

**Release HOLD:** D02historical cause unknown; supported final service normal/race incomplete and new unrelated failures unresolved. Parent owns whether to authorize a new bounded diagnosis after coordinating workload. No speculative code repair is proposed.
