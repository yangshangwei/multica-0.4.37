# FG durable operations and settings

Implementation is verified; the complete FG determination is recorded in fg-verification.md.

## Contract and execution plan

Use settings enable as the first real durable-operation writer, behind the
closed-by-default server rollout flag iterations_i1. Capability discovery is
explicit: supported follows that release gate, enabled combines it with stored
settings, manual=true, atomic_handoff=false until CG supplies that operation.
Settings reads return persisted state and do not create a default row. The flag
is not enabled in local/project configuration or published as a frontend flag.

RunOperation owns one bounded RC transaction per attempt, reusing RunTransaction.
The service adapts existing Begin starters by setting RC before any query.
Workspace KEY SHARE, subscriber/current-member locks and transport/role checks
precede catalog/I1 fences and operation lookup. Request hash and server operation
ID are stable across retries. A replay returns the stored result before mutable
settings/rollout checks. Only the request-index23505 raised while saving an
operation joins the same three-attempt retry budget; all other uniqueness
failures remain errors. An uncertain commit never triggers another business
attempt; GET and resubmission retain the original request identity.

Enable validates a strict envelope and confirmed IANA timezone. Under the
writer fence it initializes/locks settings, checks revision, SHARE NOWAIT-locks
the P1-owned workspace timezone, samples database wall time, applies the change
and saves the operation in one commit. Already-enabled settings do not bump
revision, but a new valid request still receives a durable result. No new period,
event, execution or notification is created by enabling settings.

## Evidence during implementation

- Initial test setup used a nonexistent featureflag provider and the first SQL
  draft referenced an absent settings updated_at column. Those were prerequisite
  errors; corrected to the repository StaticProvider and existing schema.
- Valid HTTP RED: `/tmp/i1-fg-settings-red3.jsonl` returned404 for default settings
  and enable-with-durable-replay, expected200. No such route was available.
- First integrated green exposed a fault fixture that also failed GET commits;
  recovery now uses a normal handler after simulating the actual write commit
  followed by EOF. This was a fixture issue, not successful unknown-result proof.
- Read-only review found legacy agent-to-member promotion while waiting for
  authorization. `/tmp/i1-fg-legacy-authority-red.jsonl` proves an initial agent
  whose task vanished could enable as human (200). Initial classification is now
  captured/denied before any transaction; each retry checks the same member.
- `/tmp/i1-fg-settings-green2.jsonl`:17 test entries pass, no failures/skips,
  with race detection. Includes concurrent same request, genuine PostgreSQL
  request-unique recovery after one stale lookup, no retry of other uniqueness,
  role demotion between rolled-back attempts, real commit+EOF recovery,
  all-or-nothing settings persistence, and timezone reference hold-through-commit.

All database runs use `multica_i1_w13_20261006`, a task-only isolated database,
and the ambient agent CLI guard. Remaining capacity and integrated checks must
pass before FG can be certified. Lifecycle/detail/preview/closure/UI remain in
later gates; no atomic-handoff capability is advertised.

## Final integrated checks

Root integration after all authorization, operation and pipeline changes:
- Handler520 pass,0 fail,2 existing opt-in skips (TriageQueueScaleBaseline,
  TriageWireFixtures): `/tmp/i1-fg-final-handler.jsonl`.
- Additional locked-grant tests9 pass: `/tmp/i1-fg-final-grants.jsonl`.
- Service/channel210 pass: `/tmp/i1-fg-final-service-channel.jsonl`.
- Iteration/featureflag/public-API domain100 pass: `/tmp/i1-fg-final-domain.jsonl`.
- Real JWT settings/operation routes3 pass: `/tmp/i1-fg-settings-routes.jsonl`.
- Full Go build/vet exit0; `/tmp/i1-fg-final-{build,vet}.log`.
- All81 sqlc generated hashes match a second generation; `/tmp/i1-fg-final-sqlc.log`.
- TypeScript lint/typecheck15 tasks and10,054 unit tests pass, no cache hits:
  `/tmp/i1-fg-final-ts-{static,tests}.log`. Root scripts exclude Mobile.

Independent read-only review found no remaining operation/settings blocker
following the captured-initial-actor fix. The mutation callback supplies the
locked wall-clock sample; only stable identity fields are stamped by the runner.
Credential validation retains the existing middleware point-in-time contract.
Full Go repository tests, Web/Electron E2E and remote CI belong to final VG.

The final settings-only race run includes an explicit admin success, cloud-PAT
refusal and trailing-JSON rejection:18 entries passed,0 failures/skips,
`/tmp/i1-fg-settings-final.jsonl`. Built-in guidance checks also passed22 entries
in `/tmp/i1-fg-builtin-tests.jsonl`.
