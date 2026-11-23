# Research: R23 rollback control reply was rejected after commit

- Query: Explain R22's unconfirmed control request `61b4733a-f70e-424c-8f4b-c337ea9c3f52`, trace its original receipt and identify the minimum owning-layer regression/repair scope.
- Scope: Internal, read-only original result, case evidence, recorded after-DB snapshot, consumed client/source and existing private bridge/harness files. No requests, replay, API/DB reads, process/build/test actions or source edits.
- Date: 2026-11-17
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

### Exact observed failure

The retained R22 original four-case result is **1 PASS / 1 FAIL / 2 NOT_RUN**, retry0. First complete role/review/QA scenario passed in76.407s. The second ran49.857s and reached actual QA, release approval, uncertain deployment, authenticated successful deployment, equal-native-version rollback rejection, and a new explicit rollback approval. It then failed at `StageExecutionControl.send`'s data-parser catch (`collaboration-stage-control.ts:203`), which calls `finish()` and emits the generic original-request uncertainty error at `:189`.

This is a **test-only reply decoder contract mismatch**, not an observed timeout, missing service receipt or lost-ACK transport. The actual harness emitted the same request's successful rollback receipt, and the recorded after snapshot contains its durable intent and successful integration observation. The failing client rejects the provider's valid opaque conditional version as a decimal protocol revision.

Original result SHA256: `a6a9b58fe105b95a5497ac014cb598d43efb1457a96dcd338eb4058a8941f968`. Original case2 evidence SHA256: `b1bc224f6e92de157121f10f97977eba9d5632d6eab78755d0e42e99fceb0d83`. All original outcomes remain unchanged.

### Correlated original reply and durable result

`r22-runtime-capabilities-read-r01/runtime/harness.stdout.log` line24 contains `COLLABORATION_STAGE_REPLY` with:

| Field | Actual original value |
| --- | --- |
| request_id | `61b4733a-f70e-424c-8f4b-c337ea9c3f52` |
| ok | `true` |
| result.issue_id | `01a1e574-a1ac-74de-89bd-9a58186b56ad` |
| result.intent_id | `01a1e575-5feb-7382-a862-81b0f0793478` |
| result.approval_decision_id | `01a1e575-5cf4-71a5-9607-64b9d8f0b9ab` |
| result.kind / state | `rollback / succeeded` |
| result.fact_id / observation_id | `01a1e575-600d-7ccd-800c-cd8ec86aa8ef` |
| calls.count | `2` |
| calls.last.conditional_version | `1/after/01a1e575-3092-7cb9-bfa2-8a28d87745ff` |

The log was24 lines/9,270 bytes at inspection, SHA256 `30e41954b0128cc2ecc5c87416220939df193957629fa25f606d3aaf4ed2aa21`. It belongs to a still-owned API; a later legitimate append, such as shutdown, needs a separate final hash rather than rewriting this inspected observation. The JSON companion binds the exact selected line and payload separately.

Recorded `database-after-four-r03.json` (SHA256 `e6fd17ce87aea0b97021b5299d321201c9ca5fe2dd5a0f7a8435e0ac6f142ff3`) contains:

- `database.stage_intents[3]`: matching request/issue/candidate/approval, rollback intent `01a1e575-5feb…`, admitted at `2026-11-16T17:43:57.162Z` and the exact previous artifact digest.
- `database.stage_observations[5]`: matching intent and observation `01a1e575-600d…`, `origin=integration`, `result=succeeded`, native callback identity and external operation `rollback/provider:95b2633a3e`, created at `17:43:57.198Z`.
- Summary: gate false/true/false at epoch28,27 Host runs settled,12 artifacts complete,3 metrics, one older unresolved stage intent. This recorded snapshot had5 other connections while the owned API was running; it is not a shutdown/zero-connection proof.

The service's observation writer returns a receipt only with the actual `tx.Commit` result (`collaboration_stage_execution_observations.go:89–93`). The harness's `handle:173` emits `ok=true` only on a nil application error. The matching retained durable observation independently corroborates commit. This does not mark the failed scenario passed: its later exact rollback fact projection, original-key receipt, Desktop display and final screenshot assertions were not reached.

### Original request shape and provenance limit

At `collaboration-stage-execution.spec.ts:326–327`, the request copies the earlier retained `original` deployment prepare envelope, replaces request ID and approval ID with the explicitly fresh rollback decision, and sends it once. Case2 HTTP[33] records that fresh decision's actual200 response at `17:43:56.430Z`.

The source-derived request is:

```json
{
  "request_id": "61b4733a-f70e-424c-8f4b-c337ea9c3f52",
  "action": "execution.prepare",
  "issue_id": "01a1e574-a1ac-74de-89bd-9a58186b56ad",
  "approval_decision_id": "01a1e575-5cf4-71a5-9607-64b9d8f0b9ab",
  "candidate": {
    "candidate_id": "01a1e574-a533-7480-9c02-54a3f79dd6a7",
    "version": "1",
    "content_digest": "978dbb13848dbfe044d1e1074776785016a963849346e1d425a0f17b022cfc88"
  },
  "target_digest": "69f136894a3bb3a99dbbba3f55a2644c13f8eff1a73b580fea107e66f229b5f3"
}
```

