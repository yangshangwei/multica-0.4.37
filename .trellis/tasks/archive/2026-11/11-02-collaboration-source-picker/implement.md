# Execution plan

1. Review the PRD/design and curate real spec/research context before activating the task.
2. Implement independent lanes with native Trellis implementers:
   - UI lane owns the shared parent selector, creation/workflow query-state wiring, English/Chinese copy, and their focused views regressions.
   - Server lane owns complete-identifier matching in the source-parent query/service, sqlc regeneration and isolated backend regressions.
   - Each lane writes a failing regression before repair, reports exact commands and preserves unrelated edits.
3. Integrate both lanes and run the Trellis checker against only task-owned changes. Verify meaningful shared tests, affected package TypeScript/lint, `gofmt`, backend tests and `go vet`; use scoped static analysis without cleaning unrelated reported debt.
4. Exercise the resulting creation form in an owned production browser environment: click without search, title/number/complete-identifier filtering, clear/reopen, selection/clear, keyboard/Escape, transient failure/retry and narrow/wide English/Chinese captures. Use the visual-verdict contract before any visual follow-up edit and preserve verdict JSON.
5. Capture the source-picker/search contract in relevant Trellis specs, record evidence/progress, review the final staged diff, and commit only this task's files with the Lore intent/trailers. Archive and journal after verification.

No unrelated worktree changes, global test-service gates, installed AI CLIs or current user accounts are part of this task.
