# Account collaboration reads, controls and recovery

## 1. Scope / Trigger

Apply to `packages/core/collaboration`, shared collaboration views and the
`/account` route. The current surface handles existing records. Creation,
attachments, resource-authoring forms, full source-page integration and general
AI execution retain their separate product gates.

## 2. Signatures

`CollaborationScope`, `collaborationKeys`, `protectCollaborationRead` and the
draft/operation helpers bind reads and writes to Server, viewer and access epoch.
`AccountWorkbenchPage` is shared by Web and Desktop; platform adapters own routes.
`GET /api/collaboration/issues/{id}/activity` supplies cursor-based activity.
Cancellation and Switch recover by GET with the original request key.

## 3. Contracts

React Query owns issue/workbench/comment/activity/notification data. Persisted
drafts contain client intent and original observations, never a second server
state store. Current and participating lists remain separate; transferring work
does not remove the participant's detail or notification-independent entry.

API clients import pure schemas, not operation modules that call the API. For
example, `client.ts` imports `command-schema.ts`; `command.ts` may call `getApi`.
Importing the latter from the client creates an initialization cycle and can
leave `ApiError` undefined under legitimate partial mocks. Shared schema leaves
may use `api/schema`, but must not import the API barrel/client or request helpers.

Activity has stable IDs and exact string sequence numbers, minimal account
summaries and explicitly projected details. New handoff events retain their
actual `handoff_id` in the same transaction. Older unlinked events stay factual
generic entries; never infer notes or identity from timestamp proximity. Unknown
payload fields and resource/credential data do not leave the projection.

Missing optional history routes on older servers are unavailable, not proof of
lost issue access. A structured object denial does clear protected data. Both
success and error paths reject late responses from an obsolete account/access
epoch. Recoveries started with current access still use the issue guard so a
fresh denial clears its cache. Only already-known access loss uses the
account-only guard and accepts content-free recovery.

Cancelled/Switch redacted receipts contain exactly `redacted`,
`protocol_version`, `issue_id`, `request_id`, `action`, `status`, `code`.
Zero-valued extra execution fields are not the same wire contract. Missing or
malformed receipts keep the original request pending; GET recovery never creates
a replacement write. Refetch current authorized state after a real receipt.

Server `control_ai=true` survives only with current write context and supported
execution data. Stop and waiting cancellation do not require a live grant,
online runtime or open execution gate. Resume submits its observed gate, target,
hold and control versions for server validation. Submission/creation rollout
remains independent. Exact-recipient lookups and edit forms reject stale responses
or baselines without silently replacing user intent.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown execution or absent write context | No inferred control authority |
| Supported participant with closed rollout gate | Stop/R45 remain available; Resume still validates |
| Unknown write response or malformed receipt | Retain original key and pending intent |
| Fresh recovery permission denial | Clear protected issue cache and obsolete drafts |
| Access already denied before recovery | Accept only content-free own receipt |
| Optional history route absent without object denial | Show history unavailable; retain authorized detail |
| Late lookup/read after scope change | Abort/ignore; preserve the current scope |

## 5. Good / Base / Bad Cases

- Good: a cancellation POST commits but its response is lost; reload performs
  the original GET and resolves one operation without a second POST.
- Base: ordinary MCP/autopilot flows keep a usable shared `ApiError` constructor.
- Bad: deleting an unresolved request key, restoring a stale private receipt,
  or exposing an arbitrary activity payload as UI data.

## 6. Tests Required

Core command/Switch/action/comment/control suites own recovery and epoch matrices.
Schema tests own malformed wire shapes and source-only denial. Existing MCP and
autopilot partial-import tests guard API initialization. Shared views verify
wiring, original intent, per-target outcomes and old comment anchors. Web E2E
uses production Next plus actual API for cancellation recovery, mention/comment,
handoff/activity, participating retention, another browser and revocation.
Do not count test-seeded existing records as public creation acceptance.

## 7. Wrong vs Correct

Wrong: import request-bearing cancellation helpers from `ApiClient`, or always
drop the issue scope for GET recovery.

Correct: import the pure schema leaf; retain the issue guard until access loss
is already known, then allow only the exact content-free receipt contract.
