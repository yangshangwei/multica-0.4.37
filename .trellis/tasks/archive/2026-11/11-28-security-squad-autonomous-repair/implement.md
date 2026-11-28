# Implementation plan

## Approval
The preceding six-part plan was approved by the user: “请按照修复方案直接修改”; task tracking was approved separately. Record and execute that plan without repeating permission questions.

## Ordered work
- [x] Trace restricted reviewer return and record exact integration in design.md.
- [x] Implement backend controlled return with behavioral regression tests (backend agent owns runtime source/tests).
- [x] Update squad/leader workflow, local diagnostic/test role+skills and template contract tests (template agent).
- [x] Synchronize bilingual docs, skill source maps, catalog wording/tests and generated docs (documentation agent).
- [x] Verify targeted tests, compile/vet, affected UI checks and independent review; repair findings.
- [x] Update specs and explicit saved-instance upgrade evidence, record remaining operational limits.
- [x] Commit only task-owned changes with Lore decision trailers and close task.

## Checks
Read database-test-isolation.md before fixture execution. Root allocates databases and provides exact package permits; children do not create/migrate/drop databases. First run new regressions where feasible before implementation. Keep restricted comments inert, preserve status ownership, verify retries/dedupe and current-authority refusal. Run Go compilation/static analysis, relevant template/handler/service integration tests, catalog unit tests and docs generation checks. Retain actual test inventory, including failures and skips.

## Existing work and rollback
This checkout has extensive unrelated edits; targeted source files were clean at initial inspection except unrelated project/resource skills. Do not reset or stage unrelated work. Revert only this task's identified hunks if needed. Do not modify production services or databases. Saved custom instructions require an explicit, inspected instance update.
