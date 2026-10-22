# Independent review: S3b command history and S5 administration backend

Date: 2026-10-21. Active task: `10-20-collaboration-gap-recovery`.
Scope: delivered S3b service/SQL/migrations/handler/core/views and S5 service,
handler, SQL, operation allowlist and route registration. This review does not
claim S4 delivery, S5 frontend/native acceptance or complete parent acceptance.
Existing uncommitted work was preserved. No commit, rollout, real provider,
account access or new dependency was introduced.

## Findings fixed

### F1 — P2: filtered candidates produced invalid page continuations

Location: `server/internal/service/admin_collaboration_view.go:190`,
`ReopenCandidates` (the original pagination block began at line 203).

The SQL query selected `limit + 1` users, then the service chose its cursor and
removed emergency-disabled users. A denied prefix returned an empty page with
a cursor; a denied boundary returned a short page whose cursor named a hidden
user; a denied tail advertised another page without any eligible candidate.
The existing strict core schema correctly rejected those responses.

`TestAdminCollaborationCandidatesPaginateEligibleParticipants` reproduced all
four cases against the isolated real database before the fix: `denied_prefix`,
`denied_page_boundary`, `denied_tail`, `all_denied`. All four failed for the
expected page/cursor assertions, rather than fixture or compilation errors.

The service now fills the visible page through bounded keyset queries in the
same authorized repeatable-read transaction. It retains the existing emergency
denylist predicate, sets a continuation only after finding another eligible
user, and names the last returned user. No API, schema or eligibility relaxation
was required. The four cases and the existing scope regression now pass.

### F2 — P2: R38/R40 disagreed with authentication about disabled accounts

Locations: `admin_collaboration_view.go:75`, `:112`, `:141`, `:190`;
`admin_collaboration.go:142`, `:361`;
`server/pkg/db/queries/admin_collaboration.sql:25`.

The new management paths tested only `disabled_at`. Authentication also denies
the existing emergency-denylist accounts. Consequently an emergency-disabled
creator was absent from management, and an emergency-disabled handler was
reported as healthy and did not require a successor when reopening.

The coordinator explicitly confirmed that R38/R40 disabled means persisted
`disabled_at` **or** `auth.IsTemporarilyDisabledUser`; missing credentials,
password-change restrictions and leaving a workspace do not create this
exception. Six synthetic-account scenarios in
`TestAdminCollaborationTemporaryDisableMatchesAccountAvailability` failed before
the fix. The two password-restriction negative cases already passed.

List, detail, candidate eligibility and locked lifecycle/successor decisions now
use that same definition. The coordinator approved the narrow auth helper
`TemporarilyDisabledUsers`, which returns sorted independent snapshots of the
existing ID/email maps. No new configuration source was added. Internal SQL
parameters filter creators before `LIMIT`; email matching preserves case and
the Unicode whitespace normalization used by `strings.TrimSpace`. Emails and
denylist values are not part of a human DTO. The coordinator regenerated sqlc.

Regression coverage includes the six lifecycle combinations, before-limit
filtering past an enabled creator, UUID filters, empty-filter controls,
uppercase/Unicode whitespace, independent auth snapshots and preservation of
the password-restriction negative cases. All pass in the final race run.

## Review conclusions

- **S3b:** no additional defect found. Commands keep their original body,
  author UUID, typed target, attachment IDs and input identity after display
  comment changes/deletion. Cancellation state, reason, exact event and linkage
  commit together; real trusted principals supply human attribution, while
  background cancellation retains null actor. The migration backfills only a
  fully correlated singleton legacy manual receipt. Missing old evidence stays
  explicitly unknown.
- History reads use current issue ACL before cursor existence/data reads.
  Source-member access stays independent from participation and grants no
  cancellation authority. M11 cancel-first/claim-first, automatic detachment,
  nonlaunch return and Switch history retain exact input/run identities; a
  losing cancellation never becomes Stop. Separate optional history parsing
  preserves installed-client compatibility and current account/access-epoch
  cache protection.
- **S5:** actual complete password-session identity, current super-admin role,
  active organization/source scope and sorted platform/user/source/issue locks
  remain enforced. Reopen borrows the original lifecycle and handler helpers;
  successor, workbench responsibility, information-chain supersession, both
  operation stores, real-actor audit/history and proof barriers stay atomic.
  It creates no administrator participant or ordinary content/resource right.
