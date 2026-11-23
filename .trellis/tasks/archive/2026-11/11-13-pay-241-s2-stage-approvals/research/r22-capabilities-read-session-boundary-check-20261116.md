# R22 r03 persisted-session query boundary

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Verdict — ACCEPTED for one fresh RED input

The separately frozen r03 test-only count correction and fresh command input are **accepted, zero blocking findings**. Actual RED outcomes, a product GREEN, the original R20 holder and full S2 acceptance remain unobserved/separate. All original source/results/reviews are preserved unchanged.

## Correct query and actual session identity

Test SHA256 `d9535bfa9f9965517391fc3ba1255ed54970b6b249b049f5d0f79f67a4cc37ea` differs from r02 only at line51: query `collaboration_run.session_id=$1` receives `f.issue.SessionID`. The zero-run assertion/error message and every other test byte remain unchanged; mode0644 is retained.

This is the **persisted session belonging to the same fixture issue/R4 root**:

1. `newManagedCollaborationFixture` stores the actual result of `CollaborationService.Create`.
2. Creation calls `CreateCollaborationSession(created.Issue.ID)` and returns `ResolveIssueAccess` for the same created issue before committing.
3. `GetCollaborationAccess` joins `collaboration_session s ON s.issue_id=i.id`, with the same requested issue, and selects `s.id AS session_id`.
4. `ResolveIssueAccess` maps that UUID directly to `CollaborationIssueAccess.SessionID`. The inner join and nonnull session ID do not fabricate a missing session.
5. `newStageRuntimeFixture` and its stage/R4 fixture share the original returned queue issue/session; `ownerSwitchNamespace` copies that issue/session without changing it. The actual R4 requests still use its `f.issue.Issue.ID` and exact candidate.

Migration571 defines run.session_id and session.issue_id;620 retains current run.session_id. Existing production `ListCollaborationRuns` uses the same `WHERE session_id=$1` predicate. This corrected effect count therefore scopes the intended fixture root, rather than an unrelated runtime/child or a nonexistent run column.

## Preserved scope and inventory

Manifest SHA256 `0798b58ba06ca14cc3fb7c3eab50b7c61fb4ea87069776c7c44a9668ff977c4f` records5047 files with the same path set. The new test is the sole changed byte/mode entry; all5046 baseline manifest entries remain identical. The prior full source audit is reused; no broad physical graph re-audit was performed. The unchanged execution runner must recheck the current complete source before/after its command.

Inventory SHA256 `46ee80a5cb6691106b79ed0fbf43b10676f5515e46e386f8c5bec78ccecb28e2` stays4 top-level/8 named/7 leaf with the anchored original selector. Valid FileCount31, actual same-QA password/binding and artifact R4 prerequisites, GET/writer post-account `LockAgentRuntime` barriers, five-second contexts, real distinct PID checks, release/cancel/drain cleanup and all writer/authority assertions remain unchanged.

## Fresh command and runner

Draft SHA256 `203dbd1bdaccb8c5c813fe45e0ab0e916d4c83a7904b46e3bca317a773159131` has12 matching inputs, including the original setup-failed result and outcome review. It binds corrected `source-control-r03/server`, the unchanged `go test -json ./internal/service` anchored/count1/five-minute command, full eight names/two expected failed leaves, fresh `multica_pay241_20261116_s2_capread_red_r22r03` and absent `profiles-r03/red-r03` output. It requires the coordinator's final review binding and fresh source/DB/resource guard.

New runner SHA256 `8e2f18954d9bd36659e8c46db2e487ce7c639931c16f9962408aaa4683a4d8b2` changes only three allowlist lines: explicitprofiles-r03 profile/output paths and the one fresh RED database. Old consumed runner SHA `a7f084ad…` is unchanged. Exact argv/source/input/environment guards, serial slot, fresh exclusive logs, JSON inventory/error parsing and after source/database audits remain identical. Python syntax was parsed without importing/executing the runner.

The fresh0600 environment and clone receipts match their hashes: recorded zero other connections,853 ledger entries/663 indexes/211 table counts equal the original donor. Current trusted HOME/temp chain metadata matches, all TMP keys share its0700 temp and offline Go settings are retained. No current DB query was performed here; the coordinator owns execution readiness and PAY/global suite budgets.

## History and next gate

The original invalid-summary NOT_ACCEPTED review, earlier corrected static review and actual setup-blocked result/outcome remain unchanged. In particular, the prior run's8 named failures at42703 remain its actual result; this correction does not reclassify them as expected RED.

Bind this accepted supplement separately and execute r03 RED once after the fresh guards. Require actual progression through both read-order failures, the real passing registration writer and all four passing authority leaves, then independently reconcile full inventory/source/database after before considering a product GREEN.

Only this Markdown and its JSON companion were written. No Go/tests/DB/API/build/process/source/Git action or old report edit was executed. No business acceptance count advanced.
