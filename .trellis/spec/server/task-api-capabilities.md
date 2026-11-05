# Task API capabilities for reporters and reviewers

## 1. Scope / trigger
Fresh server-created Progress Reporter/Code Reviewer/Security Reviewer v2 and Migration Reviewer v3 credentials add a task resource/action ceiling. Existing versions, custom agents and humans retain legacy authority. Template key/version are server provenance; display names, prompts, attached skills and autonomy edits cannot grant or remove this ceiling.

## 2. Signatures
- Migration765: nullable `task_token.api_capability jsonb`; sqlc hides this server-only field from JSON serialization.
- `auth.TaskCapability`: version1/profile/issue_id/project_id/originator_id/runtime_id/claim_generation. Unknown/malformed profiles fail closed.
- `auth.WithVerifiedTaskToken` context set only after real `mat_` verification in both legacy/password modes.
- `Handler.TaskCapabilityGuard` immediately after Auth on each authenticated route group, before wildcard CDN-cookie refresh.
- `TaskService.FinalizeTaskClaim` compares payload scope against locked current task/runtime/principals and inserts the token in the existing transaction.
- `CreateComment` SQL accepts server-only nullable `suppress_dispatch`; stored eligibility is admission eligibility AND NOT suppression.

## 3. Contracts
Reporter may GET the current report and same frozen-project issues/comments/timeline; absent project means current report only. List requires one exact project_id equal to the frozen project; duplicate query keys and encoded route ambiguities are refused. Reviewer may GET only its current issue/comments/timeline. Both may POST a comment to the current issue. All other methods/resources, status writes, attachment binding, unknown paths and credential exchanges are denied.

The current originator must remain a member with invoke authority; runtime owner must remain a member. Disabled users, archived agent, changed issue/project/runtime/claim generation, terminal task, revoked/expired credential and missing required scope refuse access. Never replace runtime-owner identity with originator or borrow a resource creator's authority. Reuse the existing owner/private/public-to predicate; database uncertainty yields503, permission refusal403.

GET handlers read actual data and authority through a repeatable read snapshot; the canonical issue loader enforces allowed scope on the consumed row. Source-context frozen captures stay readable, but live source issue/anchor projections require the same capability scope. Out-of-scope live changes, attachment deltas and author refreshes are withheld as unavailable; same-project reporter projections remain available. Comment writes lock/recheck live token, task, principals, invocation targets and issue, then insert the inert comment in that transaction. This provides a consistent request authorization point; it is not a claim that bytes already delivered can be revoked.

A restricted comment persists `dispatch_eligible=false`, publishes existing suppress_execution metadata and cannot trigger agents/plugins immediately or on completion reconciliation. Do not auto-unresolve a thread or cancel deferred escalations. Service-generated final-output comments and failure recovery respect the immutable role restriction after task-token cleanup. Human notifications and ordinary comment revision/activity remain part of comment delivery. Existing independently configured task lifecycle plugin subscriptions remain platform-authorized observers; comment non-dispatch does not disable those standing subscriptions.

Claiming a restricted role for a non-issue/unattributed task or with external MCP/plugin tools is refused explicitly before launch; do not give a scoped mat_ alongside an unrestricted broker mdt_. Daemon plugin/credential endpoints enforce a task backstop. Clear unproven prior provider-session reuse and sibling summaries; inject an authoritative capability brief overriding generic status/leader-wake instructions. Saved role/skill copies are not silently rewritten.

This is an API/platform grant ceiling, not a shell/filesystem/network sandbox. User-supplied prompt/history/configuration and ambient host credentials are outside that guarantee. Allowed resources may contain their existing resource-specific short-lived attachment URLs; token revocation does not revoke previously issued URLs. Never mint a wildcard CDN cookie for restricted credentials.

## 4. Validation / error matrix
| Condition | Result |
| --- | --- |
| Current valid capability and exact allowed resource/action | Continue through existing authorization |
| New restricted role with missing capability |403; never legacy fallback |
| Legacy role with NULL capability | Existing behavior |
| Changed generation/project/principal/invoke permission or terminal task |403/no write |
| DB lookup/transaction unavailable |503/no fallback |
| Restricted comment with attachment IDs |403/no comment or binding |
| Explicit mention, reply, implicit leader return or replay from restricted comment | Persist comment, no execution; explicit target receipt blocked |
| Configured external tool/non-issue restricted claim | Actionable refusal, no token/launch loop |
| Rollback with capability row or concurrent first mint | Refuse while retaining security column |

## 5. Good / base / bad cases
Good: a project reporter gathers project issues and posts one report comment; an authorized actor controls lifecycle. Base: old saved Reporter v1 keeps its existing behavior. Bad: generic GET access, header-selected profiles, or suppression stored only in request context; all can widen authority or reactivate historical comments.

## 6. Tests required
Real router red/green for reviewer title mutation; actual claim both endpoints plus legacy/password Auth; exact UUID/human identifier and project/no-project list, unknown routes and writes; token lifecycle and current originator revoke; read/project-move snapshot and transactional write recheck; durable explicit/implicit/replay/final-output/plugin suppression; unchanged resolved threads/deferred escalations; tool/backstop and transaction refusal; wildcard-cookie refusal; saved-template controls. Migration tests use private schemas for empty/populated/concurrent down and retain actual named execution evidence. No model behavior claim follows from these tests.

## 7. Wrong vs correct
Wrong: check project once in middleware, then let the handler load a changed row; delete tokens and forget comment dispatch provenance.
Correct: authorize the actual read snapshot/write transaction and keep dispatch eligibility on each persisted comment.

Rollback requires ACCESS EXCLUSIVE NOWAIT, capability existence check and DROP inside the same DO block. Revoke restricted tokens before an operational rollback; reverting server code while keeping such credentials can weaken enforcement. Local validation is not authorization for deployment or rollback.

## Targeted CLI and model verification

Use the compiled Multica CLI with an actual server-claimed task token to prove the
allowed flow. Reporter `issue list --project <full UUID>` resolves without project
inventory; a short project prefix legitimately needs a forbidden inventory read.
Do not expand the capability merely to make prefix lookup pass. Full-router tests
must assert returned IDs/totals across ordinary/open_only, pagination and filter
intersections, with outside-project and no-project sentinels. Vary runtime-owner
authority independently of an unchanged authorized originator.

Real-model trials require explicit authorization and a fixed invocation/time
budget. Freeze expected actions before execution; compare complete keyed maps of
the recorded business fields (add/remove/change), exact comment actor/source task/
dispatch eligibility, decoded real requests, command events and cwd hashes.
Model self-report and a zero process exit alone are not proof. Preserve an initial
weak oracle and add a separate stronger evidence comparison without rerunning the
model. Label guided prompts as guided samples, not broad attack resistance.

Keep business actions and process completion separate. A leader may commit the
correct queued/blocked receipt handling and failed evaluation, then time out
without a final response. Record action evidence and process failure together;
never relabel the complete trial as passed or infer a backend/model root cause
without further evidence. Missing final usage is unknown, not zero cost.
