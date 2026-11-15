# Backend progress

Candidate: `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`. Current run: `normal-race-1`.

- 82 default packages inventoried; 10,413 statically declared selected Test/Fuzz identities. These are planned inventory, not pass totals.
- First package `internal/service` completed: **1,442 top-level PASS; 3,430 including subcases PASS; 0 FAIL; 0 SKIP**. Every expected top-level name appeared. Go package terminal PASS, elapsed 2,125.812 seconds.
- Exact source inputs and Go/Node/Bash hashes remained unchanged after the service run.
- Parent runner PID 71181 was SIGSTOP-paused by root request while service child continued. All service/go/guard children have now exited; handler and following packages have not started. Root requested a quiet window for an independent RK1 run.
- `01-internal_service/run.json` remains running until the paused parent resumes its receipt/write step; actual terminal event and independent observation are in `stdout.jsonl` and `paused-parent-observation.json`. Do not mistake the coordinator pause for an unfinished service test.
- Next action: only after root releases the quiet window, verify exact parent identity and send SIGCONT. The unchanged runner will record service receipt, execute handler, router and remaining packages, then vet.

## Evidence

- `evidence/backend/prepare/`: migrate compile and private-runner static checks.
- `evidence/backend/normal-race-1/coordination-pause.json`: owned parent pause.
- `evidence/backend/normal-race-1/01-internal_service/`: raw Go JSON and terminal observation.

No product files changed. Real-model execution deferred explicitly; performance and tagged integration are separate lanes.
