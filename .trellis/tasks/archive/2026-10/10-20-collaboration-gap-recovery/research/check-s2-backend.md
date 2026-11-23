# Independent S2 backend check — 2026-10-21

Status: **backend source review passed; no concrete findings require a fix.** This is the bounded resource-offer backend review, not full AC-I03, Web/Desktop or parent-task acceptance.

## Scope and attribution

Reviewed the S2 offer contract and backend implementation record, offer mutations/read projections/options, HTTP handlers, transaction-local target registration, original-operation recovery, SQL, migrations 732–734, and coordinator-owned route/index-cleanup/retention integration. S1/S3a were not re-reviewed. Concurrent S3 cancellation code and migrations 735/736 are outside this review.

The delivered SHA-256 manifest at `/tmp/multica-gap-recovery-20261021-h6phxwo9/s2-backend-delivered-sha256.json` matches **19 of 20 files**. The one changed file is `collaboration_queue.go`, whose newer cancellation work belongs to S3. The S2 receipt fields, action initialization and replay validation in that shared file were inspected directly; unrelated cancellation changes were not attributed to S2. No product source, generated SQL, schema, gate or dependency was changed by this reviewer.

## Findings (fixed)

None. No mechanical corrections were needed.

## Findings (not fixed)

None within this backend review scope. Frontend integration, executable spec updates and product-level acceptance remain coordinator work, as recorded below.

## Contract checks

- **Actual ownership and directory authority:** offer candidates and direct POST share the trusted adapter predicate for agent/runtime visibility. Supply independently verifies that the caller owns at least one actual runtime, checks every selected agent's current runtime/workspace/owner, and requires the complete visible squad roster. `collaborationConfigurationSnapshot` preserves each actual owner's invoke and execution-workspace membership requirements. An agent owner, directory administrator or caller-supplied roster cannot replace actual runtime ownership.
- **Role separation:** the source-hidden human participant can supply from their own authorized execution inventory. Only a current creator/handler can adopt. Adoption reads the explicitly disclosed offer and does not require the organizer to join its execution workspace. The original organizer options endpoint retains its organizer and execution-membership checks. Ordinary source readers and unrelated participants do not gain pending-offer reads.
- **Locked, non-restoring authority:** the existing queue transaction locks the current participant user set and source/session aggregate, then the offer/resources/identities. Supply additionally takes `FOR SHARE NOWAIT` on the provider's execution membership because catalog visibility can depend on its role. Every resource records exact owner account/participant generations and managed/enrolled binding identity alongside the configuration fingerprint. Rejoin, account-generation change, restored configuration/ownership, rebind and close/reopen cannot revive an old offer. Current read projections are read-only; no GET repairs or refreshes the retained authority vector.
- **Atomic adoption and independent consent:** both old registration and adoption call the same `t.registerTarget` callback inside the caller-owned transaction. It preserves organizer authority, target-set confirmation, duplicate typed-target rejection, owner/configuration checks, initial hold, channel creation and target event. Offer CAS, adopted actor/target/version, offer event and original operation commit with those writes. Supply creates no target; adoption creates no grants or execution. Every actual owner still uses the existing consent path. No automatic Resume, Stop or Switch was added.
- **Withdrawal and history:** only the current participating provider can withdraw their own offered row using its exact version. A stale configuration or closed issue does not prevent withdrawal. Adopted and withdrawn rows cannot return to offered state through these writers, and adopted withdrawal cannot bypass grant revocation. The original roster and disclosed identities remain immutable; a later squad member is not included when reading old offers.
- **Reads and disclosure:** list and detail use the authorized repeatable-read content transaction, then require participant control authority. SQL applies issue plus provider-or-current-organizer ACL **before LIMIT**. Cursor parsing rejects duplicate/unknown fields, malformed encoding, wrong issue/viewer and invalid limits; every page reauthorizes. Responses project only the frozen roster's minimal labels/identities. Configuration fingerprints, internal authority vectors, installation/binding credentials, grants and unrelated private inventory do not enter offer DTOs.
- **Original operations:** public input UUIDs and roster ordering normalize before hashing; each mutation includes its action/object/version in the existing actor/issue/request payload. Same-key payload/action drift refuses, lost commits are not automatically retried, and successful replay returns the original outcome before reevaluating later resource changes. New receipt validation checks action/status/code/scope and offer identity/version shape. Current actor authority is checked before recovery; departed actors receive the existing explicit seven-field HTTP DTO without offer/target/version extensions. Disabled or stale-password actors remain denied.
- **Integration and migrations:** all six routes are in the existing authenticated collaboration group and use the human handler backstop, no-store and explicit issue scope with ambient workspace headers cleared. Migrations 732–734 are additive, with no foreign keys or inline indexes. Both indexes use separate single-statement concurrent migrations and have exact cleanup-map entries in `cmd/migrate/main.go`. The workspace deletion manifest classifies `collaboration_resource_offer` as retained. All three down files explicitly refuse reversal, including empty-maintenance downgrade; no destructive fallback is implied.

