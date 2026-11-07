# First-run environment finding

The first `cmd/server` race run used the coordinator-assigned database `multica_func7af9_go_06_20261102`. Database identity was verified before tests. The run completed with exit 1: 363 top-level passes, 3 top-level failures and 7 top-level skips. Subtest counts are recorded separately in its `run.json`.

The three failures are `TestPoolSizing_DefaultsWhenNothingSet`, `TestPoolSizing_URLParamsHonoredWhenEnvUnset` and `TestPoolSizing_PartialURLParam`. The coordinator's generated permit injected `DATABASE_MAX_CONNS=4` and `DATABASE_MIN_CONNS=0`. These override the tests' expected unset environment; zero min is itself invalid and falls back before min is clamped to max. Thus actual values were 4/4 instead of the defaults/URL values under test. This is direct evidence of fixture environment contamination, not evidence of a product defect.

The first failure is retained in `06-cmd_server/stdout.jsonl`, `stderr.txt`, `run.json` and `test-outcomes.json`. No assertions, timeouts, product code or tests were changed. Independent packages continue per the original permit. The child has not changed any permit, allocated/migrated/dropped/switched any database, or automatically retried a test.

Recommended correction: retain API pool caps on the task API, omit global pool overrides from Go unit-test environments, and have the coordinator allocate a fresh database/explicit new permit for a separately numbered `cmd/server` rerun. Existing first-run failure remains failed. Any revised environment and rerun must be recorded rather than overwrite the first run.

A parent MESSAGE was received with an empty body during execution, so no new authorization was inferred from that message.

## CLI configuration override contamination

The first `internal/cli` race run completed with exit 1: 71 top-level passes, 4 top-level failures, 1 top-level skip. The failing tests are `TestCLIConfig_BackwardCompat_OldFileLoadsWithNilBackends`, `TestCLIConfig_BackwardCompat_NilBackendsOmittedFromJSON`, `TestCLIConfig_OpenClawOverride_PartialFieldsOmitted`, and `TestCLIConfig_ProfileCommandOverrides_OmittedWhenEmpty`.

These tests set `HOME` to their own `t.TempDir()` and write/read `.multica/config.json` there. The coordinator's global `MULTICA_TASK_CONFIG_ROOT` overrides that location through the implementation's intended task-local root precedence, so the tests read a different configuration location. See source `server/internal/cli/config_test.go:19`, `config_test.go:55` and `config.go:274`. This is another directly established environment contamination, not a product-compatibility regression.

Recommendation: do not inject task-config/workspace-root overrides globally into a unit suite that owns its environment per test. Preserve the original failed run; obtain a revised permit and separately numbered replay. This package is DB-free, so its replay does not require a new database. The CLI guard and explicit fake/missing Agent executables should remain active.

## Daemon configuration and identity contamination

The first `internal/daemon` race run completed with exit 1: 890 top-level passes, 7 top-level failures, 2 top-level skips. All seven failures refer to shared task-local configuration/state replacing the test's isolated HOME location:

- `TestLoadConfig_BackendOverrides_BackwardCompat_NoConfigFile`: unexpectedly finds OpenClaw state from a prior CLI fixture at the shared root.
- `TestPrepareReasonixTaskStateHome` and `TestPrepareDshTaskSessionRoot`: actual paths use the injected task root, expected paths use that test's temporary HOME.
- `TestEnsureDaemonID_Persists`, `TestEnsureDaemonID_PromotesPreChangeProfileFile`, `TestEnsureDaemonID_RegeneratesCorruptFile`, `TestLegacyDaemonUUIDs_ScansProfileDirs`: the injected root bypasses each test's identity fixture directory and shares previous fixture state.

The first evidence is retained under `20-internal_daemon/`. The recommendation is unchanged: remove global test-root overrides from subsequent permits, preserve all original failures and allocate a fresh per-package DB for a separately numbered daemon replay. No production fix is warranted on these facts.

## Last package already started before correction

`internal/daemon/execenv` was already running when the coordinator corrected future permits. Its original result is exit 1: 656 top-level passes, 4 top-level failures, no skips. The failed Hermes memory/session store path-layout and prune tests likewise use the inherited shared task root instead of their temporary HOME. See `22-internal_daemon_execenv/` for the exact identities and original evidence. This is the fourth original affected package and needs a separate DB-free replay.

## Coordinator correction applied to future packages

The child found and read the coordinator's durable `research/go-environment-correction-v2.json`, timestamp `2026-11-02T00:19:14.862665+00:00`, after parent message bodies arrived empty. It removes the four global pool/config/workspace overrides from 60 not-yet-started permits, begins at package 23 (`internal/daemon/processtree`), retains original v1 env files and hashes, and gives each future package a private HOME with explicit existing Go caches. Database assignments are unchanged. The running child reads each permit env file at package start, so packages 23 onward use that recorded correction without the child editing permits or switching databases.
