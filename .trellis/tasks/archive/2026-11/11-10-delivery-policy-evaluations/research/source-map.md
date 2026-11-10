# Research: Project delivery policy and durable evaluations

- Query: Find the smallest independently testable continuation of M1: versioned project policy and persistent observations, reusing the accepted evaluator and controlled artifact evidence.
- Scope: Internal, read-only source research in `/Volumes/artisan/code/2026/mc-delivery-policy-20261110`; baseline supplied by coordinator `c3bc9abd1`.
- Date: 2026-11-10
- Authority: Parent session owns product decisions, planning, implementation and verification. Research makes no product edits, runs no database tests and creates no database state.

## Findings

### Existing implementation to reuse

| File | Existing responsibility |
| --- | --- |
| `server/internal/handler/delivery_evidence.go:119` | Original human-only read-only HTTP evaluation boundary; commit succeeds before returning observation. |
| `server/internal/handler/delivery_evidence.go:155` | `evaluateDeliveryEvidence` accepts the caller's transaction, queries, verified principal and resolved issue; all observation logic is reusable without a new execution engine. |
| `server/internal/handler/delivery_evidence.go:22` | Fixed algorithm contract `delivery-artifact-observe-v1`; distinct from a project's configuration revision. |
| `server/internal/handler/delivery_evidence.go:35` | Response already contains issue/project versions, original execution/approval, selected/observed source, spec/artifact/receipt digests, producer identity, observation time and expiry. |
| `server/internal/handler/agent_artifact.go:62` | `artifactPrincipal` accepts verified JWT/PAT humans and rejects forged agent/task headers; restricted and machine identities cannot become human evaluation actors. |
| `server/internal/handler/agent_artifact.go:79` | `artifactTx` supplies a bounded lock timeout, transaction-bound queries, artifact schema readiness and no automatic retry. |
| `server/internal/handler/agent_artifact.go:102` | Workspace existence/teardown fence. |
| `server/internal/handler/agent_artifact.go:114` | Current JWT/PAT/password-session/managed-binding credential checks under locks. |
| `server/internal/handler/agent_artifact.go:188` | Sorted live user/member SHARE locks, disabled-user rejection and current membership roles. |
| `server/internal/handler/agent_artifact.go:236` | Artifact authority: current producer-agent and caller authority, original human initiator and optional approver; completed tasks are allowed on read paths. |
| `server/internal/handler/handler.go:1132` | `loadIssueForUser` resolves alias/UUID, workspace, task capability and legacy collaboration restrictions; write resolved UUIDs rather than URL text. |

Do not rewrite the original `/delivery-evidence/evaluate` route or widen its accepted body. A separate persistent evaluation route can use the same typed request and function inside its write transaction. The original observe API should remain ledger-free and keep its compatibility tests.

The existing observer is intentionally conservative: four PR/link tables take SHARE NOWAIT to fence link phantoms, PAT last-used writes can legitimately contend, and receipt expiry is the existing 15-minute artifact admission horizon. Those limits must survive persistence. Stored PR heads still represent database observations, never remote Git HEAD verification.

### Project policy locking and authority

`server/pkg/db/queries/project.sql:12` locks metadata writes with `FOR NO KEY UPDATE`; `:19` locks execution configuration/deletion with `FOR UPDATE`; `:89` and `:92` provide SHARE reads and NOWAIT SHARE reads. `server/pkg/db/queries/lifecycle.sql:5` locks issue revisions with `FOR NO KEY UPDATE NOWAIT`.

Append-only policy revisions can be serialized by a project `FOR UPDATE NOWAIT` lock and selected by a separate query after locking. This avoids adding a mutable policy pointer or fields to every project model/response. A unique concurrent index on `(workspace_id, project_id, revision)` makes the revision identity explicit and supports selecting the latest revision. Acquire the same project SHARE lock before policy lookup on evaluation; it also fences the absence of the first policy row.

READ COMMITTED with fresh statements is appropriate: a child-policy INSERT does not change the project tuple. The resource contract in `.trellis/spec/server/project-execution-squad.md` explains why a waited-for project lock under REPEATABLE READ can leave a child lookup using an old snapshot. NOWAIT is simpler for this increment and matches the evaluator's existing refusal semantics.

`projectHumanActor` (`project_write_fence.go:49`) is useful evidence of the human-only project convention. `runProjectTransaction` (`:74`) retries transactions and only fences membership; it does not supply the observer's exact current credential checks. Prefer the artifact/current-principal transaction pattern for the new endpoints rather than inheriting those retries or copying a second authority implementation.

