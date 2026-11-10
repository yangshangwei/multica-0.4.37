# Completion writers at05de761de
Read-only native completion_path_research mapped current code; no shared gate exists.

| Writer | Owner | Boundary / gap |
|---|---|---|
| ordinary PUT and move | handler/issue.go updateIssueAtomicallyOnce; issue_move.go | issue UPDATE + project SHARE; optional revision CAS; safe conflict retries |
| batch | issue.go BatchUpdateIssues | invokes ordinary writer per issue; partial commits, not all-batch atomic |
| create done / comment sub-issue | service/issue.go CreateInTx; handler/source_context.go | catalog, parent/project, counters and insert same transaction; no pre-existing own artifact |
| GitHub/VCS | handler/github.go advanceIssueToDone; service/issue_status_write.go WriteIssueStatus | separate RC tx and PR aggregate; no project policy lock/human evaluator |
| collaboration | service/collaboration_lifecycle.go ChangeLifecycle/prepareCompletion/changeLifecycle/recordStatusChange | workflow epochs/idempotency/exact submission or self-result+closure; own transaction |
| R38 admin collaboration | service/admin_collaboration.go ChangeLifecycle | direct changeLifecycle bypasses prepareCompletion; must not fabricate initiator acceptance |

Triage accept restricted backlog/todo; autopilot/onboarding/channel create fixedtodo; collaboration create nonterminal; background restoretodo; child-done only wakes parent. Lifecycle handoff writes metadata/comments, not completion status.
Custom status key/category immutable; Resolve under catalog shared lock validates active target. Do not compare raw done alone or use tolerant Effective as admission validation. Current observer POST already rejects independent collaboration via loadIssueForUser/rejectLegacyCollaborationAccess; source-member human/evaluator authority cannot be silently widened to cross-account acceptance.

Future decisions: private evaluator sharing/system completion; skip and R38; create-done evidence; done→done/edit/project move semantics; old-client refusal and batch partial-result protocol. Future policy lock must be held in EACH actual completion transaction, never accept this hint as a ticket.
