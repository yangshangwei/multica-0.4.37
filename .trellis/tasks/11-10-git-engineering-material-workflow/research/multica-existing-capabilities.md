# Multica integration evidence

Date: 2026-11-10. Scope: current local checkout, including pre-existing uncommitted changes. This is source/test inspection, not a claim that the inspected features are deployed or enabled. No product code was changed, no tests or real agents were executed, and no external system was modified.

## 1. Existing Git integration mirrors PR facts; it is not a Git material protocol

`HandleGitHubWebhook` verifies HMAC and handles installation, pull request, and CI events. Other event kinds are acknowledged and ignored; this entry point does not consume `push` or GitHub `issues` events to import repository documents or issue definitions. CI events trigger an authenticated snapshot refresh. The integration documentation explicitly describes read-only GitHub permissions, not commit/PR-writing authority.

- Source: [github.go:1052](../../../../server/internal/handler/github.go#L1052), especially event switch at line 1076.
- Product contract: [github-integration.mdx:8](../../../../apps/docs/content/docs/github-integration.mdx#L8).
- Self-hosted provider boundary: [vcs_webhook.go:86](../../../../server/internal/handler/vcs_webhook.go#L86); adapters live in `server/internal/integrations/vcs/` (GitLab and Forgejo/Gitea). The current provider path is unavailable on deployments where `isVCSAvailable()` is false.

Implication: the existing connection is reusable for PR associations and status observations, but importing requirements/design/task/bug files, writing changes back through PRs, and managing Git write credentials need explicit contracts. This finding does not assert that no other code can write Git; daemon/CLI execution is a separate path.

## 2. Project resources already bind work to repositories, but are not material identities

Project resources support `github_repo` with URL/ref and `local_directory` with daemon/path/execution mode. Despite the legacy type name, the URL validator accepts valid HTTP(S) or SSH Git URLs. A project can have multiple resource rows. At claim time, project repository resources override workspace repositories; absent/stale project context can fall back to workspace repositories. A database read failure is propagated, not silently treated as no project.

- [project_resource.go:74](../../../../server/internal/handler/project_resource.go#L74): known resource types and URL/ref normalization.
- [project_resource.go:950](../../../../server/internal/handler/project_resource.go#L950): fallback and error contracts.
- [project_resource.go:1017](../../../../server/internal/handler/project_resource.go#L1017): resource-to-claim mapping.
- [project.ts:95](../../../../packages/core/types/project.ts#L95): public resource shapes.

Implication: reuse the project/resource association, but separately identify each engineering object by repository identity, stable object ID, path and content version. A repository URL or branch is insufficient to identify a reviewed requirement. An approved execution must also decide whether existing workspace fallback is acceptable; explicit material binding should not quietly drift to another repository.

## 3. A business Issue and an Agent run are separate objects

`Issue` stores business title/description/status, assignee, parent, acceptance criteria, context references, project and revision. `AgentTaskQueue` stores one execution's issue association, agent/runtime/session, attempt, parent, retry/rerun lineage and result. The completion service states and implements that completing an Agent task does not change the Issue status; the agent manages Issue status through the CLI.

- [models.go:1558](../../../../server/pkg/db/generated/models.go#L1558): Issue fields.
- [models.go:280](../../../../server/pkg/db/generated/models.go#L280): AgentTaskQueue and run lineage.
- [task.go:4407](../../../../server/internal/service/task.go#L4407): `CompleteTask` explicitly separates run completion from Issue status.

Implication: a task definition checked into Git should map to a durable business object and its versions, not overwrite a transient run record. A single approved definition may generate multiple executions; each needs its own provenance. “Process exited”, “PR merged” and “business acceptance satisfied” must remain separate claims.

## 4. Daemon worktrees are usable execution infrastructure, with a material version caveat

Local-directory `in_place` mode serializes work in the user's directory; `worktree` mode gives tasks isolated worktrees and durable conversation branches. Ownership metadata, not a branch name alone, determines continuation. The implementation snapshots and replays tracked and untracked user edits, preserving the user's working tree and retaining agent work on a branch.

- [project.ts:108](../../../../packages/core/types/project.ts#L108): execution mode semantics.
- [local_worktree.go:20](../../../../server/internal/daemon/execenv/local_worktree.go#L20): snapshot, isolation and preservation guarantees.
- [local_worktree.go:62](../../../../server/internal/daemon/execenv/local_worktree.go#L62): private Git index and retained snapshot refs.
- Test examples: [local_worktree_test.go:107](../../../../server/internal/daemon/execenv/local_worktree_test.go#L107), line 321 (concurrent tasks), line 897 (conflicting later user edits), line 1244 (refuse an unowned branch).

Implication: approved source SHA alone may not describe actual input when uncommitted edits are replayed. A trusted input manifest must either bind the exact captured bytes or require a deliberately chosen clean-source mode. Do not equate worktree isolation with a security sandbox.

## 5. The plugin framework is concrete but has a bounded surface and deployment gate

The host enables `issue_panel` and `modal`, not `sidebar_panel`. It supports UI/manual/event/agent/schedule triggers, HTTP/MCP transports and skill resources. Agent hooks are offered as tools that an agent chooses to invoke; they are not mandatory before/after-run interceptors. `plugins_v1` defaults false, and user-facing discovery/management availability is separate from already-pinned running task manifests.

- [capabilities.go:28](../../../../server/pkg/plugincontract/capabilities.go#L28): supported slots, triggers, transports and skill resources.
- [plugin_event_dispatch.go:24](../../../../server/internal/service/plugin_event_dispatch.go#L24): explicit absence of mandatory agent lifecycle hooks.
- [keys.go:23](../../../../server/internal/featureflags/keys.go#L23), [keys.go:59](../../../../server/internal/featureflags/keys.go#L59): feature gate and false default.
- [README.md:18](../../../../packages/plugin-sdk/README.md#L18): sandboxed iframe, scopes and immutable installed versions.

Implication: a review UI and bounded integrations can fit a plugin. The existing plugin framework does not by itself provide an enforceable full lifecycle engine. Deployment availability must be verified rather than inferred from these source files.

## 6. Plugin Action API permissions and delivery semantics limit autonomous orchestration

The registered Action API exposes context, read/patch Issue, comments and storage. Issue patch is explicitly limited to title/description, with expected-revision support; it does not expose assignment, status, hierarchy or Issue creation. Plugin comments intentionally do not trigger Agent execution through mentions. Thus a plugin cannot obtain the full requested decomposition/dispatch workflow merely by using the existing SDK.

- [router.go:104](../../../../server/cmd/server/router.go#L104): complete registered Action API route set.
- [plugin_action.go:376](../../../../server/internal/handler/plugin_action.go#L376): content-only update and reasons.
- [plugin_action.go:408](../../../../server/internal/handler/plugin_action.go#L408): expected revision and conflict handling.
- [plugin_action.go:655](../../../../server/internal/handler/plugin_action.go#L655): comment dispatch boundary.
- [index.ts:209](../../../../packages/plugin-sdk/index.ts#L209): SDK Issue methods.

Event delivery is also intentionally best effort: a 512-entry in-memory queue drops and counts overflow, rather than retaining a durable log. Collaboration Issue events are withheld by the dispatcher. This path therefore cannot be assumed to deliver every material transition or cover every Issue kind.

- [plugin_event_dispatch.go:29](../../../../server/internal/service/plugin_event_dispatch.go#L29), [plugin_event_dispatch.go:148](../../../../server/internal/service/plugin_event_dispatch.go#L148): queue and drop behavior.
- [plugin_event_dispatch.go:234](../../../../server/internal/service/plugin_event_dispatch.go#L234): collaboration filtering.
- [plugin_event_dispatch_test.go:70](../../../../server/internal/service/plugin_event_dispatch_test.go#L70): test deliberately asserts overflow is dropped.

Implication: propose a specifically authorized server operation for material import and dispatch, and a reconciliation/replay contract. A notification hook is useful to prompt reconciliation; it cannot be the sole authority for truth or an execution gate. Reusing broad human credentials to sidestep the plugin boundary is not a substitute for defining the intended permission.

## 7. Current PR-to-Done behavior is not a comprehensive delivery gate

`advanceIssueToDone` rechecks that the Issue is not terminal and that the combined GitHub/VCS association has no open/draft working PR and at least one merged PR with closing intent. That predicate does not check CI, requirements/design approval, test evidence or artifact acceptance. This is current behavior, not a bug claim against the documented feature.

- [github.go:1657](../../../../server/internal/handler/github.go#L1657): documented aggregation rule.
- [github.go:1897](../../../../server/internal/handler/github.go#L1897): actual transactional completion predicate; line 1910 contains the final check.
- [github_test.go:393](../../../../server/internal/handler/github_test.go#L393): merged PR progression fixture.
- [github.sql:132](../../../../server/pkg/db/queries/github.sql#L132): displayed CI checks use the current stored snapshot head; display is separate from completion.

Implication: a new “requirements accepted → implementation verified → done” contract must decide how every existing terminal writer participates, including this webhook path. Showing evidence in a plugin panel cannot enforce that contract by itself.

## 8. Approval/evidence foundations exist, with sharply bounded guarantees

Generic Agent approval records a human-authorized action and an execution note. Its source explicitly says it is not a host command sandbox, and the legacy execution-record endpoint is agent/human reported. The controlled artifact path is stronger: exact frozen specification/digests, human approval and one bounded packaging invocation. Its concrete action is local synthetic tar packaging, not general deployment or arbitrary workflow approval.

- [agent_approval.go:22](../../../../server/internal/handler/agent_approval.go#L22): stated generic approval boundary.
- [agent_approval.go:432](../../../../server/internal/handler/agent_approval.go#L432): execution-note recording; controlled approvals are diverted at line 448.
- [spec.go:19](../../../../server/internal/artifact/spec.go#L19): `package_release_candidate`, `local-synthetic`, `artifact-tar-v1`, limits and expiry.
- [agent_artifact.go:583](../../../../server/internal/handler/agent_artifact.go#L583): digest-bound execution.
- Tests: [agent_artifact_test.go:239](../../../../server/internal/handler/agent_artifact_test.go#L239) (one physical invocation), line 274 (unknown never retries), line 598 (approver revocation).

The delivery-evidence endpoint is expressly observe-only: it binds current Issue/project/producer facts, a controlled receipt and a database-observed PR head. It neither changes Issue status nor creates a reusable permission to complete/publish, and does not remotely verify Git HEAD. A pass is a bounded observation, not comprehensive acceptance.

- [delivery_evidence.go:113](../../../../server/internal/handler/delivery_evidence.go#L113): observe-only contract.
- [delivery_evidence.go:151](../../../../server/internal/handler/delivery_evidence.go#L151): `database_observed_pr_head_not_remote_verified` provenance.
- [delivery_evidence.go:339](../../../../server/internal/handler/delivery_evidence.go#L339): hold/unknown/pass matrix.
- [delivery_evidence_test.go:58](../../../../server/internal/handler/delivery_evidence_test.go#L58): observation leaves Issue revision and receipt untouched; line 85 tests stale/missing/ambiguous source, mismatched artifacts, expired authority and altered revisions.

Implication: reuse the idea of exact inputs, version binding, current authority and retained uncertainty. Do not advertise the current controlled packaging proof as a general requirement/design/CI acceptance system.

## Bounded design implications, not implementation decisions

The important additional contract is a chain of identified, versioned objects: original request → agreed requirement/acceptance criteria → approved design → business work item → individual Agent run → source candidate → independently produced test/acceptance evidence. Each edge needs an identity, version and author/approver. Git can retain reviewable definitions and evidence references while Multica retains execution state; whether live business status also belongs in Git remains a user decision.

A useful first integration experiment is one repository and one realistic requirement with a mid-run change. Test the existing reuse points above and explicitly exercise stale approval, lost/duplicate/reordered events, conflicting edits and uncertain execution. It should fail safely without creating duplicate business objects, executing outdated plans or falsely declaring Done. That validates a narrow semantic bridge; adding more office-system connectors does not establish those properties.

Unverified here: actual deployment flags, installed plugin runtime, credential grants, external provider webhook state, real Agent behavior, production workflow coverage and measured human time savings. These require separate empirical verification.
