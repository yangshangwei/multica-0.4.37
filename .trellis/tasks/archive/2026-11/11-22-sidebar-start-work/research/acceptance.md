# Sidebar start-work acceptance

The approved three-entry shared header is implemented and accepted. New issue and AI Chat show the authoritative selected workspace; collaboration shows cross-workspace/Beta. Ordinary creation from account scope waits for the real target provider, API mirror and initiating API/session/viewer, then opens once. Chat retains its native route, selected state and unread-message semantics, including its accessible positive count. Creation preference, draft marker and collaboration source/focus return remain intact.

## Validation

- Initial task-owned behavioral checks: 9 Views files / 186 tests and 3 Core files / 22 tests passed. Reviewer added unread-description regressions (sidebar/parity 124 tests), additional navigation/auth/storage boundary checks (83 tests), and the anchored-history correction (4 files / 30 tests). Counts overlap and are not summed as a unique total.
- Final main-session sidebar/registry/parity check: 3 files / 128 tests passed against the latest original source.
- Forced Core/Views/Web/Desktop lint and typecheck: 11 tasks successful, zero cache hits. Post-fix Views typecheck and changed-file eslint passed. Existing warnings outside the task were retained.
- Repository UI wildcard export/static check passed; task context validation and git diff whitespace checks passed. Impeccable's detector returned no findings.
- Production Web and Desktop rebuilt from the final isolated source snapshot. Workspace package links resolve that snapshot; external installed dependencies are reused without adding packages. `acceptance.json` records runtime hashes, URLs, base commit and exact Web build ID.
- Final Playwright: **6 passed**, **0 failed**, **0 skipped**, **0 retries**, 44.8 seconds. Covers actual Electron plus production Web, direct ordinary/collaboration creation, correct workspace request headers, one popup identity/no replay, AI Chat, both locale/theme combinations, compact keyboard/text geometry, original query-error recovery and saved 620px/800px return positions.
- Screenshots were inspected as one wide/compact batch. Final visual verdict 95/100, pass. Long workspace captions intentionally ellipsize with full accessible descriptions; primary labels and focused controls remain unclipped.

## Diagnoses retained

The first ordinary-modal assertion tried to read an inert background navigation link; it now checks after close. Native links intentionally render absolute shareable URLs, so the browser helper compares pathname. Cached legacy issue GETs are not guaranteed; after a cold reload the read-only table rows channel verifies the workspace header.

The original anchored return regression exposed an existing shared-detail omission. Both platform adapters set history traversal separately from scope resume. Trace evidence proved departure at 800px, then the new returning detail explicitly focused description and moved to 325px. A new existing-detail test reproduced the missing restoration before the minimal combined guard fix; the original 800px E2E expectation now passes. Temporary instrumentation was removed. The navigation/detail specs preserve this executable contract.

## Boundaries and remaining coverage

No backend, database schema, new dependency, mobile UI or chat-to-task conversion was changed. Real model/daemon execution, installer delivery, physical touch and spoken assistive technology are outside this menu increment. Wider E2E selectors affected by the concurrent Settings-label commit are listed in check.md and remain unrelated work; this task does not claim the entire repository E2E suite passed.

Task-owned test services were shut down cleanly. Other running environments and pre-existing dirty files were left intact. The final report and extracted geometry/scope evidence are under `artifacts/qa/sidebar-start-work-20261122/accepted/`.
