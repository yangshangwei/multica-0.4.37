# FCG full-check backend regressions — 2026-10-07

## Diagnosis and scope

Source failure evidence: `/tmp/i1-full-check-3.log:8315` onward.

- `TestAutopilotCreateIssuePositionBelowCurrentMinimum` selected an arbitrary agent with `SELECT id FROM agent WHERE workspace_id = $1 LIMIT 1`. Its manual dispatch correctly failed the real agent invocation gate when the selected agent was not invocable by the caller.
- `TestQuickCreateIssueParentTrustBoundary` used the same arbitrary-agent selection, then changed that agent's shared runtime CLI metadata. Its parent/field/version assertions encountered invocation403 before reaching the behavior under test.
- `TestWorkspaceDeletionManifestCoversPublicSchema` did not classify seven I1 tables. The production `DeleteWorkspaceIterationData` query already deletes all seven in the workspace teardown transaction (`workspace.go` deletion graph); existing I1 deletion/rollback tests exercise that behavior.

The narrow pre-change rerun reproduced the manifest failure; the two agent-dependent tests passed alone, demonstrating their dependence on surrounding shared fixture state. Log: `/tmp/i1-fcg-regression-red.log`. The earlier full-check failures remain the failure evidence for the agent selection issue.

## Changes

Only test fixtures and the test manifest changed:

- `server/internal/handler/issue_create_position_test.go`: dedicated owner-authorized agent and runtime built through dbfx.
- `server/internal/handler/quick_create_parent_test.go`: dedicated owner-authorized agent/runtime, with the required CLI version on its own runtime. Removed shared-runtime selection/mutation/cleanup.
- `server/internal/handler/workspace_delete_manifest_test.go`: classify `workspace_iteration_settings`, `iteration`, `iteration_participation`, `iteration_event`, `iteration_snapshot`, `iteration_operation`, and `iteration_notification` as workspace-delete, matching the existing transaction query.

No production permission check, authorization bypass, grant interpretation or deletion behavior changed. Private-agent denial behavior remains intact. No dependency or migration was added.

## Verification

Dedicated DB: `multica_i1_cg_20261006_2310`, explicitly supplied through DATABASE_URL. Every Go test uses `scripts/go-test-with-agent-cli-guard.sh`.

Narrow `-race` rerun of both failed fixture groups, manifest coverage, and `TestDeleteWorkspaceIteration*`: PASS 2.431s (`/tmp/i1-fcg-regression-green.log`).

Full handler regression (`go -C server test -race -p 1 -parallel 2 ./internal/handler -count=1`) is recorded separately in `/tmp/i1-fcg-handler-full.log`; completion result will be appended after its process exits. This is not a claim that the entire make-check pipeline has passed.

Full handler regression completed: **PASS, 199.120s**, exit 0. `/tmp/i1-fcg-handler-full.log`. This run includes real permission-denial tests, workspace deletion coverage, and the formerly order-dependent fixture tests together. `git diff --check` also passed. No known failures remain in this assigned regression slice; root owns the resynchronized full pipeline.
