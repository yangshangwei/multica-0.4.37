# Controlled artifact API implementation lane

Implemented typed create, current-authority restricted detail, one-time claim and retained receipt; bound human digest decisions/cancellation and legacy self-report refusal. Configuration is server-owned, default disabled. No UI or generic command execution was added.

Authority uses workspace deletion fence, live token and joined subject locks, supplemental independently active human user/member rows, password/PAT and managed-binding locks with NOWAIT. Current JWT expiry is propagated from verified claims and checked again after staging validation immediately before writes. Agent detail and replay revalidate originator invocation permissions. Internal result finalization is nonce-bound and independent of deleted workspace/task/principal rows.

After definite claim commit the original server operation has a 120-second context independent of client disconnect. Only this original owner may call the adapter; no future request takes over. Any finalization error after the adapter ran returns 503/unknown with execution identity, including lock contention. Preclaim conflicts remain 409 with zero invocation. A subsequent authorized read may recover an actually committed final receipt.

## Verification history

- compile-red.log: new handler field and method were absent, expected compilation failure before implementation.
- api-handler-01: fixture lacked accountable_user_id while supplying originator; 1 pass/22 failures. Fixed fixture, not constraints.
- api-handler-02: 20 pass/3 failures (replay expected 201 instead of documented 200; expiry fixture violated exact15-minute schema check). Fixed test contract.
- api-handler-03: 23 pass/0 fail/0 skip.
- api-handler-04: 39 pass/0 fail/0 skip, including actual workspace deletion in eight lifecycle states and finalization after deletion.
- api-handler-05: 47 pass/0 fail/0 skip, including five commit uncertainty injection cases and same-key concurrent creation.
- api-handler-06: 51 pass/4 failures: credential-revocation test reused consumed request body on its second call. Rebuilt request retaining verified context.
- api-handler-07: 66 pass/0 fail/0 skip (includes eleven original legacy approval tests).
- api-handler-race-01: same66 named pass, no failures/skips. Included terminal-task creation replay retained identity.
- api-review-regressions-01: two race regressions pass after originator-invocation reader fix; initial finalization contention expectation409 was later corrected to503 after review, so this record is not final503 evidence.
- api-admission-race-02: eleven named race passes, actual Execute precommit barrier and physical packager count for independently acting human, password/PAT/task-token and workspace writers; actual Create-before-DeleteWorkspace fence.

Original failed records remain under parent research/ with private log hashes. Reviewer-found invocation-reader bug was corrected with a positive post-fix regression; no pre-fix failing test run was preserved for that finding. Finalization503 classification and distinct-originator member revocation both pass in api-review-regressions-final (race2/0/0). Main will run the definitive combined suites in fresh final databases.

## Verification boundaries

All handler runs use the parent's explicit task-owned PostgreSQL environment and CLI guard. Adapter positive paths use real confined filesystem packaging, not an invented output. Commit uncertainty wrappers wrap actual pgx commits (before/after), and the between-update failure proves transaction rollback. Same-key race permits documented NOWAIT409 losers and verifies the surviving original request on retry. Existing direct-SQL reverse-lock tests are supplemental; actual Execute/physical invocation barriers provide the acceptance proof.

Router auth and managed-binding tests belong to the database/router lane. The main session owns independent review, full build/vet/static checks, final database suites, spec, integration and commit. This lane does not claim M1 or real-provider acceptance complete.
