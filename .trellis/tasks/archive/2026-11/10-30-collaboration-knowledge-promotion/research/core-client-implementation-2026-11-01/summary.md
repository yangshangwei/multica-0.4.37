# B1 implementation and verification

Completed the core human knowledge client seam: strict supported/unsupported parsing; typed source, detail, target and original-operation endpoints; immutable request intent and content-free recovery identities; exact revision writes; finite monotonic projection leases; current-epoch Query clearing; and target/source/disconnect lifecycle handling. No HTTP routes or views were enabled.

Verification on the source hashes in `source-manifest.json`:

- 207 tests passed across 13 focused suites; zero failures and zero skips. `vitest.json`, `test-inventory.json`, and `tests.log` retain every test name/result.
- Full `@multica/core` TypeScript check passed.
- ESLint passed on all 23 changed/new core source and test files.
- `git diff --check -- packages/core` passed.
- Exact argv, working directory, exit status and duration are in each `*-command.json`; HEAD, branch, Node/pnpm/platform and scope are in `verification.json`.

The suites include seven new feature suites plus existing API client, source transport, WebSocket, collaboration realtime and access regressions. Pure schema/lease/cache/action policy runs in Node. The one jsdom suite proves the real hook → existing React WSProvider → WSClient close path clears a mounted target-only private summary without polling or a source subscription.

The cancellation regression initially reproduced private data restoration when `cancelQueries()` used TanStack's default rollback. Clearing `Query.state.data` alone was insufficient. Reusing the repository's manual `query.setData(undefined, {manual:true})` pattern also replaces its saved rollback snapshot; the named default-rollback regression now passes. The final tests also cover pending 202 recovery, HTTP/receipt contradictions, source-unavailable quoted decision text, original actor recovery after fresh login, and storage failure before transport.

Simplifications: one pure shared source/access event schema; one ref-counted knowledge responder per connection/scope; one finite lease policy for source/detail/target pages. Conservative scope clearing handles hidden and not-yet-loaded source/target relationships without a second entity graph. The storage record is only client-generated recovery identity; server projections and full request text are never persisted.

Changed files are inventoried exactly in `source-manifest.json`: new `collaboration/knowledge-*` modules/tests, shared realtime schema, knowledge API/realtime tests, and narrow edits to API client, WSClient/account test, existing realtime/provider wiring, event union and collaboration barrel. Existing dependency trees were symlinked from the original checkout; no dependency or lockfile changed. No commits, Go/SQL/daemon/router/view edits, external model calls or provider-account actions were performed by this B1 lane.

Remaining integration work is explicit in `api-seam.md`: HTTP producer alignment (including bounded lineage history semantics and cursor authorization boundaries), shared view adoption and recovery UI, durable backend target invalidation, G2 runtime/session enablement, and actual Web/Desktop product acceptance. The current results verify core behavior only.
