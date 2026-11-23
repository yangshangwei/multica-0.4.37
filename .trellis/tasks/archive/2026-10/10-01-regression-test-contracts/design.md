# Test contract alignment

Reuse Playwright TestApiClient and the existing Vitest harnesses. Current component semantics and persisted API results are the source of truth. Keep accessibility assertions precise and scope controls to the active interaction surface.

The canonical AI suite covers refinement, question merging/undo, cancellation, stale edits, responsive creation and queue persistence. Retire the old suite requiring an adoption step removed by the product, adding only missing valid coverage.

Prior failure reports are the baseline. Validate against a dedicated checkout so unrelated local product edits cannot contaminate the app build; mirror only this task's final test changes for execution.
