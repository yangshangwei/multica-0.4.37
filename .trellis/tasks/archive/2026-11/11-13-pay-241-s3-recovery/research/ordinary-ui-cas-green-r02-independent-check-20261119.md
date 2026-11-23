# Ordinary UI CAS GREEN R02 independent acceptance — 2026-11-19

**Disposition: ACCEPTED_BOUNDED_ORDINARY_UI_CAS_GREEN_R02.** The fresh
production Web/Desktop round completed both tops and all six subcases with
retry 0: two commit orders per platform plus a separate versionless API
control. This accepts the defined ordinary UI CAS slice and the corresponding
S3 R04 boundary. It does not establish a universal missing-revision contract,
heterogeneous batch CAS, mobile behavior, installed production Main behavior,
or full S3/R09 completion.

The independent result is recorded in the companion JSON:
[ordinary-ui-cas-green-r02-independent-check-20261119.json](ordinary-ui-cas-green-r02-independent-check-20261119.json),
SHA-256 `67ba30430bd9f085e6b3d7707139b9a1893ed534e41176ca0960c66db0fc83b2`.
No test, API, database, service, or native process was re-executed by this
review. The evidence below is a read-only reconciliation of the retained raw
round.

## Round and case denominator

The Playwright result exited 0 with two expected tops, zero unexpected,
skipped, or flaky results. The persisted inventory is exactly two tops,
six subcases, one worker, and zero retries; every top and subcase is
`PASSED`. The retained prior rounds remain separate: R01 locator/admission
failure, RED R02 missing-token failure, and GREEN R01 comparison failures are
not spliced into this result.

| Platform | Priority first | Assignee first | Versionless API |
| --- | --- | --- | --- |
| Web | PASS | PASS | PASS |
| Desktop | PASS | PASS | PASS |

Across the four UI contention cases, the raw evidence records eight genuine
original PUTs and four explicit fresh PUTs. Across the two compatibility
controls, it records two direct versionless PUTs and zero UI PUTs. Four
original stale requests return exact 409 conflicts; four named winners and
four fresh continuations return 200.

## UI CAS behavior proven

Every contention subcase begins with two authenticated UI sessions reading the
same child at numeric revision `R=1`, low priority, and the original human
assignee. Both original picker requests are held before forwarding and carry
`expected_revision: 1`. Web requests show cookie plus CSRF authentication and
no bearer header; Desktop requests show bearer authentication and no cookie or
CSRF header.

For each commit order:

- the named winner returns `200` and advances the child to revision 2;
- the stale loser returns the exact `revision_conflict` body with resource ID,
  `expected_revision: 1`, and `actual_revision: 2`;
- the existing visible conflict message is shown;
- the losing client reads the authoritative revision-2 child and preserves
  the winning field and the other field;
- one explicit fresh picker request carries `expected_revision: 2`, returns
  `200`, and advances the child to revision 3 with high priority and no
  assignee; and
- both UI sessions converge on High / Unassigned without automatic resend.

Each case records two original UI PUTs, one fresh UI PUT, natural
`route.continue()` response association, fulfilled hold collection, and no
new run, command, or dispatch. The four Desktop native sessions exit with
code 0 and no signal. All Web/Desktop sessions close without page errors or
cleanup errors. The one generic Web assignee-first parent comment observer
records an `Error` qualification alongside an observed 200/aborted comment
request; this suite does not establish a new comment-recovery claim. Scoped
child traffic, CAS requests, parent DTO comparison, and cleanup remain
successful.

The two separate versionless controls each send exactly `{"priority":"high"}`
without `expected_revision`, receive `200`, and advance from revision 1 to 2.
The PUT response intentionally omits `labels`; the subsequent authoritative
GET returns `labels: []`, and the fixture compares the full GET DTO against the
full PUT DTO plus precisely that documented projection. This preserves the
legacy optional-version API boundary.

## Protected state and provenance

