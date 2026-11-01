# Root-owned environment follow-up

The first repaired legacy run completed: **9 pass, 1 intentional password skip, 0 fail**. The first repaired password run completed: **3 retained-attachment cases pass, 2 fail**. No product/CLI/provider change was made by this child.

Two remaining case prerequisites need the root environment owner:

1. Leave stops at its first `Resume all`, before the changed invite helper. Actual trace response: `POST /api/collaboration/issues/:id/control` → `503`, code `collaboration_execution_unavailable`. `collaboration_control.go` returns that code when the execution gate is disabled. Please configure the owned password acceptance environment for the existing protocol-only control fixture; do not change test expectations, limits or production defaults.
2. Source-surface passed its changed Recipient assertion and the earlier source layout assertions, then stops in `runReorderCLI` because `COLLABORATION_SOURCE_CLI` is missing. The fixture requires an absolute path to a freshly built source acceptance Multica CLI, hashes it, invokes only `issue reorder` in its isolated task config, and never uses an installed Agent CLI. Root owns the source-bound CLI/build fingerprint and runner env update.

Raw results remain at `.omx/private/e2e-maintenance/repaired-password-5/`; the safe machine record is `password-first-results.json`. No automatic retry was added or used.

`runtime-password-3` is now **COMPLETE: 3 pass / 0 fail / 0 skip**, including existing Web/native handoff and the new positive canonical ENV-1 regression. No password test is currently running, so root may update/restart its owned service and prepare the CLI. After root refreshes service permission/provenance for the two missing prerequisites, only Leave and source-surface require rerun; the three retained passes remain version-bound evidence.

## ROOT all prerequisites READY

Runtime-password-3 completeexit0/3pass/0skip observed before DB change. Root verified exact collab_joint_password_c3_20261102 identity and enabled only its protocol execution gate, incrementing epoch; host_execution_enabled remains false, productiondefaults untouched, no daemon or Provider. Existing sourceCLI env now available in both lanes. Run ONLY remaining Leave+source-surface under newlabel --lane password; old failures preserved. Source C3 unchanged. Private gateproof root password-maintenance/protocol-gate-enabled.json; include sanitized facts/hash in final maintenance evidence. No APIrestart was necessary.

## Resolved

The child ran only the two affected cases after the amendment: `repaired-password-final-2` exited 0 with 2 pass / 0 skip / 0 failure. Root's gate proof and hash are retained in `root-gate-amendment.json`; all final case/CLI fingerprints are in `acceptance-ledger.json`. No browser test remains active.
