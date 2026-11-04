# RK1 E0a paid-entry preparation

Status: preparation only; `runnable=false`. This review performed source reads
only, with no test, build, database, installed Provider/version probe or account
call. The deterministic gate is accepted on current source, but the previously
released heavy-test window is not an exclusive model window. A native messaging
502 is not an acknowledgement. Root must receive the explicit window agreement
before any launch or the proposed upfront build.

## Accepted inputs and current remaining facts

- Tested product HEAD: `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`; source ID
  `6b342ba23c2f5ae6826e8841794d086d1258274d3a39eac84983bfb9f429bdea`, 7,990 files.
- Seventeen-file framework:
  `5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9`.
- Unmodified `run_rk1.py`:
  `3bc7962c996143200b1834e9246ec215e8a0d3e03fc38e832abd2eddb1769c99`.
- Selected Provider path `/Users/artisan/.local/bin/claude`, resolved installation
  `/Users/artisan/.local/share/claude/versions/2.1.318`, recorded bytes
  `677a2ea7bf576cedd20e191d973914ed8a41796f53a63b1923381ddc4d5b5e7d`.
  The path name is installation metadata; the supported observed version is
  still obtained by the existing harness's single 15-second version probe.
- Requested model remains `claude-opus-5-5`, provider-default thinking. Do not
  substitute a model or claim that requested and observed model are identical.
- Current normal02/race03 each have 62 named passes; independent reviews checked
  912/915 conditions. Full password7 is accepted separately. The failed race01,
  overlapped race02 and original-series E5 canary finding remain unchanged.
- Runtime allowance remains 25, consumed 0; original remaining8 is stopped and
  cannot be transferred. E0a uses one measured launch. Knowledge remains outside
  this allowance.

The clean execution checkout is
`/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104`. At this review its
HEAD is still 1a32, product roots and the launcher directory are clean, and its
RK1 public-results and private-invocation inventories are empty. However, its
`framework-registration.json` is the old copy with empty source ID. The updated
registration currently exists in the main checkout, not the execution copy.

Root must review and commit the exact registration/addendum updates, then apply
only that reviewed task-record commit to the execution checkout. Preserve the
tested product/framework bytes and recheck the canonical source ID. Record the
new execution HEAD separately from tested product HEAD. Do not run the launcher
from the dirty main checkout: it derives REPO from its own `__file__`, not cwd.
Do not copy registration over as an uncommitted change: `inputs_dirty()` rejects
any change under `real-model-rk1/` other than `results/`.

The new current-acceptance fields supersede old final639/v10 fields still retained
in registration. Those historical fields do not grant current clearance. Before
launch, the effective registration must have the reviewed current source,
framework, launcher, executable hash and `operator_configuration`; only after
the model-window agreement may Root commit `runnable=true` with accurate reasons.

## Exact environment boundary

Keep the original operator HOME `/Users/artisan` and its existing Claude files,
hooks, skills, plugins and credential resolution. Do not use a fake-proof HOME,
copy account files into a sandbox, unset genuine Provider credentials, inject a
different `CLAUDE_CONFIG_DIR`, or alter output caps/retry configuration. Public
evidence contains hashes, presence facts and non-sensitive paths only.

Use the launcher’s `operator_facts()` result under the **actual final subprocess
environment** as the equality contract. Its current registration records all
listed provider/proxy/config selector environment variables absent. Preserve
absence; adding an empty value is different. Preserve legitimate user-facing
`CLAUDE_CODE_*` settings. Remove only exact session markers from the launcher's
`SESSION_ONLY_ENV`; do not blanket-strip the user-facing namespace. If plugin
contents or operator configuration change, stop before launch and record/review
the actual updated binding; do not relabel old hashes.

Prepare the subprocess environment as a structured dictionary from the operator
environment, not by sourcing API dotenv or a prior browser environment. Remove
inherited task/server/test state before setting the few required values:

- Prefixes `MULTICA_`, `RK1_`, `RK1K_`, `DATABASE_`, `POSTGRES_`, `PG`,
  `PLAYWRIGHT_`, `COLLABORATION_`, `E2E_`, `NEXT_PUBLIC_`, and `REDIS_`.
- API/browser-only exact keys such as `APP_ENV`, `PORT`, `FRONTEND_PORT`,
  `FRONTEND_ORIGIN`, `REMOTE_API_URL`, `CORS_ALLOWED_ORIGINS`, `ALLOWED_ORIGINS`,
  `ALLOW_SIGNUP`, `DISABLE_WORKSPACE_CREATION`, `ENV_FILE`, `JWT_SECRET`, and
  `ELECTRON_RUN_AS_NODE`.
