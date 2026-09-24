# Repeatable desktop download operations

The user requested a detailed operational handoff, documentation updates and shell helpers to reduce manual work. Continue the already authorized task; real installed-client acceptance remains pending.

## Scope and approach

- Extend the existing `scripts/desktop-updates.sh` entry point rather than add a second publication implementation.
- Add persistent `updates.env` support and an explicit `--env-file PATH` option. Resolve dotenv with Docker Compose, never source/eval operator configuration. Preserve legacy environment-variable and Make invocations.
- Add `collect SOURCE DESTINATION`, `export-image OUTPUT PLATFORM`, and `verify [METADATA] [--expected-version VERSION]` commands. Existing publish/configure behavior continues to use the current tested Node helpers.
- Export a pinned Nginx image for an explicit Linux server architecture, produce a loadable archive, and record the appropriate image tag. Exporting does not restart running services.
- Add a Node HTTP verifier that checks generated metadata, expected version, safe same-feed artifact references, HEAD/Range/full-download integrity and available blockmaps. Reuse existing parsing/validation where practical. Reject redirects and report evidence; do not change server or client state.
- Supply an example environment file and document repeatable first deployment, normal release, verification, client configuration and recovery. Explain what a pure Nginx host needs versus a publishing/verification machine with Node and repository dependencies.
- Keep business server/database upgrades on the existing offline-upgrade workflow; link it explicitly. Do not rebuild packages, change real client configuration, restart the existing service or declare real-device acceptance from download checks.

## Implementation lanes

1. Shell orchestration and behavior tests: `scripts/desktop-updates.sh`, `scripts/desktop-updates.test.mjs`, `deploy/desktop-updates/updates.env.example`.
2. HTTP verification and behavior tests: `apps/desktop/scripts/verify-updates.mjs`, its colocated tests, minimal shared validation exports in `update-artifacts.mjs` if needed.
3. Main agent: documentation, task/spec records, integration, final review and actual isolated-service checks.

All lanes preserve other agents' edits and pre-existing unrelated dirty files. No new dependencies.

## Verification

- Demonstrate failing tests for the new shell/configuration and verifier behaviors before implementation.
- Cover persistent config, paths containing spaces, no shell evaluation, rejected arguments and failed Docker operations; cover checksum/size/version errors, missing artifacts, redirects and incomplete/invalid Range responses.
- Run relevant Node/Vitest suites, shell syntax checks, desktop lint/typecheck and document/configuration checks.
- Exercise the actual shell helpers against a temporary Compose project and real existing ia32 build: collect/publish, verify SHA-512 and range, expected-version mismatch, image export/load and restart persistence as applicable. Preserve the existing local service.
- Record current commands, outputs, limitations and remaining real-client acceptance in task artifacts and the operation guide.

## Result

All operator-helper and documentation work is implemented and locally verified.
See [operations-verification.json](operations-verification.json) for ten actual
Docker/HTTP checks and test counts, and
[the operator runbook](../../../../../docs/desktop-intranet-update-runbook.zh-CN.md)
for first deployment and repeated release steps. The development task is archived
at the user's request. Signed installed-client acceptance remains a deployment
follow-up in the runbook and is not claimed complete by this archive.
