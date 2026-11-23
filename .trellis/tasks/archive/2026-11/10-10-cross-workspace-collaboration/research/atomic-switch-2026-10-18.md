# Consent-ready atomic Switch

This slice implements E01/E07's final Switch while the existing task remains
`in_progress`. The current `ReconfigureTarget` changes live configuration before
owners consent. It is now confined to a `_test.go` pending-configuration fixture;
its production method and public route are removed. The replacement stages
immutable proposals.

1. An organizer proposes a current valid resource roster against a target's
   target/config versions and the issue lifecycle. This writes no target, grant,
   queue or execution changes. The proposal stores its configuration fingerprint.
2. Each actual resource owner, already an effective participant, consents to the
   immutable proposal and current installation identity. Consent captures owner
   account/participant generations. Missing, changed or obsolete consent is never
   inherited from the live configuration or another proposal.
3. The organizer commits using current target control and hold observations.
   Under the existing account/source/session/channel/target transaction fences,
   verify the original lifecycle/configuration, pause or hold, all old dependent
   groups/runs' termination proof, every owner's current consent and resource
   identity. Then revoke old grants, advance the target configuration, materialize
   the proposed grants and move only waiting inputs to the new leader lane tail.
   Commit leaves `switch_hold`; Resume remains an explicit later operation.

Target typed identity, old lane controls and group/run/proof/input history remain
unchanged. Replay returns the original operation receipt; uncertain commits never
trigger an automatic new mutation. Stale consent or validation refusal writes no
business changes. Proposal/consent reads expose only authorized resource IDs and
status, without configuration bodies, paths or installation credentials.

Owned files: new Switch service, query, handler and focused tests; a proof-barrier
helper extracted from `collaboration_authority.go`; receipt field integration;
migrations 713–716. The parent owns route/core wiring, generated sqlc, migration
index recovery registration and full integration. No dependency or legacy
execution fallback is added. Runtime/group M08/M11 work is owned separately.

Validation: real PostgreSQL tests for stage-only behavior, actual multi-owner
consent, stale lifecycle/config/owner generations, proof/hold refusal, immutable
history, waiting FIFO movement, exact replay and deterministic consent/commit
race in both orders. Run focused Go tests through the repository's fake-CLI guard,
gofmt/vet and migration checks before parent integration review.

## Implemented and verified

- Staging, per-owner consent/withdrawal, final Switch, authorized proposal list/
  detail, owner-only current-version grant references, and GET operation recovery
  are implemented. The parent registered routes and generated sqlc.
- Refusals preserve live target/grant/input state; final writes, grant replacement,
  lane movement, audit and receipt commit together. Required-write fault injection,
  uncertain commit, two-proposal competition and both last-consent/commit orders
  have database regressions.
- Two new proof regressions first failed: a settled group still occupying the
  active pointer and a paused target mirror over an active channel. The shared
  proof helper now refuses both before applying Switch.
- Go 1.26.6, isolated `collaboration_switch` PostgreSQL at migration 718:
  `go test -race -p 1 ./internal/service ./internal/handler -run
  '^TestCollaboration(AtomicSwitch|Switch|Reconfigure|FIFO|Consent|OwnerConsent|Registration|Hold)'
  -count=1 -v` passed through `scripts/go-test-with-agent-cli-guard.sh`:
  **31 top-level tests and 54 subtests; zero failures/skips**. Service 20.984 s,
  handler 2.643 s. `go vet` for both packages, gofmt and `git diff --check` passed.
- Evidence: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-completion-20261018-l73rn78t/switch-focused-race-718.log`.
  The earlier `switch-focused-race.log` is a schema-mismatch run after another
  lane generated migration 717's `prompt_snapshot` before this database upgrade;
  it is retained as failed evidence, not counted as a passing product test.

The new frozen group-dependency table (717+) must join the final Switch proof
discovery when that lane's runtime implementation lands; the current helper
discovers target ownership through roots, current command links and immutable
input history. Parent integration owns this handoff, core/client DTO wiring,
built-in skill/spec synchronization, migration recovery tests and independent
review. This slice does not establish M08/M11 or full Web/Desktop acceptance.
