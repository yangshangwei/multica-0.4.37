# Research: Runtime delivery, limited administration and complete acceptance

- Query: What remains in the explicit runtime/provider, P7 administration and P8 acceptance deliverables?
- Scope: Internal source and approved parent contracts; planning only.
- Date: 2026-10-20
- Parent: `.trellis/tasks/10-10-cross-workspace-collaboration/`.
- Evidence: Main-session source inspection after the research worker encountered a transient token-rate limit. No product code, database, provider account or deployment was changed.

## Findings

### Runtime files and providers

The approved scope is the same Server, Web/Desktop and supporting CLI/daemon; new Mobile collaboration is excluded (parent `prd.md:44`, R22). R27 requires explicitly delivered attachments or committed Git content rather than copying an old working directory. R28 requires the authorized environment's own provider account and quota. D07 distinguishes provider business capability, daemon management identity and supervisor-only termination capability.

- `server/pkg/protocol/collaboration.go:43` carries attachment IDs, but `server/internal/daemon/collaborationrun/controller.go:34` only transports run identity and JSON inputs. `prepareRoot` at `:222` creates a fresh root and copies the explicit agent/CLI binaries; `:259` creates an empty `/work`. There is no attachment-byte or committed-Git preparation in this path.
- `server/internal/daemon/collaborationrun/controller.go:23` explicitly supports only configured static Linux executables. `openStaticExecutable` at `:285` rejects scripts or dynamically linked providers. `controller_linux.go:78` starts `/agent`, and `:88` builds a minimal environment rather than importing the owner's account home. This is a verified narrow execution contract, not a general provider adapter.
- Ordinary providers already exist in `server/pkg/agent/agent.go:297`–`:345` and the factory at `:382`. Their availability is not evidence that they can execute under the collaboration controller. Reuse their supported-type registry and parsing where appropriate, but do not borrow ordinary platform tokens or ordinary task scheduling.
- `controller_linux.go:23` uses user/PID/mount namespaces. PID 1 teardown at `:100` accounts for descendants that change session/group or double-fork. `controller_unsupported.go:7` refuses containment on non-Linux platforms. `server/pkg/protocol/collaboration.go:6` currently declares only `linux_pid_namespace_v1`; claim/permit/proof validation in `collaboration_runtime_capabilities.go:126`, `:247`, `:453` binds that value.
- `server/internal/daemon/collaboration.go:113` enables execution only after managed identity and the actual containment probe succeed. Resource consent does not establish operating-system support.

Required delivery therefore includes a versioned preparation/provider capability contract, exact attachment versions and scoped retrieval, committed Git materialization in a private run directory, provider account/configuration scoped to the authorized environment, independent outcome/telemetry parsing, and actual descendant containment. Existing providers without these guarantees must remain visibly unsupported. A process-group observation or an ordinary provider's `ProcessExited` flag is insufficient.

Canonical acceptance: AC41/42 (Git/attachments), AC43/44 (provider account/provenance), AC32/33/94/95/96 (runtime integration, capacity, independent proofs and authority). General-provider and non-Linux coverage must be recorded per combination, not inferred from one static executable.

### P7 limited administration

The parent design at `design.md:96` fixes R38/R40: only a disabled creator permits explicit administrator complete/cancel/reopen; a successor is required only for reopen when creator and handler are both disabled. The successor must already be an effective participant. The administrator does not join the issue, gain ordinary transfer/resource authority, change creator identity or gain private content access.

Current ordinary lifecycle code is `server/internal/handler/collaboration_lifecycle.go:42` and `server/internal/service/collaboration_lifecycle.go:287`. Its request (`:18`) contains no administrator reason/successor and uses the ordinary workflow authorization. `changeLifecycle` at `:346` is the existing close/reopen primitive: close cancels waiting inputs at `:373`; reopen preserves paused state and historical barriers.

Repository searches for `CollaborationAdmin`, `AdminCollaboration` and `successor_user_id` under server/core/views/platform routing found no collaboration-specific administration implementation. This is a source-inspection result, not a test execution.

`server/internal/service/platform_admin.go:62` provides the existing password-session, current account/session-version, write-role and active management-organization checks. The mutation lock entry at `:115` is the existing authority-lock owner. P7 should extend this management domain and reuse lifecycle effects in the same transaction; it must not add a generic admin bypass to ordinary collaboration capabilities.

Required implementation:

