# Implementation plan: OS-neutral collaboration execution

This is a planning artifact. Do not start implementation until the final plan
is approved and `task.py start` activates the task.

## Ordered work

1. **Lock the current contract.** Add or refresh focused tests around the
   existing Linux capability allowlist, claim/permit/proof containment equality,
   retained ledger barriers and prepared bundle validation. Record the exact
   acceptance fixtures before extracting interfaces.
2. **Run native containment spikes.**
   - macOS amd64 and arm64: prove the selected sandbox mechanism, network
     behavior, descendant cleanup, scope identity and private file root.
   - Windows amd64 and arm64: prove Job Object ownership, no-breakaway child
     behavior, handle inheritance, restricted identity/filesystem boundary and
     deterministic reaping.
   - Reject an OS adapter design that cannot produce durable terminal proof.
3. **Extract common runtime interfaces.** Split the generic Runner, ledger,
   preparation, output, capability and proof code from Linux-only controller
   details. Keep Linux behavior unchanged while the interface is introduced.
4. **Generalize protocol validation.** Add versioned containment capabilities,
   OS/arch matching, adapter/version validation and cross-boundary equality
   checks. Keep old Linux wire rows and clients compatible; add migration only
   if the preparation manifest or DB representation must change.
5. **Port executable and bundle validation.** Introduce ELF/Mach-O/PE
   inspectors, canonical logical destinations, platform loader/library closure,
   system-file allowlists and native source-file opening/permission checks.
6. **Implement the macOS adapter.** Add native build-tag files, helper entrypoint,
   root/work/input setup, credentials and network policy, scope identity,
   stop/wait/reap behavior and capability probe. Preserve the common Runner and
   ledger semantics.
7. **Implement the Windows adapter.** Add native build-tag files, Job Object and
   process creation setup, token/ACL or AppContainer boundary, helper entrypoint,
   private root/input setup, scope identity, stop/wait/reap behavior and probe.
8. **Generalize daemon registration and polling.** Remove Linux-specific request
   construction from `collaboration.go` and `collaborationrun/client.go`; use the
   adapter capability for claim, permit and proof. Keep candidate polling and
   capacity semantics platform-neutral.
9. **Add native integration coverage.** Extend the server router and daemon
   suites for each OS/arch. Cover direct child, double fork, leader exit,
   closed stdio, cancellation, Stop/Resume, provider outcome, network loss,
   restart recovery, attachments/Git and malformed capability/manifest cases.
10. **Wire CI and packaging.** Add native macOS and Windows lanes for x64 and
    arm64 where hosted runners permit, publish raw JSON evidence, and keep Linux
    amd64/arm64 lanes. Do not count emulation or cross-build-only checks as
    native acceptance.
11. **Update operator and user docs.** Explain that collaboration is selected
    uniformly while the daemon reports an internal native capability; document
    probe failures, supported architectures, install artifacts and recovery
    diagnostics. Keep ordinary execution documentation unchanged.
12. **Run final quality gate.** Execute focused Go tests, full Go tests, TypeScript
    typecheck/tests, lint/static analysis and all native acceptance jobs. Review
    the diff for protocol compatibility, security boundary regressions and
    accidental ordinary-runtime behavior changes.

## Primary files / ownership groups

- Protocol and server admission: `server/pkg/protocol/collaboration*.go`,
  `server/internal/service/collaboration_runtime*.go`, related SQL queries/tests.
- Common runtime: `server/internal/daemon/collaboration.go`,
  `server/internal/daemon/collaborationrun/{controller.go,runner.go,ledger.go,preparation.go}`.
- Linux adapter: existing `controller_linux.go`, `preparation_linux.go`,
  Linux provider/bundle inspector and tests.
- macOS adapter: new `*_darwin.go` controller, preparation, executable inspector,
  native helper/probe and tests.
- Windows adapter: new `*_windows.go` controller, preparation, executable
  inspector, native helper/probe and tests; reuse only proven pieces from
  `processtree` / `execenv/isolation_windows.go`.
- Documentation/spec/CI: `.trellis/spec/server/collaboration-*.md`,
  `.github/workflows/*`, release/install docs and runtime diagnostics.

## Validation commands

At minimum, run the focused package tests after each stage:

```bash
go test ./server/pkg/protocol ./server/internal/daemon/collaborationrun ./server/internal/service
go test ./server/cmd/server ./server/internal/daemon
go test ./...
pnpm typecheck
pnpm test
make check
```

On native hosts/runners additionally run the platform-tagged collaboration
acceptance suites and preserve their raw evidence. The exact command names and
environment gates should be added to the implementation task after the native
spikes establish the adapter contracts.

## Rollback points

- Revert protocol allowlist changes while keeping the extracted Linux adapter.
- Disable macOS or Windows capability registration independently if native
  probes fail; ordinary runtime polling remains available.
- Keep old Linux containment values and stored rows readable throughout rollout.
- Do not ship a platform adapter whose proof path falls back to PID/PGID or
  ordinary task completion heuristics.
