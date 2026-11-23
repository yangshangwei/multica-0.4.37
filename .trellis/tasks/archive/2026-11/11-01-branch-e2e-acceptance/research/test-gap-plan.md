# Targeted acceptance gap

Boundary: add one isolated test under the frozen worktree .omx/branch-acceptance only. No tracked product/test source changes and no imported worktree repair.

ENV-1 lacks a UI assertion differentiating a retained run frozen at memory v0 from live memory v1. Reuse the current retained-handoff fixture, seed the explicit immutable base, write the newer document with the real API, attach the persisted base and live handoff projection, then assert the UI never describes v1 as this run startup. This verifies projection/UI correctness, not real process ingestion.

The implementation subagent failed with upstream 429 before producing files; the main session added this one bounded test. RECALL-1 remains covered by the existing audit/ancestry and Go regression matrix; no unsupported private runtime credential is synthesized.

Run with the task password environment, after the password suite releases its API: `pnpm exec playwright test --config=.omx/branch-acceptance/playwright.config.ts --workers=1 --retries=0 --reporter=line,json`.

## Workbench test interaction repair
A derived ignored copy retains every original assertion and timeout while using the existing current recipient-picker helper at its two transfer points. The original tracked spec is unchanged and both original failures remain. This verifies the complete original workflow instead of silently weakening or deleting failed assertions.
