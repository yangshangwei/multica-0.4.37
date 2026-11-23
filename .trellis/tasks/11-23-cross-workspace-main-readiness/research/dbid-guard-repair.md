# UUID write-guard contract repair

2026-11-23. Only private `src/server/pkg/dbid/writes_test.go` changed. Base source is `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94`; production backend files, SQL, IDs and data were not changed. Parent approved the bounded test repair after the frozen browser run/rail red capture; core and backend lanes received a file-frozen signal before their next source binding. No staging or commit.

## Cause and preserved production contract

`TestEveryV7ParamsLiteralSetsAnIDAndImportsDBID` required a `dbid` import in every file containing a tracked params literal, even when the ID comes from a durable outbox row. Two new `inboxstate.DeliverTx(..., db.CreateInboxItemParams{ID: row.ID, ...})` callers correctly have no minting import:

- `internal/handler/project_update_notifications.go:153` receives a locked `db.ProjectUpdateNotification` row. Its ID was minted using `dbid.NewV7()` when the outbox entry was created (`handler/project_update.go:688`).
- `internal/service/iteration_notifications.go:158` uses the result of `q.LockIterationNotification`. Migration `550_iteration_tables.up.sql:81` gives the notification ID a database UUID default. `.trellis/spec/server/iterations.md:270` explicitly requires using that outbox ID as the inbox ID.

`inboxstate.DeliverTx` preserves a valid supplied ID, generates one only when absent, and dispatches these notification types to their dedicated inbox inserts. `CreateIterationInbox` and `CreateProjectUpdateInbox` use the same original ID with `ON CONFLICT(id) DO NOTHING`. Replacing `row.ID` with a new UUID just to satisfy an import heuristic would defeat replay identity and deduplication.

## Bounded guard change

The general missing-ID and minting-import checks remain. A literal may omit the file's minting import only at this precise AST boundary:

- direct third argument to `DeliverTx` from the actual imported `internal/inboxstate` package (aliases supported, shadowed package names rejected);
- generated `CreateInboxItemParams`, literal type `project_update` or `iteration`, and `ID` exactly from a row's `ID` field;
- matching generated notification row type on a parameter, or the first result of the matching `Lock…Notification` method on a `*db.Queries` parameter.

No filename whitelist, unconditional exception, import suppression or production compatibility layer was introduced. Eleven synthetic AST cases cover the two accepted shapes and nine refusals: missing ID, direct creation, newly generated/unproven ID, unknown row, mismatched notification row/type, ordinary inbox type, untrusted lock receiver, unrelated row read and shadowed delivery import. Aliased import identities are exercised.

This remains a syntactic maintenance guard, not a complete Go type/dataflow proof or a runtime UUID-version guarantee. A typed parameter is trusted by the guarded boundary; arbitrary later reassignment is not analyzed. The existing guard already allowed local variables whenever a minting import existed. Current row provenance and durable identity were checked against production code and SQL, while handler/service integration suites own transactional replay semantics. Future new shapes fail closed into the original import requirement rather than gaining broad inference.

## Verification

Pinned Go **1.26.6**, darwin/arm64. Normal/race commands use the existing Agent CLI guard; this pure AST package needs no database or real-agent credentials.

| Evidence | Result |
| --- | --- |
| `dbid-guard/red.jsonl` / `.exit` | Original full package reproduced exit 1: 4 top tests / 23 named terminals, 22 passed and the existing AST guard failed at the two durable callers. |
| `dbid-guard/normal.jsonl` / `.exit` | Full package after repair exit 0: **5 top tests / 35 named PASS**, no failure/skip. |
| Backend authoritative `backend/packages/bd354a329a15/test.jsonl` and `receipt.json` | Full package `-race -json -count=1 -p 1 -parallel 2 ./pkg/dbid`: **35 named PASS / 5 tops**, exit 0. Reviewer parsed the retained JSON and receipt. |
| `backend/dbid-final-command.json` | `race_exit=0`, `vet_exit=0`, identical source SHA-256 before/after. |
| Pinned `gofmt -l` and scoped `git diff --check` | Empty formatting output / success. |

All paths above are relative to `.omx/private/qa-work/main-readiness-20261123/`. The backend lane retained its original failure separately in `backend/dbid-original-failure/` and reconciled the original named failure to the successful full-package replay; this report does not overwrite historical outcomes.

The test population increases by one top-level matrix test plus its eleven subtests, **12 named terminals**, not eleven unrelated product cases. Root owns any concise spec note and aggregate ledger updates.

Final source SHA-256: `71b2ed4d293ef1872a0df052821480b9d897887a7d20a9b0db5515fe19b74360`.
