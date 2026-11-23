# Frozen v8 adapter handoff

V8 passes the private regression gate and is ready for coordinator-owned independent review. The source bytes are frozen. This agent created no commit, review clearance or complete product check.

Implementation: `.omx/private/collaboration-resume-20261101/adapter-v8-knowledge-advance-20261102/`. Evidence: this directory. Ownership stayed inside those two new directories. The source check confirms all six incoming v7 files and all 59 previously hashed v7 evidence files remain unchanged. No v5/v6/v7, shared final4, launcher, product/spec/status, service or other resource was edited.

## Corrections

- `check-bin/go` now classifies the complete pinned Go1.27 build/test registry: 70 value flags and 26 Boolean flags, including 29 actual forwarded test aliases. This covers `-installsuffix`, `-gccgoflags`, the remaining build flags, coverage aliases and the `IsBoolFlag` exceptions `buildvcs` / `v`. Unsupported options before a test-binary boundary refuse with a content-free error before any Go/SQL operation, including a future bare flag whose value resembles `-c`.
- The scanner normalizes Go's two-dash option spelling, honors `--args` / `-args` / `--`, preserves the last actual true/false `-c` setting and stops at the positional test-binary tail. Leading `-C` / `--C` and their inline forms, including Go's leading command-local position, use the same directory for package listing. Original argv remains intact for forwarding. The mode1 argument splitter is unchanged.
- Mode0 refuses actual `-modfile` / `-overlay` options in separate or inline form before listing or test execution. Those strings remain valid as another flag's value or as binary arguments. Genuine compile-only helpers remain exempt. This deliberately matches the current full-check selection, which needs neither source override.
- `check-go-package-binding-test.py` adds eight source-backed regression methods and removes the newly forbidden source overrides from one trusted-flag positive matrix. `go-flag-registry.json` is independent test data; the adapter itself has no runtime dependency on it. The inherited isolation harness, pnpm adapter and pnpm tests remain byte-identical to v7. No dependency or new parser abstraction was introduced.

## Results

| Evidence | Methods / calls | Failures | Errors | Exit |
| --- | ---: | ---: | ---: | ---: |
| Eight new methods against exact v7 (`red-v7.*`) | 8 methods | 246 assertions/subtests | 0 | 1 |
| Full v8 binding/scanner suite (`full-green-v8.*`) | 38 methods | 0 | 0 | 0 |
| Inherited v8 fake-executable suite (`inherited-v8.*`) | 14 methods | 0 | 0 | 0 |
| Reused unchanged pnpm proof | 5 methods | 0 | 0 | 0 |
| Plain offline Go-list oracle | 2 metadata calls | 0 | 0 | 0 each |

The final distinct passed inventory is 57 named methods, including the reused pnpm proof. The full matrix retains 658 observations. The new regressions include both original witnesses, `test --run -c example/first`, `test -c=true --c=false example/first`, all source-backed flag classes/aliases, unknown options in empty/mode0/mode1 contexts and the requested source-override value/boundary cases.

All regression runs used `deny network*`. New matrix exec/SQL boundaries are mocked; the inherited suite executes only test-created Python fake executables/guard. Actual Go tests/TestMain, SQL/DB/migrations, browsers/services, installed Agent CLIs and Provider/account calls were zero.

The separate `plain-go-list-oracle.json` records exactly two permitted read-only Go metadata executions under network denial and explicit offline/private HOME/cache settings. The same minimal source lists as `example/first` normally and `alternate/first` with actual `-modfile alternate.mod`; both exit 0 and all three fixture-source hashes remain unchanged. There are no test files or TestMain in that fixture. The Go binary hash is `71c4991041d8e44975c882e4f72005719c958013d3340dc665a3808b72ddf702`.

`source-check-command.json` / `source-check.json` record exit 0, five Python sources compiled in memory, ten exact final snapshots, equality with the 70/26 official registry, preserved v7 source/evidence hashes and clean whitespace diagnostics. The two `git diff --no-index --check` commands return 1 for ordinary diffs with empty stdout/stderr; that actual status is retained.

## Frozen inputs for review/integration

| Source | SHA-256 |
| --- | --- |
| Incoming v7 Go | `eafed32e38ea69de0f73272df5c9cc817ec0741726b54fc19d27466742f57076` |
| Final v8 Go | `0d8d62797c887440d23b129f2e0d86eab4bbea88dcffc463de6478ea1bc65330` |
| Unchanged pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |

- `v7-to-v8-go.patch`: minimal Go correction, 89 lines including context.
- `incoming-v6-to-v8-go.patch`: cumulative production-adapter correction against the preserved incoming v6, 118 lines including context.
- `adapter-v7-to-v8.diff` / `adapter-v6-to-v8.diff`: complete source/test/registry/oracle-fixture deltas.
- `incoming/` / `final-source/`, `incoming-manifest.json` / `source-manifest.json`: full source snapshots, modes/sizes and hashes. The incoming v6 snapshot is the earlier frozen baseline; live v6 remains unowned.
- `references/`: the independent v7 finding/metadata records, complete cached Go registration/alias/Boolean/parser excerpts with full-file hashes, and the 96-name classified registry. No real credentials are included.

The pnpm proof is retained at the earlier evidence directory's `pnpm-v7.json`; `source-check.json` verifies its exact wrapper hash and 5/0/0 results without rerunning it. Its scope is unchanged pnpm bytes, independently of its historical Go metadata.

## Exact verification entry points

Each `*-command.json` retains the executed argv/environment/exit/counts. To run the final private suites from the repository root:

```sh
ADAPTER_DIR=/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261101/adapter-v8-knowledge-advance-20261102
ARTIFACT_DIR=/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/adapter-v8-repair-20261102

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ADAPTER_DIR/check-go-package-binding-test.py"

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ADAPTER_DIR/check-go-isolation-test.py"

env -i PATH=/opt/homebrew/bin:/usr/bin:/bin TMPDIR="$ADAPTER_DIR/.tmp" PYTHONDONTWRITEBYTECODE=1 \
  /usr/bin/sandbox-exec -f "$ADAPTER_DIR/deny-network.sb" \
  /opt/homebrew/opt/python@3.14/bin/python3.14 -B "$ARTIFACT_DIR/check-source.py" "$ADAPTER_DIR" "$ARTIFACT_DIR"
```

For red, set `CHECK_ISOLATION_WRAPPER` to the exact v7 path and use the eight method selections in `red-v7-command.json`. `CHECK_BINDING_EVIDENCE` can select a reviewer-owned report path. Read the retained oracle commands/results rather than running any real Go test.

## Remaining limits

The pinned registry intentionally rejects unsupported options before binary boundaries; recheck actual Go definitions/aliases before adding a flag or changing toolchains. Mode0 source overrides are deliberately unsupported, and mode1 keeps its existing repository-check splitter. This is private adapter evidence, with only two real metadata calls; it provides no real Go/DB/product/Provider/account acceptance. Independent pass, applying/rebinding the shared adapter/launcher and the sole full check remain coordinator responsibilities. V8 stays frozen for that review.
