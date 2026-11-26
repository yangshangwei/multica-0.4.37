# Research: Independent review of continued root-cause plan

- Query: Are the three diagnostic lanes sufficiently discriminating and inside the user constraints?
- Scope: internal; parent proposal and current evidence
- Date: 2026-11-25

## Findings

Accept the three-lane plan, subject to the following concrete boundaries. Service fixture work, CLI request/pgx timings, and Host status classification are independent questions. Their shared slow appearance does not prove a common cause. Serialize heavy full runs; keep fresh DBs and pinned tools, own Redis per package, external HOME/TMP, original40m service and240s/ten-operation CLI thresholds. Retain original outcomes and old raw evidence unchanged.

Service should first prove old fixture startup/drains mutate unrelated retained invalidation history, then preserve that history while allowing new test-owned work through the unchanged production worker. Full-package normal/race remains required; history isolation cannot close historical NOWAIT attribution or unrelated checkpoint/expiry failures by itself.

CLI should measure completed pgx call timings and client gaps tagged with request/backend identity. A 22s transaction dominated by ClientRead cannot by itself identify a slow SQL, lock, pool wait or host scheduler. Use request IDs and exact source/build/config bindings; don't sum overlapping HTTP/Docker times or expand pool/timeouts.

Host raw-log classification is now available in runtime-503-root-cause.md. Its deterministic controls show attempted503 can be logged after client cancellation and paired realtime hints cause separate cancellation/refetches. Next bound run must join client outcome to server context/code, retaining attempted status; don't suppress genuine503 or assume all historical503 were client aborts.

Native preparation must capture current protocol state before any preparation or launch. Default knowledge ON requires its independent full journey while retaining OFF assertions. Namespace and ordinary upload need the explicit environment/product dispositions in runtime-environment-contracts.md.

## Caveats / Not Found

This is a plan and mechanism review, not a clearance for a particular build or a release recommendation. All actual repairs require source-bound regression and independent final review. User has withheld commit/push/merge authorization for this round.
