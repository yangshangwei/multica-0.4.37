# Comment authority design and boundary

## Gap and owning boundary
UpdateComment and DeleteComment accept the runtime owner's owner/admin membership even when Auth has stamped task_token/cloud_pat. The role exception must also require !isMachineCredentialActor(r), using actor_guards.go without changing its classifier. Authorship remains an independent branch. Legitimate human JWT/cookie/mul_ PAT management, task agent self-edit/delete, revision CAS, authorship, source-task provenance and dispatch rules stay intact.

## Write ownership
- server/internal/handler/comment.go: only the two management-role predicates.
- New focused comment authority handler/router tests: direct-call backstop and real NewRouter/Auth matrix with actual locally minted mat_, human JWT/PAT, and local Cloud-verifier fixtures if supported.
- .trellis/spec/server/security-boundaries.md: comment management principal contract.
- This child task and new comment-lane evidence/private data.
No auth-helper mutation planned; coordinate with parent before any shared-file expansion. No generic upload change, migrations, frontend edits or broader refactor.

## Credential flow
Auth strips client actor-source, stamps verified task_token/cloud_pat and user identity; workspace membership and task capability guard precede ordinary comment routes. Handler resolves agent/member authorship separately, then checks current membership plus human credential for the admin exception before content decoding, attachments, task cancellation, revision writes, events or notifications. mdt_ uses daemon-only authentication, not ordinary comment Auth. mul_ remains human-equivalent regardless of the process holding it.

## Original evidence → hypotheses → discriminating tests
Original root (read only): /Volumes/artisan/code/2026/multica-0.4.37. Legacy-02 and password-01 comment-authority probe-result.json retain machine owner/admin edit/delete/attachment-clear/spoof failures.
H1: missing human predicate causes elevation. Same actor/role/comment requests must fail before and pass after the minimal predicate change.
H2: routing/auth stamping alone already prevents elevation. Real mat_ NewRouter requests discriminate this from a synthetic-header-only test.
H3: rejecting machines globally would fix it safely. Agent-self positive controls and legitimate human JWT/PAT controls disprove that shortcut.
H4: generic comment_id upload shares management semantics. Trace upload implementation, owning docs, callers and existing tests separately; retain observations and report unresolved contract rather than expanding scope.

## Compatibility and rollback
No response/schema changes. Denials remain403; inaccessible workspace/comment remains current401/403/404 semantics. Successful mutations retain author and normal revision/conflict semantics. Denial snapshots compare comment, attachments, issue, tasks, notifications and relevant audit/event rows. Event assertions use existing test facilities when available. No new audit schema; existing actor-attributed publish is the available witness. Rollback is the focused diff only; never revert other agents.

## C1 independent-review correction — accepted and implemented by parent

Parent reviewed and accepted this narrow boundary. The implementing subagent was interrupted by automated review; parent proceeds with local defensive implementation and regression verification. No external target or deployed credential is involved.

The prior administrator-role repair and six successful full/focused records prove the narrow role gate; they do not close valid-pair author impersonation. Independent C1 identifies `resolveActor`'s legacy header fallback: it proves only that an agent exists in the workspace and the named task belongs to it. It checks no authenticated relationship and accepts terminal pairs. `auth.AuthenticatedPrincipal.AgentActor` expressly labels these headers deny-only, never authorization.

Minimal proposed change: introduce a local comment-mutation actor resolver in comment.go and use it only in UpdateComment/DeleteComment. Verified `auth.VerifiedTaskTokenFromContext` is the sole source of agent author identity; validate its nonempty agent and matching authenticated user/workspace before using it. Non-task callers resolve to their authenticated member identity, ignoring X-Agent-ID/X-Task-ID. An inconsistent server-stamped task source with no verified task context fails closed, never falls back to its owner's member author. Keep the existing machine-admin denial and shared resolveActor unchanged. The same mutation actor must feed lineage and published event attribution, so human admins with valid spoof headers still publish as themselves and cannot inherit source_task_id.

Contracts: real mat_ self writes remain; human JWT/PAT self-human writes and human owner/admin management remain; legacy Cloud member-author self operations remain tied to verified Cloud owner ID (including with actor hints), with no agent impersonation or admin exception. A non-admin human owning an agent is not that comment's author merely by naming its agent/task: no existing credential binding establishes that delegation. Human admin may manage the agent comment through its own role. Current daemon execution uses issued task credentials; legacy header fallback remains available for unrelated attribution paths, outside this bounded fix.

Regression draft: actual NewRouter HTTP legacy Cloud owner/admin/member and ordinary JWT/PAT both modes, valid unrelated agent owned by another human/runtime, running/terminal task pairs, body/attachment/delete; full persisted snapshots and synchronous bus counts. Positive controls include human self/admin with the same valid headers and Cloud self. Existing actual-claim matrix remains the canonical mat_ positive. Direct handler backstop must reject an unverified pair; the existing autopilot edit-lineage test must provide verified token context to model its documented task editor. No global actor/auth helper mutation.

RED runs before product edits on exclusive comment_spoof_red01. GREEN and fresh full handler/router normal/race follow review. Old evidence and prior accepted narrower outcomes stay unchanged.
