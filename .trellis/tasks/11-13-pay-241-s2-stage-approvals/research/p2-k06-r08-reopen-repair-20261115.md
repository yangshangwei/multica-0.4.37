# K06 reopen and unused-grant contract corrections

Prepared a new private test-only source for the next canonical run. The r08 case reached real M1 completion, the corrected public workflow GET, three successful P2 policy changes and a successful reopen, then failed an incorrect expectation that reopening made the execution queue `open`. The existing product intentionally reopens into `paused` with `issue_reopened`; it requires an explicit later Resume.

The immutable r08 raw SHA256 is `79b48d0b2306c96c0f65fc24c5600dd3baf94e8d0d6ea4af609e72ff8e37c0d2`. That run did not reach the later fresh unused-grant request. Its result, source and database20 remain retained.

## Two bounded corrections

1. `ReopenCollaborationSession` in the existing lifecycle SQL writes `paused`, `issue_reopened` and one new control version. The workflow also advances the lifecycle once and pauses its targets. The test now checks the non-done issue, exact lifecycle increment, paused/reason fields, and unchanged run, command, root-task and consumed-terminal counts. A passive immediate log captures those actual fields during the next run. No Resume is added.
2. The fresh request uses current target/lifecycle observations with an old unused M1 authorization. The existing `completionMutationDigest` compares the complete collaboration target before reaching the stage guard. Its conflict is `delivery_completion_conflict` with HTTP 409; `completionWorkflowRefusal` preserves that code. ROOT chose that exact existing contract. The test preserves one new bounded refusal, no pending-grant terminal, and unchanged original GET/body-key replay and all other business facts. It adds no second request or permissive `OneOf` assertion.

The earlier public-DTO correction remains. All original actor, authorization/terminal, P2 enable-disable-reenable, role/scope absence, archive, exact operation/body/key, historical recovery and side-effect assertions remain. The K06 tail after its recovery baseline differs only by replacing the disproven refusal code.

## Source and preparation evidence

- New source: `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/p2-k06-repair-current-r03/source`.
- Complete manifest: 4,968 files, SHA256 `80abb746c9c905bbd54e73db05bfbce0c84b02f9057b7c77b67632e3496549a1`.
- Parent: the preserved `a3261a9d…` source. Only `server/cmd/server/collaboration_stage_routes_test.go` changes; all 4,967 other bytes/modes match.
- Formatting, actual three-package Go-list and the existing standard-library AST extractor each exited 0. K06 remains unique at line 213. A full post-metadata path count confirms exactly 4,968 regular files.
- This implementer executed no new Go test, database query, SQL control, API or browser. Queue/reason and fresh refusal behavior await ROOT-bound execution; the static source explanation is not a new runtime PASS.

The old `a3261a9d…` source, its failed run/database, and the separate current-ROOT `d1b63872…` graph are unchanged. ROOT owns the new normal22 and previously unused race21 configuration binding. The companion JSON binds the patch, source contracts, raw failure and command receipts. P2 and full S2 acceptance remain open.
