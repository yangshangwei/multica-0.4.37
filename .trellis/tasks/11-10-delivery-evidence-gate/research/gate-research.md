# Research: delivery evidence observation and later enforcement

- Query: locate existing lifecycle validation, completion writers, trusted artifact evidence and current identity checks; select the smallest independently verifiable M1 increment.
- Scope: internal, source inspection only, frozen starting commit a3b722a47.
- Date: 2026-11-10
- Worktree: /Volumes/artisan/code/2026/mc-delivery-evidence-20261110

## Findings

### Existing implementation and gap

No delivery evidence gate or project delivery policy implementation was found in the current product source. The existing lifecycle handoff APIs validate self-reported RCA/rollout/evaluation/governance structures and persist metadata plus comments/follow-up work. They do not independently bind a successful decision to a controlled artifact, current candidate, producer authority or policy version.

- `server/internal/service/lifecycle_handoff.go:9`: `LifecycleDecision` already defines `pass`, `hold`, `unknown`; reuse its vocabulary rather than introduce booleans that collapse missing evidence into false/success.
- `server/internal/service/lifecycle_handoff.go:187`: `EvaluateRolloutEvidence` validates declared rollout evidence.
- `server/internal/service/lifecycle_handoff.go:245`: `ValidateAgentEvaluation` validates submitted evaluation cases.
- `server/internal/service/lifecycle_handoff.go:322`: `ValidateGovernanceEvidence` maps required signals to pass/hold/unknown.
- `server/internal/handler/lifecycle_handoff.go:126`: `CreateLifecycleHandoff` parses a kind, computes the decision, then calls `persistLifecycleHandoff`; this is a write path that may also create follow-up work.
- `server/internal/handler/lifecycle_handoff_transaction.go:192`: `persistLifecycleHandoff` re-resolves current data with bounded retries before an atomic metadata/comment/follow-up transaction.
- `server/pkg/db/queries/lifecycle.sql`: workspace and issue/agent/task locks plus `SetLifecycleMetadata` exist. They are useful patterns, not a trusted evidence database.

### Project context and policy storage

There is no project policy field or endpoint suitable for reusing as delivery policy storage. `db.Project` (`server/pkg/db/generated/models.go:2152`) has identity, status, revision, description revision, dates, lead and execution squad fields. `ProjectResponse` (`handler/project.go:31`) mirrors those and statistics. Existing project acceptance is a human-authored progress record and must not silently acquire machine-authoritative meaning.

- `server/internal/handler/project_update.go:227`: acceptance requires a conclusion, scope and revision binding, but can use an explanation rather than independently verified evidence.
- `server/internal/handler/project_update.go:539`: project update previews hash actor, draft, evidence versions, recipients and description revision; useful CAS precedent.
- `server/pkg/db/queries/project.sql:92`: `LockProjectForAssociationNowait` locks project `FOR SHARE NOWAIT`, suitable for observing an issue's current project while avoiding the project-delete versus issue-write lock inversion.
- `server/internal/handler/project_health.go:98`: project acceptance explicitly states whether its description revision is current; a delivery observation likewise must state its exact observed revision and lack of future authority.

**Do not use arbitrary issue metadata as trusted policy or SHA source.** `handler/issue_metadata.go:136` permits ordinary issue actors to set primitive metadata keys; its sibling delete path can remove keys. Lifecycle records are also part of that metadata. If an advisory summary is ever stored there, a later check must recompute authority from protected facts instead of trusting the summary.

### Completion writers that an enforcing gate must cover

An enforcing `done` guard is substantially wider than a new observation endpoint:

| Writer | Relevant symbol/path | Implication |
| --- | --- | --- |
| Ordinary single update | `handler/issue.go:3770` `updateIssueAtomicallyOnce`, actual `qtx.UpdateIssue` at 3885 | Guard after fresh row lock and before mutation, not only outside transaction |
| Batch update | `handler/issue.go:4828` `BatchUpdateIssues`, calls the atomic helper near 5115 | Decide honest partial failure behavior; current batch can skip some failures and return an updated count |
| Move | `handler/issue_move.go:141` delegates to `UpdateIssue` | Moving project and marking done in one request must not bypass the source/destination policy |
| Iteration assignment | `handler/issue.go:3955` early dedicated path; atomic helper also branches on current_iteration_id | Must inspect whether combined status changes are accepted before claiming coverage |
| Issue creation | `handler/issue.go:2990`, `service/issue.go:538`, onboarding direct inserts | A created-in-done issue is not a transition through update |
| PR merge auto-done | `handler/github.go:1898` `advanceIssueToDone` → `service.WriteIssueStatus` | This system writer has no caller-supplied evidence token |
| Shared status writer | `service/issue_status_write.go:26` `WriteIssueStatus` | Owns RC transaction; locks workspace, status catalog, iteration and issue; system policy runs after lock |
| Collaboration lifecycle | `service/collaboration_lifecycle.go:552` | Separate creator/target state machine, not ordinary UpdateIssue |
| Collaboration workflow | `service/collaboration_workflow.go:226` | Separate workflow transaction and direct status write |

