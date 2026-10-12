# Recovery-only main integration

The user authorized merging the completed recovery feature into main.
The integration branch starts at main `5af9e3f49` and replays the feature and
archive commits as `f14da8a30` and `da1291681`. The mixed development branch's
MCP selection work and its journals are excluded.

Pre-landing review found no blocker. All 38 runtime/test blobs and the complete
desktop/core/auth-locale trees match the previously accepted recovery version.
The feature patch IDs match. No manifests, lockfile, backend or CI changed.

Fresh isolated checks: 932 core tests, 1034 desktop tests, 64 locale tests;
core/desktop lint; core/desktop/views/web TypeScript; desktop build; and six
real Electron fixture groups all passed. The Electron profile/config and
loopback services are test-owned and removed when the run finishes.

Local knip reports 17 existing files and two existing dependency declarations.
An actual run at the unmodified main base produces identical findings. This is
not introduced by the recovery patch. The remote CI workflow, including its
full build and knip step, remains the final merge gate.

Logs, native captures and the independent merge review are in
`.omx/state/desktop-connection-recovery/` of the isolated integration workspace.
CI/PR outcome is recorded in the GitHub PR and the coordinator's merge report.
