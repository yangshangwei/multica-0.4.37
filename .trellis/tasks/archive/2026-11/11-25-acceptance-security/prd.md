# Security acceptance remediation

## Goal
Close supported-toolchain/dependency scan findings and classify the public GitHub skills import failure using fresh evidence while preserving every original result.

## Requirements
- Preserve original D03 scan exit 3 / 14 reachable findings and public skills failure (original defects D10, called D11 in parent plan).
- Distinguish installed Go 1.27.0 from supported Go 1.26 latest patch and platform-specific findings.
- Verify current upstream vulnerability records and apply only minimum safe updates of existing dependencies; no new module dependency.
- Improve only GitHub skills error diagnostics if evidence identifies missing classification. Never infer rate limiting from HTTP 403 alone; never expose tokens or response bodies.
- No commits, pushes, deployment, global tool installation/configuration, personal credentials, old runner execution, retry/deadline weakening or security-policy changes.

## Acceptance
- [x] Original raw evidence hashes and official vulnerability records retained.
- [x] Supported patched toolchain baseline and repaired scans show exact versions, commands, exits and scanner provenance.
- [x] Release OS/architecture applicability and actual candidate binary build information are recorded distinctly from runtime execution.
- [x] Credential-free GitHub status and allowlisted headers captured; classification distinguishes quota, permission/auth and unknown refusal.
- [x] Changed behavior has focused normal/race regressions and static checks; parent owns complete integration regression.
