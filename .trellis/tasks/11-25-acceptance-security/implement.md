# Execution plan

1. [x] Load parent artifacts and repository/spec guidance; map originals and write child boundary.
2. [x] Capture immutable original hashes, official advisories and supported toolchain provenance; collect anonymous GitHub diagnostic headers.
3. [x] WAIT for parent's planning-review accepted message. Then task.py start this child before any product edit.
4. [x] Scan unchanged dependencies under supported patched Go, preserving baseline. Select smallest fixed module versions, inspect graph diff for accidental additions/upgrades.
5. [x] Add failing focused diagnostic regression, implement minimal error classification if warranted; gofmt.
6. [x] Run focused normal/race handler tests with existing agent CLI guard and isolated DB; go vet/build relevant packages; go mod verify.
7. [x] Run supported patched scanner on all release GOOS/GOARCH targets; build actual native API and CLI plus cross-target CLI binaries and retain go version -m / SHA256. Report runtime limits explicitly.
8. [x] Record commands/exits/source hashes/resources plus remaining gaps in report.md and evidence manifest; parent owns broader full module regression and integration review.

Rollback: scoped owned-file diff only. Retain all failed runs and original evidence; do not reset shared checkout. No commits/push/deploy.
