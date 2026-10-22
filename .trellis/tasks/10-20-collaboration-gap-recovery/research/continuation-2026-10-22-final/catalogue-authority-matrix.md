# AC96 combined current service authority evidence

The existing S6 task now has one combined current regression,
`internal/service::TestCollaborationCatalogueAuthorityMatrix`, connecting
complete catalogue admission to managed derived execution and revocation. This
lane did not create a task, change an acceptance definition, edit product/spec
code, commit, or archive either task.

Changed code: `server/internal/service/collaboration_catalogue_matrix_test.go`
(446 lines). The test reuses tracked `newManagedCollaborationFixture`,
`ownerSwitchNamespace`, `ownerSwitchResource`, `ownerSwitchConsent`,
`ownerSwitchRegister`, `authorityCatalogue`, managed controls/Claim/Permit and
testutil rows. Local closures capture current workflow observations or test
assertions; no production abstraction or dependency was added.

## Execution

From `server/`, the recorded command is:

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^TestCollaborationCatalogueAuthorityMatrix$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

`catalogue-race-final.jsonl` passes **1 top-level test, 0 failures, 0 skips**.
The Go test elapsed 2.750s, package 4.720s, enclosing wall time 16.490s.
`catalogue-race-final-command.json` records branch, HEAD, exact argv/cwd and
22 relevant source hashes; its result confirms those sources did not change.
The test hash is
`13f89645cc13574d43328d0f08b6aa236daa09ba8b43c9a130960881510112c3`.
The database alias is task-owned `service_s5`, schema through migration 737.
Root coordinated its use serially; production/API databases were not used.

`go vet ./internal/service`, `gofmt -l` for the new file, and the scoped
`git diff --check` all pass. Exact command/result/stdout/stderr receipts use the
`catalogue-vet-final`, `catalogue-format-final` and `catalogue-whitespace-final`
prefixes.
A compile-only guarded `go test ./internal/service -run '^$' -p 1 -parallel 1
-count=1` also passed before the DB test.

Two earlier focused attempts remain retained and are not acceptance passes:

- `catalogue-race-first`: the test incorrectly expected `not_found` for an
  uninvited owner. The actual queue entry requires participation before target
  consent and correctly returns `forbidden`; the final assertion pins that code
  and complete unchanged business state.
- `catalogue-race-second`: restoring `agent.instructions` with NULL violated
  its existing NOT NULL schema. The final test reads and restores the exact
  persisted original value, then verifies its increased configuration epoch
  still rejects obsolete consent. No schema or product behavior was changed.

The original compile attempt used three guessed identifiers; it failed before
execution. Reading canonical protocol/principal types corrected them to DB
sequence assertions, `GrantID`, and `CredentialCloud`. This is test authoring
history, not a product regression or current verification failure.

The earlier `catalogue-race-third` run was green, but root's first isolated HEAD
run exposed an uncommitted `CommandHistory` dependency used by the test. The
final test uses tracked `ExecutionView`, `ListCollaborationBatchInputs` and
`RunRecords` to assert the same exact original IDs, author, typed target,
channel, sequence, body and stopped state. No product code was changed.

## Exact original clauses

All line references below refer to the new test file and its recorded hash.

