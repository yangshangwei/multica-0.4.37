# Independent P1 review: ordinary authorization and schema

Date: 2026-11-11, Asia/Shanghai. Reviewer: native `trellis-check` /
`check_resume_evidence`. Intake HEAD: `fb0558881`.

## Verdict

**REVIEW_FIXES_VERIFIED_FINAL_P1_VALIDATION_PENDING.** Three concrete defects
were reproduced and corrected with retained red/green evidence. One consumer
permission mismatch and final managed/normal/race acceptance remain assigned to
the main session and implementation owners. This is not a full P1 or gate PASS.

The review began against the 30-file P1 snapshot plus the three-file explicit
issuer prerequisite. Later collaboration/VCS union edits are separate P3 work;
this report does not certify that entire changing scope. Original P0 and prior
P1 source/run records remain unchanged.

## Findings (fixed)

### P1-R1: readiness did not verify the actual schema contract

The original runtime query and migration 788 checked five index shapes but only
the names/validated flags of two CHECKs. Replacing either CHECK with same-name
`CHECK (true)`, removing target/consumer/issuer checks, dropping required NOT NULL
or changing a column type still returned ready.

- Red: `p1-schema-red01`, seven failing scenarios plus parent, on a new permitted
  migration database. Each observed actual readiness `true` where `false` was
  required. The original PostgreSQL catalog definitions were logged before DDL.
- Fix: the single runtime predicate now checks exact ordered column names/types/
  nullability/defaults for all 73 authorization and 22 terminal columns, every
  named validated CHECK definition (52 and 14), and the existing five exact
  index definitions. Missing/additional/weakened/unvalidated structure refuses.
  Migration 788 embeds that exact predicate; a static contract compares it.
  Migration 781's actual table definition was not rewritten.
- Green: `p1-schema-green01`, 17 named passes, zero failures/skips. This includes
  the old seven migration results plus nine negative cases and their parent;
  later cases also cover terminal clock default drift and an unvalidated CHECK.

Fingerprints use PostgreSQL 17 catalog rendering. They intentionally fail closed
if a legitimate PostgreSQL upgrade changes that rendering. Recovery requires
verifying the new real catalog and introducing/updating the explicit schema
contract through a reviewed migration; never bypass readiness or delete retained
history to make a downgrade pass. They are schema-integrity fingerprints, not
signatures for hostile SQL operators.

### P1-R2: typed expiry could create an unrecoverable authorization

The HTTP parser normalizes UTC expiry to microseconds. Direct typed service
requests previously passed Validate with a non-UTC zone or nanoseconds, storing
a request digest that differed from strict recovery after PostgreSQL precision
normalization.

- Red: `p1-review-expiry-red01`, two failing cases plus parent.
- Fix: typed Validate rejects non-UTC and non-microsecond expiry. It does not
  silently rewrite a signed internal intent. HTTP keeps its existing UTC
  normalization before validation.
- Green: `p1-review-contract-green03`, 23 named passes, zero failures/skips:
  original type contracts, new expiry cases/HTTP positive and migration statics.

### P1-R3: expiry could pass before the final SQL write

Finish checked the consumer before potentially lengthy evidence reads, then
checked grant/evidence time in Go and executed an unconditional terminal INSERT.
An actual consumer JWT or the authorization could expire before that INSERT and
still produce a consumed result.

- Red: `p1-review-late-expiry-red01`, two failing cases plus parent. An actual
  transaction wrapper delayed only the real terminal INSERT until the database
  clock passed the bound expiry; it did not fabricate SQL results. Both cases
  had a real controlled artifact, valid preparation and actual status update.
- Fix owned by reviewer: conditional terminal INSERT rechecks `clock_timestamp()`
  against the original authorization/evidence/issuer finite expiry and the
  actual consumer expiry. A zero-row result is a conflict and the writer must
  roll back. Revocation remains permitted after expiry.
- Complementary fix owned by the core implementer: recheck actual consumer and
  issuer after observation, then derive consumer expiry from the actual JWT,
  verified task token or freshly locked PAT row. Required missing JWT/task expiry
  refuses; a genuinely unbounded PAT or integration remains nullable.
- Green: `p1-review-late-expiry-green01`, three named passes, zero failures/skips.
  Both actual late INSERT paths now refuse and the test confirms rollback leaves
  the old status and no terminal fact. Red and green used separate fresh databases
  and separately compiled, source-frozen handler binaries.

## Findings (not fixed in this review)

