# S4 runtime, preparation and protected-summary review

Date: 2026-10-21. Reviewer: `check_s4_runtime`.
Active task: `10-20-collaboration-gap-recovery`.
Source HEAD: `8ecb98883504443a6ec1a5fb4a4c6295a985ae04` plus the shared task changes.
The coordinator's baseline at `/tmp/multica-gap-recovery-20261021-h6phxwo9/manifest.json` and other agents' changes were preserved.

Reviewed the delivered preparation protocol, migration 737, registration and
claim/permit/attachment/observation services, late-file refusal settlement,
daemon registration/client/bundle/preparation/Git/controller/Runner paths and
the pure Claude codec. The coordinator subsequently added the completed human
summary projection, cursor/privacy checks, core schemas/query and shared view
to this review. No SQL generation, router, API barrel, locale, rollout or release
mutation was made here.

## Findings (fixed)

### F1 — Repository configuration could become implicit Claude configuration

- File: `server/pkg/agent/claude_codec.go:29`.
- Issue: the fresh collaboration adapter reused ordinary Claude arguments without
  disabling filesystem setting sources. A selected Git commit can legitimately
  contain `.claude/settings.json`, `.claude/settings.local.json`, skills,
  commands or plugin enablement. Fresh HOME alone does not disable the CLI's
  project/local sources. Those business files could therefore influence the
  provider's configuration or hooks outside the frozen invocation.
- Fix: the collaboration-only argument builder now always includes exact
  `--setting-sources=`, `--disable-slash-commands`, and
  `--settings {"disableAllHooks":true}`. Existing strict empty MCP, exact
  instructions/model/effort and fresh HOME/config directory remain in the same
  invocation. The ordinary `buildClaudeArgs` implementation was not changed.
  Project files remain available as task data. There is no option-dropping or
  ordinary-provider retry when an explicit executable rejects these flags.
- Authorization: the coordinator approved this exact adapter change after
  receiving the upstream semantics and proposed options.
- Regression: `TestClaudeFreshStreamArgumentsIsolateRepositoryConfiguration`
  failed for all three missing controls before the fix. The real Linux
  `TestLinuxPreparedGitKeepsClaudeConfigurationAsData` also failed before the
  fix after successful Git/workspace preparation, while preserving the original
  exited proof and producing no provider outcome. Both now pass.
- The native regression commits settings/local settings, MCP, skill and command
  files through the explicit Git fixture, checks their unchanged bytes inside
  the prepared workspace, and verifies the actual fake-provider argv and
  explicit account environment. It verifies native wiring; it does not claim
  that an authenticated Claude release was executed.

Reviewer source changes:

| File | Change |
| --- | --- |
| `server/pkg/agent/claude_codec.go` | Mandatory collaboration-only isolation options |
| `server/pkg/agent/claude_codec_test.go` | Canonical argv regression |
| `server/internal/daemon/collaborationrun/preparation_linux_test.go` | Native fake-provider argv and retained-data assertions |
| `server/internal/daemon/collaborationrun/preparation_git_linux_test.go` | Explicit committed configuration fixture and native regression |

No refactor or new dependency was needed.

### Upstream evidence

Read public source only; no provider lookup, executable, account or paid call.

