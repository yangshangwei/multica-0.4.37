# Independent adapter v5 review

Verdict: **changes-required**. `blocking_findings_open: 2`. Reviewer: `/root/adapter_v5_review` (`trellis-check`). This verdict does not clear the fourth launcher.

The exact v4 AV4-1 mode-absent `go test` path now refuses with exit 2 before Go/SQL. V5 still fails the two ordinary-test isolation contracts below. The v4 rejection and historical complete-check failures remain intact.

## Findings (fixed)

None. This reviewer owns the two reports and private review output only; the adapter and product sources remain unchanged.

## Findings (not fixed)

### AV5-1 — high, blocking, open

- File: `.omx/private/collaboration-resume-20261101/adapter-v5/check-bin/go`, line 60; validation at 73 and forwarding at 75. Coverage gap: `check-go-isolation-test.py`, line 99.
- Issue: recursive mode validates a clone-shaped URL and allocation database/template/state, then forwards arbitrary original arguments. It never reads `allocation.package` or resolves the effective package selection. Other packages and multi-package calls can reuse the owner clone.
- Evidence: with allocation package `example/first`, the extra probe uses valid private module/package directories. Six unsafe selections are forwarded: no-selector from the second-package cwd, absolute `-C`, `-C=`, relative `-C ../second`, `./...`, and two explicit packages. All reach mocked `execv` on the same clone, with no expansion or fresh allocation. The first probe additionally confirms missing package identity and completed records are accepted; missing/wrong database records correctly refuse.
- Fix requested: require the canonical allocation package, resolve selectors with the pinned Go and identical global `-C` semantics, use `.` when selectors are absent, and accept only the exact permitted single package. Refuse other/multiple selections before testing or send them back through fresh per-package allocation. Preserve the normal allocation and compile/non-test paths. Add the missing negative matrix; the current recursive positive test has no package identity and cannot catch this defect.
- Not fixed: outside assigned writes. Coordinator owns a new adapter version and independent recheck.

### AV5-2 — medium, blocking, open

- File: the same `go` adapter, line 14; mode1 checks start at 59 and do not require explicit `REAL_GO` before SQL/list/test.
- Issue: with mode1 and a valid local template/evidence/PSQL but `MULTICA_TASK_REAL_GO` absent, ordinary tests use the hard-coded `/opt/homebrew/bin/go` fallback.
- Evidence: the independent test observes mocked template SELECT → fallback Go list → clone CREATE/SELECT → fallback Go test Popen, finishing exit 0 instead of refusing before subprocesses.
- Impact: this path still allocates a clone. The defect is incomplete explicit context and executable/toolchain binding; the reproduction does not demonstrate default-database writes.
- Fix requested: validate every required nonblank ordinary-test setting, especially explicit `MULTICA_TASK_REAL_GO`, before SQL/list. Reserve fallback for non-test or genuine compile-only calls. Test mode1 with absent/empty REAL_GO and require exit 2 plus zero Go/SQL calls.
- Not fixed: outside assigned writes; repair with AV5-1 in a new version.

## Verification

- Lint: **not run**; no repository Python lint command is configured for these private adapters. In-memory syntax/AST checks pass for the three reviewed Python sources. No project lint pass is claimed.
- TypeCheck: **not applicable** to this Python-only review; no TS/Go source edits and no project typecheck pass claimed.
- Tests: **fail** for clearance. Existing suite: **11/11 pass**, exit 0. Independent suite: **20 methods**, **15 passing / 5 failing methods**, **8 failing assertion/subtest entries**, **0 errors**, exit 1. Failures identify AV5-1 and AV5-2.
- Normal allocator contracts pass with fake SQL/Go: distinct clones per package, template/clone current-database checks, named JSON events, silent-TestMain refusal, compile-only no-test accounting, original flag/`-C` preservation and failing process exit preservation.
- pnpm contracts pass: test selection, grep/project, workers, retries, timeout and output stay unchanged; default reporter addition and both explicit reporter forms are correct. The actual candidate repository Agent CLI guard wrapped only a test-created fake pnpm and installed its marker/sentinels; no agent CLI was resolved or run.
- Stripped cached forwarding: actual network-denied `go version` exits 0 with `go version go1.27.0 darwin/arm64`, no DB/private isolation settings. This is version-only evidence.

Actual commands and exits are included in `adapter-v5-review.json`. They invoke `/usr/bin/sandbox-exec -f <review>/deny-network.sb` followed by the pinned Python fake suites/probes, the exact adapter `go version`, or exact pnpm adapter with a fake pnpm. The observational selector probe exits 0 while recording six unsafe forwarded selections; that exit is not a safety-gate pass.

## Source and evidence binding

| Artifact | SHA-256 |
| --- | --- |
| v5 go | `2c32f8309a17c5558e004184ec490df3bb6aebbf6101ee0f9156dae3d09e7943` |
| v5 pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| existing contract source | `5c5aad4d0e04427a1c53eadec2067a2ac355701c9d4605e722a87ccf9acad4e7` |
| independent contract source | `bbc0d80f3f7edf10c76de8ff979f07fbe8b4dc3d437a4f4e6495ec7ac350e5da` |
| existing suite raw | `4b9e2fa6f246c1bb2bdac7d179500e3acd4671eced2f866c25835fac5e8e6da5` |
| independent suite raw | `030f5c23cd6ce4dbc3d492bfa4dc56d7412e517d837629bf3ce74165fe20a38a` |
| original recursive probe raw | `87add1326eee758491d9fd2db07413a84a818e2a7d135ec0e9ba9eb68b632891` |
| valid-directory selector probe raw | `006f464918dc69b767f6c4cf97bd0173bbd9de404ea07368aee58a9589bfcaec` |
| actual repository guard + fake pnpm raw | `08d5f64940ee6666e902ab1228e7c8c3f32cd2c3d016557518ff83f726e85b10` |
| stripped cached version raw | `c4a5200d52279a5227d4b3bced622c76081c45d24dba750db29903f9b14641c3` |

Both adapter hashes match the preparation and local verification records. Candidate Git HEAD is `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`; server/apps/packages tree IDs match preparation. The JSON contains all inspected source, artifact and raw hashes plus exact commands. Private review output is under `.omx/private/collaboration-resume-20261101/adapter-v5-independent-review-20261102/`.

Retained preparation replay was read, not rerun: both Web/Desktop named results are passed with zero retries. Results/output hashes, stats, both test-source hashes and the sealer helper hash match their recorded evidence. The raw log is `c2b0c1b3cdcf0fd440087d848c2f7b7ff27ec9c26fea7b0bf4059ef940201153`; results are `2c339738c485761b3d84cf90040f98ef7b2454e90537797216a653196f8e20be`. Local 11-contract raw and historical v4 six-contract raw hashes also match. These positives do not override the new blocking contracts.

## Limits and handoff

No real database allocation/migration/TestMain, services, browsers, real pnpm, agent CLI, Provider, account access, downloads, global environment change, product/adapter edits, commits or child agents occurred. The Go tool ran only `version`; test/SQL/pnpm requests otherwise used test-created executables or mocks. The repository guard ran with a test-created fake pnpm.

Live candidate/environment startup prerequisites and RKI6 belong to the coordinator and its other reviewer. This report does not certify current renderer, seed DB, listeners or working-tree cleanliness. Existing specs already express the violated isolation rules; repair the adapter/contracts and update task/launcher binding in a new version, then obtain an independent recheck.

The original third complete check remains **exit 2**, browser **296 pass / 47 fail / 89 skip / 7 not run**. The fourth check has no clearance from this report. Real-model, installation/account and release acceptance remain outside this slice.
