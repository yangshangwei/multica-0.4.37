# Progress preparation

2026-10-05, isolated worktree `codex/projects-p1`. Product implementation awaits the parent's FG evidence. Read the parent design, API contract, test specification, implementation ownership, backend/frontend maps, source boundary excerpt, root rules, server/triage specifications. Apply executing-plans and TDD within the parent-authorized execution lane.

## Interfaces and data

- Existing migration 536 contains the five new tables and required notification fields. 537–549 contain separate concurrent indexes. No added schema columns are needed for this domain: the worker derives title/actor and identity-only details from the project and immutable source revision.
- `sql-proposal.sql` contains proposed queries for foundation's sole SQL/sqlc ownership. Request lookup is workspace/actor/request-bound, while response lookup reasserts current project authorization. Each stored request points to its original revision.
- Preview/publish/correct use `runProjectTransaction`: first SQL sets RR READ WRITE, workspace/member fence/member locking read, then project and evidence locks. Source rows and mention member rows use NOWAIT to force a whole-transaction retry rather than trust a stale RR authorization view.
- Health supplies the shared nonrecursive statistics type and qtx collector. Acceptance description text and revision are persisted in each immutable revision; a correction preserves the prior acceptance version.
- Notification candidate selection has no row locks. Single-candidate RC delivery locks workspace, recipient fence/member, project, then outbox. Inbox insertion and outbox delivery share the commit; correction only creates rows for recipients never notified before.
- Existing util.ParseMentions recognizes member/agent/squad/all. Only explicit member mentions can become recipients; no call to comment/task/trigger APIs occurs.

## Test slices

1. Contract validation and deterministic canonical hash: normalization, Unicode limits, URL safety, optional nulls, unknown enums, evidence deduplication/order, timestamp exclusion.
2. Preview/create/replay/correct: real DB, immutable revision 1 after revision 2, stable timeline/cursors and historical identities, same UUID conflicts.
3. Acceptance and evidence: description CAS and immutable old acceptance corrections; issue revision and execution result/state changes; chat/private-agent authorization; stale authorization lock barriers; redaction on every read.
4. Notification durability: member/agent split with zero execution records, one stable inbox, new correction mentions, two workers, failure/backoff/dead-letter, cancellation, project delete and member revoke in both lock orders.
5. Shared health integration: same RR transaction actual snapshot, stale preview rejects, no-statistics publication unaffected by unrelated tasks, stored snapshot remains unchanged.

All DB fixtures use testutil/dbfx and the isolated `.env.worktree` database through the agent CLI guard. Real agent tools, production databases and external fetches are outside this test scope. No product behavior or tests are claimed complete by this preparation document.
