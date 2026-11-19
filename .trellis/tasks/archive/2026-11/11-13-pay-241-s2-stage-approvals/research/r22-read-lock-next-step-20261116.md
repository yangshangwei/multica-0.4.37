# Research: R22 capability GET and current-user read contention

- Query: After R21's workflow receipt read repair, which actual reads or controlled Host paths can still contend with the QA R4 user fence, and what is the smallest feasible next regression?
- Scope: Internal; current source and exact retained research/private tracer files only. No DB/API/browser calls, tests, builds, process controls, credential discovery or Git operations.
- Date: 2026-11-16
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

### Current repair and evidence limit

Current ROOT `GetWorkflowOperation` at `server/internal/service/collaboration_workflow.go:708` uses `LockArtifactUsers`. Its file hash is `51e7eb2b52462fa5e771014b2000173752f30eaa07997f5c66d170054d5f5d09`, matching accepted R21 integration evidence. This is the independently supported workflow receipt/read repair. It does not identify the original R20 QA `LockArtifactUsers / 55P03` holder.

R4 enters `readCollaborationStageEvidence` (`collaboration_stage_evidence_read.go:21`) and preserves its private read marker through M1 artifact and standing issuer/report observations. `delivery_evidence.go:96` selects its shared issue read; `:104` takes the actual current-user shared fence. Artifact task-principal, approval and execution reads select shared variants. Standing issuer/session and exact report-run reads remain shared at `delivery_completion_collaboration.go:44` and `collaboration_stage_qa.go:63`. R4 query failure still locks `StageCodeForms` at `packages/views/collaboration/stage-code-forms.tsx:45`; an independently successful T4 cannot clear that R4 error.

The three other inspected recovery readers retain exclusive user fences: `GetCreationResult` (`collaboration_creation_result.go:150`), `GetCommandOperation` (`collaboration_command_operation.go:34`) and `getCollaborationResourceOperation` (`collaboration_switch_view.go:234`). Their reachability in the R20 failure window remains unproved; broad repair is not justified by this research.

### Newly identified actual capability GET

The controlled Host poller performs `EnsureCapabilities` **before** listing candidates on every dispatch (`server/internal/daemon/collaboration.go:280`, `:303`). The actual client first performs:

```text
GET /api/collaboration/runtime/capabilities?runtime_id=<issued runtime>
```

`server/internal/daemon/collaborationrun/client_preparation.go:31` issues that GET. `EnsureCapabilities:68–73` immediately returns when the full current registration matches. An identical registration therefore needs no POST. The earlier hypothesis that every poll necessarily POSTs registration is too broad.

The real GET chain is:

```text
RuntimeCapabilities (collaboration_runtime_preparation_capabilities.go:145)
  -> collaborationRuntimeMutation:146
  -> lockPreparationRuntime:147 / :56
  -> LockManagedRuntime (managed_runtime.go:123)
  -> auth.LockPasswordSession (password_session.go:175)
  -> LockPasswordUser (password.sql:16)
     SELECT id FROM "user" WHERE id=$1 FOR UPDATE
```

`LockManagedRuntime:131` subsequently takes the original daemon namespace advisory fence; `:148` takes the installation admission fence. `lockPreparationRuntime:64` takes the original exclusive runtime-row fence. The GET formats the saved current registration and returns; its closure does **not** insert/update rows, advance scan cursors, register capabilities, claim a run, or dispatch work. `CheckPasswordVersion` (`password_session.go:73–105`) and `CheckManagedBinding` (`managed_binding.go:29–81`) contain only getters.

This GET is a concrete additional read/read contention candidate after the R21 workflow fix. A settled idle QA daemon can reach its QA owner's exclusive user row without new work: the capability GET precedes candidate listing. Its role as the **actual R20 holder** is still unproved.

### Distinguish capabilities from run-list promotions and writes

An empty `ListRuntimeRuns` scan does not itself execute the full participant-user writer fence. Scan pages lock runtime/cursor metadata (`collaboration_runtime.go:493`). Waiting-target promotion reaches `lockCollaborationRuntimeSource(... current=true, daemon=true)` at `:694`, which collects creator/handler/participant identities and uses `LockCollaborationUsers FOR UPDATE` at `:133`. The SQL waiting predicate requires an actual `waiting` command with null group (`collaboration_runtime.sql:88–96`); candidate lists exclude settled runs (`:62`).

Real current-authority claim, permit and new outcome writers also reach that exclusive source fence (`collaboration_runtime_capabilities.go:151`, `:344`, `:455`; Host permit at `collaboration_host_runtime.go:96`). Their original serialization is intentional and remains outside a capability GET repair. Proof/stop paths use their separate original cleanup authority. Online status alone does not prove that any particular promotion or writer was active.

## Recommended bounded regression

Prioritize an actual `RuntimeCapabilities`/R4 read/read regression. It establishes a directly reachable contract without needing to assign the earlier holder. Preserve the original R20 and R21 results and use coordinator-bound private source and fresh DB profiles.

