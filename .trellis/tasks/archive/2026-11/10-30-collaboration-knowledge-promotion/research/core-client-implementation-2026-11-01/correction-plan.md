# B1 review corrections

Read-only review source: `../foundation-review-2026-11-01.md` and JSON. This correction does not change the review disposition; main owns independent recheck and closure.

Boundary:

- KF-01: classify only project/workspace knowledge routes as private for request/error logging in `api/client.ts`; preserve captured workspace headers and ordinary route behavior. Add request/error sentinel regressions for both target kinds and 4xx/5xx responses.
- KF-02: permit server-granted content-free rejection of pending source-unavailable candidates in `knowledge-schema.ts` and `knowledge-actions.ts`. Keep unavailable approval and restricted target-only rejection denied; retain private-field suppression and exact revision/reason requests. Cover expired pending rejection and original intent recovery.
- A2 producer alignment: require `history_truncated:boolean` on every declared-v1 detail, with no missing-field default. Keep the existing 50-row bound and same-lineage validation. Update fixture and API seam; source collection pages all candidate versions.

Record failing regressions before product edits, then run the affected canonical suites, core typecheck and scoped lint. Retain the prior 207-test baseline evidence and add separate correction logs, inventories and source hashes. No Go, views, router, provider, external-model calls, commits, or subagents.