1. Dedicated minimal administration read/candidate/action/operation-recovery routes inside existing administration scope.
2. Lock-time creator-disabled, admin-role/session/organization and source-scope rechecks; nonempty reason and exact revision/status confirmation.
3. Conditional successor validation; reject that field for other actions and non-double-disabled reopen.
4. One transaction for lifecycle, handler/current-workbench responsibility, superseded information-request chain, true administrator audit/operation/history and retained outbox. Payload identity includes the successor, so reusing a key with a different candidate refuses.
5. Shared Web/Desktop action and candidate forms; refresh invalid choices without automatically selecting another person; no full-content or provider-log administration projection.

Canonical acceptance: AC65/66/69/70. Test creator restoration, admin/session/role/scope revocation, successor departure/disable and concurrent responsibility changes with deterministic transaction barriers. Use a separate synthetic password/admin-enabled production environment. Default legacy `make check` does not prove these cases.

### Complete acceptance and historical evidence

Parent `implement.md:243`–`:252` assigns every AC01–AC97 one primary layer. Keep that mapping: P1 data/access, P2 responsibility, P3 content/realtime, P4 resource/protocol, P5 queue/proof, P6 Web/Desktop wiring, P7 limited administration, and P8 integrated atomic creation.

Current historical evidence is explicitly incremental: `verification-preview-wakeup-2026-10-20.md` proves preview and ordinary-parent wakeup, keeps the parent in progress, and excludes full-repository, packaged Desktop and real-provider tests. `research/qa-report-2026-10-19.md` records real Web and macOS Electron harness results plus provider/Linux/packaging limits. These records cannot be relabeled as this task's fresh results or full acceptance.

P8 must keep a 97-row ledger with requirement, primary owner, exact test identity, command, source/environment provenance, actual executions, pass/fail/skip/not-run and explicit limitations. Native harness, production Web, packaged Main startup and other operating systems are different evidence scopes. Old Mobile/CLI/daemon compatibility remains required even though new Mobile features are excluded.

Required broader evidence includes three accounts/two spaces; shared private-resource supply and authorization; full handoff/information/acceptance/lifecycle journeys; real source/account HTTP/WS and file permissions; ordinary-task and old-machine-entry negative regressions; Server/daemon restart and uncertain response; revoked owner/resource/configuration; capacity-one squad continuation; migration/gate rollback boundaries; full native process tree and packaged-client provenance. Default creation/execution gates remain closed. Read and termination convergence must keep working with gates closed.

## Execution order and ownership

| Slice | Dependency | Primary ownership | Verification |
| --- | --- | --- | --- |
| Fair scheduling/proof recovery | First; supplies reliable control foundation | Runtime service/SQL and daemon poller/ledger | Deterministic prefix, cursor, restart, capacity and original-proof tests |
| Private resource supply | Define minimal offer/read/write contract before clients | Resource service/handlers/SQL, then core and shared views | Owner differs from organizer, organizer has no execution-space membership, no catalog expansion |
| `@all` and cancellation history | Additive shared API contract before view integration | Mention helper/composer; protected input/event projection | Human-only mixed mention, immutable cancellation provenance, refresh/native access |
| File/Git/provider preparation | Authority/control foundation and explicit capability contract | Protocol/service file capability, daemon controller, existing provider parsers | Fake CLI/native runtime matrix, scoped bytes/commit, no broad credentials |
| P7 administration | Reuse stabilized lifecycle/history/proof effects | Admin service/handlers, core and shared forms/platform wiring | Password/admin-enabled DB and Web/native scope/transaction tests |
| Full P8 acceptance | Every implemented slice has its own evidence | Coordinator plus independent checking | All 97 IDs accounted for; unsupported/unrun scopes remain explicit |

Shared router, generated SQL, API barrels, locales and common schemas need a single integrating owner. Do not run concurrent production builds in the same checkout.

## Caveats / Not Found

- No new runtime, provider, operating-system, administrator or packaged-client tests ran for this research.
- Real provider smoke requires separate explicit account/quota authorization and the existing `agentintegration` plus `MULTICA_RUN_REAL_AGENT_SMOKE=1` guards. It is not authorized by task planning.
- Windows Job ownership and a verified macOS containment mechanism are required before advertising those combinations. An unavailable mechanism remains a delivery gap and an explicit unsupported result, not a successful capability declaration.
- Ordinary parent wakeup remains best effort in the previously accepted increment; making that delivery durable would require separately scoped work and is not silently added here.
