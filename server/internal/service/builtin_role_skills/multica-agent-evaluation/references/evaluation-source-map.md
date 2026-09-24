# Evaluation source map

| Contract | Implementation | Regression |
| --- | --- | --- |
| Versioned six-category evidence, finite correctness verdicts, safety blocked | `server/internal/service/lifecycle_handoff.go` `ValidateAgentEvaluation` | `lifecycle_handoff_test.go` `TestValidateAgentEvaluationCorrectnessVerdicts`, existing version/category and fixture tests |
| Failed correctness cannot produce a passing HTTP handoff | `server/internal/handler/lifecycle_handoff.go` `CreateLifecycleHandoff` | `lifecycle_handoff_atomic_test.go` `TestLifecycleAtomicFailedCorrectnessCannotPass` |
| Evidence, audit and optional issue/task commit together; whole metadata limit 8 KiB | `server/internal/handler/lifecycle_handoff_transaction.go`; `server/internal/service/issue_task_transaction.go` | `lifecycle_handoff_atomic_test.go` rollback, budget, concurrency and idempotent task tests |

The API retains actual-result values for existing safe-stop cases. It does not execute paid model evaluations or production actions.
