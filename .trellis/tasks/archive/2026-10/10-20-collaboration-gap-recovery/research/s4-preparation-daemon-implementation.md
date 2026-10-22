# S4 daemon / controller / provider implementation

Date: 2026-10-21. Active task: `10-20-collaboration-gap-recovery`.

This report covers the daemon lane dispatched by the coordinator. The server,
SQL, public protocol, routes, UI and shared specifications have separate owners.
The existing S1 recovery cursor, retained deadline, nonce ledger, capacity and
proof changes were preserved; they are not new work attributed to this lane.
No commit or archive operation was performed.

## Implemented behavior

- Local `CollaborationRunnerConfig.bundle` selects an explicit native file
  bundle. It requires `expected_runtime_config_epoch` and excludes simultaneous
  legacy `executable`, `cli_executable` or `arguments` overrides. The existing
  no-file static path remains available. Inline configuration retains 64 KiB.
- Before candidate admission the poller registers the complete nonsecret
  capability summary. A current identical registration is a no-op. A lost POST
  is reconciled by GET and exact deployment/installation/binding, full summary
  and current recorded epoch; the daemon never adopts a later epoch to overwrite
  changed configuration. Only a verified configured bundle adds preparation v1
  and its digest to Claim.
- Claim and preparation GET validate the original run identity, ordered command
  IDs, exact resource tuple, invocation, attachment membership, versions and
  manifest digest. The current server-owned invocation is the authority for
  instructions, model and thinking level. The Claude adapter uses the shared
  invocation validator and canonical thinking catalog; no local model override,
  resume pointer, ordinary executor or fallback is introduced.
- Pure Go preparation happens after the original permit and before nonce burn:
  explicit regular-file snapshots, digest/ELF checks, selected API-key read,
  mcb-only exact attachment downloads and private, fsynced local metadata.
  Downloads reject redirects, partial/encoded bodies, wrong run/file/version,
  short/long content and ordinary/termination credentials. Display filenames
  never determine a destination.
- Source descriptors are opened component by component with no-follow and
  close-on-exec. Shared-writable files, unsafe parents, symlinks, devices as
  ordinary bundle files, missing dependencies, unsupported executable formats
  and wrong architecture refuse. ELF interpreters and DT_NEEDED libraries must
  exist in the explicit bundle. Non-ELF data is limited to declared resolver,
  hosts, NSS and CA-certificate files; arbitrary home/config/session data cannot
  bypass the account allowlist as readonly bundle data.
- The Linux init mounts fresh work/home/tmp storage before copying attachments
  into `/work`. The provider bundle is readonly; HOME is fresh and only the
  explicit Claude API-key source is forwarded. Host environment, provider homes,
  ordinary platform credentials and termination credentials are not inherited.
- Every business helper and provider process starts after durable nonce burn.
  Namespace readiness is returned before long Git preparation, so the existing
  supervisor can record and stop the whole scope. A pre-burn error can submit
  the original nonlaunch proof. A post-burn unobserved start/crash stays unknown
  and retains capacity. Observed namespace termination permits only the original
  exited proof, even when the provider never started.
- Fixed Git preparation uses the explicitly selected bundle and full SHA-1 or
  SHA-256 commit in a nested PID/mount/network namespace. Only fixed Git argv
  runs there: fresh bare init, bundle verification/import, commit-type check,
  full tree inventory, and archive. There is no working-tree/config/hook copy,
  remote fetch, account, platform token or inherited Git configuration. Helpers
  such as index-pack are explicit files adjacent to the selected Git executable.
- Go tar materialization checks the inventory and each exact Git blob hash,
  paths, sizes, entry count and framing budget. It rejects links/devices,
  submodules, LFS pointers, duplicate/conflicting paths, export-ignore omission,
  substituted content and attachment-directory collisions. The only permitted
  global PAX header is Git's comment matching the admitted full commit.
- The nested init is reaped before its root and object descriptors are removed.
  Git-only devices are removed too. The supervisor then rechecks current mcb
  admission and the same manifest, drops privileges and starts exactly one
  provider. No preparation subprocess or object store survives into that stage.
- `static_json_v1` and `claude_stream_json_v1` produce independent provider
  results. Claude reuses the canonical argument/input/control/assistant/terminal
  leaves and keeps stdin open for control responses. Explicit usage is parsed
  without float64, including integers above 2^53, zero, missing fields and exact
  decimal/scientific cost. Unsupported or over-limit telemetry is partial or
  unavailable. Process exit alone produces no provider result.