- **P1-R4, consumer permission mismatch:** `completionFreezeConsumer` and the
  ordinary task consumer check require Operator, while the ordinary completion
  writer permits Contributor. This accidentally applies a producer requirement
  to a separately designated consumer. Main explicitly adopted the original
  writer permission intersection and assigned the implementation owner to change
  only consumer checks after a real regression with distinct Operator producer
  and Contributor consumer. The observer's producer Operator rule and restricted
  reporter refusal must remain. This was still pending at report time.
- **Managed execution evidence:** task/runtime/namespace locks, immutable
  ExecutionBindingID/Epoch, explicit absence and token binding comparisons were
  inspected. A complete real managed-task signing/consumption positive and
  binding/absence/revocation races were not run by the original P1 lane. The
  existing explicit-issuer helper test proves cross-subject password isolation,
  not the whole managed consumer route or writer. Root must finish that evidence.
- **Final coherent validation:** the original 22 handler, seven migration and
  11 router results are iterative snapshots. New P3 core changes cannot inherit
  them as final evidence. Current normal/race suites, whole build/vet, sqlc
  idempotence, source freeze and integration review remain with root.

## Other inspected boundaries

Typed request variants and strict parsing keep foreign evaluation keys out of
authorization. The grant records a distinct issuer credential snapshot and the
consumer uses its real current credential; no issuer authentication context is
manufactured. Stored request and observation fields are revalidated before new
admission. The original consumed-operation branch precedes fresh done/done-edit
admission, so replay returns its minimal receipt rather than another status,
notification or Stop write.

Ordinary GET/revoke use current resource access and expose only minimal grant
and completion facts; private request JSON, observations and credential metadata
stay internal. Revocation and consumption serialize on the authorization row
and one terminal unique index. Same-transaction Finish checks actual persisted
state and leaves failure rollback to its caller. No automatic uncertain retry,
usable skip or public enforce enablement follows from this package.

The explicit issuer prerequisite correctly separates the passed issuer session
from the ambient consumer session while sharing the original password-version
algorithm. Legacy `CheckPasswordVersion` keeps its ambient managed binding
checks. Its retained actual red/green records were independently rehashed and
parsed (one behavioral subcase plus parent failed, then eight named passed).

## Reviewer-owned changes

- `server/cmd/migrate/delivery_completion_test.go`: real schema degradation
  matrix, canonical catalog capture and migration/runtime refusal checks.
- `server/internal/migrations/delivery_completion_test.go`: static equality of
  migration readiness and the runtime predicate.
- `server/internal/service/delivery_completion_types.go` and its test: canonical
  typed expiry refusal and unchanged HTTP normalization control.
- `server/internal/handler/delivery_completion_authorization_test.go`: actual
  final-INSERT expiry races and rollback assertions.
- `server/pkg/db/queries/delivery_completion.sql` and generated completion file:
  exact readiness and SQL clock consumption fence with `ConsumerExpiresAt`.
- `server/migrations/788_delivery_completion_ready.up.sql`: same exact runtime
  predicate at migration readiness.

Unified sqlc generation also regenerated ready GitHub/VCS trust-revision queries
and models from other owners' source. Those business changes are not attributed
to this reviewer and need their own review. Exact current hashes for the eight
reviewer-modified files and generated handoff outputs are in the JSON companion.
Query/generated ownership is released back to root after this report.

## Verification and retained failures

Rehashed and parsed 19 relevant run records and their original logs; named
inventories, counts, hashes and one terminal per started name agree. Both handler
test compilation records had unchanged source before/after. Reviewer commands
used the private launcher, unique IDs and only the four explicitly assigned fresh
databases. No database was allocated, dropped, borrowed or restarted by reviewer.

- Actual schema regression: 17 pass, 0 fail/skip.
- Pure type/static contracts: 23 pass, 0 fail/skip.
- Actual final-INSERT expiry regression: 3 pass, 0 fail/skip.
- `p1-review-build01`: full backend build passed before later P3 changes.
- Owned Go formatting and scoped diff whitespace: pass.
- Whole current vet and final ordinary/race acceptance: pending root, not claimed.

Two further failures remain intact: `p1-review-contract-green01` had 22 pass and
one reviewer test-path failure because `ResolveDir` selected the source directory;
the test now uses existing `realMigrationsDir`. `p1-review-contract-green02` ran
two migration static passes, while service test compilation encountered another
lane's temporary unused `bytes` import in GitHub code. Neither is behavioral red
or a green whole run. Tool output classification briefly obscured the path
diagnostic; the original launcher log remains the exact evidence.

No provider call, dependency change, commit, push, deployment, external message
or task archival was performed. This report must not close P1, the complete
delivery gate, M1 or the six-business parent.
