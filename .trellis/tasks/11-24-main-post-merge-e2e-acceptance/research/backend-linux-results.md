# Linux backend supplemental acceptance

Final status, 2026-11-25: **3 targeted assertions PASS; 7 namespace cases BLOCKED**.
Candidate: `103e2bc329a9fb845d9f45c20b2a9bb3ebb54625`.

The parent independently accepted the prepared manifest and wrote
`parent-clearance.json` before execution. All four approved commands completed.
The canonical runs produced four named PASS terminals (three top-level tests
and the selected root subtest), zero FAIL, zero SKIP and zero missing names.
Both namespace prerequisites returned exit 2 with `operation not permitted`.
Final source/dependency/tool/binary/argv bindings matched, and the owned-container
audit was empty. No database or service was started.

The parent subsequently independently checked all five stdout/stderr receipts
(four executions plus the final container audit), confirmed the two process
passes, root preflight pass and both namespace refusals, and confirmed zero
remaining owned containers. Final source identity remains the candidate above;
the runner's post-execution source/dependency/tool/binary/argv verification passed.

This lane supplements the original macOS skips without changing their records.
It does not replace the retained backend race failure, public-skills HTTP 403,
Windows/pg_bigm gaps, or placeholder-test skips.

## Owned files and scope

- Private runner: `.omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend-linux-acceptance/`.
- Evidence: `artifacts/qa/main-post-merge-e2e-20261124-080504/backend-linux-acceptance/`.
- Host compiler HOME/scratch: `/Users/artisan/.multica/qa/main-post-merge-e2e-20261124-080504/backend-linux-acceptance/`.
- New authored files: `run.py`, `container.sh`, `namespace_probe.go`, `offline_checks.py`, and this report. Generated source/dependency manifests, compile receipts and binaries remain in the new owned directories.

The smallest gap is three canonical assertions skipped on macOS: Linux process
reaping, stdin pipe inspection, and a root-only pure preflight assertion. The
runner invokes the original test binary with exact anchored names and the
existing canonical agent CLI guard. It introduces no general acceptance
framework, product edits, dependencies, assertion changes, retry changes or
timeout changes. The ordinary entry's 40-minute package timeout and parallel=2
are retained. This is CGO-disabled normal-mode evidence only, never race evidence.

## Preparation and offline evidence

Both cross-compilations succeeded with installed `go1.27.0 darwin/arm64`,
`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`,
`GOENV=off GOWORK=off GOFLAGS=-mod=readonly`, explicit cache paths and private
compiler HOME/scratch. No module installation or download was requested.
`binary-type.stdout` proves both outputs are statically linked Linux arm64 ELF.
`binary-buildinfo.stdout` records the toolchain, build settings and linked modules.

`source-before.json` binds 5,288 server/guard paths with SHA256 and mode.
`dependency-bytes.json` binds 2,064 actual dependency files, including standard
library and cached module bytes. Preparation checked both snapshots again after
compilation. The manifest also binds Go/compiler/linker, Docker, the guard,
runner and binaries. `git diff --stat` remained empty.

`offline-checks.json`: **9 passed**. Python/shell syntax, valid prepared bindings,
altered binary binding refusal, altered command refusal, missing-clearance
refusal before execution, and the canonical guard's success/failure/ambient-CLI
refusal behavior all passed. The CLI refusal intentionally returned exit 1;
its sentinel executable handled the name `codex`, with no actual CLI access.
No canonical test body, container, database or service ran during preparation.

Key SHA256 bindings:

| Input | SHA256 |
| --- | --- |
| `prepared.json` | `2f9a43d2c5418580380265c0d7677f342e60efb4d76e75f1ca81bd8efe84801b` |
| `run.py` | `edad82c7cf92f30a134b3ad084d64c72c72dd9b0379098a8f82d38f7480327ed` |
| `input/agent.test` | `68c61f68aa3039d8e13b472584778b0c0431469a4f4735868d9a6786648d4aa1` |
| `input/namespace-probe` | `4854f26299b8e1d032196ba1c845e3bee6903e1ddb42c4bffb001b0efaf1e39f` |

## Exact review and execution contract

`prepared.json.runs` contains all exact argv arrays, in execution order:

1. `process-r01`: UID1000; exact `TestCodexExecuteRetriesAfterSignaledProcessIsReaped` and `TestPiExecuteAttachesStdinPipe`.
2. `root-preflight-r01`: UID0; only `TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` plus its parent identity. The test calls the pure preflight function; no sudo command or Claude process is launched.
3. `namespace-user-r01`: UID1000 prerequisite probe.
4. `namespace-root-r01`: UID0 prerequisite probe, still with every capability dropped.

Every container selects only cached image
`sha256:db164dd5195d37ded1e6732eec4cc6b87decac66825be15fae9c7b87b2bac6c4`,
with `--pull=never --network=none --read-only --cap-drop=ALL
--security-opt=no-new-privileges:true`, default Docker seccomp/AppArmor policy,
private executable tmpfs HOME/tmp, explicit UID, a read-only input mount,
`--rm`, and a task ownership label. Environment is cleared using `env -i`.
There is no repository, host account, Docker socket, database credential or
host-home mount. Shell prerequisites and effective UID/capability/seccomp facts
are checked/recorded before the canonical CLI guard launches the selected binary.

