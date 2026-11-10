# Actual task cancellation intersection

Added only `server/internal/handler/agent_artifact_cancel_test.go`. The tests attach a real chat session to the task and call production `TaskService.CancelTaskInTx`; this is the existing transaction core used by `CancelTaskWithResult`. Its completed update before caller commit provides a natural barrier, with no hand-written task lock substituting for cancellation.

Cancel first: the core returns changed/cancelled/completed_at, another connection still sees running, controlled execution gets409 authority busy, zero claim and zero calls to the real packager. Commit makes cancelled visible; another admission returns403 and still no pack call.

Claim first: hold the real adapter at Pack entry after durable unknown commit, perform and commit the real cancellation, then release the original adapter. Its nonce owner records succeeded/legacy executed, replay retains execution/receipt, actual Pack count remains1, and task status remains cancelled.

Normal `cancel-red-01` and race `cancel-race-01` both2pass/0fail/skip. The initial label is not a claim of red failure. Source hash and raw records are in verification.json. Separate HTTP cancellation auth and postcommit events remain covered by existing suites, not claimed by these two transaction intersection tests. Handler database window released to API owner after race completed.