`requireProjectAdministrator` (`project.go:850`) and planning-timezone writes (`project_timezone.go:113`) enforce owner/admin management. Ordinary project writes have a different policy. There is no existing delivery-policy management authority to inherit automatically. The coordinator asked the user to choose owner/admin versus owner/admin plus project lead; the user continued after the owner/admin recommendation, which is now the implemented restriction. If owner/admin is chosen, check the member role returned under `artifactUsers` locks; do not read an unlocked role and later append.

### Proposed bounded contract

These are implementation recommendations subject to final task artifacts, not claims of implemented behavior.

1. `GET /api/projects/{id}/delivery-policy`: current authorized human member; return `revision: 0`, `mode: off`, the fixed algorithm and target stage when no policy exists. The zero revision is a virtual default, not a fabricated historical write.
2. `PUT /api/projects/{id}/delivery-policy`: strict bounded JSON with expected policy revision, fixed algorithm, `mode: off|observe`, fixed `stage: done`; current management authority checked in the transaction. Append revision N+1 under the project lock and expected-revision CAS. No free-form checks, arbitrary validity window, warn/enforce, skip, deployment or automatic backfill.
3. `POST /api/issues/{id}/delivery-evidence/evaluations`: mandatory single canonical nonzero UUID `Idempotency-Key`; original eight observation fields plus `expected_policy_revision`. Require a current project with observe policy and exact policy revision before creating a new evaluation. Existing no-project observations remain available through the original stateless endpoint.
4. `GET /api/issues/{id}/delivery-evidence/evaluations/{key}`: recover only a currently authorized original human evaluator's key in workspace/user namespace. Use issue UUID as a request binding, not a namespace shortcut allowing one key to create two evaluations.

Project policy `revision` and `policy_version` have different meanings: the former identifies a retained project configuration, the latter fixes the evaluation algorithm. `stage: done` identifies the future transition being observed; it must not imply this release intercepts status transitions or require the issue already to be done. `off` disables new durable evaluation admission, not historic retention, current authorization checks, the existing stateless observer or a separate security boundary.

For policy PUT, expected-revision CAS is sufficient without a second idempotency ledger: after a lost response, GET returns the committed revision; repeating the old CAS conflicts. Define explicitly whether an identical PUT at the current expected revision creates another audit revision or is a no-op. The simplest append-only rule is one revision per accepted PUT, including an explicit same-value setting; never silently overwrite an older revision. Policy rollback is another appended configuration, never revision deletion.

The new evaluation should persist an immutable nested `observation` and a separate `current_applicability` projection. Do not return a bare saved `pass` where clients could treat it as current authorization. Always state snapshot provenance and that it grants neither done nor release permission.

### Persistent identity and transaction

A minimal two-table design is enough:

- `project_delivery_policy_revision`: workspace/project UUIDs, monotone revision, fixed algorithm version, mode, stage, verified human creator and database creation timestamp. Append only.
- `delivery_evidence_evaluation`: workspace, original evaluator user, key, resolved issue, canonical request digest and request JSON, original project/policy binding, immutable observation JSON, and recorded timestamp (plus generated evaluation ID only if the chosen API needs one).

Reuse the request canonicalization pattern in `comment_delegation.go:206`, but do not copy dispatch claims or mutable unknown-to-recorded state: evaluation has no external side effect and can atomically INSERT the final pass/hold/unknown observation in the same transaction as all its locked checks. The unique key index should be `(workspace_id, evaluator_id, request_key)`, not include the issue ID, so cross-issue key reuse returns 409. The resolved issue and every expected binding, including policy revision, belong to the request digest. Renewed valid JWT/PAT for the same user can recover; raw tokens, token hashes and supplied actor headers are not persisted evaluation identity.

Claim/check the existing key only after current caller/resource authorization. Same key and same canonical request recover its original observation without another INSERT; changed issue or body conflicts. In concurrent duplicate requests, the issue lock/unique index yields either the original row or a bounded 409; it must not create two rows. A failed or uncertain commit returns no usable success; GET with the original key resolves a committed row. If no row committed, a later explicit POST can compute a new observation because no execution or other external effect occurred. This is deliberately different from unknown execution receipts, whose side effects must never be retried.

An observed `unknown` is a committed immutable evaluation result, not an unfinished work claim. Never promote the same key to pass after evidence arrives. A new explicit evaluation/key is required. Do not extend `valid_until` while recovering; preserve the artifact expiry used by the original observation.

### Important recovery authorization trap

The existing evaluator checks expected issue/project revisions before checking retained artifact producer and approver authority (`delivery_evidence.go:174` onward). Therefore catching its revision-conflict 409 and returning the stored observation as stale is not sufficient authorization: it can skip later current-authority checks.

