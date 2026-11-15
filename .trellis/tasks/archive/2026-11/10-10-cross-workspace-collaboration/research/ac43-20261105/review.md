# AC43 acceptance harness review — 2026-11-05

## Verdict

Final result: **partial acceptance confirmed, with observed Codex tool/configuration drift**. Three of four scenario families passed in the retained final attempt; the cross-environment squad was blocked by the actual Claude upstream safeguard response. The named full test exited1 and the parent must not be reported fully accepted or archived on this evidence. The detailed machine-readable review is `final-independent-review.json`.

Earlier sections below preserve pre-execution clearances and original failures. External account/billing reconciliation is excluded by the user's latest instruction; same-machine runtime/configuration separation does not establish OS security isolation or two independently billed supplier accounts.

The reviewer did not call installed providers or execute database tests. The implementer/coordinator retained fixture and real-run evidence, which the reviewer inspected independently.

## Findings fixed by the implementer

1. `ac43_real_provider_test.go`: substring matching could accept an echoed instruction containing the answer marker. The final response now must equal the assigned marker after whitespace trimming.
2. `ac43_real_provider_test.go`: issue creator and current handler evidence originally copied the fixture's expected IDs. They now come from the actual persisted issue row and must match the scenario's expectations.
3. `ac43_real_provider_test.go`: same-environment comparison originally omitted binding/resource epochs and frozen invocation. The evidence/comparison now includes binding epoch, resource epoch, config version and invocation SHA-256 as well as runtime, installation, binding, grant and capability identity.
4. `TestAC43FixtureContract`: an unset task-specific database assignment originally failed every unrelated `agentintegration` suite. Both AC43 entry points now skip when the assignment is absent and reject an explicitly assigned mismatching database.

No production code was changed. The reviewer communicated findings while the implementer retained file ownership, then reviewed the final source.

## Verified design and evidence boundaries

- Native installation enrollment uses distinct Ed25519 challenge signatures; each runtime has its own binding and registered provider capability before resource-owner consent.
- Three human principals remain distinct: issue creator, original handler and resource owner. Both executing agents belong to the recorded resource owner.
- Real execution uses the production `Runner` and `HostExecutor`, managed launch lifecycle, current Multica CLI, actual router and migrated database schema with copied production triggers.
- Human setup/control/transfer calls the real production service directly; it is service integration evidence, not browser or complete human HTTP-route evidence. Runtime list/claim/permit/outcome/proof use HTTP.
- Per-run checks cover persisted runtime/installation/agent/grant provenance, input authors in the actual claim and paginated public records, final successful outcome and settled managed proof. Cross-environment squad checks require a real CLI delegation, member lineage and leader continuation.
- Test database and launcher variables are removed during provider execution and restored afterward. Raw claim credentials remain in memory; serialized HTTP trace fields are scrubbed. Private ledgers and host logs must remain in the private evidence directory.
- The provider capability/configuration hash binds managed invocation facts; it does not identify a supplier billing account. Coordinator source/config/executable manifests provide the surrounding run identity.
- Only the two positive runtime fixtures receive refreshed heartbeats. There is no product freshness relaxation.

## Verification

- `gofmt -l` on both new Go files: clean.
- `go vet -tags=agentintegration ./cmd/server`, from `server/`: passed.
- `go test -tags=agentintegration -c -o <private>/ac43-reviewed.test ./cmd/server`: passed; compile only, no `TestMain` execution.
- Implementer's `TestAC43FixtureContract`: one named pass, no skips, in retained `fixture-contract-2.log`. The original failed `fixture-contract-1.log` is retained; its fixture executable filename collided with RK1's provider directory and was corrected before the passing run.
- One reviewer vet invocation used the repository root instead of the Go module directory and failed before analysis; rerunning from `server/` passed.
- TypeScript lint/typecheck: not applicable; only tagged Go acceptance files changed.
- Real-provider scenarios: pending coordinator execution and result review.

## Reviewed artifacts

Baseline source is `d862c5c4bb55d11074d9f4c51643871764e3a9bb` in the detached checkout recorded by `environment.json`.

| Artifact | SHA-256 |
| --- | --- |
| `server/cmd/server/ac43_fixture_test.go` | `91a405a592ef077b502fc689d8947ded9823666dd9b1dede3e8b095b66c3a943` |
| `server/cmd/server/ac43_real_provider_test.go` | `3fbffa84a3ea86d91be52cc5bd5e526bc5c35b42f813cbfb850f69b1917f7f56` |
| Private `ac43-reviewed.test` | `3168d725f2cf1df6044fb29726dd703718003dc5c2dfb42e982533db4511f95f` |

## Remaining checks

After execution, verify the exact named test result, all seven run records and four scenario assertions; inspect original failures rather than treating a later pass as their deletion. Compare source/config/binary hashes before and after, verify absence of credential literals in publishable records, and confirm owned processes/connections ended. Keep the excluded external billing clause explicit in the parent task's acceptance decision.

## Real attempt 1 — harness failure before provider launch

