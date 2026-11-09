# Controlled artifact execution design

Status: revised planning contract, 2026-11-10. This replaces the contradictory accumulated revisions. Implementation and independent acceptance remain pending; plan-review.md is retained unchanged.

## Scope and source facts

One in-process adapter packages operator-staged synthetic files into deterministic uncompressed tar. No deployment, publication, remote push, shell, subprocess, model, production credential access, artifact download API or general host sandbox is included. This child does not complete M1.

Inspected source:
- server/internal/handler/agent_approval.go: generic plan is caller prose; execution is a self-reported note; cancel accepts pending/approved. The workspace queue returns full plan to ordinary members.
- server/pkg/db/queries/agent_approval.sql: status predicates protect individual updates, but there is no durable controlled execution row.
- server/internal/handler/agent.go: canManageAgent admits agent owner or workspace owner/admin. No new auditor role is assumed.
- server/internal/auth/task_capability.go: verified task-token context, current task generation, runtime and originator authority are available. Reporter/reviewer capability ceilings must still deny this operation.
- server/internal/service/resource_lock_unix.go and server/internal/daemon/collaborationrun/provider_bundle_unix.go: confined roots and component-wise no-follow opening patterns already exist. x/sys is already a direct dependency.
- server/cmd/migrate/main.go and the server security-boundaries spec: concurrent-index cleanup registration and maintenance-only multi-file rollback.

## Atomic creation before approval

POST /api/agent-approvals/artifact-package validates the typed specification and atomically creates a pending approval plus ONE unclaimed execution row containing the canonical spec. Generic legacy approvals can never acquire controlled scope later. Claim updates the precreated row; it does not insert another row.

Require a canonical UUID Idempotency-Key. Strict JSON rejects unknown/duplicate keys, trailing documents, missing fields and wrong types. Fields are agent_id, task_id, resource_kind/id, action, staging_id, source_revision, staging_manifest_digest, input_manifest_digest, entries, target_environment and policy_version. No caller actor identity, absolute path, expiry, prose or argv is accepted.

First policy: resource_kind=issue; action=package_release_candidate; environment=local-synthetic; policy=artifact-tar-v1. Full canonical UUIDs only. Both human and agent requests bind an existing dispatched/running task and its actual issue. A human supplies task/agent IDs subject to management authority; agent-supplied IDs must equal its verified token/task. Mismatch is refused, not ignored.

staging_id selects an immutable operator-owned generation below the configured staging root. The trusted staging manifest binds workspace, agent, task/generation, issue, declared revision and input manifest. HTTP callers cannot write this manifest. Verify its bindings at creation and before execution. source_revision accepts lowercase 40/64-hex revision syntax but is explicitly labelled DECLARED, NOT INDEPENDENTLY GIT-VERIFIED in detail and receipt. The trusted manifest proves selected bytes and the operator's provenance statement, not a Git checkout merely inferred from a string.

## Limits and canonical hashes

| Item | artifact-tar-v1 policy |
| --- | --- |
| Request body / canonical full spec | 32 KiB / 24 KiB maximum |
| Entries | 1–64 explicit regular files; no directory enumeration |
| File / total bytes | 8 MiB / 32 MiB; enforce before allocation and during reads |
| Paths | 1–96 ASCII bytes; at most 8 slash-separated components; each component matches [A-Za-z0-9][A-Za-z0-9._-]* |
| Refused paths | Absolute/volume/backslash/NUL, empty/dot/dot-dot components, trailing dot, Windows reserved basenames, exact/case-fold duplicates and file/parent collisions |
| Entry hash / size | 64 lowercase SHA-256 hex; nonnegative integer byte size |
| Admission lifetime | Database created_at + 15 minutes; approval/replay cannot extend it |
| Adapter deadline | 120 seconds from definite claim commit |
| Receipt metadata | 4 KiB server-generated codes/counts; no caller notes, raw exception text or absolute paths |

Canonical bytes: versioned fixed-field-order UTF-8 compact JSON, integer decimal sizes, canonical UUIDs, no HTML escaping, whitespace or trailing newline; entries sorted by unsigned ASCII path bytes. Store canonical bytes and digest; never hash PostgreSQL JSONB text. Timestamp encoding is UTC RFC3339 with exactly six fractional digits. Tests pin literal bytes and SHA-256 vectors.

