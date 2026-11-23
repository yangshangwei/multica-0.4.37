# Extras-B R01 initial-control diagnosis

Date: 2026-11-19. Active task: `11-13-pay-241-s3-recovery`.

The four original control receipts confirm an authored fixture scope error:
each initial `Resume target` returned `409 issue_paused` because the issue
session was still paused. The server enforced its existing contract. The
correction uses the existing real `Resume all` control for initial activation;
later scoped controls retain their original behavior. Fresh runtime acceptance
has not yet run on this correction.

## Retained execution and original decisions

Evidence root:
`/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly`.

- `verification/runtime-extras-b-r01/result.json`: exit 1; source, configuration
  and tool drift absent. Its raw `stdout.log` SHA256 is
  `f0ebdd4b4efa6ad2f7d3376e37f6639f591a6a48b6d1d5209dc98350d77be69b`.
- The raw Playwright report contains four unexpected top-level results,
  retry 0, skipped 0 and flaky 0. Each failure originates at
  `e2e/fixtures/collaboration-host-runtime.ts:756`, the unchanged assertion that
  the actual control response must be 200.
- Main recovered the four existing actor/issue/request-scoped operation rows
  through a read-only query on
  `multica_pay241_s3n_20261119_uwkcehly_runtime_r02`. No control was replayed and
  no data was changed. The receipt artifact is
  `audit/runtime-initial-control-receipts-r01.json`, SHA256
  `36065a47900b8da21b0119b490ed0e0676ae46543fcd1f0e0d614a130e1a90e7`.

All four rows have `kind=resume_queue`, `result_code=issue_paused`, a completed
original operation, and a retained result containing `action=resume`,
`scope=target`, `status=409`, `queue_state=paused`, `control_version="1"`,
`target_set_version="2"` and `protocol_version=2`. Their original target/channel
IDs are present; command, batch, group and run IDs are null. Control receipt
protocol 2 is distinct from the human execution-view declaration 6 and runtime
protocol 3.

| Original case | Issue ID | Original control request ID |
| --- | --- | --- |
| Web notifications | `01a1f175-5130-74ab-8a9b-14eaff4c07c0` | `9778b4cd-da53-415c-9094-fc03c9bd7a28` |
| Web author withdrawal | `01a1f175-6129-756e-8224-76706332eb44` | `f7a0a894-1ef5-4d72-8d59-5e330860ce52` |
| Desktop notifications | `01a1f175-7801-7ad5-ae63-c57553cc6e9d` | `c5df2c71-7e0b-43cb-b423-7561672ec3e0` |
| Desktop author withdrawal | `01a1f175-8e37-7962-8b3a-6513de9ae649` | `ba28dd0b-99a5-41df-81ba-b4e3ba3a039e` |

The canonical case JSON files are under
`runtime/extras-b-r01/results/<case-directory>/`. Their hashes are:

| Artifact | SHA256 |
| --- | --- |
| `web-runtime-notifications.json` | `94830a243a78a82ede0cd9e66782266dbf3ceb9f02a55b6ca08e25e42930fcd6` |
| `web-runtime-withdrawal.json` | `4b3f822a2bf6b10a0747933dc4fcc791cbbf45348f4a87e265f8a71c54d86bc1` |
| `desktop-runtime-notifications.json` | `4c2cc1da620d8bb63be5913026dd39ea46982865f71edee51e9db26c3c62fd3c` |
| `desktop-runtime-withdrawal.json` | `08c39d90bdf40629ff0de407d438aeb16a0c48f051774a89142bb1779a1831d9` |

Each bootstrap checkpoint records the paused issue session and channel, target
`hold_reason=initial`, hold version 2, target/config/control versions 1, target
set version 2, and empty commands/groups/runs. The execution gate was enabled
with the actual per-case epochs 3, 5, 7 and 9. Enabling that gate did not activate
the issue session.

The observed controller is the actual current handler in every case. Web uses
cookies and CSRF; Desktop uses bearer authentication. The external participant
is the intended author in the withdrawal case, while the handler opens the
separate controller session. No author/controller substitution is proposed.

R01's ten planned subcases retain **2 FAILED / 8 NOT_RUN**. Both notification
tops fail before entering their first `run(name)`; each withdrawal top enters
its first author-withdrawal subcase but fails before submitting an input. All
four artifacts have empty Provider observations and daemon ledgers; each final
capacity message has `claims=0`. The runtime transport has no fact with a run
ID. Intended managed Provider launches are **0 of 10** and original human
execution inputs are **0 of 12**. Runtime result delivery, original run/attempt
recovery, live/queued withdrawal, business-token reads and proof outcomes remain
unexecuted. Public actor/resource setup, initial attachment upload, daemon
registration and readiness are separate observations.

## Source-backed cause and gate order

Source bindings for R01:

- Server: retained `source/server-green-r02`.
- Runtime fixture in `source/client-harness-r05`:
  `fcbac0e765550890c009ae8c96d162f84985b3689a797723432e5db930be4c39`.
- Shared host helper:
  `3ea2e6a51cd782f55d2268427739e6749f93c8bf23069f817147f2cff603fc92`.
- Shared recovery observer:
  `7f125d331adcfa85dab491b3def6ec07bca084238b71eae90f8e5eafcbffb3d1`.

`createRuntimeIssue()` creates the issue, performs the public transfer and
invitation, creates an owned agent, and calls the existing offer/adoption/owner
consent helper. It then explicitly asserts `current.session.queue_state ===
"paused"` before returning (`collaboration-recovery-runtime.ts:368–409` in
retained R01). Adoption and consent do not imply Resume. The session's initial
paused state also follows the schema default and `CreateCollaborationSession`
insert (`migrations/571_collaboration_schema.up.sql:35` and
`pkg/db/queries/collaboration.sql:53`).

