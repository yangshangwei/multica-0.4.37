# S3 notification/runtime independent preflight — 2026-11-19

**R01 preflight found one concrete evidence-retention gap; the R02 owner fix is independently verified below. Actual Extras-B remains NOT_RUN.** The frozen source graph supports the intended controlled execution and original-request recovery paths. R02 now preserves each withdrawal subcase's complete client closure record before the next scope. NR-P02 still requires the actual round's original-identity reconciliation. No production defect, executed runtime result, or full S3 acceptance is claimed.

- Reviewer: `/root/notification_runtime_check`, already-dispatched native `trellis-check`.
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Owned write: this report only. No sealed input, ROOT product/test source, task metadata, specification, configuration or Git state was changed.
- Main owns database allocation, commands, services, browser/Provider execution, integration and live PID collection. This review performed only filesystem/source/static-receipt inspection; it started no test body, API request, database connection or process.
- Private evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-continuation-20261119-uwkcehly` (`B` below).
- Accepted R08 client root: `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep/source-block-ui-run-r05` (`C` below).

The full saved native hook output was read, followed by root `CLAUDE.md`, task PRD/design/implementation, the 2026-11-19 continuation plan, runtime readiness and author implementation reports. Applicable host-execution, recovery, current-authority/content, notification, database/configuration and Web/native QA contracts were checked against actual source.

## Findings (fixed)

None. This review's write scope excludes the sealed fixture and shared helpers. The report records the findings without widening that scope.

## Findings (not fixed)

### NR-P01 — first withdrawal client and cleanup records are overwritten (R01; closed statically by R02)

**Concrete harness evidence defect; acceptance blocker until the original record is retained.** This is not an executed product failure.

The following evidence describes the retained R01 source. The R02 disposition at the end of this report supersedes this blocker for the new harness; it does not relabel any R01 execution as accepted.

- File: `B/source/client-harness-r01/e2e/fixtures/collaboration-recovery-runtime.ts:870` and `:875`.
- Issue: each of the two withdrawal subcases invokes `withSessions` separately with the same `RecoveryEvidence` instance. The first invocation owns the author and handler/controller sessions. The second owns the grant-withdrawal handler session.
- Actual helper: `B/source/client-harness-r01/e2e/fixtures/collaboration-recovery.ts:383`–`:387` writes `sessions` and `all_sessions_cleanup` with `evidence.set`. `RecoveryEvidence.set` at `:40` uses `Object.assign`; `save` at `:44` rewrites the same JSON file. The second successful invocation replaces the first invocation's complete writes, socket inventory, page responses, native PID/exit, profile lifecycle and cleanup outcomes.
- The first subcase result at runtime fixture `:978`–`:981` retains business snapshots/receipts but no session or cleanup copy. Reconnect checkpoints retain selected transport facts, not the complete final closure record. The prior in-memory sessions leave scope after `withSessions` returns.
- Recommendation: Main can retain an immutable, allowlisted evidence snapshot after the first `withSessions` has completed and before the second overwrites it. Alternatively, the test owner can be explicitly authorized to retain a per-subcase closure checkpoint in a new source revision. This reviewer made neither change. Never copy a credential-bearing native profile or raw run ledger as public evidence.
- Sent to Main immediately with the exact frozen locations. Final acceptance must reconcile all three withdrawal client sessions per platform, including both first-subcase closures; the final top-level JSON alone cannot supply that denominator.

### NR-P02 — exact live grant/run and live-input preservation needs independent reconciliation

**Verification boundary; no wrong-grant execution defect established.** The current fixture creates one Agent target and one grant per issue, so its selections are unambiguous in that current construction. The stronger original-identity claim nevertheless exceeds its explicit assertions.

- File: runtime fixture `:915`, `:967`–`:970`.
- The public revoke request selects the target's `resources[0]`; its before/after grant check selects the first grant matching the issue. It does not explicitly compare that selected grant ID/epoch with the live run's retained `grant_id`/`grant_epoch`.
- The queued command is explicitly checked by original command ID, author, participant/account epochs, content digest, cancelled state, null group and absence from group inputs (`:887`–`:890`, `:931`–`:932`, `:957`–`:965`). The actual participant epoch advances exactly once. This is substantial queued-author identity proof.
- Those assertions do not compare the live input's complete original tuple, both commands' lifecycle/input-event/target/channel/batch/sequence tuples, or every immutable run/reservation field after withdrawal. Likewise, `:930` uses an optional lookup with `not.toBeNull`; that assertion alone cannot establish that the original capability row was retained rather than absent.
- Recommendation: independently use the already-retained audit rows (`:477`–`:511`) to link original run → exact grant → revoke request → retained revoked grant, and compare immutable live/waiting input, run, attempt, capability and reservation identities before and after. Treat changing control/progress/claim-generation/settlement fields according to their actual Stop contract, rather than requiring whole-row equality. Check that the original capability row exists with the same hash and an actual revocation timestamp.
- Sent to Main as a provenance obligation, not a request to redesign the fixture or product. These checks can be made against the actual round's allowlisted raw evidence without changing sealed inputs.

## Frozen source and static receipts

The two owner seals match the authored R03 inputs:

| Frozen file | SHA-256 |
| --- | --- |
| Runtime spec | `3ad9fe6cab428dd5043c34c30f7f07b79cc42d5efdff978343fc5ad2bbcd1ef8` |
| Runtime fixture | `81d9fd6ffff94deb5b400405080377eed4d5959e9d5bbe0defd4166b5219bf9d` |
| Current recovery helper | `7f125d331adcfa85dab491b3def6ec07bca084238b71eae90f8e5eafcbffb3d1` |
| Host runtime helper | `3ea2e6a51cd782f55d2268427739e6749f93c8bf23069f817147f2cff603fc92` |
| Controlled Provider source | `fc1095dc4fea11ad6a3e0a96647d99996687ba10937c00ebda53097f26fcd77f` |
| Native helper | `b4038d1ba4eeb3e47c3d9730326d2dcb7f41af07e929bd84013712a5775373f1` |

The harness's `packages`, `apps` and installed workspace package links resolve to actual `C` inputs. They do not resolve to the nonexistent prospective integrated source or ROOT product modules. Retained TypeScript `--listFiles` resolves **411 non-node_modules source paths: 151 harness and 260 accepted R08 client paths**, with no ROOT product path. All 260 accepted paths occur in the accepted client manifest.

Independent filesystem comparisons found:

- Main's three static-command before/after source manifests: 255 entries each, equal; current byte/mode drift zero.
- Main's daemon/CLI build manifests: 5,077 entries each, equal before/after and currently unchanged. They match the accepted R08 server manifest exactly after removing its `server/` path prefix, including modes.
- Accepted client manifest: all 3,624 recorded entries currently match bytes/modes.
- Provider build input: one source entry unchanged; the explicit `source/provider-r01` copy equals the harness Provider source.
- Workspace links were independently resolved at review time. The 255-entry harness manifests contain regular files only; they do not themselves establish an initial/final link or client-package audit for the later execution. Main must retain those separate bindings around the actual round.

The recorded commands under `B/verification` have exit 0, matching raw stdout/stderr hashes, no recorded config/tool/source drift, and `ambient_environment_inherited=false`:

| Main command receipt | Independently inspected result |
| --- | --- |
| `build-host-daemon-r01` | `go test -c ... ./internal/daemon`; compile only, no TestMain/body |
| `build-host-cli-r01` | explicit `go build ... ./cmd/multica` |
| `build-host-provider-r01` | explicit stdlib Provider source build |
| `runtime-typecheck-r01` | strict selected files, `--noEmit` configuration and complete file list; exit 0 |
| `runtime-eslint-r01` | two exact selected files, `--no-ignore`, zero errors/warnings/suppressed messages |
| `runtime-list-r01` | four exact Web/Desktop × notifications/withdrawal tops; workers 1, retries 0; definitions only |

Current executable files are mode `0755`:

| Build artifact | Current SHA-256 |
| --- | --- |
| `s3-host-daemon-r01.test` | `a710c5c52150cf68fdf776b58d18143ebd2c2892a60483a6ed63608894e01a7b` |
| `s3-host-cli-r01` | `8bbd257e8b060659f49c909840938975bcd2351240774aea8f82ae0aa547dcfc` |
| `s3-host-provider-r01` | `e6987442d61629087fd44f9eb39bd5ee0b23b007b38710cd0b34677cf005fbaf` |

These are inspected Main receipts/current hashes, not commands rerun by this reviewer or actual runtime outcome evidence.

## Source-path assessment

### Controlled natural outcomes and original recovery

The runtime transport (`:82`–`:255`) observes actual claim/permit replies, holds the actual proof request before forwarding, and drops only genuine successful outcome/proof acknowledgements. It stores original authorization and request bytes privately; evidence exports hashes. No fixture supplies a permit, credential, outcome receipt or termination proof.

The host helper runs the explicit guarded daemon argument `__multica_collaboration_host_router_poller` (`collaboration-host-runtime.ts:349`). The copied server's guard at `internal/daemon/collaboration_host_router_test.go:55` exits before normal test execution and uses real managed registration, capacity and poller paths. The Provider checks its issued host identity and absence of ambient credentials, executes the actual selected run-local Multica CLI and reads prepared file bytes before reporting its callback (`collaboration-host-provider.go:342`–`:505`). An explicit `f` release generates a real error result, an `r` release a real success result; Stop uses the existing supervised process cancellation. These are controlled executions, not commercial-model calls or OS isolation guarantees.

`recoverOriginal` (`:602`–`:630`) closes the current poller while retaining the exact directory/ledger, restarts a distinct PID with the same managed runtime/configuration, and compares original proof/outcome/identity/launch hashes. Its old run's claim/permit facts and Provider-visit count must remain unchanged. It then submits the same original proof and any actually recorded outcome, accepting only the content-free ACK. When no natural outcome exists, that ACK is explicitly not claimed. Whole-round run/attempt/visit counts supplement the old-run-specific no-launch assertion.

The server `RecordRuntimeOutcome`/`acknowledgeRecordedRuntimeOutcome` (`internal/service/collaboration_runtime_capabilities.go:428`, `:534`) matches the issued capability hash and immutable stored event, attempt and frozen termination identity before content-free historical acknowledgement. New or changed results retain the current-authority/conflict gates. Explicit late requests are separate negative probes, never automatic replacement execution.

### Failure, result recipients and complete denominators

The notification top executes failure → explicit new success → Stop on the same issue (`:706`–`:709`), although its inventory display order starts with success. The held-proof phase requires unknown/unsettled state and zero result notices. Settlement requires `host_exit_observed`, one launch and truthful limited cleanup. Exactly creator/resource owner, current handler and participating viewer receive one appropriate result notice each; outsider receives none (`:736`–`:742`). Later run records and all earlier notice IDs/projections must equal the originals (`:761`–`:767`).

The actual writer `recordCollaborationGroupResult` creates result facts and freezes current valid participant recipients only after a newly guarded group settlement (`internal/service/collaboration_result_notifications.go`). It does not impersonate the human author or use ordinary subscribers. The real shared notification link closes its popup without automatically marking a notice read (`C/packages/views/collaboration/notifications.tsx:59`–`:64`), so the fixture's equality assertion is consistent with that UI path.

The suite plans **4 tops / 10 subcases / 10 intentional Provider launches / 12 original human inputs**. Each top creates every named NOT_RUN entry before setup (`:450`–`:460`); a primary failure preserves its failed entry and remaining NOT_RUN entries. Notification final counts are three runs/groups/attempts/commands/comments, one input upload and nine result notices per platform. Withdrawal counts are two independent runs/groups/attempts, three comments/commands and two uploads per platform. Preparation/readiness and run-local CLI subprocesses are additional process observations, not extra intentional Provider launches. No actual terminal inventory exists yet.

### Original-token withdrawal and protected reads

`establishProtectedReads` (`:791`–`:845`) requires original-token 200 responses for context, preparation, exact input bytes, comment page/content, memory, live handoff, original snapshot rehydration, original memory base and every real returned source chunk. The emitted business hash must match the original database capability (`:896`–`:897`), remain distinct from termination/daemon credentials, and remain identical before/after denial (`:858`). Replay uses the original stored authorization/body; GET uses the original issued permit token. A wrong-token or nonexistent-reader substitute therefore does not satisfy the positive prerequisites.

The public participant removal or exact owner-grant revoke happens while only the owned poller is suspended and the Provider callback remains unreleased (`:900`–`:925`). The actual server removal writer revokes the participant, cancels that author's waiting commands and Stops invalidated work in its workflow transaction (`internal/service/collaboration_workflow.go:297`–`:334`). The grant writer resolves the actor's exact selected grant and Stops its dependent work (`internal/service/collaboration_targets.go:35`–`:72`). No reenrollment, new consent or Resume repairs the old run.

All protected readers reuse `lockRuntimeBusiness`, its retained capability and complete current admission (`internal/service/collaboration_squad.go:85`–`:107`; `collaboration_runtime.go:336`–`:427`). This includes both original snapshot and memory-base replays. Original termination authority remains separate. Removed human authors may recover content-free comment receipts but lose current issue access; retained handlers keep issue access after grant loss.

The fixture records every actual per-path denial but admits 403/404/409 (`:852`). Main's independent acceptance must inspect those exact codes and bodies against the original reader and withdrawn identity, before and after settlement. Generic non-200 counts, unrelated reader errors, or title/description absence alone cannot establish the required current-authority refusal. NR-P02 identifies the additional immutable-identity comparisons needed from raw audit rows.

### Ordinary effects, failure retention and closure

The audit enumerates 21 actual relation sets using session → issue joins and original IDs; it requires empty issue-scoped ordinary dispatch, attachment, inbox and subscriber sets (`:474`–`:546`). It hashes private content, nonces and JSON payloads rather than exposing them. Main still owns the whole-database/catalog and protected-history retention audit; issue-scoped emptiness does not establish a whole-database count.

Primary errors are saved before teardown and rethrown after separate daemon/transport/gate cleanup (`:339`–`:364`). Withdrawal separately preserves a primary failure over thaw failure (`:982`–`:994`). Current client acquisition/cleanup preserves partial lifecycle failures. NR-P01 concerns the successful first subcase's subsequently overwritten closure evidence, not failure masking in a failed first subcase.

Host cleanup exports allowlisted ledger facts and removes its private credential-bearing directory after shutdown. Final ledger export reduces native process identity to `process_observed`; it does not enumerate Provider or CLI PIDs. Main must collect actual daemon/Provider/CLI lineage and terminal facts before this removal, including the initial run-local CLI reads and any additional readiness/probe subprocesses. A successful callback or `closed_ms` alone is not a native wait/PID-closure proof. Raw credential ledger/profile publication remains excluded.

The original three gate booleans are restored after owned cleanup with a monotonic epoch; fallback SIGKILL remains failed shutdown. Main reported a fresh runtime database with 855 migrations and 663 valid indexes. This reviewer made no database read and therefore does not independently certify those counts, runtime gate closure or connections. API deployment, production Web API/WS origin, matching renderer/preload, listener ownership, full source/config/link/binary before/after identity and final owned-process/connection zero remain actual execution gates.

## Verification

- Lint: **Main's recorded selected lint receipt inspected: exit 0, two files, zero errors/warnings. Reviewer execution NOT_RUN** under the explicit inspection-only assignment.
- TypeCheck: **Main's recorded strict receipt inspected: exit 0; source graph and current bytes/modes independently reconciled. Reviewer execution NOT_RUN.**
- Tests: **NOT_RUN / 0 by this reviewer.** Four-top `--list` and three successful compile receipts establish static prerequisites only. No actual daemon, Provider, CLI, API, database or browser test body was executed here.
- Fixed source issues: none. Unfixed concrete retention issue: NR-P01. Additional original-identity audit boundary: NR-P02.
- Actual runtime acceptance, ordinary subscription/server GREEN follow-up, full S3 R01–R09, real models, installed Electron Main, Agent-output uploads, S4/S5 and parent matrix/task closure remain separate. This report does not update any of them.

## R02 retention follow-up — same independent reviewer

**NR-P01 is closed for the R02 source at static review. Runtime acceptance remains NOT_RUN.** Main authorized this follow-up on `B/source/client-harness-r02`; this reviewer changed only owned reports.

The runtime fixture SHA-256 is `fcbac0e765550890c009ae8c96d162f84985b3689a797723432e5db930be4c39`. Its only delta from the retained R01 fixture is the withdrawal-scope wrapper:

- At `:875`, `try { return await withSessions(...) }` awaits both the exercise and shared helper's complete cleanup, including its final session/cleanup evidence write.
- At `:998`–`:1007`, a `finally` appends `runtime_withdrawal_client_scope_closed` or `runtime_withdrawal_client_scope_without_acquired_clients` with the exact subcase, issue ID, complete sessions and all cleanup outcomes.
- `RecoveryEvidence.checkpoint` uses `structuredClone` at unchanged helper `:42`. Each appended scope record therefore survives later replacement of the mutable top-level `sessions` keys.
- The wrapper runs on successful, failed and zero-acquired-client `withSessions` paths. It does not replace the original thrown failure with a successful result or bypass cleanup. If public issue creation fails before the wrapper, no client has been acquired for that subcase; its original inventory/failure remains retained.
- The global `collaboration-recovery.ts` helper remains byte-identical to R01. The original R01 sources and statics remain available.

The new four-file `clients-typecheck-r02`, `clients-eslint-r02` and `clients-list-r02` Main receipts were independently inspected: each exits 0 with matching raw hashes and no recorded source/config/tool drift. Lint covers the two runtime/subscription specs and fixtures, with zero errors, warnings or suppressed messages. The list contains exactly four runtime and two subscription tops. These receipts establish code/static disposition only; no browser/Provider body or native closure is certified here.

The subscription fixture SHA-256 is `5c4a0f6fc16722748159ec4d58168610c09fcdd35664127c11974e3ce6dbaf2b`. Its separately owned audit cleanup change records a cleanup failure in `finally` and throws it afterward only on the successful primary path. A primary exception remains the original exception. This does not change global helpers or business source.

NR-P02 remains open for actual original run/grant/live-input/capability/reservation reconciliation. The new immutable scope checkpoints still need actual complete records and successful native/process/DB/gate closure before acceptance.
