# Retained host first-visit failure review

2026-11-25; candidate `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.
Scope: read-only examination of the two retained host runs, their effective
configuration, canonical fixtures/provider code, and the subsequently supplied
cross-client preflight failure. Only this note was created. No tests, services,
database queries, container operations, instrumented replay or source/runner
changes were performed. The diagnosis skill's execution/reproduction phases
were intentionally omitted under the explicit read-only instruction.

## Conclusion

**Both host failures are localized to prelaunch permit acquisition. Root-cause
classification remains UNKNOWN between runtime/API availability and cancellation;
no private provider/configuration mismatch or product lifecycle defect is proved.**
This is substantially narrower than an unexplained missing callback: the daemon
received a claim, never obtained an authorized launch, and successfully reported
that no provider process started. Therefore memory serialization, provider
callback parsing, provider credentials and order-export business actions were
not the failing exercised boundary.

The new cross-client failure is **a separate namespace capability-probe admission
failure**, before daemon readiness or a run claim. Its generic warning does not
establish a shared cause with the host permit failures.

## Exact host evidence

Evidence root below is
`artifacts/qa/main-post-merge-e2e-20261124-080504/runtime-supplement/`.
Both Playwright results preserve the original 30-second `waitForVisit(1)` failure:
`{errors: [], exit: null, count: 0}`. This means the callback had not arrived while
the daemon process was alive; `errors: []` is the fixture's callback/process
collector, not an assertion that daemon API requests succeeded.

| Fact | `memory-runtime-01` | `order-export-01` |
| --- | --- | --- |
| Canonical failing call | `e2e/collaboration-memory-runtime.spec.ts:87` | `e2e/collaboration-order-export-20261109.spec.ts:142` |
| Run ID | `3d89d35a-f416-4f5e-8f50-97c6dbad58bb` | `8e72bd1d-8ea1-4860-9ce6-85f0c7002041` |
| Native readiness | protocol 3, host supervision, preparation 2, config epoch 2 | Same |
| Claim received by daemon | 200; one claim | 200; one claim |
| Permit observation in daemon summary | status **0**, one attempt | status **0**, one attempt |
| API permit access log | `api.log:552`: 15:11:32.931, HTTP 503, 920.901ms | `api.log:2038`: 15:17:12.018, HTTP 503, 9.373416s |
| Terminal/proof received | `not_started` 200; `never_launched_proven` 200 | Same |
| Final launch ledger | `authorized:false`, `process_observed:false`, phase `never_launched` | Same |
| Final daemon state | exit 0; active tasks 0; available slots 1; no capacity violation | Same |

The memory summary is the inline `memory-runtime-handoff-evidence` attachment
in `memory-runtime-01/playwright.json` (also copied into its existing
`failure-classification.json`). Its decoded bytes hash is
`fac2a113ed273d8081dd6935f48a352126bc1db6de0fe8f26c79ca1a81e36f7e`.
The order summary is `order-export-01/results/` →
`collaboration-order-export-dd7b2-preserve-AI-across-transfer-chromium/`
`multi-account-closure-evidence.json`, specifically the developer owner's cleanup
summary. The other runtime owners had zero claims and empty ledgers.

Important distinction: **daemon status 0 is not an HTTP response status**.
`server/internal/daemon/collaboration_host_router_test.go:142–203` records a status
only when `RoundTrip` returns a non-nil response. It does not retain the transport
error. These runs had `hold_receipt_acks:false`, and that fixture's optional ACK
loss applies only to outcome/proof, never permit. Thus the daemon did not observe
the HTTP response that the server separately logged as 503. It is unjustified to
describe the daemon as receiving and ignoring that 503.

`server/internal/handler/collaboration.go:93–99` maps non-domain errors to generic
503. `PermitCollaborationRuntimeRun` at `collaboration_runtime.go:205` forwards
service errors to that mapping without recording their underlying cause. The
access log alone cannot distinguish transaction/DB failure from a canceled
request. No SQL error, request-context error or specific refusal body is retained.

## Why the providers were not reached

`server/internal/daemon/collaborationrun/host_runner.go:75–104` starts stop
observation and calls `supervisor.authorize` **before** host preparation, memory
reads, backend creation and provider execution. `authorize` at line 315 obtains
the permit before authorizing the local ledger. Its error branch records the
stopped state and invokes recovery. Recovery turns an unstarted pending attempt
into `not_started`, then publishes the never-launched proof.

The retained ledgers match this path exactly. There is no evidence of a started
provider that failed before sending its callback. Both canonical providers check
their actual issued context and run-local CLI before connecting to the fixture;
those checks occur later than the failed permit boundary. Increasing the callback
wait or altering provider input would not explain the observed missing permit.

Configuration inspection was restricted to an allowlist of nonsecret keys in
each `config-effective.private.json`. Memory selected the explicit
`runtime-supplement/bin/memory-provider`; order selected
`runtime/bin/multi-account-provider`. Both selected the current native daemon
test executable and Multica CLI. This agrees with canonical intent and
`family_options.py:58–62,135–136`; there is no evidence that either lane accidentally
used the generic host provider. Both native fixtures reached readiness, capability
registration and claim, ruling out the cross-client initialization boundary for
these host runs. `gate_at_browser_start` being false is expected: the canonical
fixture opens/restores its own lease after browser startup; a successful claim
and the fixture flow must not be replaced by that earlier snapshot.

## What explains the delay, and what is still unproved

The logs contain material availability symptoms before the failed visit:

- Memory: final resume returned 200 at 15:11:03.483 (`api.log:448`). A runtime-list
  request had returned 503 after 10.011301s at 15:11:14.010 (`:487`). The eventual
  claim returned 200 at 15:11:31.966 after 6.728354s (`:540`), roughly 28.5 seconds
  after resume. Across the retained log there are 47 HTTP 503 responses.
- Order: the first developer comment returned 201 at 15:16:42.445 (`:1832`). The
  claim returned 200 at 15:17:02.589 after 9.17413s (`:1974`), roughly 20.1 seconds
  later. The permit ended around 29.6 seconds after the comment response. Across
  the retained log there are 175 HTTP 503s and one 500.
- Both APIs used the explicitly configured pool max12/min1. Memory logged pool
  pressure at `:378,484,514`; order logged it repeatedly, including acquired12/
  idle0 at `:1835` and canceled acquisitions at `:1965,2042`. These establish
  pressure/slow-request observations, not a proved reason for any particular
  permit transaction failing or proof that raising the pool would fix it.

Ranked explanations against the retained evidence:

1. **API/DB delay consumes the first-visit window, with cancellation during
   failure cleanup.** Supported by the delayed successful claims, broad 503s and
   status0 transport observations. `collaboration-host-runtime.ts:372–393` closes
   daemon stdin during cleanup; the native harness at
   `collaboration_host_router_test.go:291–293` cancels its run context on EOF.
   Therefore the final never-launched receipt can be cleanup evidence after the
   original wait failure. There is no retained precise wait-start/EOF/transport
   error timeline to prove this ordering, so this remains an inference.
2. **Permit-side API/transaction failure or another context cancellation occurs
   first.** Also possible; generic 503 plus no received response is insufficient
   to select it over explanation 1. The HTTP client timeout is 10 seconds
   (`collaborationrun/client.go:45–50`), and permit replay is three attempts with
   existing backoff (`runner.go:208–243`). Only one attempt was recorded in each
   run; context cancellation can prevent subsequent attempts. The specific
   cancel source was not captured.
3. **A private provider selection, callback or launch configuration mismatch.**
   Not supported: correct provider selection, native readiness, successful claim,
   and no authorized/process-observed launch contradict attributing the failure
   to provider/callback behavior.

A further fixture fragility is visible in order: the native harness registers
once and directly runs its batch poller, without a heartbeat loop. The API log
contains no heartbeat requests. The developer's last registration returned at
15:14:19.564 (`:82`); its claim completed about 163 seconds later, while runtime
sweeps marked one runtime offline at 15:16:43.252 (`:1840`) and two at
15:17:13.245 (`:2043`). These aggregate sweep lines do not identify the affected
runtime and do not prove that freshness caused this permit's 503. Memory's first
runtime was only about 102 seconds past its last registration at claim completion,
so freshness alone does not explain both failures.

Diagnostics were absent in both effective configurations. The native harness
uses an `io.Discard` logger by default (`collaboration_host_router_test.go:79–84`),
and even its optional observer omits attributes/error causes. An empty stderr
cannot establish a successful lifecycle or supply the missing cancellation cause.
No root-cause pass, resolved defect, harness-only exemption or justified deadline
change follows from this review. Both original failures remain FAIL.

## Cross-client comparison

`runtime/cross-client-01/playwright.log:15–16` records
`managed_identity_or_execution_capability_required` followed by
`native collaboration poller was not admitted`. The failure is at
`e2e/fixtures/collaboration-cross-client-execution.ts:289`, waiting for **ready**,
not the later provider-visit assertion. Its API log shows the actual Linux daemon
registration succeeded at 15:23:59.533 (`:250`), but no run claim/permit follows.

The current `collaboration_router_test.go:80–105` sets `d.managed`, enables managed
transport, installs the issued credential and checks registration before calling
`initializeCollaborationPoller`. In this path, the combined warning at
`collaboration.go:154–163` therefore localizes the failure to
`NamespaceController.Probe(ctx)` being unsuccessful, rather than proving bad
managed credentials. `controller.go:107–121` discards the detailed probe cause
when the helper exits unsuccessfully, and initialization does not log that error.
The retained evidence does not discriminate executable/root preparation from
clone/mount/helper setup refusal.

This is not the backend Linux lane's network-none/capability-dropped probe. The
canonical cross-client fixture and its reviewed adapter specify different
container/image/security settings; the wrapper pins ownership/digest while
preserving that fixture command. No settings were executed or changed in this
review. The backend probe's EPERM must not be imported as this run's reason.
The common symptom is a timeout, but these runs fail at different protocol
boundaries. No shared concrete cause is established.

## Evidence integrity and remaining handoff

Both host `completion.json` manifests were checked read-only against every
listed evidence file and matched. Their `source-end.json` records the same
candidate/fingerprint and no generated source differences. The previous
failure-classification files and all logs remain untouched.

| Retained file | SHA256 |
| --- | --- |
| memory `playwright.json` | `c99c158c7e2dd6f57c3fa8efa92bda32df2f6a6bd0f45966b37f3a01ac8b84cb` |
| memory `api.log` | `454c14141a65ec422756e715ea752d0f7d100f000f6d8ea59c6b07fec28631b9` |
| order `playwright.json` | `a6dc05a09efac90210489722a0cbfef508058aa77bc4a9ab4bb7967cbbd1a8fa` |
| order `api.log` | `c59b96b8b9416fbbfe8c2507b0301f37af142ec90ed50be9a505035f4b4761f5` |
| shared host `source-end.json` | `382690cb838b7f922cd453dd5c730549936f0dd2fab6d629b6b6984dafd4e36b` |
| cross-client `playwright.json` | `14de0b21381b53c29f99f1ebb6bf76f99791e367e6becd332f2c39e44cfb7177` |

For a later separately authorized investigation, the missing discriminating
evidence is the sanitized permit transport/context error and monotonic
wait-failure/daemon-EOF ordering, plus the namespace probe's actual error for the
cross-client lane. Existing pool-pressure/sweeper logs are useful correlations
but cannot replace those facts. No extra run or instrumentation is proposed as
part of this bounded sidecar.