## Verification

Reviewer-run static checks on the current tree:

| Check | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.6 go vet ./internal/service ./internal/handler ./cmd/migrate`, from `server/` | **PASS**, exit 0, 4.502s; includes Go compilation/type analysis |
| `gofmt -l` for delivered S2 Go files, excluding the concurrently edited shared queue | **PASS**, no output |
| Scoped `git diff --check` | **PASS**, exit 0 |
| Explicit final-newline/trailing-whitespace checks for delivered files outside the shared queue | **PASS** |

The vet command and output are saved in `/tmp/multica-gap-recovery-20261021-h6phxwo9/check-s2-backend/vet.json` and `vet.log`. This backend assignment has no TypeScript changes, so no frontend typecheck is claimed here.

The implementation's focused race evidence was read and counted from the actual JSONL, rather than rerunning broad suites without a new failure or source change:

| Evidence | Verified result |
| --- | --- |
| `s2-service-handler-race.jsonl` | Service: **23 top-level / 46 nested PASS**, 15.811s; handler: **12 top-level / 3 nested PASS**, 2.789s |
| `s2-migrations-race-final.jsonl` | Migrations: **4 top-level / 2 nested PASS**, 2.930s |
| `s2-migration-constraints-preservation.jsonl` | Strengthened constraint/history case: **1 top-level PASS**, 1.726s; this is a repeat/strengthening, not a fifth distinct migration test |

All three final evidence files contain zero failing/skipped tests and successful package results. Their hashes, exact top-level test identities and counts are in `check-s2-backend/evidence-summary.json` under the same external task directory. These are the implementer's executions against the task-owned `service_s2` database, with matching delivered source hashes, inspected by this reviewer. The coordinator subsequently confirmed that `service_s2` has the additive S3 schema through 736; this review did not perform additional database mutations or tests.

The inspected canonical tests cover separate provider/creator/handler accounts without cross-space membership; real HTTP inventory predicates and hidden squad members; every owner's authority and independent consent; stale/non-restoring offers; role and competing adoption/withdrawal commit fences; ACL-before-LIMIT and viewer-bound cursors; exact redacted recovery; required-write rollback across Query/QueryRow/Exec including the final adopted event; unknown commit and corrupted receipt cases; original registration/consent/hold/inventory behavior; and real failed/interrupted concurrent-index cleanup with retained-row equality.

The initial eight existing registration/consent/hold/inventory tests passed before the callback extraction, as recorded in the implementation report. The new tests' initial missing-method/type compile-red and the migration test's 22023-versus-23514 expectation correction are kept as development history, not relabeled as independent product failures or extra final passes.

## Handoff and limits

- Repository file created by this reviewer: this report only. No commits, archival, provider execution, environment-mode changes or rollout gate changes occurred.
- Coordinator spec synchronization should document the new offer routes, provider/organizer split, frozen owner/configuration/binding facts, terminal/stale withdrawal, retained history, original-operation recovery and membership-role fence in the existing resource collaboration contract. No spec files were edited in this assignment.
- The bounded S2 backend increment is ready for integration. Frontend schemas/drafts/forms, actual production Web and Electron journeys, visual review, revocation across open clients, packaged-client/provider/OS evidence and full AC-I03/P8 remain separate requirements. **The 39 distinct backend top-level tests do not prove complete two-platform product acceptance.**
