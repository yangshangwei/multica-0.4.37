# Research: Controlled daemon shutdown before human QA

- Query: Does an already settled, build-bound QA report require its controlled daemon to remain online until human approval?
- Scope: Internal, read-only source inspection. No DB/API/browser calls, tests, builds, process signals, or fixture edits.
- Date: 2026-11-16
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Findings

The inspected product contract does **not** require the QA report daemon to remain online after its run has been proven ended and settled successfully. This supports a narrowly verified fixture cleanup experiment, not a claim that stopping the daemon fixes the unproved R20 lock holder.

### The report's authority and applicability

- `testReportReadRun` (`server/internal/service/collaboration_test_report_view.go:114–147`) checks current source membership, the exact issue/session/run, frozen termination identity and frozen test-subject source authority.
- `loadFrozenTestSubject` (`collaboration_test_subject.go:249–268`) reads the immutable server event, exact run/issue identity, validated subject and binding digest. It does not read local daemon files or heartbeat status.
- `testReportApplicability` (`collaboration_test_report_view.go:188–246`) checks report/run agent identity, authorized attempt, managed stage/current candidate, current source/CI, retained logs, absence of forced failure, and `SettledAt` / `collaborationRunProvenEnded` / successful business outcome. It has no online, heartbeat-age, runtime-capacity or future-invocation requirement.
- `stageQAReportInTx` (`collaboration_stage_qa.go:27–87`) adds the exact candidate/version/digest, retained logs, run fence and passed/usable requirement. Human QA approval (`:157–232`) retains current source, signer/dependency, artifact and final-clock checks. It does not require a future launch-capable report daemon.

### The artifact producer is a different resource

The actual retained R20 `StageJourney.releaseReady` creates its ordinary artifact producer in the **source workspace**, owned by Product, at spec line 137, then packages the old/current builds at 138–139. It later creates the **QA execution workspace and controlled daemon**, owned by Quality, at 169–173. Closing the latter does not close or remove the former.

Historical human artifact authority uses `CheckArtifactAuthorityInTx(... full=false, manage=false)` (`delivery_evidence.go:277`). Its running-task gate executes only for `full || kind == "agent"` (`artifact_authority.go:284–305`); a human reading retained evidence does not inherit that gate. `RuntimeLookup.Get` (`agent_runtime_lookup.go:43–46`) returns the existing runtime row unchanged. The subsequent evidence checks verify runtime/workspace/agent binding (`delivery_evidence.go:280–289`), current users/members/invocation grants, exact approval/execution/spec/receipt/output identities and evidence expiry. There is no explicit online-status requirement in this retained human path.

Account, source membership, roles, grants, artifact producer identity and finite evidence expiry remain mandatory. Stopping a different QA daemon is not permission to bypass them.

### What this fixture's close actually does

- `createStageTestResource.close` (`e2e/fixtures/collaboration-stage-approvals.ts:266–272`) calls the native controlled daemon's close, records its cleanup receipt, then stops its owned heartbeat. It performs no member/agent/runtime deletion, credential revocation or stage mutation.
- The native close (`collaboration-host-runtime.ts:374–390`) sends stdin EOF, waits for process exit with its existing 20-second kill bound, closes callback sockets, captures the allowlisted local ledger and removes the test-owned local directory. It does not delete retained server report events or the separate trusted artifact staging/output roots.
- Native harness EOF cancels the poller context; `runCollaborationHostRouterHarness` waits for owned work and reports claims/active tasks (`server/internal/daemon/collaboration_host_router_test.go:240–248`). Normal poll interval is 50 ms (`:190–192`).
- `startRuntimeHeartbeat.stop` (`e2e/fixtures/collaboration-performance.ts:984–985`) cancels its timer/in-flight heartbeat and waits. It sends no explicit removal/revocation request. **Do not claim runtime online status remains indefinitely:** future freshness naturally expires when heartbeats stop.

The original R20 order waits for settled / observed Host exit and reads/validates the retained report and logs at spec 203–210, but closes the QA resource only at line 221, after the human QA action at 218–219. The daemon can therefore still poll while the form mounts and fetches R4. That is a plausible contention source; its exact lock-holder role remains unproved by the original GET-only trace.

## Required proof before an action-order change

No action order was changed by this research. If the coordinator selects a cleanup experiment, require all of the following on fresh, explicitly bound evidence:

1. First obtain the original settled / `host_exit_observed` / successful business outcome, exact report and verified logs.
2. Close **only that owned QA resource**, with actual native `exit_code=0`, `active_tasks=0`, complete allowlisted ledger and no cleanup error. Keep the existing close deadline and artifact producer untouched.
3. Perform fresh authoritative R4 and T4 reads after close: same candidate/version/digest, same report/run/artifact, passed/usable report and logs, and current QA signability. Do not infer these from the pre-close read.
4. Retain current human role/source/dependency/expiry checks, the original five-second enabled assertion and all original case outcomes. Never alter data to mark a run settled, the runtime online, or a report usable.

A matching post-close read proves that this cleanup preserves applicability for the observed fixture. It does not prove the earlier R20 holder was the daemon or that all remaining read contention is fixed. The independent `GetWorkflowOperation` read/read regression and any later holder capture retain their separate evidence.

## Files found

- `e2e/collaboration-stage-execution.spec.ts` — separate producer/QA resources and original settle/report/approval/close order.
- `e2e/fixtures/collaboration-stage-approvals.ts` — owned QA daemon plus heartbeat lifecycle.
- `e2e/fixtures/collaboration-host-runtime.ts` — native EOF/exit/ledger/local-directory cleanup.
- `e2e/fixtures/collaboration-performance.ts` — heartbeat stop without explicit server revocation.
- `server/internal/daemon/collaboration_host_router_test.go` — actual controlled poller and EOF work drain.
- `server/internal/service/collaboration_test_report_view.go`, `collaboration_test_subject.go`, `collaboration_stage_qa.go` — exact retained report and current human approval contracts.
- `server/internal/service/delivery_evidence.go`, `artifact_authority.go`, `agent_runtime_lookup.go` — historical artifact identity/current authority, with no explicit online requirement for human reads.

## External references

None required; current source and the retained R20 test source are the primary evidence.

## Related specs

`.trellis/spec/server/collaboration-stage-approvals.md` preserves current source/signer authority, exact frozen report/artifact identities, successful settlement and final clocks. `.trellis/spec/server/verification-environment.md` requires a fresh source/runtime binding and retained failed outcomes.

## Caveats / Not Found

- Static applicability rules do not substitute for the required post-close product observation.
- In-flight or newly claimed work may still produce cancellation/cleanup effects; actual native zero-work cleanup must be checked rather than inferred from server settlement.
- No actual exclusive holder was identified here, no fixture/product code changed, and no acceptance state advanced.
