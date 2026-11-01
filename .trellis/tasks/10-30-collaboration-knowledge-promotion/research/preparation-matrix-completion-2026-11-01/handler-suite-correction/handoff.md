# Handler full-suite fixture correction

The approval `503 knowledge_unavailable` was caused by fixture interference in `TestKnowledgeMemberRemovalFansOutHistoricalTargetsOutsideSource`. It published immutable history into the shared `testWorkspaceID`, then `InsertNoID` cleanup deleted that workspace's revision counter. Later approvals correctly refused to initialize a fresh counter over retained publication history.

Only `server/internal/handler/collaboration_knowledge_invalidation_test.go` changed: this test now creates a second, unique `knowledgeHookWorkspace` as its external target and uses that target for seeding, publication, and the historical fanout assertion. The source and target remain separate. No production guard, shared cleanup behavior, or other test source changed. `fixture.patch` records the complete correction.

| Run | Result |
| --- | --- |
| Ordered red | Historical-target test passes; human roundtrip and daemon fixture both reproduce `knowledge_unavailable`; exit 1. |
| Same ordered tests after correction | 3 tests pass; zero failures/skips. |
| Full handler package | 5,254 passing test events, 2,811 top-level passes, 4,797 passing leaf cases; zero failures; 53 skips. All 61 knowledge test events pass without skips. |
| All handler knowledge tests with race detector | 25 top-level passes, 54 passing leaf cases, 61 passing test events; zero failures/skips. |
| Static checks | `go vet ./internal/handler`, `gofmt -l`, and scoped `git diff --check` pass. |

Each command used `/tmp/multica_knowledge_c2_20261101_7d95f46f.env`, checked nonempty `DATABASE_URL`, and ran exclusively on `multica_knowledge_c2_20261101_7d95f46f`. Database runs were sequential. `*.command.json` records actual argument arrays, working directories, UTC start/end timestamps, process results, log hashes, and complete run/pass/fail/skip inventories. `run.py` is the evidence recorder used for these invocations.

The full-suite skips comprise 46 tests requiring `REDIS_TEST_URL` plus seven existing stub/opt-in/export/catalog cases. Exact names and reasons are retained in the JSON log. This run does not substitute for main's Redis or opt-in integration coverage. No Provider calls, commits, sqlc generation, or access to other agents' databases occurred.

The original twelve-leaf preparation matrix is unchanged. Both original test source hashes and all four final normal/race log hashes were rechecked. `../historical-command-provenance.json` supplies the previously missing exact original commands and timestamps, recovered from native session tool calls and original Go package events. It distinguishes tool dispatch time from package start/end and identifies the session, record line, call ID, and record hash; no timestamp was inferred from file modification time.

Source edits are frozen. Main owns integrated acceptance and final task status.