Custom statuses can inherit category `done`; compare canonical category using `service.IssueStatusCategory` / `issuestatus` rather than literal key. Project removal, project deletion, source revision changes, already-done edits, reopening, policy version changes, and simultaneous evidence expiry must receive explicit semantics before broad enforcement. Existing human acceptance and collaboration workflow should not change accidentally in this first increment.

### Controlled artifact source of truth

The previous child already supplies a protected fact source:

- `server/migrations/766_agent_artifact_execution.up.sql`: one retained execution, exact frozen spec and policy, initiator/originator/agent/task/runtime/generation, declared source SHA, input manifest digest, admission expiry, claim identity, result status, receipt, output digest and artifact ID. No new execution is needed for observation.
- `server/pkg/db/queries/agent_artifact_execution.sql`: `GetAgentArtifactExecutionByApproval`, `LockAgentArtifactExecutionByApproval`, `ArtifactExecutionDatabaseTime`, existing live user/member/password/PAT locks.
- `server/internal/handler/agent_artifact.go:387` `artifactSpec`: verifies canonical specification digest and every denormalized binding against original canonical bytes; reuse this rather than reading only result_status.
- `server/internal/handler/agent_artifact.go:724` `finalizeArtifact`: writes terminal receipt and executed approval atomically; verified success has output digest/id/size. Claim/unknown is explicitly not success.
- Receipt at `agent_artifact.go:748` contains finished time, execution/approval/spec IDs, claimant kind/id, issue, action, declared revision, input digest, environment, artifact policy, result/code, artifact id/output digest/bytes and claim time.

`receipt_digest` hashes the original marshaled receipt bytes. The stored column is JSONB, whose textual rendering is not the original byte stream. Never recompute this hash directly over PostgreSQL's JSONB output. Validate structured receipt fields against locked execution/approval facts; any stronger independent hash check needs the existing exact canonical encoding contract or retained original bytes.

Admission expiry is 15 minutes after creation and governs permission to begin packaging. It is **not** an existing artifact integrity lifetime. A first observation policy may conservatively use that instant as its evaluation horizon, but must name and document that policy. Passing this horizon should not mutate or reinterpret the historical succeeded artifact receipt.

### Current authority and lock reuse

- `agent_artifact.go:62` `artifactPrincipal`: only middleware-verified JWT/PAT/task identities; rejects legacy actor headers, restricted tokens, agent actor spoofing.
- `agent_artifact.go:79` `artifactTx`: bounded lock timeout plus schema readiness.
- `agent_artifact.go:102` `artifactFence`: live workspace teardown fence.
- `agent_artifact.go:114` `artifactCredential`: live original credential, session expiry/version, task binding and managed installation fences.
- `agent_artifact.go:188` `artifactUsers`: sorted SHARE locks on active users/members, including role changes.
- `agent_artifact.go:230` `artifactManager`: current agent owner or workspace owner/admin.
- `agent_artifact.go:236` `artifactAuthority`: existing full admission vs retained audit-read distinction.
- `server/pkg/db/queries/task_token.sql:16` `LockTaskCapabilityPrincipals`: locks task/agent/runtime/issue/originator/runtime-owner in one query with NOWAIT.
- `task_token.sql:31` `LockTaskCapabilityInvocationTargets`: stabilizes current invocation grants.

**Important reuse limit:** `artifactAuthority(full=true)` requires a running/dispatched task. `full=false` permits human reads after producer completion, but does not check original originator/runtime-owner authority and invocation permissions in that human case. `GetArtifactPackage` passes no approver and is an audit read, not proof of current delivery authority. An evaluation green therefore needs additional targeted current-producer/approver checks; do not claim full current authority by simply calling the audit reader.

A narrow evaluation-only authority helper can reuse the original locks and allow terminal producer tasks while requiring exact original task/agent/runtime/generation/issue bindings, active originator/runtime owner membership, nonarchived agent, current original invocation permission, active approver management authority, and current authenticated observer credential. Alternatively intentionally restrict the first observation to active tasks and state the restriction explicitly. Do not expand old artifact read/execution semantics as an accidental side effect.

Use fresh RC reads after locks. Acquire workspace first, original task authority, project (NOWAIT if issue already held), caller/original principals and approval then execution. NOWAIT prevents reverse graph waits. Read database time after locks near verdict selection. A post-observation revocation cannot invalidate the statement that a snapshot was true previously; response must not be a reusable authorization token.

