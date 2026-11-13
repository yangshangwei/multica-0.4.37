# Multi-account collaboration closure QA

## 1. Scope / trigger

Use when claiming a complete cross-workspace human/Agent delivery flow. Separate
human-only, squad and context tests do not establish one shared issue's closure.
The canonical synthetic scenario is `e2e/collaboration-multi-account-closure.spec.ts`.

## 2. Signatures

- `COLLABORATION_HOST_PROVIDER` selects the explicit task-owned
  `collaboration-multi-account-provider.go` binary for the closure spec.
- Host squad/switch specs use `collaboration-host-provider.go`; memory-runtime
  uses `collaboration-memory-provider.go`. Their callback protocols differ.
- Public collaboration comments expose `author_type`; `author` is a human
  summary and is null for Agent comments. There is no public `author_id` field.
- Agent comments additionally expose minimal `author_agent: {agent_id, name}`;
  require exact agreement with the same run's Agent, while retaining `author=null`.
- Exact Agent attribution comes from the same run's records and resource,
  plus `memory.author.agent` and `memory.author.resource_owner`.
- Host closure opens/restores the host execution gate itself. Existing namespace
  offline-notification and synthetic live-revoke fixtures instead require
  `enabled=true`, `creation_enabled=true`, `host_execution_enabled=false` in
  their isolated database. Restore prior flags and run these profiles sequentially.

## 3. Contracts

Use three independent accounts, source/resource workspaces and enrolled runtime
identities. Resource suppliers offer, a creator/handler adopts, and each owner
consents. An external supplier cannot inherit the manager's adoption authority.

The deterministic Provider must actually launch through the production poller
and write through the run-local CLI. Verify the same issue ID, command author,
resource owner, Agent, installation and exact prior context on every run. Check
the second delivery's exact acceptance after the first delivery's return.

Verify the five Agent result names and localized Agent badges in all three Web
sessions, including matching reply hints and absent human edit/delete controls.
Use the same issue for a focused, isolated-profile Electron renderer check before
completion, while Reply remains allowed. Preserve human display, account header
isolation, zero daemon starts and failure-aware native/profile cleanup. These
checks do not claim installed Main or a second runtime closure.

Scope name/badge checks to the direct comment header containing `time`; the
permission disclosure also contains the text "Agent". Verify structural selectors
against the installed Playwright engine: combining `:scope` and a relative
`:has(> time)` returned no nodes, while `xpath=./div[time]` selects that header.
Reply hints use their own container. Never fix ambiguity with an arbitrary index.

Capture failures from custom browser contexts before closing them. Persist
evidence even when body execution or cleanup fails. A passed evidence result
requires successful body assertions, restored gate, closed owned daemons and
each actual run's sealed `proof_acknowledged` / `host_exit_observed` ledger.

## 4. Validation / error matrix

| Observation | Required classification |
|---|---|
| A comment lacks `author_id` | Check the public DTO; do not weaken identity checks or invent the field |
| Comment ID/body matches an exact run record with correct resource attribution | Supported Agent authorship evidence |
| Run queued or permit issued without a Provider observation | Not an executed Agent |
| New test fails before a field-shape correction | Retain the original failed attempt separately from rerun |
| Cleanup or evidence persistence fails | Fail the test; do not emit a passed evidence result |
| Namespace fixture under host cutover | New namespace promotion is rejected with 409 `execution_mode_unsupported`; correct the task gate profile, not production admission |
| Full-page screenshot includes still-loading history | Use for visible completion only; history requires its separate assertions |

## 5. Good / base / bad cases

Good: three distinct owners produce 1/2/2 actual runs, context v1 to v6,
two submissions and one exact initiator acceptance; ledger checks agree.
Base: existing single-owner squad tests retain their original Provider.
Bad: count an unlaunched continuation as a process, or call synthetic business
text proof that a real model repaired payment code.

## 6. Tests required

Keep private ordinary issue denial and unchanged-state checks, repeated original
AI POST key/body with the same receipt and no extra process, separate owner
consent, explicit Resume, ordinary-comment non-dispatch, context conflict and
human-only completion. Stop during a process and squad delegation have separate
host-switch/squad scenarios; report their ownership and coverage separately.

## 7. Wrong versus correct

Wrong: concatenate unrelated passing suites into a complete multi-owner story,
or label all attempts green after fixing a test assertion.

