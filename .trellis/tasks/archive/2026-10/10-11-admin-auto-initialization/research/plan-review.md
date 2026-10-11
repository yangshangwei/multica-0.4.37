# Research: technical review of initialization plan

- Query: Review the task PRD, design and implementation plan against researched authentication contracts; identify unsafe retries, scope bypasses, compatibility regressions and missing orchestration.
- Scope: internal; planning-artifact review, read-only outside this research file.
- Date: 2026-10-11

## Findings

Reviewed `prd.md`, `design.md`, `implement.md`, `research/auth-contracts.md` and `research/upgrade-integration.md`. The design changed during review; the findings below use the revision that already adds database-wide marker lookup, nullable deployment identity for manual preservation, explicit CLI transaction ownership and the destroyed-history limitation. Context manifests were not opened, preserving research-role isolation.

The core design is viable without a database `prepared` state. **Review outcome: no remaining blocking findings.** Both safety clarifications below were incorporated by the parent and rechecked in the updated artifacts. The P2 items remain implementation/validation checkpoints, not reasons to redesign or delay the planning handoff.

### Resolved P1 — require fresh durability confirmation when reusing an intent file

- Resolution verified: `design.md:100` explicitly requires re-syncing the existing file and parent before commit; `implement.md:70` names the publish-success/directory-sync-failure/retry regression. The failure scenario below is retained as rationale, not an open finding.

- Anchors: `design.md:99` (new-file publication and parent sync), `design.md:106` (read existing file), `design.md:109` (confirm durability), `design.md:120` (reuse file), `implement.md:69–71` (file failure and retry matrix).
- Concrete failure: file write/sync and no-clobber publication succeed, but parent-directory sync fails. The transaction correctly rolls back, leaving a valid visible final file. A retry that validates content, ownership and mode then treats existence as proof of durability can commit the account while the directory entry is still not guaranteed durable. A subsequent host crash can lose the only password file. This is a normal I/O-failure path, not deliberate external destruction.
- Smallest fix: explicitly require validating and syncing the opened existing file plus its parent directory on every precommit reuse. A previous process's visible file does not prove its final directory sync completed. Preserve the same secret and intent. Failure to sync again must prevent the account commit. The existing “confirm saved according to 5.1” language should name this reuse case so the implementation cannot interpret sync as new-files-only.
- Validation: inject a first-attempt failure after successful final publication but before/at directory sync; retry from the valid file. Assert re-sync occurs before DB commit; a second sync failure keeps all provisioning rows absent; a successful retry uses the original password/intent.

### Resolved P1 — resolve missing-file versus second-host success ambiguity

- Resolution verified: `design.md:127` and `:131` require non-success `credentials_unavailable` for a committed pending credential with no matching local file, including another host. `implement.md:65` and `research/upgrade-integration.md:34` carry the same contract. No location registry, recoverable database secret or automatic reset was added.

- Anchors: `design.md:57–58` (receipt and error outcomes), `design.md:121–126` (initial credential, missing file and second-host rows), `implement.md:65`, `:73`.
- Concrete ambiguity: host A commits the restricted administrator and keeps the file; host B points at the same database but a different local deployment directory. On B, `created` marker + initial credential + absent file is indistinguishable from A's file having been deleted. The current marker has no trustworthy file-host/location field. A generic successful `already_initialized`/preserved receipt for B would conflict with the explicitly required missing-file recovery error and could allow an upgrade to claim credential delivery without an available password.
- Smallest fix: use one explicit non-success `initial_credentials_unavailable` (or equivalent) result whenever the account still uses the initial restricted credential and the caller cannot validate its matching local file. Include non-secret initialization/user IDs and direct the operator to the original initialization host/file; only use explicit existing recovery if that file is actually lost. Do not automatically reset or generate a new local password. This needs no new database field or host-registration model.
- Keep the intentional distinct case: if the credential has already advanced through password change/recovery, no initial file is required to avoid reinitialization. Preserve the marker and report the current state without claiming that the stale initial file is current. A disabled/revoked account also must never be revived.
- Validation: exercise a committed restricted account from two separate filesystem roots; the creator reuses the matching file, the other caller returns credentials-unavailable with no new file/credential/audit. Exercise the same absent-file state on the creator after simulated external deletion and assert the same failure contract. After password change, both roots should preserve the completed account without recreating a file.

### P2 — implementation checkpoints, not additional architecture blockers

