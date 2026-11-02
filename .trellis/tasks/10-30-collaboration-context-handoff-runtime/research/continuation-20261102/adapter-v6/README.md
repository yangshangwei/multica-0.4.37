# v6 verification adapter handoff

Status: implemented; independent review and shared full-check execution remain
with the coordinator. No full check, service, database allocation or real
Provider was started by this implementer.

## Repair and evidence

The Go adapter removes `DATABASE_MAX_CONNS` and `DATABASE_MIN_CONNS` only after
non-test/compile-only forwarding, before either executable-test path. The
existing API/non-test paths retain their task budget. The production tests and
their assertions are unchanged. `check-bin/pnpm` is byte-identical to v5.

| Verification | Observed result | Evidence |
| --- | --- | --- |
| v5 child-environment red regression | 2 failures, exit 1 | `red-v5-pool-environment.json` / `.log` |
| v6 original isolation and new environment contracts | 17 passed, exit 0 | `green-v6-isolation.json` / `.log` |
| Literal parser, downstream shell loading, actual hash/review/candidate gates and preserved exit codes | 11 passed, exit 0 | `green-v6-launcher.json` / `.log` |
| Existing exact-file `dbstats.go` / `dbstats_test.go` against v5 | 7 run: 3 pass, 4 fail, exit 1 | `pool-precedence/v5.json` / `.jsonl` |
| Same unchanged existing files against v6 | 7 run: 7 pass, 0 fail/skip, exit 0 | `pool-precedence/v6.json` / `.jsonl` |
| Python syntax and compilation | Five copied sources parse; all six Python sources compile | `source-manifest.json`, `python-compilation.json` |

The exact-file Go commands exclude `integration_test.go` and its `TestMain`.
They run under `sandbox-exec` with network denied. Their mode-0 allocation is an
owned fake record only; no corresponding database was created or connected.
Production source hashes before/after match. See `check-pool-precedence.py` and
`pool-precedence/summary.json`.

Two early launcher contract attempts had fixture defects: an escaped NUL in a
fake subprocess argument, then a `/var` versus `/private/var` path expectation.
Their original logs/records remain as `launcher-attempt1-fixture-escape-failure`
and `launcher-attempt2-fixture-path-failure`. The final eleven-test pass uses the
same production launcher template and corrected owned fixtures.

## Launcher template

`run-make-check.py` is a source template. The coordinator installs it under the
selected candidate's `.omx/private/final-candidate/` and supplies
`launcher-config.json`. `launcher-config-example.json` records the prepared
final5 identity, paths and input-env hash; it is not an execution permit.

The launcher:

1. Verifies candidate root/HEAD/cleanliness and actual adapter, launcher and
   source-env bytes against configuration and independent clearance.
2. Parses literal shell-quoted assignments without sourcing the input file.
3. Writes a new mode-0600 `check-source.canonical.env` using `shlex.quote` for
   every value. Downstream Make/Bash reads this canonical file, so substitutions
   remain literal at both loaders; the input file remains unchanged.
4. Sets an explicitly allocated private `HOME` for child processes while
   preserving configured Go cache/module paths and Playwright browser files.
   It does not set `MULTICA_TASK_CONFIG_ROOT` or `MULTICA_TASK_WORKSPACES_ROOT`,
   allowing tests to select their own HOME/task roots.
5. Verifies the private mode-0700 build-temp allocation is outside Git, retains
   API 10/1, and preserves the existing guarded per-package/Playwright paths.
6. Refuses prior evidence overwrite and records actual command, exit code and
   source/canonical/launcher/adapter hashes.

The independent clearance must include `verdict` (`pass` or `pass-with-notes`),
`blocking_findings_open: 0`, exact `adapter_sha256` and exact `launcher_sha256`.
No implementer-issued clearance is supplied.

This parser intentionally supports the task's literal POSIX shell-quoted
assignments, not executable shell programs or expansion-based configuration.

## Source hashes and coordination

- Go adapter: `eabbfdfe047ec686a6e25d413be9dc14dc1ce068ac72317530d6ec049f84aa29`
- pnpm adapter: `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48`
- Launcher: `836f2d066b74edbb14e39fda49ac314e40f11b9618cbd8d07eb44c020747ad1f`

At 13:09 UTC, matching runtime copies were created in
`/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/collaboration-resume-20261102/adapter-v6`.
Copying refused existing files with different bytes; earlier v5 files remained
unchanged. `source-manifest.json` is the snapshot at that time.

The coordinator then identified another authorized session preparing v6 and
owning the shared full check, Redis 16422 and renderer 19845. This implementer
stopped all writes to the possibly shared runtime directory. It has no running
processes and has not installed the template into either candidate. The
coordinator must resolve private-path ownership, independently review the
chosen exact bytes/configuration, and share that owner's single full-check
result. The versioned source and red/green evidence in this directory remain
the handoff surface.