1. Create a currently authorized, artifact-bound stage fixture and a real managed runtime whose owner is the same human used for R4. Require a pre-read that positively contains the exact artifact candidate; a source-withheld result or empty PR-only candidate cannot prove this fence.
2. **Capabilities first:** hold the real capability GET after its completed managed account/user fence, then run same-owner real R4 on a distinct backend PID within the existing five seconds. `LockAgentRuntime` in `lockPreparationRuntime:64` is an existing downstream query barrier; verify it is reached rather than merely assuming a query name.
3. **R4 first:** hold actual R4 after its artifact/current-user fence, then execute the same runtime's actual capability GET before releasing R4. Preserve the original five-second deadline and compare the exact saved registration/epoch after both readers complete.
4. Retain an actual account/password writer-held negative control: shared readers must not cross the original exclusive writer fence or return private partial evidence. Preserve original workflow writer and registration/claim/permit fences.
5. Keep anonymous/human/task credentials, wrong runtime/owner/binding/workspace, revoked binding, disabled account and stale password version non-authorizing. Reuse existing authority tests where their unchanged source is bound; add only regressions required by the new read boundary.
6. Snapshot capability bytes/epoch, invalidations and relevant cursor/operation counts before/after to demonstrate that the GET produced no durable write effects. No manual `settled_at`, permission, report usability or version writes are allowed to manufacture a passing concurrency observation.

Expected old-code read/read failures are a hypothesis until the coordinator executes the bound original-code RED. No RED/GREEN or new runtime result was run by this researcher.

## Existing fixture and identity recipe

- `newStageRuntimeFixture(t, true, stageArtifactReadFixture)` (`collaboration_test_report_test.go:27`) creates real persisted managed installation/binding prerequisites, current password credentials for all stage humans (`:39–50`), current JWT/PasswordSession contexts, original review, QA stage, and actual packager-backed artifact identity. It defaults to the original Linux runtime protocol; do not rename that setup as a native Host run.
- `newStageExecutionFixture(t)` (`collaboration_stage_execution_fixture_test.go:180`) additionally writes the typed report through the real service, finishes its run via `finishSquadRun` (`:192–203`) and retains a standing QA approval. QA is explicitly an artifact-read admin (`:189–190`). This is useful for the full standing report/issuer chain; its existing protocol proof still is not native Host-process proof.
- `ownerSwitchNamespace(t, f.managedCollaborationFixture, f.stage.qa, name)` (`collaboration_owner_departure_targets_test.go:22`) builds a second actual runtime and matching persisted binding for the QA owner, with an explicit current `auth.PasswordSession{Kind:"daemon_token", UserID, Version, WorkspaceID, DaemonID, BindingID, BindingEpoch}` (`:37`). The QA password row already exists in the stage runtime fixture. It does not need invented bearer values or a fabricated authority object.
- No `ManagementPrincipal` symbol was found in the inspected auth/service code. The actual service gate consumes the verified `PasswordSession` plus persisted runtime/binding/install/account facts. Use `auth.WithPasswordSession` exactly as the existing managed fixtures do; a human `AuthenticatedPrincipal` alone is rejected by `authorizeCollaborationDaemon`.
- `holdStageArtifactR4` holds `reviewerCtx`; a same-QA regression may create a local fixture copy with `reviewerCtx=qaCtx` for this reader, preserving shared helpers and comparing the current QA-authorized artifact result first.
- If the new test specifically needs a server-issued **Host protocol** run, use `hostCapabilityRegistration`, actual target registration/current-owner consent, `hostClaimRequest`, `hostPermitRequest`, actual outcome recording and `AcceptRuntimeProof` with `hostFinalProof` from `collaboration_host_runtime_test.go`. `newHostCollaborationFixture:26` and `hostFinalProof:149` are the established source patterns. With a different QA owner, use actual service claim requests rather than `claimHostRun`'s convenience assertion that agent owner equals fixture creator. Never overwrite a shallow-copied fixture's shared creator identity.
- A service fixture's synthetic launch observation establishes server protocol/settlement only. Native daemon exit and a real idle poll loop remain the existing production Web/Desktop controlled-Host runtime acceptance responsibility.

## Minimum prospective product boundary after genuine RED

Expected owned files are one new service concurrency test, `server/internal/service/collaboration_runtime_preparation_capabilities.go`, and `server/internal/service/managed_runtime.go`. Do not change SQL, generated files, public response shapes, ordinary authentication, or writer entry points unless the actual counterexample proves another required boundary.

Route only `RuntimeCapabilities` through a private managed **account read-fence** variant. Preserve the existing current PasswordSession preconditions (`Setup=false`, `Change=false`, positive version and valid subject UUID); use existing `q.LockArtifactUsers([]actor)` and require one matching user row, then call `auth.CheckPasswordVersion` under that shared lock. Preserve every existing account/password/binding/deployment/org/member predicate. Keep original daemon namespace advisory behavior, owner/current-runtime checks and installation-admission behavior. `LockManagedRuntime` and `auth.LockPasswordSession` must retain their existing exclusive behavior for minting, registration, claim, permit and other writers.

