# Current ROOT server integration preparation

The new server graph preserves current ROOT and adds only the three reviewed P2 paths in a private copy. It does not overwrite ROOT with the old r06/P2 snapshot. Both `go build ./...` and `go vet ./...` exited 0 on this graph, with the full source/configuration unchanged before and after. Runtime acceptance remains pending.

## Source boundary

The exact Git tracked-plus-untracked server inventory contains 5,038 unique regular files. Two complete ROOT reads and the initial copy match in byte, size and mode. Before each P2 overlay, the three current ROOT hashes matched the original `6405f76f…`, `a968c375…` and `0aaf32df…` guards.

The only overlays are:

- `server/internal/handler/collaboration_stage.go`.
- `server/internal/handler/collaboration_stage_test.go`.
- `server/cmd/server/collaboration_stage_routes_test.go`.

Destination ROOT file modes are preserved separately from the private P2 source modes. All other 5,035 ROOT paths match. This includes the 35 recorded peer production differences, knowledge hooks/v2/default behavior, migration865, generated/schema/protocol changes, the virtual revision0/off stage-project fix and stronger ROOT tests. The 73 ROOT-only paths are included; three old private diagnostic helpers absent from ROOT are not silently injected.

Source: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/server-current-r07-r01/source`. The bare-array manifest SHA256 is `d1b63872d72ca66429f3f4a51b6efd4b5a670a57d8233adf8de4fa7fbaf730a0`.

## Actual metadata and static checks

The actual Go-list plus standard-library AST extraction finds 1,631 service tops, 3,066 handler tops and 486 router tops. Every original r06 selected top exists exactly once: 37 completion/exit/writer plus 79 foundation/review/report/M1, with no overlap, missing or duplicate name. Their new source positions and complete sets are recorded in `service-selected-inventory.json`, SHA256 `c4d54d3870796cad1d7ff436b1c42a36ef3a4f5327dcbfcb02e2ba2fa9a7bf8e`.

The scoped metadata configuration contains only the verified Go/path/private-state keys; no database or account configuration is inherited. The actual Node is the installed 22.23.2 executable. An initial assumed Node path was absent before any Go execution; its retained preparation failure and corrected executable proof are separate from the successful commands.

`go build ./...` and `go vet ./...` ran sequentially from the frozen server module and exited 0. Their original logs, commands, result receipts and complete before/after 5,038-file audits are under this private root's `static/`. Neither command executes `TestMain`; this slice ran zero Go tests, database connections, SQL, application listeners, browser or external model actions.

## Migration and runtime handoff

The graph includes the complete current server migrations, built-in skills and embedded documentation. Migration865 is a single concurrent-index build on existing knowledge columns: `collaboration_knowledge_candidate (automation_job_id, lineage_id) WHERE proposal_origin='automation'`. It does not introduce those columns. Its supporting knowledge schema comes from the current migration844 family; its down path first asserts retained automation history is empty before dropping the index.

ROOT must build/run the matching migrator on new task-owned environments, verify current ledger ordering and actual knowledge/stage schema/index shapes, and bind final API/CLI/Host binaries. An old 835/exact864 template is not proof that865 executed or that its index exists. This implementer performed no migration or database inspection.

This initial graph includes the `a3261a9d…` K06 test. Subsequent runtime evidence exposed its queue and fresh-refusal expectation errors, now prepared separately as test-only `80abb746…`. The `d1b63872…` graph and successful production static evidence remain unchanged. ROOT must seal a new final graph with the later reviewed router-test overlay and rebuild its test binary before final execution. The 35 peer production bytes and production build are not replaced by that test-only correction.

Four actual Web/Desktop scenarios, P2 runtime/source acceptance, A01–A12 and the parent matrix remain open. Static preparation does not declare current ROOT or full S2 accepted.
