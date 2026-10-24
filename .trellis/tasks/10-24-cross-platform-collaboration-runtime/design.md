# Technical design: OS-neutral collaboration execution

## Design objective

Expose one collaboration execution contract to the product while allowing the
daemon to use a native containment implementation on each host OS. The server
must reason about an admitted capability and a frozen proof identity, not about
Linux-specific implementation names.

The implementation must keep the current fail-closed behavior. A host that
cannot prove its advertised containment is unavailable for collaboration runs;
it must never silently fall back to ordinary task execution.

## Boundaries

### Server protocol and persistence

1. Replace the single-value containment check with a small allowlist of signed,
   versioned adapter capabilities. The first values are expected to be:
   `linux_pid_namespace_v1`, a macOS adapter value, and a Windows adapter value.
   The exact macOS and Windows values are not final until their native probes
   prove the required properties.
2. Keep `CollaborationRuntimeProtocolVersion` and the run identity, claim,
   permit, outcome, stop-intent and proof shapes stable when possible. The
   `containment` text already persists on runs and attempts, so adding accepted
   values should not require a database migration. Bump the protocol only if
   the wire contract or proof fields must change.
3. Validate the following at every boundary: known containment, host OS/arch,
   adapter version, registered capability digest/epoch, and equality between
   claim, run, attempt and proof. Unknown combinations remain rejected.
4. Treat the capability as a frozen run input. A later capability or config
   change invalidates new claims, while an in-flight run retains its existing
   proof and capacity barrier.

### Daemon orchestration

Extract the platform-independent parts from `NamespaceController` into an
OS-neutral containment interface. The common runner owns:

- explicit executable/bundle selection;
- immutable input and capability validation;
- claim/permit/stop/outcome/proof calls;
- nonce burn and ledger recovery;
- bounded output and preparation facts;
- capacity release only after a verified terminal proof.

The platform adapter owns:

- native capability probe and adapter identifier;
- process creation attributes and descendant containment;
- temporary root/work/input layout;
- privilege/token/ACL reduction;
- native executable inspection and dependency closure;
- scope identity and termination observation;
- native helper entrypoint used by the daemon binary.

The existing `NamespaceController` can remain as a compatibility facade while
the shared type is introduced, but new code should depend on the interface so
the server never needs an OS branch.

### Native adapters

#### Linux

Keep the existing user/PID/mount/IPC/UTS namespace, tmpfs, proc, pivot-root,
explicit device and `prctl` implementation. Move the ELF inspector and Linux
preparation helpers behind the Linux adapter. Preserve the current namespace
identity and proof behavior before enabling the other adapters.

#### macOS

Implement a native adapter only after a host-level spike establishes a supported
containment primitive for the versions we ship. The adapter must provide:

- a profile or equivalent that restricts file access to the generated run root,
  declared inputs, provider closure and required network endpoints;
- inherited descendant containment that survives leader exit;
- a stable scope identity and a probe that can demonstrate all descendants are
  gone before the ledger releases capacity;
- explicit temporary HOME/TMP and credential environment handling.

`Sandbox-exec` or a process group may be used only if the spike demonstrates the
required security and proof properties on supported macOS versions. Ordinary
PGID cleanup or an unverified deprecated command cannot be advertised as the
macOS equivalent of the Linux namespace. If no supported primitive can satisfy
the contract, the adapter reports unavailable and the release remains blocked.

#### Windows

Implement a native adapter around a Job Object with kill-on-close and child
process restrictions, plus explicit process creation and handle inheritance.
The adapter must also establish the filesystem, environment and credential
boundary required by prepared bundles. Existing `server/internal/daemon/processtree`
and `execenv/isolation_windows.go` are reusable process-tree references, not
proof of the collaboration contract by themselves.

The Windows spike must decide whether a restricted token/AppContainer or an
equivalent ACL-backed private root is required. The adapter is not complete
until it proves: no breakaway descendants, no inherited control handles, safe
leader-exit behavior, deterministic kill/reap, and no host credential leakage.

### Executable and bundle inspection

Introduce an `ExecutableInspector` boundary with platform implementations:

- Linux ELF: retain architecture, interpreter and declared library checks.
- macOS Mach-O: validate thin/fat slices, host architecture, load commands,
  `@rpath`/`@loader_path` resolution and declared dylib closure.
- Windows PE: validate machine type, subsystem/entrypoint and declared DLL
  imports/side-by-side closure without PATH discovery.

Keep logical bundle destinations canonical and platform-neutral. Host source
paths remain local configuration and never enter the wire manifest. Use an
adapter-specific system-file allowlist for certificates, resolver settings and
other network prerequisites; do not reuse Linux `/etc` assumptions on macOS or
Windows.

### Capability registration and selection

At daemon startup:

1. Determine `GOOS`/`GOARCH` and create the matching adapter.
2. Run the native containment and executable/bundle probes.
3. Build the full capability summary, including OS, arch, adapter/version,
   supported preparation format and limits.
4. Register or recover the exact summary at the existing config epoch.
5. Enable the collaboration poller only after the probe and registration pass.

The server keeps candidate selection OS-agnostic. It sees a runtime with an
admitted capability; it does not select Linux, macOS or Windows branches in the
queue or UI.

## Data flow

```text
daemon host
  -> adapter probe + executable/bundle probe
  -> capability registration (OS/arch/adapter/version/digest)
  -> server freezes matching capability on first claim
  -> claim/permit carry the same containment value
  -> common Runner invokes adapter with immutable input
  -> adapter emits scope identity and terminal evidence
  -> common Runner seals ledger and submits outcome/proof
  -> server accepts proof only when containment and identity still match
```

## Compatibility and rollout

- Existing Linux clients and stored `linux_pid_namespace_v1` rows remain valid.
- A new daemon advertises only the capability it actually probes; it never
  rewrites an old registration in place after a host/config change.
- During rollout, server acceptance can be widened first while no new adapter
  is advertised; each platform is enabled only after native CI and installed
  client evidence is available.
- A failed probe leaves ordinary Multica runtimes usable and marks only the
  collaboration capability unavailable.
- No user-visible OS switch is added. Diagnostics may show the adapter and
  reason for unavailable capability so operators can repair the host.

## Risks and deferred decisions

- macOS Seatbelt availability and network/file semantics vary by OS release;
  the spike must establish a supported minimum before implementation is
  accepted.
- Windows Job Objects solve process-tree ownership, not full filesystem or
  credential isolation. The token/AppContainer/ACL choice must be proven by
  native tests before choosing the adapter identifier.
- Mach-O and PE dependency closure is substantially different from ELF and may
  require separate bundle format versions. If the existing manifest cannot
  represent those closures safely, version the preparation format instead of
  weakening validation.
- Native CI runners and real installed provider CLIs are required; Docker,
  Wine, cross-compilation and renderer mocks are supplementary evidence only.
