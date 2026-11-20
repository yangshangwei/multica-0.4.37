# Ordinary UI CAS RED R02 independent check — 2026-11-19

**ACCEPTED AS THE INTENDED MISSING-REVISION UI RED.** The complete invocation
still correctly exits 1: both platform tops fail, two priority-first subcases
fail at the expected revision assertion, and four later subcases remain
NOT_RUN. This is a valid baseline defect witness, not a passing feature round
or TC-16 acceptance. The earlier R01 locator/admission failure remains separate
and unchanged.

This reviewer read retained files only and wrote this report. No DB/API,
service, native process, test, source, configuration or history mutation was
performed. Campaign `C` denotes
`/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe`.

## Genuine request evidence and exact failure

Both real UI sessions on each platform received HTTP 200 for the same
ordinary child at numeric revision 1, low priority and the original human
assignee. Both displayed those fields. The fixed assignee locator reached the
actual control; each picker click completed and each original PUT was captured
before either was forwarded.

| Platform | Actual held priority body | Actual held assignee body | Recorded failure |
| --- | --- | --- | --- |
| Web | `{"priority":"high"}` | `{"assignee_type":null,"assignee_id":null}` | fixture line 262: missing `expected_revision: 1` on priority |
| Desktop | same priority body | same assignee body | same exact assertion at line 262 |

All four bodies omit `expected_revision`. Their SHA-256 values match the saved
body records. Web requests carry actual cookies/CSRF and no bearer header;
Desktop requests carry bearer authentication. Each original request targets
the UUID of its observed child. The fixture uses genuine picker callbacks and
the existing held-route helper; it does not inject a token or synthesize a UI
PUT.

The first assertion fails before the following assignee assertion and before
`winner.forward()`. The assignee omission is directly observed in the captured
body, not claimed as a second reached assertion. All four held results remain
null, all four releases and clicks are fulfilled, and the requests are aborted
by `HeldRequest.close()` during cleanup. There are zero forwarded field PUT
results, zero explicit fresh PUTs and zero versionless API controls in RED2.
No 200/409 competition or silent overwrite is inferred from aborted requests.

The whole reporter has two unexpected failures, zero expected/skipped/flaky
tops, one result per top and retry 0. Playwright replaced worker 0 after the
Web failure with worker 1 for Desktop; configured concurrency remains one and
this is not a retry. The complete six-subcase inventory is preserved:

| Platform | priority-first | assignee-first | versionlessAPI |
| --- | --- | --- | --- |
| Web | FAILED_OR_INCOMPLETE | NOT_RUN | NOT_RUN |
| Desktop | FAILED_OR_INCOMPLETE | NOT_RUN | NOT_RUN |

## Admission, source and closure

The accepted admission predates test startup: `12:45:29Z` versus
`12:46:29Z`. It binds the original runtime permit, exact database, actual
API health PID/commit/deployment and production Web/renderer readiness. The
startup comparison retains identical ledger/index/gate values and only
`sys_cron_executions` count 0→1. The other selected table counts remain equal
before the body. This is the new admitted R02, not the earlier failed
newline-sequencing path.

The separate Main helper `resume-ui-environment-r01.py` validates recorded
hashes/modes/targets before startup and again after service readiness, then
writes the admission. Its shutdown validates the manifests again. The original
runner and source are unchanged. Recomputed evidence matches:

- 3,646 harness source entries before/after/current;
- 1,573 RED production artifact hashes, with the same two previously disclosed
  rows lacking historical mode fields;
- 1,096 browser/native resource entries;
- runtime permit, closed configuration, actual argv, recorder and tool hashes;
- complete stdout/stderr hashes, with empty stderr and recorded source,
  configuration and tool drift all zero.

All four UI sessions closed pages and contexts with no cleanup/page errors.
The two actual Desktop processes, PIDs 89617 and 89633, exited normally with
code 0 and no signal. Their retained failed PUTs are expected route-abort
observations. Other `ERR_ABORTED` GETs and two Web `/api/client-usage` POSTs
remain in the raw evidence; they are not reclassified as field writes.

Owned closure at `12:47:28Z` matches the original launch/ready/stopped PID,
PGID and configuration hashes for API 88994, Web 88999 and renderer 89015.
Each stop receipt records SIGTERM and owned listener absence. The retained
post-stop audit shows zero other database connections, 856 migration entries,
663 indexes and the unchanged gate: epoch 2, creation true, execution/host
false and no freeze. Failed fixtures and their history are retained.

## Protected business facts and observed credential projection

