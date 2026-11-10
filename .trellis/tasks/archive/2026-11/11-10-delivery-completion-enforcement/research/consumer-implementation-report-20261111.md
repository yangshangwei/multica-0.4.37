# Consumer authorization regressions

Date: 2026-11-11. Native test implementer: `implement_completion_ledger`.
Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`.

## Files and observed behavior

- `server/internal/handler/delivery_completion_consumer_test.go`: a real Operator-produced controlled artifact and a distinct Contributor consumer with a different human owner, agent, runtime, ordinary task and generation. The test drives authorization creation and the actual ordinary `UpdateIssue` writer, then checks persisted status, exact consumer identity, one terminal fact and the retained successful producer artifact.
- `server/cmd/server/delivery_completion_managed_integration_test.go`: actual password-mode middleware accepts issuer A's JWT and consumer B's managed task token. A and B, their tasks and agents remain distinct. The existing router creates, human-approves and physically executes A's Operator package; archive bytes, tar entry/content and SHA are inspected through the existing real archive assertion. No successful artifact, receipt or authorization is inserted by SQL.

Managed installation/binding/task credential identity uses the established `bindControlledArtifactRouter` fixture and real password/task authentication. These are local synthetic accounts and primitive identity rows, not a claim that installation enrollment or a provider ran. The real consumer PUT completes the issue with B's exact Contributor task, and its original repeat returns the same result without another status revision or terminal fact. Real task-token GET recovers a minimal receipt without private source/credential fields.

The source policy is installed as an explicitly labelled internal enforce fixture. Public policy activation is still a separate P4 gate; these tests do not imply that public activation or the whole delivery gate is complete.

## Red and green evidence

`p1-consumer-red01` ran the compiled original test against fresh `enf_p1_consumer_red01`. Authorization creation expected 201 but returned 403 `delivery_completion_denied` for the valid distinct Contributor consumer. The binary used plain `-test.v`, so the generic JSON inventory is zero; its raw log explicitly contains one named failure. `consumer-red-binary-20261111.json` records this distinction and binds that exact binary and compile-source record. The original failure is preserved.

The root assigned the correction to the core owner: only the two consumer minimum-autonomy checks changed to Contributor. Producer observation remains Operator. The test lane did not modify core code or assertions to make that behavior pass.

`p1-consumer-green01`, using a newly compiled binary, test2json and fresh `enf_p1_consumer_green01`, passed its one named test with zero failures/skips. It proved actual ordinary PUT plus durable consumption after the corrected grant, not just successful issuance or enqueue.

`p1-managed-router01`, using test2json and fresh `enf_p1_managed_router01`, passed 10 named tests (3 top-level plus 7 live-fence cases), with zero failures/skips:

- distinct issuer A / managed owner B, real Contributor completion, original replay and minimal GET;
- after authentication but at request-body read: binding revoked, binding epoch changed, installation retired, organization mapping removed, issuer password version changed, consumer password version changed, source member removed;
- a grant signed for explicit absence of a managed binding cannot adopt a later valid managed binding/token on that same task.

Each live-fence callback actually executed after middleware acceptance; rejected requests left issue status/revision and terminal count unchanged, and preserved the real producer success record. Raw HTTP logs show 403 for the individual authority failures and 409 for removed organization mapping. The absence-to-binding case returned403.

After inspecting those logs, the current managed test assertions were narrowed from any4xx to these exact expected codes. `p1-managed-router-compile03` passed for this final test source, but its runtime normal/race belongs to root's final frozen suites. Do not represent the earlier broad-assert run as execution of the later exact-code assertions.

## Verification identity and limits

`consumer-verification-20261111.json` records current test SHA-256, original/green/managed binary hashes, compile-source records and raw-log hashes. Source during a later run may have changed in another lane; binary provenance comes from its matching compile record, not merely the HEAD at execution time.

All four compile-only records passed. This lane created no database or migration, reused no completed database, and ran only the exact package/database permits supplied by root, sequentially. It did not run supplier/model calls, external messages, deployment, push or commits. No new dependencies were introduced.

An initial read attempted nonexistent candidate autonomy source paths, then used `rg --files` to find the real file; it was a read-only lookup failure, not a product red. No fixture failure occurred in the consumer or managed runtime runs.

Root still owns final ordinary/race, complete writer inventory, independent review, public policy activation and safe integration. M1/full-delivery completion is not established by this test slice.
