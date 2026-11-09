# Linux namespace admission: fixture executable permissions

The original `controlled/controlled-2/pw-07` browser case failed before any collaboration claim. Its managed registration succeeded, but the daemon printed `managed_identity_or_execution_capability_required` and never emitted readiness. The combined production warning is not proof of a credential defect: admission also requires an actual successful `NamespaceController.Probe`.

## Root cause proved without product changes

A private diagnostic program outside Git called the unchanged namespace controller with the exact original compiled provider and daemon helper, in the same cached arm64 image with the same privilege/security options. No API/database connection or provider account was involved.

- A small diagnostic helper with mode 0755 passed the real kernel probe.
- The exact daemon helper from the failed run returned EACCES at `fork/exec`.
- Container `stat` showed the copied daemon and provider had **mode 0700, UID 501**, inherited from the controller's private umask and Docker's copy ownership. The privileged outer root process can run them, but the nested user namespace maps root to UID 0 and cannot use host DAC override for UID 501's owner-only executable.
- Changing only the copied files' owner to 0 made the same probe pass.
- Independently, preserving UID 501 and all binary bytes while changing only mode to 0755 made the probe pass. This is the selected repair shape.

The diagnostic overlay exposed only the otherwise hidden `cmd.Start` error and helper stderr; it was not applied to product source. Kernel containment, managed identity, admission checks and browser assertions remain unchanged.

## Narrow controller repair

Only `/Users/artisan/.multica/qa/core-20261108/controlled/run.py` changed. Explicit controlled fixture compiler children now receive `umask=022`; their credential-free binaries are executable after Docker copies them into a nested namespace. The outer controller, parent evidence directories, ordinary subprocesses, private configuration and secrets retain umask 077 / directory mode 0700. No daemon source or E2E fixture edit was needed.

A credential-free process-boundary regression verified compiler child output mode 0755 and ordinary child output mode 0700. The original controller is preserved as `controlled/run.before-namespace-mode.py`, with before/after hashes in `controlled/namespace-mode-repair.json`.

## Evidence and acceptance boundary

Private evidence directory: `/Users/artisan/.multica/qa/core-20261108/namespace-diagnosis/`.

- `probe-1.json`: kernel/controller positive with the small helper.
- `probe-2.json`, `probe-4.json`, `probe-5.json`: exact daemon helper failure and exposed EACCES.
- `probe-6.json`: container ownership/modes and same-byte owner-change counterfactual.
- `probe-7.json`: same-byte 0755 helper/provider positive.
- `compiler-mode/`: compiler/ordinary child umask regression records.
- `probe-3.json`: preserved diagnostic setup failure from a temporary mixed Go package directory; corrected privately before subsequent probes.

All named diagnostic containers were removed; they used `--network none`. Original browser/API failure and cleanup remain in `controlled/controlled-2/pw-07/` and its parent `final.json`.

The successful isolated probe establishes the fixture admission cause and repair, not full business acceptance. The parent is integrating a newer candidate; a newly numbered complete `pw-07` run against that candidate must pass before this gap is closed. Historical `725378406` evidence remains tied to that source and cannot be relabeled as the newer candidate.

## Current-candidate business replay

Root's `controlled/controlled-3/pw-07` passed namespace admission after the compiler-mode repair, then exposed a separate collapsed Mention fixture interaction. Root added the existing disclosure helper to `sendCrossClientComment` (and the identical Host sender) without product changes. The complete **`controlled/controlled-4/pw-07` case passed in 59.3 seconds** on integrated `6201a5810`, exercising production Web, actual Electron and independent Linux namespaces. The mode repair and later visible-disclosure repair are separately evidenced; earlier failed runs remain failed. Other controlled lanes continue under root ownership.