1. **Direct CLI skip behavior:** `server/cmd/server/platform_admin.go:23` currently rejects every platform-admin command outside password mode. Dispatch the new initializer so its documented disabled-mode/admin-disabled result can occur before DB/file work, while keeping the existing bootstrap guard. Test the actual CLI entry rather than only a service helper. Existing global invalid-configuration checks may still reject malformed configuration; they must never accidentally create a credential while “skipping.”
2. **Retention and manual mutation:** the new preserved-marker insertion must be a no-op when a `created` marker already exists, not an upsert that overwrites initialization provenance. It must share the role transaction so rollback and uncertain commit retain the existing atomicity. The current nullable-deployment-ID clarification correctly avoids requiring new deployment settings from manual bootstrap. Add a test retaining the original created marker across subsequent successful manual grants.
3. **Eligibility coverage:** the parent has intentionally selected preservation of any current platform role, including observers. Include that explicit observer case and a historical bootstrap-operation-only / role-audit-only pair in the service matrix. Do not use `CountEffectiveSuperAdmins` or current-user joins to implement this predicate (`admin.sql:15`, `platform_admin.go:148`).
4. **Marker migrations:** include both terminal states, particularly a suppression-only marker with no new account/audit, in down-migration refusal tests. Follow the existing deterministic `ACCESS EXCLUSIVE NOWAIT` fence rather than an unfenced `EXISTS` guard (`465_platform_admin.down.sql:4`, `admin_rollback_guard_test.go:121`). Add the new table to isolated service fixture cloning (`platform_admin_test.go:92`) so a test does not fall through to shared `public` singleton state.
5. **Forced-change behavior:** retain the chosen server-only same-password rejection under current credential/flag recheck; add a canonical handler test proving hash/version/restriction unchanged on rejection, and one ordinary non-forced same-value control. A localized explicit 400 code can use the existing form error path (`password-error.ts:54`, `password-form.tsx:110`) without new state/API fields. Do not make all password changes use the new policy.
6. **Shell orchestration proof:** record exact test entry points for the new helper and one-off Compose command before the final check. The plan's existing changelog tests provide fixture patterns but do not inherently exercise the new mount, UID/GID, skip, failure-before-success and standalone retry paths. The planned isolated real upgrade rehearsal remains necessary evidence, not something this source review establishes.

### Scope and compatibility decisions already handled

- The revised `design.md:63` prevents an organization-ID change from bypassing an existing marker; merely using a per-organization unique index without that lookup would have been insufficient.
- `design.md:88` preserves bootstrap's existing password-mode-only preconditions by allowing a NULL deployment ID in a preserved marker. `created` records still require full provenance.
- `design.md:104` gives the CLI file/transaction responsibility and keeps the service's borrowed transaction free of plaintext/file I/O. This matches the repository recovery pattern at `platform_password_recovery.go:39`.
- Current/historical admin evidence and a durable terminal marker are distinct from effective-admin counting. Pending/disabled/denylisted administrators remain protected from automatic replacement.
- All account/credential/role/marker/operation/audit changes remain one transaction, with no file written after a successful account commit. Deployment operations with NULL actor IDs cannot rely on the human-only idempotency index (`471_admin_operation_actor_key_unique.up.sql:1`); marker reconciliation supplies the new CLI boundary.
- A dedicated first-time path retains ordinary last-effective-admin checks, complete-session admin authorization and existing break-glass recovery. The plan introduces no public provisioning endpoint or signup-policy change.
- No automatic startup hook or package release is authorized by the planning artifacts. Existing API restarts remain inert for provisioning.

### Files and references

- Task `prd.md`, `design.md`, `implement.md`: approved requirements, proposed state/transaction model and ordered implementation/validation plan.
- Task `research/auth-contracts.md`: detailed source anchors and canonical auth/recovery tests.
- Task `research/upgrade-integration.md`: upgrade/Compose/container delivery evidence and integration ownership.
- `.trellis/spec/server/security-boundaries.md`: session, lock order, audit and rollback guard contracts.
- `.trellis/spec/server/offline-delivery.md`: configuration preservation and upgrade failure boundaries.
- `CLAUDE.md`: SQL/index rules, package/test ownership and verification requirements.
- External references: none; this is a repository-only review. Dependency versions remain those recorded in `auth-contracts.md`.

## Caveats / Not Found

- No implementation or runtime tests were executed. No product code, plan artifact, task state or git data was changed by this review.
- Explicitly unsupported destruction is not a new design blocker: deleting both legacy role and audit/operation evidence before the new marker exists makes “never initialized” unknowable. The revised design already documents this case. Likewise, deleting the only postcommit plaintext file requires controlled recovery; the database hash cannot reconstruct it.
- The two resolved P1 findings tightened durability and result semantics within the chosen design; neither required a new prepared-state protocol, reversible password storage, or wider product scope.
