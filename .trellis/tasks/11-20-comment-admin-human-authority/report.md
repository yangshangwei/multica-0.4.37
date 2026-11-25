# Comment authority lane — implementation and required regression complete

## Behavior and files
`server/internal/handler/comment.go` changes only two owner/admin predicates to require `!isMachineCredentialActor(r)`. Authors keep existing self authority; human JWT/PAT owner/admin management remains. Machine owner/admin cannot edit/delete other human/agent comments, including attachment-only changes or spoofed actor headers. `actor_guards.go` and generic upload are unchanged.

New tests: `server/internal/handler/comment_authority_test.go`, `server/cmd/server/comment_authority_integration_test.go`. Security contract: `.trellis/spec/server/security-boundaries.md`. Required built-in skill/source map wording updated; original500-line cap unchanged and conformance verified. No new abstraction/dependency/migration, commit, push or provider execution.

## Evidence and limits
All new public records: `artifacts/qa/main-acceptance-fixes-20261125/comment/`; source/tools/config/resource bindings and exact command/inventory live in each run's intent/receipt/source.json. Original evidence remains readonly and hashed in original-evidence.json.

| Run | Exit | Named results | Meaning |
| --- | --- | --- | --- |
| existing-baseline |0|16PASS|Moving-checkout diagnostic only|
| red02 |1|220PASS/50FAIL|32 task+2 localCloud expected leaf permission failures;6 membership404 expectation errors; parents also counted|
| green01 |0|274PASS|Auth matrix; parent allocation assignment raced startup; explicitly not final acceptance|
| router01 |0|274PASS|Explicit router allocation; actual HTTP NewRouter/Auth; test-inserted task tokens|
| claim-legacy01 |0|134PASS|Actual daemon claim/start issuance, complete controls, API cancellation/revocation|
| claim-password01 |0|134PASS|Same matrix under password auth; no Provider|
| handler-normal01 |1|6150PASS/53SKIP/4FAIL|Frozen exporter omitted example plugin fixture; retained harness failure|
| handler-race01 |1|4575PASS/9SKIP|Package default10m timeout;17 started tests had no terminal event, retained in incomplete-tests.json|
| handler-normal02 |0|6200PASS/7SKIP|Full handler; corrected fixture closure, dedicated Redis, original40m/p1/parallel2|
| handler-race02 |0|6200PASS/7SKIP|Full handler race, same corrected conditions|
| router-normal01 |0|1751PASS/8SKIP|Full cmd/server, including all current actual-claim and side-effect tests|
| router-race01 |0|1751PASS/8SKIP|Full cmd/server race on fresh separate DB|
| skill-conformance01 |0|all template cases passed|Pure service conformance, no DB|

`go vet ./internal/handler ./cmd/server` passed with supported Go1.26.11 on the claimed-password frozen source. Gofmt and focused diff check passed. Original40m flags were restored from original read-only run_backend.py, not extended. All failed databases/logs are retained; no outcome rewritten.

Claimed-token matrices cover role owner/admin/member × JWT/PAT/mat_ × self/other human/other agent × body/attachment/delete/spoof. Denied writes compare full comment/attachment/issue/task/inbox rows and domain-event counts; a queued consumer proves refusal cannot cancel existing work. Human/self controls assert original authorship and revision2; stale CAS refuses without state change. Real mdt_ is denied by ordinary Auth. Actual claim/start credentials are never restored with manually inserted token rows. Fixture users/workspaces/agents/tasks remain DB setup; this is real issuance/auth/HTTP proof, not full public registration UI or provider execution.

Seven existing handler skips: one stub-query TODO, two iteration perf opt-ins, triage perf, triage wire exporter, live GitHub integration, and no HTTP MCP catalog recipe. Exact reasons saved in handler-normal02/skips.json. No new skip and no skipped test counted as PASS.

## Resource audit
External `/private/tmp/acceptance-fixes-4b9d3494a2/comment-*/` frozen source/HOME/TMP per run. Parent owns all PostgreSQL/Redis allocations. Tests shut down their httptest listeners. Final handler normal/race audit:0comments,0workspaces,5retained users each; databases retained, no speculative cleanup. Timeout race01 retains5users/1workspace/5comments.

Redis correction: handler test helper overrides URL suffixes10/11 to effectiveDB14 and FlushDB. Handler normal/race ran sequentially; auth11/middleware13/WeCom15 do not overlap effective handlerDB14. Router cache tests honor URL suffixes12/13, so parent must isolate router race13 from middleware13. Comment HTTP fixture uses nil Redis. See redis-effective-databases.json; original receipts retain configured indexes.

## Remaining integration gate
Full cmd/server normal01 is running, then race01 will follow sequentially. Earlier archived-fixture preflight concern was misclassified: rk1_runtime_continuation_test.go has agentintegration && !windows and is excluded from default suites. No fixture alias or source change is needed. Parent remaining packages now use dedicated Redis; router indexes12/13 retain their original allocation. Full router tests include final claimed-token and side-effect assertions.

Generic UploadFile comment_id association remains current same-workspace membership plus separate uploader provenance. No confirmed requirement equates it with comment management. It remains a separately reported product-contract question, not a fixed outcome. Local Cloud verifier tests do not establish deployed Fleet behavior. Existing event operator attribution is not a new durable edit audit log.

## Required command forms
Each full run consumes a frozen external source checkout, its parent-allocated JSON environment and the supported compiler. Exact command is `bash scripts/go-test-with-agent-cli-guard.sh go -C server test -json -count=1 -timeout=40m -p 1 -parallel 2 [ -race ] ./internal/handler` or `./cmd/server`. Both modes returned0 for both packages. Source/tool/config/resources are bound in per-run intent/receipt and the separate-final audit; no original log or result was replaced.

Parent resource ownership remains; this lane does not drop failed/successful databases or stop shared PostgreSQL/Redis. No ordinary API process remains: httptest listeners end with test process. No commits/push/deploy.

Final process audit: no remaining owned test/runner process. Shared PostgreSQL/Redis and all frozen snapshots/databases are retained for parent review. Final source/tool audit and resource report are separate-final-audit.json and final-resources.json. Independent final review is assigned to Nietzsche by parent.
