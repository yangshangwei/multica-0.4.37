# Collaboration configuration changes and shared skills

## 1. Scope / Trigger

Use this contract for target environment changes, resource-owner consent and
shared skill writes. It extends [collaboration targets](collaboration-targets.md).
The task remains in progress; these boundaries do not enable public creation or
claim complete squad or Web/Desktop acceptance.

## 2. Signatures

- `POST /api/collaboration/issues/{id}/targets/{targetId}/switch-proposals`
- `GET .../switch-proposals` and `GET .../switch-proposals/{proposalId}`
- `POST .../switch-proposals/{proposalId}/consents`
- `POST .../switch-proposals/{proposalId}/consents/{agentId}/revoke`
- `POST .../switch-proposals/{proposalId}/commit`
- `GET /api/collaboration/issues/{id}/switch/operations/{requestId}`
- `GET /api/collaboration/issues/{id}/resources`
- `GET .../resources/options?execution_workspace_id=UUID`
- `GET .../resources/identity?target_id=UUID&agent_id=UUID[&proposal_id=UUID]`
- `GET .../resources/operations/{requestId}` and `GET .../participants`

`CollaborationService` implements `ProposeTargetSwitch`,
`ConsentTargetSwitchResource`, `RevokeTargetSwitchConsent`, `CommitTargetSwitch`
and the authorized proposal/operation readers. The old `runtime-binding` route
is removed. Pending-configuration construction exists only in a Go test fixture.

Migration 709 installs skill/file/binding/plugin-release triggers into the
existing agent config epoch and invalidation outbox. Migrations 713–716 persist
Switch proposals and consent. Each index has its own concurrent migration and
invalid-index cleanup registration. These new migrations reject downgrade.

## 3. Contracts

All Switch writes carry one `Idempotency-Key` plus exact positive decimal
`expected_lifecycle_epoch`, `expected_participant_epoch`, `expected_account_epoch`.
Propose additionally captures the target/config versions and intended resource
roster. Proposing and consenting do not mutate the live target, grant or lane.
Each owner consents only to their own resource and current managed/enrolled
identity; consent freezes their account/participant generations. Current grants
cannot stand in for a proposal's consent.

Commit carries `control`: target/config/hold/control versions, nullable observed
group ID and nonnegative group generation. Under the shared aggregate locks it
rechecks the proposal lifecycle/configuration, current owner consent and runtime
binding, target pause/hold, coherent lane mirrors and every old dependent group's
termination ledger. Only then it advances the target configuration, replaces
grants, moves waiting inputs to the destination tail and leaves `switch_hold`.
The typed target and historical input/group/run/proof identities never change.
The old lane's controls stay unchanged. Explicit Resume is still required.

Proposal views expose resource identifiers and consent status without private
configuration, fingerprints or installation credentials. `grant_id` and
`grant_epoch` are nullable and visible only to the owning viewer after commit,
while the live target still matches that proposal. A later Switch must not expose
its grant through an earlier proposal. GET recovery locks and authorizes the
current account before actor-key lookup. A current controller receives the full
original success/refusal; a departed account receives exactly `redacted:true`,
`protocol_version`, `issue_id`, `request_id`, `action`, `status`, `code`.
Machine credentials, disabled accounts and stale password identities are still
denied. Redacted recovery never restores object access or returns target, proposal,
grant or channel identities. Later consent never changes an old receipt.

The resource form reads a minimal current target roster. Grant IDs and epochs
are owner-only; installation/binding identity has its own current-owner endpoint
and must match a resource in that issue's target or proposal. Candidate discovery
requires the current organizer and an explicitly selected workspace where that
human already has resource visibility. It does not add owners as participants
or reveal an arbitrary workspace's directory. A missing supported enrollment
produces an unavailable identity, not a guessed tuple.

Proposal display may include minimal agent, runtime and owner labels from its
existing roster. These additive labels never expose machine configuration or
grant installation authority. Register/consent/revoke receipts include their
original request ID and action; uncertain form writes recover those identities.
The participant-management reader includes unrevoked disabled accounts with a
minimal `account_active` flag and the same-snapshot collaboration revision.
Keep it distinct from the effective mention list. Only a current creator may
remove another participant; removing the handler still atomically hands back to
the creator. Being visible in the management list grants no execution authority.

Skill name, description, body, supporting-file path/content, origin/release
configuration, binding enablement and plugin version changes affect execution.
Their writer transaction advances every enabled bound agent's config epoch and
persists invalidation. The existing consumer holds exact affected targets and
stops dependent groups without pausing lanes. Original proof identities survive.
Disabled bindings are unaffected by content changes. Pure `config.presentation`
and exact repeated writes preserve consent; unknown config fields are fenced.

