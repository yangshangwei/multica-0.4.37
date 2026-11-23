# P1 authorization router proof

Scope: authenticated authorization HTTP boundary only. Owned files are
`server/cmd/server/delivery_completion_authorization_integration_test.go` and this
report. The parent owns routing, production code, database allocation and final
integrated completion acceptance. No provider or external service was used.

## Observed result

`p1-router-iter03` passed **11 named results, 0 failed, 0 skipped**, with one
terminal event for each started name. This is nine top-level tests plus the two
legacy/password subcases. The actual raw Go JSON names and log SHA-256 were
recomputed and matched the retained run record.

This run executed the previously compiled `p1-router-tests` binary through
`go tool test2json`, against the newly allocated `enf_p1_router_iter03` database.
The parent migrated that database through 791. The binary was compiled in
`p1-router-compile01`; its 88-file source snapshot was unchanged across compilation.
The runtime run record's contemporaneous worktree snapshot is **not** substituted
for the binary's compile-source identity.

| Binding | SHA-256 |
| --- | --- |
| Executed binary | `c857e4f0988aa4f2654e9ca6a898555914993d8c37d10250b1de02bb8d75fc88` |
| Compile record | `2af7f563e4a1c4ed4acd674f8e97ed6fe068bb66254f91515d8a757331cc7fe4` |
| Test source actually compiled/run | `7d4a785083d6dd85b04f7c5dbff6646fc18a02b45664473d88b06f327c8a88b3` |
| Current enhanced test source | `ab3c5cd7eedc49b9ac2cfcec8e4bfa21aa6a65356ccf23046da9131228a2a3c9` |

Binary path:
`/private/tmp/multica-completion-enforcement-20261111-y9c0v8ro/binaries/p1-router-tests`.
Complete compile-source hashes: `research/runs/p1-router-compile01.json`.

## Actual behavior proved

The tests use real `NewRouter` JWT/task-token middleware. They reuse the existing
controlled artifact create/approve/execute API chain, check real physical tar
entry bytes and SHA, and inspect committed authorization/terminal identities.
No successful artifact, grant, consumption or revocation is seeded.

The only preinstalled gate configuration is a synthetic task-owned enforce policy
row: public enforce activation belongs to P4. These tests do not establish that
public policy enablement or any actual completion writer is complete.

| Top-level test | Proof |
| --- | --- |
| RealArtifactAndRestart | Legacy/password real packaging, exact issue/operation/issuer/artifact bindings, same-key identity, changed-intent conflict, reconstructed router and renewed credential recovery, unchanged expiry/lifecycle/private evaluation/archive and no new task admission |
| RejectsMachineAndSpoofedIdentity | Missing bearer with forged identity, actual task token claiming human, human agent/task headers cannot issue; no authorization/lifecycle side effect |
| DesignatedHumanKeepsEvaluationPrivate | Another designated human reads only the minimal grant; an actual original private evaluation stays private; issuer-key recovery is not transferred; another valid member cannot read the grant |
| DesignatedTaskReadsOriginalGrant | Actual task token owned by the issuer follows the designated task branch, recovers the exact original grant, and cannot revoke as its human owner |
| ConflictingWorkspaceSelectors | Conflicting ID/slug cannot issue, read or commit revocation under another scope |
| CurrentCredentialRecheck | Password version is revoked after real middleware authentication, before the handler transaction; issuance refuses; renewed credential works, then subsequent revocation blocks reads without deleting history |
| ExpiryBounds | Beyond evidence expiry, already expired and beyond a short current issuer JWT lifetime all refuse without storing an authorization |
| SkipAndAmbiguousJSON | Explicit skip produces the specific unsupported refusal; duplicate and trailing JSON refuse without storage |
| RevocationRecovery | Designated ordinary consumer cannot revoke; issuer produces one original actor/key terminal; restart/replay reads it without changing lifetime or lifecycle; changed original reason conflicts |

## Retained attempts and limits

| Run | Exit | Actual outcome |
| --- | ---: | --- |
| p1-router-iter01 | 1 | Zero tests; concurrently edited ordinary writer had unused pgconn import |
| p1-router-iter02 | 1 | Zero tests; this new test file had an unused net/http import |
| p1-router-compile01 | 0 | Compile-only go test -c; no TestMain/database execution |
| p1-router-iter03 | 0 | Actual compiled router tests: 11 pass, 0 fail, 0 skip |

The first failure is a shared compilation-window failure. The second is an owned
test-authoring compilation failure, fixed by deleting only the import. Neither is
product-behavior red. Both allocated databases and original logs/records remain
retained by the parent; no test run was repeated against the same database.

Source review identified the same-owner task GET dispatch and explicit skip
response before actual runtime proof. The ledger owner fixed those before this
run. They are not falsely represented as observed failing route tests.

After the passing run, review of the existing fixture showed that its snapshot
compared task count but not all fields of the existing producer task. The current
test now also compares the complete `agent_task_queue` row JSON before/after
issuance and recovery in both legacy/password cases. This adds seven assertion
lines without changing production code or fixtures. **That strengthened assertion
has not yet been executed.** The parent explicitly owns rebuilding and testing
this current source in the final normal/race runs. The intermediate 11-pass proof
continues to bind the older compiled test hash shown above.

## Verification and handoff

- All four referenced raw logs match their recorded SHA-256; the actual 11 names,
  zero skips and unique terminals were independently parsed here.
- Current test/report text is UTF-8, has no trailing whitespace and has one final
  newline. The test is gofmt-clean. No other product or specification file was
  changed by this lane.
- No database/container lifecycle operation, migration, provider call, external
  message, dependency change or commit was performed by this lane. Database
  execution used only the parent's exact permits and private launcher.
- Final rebuilt normal/race router validation, the added complete-task-row
  assertion, real completion writer integration, full delivery enforcement and
  M1 acceptance remain with the parent. This bounded proof closes none of them.
