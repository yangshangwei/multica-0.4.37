# Verification environment

Task-owned private files: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-security-v3-PJ9pJ5` (directory0700, configuration0600; credentials intentionally omitted).

Fresh databases allocated by root from local PostgreSQL admin connection, each identity verified before migration:
- `multica_security_v3_20261128_51f06eb9_service`
- `multica_security_v3_20261128_51f06eb9_handler`
- `multica_security_v3_20261128_51f06eb9_router`

All migrations returned exit0. Allocation and migration logs retained privately. Initial cluster load54/100 connections; serialize DB packages. Backend child owns service/handler filtered execution permits through private run-go.mjs. The runner verifies current_database, uses a private per-package HOME, pins Go cache paths, generates nondefault signing secrets, strips API-only overrides, guards real provider CLIs and retains JSON event counts. Root owns further final verification allocation.

Initial static check `go -C server vet -p 2 ./internal/service ./internal/handler ./internal/attribution ./pkg/db/generated` passed; final changes require final check. `git diff --check` and task manifests validation passed.

The screenshot instance cannot currently be read through the default CLI: `server/bin/multica squad get 30806e57-7e7d-4eab-951a-eca9b15c1d50 --output json` returned “No server configured.” No live instance or production environment has been changed. Source/template delivery and running-instance upgrade must be reported separately.