Each hash uses its distinct ASCII domain prefix followed by newline and canonical bytes:
- input_manifest_digest: multica-artifact-input-v1 + canonical entries.
- staging_manifest_digest: multica-artifact-staging-v1 + canonical trusted manifest, including provenance and input digest.
- request_digest: multica-artifact-request-v1 + normalized request and verified initiating principal; excludes generated IDs/timestamps for creation replay.
- invocation_digest: multica-artifact-invocation-v1 + typed adapter/policy/staging ID/staging digest/input digest/environment/USTAR/header-policy fields. This is a normalized in-process invocation, NOT executable argv.
- spec_digest: multica-artifact-spec-v1 + request bindings, trusted initiator/originator/agent/task/runtime/generation, opaque operator-root configuration identity, invocation digest, creation and expiry times.

Creation idempotency is unique on workspace + initiating actor kind/ID + key. Same key/request_digest returns original IDs/spec/status without renewal, after current read authorization; changed digest returns 409. Handle concurrent insertion with ON CONFLICT DO NOTHING and winner lookup, not a lookup in an aborted transaction. An expired/failed request needs a new key and new human decision, never a reset.

## Approval display and authority

Legacy summary/plan are bounded generated summaries: action, local environment, declared revision, file count/total bytes, expiry, digests and restricted detail endpoint. They contain no manifest filenames, staging names, root identity or output path. This preserves the broad queue response shape without disclosing the private specification.

GET /api/agent-approvals/{approvalId}/artifact-package returns canonical spec and receipt, with no-store, to:
- the currently active initiating human who remains a workspace member with issue access;
- a current human agent owner or workspace owner/admin;
- the originating verified agent task with the exact frozen generation/agent/resource and still-valid credential/invocation authority.

No new auditor or platform-admin bypass is introduced; owner/admin are existing audit readers. Do not expose absolute output paths; only opaque artifact ID/digest. Deleted/missing scope or uncertain authority fails closed.

Reuse POST /{approvalId}/decision. For controlled rows require decision, note and expected_spec_digest only; compare digest with the frozen row, refuse replacement fields and expired admission. Legacy rows retain their existing body. Both route and handler enforce human-only canManageAgent-equivalent authority. A manager reviews full fields through restricted detail; existing UI rendering of those fields is not claimed by this backend child.

At creation, decision and unclaimed execution admission revalidate in the transaction: credential/account state and password session version where applicable; membership; issue/task/agent relation; task status dispatched/running, generation/runtime and originator; unarchived agent; current originator invoke permission; runtime-owner membership; and task capability ceiling. Agent identity comes only from auth.VerifiedTaskTokenFromContext, never human X-Agent-ID/X-Task-ID headers. Deny cloud PAT/header-only impersonation on the new surface. A human creates/claims only with current management and invocation authority. Execution requires autonomy at least Operator; prompt/name/autonomy edits cannot widen the approved scope.

Authorization uses transaction-bound queries and the explicit graph below. There is no universal repository principal/task/issue lock order. The controlled path meets existing inversions with NOWAIT and whole-transaction refusal, not a new blocking wait or automatic retry.

### Source-backed lock graph

| Existing writer | Actual acquisition sequence / conflicting rows |
| --- | --- |
| Workspace deletion | `workspace.go:DeleteWorkspace` takes `LockWorkspaceForDelete` (`workspace FOR UPDATE`), then chat sessions, task owners/tasks, tokens and leaf cleanup including approvals; the workspace lock lasts to commit. |
| Member removal | `workspace_revoke.go:revokeAndRemoveMember` takes workspace KEY SHARE, subscriber advisory lock, then archives agents/cancels tasks/offlines runtimes, removes invocation targets under agent locks, and finally deletes member. |
| Member role change | `workspace.go:UpdateMember` takes member FOR UPDATE through `LockKnowledgeMemberRoleChange`, then writes the role. |
| Password/account revocation | `password.sql:LockPasswordUser` / platform account mutation holds user FOR UPDATE; `auth/password_revoke.go` then revokes PATs/deletes task tokens and updates tasks/runtimes. |
| Task cancellation | `service/task_cancellation.go` locks chat_session before the task update. Token deletion can also independently revoke a credential. |
| Invocation permission change | `agent_invocation_target.sql` locks the agent FOR UPDATE before insert/delete of target rows, including the empty-target case. |
| Existing scoped task authorization | `handler/task_capability.go:validateTaskCapability` locks token, then joined task/agent/runtime/issue/originator+runtime-owner member/user rows, then invocation targets. `task_token.sql` uses NOWAIT on all those locks; the joined SQL does not promise an internal per-table order. |