Recommended replay/read flow:

1. Resolve and lock the current issue, current workspace/member/credential and current project; verify the stored row's workspace, actor/key namespace and original issue binding.
2. Keep the stored request and observation immutable. For current recheck only, create a temporary copy of the request with the already-locked current issue/project revisions. Keep its original approval, source/spec/artifact expectations and fixed algorithm.
3. Invoke `evaluateDeliveryEvidence` in the same transaction using that temporary request. This traverses its current artifact authority checks even when versions have changed. Authorization refusal remains refusal; do not catch it as an innocuous stale marker.
4. Independently compare original issue/project/policy identities and revisions and expiry to mark current applicability stale. If those match, only an originally passed observation whose live recheck still passes and still binds the same execution/producer/receipt can be described as currently matching. It is still a point-in-time observation with no transition authority.
5. Original hold/unknown remains nonpassing regardless of a later live recheck; no row mutation. Read-time lock contention/database error remains an HTTP error, never a guessed current state.
6. Perform current authorization for saved hold/unknown too when evidence is retained; do not use their nonpass status as an exemption permitting data after revocation. A formerly missing approval can now belong elsewhere; the recheck should enforce its scope rather than leaking new data.

Comparing only current verdict is insufficient. A changed original execution, producer generation or receipt digest cannot become a current match just because some later evidence passed. The final implementation should compare original retained identity and digests as well as expected request fields.

For a current project move, recheck artifact authority using the current project pair and classify the original binding stale. The coordinator's final design chooses public-read refusal when the original project, issue or workspace has been deleted: verify original project existence separately from the current project binding, retain the rows, and return 404/403 as appropriate. Do not bypass loaders to retrieve orphan history directly by key. Newly added policy management/read routes must also check real project existence in the workspace, not let retained history resurrect a deleted project. If the original observation had a retained execution identity and that execution or required producer/approval binding is now absent, deny historical read rather than using the evaluator's early `evidence_missing` return as authorization for old material. An evaluation that originally observed missing evidence has no retained producer details to authorize; its saved unknown still cannot become pass.

### Deletion and rollback

Project deletion holds the exclusive project lock (`project.go:901`), detaches issues and increments their revisions (`project.sql:122`), then deletes project-owned update records (`project.go:1006`) and the project. A separate policy/evaluation history table is naturally retained if it is not added to those deletes. Document that intentional retention and assert it with real deletion tests. Current workspace and issue fences prevent an observation from being inserted through a concurrent teardown.

Workspace teardown intentionally retains controlled execution evidence and associated approvals (`workspace_delete.sql:341`); `TestControlledArtifactWorkspaceDeletion` (`agent_artifact_test.go:340`) is the closest existing retention fixture. New policy/evaluation tables should preserve audit facts across deletion while refusing subsequent public reads. No foreign keys or cascading actions may be added.

Migrations are the sqlc schema source (`server/sqlc.yaml:5`), so there is no separate schema.sql file to edit. Last current prefix is 775. Use separate table migration(s), one single-statement concurrent migration per index, then a readiness/shape check. Register each concurrent build with `concurrentIndexCleanups` (`cmd/migrate/main.go:122`) and maintain matching up/down cleanup coverage.

Existing pattern: `771_comment_delegation_receipt.up.sql`, `772...key.up.sql`, `773...ready.up.sql`, `775...replay_ready.up.sql`; `771/773/775` downs take ACCESS EXCLUSIVE NOWAIT and refuse retained rows. The highest new down must protect BOTH policy history and evaluations before any index drop, and the table-drop migration repeats that fence. Do not allow evidence deletion to make downgrade succeed. Separate concurrent index drops require admission stopped for the entire maintenance window. Runtime must fail closed if required index identity/shape is missing, not just trust schema_migrations.

Caveat: `workspaceDeletionManifestCoversPublicSchema` (`workspace_delete_manifest_test.go:290`) has a historical `keep` rule that forbids a workspace_id column (`:360`), and its map currently does not mention the already-retained artifact/delegation tables. A new retained workspace-owned table needs an explicit retention category/contract if updating that broad manifest. Do not mechanically classify it as delete or claim that unrun broad manifest coverage is green. The coordinator should distinguish baseline drift from this task's narrowly tested deletion proof.

### Actual test seams and acceptance

