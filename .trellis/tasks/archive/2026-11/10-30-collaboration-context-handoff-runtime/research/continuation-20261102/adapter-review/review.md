# v5 private verification adapter independent review

Verdict: **changes-required**. Two blocking findings remain in the prepared fullcheck path. No product, adapter, launcher or shared task record was modified; no clearance was issued.

## Findings (not fixed)

- **AV5-1 — medium, blocking:** `mc-collaboration-final4-20261102/.omx/private/final-candidate/run-make-check.py:25` globally exports `DATABASE_MAX_CONNS=10` / `DATABASE_MIN_CONNS=1`. The v5 Go wrapper copies those keys to each package child at `check-bin/go:136–147`; `scripts/check.sh` → `test-go.sh` → the agent-CLI guard does not strip them. Both reviewer-owned fake package children demonstrably receive 10/1. Existing `cmd/server/dbstats_test.go` cases require the keys to be unset to test defaults and URL precedence. Keep the API budget, strip it from Go test processes, and add a no-DB contract. This proof ran no real TestMain or database query.
- **AV5-2 — medium, blocking:** `run-make-check.py:16` runs `bash -c 'set -a; . "$1"; env -0'` to load `check-source.env`. This is shell evaluation, contrary to `.trellis/spec/server/database-test-isolation.md` (“do not … source a dotenv file as shell code”). The identical loader executes a harmless `$(printf …)` substitution in a reviewer-owned fixture. The actual private env was never executed and is not claimed to contain dangerous content. Replace this with the controlled dotenv parser and a regression proving unsupported substitutions are not evaluated.

Both findings are outside the reviewer's assigned write scope; the main session owns repair delegation.

## Prior finding closed

AV4-1 is fixed by v5: non-test and real compile-only calls can forward without DB markers, but ordinary `go test` refuses incomplete isolation context before real Go or SQL. The missing-mode cases also cover `-C`, a test-binary `-args -c`, `-run -c` as a flag value, and `-c -c=false`. Recursive tests require a local, named clone with a matching allocation record.

## Nonblocking notes

- Recorded adapter hashes equal actual bytes at review time. The launcher compares the clearance against constants, but not against current file bytes. Recompute and compare those bytes immediately before launch (already required by the continuation plan); an explicit equality in the replacement launcher is preferable.
- This is a private adapter for the fixed make-check commands. Arbitrary Go `-args`, overlay and linker-flag combinations were not verified.
- The source env declares no private HOME/cache paths, and the wrapper preserves caller HOME. Follow the package-home isolation rule where the full run needs account isolation. This reviewer did not execute account/home-writing tests.

## Verification

- Pinned v5 contracts: **11 pass, 0 fail, exit 0**; source hash `5c5aad4d0e04427a1c53eadec2067a2ac355701c9d4605e722a87ccf9acad4e7`.
- Go adapter SHA: `2c32f8309a17c5558e004184ec490df3bb6aebbf6101ee0f9156dae3d09e7943`.
- pnpm adapter SHA: `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48`.
- Database-free supplemental fake children confirm API-pool inheritance; owned dotenv fixture confirms shell evaluation.
- Stripped `go version`: **exit 0**, Go1.27.0 darwin/arm64, under `sandbox-exec` with all network denied and no DB or isolation marker environment.
- Python AST syntax: pass for both adapters, contract test and launcher. Bash syntax: pass for candidate `check.sh`, `test-go.sh` and the CLI guard.
- Temporary allocation is still mode0700, owner501 and outside Git. Candidate HEAD remains `64a9994ec`, with a clean working tree.
- Repo lint and TypeScript typecheck were not run in this read-only private-harness slice; the full candidate owner must run them after fixes. No Python lint package is installed. This is not a whole-candidate pass.

Reproduction: `python3 .trellis/tasks/10-30-collaboration-context-handoff-runtime/research/continuation-20261102/adapter-review/replay.py`.

Evidence: `verification.json`, `contracts-replay.log`, `preparation-readonly.json` and `review.json`. Historical failed runs remain unchanged. No DB, API, browser, GUI, container or real Provider was run; only report files and reviewer-owned temporary fixtures were written.