Creation, human decision and first claim use READ COMMITTED and this sequence, with the following steps retained until commit:

1. Take workspace **FOR KEY SHARE NOWAIT** using `TryLockCollaborationAuthorityWorkspaces` with exactly the bound workspace and require exactly one result. This is the existing deletion fence's reader lock with NOWAIT, not a new advisory namespace. No durable insert or admission update precedes it. If deletion already holds its FOR UPDATE lock, refuse; if deletion committed, report missing scope.
2. For a task-token caller, lock the original verified `task_token` row **FOR SHARE NOWAIT** using `LockTaskCapabilityToken`, then reread its full live hash/ID/user/agent/task/workspace/auth-version/binding fields through the same transaction and compare to verified context. For a human JWT/PAT, derive caller identity from authenticated principal context and defer PAT-row lock to step 5. Hints read before locking only locate rows; none grant authority.
3. Lock the subject graph with the actual `LockTaskCapabilityPrincipals` query: issue **FOR NO KEY UPDATE NOWAIT**; task, agent, runtime, originator member/user and runtime-owner member/user **FOR SHARE NOWAIT**. This joined lock primitive is usable for an ordinary task even though the restricted-profile parser is not. Then lock invocation targets **FOR SHARE NOWAIT**. The agent SHARE lock also conflicts with target insert/delete writers, covering an empty target set. Re-read and compare the frozen task/generation/runtime/issue/agent identities and current permissions after locks; no `api_capability` or ordinary-agent fallback may widen existing route ceilings.
4. Supplement the joined query with all independent human principals: current human requester/decision maker/claimer, human initiating principal, and (at claim) retained human `decided_by`, deduplicated with originator/runtime-owner IDs. Lock their `user` rows in UUID order **FOR SHARE NOWAIT**, then their exact workspace `member` rows ordered by user ID/member ID **FOR SHARE NOWAIT**, requiring one row per expected principal. SHARE, not KEY SHARE, is needed to fence non-key role/disabled-state updates. Require current initiator membership/access; require current decision maker/claimer management as appropriate; at first claim also require the retained approver to remain active and authorized to manage the agent. A revoked approval principal makes the unclaimed request ineligible; its historic decision is not erased.
5. For password-mode credentials, under those user locks lock the presented principal's existing password-credential row **FOR SHARE NOWAIT** and run `CheckPasswordVersion` against the original verified session/version, never the newly fetched version. Existing password writers take user FOR UPDATE first, so they cannot cross this point. For human PAT, add an exact-hash/current-user/nonrevoked/not-expired **FOR SHARE NOWAIT** query (the current `LockKnowledgePAT` waits and must not be called unchanged here). Check JWT/PAT/task-token expiry again against the write-time clock. Do not promote setup/recovery credentials or cache hits to current authority.
6. If the verified credential carries a managed binding, take `TryLockCollaborationAuthorityNamespace` for its existing workspace/daemon key, then binding and installation SHARE NOWAIT locks; lock the matching organization/workspace mapping and organization rows SHARE NOWAIT before rechecking `CheckManagedBinding`'s state/epoch/version/deployment/organization facts. Use the existing namespace key, not a parallel fence. Missing/busy/mismatched rows refuse. Unbound local credentials do not invent installation rows.
7. For decision/claim, lock the existing approval **FOR UPDATE NOWAIT**, then its execution row **FOR UPDATE NOWAIT**. Recheck that IDs, canonical digest, originating principals and `decided_by` equal the lookup hints used for steps 1–6; any change rolls back rather than expanding the lock set or restarting. Creation instead inserts both rows under the same already-locked subject graph. Perform the final approval/state/expiry checks and write before releasing any lock.

The statement-level row locks in steps 2–7 are NOWAIT, including new principal/credential queries. A task/password writer that already holds an inverse row causes immediate refusal and releases every acquired controlled lock; if controlled admission holds the relevant SHARE lock first, that writer cannot commit revocation before the admission commits. No chat-session lock is needed by this operation because it never edits a task/chat; it refuses an intersection with cancellation rather than waiting on it. READ COMMITTED rechecks after acquisition prevent combining pre-revocation reads with post-revocation facts.

