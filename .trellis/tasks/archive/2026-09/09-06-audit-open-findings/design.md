# Design

Three independent lanes. Each touches one layer and reuses the guard or principal the codebase already has instead of adding a new model.

## 1. Credential minting is a human-only surface

**Boundary.** `server/cmd/server/router.go` plus two handlers in `server/internal/handler/`.

- Router: `POST /api/cli-token` gets `handler.RequireHumanActor`; the `/api/tokens` route group gets `r.Use(handler.RequireHumanActor)` so list, create, renew and revoke are all covered. This is the same middleware `/api/cloud-billing` and the approval decision route already use, and `isMachineCredentialActor` already classifies `task_token` and `cloud_pat` as machine.
- Handlers: `IssueCliToken` and `CreatePersonalAccessToken` repeat the check and 403 before doing anything, mirroring `DecideAgentApproval`. These two are the only calls that create a new credential, so they carry the fail-closed backstop; list, renew and revoke rely on the router gate (`RenewCurrentPersonalAccessToken` already rejects anything that is not a `mul_` token).

**Why the whole group.** Listing exposes token names and prefixes, and revoking is a denial of service against the owner. Both are account-level credential management with no agent use case. The daemon renews with a `mul_` PAT and the CLI exchanges a browser JWT for a PAT; neither is a machine actor under `isMachineCredentialActor`, so both keep working.

**Not in scope.** Scoping JWTs or PATs to their minting actor, changing how task tokens are issued, or reclassifying unknown actor sources. Blocking the exchange closes the laundering chain the audit demonstrated; the downstream guards (`UpdateAgent` ceiling, approval decision) are already correct for genuine human credentials.

## 2. Trigger authorization principal follows the delegation chain

**Boundary.** `server/internal/handler/autopilot.go` only. Dispatch (`service.autopilotAdmitInvoke` → `ResolveAutopilotTriggerPrincipal` → `canMemberInvokeAgent`) is unchanged.

**Mechanism.** A new helper resolves the accountable human for an automation write:

- member actor → the member themselves (today's behavior);
- agent actor → `invokeOriginatorFromRequest`, the task's `originator_user_id`, which is the same human `canInvokeAgent` judges the agent by on every direct assignment;
- no human in the chain → 403 and no write.

`CreateAutopilotTrigger` uses it for `created_by` on both the schedule path and `createWebhookTriggerWithMintedToken`. `created_by` is documented in the code as the immutable authorization principal the run acts as, so this is the one column whose value was wrong.

**What stays as the token's user.** `autopilot.created_by`, `published_by` and the rule-version publisher keep the acting identity (the token's user). `requireAutopilotWrite` evaluates that same identity, so moving resource ownership to the originator would lock an agent out of the autopilot it just created. Config responsibility and run authorization are already separate concepts in this file (MUL-6951); this change only corrects the second.

**Consequences.** An agent-created trigger whose originator cannot invoke the target is still created and every dispatch is refused with `invocation_not_allowed`, the same outcome a human creator without access gets today. Refusing at creation time would be friendlier but would also diverge from the member path; it is a candidate follow-up, not part of this fix. A task with no human originator (autopilot-originated chains) cannot create a trigger, because `ResolveAutopilotTriggerPrincipal` requires a member principal and an inert trigger would be worse than a clear 403.

## 3. A 401 is checked against the credential it answered

**Boundary.** `packages/core/api/client.ts`. The auth store is unchanged.

**Mechanism.** `ApiClient` keeps a monotonic `authEpoch`. Every request path that can call `handleUnauthorized` (`fetchRaw`, `uploadFile`, `publishPluginPackage`) captures the epoch when it builds its headers and passes it back on 401. `handleUnauthorized` returns without side effects when the epoch has moved; otherwise it clears the token, bumps the epoch and notifies as today. The epoch bumps on `setToken` (bearer logins and logouts), on a successful `verifyCode`, `googleLogin` or `deviceLogin` (cookie-mode logins, where the client never sees a token), and on teardown.

**Why an epoch and not token equality.** Cookie mode has no client-side token to compare, and the client is the one component that knows both when the credential changed and which request a response belongs to. Bumping inside the login methods keeps the store free of transport concerns.

**Compatibility.** A boot-time identity probe, a wrong-code login and the MUL-7028 mid-flight rejection all send and receive within one epoch, so their behavior is identical. Desktop's deep link (`loginWithToken` → `setToken` → `getMe` 401) is a same-epoch rejection and still tears down.

## Verification environment

DB-backed Go tests run against `fix_open_findings`, a clone of the audit container's migrated `audit_template` (`127.0.0.1:53656`), through `scripts/go-test-with-agent-cli-guard.sh`. TypeScript checks run from this worktree's filtered `@multica/core` install. The human's development environment on 5432 is not a test target.

## Rollback

Each lane is one commit. Reverting a lane restores the previous behavior with no data to migrate; trigger rows written by agents between deploy and rollback keep the originator as `created_by`, which dispatch already handles.
