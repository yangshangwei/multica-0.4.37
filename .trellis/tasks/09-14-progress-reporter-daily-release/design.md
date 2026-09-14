# Progress reporting design

## Approved direction

Extend the existing server-embedded registries. A one-off workspace Agent would not meet the requested built-in catalog behavior; a new squad or execution subsystem would add unnecessary workflow. The user approved the dedicated reporter approach and explicitly asked to implement and release it.

## Role

Add one listed `progress-reporter` role with localized title/description, canonical Simplified Chinese instructions, and one embedded `multica-progress-report` role skill. Use Contributor autonomy so it can close out its assigned report issue, with narrower instructions that prohibit changes to source tasks and repository files. Concurrency is one: daily and weekly reports share a stable process without competing runs. Agent creation stays on the existing from-template endpoint and copies server-decided content atomically.

## Reporting templates

Place the new daily template directly before weekly in the server roster; current UIs already preserve roster order. Use `create_issue`, `0 18 * * *`, and `每日进展报告 — {{date}}`. Weekly remains `0 17 * * 1` with the existing dated title. Daily is version 1; update weekly to version 2 only for the clarified evidence/closeout contract. Existing saved automations and agents remain workspace-owned and are not overwritten.

Both templates require a report even when no progress exists, state their reporting window, and write onto the already created report issue. After the comment succeeds, only this issue moves to `in_review`; this uses the existing event-driven automation completion path. A failed publication or incomplete data must be disclosed rather than fabricated. No new scheduler/status behavior is introduced.

## Evidence and permissions

The skill explains bounded/paginated reads and status history rather than equating `updated_at` or current `done` with period completion. It follows workspace/project scope and reports data truncation. Counts cite underlying task identities and explicitly define closed states. Reporting artifacts are excluded from business throughput. External systems require actual configured access; default reporting covers Multica data only.

## Verification and delivery

Registry/content tests establish the product defaults; real DB handler tests establish creation/provenance/claim behavior. Browser E2E uses isolated real APIs/runtime fixtures to prove the catalogs, creation and dispatch, with report-comment and state-transition checks. It must clearly distinguish fixture execution from actual LLM quality evaluation. Visual QA compares the same shared catalog style against the provided screenshots.

Release follows the existing fork runbook. All `gh` operations specify the fork explicitly; desktop builds use `--publish never` to avoid upstream publication. Verify backend and frontend versions inside the offline images. Prefer a durable version-input fix over an undocumented temporary build overlay if the current builder still stamps the frontend `dev`. Use regular pushes and a new tag, never force-update a published tag.