Bound cancellation uses workspace fence -> verified caller credential and current caller user/member locks -> current agent SHARE lock/management checks -> approval FOR UPDATE NOWAIT -> execution FOR UPDATE NOWAIT. An agent cancelling its own request additionally takes the live task-token and subject-graph locks above; a human manager may cancel an unclaimed request after the source task becomes terminal, without needing the task's original execution permission. Cancellation still refuses any claimed row. The legacy self-report endpoint uses the same workspace/caller and approval/execution locking for bound detection, then refuses controlled execution; the unbound branch retains existing behavior.

Internal receipt finalization is deliberately different: **approval FOR UPDATE NOWAIT -> execution FOR UPDATE NOWAIT -> nonce/spec/state CAS**, with no workspace, membership, user, credential, task, runtime, agent, chat-session or installation lookup/lock. It records a previously admitted operation, not fresh authority. The retained rows remain sufficient if any live subject was deleted. It cannot call the adapter and does not insert an activity row or publish a workspace event after deletion. Public reads still require current workspace/read authority.

Set a 250 ms transaction-local lock timeout to bound implicit uniqueness/index waits not covered by explicit NOWAIT; statement/transaction deadlines also remain bounded. Lock conflict (`55P03`, including timeout), deadlock or stale hints rolls back the entire preclaim operation and returns a specific 409/busy or conflict response with zero invocation. Missing/revoked authority is 403/404. There is no server-side retry loop, including after a definite rollback. An ambiguous commit is 503/unknown, never retried. A lock/failure during internal finalization leaves the persisted claim unknown and preserves filesystem evidence; this child does not enqueue finalization retries. These are implementation requirements to verify with the actual SQL, not claims that concurrency tests already passed.

## Execution row and transitions

Add agent_artifact_execution with no FK, inline PK or inline unique constraint. Fields:
- execution/approval/workspace IDs; initiating actor kind/ID; originator, agent/task/runtime IDs and generation; issue/resource/action;
- key/request digest, canonical spec bytes/spec digest, declared revision, staging/input/invocation digests, policy/environment/private root configuration identity;
- database creation/expiry times, nullable claimed actor/nonce/claimed_at/started_at/finished_at;
- nullable result_status, bounded result code/metadata, output digest/size/opaque artifact ID, receipt digest/update time.

NULL claimed_at AND NULL result_status means unclaimed. Once claimed, result_status is exactly unknown, failed or succeeded. SQL checks reject other values; readers fail closed if an unrecognized state is encountered.

| Event | Approval | Execution / side effect |
| --- | --- | --- |
| Atomic create | pending | One unclaimed row, zero invocation |
| Human decision | approved/rejected | No spec mutation or invocation |
| Preclaim cancel | cancelled | Retained unclaimed row, no invocation |
| Expiry/revoked authority | Retain decision | Unclaimed but ineligible |
| First admitted claim | Remains approved | CAS unclaimed to unknown + immutable claim nonce, commit before invoking |
| Known refusal with proven no publication/cleanup | Remains approved | CAS unknown to failed |
| Verified result and definite final commit | executed | CAS unknown to succeeded in SAME transaction |
| Crash/deadline/uncertain publication/commit | Retain actual committed state | Caller sees unknown, never another invocation |
| Exact replay | Retain decision | Return existing receipt, zero new invocation |

POST /api/agent-approvals/{approvalId}/artifact-package/execution accepts only expected_spec_digest. Changed digest conflicts even on replay. If already claimed, current read authority may return the original unknown/failed/succeeded receipt without admission renewal; expiry of admission does not expire historical results.

For unclaimed rows lock/revalidate current authority, approved status, matching operator roots and clock_timestamp() < expires_at; compare-and-set unknown and nonce. Only the request receiving DEFINITE claim commit success can call the adapter once. Ambiguous claim commit returns 503/unknown and invokes zero times, even if later lookup finds the claim.

Internal finalization matches execution/spec/nonce and unknown status. Verified success updates receipt AND legacy approved->executed atomically; failure of either rolls both back. Machine-generated receipt facts/digest cover identities, binding, outcome, input/output hashes and timestamps. Failed/unknown never receive the legacy executed marker. Bound cancellation locks both rows and refuses any claimed row. Legacy self-report /execution always rejects controlled rows; unbound approvals retain existing behavior.

