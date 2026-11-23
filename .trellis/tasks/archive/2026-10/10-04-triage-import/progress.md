# CSV import lane progress

2026-10-05 — implemented and targeted verification passed; awaiting parent integration review.

## Changed files

- `server/internal/handler/triage_import.go`: frozen preview/get/commit/failures endpoints, strict request UTF-8 validation, scoped candidate resolution, durable batch/row planning and retry semantics, external-ID dedup, batch summaries, faithful failure CSV export.
- `server/internal/handler/triage_import_test.go`: twelve PostgreSQL integration tests including concurrency and the 1,000-row baseline.
- This progress artifact. No migrations, generated queries, core/UI files, or commits were changed by this lane.

The backend lane owns endpoint registration, shared intake/fences, candidate authority checks, outbox delivery, schema and history projections. The parent owns `internal/triagecsv` parsing and parser tests.

## Implemented contract

- Preview accepts valid UTF-8/BOM CSV only. Raw JSON bytes are validated before decoding; replacement characters from a lossy browser decode are rejected. Parsed CSV is limited to 5 MiB and 1,000 rows. NUL headers receive a whole-file error; NUL data cells remain per-row errors.
- The stable `(workspace, actor, request_id)` and payload hash replay an unchanged preview; changed payload/mapping requires a new request ID. Preview creates only batch and row records: no issues, tasks or notifications.
- Scoped project/issue-label names, member emails and visible agent names resolve to candidate IDs. Missing/foreign/ambiguous/private candidates are cleared with warnings. No project/label is auto-created. State/iteration columns are ignored with warnings. Similar titles are suggestions only.
- Each selected row takes workspace/settings/member fences before batch/row locks and uses a nested savepoint for intake. Current references and enabled state are revalidated. Invalid references or failed writes leave no orphan issue/counter change and retain a retryable row failure.
- Successful row identity survives issue deletion. Replay returns the same stored issue ID and never creates again. Repeated external IDs default to skip; explicit per-row override permits another intake, preserving row idempotency. Scoped advisory locks cover external-ID lookup and commit; same-row and cross-batch races are tested.
- Selected commit results/counters describe the requested rows. Unselected rows remain available for future selection. Full batch GET is the authority for cumulative outcomes; UI owner confirmed merge/refetch handling.
- Intake keeps project/assignee as candidates, formal status backlog, admission pending; no execution/mention events. CSV suppresses row notices. Stable batch/recipient outbox identity updates cumulative counts and one inbox summary per importer/configured recipient on retries.
- Failure export uses native CSV escaping, original source columns/cells, row number and readable error. Formula-like cells are neutralized. Source cells are stored as base64 JSON byte arrays under `normalized.cells`, because PostgreSQL JSONB cannot represent original NUL string values in invalid rows.
- External IDs over 1,000 UTF-8 bytes receive a preview row error to stay below the PostgreSQL btree entry limit for the `(workspace_id, external_id)` index. Filename is limited to 1,000 bytes and cannot contain NUL/newlines. Intake title/description limits come from the parent's common validation (500 runes/1 MiB).

## Verification evidence

Commands were run from the isolated worktree. Database tests source `.env.worktree` and acquire/release `/tmp/multica-triage-handler-test.lock`.

- `go test ./internal/handler -run TestTriageImport -count=1 -v` — PASS, twelve tests, final run 7.074s.
- `go vet ./internal/handler ./internal/triagecsv` — PASS.
- `go test ./internal/triagecsv -count=1` — PASS.
- `gofmt` and scoped `git diff --check` — clean.

Named regression coverage:

1. Inert durable preview, unchanged replay and conflicting request hash.
2. Partial success, duplicate skip/override, later unselected-row selection, retry and deleted-result replay.
3. Deleted project revalidation, faithful multiline/quoted/formula-safe failure CSV.
4. Human/owner access and strict encoding/size/row limits.
5. Project/label/member/visible-agent resolution, candidate-only semantics, provenance/reviewer, no execution, cumulative inbox summaries.
6. Foreign/ambiguous/private candidates, batch owner/foreign workspace access and membership revocation.
7. Label invalidation/repair retry, atomic failure, corrected cumulative summary.
8. Concurrent different batches with one external ID create exactly once by default.
9. 1,000-row preview and commit complete without record loss.
10. Disabled feature rejects preview/commit but preserves reads/download; malformed row selections cause no partial writes.
11. Concurrent same-row retries return one issue ID; overlong indexed external IDs fail in preview.
12. Original NUL data cells survive failure download without entering JSONB as illegal strings.

## Baseline and remaining integration work

Final local PostgreSQL run for one 1,000-row file: preview **655.7 ms**, commit **5.142 s**. Earlier runs were preview603–642ms, commit5.16–6.36s. This is a local small-workspace baseline, not the PRD's populated-workspace P95 certification.

Parent must run the full cross-lane audit/tests, populated-workspace performance baseline and Web/Desktop visual/user-flow checks. Agent candidate locking/visibility behavior was reviewed with the backend owner; its shared helper remains under that lane's verification ownership. No production deployment or commit performed here.
