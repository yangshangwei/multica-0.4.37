# Owner identity, departure and public Switch current evidence

Existing S6 resumed under the existing task and branch; no task, plan, public gate,
SQL/schema, provider quota or rollout changed. Root owns ledger/spec/commits.

The smallest product fix is inside `collaborationIdentityCurrent`: after checking
stored binding against the resource owner/runtime/workspace tuple, password
validation derives a child context from the request containing that verified
owner session. `CheckPasswordVersion` still checks the user's version/account and
managed binding. The request's caller context, cancellation and deadline survive;
caller authentication remains independent. The duplicate explicit binding check
is removed because the same password check now validates the managed binding.

Files changed: `server/internal/service/collaboration_authority.go`,
`collaboration_resource_owner_identity_test.go`,
`collaboration_owner_departure_targets_test.go`, and
`collaboration_public_switch_resume_test.go`. The three retained uncommitted tests
were finished and strengthened without replacing their intended assertions.
Evidence and command/result artifacts use only this directory's owner-switch prefix.

## Verification

- Genuine current red: `owner-switch-resource-identity-red-current-20261022`.
  Same-owner daemon and human pass; different valid daemon fails with
  `current=false want=true`. Authority SHA is3001af84d292d08bcebd963725a00bbb004cb42422e833d70c82cc4fcad772dd.
- First fix: `owner-switch-resource-identity-green-current-20261022`,1 top-level /
  33 pass events, no failures/skips. Final expanded identity has13 scenarios ×
  three caller contexts:1 top-level /53 total passes in the final adjacent run.
  Unknown owner/password/binding, stale password/epoch, revoked binding, lost
  membership, disabled owner and retired installation fail closed. DB failure and
  cancellation before/during the owner check propagate. At the real password
  query boundary owner identity and inherited deadline are observed. Different
  daemon Claim and stale caller List cannot acquire another owner's authority;
  a human cannot become a daemon through the resource check.
- `owner-switch-journeys-first-current-20261022`: two actual service journeys
  passed first post-fix, no failures/skips. They use public Register/Consent,
  Submit/List/Claim/Permit, owner Leave and staged Propose/Consent/Commit/Resume.
- Final3 owned tests: `owner-switch-final-current-second-20261022`,3 top-level /
  55 total, zero failures/skips,19.832s. The later scoped delegation cleanup is
  verified with all3 in the final adjacent run.
- Final current adjacency: `owner-switch-adjacent-final-20261022`,26 top-level /
  132 including subtests, zero failures/skips/missing expected identities,
  36.991s wrapper elapsed, serial `-race -p1 -parallel1 -count=1 -timeout=30m`.
  Actual selector and per-file source hashes are in the command/result JSON;
  database is task-owned `service_s5`, schema737. Raw JSONL SHA:
  2efacb3739db3833d3dfdb16407e2687f29b01e57c9b059cce7cf03ad874d2ee.
- Gofmt, scoped `go vet ./internal/service ./internal/auth` and diff-check exit0
  with empty diagnostics; `owner-switch-static-*` keeps commands/results/hashes.
  Root independently verifies exact HEAD plus these files on service_s6_commit.
  Do not infer independent HEAD success from current-tree compilation alone.

## Whole-clause recommendations

AC49: recommend complete at P4 after root independent review. D owns two distinct
managed direct targets plus another owner's squad member. Real Leave retracts all
three grants, cancels D's waiting inputs, preserves other authors, holds all
three targets, seals all four permitted roots/member runs and retains unknown
proof/reservations. Invalid candidates disappear and stale derived Claim fails.
Unrelated owner's target/channel/grants/commands/current+settled records are exact
snapshot-equal; all consumed inputs, frozen dependency/resource identities,
termination identities and attempts remain preserved. Own GET recovery is inert;
source membership never restores collaboration control. The member's saved
GrantID/GrantEpoch/runtime/binding/owner/parent match its actual resource,
and leader credential/business token cannot expand into the member run.

AC40: recommend complete together with existing canonical clauses at P4 after
independent review. A real `UpdateSquad` query changes resource configuration;
public staged Switch leaves live target/grants/inputs/history equal until commit.
Replacement owner must separately consent, organizer substitution fails. Original
S target ID/kind/resource stay fixed. Waiting inputs append in original relative
order after the destination's earlier direct input; third input can be cancelled
singly. Destination current work/direct target remain unchanged, the earlier
direct head executes normally while S stays held, and proof/poll do not Resume S.
Only fresh explicit target Resume consumes the two retained typed S inputs on the
new leader/runtime. Original stopped/cancelled commands and full old settled
group/run/input/dependency/proof/attempt rows plus old channel remain equal.
Canonical adjacent tests cover other direct Switch/consent/whole-control clauses,
ordinary transfer, source projection and original-key recovery.

AC96: remains partial. This lane now supplies actual distinct-owner managed
leader/member provenance and leader-token separation, all-owner departure,
public owner-consented Switch and old binding rejection. The complete combined
new-participant use of existing squad grants, busy/paused/offline selectable
catalogue, derived-permit resource change and ordinary minimal-reference matrix
has not been asserted as one current integrated matrix here. Existing adjacent
individual tests are support, not automatic whole-criterion promotion.

## Retained failed attempts and corrected boundaries

The inherited owner-identity red artifact under continuation-2026-10-21 was only
a build failure: unused pgtype import and nonexistent runtime Identity.ConfigVersion.
These test prerequisites were corrected; runtime protocol2 stays unchanged,
and group config_version is checked against actual DB identity. The retained
mixed-owner departure diagnostic was a genuine config_invalid behavior failure.

The expanded first owner_missing fixture attempted to DELETE a referenced user
and hit existing agent_owner_id_fkey. It is retained as a fixture setup failure;
final owner_unknown uses a nonexistent resource UUID tuple. Missing password/
binding are separate cases. No database relation or authorization was weakened.

The first strengthened departure aggregate incorrectly compared live
claim_generation as immutable. Actual RequestCollaborationRunStop advances it
once to revoke old admission. Field diagnostic proves1→2; final test requires
that increment while retained termination.runtime_identity/attempts/resource/
nonce/prompt facts remain unchanged. Frozen dependency detachment status is a
legitimate unread transition; its original authority identity remains compared.
The original run proof/barrier assertions were retained and strengthened.

Root's first isolated pair is PASS2/2 on pre-cleanup source9660a9e, not a compile
failure. Its source_changed_during_run reports the later scoped delegation cleanup
in the shared current tree; the raw first receipt stays historical. Root reruns
latest source before committing. No shared helper was removed on an inferred
compile failure.

## Limits

Claim/Permit are actual database service operations; proof payloads are synthetic.
They do not observe kernel descendants, actual provider/version/account quota,
installed Electron full Main, other OS support or deployment. No browser/native
run was performed by this owner lane. Current prod gates stay closed. Root
coordinates other current Web/Electron evidence and Lore commits; this lane made
no commit/ledger/spec/task/archive mutation. Full97 acceptance is still open.