Every one of the six subcases captures ten before and ten after read-only SQL
collections. All 120 snapshots have identical SQL text, bindings, row values,
and row hashes before and after. Each case retains one parent, one
collaboration row, one session, four events, three operations, one handoff,
and zero commands, execution groups, runs, or dispatches. The complete parent
DTO is equal before and after in all six cases, including the authority-derived
`information_workflow.context_version`; the comparator uses the original
credential hash for both reads.

No additional database query is required for this acceptance. The fixture's
read-only SQL snapshots and final catalog are sufficient; the final catalog
reports database identity, 856 migration entries, 663 indexes, original gate
state, one expected `sys_cron_executions` startup row, and zero connections
after shutdown.

## Source, builds, native resources, and closure

The fresh admitted source/configuration/permit binds the R02 harness, GREEN
production product, 1,676 production artifact rows, and 1,096 browser/native
resource rows. Recomputed manifest checks report zero hash, mode, or target
mismatches. The API build, Web build, and Desktop build all exit 0 with zero
configuration/tool drift; Desktop's 495 generated output files are the
registered build output and its pre-existing input files are unchanged.

All three owned services have matching launch/readiness/stop PID, PGID, CWD,
configuration, and listener records. Closure records SIGTERM shutdown for API,
Web, and renderer, gate epoch 2 unchanged, 856 migrations, 663 indexes, and
zero database connections. The source/product/native build identities are
recorded in the companion JSON. The actual served client is the GREEN
production build; the API health/build stamp is `89740ed14` with its separately
bound source overlay and binary hash. A commit label alone does not identify
the dirty source candidate. Installed production Main is outside this
acceptance.

Post-cloning harness `@multica` nonmobile links resolve to the frozen RUN R02
package graph; the product's consumed schema/parser bytes match the GREEN
candidate. The root mobile link remains unconsumed and is outside this slice.
No ROOT product integration or Git commit is claimed by this report.

## Formal scope disposition

- **TC-16:** PASS for the defined current Web/Desktop ordinary field
  contention slice, together with the previously accepted collaboration field
  CAS evidence, when Main integrates the exact accepted candidate and applies
  the matrix update. This PASS covers known positive rendered revisions, exact
  winner/409 behavior, preserved losing field, visible conflict, explicit
  fresh continuation, and the separate versionless compatibility control.
- **S3 R04:** Satisfied within the reviewed ordinary optional-version boundary
  and existing collaboration R06 coverage. The requirement does not silently
  become a universal requirement for older/missing revisions or all batches.
- **Not proven here:** missing/invalid revision callers, heterogeneous selected
  batches, per-item batch CAS maps, mobile, installed production Main,
  real-model/payment/S4 inputs, final S5, and the remaining S3 R01/R08/R09
  source/integration gates.

## Retained limitations and evidence anchors

The generic operation schema attached by the shared held-request observer is
not the owning ordinary issue response schema; its diagnostic invalid fields
are retained and do not override the exact `ordinaryIssueSchema`, status,
body, and revision assertions. One Web comment observer error is retained as
a nonblocking qualification and is not used to claim comment recovery.
Checkpoint evidence names `before` and `after` are retained by the fixture;
the companion JSON carries credential hash, actor, path, status, and all
case-level hashes needed to audit the comparison.

The previous RED and failed GREEN result hashes remain preserved in the
companion JSON. This acceptance is bounded to the six newly passed subcases;
full S3 remains open until its own R01–R09 evidence and final integration
review are complete.

## Findings and verification

No blocking finding or product/fixture fix remains in this accepted scope.
Only these two independent reports were written by the reviewer. The retained
observer/source/compatibility qualifications above are not expanded into
unsupported promises.

- Lint and TypeCheck: PASS by the already accepted code/static reports and
  unchanged R05 harness checks; not rerun for this read-only acceptance.
- Actual UI tests: PASS, 2 tops / 6 subcases, retry 0, complete fresh R02.
- Independent raw/source/configuration/build/catalog/closure reconciliation:
  PASS, with the explicit scope qualifications above.
- Additional DB/API/service/native actions by this reviewer: 0.
