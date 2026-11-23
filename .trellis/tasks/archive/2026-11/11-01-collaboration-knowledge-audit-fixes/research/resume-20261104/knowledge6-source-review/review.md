# #6 current six-file readiness review — 2026-11-04

**Changes required before dynamic acceptance.** The six files still match the 2026-11-03 K3 review. The issue-local polling defect remains unresolved. All 17 borrowed RK1 files match the framework baseline and candidate `d0b132fff`. This review changed no source and performed no database, suite, service, or Provider execution.

## Open findings

1. **KF-NEXT-LIST, confirmed framework defect.** `server/cmd/server/rk1k_fixture_test.go:229` queries `ListCollaborationRuns` and selects unsettled rows without calling the actual runtime list. The human comment and queue-resume paths leave waiting inputs; `ListRuntimeRuns` drains wakes, promotes waiting batches, and then returns eligible IDs. K1 calls this endpoint through `nextRun`; K3's new helper bypasses it. `normal8` fails before the first root launch with zero pending runs. The later continuation seed-member change from `delegate` to `knowledge` is retained but cannot repair this earlier path.
2. **KF-CURRENT-BIND, acceptance prerequisite.** The framework is still based on `118fc53ea`, with 61 server paths differing from `d0b132fff`. Its six files are uncommitted. The accepted final product/source, current CLI, configuration, dedicated databases, and successful complete normal/race evidence are not bound. A prior compile or old-base result does not close this requirement.

The minimum owning correction stays in the new fixture helper: each bounded iteration calls `f.client.List(f.t.Context(), f.base.runtime)` with the actual MDT client; intersect its eligible IDs with the current issue/session's runs; retain the exactly-one check and timeout. Never choose an arbitrary unsettled run or create one through test-only SQL. Preserve `normal8`, the seed-member change, all six files, and all borrowed 17 files before merging the final accepted source.

## Independently observed evidence

`normal8/raw-go.jsonl` and its `exit-code` retain exit 1: E0b/K1/K2 pass; K3 and the parent test fail. Its 13 saved sample records comprise 12 completed fake launches and the zero-launch K2 refusal; paid and seed starts are 0. The five ordinary K3 rows pass. Root/member/continuation have no sample, and `proof/summary.json` is absent.

All six files were read. The current static contracts retain separate task/run identities; real human producers; actual daemon claim/run/report and HostExecutor paths; original preparation/revocation barriers; observed native ID/result facts; durable capture faults; and fixed 1/4/2/8 measured plus 3 seed accounting. The real admission function still unconditionally refuses before executable/account/model discovery. Pass-through accepts only the exact bytes of the test binary as a stand-in. No additional confirmed product or framework blocker was found in this static slice.

The seven selected framework tests cover finite fake proof, entry/wrapper guards, input/native/credential parsing, and evidence faults. Full stale/unavailable/busy/expiry/noncanonical-ACK boundaries must be mapped to existing canonical product regressions separately; do not claim the selected seven tests cover all those application paths.

## Validation handoff

After #7 closes and the final source is integrated, compile the tagged package with `go -C server test -c -tags=agentintegration -o <new-absolute-output> ./cmd/server` and run tagged vet for `./cmd/server` and `./internal/daemon`. Compilation must not execute `TestMain`. The JSON contains exact proposed guarded normal and race argv. Each uses the seven-test filter, excludes `TestRK1KnowledgeRealModel`, and requires its own new migrated `multica_rk1k_*` database and new evidence output. An existing FakeProof-only permit must be replaced by an exact reviewed filter permit before expanding selection.

Require both complete normal/race exits 0; named E0b/K1/K2/K3 terminals; eight measured K3 identities with actual role/parent/wake; 15 measured fake starts including E0b; 3 separate seed starts; 0 paid; zero third process; final summary and raw artifact hashes. Diagnostic guard/fault launches stay separate from scheduled counters. Bind source before/after, six and 17 file hashes, pinned toolchain/current CLI, exact private environment, migration 764/database identity, and the final independent six-file review.

## Verification and limits

- `gofmt -l` on all six files: exit 0, no output; format and parse check passed.
- Current/previous six hashes and 17 baseline/candidate hashes: matched.
- Raw normal8 terminals, samples, physical launch records, exit, and absent summary: independently read and reconciled.
- Lint, tagged compile/vet, typecheck, and normal/race tests: not executed in this read-only assignment.
- No source simplification or implementation edit was made. Knowledge allocation/calls remain **0/0**, real entry remains closed, and production injection remains default false.