- Preparation observations have deterministic original attempt/stage event IDs
  and remain distinct from provider outcomes and termination proof. Failed
  recording or lost business authority does not block the narrow proof path.

## Explicit entropy-device semantics

Both bundle and Git configuration accept optional `devices: ["/dev/urandom"]`.
No other device or duplicate entry is accepted. Device selection is included in
the local bundle digest. The source is opened without symlink following, its
actual character-device identity must be major 1/minor 9, and the checked FD is
bound before it is closed. The bind protects source inode metadata from changes.

This is a device capability, **not a readonly descriptor guarantee**. The native
test verifies successful reads, denied chmod, and successful opening for writes;
readonly mount flags do not prohibit character-device writes. No test writes
entropy contents or prints random bytes. A provider sees the device only if its
own bundle declares it; a Git-only declaration is removed before provider start.

## Changed files

| Area | Files |
| --- | --- |
| Daemon configuration and poller | `server/internal/daemon/collaboration.go`, `collaboration_preparation_test.go` |
| Runtime HTTP boundary | `server/internal/daemon/collaborationrun/client.go`, `client_preparation.go`, `client_preparation_test.go` |
| Explicit bundle and safe snapshots | `server/internal/daemon/collaborationrun/provider_bundle.go`, `provider_bundle_unix.go`, `provider_bundle_unsupported.go`, `provider_bundle_test.go` |
| Preparation and fixed Git | `server/internal/daemon/collaborationrun/preparation.go`, `preparation_git.go`, `preparation_linux.go`, `preparation_git_test.go`, `preparation_git_linux_test.go`, `preparation_linux_test.go`, `preparation_recovery_linux_test.go` |
| Supervisor and runner wiring | `server/internal/daemon/collaborationrun/controller.go`, `controller_linux.go`, `controller_test.go`, `controller_other_test.go`, `runner.go`, `runner_preparation_test.go` |
| Canonical pure Claude leaves and codec | `server/pkg/agent/claude.go`, `claude_codec.go`, `claude_codec_test.go` |

The Claude extraction is narrow: the ordinary backend still owns its executor
and uses the same assistant/control/terminal behavior. Collaboration never calls
`agent.New`, ordinary `Execute`, PATH discovery, or ordinary execenv helpers.

## Verification

Source HEAD: `8ecb98883504443a6ec1a5fb4a4c6295a985ae04` with the task's uncommitted
source changes. Native test binary:

```text
/tmp/multica-s4-collaborationrun-final.test
sha256 989cc83ceea5a6476351708fa0bf36be7aabb40559eeb72af13a1087d0150dbe
```

Commands below were executed, with exit 0 unless explicitly recorded as RED.
Go commands run from `server/`.

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GOTOOLCHAIN=go1.26.6 \
  go test -c ./internal/daemon/collaborationrun \
  -o /tmp/multica-s4-collaborationrun-final.test

docker exec -e MULTICA_REQUIRE_COLLAB_NAMESPACE=1 \
  -e MULTICA_COLLAB_TEST_GIT_CONFIG=/opt/multica-git-fixture/git-tools-with-entropy.json \
  multica-gap-recovery-native-full /collaborationrun-s4-final.test \
  -test.run '^TestLinux' -test.v -test.count=1

GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race ./internal/daemon/collaborationrun ./internal/daemon ./pkg/agent \
  -run 'Collaboration|Preparation|ProviderBundle|ClientPreparation|Runner|Claude|StreamJSON' \
  -count=1

GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/collaborationrun ./internal/daemon ./pkg/agent
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 GOTOOLCHAIN=go1.26.6 \
  go vet ./internal/daemon/collaborationrun ./pkg/agent
