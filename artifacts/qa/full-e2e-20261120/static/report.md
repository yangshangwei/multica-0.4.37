# Fresh static, unit and script verification

Candidate HEAD: `efdb597d6b80f930b5956de30bc8e4780e4023c7`. Existing dirty working tree was included; this lane made no product or test source edits.

## Main result

- Forced main lint/typecheck: 15/15 tasks passed, zero cache hits. Lint: 0 errors, 31 warnings.
- Mobile lint/typecheck: passed; 0 errors, 7 warnings.
- Fresh Vitest: 14,709 passed across 1,166 files; 0 failed, 0 skipped.

| Package | Files | Pass | Fail | Skip |
| --- | ---: | ---: | ---: | ---: |
| core | 372 | 5251 | 0 | 0 |
| views | 619 | 7606 | 0 | 0 |
| web | 34 | 298 | 0 | 0 |
| desktop | 103 | 1294 | 0 | 0 |
| docs | 10 | 64 | 0 | 0 |
| mobile | 28 | 196 | 0 | 0 |

## Additional fresh checks

- Initial Node script suite: 207 pass / 0 fail / 5 skip. Four skipped opt-ins were subsequently executed: Compose path handling 6/6; real offline installer script smoke 8/8. The backend lane subsequently completed the seed PostgreSQL integration: 13/13 named checks, zero skips, in its fresh disposable clone (`backend/seed-integration.log`).
- Native/fixture Node suite: 118 pass / 1 setup failure. The exact failed case passed after supplying its required command/evidence binding; original failed run retained. Unique final fixture cases: 119 pass.
- Python localization: 17/17 pass.
- UI wildcard guard (63 files) and mutation regression: pass. Mutation regression ran in a copied temporary source tree, preserving the shared candidate.
- Installer, entrypoint, Make naming, self-host Compose configuration, Helm chart configuration and iOS-run script regressions: pass.

## Failure retained and resolved

The stage-execution metric regression requires an absolute 0700 evidence directory and a JSON command binding carrying exact source/control/test SHA-256 values. The broad Node invocation initially omitted these. A dedicated replay supplied the required owned paths without editing or relaxing the test; it retained the actual Unix socket request/ack and verified metric freshness of 120 seconds. See `native-fixture-node.log`, `stage-control-replay.log` and `stage-control-evidence/`.

## Scope and remaining limitations

- Mobile checks cover static and headless helpers/data only, not real iOS/Android rendering.
- Node fixture tests prove fixture behavior, not complete native runtime acceptance.
- Loaded Docker image serves only Node runtime for mounted current scripts; does not verify current Web image.
- PowerShell installer suite not run: pwsh unavailable and Windows-specific environment not present.
- 31 main and 7 mobile existing lint warnings remain; no errors or product change justified by this lane.

## Evidence

- `summary.json`: reconciled checks and explicit limitations.
- `vitest-case-inventory.json`: all 14,709 current case identities and outcomes.
- `unit-*.json` / `unit-*.log`: package output and per-case reports.
- `*-command.json`, `unit-commands.json`, `supplemental-commands.json`: actual commands, duration, exits and tool identities.
- `stage-control-evidence/`: actual private control transport evidence.
