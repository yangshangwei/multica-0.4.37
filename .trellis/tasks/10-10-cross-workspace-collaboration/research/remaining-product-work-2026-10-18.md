# Remaining collaboration product work

Updated 2026-10-19 after the account creation/file/resource/realtime increment.
The filename is retained for existing handoff links. The parent remains
`in_progress`; current evidence is in `../verification-product-2026-10-19.md`.

## Implemented and verified in the account workbench

- Existing account lists/details, human handoff/lifecycle/editing foundations,
  comments/history, control and original-key recovery from the previous pass.
- Public human creation API and shared Web/Desktop creation form, explicit
  source/recipient choices, private draft uploads, original upload/create result
  recovery and same-UUID navigation. Default rollout gates remain disabled.
- Controlled file metadata/list/preview/download, revocable native capabilities,
  comment attachment delivery and immutable input references. Removing display
  references through the service preserves issue delivery and other references.
- Whole-draft one-time consumption, current intended-issue metadata, bounded
  byte handling, private-prefix protection and unknown-commit object retention.
- Durable authorized creation refusals. Only correlated HTTP 200 proof clears a
  pending request; retained input/files may then be explicitly resubmitted with
  a new key. Old rejected keys cannot later create another issue.
- Target registration, owner-only consent/revoke, Switch authoring/consent/commit
  forms, and participant management including disabled accounts. Web and actual
  native Electron resource journeys use synthetic non-executable runtimes.
- Account/issue WebSockets, final-send authority checks, retained invalidation
  outbox, runtime transition facts, snapshot cursor handling and fenced Query
  recovery. Web evidence observes the actual account WS and clears an open
  preview/detail on revocation without reload. Polling remains recovery.

## Required product work still absent or incomplete

- Source-page creation entry and shared collaboration routing from ordinary
  source detail/list/board/timeline. External-account summaries/filters and safe
  drag-to-transfer/status behavior are not supplied by the account-only entry.
- Labels/custom-property editors and source project/parent/iteration/stage
  selectors. Current creation defaults status/priority and does not expose initial
  iteration authoring. Existing server field support is not full UI acceptance.
- Comment edit/delete/resolve/unresolve/reactions, replies and issue reactions.
  Comment create, attachment delivery, anchors and history are present; endpoint
  reference-removal tests do not establish the missing editing controls.
- Complete run transcript/usage/resource provenance and input snapshot display.
  Current execution facts expose identities/state/proof/membership, with richer
  recorded content still requiring authorized projections and shared views.
- Every remaining R52 AI-result notification kind and complete trigger preview
  facts for merged batches/continuations. Invalidation facts are not a substitute
  for those persistent notification and presentation semantics.
- Runtime attachment-byte and committed-Git preparation, general provider
  adapters and non-Linux containment. Static Linux CLI/daemon tests do not prove
  user-installed providers or file preparation; normal tests never invoke them.
- P7 limited administrator lifecycle/successor forms and their password/admin
  authorization, transaction and native integration acceptance.
- Full P8/97-criterion acceptance and rollout preparation across all old entry
  points/platforms. Do not enable creation or execution merely because this
  account-workbench increment passes.

## Remaining operational evidence and limits

- Real S3-backed delivery and a distributed Redis outage exercise were not run.
  Adapter/relay contracts and local real-HTTP/WS flows have their stated tests.
- Automated reclamation of unknown-commit encrypted orphans and unconsumed draft
  objects remains follow-up lifecycle work. Do not delete uncertain winners.
- Native evidence uses macOS Electron and production renderer/real HTTP with the
  existing bounded native-service harness. It is not Windows/Linux Desktop or a
  packaged full-Main-startup acceptance claim.
- Already downloaded bytes cannot be remotely recalled. New accesses and cached
  UI projections are reauthorized/cleared under the documented boundaries.