Existing read helpers are `LockArtifactUsers` (exact same user set; `FOR SHARE NOWAIT`), `TryLockCollaborationAuthorityBinding`, `TryLockCollaborationAuthorityInstallation`, and `TryLockCollaborationAuthorityRuntimes` (id-only runtime set; shared). They are reusable when a tested read boundary needs them. No existing shared daemon-namespace helper was found; widening that advisory behavior is outside this bounded lane. Do not automatically replace runtime/binding/install fences merely because a helper is available. A user-fence regression does not prove every same-namespace or same-runtime read combination.

## Passive capture alternative and limits

The current R21 consuming private tracer is `runtime/build-source-r01/server/cmd/server/pay241_read_diagnostics_test.go`, SHA256 `c960ff3f49264426ef2a65708650cbe05bcdbf7d8eaa0b2217bc0bbd29e5bd74`. `pay241DiagnosticPath:103` permits only selected issue/stage/report GETs; it excludes capability GET, operation recovery routes, writes and background contexts. `TraceQueryStart:125–128` drops unscoped contexts. `TraceQueryEnd:160` retains successful lock observations only for `LockLifecycleIssue`, not `LockPasswordUser` or `LockCollaborationUsers`. The existing trace therefore cannot identify these actual successful user holders.

If holder proof is needed, widen only the private test-only metadata tracer to the actual capability GET and watched user-lock start/end/transaction generations, including successful native pgx callbacks and unscoped background locks. Preserve its safe body-capture allowlist; do not log raw SQL, arbitrary arguments, credentials or POST bodies. Bind verified owner/runtime/request/PID and canonical lock-query/hash/source frames. Use a separately bounded, tracer-free metadata observer for `pg_stat_activity`/`pg_locks` where necessary, avoiding SQL reentry on the API connection. Record capture delay/drops/errors and no raw activity query text.

A NOWAIT reader is not left waiting, so empty `pg_blocking_pids` does not prove absence of a holder. Relation `RowShareLock` alone does not identify an exclusive tuple owner. Require actual successful exclusive acquisition, same target/owner, still-active transaction identity, and a real lock edge or equivalent same-target proof before naming the R20 holder. The proposed two-connection regression proves its own deterministic coexistence gap, not retrospective runtime attribution.

## Files found and code patterns

- `server/internal/daemon/collaboration.go` and `collaborationrun/client_preparation.go` — actual poll order and GET-only matching-capability recovery.
- `server/internal/service/collaboration_runtime_preparation_capabilities.go` — getter/registration separation and shared current-runtime authorization helper.
- `server/internal/service/managed_runtime.go`, `server/internal/auth/password_session.go`, `managed_binding.go` — exact account, namespace, binding and managed installation predicates.
- `server/pkg/db/queries/password.sql`, `agent_artifact_execution.sql`, `collaboration_authority.sql` — original exclusive and existing shared fences.
- `server/internal/service/collaboration_runtime.go` and runtime SQL — actual promotion/writer reachability and settled/empty-scan limits.
- `server/internal/service/collaboration_test_report_test.go`, `collaboration_stage_execution_fixture_test.go`, `collaboration_owner_departure_targets_test.go`, `collaboration_host_runtime_test.go` — current password, stage, managed-owner and Host protocol fixtures.
- `server/internal/service/collaboration_stage_evidence_read_concurrency_test.go` — real passthrough barriers, distinct PG PID and post-authority held-reader pattern.
- `research/r21-qa-form-read-failure-diagnosis-20261116.*`, `r21-runtime-poller-qa-applicability-20261116.*`, `r21-runtime-product-entry-independent-check-20261116.*` — original runtime failure, cleanup limits and accepted consuming products.

## External references

No external documentation, account or provider was accessed. Current SQL, actual native pgx tracer implementation and repository consumers are the primary evidence.

## Related specs

Read `.trellis/workflow.md`, `.trellis/spec/server/collaboration-stage-approvals.md` and `.trellis/spec/server/verification-environment.md`. Stage spec lines 60–98 require read/read compatibility, original writer fences and actual source/PID attribution; lines 83–91 explicitly prohibit weakening writer locks, isolation or form freshness to hide an error. Source/profile admission and final retained evidence remain coordinator-owned.

## Caveats / Not Found

- This research changes only its two declared files. No code, spec, task state, ledger, DB, process or runtime result was changed.
- The original R20 holder remains unproved; newly found capability GET is a source-backed reachable contender and a separate feasible regression.
- Existing synthetic service fixtures do not prove native Host process cleanup or final S2 acceptance.
- Current capability GET's coarse account/runtime/advisory fences couple a frequent declarative read to mutable credential/namespace authorization. Repair the proven read boundary with existing semantics; a global managed-auth or writer-lock redesign would need a separate contract and is not required by this research.
