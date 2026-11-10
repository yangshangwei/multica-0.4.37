# Independent P3 VCS review and trust migration proof

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`.

## Verdict

**PASS_VCS_CODE_AND_TRUST_MIGRATIONS_FINAL_ROUTER_PENDING.** No blocking product
defect was found in the 15 frozen VCS files. All 15 hashes match the producer's
inventory. The reviewer added only
`server/cmd/migrate/delivery_webhook_trust_test.go` and this review's records.

The new actual migration matrix passes seven named results with zero
failures/skips. Current rebuilt router coverage is still 18 expected named
results, including the provider-mismatch case added after the earlier 17-pass
round. Final normal/race, all-writer/P4 and full gate/M1 acceptance remain open.

## Findings (fixed)

No product correction was needed in this review. The missing actual trust
migration coverage was added without changing either migration or weakening
an assertion.

The first test execution used the wrong working directory for the repository's
existing relative migration fixture. It failed before trust DDL assertions.
The unchanged compiled binary was rerun from `server/cmd/migrate` on a new
permitted database; the original failure remains retained.

## Findings (not fixed)

No unresolved defect in the bounded VCS source review. Root must still rebuild
and execute the current router inventory and final ordinary/race runs, verify
integration with the changing common core, and finish full source/validation
freezing. These are remaining acceptance obligations, not a claim that current
route or full gate behavior is already finally accepted.

## Source assessment

- Webhook identity carries the actual kind, workspace, local integration UUID,
  trust revision, PR UUID/repository/number/head, provider/instance and original
  event ID/body digest. Freeze and consumption compare these to currently locked
  installation/connection and linked PR rows. VCS stored PR provider is explicitly
  compared with the connection provider as well as its UUID and head.
- Signature callbacks are restricted to the original cloned body/headers and
  currently locked encrypted VCS secret or current GitHub process configuration.
  They return signature/config facts, not business admission. Current instance,
  provider and trust comparisons happen in the service transaction; callback nil,
  bad shape, changed configuration/secret or disconnected integration refuses.
- GitHub startup configuration identity is internal, domain-separated metadata;
  blank configuration cannot authorize. Proof fields and callbacks are excluded
  from public JSON. No human authentication context is constructed for the
  integration, and the status actor stays `system` while the terminal records
  the real integration/event identity. Remote provider truth is not inferred from
  local signature and database observations.
- Matching chooses exactly one unconsumed exact authorization under the current
  issue/policy/integration/revision/head/target. Missing or ambiguous candidates
  refuse. It never selects the newest arbitrary grant, and nil proof cannot
  bypass enforce. The common observer remains the sole evidence algorithm.
- `WriteIssueStatus` keeps its original transaction and RC/issue/W01/catalog
  fences. The original PR aggregate is checked independently, then checked again
  after Prepare acquires the observer's PR/link table fences. State, iteration
  record and Finish/consumption commit together. Failed commit returns an error;
  there is no retry or autocommit fallback.
- Current custom terminal categories are resolved by the original policy before
  completion. Collaboration issues still exit before ordinary system status
  mutation. Failed-task reset remains a `todo` call with explicit nil proof and
  does not enter the done gate. Existing off/observe semantics remain distinct
  from enforce; a webhook 202 is not represented as state completion proof.
- GitHub reconnect and permission restore advance trust; display enrichment uses
  a separate update and does not change it. VCS reconnect/provider replacement
  and secret rotation increment the existing row revision. Deletes remove the
  current identity; a later rebuilt row cannot reuse the old local UUID. Integer
  overflow fails the database write rather than wrapping trust.
- Each down migration locks the affected integration table and retained
  authorization table through its column removal. Both down steps refuse either
  GitHub or VCS grants, so the highest step cannot partially downgrade the other
  provider first. No protected history deletion, FK or new index is introduced.

## Added actual migration tests

The new file uses the existing private-schema fixture and actual 781–788
prerequisites, then runs actual 792/793 files. Minimal pre-existing integration
tables isolate the ALTER behavior; directly inserted authorization rows are
explicit schema-retention fixtures, not successful product authorizations.

- Existing and newly inserted integration rows get trust revision 1; zero,
  negative and NULL revisions are rejected. A valid revision update succeeds.
- Empty down removes both trust columns while preserving integration rows;
  re-upgrade restores the positive default. This tests an empty authorization
  history, not a way to reset trust while retaining grants.
- Separately retained GitHub and VCS grants each refuse the whole down sequence
  at its highest step and refuse either individual down file. Catalog, migration
  ledger, integration rows and authorization JSON remain byte-identical.
- An actual uncommitted grant INSERT prevents both DDL steps with SQLSTATE 55P03,
  even though the row is not visible to the contender. After commit, retention
  refusal remains and the grant survives. A bounded statement timeout exposes an
  accidental wait rather than hiding it as success.

`p3-vcs-migrate02`: **7 pass / 0 fail / 0 skip**, fresh
`enf_p3_vcs_migrate02`, same frozen compile01 binary, correct package cwd.
`p3-vcs-migrate01`: **0 pass / 5 fail**, fresh
`enf_p3_vcs_migrate01`, wrong cwd caused the fixture's `../../migrations` lookup
to fail. The two table subcases were never reached, explaining the smaller
failed inventory. This is a retained runner/fixture-path failure, not product red.

## Evidence and verification

- Independently rehashed all 12 producer run records/logs and all three recorded
  binaries/compile snapshots. Named inventories and unique terminals agree.
- Verified the real original red: GitHub and Forgejo enforce issues advanced
  without authorization. Two failing subtests plus parent are three failures.
- Kept producer rounds separate: router 17, trust handler 4, signature handler 7.
  The later wrong-provider route negative is not retroactively covered by the
  router 17 round. Existing compile03 failure was retained as another lane's
  temporary test compile error, not behavioral red.
- Reviewer migration compile01 passed without TestMain; binary and new test
  source hashes remain unchanged through the corrected actual execution.
- New and scoped Go files are gofmt-clean. Reviewed source hashes are frozen.
  No whole backend vet rerun during unrelated active edits; root owns the final
  coherent build/vet/sqlc/static check.
- Exact source, record/log and binary hashes, counts and limitations are in
  `p3-vcs-independent-review.json`.

Reviewer used only the two exact new migration databases assigned by root and
task-private schemas. No database allocation/drop, historical environment access,
container lifecycle action, external network/provider call, commit, push or
deployment. An initial read guessed `service/delivery_webhook.go`; the inventory
identified `delivery_completion_webhook.go`. That read-only lookup failure caused
no product or database action.

This report adds VCS code and migration evidence. It does not close P3, public
enforce activation, full completion enforcement, M1 or the six-business parent.
