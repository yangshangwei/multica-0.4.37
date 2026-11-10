# P3 GitHub/VCS completion integration

## Scope and changes

The actual GitHub and self-hosted VCS webhook paths now carry the verified
integration, trust revision, PR/repository/head, original event identity/body
SHA and a narrow current-signature recheck into `WriteIssueStatus`.
The business actor remains `system` with no human identity. The original PR
aggregate remains required; the completion transaction rechecks it after the
single observer has acquired PR/link fences.

`delivery_completion_webhook.go` freezes exact webhook consumers for the common
authorization core, locks current integration/PR/link facts and selects exactly
one matching unconsumed authorization. Zero or ambiguous candidates refuse.
It validates current signature bytes and configuration again in the actual
Prepare/Finish transaction; there is no second observer or optional nil gate.
Private proof fields are excluded from JSON. Ordinary reset-to-todo explicitly
passes nil and preserves its existing behavior. The two exact direct test
calls and that one production reset call were updated under parent delegation.

Migrations 792/793 add positive monotonic `trust_revision` fields. Production
reconnect and VCS secret rotation advance them; GitHub permission restoration
also advances trust, while display enrichment uses a separate existing-row
update without changing trust. API integration projections expose this nonsecret
revision for exact consumer binding. No dependency, foreign key or new index is
added. Down migrations hold their table locks through the column removal and
refuse any retained GitHub/VCS completion authorization before dropping either
side's trust column. Their actual rollback matrix remains pending with the parent.

The parent owns `ArtifactAccess`'s server-only GitHub config identity and its
real handler population. The collaboration/core owner integrated the common
consumer switches and terminal union. Generated models/queries and late-expiry
conditional INSERT are owned by the independent review lane; this lane did not
run sqlc or edit generated output, core authorization files or router registration.

## Observed tests

- `vcs-router-red01`: real signed GitHub and Forgejo webhooks completed enforce
  issues without authorization (`todo/revision2 -> done/revision3`). Two failing
  subtests plus the parent give **3 failed, 0 passed, 0 skipped**. This is actual
  behavioral red against the separately frozen prior binary/source snapshot.
- `vcs-router-iter01`: **17 passed, 0 failed, 0 skipped**. Real NewRouter signed
  GitHub/Forgejo/GitLab events plus actual controlled tar/grant chains prove
  missing-authorization refusal, exact state/terminal consumption, no repeated
  consumption, trust/config/secret/disconnect denial, ambiguous-grant denial and
  the independent still-open-sibling aggregate barrier. HTTP 202 acknowledges the
  mirrored webhook, never proves issue completion; tests inspect persisted state
  and terminal actor/integration/event/operation facts.
- `vcs-handler-iter01`: **4 passed, 0 failed, 0 skipped** on a separate database,
  proving actual production reconnect/display/rotation SQL and public revision
  projection.
- `vcs-handler-signature01`: **7 passed, 0 failed, 0 skipped** on another fresh
  database using the `vcs-handler-compile04` binary. In addition to the four above,
  real signed GitHub/VCS handler flows change current config/secret only after
  initial signature authentication, before the status transaction begins. VCS
  deliberately keeps its revision unchanged in this corruption probe: final
  current-secret verification still rejects, proving it is not merely a cached
  epoch/success flag. Status/revision and unused authorization remain unchanged.

All raw log hashes, named test inventories and unique terminals were recomputed
for the run records listed in `p3-vcs-implementation.json`. Counts are per round;
normal/race, parent/subtest and previous-attempt results are not added together as
independent behavior coverage. Tests used only parent-allocated exact databases.
No real provider account, external webhook or remote API was contacted.

## Source and failure provenance

Compile-only `test -c` was used to preserve source identities while other lanes
worked. The red binary and both handler binaries have SHA-256 and their complete
compile-source before/after snapshots recorded in the companion JSON. Runtime
worktree hashes do not substitute for those compile snapshots.

