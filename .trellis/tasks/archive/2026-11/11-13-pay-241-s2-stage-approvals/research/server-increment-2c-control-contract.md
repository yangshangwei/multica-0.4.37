# S2 increment 2c internal source and test-control contract

Frozen 2026-11-14 by the sole server owner after the accepted 2C-R01–R03 review.
This is an internal server/test boundary. None of these values are decoded from
normal HTTP, runtime, plugin or CLI requests. Production construction supplies
no source by default, so release/rollback execution is unavailable.

## Trusted source and immutable identities

One purpose-specific `StageExecutionSources` dependency is supplied by trusted
server construction, forwarded by `Handler.stageService()`, and later by every
2d writer's common intent/guard. No global mutable resolver, caller context
impersonation, public verifier registration or simulation switch exists.

The dependency resolves configured deployment targets and registered metric
sources under the caller's actual `pgx.Tx`; it authenticates already captured
consumer/observation evidence under current trust locks. It never decides stage
policy. Remote preflight/capture and the execution callback run outside database
transactions. Normal data cannot provide an authentication function or source
implementation.

A target retains non-secret adapter name/version, local integration UUID,
trust/config revisions, stable remote instance/tenant/target identity,
environment and provider conditional state. `target_digest` is the canonical,
domain-separated digest of that reviewed versioned identity. The **physical
conflict key** is a separate domain-separated digest of the stable provider
concurrency domain, excluding workspace/root IDs, local integration bindings,
friendly names, trust/config epochs and conditional versions. A resolver that
cannot establish aliases' shared physical domain is unsupported. The global
unresolved-target query and transaction advisory lock use that stable key.

Code PR, deployment and metric source identities are distinct. Metric source
identity includes the registered query/rule IDs and digests, actual ordering
contract and finite freshness deadline. A source without reliable ordering or
conditional deployment/original-query semantics is unsupported. S2 invents no
payment threshold, metric samples or production provider configuration.

## Internal method and result contract

- `PrepareStageExecution(ctx, request, sourceProof)` accepts exact source
  workspace/root, candidate ID/version/digest, approval decision UUID, original
  request UUID and target digest. Kind/build/original deployment/metric reference
  are derived from the selected immutable approval. The registered source
  separately authenticates the real integration consumer. It returns only a
  content-free operation receipt, never an executable permit.
- `RecordStageExecutionObservation(ctx, intentID, sourceProof)` authenticates an
  original-operation observation against the retained intent and current
  source trust. It does not require the former approver's current credential.
  Payload binds native observation identity/version, original external request,
  native operation ID when known, target, actual before/after artifacts, result,
  observed time and bounded redacted evidence digest/summary.
- `StageExecutionReceipt(ctx, original identity/request, sourceProof)` is
  historical lookup only. Exact request/digest recovery is checked before new
  admission. It cannot reconstruct a callback opportunity.
- `RecordStageRollbackObservation(ctx, deploymentFactID, sourceProof)` captures
  an authenticated metric observation: exact original deployment/target,
  registered metric/query/rule identity, canonical positive native ordering
  version, observation window/time/deadline, condition and bounded summary.

Minimal operation receipt:

```json
{"version":1,"issue_id":"<uuid>","request_id":"<uuid>","intent_id":"<uuid>",
 "approval_decision_id":"<uuid>","kind":"deployment","state":"pending","fact_id":null}
```

Observation receipt additionally identifies its immutable observation UUID;
metric receipt identifies observation UUID, positive server append version and
canonical digest. These internal receipts contain no proof bytes, target source
secrets or replayable invocation value. Public discovery/history is only R4's
frozen 8.10 projection.

## Admission and one-shot callback

Admission locks current users/workspace/catalog/root using the existing order,
then exact source/build/approval dependencies, current registered consumer and
stable physical conflict domain. It performs slow observers before the final
DB clock/issuer/source checks. Unresolved history on that physical target blocks
new intents across local roots, bindings and workspaces with an anonymous error.
One intent per approval and original source request is enforced in the database.

After the intent **definitely commits**, the service itself owns synchronous
entry into the registered callback. No handle escapes. Its private atomic entry
guard can be consumed only once; repeated calls, lookup or restart get history.
The stored entry deadline is finite and bounded by the earliest consumed
approval/evidence/source-proof expiry and a 30-second server entry cap. The
post-commit check uses elapsed monotonic time from the final DB-clock reading,
so clock offset cannot extend the remaining window. It bounds callback entry,
not the eventual completion time of an operation already entered.

