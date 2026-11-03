# Current contracts and reuse map

Research on the current dirty worktree, 2026-11-02. Source evidence, not runtime
acceptance. Native helper attempts ended with disconnection/unavailable model;
this note is primary-agent inspection.

## Server

- collaboration_handoff.go SubmitAcceptance checks current handler, eligible
  creator, open lifecycle and directed status digest, but no nonblank note/basis.
- collaboration_lifecycle.go ChangeLifecycle accepts creator completion from
  any nonterminal status. Generic status options include in_review. Changing the
  contract requires all lifecycle callers/fixtures to provide delivery evidence.
- collaboration_workflow.go handoff owns handler/status/iteration fact/storage;
  finishWith writes exact handoff_id to immutable collaboration_event. Reuse the
  event payload for delivery basis/file associations and exact decision identity.
- collaboration_activity.go already refuses timestamp joins and validates actor/
  purpose/target against exact handoff IDs. Extend factual projections, retain
  generic unavailable audit entries for old payloads.
- collaboration_attachments.go bindCollaborationAttachments validates decoded
  IDs, uploader, intended issue, source and draft, delivers atomically and creates
  a retained issue reference. No new handoff reference kind is necessary.
- issue_handoff purpose constraint admits acceptance and transfer; revision can
  reuse transfer storage with a distinct acceptance_returned event. No result
  table or purpose migration is needed.
- issue_collaboration has no manual policy. ResolveIssueAccess, command batches,
  target/resource writers, previews, Resume and runtime claims/permits must enforce
  the selected task policy. Stop/recovery must remain usable.

## Core

- API methods live in packages/core/api/client.ts, not per-endpoint .ts files.
- collaboration/schema.ts optional directed capabilities default false; a
  malformed critical issue/authority/execution projection makes reads unavailable.
  New evidence must be optional for old servers but strict when present.
- action.ts records revision/lifecycle/participant/account observations once;
  recovery checks the actor's original receipt then retries the unchanged request.
- operation.ts saves before POST, retires only a validated receipt and waits for
  current issue refresh. Its 409 handling currently special-cases stale_revision;
  new fixed refusal receipts must settle without stranding result input.
- comment.ts uses validated trigger preview, exact versions, encrypted drafts and
  attachment IDs. comment-composer.tsx correlates inspected draft/text/parent/
  suppression/files/execution view, but insists on a separate preview click.
  One-action ordinary publish should still await the same preflight and verify
  captured intent; any reported AI target requires explicit confirmation.

## Views/platforms

- Shared CollaborationDetail is used by account and source entries; platform
  adapters own routing. It already handles repeated comment/run anchors and
  restored account-detail scroll; keep these effects when recomposing DOM.
- Workflow and en/zh-Hans collaboration locales have pre-existing source-picker
  hunks. Baseline backups are in /tmp as recorded by worktree-baseline.json.
- Execution panel owns recovery for controls, resource offers and switches;
  pending.execution, selected runs, channels/targets/groups/commands/runs and
  history must prevent an unsafe empty-panel omission.
- Existing memory/knowledge panels own separate leases/permissions/history and
  recovered operations. Disclosure may change; those contracts may not.
- Use the existing collaboration-native/Electron account fixtures; production
  Web is required. make status shows a mismatched API listener and an existing
  development Desktop owned by another context; neither is a task acceptance
  environment. Allocate task-private verification instead of replacing it.