- `deliverySuccess` (`handler/delivery_evidence_test.go:49`) creates and actually executes a controlled archive before observation; reuse it to assert the durable row's values and unchanged artifact/queue/lifecycle.
- `newDeliveryEvidenceRouterFixture` (`cmd/server/delivery_evidence_integration_test.go:57`) runs real human decision and task-token packaging and creates an observed PR head. `archive` (`:119`) independently checks tar contents and its SHA against committed bytes.
- Router tests cover legacy/password JWT and PAT, task/forged/missing credentials, post-auth membership/password/PAT revocation. The PAT positive control observes the real last_used write before the one evaluation (`:239`), rather than retrying a contention error into green.
- Handler observation tests cover original distinct principal revocation (`:288`), writer-first/read-first locking (`:148`, `:202`), altered receipts and true commit failures (`:378`). Durable tests should assert both the immutable saved row and current applicability after the same changes.
- `cmd/migrate/comment_delegation_test.go` uses actual per-test schemas, real migration runner, absent/invalid index tests, nonempty rollback refusal and concurrent writer rejection. `internal/migrations/comment_delegation_test.go` provides the complementary static no-FK/single-concurrent-index contract.

Required new matrix: default off/no write; policy role denial and role-revocation race; monotone append and stale CAS; no owner/project cross-scope lookup; real pass row/actual tar match; same key recovery after process restart and same user with new credential; changed request conflict; concurrent duplicate at most one row; before/after commit faults; policy off/change/ABA revision invalidation; issue/project change and move/delete; old/ambiguous/missing PR; expiry; current original principal revocation even when versions are stale; saved unknown/hold never promoted; original stateless observer still writes no ledger; old client completion payload unaffected. Fresh owned handler/router/migration databases, canonical external TMPDIR, normal/race, build/vet, sqlc idempotence, retained failed logs and final hash freeze remain required. No provider/model call is needed or authorized.

### All completion paths remain deferred

| Path | Concrete owner that a future enforcement increment must cover |
| --- | --- |
| Ordinary update | `handler/issue.go:3925` -> `updateIssueAtomically` (`:3571`) -> `updateIssueAtomicallyOnce` (`:3771`). |
| Batch update | `handler/issue.go:4828`; each issue reaches the same atomic update at `:5118`; existing partial-update/response behavior must be considered. |
| Create as completed | `handler/issue.go:2990` builds an arbitrary resolved target status and calls `IssueService.Create` (`:3194`); `service/issue.go:238`, `:382`, `:538` own transaction/INSERT. Quick-create has a separate entry at `issue.go:2613`. |
| GitHub/VCS completion | `handler/github.go:1898` calls `service.WriteIssueStatus`; `service/issue_status_write.go:26` owns its own locked transaction. It intentionally refuses collaboration issues. |
| Independent collaboration | `service/collaboration_lifecycle.go:340` (`ChangeLifecycle`), `:424` (`prepareCompletion`), `:480`/`:552` terminal mutation; creator/current capabilities and status-category checks are independent of ordinary issue writes. |

Custom statuses are categorized separately (`service/issue_status_write.go:106` and `collaboration_lifecycle.go:61`). A future gate must inspect done category, not just literal `status == "done"`. No edit to one of these writers in this observe-only package should be mistaken for global completion enforcement.

## Related specs and external references

Related internal contracts: `.trellis/spec/server/delivery-evidence-observation.md`, `controlled-artifact-execution.md`, `durable-delegation-receipts.md`, `security-boundaries.md`, `database-test-isolation.md`, `project-execution-squad.md`, and root `CLAUDE.md`. Phase guidance read from `.trellis/workflow.md` and server index. No external APIs, new libraries or external sources were needed; this report is grounded in the checked-out implementation.

## Caveats / Not Found

- Policy management authority is the sole identified unresolved business decision in this research; the coordinator resolved owner/admin-only after the user continued. Do not infer project-lead permission from ordinary editing. Reviewed the coordinator's newly written PRD/design: no additional mandatory user-owned decision was identified; the remaining lock, missing-binding and original-project checks are technical implementation requirements within the authorized scope.
- Policy stage, fixed algorithm, expiry horizon, absence of skip, no hint/enforce and no additional execution engine follow the coordinator's bounded proposal. Arbitrary check catalogs, user-defined lifetimes and exemptions would be new product scope.
- Shared historical read access beyond the original evaluator is deliberately omitted from the minimal namespace proposal. It can be designed later for full enforcement/audit views; knowing another user's key does not grant access.
- No new policy/evaluation implementation existed at the inspected baseline. This is research, not final verification or proof that planned tests pass.
- Existing real-model acceptance/stop budgets and prior failed runs remain unchanged.

## Coordinator resolution after research
User continued after owner/admin-only recommendation. Policy manager is current workspace owner/admin. Deleted project policy read refuses404; surviving issue history can be read as stale after full current artifact authority. Deleted issue/workspace refuses public recovery. Final task design controls these choices.
