# A2 human API and consumer boundary

This documents implemented A2 code in the isolated knowledge worktree. It does not claim daemon injection, Provider understanding or the G2 rollout gate has passed. F3 owns source invalidation hooks, scan/drainer and relay; C2 owns ordinary task/claim/session/launch integration.

## Human service and HTTP

`CollaborationService` now implements `ProposeKnowledge`, `ApproveKnowledge`, `RejectKnowledge`, `WithdrawKnowledge`, `GetKnowledgeOperation`, `ListKnowledgeCandidates`, `KnowledgeCandidate` and `ListTargetKnowledge`. The new handler methods and nine production registrations follow Core's `api-seam.md` routes.

Source/candidate/action/recovery routes use the authenticated account group and discard ambient workspace headers. Target GETs use normal workspace/project groups and an explicitly captured workspace identity. Responses are no-store. The exact Core DTO contract is implemented, including required `history_truncated`; source pages contain every version in `(proposed_at,id)` order, so versions outside the detail's newest 50 remain reachable.

Every page carries the same first-page `projection_checked_at`, `valid_until` and nullable `next_expiry_at`. The opaque cursor binds viewer, scope, position, upper bound and lease; it cannot renew the lease. Private response leases are at most 30 seconds, no later than relevant expiry or a known PAT expiry. All reads reauthorize in a repeatable-read snapshot; a stale page can never substitute cached authority. Six actual HTTP fixtures were checked against the current Core Zod schemas without editing frontend source.

Source authority is current Read plus creator/current handler, independent of a participant row or open issue status. Proposal target visibility is separate: source workspace and the still-current visible source project are allowed to external managers; other destinations require normal membership visibility. Source authority is checked before destination resolution, so owning a destination cannot reveal whether a hidden source exists.

Confirmation requires current source Read plus target membership and human project-lead or workspace owner/admin authority. Agent lead does not confer human confirmation. Rejection only reduces pending state and therefore retains the same authority requirement without material availability/expiry eligibility. Withdrawal uses current source management or target confirmation authority; target-only withdrawal and its recovery are content-free.

Human writes hold actor and original PAT (when applicable), sorted workspace/member fences, status catalog, project existence/necessary lead authority, independent counter, source and candidate locks. Owner/admin and source-authorized withdrawal paths do not take an unnecessary project-lead SHARE lock; genuine lead authority does. NOWAIT intersections use the existing bounded whole-transaction retry. Unknown commit is never automatically repeated under a replacement key.

Successful create is atomic candidate + original operation + source audit. Invalid/unauthorized new proposals leave no candidate/operation/hint and cannot fabricate a recoverable state. Existing authorized state refusals are retained with the exact original request. Operation kinds are `knowledge_propose|knowledge_approve|knowledge_reject|knowledge_withdraw`, within the unchanged source/actor/request namespace shared with memory/commands. Receipt actions remain `propose|approve|reject|withdraw`; success codes are `knowledge_proposed|knowledge_approved|knowledge_rejected|knowledge_withdrawn`.

Correction creates a new immutable version. Approval atomically supersedes the existing approved version, approves the reviewed candidate, advances the target counter once and enqueues one stable direct hint with cause ID equal to the original operation ID. An older pending version cannot replace a newer approved version. Replaying the operation does none of those writes again. Global target budgets include unreadable-source contributions until withdrawal/expiry; visibility filtering cannot manufacture capacity. Base strings, parent revisions, task input and execution state are unchanged.

## Source availability and credentials

`KnowledgeSourceAvailability(ctx, q, issueID, requiredRefs) (bool, error)` borrows the caller's transaction, opens no nested transaction and grants no authority. Every required reference must retain its exact original source/version/hash. Memory purge and comment deletion suppress derived material. Historical comment text may be reconstructed from its bounded original before/after events; versions are not forced to latest. Recorded run references use the public stable `input:`, `event:` and `record:` identities. Dependency errors are errors, not empty knowledge.

All human source/list/detail/history/target projections apply this availability check before material serialization. Unavailable material omits body, summary, hash, refs, applicability and potentially quoted decision reasons. Target-only rows contain only candidate ID, state/revision and withdrawal capability. Ordinary withdrawal/expiry does not erase otherwise lawful historical text.