- **Creator restoration:** the coordinator chose the existing narrow recovery
  contract. Every POST, including the original key, is refused after restoration;
  the original result remains recoverable by dedicated GET under current admin
  and source authority. The existing recovery regression now explicitly checks
  the identical POST refusal and unchanged stored result before its GET. No
  replay ordering or extra authority was introduced.
- The root's new top-level `scope` on list/detail/candidates is assigned from
  authorization inside the read transaction and passed its projection test.

## Verification

Database alias: `review_s3_s5`, an isolated task database through migration 737.
Toolchain: `go1.26.6 darwin/arm64`. The agent CLI guard was used for Go tests.
No native containment or provider execution is implied by these tests.

Final results, with no failures or skips:

| Scope | Top-level tests | Subtests |
| --- | ---: | ---: |
| Go auth | 2 | 0 |
| Go service | 34 | 117 |
| Go handler | 5 | 0 |
| Go migrations | 1 | 0 |
| Go migration runner | 4 | 0 |
| TypeScript core history | 64 | — |
| Shared history/execution views | 8 | — |

The Go total is **46 top-level tests plus 117 subtests**; the TypeScript total
is **72 tests**. These are fresh review executions, not added to previous
implementation counts. Exact identities and package results are in
`/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s3-s5-go-race.jsonl` and the two
`check-s3-{core,views}-tests.json` files.

Commands from `server/`:

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py review_s3_s5 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race -p=1 ./internal/auth ./internal/service ./internal/handler ./internal/migrations ./cmd/migrate -run '^(TestTemporarilyDisabledUsers|TestAdminCollaboration|TestCollaborationCommand(Cancellation|History)|TestCollaborationCancellationMigration|TestCollaborationSquadHumanJoinClaimAndUnreadCancellation|TestCollaborationCommandHistoryMigrationRepairsInterruptedIndex|TestConcurrentIndexCleanupsMatchTheirMigrations|TestEveryConcurrentUpBuildHasCleanup|TestEveryConcurrentDownBuildHasCleanup)' -count=1 -json
GOTOOLCHAIN=go1.26.6 go vet ./internal/auth ./internal/service ./internal/handler ./internal/migrations ./cmd/migrate
```

Commands from the repository root:

```bash
pnpm --filter @multica/core test command-history --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s3-core-tests.json
pnpm --filter @multica/views test command-history execution-panel --reporter=json --outputFile=/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s3-views-tests.json
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core exec eslint collaboration/command-history-schema.ts collaboration/command-history-schema.test.ts collaboration/command-history-queries.ts collaboration/command-history-queries.test.ts collaboration/command-history-test-fixtures.ts
pnpm --filter @multica/views exec eslint collaboration/command-history.tsx collaboration/command-history.test.tsx collaboration/execution-panel.tsx
```

Both package typechecks, scoped ESLint, Go vet, gofmt and scoped
`git diff --check` passed. The first views typecheck found two missing
`admin.collaboration.reasonInvalid` references in the live S5 frontend lane.
Those files/locales were outside this review's write scope. The coordinator
supplied the locale key; the final full views typecheck passed. The failed log
is retained as `check-s3-views-typecheck.log`, and the pass as
`check-s3-views-typecheck-final.log`.

Behavioral RED logs are retained as `check-s3-s5-candidates-red.jsonl` and
`check-s3-s5-temporary-disabled-red.jsonl`; the initial candidate GREEN log is
`check-s3-s5-candidates-green.jsonl`. Final source hashes and test identities
are recorded in `check-s3-s5-verification.json` beside those logs.

## Files changed by this review and remaining handoff

This reviewer changed only the two auth files, `admin_collaboration.go`,
`admin_collaboration_view.go`, `admin_collaboration_authority_test.go`, the admin
SQL source and this report. The generated SQL is the coordinator's integration.
The changes reuse the existing authentication predicate and transaction owners;
they add no parallel authority model, persistence layer or dependency. No S3
product or UI changes were necessary.

No known unresolved code defect remains in this review scope. The coordinator
still owns final spec/source-map sync, S5 frontend and production Web/actual
Electron acceptance, and the 97-row acceptance ledger. Preserve the three
contracts above in the server/core specs: effective disabled-account semantics,
eligible candidate pagination, and GET-only recovery after creator restoration.
This review did not run full `make check`, new browser/native journeys, packaged
clients, provider/OS support tests or production services. Shared-workspace
focused checks are not a frozen production-build provenance record.