For both entered subcases, the ten before/after SQL collections match exactly
by SQL text, bindings and full returned rows. All forty SQL snapshot hashes
match their actual row bytes. Per platform the retained counts are one parent,
one collaboration, one session, four events, three issue-scoped operations,
one handoff, and zero commands/groups/runs/dispatches. Thus the selected
persisted parent/history/execution facts are unchanged.

The Web parent DTO is also exactly equal. The Desktop DTO differs only at
`information_workflow.context_version`:

- before: `9353a9080c74696076909787948c6c3342719cb550abf6c68c76e28d5ce07a17`;
- after: `994cee56cead579f5567aa2c02cb0759b34fd09ef244e3420da99ecb74788a78`.

All other parent DTO fields are equal. Do not claim full DTO equality for this
RED run. The fixture rethrows its primary missing-revision error before its
final equality assertion; the observed derived-field difference is a
separate next-round concern, not the recorded RED failure.

## GREEN R01 follow-up diagnosis requested by Main

The later complete GREEN R01 also remains failed: zero passing tops, two
passing subcases, two failed subcases and two NOT_RUN subcases. Its raw stdout
hash matches the recorded result. Web passed both genuine UI orderings, then
the versionless API control received 200 but failed full GET-versus-PUT
equality because GET adds `labels: []`. Desktop completed all three correct
priority-first PUTs and visible convergence, then failed the final parent DTO
equality only on `information_workflow.context_version`. Each of its four
entered subcases retains unchanged ten-collection SQL fingerprints. Partial
GREEN progress must not be spliced into a whole accepted round.

### Credential-sensitive context is compared across different logins

The actual source explains the comparison boundary:

1. `exerciseRecoveryOrdinaryUI` reads `parentBefore` using
   `fixture.recovery.creator.client` before opening UI sessions.
2. Desktop `openSession` calls `actor.login()` for each independently
   authenticated session (`collaboration-recovery.ts:290`).
3. `passwordActor.login` calls the shared `client.loginPassword`
   (`collaboration-offline-handoff.ts:33`), which replaces that client's
   private bearer token (`e2e/fixtures.ts:66`). Web's `authenticateWeb` instead
   uses `page.request` and does not replace the fixture API client's token.
4. `informationAuthorityVersion` includes the current credential kind and
   expiry, password version and PAT generation, alongside actor/current
   authority/member facts
   (`server/internal/service/collaboration_information_state.go:316`).
5. `loadInformationState` incorporates that authority fingerprint into
   `context_version` (`:418`–`:449`). Therefore a new authenticated credential
   may change this read-context token without changing the protected issue or
   history rows.

Both reads have the same user but the fixture intentionally changes the
credential between them on Desktop. Exact original JWT expiry values were not
retained, so this report does not invent a cryptographic replay of either
context digest. The source chain, the unique raw field difference and the
unchanged selected persisted facts establish the mismatched observation
boundary; a fresh run with a pinned comparator credential should verify the
correction.

**Recommended narrow fix:** capture the current creator token in a local
closure before `parentBefore`, then use that same credential for both
read-only parent GETs. Require HTTP 200, parse through
`CollaborationIssueSchema`, and retain full DTO equality including
`context_version` plus all ten SQL fingerprint comparisons. Save only a
credential hash/actor identifier as evidence, never the token. Do not alter
the real UI logins or field request authentication.

Use an explicit read-only `fetch` against `fixture.apiUrl` for this comparator.
Passing an Authorization override through `creator.client.requestJSON` does
not pin it: `TestApiClient.authedFetch` unconditionally overwrites the supplied
header with its current private token (`e2e/fixtures.ts:482`). A direct GET is
an audit comparator, not UI-write proof; the existing genuine PUT denominator
and exact response association remain unchanged. There is no need to weaken
the product authority fingerprint or drop the full parent DTO assertion.

### GET labels are an authoritative projection; PUT intentionally omits them

`server/internal/handler/issue.go:103`–`:109` documents the existing contract:
list/detail load labels, while `UpdateIssue` and its broadcast intentionally
omit them so cache merge preserves known labels. `GetIssue` explicitly
attaches an empty list when no labels exist (`:2365`), while `UpdateIssue`
starts with `issueToResponse` and does not load labels (`:4451`). The GREEN
failure reports exactly this additive difference and no other field.

