# Collaboration runtime preparation

> 2026-10-26: ordinary host execution has a separate [implementation contract](./collaboration-host-execution.md) and default-closed admission. The Linux namespace guarantees below apply only to the retained legacy branch; do not apply them to host execution. Platform/account acceptance remains tracked by the active runtime task.

Apply to preparation negotiation, explicit provider bundles, attachments/Git,
the contained Runner and their source-owned human summaries. Runtime/proof
protocol 2 is unchanged; preparation and provider bundles negotiate version 1.

## Admission and immutable identity

`agent_runtime.collaboration_capabilities` stores the full registration and its
runtime configuration epoch. Normal runtime metadata refresh cannot erase it.
Registration uses the expected epoch and current managed installation/binding;
an exact current registration is a no-op, and GET recovers an uncertain write.
The daemon registers before listing work. A changed configuration requires a
new matching registration, not an inferred capability from a provider name.

First negotiated claim freezes `prompt_snapshot` and `preparation_snapshot` in
one transaction. The manifest binds the complete run identity, actual command
IDs, resource/configuration epochs, selected attachment versions, explicit Git
commit, provider adapter and limits. Invocation instructions/model/thinking
come from the locked agent. Never re-read live configuration to rebuild an old
claim, replace a nonce or invent a new attempt after a lost response.

Attachment selection is one rule for promotion, claim freeze and never-launched
refusal settlement. The manifest holds every delivered file that the prepared
inputs reference. Remaining capacity is evaluated separately for each selected
runtime: file count, per-file bytes, total input bytes and workspace space after
any Git reservation. Optional issue history is considered newest first
(`delivered_at DESC`, then `id`) and may be omitted when it does not fit. An
issue's delivered files are permanent and can outnumber the bound, so only a
referenced set above the platform or selected runtime limit refuses with
`runtime_preparation_limits`; a referenced file that is no longer delivered
refuses with `runtime_preparation_unavailable`. A claimed run keeps its frozen
selection, and later deliveries never widen it. Optional history must never
make a text-only input wait; referenced files remain all-or-nothing.

The initial prepared adapters are `static_json_v1` and
`claude_stream_json_v1`. Nonempty unsupported agent overrides, MCP/Composio,
skills, service tier, autonomy or squad instructions refuse rather than being
silently ignored. Legacy no-file claims retain their existing wire contract;
they do not acquire file authority or general provider support.

File-dependent promotion must check preparation support before creating a
reservation. If files arrive after promotion, only an exact untouched,
never-permitted run may converge through the existing server nonlaunch proof
and normal post-proof effects. Check claim/prompt, attempt, business capability,
termination and event evidence. Apply this per run to members, continuations
and unread M11 work; preserve live siblings, original input sequence, pause,
holds and every genuinely unknown reservation. An old client request against a
capable runtime is not evidence of nonlaunch.

## Files, bundles and process boundary

The run-bound `mcb_` preparation/attachment APIs check the current authority,
exact identity/version and frozen manifest. Read/decrypt storage outside the
transaction, then recheck the same authority before returning bytes. Clients
reject redirects, identity/version/header mismatches and truncated/extra bytes.
Human attachment endpoints and ordinary signed URLs cannot substitute for this
business capability.

Limits remain explicit: at most 32 attachments, 100 MiB per attachment,
256 MiB total input, 512 MiB workspace, 256 MiB expanded Git data, 65,536
entries and 600 seconds of preparation. The local provider file closure has
its separate 1,024-file/512 MiB aggregate/256 MiB per-file limits. Keep both
validators and their fixtures in sync when changing a limit.

Go validation/download/staging happens before the irreversible burn. Every
business Git/helper/provider subprocess starts after burn in the supervised
scope. Materialize held files into the new tmpfs after root isolation; copying
into the old `/work` before its remount leaves an empty workspace.

`CollaborationRunnerConfig.bundle` is explicit single-runtime opt-in, together
with `expected_runtime_config_epoch`; it cannot mix with the legacy executable,
CLI executable or argument fields. `ProviderBundleConfig` lists every regular
source file, destination, hash, size and mode. Validate every path component
without symlink traversal and the ELF interpreter/library closure. No PATH
discovery, package installation, original worktree or ambient account fallback.

