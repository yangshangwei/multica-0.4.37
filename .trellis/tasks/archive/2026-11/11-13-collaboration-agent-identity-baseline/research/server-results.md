# S1 server implementation evidence

Implemented the optional public `author_agent` summary in the shared comment projection. Only trusted Agent author IDs qualify; deduplicated names use the existing batch query. Missing Agent rows retain the ID and empty name. Human authors and runtime read/write protocols are unchanged.

Changed files: `server/internal/service/collaboration_comments_view.go`, service `collaboration_comments_test.go` and `collaboration_comments_recall_test.go`, handler `collaboration_comments_test.go`. No dependency, migration, permission, or runtime write changes.

Named service regressions: 53 tests/subtests passed (no skips/failures). Handler regressions: 9 tests passed (no skips/failures). Both ran sequentially through the coordinator's identity-checking private environment wrapper on separate owned databases. The new tests first failed on the missing summary/query, then passed. Full exact inventories and source/log hashes are in `server-results.json`.

Coverage includes authenticated runtime comment-to-run-to-Agent attribution, retained history after grant/target revocation and archive, current names, hard-deleted Agent fallback, lost reader access, no lookup for human-only reads, propagation of true lookup errors, minimal cross-workspace HTTP fields, and public list/detail/byte-budget pagination with escaped long names and placeholder recovery.

`git diff --check` and gofmt passed. Explicit `go vet -p 2 ./internal/service ./internal/handler` passed in the isolated candidate. Focused `-race` checks then passed sequentially: service 3 top-level cases / 6 including subtests, handler 1 case; no failures/skips. All four server files matched the root before and after checks. No code changes were required. Logs: `server-vet.log`, `server-service-race.jsonl`, `server-handler-race.jsonl`. This is server protocol/DB evidence; Web/Electron and real-process acceptance belong to the coordinator.

The initial compile attempt also captured a corrected test int64 comparison and a concurrent F2 reportID shadowing error. The F2 owner fixed its own file; this lane did not alter it. Original failure logs remain.
