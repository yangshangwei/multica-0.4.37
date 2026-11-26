# Research: Native timing adapter review

- Query: Review the bounded old-CLI Docker timing probe and guard without executing adapters.
- Scope: internal, static inspection.
- Date: 2026-11-25

## Findings

**Not accepted yet.** Exact review decision and inspected script SHA256 values are in `native-adapter-review.json` beside this report.

1. The supplied `.omx/private/acceptance-fixes/native/adapter-review-request.json` was absent on three direct checks, including review completion. A request digest cannot be fabricated; request-bound acceptance awaits the concrete file.
2. `timing_probe.py:43–54` aborts cleanup at the first exception, skips remaining resources and can omit its completion receipt. It also lacks guaranteed bounded reaping of the guard process started at `:33` on the error path. Preserve primary/cleanup failures, try each exact resource, always write a terminal receipt, and reap that child.
3. `cli_guard.py:99–107` records creations only after successful Docker return. An ambiguous create/timeout can leave an unrecorded resource that final cleanup never inspects. Persist exact create intent first and reconcile only that exact name with matching owner/image before cleanup.

The static allowlist preserves the fixed v0.6.7 image, network-none/read-only settings, and relay cap-drop ALL/no-new-privileges/readonly mount. Stage containers are not started. The synthetic token/409 interaction is explicitly diagnostic and cannot prove the actual ten-operation API journey. Normal recorded-resource operations inspect ownership and use exact container IDs.

The probe explicitly points `DOCKER_CONFIG` at a personal configuration directory and does not pin an endpoint/context. Bind the intended local engine in the concrete request; a task-owned minimal config with an explicit local endpoint avoids default-context drift. No personal Docker config was inspected.

## Files found / related specs

- `.omx/private/acceptance-fixes/native/{timing_probe.py,cli_guard.py}` — reviewed execution and ownership/cleanup paths.
- Same directory `adapter-provenance.json` — declares reuse of the historical classify allowlist only, without historical runner/clearance imports.
- Same directory `{prepare_packages.py,freeze.py,verify_asar.py}` — inspected for readiness only; no package-preparation acceptance granted without its request/input/protocol recovery review.
- `e2e/fixtures/collaboration-installed-clients.ts` — previously read canonical fixed image, exact relay policy and complete ten-operation contract.
- `.trellis/spec/web/frontend/e2e-run-environment.md` and parent plan — frozen package inputs, exact ownership and protocol-state restoration requirements.

## Caveats / Not Found

- No adapters, Docker operations, services or product changes were executed. No external references were needed.
- The researcher role only allows writes under the parent research directory. The parent can copy the exact decision JSON to `native/adapter-review.json`; it must retain `accepted:false` until the issues are resolved and revised bindings reviewed.
- Package preparation is not cleared by this probe review. The existing protocol snapshot's contents and recovery were not verified here.
