# Service root-cause continuation results

## Confirmed fixture defect

On current main 9ffa3728, `newKnowledgeInvalidationFixture` ran the real global drainer over every retained pending knowledge scan/hint before creating its observable publisher. That mutates unrelated retained history and charges the next test for earlier suites’ work. Each all_sources scan walks every previously published candidate, one candidate per short transaction. Historical race first invalidation test spent 428.96 s here; neither this mechanism nor that duration proves deadlock or the initial system slowdown.

New RED seeded four unrelated global scans, eight published candidates and one pending hint, then compared complete job/candidate/target JSON before/after construction. The unchanged bootstrap failed exactly because it processed unrelated pending history (test0.60s, bootstrap310.394ms). It is a deterministic ownership failure; current low latency does not reproduce historical428.96s.

Minimal repair reserves preexisting pending jobs in one fixture-owned transaction through existing FOR UPDATE SKIP LOCKED semantics. The production worker/SQL remain untouched. Newly enqueued jobs, current two-fixture fairness work, actual failure/replay tests and original assertions remain visible. Empty reservations release their connection immediately; cleanup rolls back before parent fixture teardown/pool close. The two admin fixture constructors apply the same boundary before mutations.

Independent reviewer caught that the regression’s final drain would itself process package history; its outer fixture now reserves old jobs before creating its own seeded history. Nested cleanup proves its reserved jobs are unchanged and then become processable by their owner.

## Actual verification

All commands use pinned Go1.26.11 SHA25620e07ffb4f08faaf125e413db024b6e988d3ec2799dcb3568a720d14be0ad133 and the current CLI guard. Narrow diagnostic commands use p1/parallel2/GOMAXPROCS2 and5m bound; this does not change the original full40m budget. Raw admissions, source hashes and receipts are under `.omx/private/root-causes/service/`.

| New run | Actual result | Scope |
|---|---|---|
| baseline-subset-01 | exit0,19named/5top PASS,24.636s package | unchanged original checkpoint/stage-revocation/purge/fanout/consumer matrix |
| fixture-red-01 | exit1,1named FAIL,1.825s package | deterministic old bootstrap mutates unrelated history |
| fixture-green-01 | exit0,2named PASS,5.007s package | first repaired regression incl nested cleanup release |
| invalidation-green-02 | exit0,29named/20top PASS,10.557s package | final corrected entire invalidation file incl fairness, source locks, crash/replay, relay failure, admin flows |

The attempted scheduling interruption after RED found no owned live process and sent no signal; raw test failure is authoritative.

## Complete supported Linux runs and remaining platform boundary

Full normal/race used separate freshly migrated rc_service_normal01/rc_service_race01, dedicated Redis (fixtures actually use12), the same9073-file frozen current source and unchanged40m/p1/parallel2. Each completed **4631 named PASS /1891 top-level PASS /0 FAIL /0 SKIP**, exit0, zero missing/unfinished/duplicate/parse failures. Actual package durations1009.990s normal and2066.422s race (runner wall includes compilation). Frozen bytes/config/tool remained unchanged. `go vet -p2 ./internal/service ./internal/handler` exit0; formatting/diff checks clean.

The default-policy Linux/arm64 Go1.26.11 container executes as uid1000 and shares the task PostgreSQL network namespace, avoiding the measured host published-port delay. Actual compiler SHA2562c77b9243b17c9d5ecd5983acbdeaee36d470a9b62a2952f1279c319cc9b97fb, official image digestsha256:ea47ee45f1284d60f9cc09cb2c3bb4f508c256b640bd8bb30342ff13d1adcb52. No privileged/capability/security-policy relaxation.

Normal overlapped CLI04 and bounded Go/frontend builds/Host order preparation; race overlapped subsequent Host/Mac work and bounded handler contract compilation. These successful assertions are valid, but total durations are not controlled benchmarks. Per-run concurrency notes retain context.

Linux normal/race source is the initial frozen server plus fixture repair; later request-logger and upload test files have separately bound owning-layer checks, not silently folded into this full result. Darwin full package was not re-executed: the observed published-port transport boundary remains an environment limitation, and repeating an unchanged unremedied path is not warranted. Linux CI acceptance is established; original Darwin failure records remain.

Final scoped DB audit: all4 databases clients0/issues0; normal/race each retain1647 knowledge invalidation rows, diagnostic55, handler0. No history deletion. Exact owned runner/Redis are stopped and retained; PostgreSQL stays under parent ownership. Public selected receipts: artifacts/qa/main-root-causes-20261125/service/. Complete raw evidence: .omx/private/root-causes/service/linux-{normal,race}-01/.

The historical55P03 holder remains unknown. Later autovacuum samples were not simultaneous with the failed call. No Go DATA RACE was present. New narrow original wrong_consumer PASS does not close historical full-package failure. Unchanged checkpoint/stage/purge/fanout currently pass rapidly in a quiet new cluster; separate CLI timing work distinguishes current transport/query timings and cannot reconstruct a missing historical trace.