The first account adapter is one explicitly selected Claude
`ANTHROPIC_API_KEY` environment source. OAuth homes, sessions, refresh files and
other layouts remain unsupported. Only the business capability reaches the
provider; managed and termination credentials stay with the supervisor.

Git uses an explicit local bundle and full SHA-1/SHA-256 commit. Validate objects
and exported paths/types/blob bytes, reject unsupported symlinks, submodules,
attributes/filters and LFS content, and enforce entry/byte limits. The bounded
Git PAX global comment must name that exact commit; it cannot override paths or
sizes. The nested PID/mount/network scope and all object descriptors are closed
and reaped before provider credentials/start.

An explicit `/dev/urandom` dependency is the sole allowed device (char 1:9).
Bind a validated descriptor, not a replaceable path. Git-only access is removed
before the provider; provider access needs its own declaration. A read-only
mount does not make a character-device descriptor read-only: tests prove that
writes can still open, so never claim stronger protection.

### Bundle file snapshot fixtures

`openBundleSource(source)` supports Linux and Darwin file validation; this does
not advertise the Linux namespace controller on macOS. Every path component must
be owned by the current user or root and refuse group/other write access, except
root-owned sticky ancestors. The final source must be a regular file. Violations
return `ErrUnsupported`; do not loosen these checks for test workspaces.

Build `TMPDIR` is a lifetime/location setting, not a trusted source ancestry.
A private child below a group-writable volume still fails source validation.
`bundleTestFile` therefore creates its Linux/Darwin source beneath standard
`/tmp`, canonicalizes it, and cleans only that exact test-owned directory:

```go
temp, err := os.MkdirTemp("/tmp", "multica-bundle-test-")
// Check err; register exact-directory cleanup, then EvalSymlinks(temp).
```

Retain the valid-byte, symlink/parent-symlink, changed hash/size, nonregular-file,
shared-writable-file and shared-writable-parent matrix in `provider_bundle_test.go`.
Valid sources must snapshot on Linux/Darwin even with an unrelated unsafe build
`TMPDIR`; a shared-writable source ancestor must still refuse. Other platforms
assert `ErrUnsupported` and no output artifact, rather than claiming Unix support.
Do not replace the positive case with a skip, mutate global `TMPDIR`, or chmod a
shared volume. Recheck normal/race with the original build temp setting and keep
actual Linux execution distinct from Windows compile-only evidence.

## Claude configuration remains explicit

`ClaudeFreshStreamArguments` always supplies `--setting-sources=`,
`--disable-slash-commands`, inline `--settings {"disableAllHooks":true}`,
strict empty MCP and fresh HOME/config. Repository settings, hooks, skills and
commands remain business files; they cannot become implicit CLI configuration.
These options belong only to the collaboration adapter. Do not change the
ordinary argument builder or drop a rejected option and retry another executor.

## Records and verification

Preparation observations are facts, not launch permission, provider outcome or
process proof. The human records API projects only attachment identity/version/
name/size, fixed commit and at most four validated stage facts at the original
record cursor watermark. Do not expose the manifest, its digest (which includes
private instructions), bundle digest, invocation, credentials or local paths.
No observations means not recorded. Legacy omitted summaries remain valid.

Canonical tests are in `collaboration_runtime_preparation*_test.go`,
`collaboration_runtime_invocation_test.go`, `collaboration_preparation_records_test.go`
and `internal/daemon/collaborationrun/`. Real Linux namespace tests, explicit Git
and dynamic ELF fixtures are required. Darwin skips do not count. The actual
CLI/daemon/router capacity and partial-Resume tests remain required separately.

Current evidence uses synthetic providers and explicit test-owned Git tools.
An authenticated provider/version smoke, packaged installation and each other
provider/OS combination need their own evidence. Windows/macOS containment
continues to refuse; production creation/execution gates remain independently
controlled.