**Recommended narrow fix:** retain the original PUT and GET bodies; require
the PUT to omit `labels` and the authoritative GET to contain `labels: []` for
this deliberately unlabelled fixture. Compare every remaining parsed DTO
field exactly. Do not switch to a broad `toMatchObject` or suppress other
differences; keep numeric revision, field values, ID and parent assertions.
This is a fixture contract correction, not a product change.

## Findings fixed / not fixed

No source or runtime fix was made by this reviewer. The two fixture comparison
corrections above are recommendations to their existing owner. Main owns fresh
source/config/database/permit binding and the next complete GREEN round.
Original R01, RED2 and GREEN R01 raw results and all unstarted subcases remain
unchanged. TC-16 and full S3 remain open.

## Verification anchors

| Retained artifact under C | SHA-256 |
| --- | --- |
| `verification/ui-red-r02/result.json` | `97420f34d4aa9a0c74b0b90513ae6c7b245e4f102dba12ddb5f0aee0d3d0ff10` |
| `verification/ui-red-r02/stdout.log` | `453228eb0540186e32aa75dc72fc50e5e0c84ecda234f730206a516ffe8e8662` |
| `verification/ui-red-r02/playwright-output/ordinary-ui-cas-round-inventory.json` | `67dc9c2d0c75488b31d8e449dbb2862a7677e7e9d8c33ed8040d63feaf61812e` |
| `audit/ui-red-r02-admission.json` | `7c5db657e8fb089a11c3300aa7c82ecd0619c1b36587a2e7dfb56b225d390c5f` |
| `audit/ui-red-r02-owned-closure.json` | `29ffcf196b78021078509509e3b0b57f86c29af45a452e3e2f15f1a7302790af` |
| `audit/ui-red-r02-after-owned-shutdown.json` | `da1665ddf6e22179e8ef5ed2ef6902de52e693f4fcf4fcc5a95360c439f71c86` |
| `verification/ui-green-r01/result.json` | `d42f09e3f3e70d13c1684142f2232141428319f81a7932993ec95f95c286bebd` |

Lint and typecheck were not rerun because this assignment changed only this
report. Accepted code/static results remain attributed to the prior independent
check. The actual RED2 test result is **EXPECTED FAIL / valid RED**, while
GREEN R01 is **NOT ACCEPTED**. This review establishes neither a successful
complete UI contention round nor final S3 acceptance.

## R05 fixture correction clearance for full GREEN R02

**SCOPED CODE/STATIC CLEARANCE: no blocking finding.** This later read-only
follow-up compares frozen `C/source/client-ui-harness-work-r05` with the
preserved `client-ui-harness-green-run-r01`. The complete 3,646-file source
comparison has exactly one changed file and no mode changes:
`e2e/fixtures/collaboration-recovery-ordinary-ui.ts`, SHA-256
`fea2807c7c27c74bc9f7ea440081e58e04fb0dee1acc8312a18b1069bdd26fa9`
(prior SHA `d8441e916aecc36a09725ed8e9f9108dbaceeef68b64c34f7a04df48197db977`).

The change implements both recommended corrections without weakening the
selected invariants:

- It captures the original comparator token before UI sessions open and uses
  the same explicit read-only GET closure before and after. Both reads require
  HTTP 200 and `CollaborationIssueSchema`; evidence retains actor, credential
  hash, path and status only. Native session authentication remains unchanged.
- Full `parentAfter` equality with `parentBefore`, including
  `information_workflow.context_version`, and all ten SQL before/after
  collection comparisons remain unchanged.
- The versionless API control requires raw PUT `labels` absence, retains the
  authoritative GET, requires `labels: []`, and compares the entire parsed GET
  with the entire parsed PUT plus exactly that documented projection. All
  other fields remain exact.
- Genuine picker requests, original holds/response association, both numeric
  revisions, exact 200/409 result, visible conflict, explicit fresh action,
  request counts, timeouts, cleanup and the two-top/six-subcase inventory are
  unchanged.

The retained `e2e-cas-ts-r05`, `e2e-cas-lint-r05` and
`e2e-cas-discovery-r05` commands all exit 0; raw stdout/stderr, current config
and tool hashes match. Their source/config/tool drift is empty. Strict
TypeScript and zero-warning lint pass; discovery lists the same two tops in
one file. Current WORK R05 exactly matches the static command's complete
before/after snapshot. This reviewer did not replay any check or runtime body.

Main may bind these frozen bytes to the next fresh full GREEN R02
source/config/database/permit and perform its normal runtime admission.
This clearance does not accept an unexecuted GREEN round or change any prior
failure. All original RED2/GREEN R01 evidence remains immutable, and no product
edit is required by this fixture correction.
