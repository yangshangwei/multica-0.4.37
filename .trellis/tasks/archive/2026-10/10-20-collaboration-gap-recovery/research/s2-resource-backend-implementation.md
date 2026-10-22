# S2 resource-offer backend implementation

Active task: `10-20-collaboration-gap-recovery`. Scope: backend only; the main
session owns generated SQL, router registration, migration recovery registration,
client integration and final acceptance.

## Change boundary and reuse plan

The existing organizer inventory cannot disclose a participant owner's private
resource to an organizer outside that workspace. Add issue-scoped immutable
offers, owner-only supply/withdraw, and organizer adoption into the existing held
target. Every owner still consents through the existing resource API.

- New service/read/handler/test files own offer snapshots, ACL, transitions and
  original-operation recovery. New SQL and migrations 732–734 retain these facts.
- `collaboration_resource_options.go` shares complete visible roster construction
  while keeping organizer inventory authority and adding an actual-owner filter.
- Extract only `RegisterTarget`'s transaction callback into `t.registerTarget` so
  adoption and registration use the same role/version/owner/configuration/hold
  checks, channel creation and event writes. No second committing wrapper.
- Add optional offer receipt identity/version and action initialization to
  `collaboration_queue.go`; extend the existing resource recovery allowlist and
  receipt validation. Do not change queue, cancellation, runtime or proof behavior.
- Keep gates, dependencies, private directories, grants and unselected resources
  outside this feature's authority. No commits or task lifecycle changes here.

Before extraction, the original eight registration/consent/hold/inventory tests
passed on the task-owned `service_s2` database (Go 1.26.6, CLI guard):

```text
TestCollaborationRegistrationChecksActualResourceOwner
TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent
TestCollaborationConsentUsesCompleteSquadRoster
TestCollaborationTargetConsentRetainsInitialHold
TestCollaborationResourceOptionsRespectExistingVisibility
TestCollaborationResourceHTTPExplicitScopeAndOwnerIdentity
TestCollaborationResourceOptionsExplainDisabledOwner
TestCollaborationResourceHTTPRefusesMachineBeforeDatabase
```

Command: from `server/`,
`python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s2 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/service ./internal/handler -run '^(TestCollaborationRegistrationChecksActualResourceOwner|TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent|TestCollaborationConsentUsesCompleteSquadRoster|TestCollaborationTargetConsentRetainsInitialHold|TestCollaborationResourceOptionsRespectExistingVisibility|TestCollaborationResourceHTTPExplicitScopeAndOwnerIdentity|TestCollaborationResourceOptionsExplainDisabledOwner|TestCollaborationResourceHTTPRefusesMachineBeforeDatabase)$' -count=1 -v`.
Result: both packages PASS; 8 top-level tests, no skips.

## Implementation and verification

Backend implementation and focused verification are complete. Independent review,
client integration and browser/native product acceptance belong to the coordinator.

### Delivered files

New production files:

- `server/internal/service/collaboration_resource_offers.go`: the three mutations,
  normalized payload identity, retained authority vector and receipt validation.
- `server/internal/service/collaboration_resource_offers_view.go`: current ACL,
  bounded list/detail and fixed-roster minimal summaries.
- `server/internal/handler/collaboration_resource_offers.go`: six public adapters,
  strict bodies/query/cursors, human backstop and no-store/explicit scope.
- `server/pkg/db/queries/collaboration_resource_offers.sql`: offer persistence,
  ACL-before-LIMIT reads and the provider's directory-role fence.
- Up/down migrations `732_collaboration_resource_offers`,
  `733_collaboration_resource_offer_id`,
  `734_collaboration_resource_offer_issue_history`.

Modified production files: `collaboration_resource_options.go`,
`collaboration_targets.go`, `collaboration_queue.go`,
`collaboration_resources_view.go` (only the approved operation-kind allowlist),
and `collaboration_switch_view.go` under `server/internal/service/`.

New tests:

- `server/internal/service/collaboration_resource_offers_test.go`
- `server/internal/service/collaboration_resource_offer_authority_test.go`
- `server/internal/service/collaboration_resource_offer_recovery_test.go`
- `server/internal/handler/collaboration_resource_offers_test.go`
- `server/cmd/migrate/migrate_collaboration_resource_offer_test.go`

The coordinator generated SQL, registered routes and concurrent-index cleanup,
classified the table as retained, and migrated the task-owned `service_s2` DB.
Those integrations are coordinator-owned. S3 owns later cancellation changes in
the shared queue file; this lane changes only offer receipt/action/replay fields.

### Public API handoff

All routes are below `/api/collaboration/issues/{id}/resources`:

| Method/path | Handler | Service |
| --- | --- | --- |
| GET `/offers/options?execution_workspace_id=UUID` | `GetCollaborationResourceOfferOptions` | `ResourceOfferOptions` |
| GET `/offers?limit=50&cursor=...` | `ListCollaborationResourceOffers` | `ListResourceOffers` |
| GET `/offers/{offerId}` | `GetCollaborationResourceOffer` | `GetResourceOffer` |
| POST `/offers` | `OfferCollaborationResource` | `OfferResource` |
| POST `/offers/{offerId}/withdraw` | `WithdrawCollaborationResourceOffer` | `WithdrawResourceOffer` |
| POST `/offers/{offerId}/adopt` | `AdoptCollaborationResourceOffer` | `AdoptResourceOffer` |
| GET `/operations/{requestId}` | existing `GetCollaborationResourceOperation` | existing `GetResourceOperation` |

POST uses the existing `Idempotency-Key` and positive decimal-string
`expected_lifecycle_epoch`, `expected_participant_epoch`, `expected_account_epoch`.
Supply adds `execution_workspace_id`, `kind`, `resource_id`, and complete `roster`.
Withdraw adds `expected_offer_version`; adopt adds that and
`expected_target_set_version`. The offer ID is only a path parameter. Unknown,
duplicate, malformed and authority/configuration-injection fields are rejected.
UUIDs and roster order normalize before the original-key digest.

Options retain `CollaborationResourceOptions`. `CollaborationResourceOffersPage`
returns protocol 1, issue/current lifecycle/current target-set version, `can_offer`,
items and nullable `next_cursor`. Each item and detail is
`CollaborationResourceOfferView`: protocol 1, issue/offer/version/state,
`offered_by`, typed resource/execution workspace/name, fixed `resources`, timestamps,
nullable adopted actor/target, available/reason/can_adopt/can_withdraw.
Account summaries use the existing `{user_id,name,avatar_url,external}` shape.
`CollaborationOfferedResource` includes only the roster identities, agent name,
minimal runtime and owner; no grant/configuration/installation/binding fields.

The cursor is strict Base64URL JSON of issue/viewer/before-created-at/before-ID,
at most 512 bytes. The default/max page sizes are 50/100. Service SQL applies the
owner-or-organizer predicate before LIMIT; every page reauthorizes the current user.

The existing receipt gains optional `offer_id` and exact decimal-string
`offer_version`. Success pairs are:

- `offer_resource` / `resource_offered` / 201 / version 1 / issue scope.
- `withdraw_resource_offer` / `resource_offer_withdrawn` / 200 / version 2 / issue scope.
- `adopt_resource_offer` / `resource_offer_adopted` / 201 / version 2 / target scope.

Offer-specific error codes are `invalid_resource_offer`,
`invalid_resource_offer_query`, `resource_offer_not_found`, `resource_not_owned`,
`stale_offer`, `offer_withdrawn`, and `offer_adopted`. Existing directory,
configuration, registration, owner, lifecycle, stale-authority/scope and idempotency
codes retain their meaning. Corrupt/mismatched receipts do not confirm success or
recreate work. Departed actors receive the original exact seven-field redacted
HTTP result, with no offer/target/version extensions.

### Focused behavior and concurrency evidence

- P/H/D are separate accounts. D has only the private execution membership;
  P/H have no membership there. D supplies, H adopts, then D consents through
  the existing identity/consent path. The target remains initially held/paused.
