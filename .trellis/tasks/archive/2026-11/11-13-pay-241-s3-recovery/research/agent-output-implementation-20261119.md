# Agent output implementation — 2026-11-19

Status: implementation and static checks are complete in the allocated private Go module. Database, native Provider/CLI, current-client and full S3 acceptance remain pending Main-owned execution. This report does not mark R02, R09 or S3 complete.

The candidate is `/Volumes/artisan/.multica-acceptance/pay241-s3-output-20261119-tm89sjgy/source/server-work-r01`. The companion JSON binds all 31 owned changed files, 28 new test names, source-manifest hash and actual static-check receipts. ROOT server was read-only for this child. Main's exact route RED file remains byte-equal to `server-red-r01`; its SHA256 is `db50f2c0ddef0de1b4be1ace3924a75e539d425c45b388f9f27b046efe7c1f9d`.

## Implemented behavior

- `POST /api/collaboration/runtime/runs/{runId}/comments/attachments` accepts the original `comment_id`, `content`, optional `parent_id` and one file. It rejects uploader/run/attempt/source/draft overrides. UUID parsing and zero-ID refusals precede writes; the existing two byte slots and 100 MiB file bound are retained.
- The current mcb, real Agent/run/authorized attempt and original source/group/grant/reservation are checked before storage and again inside the final runtime transaction. Filename, actual sniffed type, length, SHA-256, content/parent and actual attempt form the immutable input. JSON versus multipart reuse conflicts in either order; old no-file event bytes remain unchanged.
- Existing private `Storage`/`secretbox` ciphertext is reused. File metadata, permanent issue/comment refs, actual Agent comment, original `comment:<UUID>` event/receipt, squad delegation/wake effects and notifications use one existing lock-owned comment writer. The new receipt includes typed private metadata with `uploader_agent_id`, `run_id` and canonical `attempt_seq`; human uploader/draft are null on Agent output, and added provenance is omitted for existing human rows.
- Runtime callback retries reset object ownership. Definite losers remove only their own random object; a possibly inserted object is retained only on the existing unknown-commit marker. Each run/current attempt admits at most 32 committed output rows under the run lock. Outcome closure rejects new outputs.
- First outcome commit stores all current-attempt output IDs in deterministic order as flat server-owned `output_attachment_ids`. A shared stored decoder serves both original ACK paths, checkpoint/retention/seal, both records paths, test-report logs and handoff. Original Provider/Runner/ledger/request wire is unchanged. Result and comment records project the original IDs through `attachment_ids`; no read dynamically infers outcome files.
- Runtime file reads authorize exact own run/Agent/attempt output or original frozen input. A run with no input snapshot receives `attachment_not_found` for both unfrozen and missing input IDs. Real malformed/query snapshot failures remain errors. The existing handler final row/current-authority check before bytes is retained.
- CLI `issue comment add --attachment` accepts one explicit bounded regular file through the invocation CWD's pinned `os.Root`, rejects URL/multiple/escaping/special/oversized files and external/profile overrides, preserves the original `--request-id`, submits one multipart POST and verifies the complete typed comment/file receipt. No journal, automatic key replacement, retry or human profile was introduced.
- Migration 868 adds only nullable Agent/run/attempt attribution with exact human/Agent XOR/private-row constraints. It adds no FK, cascade or index. Its single locked down block refuses retained or concurrently written output before restoring the human constraints. sqlc refreshed attachments/delivery/preparation scanners and the model.
- Built-in `multica-working-on-issues` guidance and source map describe the one-file workflow, original intent recovery and CWD limit.

## Static checks actually run

The closed `configs/nodb-r01.json` and recorded helper launched every command. `output-sqlc-r01` exited 0 and changed only the four generated mirrors. `output-compile-packages-r03` used genuine `go test -c` for service, handler, CLI, cmd/multica, cmd/migrate and cmd/server; `output-vet-packages-r02` checked the same six packages. Both exited 0 with zero source/config/tool drift. `output-gofmt-r03` also exited 0 with no remaining source changes. Earlier compile/fmt/vet records are retained.

No test body, database migration/query, runtime, Provider executable, browser, git or journal operation was executed by this implementer. Compilation is not a functional PASS.

## Owning-layer execution inventory

| Package | New top-level count | Exact new filter |
|---|---:|---|
| `./internal/service` | 17 | `^TestCollaborationRuntimeOutputO[0-9]{2}` |
| `./internal/handler` | 2 | `^TestCollaborationRuntimeOutputHTTP` |
| `./internal/cli` | 4 | `^TestCollaborationOutputO09` |
| `./cmd/multica` | 3 | `^TestCollaborationCLIOutputO09` |
| `./cmd/migrate` | 2 | `^TestCollaborationRuntimeOutputMigration` |

The 28 tops cover O01–O09 in their owners: Agent/ciphertext/reference identity; original unknown commit; same-input convergence; changed fingerprint races and JSON aliases; required-write rollback and rollback→other winner→unknown replay cleanup; storage/read revocation and non-disclosing input refusals; concurrent cap32, host old-attempt refusal/current freeze, terminal and explicit new-run failure preservation; original full outcome ACK, checkpoint/seal/report/records/handoff and legacy bytes; CLI CWD/path/override/multipart/typed receipt/unknown response/redirect controls. Main's immutable router top is separate. Actual named/subcase inventories must be recorded by Main's fresh package/mode runs.

## Remaining handoff

Main owns fresh normal/race databases, actual migration/route GREEN, complete named inventories and gate/connection/retained-history cleanup. The independent implementation check and a new controlled Provider using the actual CLI, matching production Web and Desktop output journey remain required. Current compilation or prior controlled R03 evidence does not accept this new output capability, ROOT integration, R02/R09 or full S3.
