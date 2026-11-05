# Independent CI / database precondition review

Reviewer: native `trellis-check` (`p1_independent_check`), independent of implementation. Scope is the nine CI/TestMain files and the updated database/offline/docs verification contracts. Worktree: `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`. Reviewed source hashes: `research/independent-reviewed-sources.json`.

## Findings fixed

None. No code change was necessary in this review.

## Findings not fixed

No blocking correctness or scope issue found in the reviewed diff.

The following verification limits remain open as explicitly declared follow-up, not passing results:

- The healthy PostgreSQL fixture lifecycle and testutil seed-error branch were not exercised with a server. The code retains the successful fixture contexts and changes explicit seed failure to exit 2, but negative startup tests do not prove a full DB suite succeeds.
- The complete hosted Release job (Go 1.26.x with real ephemeral PG17/Redis) has not run locally. Local Go 1.27 compilation and configuration checks do not substitute for it.
- Per-test database skips and whole-suite expected-test inventory are outside this bounded fix. Optional testutil runs intentionally contain an explicit skipped DB builder when no database is configured.

## Boundary review

- The complete test wrapper rejects blank configuration before `go list`, preserving the existing package split, serialization, timeouts and agent CLI guard.
- Handler and router require DATABASE_URL, fail with exit 2 for parsing/connection failure and no longer silently pass. The 5-second Ping context is canceled independently; the normal fixture lifecycle still receives its original context.
- Testutil has no localhost fallback. No configured URL runs only pure helpers/explicit DB skips; configured parse, connection and seed errors terminate nonzero with fixed diagnostics.
- The offline Node matrix compiles actual binaries with `go test -c`; subsequent `-test.run=^$` is deliberately testing TestMain refusal, not used as a compile-only claim. The child environment strips PG*/DB/Redis settings and uses a nonexistent private Unix socket, a nonexistent pass/service file and forced read-only PGOPTIONS. Optional no-URL cases refuse source containing the old literal fallback before execution.
- The script explicitly rejects Windows; both new workflow call sites use ubuntu-latest. No repository-wide wildcard test invocation was found that would unintentionally call it in the Windows smoke job.
- Compiles are serialized by package and bounded at 240 seconds each with Go package concurrency 2; child startup has a 15-second timeout and the outer test a 900-second budget. A cold hosted run remains the place to validate timing, not a reason to remove checks.
- Release's declared database name, URL and readiness command agree; the identity query and migrations precede full Go tests. Existing release DAG, vulnerability scan and tag-scoped exception are preserved.
- Main CI includes the new source/config/test paths and explicitly sets Node22 before running the checks. Existing Go source changes already trigger the backend lane.

## Verification

Independently executed syntax/format checks on the reviewed bytes: `bash -n scripts/test-go.sh scripts/test-go.test.sh`; `node --check` for both JS tests; `gofmt -l` on the three Go TestMain files; scoped `git diff --check`. All exit 0 with no findings.

Reviewed retained implementation evidence rather than repeating costly compile/tests without a new failure:

- Wrapper green log: PASS.
- Workflow green log: 5 pass, 0 fail, 0 skip; baseline retained 2 new failures.
- Actual-binary precondition green log: 12 named child scenarios plus parent = 13 pass, 0 fail, 0 skip in Node's outer inventory. Two optional children separately assert one real helper pass and one intentional Go DB-test skip.
- Baseline retained 8 failing child scenarios; the previous unsafe testutil fallback was guarded before execution.
- Three-package Go vet completed with exit 0; the negative matrix compiled each of the three actual package test binaries successfully.

No live DB, provider, paid model, credentials, release, remote CI or network action was used by this reviewer. No edits were made to another lane or the original shared checkout.