The parent independently reviewed `prepared.json` and its bound scripts and
wrote `parent-clearance.json` in the evidence directory with these required fields:

```json
{
  "verdict": "accepted",
  "blocking_findings_open": 0,
  "prepared_sha256": "2f9a43d2c5418580380265c0d7677f342e60efb4d76e75f1ca81bd8efe84801b"
}
```

The accepted execution used this command from the repository root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 .omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend-linux-acceptance/run.py execute
```

The runner refuses source/tool/binary/command drift, records raw stdout/stderr
and exact exits, converts canonical test output through pinned Go `test2json`,
requires the exact named run/terminal set with zero failures/skips, checks final
bindings, and requires zero containers with this lane's ownership label.
Clearance was created by the parent, not the implementer. Its SHA256 is
`a420bfbad0a09211639264919936c43df691c5ac4ff4bffcf8ac128e5d224100`.

## Seven namespace cases

The probe uses the same `CLONE_NEWUSER|CLONE_NEWPID|CLONE_NEWNS|CLONE_NEWIPC|
CLONE_NEWUTS`, UID/GID maps and disabled setgroups mapping as
`server/internal/daemon/collaborationrun/controller_linux.go` to start `/bin/true`.
It is a prerequisite diagnostic, not any canonical test. A kernel/policy refusal
will establish BLOCKED for this admitted configuration; no privileged or
unconfined retry is authorized. A successful clone alone would still leave
mount/pivot_root and full containment unproved and require further review.

Both probes actually failed at `fork/exec /bin/true: operation not permitted`,
including UID0 with no capabilities. The recorded process status in both cases
was `CapEff=0000000000000000`, `NoNewPrivs=1`, `Seccomp=2`. This proves the
clone prerequisite is unavailable under this approved policy; it does not
attribute refusal to a particular seccomp rule or prove every possible Linux
host lacks namespaces. No security policy was relaxed or retried.

The following are **BLOCKED** by that prerequisite, with their canonical bodies
NOT_RUN:

- `cmd/server.TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux`
- `cmd/server.TestCollaborationPartialResumeRoutesLinux`
- `cmd/server.TestCollaborationSingleAgentRuntimeRoutesLinux`
- `cmd/server.TestCollaborationHeldSquadAllowsDirectAgentRoutesLinux`
- `cmd/server.TestCollaborationRuntimeProofSurvivesOwnerDisableLinux`
- `cmd/server.TestCollaborationSquadCapacityOneRoutesLinux`
- `internal/daemon.TestCollaborationBatchPollerSlotsAndRunnerLinux`

The first six require an exclusively allocated fresh database and pinned CLI;
neither was allocated or started. The seventh uses local HTTP fixtures but
still requires real namespaces. Their binaries are intentionally not compiled
because the namespace prerequisite cannot be met in the admitted environment.
All original skip identities and historical failed evidence are retained.

## Actual outcomes and cleanup

| Run / named assertion | Exit | Result |
| --- | --- | --- |
| `process-r01 / TestCodexExecuteRetriesAfterSignaledProcessIsReaped` | 0 | PASS, 0.21s |
| `process-r01 / TestPiExecuteAttachesStdinPipe` | 0 | PASS, 0.02s |
| `root-preflight-r01 / TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` | 0 | PASS, 0.00s; parent test also PASS |
| `namespace-user-r01`, UID1000 prerequisite | 2 | BLOCKED, operation not permitted |
| `namespace-root-r01`, UID0 prerequisite | 2 | BLOCKED, operation not permitted |

The Codex fixture log records its first fake process killed and reaped, a safe
second launch, completion, then successful reaping. The Pi fixture used the
explicit test-created `/tmp/.../pi`, with the original FIFO/payload assertions.
Neither run resolved an ambient provider executable. The root case ran only
the selected pure function assertion and did not launch sudo or Claude.

Evidence beneath the owned artifact directory:

- `process-r01.stdout`, `.stderr`, `.json`, and `process-r01-events.stdout`: raw canonical execution, Docker exit and converted named Go events.
- `root-preflight-r01.stdout`, `.stderr`, `.json`, and `root-preflight-r01-events.stdout`: selected root subtest and parent terminals.
- `namespace-user-r01.stdout/.stderr/.json` and `namespace-root-r01.stdout/.stderr/.json`: exact refusals, effective security facts and command exits.
- `result.json`: final matching bindings, clearance hash, accepted canonical identities and zero remaining owned containers.
- `containers-after.stdout` (empty) and `.json` (exit 0): post-run label-scoped audit. A second read-only audit was also empty; no recovery kill/remove was needed.
- `outcome-inventory.json`: supplemental disposition for all nine original Linux identities plus the root subtest.

The `test2json` timestamp and package elapsed fields describe post-run parsing;
they are not measured container execution duration. Per-test elapsed values
above come from the actual test frames. This remains focused normal-mode
acceptance, not full Linux, race, router/daemon namespace or paid-provider coverage.
Preparation checks and historical failures/skips remain unchanged. Only this
report and new task-private evidence were added/updated; tracked product source
and canonical assertions remain unchanged.
