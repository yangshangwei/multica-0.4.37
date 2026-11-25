# Research: Runtime R1/R2 closure and Namespace disposition

- Query: Independently verify R1/R2 against stable final02 code/tests/raw receipts, review the updated specs, and retain the exact Namespace policy blocker without inventing historical causality.
- Scope: internal; source/evidence review only, ahead of supported-toolchain full Host replay and final integrated checks.
- Date: 2026-11-25
- Worktree: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`
- Candidate: `/private/tmp/acceptance-fixes-4b9d3494a2/runtime-frozen02/source`.

## Findings

**R1 CLOSED; R2 CLOSED on final02. Nil-safe Unwrap remains correct. No additional runtime code finding in this focused review.** These close diagnostic defects identified by this reviewer; they do not establish that historical D04/D05 causes were found or repaired.

### R1 — Separate ready-pipe observations from concurrent cancellation

`server/internal/daemon/collaborationrun/controller.go:168–187` now distinguishes failed read, absent response, malformed response and an explicit allowlisted helper refusal. At `:350`, the caller captures `ctx.Err()` before Stop/Wait. `namespaceReadyFailure()` at `:190` joins the original read/refusal error with that captured context; `Code` retains the reply observation while `Context` independently retains cancellation/deadline. This preserves `errors.Is` without asserting that cancellation caused malformed bytes or vice versa.

`TestNamespaceReadyFailureRetainsConcurrentCancellation` at `controller_test.go:86` checks empty/malformed/refused reply cases with concurrent cancellation, `ErrUnsupported`, preserved EOF identity and no private-response disclosure. All three subtests and their parent passed in the raw final02 full-package race output. `TestNamespaceProbeDiagnosticPreservesCauseWithoutPrivateText` also passed, including nil unwrap entries being prohibited.

### R2 — Distinguish EOF from input read failure

`server/internal/daemon/collaboration_host_router_test.go:187–195` now records `stdin_eof` only when `io.Copy` returns nil, otherwise fixed `stdin_read_failed`, then preserves cancellation. The actual harness calls this helper at `:350`. The regression at `collaboration_host_router_boundary_test.go:281` drives both an empty reader and a synthetic failed reader, asserting one correctly classified event and cancellation. Its raw final02 race terminal is PASS.

The existing actual-HTTP cancellation/deadline/503 controls and transport-error secrecy control also passed in that same full daemon run. No raw stdin error was added to the recorded event.

### Evidence checks performed

| Evidence | Independent result |
| --- | --- |
| Frozen final02 source | 9,368 file byte/mode entries checked; zero mismatches |
| Runtime-owned files | All six match `source-ready.json`, final02 and current worktree |
| Command receipts | Nine final02 command stdout/stderr hashes and source-manifest links match; each exit0 with unchanged source/config/tool/runner recorded |
| Daemon full race | 2,055 started/terminal identities:2,053 PASS,2 existing SKIP,0 FAIL; raw events match retained inventory |
| Controller full race | 383 started/terminal identities:383 PASS,0 SKIP/FAIL; raw events match retained inventory |
| Required R1/R2 controls | All named parent/subtest terminals present and PASS |
| Typecheck/vet/gofmt | Three exit0 receipts; retained raw streams exactly match recorded empty outputs |
| Linux controller/daemon | Both `go test -c` exits0; compile-only, no Namespace execution inferred |

These Go executions used the explicitly recorded local Go1.27.0. The supported Go1.26.11 memory/order runs announced by the parent are pending separately and are not counted here. Frozen01 memory PASS is evidence about that earlier candidate only; it neither verifies final02's full Host journeys nor explains the original pressure/permit failure.

## Namespace disposition for final integration

**Accept the parent's refusal to execute privileged/unconfined containers. Record exactly nine new-run identities as environment/policy BLOCKED, with each original failure retained as FAIL and its actual old Probe cause UNKNOWN.**

The canonical `e2e/fixtures/collaboration-cross-client-execution.ts:260` explicitly requests `--privileged`, `seccomp=unconfined`, and `apparmor=unconfined` with `redis:7-alpine`. The user's prohibition rules out those flags. No alternative-policy execution can prove why the historical same-scenario probe failed, and there is no basis for attributing those failures to the separate backend cap-drop/default-policy EPERM.

Independently reconciled the original snapshot24 ledger and verified its linked failure-classification hashes:

| Original lane | Exact affected original identities |
| --- | ---: |
| cross-client-01 | 1 |
| lane-close-01 | 4 |
| lifecycle-01 | 2 |
| offline-notifications-01 | 2 |
| Total | 9 |

Full immutable identity strings/titles, original statuses and the new blocked disposition are persisted in `runtime-findings-closure-audit.json`. The two other historical lane-close passing identities are not swept into this blocker. The seven backend Linux Namespace blockers are also a separate inventory.

**Report alignment needed:** the reviewed runtime `report.md` continuation item2 still asks for an admission decision and calls the nine journeys `NOT_RUN/UNKNOWN`. The parent has now made the decision. Update final reporting to `BLOCKED` for execution and `UNKNOWN` for old cause, retaining original FAIL evidence. This is a reporting update after the parent's decision, not a request for further approval.

## Spec review

The added `Prelaunch failure diagnostics` section in `.trellis/spec/server/collaboration-host-execution.md:340` matches the implemented R1/R2 contract: content-free labels, separate status0/503, precleanup context, nil-free unwrap, process-local clocks and no cross-policy or historical-cause inference.

The added `Supported-toolchain vulnerability scans` section in `.trellis/spec/server/verification-environment.md:355` matches the independently accepted security evidence: actual compiler/scanner identities, native scanner for cross-target analysis, source/binary versus runtime separation, and fresh quota evidence that cannot explain a missing-header historical403. No correction found in these additions. Their hashes are retained in the audit.

## Files found

- Runtime `report.md`, `source-ready.json`, `final02/source.json` — declared scope, candidate and owned-file identities.
- `final02/{daemon-full-race01,collaborationrun-full-race01}/{receipt,inventory}.json` and raw `stdout.log`/`stderr.log` — independently parsed terminal inventories and closure controls.
- Final02 build/Linux-compile/typecheck-vet receipts — owning-package and static boundaries.
- `controller.go`, `controller_test.go`, Host router harness/boundary tests — R1/R2 implementation and meaningful controls.
- Canonical cross-client execution fixture and original snapshot24 skip-disposition/failure-classification records — exact nine Namespace blockers and historical UNKNOWN attribution.
- `runtime-findings-closure-audit.json` — independent hashes, named controls, raw inventory reconciliation, specs and exact blocked identity mapping.

## Caveats / Not Found

- No tests, services, target launchers, Docker/SQL or Git operations were run by this reviewer; no product/spec files were edited. Research files only.
- Postreview full memory/order under Go1.26.11, actual packaged Main/oldCLI, and final owned-resource cleanup await their own completed receipts. No pass is inferred from announced or running work.
- C1 scope-restoration closure is not reviewed here. API-issued `mat_` follow-up is also still required: generated token strings plus hashed fixture rows do not substitute for the original issuance path. Preserve both as final-review gates.
- The report's previous memory resource cleanup is not live re-certified here; final cleanup must cover the later runs as well.
- No external references were required; this review relies on local code, raw receipts and the explicit current user/parent execution constraint.