This is explicitly **source reconstructed**, using retained original fields, fresh actual approval and correlated receipt/DB identities. The original encoded request byte frame is not saved by the inspected client or bridge. Do not label this reconstruction as an original wire capture or send it again. The client failed before assigning `rolledBack`, so the final step that would persist `rollbackRequest` was not reached.

### Client/bridge/harness chain

1. `StageExecutionControl.send:177` encodes a bounded closed request. Its five-second/20-second/480-second product/helper/case deadlines remain separate and unchanged; this control's timer is20 seconds.
2. The task-private bridge accepts the exact request line and forwards it once to harness stdin (`bridge.mjs:95–103`). It preserves bytes and correlation; it does not own stage business decisions.
3. Harness `apply` constructs the actual `StageExecutionRequest` and calls `PrepareStageExecution` (`collaboration_stage_execution_control_test.go:242–269`). Its loop serializes the reply to stdout (`:389–395`).
4. Bridge strips the reply prefix, checks request ID/ok/frame size, finds the original pending socket and forwards the payload unchanged (`bridge.mjs:196–208`). It does not reinterpret `calls.last.conditional_version`.
5. Client `parseStageControlReply:146–148` checks the closed invocation fields and calls decimal `version(last.conditional_version)`. The observed token contains `/after/` and therefore fails `version:49–50`'s positive canonical decimal regex. The data catch at `:203` then reports result unconfirmed, even though the reply includes a successful terminal receipt.

The source intentionally creates this opaque token after successful execution (`collaboration_stage_execution_source_test.go:221–226`) and copies it into the next invocation's call snapshot (`:158–160`). `StageExecutionTargetDigest` validates conditional state with existing bounded UTF-8 text, not a decimal sequence (`collaboration_stage_execution_identity.go:11–12`, `:29`). Successful observations likewise permit bounded opaque `AfterConditionalVersion` (`collaboration_stage_execution_observations.go:32`). Conditional state is excluded from the stable physical/configuration target digest and compared exactly at admission; it is not a protocol/candidate revision.

## Minimum regression and repair proposal

Expected write scope for a separate implementer is **only**:

- `e2e/fixtures/collaboration-stage-control.ts`: in the reply decoder, validate `calls.last.conditional_version` with existing `text(...,256)` instead of decimal `version`. No generic decoder weakening or transport change.
- `e2e/fixtures/collaboration-stage-control.node-test.mjs`: owning-layer parser and socket regression using the retained opaque token/terminal rollback reply shape.

Run the new regression on unchanged code and retain RED. Minimum meaningful coverage:

1. A successful rollback reply with the actual `1/after/<intent>` conditional token parses and preserves its exact string/receipt/call count; the old source must reject it.
2. The same reply delivered through the existing private AF_UNIX fixture resolves the original request after one send. This catches the generic-unconfirmed conversion at the real client boundary.
3. Empty/null/non-string, NUL, invalid Unicode and over256-byte conditional tokens stay rejected; multibyte strings must respect UTF-8 bytes, not JavaScript character count.
4. Existing wrong request ID, duplicate fields, unknown envelope/invocation fields, invalid digest/kind/count, invalid UTF-8 and oversized frame controls remain enforced. Existing disconnection no-replay and original-key receipt controls stay intact.

Candidate/protocol/native sequence validators, source configuration request shape, `result` schema,64KiB limits,20-second control timer, private socket ownership, no-replay behavior, original four-case bodies and5/20/480-second deadlines remain unchanged. This diagnosis does not require Go, SQL, bridge, production UI, runtime lock or provider-token generation changes. Request-side initial source configuration's conditional version is a separate existing contract; its current initial value did not cause this reply failure, so do not widen it in this bounded repair.

After RED/GREEN and independent source/codec checks, main must bind the new test-only input to accepted consuming products and execute a fresh original four-case run with new artifact generations. A parser regression pass does not retroactively complete R22's failed scenario, A01–A12 or the parent task.

## Files found / code patterns

- Original R22 `runtime/four-scenarios-r01/result.json` and case2 `stage-execution-evidence.json` — full denominator, failure stack, original/fresh decisions and reached assertions.
- R22 `runtime/harness.stdout.log`, `bridge.log`, recorded `database-after-four-r03.json` — correlated terminal receipt, transport readiness and retained durable evidence.
- Consumed R20 frontend `e2e/fixtures/collaboration-stage-control.ts`, original execution spec and owning `.node-test.mjs` — exact reply validator, data catch, request construction and existing isolated socket tests.
- Current consumed R22 Go execution control/source and service identity/observation files — intentional opaque conditional CAS state and actual committed execution receipt.
- Existing task-private `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/runtime-r05/bridge.mjs` — once-only raw line forwarding and reply correlation.

## External references / related specs

No external service/provider/account was accessed. The stage authority and verification-environment specs require exact source/request/result identity, independent current authority and preservation of failed/unknown outcomes. This is a private controlled adapter/codec contract; it is not real payment or production rollback evidence.

## Caveats / Not Found

- No original sent byte frame or client received-byte capture was found. The server's exact emitted reply and source-proven rejecting field are retained; request body provenance is labelled reconstructed.
- The original R20 lock holder remains unproved. This new codec failure is separate from accepted R21/R22 GET lock regressions.
- No request, replay, API/DB query, process, build, test or source action was performed. Only this report and its JSON companion were written; W10 preparation and peer files were preserved.
- The successful durable rollback receipt does not replace unexecuted final Web/Desktop/assertion/cleanup gates or change the1/1/2 result.