A confirmed claim is not a renewable lease. During its 120-second operation the original owner may finalize unknown; no other request/background job/restart may run it. Expiry/revocation or workspace deletion after admission cannot create a second run or erase the admitted result. The already admitted local operation may finish after deletion, with nonce-only internal finalization against the retained approval and execution rows. It does not require the deleted workspace/task/principals to reappear and still changes receipt and legacy executed atomically. No deletion hook starts, repeats or cancels this in-process packager; cancellation/timeout of its execution context still leaves unknown. Revoked callers or a missing workspace cannot read results. Timeout/process loss/ambiguous finalization has no mutation reconciliation path in this child. An authorized fresh read may recover an actually committed original success/failure while scope still exists, never infer success from a file.

Known failed means no published artifact and confirmed cleanup. Uncertain claim/receipt commit, panic, timeout, sync/publication/reopen/cleanup failure stays unknown. HTTP status: 400/422 invalid contract, 403 authority, 404 wrong scope, 409 changed binding/state/collision, 410 admission expiry, 503 disabled adapter/DB uncertainty. Missing schema/index or unknown stored status never falls back to legacy execution.

## Filesystem contract

Operator configuration supplies staging/output roots and opaque configuration identity outside HTTP. Missing config disables the adapter. Linux/macOS use os.Root plus component-wise no-follow openat from the existing x/sys dependency; unsupported platforms have a fail-closed stub. Lstat followed by an ordinary path open is insufficient.

The deployment operator/service OS account is trusted. Callers must have no write access to these roots. Stage/output roots are distinct, non-nested, symlink-free local roots; pin device/inode identities before/after use. Stage generation directories/files are sealed 0500/0400; output root is service-owned 0700. Reject group/world-writable ancestry, except root-owned sticky temporary ancestors in isolated tests. Ownership/mode checks follow existing owned-root patterns. Root/config changes invalidate the old spec. This does not defend against malicious root/operator/same-UID processes or prove absence of transient privileged bind mounts.

Open every parent via a pinned descriptor with O_DIRECTORY|O_NOFOLLOW|O_CLOEXEC, and leaf with O_NOFOLLOW|O_NONBLOCK|O_CLOEXEC. Compare Lstat/opened identities; require expected device, owner/mode, regular file, link count 1 and approved size. Retain/recheck parent and root identities. Refuse symlinks, hardlinks, device boundaries, special files and replacement.

Read exactly each approved byte count plus an extra-byte check, hash it, enforce aggregate limits/deadline, and retain at most 32 MiB as a private snapshot. Recheck opened file and parent/root identity/metadata after capture. The tar writer consumes and hashes those same verified snapshot bytes, never an unchecked reread. No ambient Git, globbing or credential lookup.

After validation, exclusively create execution-UUID directory 0700 under output root; EEXIST is refusal, never resume. Create artifact.partial with exclusive no-follow 0600. Use archive/tar FormatUSTAR, sorted regular files, fixed 0644 mode, UID/GID 0, empty user/group/link names, epoch modification time, no access/change timestamps/PAX/xattrs. Portable 96-byte paths avoid implicit PAX headers. Only manifest files enter the tar; revision/IDs remain in receipt, allowing byte equality across equal inputs.

Close tar, fsync/close file, confined reopen/hash, publish to fixed artifact.tar with no-replace semantics within the newly owned execution directory; sync execution directory and output-root directory; reopen final no-follow regular file and verify identity/link count/hash. Persist success only after all steps. Filesystem and PostgreSQL commits are separate facts.

Before publication, remove only owned partial output/new empty directory, sync cleanup, and record failed only if no publication is certain. Never delete pre-existing collision targets. Uncertain cleanup/publication/sync/reopen/deadline/finalization preserves private evidence and unknown. A crash can leave a partial or final file but never grants a retry or implies success. No automatic artifact/receipt purge is added.

## Migration rollout and rollback order

Allocate migration numbers at implementation:
A table/checks; B execution-ID unique index; C approval-ID unique index; D idempotency composite unique index; E final validation/fence.
B/C/D each contain one CREATE UNIQUE INDEX CONCURRENTLY and register invalid-index cleanup in cmd/migrate/main.go. E validates exact index shape/validity. Creation/claim refuse partial schema; generate sqlc from source queries.

