# Research: C2 snooze observation boundary closure

- Query: Independently verify the sealed observation boundary, three deterministic late-reader controls and complete six-case final03 replay without silently excluding required reads.
- Scope: internal, source and raw evidence review.
- Date: 2026-11-25

## Findings

**C2 CLOSED for the bound snooze-final03 candidate. All six original cases passed once, with zero observation failures and zero excluded requests.** The historical final02 late client_closed fact and authentic PASS terminals remain unchanged; this closure comes from new source/controls/run evidence.

### Why the new boundary preserves observation requirements

- `ObservedRequestLifecycle.admit()` records request identity before response arrival. Sealing only stops admission of newly starting requests; its WeakSet retains all previously admitted identities, including requests whose response arrives after the seal. Finishing a request does not remove it from that admitted set.
- `drain()` repeatedly waits for both admitted in-flight requests and response-reader promises. A reader appearing while drain is waiting remains in the next iteration. Failures remain stored and fail subsequent drains instead of disappearing when a promise settles.
- `SnoozeTraffic` still records every failed admitted read with its original fixed classification and keeps strict response/schema parsing plus `errors=[]`. `body_resource_unavailable` and `client_closed` are not ignored.
- `openSnoozeSession.close()` seals only on entry to teardown, after the test's business assertions. It drains before closing the browser, then invokes `finalCheck()` in the close finally. `traffic_closed` is emitted only after that final drain/error assertion succeeds. `closeSnoozeJourney()` propagates a rejected close after persisting evidence, so a late admitted-reader error now fails the case.
- New requests after that explicit teardown boundary are counted in `excluded_requests_after_seal`, not silently represented as observed. **The actual final03 run excluded zero requests in all nine sessions**, so acceptance of this run does not depend on deciding that any omitted business read was unimportant.

### Meaningful controls

The three retained Node controls pass: an admitted late reader keeps drain pending and propagates failure; a post-boundary request increments the exclusion count without dropping an earlier reader; and the actual transpiled `SnoozeTraffic` class rejects finalCheck for a200 body reader arriving after sealing with a client_closed failure. The last control verifies the failure fact remains and no traffic_closed success is emitted. This directly exercises the previously missed error path.

The frozen/current test-file difference is one explicit `node:url` URL import added after freezing for the Node test context. Observer implementation, lifecycle implementation and business spec are byte-identical to the tested frozen candidate. The latest three-PASS Node log is retained separately; no executable product/observer change is hidden by this test-only import.

## Independent raw verification

| Check | Result |
| --- | --- |
| Playwright | Six cases, each one PASS result/retry0; zero skips, flaky cases, unexpected results or top-level errors |
| Completion | exit0, frozen unchanged, no cleanup errors, original gate booleans restored |
| Observation facts | Zero traffic_observation_failed; nine distinct sealed sessions and nine matching successful closes; every seal precedes its close |
| Exclusions | Zero after-seal requests across all nine sessions; every closed observation_errors list empty |
| Evidence files | All153 listed files rehashed; zero mismatches |
| Frozen source | All142,785 file hashes match; all7,272 listed symlinks resolve inside the frozen root |
| Before/after audit | Exact equality; current freeze manifest matches both receipt pins |
| C1 build linkage | Recorded new Web/Desktop build-receipt hashes match the C2 freeze bindings |

Detailed identities, six original case names, all nine boundaries, control log and hash audit are in `c2-snooze-closure-audit.json`.

## Files found / related specs

- `e2e/fixtures/observed-request-lifecycle.ts` — finite admission, retained request identity and complete reader drain.
- `e2e/fixtures/observed-request-lifecycle.node-test.mjs` — three deterministic controls, including real observer late200 failure.
- `e2e/fixtures/inbox-snooze.ts:230–294,365–376` — observer wiring, explicit seal and post-close assertion.
- `e2e/inbox-snooze.spec.ts` — original six business journeys and unchanged deadline/assertion semantics.
- Public clients `snooze-final03/` and corresponding private raw reports/evidence — new bound replay.
- Private clients `c2-freeze.json`, C1 build records and `c2-observer-node-final.log` — candidate/control provenance.
- Root testing rules and web E2E evidence contracts — retain failures and require actual complete observations.

## Caveats / Not Found

- No tests, adapters, browsers, services, SQL/Docker or Git operations executed by reviewer. Research outputs only.
- This closes C2 and accepts this six-case observation replay; it does not close C1 manual-detail acceptance or infer completion of concurrent handler race/Main work.
- At review time the client progress file records later manual-final02 Desktop capture-geometry failure after successful scope restoration, with full replay still needed. Do not relabel that whole manual journey PASS because its restoration substep passed.
- Final global protocol/resource cleanup after later runs remains separate. No external reference needed.