The coordinator's first execution failed while opening the first run's ledger. `OpenLedger` validates/creates one directory level, but the harness had not created `out/private-ledger`. The retained `real1/summary.json` reports `failed`, with zero completed runs. `real1-test.private.log` and the first per-run record remain intact. The source call order places this failure before HostExecutor construction and `runner.Run`, so no provider launch occurred in this attempt.

The same single-level directory precondition also applies to `HostExecutor.Probe`: the parent `out/environment-N` must exist before creating its `workspaces` child. Both execution-parent cases are being fixed together and added to the fixture contract before the next attempt. This is a harness setup defect, not a passing model sample or product failure.

## Harness v2 clearance

The implementer added `ac43ExecutionPaths`, creating and checking both private parent directories before invoking the production leaf-directory owners. The expanded fixture contract exercises `Ledger.Lock/Close`, `HostExecutor.Probe` with a fake executable, and mode0700 for parents and leaves. `fixture-contract-3.log` contains one actual pass with no skipped/failed test; no provider launched. Reviewer gofmt and `go vet -tags=agentintegration ./cmd/server` passed on v2.

- `ac43_fixture_test.go` remains `91a405a592ef077b502fc689d8947ded9823666dd9b1dede3e8b095b66c3a943`.
- `ac43_real_provider_test.go`: `61cebb651151c6fc05cbb3f70e91366be7e4d791b9d06327c3da9ff4f181f60d`.
- Compiled private `ac43-v2.test`: `4a530e1517dc811ca0d22e6022b1cb84c5f909a9874f3ce585fdc97072e1096f` (reviewer independently recomputed).

V2 is ready for the coordinator's next fresh-database attempt. A harmless leftover `var err error; if err != nil` before log creation is a nonblocking cleanup item; do not change a running candidate merely to remove it.

## Real attempt 2 — first environment passed, second gateway refused

The reviewer inspected `real2-manifest.json` and actual per-run evidence. The named real test exited1 on the reviewed v2 binary/source. Environment A (Claude) completed successfully with `passed_runtime_provenance`, a settled run and `host_exit_observed` proof. Environment B (Codex) reached one authorized managed launch and settled with business outcome `failure`; its one attempt was sealed with `host_exit_observed`. The retained provider outcome reported gateway403: “This account only allows Codex official clients”. This is a real failed attempt, not a skipped test or completed cross-environment scenario.

The scope has not passed: mixed authors, transfer and squad were not reached in this attempt. The coordinator is investigating the permitted adapter/gateway boundary and separately preparing an explicitly partial Claude-only continuation. No header/client-identity spoofing or gateway restriction bypass is approved by this review.

## Harness v3 partial-scenario clearance

The coordinator requested the independent Claude-only business flow while the legitimate Codex gateway failure is investigated. `AC43_SCENARIOS=claude_flow` runs the A baseline, mixed-author batch and ordinary transfer, then records `passed_partial_runtime_provenance` with B and the cross-environment squad explicitly not run. Default `full` retains all four scenario families, with the independent mixed-author/transfer checks ordered before B. Unknown selectors fail before provider version probing. The dead error branch noted above was removed.

The reviewer inspected the delta and independently verified hashes. Gofmt and `go vet -tags=agentintegration ./cmd/server` passed. Source entry hash is `ae4acc02dae8a01a99a26349d3dfa394087b55e61dafcd76dac091f2834db00e`; private `ac43-v3.test` hash is `8a55be03c63407f9dd4562ab52f78f4e6ecfce65df6b52aee234b4ab42c7b900`. The fixture source remains unchanged. This clearance permits the explicitly partial run; it cannot turn the failed B attempt or unrun squad into a pass.

## Real attempt 3 — Claude-only business provenance passed

The reviewer independently read the completed summary and all three run records. The A baseline, mixed-author batch and ordinary transfer each returned its exact assigned marker, succeeded, and sealed its one authorized managed attempt. The twelve compared runtime/binding/grant/resource/configuration/invocation fields are equal across all three runs. The fixture's creator, original handler and resource owner are three distinct users; the mixed batch retains both actual human input authors, and the persisted transfer changes the handler from the original handler to the creator.

The recorded verdict is correctly `passed_partial_runtime_provenance`, with only mixed authors and ordinary transfer listed as complete scenario families. B and the cross-environment squad remain explicitly unrun in this attempt. The earlier gateway failure is retained independently.

## Codex exec continuation review boundary

The user subsequently selected direct `codex exec`. The coordinator requested a task-only protocol bridge without changing the product adapter or spoofing an HTTP client identity. A clearly named bridge-local session handle may map to the real thread ID observed from `codex exec`; it must never be presented as that actual provider thread. Each measured turn must invoke one real exec process, retain its actual events and usage, await its exit, and report success only after both a successful terminal event and exit0. Unknown resume/unsupported requests must fail; no bootstrap model call or implicit latest-session fallback is needed.