The original local `control()` wrapper always passed `"target"` to
`hostControl()` (`collaboration-recovery-runtime.ts:427–435`). Its initial callers
were the notification handler and the two withdrawal-controller branches
(`:703`, `:881–882`). The shared helper correctly clicked the requested scope's
real control and asserted its actual 200 response; it did not choose initial
issue activation on the caller's behalf.

`ResumeQueue()` checks AI policy and closed state, then returns
`issue_paused` for target scope whenever the locked session is not active
(`server/internal/service/collaboration_control.go:215–223`). This refusal
precedes target control/hold/gate version checks, target grant/configuration
validation and channel proof checks. Earlier queue mutation guards still check
current human authority, original key, lifecycle/participant/account epochs and
the host-view declaration (`collaboration_queue.go:268–432`). The stored code
identifies the actual refusal branch; it is not inferred from HTTP 409 alone.

The refusal is completed as an original `collaboration_operation` and committed
before being returned. `writeCollaborationResourceResponse()` writes that
retained receipt using its actual status (`collaboration_authority.go:55–60`).
Original-key recovery remains read-only; changing the issue later cannot turn
these four refused keys into new Resume intentions.

The existing host acceptance journey calls initial
`hostControl(..., "resume", "issue")` before its first input
(`e2e/collaboration-host-runtime.spec.ts:55`, also `:123` for its separate issue).
It uses target scope for later scoped Stop/Resume. The contract is explicit in
`.trellis/spec/server/collaboration-targets.md:196–225`: target Resume cannot
bypass a paused issue.

The bootstrap's parsed `capabilities.submit_command=false` and
`shared_runtime_unavailable` marker do not establish a second runtime failure.
The Core issue schema deliberately sets the client submit flag false
(`packages/core/collaboration/schema.ts:296–300`), and the execution projection
initializes that unavailable marker (`collaboration_execution_view.go:294–303`).
They must not be substituted for the original control result or interpreted as
an executed runtime admission check.

## Minimal authorized correction

Only `e2e/fixtures/collaboration-recovery-runtime.ts` is changed:

1. The local `control()` wrapper accepts explicit `"target" | "issue"` scope,
   defaulting to target. It passes that scope to the unchanged shared helper.
2. The three lexical initial activation calls pass issue scope: notifications,
   the handler controlling participant-authored withdrawal, and the handler
   controlling grant withdrawal. The full four-top composition has six planned
   new issues, each using one real initial issue Resume.
3. Initial issue-scope checkpoints use `actual_platform_issue_control`; later
   target controls preserve `actual_platform_target_control`. Captured request
   identity/input and validated receipt remain unchanged.

No second target Resume is necessary for valid initial targets. In the existing
issue-scope transaction, `ResumeQueue()` selects the complete observed target
set and all channels, activates the session (`:301–306`), resumes/mirrors healthy
channels (`:317–333`), and releases valid initial holds (`:367–376`). Fresh empty
run/group snapshots supply no unresolved termination barrier. Current authority,
grant/configuration support and proof checks continue to apply. Invalid target
authority would remain an explicit per-target hold/result, not be bypassed.

The new ROOT runtime fixture SHA256 is
`b668eab5c0d00969e67871a66865029ed7a41bef6be8fd59603a4f045b085e7d`, mode `0644`.
The retained R01 fixture/source and original failures are unchanged. The shared
host helper, recovery observer and runtime spec remain byte-identical. The
planned case names, counts, input/proof/notification/withdrawal assertions,
200-response expectations, later target Stop and original receipt recovery are
unchanged. This correction adds no consent, synthetic control, direct SQL/schema
mutation, retry, budget increase or second control request.

## Evidence gaps and verification handoff

The canonical R01 artifacts preserve original control request IDs, actor IDs,
authentication presence, status and response headers. They do **not** contain
the original control request JSON, response body bytes/digest or control schema
summary. The broad request observer records metadata only. Its response-schema
branch omits `/control` (`collaboration-recovery.ts:186–220`). The shared helper
asserts 200 before reading request JSON/response JSON, and the local wrapper
throws the helper failure before recording its own request/receipt checkpoint.
The persisted receipt now closes the original **logical decision** gap. Its
normalized `payload_hash` cannot reconstruct the original HTTP JSON bytes or
the exact submitted observation values.

If another new profile fails at control, the smallest passive diagnostic belongs
to this local runtime fixture: register a natural response observer alongside
the original request capture before calling `hostControl`, save bounded
response bytes/hash and allowlisted refusal/receipt fields even if the helper
rejects, and retain the captured request observations. Keep the actual request
unchanged and the shared helper's 200 assertion intact. That optional diagnostic
is not implemented in this correction.

Main's SQL evidence is exactly bound with a trailing-LF distinction. The saved
`audit/runtime-initial-control-receipts-r01.sql` has raw file SHA256
`478630d04eb355aa3059819d6453b62433f31567801499460dcced0e485b1ce5`; removing its one
final LF yields SHA256
`11f3c424ff08e44bd13097778fb45fd24145f28a562451fd15be34fd8beade33`, exactly the
JSON's recorded `sql_sha256`. The recorder hashed the original SQL variable and
the file writer appended that LF. This was verified from retained bytes without
running SQL or changing either artifact. The exact four returned original-key
rows ground the diagnosis.

This lane performed source/artifact inspection and a bounded source-delta check.
It ran no SQL, Go, browser, Provider, API request, process/configuration control
or commit. Main owns fresh source/harness binding, targeted TypeScript/ESLint
checks and the new R02 database/API/deployment/configuration and complete
four-top run. The correction is prepared for verification; Extras-B and full S3
remain open, with all R01 failures and unstarted identities retained.