- The exact launcher session-marker list. Keep unrelated operator settings and
  their presence state; never print the full resulting environment.

Then supply only the recorded owned database URL and pinned build/temp settings:

| Variable | Required value / rule |
| --- | --- |
| `PATH` | Prepend `/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin` to the operator's normal PATH; never a fake CLI guard/sentinel or adapter directory. Pin the selected git executable in the receipt. |
| `GOROOT` | `/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64` |
| Go executable SHA-256 | `a1c83801d1756c3eca78366c6b585f2c21c20694fb1c7eb92c446a0580420412` |
| `GOCACHE` | Recorded existing cache `/Volumes/artisan/dev-cache/go-build`, or a new recorded private cache; use exactly the same value for upfront and nested CLI builds. |
| `GOMODCACHE` | `/Users/artisan/go/pkg/mod` |
| `GOPATH` | New task-owned private path, recorded without exposing account configuration. |
| `TMPDIR`, `TMP`, `TEMP` | One newly allocated owner-only 0700 directory outside every Git checkout. |
| `GOTMPDIR` | Explicitly the same task temp directory, or absent for both builds/tests; never an inherited foreign path. Go1.26.6 `testing.TempDir` consults this before falling back through `os.MkdirTemp` to TMPDIR. |
| `GOTOOLCHAIN`, `GOENV`, `GOWORK` | `local`, `off`, `off` |
| `GOPROXY`, `GOSUMDB`, `GOTELEMETRY` | `off`, `off`, `off` |
| `GOMAXPROCS`, `GOFLAGS` | `2`, `-p=2`; no race, overlay, trimpath, buildvcs override or extra ldflags. |
| `DATABASE_URL` | Root-allocated and verified fresh `multica_rk1_*` database migrated through `764_collaboration_ai_policy`. Keep URL private. |

Do not inherit API pool overrides, resource-publish paths or knowledge settings.
The existing launcher creates its own `MULTICA_RUN_REAL_AGENT_SMOKE` and
`MULTICA_RK1_*` variables; do not bypass it with a direct real-test command.
The provider backend strips inherited `MULTICA_*` and adds only the admitted
collaboration environment. The RK1 wrapper additionally strips `DATABASE_URL`,
`RK1_*` and `PG*` before invoking real Claude. HOME and genuine Provider settings
remain available. API-only keys outside those production filters must already
have been removed from the parent dictionary.

Allocate/migrate/verify the owned database only after the coordinated preparation
window. Record actual `current_database()`, migration764, fresh-state and cluster
connection headroom. Never reuse the fake normal/race, browser or failed database.
The launcher's `--check-only` validates the URL prefix but does not connect to it;
the caller owns DB identity/migration evidence. `TestMain` runs before the selected
test and can exit0 on an unreachable database, so a final exit0 requires the
requested actual E0a public record and complete lifecycle evidence as well.

## CLI artifact capture without changing the framework

Existing source/tool facts prove what the nested build is instructed to compile;
they do not identify its actual output bytes. `rk1BuildEnv` creates a new
`t.TempDir`, copies the test wrapper, and executes exactly
`go build -o <temporary>/multica ./cmd/multica` from `server/`.
Neither `rk1Trial` nor `rk1LaunchMeta` stores that Multica binary hash. Do not use
the older EDA CCG binary hash as this regenerated executable's identity.

After the final task-record commit is present in the execution checkout and the
explicit window is granted, perform one upfront build using the pinned Go and
the exact final environment:

```text
cwd: /Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/server
argv: [<pinned-go>, "build", "-o", <new-private-path>/multica, "./cmd/multica"]
```

Keep argv, actual exit, environment fingerprint, source ID, execution HEAD,
Go binary hash, CLI bytes/hash/size and `go version -m <binary>` build information.
The latter reads the file with Go; it does not execute Multica or a Provider.
Use an ignored `.omx/private/` output or the external task temp directory.
Confirm the output path is ignored and whole-checkout Git status does not change.