Multi-file rollback requires all API/background writers stopped and adapter config disabled throughout. E is highest and rolls down FIRST: one DO block acquires ACCESS EXCLUSIVE NOWAIT on execution table and refuses ANY rows, including unclaimed/failed/unknown/succeeded. Thus populated rollback stops before uniqueness disappears. Busy writer means refusal, not waiting. D/C/B down are separate single DROP INDEX CONCURRENTLY statements. A down repeats NOWAIT + empty check + DROP TABLE in the SAME DO block.

Per-file locks do not cover the full rollback; E does not replace maintenance. Interrupted rollback leaves admission disabled until schema/indexes are fully restored. Test empty, populated, uncommitted first insert, concurrent claim and partial rollback in private schemas, plus runner ordering proving E refusal precedes index deletion. Never delete evidence to enable rollback. Reverting to older application code with controlled rows is unsupported because old legacy routes lack bound-row guards. No FK/cascade or automatic retention cleanup.

## Retain human decisions through workspace teardown

Change only the approval arm of `server/pkg/db/queries/workspace_delete.sql:DeleteWorkspaceLeafData`: delete workspace approvals **only where NOT EXISTS an agent_artifact_execution row with matching approval_id**. Retain bound approvals in every decision/result state together with the execution rows, including pending/unclaimed, rejected/cancelled, failed/unknown/succeeded. `decided_by`, `decided_at`, decision note/status, canonical-plan digest and final execution marker stay on the retained original approval; there is no second decision-history table. The predicate must match by approval ID regardless of a malformed receipt workspace value, so an integrity problem never causes decision evidence to be purged. Do not add execution receipts to another teardown delete arm.

The existing `DeleteWorkspace` transaction already holds workspace FOR UPDATE before `DeleteWorkspaceLeafData`; retain that order. The new create/decision/claim workspace KEY SHARE NOWAIT fence ensures a controlled create either commits first and is visible to the deletion statement's READ COMMITTED snapshot, or refuses with no rows. There is no late insert between deletion's sweep and final workspace delete. Teardown does not need approval/receipt locks on excluded rows and does not touch output files. Existing retained-collaboration rejection remains unchanged; use a workspace without collaboration records when testing this path.

After workspace deletion, all public artifact detail/list/decision/cancel/create/claim requests fail workspace authority before returning retained data, including requests made by a former owner. Internal finalization still uses only the retained rows and the original claim nonce, as above. Deletion before first claim gives zero invocation; deletion after definite claim cannot erase the human decision or prevent a verified result from atomically setting the retained approval executed. A process that lost its claim owner remains unknown; deletion does not authorize recovery. Deletion after success/failure/unknown preserves both rows and their digests. Unbound legacy approval cleanup and the remainder of workspace teardown remain unchanged.

Expected integration files therefore include `workspace_delete.sql`, generated `workspace_delete.sql.go`, `handler/workspace.go` (only fence/cleanup integration or comments required by this contract), and focused workspace-deletion/approval concurrency tests, in addition to the artifact handler/service/query files. No new workspace-deletion approval flow, FK/cascade, output purge or retention administration API is added.

## Original review closure map

| Blocker | Resolution | Required matrix cases |
| --- | --- | --- |
| 1 Freeze before approval | Atomic create, canonical bytes, digest-aware human decision, idempotency/expiry | 1, 2, 5, 6 |
| 2 Separate states | Precreated row, unknown claim, guarded legacy/cancel, atomic success, bound approval retention through teardown | 3, 4, 6, 9, 10 and D1–D4 |
| 3 Current provenance | Verified context, explicit workspace/token/joined/supplemental-principal graph with NOWAIT, trusted staging | 2, 5, 6 and C1–C4 |
| 4 Filesystem | Owned no-follow descriptors, bounded snapshot, deterministic publication | 7, 8, 9 |
| 5 Migration | Concurrent indexes and highest down fence before index drops | Schema/concurrency tests |
| 6 Reads/recovery | Restricted detail, safe queue plan, no unknown rerun | 2, 6, 9, 10 |

The ten cases and execution gates are in implement.md. This planning revision does not claim an independent PASS.
