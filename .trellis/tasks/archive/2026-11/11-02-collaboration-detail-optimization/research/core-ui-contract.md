# Core UI contract

Core-owned contract for the shared Views lane. Wire fields match
`server-wire-contract.md`; implementation and tests are in progress.

## Result and policy forms

- `useCollaborationDeliveryForm(scope, "result")` is separate from `"comment"`.
  Its `form.content` is the result, `form.basis` the basis, and
  `form.sourceCommentId` an optional exact comment UUID. `update` accepts those
  fields and existing `selectedAttachmentIds`. Preparing from a comment copies
  its snapshot only on an explicit UI action; it never submits or transfers.
- Creation stores `form.aiPolicy` (`"manual" | "assisted"`), with new forms
  defaulting to assisted. `prepareCollaborationCreation` sends `ai_policy`.
- Result intents retain `deliveryDraftId: form.draftId` for bundle retirement.
  Send `draftId` only when binding fresh uploaded files; reused delivered files
  send `attachmentIds` without their old draft. Optional evidence remains subject
  to the existing protected issue/file validators.

## Action intents

```ts
{action:"submit_acceptance", note:form.content, basis:form.basis,
 expectedStatus, statusOptionsVersion, deliveryDraftId:form.draftId,
 draftId?:string, attachmentIds?:string[], sourceCommentId?:string}
{action:"return_acceptance", submissionId, targetUserId, note,
 expectedStatus, statusOptionsVersion}
{action:"complete", submissionId, status, statusOptionsVersion, note?:string}
{action:"complete", note:form.content, basis:form.basis, status,
 statusOptionsVersion, deliveryDraftId:form.draftId,
 draftId?:string, attachmentIds?:string[], sourceCommentId?:string}
{action:"change_ai_policy", aiPolicy:"manual"|"assisted"}
```

`executeCollaborationAction` retains unknown original intent and only retires an
exact result bundle after a validated successful `delivery_confirmation`.
Known durable refusals retire the operation while preserving the mutable form.
For pending acceptance, `prepareCollaborationAction` captures
`submissionAttachmentIds` internally from the exact current submission. Views
does not send evidence IDs again; recovery compares that original observation.

## Read and presentation exports

- `CollaborationSubmission`, `CollaborationDeliveryDecision`,
  `CollaborationDeliveryHistory` match the exact frozen Server wire shapes.
- `collaborationDeliveryHistoryOptions(client,scope,issueId)` is a protected
  infinite Query using bounded descending pages and AbortSignal.
- `deriveCollaborationDetailState(issue, viewerId)` returns `stage`, `primary`,
  `secondary`, `currentSubmission`, `latestSubmission`, `canAccept`, and
  `canSelfComplete`. Actions are existing `CollaborationAction["action"]` values
  plus `"discuss"`; stage values are `"working"`, `"waiting_information"`,
  `"pending_acceptance"`, `"legacy_review"`, `"completed"`, `"cancelled"`.
- `collaborationHasAIActivity(issue,{pending,selectedRunId,error,history})`
  returns true for meaningful execution facts, selection,
  unknown data, errors or unresolved intent. Policy/gate alone never erase
  history; false requires a supported empty execution snapshot with no recovery.

The UI must still show errors/unavailable history factually and use current
Server capabilities for each rendered action. Parsing absent policy/delivery
keeps them undefined, rather than inventing enforcement or acceptance.
For creator-handler review without a linked submission, an authoritative present
`delivery.current=null` plus complete capability/no information request permits
explicit self-result completion (`canSelfComplete=true`, `canAccept=false`).
Absent summary withholds that affordance; generic status repair remains explicit.

One-click comment capture remains owned by the existing composer correlation;
Core `prepareCollaborationComment` and its validated preview guards stay intact.
