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

## 7. Wrong vs Correct

Wrong: update live configuration, move waiting input, then ask its new owner
for consent. Correct: freeze and consent to the proposal first; apply all live
changes in the final Switch transaction.

Wrong: reimport identical files by deleting and recreating them. Correct: compare
paths/content under the skill fence and change only the effective bundle diff.