| Original AC96 clause | Current assertion and real service evidence |
| --- | --- |
| Only creator/current handler organizes | The canonical managed fixture has creator registration. Lines 71–91 reject an uninvited owner and ordinary full participant, then the current handler registers the exact three-owner squad. A real Transfer at line 136 moves responsibility to the creator; lines 140–142 reject the former handler. After actual Leave, lines 348–350 reject the now source-only former participant. |
| Actual leader, every member and environment require participating owners' current per-issue consent | One creator-owned leader and two independently owned members have distinct runtime/daemon/binding principals. Before the third owner is invited, `RegisterTarget` returns `resource_owner_not_participant` without partial business state; uninvited consent returns `forbidden`. No squad grant exists after registration. Leader-only and two-owner stages remain absent. Lines 101–125 reject incomplete explicit input, organizer proxy consent, and member owner using the leader binding. Only the third own consent admits the squad. |
| New participant reuses valid grants without individual confirmation | Lines 145–168 invite a fresh password actor through the real workflow. Exact original three grant rows stay identical. The newcomer sees the existing squad and submits waiting input, acquires no source membership, and never performs a consent call. |
| Valid paused/offline/busy target stays selectable and waits correctly | Four newcomer inputs are accepted during issue pause, actual leader offline, real running ordinary task occupancy in the namespace, and target pause. Explicit catalogue reasons include `issue_paused`, `runtime_offline`, `target_paused`, then live `target_busy`. Canonical runtime List does not consume temporary waiting input or mutate business state. After the exact occupancy ends and fresh target Resume, one root consumes all four original author/sequence/body identities in FIFO. Live polling reoffers only the same root and creates no extra run. |
| Each derived execution retains actual grant source and rechecks | Lines 217–235 freeze all three exact owner/resource/auth/participant/installation/binding grants for every consumed input. A real leader runtime comment delegates only A and B; an outside-roster direct-agent mention creates no run. Lines 264–285 assert both derived member runs' exact parent/group/command/grant/epoch/owner/runtime/binding/installation and unchanged prompt. Lines 295–314 reject member Claim replay, Permit replay and RuntimeContext after another member's configuration changes. |
| Leader credential does not extend member rights | For each member, leader daemon Claim and leader business RuntimeContext both refuse without changing business state; the actual member's own Permit and RuntimeContext succeed. This uses distinct real persisted daemon principals, not a owner-ID substitution or mocked authorizer. |
| Incomplete, changed, revoked resources deny old choices/credentials | Incomplete direct submission returns `grant_required`; changed member configuration removes the squad and returns `target_configuration_changed` on old selection. Restoring the original configuration value does not restore the obsolete grant. Advancing the real member binding epoch rejects its old daemon token as `managed_runtime_required`. Explicit member RevokeGrant seals all three possibly live run identities and preserves reservations/proof barriers. Old selection remains refused after withdrawal and departure. |
| Owner departure retracts all resource grants | Member A has its squad grant and a distinct explicit direct-agent grant. Actual Leave after responsibility transfer revokes all A-owned grants, including the direct grant with the original epoch advanced once, and revokes the participant row. Catalogue then contains only the unaffected creator direct target; the third-owner/configuration-stale squad and departed-owner direct target remain excluded. |
| Readable history and legitimate Stop survive | The still-authorized newcomer can Stop the whole issue with current observations and read both member record streams with original grant/owner/run identity. Every exact retained termination token settles its original proof, including the binding-epoch-stale member. Lines 375–410 correlate public execution history, original batch rows and root input records: all original command IDs/author/target/channel/sequence/body remain readable with `stopped` state. No proof implies Resume or a new run. |
| Ordinary child and unauthorized machine remain minimal-reference-only | Lines 412–424 capture an ordinary child's own source body/comment without parent description or squad identity; the ordinary SQL writer refuses a parent collaboration runtime comment as the child's execution input. Lines 425–444 verify task/cloud/agent-actor PAT principals have exactly `Reference: true`, and real service content/record/Stop paths deny access. This does not mint a member capability or read a parent prompt. |

## Evidence scope and remaining boundaries

This closes the previously missing **combined service-layer** all-owner/new
participant/derived-runtime matrix. The fixture uses real database service
transactions and passwords/bindings as authority facts, but no password-login
UI, real provider executable/account/quota or kernel process. Sealed proof
payloads are synthetic protocol evidence, as in the canonical managed tests.

This lane does not rerun the Web/Electron owner forms, actual machine HTTP
title/ID reference serialization, packaged installed-client or other-OS
journeys. For whole-criterion promotion, root must correlate the current canonical
HTTP minimal-reference evidence and existing Web/Electron owner journeys with
this service receipt; service denial is not itself a complete browser or
reference-wire pass. The original clause/owner mapping and production gates
remain unchanged. Parent and child stay `in_progress`.

`catalogue-isolation-final-preparation.json` describes an outside `git archive` of
HEAD `cae15dd72ee568f28ce29b19f0c76765b72f26cd` with only the new test added.
All 13 reused helper functions are present in that HEAD. Its final compile-only
guarded check passes: `catalogue-isolated-compile-result.json`, 9.930s wall,
without DB execution. Root's subsequent serial isolated race also passes
**1/1, zero failures/skips, zero source drift**, recorded in
`catalogue-isolated-final-command.json`, `catalogue-isolated-final-result.json`
and `catalogue-isolated-final.stdout`. Test elapsed 3.650s, package 5.590s,
enclosing wall time 19.467s. This independently verifies the new file against
the exact HEAD sources without bringing shared uncommitted implementation into
its commit boundary.