```

Native environment: task-owned `multica-gap-recovery-native-full`,
`node:22-bookworm`, Linux arm64 / Docker Desktop linuxkit 6.12.68. It has no host
mounts or network, explicit SYS_ADMIN and seccomp/systempaths adjustments for
the actual user/PID/mount probes, and an executable private `/tmp` tmpfs. These
are test-container conditions; Docker is not a runtime product dependency.
Selected Git is the container's explicit Git 2.39.5 fixture with regular-file
snapshots of its ELF loader and DT_NEEDED libraries. Each test creates its own
repository, commits, bundle and uncommitted sentinels.

Final native result: **14 top-level tests passed, 0 failed, 0 skipped**. Exact
test identities are in `native-s4-final.log`:

- Existing namespace detached/double-fork/early-stdio, privilege/file boundaries,
  Runner outcome/Stop/revoked business/proof and supervisor crash regressions.
- `TestLinuxPreparedGitExportsOnlyFixedCommitWithExplicitDynamicTools` — SHA-1
  and SHA-256, later branch commit and uncommitted/untracked source changes.
- `TestLinuxPreparedGitRefusesUnsupportedCommittedContent` — missing commit,
  symlink, submodule, export-ignore and LFS.
- `TestLinuxProviderBundleRejectsMissingNativeDependenciesAndWrongArchitecture`.
- `TestLinuxPreparedRunMaterializesFilesAndUsesFreshExplicitAccount` — static
  JSON and Claude stream, including bidirectional control and exact usage.
- `TestLinuxPreparedRunDistinguishesPreburnAndContainedFailure`.
- `TestLinuxPreparedGitReapsDetachedObjectReaderBeforeProvider`.
- `TestLinuxPreparedEntropyRequiresExplicitProviderDevice`.
- `TestLinuxPreparationProbeNeverReadsAccountOrGitInputs`.
- `TestLinuxPreparedStopDuringGitUsesExitedProofWithoutProvider`.
- `TestLinuxPreparedSupervisorCrashDuringGitRetainsUnknown`.

Portable guarded race run: all three packages passed. It also covers exact
capability recovery, malformed/missing invocation, strict downloads, config
exclusivity/epoch, registration before candidate admission, source symlink and
mutation checks, pre-burn refusal, unknown starts and original ledger recovery.
Both vet targets, scoped `git diff --check`, and gofmt checks passed. The static
search found no diagnostic traces, ambient discovery/environment inheritance or
ordinary executor fallback in the new product files.

Evidence directory:
`/tmp/multica-gap-recovery-20261021-h6phxwo9/linux-v1/`.
Final logs are `native-s4-final.log` and `portable-s4-race.log`.

## RED cases and diagnosis retained

Initial failure-first client/bundle/codec tests failed because their new methods
did not exist, then passed after implementation. A portable Git PAX regression
was also observed failing before its fix.

The first actual Linux run failed the basic probe for old and new tests. A
limited stage/errno diagnostic identified `mount_proc_errno_1`. The coordinator
corrected the task container's masked parent `/proc` and executable tmpfs;
product containment requirements were unchanged. Initial logs remain
`native-tests.log`, `native-tests-apparmor.log` and `probe-v2-root.log`.

The first actual Git positive cases failed; negative refusals were not counted
as correctness evidence while the positive case remained broken. Git's global
PAX comment was first reproduced and added to the portable regression. A
temporary Go overlay diagnostic, outside product sources, then located the
remaining failure in index-pack's `/dev/urandom` dependency. Both required fixes
were applied, and positive plus negative cases were rerun together. Relevant
logs are `native-v3-real-git.log`, `native-v4-real-git.log`,
`native-git-diagnostic.log` and the passing `native-v5-real-git.log`. The final
binary has none of the temporary diagnostic overlay's stderr traces.

## Remaining boundaries / coordinator follow-up

- This lane has real Linux containment, actual Git and fake-provider protocol
  evidence. No authenticated Claude executable/account, paid model smoke,
  general provider×OS matrix, native Windows/macOS controller or packaged
  Desktop acceptance was exercised. Other adapters and scripts remain
  unsupported; a real selected binary/dependency/account bundle still requires
  its own evidence. The existing real-agent smoke dual gate remains intact.
- Account support is deliberately the selected Claude API-key environment
  source only. OAuth files, shared refresh, provider histories, session reuse,
  arbitrary custom arguments/config/MCP/skills and ordinary execution fallback
  are outside this adapter. Unsupported agent configuration is rejected by the
  separately owned server invocation boundary.
- Limits remain explicit and bounded: 32 input files, 100 MiB per attachment,
  256 MiB total input, 512 MiB workspace, 256 MiB Git expansion, 65536 entries and
  600 seconds preparation. The local runtime-file bundle has its own 1024-file,
  256 MiB-per-file, 512 MiB aggregate bound. Files larger than inline config
  capacity do not trigger directory discovery or installation.
- Native Runner HTTP tests use httptest; server/storage authorization and SQL
  have separate test evidence. Complete product integration, human preparation
  summaries, rollout and the 97-row acceptance ledger remain the coordinator's
  work. The parent task and AC-I06 are not marked complete by this report.
- Independent Trellis check and the main session's spec/built-in workflow source
  map updates should use this final behavior and the explicit entropy semantics.
  Creation/execution rollout gates were not enabled.
