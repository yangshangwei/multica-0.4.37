# Server delivery implementation

Boundary: extend the existing locked human workflow, immutable handoff/event
records, retained attachment binding and proof-aware execution controls. The
behavior gap is optional result prose, completion without exact delivery, and
missing task-owned manual admission. Server owns this file and `server/` only;
Core/Views and root-owned specs, environments and E2E files are separate lanes.

Expected changes: delivery projection/query and canonical tests; handoff,
lifecycle, workflow, access and creation; strict account handlers/routes;
manual policy migration and command/Resume/runtime/continuation admission;
generated sqlc; affected built-in workflow documentation. Existing source edits,
ordinary issue behavior, daemon_test.go and runtime proof identities survive.
No replacement result store, dependencies, new indexes/FKs, real providers,
deployment, commits or database allocation/migration execution.

Initial evidence: the coordinator's exact service permit ran all three retained
red regressions on `multica_detail_20261103_080906_service`, with zero skips.
`server-delivery-red-20261103-081753.json` records the assertion failures:
generic change/reopen permit review, blank result returns acceptance_submitted,
and external completion returns completed without a delivery.

Execution remains bounded: write canonical behavior tests before each change;
use only the coordinator's service/handler/router runner; notify root when
migration 764 is ready; serialize package execution and preserve failed runs.