- [Anthropic SDK types](https://raw.githubusercontent.com/anthropics/claude-agent-sdk-python/main/src/claude_agent_sdk/types.py),
  observed lines 2653 onward: omitted `setting_sources` loads user/project/local
  sources by CLI default; `[]` disables filesystem settings. Its skills contract
  also states that the default does not mean skills are off.
  SHA-256 `c06ccef3487b2f5ad81000a72a772b823760db1c32e85693eea57c8b16a93aa2`.
- [Anthropic SDK subprocess transport](https://raw.githubusercontent.com/anthropics/claude-agent-sdk-python/main/src/claude_agent_sdk/_internal/transport/subprocess_cli.py),
  observed lines 729–730: the empty selection is encoded as `--setting-sources=`.
  SHA-256 `23c0b35acfcff5426255f1f17433e66fc7f8f6d5b8f339e93e085233c569e11c`.
- [Claude Code changelog](https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md)
  documents `--disable-slash-commands`, `disableAllHooks`, and earlier fixes for
  project skills bypassing setting-source restrictions. This supports using
  the explicit controls together rather than relying on HOME or setting sources
  alone. SHA-256 `901f2b926ddc741a54ab1be08c43da7f5c40583599ba04816652fc8fb55ac1a7`.

The exact reference excerpts and fetched transport/changelog are retained in
the evidence directory below. The documentation site returned HTTP 403, so the
official public repositories supplied the evidence. A real selected Claude
version must still be verified to honor every required option; this review does
not broaden supported executable/version claims.

## Findings (not fixed)

No unresolved concrete defect was found in the other reviewed S4 paths.

The coordinator retains these integration and acceptance items:

- Sync the server runtime/preparation and recorded-run specs plus built-in
  guidance with the new delivered preparation contract and F1's mandatory
  options. The older static-only/file-pending wording is no longer a complete
  description of this increment. These shared documents were reserved to the
  coordinator; the exact behavior and source evidence are recorded here for
  that update.
- Real provider/version behavior, authenticated account/quota smoke, the full
  provider × OS matrix, Windows/macOS containment, packaged Desktop and new
  preparation-summary Web/Electron rendering remain outside this review's
  executed evidence. Unsupported platforms continue to refuse. Neither S4's
  full acceptance nor the parent 97-item acceptance is marked complete here.

## Reviewed boundaries

- First negotiated claim freezes prompt and preparation together. Replays use
  the original files, invocation, run identity and versions. Legacy claims do
  not gain preparation fields or silently receive file authority.
- Runtime attachment download uses the exact current `mcb_`, retained manifest,
  version and row; storage runs outside locks, then the same authority is checked
  before sending bytes. Client status/header/length/encoding/redirect checks
  refuse substituted or partial delivery.
- Late-file refusal settles only the exact untouched never-permitted run after
  complete retained-group/delegation and start-evidence checks. Issued claims,
  live siblings and unknown work preserve their barriers. Shared post-proof
  effects handle member wakeups and group settlement; required-write/unknown
  commit failures do not become confirmed refusal.
- Preparation before burn performs Go file/HTTP work. Git and provider business
  subprocesses run after burn in the supervised namespace. Fixed Git validation,
  nested namespace reaping, object-descriptor cleanup, fresh tmpfs materialization,
  explicit bundle/loader/library and account inputs were inspected. The explicit
  `/dev/urandom` capability is not described as a read-only descriptor; Git-only
  device access is removed before provider execution.
- Stop during Git produces the original observed exited proof; supervisor death
  retains unknown state. Missing/denied provider recording never supplies a
  business outcome or blocks the narrow proof path.
- The human summary is an allowlisted projection with no invocation, digest,
  destination, capability, installation credential or raw event. It validates
  retained manifest/resource/input/attempt correlation without consulting a
  mutable live runtime. Latest stage observations use the existing cursor
  watermark; unknown/fifth stages and mismatched observations refuse. Core
  rejects conflicting summaries within a page chain. Current access loss clears
  the protected summary along with the record pages. No additional fix was
  needed in this slice.

## Verification

All final commands exited 0. Evidence directory:
`/tmp/multica-gap-recovery-20261021-h6phxwo9/review-s4/`.
`verification-summary.json` contains exact test counts, skipped identities,
initial failures and binary provenance.

| Scope | Final result | Evidence |
| --- | --- | --- |
| Backend race, serial packages on `review_s4` | Service 81 top-level + 100 subtests; handler 7; migrations 3; protocol 3 + 12 subtests. No failures/skips. | `backend-race-serial.jsonl` |
| Prepared-summary/record service race | 10 top-level + 18 subtests; no failures/skips. Overlaps the broader service run. | `records-race.jsonl` |
| Guarded portable race after F1 | collaborationrun 16 + 84; daemon 21 + 14; agent 78 + 50. No failures; two existing platform/root-specific skips listed below. | `daemon-settings-green.jsonl` |
| Actual Linux final suite | 15 top-level tests passed; no failures/skips. | `native-settings-green.log` |
| Core recorded-run/schema/query tests | 103 tests / 3 files passed. | `core-records-tests.log` |
| Views disclosure/preparation/navigation tests | 10 tests / 3 files passed. | `views-records-tests.log` |
| Scoped core/views ESLint | Pass. | `core-records-lint.log`, `views-records-lint.log` |
| Core/views TypeScript checks | Pass. | `core-typecheck.log`, `views-typecheck.log` |
| Go vet, host and Linux arm64 | Pass; Go compilation also passed through the race tests and native binary build. | `go-vet-final.log`, `linux-vet-final.log` |
| Reviewer files gofmt and worktree diff whitespace | Pass, empty output. | Recorded tool execution |

The portable skips are `TestCollaborationBatchPollerSlotsAndRunnerLinux` on
darwin and `TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` for
the non-root host. They are not counted as native or root-mode coverage. The
separate 15-test Linux suite is the native evidence for the runtime changes.

Go commands ran from `server/`, with Go 1.26.6 and the coordinator's private
database wrapper. The database URL was not printed or copied into repository
files.

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py review_s4 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -race -p 1 \
  ./internal/service ./internal/handler ./internal/migrations ./pkg/protocol \
  -run '^TestCollaboration(Runtime|Managed|Termination|RemovalOfNonRoot|GroupDepend|Squad|Batch|Preparation)' -count=1 -json

python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py review_s4 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service \
  -run '^TestCollaboration(PreparationRecords|RunRecords|RuntimePreparationObservationKeepsInstalledRecordsReadable)' -count=1 -json

GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -race -p 1 -parallel 1 ./internal/daemon/collaborationrun ./internal/daemon ./pkg/agent \
  -run 'Collaboration|Preparation|ProviderBundle|ClientPreparation|Runner|Claude|StreamJSON' -count=1 -json

GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon/collaborationrun ./internal/daemon \
  ./pkg/agent ./internal/service ./internal/handler ./internal/migrations ./pkg/protocol
GOTOOLCHAIN=go1.26.6 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go vet ./internal/daemon/collaborationrun ./pkg/agent

GOTOOLCHAIN=go1.26.6 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go test -c ./internal/daemon/collaborationrun -o /tmp/multica-s4-check-settings-green.test
docker exec -e MULTICA_REQUIRE_COLLAB_NAMESPACE=1 \
  -e MULTICA_COLLAB_TEST_GIT_CONFIG=/opt/multica-git-fixture/git-tools-with-entropy.json \
  multica-gap-recovery-native-full /collaborationrun-s4-check-settings-green.test \
  -test.run '^TestLinux' -test.v -test.count=1
```

Frontend commands ran from the repository root:

```bash
pnpm --filter @multica/core test run-preparation-schema.test.ts run-records-schema.test.ts run-records-queries.test.ts
pnpm --filter @multica/views test preparation-summary.test.tsx execution-panel.test.tsx run-navigation.test.tsx
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core exec eslint collaboration/run-records-schema.ts collaboration/run-records-queries.ts collaboration/run-preparation-schema.test.ts collaboration/run-records-queries.test.ts
pnpm --filter @multica/views exec eslint collaboration/run-records.tsx collaboration/preparation-summary.tsx collaboration/preparation-summary.test.tsx
```

Native environment: the existing task container `multica-gap-recovery-native-full`,
Linux arm64, explicit SYS_ADMIN/seccomp/proc setup, no network or host mounts,
executable private `/tmp`, and the original explicit Git/loader/library fixture.
Its one-hour sleep had expired; the reviewer inspected and restarted that same
container. No original binary or fixture was overwritten. The new final artifact
is `/tmp/multica-s4-check-settings-green.test`, copied as
`/collaborationrun-s4-check-settings-green.test`; SHA-256 on host and container:

```text
372ea64d61135ce4461bd941005387d850928a7413882aaa068f366588e7a864
```

Initial results remain retained and are not added to final counts:

- `backend-race.jsonl`: running service and handler packages concurrently on the
  same review database produced one handler `collaboration_execution_unavailable`
  refusal while fixtures changed the shared gate. The identical selector with
  `-p 1` passed; no product assertion or gate was weakened.
- `daemon-race.jsonl`: an existing parallel Claude context-exhaustion fixture
  reached its five-second timeout. Its isolated/serial run and the final complete
  selector with `-parallel 1` passed. No timeout or product logic was changed.
- `native-original.log`: the stopped container prevented execution. After restart,
  the original supplied binary passed 14/14 tests (`native-original-restarted.log`).
- `claude-settings-red.log` and `native-settings-red.log`: expected failing
  regressions before F1; final green results use the same assertions.

No real-provider/account/paid smoke, full `make check`, new browser/Electron
session, release, commit or rollout change was performed by this reviewer.
