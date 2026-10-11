# Desktop core audit remediation

The user authorized fixing all ten findings in report.md. They declined a Trellis task for this session; this document records the bounded implementation and verification plan without creating one.

## Boundary

- Preserve existing layout, routing, permissions, drafts, data identity and business rules.
- Repair query error classification at the owning query/view boundary, keeping authorized cache and editor identity during transient failures. Definitive access loss must still remove protected content.
- Restore search and view selection at compact widths using existing controls and stores.
- Add named controls, visible keyboard focus, and non-drag manual ordering actions using existing mutation paths and group/sort constraints.
- Pair existing semantic foreground/background colors for project status and priority. Correct the audited online text contrast without changing the product palette.
- Await batch-delete outcomes, retain failed selections, and provide localized progress/results.
- No new dependencies, backend changes, native-service changes, broad refactors or product redesign.

## Ownership

1. Projects: projects-page, project-detail, project-badge, core/projects/config, related tests and projects locales (F02/F03/project F04/F05/F10).
2. Task and triage recovery: core/issues/canonical-id, issue-detail-route, issue-detail, triage-page and their tests; triage locales (F01/F08/task drawer F04).
3. Task interactions: list/board/swimlane reorder, task/My Issues compact headers, batch toolbar and related tests (F06/F07/batch F04).
4. Coordinator: chat controller/page/window recovery, inbox label, shared semantic success token if needed, shared issues locale integration, Electron evidence, final checks and spec update.

Owners must preserve pre-existing uncommitted work and avoid writing another lane's files. Issues locale changes are coordinated centrally.

## Verification

- Add meaningful regressions for HTTP 404 vs 500/offline, cached refresh failures, retries, retained editors/selection, batch partial failure, scope changes and reorder boundaries before behavior fixes where practical.
- Run targeted tests per lane, then package typechecks and lint for affected files; run locale parity and relevant desktop regressions.
- Reuse the isolated read-only Electron audit harness for error injection, keyboard focus, named controls, light/dark contrast, 900px window and native 200% zoom. Use isolated/mocked writes for reorder/delete verification, never mutate retained user data.
- Batch one visual review after implementation, fix confirmed remaining issues together, and perform at most one final confirmation pass.
- Persist screenshots, measured ratios, pass/fail evidence and a structured visual verdict. Re-run the detector once after all UI changes. Finish with a bounded polish review and a concise remediation report.
