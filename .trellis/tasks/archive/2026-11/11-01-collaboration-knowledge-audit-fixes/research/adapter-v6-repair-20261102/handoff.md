# Private adapter repair handoff

The owned v7 adapter passes its private regression checks and is ready for the coordinator's independent Trellis review. No commit, launcher clearance or complete product check was created by this agent.

Owner: `/root/adapter_v6_fix`. Implementation: `.omx/private/collaboration-resume-20261101/adapter-v7-knowledge-advance-20261102/`. Evidence: this directory. The pre-existing v6 belongs to the other coordinated session; no v5/v6, launcher, product/spec/status, prepared worktree, Redis or renderer files/resources were changed by this agent.

## Changes

- `check-bin/go`: collect package selectors with the existing compile scanner; mode 0 requires a canonical recorded package, refuses multiple/recursive selectors and compares the pinned Go `list` result with that package. Original cwd and leading `-C` / `-C=` arguments are used for listing, and permitted commands retain their original argv. Ordinary mode 1 requires nonblank explicit Go/psql/evidence context before subprocess/SQL activity. The inherited v6 pool-variable removal remains intact.
- `check-bin/go`: add the cached Go source's forwarded value flags and their registered `-test.*` aliases to the same scanner. Values resembling `-c` now remain values; genuine compile flags, their final true/false setting and `-args` / `--` boundaries retain their behavior.
- `check-go-isolation-test.py`: retain the inherited 14 methods; add canonical owner fields to its two recursive allocation fixtures and make the fake Go list resolve the permitted relative selector to one owner. `check-go-package-binding-test.py` and `check-pnpm-reporting-test.py` provide the new mocked matrices. `deny-network.sb` supplies network denial.
- `check-bin/pnpm` is copied byte-identically. Dependencies are unchanged. Existing parser/recorder patterns were reused; no new adapter abstraction was introduced.

## Retained evidence and results

| Run | Named methods | Failed assertions/subtests | Errors | Child exit |
| --- | ---: | ---: | ---: | ---: |
| Initial matrix against exact v5 (`red-v5.*`) | 27 | 55 | 0 | 1 |
| First v7 matrix (`green-v7.*`) | 27 | 0 | 0 | 0 |
| Adjacent scanner against exact v5 (`scanner-red-v5.*`) | 3 | 121 | 0 | 1 |
| Adjacent scanner against first v7 (`scanner-red-v7.*`) | 3 | 114 | 0 | 1 |
| Corrected scanner (`scanner-green-v7.*`) | 3 | 0 | 0 | 0 |
| Final complete binding/scanner matrix (`final-green-v7.*`) | 30 | 0 | 0 | 0 |
| Final inherited fake-executable suite (`legacy-final-v7.*`) | 14 | 0 | 0 | 0 |
| Unchanged pnpm contracts (`pnpm-v7.*`) | 5 | 0 | 0 | 0 |

The final distinct inventory is 49 named methods; the final binding matrix retains 222 observations. The scanner's 3 methods are included in the 30-method complete matrix. The earlier inherited-suite results are also retained in `legacy-v7.*`. The pnpm evidence is reused because its exact tested bytes remain unchanged; its subject is pnpm, independently of the historical Go hash in that command record.

All runs used `/usr/bin/sandbox-exec` with `deny network*`. The new matrices run exact adapter source with executable/SQL boundaries mocked. The inherited suite executes only test-created Python fake Go/psql/pnpm binaries and a fake guard. Actual Go tests/TestMain, Go list, SQL connections, database allocation/migration, browsers, installed Agent CLIs and Provider/account calls were zero. No product suite or service was started. `-bench -c`, `-benchtime -c` and `-test.run -c` all reached mocked compile-only forwarding without isolation context before the scanner fix; this is a scanner witness, not a claim that malformed flag values would pass real Go validation.

`source-v7-command.json` / `source-check.json` prove five Python sources compile in memory, all final source snapshots match and the two focused Go/fixture diffs have no whitespace diagnostics. `git diff --no-index --check` returns 1 for these ordinary diffs, with empty stdout/stderr; this is recorded, not rewritten to 0.

## Source binding and integration inputs

| Source | SHA-256 |
| --- | --- |
| Exact v5 Go | `2c32f8309a17c5558e004184ec490df3bb6aebbf6101ee0f9156dae3d09e7943` |
| Incoming v6 Go | `b8c5d5adbece4f781a00a230b4f6ffbbfed195f4e44826d53997d72b3f160079` |
| First-green v7 Go | `c987ea45cf75e4fa2fe5858a6bc5d82bb1f30ac289162015ebe544dab8b6382f` |
| Final v7 Go | `eafed32e38ea69de0f73272df5c9cc817ec0741726b54fc19d27466742f57076` |
| Unchanged v5/v6/v7 pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |

`incoming/` contains complete source snapshots for the two incoming adapters, including the unowned v6 launcher test for provenance. `first-green-source/` retains v7 before the scanner correction. `final-source/` contains all six final adapter/test/profile files. `incoming-manifest.json`, `first-green-manifest.json` and `source-manifest.json` record their hashes/modes/sizes. These snapshots contain source and synthetic fixtures, no real credentials.

Use `v6-to-v7-go-and-fixtures.patch` for the small existing-file integration delta (84 lines including context). `adapter-v5-to-v7.diff` and `adapter-v6-to-v7.diff` retain the complete relevant source/test/profile deltas, including newly added files. `cached-go-testflag-reference.json` binds the read-only Go 1.27.0 registration/alias reference used for the scanner correction; no Go command was invoked to obtain it.

The live v6 Go/pnpm/isolation harness still matched incoming hashes during the source check. Its unowned `check-launcher-test.py` changed concurrently; both observed hashes are retained in `source-check.json`. The other coordinator owns that work and the final shared adapter/launcher binding.

## Verification entry points

The exact executed argv/environment/exit/counts are in every `*-command.json`. From the repository root, these reproduce the final private checks with a minimal environment:

```sh
ADAPTER_DIR=/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v7-knowledge-advance-20261102
ARTIFACT_DIR=/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/adapter-v6-repair-20261102

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ADAPTER_DIR/check-go-package-binding-test.py"

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ADAPTER_DIR/check-go-isolation-test.py"

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ADAPTER_DIR/check-pnpm-reporting-test.py"

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ARTIFACT_DIR/check-source.py" "$ADAPTER_DIR" "$ARTIFACT_DIR"
```

Set `CHECK_ISOLATION_WRAPPER` to the exact v5 path when reproducing red. The pre-scanner 27-method test source is retained under `first-green-source/`; the current expanded 30-method suite adds the separate scanner regressions. `CHECK_BINDING_EVIDENCE` / `CHECK_PNPM_EVIDENCE` can select a reviewer-owned result file.

## Remaining limits and handoffs

- Independent review, applying/rebinding the accepted delta and the sole final4 full check belong to the coordinator/shared owner. This implementation creates no clearance.
- Mode 0 retains the existing `running` / `complete` allocation states and permits the same recorded package; it does not allocate another clone. Mode 1 retains its existing argument splitter.
- Discovery uses original cwd/leading `-C` with selectors. Broader outer-mode test-binary forwarding and flag-dependent module/overlay identity were not redesigned. Verify those separately before extending this private adapter beyond the recorded repository check selection.
- Executable availability/content pins, actual database/service identity and final source/launcher hash binding remain coordinator responsibilities. The forwarded value-flag set/aliases must be rechecked when changing toolchains.
- This evidence proves private adapter behavior with fakes. Real Go/DB, product, provider, account and release acceptance remain separate.
