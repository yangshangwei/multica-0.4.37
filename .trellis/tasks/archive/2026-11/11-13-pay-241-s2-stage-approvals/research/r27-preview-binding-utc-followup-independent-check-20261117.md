# R27 binding and UTC R02 independent check

Verdict: **ACCEPTED_R27_BINDING_UTC_NORMAL_RACE** for the frozen private
`source-green-r02` only. Both actual normal and race runs pass the same eight
named identities. ROOT integration and full preview acceptance remain open.

Private root: `/Volumes/artisan/.multica-acceptance/pay241-r27-preview-20261117-x12xjirm`.
Support: `binding-green-r02-independent-check/` contains separate source/static,
allocation and raw normal/race reconciliation records. This reviewer executed
no product/test, TestMain, DB/SQL, API/browser, build/vet, process-management,
Git or subagent command. Only these new reports and private support were written.

## Findings (fixed)

- File: frozen `server/internal/service/collaboration_completion_preview.go:241`.
  Issue: the temporary guard lacked its exact submission binding. Fix by the
  implementation owner: initialize `boundSubmission` from the validated target
  before the unchanged checker. Genuine human and current-submission R38
  previews now reach `authorization_available` with empty stage blockers.
- File: the same frozen file, line287.
  Issue: final expiry retained the database's local zone, violating the existing
  RFC3339 UTC contract and failing R01. Fix by the owner: apply the existing
  `expiry.UTC().Truncate(time.Microsecond)` after the 24-hour, evidence, issuer
  and stage horizon minima, before the validity comparison. This cannot extend
  a horizon. Both current previews retain the exact saved fixture expiry and
  now serialize UTC microseconds. The original assertions remain unchanged.

No further defect was found in the reviewed binding/UTC scope. No product
edit or additional abstraction was introduced by the reviewer.

## Findings (not fixed)

- Full preview/R4 read-lock compatibility, missing-binding negatives,
  source/revocation fences, self/admin concurrency and the original four joint
  scenarios remain separate verification gates. Go race passing this serial
  selection is not evidence for those PostgreSQL lock scenarios.
- The accepted private product/tests have not been integrated into ROOT by
  this check. Main owns the three-file hash-guarded integration and ROOT
  compile/vet. Final A01–A12/S2, normal recovery, S3, real S4, final S5 and parent
  acceptance remain open. Unchanged R38 helpers do not require a new full matrix
  for this two-line product delta. R24/R25 historical holders remain unproved.

## Verification

- **Source:** all 5,050 actual files and modes match the frozen R02 manifest.
  Its only R01 delta is the normalization line; cumulative original-admin-RED
  delta is exactly the guard initializer plus normalization. Both recorded
  patches match actual bytes. Product SHA256
  `5fc937a4627c9271be1698847221be1abc4391934072ae87d117b8dc3b6b91ba`.
  The managed/admin tests retain SHA256 `f1aa77c0…` / `658bb12c…`; actual AST
  inventory remains two top-level, six leaves, eight named identities.
- **Protected behavior:** all checker, immutable row/operation/receipt/handoff,
  current credential/access/nomination, source, lock, default writer and R26
  candidate-preparation inputs remain unchanged. No dependency/interface change.
- **Lint/static analysis:** no checker execution. Retained scoped service
  `go vet` exited 0, without timeout or output; artifacts match their hashes.
- **TypeCheck:** no checker execution. Retained isolated Go compilation exited
  0, without timeout/output. Binary SHA256
  `7fe749b3c8a18212a87c9828f2af5e2514f357cf3ca793f6f4e74f40a54928a3` matches.
- **Tests:** actual normal and race each **8 PASS / 0 FAIL / 0 SKIP named**, exit0.
  Both parents and all six leaves ran once. Zero missing/extra/pending/parse/
  drift/retry. The exact same frozen source is used with separate admitted
  normal/race databases and profiles; compile's profile is recorded separately.
  Commands retain `-count=1 -timeout=10m ./internal/service` and the exact
  two-test selector; only race adds `-race`. No ambient environment inheritance.

| Leaf | Normal / race result and retained contract |
| --- | --- |
| Human `enforce_authorization_available` | PASS / PASS: exact current human binding, canonical observation, saved UTC expiry |
| `policy_off_non_authorizing` | PASS / PASS: off; no binding/observation/expiry |
| `stale_target` | PASS / PASS: original 409 `delivery_completion_conflict` |
| `wrong_actor` | PASS / PASS: original 403 `delivery_completion_denied` |
| `expired_credential` | PASS / PASS: original 403 `delivery_completion_denied` |
| R38 `admin_available` | PASS / PASS: exact admin/org/current-submission binding and saved UTC expiry |

Both genuine service fixture paths retain real `SubmitAcceptance`, fresh target
construction and actual borrowed preview transactions with final credentials
and commit/rollback. R38 independently establishes current nomination, source
artifact visibility, real administrator password/role/org and actual creator
disable/revocation. No fake ready response, SQL-seeded formal submission, new
M1 grant or administrative lifecycle completion is introduced. Simulated human
signatures and external facts remain explicitly synthetic fixture evidence.

All twelve normal/race leaves have equal complete 72-relation plus stage-exit
hashes. Admin/control and adapter call/target/invocation assertions report no
change. Authorization/terminal/evaluation rows stay zero; no original PgError.
Non-permission flags remain false; tested denials retain no private result. All 5,057
inputs in each permit and raw/result/profile/static references match. Every
before/after database has other connections 0 and unchanged ledger/indexes.
Six setup handoffs and configuration/knowledge invalidation growth after fixture
cleanup stay recorded; this is not a whole-database or archive-byte audit.

**Allocation support is separate from business RED.** Green2 preparation exited
1 because the final aggregate filename collided with its already-written
individual allocation after CREATE/clone/audits completed. The original failure
and script remain unchanged; allocation was not replayed. The new read-only
confirmation, original clone, unchanged donor and actual pre-test snapshot have
the same database/profile identity, ledger, indexes and complete table counts;
fixture security/business rows are empty with seed singleton/history rows retained.

Initial managed/admin RED, R01 UTC-failed normal, sealed R01 NOT_ACCEPTED reports
and its unused race permit remain preserved. Parent remains
**0 PASS / 9 PARTIAL / 15 NOT_RUN**; this review closes only binding plus UTC R02.
