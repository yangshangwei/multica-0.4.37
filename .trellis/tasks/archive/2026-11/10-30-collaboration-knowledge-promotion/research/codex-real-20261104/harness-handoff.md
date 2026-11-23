# Codex knowledge harness implementation

## Scope

The test-only framework now exercises the production Codex app-server adapter through ordinary daemon claim/run/report and collaboration Host execution. Claude fake proof and the original seventeen RK1 files are preserved. No installed Agent or model was executed by this implementer.

Changed files: `rk1k_entry_test.go`, `rk1k_fixture_test.go`, `rk1k_scenarios_test.go`, `rk1k_provider_test.go`, new `rk1k_codex_{provider,registration,controls}_test.go`, and the tagged `rk1k_driver_agentintegration.go`. Task-local tooling: `compile-harness.py`, `run-fake.py`, `registration-template.json`.

## Contracts

- Explicit Codex provider fixture/driver, actual scoped human producers and server knowledge preparation/authorization; the native RPC wrapper does not translate Codex into Claude frames.
- Unique successful thread/start or thread/resume and turn/start response identities, exact thread+turn events and terminal, exact requested resume identity, no post-terminal answer replacement. K2 compares actual resume against the claim's S1.
- Host inputs require actual ephemeral policy and fresh thread plus scoped CLI memory/handoff reads. Ordinary input requires actual task-token project read and real quick-create linkage.
- Real registration is explicit, hash-bound and expired by default in the template. It binds test binary/source/CLI target/config/model, exact database, output, ledger and 15-slot cap. Fake expected answers, chosen native IDs and fault parameters are removed from real child plans.
- Ledger stages are reserved slot → actual process start → captured process completion → durably published outer sample acceptance. A pre-start failure conservatively consumes a reservation but does not claim an actual start. Any incomplete predecessor, missing/failed evidence, duplicate identity, concurrent owner or exhausted cap refuses the next process. Top-level failure durably stops the series, including zero-start K2 or fake seed failures.
- Complete wrong model answers are recorded `finished/fail`, separate from incomplete execution; the existing scenario requirement then stops the series without extra calls.
- Compile-only tooling binds all tracked/nonignored server files (including module files and embedded assets), binary SHA and before/after equality. Execution tooling binds actual fresh database through PG environment, ambient-agent sentinel, source/CLI/binary, strict JSON terminal inventory and full 15+3/0 summary.

## Verification so far

- Tagged `go test -c ./cmd/server` and tagged `go vet ./cmd/server` passed before dynamic tests; all owned Go files are gofmt-clean.
- `normal-01-run` failed during E0b fixture setup with SQLSTATE23505 before any Provider launch. Cause: base fixture already owns a Codex runtime for that workspace/daemon. Correction reuses the genuinely registered runtime only after workspace/owner/local-mode/profile/daemon assertions; real capabilities and bindings remain authoritative. Original raw/database retained.
- `normal-02-run` passed: exit0, accepted true, full source unchanged, 45 named passes / 8 exact top-level tests, 0 failures/skips/invalid JSON. Durable summary has 18 samples, 15 measured fake starts + 3 fake seeds, 0 paid starts. This includes H1–H4 review controls and exact-resume follow-up.
- Normal02 evidence: `.omx/private/knowledge-codex-20261104/normal-02-run/{result.json,tests.jsonl,proof/summary.json}`; compilation binding `codex-normal-02-compile.json`, binary SHA `82efe5dfa8a169c8d7730e784c492afba45b322ff26c0108ff40cab2c6bdcf83`, CLI SHA `740566ba265e15e46cdbb8fbe29fdfbd4b80147d8d0182e25e0539e6a6698f34`.

## Remaining checks

The coordinator requested final normal03 and race after adapter A2-R log redaction repair, using fresh allocated databases and a newly bound binary/CLI. Do not treat normal02 as a run of that later source. Independent final source/artifact review and root-owned real registration/execution remain pending.

Real timeouts: ordinary driver five minutes, Host five minutes with six-minute enclosing context; fake driver one minute; K2 frozen third authorization barrier thirty seconds; existing eligible-run polling ten seconds. No thresholds were weakened after failures. The fake test command has a twenty-minute suite timeout; root must choose and bind the separate real-suite timeout before execution.

## Final source fake verification

Final normal03 and race01 both **exit0 / accepted / source_stable**, each **45 named passes, 0fail/skip/invalid JSON, 8 exact top-level tests**, durable summary18 samples /15 measured fake +3 fake seed /0 paid. Both compiled from identical full server manifests after adapter A2-R and root embedded docs freeze. CLI SHA `22dd56e7f63decfeee03654fb02602a898bfa927d7c335f9cb8a590a8b7686ab`; normal binary `cc198f11e87566ad6a1bf20270d461ad8f3fd38e95496cd598818f1cd74eb2d1`; race binary `b1730b6855842457d8047bd6e97696f8eaaf0cc7141fc1fb151de32753b3b0ba`. Full references/hashes/timestamps are in `harness-final-fake-results.json`.

The first normal03 compilation correctly rejected source drift when root updated an embedded skill; retained compile03 is not accepted. Recompiled normal03b and race01 had stable complete source manifests.

Root reported an independent runtime task's quiet window overlap after race01 had already started. The original runs finished unchanged and are preserved; they are not described as uncontended. Normal03 UTC16:45:28.526316–16:46:18.058309; race01 UTC16:47:55.393461–16:50:32.551920. No owned normal/race test process remained at16:50:57.248057UTC. No further heavy work was started; root owns cross-task coordination and real window.

Remaining: final independent review, root binding/real execution (fixed90-minute whole-suite timeout, existing per-execution bounds unchanged), and final integration/commit. The implementer has made zero real model calls.