`auth.AuthenticatedPrincipal.PATTokenHash` carries only the original verified credential identity and has `json:"-"`. The legacy cache hit, legacy database lookup and password authentication branches all populate it. Knowledge reads/recovery query the original live token, owner, revocation and expiry; writes hold its row SHARE NOWAIT under the actor lock and recheck freshness before completion. Missing provenance fails closed. Read leases clamp to known token expiry.

The shared `auth.ValidateAuthenticatedPAT(ctx,q)` check runs inside `ResolveIssueAccess` before private source content. Reused memory/comment/run-record readers therefore cannot obtain body using only a stale cached PAT identity. It uses the same authorized query snapshot, preserves JWT behavior, and checks password issuing version where relevant. The one existing Kind-only PAT recipient fixture was changed to a real test token. This is not a global Auth cache redesign.

## A/C transaction-bound interface

The manifest/revision service names alias C2's `protocol.TaskKnowledgeManifestEntry`, `TaskKnowledgeManifest` and `TaskKnowledgeTargetRevision`; those protocol files are a coordinated dependency, not A2-owned source. Manifest digest is SHA-256 of canonical Go JSON with entries sorted by candidate ID.

`KnowledgeConsumerFacts` is server-only input: supported surface, actual workspace/project, originator, runtime owner, agent/runtime, task, private chat or collaboration issue, admitted input authors and proven audience. It contains no accountable-user fallback and is never decoded from an HTTP request. C owns construction/revalidation from actual task/runtime/scope rows and the caller's execution credential.

- `ResolveApprovedKnowledge(ctx,q,facts) (KnowledgeResolution,error)` returns eligible bounded contributions, exact manifest/digest, target watermarks and coarse omission reason. It takes no hidden locks and opens no transaction.
- `RevalidateKnowledgeManifest(ctx,q,facts,manifest) (bool,reason,error)` validates those exact immutable exposures without replacing them with the current collection. Target revision is a watermark, not read authority.
- `CollectKnowledgeReadLockHints(ctx,q,facts,manifest *KnowledgeManifest)` reads bounded metadata only. Nil discovers the current potentially available collection; explicit manifests retain exact-history semantics.
- `LockKnowledgeReadTargets(ctx,q,hints)` belongs after C's ordered user/credential/workspace/member fences and before Chat. C must already hold real parent workspace locks. Counter initialization keeps the 25ms conflict bound.
- `LockKnowledgeReadSources(ctx,q,hints)` belongs after Chat and before late runtime/task NOWAIT locks. It does not lock back into parent/user rows.
- `ValidateKnowledgeResolutionLocks(resolution,hints)` is required before disclosure. It binds every returned contribution/manifest entry to the acquired candidate set, including when an unavailable source could reappear. It does not reject an original replay merely because an unrelated target watermark changed.

Fresh discovery omits already-missing/purged reference dependencies so one unavailable retained contribution does not block healthy ones. This is a metadata reduction, never an availability or permission grant. A disappearing source after discovery still returns context-changed rather than silently skipping a lock; explicit old history refuses. C's mandatory consumer issue/workspace fences remain independent.

Workspace-visible output is eligible only within the same source workspace identity; coincidentally equal cross-workspace membership is not proof. Private chat uses its actual creator and current private-agent read gate, plus actual originator/runtime owner source access. External channel delivery or unproven audience is withheld. Collaboration audience checks its real destination and current readers. Current state, expiry and required-source availability are checked independently of target watermarks.

## Generated boundaries and ownership

The two reserved task metadata column names use `go_struct_tag: 'json:"-"'` wildcard overrides. Table-qualified overrides missed an actual CTE result; both real serialization cases and a generated-field inventory now protect that boundary. No generated file was hand edited. Raw SQL `to_jsonb(task)` can bypass Go tags and remains a C2 public-projection audit responsibility.

`review-snapshot.json` separates 27 A2-exclusive files, shared router/generated files, and C2/F3 dependencies. `owned-files.txt` is the exclusive integration allowlist. `checks.json`, expected-test inventory, raw failures, successful reruns and wire fixtures remain under this directory. A2 does not own task/daemon/session mutation files, F3 existing-writer hooks, frontend views or formal task status.