Correct: retain the same-issue closure evidence, exact final case inventory and
first failed attempt. State that synthetic processes establish orchestration,
not commercial model reasoning, multiple physical machines or OS containment.

## Controlled timeout and restart acceptance

### 1. Scope / trigger

Use `e2e/collaboration-timeout-recovery.spec.ts` for a real test-owned process
deadline followed by recovery of already committed results. This is a separate
increment from the three-account closure and does not certify real model accounts,
payment, installed Desktop Main, or Agent output attachment uploads.

### 2. Signatures

The test-only host poller accepts optional `agent_timeout_ms` and
`hold_receipt_acks` through its existing stdin configuration. The shared fixture
accepts `agentTimeoutMs` / `holdReceiptACKs`, and exposes `stopPreservingState()`,
`restart()` and `ledgerEvidence()`. These are test interfaces, not production
runtime configuration or public application API.

### 3. Contracts

Omitted execution timeout stays 180,000 ms. A supplied timeout must be an integer
between 1,000 and 180,000 ms; explicit null, zero and other invalid values refuse
before the harness prepares writable state. The closure Provider's callback
deadline is 150 seconds, so withholding release under the old three-minute
execution deadline proves callback failure, not execution timeout. The canonical
acceptance uses 15 seconds and first observes the real Provider startup.

The ACK barrier forwards the original request to the actual API. Only after a
successful response does it withhold the outcome/proof reply. Preserve the real
HTTP status and request/response digests; never fabricate a successful response
or rewrite a failed API result. Keep the barrier until the old poller exits and
the original pending ledger has been observed.

`stopPreservingState()` stops only that process. `restart()` keeps the same
directory, runtime, management identity and ledger, starts a different process,
and releases the ACK barrier. Only final `close()` destroys the owned directory.
Preserve the existing Windows/POSIX close behavior and its Node regression.

Recovery must retain the exact run, attempt, outcome event, proof receipt and
payload digests. Count unique execution facts separately from HTTP retries:
initial preparation and pre-launch authority checks legitimately request the
same permit twice. Recovery must add no claim, permit, launch or business write.
Compare comments, input attachments, memory, commands and results separately.

The closure Provider writes its CLI comment before reporting its callback and
does not implement the ordinary host Provider's `act` protocol. A later explicit
human instruction creates a new run; it must preserve the old failure. After a
session Resume, read current task controls before deciding whether a target also
needs an explicit Resume.

### 4. Validation / error matrix

| Observation | Required classification |
| --- | --- |
| Callback fails before execution deadline | Callback failure; not timeout proof |
| Actual deadline plus retained timeout result and exit proof | Supported controlled execution timeout |
| Original poller recovers a single dropped reply itself | Same-process retry; not restart recovery |
| New directory or fresh ledger during restart | Invalid recovery experiment |
| API responds 403/503 | Preserve that response; ACK barrier must not hide it |
| Stop wins before timeout | Preserve legal Stop termination, including absent outcome |
| Timeout wins before Stop | Preserve original failure and termination identities |
| Input attachment remains stable | Input retention only; not output-upload idempotency |

### 5. Good / base / bad cases

Good: the API retains failure/proof while the old poller still has an unacknowledged
ledger, then a new poller acknowledges those exact facts with no new launch.
Base: callers omitting the new test options keep the original fixture behavior.
Bad: force a timeout outcome in memory or count repeated permit HTTP calls as
multiple processes.

### 6. Tests required

Keep `TestHostTimeoutRetainsOriginalReceiptsAcrossNewRunner`,
`TestHostTimeoutAndStopKeepTheirActualFirstOutcome`,
`TestCollaborationHostHarnessTimeoutBoundary` and
`TestCollaborationHostHarnessHoldsOnlyCommittedReceiptAcknowledgements` in normal
and race runs. The canonical E2E verifies actual API persistence, a pending local
ledger, different poller PIDs, unchanged business effects, Web/Electron retained
failure, and a distinct successful explicit retry. Restore the original gate
booleans with a monotonic epoch and preserve all body/cleanup failures.

### 7. Wrong versus correct

Wrong: use `close()` and create a new fixture, or time a comment from release even
though the closure Provider already wrote it before the callback.

Correct: retain the same on-disk ledger through process restart and bind every
claim to the actual source/binary and its own recorded observation interval.