### Source SHA limitations

There is no universal trusted current repository SHA on an ordinary issue or project. The local artifact adapter explicitly treats `source_revision` as operator-declared, not independently Git verified.

`server/pkg/db/queries/github.sql:193` `GetIssueLatestLinkedPRHeadSHA` selects the latest non-reference GitHub/self-hosted VCS head (open/draft preferred). `agent_task_queue.context.head_sha` is an enqueue-time snapshot used for review deduplication. Neither represents a universal repository checkout HEAD; PR selection can change and mirrored data can lag.

For this increment, require exact selected `expected_source_revision` + spec digest/output digest and current issue/project revisions, and advertise `declared_not_independently_git_verified`. A mismatch proves the artifact is for a different **selected candidate**, not that it is older than an independently observed repository HEAD. A client deliberately selecting the old declared SHA may still obtain a coherent historical observation. Future enforcement must introduce an authoritative current candidate identity / PR version binding before making the stronger claim.

## Recommended smallest deliverable

Add a new read-only `POST /api/issues/{id}/delivery-evidence/evaluate` (route spelling coordinator-owned) with a fixed `delivery-artifact-observe-v1` policy. It computes a snapshot; no status write, queue action, approval, packaging, arbitrary producer-supplied verdict, UI or default policy change.

Request contract: canonical IDs, exact current issue revision and optional current project/revision pair, selected source revision, controlled artifact approval/spec/output binding. Strict unknown/duplicate/null/case-alias/trailing JSON rejection is recommended to match the evidence boundary. If missing evidence is a supported business input, return unknown rather than pretend a missing request field is a successful check.

Response contract: explicit `mode=observe`, `enforced=false`, `authorizes_transition=false`; `pass/hold/unknown`, stable reason codes, fixed policy, observed issue/project revisions, original producer/approver/claimant IDs, spec/receipt/input/output digests, observed time and policy horizon. Return `Cache-Control: no-store`. No current root directory names or filenames should leak beyond existing artifact detail reader authority.

Interpret pass as “the selected controlled artifact's stored success and current required authority were verified for this exact snapshot.” Never label it “ready to deploy,” “merge authorized,” or “current repository HEAD verified.” Missing/unclaimed/unknown evidence stays unknown; mismatched candidate/spec/output, failed/rejected evidence and expired horizon cannot pass; denied current observer identity is an authorization error with no evidence disclosure. Current revoked producer/approver cannot produce green.

No schema is necessary for this read-only first slice; the protected artifact receipt already persists provenance/result. Durable independent delivery evaluations and project-configurable policies remain a later child. Reuse `LifecycleDecision`, not lifecycle metadata persistence or arbitrary project acceptance records.

## Acceptance evidence to require

1. Real artifact Create → Decide → Execute produces exactly one archive and a succeeded receipt; evaluate that receipt through real auth middleware.
2. Matrices for wrong selected SHA, spec/output digest, issue/project identity and revisions; missing, pending, unknown, failed, rejected, expired evidence; malformed requests.
3. Independent current observer, originator, runtime owner and approver revocation; managed/password/PAT/task identity as relevant. Do not use fabricated actor headers as auth evidence.
4. Held-lock races in both orders for issue/project mutation, artifact finalization and authority changes: no pre-lock stale pass, bounded refusal on contention, coherent snapshot after release.
5. Evaluate after producer task completed if contract supports it, without changing artifact execution replay behavior.
6. Unconfigured packager should not prevent read-only evaluation of already-retained evidence unless intentional; evaluation must never call Prepare or Pack.
7. Legacy issue update, project response and artifact endpoints retain old behavior; an observation result does not enable or prevent done through existing clients.
8. Owned database, normal/race runs, freeze source identity, count actual execution/receipt and preserve all failed attempts. No real model calls.

## Related specs and external references

- `.trellis/spec/server/controlled-artifact-execution.md`: frozen artifact and unknown/retention contracts.
- `.trellis/spec/server/task-api-capabilities.md`: task token scope/current authority.
- `.trellis/spec/server/database-test-isolation.md`: owned test databases.
- `.trellis/tasks/11-05-agent-squad-rnd-improvements/design.md`: observe → prompt → block staged delivery policy; pass/hold/unknown and commit/artifact binding.
- No external API or dependency is needed; no web research was performed.

## Caveats / Not Found

- This research is static code inspection, not execution or exhaustive proof of every issue writer. The listed status write call sites are enough to show why an enforcing gate cannot be claimed from an endpoint-only change.
- No project policy storage, authoritative standalone candidate SHA, or immutable delivery-evaluation record exists in the inspected baseline.
- Observation-only is a deliberately limited first M1 increment; it does not finish the overall evidence gate or parent roadmap.
- No product/spec files or git state were changed by this research.