`vcs-handler-compile03` failed with zero tests because the simultaneously added
collaboration test referenced `ArtifactDigest` instead of `ExpectedArtifactDigest`.
Its owner corrected that typo; this lane did not modify their test. The original
failure record/log remains. It is a shared compilation-window failure, not VCS
behavioral red. Compile04 then passed. All other listed compile/build runs passed.
A reviewer also reported an earlier shared build window while this lane's import
and caller changes were being written; their original record remains separately
owned, with no retrospective green replacement.

After the 17-case router pass, source review added an explicit requirement that a
VCS PR's stored `provider` match the current connection's provider, beyond matching
its connection UUID/head. Freeze and consumption now check that fact, and one
real authorization-route rejection test was added. The current router inventory
is therefore **18 named results, not yet run as a whole**. The latest seven-case
handler signature run includes the updated provider-checking helper, but does
not replace this new route negative or full rebuilt normal/race evidence.

## Current owned source inventory

| File | SHA-256 |
| --- | --- |
| server/internal/handler/github.go | `6b88995907af1d48480828579b22d3d1c35382629445acbe7aad936fca053c06` |
| server/internal/handler/vcs.go | `289a9da24ea5844b338403ba3f56b0adee515e30edc28415a5e822dfd2d0d38a` |
| server/internal/handler/vcs_webhook.go | `7f40b6cea1c864b29b7722de1fe5ac6476c2077861282b8076ab359eebd3ea52` |
| server/internal/service/issue_status_write.go | `71c96fea42ad2f07017577091face61c5295921883b74a09744d2153771ed286` |
| server/internal/service/delivery_completion_webhook.go | `ab39ace3439f5332cbaa2a789608cf5766ff828ff7cec5a5fe4afec0f47a573b` |
| server/internal/service/task.go | `8ad09048e05873b9723045682352bcb85d309be90d8584631d9d2b83fe817481` |
| server/internal/handler/collaboration_boundary_test.go | `02c2f9cd5ba6d7b89896becb0eb365ded92c118908f6708fd389bdcfaac83300` |
| server/pkg/db/queries/github.sql | `17f3df846b5933b9f9934ad25473e529b49917eacd90190ac9c7e14fa32b0faf` |
| server/pkg/db/queries/vcs.sql | `fff09916b59c267b87a13adc73337f1353a20ffdb52900cb485cb1a2d887753c` |
| server/internal/handler/delivery_completion_webhook_test.go | `ba3a40fe81548020e0fde18f8cdc048759629b8c322a981fbd89be8cc1d2b2aa` |
| server/cmd/server/delivery_completion_webhook_integration_test.go | `d864747dc5a6c508d506db604dae996d67b2b36450d48f566a6bba298ceaa3b5` |
| server/migrations/792_delivery_github_trust.up.sql | `b3df9c6e000f93369f142d6e4506048e15fa33cba4ecee3ea52187f666f96c67` |
| server/migrations/792_delivery_github_trust.down.sql | `4c82ae887f817c65b567ef6fd143d18a21a417ac6e807daaed741417fba24416` |
| server/migrations/793_delivery_vcs_trust.up.sql | `b5fc19e8f36f937a042fc5b907c9e6ddf82807a6a76cc07255555991537f67e2` |
| server/migrations/793_delivery_vcs_trust.down.sql | `d958477056ee3feaef46eb1efc8e82434138b098bdacb4e3b7fd640b2d8306ae` |

## Remaining parent checks

- Rebuild/run current router18 plus authorization11 and other writer integration
  on final fresh normal/race databases; do not use the earlier 17-case snapshot
  as if it covered the later provider-binding negative.
- Actual 792/793 migration empty downgrade/re-upgrade and retained-GitHub or
  retained-VCS history refusal in both relevant steps, plus concurrent writer/DDL
  refusal. No evidence deletion or rollback of historical databases is permitted.
- Independent P3 code/security review of real signature identity, late expiry,
  all constructor/entry wiring and preserve-existing off/observe tests, including
  failed-task reset and direct collaboration boundary tests.
- Complete all-writer/P4 policy activation, then final source/log freezing and
  safe local integration. No full delivery-gate or M1 completion is claimed here.

Current owned Go files are gofmt-clean and the working diff has no whitespace
errors. This lane made no commit, deployment, task-state change or database
lifecycle action. The completed code is ready for parent integration/review;
remaining tests above are intentionally explicit.
