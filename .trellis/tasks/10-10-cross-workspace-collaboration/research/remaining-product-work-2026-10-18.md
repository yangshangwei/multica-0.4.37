# Workbench continuation: concrete product boundary

This pass completes the current/participating account shell, real activity read,
comment history/link restoration, exact-recipient and edit-baseline fixes,
original-key cancellation recovery, and truthful target/lane feedback. It is not
full P6/P8 acceptance.

## Implemented UI

- Account list/detail independent of workspace membership, selected detail retained
  after handoff, account notification read/archive and unread count.
- Invite, transfer, request/return information, submit acceptance, nonterminal status,
  creator lifecycle actions, leave, and title/body/date/priority editing.
- Human comment composer with issue-scoped mention candidates, named notification
  recipients, explicit suppression and per-target rejection/waiting reasons.
- Supported server human Stop/Resume controls and waiting-input cancellation;
  original requests persist before transport and recover without new identities.
- Activity with exact new handoff provenance and safe before/after edits; older
  unlinked handoff records remain generic. Per-comment history is lazy-loaded.
- Current channel/hold state, original consumed group membership, member/continuation
  proof summaries, Switch proposal inspection and saved Switch request recovery.

## Required product affordances still absent

- Target registration, initial owner consent/revoke, Switch proposal authoring,
  Switch owner consent/revoke and atomic commit forms. Core Switch operations are
  implemented, but the current view only inspects proposals and recovers saved ones.
- Labels/custom-property editors and source project/parent/iteration/stage editors.
  The current edit form only changes title, description, dates and priority.
- Participant management/removal UI. Add participant and Leave are present; there
  is no participant list with creator-only Remove controls.
- Comment edit/delete/resolve/unresolve/reaction controls, replies, and issue
  reactions. Comment display, create, anchors and read history are present.
- Controlled attachment upload/view/download and retained issue attachment list.
  Restricted RichContent intentionally renders linked resources without normal
  workspace unfurls/download paths; this is not attachment acceptance.
- New collaboration creation form and source-page creation entry. Public server
  creation remains unregistered, and no existing-issue conversion is exposed.
- Limited delegated administrator lifecycle/successor UI (P7).
- Source issue detail/list/board/timeline collaboration routing, external-account
  filters and safe drag-to-transfer integration. The shared sidebar account entry
  does not supply those source-page affordances.
- Account/issue WebSocket subscriptions and immediate account event invalidation.
  Current reads refresh on mount, focus, reconnect and the existing 15-second poll.
- Full run transcript/usage/resource provenance and input snapshot display. The
  existing execution DTO exposes IDs/state/proof/membership, not those bodies.
- Server trigger preview still does not provide explicit merged-batch/continuation
  grouping facts. The UI reports supplied target outcomes and does not invent
  merge decisions from names or transient current state.
- Public AI submission/execution capability remains gated; keeping typed controls
  available does not establish squad runtime acceptance or open creation.

## Evidence limits

The Web test uses normal test accounts and a normal source issue, then seeds only
its collaboration rows in the root-owned disposable database because creation is
not public. The cancellation target has no agent/runtime/grant/executable and cannot
launch AI. That test proves existing-record account flows, not AC67 or actual AI
execution. Native Electron evidence belongs to the Desktop lane. Full 97-criterion
task completion remains with the root session.