Default Go VCS stamping matters: pinned Go's `gitStatus` runs repository-wide
`git status --porcelain` and embeds `vcs.revision`, `vcs.time`, `vcs.modified`.
Changing only task documents can therefore change CLI bytes while source ID stays
constant. Build after the final execution commit, with no intervening commits or
tracked/untracked writes outside ignored outputs. Do not add `-buildvcs=false`
to hide this difference. A later scenario must bind its actual HEAD/status again;
new public results can change `vcs.modified` even though the launcher permits them.

Run a small passive file observer in the launching tool process, with no model
calls, while the unchanged launcher runs. Observe only the newly allocated
external task temp directory. Expected paths from the inspected Go1.26.6 testing
implementation and RK1 call order are:

```text
<task-temp>/TestRK1RealProvider<random>/001/multica
<task-temp>/TestRK1RealProvider<random>/<NNN>/host-work/<session-key>/runs/<run-id>/bin/multica
```

The first path is the actual nested build output; `001` is the first TempDir in
`TestRK1RealProvider`. The second is the exact copy installed by
`HostExecutor.prepareCLI` and placed first on the provider PATH. Discover the
test-root suffix rather than inventing it. Use the saved launch record's `cwd`
(`.../host-work/<session-key>/work`) and run ID to bind the corresponding
`../runs/<run-id>/bin/multica` observation.

Minimal observer contract: poll only these glob patterns while the launcher PID
is alive; for each candidate open a regular non-symlink file owned by this user,
check inode/size/mtime before and after reading, copy the bytes once to the private
artifact directory, and record path/inode/size/SHA-256/timestamp. Retry an unstable
read, never execute the file, never alter or delay the test's files/processes.
Compare the observed nested binary and prepared per-run copy to the upfront
binary. Go's TempDir cleanup removes both paths at test end; a capture after
process exit is too late. Start the observer before the launcher. No compiler
shim, wrapper executable, watcher injection into Provider env, or framework edit
is needed.

An absent or mismatched capture is an evidence gap: retain the actual trial and
stop later scenarios for review; do not rerun the sample to obtain a nicer hash.
Passive capture cannot promise a pre-Provider interception gate, and must not be
described as one. The prelaunch source/tool/upfront-build contract and observed
actual output comparison are distinct evidence facts.

## Unmodified entry and stopping contract

After Root has committed the enabled registration, verified all prior records,
performed the owned DB/build preparation and received the model-window agreement,
use the prepared structured environment for both invocations:

```text
python3 <execution-checkout>/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py RK1-E0a 1 --check-only
python3 <execution-checkout>/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/run_rk1.py RK1-E0a 1
```

Run the second only after the first succeeds and the window still applies.
Check-only does not execute Go, Provider, version probing or DB calls, but requires
`runnable=true` and writes `.omx/private/rk1/source-manifest-check.json`.
Its success does not reserve a window or prove the database is reachable.

The launcher selects only `^TestRK1RealProvider$`, E0a trial1, with its existing
12-minute outer Go timeout. The existing Provider timeout is five minutes; the
version probe is the harness's existing 15-second probe. Do not add health/model
calls or extend any timeout. E0a must retain exactly one measured launch, correct
context exposure, negotiated handoff/recording, matching actual stdin binding,
checkpoint/seal/outcome/termination and unchanged scoring. Preserve requested and
observed model/version separately. Capture actual launch exit, public/private
accounting and stop state; no trial/result replacement or same-label overwrite.

First incomplete/environment/framework result or canary blocking review stops
all later scenarios. While the real model runs, the coordinator and helpers must
not issue coding/review model requests on the shared gateway. A passive local
observer and process/log inspection require no model request. Release the model
window explicitly before resuming agent reasoning. Public push, external CI,
deployment and publication remain outside this local test allowance.

## Source references and outstanding entry facts

Source inspected: `run_rk1.py` (`inputs_dirty`, `operator_facts`, `main`);
`server/cmd/server/rk1_entry_test.go:223` and `:265`;
`rk1_fixture_test.go:49`; `rk1_provider_test.go:115`;
`rk1_run_test.go:81`; `host_preparation.go:131` and `:296`;
`server/pkg/agent/claude.go:995`; pinned Go
`src/testing/testing.go:1431` and `src/cmd/go/internal/vcs/vcs.go:232`.

Remaining before E0a: final reviewed/committed registration in the execution copy;
explicit model-window acknowledgements; fresh owned migrated paid DB allocation;
final environment/operator/executable equality; unchanged whole-tree VCS state;
upfront CLI build and observer setup; successful launcher check-only. None of
these pending facts is inferred from a deterministic pass or failed message send.