- Complete visible multi-owner squads require each owner before supply and
  adoption, and each independent consent afterward. No member target is created.
- Missing/disabled owners, lost invocation rights, participant/account epochs,
  configuration restore, runtime owner restore, binding replacement, close/reopen,
  target-set drift and duplicate typed targets refuse without half adoption.
- Display, capacity and offline observations preserve a valid offer. Closed or
  stale unadopted offers remain withdrawable by their current participating provider.
- Commit barriers prove adopt/withdraw and two organizers serialize. A provider's
  membership uses FOR SHARE NOWAIT because the inherited KEY SHARE fence does not
  prevent role updates used by inventory visibility.
- Required-write faults cover Query, QueryRow and Exec, including the final event
  after the adopted row, and preserve the full offer/control/event/operation state.
- Unknown commits execute once and recover through the original key. Receipt
  tampering, changed payload/action, departure, disabled accounts and stale password
  sessions retain existing recovery boundaries.
- Migration tests use only newly owned private schemas. They verify table/index
  separation, repeated up, constraints/no FK, exact retained history, populated
  and empty-maintenance down refusal, a real failed 733 unique build, and an actual
  cancelled concurrent 734 build followed by the registered cleanup/retry.

### Actual commands/results

Commands below ran from `server/` through
`python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s2`.
The wrapper selects the dedicated database and Go 1.26.6 without printing secrets.

```bash
bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service ./internal/handler -run '^(TestCollaborationResourceOffer.*|TestCollaborationRegistrationChecksActualResourceOwner|TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent|TestCollaborationConsentUsesCompleteSquadRoster|TestCollaborationTargetConsentRetainsInitialHold|TestCollaborationResourceOptions.*|TestCollaborationResourceHTTP.*|TestCollaborationResourceOperationRecoveryKeepsOriginalScope|TestCollaborationSwitchRecoveryAfterDepartureIsOwnAndRedacted)$' -count=1 -json
bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./cmd/migrate -run '^TestCollaborationResourceOfferMigrations' -count=1 -json
bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./cmd/migrate -run '^TestCollaborationResourceOfferMigrationsEnforceStateConstraints$' -count=1 -json
go vet ./internal/service ./internal/handler ./cmd/migrate
```

Final results: service 23 top-level and 46 nested PASS; handler 12 top-level and
3 nested PASS; migrations 4 top-level and 2 nested PASS. The additional strengthened
constraint/history-preservation check passed. All final commands exited 0; no skip.
Gofmt, changed-file diff checks, and explicit whitespace/final-newline checks passed.

Evidence files under `/tmp/multica-gap-recovery-20261021-h6phxwo9/`:

- `s2-service-handler-race.jsonl`
- `s2-migrations-race-final.jsonl`
- `s2-migration-constraints-preservation.jsonl`
- `s2-backend-delivered-sha256.json`: hashes for the 20 production/test/SQL files
  delivered by this lane; a delivery snapshot, not a claim about later S3 edits.

The first new test invocation was compile-red because the offer methods/types and
receipt fields did not yet exist. Its exact command was
`bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/service -run '^TestCollaborationResourceOffer' -count=1`
through the same wrapper. Later test preparation failures were corrected: imports/
count types, the actual `EnsureCollaborationChannel` fault point, the existing unique
active-enrollment constraint (the test now exercises real binding replacement), and
a second HTTP fixture attempting to reinsert the same account fence.

The original failed migration log remains `s2-migrations-race.jsonl`. The non-array
roster was correctly refused with PostgreSQL 22023 before the 23514 CHECK outcome
expected by that test. The assertion now accepts those explicit type/constraint
refusals and proves the retained row/catalog are unchanged. Migration SQL and the
runner were not weakened to satisfy the test.

### Remaining integration limits

Independent backend review, frontend schemas/drafts/forms and production Web/native
Electron journeys are pending at the main-session level. This backend evidence
does not establish full AC-I03/P8, real providers, other OS execution or packaging.
No provider executable/account was discovered or accessed. Gate settings, commits,
task status, specs and acceptance matrices remain coordinator responsibilities.