| Event | State/effect |
|---|---|
| Intent commit failed/ambiguous | Zero callback entries; no opportunity returned; any retained intent stays unresolved |
| Sole live guard sees cancellation/expiry before entry | Retire entry; zero calls; internal-only `not_started` may be appended with null native operation ID |
| Pre-entry finalization failed/ambiguous | Unresolved; never restore entry or assert successful persistence |
| Process disappears after commit | Pending/unknown; absence of a guard is not proof of no execution |
| Error/cancellation/panic/transport uncertainty after entry | Unknown unless authenticated provider evidence proves a definite outcome |
| Authenticated original-operation lookup | Append/recover history only; never call again |

Local pre-entry `not_started` and post-entry uncertainty use the original
service-owned guard and a bounded five-second finalization context even after
caller cancellation. They have no public assertion path. Pending/unknown/
not_started are operation history, not execution facts. A definitive zero-start
or failed terminal releases uncertainty for a fresh explicit approval after
current target checks, but the consumed approval remains nonrevocable.

At most one definitive terminal observation belongs to an intent. Equivalent
native terminal identity/content recovers the existing fact; conflicting
content refuses. Unknown can precede a definitive result. Success must observe
the intended artifact. Failed facts name the intended artifact without claiming
it became live. Timeouts never manufacture failure or permit a retry.

## Metric ordering and late results

The stream key binds original deployment fact, original target, stable metric
source and registered query/rule identity. Every distinct authenticated capture
is retained with a server append version. Current selection follows **native
ordering**, not arrival time. Equal native version and different canonical
content makes the stream ambiguous in durable history; both arrival orders
invalidate old anomaly approvals. Older events cannot clear it. Only a strictly
newer authenticated observation resolves ambiguity. A healthy/unknown newest
observation, expiry or current source/rule drift blocks rollback.

Historical authenticated execution results may arrive after approval expiry or
revocation constraints change. They remain exact history; A9 and 2d still require
current valid approvals and source dependencies. No renewal, endorsement of an
existing deployment, second invocation, automatic stage advance or fabricated
human actor is introduced. Rollback is verification-stage, immediate-previous-
artifact only, with provider conditional state for the original deployment.

## Test-only actual-router control surface

A `cmd/server/*_test.go` executable may construct the actual router/auth and
required outbox/realtime/background services with one explicitly injected
controlled source. Its private JSONL stdin/stdout pipe is separate from HTTP.
No human approval or execution fact is seeded. Public A7/A9 signatures remain
real authenticated actions; a complete browser journey waits for legitimate2d
activation. Root allocates a dedicated router/harness database before execution.

The private controller is a closed union with an outer `request_id` UUID and
`action`; all replies echo it and return only safe result identity/state:

- `source.configure`: bind this test's workspace, exact source integration UUID,
  environment and stable remote instance/tenant/target names; select the
  controlled source's current conditional version, artifact and finite metric
  rule. The harness checks real database integration trust and source fences.
  These are synthetic source fixtures, not HTTP `verified` assertions.
- `execution.prepare`: `issue_id`, `approval_decision_id`, `request_id`,
  `candidate` and `target_digest`; the controller obtains its source proof from
  the injected source and calls the production method above.
- `execution.observe`: `intent_id` and a controlled native observation key/
  version; the source captures/signs its configured result or answers its
  original-operation lookup before the production recorder is invoked.
- `metrics.observe`: `deployment_id` and a controlled native observation key/
  version/window/condition; the source authenticates the capture against its
  registered rule before the production recorder is invoked.
- `source.behavior`: set the next controlled callback to definite success,
  definite failure, proven not_started, unknown, panic or delayed entry/result.
  The fixture records exact call count, external key/target/artifact/conditional
  binding. It performs no real deployment and has no production counterpart.
- `execution.receipt`: recover the exact internal original request through the
  production historical lookup; no callback occurs.
- `shutdown`: stop only this harness's owned listener/background workers; retain
  scenario history. Account for the separate default TestMain listener too.

The final test harness input struct may add only mechanical fixture transport
fields needed to address these operations; its exact source and executable are
part of root's acceptance binding. It cannot add public routes or select a
production simulation mode. Actual S4 provider idempotency, conditional-write,
metrics and deployment/rollback evidence remain NOT_RUN.