`replaceSkillFiles` diffs under the caller's locked skill row and transaction;
unchanged files keep their IDs. `SetAgentSkills` diffs the full desired set and
reenables requested disabled bindings instead of deleting/reinserting everything.
ClawHub records an upstream version when supplied. Plugin release identity lives
on `plugin_installation.package_version_id`, even for byte-identical skill files.
Skill/file writers take skill fences then agent `NOWAIT` locks; binding writers
take the reverse skill fence with `NOWAIT`. Lock conflict rolls back the entire
writer. Migration 709 retires pre-upgrade untracked skill consent once because
the previous epochs cannot prove what content was consented to.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing owner proposal consent | `switch_consent_required`; live state unchanged |
| Proposal, lifecycle, resource or owner generation changed | Stale refusal; no implicit new proposal |
| Target active without hold | `target_pause_required` |
| Target/lane mirror contradiction or retained unproven work | Refuse; preserve barrier and history |
| Old commit response lost | Recover by original key, never manufacture a new intent |
| Recorded proposal consent withdrawn | Permanent withdrawal for that proposal |
| Shared skill changes with enabled bindings | Immediate old-epoch admission rejection; durable hold/stop convergence |
| Unchanged bundle / presentation only | No epoch or consent change |
| Skill writer meets admission lock | Whole transaction rolls back, no partial content/outbox |

## 5. Good / Base / Bad Cases

- Good: organize a candidate while the old target exists, collect each owner's
  consent, stop/hold and prove old work ended, then commit and explicitly Resume.
- Base: refreshing identical files retains both file IDs and existing consent.
- Bad: renaming a skill's execution name is treated as presentation metadata,
  or repeated delete/reinsert of identical files revokes consent unnecessarily.

## 6. Tests Required

- `collaboration_switch_test.go`: stage-only state equality, every owner,
  stale versions/withdrawal, deterministic consent/commit races, write rollback,
  unknown commit recovery, immutable queue/history and viewer-specific grants.
- `collaboration_switch_routes_test.go`: real managed registration and HTTP
  stage/consent/commit/detail/recovery; old route is absent; no implicit run.
- `collaboration_skill_invalidation_test.go` in service/handler/migrations:
  actual body/file/version/delete/refresh/plugin writes, enabled/disabled and
  shared squad fanout, preserved fresh consent, original termination proof,
  no-op identity, restart/outbox and writer lock rollback.
- Switch migration and concurrent-index tests; empty database upgrade plus
  retained history and unconditional downgrade refusal in disposable schemas.
- Resource service/handler/schema tests cover explicit workspace visibility,
  owner-only tuples, stale configuration with still-revocable own grants,
  exact redacted recovery and disabled participant management. Shared forms
  verify role-gated controls and original-key recovery; real browser tests
  exercise consent/withdrawal/Switch and disabled-handler removal through APIs.

## 7. Wrong vs Correct

Wrong: update live configuration, move waiting input, then ask its new owner
for consent. Correct: freeze and consent to the proposal first; apply all live
changes in the final Switch transaction.

Wrong: reimport identical files by deleting and recreating them. Correct: compare
paths/content under the skill fence and change only the effective bundle diff.

## Current public Switch and owner-departure evidence

`TestCollaborationPublicSquadSwitchWaitsBehindBusyDestinationAndFreshResume`
uses actual public service Propose/Consent/Commit/Resume after a real squad
configuration update. Waiting S inputs move to the destination's tail with their
original typed target, after an earlier direct input. That direct target continues
while S remains `switch_hold`; a single waiting S cancellation stays cancelled.
Proof/polling does not Resume S. Only fresh explicit Resume consumes surviving
inputs in the new runtime. Compare complete old channel, stopped/cancelled
commands and frozen group/input/dependency/proof/attempt identity, not just counts.

`TestCollaborationOwnerDepartureRetractsMultipleTargetsAndSquadMember` removes
one owner with two direct targets and a member of another owner's squad. All
matching per-issue grants withdraw, that owner's waiting inputs cancel, other
authors' inputs remain, and all affected permitted roots/member runs retain
unknown-proof reservations. Other owner's completed/current/future history stays
unchanged. A Stop legitimately advances live `claim_generation` once; retained
termination identity, attempt/nonce and original resource/prompt facts remain
immutable. Do not compare that revocable live generation as a frozen proof field.
Cleanup must include the fixture's scoped delegation rows. These tests use real
service authorization with synthetic proof requests; AC96's complete catalogue
and derived matrix and actual provider/kernel evidence remain separate.