Any resulting pass will be labelled `codex_exec_test_bridge` and prove the explicitly chosen exec path under managed wrapper/child supervision. It will not replace the failed built-in app-server sample or establish that the gateway admits Multica's native app-server path.

## Codex exec bridge clearance

The reviewer inspected the complete bridge and independently ran all ten credential-free Python tests: 10 passed in2.529s. The bridge starts no model at initialize/thread setup. A measured turn starts exactly one actual `codex exec --json --sandbox read-only` child with the original text prompt and selected model/cwd/task home. The real `thread.started` identity is recorded separately from the visibly bridge-local adapter handle. Completion requires a real terminal event, validated reported usage, exact observed answer and successful child exit after draining stdout. Unsupported resume and unmapped side-effect items fail. Group-stop testing confirms the child is reached and waited; the evidence describes wrapper plus child supervision.

The one narrow warning exception accepts only the observed pre-turn `disable_response_storage` configuration diagnostic and preserves/counts it privately. The same warning inside an active turn fails. Provider error details stay in private raw output rather than being copied into adapter diagnostics. A review finding corrected the physical-child count to remain0 until `Popen` actually succeeds.

Final reviewed bridge SHA-256 is `4063d0d06ca4ec2aee9d45affe924f050ce2c1c0b47460f20a4c17f35c54dfac`; its test file is `d2de97741c1cb3984637b46743f34b2551aa98368db71691b2a0d0905d51e075`. This permits the coordinator's full business matrix with the explicitly selected exec bridge for B. No gateway identity/header spoofing, account workaround, product adapter edit or synthetic successful provider result is present.

## Real attempt 4 and bridge warning-path repair

The three independent Claude runs passed again. B started one actual Codex exec child, but the bridge rejected the known pre-turn configuration warning because Codex truncated its long displayed source path with an ellipsis and omitted the closing parenthesis expected by the original parser. The reviewer inspected the retained raw warning and launch record: one physical child, actual provider thread ID observed, bridge failure `item outside active turn`, child exit-9 after managed rejection. This attempt does not prove B completion.

The narrow repair keeps whole-string matching of the exact warning prefix/count1 and the exact `disable_response_storage` suffix while accepting a single-line displayed path, including its truncation. Other settings and the same warning during an active turn still fail. Reviewer independently ran12 credential-free tests, all passed in2.725s. Reviewed bridge SHA is `5809e94bb129227e01ebb80cbf3577b6ac524d06b35a6a106f57b29bac318f0d`; test SHA is `f4dfde51fb86728a7d59d992c5b4f5e6169e20ff62a574919ef7c1fea0e998fb`. The original failed attempt is retained.

## Final attempt 5 — independent result review

The actual named test ran against `multica_ac43_20261105_5dafd1c6` and exited1. The reviewer checked the original summary, five per-run records, scrubbed runtime HTTP traces, exec launch/raw stream, retained rollout metadata and source/config/executable manifests.

- Four runs passed: Claude A baseline, mixed authors on A, ordinary transfer on A, and actual Codex exec B. Each has the exact assigned final response, successful business outcome, one authorized/sealed managed attempt and settled `host_exit_observed` proof.
- Three scenario families passed: multiple targets in distinct logical environments, mixed human authors, and ordinary handler transfer. Runtime/installation/binding/grant/resource/configuration/invocation facts remain unchanged across the three A runs. The real input author set and persisted handler transfer match the distinct fixture principals.
- The squad leader reached one actual Claude managed launch, which returned an upstream safeguard rejection. It settled as business failure with `host_exit_observed`; no member or continuation run evidence exists. This is retained as blocked, without rephrasing or changing provider/model to bypass that response.
- B's exec launch has a distinct real provider thread ID and bridge-local handle, one waited child in the wrapper's process group, exit0, drained stdout, exact `AC43_B_OK` text and usage equal to the actual `turn.completed` event. No tool/command side-effect events occurred. The exec wrapper and child PIDs were absent when independently checked after execution.
- Per-run and scrubbed HTTP JSON contain no literal runtime credential or standard API-key token matching the review scan. Private raw streams, homes and ledgers remain private and are not publishable artifacts.

The four harness hashes/modes, five pinned product hashes, clean tracked product checkout, reviewed test binary, Claude configuration/executable and Multica CLI all match their recorded identities. **The global Codex configuration and JavaScript entrypoint changed**, so the final environment is not unchanged. The actual B launch recorded the original entrypoint hash; its retained rollout records CLI0.165.0, `originator=codex_exec` and the same real provider thread ID. Its retained run-local configuration still matches the initially inspected endpoint, model and credential fingerprint. These observations support that actual sample, not acceptance of the newly installed global Codex version. The manifest hashes the Codex entrypoint; no independently preregistered Rust binary hash is available.

The machine review retains both failed global-identity checks (25/27 checks passed) and records this limitation instead of converting them to pass. The built-in Codex app-server gateway403 failure remains separate from the successful explicitly selected exec bridge path. External billing remains excluded. No product bug fix, native installer acceptance or complete AC43/parent completion is claimed.
